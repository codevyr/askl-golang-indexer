package indexing

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
)

// TestRealProjectIsNumbered runs the whole pipeline over a real Go module and
// checks the containment tree the parser actually produces.  The unit tests in
// pkg/index drive ProtoIndex directly with hand-written ranges; this is the one
// that would catch a shape go/types emits and we never imagined -- a method on
// an embedded type, a closure inside a composite literal, two declarations the
// AST reports with identical extents.
//
// It asserts the same invariants askld's upload validator enforces, so a
// project that passes here is one the server will accept.
func TestRealProjectIsNumbered(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	write("go.mod", "module example.com/m\n\ngo 1.21\n")
	// Shapes worth covering: a func holding a closure holding another closure,
	// a method on a struct, a var initialised by a func literal, and a second
	// package in a nested directory so the directory chain is more than one
	// level deep.
	write("main.go", `package main

import "example.com/m/pkg/a"

type Counter struct{ n int }

func (c *Counter) Bump() int {
	c.n++
	return c.n
}

var Hook = func(x int) int {
	inner := func(y int) int { return y * 2 }
	return inner(x)
}

func Run() int {
	c := &Counter{}
	step := func() int {
		deeper := func() int { return a.Target() }
		return deeper() + c.Bump()
	}
	return step()
}

// Chained and indexed calls: the callee of mk().Bump() is a selector whose
// receiver is ANOTHER call, so a token taken from callExpr.Fun would span it.
var reg = map[string]*Counter{}

func mk() *Counter { return &Counter{} }

func Chain() int { return mk().Bump() + reg["k"].Bump() }

// A grouped declaration: one range, two names, two closures.  Giving each name
// the whole spec made the first childless and handed the second BOTH closures.
var First, Second = func() int { return Run() }, func() int { return Run() + 1 }

func main() { _ = Run() + Hook(1) + Chain() + First() + Second() }
`)
	write("pkg/a/a.go", `package a

func Target() int { return 7 }
`)

	indexPath := filepath.Join(t.TempDir(), "index.pb")
	if err := ParseModules([]string{root}, root, Options{
		ProjectName: "numbering-e2e",
		IndexPath:   indexPath,
		ParseTypes:  true,
	}); err != nil {
		t.Fatalf("ParseModules: %v", err)
	}

	payload, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	var project indexpb.Project
	if err := proto.Unmarshal(payload, &project); err != nil {
		t.Fatalf("unmarshal index: %v", err)
	}
	if len(project.Objects) == 0 {
		t.Fatal("index has no objects")
	}

	symByID := make(map[int64]*indexpb.Symbol, len(project.Symbols))
	for _, s := range project.Symbols {
		symByID[s.LocalId] = s
	}

	type node struct {
		enter, exit int64
		level       int32
		fold        int32
		start, end  int32
		name        string
	}

	numberedObjects, numberedInstances, numberedRefs := 0, 0, 0
	nameSpans, sigSpans, contentSpans, fullSpans := 0, 0, 0, 0
	for _, obj := range project.Objects {
		where := obj.FilesystemPath
		if len(obj.SymbolInstances) == 0 {
			t.Errorf("%s: object has no instances at all", where)
			continue
		}

		nodes := make([]node, 0, len(obj.SymbolInstances))
		for _, i := range obj.SymbolInstances {
			name := "?"
			if s := symByID[i.SymbolLocalId]; s != nil {
				name = s.Name
			}
			nodes = append(nodes, node{
				enter: i.DfsEnter, exit: i.DfsExit, level: i.NestingLevel,
				fold: i.FoldDepth, start: i.StartOffset, end: i.EndOffset, name: name,
			})
		}
		sort.Slice(nodes, func(a, b int) bool {
			if nodes[a].enter != nodes[b].enter {
				return nodes[a].enter < nodes[b].enter
			}
			return nodes[a].exit > nodes[b].exit
		})

		var stack []node
		prevEnter := int64(-1)
		for _, n := range nodes {
			if n.exit-n.enter < 1024 {
				// Covers the all-zero case too, which is what an index produced
				// before the containment tree existed looks like.
				t.Errorf("%s: %s interval [%d, %d) is narrower than one DFS stride",
					where, n.name, n.enter, n.exit)
			}
			if n.enter == prevEnter {
				t.Errorf("%s: %s shares DfsEnter %d with another instance",
					where, n.name, n.enter)
			}
			prevEnter = n.enter

			for len(stack) > 0 && stack[len(stack)-1].exit <= n.enter {
				stack = stack[:len(stack)-1]
			}
			if len(stack) > 0 && n.exit > stack[len(stack)-1].exit {
				p := stack[len(stack)-1]
				t.Errorf("%s: %s [%d, %d) straddles out of %s [%d, %d)",
					where, n.name, n.enter, n.exit, p.name, p.enter, p.exit)
			}
			if int(n.level) != len(stack) {
				t.Errorf("%s: %s says NestingLevel %d but sits %d levels deep",
					where, n.name, n.level, len(stack))
			}
			if n.fold != 0 {
				t.Errorf("%s: %s has FoldDepth %d; nothing folds in Go",
					where, n.name, n.fold)
			}
			stack = append(stack, n)
			numberedInstances++
		}

		// A finer span must sit inside the extent it subdivides: the upload path
		// only rejects an inverted span, so one naming bytes outside its
		// instance would be stored and later shown as another symbol's text.
		for _, i := range obj.SymbolInstances {
			for what, sp := range map[string]*indexpb.Span{
				"name": i.Name, "signature": i.Signature, "content": i.Content,
			} {
				if sp == nil {
					continue
				}
				if sp.Start > sp.End || sp.Start < i.StartOffset || sp.End > i.EndOffset {
					t.Errorf("%s: %s span [%d,%d) is not inside extent [%d,%d)",
						where, what, sp.Start, sp.End, i.StartOffset, i.EndOffset)
				}
			}
			if i.Name != nil {
				nameSpans++
			}
			if i.Signature != nil {
				sigSpans++
			}
			if i.Content != nil {
				contentSpans++
			}
		}

		// Range nesting implies interval nesting: no Go construct can produce
		// the co-extensive synthesised symbols that forced the DFS encoding, so
		// here the two must agree wherever one range strictly contains another.
		for a := range obj.SymbolInstances {
			ia := obj.SymbolInstances[a]
			for b := range obj.SymbolInstances {
				if a == b {
					continue
				}
				ib := obj.SymbolInstances[b]
				if !(ia.StartOffset <= ib.StartOffset && ib.EndOffset <= ia.EndOffset &&
					(ia.StartOffset != ib.StartOffset || ia.EndOffset != ib.EndOffset)) {
					continue
				}
				if !(ia.DfsEnter < ib.DfsEnter && ib.DfsEnter < ia.DfsExit) {
					t.Errorf("%s: range [%d,%d) strictly contains [%d,%d), but interval [%d,%d) does not contain [%d,%d)",
						where, ia.StartOffset, ia.EndOffset, ib.StartOffset, ib.EndOffset,
						ia.DfsEnter, ia.DfsExit, ib.DfsEnter, ib.DfsExit)
				}
			}
		}

		for _, ref := range obj.Refs {
			var owner *node
			for k := range nodes {
				n := &nodes[k]
				if n.enter < ref.DfsEnter && ref.DfsEnter < n.exit &&
					(owner == nil || n.enter > owner.enter) {
					owner = n
				}
			}
			if owner == nil {
				t.Errorf("%s: ref [%d,%d) at DfsEnter %d lies outside every instance interval",
					where, ref.FromOffsetStart, ref.FromOffsetEnd, ref.DfsEnter)
				continue
			}
			if ref.NestingLevel != owner.level+1 {
				t.Errorf("%s: ref [%d,%d) says NestingLevel %d, but its interval puts it inside %s at level %d",
					where, ref.FromOffsetStart, ref.FromOffsetEnd, ref.NestingLevel,
					owner.name, owner.level)
			}
			if ref.FoldDepth != 0 {
				t.Errorf("%s: ref [%d,%d) has FoldDepth %d", where,
					ref.FromOffsetStart, ref.FromOffsetEnd, ref.FoldDepth)
			}
			// `full` encloses the token, the opposite containment from an
			// instance's finer spans.  `a.Target` inside `a.Target(x)`.
			if ref.Full != nil {
				fullSpans++
				if ref.Full.Start > ref.FromOffsetStart || ref.FromOffsetEnd > ref.Full.End {
					t.Errorf("%s: full span [%d,%d) does not contain its own token [%d,%d)",
						where, ref.Full.Start, ref.Full.End, ref.FromOffsetStart, ref.FromOffsetEnd)
				}
				if ref.Full.End > int32(len(obj.Content)) {
					t.Errorf("%s: full span [%d,%d) runs past the end of a %d-byte object",
						where, ref.Full.Start, ref.Full.End, len(obj.Content))
				}
			}
			if !(owner.start <= ref.FromOffsetStart && ref.FromOffsetEnd <= owner.end) {
				t.Errorf("%s: ref [%d,%d) was numbered inside %s, whose extent is [%d,%d)",
					where, ref.FromOffsetStart, ref.FromOffsetEnd, owner.name,
					owner.start, owner.end)
			}
			numberedRefs++
		}
		numberedObjects++
	}

	// The container chain, outermost first: module, directory, file.  The order
	// is this producer's choice -- the engine's traversal never names a
	// container type -- but it has to be a nested CHAIN, not three roots.
	for _, obj := range project.Objects {
		if obj.Filetype == "directory" {
			continue
		}
		fileLen := int32(len(obj.Content))
		type link struct {
			kind  indexpb.SymbolType
			level int32
			enter int64
		}
		var chain []link
		for _, i := range obj.SymbolInstances {
			if i.StartOffset != 0 || i.EndOffset != fileLen {
				continue
			}
			kind := indexpb.SymbolType_SYMBOL_TYPE_UNSPECIFIED
			if sym := symByID[i.SymbolLocalId]; sym != nil {
				kind = sym.Type
			}
			chain = append(chain, link{kind, i.NestingLevel, i.DfsEnter})
		}
		sort.Slice(chain, func(a, b int) bool { return chain[a].enter < chain[b].enter })
		want := []indexpb.SymbolType{
			indexpb.SymbolType_MODULE,
			indexpb.SymbolType_DIRECTORY,
			indexpb.SymbolType_FILE,
		}
		if len(chain) != len(want) {
			t.Errorf("%s: %d instances cover [0,%d), expected the chain %v",
				obj.FilesystemPath, len(chain), fileLen, want)
			continue
		}
		for k, got := range chain {
			if got.kind != want[k] || int(got.level) != k {
				t.Errorf("%s: chain link %d is %v at level %d, expected %v at level %d",
					obj.FilesystemPath, k, got.kind, got.level, want[k], k)
			}
		}
	}

	// Co-extensive instances of the SAME kind cannot be ordered by containment,
	// so whatever lies inside their shared extent is handed to exactly one of
	// them -- arbitrarily, with the other left a childless leaf.  That is how
	// `var First, Second = f1, f2` ended up with Second owning First's closure.
	//
	// The container chain is exempt because kindRank orders it; two instances
	// of the same rank have nothing to break the tie.  Sharing an extent with
	// nothing inside it is harmless and common: a grouped declaration's doc
	// comment is attached to every name it documents, and a comment contains no
	// code.
	for _, obj := range project.Objects {
		for a := range obj.SymbolInstances {
			ia := obj.SymbolInstances[a]
			for b := range obj.SymbolInstances {
				if b <= a {
					continue
				}
				ib := obj.SymbolInstances[b]
				if ia.StartOffset != ib.StartOffset || ia.EndOffset != ib.EndOffset {
					continue
				}
				if rankOf(symByID, ia) != rankOf(symByID, ib) {
					continue // the module/directory/file chain
				}
				for _, inner := range obj.SymbolInstances {
					if inner == ia || inner == ib {
						continue
					}
					if inner.StartOffset >= ia.StartOffset && inner.EndOffset <= ia.EndOffset &&
						(inner.StartOffset != ia.StartOffset || inner.EndOffset != ia.EndOffset) {
						t.Errorf("%s: %s and %s share the extent [%d,%d) and %s lies inside it, so one of them gets it arbitrarily",
							obj.FilesystemPath, nameOf(symByID, ia), nameOf(symByID, ib),
							ia.StartOffset, ia.EndOffset, nameOf(symByID, inner))
					}
				}
			}
		}
	}

	// STATED PARENTAGE.  `var First, Second = f1, f2` gives both names one
	// range, and First's identifier is not contiguous with its own initialiser,
	// so no extent can say that f1 is First's and f2 is Second's.  The parser
	// knows, and says so; the numbering honours it even though the child's range
	// lies entirely OUTSIDE the parent's, which is what the DFS encoding is for.
	//
	// Before this, both names took the whole spec as their extent, became
	// co-extensive, and the sort order handed the last of them every closure in
	// the declaration: `First { ... }` found nothing while `Second { ... }`
	// found both -- one of them First's.
	for _, obj := range project.Objects {
		if filepath.Base(obj.FilesystemPath) != "main.go" {
			continue
		}
		owner := map[string]string{} // closure symbol -> the instance containing it
		var closures []*indexpb.SymbolInstance
		for _, i := range obj.SymbolInstances {
			if sym := symByID[i.SymbolLocalId]; sym != nil && strings.Contains(sym.Name, ":<anon") {
				closures = append(closures, i)
			}
		}
		if len(closures) < 2 {
			t.Errorf("expected the fixture's grouped declaration to contribute closures, found %d", len(closures))
		}
		for _, c := range closures {
			var best *indexpb.Symbol
			var bestEnter int64
			for _, p := range obj.SymbolInstances {
				if p.DfsEnter < c.DfsEnter && c.DfsEnter < p.DfsExit && p.DfsEnter > bestEnter {
					best, bestEnter = symByID[p.SymbolLocalId], p.DfsEnter
				}
			}
			name := symByID[c.SymbolLocalId].Name
			if best == nil {
				t.Errorf("%s has no containing instance", name)
				continue
			}
			owner[name] = best.Name
		}
		// A closure's name is derived from whoever claimed it, so the two have
		// to agree: a closure named after First that First does not contain
		// means the name and the tree came from different places.
		for name, got := range owner {
			// The LAST segment: a closure inside a closure is named
			// `Run:<anon262>:<anon287>` and belongs to `Run:<anon262>`, not to
			// Run.  Those come out right from the ranges alone; the grouped
			// declaration is the case that needs the stated parent.
			want := name[:strings.LastIndex(name, ":<anon")]
			if got != want {
				t.Errorf("%s is contained by %s, expected %s: the closure's name and its place in the tree disagree",
					name, got, want)
			}
		}
	}

	// Per-kind, read off the bytes.  The global counts above cannot tell which
	// producer emitted them, so dropping every span from named func
	// declarations still left them non-zero -- one kind's spans standing in for
	// another's is exactly the regression "optional" invites.
	wantSpans := map[string]struct{ name, sig, content string }{
		"example.com/m.Run": {
			name:    "Run",
			sig:     "func Run() int",
			content: "{", // body starts at the brace
		},
		"(*example.com/m.Counter).Bump": {
			name:    "Bump",
			sig:     "func (c *Counter) Bump() int",
			content: "{",
		},
		"example.com/m.Counter": {
			name:    "Counter",
			sig:     "", // types carry no signature span, by decision
			content: "struct{ n int }",
		},
	}
	seenSpans := map[string]bool{}
	for _, obj := range project.Objects {
		for _, i := range obj.SymbolInstances {
			sym := symByID[i.SymbolLocalId]
			if sym == nil {
				continue
			}
			want, ok := wantSpans[sym.Name]
			if !ok || i.InstanceType == indexpb.InstanceType_DOCUMENTATION {
				continue
			}
			seenSpans[sym.Name] = true
			text := func(sp *indexpb.Span) string {
				if sp == nil {
					return ""
				}
				return string(obj.Content[sp.Start:sp.End])
			}
			if got := text(i.Name); got != want.name {
				t.Errorf("%s: name span is %q, expected %q", sym.Name, got, want.name)
			}
			if got := text(i.Signature); got != want.sig {
				t.Errorf("%s: signature span is %q, expected %q", sym.Name, got, want.sig)
			}
			if got := text(i.Content); !strings.HasPrefix(got, want.content) || want.content == "" && got != "" {
				t.Errorf("%s: content span is %q, expected it to start with %q", sym.Name, got, want.content)
			}
		}
	}
	for name := range wantSpans {
		if !seenSpans[name] {
			t.Errorf("%s is not in the index, so its spans were never checked", name)
		}
	}

	// The point of the ref change: a call's token is the function, not the whole
	// call.  Check it on the fixture's own text rather than on a count, since a
	// count cannot tell `a.Target` from `a.Target(c.Bump())`.
	for _, obj := range project.Objects {
		if filepath.Base(obj.FilesystemPath) != "main.go" {
			continue
		}
		for _, ref := range obj.Refs {
			if ref.Full == nil {
				continue
			}
			tok := string(obj.Content[ref.FromOffsetStart:ref.FromOffsetEnd])
			whole := string(obj.Content[ref.Full.Start:ref.Full.End])
			// The token is the IDENTIFIER that names the callee -- `Target`, not
			// `a.Target` and certainly not `mk().Bump`.  Any bracket in it means
			// it swallowed a receiver expression, and a receiver that is itself
			// a call is a separate reference nested inside this one.
			if strings.ContainsAny(tok, "()[]") {
				t.Errorf("reference token %q includes call or index syntax; from_offset_* is the identifier alone", tok)
			}
			if !strings.Contains(whole, tok) || !strings.HasSuffix(whole, ")") {
				t.Errorf("full span %q is not a call containing the token %q", whole, tok)
			}
			if ref.Full.Start > ref.FromOffsetStart || ref.FromOffsetEnd > ref.Full.End {
				t.Errorf("full span %q does not enclose its own token %q", whole, tok)
			}
		}
	}

	// The parser pulls in builtin/unsafe/cmp alongside the fixture, so the exact
	// counts drift with the toolchain.  Floors only: enough to prove the walk
	// reached real nested code rather than passing on an empty index.
	if numberedObjects < 4 || numberedInstances < 20 || numberedRefs < 5 {
		t.Errorf("suspiciously little was checked: %d objects, %d instances, %d refs",
			numberedObjects, numberedInstances, numberedRefs)
	}
	// The span checks above pass vacuously on an index with no spans, and
	// "optional" is exactly the shape that stops being emitted without anyone
	// noticing.  The fixture has named funcs (all three spans), func literals
	// (signature and content, no name), interface-free types and a var -- so
	// each kind must appear.
	if nameSpans == 0 || sigSpans == 0 || contentSpans == 0 || fullSpans == 0 {
		t.Errorf("the span assertions were vacuous: %d name, %d signature, %d content, %d full spans",
			nameSpans, sigSpans, contentSpans, fullSpans)
	}
	t.Logf("validated %d objects, %d instances, %d refs; %d name / %d signature / %d content / %d full spans",
		numberedObjects, numberedInstances, numberedRefs, nameSpans, sigSpans, contentSpans, fullSpans)
}

