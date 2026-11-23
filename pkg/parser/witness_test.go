package parser_test

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
	"github.com/planetA/askl-golang-indexer/pkg/parser"
)

// relationRow is one relation row of the index, read as the engine reads it.
type relationRow struct {
	from, to string
	// Where the row sits, and the witness's object when it has one.
	in, at *indexpb.Object
	ref    *indexpb.SymbolRef
}

// relationsOf indexes one fixture package and returns its relation rows.
func relationsOf(testDir string) ([]relationRow, string) {
	idx, err := index.NewProtoIndex(index.WithProject("test_project"))
	Expect(err).ToNot(HaveOccurred())
	defer idx.Close()

	cwd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	fixtureRoot := fmt.Sprintf("%s/test/src/%s/", cwd, testDir)

	pkgParser := parser.NewParser(fixtureRoot, idx)
	defer pkgParser.Close()
	Expect(pkgParser.Load()).To(Succeed())
	Expect(pkgParser.AddPackages()).To(Succeed())
	Expect(idx.ResolveReferences()).To(Succeed())
	Expect(idx.Wait()).To(Succeed())

	project := idx.Upload()
	symbolName := map[int64]string{}
	for _, s := range project.Symbols {
		symbolName[s.LocalId] = s.Name
	}
	objectByID := map[int64]*indexpb.Object{}
	for _, obj := range project.Objects {
		objectByID[obj.LocalId] = obj
	}

	var rows []relationRow
	for _, obj := range project.Objects {
		for _, ref := range obj.Refs {
			if ref.Relation == indexpb.RefRelation_REF_RELATION_NONE {
				// A reference's own range is where it happened.
				Expect(ref.Witness).To(BeNil(), "a reference in %s has a witness", obj.FilesystemPath)
				continue
			}
			// The source is the narrowest declaration around the row.
			var owner *indexpb.SymbolInstance
			for _, inst := range obj.SymbolInstances {
				if inst.StartOffset <= ref.FromOffsetStart && ref.FromOffsetEnd <= inst.EndOffset &&
					(owner == nil || inst.EndOffset-inst.StartOffset < owner.EndOffset-owner.StartOffset) {
					owner = inst
				}
			}
			Expect(owner).ToNot(BeNil())
			row := relationRow{
				from: symbolName[owner.SymbolLocalId],
				to:   symbolName[ref.ToSymbolLocalId],
				in:   obj,
				ref:  ref,
			}
			if ref.Witness != nil {
				row.at = objectByID[ref.Witness.ObjectLocalId]
				Expect(row.at).ToNot(BeNil(),
					"%s -> %s: the witness names object %d, which the index does not hold",
					row.from, row.to, ref.Witness.ObjectLocalId)
			}
			rows = append(rows, row)
		}
	}
	return rows, fixtureRoot
}

// happened says where a relation was made so: the witness's file, its line,
// the text it covers, and how many places there are.
func (r relationRow) happened(fixtureRoot string) string {
	if r.ref.Witness == nil {
		return "nowhere"
	}
	w := r.ref.Witness
	content := r.at.Content
	Expect(int(w.OffsetStart)).To(BeNumerically("<=", int(w.OffsetEnd)))
	Expect(int(w.OffsetEnd)).To(BeNumerically("<=", len(content)))
	line := 1 + bytes.Count(content[:w.OffsetStart], []byte("\n"))
	return fmt.Sprintf("%s:%d `%s` (first of %d)",
		strings.TrimPrefix(r.at.FilesystemPath, fixtureRoot), line,
		content[w.OffsetStart:w.OffsetEnd], w.Sites)
}

// whereRelationsHappened is every relation row with an end in the fixture, as
// "from -> to @ file:line `text` (first of N)", sorted.
func whereRelationsHappened(testDir string) []string {
	rows, fixtureRoot := relationsOf(testDir)
	const importPath = "github.com/planetA/askl-golang-indexer/pkg/parser/test/src/"
	said := []string{}
	for _, r := range rows {
		if !strings.Contains(r.from, importPath) && !strings.Contains(r.to, importPath) {
			continue
		}
		said = append(said, fmt.Sprintf("%s -> %s @ %s",
			strings.ReplaceAll(r.from, importPath, ""),
			strings.ReplaceAll(r.to, importPath, ""),
			r.happened(fixtureRoot)))
	}
	slices.Sort(said)
	return said
}

