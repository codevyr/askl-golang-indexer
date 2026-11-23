package index

import (
	"go/token"
	"testing"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
)

// A RELATION NAMES THE FIRST PLACE THAT MADE IT SO, AND COUNTS THEM ALL.
//
// A relation row sits on the declaration of its source and says who is
// related.  Where the code made it so is its witness: of all the places the
// relation was added with, the first in file order -- lowest object path,
// then lowest offset -- and how many distinct ones there are.  "First" is a
// property of the text, so it cannot depend on the order the places were
// seen in, which is the order the parser's workers happened to run in.

type witnessed struct {
	path       string
	start, end int32
	sites      uint32
}

type witnessFixture struct {
	t   *testing.T
	idx *ProtoIndex
	// The files, by the letter the test calls them: `z` is added first and so
	// has the LOWEST id and the HIGHEST path.
	files map[string]FileId
}

const (
	witnessZ = "/src/m/z.go" // holds the interface, and the rows
	witnessA = "/src/m/a.go"
	witnessB = "/src/m/sub/b.go"
)

func newWitnessFixture(t *testing.T) *witnessFixture {
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
	f := &witnessFixture{t: t, idx: idx, files: map[string]FileId{}}
	// Ids ascend in the order added, paths do not: z < b < a by id, and
	// a < b < z by path.
	for _, path := range []string{witnessZ, witnessB, witnessA} {
		id, err := idx.AddFile(&mod, "/src", path, "go", make([]byte, 200))
		if err != nil {
			t.Fatalf("AddFile %s: %v", path, err)
		}
		f.files[path] = id
	}
	at := func(path string, off int) token.Position {
		return token.Position{Filename: path, Offset: off, Line: 1, Column: off + 1}
	}
	// The interface method and a second one in z, and what satisfies them in a.
	for _, sym := range []struct {
		path, name string
		kind       SymbolType
		start, end int
	}{
		{witnessZ, "(m.I).M", SymbolTypeField, 10, 20},
		{witnessZ, "(m.I).N", SymbolTypeField, 30, 40},
		{witnessA, "(m.T).M", SymbolTypeFunction, 100, 120},
		{witnessA, "(m.T).N", SymbolTypeFunction, 130, 150},
	} {
		if _, _, err := idx.AddSymbol(mod, f.files[sym.path], sym.name, ScopeGlobal, sym.kind,
			InstanceTypeDefinition, at(sym.path, sym.start), at(sym.path, sym.end), Spans{}, NoParent); err != nil {
			t.Fatalf("AddSymbol %s: %v", sym.name, err)
		}
	}
	return f
}

func (f *witnessFixture) pos(path string, off int) token.Position {
	return token.Position{Filename: path, Offset: off, Line: 1, Column: off + 1}
}

// holds adds `(m.I).M holds (m.T).M`, made so at [start, end) of `path`.
func (f *witnessFixture) holds(path string, start, end int) {
	f.t.Helper()
	f.holdsN("M", 10, 100, Place{File: f.files[path], Start: start, End: end})
}

func (f *witnessFixture) holdsN(method string, rowAt, targetAt int, at Place) {
	f.t.Helper()
	if err := f.idx.AddRelation(f.files[witnessZ], f.pos(witnessA, targetAt), "(m.T)."+method,
		f.pos(witnessZ, rowAt), f.pos(witnessZ, rowAt+1), RelationHolds, at); err != nil {
		f.t.Fatalf("AddRelation: %v", err)
	}
}

// rows resolves and returns every row of z by its start offset.
func (f *witnessFixture) rows() map[int32]*indexpb.SymbolRef {
	f.t.Helper()
	if err := f.idx.ResolveReferences(); err != nil {
		f.t.Fatalf("ResolveReferences: %v", err)
	}
	project := f.idx.Upload()
	rows := map[int32]*indexpb.SymbolRef{}
	for _, obj := range project.Objects {
		if obj.FilesystemPath != witnessZ {
			continue
		}
		for _, r := range obj.Refs {
			if _, twice := rows[r.FromOffsetStart]; twice {
				f.t.Fatalf("two rows at %d", r.FromOffsetStart)
			}
			rows[r.FromOffsetStart] = r
		}
	}
	return rows
}

// witness is the witness of the one `(m.I).M` row, with its object's path in
// place of the id.
func (f *witnessFixture) witness() *witnessed {
	f.t.Helper()
	return f.witnessOf(f.rows()[10])
}

