package indexing

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
)

// TestAHoldsRowSitsDirectlyInItsInterfaceMethod runs the whole pipeline and
// reads the index as the engine will.
//
// A RELATION BELONGS TO ITS SOURCE ONLY.  The engine attributes a reference to
// every declaration around it, and a relation row to the ONE declaration it
// sits directly in: the instance one nesting level above the row whose DFS
// interval contains it.  For `(Reader).Read -> (*File).Read` that instance
// must be the interface method -- not the interface, and not the file.
func TestAHoldsRowSitsDirectlyInItsInterfaceMethod(t *testing.T) {
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
	// The interface in one file and what satisfies it in another, so the row
	// has to be written in the interface's file to belong to its method.
	write("reader.go", `package main

// Reader is read from.
type Reader interface {
	// Read reads.
	Read() int
	Close()
}
`)
	write("main.go", `package main

type File struct{}

func (f *File) Read() int { return 1 }
func (f *File) Close()    {}

type Pipe struct{}

func (p Pipe) Read() int { return 2 }
func (p Pipe) Close()    {}

func open(n int) Reader {
	if n > 0 {
		return &File{}
	}
	return Pipe{}
}

func main() {
	r := open(1)
	_ = r.Read()
	r.Close()
}
`)

	indexPath := filepath.Join(t.TempDir(), "index.pb")
	if err := ParseModules([]string{root}, root, Options{
		ProjectName: "roles-e2e",
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
	symByID := make(map[int64]*indexpb.Symbol, len(project.Symbols))
	for _, s := range project.Symbols {
		symByID[s.LocalId] = s
	}
	name := func(id int64) string {
		if s := symByID[id]; s != nil {
			return s.Name
		}
		return "?"
	}

	held := map[string]bool{}
	calls := map[string]bool{}
	for _, obj := range project.Objects {
		for _, ref := range obj.Refs {
			// The instances the engine would attribute this row to as a
			// relation: one level up, around it in the containment order.
			var owners []*indexpb.SymbolInstance
			for _, inst := range obj.SymbolInstances {
				if inst.NestingLevel+1 == ref.NestingLevel &&
					inst.DfsEnter < ref.DfsEnter && ref.DfsEnter < inst.DfsExit {
					owners = append(owners, inst)
				}
			}
			to := name(ref.ToSymbolLocalId)

			if ref.Roles&uint32(indexpb.RefRole_REF_ROLE_CALL) != 0 {
				if len(owners) == 1 {
					calls[name(owners[0].SymbolLocalId)+" -> "+to] = true
				}
			}
			if ref.Relation == indexpb.RefRelation_REF_RELATION_NONE {
				continue
			}
			if ref.Relation != indexpb.RefRelation_REF_RELATION_HOLDS {
				t.Errorf("%s: a %v row to %s; Go has only holds", obj.FilesystemPath, ref.Relation, to)
				continue
			}
			if ref.Roles != 0 {
				t.Errorf("a holds row to %s states roles %d: no token says it", to, ref.Roles)
			}
			if filepath.Base(obj.FilesystemPath) != "reader.go" {
				t.Errorf("a holds row to %s is written in %s, not in the interface's file",
					to, obj.FilesystemPath)
			}
			if len(owners) != 1 {
				t.Errorf("a holds row to %s has %d owners one level up, want exactly 1", to, len(owners))
				continue
			}
			owner := symByID[owners[0].SymbolLocalId]
			if owner == nil || owner.Type != indexpb.SymbolType_FIELD {
				t.Errorf("a holds row to %s belongs to %s (%v), want the interface method (a FIELD)",
					to, name(owners[0].SymbolLocalId), owner.GetType())
				continue
			}
			held[owner.Name+" -> "+to] = true
		}
	}

	for _, want := range []string{
		"(example.com/m.Reader).Read -> (*example.com/m.File).Read",
		"(example.com/m.Reader).Read -> (example.com/m.Pipe).Read",
		"(example.com/m.Reader).Close -> (*example.com/m.File).Close",
		"(example.com/m.Reader).Close -> (example.com/m.Pipe).Close",
	} {
		if !held[want] {
			t.Errorf("no holds row %s; have %v", want, held)
		}
	}
	if len(held) != 4 {
		t.Errorf("holds rows: %v, want exactly four", held)
	}
	for _, want := range []string{
		"example.com/m.main -> example.com/m.open",
		"example.com/m.main -> (example.com/m.Reader).Read",
		"example.com/m.main -> (example.com/m.Reader).Close",
	} {
		if !calls[want] {
			t.Errorf("no call %s; have %v", want, calls)
		}
	}
}