var _ = Describe("Where a relation happened", func() {
	// AN INTERFACE METHOD HOLDS A METHOD BECAUSE A VALUE OF THE METHOD'S TYPE
	// WAS HANDED OVER WHERE THE INTERFACE WAS EXPECTED, AND THAT EXPRESSION IS
	// WHERE IT HAPPENED.  The row sits at the interface method's name and says
	// who is related; its witness is the first such expression in file order
	// (lowest path, then lowest offset), with how many there are.
	const (
		file   = "(witness/kinds.Reader).Read -> (*witness/kinds.File).Read"
		pipe   = "(witness/kinds.Reader).Read -> (witness/kinds.Pipe).Read"
		sock   = "(witness/kinds.Reader).Read -> (witness/kinds.Sock).Read"
		stream = "(witness/kinds.ReadCloser).Read -> (*witness/kinds.Stream).Read"
		closes = "(witness/kinds.ReadCloser).Close -> (*witness/kinds.Stream).Close"
		anon   = "(witness/kinds.Reader).Read -> (interface).Read"
	)

	It("names the expression handed over, the first in file order, and counts them all", func() {
		said := whereRelationsHappened("witness")

		// A `*File` is handed over six times: returned in kinds.go; assigned
		// twice, passed once, and yielded by `open()` and by `both()` (as the
		// second of two values) in witness.go.  kinds.go is the lower path, so
		// its place is first -- though its offset is not the lowest.
		Expect(said).To(ContainElement(file + " @ kinds/kinds.go:38 `&File{}` (first of 6)"))
		// A package-level `var _ Reader = Pipe{}` says it as a return does.
		Expect(said).To(ContainElement(pipe + " @ kinds/kinds.go:34 `Pipe{}` (first of 2)"))
		// Sock's method is declared in kinds/sock.go and the interface in
		// kinds/kinds.go; it happened in neither.
		Expect(said).To(ContainElement(sock + " @ witness.go:52 `kinds.Sock{}` (first of 1)"))
		// One place makes every method of the interface hold its method.
		Expect(said).To(ContainElement(stream + " @ witness.go:58 `&kinds.Stream{}` (first of 1)"))
		Expect(said).To(ContainElement(closes + " @ witness.go:58 `&kinds.Stream{}` (first of 1)"))
		// An interface written out, handed over as itself and as the
		// constraint of a type parameter.
		Expect(said).To(ContainElement(anon + " @ witness.go:65 `x` (first of 2)"))
	})

	It("names a place of this module for a relation that crosses its edge", func() {
		// The rows sit in the standard library's files, where the interfaces
		// are declared; what made them so is written here.
		Expect(whereRelationsHappened("witness")).To(ContainElements(
			"(fmt.Stringer).String -> (witness.Name).String @ witness.go:90 `Name(\"a\")` (first of 1)",
			"(builtin.error).Error -> (witness.Failure).Error @ witness.go:90 `Failure{}` (first of 1)",
		))
	})

	It("says where every relation of the fixture happened, and no more", func() {
		Expect(whereRelationsHappened("witness")).To(Equal([]string{
			"(builtin.error).Error -> (witness.Failure).Error @ witness.go:90 `Failure{}` (first of 1)",
			"(fmt.Stringer).String -> (witness.Name).String @ witness.go:90 `Name(\"a\")` (first of 1)",
			closes + " @ witness.go:58 `&kinds.Stream{}` (first of 1)",
			stream + " @ witness.go:58 `&kinds.Stream{}` (first of 1)",
			file + " @ kinds/kinds.go:38 `&File{}` (first of 6)",
			anon + " @ witness.go:65 `x` (first of 2)",
			pipe + " @ kinds/kinds.go:34 `Pipe{}` (first of 2)",
			sock + " @ witness.go:52 `kinds.Sock{}` (first of 1)",
		}))
	})

	It("gives every relation a witness in an object of the index, the standard library's too", func() {
		// `os.Stdout` handed over as an `io.Writer`: both ends are the
		// standard library's, and so may the first place be.  Wherever it is,
		// it is an object this index holds (relationsOf checks that of every
		// witness), and this module's place is one of those counted.
		rows, _ := relationsOf("witness")
		Expect(rows).ToNot(BeEmpty())
		found := false
		for _, r := range rows {
			Expect(r.ref.Witness).ToNot(BeNil(), "%s -> %s happened nowhere", r.from, r.to)
			Expect(r.ref.Witness.Sites).To(BeNumerically(">=", 1))
			Expect(r.ref.Roles).To(BeZero())
			if r.from == "(io.Writer).Write" && r.to == "(*os.File).Write" {
				found = true
			}
		}
		Expect(found).To(BeTrue(), "no row (io.Writer).Write -> (*os.File).Write")
	})
})
