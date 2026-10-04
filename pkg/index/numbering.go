package index

import (
	"sort"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
	"github.com/planetA/askl-golang-indexer/pkg/logging"
)

// dfsStride is the spacing between consecutive DFS numbers, and therefore the
// room every instance leaves between its own DfsEnter and DfsExit.
//
// It is free -- each instance consumes a stride as it is numbered -- and the
// engine requires it: query-time rows (a search match, say) are numbered into
// the gap above an owner's DfsEnter, so an instance narrower than a stride
// cannot hold its own matches.  They would land past its DfsExit and silently
// stop being its descendants, because containment only ever reads the child's
// DfsEnter.  askld's upload validator rejects anything narrower (DFS_STRIDE).
const dfsStride = 1024

// foldStep is the fold-depth increment crossing into an instance: 1 when the
// instance itself is a generated boundary, 0 otherwise.  Always 0 in Go, where
// nothing is generated at a level finer than a whole file -- but this is the
// rule the schema states and askld's upload validator enforces, so it is
// written out rather than folded into the constant it currently equals.
func foldStep(inst *indexpb.SymbolInstance) int32 {
	if inst.Folded {
		return 1
	}
	return 0
}

// numberObjectTrees derives every object's containment tree and numbers it.
//
// Containment is NOT a source range in the index: an instance owns the
// half-open interval [DfsEnter, DfsExit), and Y is inside X exactly when
//
//	X.DfsEnter < Y.DfsEnter < X.DfsExit
//
// The encoding exists because C has shapes no range can express -- everything a
// macro generates shares one extent -- so the tree has to be stated rather than
// inferred.  Go has no preprocessor, so here the ranges carry almost all of it;
// kindRank and the producer's stated parents cover the rest.
//
// Call after createModuleSymbols and createDirectorySymbols -- the only point
// at which every instance exists.  Idempotent: the numbering is a pure function
// of the ranges and the stated parents, so Marshal-then-Upload recomputes the
// same answer.
func (i *ProtoIndex) numberObjectTrees() {
	// Rank by symbol, read from project.Symbols rather than from the symbolByID
	// bookkeeping map: that map is an index of what we are numbering, not its
	// source, and createDirectorySymbols does not populate it -- a directory
	// would silently rank as an ordinary declaration and come out *inside* its
	// own file.
	rankBySymbol := make(map[int64]int, len(i.project.Symbols))
	for _, sym := range i.project.Symbols {
		rankBySymbol[sym.LocalId] = kindRank(sym.Type)
	}
	for _, obj := range i.project.Objects {
		var stated map[int]int
		for k := range obj.SymbolInstances {
			if p, ok := i.statedParent[instanceSlot{object: obj.LocalId, idx: k}]; ok {
				if stated == nil {
					stated = make(map[int]int)
				}
				stated[k] = p
			}
		}
		numberObjectTree(obj, rankBySymbol, stated)
	}
}