// TestGitOnlyFilesAreNumbered covers the objects the Go parser never walks: a
// README, a Makefile, an empty file.  --include-git-files adds them with
// AddFile(nil, ...) -- no module, so their whole object is a directory
// instance and a file instance, co-extensive, and for an EMPTY file both are
// [0, 0).  The C indexer's equivalent of these were all left at [0, 0) and
// would have failed upload, which is what W1 fixed; nothing would have told us
// here either, since no query names a README.
func TestGitOnlyFilesAreNumbered(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("go.mod", "module example.com/m\n\ngo 1.21\n")
	write("main.go", "package main\n\nfunc main() {}\n")
	write("README.md", "# m\n\nnothing to see.\n")
	write("EMPTY", "")

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v failed (%v), skipping: %s", args, err, out)
		}
	}
	run("init", "-q")
	run("add", ".")
	run("commit", "-qm", "init")

	indexPath := filepath.Join(t.TempDir(), "index.pb")
	if err := ParseModules([]string{root}, root, Options{
		ProjectName:     "git-files",
		IndexPath:       indexPath,
		ParseTypes:      true,
		IncludeGitFiles: true,
	}); err != nil {
		t.Fatalf("ParseModules: %v", err)
	}
	payload, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	var project indexpb.Project
	if err := proto.Unmarshal(payload, &project); err != nil {
		t.Fatalf("unmarshal index: %v", err)
	}

	want := map[string]bool{"README.md": false, "EMPTY": false}
	for _, obj := range project.Objects {
		base := filepath.Base(obj.FilesystemPath)
		if _, ok := want[base]; !ok {
			continue
		}
		want[base] = true
		if len(obj.SymbolInstances) == 0 {
			t.Errorf("%s has no instances", base)
			continue
		}
		levels := map[int32]int{}
		for _, i := range obj.SymbolInstances {
			if i.DfsExit-i.DfsEnter < 1024 {
				t.Errorf("%s: interval [%d,%d) is narrower than one DFS stride",
					base, i.DfsEnter, i.DfsExit)
			}
			levels[i.NestingLevel]++
		}
		// A directory instance containing a file instance: a chain, so each
		// level holds exactly one.  Two instances at level 0 would mean the
		// file came out BESIDE its directory rather than inside it, and
		// `dir { file }` would be empty.
		for lvl, n := range levels {
			if n != 1 {
				t.Errorf("%s: %d instances at NestingLevel %d, expected a chain",
					base, n, lvl)
			}
		}
	}
	for base, seen := range want {
		if !seen {
			t.Errorf("%s never reached the index, so nothing was checked", base)
		}
	}
}

func nameOf(byID map[int64]*indexpb.Symbol, i *indexpb.SymbolInstance) string {
	if s := byID[i.SymbolLocalId]; s != nil {
		return s.Name
	}
	return "?"
}

// rankOf mirrors the producer's kindRank: the container chain is ordered by
// kind, everything else only by its range.
func rankOf(byID map[int64]*indexpb.Symbol, i *indexpb.SymbolInstance) int {
	s := byID[i.SymbolLocalId]
	if s == nil {
		return 3
	}
	switch s.Type {
	case indexpb.SymbolType_MODULE:
		return 0
	case indexpb.SymbolType_DIRECTORY:
		return 1
	case indexpb.SymbolType_FILE:
		return 2
	default:
		return 3
	}
}
