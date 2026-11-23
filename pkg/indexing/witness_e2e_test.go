package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
)

// TestAHoldsRowNamesWhereItHappened runs the whole pipeline and reads the
// written index as the engine will.
//
// A RELATION SAYS WHERE IT HAPPENED.  The row `(Reader).Read -> (*File).Read`
// sits at the interface method's name in reader.go, which says who is
// related.  Its witness is the first expression, in file order, that handed a
// `*File` over where a `Reader` was expected: an object of the same index by
// its local id, a byte range in it, and the number of such places.
func TestAHoldsRowNamesWhereItHappened(t *testing.T) {
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
	write("reader.go", `package main

type Reader interface {
	Read() int
}
`)
	// Two files hand a *File over.  `b_late.go` sorts before `main.go`, and
	// its place is deep in the file; main.go's places come early.
	write("main.go", `package main

type File struct{}

func (f *File) Read() int { return 1 }

type Pipe struct{}

func (p Pipe) Read() int { return 2 }

func open(n int) Reader {
	if n > 0 {
		return &File{}
	}
	return Pipe{}
}

func main() {
	var r Reader = &File{}
	_ = r.Read() + open(1).Read() + late().Read()
}
`)
	write("b_late.go", `package main

// Padding, so that this file's place has a higher offset than main.go's.
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
func late() Reader {
	var f File
	return &f
}
`)

	indexPath := filepath.Join(t.TempDir(), "index.pb")
	if err := ParseModules([]string{root}, root, Options{
		ProjectName: "witness-e2e",
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
	symName := map[int64]string{}
	for _, s := range project.Symbols {
		symName[s.LocalId] = s.Name
	}
	objects := map[int64]*indexpb.Object{}
	for _, obj := range project.Objects {
		if _, twice := objects[obj.LocalId]; twice {
			t.Fatalf("two objects with local id %d", obj.LocalId)
		}
		objects[obj.LocalId] = obj
	}

	var happened []string
	references := 0
	for _, obj := range project.Objects {
		for _, ref := range obj.Refs {
			if ref.Relation == indexpb.RefRelation_REF_RELATION_NONE {
				references++
				if ref.Witness != nil {
					t.Errorf("%s: a reference to %s has a witness", obj.FilesystemPath, symName[ref.ToSymbolLocalId])
				}
				continue
			}
			to := symName[ref.ToSymbolLocalId]
			if filepath.Base(obj.FilesystemPath) != "reader.go" {
				continue // the standard library's own relations
			}
			w := ref.Witness
			if w == nil {
				t.Errorf("the holds row to %s happened nowhere", to)
				continue
			}
			at := objects[w.ObjectLocalId]
			if at == nil {
				t.Errorf("the holds row to %s: witness object %d is not in the index", to, w.ObjectLocalId)
				continue
			}
			if w.OffsetStart < 0 || w.OffsetEnd < w.OffsetStart || int(w.OffsetEnd) > len(at.Content) {
				t.Errorf("the holds row to %s: witness [%d, %d) is outside %s (%d bytes)",
					to, w.OffsetStart, w.OffsetEnd, at.FilesystemPath, len(at.Content))
				continue
			}
			happened = append(happened, fmt.Sprintf("%s @ %s `%s` (first of %d)",
				to, filepath.Base(at.FilesystemPath), at.Content[w.OffsetStart:w.OffsetEnd], w.Sites))
		}
	}
	if references == 0 {
		t.Fatalf("the index has no references at all")
	}
	sort.Strings(happened)
	want := []string{
		// Three places hand a *File over: `&f` in b_late.go, and `&File{}`
		// twice in main.go.  b_late.go is the lower path.
		"(*example.com/m.File).Read @ b_late.go `&f` (first of 3)",
		"(example.com/m.Pipe).Read @ main.go `Pipe{}` (first of 1)",
	}
	if fmt.Sprint(happened) != fmt.Sprint(want) {
		t.Errorf("where the holds rows happened:\n got %q\nwant %q", happened, want)
	}
}