func (f *witnessFixture) witnessOf(row *indexpb.SymbolRef) *witnessed {
	f.t.Helper()
	if row == nil {
		f.t.Fatalf("no row")
	}
	if row.Witness == nil {
		return nil
	}
	for path, id := range f.files {
		if int64(id) == row.Witness.ObjectLocalId {
			return &witnessed{path, row.Witness.OffsetStart, row.Witness.OffsetEnd, row.Witness.Sites}
		}
	}
	f.t.Fatalf("witness names object %d, which is not in the index", row.Witness.ObjectLocalId)
	return nil
}

func expectWitness(t *testing.T, got *witnessed, want witnessed) {
	t.Helper()
	if got == nil {
		t.Fatalf("no witness, want %+v", want)
	}
	if *got != want {
		t.Fatalf("witness: got %+v, want %+v", *got, want)
	}
}

func TestARelationNamesThePlaceThatMadeItSo(t *testing.T) {
	f := newWitnessFixture(t)
	f.holds(witnessA, 60, 68)
	expectWitness(t, f.witness(), witnessed{witnessA, 60, 68, 1})
}

func TestTheWitnessIsTheLowestOffsetWhateverOrderItWasSeenIn(t *testing.T) {
	for name, order := range map[string][][2]int{
		"ascending":  {{40, 48}, {60, 68}, {80, 88}},
		"descending": {{80, 88}, {60, 68}, {40, 48}},
		"first last": {{60, 68}, {80, 88}, {40, 48}},
	} {
		t.Run(name, func(t *testing.T) {
			f := newWitnessFixture(t)
			for _, place := range order {
				f.holds(witnessA, place[0], place[1])
			}
			expectWitness(t, f.witness(), witnessed{witnessA, 40, 48, 3})
		})
	}
	// Where it starts decides, not where it ends: a long expression that
	// starts first is first.
	t.Run("starts first, ends last", func(t *testing.T) {
		f := newWitnessFixture(t)
		f.holds(witnessA, 50, 60)
		f.holds(witnessA, 40, 90)
		expectWitness(t, f.witness(), witnessed{witnessA, 40, 90, 2})
	})
	// Two expressions that start together: the shorter is first.
	t.Run("same start", func(t *testing.T) {
		f := newWitnessFixture(t)
		f.holds(witnessA, 40, 90)
		f.holds(witnessA, 40, 48)
		expectWitness(t, f.witness(), witnessed{witnessA, 40, 48, 2})
	})
}

func TestTheWitnessIsInTheLowestPathNotTheLowestId(t *testing.T) {
	// z has the lowest id and the highest path; a the highest id and the
	// lowest path.  Offsets run the other way, so neither "first seen", nor
	// "lowest id", nor "lowest offset anywhere" gives the answer.
	for name, order := range map[string][]string{
		"z b a": {witnessZ, witnessB, witnessA},
		"a b z": {witnessA, witnessB, witnessZ},
		"b a z": {witnessB, witnessA, witnessZ},
	} {
		t.Run(name, func(t *testing.T) {
			f := newWitnessFixture(t)
			offset := map[string]int{witnessZ: 50, witnessB: 70, witnessA: 90}
			for _, path := range order {
				f.holds(path, offset[path], offset[path]+5)
			}
			expectWitness(t, f.witness(), witnessed{witnessA, 90, 95, 3})
		})
	}
	// `/src/m/sub/b.go` against `/src/m/z.go`: the path is compared as a
	// whole, so the deeper file comes first here.
	t.Run("b before z", func(t *testing.T) {
		f := newWitnessFixture(t)
		f.holds(witnessZ, 50, 55)
		f.holds(witnessB, 70, 75)
		expectWitness(t, f.witness(), witnessed{witnessB, 70, 75, 2})
	})
}

func TestSitesCountsDistinctPlaces(t *testing.T) {
	f := newWitnessFixture(t)
	// The same place seen three times (a file parsed twice, a statement
	// visited from two paths) is one place.
	f.holds(witnessA, 60, 68)
	f.holds(witnessA, 60, 68)
	f.holds(witnessB, 60, 68) // the same offsets in another file: another place
	f.holds(witnessA, 60, 68)
	f.holds(witnessA, 60, 69) // another expression at the same start
	expectWitness(t, f.witness(), witnessed{witnessA, 60, 68, 3})
}