// kindRank orders instances that share a range: lower is further out.
//
// The exception to "the ranges are enough": every file's object carries THREE
// instances covering [0, len) -- its module, its directory and the file itself,
// from createModuleSymbols, createDirectorySymbols and AddFile.  Identical
// ranges cannot say which contains which, and unlike the clang indexer there is
// no AST scope stack to ask, because the Go parser reports a position and
// nothing else.  The symbol kind is the information this producer does have.
//
// The order is a free CHOICE, not a fact to get right: a module spans
// directories and a directory can hold several modules, so the two genuinely
// overlap and each language may answer differently.  Nothing in the engine
// depends on it -- traversal is the DFS interval plus at most one equality on
// nesting_level or fold_depth, and the only place that names container types
// (`TypeSelector::update_context`) sets default child TYPES for display, which
// tolerates either order.  So this is not a cross-producer contract, and the
// clang indexer building its chain modules-first is not something Go has to
// match.
//
// Module outermost here because that is the Go question worth asking: one
// `go.mod` usually covers a whole repository, so `mod(m) { dir }` -- which
// directories does my module span -- is natural, while `dir { mod }` is only
// interesting in a monorepo with nested modules.
//
// A file outside any module has only a directory above it; the sweep handles a
// chain of any length, which is why this is a rank and not a fixed prefix.
func kindRank(t indexpb.SymbolType) int {
	switch t {
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

// contains decides whether instance c is a container of instance x: either its
// range is strictly wider, or the two ranges are equal and its kind sits
// further out.  Equal range AND equal rank contains nothing either way, so
// those come out siblings -- the right answer rather than a tie to break.
//
// One predicate, because the parentage sweep and the ref sweep must agree about
// what contains what.  Two copies that drift leave refs numbered inside
// instances whose extents exclude them -- which askld's validator accepts,
// since it checks the tree and not the ranges.
func contains(c, x *indexpb.SymbolInstance, cRank, xRank int) bool {
	if c.StartOffset > x.StartOffset || x.EndOffset > c.EndOffset {
		return false
	}
	if c.StartOffset == x.StartOffset && c.EndOffset == x.EndOffset {
		return cRank < xRank
	}
	return true
}

// numberObjectTree numbers one object.  `stated` overrides parentage for the
// instances whose place the ranges cannot express, keyed by position in
// obj.SymbolInstances; nil when the producer stated nothing.
func numberObjectTree(obj *indexpb.Object, rankBySymbol map[int64]int, stated map[int]int) {
	insts := obj.SymbolInstances
	n := len(insts)
	if n == 0 {
		return
	}

	rank := make([]int, n)
	for k, inst := range insts {
		r, ok := rankBySymbol[inst.SymbolLocalId]
		if !ok {
			r = 3
		}
		rank[k] = r
	}

	// Sorted view, not a reordering: instance identity is a SymbolInstanceId and
	// the wire order is observable, so only this index array moves.
	//
	// Outermost-first, which is what makes the sweep below a single pass:
	// earlier start first, then wider first, then the containing kind first.
	order := make([]int, n)
	for k := range order {
		order[k] = k
	}
	sort.SliceStable(order, func(x, y int) bool {
		a, b := order[x], order[y]
		if insts[a].StartOffset != insts[b].StartOffset {
			return insts[a].StartOffset < insts[b].StartOffset
		}
		if insts[a].EndOffset != insts[b].EndOffset {
			return insts[a].EndOffset > insts[b].EndOffset
		}
		return rank[a] < rank[b]
	})

	// --- PARENTAGE -----------------------------------------------------
	//
	//	parent(x) = what the producer stated, or else the innermost instance
	//	containing x -- narrowest range, with kindRank breaking equal ranges.
	//
	// One stack sweep for the derived half, the same shape as askld's upload
	// validator.  In sort order a candidate parent always precedes its children,
	// so the stack holds exactly the instances still open at the current
	// position and the innermost container is its top.  Partially overlapping
	// ranges -- which valid Go cannot produce -- come out siblings rather than
	// falsely nested.
	derived := make([]int, n)
	for k := range derived {
		derived[k] = -1
	}
	stack := make([]int, 0, 16)
	for _, a := range order {
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			if contains(insts[p], insts[a], rank[p], rank[a]) {
				break
			}
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			derived[a] = stack[len(stack)-1]
		}
		stack = append(stack, a)
	}

	// Stated parentage wins, and is applied AFTER the sweep so a stated child
	// can still serve as a derived container for others.
	//
	// It may disagree with the ranges completely -- `var First = f1` states that
	// First owns f1 while First's extent is only its identifier -- which is the
	// point: the interval decides containment and never consults a range.
	parent := make([]int, n)
	copy(parent, derived)
	for child, p := range stated {
		if child >= 0 && child < n && p >= 0 && p < n && p != child {
			parent[child] = p
		}
	}

	// --- NUMBERING -----------------------------------------------------
	kids, roots := childLists(parent, n, order)
	numbered := numberForest(insts, kids, roots)
	if numbered != n && len(stated) > 0 {
		// Something stated is not a tree.  AddSymbol only accepts a parent that
		// already exists in the same object, which makes a cycle unstatable, so
		// this should be unreachable -- but the consequence of being wrong is
		// instances shipped at [0, 0): invisible to every query, and fatal to
		// the whole project at upload.  Range derivation is acyclic by
		// construction, so fall back to it entirely rather than emit that.
		logging.Warnf("stated parentage in %s does not form a tree (%d of %d instances reached); deriving from ranges instead",
			obj.FilesystemPath, numbered, n)
		for k := range insts {
			insts[k].DfsEnter, insts[k].DfsExit = 0, 0
			insts[k].NestingLevel, insts[k].FoldDepth = 0, 0
		}
		kids, roots = childLists(derived, n, order)
		numbered = numberForest(insts, kids, roots)
	}
	if numbered != n {
		logging.Warnf("numbered %d of %d instances in %s: the containment forest does not span them",
			numbered, n, obj.FilesystemPath)
	}

	numberRefs(obj, insts, order, rank, roots)
}

// childLists inverts a parent array into per-node child lists plus the roots,
// keeping children in `order` so the preorder walks them outermost-first.
func childLists(parent []int, n int, order []int) ([][]int, []int) {
	kids := make([][]int, n)
	roots := make([]int, 0, 1)
	for _, a := range order {
		if parent[a] < 0 {
			roots = append(roots, a)
		} else {
			kids[parent[a]] = append(kids[parent[a]], a)
		}
	}
	return kids, roots
}

// numberForest assigns every tree field in one preorder walk, one stride per
// event, and returns how many instances it reached.  All four fields are set in
// the same place so none can be written while another is forgotten.
func numberForest(insts []*indexpb.SymbolInstance, kids [][]int, roots []int) int {
	type frame struct{ node, next int }
	counter := int64(0)
	numbered := 0
	walk := make([]frame, 0, 16)
	for _, r := range roots {
		counter += dfsStride
		insts[r].DfsEnter = counter
		insts[r].NestingLevel = 0
		insts[r].FoldDepth = 0
		numbered++
		walk = append(walk[:0], frame{node: r})
		for len(walk) > 0 {
			top := len(walk) - 1
			node, next := walk[top].node, walk[top].next
			if next < len(kids[node]) {
				walk[top].next = next + 1
				c := kids[node][next]
				counter += dfsStride
				insts[c].DfsEnter = counter
				insts[c].NestingLevel = insts[node].NestingLevel + 1
				// Nothing folds in Go today -- the fold axis marks a level that
				// exists only because a macro generated it, and there is no
				// preprocessor.  Generated *files* (`// Code generated ... DO
				// NOT EDIT.`) are a file-level property, not an intra-file
				// boundary, so hiding those belongs in a query filter.
				//
				// State the rule anyway rather than the constant it collapses
				// to: askld's validator enforces `parent + (parent.folded?1:0)`
				// and both sibling producers include the term, so a path that
				// ever sets Folded would otherwise put every descendant off by
				// one and fail the whole upload.
				insts[c].FoldDepth = insts[node].FoldDepth + foldStep(insts[node])
				numbered++
				walk = append(walk, frame{node: c})
			} else {
				counter += dfsStride
				insts[node].DfsExit = counter
				walk = walk[:top]
			}
		}
	}
	return numbered
}

// numberRefs places every ref as a POINT in the same order, because a use
// contains nothing.  A ref sits one step inside its owner, so several refs in
// one owner share a number and their byte ranges are what tell them apart.
//
// The owner is the innermost instance containing the ref's range -- found by
// text, since Go refs have no scope to consult.  Equal ranges count as
// containing here, unlike for instances: a directory sentinel's whole object is
// [0, 0), self-instance and parent->child refs alike, and those refs belong to
// it.
//
// Re-running the sweep over the same order, with refs interleaved by position,
// keeps this O(n log n) instead of ref-by-instance.  A ref only *reads* the
// stack -- popping for a ref could discard an instance that a later instance
// still belongs inside.
func numberRefs(obj *indexpb.Object, insts []*indexpb.SymbolInstance, order, rank, roots []int) {
	if len(obj.Refs) == 0 || len(roots) == 0 {
		return
	}
	// Where a ref goes when no instance contains it.  Leaving it at 0 is not an
	// option: numbering starts at one stride, so 0 is below every root, and
	// askld rejects the WHOLE project over one such ref -- "lies outside every
	// instance interval" -- rather than dropping it.  0 is also the engine's
	// needs-backfill sentinel.  The clang producer repairs the same case in
	// ensureObjectRoot.
	//
	// Reachable whenever a ref's range exceeds its object: the FILE instance's
	// end comes from len(contents), a different read from the one go/packages'
	// Fset offsets came from, and a //line directive remaps a position's
	// filename independently of its offset.
	fallback := insts[roots[0]]

	refOrder := make([]int, len(obj.Refs))
	for k := range refOrder {
		refOrder[k] = k
	}
	sort.SliceStable(refOrder, func(x, y int) bool {
		return obj.Refs[refOrder[x]].FromOffsetStart < obj.Refs[refOrder[y]].FromOffsetStart
	})

	stack := make([]int, 0, 16)
	ri := 0
	// placeBefore drains every ref that starts STRICTLY before `start`.  Strict
	// is the whole point: a ref whose range equals its owner's -- every ref on a
	// directory sentinel, whose object is [0, 0) end to end -- must wait until
	// that owner is on the stack.
	placeBefore := func(start int32, flush bool) {
		for ri < len(refOrder) {
			ref := obj.Refs[refOrder[ri]]
			if !flush && ref.FromOffsetStart >= start {
				return
			}
			ri++
			owner := -1
			for k := len(stack) - 1; k >= 0; k-- {
				c := insts[stack[k]]
				if c.StartOffset <= ref.FromOffsetStart && ref.FromOffsetEnd <= c.EndOffset {
					owner = stack[k]
					break
				}
			}
			o := fallback
			if owner >= 0 {
				o = insts[owner]
			} else {
				logging.Warnf("reference [%d, %d) in %s is inside no instance; placing it at the object root",
					ref.FromOffsetStart, ref.FromOffsetEnd, obj.FilesystemPath)
			}
			ref.DfsEnter = o.DfsEnter + 1
			ref.NestingLevel = o.NestingLevel + 1
			ref.FoldDepth = o.FoldDepth + foldStep(o)
		}
	}
	for _, a := range order {
		// Only once the stack holds something: on the first iteration it is
		// empty, so draining there would hand every ref starting below the
		// outermost instance to the fallback instead of letting the sweep find
		// its real owner.  Masked while every object's outermost instance starts
		// at offset 0, which is what AddFile and createDirectorySymbols happen
		// to build.
		if len(stack) > 0 {
			placeBefore(insts[a].StartOffset, false)
		}
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			if contains(insts[p], insts[a], rank[p], rank[a]) {
				break
			}
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, a)
	}
	placeBefore(0, true)
}
