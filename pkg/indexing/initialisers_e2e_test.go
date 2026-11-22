package indexing

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
)

// TestAnInitialiserIsCode runs the whole pipeline over package-level
// declarations and reads the index as the engine will.
//
// AN INITIALISER IS CODE.  A table of handlers and a variable computed by a
// call are where most registrations in a Go program are written, and they
// are read as a function body is: a function named there is taken, a function
// called there is called, and the reference belongs to every declaration
// around it -- the variable whose initialiser it is, when one name claims it.
func TestAnInitialiserIsCode(t *testing.T) {
	root := t.TempDir()
	const source = `package main

type Handler func() int

func run() int { return 1 }

func compute() int { return 2 }

func register(h Handler) int { return h() }

func pair(h Handler) (int, int) { return 0, 0 }

type File struct{}

func (f *File) Read() int { return 0 }

var handlers = map[string]Handler{"a": run}

var limit = compute() + base

const base = 3

var _ = register(run)

var first, second = pair(run)

var asReader, isReader = any(&File{}).(interface{ Read() int })

var one, two = func() int { return run() }, run

func main() {}
`
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	indexPath := filepath.Join(t.TempDir(), "index.pb")
	if err := ParseModules([]string{root}, root, Options{
		ProjectName: "initialisers-e2e",
		IndexPath:   indexPath,
		ParseTypes:  true,
	}); err != nil {
		t.Fatalf("ParseModules: %v", err)
	}
	project := readIndexProject(t, indexPath)
	symByID := make(map[int64]*indexpb.Symbol, len(project.Symbols))
	for _, s := range project.Symbols {
		symByID[s.LocalId] = s
	}

	const pkg = "example.com/m."
	// Every reference of main.go as "line: token -> target [roles] in <the
	// functions and data around it>".
	var said []string
	interfaceMethods := 0
	for _, obj := range project.Objects {
		if filepath.Base(obj.FilesystemPath) != "main.go" {
			continue
		}
		for _, inst := range obj.SymbolInstances {
			if s := symByID[inst.SymbolLocalId]; s != nil && s.Name == "(interface).Read" {
				interfaceMethods++
			}
		}
		for _, ref := range obj.Refs {
			to := symByID[ref.ToSymbolLocalId]
			if to == nil || !strings.HasPrefix(to.Name, pkg) ||
				(to.Type != indexpb.SymbolType_FUNCTION && to.Type != indexpb.SymbolType_DATA) {
				continue
			}
			var around []string
			for _, inst := range obj.SymbolInstances {
				s := symByID[inst.SymbolLocalId]
				if s == nil || (s.Type != indexpb.SymbolType_FUNCTION && s.Type != indexpb.SymbolType_DATA) {
					continue
				}
				if inst.DfsEnter < ref.DfsEnter && ref.DfsEnter < inst.DfsExit {
					around = append(around, strings.TrimPrefix(s.Name, pkg))
				}
			}
			slices.Sort(around)
			line := 1 + strings.Count(source[:ref.FromOffsetStart], "\n")
			token := source[ref.FromOffsetStart:ref.FromOffsetEnd]
			var roles []string
			for _, role := range []struct {
				bit  indexpb.RefRole
				word string
			}{{indexpb.RefRole_REF_ROLE_CALL, "calls"}, {indexpb.RefRole_REF_ROLE_WRITE, "writes"}, {indexpb.RefRole_REF_ROLE_TAKE, "takes"}} {
				if ref.Roles&uint32(role.bit) != 0 {
					roles = append(roles, role.word)
				}
			}
			said = append(said, strings.TrimSpace(strings.Join([]string{
				strings.Split(source, "\n")[line-1], "|", token, strings.Join(roles, ","), "in", strings.Join(around, ","),
			}, " ")))
		}
	}
	slices.Sort(said)

	want := []string{
		// A name with its own initialiser has what is in it.
		`var handlers = map[string]Handler{"a": run} | run takes in handlers`,
		`var limit = compute() + base | base takes in limit`,
		`var limit = compute() + base | compute calls in limit`,
		// Initialisers no name claims belong to no declaration: a blank
		// name, and one value for two names.
		`var _ = register(run) | register calls in`,
		`var _ = register(run) | run takes in`,
		`var first, second = pair(run) | pair calls in`,
		`var first, second = pair(run) | run takes in`,
		// Each name of a grouped declaration is its identifier alone, so what
		// its initialiser names sits beside it; a function literal is the
		// name's own, and what the literal calls is the literal's and so the
		// name's.
		`var one, two = func() int { return run() }, run | run calls in one,one:<anon>`,
		`var one, two = func() int { return run() }, run | run takes in`,
	}
	for i := range said {
		// A literal's name carries its byte offset.
		if at := strings.Index(said[i], ":<anon"); at >= 0 {
			said[i] = said[i][:at] + ":<anon>"
		}
	}
	slices.Sort(said)
	slices.Sort(want)
	if !slices.Equal(said, want) {
		t.Errorf("an initialiser's references:\n  have:\n    %s\n  want:\n    %s",
			strings.Join(said, "\n    "), strings.Join(want, "\n    "))
	}
	// The interface type in an initialiser is declared once.
	if interfaceMethods != 1 {
		t.Errorf("(interface).Read has %d instances, want 1", interfaceMethods)
	}
}