func TestEachRelationHasItsOwnPlaces(t *testing.T) {
	f := newWitnessFixture(t)
	f.holds(witnessA, 60, 68)
	f.holds(witnessB, 20, 28)
	f.holdsN("N", 30, 130, Place{File: f.files[witnessB], Start: 80, End: 88})
	rows := f.rows()
	expectWitness(t, f.witnessOf(rows[10]), witnessed{witnessA, 60, 68, 2})
	expectWitness(t, f.witnessOf(rows[30]), witnessed{witnessB, 80, 88, 1})
}

func TestAPlaceOutsideTheIndexIsNoWitness(t *testing.T) {
	// The witness names an object of this project.  A place in a file the
	// index does not hold cannot be named, and is not counted: `sites` is how
	// many places a reader can be shown.
	t.Run("the only place", func(t *testing.T) {
		f := newWitnessFixture(t)
		f.holdsN("M", 10, 100, Place{File: FileId(9999), Start: 5, End: 9})
		row := f.rows()[10]
		if row == nil || Relation(row.Relation) != RelationHolds {
			t.Fatalf("the relation itself must stay: %+v", row)
		}
		if row.Witness != nil {
			t.Fatalf("a witness in an object that does not exist: %+v", row.Witness)
		}
	})
	t.Run("one of two", func(t *testing.T) {
		f := newWitnessFixture(t)
		f.holdsN("M", 10, 100, Place{File: FileId(9999), Start: 5, End: 9})
		f.holds(witnessB, 70, 75)
		expectWitness(t, f.witness(), witnessed{witnessB, 70, 75, 1})
	})
	// Nor can a place the wire format has no room for: its offsets are int32.
	t.Run("offsets past int32", func(t *testing.T) {
		f := newWitnessFixture(t)
		f.holds(witnessA, 1<<31, 1<<31+4)
		f.holds(witnessA, 5, 1<<31)
		f.holds(witnessB, 70, 75)
		expectWitness(t, f.witness(), witnessed{witnessB, 70, 75, 1})
	})
}

func TestAReferenceHasNoWitness(t *testing.T) {
	// A reference's own range is where it happened.
	f := newWitnessFixture(t)
	if err := f.idx.AddReference(f.files[witnessZ], f.pos(witnessA, 100), "(m.T).M",
		f.pos(witnessZ, 10), f.pos(witnessZ, 11), nil, RoleCall); err != nil {
		t.Fatalf("AddReference: %v", err)
	}
	row := f.rows()[10]
	if row == nil || row.Witness != nil {
		t.Fatalf("a reference with a witness: %+v", row)
	}
}

func TestARelationIsNotAddedWithoutARelationOrAPlace(t *testing.T) {
	f := newWitnessFixture(t)
	add := func(relation Relation, at Place) error {
		return f.idx.AddRelation(f.files[witnessZ], f.pos(witnessA, 100), "(m.T).M",
			f.pos(witnessZ, 10), f.pos(witnessZ, 11), relation, at)
	}
	file := f.files[witnessA]
	if err := add(NoRelation, Place{File: file, Start: 60, End: 68}); err == nil {
		t.Errorf("a relation that names no relation was accepted")
	}
	if err := add(RelationHolds, Place{File: file, Start: 68, End: 60}); err == nil {
		t.Errorf("a place that ends before it starts was accepted")
	}
	if err := add(RelationHolds, Place{File: file, Start: -1, End: 60}); err == nil {
		t.Errorf("a place that starts before the file was accepted")
	}
	if rows := f.rows(); len(rows) != 0 {
		t.Errorf("a refused relation left a row: %v", rows)
	}
}

func TestARelationThatMeetsAUseAtOneTokenKeepsItsPlace(t *testing.T) {
	// One token has room for one row, and a relation outlives a use there
	// (ResolveReferences).  Whichever was seen first, the row that results
	// says where the relation happened.
	for name, useFirst := range map[string]bool{"use first": true, "relation first": false} {
		t.Run(name, func(t *testing.T) {
			f := newWitnessFixture(t)
			use := func() {
				if err := f.idx.AddReference(f.files[witnessZ], f.pos(witnessA, 100), "(m.T).M",
					f.pos(witnessZ, 10), f.pos(witnessZ, 11), nil, NoRole); err != nil {
					t.Fatalf("AddReference: %v", err)
				}
			}
			if useFirst {
				use()
			}
			f.holds(witnessB, 70, 75)
			if !useFirst {
				use()
			}
			f.holds(witnessA, 60, 68)
			row := f.rows()[10]
			if row == nil || Relation(row.Relation) != RelationHolds {
				t.Fatalf("the relation was lost: %+v", row)
			}
			expectWitness(t, f.witnessOf(row), witnessed{witnessA, 60, 68, 2})
		})
	}
}
