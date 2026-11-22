package index

import (
	"go/token"
	"testing"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
)

// TestSightingsOfOneTokenMergeByUnion: two sightings of one token referring to
// one symbol are one row -- that is the dedup key -- and the row says what
// EITHER sighting said.  Dropping the second sighting, as the dedup did before
// roles existed, would keep whichever came first: a call seen second would be
// stored as a reference that states nothing.
func TestSightingsOfOneTokenMergeByUnion(t *testing.T) {
	type sighting struct {
		start, end int
		roles      Roles
		relation   Relation
	}
	type row struct {
		start, end int32
		roles      Roles
		relation   Relation
	}
	build := func(t *testing.T, sightings []sighting) []row {
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
		for _, s := range sightings {
			if err := idx.AddReference(fileID, pos(10), "m.Run", pos(s.start), pos(s.end), nil,
				s.roles, s.relation); err != nil {
				t.Fatalf("AddReference: %v", err)
			}
		}
		if err := idx.ResolveReferences(); err != nil {
			t.Fatalf("ResolveReferences: %v", err)
		}
		var rows []row
		for _, obj := range idx.Upload().Objects {
			if obj.FilesystemPath != path {
				continue
			}
			for _, r := range obj.Refs {
				rows = append(rows, row{r.FromOffsetStart, r.FromOffsetEnd,
					Roles(r.Roles), Relation(r.Relation)})
			}
		}
		return rows
	}
	same := func(t *testing.T, got, want []row) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("rows: got %+v, want %+v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("row %d: got %+v, want %+v", i, got[i], want[i])
			}
		}
	}

	t.Run("a call seen after a plain sighting", func(t *testing.T) {
		same(t, build(t, []sighting{
			{40, 45, NoRole, NoRelation},
			{40, 45, RoleCall, NoRelation},
		}), []row{{40, 45, RoleCall, NoRelation}})
	})
	t.Run("a call seen before a plain sighting", func(t *testing.T) {
		same(t, build(t, []sighting{
			{40, 45, RoleCall, NoRelation},
			{40, 45, NoRole, NoRelation},
		}), []row{{40, 45, RoleCall, NoRelation}})
	})
	t.Run("two roles of one token", func(t *testing.T) {
		same(t, build(t, []sighting{
			{40, 45, RoleWrite, NoRelation},
			{40, 45, RoleTake, NoRelation},
		}), []row{{40, 45, RoleWrite | RoleTake, NoRelation}})
	})
	t.Run("another token keeps what it said", func(t *testing.T) {
		same(t, build(t, []sighting{
			{40, 45, RoleCall, NoRelation},
			{50, 55, NoRole, NoRelation},
		}), []row{{40, 45, RoleCall, NoRelation}, {50, 55, NoRole, NoRelation}})
	})
	t.Run("a relation said once", func(t *testing.T) {
		same(t, build(t, []sighting{
			{40, 45, NoRole, RelationHolds},
		}), []row{{40, 45, NoRole, RelationHolds}})
	})
	t.Run("a relation said twice is one row", func(t *testing.T) {
		same(t, build(t, []sighting{
			{40, 45, NoRole, RelationHolds},
			{40, 45, NoRole, RelationHolds},
		}), []row{{40, 45, NoRole, RelationHolds}})
	})
}

// TestRoleBitsAreTheWireFormats pins the numbers the engine reads.  They are
// taken from the generated enum, so this fails only if index.proto changes
// under them -- which must not happen without the engine.
func TestRoleBitsAreTheWireFormats(t *testing.T) {
	if RoleCall != 1 || RoleWrite != 2 || RoleTake != 4 {
		t.Fatalf("roles: call=%d write=%d take=%d", RoleCall, RoleWrite, RoleTake)
	}
	if NoRelation != 0 || RelationHolds != 2 ||
		indexpb.RefRelation_REF_RELATION_DECLARES != 1 {
		t.Fatalf("relations: none=%d holds=%d", NoRelation, RelationHolds)
	}
}
