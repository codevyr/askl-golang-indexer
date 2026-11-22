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

// count is how many of `said` are exactly `line`.
func count(said []string, line string) int {
	n := 0
	for _, s := range said {
		if s == line {
			n++
		}
	}
	return n
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
	// says `calls` of the callee of a call, `takes` of a function named and
	// not called and of data read, `writes` of data stored into, and `holds`
	// of an interface method and a method that satisfies it.  Of the rest it
	// says nothing, which is not "no": the membership rows (type -> method)
	// and the types a declaration names are written as they always were.
	It("says calls of a callee and holds of an interface method", func() {
		Expect(whatReferencesSay("roles")).To(ContainElements(
			// A call of a function.
			"(*roles.File).Read -> roles.helper [calls -]",
			// AN INTERFACE METHOD HOLDS THE METHODS THAT SATISFY IT.  Two
			// types are bound to `Reader`, `*File` twice: one row each.
			"(roles.Reader).Read -> (*roles.File).Read [- holds]",
			"(roles.Reader).Read -> (roles.Pipe).Read [- holds]",
			"roles.Closure -> roles.apply [calls -]",
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
		))
	})

	// A FUNCTION NAMED AND NOT CALLED IS TAKEN.  `register(handler)` used to
	// produce no reference at all, so "who uses this function" missed every
	// callback registration.
	It("says takes of a function used as a value", func() {
		said := whatReferencesSay("roles")
		Expect(said).To(ContainElements(
			// Passed as an argument, beside the call it is passed to.
			"roles.Passed -> roles.double [takes -]",
			"roles.Passed -> roles.register [calls -]",
			// Assigned to a variable; returned.
			"roles.Assigned -> roles.double [takes -]",
			"roles.Returned -> roles.double [takes -]",
			// A method value, a method expression, and a method value of an
			// interface, which takes the interface's method.
			"roles.MethodValue -> (*roles.File).Read [takes -]",
			"roles.MethodExpr -> (*roles.File).Read [takes -]",
			"roles.ThroughValue -> (roles.Reader).Read [takes -]",
			// `go register(double)` calls one and takes the other;
			// `defer double(1)` calls it.
			"roles.Spawned -> roles.register [calls -]",
			"roles.Spawned -> roles.double [takes -]",
			"roles.Spawned -> roles.double [calls -]",
			// A generic function is called through its instantiation, and
			// taken where the instantiation is the value.
			"roles.Instantiated -> roles.pick [calls -]",
			"roles.Instantiated -> roles.pick [takes -]",
			// In the initialiser of a package-level variable.
			"roles.Hook -> roles.double [takes -]",
			"roles.SetsHook -> roles.double [takes -]",
			"roles.Reassigned -> roles.double [takes -]",
		))
		// Another package's function, and a method of an instantiated type,
		// which is the generic method.
		Expect(said).To(ContainElements(
			"roles.Qualified -> roles/shared.Notify [takes -]",
			"roles.Qualified -> roles/shared.Notify [calls -]",
			"roles/shared.Run -> roles/shared.Notify [takes -]",
			"roles.BoxValue -> (roles.Box[T]).Get [takes -]",
		))
		// Parentheses change nothing: `(double)(1)` calls `double`, and
		// `(pick[int])(1)` calls `pick`.
		Expect(said).To(ContainElement("roles.CallsElement -> roles.double [calls -]"))
		Expect(said).To(ContainElement("roles.Parenthesised -> roles.pick [calls -]"))
		Expect(said).ToNot(ContainElement("roles.Parenthesised -> roles.pick [takes -]"))
		// What is no use says nothing: the method an interface type declares
		// where it is written, and `err.Error`, a function of the universe
		// with no declaration to refer to.
		Expect(said).ToNot(ContainElement(HavePrefix("roles.LocalInterface ->")))
		Expect(said).ToNot(ContainElement(HavePrefix("roles.ErrText ->")))
		// In a struct literal and in a map literal: two tokens, two rows.
		Expect(count(said, "roles.Stored -> roles.double [takes -]")).To(Equal(2))
		// A token is one or the other: nothing here is both called and taken.
		Expect(said).ToNot(ContainElement(ContainSubstring("[calls,takes")))
	})

	// A LOCAL VARIABLE THAT NAMES A FUNCTION LITERAL STANDS FOR THE FUNCTION
	// where it is called or handed on, and for itself where it is assigned.
	It("says what is done to a function literal through its variable", func() {
		said := whatReferencesSay("roles")
		Expect(count(said, "roles.Closure -> roles.Closure:<anon> [calls -]")).To(Equal(1))
		Expect(count(said, "roles.Closure -> roles.Closure:<anon> [takes -]")).To(Equal(1))
		// `inner = double` writes the variable; of the literal it says nothing.
		Expect(count(said, "roles.Reassigned -> roles.Reassigned:<anon> [- -]")).To(Equal(1))
		Expect(count(said, "roles.Reassigned -> roles.Reassigned:<anon> [calls -]")).To(Equal(1))
	})

	// PACKAGE-LEVEL DATA IS WRITTEN, READ OR CALLED, AND THE TOKEN SAYS WHICH.
	It("says writes, takes and calls of package-level data", func() {
		said := whatReferencesSay("roles")
		Expect(said).To(ContainElements(
			"roles.Direct -> roles.Limit [takes -]",
			"roles.ReadsCount -> roles.Count [takes -]",
			"roles.WritesCount -> roles.Count [writes -]",
			// `Count++` and `Count += n` read what they write: one token,
			// both roles.
			"roles.BumpsCount -> roles.Count [writes,takes -]",
			"roles.AddsCount -> roles.Count [writes,takes -]",
			// Its address is taken.
			"roles.PointsAtCount -> roles.Count [takes -]",
			// A variable that holds a function, called: the callee token is
			// the variable.
			"roles.CallsHook -> roles.Hook [calls -]",
			"roles.SetsHook -> roles.Hook [writes -]",
			// A constant can only be read.
			"roles.ReadsMax -> roles.Max [takes -]",
		))
		// `for Count = range xs` and `for _, Count = range xs`.
		Expect(count(said, "roles.RangesInto -> roles.Count [writes -]")).To(Equal(2))
		// Another package's variable, written, bumped and called.
		Expect(said).To(ContainElements(
			"roles.Qualified -> roles/shared.Level [writes -]",
			"roles.Qualified -> roles/shared.Level [writes,takes -]",
			"roles.Qualified -> roles/shared.Run [calls -]",
		))
		// `Hooks[0](1)` calls an element: no token is the callee, and `Hooks`
		// is read.  `Head.Next = nil` stores where `Head` points: `Head` is
		// read.
		Expect(said).To(ContainElements(
			"roles.CallsElement -> roles.Hooks [takes -]",
			"roles.WritesThrough -> roles.Head [takes -]",
		))
		Expect(said).ToNot(ContainElement("roles.CallsElement -> roles.Hooks [calls -]"))
		// A field and an array element of a variable are part of it, so
		// storing into them writes it; a map element is elsewhere, and
		// finding it reads the variable.
		Expect(count(said, "roles.WritesInside -> roles.Config [writes -]")).To(Equal(2))
		Expect(count(said, "roles.WritesInside -> roles.Config [takes -]")).To(Equal(1))
		// Every use of data in the fixture states a role.
		for _, s := range said {
			for _, data := range []string{"roles.Count ", "roles.Limit ", "roles.Hook ", "roles.Max ", "roles.Config ",
				"roles.Hooks ", "roles.Head ", "roles/shared.Level ", "roles/shared.Run "} {
				if strings.Contains(s, "-> "+data) {
					Expect(s).ToNot(HaveSuffix("[- -]"), s)
				}
			}
		}
	})

	It("says this of the fixture and nothing else", func() {
		// Every row, so that a reference nobody meant shows up here.  The
		// rows that state nothing are the ones that never will: membership
		// (type -> method), the type a declaration names, and a variable
		// assigned where it stands for a function literal.
		Expect(whatReferencesSay("roles")).To(Equal([]string{
			"(*roles.File).Read -> roles.helper [calls -]",
			"(roles.Reader).Read -> (*roles.File).Read [- holds]",
			"(roles.Reader).Read -> (roles.Pipe).Read [- holds]",
			"roles -> roles/shared [- -]",
			"roles.AddsCount -> roles.Count [writes,takes -]",
			"roles.Assigned -> roles.double [takes -]",
			"roles.Box -> (roles.Box[T]).Get [- -]",
			"roles.BoxValue -> (roles.Box[T]).Get [takes -]",
			"roles.BumpsCount -> roles.Count [writes,takes -]",
			"roles.CallsElement -> roles.Hooks [takes -]",
			"roles.CallsElement -> roles.double [calls -]",
			"roles.CallsHook -> roles.Hook [calls -]",
			"roles.Closure -> roles.Closure:<anon> [calls -]",
			"roles.Closure -> roles.Closure:<anon> [takes -]",
			"roles.Closure -> roles.apply [calls -]",
			"roles.Config -> builtin.bool [- -]",
			"roles.Config -> builtin.int [- -]",
			"roles.Config -> builtin.string [- -]",
			"roles.Count -> builtin.int [- -]",
			"roles.Direct -> roles.Limit [takes -]",
			"roles.Direct -> roles.helper [calls -]",
			"roles.File -> (*roles.File).Read [- -]",
			"roles.Head -> roles.Node [- -]",
			"roles.Hook -> roles.Handler [- -]",
			"roles.Hook -> roles.double [takes -]",
			"roles.Hooks -> roles.Handler [- -]",
			"roles.Instantiated -> roles.pick [calls -]",
			"roles.Instantiated -> roles.pick [takes -]",
			"roles.Method -> (*roles.File).Read [calls -]",
			"roles.MethodExpr -> (*roles.File).Read [takes -]",
			"roles.MethodValue -> (*roles.File).Read [takes -]",
			"roles.Node -> roles.Node [- -]",
			"roles.Parenthesised -> roles.pick [calls -]",
			"roles.Passed -> roles.double [takes -]",
			"roles.Passed -> roles.register [calls -]",
			"roles.Pipe -> (roles.Pipe).Read [- -]",
			"roles.PointsAtCount -> roles.Count [takes -]",
			"roles.Qualified -> roles.register [calls -]",
			"roles.Qualified -> roles/shared.Level [writes -]",
			"roles.Qualified -> roles/shared.Level [writes,takes -]",
			"roles.Qualified -> roles/shared.Notify [calls -]",
			"roles.Qualified -> roles/shared.Notify [takes -]",
			"roles.Qualified -> roles/shared.Run [calls -]",
			"roles.RangesInto -> roles.Count [writes -]",
			"roles.RangesInto -> roles.Count [writes -]",
			"roles.Reader -> (roles.Reader).Read [- -]",
			"roles.ReadsCount -> roles.Count [takes -]",
			"roles.ReadsMax -> roles.Max [takes -]",
			"roles.Reassigned -> roles.Reassigned:<anon> [- -]",
			"roles.Reassigned -> roles.Reassigned:<anon> [calls -]",
			"roles.Reassigned -> roles.double [takes -]",
			"roles.Returned -> roles.double [takes -]",
			"roles.SetsHook -> roles.Hook [writes -]",
			"roles.SetsHook -> roles.double [takes -]",
			"roles.Spawned -> roles.double [calls -]",
			"roles.Spawned -> roles.double [takes -]",
			"roles.Spawned -> roles.register [calls -]",
			"roles.Stored -> roles.double [takes -]",
			"roles.Stored -> roles.double [takes -]",
			"roles.Table -> roles.Handler [- -]",
			"roles.Through -> (roles.Reader).Read [calls -]",
			"roles.ThroughValue -> (roles.Reader).Read [takes -]",
			"roles.WritesCount -> roles.Count [writes -]",
			"roles.WritesInside -> roles.Config [takes -]",
			"roles.WritesInside -> roles.Config [writes -]",
			"roles.WritesInside -> roles.Config [writes -]",
			"roles.WritesThrough -> roles.Head [takes -]",
			"roles/shared.Level -> builtin.int [- -]",
			"roles/shared.Run -> roles/shared.Notify [takes -]",
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
