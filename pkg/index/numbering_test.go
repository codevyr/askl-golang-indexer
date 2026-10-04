package index

import (
	"fmt"
	"go/token"
	"sort"
	"testing"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
)

// fixture builds a project with every shape the numbering pass has to get
// right: a nested directory chain, two files in one module, declarations nested
// three deep, a reference at offset 0 (where the file instance also starts), a
// reference deep inside a closure, and the directory sentinels with their
// [0, 0) objects.
func fixture(t *testing.T) *indexpb.Project {
	t.Helper()

	idx, err := NewProtoIndex(WithProject("p"), WithRootPath("/src"))
	if err != nil {
		t.Fatalf("NewProtoIndex: %v", err)
	}
	defer idx.Close()

	mod, err := idx.AddModule("example.com/m")
	if err != nil {
		t.Fatalf("AddModule: %v", err)
	}

	// 220 bytes, so the file instance is [0, 220).
	mainSrc := make([]byte, 220)
	mainPath := "/src/m/main.go"
	mainID, err := idx.AddFile(&mod, "/src", mainPath, "go", mainSrc)
	if err != nil {
		t.Fatalf("AddFile main: %v", err)
	}

	// Two levels of directory below the root, so the chain is longer than the
	// co-extensive dir/module/file triple.
	libSrc := make([]byte, 80)
	libPath := "/src/m/pkg/a/a.go"
	libID, err := idx.AddFile(&mod, "/src", libPath, "go", libSrc)
	if err != nil {
		t.Fatalf("AddFile lib: %v", err)
	}

	at := func(path string, off int) token.Position {
		return token.Position{Filename: path, Offset: off, Line: 1, Column: off + 1}
	}
	// Every declaration carries a name span at its first 4 bytes and a content
	// span over its second half, so the finer spans are exercised wherever the
	// instances are.
	decl := func(file FileId, path, name string, start, end int, st SymbolType) {
		t.Helper()
		if _, _, err := idx.AddSymbol(mod, file, name, ScopeGlobal, st,
			InstanceTypeDefinition, at(path, start), at(path, end), Spans{
				Name:    NewSpan(start, start+4),
				Content: NewSpan((start+end)/2, end),
			}, NoParent); err != nil {
			t.Fatalf("AddSymbol %s: %v", name, err)
		}
	}

	// main.go: two top-level funcs; the first holds a closure, which holds a
	// var -- three levels of strict range nesting.
	decl(mainID, mainPath, "m.Run", 10, 120, SymbolTypeFunction)
	decl(mainID, mainPath, "m.Run.func1", 40, 90, SymbolTypeFunction)
	decl(mainID, mainPath, "m.Run.func1.v", 55, 60, SymbolTypeData)
	decl(mainID, mainPath, "m.Helper", 130, 200, SymbolTypeFunction)
	// A type whose range equals the second func's: two instances with the same
	// range and the same kind rank contain nothing of each other and must come
	// out siblings.
	decl(mainID, mainPath, "m.HelperAlias", 130, 200, SymbolTypeType)

	decl(libID, libPath, "a.Target", 20, 70, SymbolTypeFunction)

	// A ref at offset 0 -- an import sits at the very start of the file, which
	// is where the file, module and directory instances all start too.
	if err := idx.AddReference(mainID, at(libPath, 20), "a.Target",
		at(mainPath, 0), at(mainPath, 6), nil); err != nil {
		t.Fatalf("AddReference at 0: %v", err)
	}
	// A ref deep inside the closure.
	// With a surrounding construct: `full` encloses the token, which is the
	// opposite containment from an instance's finer spans.
	if err := idx.AddReference(mainID, at(libPath, 20), "a.Target",
		at(mainPath, 70), at(mainPath, 76), NewSpan(70, 80)); err != nil {
		t.Fatalf("AddReference in closure: %v", err)
	}
	// A ref that sits in m.Run AFTER the closure inside it has closed, with no
	// later instance to pop the closure off the sweep's stack.  The innermost
	// OPEN instance there is the closure's local var, which does not contain
	// this ref -- so an owner lookup that trusts the top of the stack instead
	// of testing containment attributes it to a variable five levels down.
	if err := idx.AddReference(mainID, at(libPath, 20), "a.Target",
		at(mainPath, 100), at(mainPath, 106), nil); err != nil {
		t.Fatalf("AddReference after closed sibling: %v", err)
	}
	// A ref whose range is exactly its owner's: legal, and the case that tells
	// "place refs before this instance" from "place refs at or before it".
	if err := idx.AddReference(libID, at(libPath, 20), "a.Target",
		at(libPath, 20), at(libPath, 70), nil); err != nil {
		t.Fatalf("AddReference co-extensive: %v", err)
	}

	if err := idx.ResolveReferences(); err != nil {
		t.Fatalf("ResolveReferences: %v", err)
	}
	return idx.Upload()
}

