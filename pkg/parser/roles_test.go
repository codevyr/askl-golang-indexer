package parser_test

import (
	"fmt"
	"os"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/parser"
)

// whatReferencesSay indexes one fixture package and returns every reference
// that touches it as "from -> to [roles relation]", sorted, with the fixture's
// import path cut from the names.
func whatReferencesSay(testDir string) []string {
	idx, err := index.NewProtoIndex(index.WithProject("test_project"))
	Expect(err).ToNot(HaveOccurred())
	defer idx.Close()

	cwd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	fixtureRoot := fmt.Sprintf("%s/test/src/", cwd)

	pkgParser := parser.NewParser(fixtureRoot+testDir, idx)
	defer pkgParser.Close()
	Expect(pkgParser.Load()).To(Succeed())
	Expect(pkgParser.AddPackages()).To(Succeed())
	Expect(idx.ResolveReferences()).To(Succeed())
	Expect(idx.Wait()).To(Succeed())

	symbols, err := idx.GetAllSymbols()
	Expect(err).ToNot(HaveOccurred())
	ownNames := map[string]bool{}
	for _, s := range ownSymbols(symbols, fixtureRoot) {
		ownNames[s.Name] = true
	}
	references, err := idx.GetAllReferencesNames()
	Expect(err).ToNot(HaveOccurred())

	const importPath = "github.com/planetA/askl-golang-indexer/pkg/parser/test/src/"
	said := []string{}
	for _, r := range ownReferences(references, fixtureRoot, ownNames) {
		said = append(said, fmt.Sprintf("%s -> %s [%s %s]",
			unnumbered(strings.ReplaceAll(r.From, importPath, "")),
			unnumbered(strings.ReplaceAll(r.To, importPath, "")),
			r.Roles, r.Relation))
	}
	slices.Sort(said)
	return said
}

// unnumbered cuts the byte offset from a function literal's name
// (`Outer:<anon113>`), so an expectation does not move with the fixture's text.
func unnumbered(name string) string {
	const mark = "<anon"
	at := strings.Index(name, mark)
	if at < 0 {
		return name
	}
	end := at + len(mark)
	for end < len(name) && name[end] >= '0' && name[end] <= '9' {
		end++
	}
	return name[:at+len(mark)] + unnumbered(name[end:])
}

var _ = Describe("What a reference says", func() {
	// A ROLE IS WHAT THE TOKEN DOES, AND ABSENCE STATES NOTHING.  The indexer
	// says `calls` of the callee of a call and `holds` of an interface method
	// and a method that satisfies it.  Of everything else it says nothing,
	// which is not "no": a variable read, a function passed as a value and the
	// membership rows (type -> method) are written as they always were.
	It("says calls of a callee and holds of an interface method", func() {
		Expect(whatReferencesSay("roles")).To(Equal([]string{
			// A call of a function.
			"(*roles.File).Read -> roles.helper [calls -]",
			// AN INTERFACE METHOD HOLDS THE METHODS THAT SATISFY IT.  Two
			// types are bound to `Reader`, `*File` twice: one row each.
			"(roles.Reader).Read -> (*roles.File).Read [- holds]",
			"(roles.Reader).Read -> (roles.Pipe).Read [- holds]",
			// `twice` names a function literal.  Passed to `apply` it is a
			// value, and nothing is stated; in `twice(2)` it is the callee.
			"roles.Closure -> roles.Closure:<anon> [- -]",
			"roles.Closure -> roles.Closure:<anon> [calls -]",
			"roles.Closure -> roles.apply [calls -]",
			// A package-level variable read: nothing is stated.
			"roles.Direct -> roles.Limit [- -]",
			"roles.Direct -> roles.helper [calls -]",
			// Membership, type -> method: nothing is stated.
			"roles.File -> (*roles.File).Read [- -]",
			// A call of a method.
			"roles.Method -> (*roles.File).Read [calls -]",
			"roles.Pipe -> (roles.Pipe).Read [- -]",
			// Membership, interface -> method.
			"roles.Reader -> (roles.Reader).Read [- -]",
			// A call through an interface value calls the interface's method.
			"roles.Through -> (roles.Reader).Read [calls -]",
		}))
	})

	It("says holds once however often a type is bound", func() {
		// duplicate_refs binds `*PathError` to `error` twice.
		said := whatReferencesSay("duplicate_refs")
		Expect(said).To(ContainElement(
			"(builtin.error).Error -> (*duplicate_refs.PathError).Error [- holds]"))
		holds := 0
		for _, s := range said {
			if strings.HasSuffix(s, " holds]") {
				holds++
			}
		}
		Expect(holds).To(Equal(1))
	})

	It("says calls of a builtin", func() {
		Expect(whatReferencesSay("mock1")).To(Equal(
			[]string{"mock1.MockFunction -> builtin.print [calls -]"}))
	})

	It("says holds of a method of an unnamed interface", func() {
		// `remover interface{ Has(int) bool }` is a slot like any other.
		Expect(whatReferencesSay("anonymous_interface")).To(ContainElements(
			"(interface).Has -> (anonymous_interface.fieldNum).Has [- holds]",
			"anonymous_interface.ClearUnknown -> (interface).Has [calls -]",
		))
	})
})