type node struct {
	enter, exit int64
	level       int32
	fold        int32
	folded      bool
	start, end  int32
	name        string
}

// foldStep mirrors the producer's own increment, so the assertions state the
// schema's rule rather than the value it happens to take in Go.
func (n node) foldStep() int32 {
	if n.folded {
		return 1
	}
	return 0
}

// sortedNodes returns an object's instances outermost-first in DFS order: exit
// DESC breaks ties so a parent precedes a child starting at the same number.
func sortedNodes(t *testing.T, proj *indexpb.Project, obj *indexpb.Object) []node {
	t.Helper()
	byID := make(map[int64]*indexpb.Symbol, len(proj.Symbols))
	for _, s := range proj.Symbols {
		byID[s.LocalId] = s
	}
	nodes := make([]node, 0, len(obj.SymbolInstances))
	for _, i := range obj.SymbolInstances {
		name := fmt.Sprintf("symbol %d", i.SymbolLocalId)
		if s := byID[i.SymbolLocalId]; s != nil {
			name = fmt.Sprintf("%s (%s)", s.Name, s.Type)
		}
		nodes = append(nodes, node{
			enter: i.DfsEnter, exit: i.DfsExit, level: i.NestingLevel,
			fold: i.FoldDepth, folded: i.Folded,
			start: i.StartOffset, end: i.EndOffset, name: name,
		})
	}
	sort.Slice(nodes, func(a, b int) bool {
		if nodes[a].enter != nodes[b].enter {
			return nodes[a].enter < nodes[b].enter
		}
		return nodes[a].exit > nodes[b].exit
	})
	return nodes
}

// TestEveryInstanceFormsAValidTree is the Go mirror of the clang indexer's
// ObjectRoots.EveryInstanceFormsAValidTree, restating every invariant askld's
// upload validator enforces.  Checking them at the producer is what keeps the
// two from drifting: a tree that fails here would be rejected at upload, and
// the engine reads only a child's DfsEnter, so a malformed tree produces
// plausible wrong answers rather than errors.
func TestEveryInstanceFormsAValidTree(t *testing.T) {
	proj := fixture(t)
	if len(proj.Objects) == 0 {
		t.Fatal("fixture produced no objects")
	}

	for _, obj := range proj.Objects {
		where := obj.FilesystemPath
		nodes := sortedNodes(t, proj, obj)
		if len(nodes) == 0 {
			t.Errorf("%s: object has no instances", where)
			continue
		}

		var stack []node
		prevEnter := int64(-1)
		for _, n := range nodes {
			if n.exit <= n.enter {
				t.Errorf("%s: %s has empty interval [%d, %d), which can never contain anything",
					where, n.name, n.enter, n.exit)
			}
			// One full stride between enter and exit, which numbering by stride
			// gives for free.  That gap is where query-time rows go, so a
			// narrower instance cannot hold its own search matches -- they land
			// past its DfsExit and silently stop being its descendants.
			if n.exit-n.enter < dfsStride {
				t.Errorf("%s: %s interval [%d, %d) is narrower than one stride",
					where, n.name, n.enter, n.exit)
			}
			if n.enter == prevEnter {
				t.Errorf("%s: %s shares DfsEnter %d with another instance; the DFS order must be total",
					where, n.name, n.enter)
			}
			prevEnter = n.enter

			for len(stack) > 0 && stack[len(stack)-1].exit <= n.enter {
				stack = stack[:len(stack)-1]
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				if n.exit > p.exit {
					t.Errorf("%s: %s [%d, %d) straddles out of %s [%d, %d)",
						where, n.name, n.enter, n.exit, p.name, p.enter, p.exit)
				}
				// The rule the schema states, not the constant it collapses to
				// while nothing in Go is folded.  Pinning `n.fold == p.fold`
				// would hide the day a producer sets Folded.
				if want := p.fold + p.foldStep(); n.fold != want {
					t.Errorf("%s: %s has FoldDepth %d, expected %d from a parent at %d (folded=%v)",
						where, n.name, n.fold, want, p.fold, p.folded)
				}
			} else if n.fold != 0 {
				t.Errorf("%s: root %s has FoldDepth %d", where, n.name, n.fold)
			}
			if int(n.level) != len(stack) {
				t.Errorf("%s: %s says NestingLevel %d but sits %d levels deep",
					where, n.name, n.level, len(stack))
			}
			stack = append(stack, n)
		}

		// The tree AGREES WITH RANGE NESTING.  In Go that is the parentage
		// rule's definition rather than an aspiration -- there is no
		// preprocessor and so no co-extensive synthesised symbols -- so it
		// holds with no exceptions for ranges that strictly contain.
		for a := range obj.SymbolInstances {
			ia := obj.SymbolInstances[a]
			for b := range obj.SymbolInstances {
				if a == b {
					continue
				}
				ib := obj.SymbolInstances[b]
				strictlyContains := ia.StartOffset <= ib.StartOffset &&
					ib.EndOffset <= ia.EndOffset &&
					(ia.StartOffset != ib.StartOffset || ia.EndOffset != ib.EndOffset)
				if !strictlyContains {
					continue
				}
				if !(ia.DfsEnter < ib.DfsEnter && ib.DfsEnter < ia.DfsExit) {
					t.Errorf("%s: range [%d,%d) strictly contains [%d,%d), but interval [%d,%d) does not contain [%d,%d)",
						where, ia.StartOffset, ia.EndOffset, ib.StartOffset, ib.EndOffset,
						ia.DfsEnter, ia.DfsExit, ib.DfsEnter, ib.DfsExit)
				}
			}
		}

		// A finer span must sit inside the extent it subdivides.  The upload
		// path only rejects an inverted span, so a span that names bytes
		// outside its instance would be stored and later shown as a slice of a
		// different symbol's text.
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
		}

		// A ref is a point in the same order, so it has to land strictly inside
		// some instance -- and its depth and fold depth have to agree with the
		// instance its NUMBER puts it in, not with the one its range suggests.
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
			if want := owner.fold + owner.foldStep(); ref.FoldDepth != want {
				t.Errorf("%s: ref [%d,%d) says FoldDepth %d, expected %d from owner %s (fold %d, folded=%v)",
					where, ref.FromOffsetStart, ref.FromOffsetEnd, ref.FoldDepth,
					want, owner.name, owner.fold, owner.folded)
			}
			if ref.Full != nil &&
				!(ref.Full.Start <= ref.FromOffsetStart && ref.FromOffsetEnd <= ref.Full.End) {
				t.Errorf("%s: full span [%d,%d) does not contain its own token [%d,%d)",
					where, ref.Full.Start, ref.Full.End, ref.FromOffsetStart, ref.FromOffsetEnd)
			}
			// The range has to agree too, which is the half the interval cannot
			// check: a ref numbered inside an instance whose extent excludes it
			// is misattributed even though the tree is well formed.
			if !(owner.start <= ref.FromOffsetStart && ref.FromOffsetEnd <= owner.end) {
				t.Errorf("%s: ref [%d,%d) was numbered inside %s, whose extent is [%d,%d)",
					where, ref.FromOffsetStart, ref.FromOffsetEnd, owner.name,
					owner.start, owner.end)
			}
		}
	}
}

// TestContainerChainIsModuleDirectoryFile pins the one thing the ranges cannot
// say.  Three instances cover [0, len) -- the module, the directory and the
// file -- and the chain has to come out in that order, outermost first.
//
// Module before directory is a CHOICE -- a module spans directories and a
// directory can hold several modules, so they overlap and nothing in the engine
// depends on which way this producer states it.  What the test pins is that the
// three are a NESTED CHAIN in a stated order, because that is what kindRank
// exists for: three roots, or a different order, both mean the ranks stopped
// being consulted.
//
// Asserting the DFS order directly, NOT a kindRank-sorted copy of it: the
// first version of this test sorted the observed types by kindRank before
// comparing, which made it a property of kindRank alone -- a numbering that
// ignored rank entirely still passed.
func TestContainerChainIsModuleDirectoryFile(t *testing.T) {
	proj := fixture(t)
	byID := make(map[int64]*indexpb.Symbol, len(proj.Symbols))
	for _, s := range proj.Symbols {
		byID[s.LocalId] = s
	}
	want := []indexpb.SymbolType{
		indexpb.SymbolType_MODULE,
		indexpb.SymbolType_DIRECTORY,
		indexpb.SymbolType_FILE,
	}

	checked := 0
	for _, obj := range proj.Objects {
		if obj.Filetype == "directory" {
			continue
		}
		fileLen := int32(len(obj.Content))

		// In DFS order, so the slice IS the chain outermost-first.
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
			if sym := byID[i.SymbolLocalId]; sym != nil {
				kind = sym.Type
			}
			chain = append(chain, link{kind, i.NestingLevel, i.DfsEnter})
		}
		sort.Slice(chain, func(a, b int) bool { return chain[a].enter < chain[b].enter })

		if len(chain) != len(want) {
			t.Errorf("%s: %d instances cover [0,%d), expected the %d-link chain %v",
				obj.FilesystemPath, len(chain), fileLen, len(want), want)
			continue
		}
		for k, got := range chain {
			if got.kind != want[k] {
				t.Errorf("%s: link %d of the chain is %v, expected %v (full chain %v)",
					obj.FilesystemPath, k, got.kind, want[k], chain)
			}
			// Each link one level deeper than the last: three roots, or a
			// chain in a different order, both have to fail.
			if int(got.level) != k {
				t.Errorf("%s: %v is at NestingLevel %d, expected %d — the chain is not nested",
					obj.FilesystemPath, got.kind, got.level, k)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no file object was examined, so the chain was never checked")
	}
}

// TestSentinelRefsBelongToTheSentinel covers the object whose every range is
// [0, 0): a directory sentinel's self-instance and its parent->child refs.
// A ref whose range EQUALS its owner's must still be owned by it, which is why
// refs are placed only once every instance starting at or before them is on the
// stack.  Placing them a moment earlier leaves each one owner-less, outside
// every interval, and every directory edge vanishes from the graph.
func TestSentinelRefsBelongToTheSentinel(t *testing.T) {
	proj := fixture(t)

	checked := 0
	for _, obj := range proj.Objects {
		if obj.Filetype != "directory" || len(obj.Refs) == 0 {
			continue
		}
		nodes := sortedNodes(t, proj, obj)
		for _, ref := range obj.Refs {
			found := false
			for _, n := range nodes {
				if n.enter < ref.DfsEnter && ref.DfsEnter < n.exit {
					found = true
					if ref.NestingLevel != n.level+1 {
						t.Errorf("%s: sentinel ref says NestingLevel %d, its owner is at %d",
							obj.FilesystemPath, ref.NestingLevel, n.level)
					}
				}
			}
			if !found {
				t.Errorf("%s: sentinel ref to symbol %d at DfsEnter %d is outside every instance; "+
					"the directory edge is invisible",
					obj.FilesystemPath, ref.ToSymbolLocalId, ref.DfsEnter)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no directory sentinel carried a ref, so this test checked nothing")
	}
}

// TestNumberingIsIdempotent: Marshal and Upload both number, and a caller may
// do either or both.  The numbering is a pure function of the ranges, so the
// second pass must reproduce the first rather than shift everything.
func TestNumberingIsIdempotent(t *testing.T) {
	idx, err := NewProtoIndex(WithProject("p"), WithRootPath("/src"))
	if err != nil {
		t.Fatalf("NewProtoIndex: %v", err)
	}
	defer idx.Close()

	mod, err := idx.AddModule("example.com/m")
	if err != nil {
		t.Fatalf("AddModule: %v", err)
	}
	path := "/src/m/main.go"
	fileID, err := idx.AddFile(&mod, "/src", path, "go", make([]byte, 100))
	if err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	pos := func(off int) token.Position {
		return token.Position{Filename: path, Offset: off, Line: 1, Column: off + 1}
	}
	if _, _, err := idx.AddSymbol(mod, fileID, "m.Run", ScopeGlobal, SymbolTypeFunction,
		InstanceTypeDefinition, pos(10), pos(90), Spans{}, NoParent); err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}

	first := idx.Upload()
	second := idx.Upload()

	type key struct {
		obj string
		sym int64
		rng [2]int32
	}
	got := make(map[key][3]int64)
	for _, obj := range first.Objects {
		for _, i := range obj.SymbolInstances {
			got[key{obj.FilesystemPath, i.SymbolLocalId, [2]int32{i.StartOffset, i.EndOffset}}] =
				[3]int64{i.DfsEnter, i.DfsExit, int64(i.NestingLevel)}
		}
	}
	for _, obj := range second.Objects {
		for _, i := range obj.SymbolInstances {
			k := key{obj.FilesystemPath, i.SymbolLocalId, [2]int32{i.StartOffset, i.EndOffset}}
			want, ok := got[k]
			if !ok {
				t.Errorf("%s: instance of symbol %d at [%d,%d) appeared only on the second pass",
					obj.FilesystemPath, i.SymbolLocalId, i.StartOffset, i.EndOffset)
				continue
			}
			if want != [3]int64{i.DfsEnter, i.DfsExit, int64(i.NestingLevel)} {
				t.Errorf("%s: symbol %d at [%d,%d) was numbered [%d,%d) level %d, then [%d,%d) level %d",
					obj.FilesystemPath, i.SymbolLocalId, i.StartOffset, i.EndOffset,
					want[0], want[1], want[2], i.DfsEnter, i.DfsExit, i.NestingLevel)
			}
		}
	}
}

// TestSpansOutsideTheExtentAreDropped: a finer span is decoration, and absence
// already means "the producer does not know", so a wrong one must be dropped
// rather than stored or allowed to fail the whole index.  Stored, it would be
// rendered as a slice of whatever text those bytes hold -- another symbol's.
func TestSpansOutsideTheExtentAreDropped(t *testing.T) {
	idx, err := NewProtoIndex(WithProject("p"), WithRootPath("/src"))
	if err != nil {
		t.Fatalf("NewProtoIndex: %v", err)
	}
	defer idx.Close()

	mod, err := idx.AddModule("example.com/m")
	if err != nil {
		t.Fatalf("AddModule: %v", err)
	}
	path := "/src/m/main.go"
	fileID, err := idx.AddFile(&mod, "/src", path, "go", make([]byte, 100))
	if err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	pos := func(off int) token.Position {
		return token.Position{Filename: path, Offset: off, Line: 1, Column: off + 1}
	}
	if _, _, err := idx.AddSymbol(mod, fileID, "m.Run", ScopeGlobal, SymbolTypeFunction,
		InstanceTypeDefinition, pos(10), pos(90), Spans{
			Name:      NewSpan(5, 14),   // starts before the extent
			Signature: NewSpan(60, 40),  // inverted
			Content:   NewSpan(40, 150), // ends after the extent
		}, NoParent); err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}

	proj := idx.Upload()
	found := false
	for _, obj := range proj.Objects {
		for _, i := range obj.SymbolInstances {
			if i.StartOffset != 10 || i.EndOffset != 90 {
				continue
			}
			found = true
			if i.Name != nil {
				t.Errorf("name span [%d,%d) starts before the extent and was kept", i.Name.Start, i.Name.End)
			}
			if i.Signature != nil {
				t.Errorf("signature span [%d,%d) is inverted and was kept", i.Signature.Start, i.Signature.End)
			}
			if i.Content != nil {
				t.Errorf("content span [%d,%d) ends after the extent and was kept", i.Content.Start, i.Content.End)
			}
		}
	}
	if !found {
		t.Fatal("the instance under test is not in the index")
	}
}

// TestRefFullSpanMustContainItsToken: `full` encloses the referencing token,
// the opposite containment from an instance's finer spans -- and a helper that
// got the direction wrong would silently keep the broken ones, or drop every
// good one.  Pin both directions.
func TestRefFullSpanMustContainItsToken(t *testing.T) {
	build := func(t *testing.T, full *Span) *indexpb.SymbolRef {
		t.Helper()
		idx, err := NewProtoIndex(WithProject("p"), WithRootPath("/src"))
		if err != nil {
			t.Fatalf("NewProtoIndex: %v", err)
		}
		defer idx.Close()
		mod, err := idx.AddModule("example.com/m")
		if err != nil {
			t.Fatalf("AddModule: %v", err)
		}
		path := "/src/m/main.go"
		fileID, err := idx.AddFile(&mod, "/src", path, "go", make([]byte, 100))
		if err != nil {
			t.Fatalf("AddFile: %v", err)
		}
		pos := func(off int) token.Position {
			return token.Position{Filename: path, Offset: off, Line: 1, Column: off + 1}
		}
		if _, _, err := idx.AddSymbol(mod, fileID, "m.Run", ScopeGlobal, SymbolTypeFunction,
			InstanceTypeDefinition, pos(10), pos(90), Spans{}, NoParent); err != nil {
			t.Fatalf("AddSymbol: %v", err)
		}
		if err := idx.AddReference(fileID, pos(10), "m.Run", pos(40), pos(45), full); err != nil {
			t.Fatalf("AddReference: %v", err)
		}
		if err := idx.ResolveReferences(); err != nil {
			t.Fatalf("ResolveReferences: %v", err)
		}
		for _, obj := range idx.Upload().Objects {
			for _, r := range obj.Refs {
				if r.FromOffsetStart == 40 && r.FromOffsetEnd == 45 {
					return r
				}
			}
		}
		t.Fatal("the reference under test is not in the index")
		return nil
	}

	if got := build(t, NewSpan(38, 50)); got.Full == nil {
		t.Error("a full span that contains its token was dropped")
	} else if got.Full.Start != 38 || got.Full.End != 50 {
		t.Errorf("full span came back as [%d,%d), expected [38,50)", got.Full.Start, got.Full.End)
	}
	// Narrower than the token: the construct would be shown without the name
	// it refers to.
	if got := build(t, NewSpan(41, 44)); got.Full != nil {
		t.Errorf("full span [%d,%d) is inside its own token and was kept",
			got.Full.Start, got.Full.End)
	}
}

// newIndex is the three-line setup the regression tests below share.
func newIndex(t *testing.T, content []byte) (*ProtoIndex, ModuleId, FileId, string) {
	t.Helper()
	idx, err := NewProtoIndex(WithProject("p"), WithRootPath("/src"))
	if err != nil {
		t.Fatalf("NewProtoIndex: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	mod, err := idx.AddModule("example.com/m")
	if err != nil {
		t.Fatalf("AddModule: %v", err)
	}
	path := "/src/m/main.go"
	fileID, err := idx.AddFile(&mod, "/src", path, "go", content)
	if err != nil {
		t.Fatalf("AddFile: %v", err)
	}
	return idx, mod, fileID, path
}

// TestMarshallingAnEmptyIndexDoesNotDisableDirectories: the run-once guard on
// createDirectorySymbols has to mark the WORK done, not the attempt.  Marked on
// entry, a single Marshal before any file was added — the index is empty, so
// there is nothing to do and it returns early — permanently suppressed every
// DIRECTORY symbol for the rest of the index's life.  MODULE then becomes the
// object root, every `dir(...)` query answers nothing, and the whole hierarchy
// shifts up a level with no error.
func TestMarshallingAnEmptyIndexDoesNotDisableDirectories(t *testing.T) {
	idx, err := NewProtoIndex(WithProject("p"), WithRootPath("/src"))
	if err != nil {
		t.Fatalf("NewProtoIndex: %v", err)
	}
	defer idx.Close()

	if _, err := idx.Marshal(); err != nil { // nothing added yet
		t.Fatalf("Marshal on an empty index: %v", err)
	}

	mod, err := idx.AddModule("example.com/m")
	if err != nil {
		t.Fatalf("AddModule: %v", err)
	}
	if _, err := idx.AddFile(&mod, "/src", "/src/m/main.go", "go", make([]byte, 50)); err != nil {
		t.Fatalf("AddFile: %v", err)
	}

	proj := idx.Upload()
	dirs, sentinels := 0, 0
	for _, s := range proj.Symbols {
		if s.Type == indexpb.SymbolType_DIRECTORY {
			dirs++
		}
	}
	for _, obj := range proj.Objects {
		if obj.Filetype == "directory" {
			sentinels++
		}
	}
	if dirs == 0 || sentinels == 0 {
		t.Errorf("after an early Marshal: %d directory symbols and %d sentinel objects, expected both non-zero",
			dirs, sentinels)
	}
	// And the chain is three links, not two: module -> directory -> file.
	for _, obj := range proj.Objects {
		if obj.Filetype == "directory" {
			continue
		}
		deepest := int32(-1)
		for _, i := range obj.SymbolInstances {
			if i.NestingLevel > deepest {
				deepest = i.NestingLevel
			}
		}
		if deepest != 2 {
			t.Errorf("%s: the container chain is %d levels deep, expected module -> directory -> file",
				obj.FilesystemPath, deepest+1)
		}
	}
}

// TestARefInsideNoInstanceLandsAtTheObjectRoot: numbering starts at one stride,
// so a ref left at DfsEnter 0 is below every root — and askld rejects the WHOLE
// project over one of them ("lies outside every instance interval") rather than
// dropping the ref. 0 is also the engine's needs-backfill sentinel. The clang
// producer repairs the same case in ensureObjectRoot.
func TestARefInsideNoInstanceLandsAtTheObjectRoot(t *testing.T) {
	idx, mod, fileID, path := newIndex(t, make([]byte, 100))
	pos := func(off int) token.Position {
		return token.Position{Filename: path, Offset: off, Line: 1, Column: off + 1}
	}
	if _, _, err := idx.AddSymbol(mod, fileID, "m.Run", ScopeGlobal, SymbolTypeFunction,
		InstanceTypeDefinition, pos(10), pos(90), Spans{}, NoParent); err != nil {
		t.Fatalf("AddSymbol: %v", err)
	}
	// Beyond the file instance's own extent, which is what a //line directive
	// or a second read of the file can produce.
	if err := idx.AddReference(fileID, pos(10), "m.Run", pos(150), pos(160), nil); err != nil {
		t.Fatalf("AddReference: %v", err)
	}
	if err := idx.ResolveReferences(); err != nil {
		t.Fatalf("ResolveReferences: %v", err)
	}

	proj := idx.Upload()
	found := false
	for _, obj := range proj.Objects {
		var minEnter, maxExit int64
		for _, i := range obj.SymbolInstances {
			if minEnter == 0 || i.DfsEnter < minEnter {
				minEnter = i.DfsEnter
			}
			if i.DfsExit > maxExit {
				maxExit = i.DfsExit
			}
		}
		for _, r := range obj.Refs {
			if r.FromOffsetStart != 150 {
				continue
			}
			found = true
			if r.DfsEnter <= minEnter || r.DfsEnter >= maxExit {
				t.Errorf("orphan ref is at DfsEnter %d, outside every interval [%d, %d) — askld rejects the whole project for this",
					r.DfsEnter, minEnter, maxExit)
			}
			if r.NestingLevel < 1 {
				t.Errorf("orphan ref says NestingLevel %d; it is inside the root, so at least 1", r.NestingLevel)
			}
		}
	}
	if !found {
		t.Fatal("the orphan reference is not in the index")
	}
}

// TestAnExtentPastTheObjectIsClamped: an instance reaching beyond its object
// pops the module/directory/file chain to make room, becomes a second ROOT, and
// takes every later instance in the file with it — so `file { X }`, `mod { X }`
// and `dir { X }` all go empty for symbols that are perfectly in range. askld
// accepts a forest, so nothing downstream notices.
//
// Reachable because an instance's offsets come from go/packages' Fset while the
// object's length comes from an independent read of the same path.
func TestAnExtentPastTheObjectIsClamped(t *testing.T) {
	idx, mod, fileID, path := newIndex(t, make([]byte, 100))
	pos := func(off int) token.Position {
		return token.Position{Filename: path, Offset: off, Line: 1, Column: off + 1}
	}
	if _, _, err := idx.AddSymbol(mod, fileID, "m.Spill", ScopeGlobal, SymbolTypeFunction,
		InstanceTypeDefinition, pos(10), pos(101), Spans{}, NoParent); err != nil {
		t.Fatalf("AddSymbol Spill: %v", err)
	}
	if _, _, err := idx.AddSymbol(mod, fileID, "m.Fine", ScopeGlobal, SymbolTypeFunction,
		InstanceTypeDefinition, pos(20), pos(40), Spans{}, NoParent); err != nil {
		t.Fatalf("AddSymbol Fine: %v", err)
	}

	proj := idx.Upload()
	byID := make(map[int64]*indexpb.Symbol, len(proj.Symbols))
	for _, s := range proj.Symbols {
		byID[s.LocalId] = s
	}
	roots, levels := 0, map[string]int32{}
	for _, obj := range proj.Objects {
		if obj.Filetype == "directory" {
			continue
		}
		for _, i := range obj.SymbolInstances {
			if i.NestingLevel == 0 {
				roots++
			}
			if sym := byID[i.SymbolLocalId]; sym != nil {
				levels[sym.Name] = i.NestingLevel
			}
			if i.EndOffset > int32(len(obj.Content)) {
				t.Errorf("%s: instance extent [%d,%d) runs past the %d-byte object",
					obj.FilesystemPath, i.StartOffset, i.EndOffset, len(obj.Content))
			}
		}
	}
	if roots != 1 {
		t.Errorf("%d instances at NestingLevel 0, expected the single module root", roots)
	}
	// Spill sits inside its file, and Fine inside the clamped Spill that now
	// genuinely contains it -- the point is that neither escaped the chain.
	if levels["m.Spill"] != 3 {
		t.Errorf("m.Spill is at NestingLevel %d, expected 3 (module, directory, file, then it)",
			levels["m.Spill"])
	}
	if levels["m.Fine"] != 4 {
		t.Errorf("m.Fine is at NestingLevel %d, expected 4 (inside the clamped m.Spill)",
			levels["m.Fine"])
	}
}

// TestFoldDepthFollowsTheFoldedFlag pins a rule that is a no-op today: nothing
// in the Go producer sets Folded, so dropping the `+ (parent.folded ? 1 : 0)`
// term changes no output and no pipeline test notices. askld's upload validator
// enforces it though, and both sibling producers include it — so the day a
// producer path marks something folded, the term's absence would put every
// descendant and every ref beneath it off by one and fail the whole upload.
//
// Driving numberObjectTree directly is the only way to state it, which is the
// point: a rule with no reachable input still has to be the right rule.
func TestFoldDepthFollowsTheFoldedFlag(t *testing.T) {
	obj := &indexpb.Object{
		FilesystemPath: "/src/m/main.go",
		SymbolInstances: []*indexpb.SymbolInstance{
			{SymbolLocalId: 1, StartOffset: 0, EndOffset: 100},               // outer
			{SymbolLocalId: 2, StartOffset: 10, EndOffset: 90, Folded: true}, // a boundary
			{SymbolLocalId: 3, StartOffset: 20, EndOffset: 80},               // below it
			{SymbolLocalId: 4, StartOffset: 30, EndOffset: 70, Folded: true}, // a second one
			{SymbolLocalId: 5, StartOffset: 40, EndOffset: 60},               // below both
		},
		Refs: []*indexpb.SymbolRef{
			{ToSymbolLocalId: 9, FromOffsetStart: 45, FromOffsetEnd: 50}, // inside symbol 5
			{ToSymbolLocalId: 9, FromOffsetStart: 35, FromOffsetEnd: 38}, // inside folded 4
		},
	}
	numberObjectTree(obj, map[int64]int{}, nil)

	// fold_depth counts the folded ancestors ABOVE an instance, so a folded
	// instance's own flag lands on its children, not on itself.
	want := map[int64]int32{1: 0, 2: 0, 3: 1, 4: 1, 5: 2}
	for _, i := range obj.SymbolInstances {
		if got := i.FoldDepth; got != want[i.SymbolLocalId] {
			t.Errorf("symbol %d: FoldDepth %d, expected %d", i.SymbolLocalId, got, want[i.SymbolLocalId])
		}
	}
	// A ref takes its owner's fold depth plus that owner's own flag: symbol 5
	// is unfolded (2), symbol 4 is folded (1 + 1).
	wantRefs := map[int32]int32{45: 2, 35: 2}
	for _, r := range obj.Refs {
		if got := r.FoldDepth; got != wantRefs[r.FromOffsetStart] {
			t.Errorf("ref at %d: FoldDepth %d, expected %d",
				r.FromOffsetStart, got, wantRefs[r.FromOffsetStart])
		}
	}
}

// TestAStatedParentMustPrecedeItsChild: stated parentage is the one thing that
// could make the containment forest cyclic, and a cycle leaves every instance
// in it at [0, 0) — invisible to every query, and fatal to the whole project at
// upload. AddSymbol makes a cycle unstatable rather than detectable: a stated
// parent has to already exist in the same object, so parentage only ever points
// backwards. Anything else is dropped with a warning and derived from ranges.
func TestAStatedParentMustPrecedeItsChild(t *testing.T) {
	idx, mod, fileID, path := newIndex(t, make([]byte, 200))
	otherID, err := idx.AddFile(&mod, "/src", "/src/m/other.go", "go", make([]byte, 50))
	if err != nil {
		t.Fatalf("AddFile other: %v", err)
	}
	pos := func(off int) token.Position {
		return token.Position{Filename: path, Offset: off, Line: 1, Column: off + 1}
	}
	add := func(name string, start, end int, parent SymbolInstanceId) SymbolInstanceId {
		t.Helper()
		_, id, err := idx.AddSymbol(mod, fileID, name, ScopeGlobal, SymbolTypeFunction,
			InstanceTypeDefinition, pos(start), pos(end), Spans{}, parent)
		if err != nil {
			t.Fatalf("AddSymbol %s: %v", name, err)
		}
		return id
	}

	// A real one, to prove the mechanism works in this fixture at all: Owner's
	// extent does not contain Owned's, so only the statement can place it.
	owner := add("m.Owner", 10, 20, NoParent)
	add("m.Owned", 100, 150, owner)

	// Each of these must be refused: a parent that does not exist, one in
	// another object, and one that comes later in the slice.
	add("m.Ghost", 30, 40, SymbolInstanceId(9999))
	_, foreign, err := idx.AddSymbol(mod, otherID, "m.Foreign", ScopeGlobal,
		SymbolTypeFunction, InstanceTypeDefinition,
		token.Position{Filename: "/src/m/other.go", Offset: 5, Line: 1, Column: 6},
		token.Position{Filename: "/src/m/other.go", Offset: 15, Line: 1, Column: 16},
		Spans{}, NoParent)
	if err != nil {
		t.Fatalf("AddSymbol Foreign: %v", err)
	}
	add("m.CrossObject", 50, 60, foreign)

	proj := idx.Upload()
	byID := make(map[int64]*indexpb.Symbol, len(proj.Symbols))
	for _, s := range proj.Symbols {
		byID[s.LocalId] = s
	}
	for _, obj := range proj.Objects {
		for _, i := range obj.SymbolInstances {
			// The real failure mode: an unreached instance keeps the column
			// defaults, and askld rejects the project over it.
			if i.DfsExit <= i.DfsEnter {
				name := "?"
				if sym := byID[i.SymbolLocalId]; sym != nil {
					name = sym.Name
				}
				t.Errorf("%s: %s was never numbered ([%d, %d)), so the forest is not a tree",
					obj.FilesystemPath, name, i.DfsEnter, i.DfsExit)
			}
		}
	}

	levels := map[string]int32{}
	for _, obj := range proj.Objects {
		for _, i := range obj.SymbolInstances {
			if sym := byID[i.SymbolLocalId]; sym != nil {
				levels[sym.Name] = i.NestingLevel
			}
		}
	}
	// module, directory, file, Owner — so a stated child of Owner is at 4.
	if levels["m.Owned"] != levels["m.Owner"]+1 {
		t.Errorf("m.Owned is at NestingLevel %d and m.Owner at %d; the stated parent was not honoured",
			levels["m.Owned"], levels["m.Owner"])
	}
	// The refused ones fall back to range derivation: inside their file, beside
	// Owner rather than under anything.
	for _, name := range []string{"m.Ghost", "m.CrossObject"} {
		if levels[name] != levels["m.Owner"] {
			t.Errorf("%s is at NestingLevel %d, expected %d — a refused parent must fall back to the ranges",
				name, levels[name], levels["m.Owner"])
		}
	}
}
