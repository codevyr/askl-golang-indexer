package parser_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/parser"
)

var builtinSymbols = []*index.SymbolDecl{
	index.NewSymbol(1, 1, "builtin.append", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.cap", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.clear", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.close", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.complex", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.copy", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.delete", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.imag", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.len", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.make", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.max", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.min", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.new", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.panic", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.print", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.println", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.real", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "builtin.recover", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(1, 1, "(builtin.error).Error", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	// builtin TYPE symbols
	index.NewSymbol(1, 1, "builtin.bool", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.uint8", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.uint16", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.uint32", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.uint64", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.int8", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.int16", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.int32", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.int64", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.float32", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.float64", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.complex64", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.complex128", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.string", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.int", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.uint", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.uintptr", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.byte", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.rune", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.any", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.comparable", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.Type", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.Type1", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.IntegerType", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.FloatType", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.ComplexType", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
	index.NewSymbol(1, 1, "builtin.error", index.ScopeLocal, index.SymbolTypeType, nil, nil),
	// builtin DATA symbols
	index.NewSymbol(1, 1, "builtin.true", index.ScopeLocal, index.SymbolTypeData, nil, nil),
	index.NewSymbol(1, 1, "builtin.false", index.ScopeLocal, index.SymbolTypeData, nil, nil),
	index.NewSymbol(1, 1, "builtin.iota", index.ScopeLocal, index.SymbolTypeData, nil, nil),
	index.NewSymbol(1, 1, "builtin.nil", index.ScopeLocal, index.SymbolTypeData, nil, nil),
	// unsafe package functions (manually parsed from compiler-provided package)
	index.NewSymbol(2, 2, "unsafe.Sizeof", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.Offsetof", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.Alignof", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.Add", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.Slice", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.SliceData", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.String", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.StringData", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	// unsafe TYPE symbols
	index.NewSymbol(2, 2, "unsafe.Pointer", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
	// cmp package functions
	index.NewSymbol(3, 3, "cmp.Compare", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(3, 3, "cmp.Less", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(3, 3, "cmp.Or", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(3, 3, "cmp.isNaN", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
	// cmp TYPE symbols
	index.NewSymbol(3, 3, "cmp.Ordered", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
}

var builtinReferences = []*index.ReferenceNames{
	index.NewReferenceNames("cmp.Less", "cmp.isNaN"),
	index.NewReferenceNames("cmp.Less", "cmp.isNaN"),
	index.NewReferenceNames("cmp.Compare", "cmp.isNaN"),
	index.NewReferenceNames("cmp.Compare", "cmp.isNaN"),
	// Module import: builtin imports cmp
	index.NewReferenceNames("builtin", "cmp"),
	// TYPE->FUNCTION: error interface -> Error method
	index.NewReferenceNames("builtin.error", "(builtin.error).Error"),
	// TYPE->TYPE: builtin type definitions (type X X pattern in builtin.go)
	index.NewReferenceNames("builtin.bool", "builtin.bool"),
	index.NewReferenceNames("builtin.uint8", "builtin.uint8"),
	index.NewReferenceNames("builtin.uint16", "builtin.uint16"),
	index.NewReferenceNames("builtin.uint32", "builtin.uint32"),
	index.NewReferenceNames("builtin.uint64", "builtin.uint64"),
	index.NewReferenceNames("builtin.int8", "builtin.int8"),
	index.NewReferenceNames("builtin.int16", "builtin.int16"),
	index.NewReferenceNames("builtin.int32", "builtin.int32"),
	index.NewReferenceNames("builtin.int64", "builtin.int64"),
	index.NewReferenceNames("builtin.float32", "builtin.float32"),
	index.NewReferenceNames("builtin.float64", "builtin.float64"),
	index.NewReferenceNames("builtin.complex64", "builtin.complex64"),
	index.NewReferenceNames("builtin.complex128", "builtin.complex128"),
	index.NewReferenceNames("builtin.string", "builtin.string"),
	index.NewReferenceNames("builtin.int", "builtin.int"),
	index.NewReferenceNames("builtin.uint", "builtin.uint"),
	index.NewReferenceNames("builtin.uintptr", "builtin.uintptr"),
	index.NewReferenceNames("builtin.byte", "builtin.uint8"),
	index.NewReferenceNames("builtin.rune", "builtin.int32"),
	index.NewReferenceNames("builtin.comparable", "builtin.comparable"),
	index.NewReferenceNames("builtin.Type", "builtin.int"),
	index.NewReferenceNames("builtin.Type1", "builtin.int"),
	index.NewReferenceNames("builtin.IntegerType", "builtin.int"),
	index.NewReferenceNames("builtin.FloatType", "builtin.float32"),
	index.NewReferenceNames("builtin.ComplexType", "builtin.complex64"),
	// DATA->TYPE: nil has type Type
	index.NewReferenceNames("builtin.nil", "builtin.Type"),
}

func sortedSymbols(symbols []index.SymbolDecl) []*index.SymbolDecl {
	sorted := make([]*index.SymbolDecl, len(symbols))
	for i := range symbols {
		sorted[i] = &symbols[i]
	}
	slices.SortFunc(sorted, func(a, b *index.SymbolDecl) int {
		return a.Compare(b)
	})
	return sorted
}

var _ = Describe("PackageParser", func() {
	var idx index.Index
	BeforeEach(func() {
		var err error
		idx, err = index.NewProtoIndex(
			index.WithProject("test_project"),
		)
		Expect(err).ToNot(HaveOccurred(), "Failed to create index")
	})
	AfterEach(func() {
		err := idx.Close()
		Expect(err).ToNot(HaveOccurred(), "Failed to close index")
	})
	DescribeTable("Parsing a package", func(testDir string, expectedSymbols []*index.SymbolDecl, expectedReferences []*index.ReferenceNames) {
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred(), "Failed to get current working directory")
		pkgPath := fmt.Sprintf("%s/test/src/%s", cwd, testDir)
		Expect(pkgPath).ToNot(BeEmpty(), "Package path should not be empty")

		pkgParser := parser.NewParser(pkgPath, idx)
		defer pkgParser.Close()

		err = pkgParser.Load()
		Expect(err).ToNot(HaveOccurred(), "Failed to load parser")

		err = pkgParser.AddPackages()
		Expect(err).ToNot(HaveOccurred(), "Failed to add packages to parser")

		err = idx.ResolveReferences()
		Expect(err).ToNot(HaveOccurred(), "Failed to resolve references")

		err = idx.Wait()
		Expect(err).ToNot(HaveOccurred(), "Failed to wait for index to finish")
		log.Println("Indexing done")

		symbols, err := idx.GetAllSymbols()
		log.Println(symbols)
		Expect(err).ToNot(HaveOccurred(), "Failed to get symbols from index")
		matchers := []types.GomegaMatcher{}
		for _, ref := range expectedSymbols {
			matchers = append(matchers, &index.SymbolMatcher{Expected: ref})
		}
		Expect(symbols).To(ConsistOf(matchers), "Symbols in index do not match expected symbols")

		references, err := idx.GetAllReferencesNames()
		Expect(err).ToNot(HaveOccurred(), "Failed to get references from index")
		matchers = []types.GomegaMatcher{}
		for _, ref := range expectedReferences {
			matchers = append(matchers, &index.ReferenceMatcher{Expected: ref})
		}
		Expect(references).To(ConsistOf(matchers), "References in index do not match expected references")
	},
		Entry("is trivial file", "mock1",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "mock1.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("mock1.MockFunction", "builtin.print"),
			),
		),
		Entry("is primitive type", "primitive_types",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "primitive_types.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("primitive_types.MockFunction", "builtin.print"),
				index.NewReferenceNames("primitive_types.MockFunction", "builtin.make"),
				index.NewReferenceNames("primitive_types.MockFunction", "builtin.append"),
			),
		),
		Entry("is generic instantiation", "generic_instantiation/app",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "generic_instantiation/lib.Box[T]).Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "generic_instantiation/app.Doer).Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "generic_instantiation/app.Call", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "generic_instantiation/app.Doer", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "generic_instantiation/lib.Box", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("generic_instantiation/app.Call", "generic_instantiation/app.Doer).Foo"),
				index.NewReferenceNames("generic_instantiation/app.Doer).Foo", "generic_instantiation/lib.Box[T]).Foo"),
				// Module import: app imports lib
				index.NewReferenceNames("generic_instantiation/app", "generic_instantiation/lib"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("generic_instantiation/app.Doer", "generic_instantiation/app.Doer).Foo"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("generic_instantiation/lib.Box", "generic_instantiation/lib.Box[T]).Foo"),
			),
		),
		Entry("is unsafe", "unsafe",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "unsafe.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("unsafe.MockFunction", "builtin.print"),
				index.NewReferenceNames("unsafe.MockFunction", "builtin.print"),
				index.NewReferenceNames("unsafe.MockFunction", "builtin.print"),
				index.NewReferenceNames("unsafe.MockFunction", "builtin.print"),
				index.NewReferenceNames("unsafe.MockFunction", "builtin.print"),
				index.NewReferenceNames("unsafe.MockFunction", "builtin.print"),
				index.NewReferenceNames("unsafe.MockFunction", "builtin.print"),
				index.NewReferenceNames("unsafe.MockFunction", "builtin.print"),
				// Module import: test unsafe package imports standard unsafe package
				index.NewReferenceNames("test/src/unsafe", "unsafe"),
			),
		),
		Entry("has duplicate interface refs", "duplicate_refs",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "duplicate_refs.DuplicateRefs", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "duplicate_refs.PathError).Error", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "duplicate_refs.PathError", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("(builtin.error).Error", "PathError).Error"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("duplicate_refs.PathError", "duplicate_refs.PathError).Error"),
			),
		),
		Entry("is an interface call", "interface_call",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "interface_call.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_call.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call.CallInterface", "interface_call.Mock).MockFunction"),
				index.NewReferenceNames("interface_call.Mock).MockFunction", "interface_call.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call.MockImpl).MockFunction", "builtin.print"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("interface_call.Mock", "interface_call.Mock).MockFunction"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("interface_call.MockImpl", "interface_call.MockImpl).MockFunction"),
			),
		),
		Entry("is an interface call (2)", "interface_call2",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call2.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call2.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call2.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "interface_call2.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_call2.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call2.CallInterface", "interface_call2.Mock).MockFunction"),
				index.NewReferenceNames("interface_call2.Mock).MockFunction", "interface_call2.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call2.MockImpl).MockFunction", "builtin.print"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("interface_call2.Mock", "interface_call2.Mock).MockFunction"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("interface_call2.MockImpl", "interface_call2.MockImpl).MockFunction"),
			),
		),
		Entry("is an interface call (3)", "interface_call3",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call3.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call3.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call3.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "interface_call3.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_call3.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call3.CallInterface", "interface_call3.Mock).MockFunction"),
				index.NewReferenceNames("interface_call3.Mock).MockFunction", "interface_call3.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call3.MockImpl).MockFunction", "builtin.print"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("interface_call3.Mock", "interface_call3.Mock).MockFunction"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("interface_call3.MockImpl", "interface_call3.MockImpl).MockFunction"),
			),
		),
		Entry("is an interface call (4)", "interface_call4",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call4.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call4.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call4.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call4.foo", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "interface_call4.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_call4.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call4.CallInterface", "interface_call4.Mock).MockFunction"),
				index.NewReferenceNames("interface_call4.CallInterface", "interface_call4.foo"),
				index.NewReferenceNames("interface_call4.Mock).MockFunction", "interface_call4.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call4.MockImpl).MockFunction", "builtin.print"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("interface_call4.Mock", "interface_call4.Mock).MockFunction"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("interface_call4.MockImpl", "interface_call4.MockImpl).MockFunction"),
			),
		),
		Entry("is an interface call via closure return", "interface_call5",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call5.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call5.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call5.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call5.CallInterface:<anon219>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "interface_call5.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_call5.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call5.CallInterface", "interface_call5.Mock).MockFunction"),
				index.NewReferenceNames("interface_call5.CallInterface", "interface_call5.CallInterface:<anon219>"),
				index.NewReferenceNames("interface_call5.Mock).MockFunction", "interface_call5.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call5.MockImpl).MockFunction", "builtin.print"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("interface_call5.Mock", "interface_call5.Mock).MockFunction"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("interface_call5.MockImpl", "interface_call5.MockImpl).MockFunction"),
			),
		),
		Entry("is an interface call via nested closures", "interface_call6",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call6.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.CallInterface:<anon219>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.CallInterface:<anon219>:<anon249>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.CallInterface:<anon219>:<anon249>:<anon280>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.CallInterface:<anon219>:<anon249>:<anon280>:<anon312>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "interface_call6.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call6.CallInterface", "interface_call6.Mock).MockFunction"),
				index.NewReferenceNames("interface_call6.CallInterface", "interface_call6.CallInterface:<anon219>"),
				index.NewReferenceNames("interface_call6.Mock).MockFunction", "interface_call6.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call6.MockImpl).MockFunction", "builtin.print"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("interface_call6.Mock", "interface_call6.Mock).MockFunction"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("interface_call6.MockImpl", "interface_call6.MockImpl).MockFunction"),
			),
		),
		Entry("return various values", "return_values",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_values.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_values.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_values.wrapErrors).Error", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_values.wrapErrors).Unwrap", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_values.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "return_values.wrapErrors", index.ScopeLocal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "return_values.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_values.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_values.MockFunction", "return_values.Foo"),
				index.NewReferenceNames("error).Error", "return_values.wrapErrors).Error"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("return_values.Mock", "return_values.Mock).MockFunction"),
				// TYPE->FUNCTION: receiver methods
				index.NewReferenceNames("return_values.wrapErrors", "return_values.wrapErrors).Error"),
				index.NewReferenceNames("return_values.wrapErrors", "return_values.wrapErrors).Unwrap"),
				// STRUCT->BUILTIN: wrapErrors has field msg string
				index.NewReferenceNames("return_values.wrapErrors", "builtin.string"),
			),
		),
		Entry("return different values", "return_values2",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_values2.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_values2.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_values2.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "return_values2.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_values2.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_values2.MockFunction", "return_values2.Foo"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("return_values2.Mock", "return_values2.Mock).MockFunction"),
			),
		),
		Entry("returns type alias", "return_alias",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_alias.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_alias.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_alias.wrapErrors).Error", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_alias.wrapErrors).Unwrap", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_alias.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "return_alias.wrapErrors", index.ScopeLocal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "return_alias.wrapErrorsAlias", index.ScopeLocal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "return_alias.wrapErrorsAliasAlias", index.ScopeLocal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "return_alias.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_alias.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_alias.MockFunction", "return_alias.Foo"),
				index.NewReferenceNames("error).Error", "return_alias.wrapErrors).Error"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("return_alias.Mock", "return_alias.Mock).MockFunction"),
				// TYPE->FUNCTION: receiver methods
				index.NewReferenceNames("return_alias.wrapErrors", "return_alias.wrapErrors).Error"),
				index.NewReferenceNames("return_alias.wrapErrors", "return_alias.wrapErrors).Unwrap"),
				// TYPE->TYPE: alias references
				index.NewReferenceNames("return_alias.wrapErrorsAlias", "return_alias.wrapErrors"),
				index.NewReferenceNames("return_alias.wrapErrorsAliasAlias", "return_alias.wrapErrorsAlias"),
			),
		),
		Entry("return another type alias", "return_alias2",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_alias2.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_alias2.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_alias2.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_alias2.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "return_alias2.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "return_alias2.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_alias2.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_alias2.MockFunction", "return_alias2.Foo"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("return_alias2.Mock", "return_alias2.Mock).MockFunction"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("return_alias2.MockImpl", "return_alias2.MockImpl).MockFunction"),
			),
		),
		Entry("return interface", "return_interface",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_interface.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_interface.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_interface.Foo:<anon298>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "return_interface.exporter", index.ScopeLocal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "return_interface.MessageInfo", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "return_interface.AdditionalPropertiesItem", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_interface.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_interface.MockFunction", "return_interface.Foo"),
				// TYPE->TYPE: MessageInfo.Exporter field references exporter type
				index.NewReferenceNames("return_interface.MessageInfo", "return_interface.exporter"),
				// STRUCT->BUILTIN: AdditionalPropertiesItem has fields with int32, string, byte, int
				index.NewReferenceNames("return_interface.AdditionalPropertiesItem", "builtin.int32"),
				index.NewReferenceNames("return_interface.AdditionalPropertiesItem", "builtin.string"),
				index.NewReferenceNames("return_interface.AdditionalPropertiesItem", "builtin.byte"),
				index.NewReferenceNames("return_interface.AdditionalPropertiesItem", "builtin.int"),
			),
		),
		Entry("return type params", "type_params",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_params.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_params.isNaN", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_params.cmpLess", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_params.insertionSortOrdered", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "type_params.Integer", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_params.Float", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_params.Ordered", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_params.insertionSortOrdered", "type_params.cmpLess"),
				index.NewReferenceNames("type_params.cmpLess", "type_params.isNaN"),
				index.NewReferenceNames("type_params.cmpLess", "type_params.isNaN"),
				index.NewReferenceNames("type_params.MockFunction", "builtin.len"),
				index.NewReferenceNames("type_params.MockFunction", "type_params.insertionSortOrdered"),
				index.NewReferenceNames("type_params.MockFunction", "builtin.len"),
				index.NewReferenceNames("type_params.MockFunction", "type_params.insertionSortOrdered"),
				index.NewReferenceNames("type_params.MockFunction", "builtin.len"),
				index.NewReferenceNames("type_params.MockFunction", "type_params.insertionSortOrdered"),
				// TYPE->TYPE: Ordered embeds Integer and Float (union type)
				index.NewReferenceNames("type_params.Ordered", "type_params.Integer"),
				index.NewReferenceNames("type_params.Ordered", "type_params.Float"),
			),
		),
		Entry("returns type assert", "return_type_assert",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_type_assert.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_type_assert.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_type_assert.Token).GetText", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "return_type_assert.Token", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_type_assert.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_type_assert.MockFunction", "return_type_assert.Foo"),
				index.NewReferenceNames("return_type_assert.Foo", "return_type_assert.Token).GetText"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("return_type_assert.Token", "return_type_assert.Token).GetText"),
			),
		),
		Entry("returns index expression with interface map", "return_index_expression",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_index_expression.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_index_expression.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_index_expression.Token).GetText", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_index_expression.TokenImpl).GetText", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "return_index_expression.Token", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "return_index_expression.TokenImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_index_expression.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_index_expression.MockFunction", "return_index_expression.Foo"),
				index.NewReferenceNames("return_index_expression.Token).GetText", "return_index_expression.TokenImpl).GetText"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("return_index_expression.Token", "return_index_expression.Token).GetText"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("return_index_expression.TokenImpl", "return_index_expression.TokenImpl).GetText"),
				// STRUCT->BUILTIN: TokenImpl has field text string
				index.NewReferenceNames("return_index_expression.TokenImpl", "builtin.string"),
			),
		),
		Entry("returns grouped variables", "return_grouped",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_grouped.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_grouped.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_grouped.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "return_grouped.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_grouped.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_grouped.MockFunction", "return_grouped.Foo"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("return_grouped.Mock", "return_grouped.Mock).MockFunction"),
			),
		),
		Entry("has a nested function", "return_nested",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_nested.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_nested.sysDialer).dialUnix", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "sysDialer).dialUnix:<anon157>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "return_nested.sysDialer", index.ScopeLocal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_nested.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_nested.MockFunction", "return_nested.sysDialer).dialUnix"),
				index.NewReferenceNames("sysDialer).dialUnix", "sysDialer).dialUnix:<anon157>"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("return_nested.sysDialer", "return_nested.sysDialer).dialUnix"),
				// STRUCT->BUILTIN: sysDialer has field Control func(...string) error
				index.NewReferenceNames("return_nested.sysDialer", "builtin.string"),
				index.NewReferenceNames("return_nested.sysDialer", "builtin.error"),
			),
		),
		Entry("returns pointer from a function call", "return_pointer",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_pointer.transport).SupportsUnixFDs", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_pointer.Conn).SupportsUnixFDs", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_pointer.newNonceTcpTransport", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_pointer.NewConn", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_pointer.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "return_pointer.transport", index.ScopeLocal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "return_pointer.Conn", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_pointer.transport).SupportsUnixFDs", "return_pointer.Conn).SupportsUnixFDs"),
				index.NewReferenceNames("return_pointer.newNonceTcpTransport", "NewConn"),
				index.NewReferenceNames("return_pointer.MockFunction", "return_pointer.newNonceTcpTransport"),
				index.NewReferenceNames("return_pointer.MockFunction", "builtin.print"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("return_pointer.transport", "return_pointer.transport).SupportsUnixFDs"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("return_pointer.Conn", "return_pointer.Conn).SupportsUnixFDs"),
			),
		),
		Entry("assigns anonymous interfaces", "assign_interface",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "assign_interface.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "assign_interface.Types).FindExtensionByName", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "(interface).FindExtensionByName", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "(interface).FindExtensionByName", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "assign_interface.Types", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "assign_interface.UnmarshalInput", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "assign_interface.UnmarshalInput2", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				// DATA symbols
				index.NewSymbol(3, 3, "assign_interface.GlobalTypes", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("(interface).FindExtensionByName", "assign_interface.Types).FindExtensionByName"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("assign_interface.Types", "assign_interface.Types).FindExtensionByName"),
				// STRUCT->BUILTIN: UnmarshalInput has field Depth int
				index.NewReferenceNames("assign_interface.UnmarshalInput", "builtin.int"),
				// STRUCT->BUILTIN: UnmarshalInput2 has field Depth int
				index.NewReferenceNames("assign_interface.UnmarshalInput2", "builtin.int"),
				// DATA->TYPE: GlobalTypes has type *Types
				index.NewReferenceNames("assign_interface.GlobalTypes", "assign_interface.Types"),
				// FUNC->DATA: MockFunction uses GlobalTypes
				index.NewReferenceNames("assign_interface.MockFunction", "assign_interface.GlobalTypes"),
			),
		),
		Entry("assign a func to any", "assign_func",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "assign_func.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "assign_func.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "assign_func.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "MockImpl).MockFunction:<anon191>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "assign_func.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "assign_func.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("assign_func.CallInterface", "assign_func.MockImpl).MockFunction"),
				index.NewReferenceNames("assign_func.CallInterface", "builtin.panic"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("assign_func.Mock", "assign_func.Mock).MockFunction"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("assign_func.MockImpl", "assign_func.MockImpl).MockFunction"),
				// STRUCT->BUILTIN: MockImpl has field ShuffleAddressListForTesting any
				index.NewReferenceNames("assign_func.MockImpl", "builtin.any"),
			),
		),
		Entry("has unary expression", "assign_unary",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "assign_unary.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "assign_unary.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "assign_unary.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "assign_unary.Mock", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "assign_unary.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("assign_unary.MockImpl).MockFunction", "builtin.make"),
				index.NewReferenceNames("assign_unary.MockImpl).MockFunction", "assign_unary.Mock).MockFunction"),
				index.NewReferenceNames("assign_unary.CallInterface", "assign_unary.MockImpl).MockFunction"),
				index.NewReferenceNames("assign_unary.CallInterface", "builtin.panic"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("assign_unary.Mock", "assign_unary.Mock).MockFunction"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("assign_unary.MockImpl", "assign_unary.MockImpl).MockFunction"),
				// STRUCT->BUILTIN: MockImpl has field ShuffleAddressListForTesting any
				index.NewReferenceNames("assign_unary.MockImpl", "builtin.any"),
			),
		),
		Entry("is an indirect interface call", "interface_indirect1",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_indirect1.Mock2).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_indirect1.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_indirect1.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_indirect1.CallInterface:<anon280>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "interface_indirect1.Mock1", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_indirect1.Mock2", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_indirect1.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_indirect1.Mock2).MockFunction", "interface_indirect1.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_indirect1.MockImpl).MockFunction", "builtin.print"),
				index.NewReferenceNames("interface_indirect1.CallInterface", "builtin.print"),
				index.NewReferenceNames("interface_indirect1.CallInterface", "interface_indirect1.CallInterface:<anon280>"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("interface_indirect1.Mock2", "interface_indirect1.Mock2).MockFunction"),
				// TYPE->TYPE: Mock2 embeds Mock1
				index.NewReferenceNames("interface_indirect1.Mock2", "interface_indirect1.Mock1"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("interface_indirect1.MockImpl", "interface_indirect1.MockImpl).MockFunction"),
			),
		),
		Entry("is an interface mapping through func parameter", "interface_call7",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call7.Mock2).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call7.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call7.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call7.CallInterface:<anon254>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "interface_call7.Mock1", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_call7.Mock2", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_call7.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call7.Mock2).MockFunction", "interface_call7.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call7.MockImpl).MockFunction", "builtin.print"),
				index.NewReferenceNames("interface_call7.CallInterface", "builtin.print"),
				index.NewReferenceNames("interface_call7.CallInterface", "interface_call7.CallInterface:<anon254>"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("interface_call7.Mock2", "interface_call7.Mock2).MockFunction"),
				// TYPE->TYPE: Mock2 embeds Mock1
				index.NewReferenceNames("interface_call7.Mock2", "interface_call7.Mock1"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("interface_call7.MockImpl", "interface_call7.MockImpl).MockFunction"),
			),
		),
		Entry("is an interface mapping through func parameter (2)", "interface_call8",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call8.Mock2).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call8.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call8.MockImpl).CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "MockImpl).CallInterface:<anon269>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "interface_call8.Mock1", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_call8.Mock2", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "interface_call8.MockImpl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call8.Mock2).MockFunction", "interface_call8.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call8.MockImpl).MockFunction", "builtin.print"),
				index.NewReferenceNames("interface_call8.MockImpl).CallInterface", "builtin.print"),
				index.NewReferenceNames("MockImpl).CallInterface", "MockImpl).CallInterface:<anon269>"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("interface_call8.Mock2", "interface_call8.Mock2).MockFunction"),
				// TYPE->TYPE: Mock2 embeds Mock1
				index.NewReferenceNames("interface_call8.Mock2", "interface_call8.Mock1"),
				// TYPE->FUNCTION: receiver methods
				index.NewReferenceNames("interface_call8.MockImpl", "interface_call8.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call8.MockImpl", "interface_call8.MockImpl).CallInterface"),
			),
		),
		Entry("checks correctness of callExprParser", "call_expr_parser",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "call_expr_parser.Writer).Write", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "call_expr_parser.Fprintln", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "call_expr_parser.File).Write", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "call_expr_parser.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "call_expr_parser.Writer", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "call_expr_parser.File", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				// DATA symbols
				index.NewSymbol(3, 3, "call_expr_parser.Stderr", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("call_expr_parser.Writer).Write", "call_expr_parser.File).Write"),
				index.NewReferenceNames("call_expr_parser.Fprintln", "call_expr_parser.Writer).Write"),
				index.NewReferenceNames("call_expr_parser.CallInterface", "builtin.print"),
				index.NewReferenceNames("call_expr_parser.CallInterface", "call_expr_parser.Fprintln"),
				// FUNC->DATA: CallInterface uses Stderr
				index.NewReferenceNames("call_expr_parser.CallInterface", "call_expr_parser.Stderr"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("call_expr_parser.Writer", "call_expr_parser.Writer).Write"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("call_expr_parser.File", "call_expr_parser.File).Write"),
			),
		),
		Entry("checks passing func as parameter", "func_as_param",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "func_as_param.FooType).Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "func_as_param.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "func_as_param.CallInterface:<anon307>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "func_as_param.ResponseWriter", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "func_as_param.Request", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "func_as_param.HandlerFunc", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "func_as_param.FooType", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("func_as_param.CallInterface", "builtin.print"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("func_as_param.FooType", "func_as_param.FooType).Foo"),
				// TYPE->TYPE: HandlerFunc func type references ResponseWriter and Request
				index.NewReferenceNames("func_as_param.HandlerFunc", "func_as_param.ResponseWriter"),
				index.NewReferenceNames("func_as_param.HandlerFunc", "func_as_param.Request"),
			),
		),
		Entry("checks generic functions", "generic_functions",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "generic_functions.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "generic_functions.NewPodSetReducer", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "generic_functions.PodSetReducer[R]).Search", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "generic_functions.Foo:<anon300>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "generic_functions.PodSetReducer", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("Foo", "generic_functions.NewPodSetReducer"),
				index.NewReferenceNames("Foo", "PodSetReducer[R]).Search"),
				index.NewReferenceNames("Foo", "builtin.print"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("generic_functions.PodSetReducer", "generic_functions.PodSetReducer[R]).Search"),
			),
		),
		Entry("checks parsing of channels", "channel",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "channel.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("Foo", "builtin.make"),
				index.NewReferenceNames("Foo", "builtin.println"),
				index.NewReferenceNames("Foo", "builtin.println"),
			),
		),
		Entry("checks anonymous interface", "anonymous_interface",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "anonymous_interface.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "anonymous_interface.ClearUnknown", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "(interface).Has", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "fieldNum).Has", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "anonymous_interface.fieldNum", index.ScopeLocal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("ClearUnknown", "(interface).Has"),
				index.NewReferenceNames("ClearUnknown", "builtin.print"),
				index.NewReferenceNames("(interface).Has", "fieldNum).Has"),
				index.NewReferenceNames("MockFunction", "ClearUnknown"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("anonymous_interface.fieldNum", "fieldNum).Has"),
			),
		),
		// This test verifies:
		// 1. Function call references (UseProvider -> Hello, UseBuiltinAndUnsafe -> make/len)
		// 2. Module import references (consumer -> provider, consumer -> unsafe)
		Entry("has nested anonymous functions", "nested_func",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "nested_func.OuterFunc", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "nested_func.OuterFunc:<anon50>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "nested_func.OuterFunc:<anon116>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("nested_func.OuterFunc", "nested_func.OuterFunc:<anon50>"),
				index.NewReferenceNames("nested_func.OuterFunc:<anon50>", "builtin.print"),
				index.NewReferenceNames("nested_func.OuterFunc:<anon116>", "builtin.print"),
			),
		),
		Entry("has package-level anonymous function", "nested_func_pkg_level",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "test/src/nested_func_pkg_level:<anon45>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				// DATA symbol for the package-level var holding the func
				index.NewSymbol(3, 3, "nested_func_pkg_level.PkgFunc", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("test/src/nested_func_pkg_level:<anon45>", "builtin.print"),
			),
		),
		Entry("has local type declaration inside closure", "local_type_in_closure",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "local_type_in_closure.Outer", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "local_type_in_closure.Outer:<anon56>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "canceler).Cancel", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				// TYPE symbols
				index.NewSymbol(3, 3, "Outer:<anon56>.canceler", index.ScopeLocal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("local_type_in_closure.Outer", "local_type_in_closure.Outer:<anon56>"),
				index.NewReferenceNames("Outer:<anon56>", "canceler).Cancel"),
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("Outer:<anon56>.canceler", "canceler).Cancel"),
			),
		),
		Entry("has inline anonymous interface in type assertion", "inline_type_assert",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "inline_type_assert.Outer", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "(interface).GetName", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("inline_type_assert.Outer", "(interface).GetName"),
				index.NewReferenceNames("inline_type_assert.Outer", "builtin.print"),
			),
		),
		Entry("has anonymous function passed as callback", "anon_func_callback",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "anon_func_callback.apply", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "anon_func_callback.Outer", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "anon_func_callback.Outer:<anon113>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("anon_func_callback.Outer", "anon_func_callback.apply"),
				index.NewReferenceNames("anon_func_callback.Outer", "anon_func_callback.Outer:<anon113>"),
			),
		),
		Entry("has multiple anonymous functions in one assignment", "multi_anon_assign",
			append(builtinSymbols,
				index.NewSymbol(3, 3, "multi_anon_assign.Outer", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "multi_anon_assign.Outer:<anon61>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "multi_anon_assign.Outer:<anon98>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(builtinReferences,
				index.NewReferenceNames("multi_anon_assign.Outer", "multi_anon_assign.Outer:<anon61>"),
				index.NewReferenceNames("multi_anon_assign.Outer", "multi_anon_assign.Outer:<anon98>"),
			),
		),
		Entry("has anonymous function reassignment", "anon_func_reassign",
			append(builtinSymbols,
				index.NewSymbol(3, 3, "anon_func_reassign.apply", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "anon_func_reassign.Outer", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "anon_func_reassign.Outer:<anon116>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "anon_func_reassign.Outer:<anon191>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(builtinReferences,
				index.NewReferenceNames("anon_func_reassign.Outer", "anon_func_reassign.apply"),
				index.NewReferenceNames("anon_func_reassign.Outer", "anon_func_reassign.apply"),
				index.NewReferenceNames("anon_func_reassign.Outer", "anon_func_reassign.Outer:<anon116>"),
				index.NewReferenceNames("anon_func_reassign.Outer", "anon_func_reassign.Outer:<anon191>"),
			),
		),
		Entry("has anonymous function returned from named function", "anon_func_returned",
			append(builtinSymbols,
				index.NewSymbol(3, 3, "anon_func_returned.Maker", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "anon_func_returned.Maker:<anon65>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "anon_func_returned.Caller", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(builtinReferences,
				// Caller calls Maker — fn := Maker() is a function call
				index.NewReferenceNames("anon_func_returned.Caller", "anon_func_returned.Maker"),
				// No reference from Caller to the inner FuncLit — fn is assigned from a call, not a FuncLit
			),
		),
		Entry("has anonymous function captured in defer", "anon_func_defer_capture",
			append(builtinSymbols,
				index.NewSymbol(3, 3, "anon_func_defer_capture.Outer", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "anon_func_defer_capture.Outer:<anon60>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "anon_func_defer_capture.Outer:<anon99>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(builtinReferences,
				index.NewReferenceNames("Outer:<anon60>", "builtin.print"),
				index.NewReferenceNames("Outer", "builtin.print"),
				// cleanup() called inside defer body — reference from defer anon to cleanup anon
				// (cleanup is only referenced inside the defer body, not directly in Outer's scope)
				index.NewReferenceNames("Outer:<anon99>", "Outer:<anon60>"),
			),
		),
		Entry("has struct with primitive fields", "type_struct_basic",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_struct_basic.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_struct_basic.Foo", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_struct_basic.MockFunction", "builtin.print"),
				// STRUCT->BUILTIN: Foo has fields X int, Y string
				index.NewReferenceNames("type_struct_basic.Foo", "builtin.int"),
				index.NewReferenceNames("type_struct_basic.Foo", "builtin.string"),
			),
		),
		Entry("has struct referencing named types", "type_struct_refs",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_struct_refs.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_struct_refs.Bar", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_struct_refs.Baz", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_struct_refs.Foo", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_struct_refs.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_struct_refs.Foo", "type_struct_refs.Bar"),
				index.NewReferenceNames("type_struct_refs.Foo", "type_struct_refs.Baz"),
			),
		),
		Entry("has struct with embedded fields", "type_embedded",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_embedded.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_embedded.Base", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_embedded.Extended", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_embedded.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_embedded.Extended", "type_embedded.Base"),
				// STRUCT->BUILTIN: Extended has field X int
				index.NewReferenceNames("type_embedded.Extended", "builtin.int"),
			),
		),
		Entry("has interface with embedded interfaces", "type_interface_embed",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_interface_embed.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_interface_embed.Reader).Read", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_interface_embed.Writer).Write", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_interface_embed.Reader", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_interface_embed.Writer", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_interface_embed.ReadWriter", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_interface_embed.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_interface_embed.Reader", "type_interface_embed.Reader).Read"),
				index.NewReferenceNames("type_interface_embed.Writer", "type_interface_embed.Writer).Write"),
				index.NewReferenceNames("type_interface_embed.ReadWriter", "type_interface_embed.Reader"),
				index.NewReferenceNames("type_interface_embed.ReadWriter", "type_interface_embed.Writer"),
			),
		),
		Entry("has type alias and type definition", "type_alias_def",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_alias_def.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_alias_def.Original", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_alias_def.Alias", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_alias_def.NewType", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_alias_def.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_alias_def.Alias", "type_alias_def.Original"),
				index.NewReferenceNames("type_alias_def.NewType", "type_alias_def.Original"),
			),
		),
		Entry("has function type referencing named types", "type_func_type",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_func_type.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_func_type.Request", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_func_type.Response", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_func_type.Handler", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_func_type.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_func_type.Handler", "type_func_type.Request"),
				index.NewReferenceNames("type_func_type.Handler", "type_func_type.Response"),
			),
		),
		Entry("has collection types", "type_collections",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_collections.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_collections.Entry", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_collections.Cache", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_collections.Entries", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_collections.EventChan", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_collections.EntryPtr", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_collections.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_collections.Cache", "type_collections.Entry"),
				index.NewReferenceNames("type_collections.Entries", "type_collections.Entry"),
				index.NewReferenceNames("type_collections.EventChan", "type_collections.Entry"),
				index.NewReferenceNames("type_collections.EntryPtr", "type_collections.Entry"),
			),
		),
		Entry("has generic type with constraint", "type_generic",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_generic.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_generic.Stringer).String", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_generic.Stringer", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_generic.Collection", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_generic.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_generic.Stringer", "type_generic.Stringer).String"),
				index.NewReferenceNames("type_generic.Collection", "type_generic.Stringer"),
			),
		),
		Entry("has self-referential struct", "type_self_ref",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_self_ref.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_self_ref.Node", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_self_ref.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_self_ref.Node", "type_self_ref.Node"),
				// STRUCT->BUILTIN: Node has field Value int
				index.NewReferenceNames("type_self_ref.Node", "builtin.int"),
			),
		),
		Entry("has external methods on struct", "type_method_receiver",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_method_receiver.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_method_receiver.Foo).Bar", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_method_receiver.Foo).Baz", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_method_receiver.Foo", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_method_receiver.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_method_receiver.Foo).Bar", "builtin.print"),
				index.NewReferenceNames("type_method_receiver.Foo).Baz", "builtin.print"),
				index.NewReferenceNames("type_method_receiver.Foo", "type_method_receiver.Foo).Bar"),
				index.NewReferenceNames("type_method_receiver.Foo", "type_method_receiver.Foo).Baz"),
			),
		),
		Entry("has cross-package type reference", "type_cross_pkg/app",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_cross_pkg/app.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_cross_pkg/app.Extended", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_cross_pkg/types.Base", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_cross_pkg/app.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_cross_pkg/app.Extended", "type_cross_pkg/types.Base"),
				index.NewReferenceNames("type_cross_pkg/app", "type_cross_pkg/types"),
			),
		),
		Entry("has complex nested type expressions", "type_nested_expr",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_nested_expr.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_nested_expr.Key", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_nested_expr.Value", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_nested_expr.Complex", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_nested_expr.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_nested_expr.Complex", "type_nested_expr.Key"),
				index.NewReferenceNames("type_nested_expr.Complex", "type_nested_expr.Value"),
			),
		),
		Entry("has empty struct and empty interface", "type_empty",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_empty.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_empty.EmptyStruct", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_empty.EmptyInterface", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_empty.MockFunction", "builtin.print"),
			),
		),
		Entry("has local type inside function", "type_local",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_local.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_local.Outer", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_local.Foo", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_local.Outer.Foo", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_local.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_local.Outer", "builtin.print"),
				// STRUCT->BUILTIN: local Foo inside Outer has field X int
				index.NewReferenceNames("type_local.Outer.Foo", "builtin.int"),
			),
		),
		Entry("has struct embedding interface", "type_struct_embed_interface",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_struct_embed_interface.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_struct_embed_interface.Writer).Write", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_struct_embed_interface.Writer", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_struct_embed_interface.LogWriter", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_struct_embed_interface.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_struct_embed_interface.Writer", "type_struct_embed_interface.Writer).Write"),
				index.NewReferenceNames("type_struct_embed_interface.LogWriter", "type_struct_embed_interface.Writer"),
				// STRUCT->BUILTIN: LogWriter has field prefix string
				index.NewReferenceNames("type_struct_embed_interface.LogWriter", "builtin.string"),
			),
		),
		Entry("has struct field with anonymous interface", "type_func_field",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_func_field.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "(interface).Bar", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_func_field.Foo", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_func_field.MockFunction", "builtin.print"),
			),
		),
		Entry("has alias chain and reference through alias", "type_chain",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_chain.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_chain.Real", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_chain.Alias", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_chain.Consumer", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_chain.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_chain.Alias", "type_chain.Real"),
				index.NewReferenceNames("type_chain.Consumer", "type_chain.Alias"),
			),
		),
		Entry("has type constraint with union", "type_constraint_union",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_constraint_union.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_constraint_union.MyInt", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_constraint_union.Numeric", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "type_constraint_union.Container", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("type_constraint_union.MockFunction", "builtin.print"),
				index.NewReferenceNames("type_constraint_union.Numeric", "type_constraint_union.MyInt"),
				index.NewReferenceNames("type_constraint_union.Container", "type_constraint_union.Numeric"),
			),
		),
		Entry("has simple var declarations", "var_simple",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "var_simple.ExportedInt", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "var_simple.unexportedStr", index.ScopeLocal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "var_simple.WithInit", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("var_simple.ExportedInt", "builtin.int"),
				index.NewReferenceNames("var_simple.unexportedStr", "builtin.string"),
			),
		),
		Entry("has block var declarations", "var_block",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "var_block.A", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "var_block.B", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "var_block.C", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("var_block.A", "builtin.int"),
				index.NewReferenceNames("var_block.B", "builtin.string"),
			),
		),
		Entry("has typed var declarations", "var_typed",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "var_typed.Config", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "var_typed.GlobalConfig", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "var_typed.GlobalPtr", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("var_typed.GlobalConfig", "var_typed.Config"),
				index.NewReferenceNames("var_typed.GlobalPtr", "var_typed.Config"),
				// struct field builtin type ref
				index.NewReferenceNames("var_typed.Config", "builtin.string"),
			),
		),
		Entry("has var usage in functions", "var_usage",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "var_usage.Counter", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "var_usage.ReadCounter", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "var_usage.WriteCounter", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "var_usage.IncrCounter", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("var_usage.ReadCounter", "var_usage.Counter"),
				index.NewReferenceNames("var_usage.WriteCounter", "var_usage.Counter"),
				index.NewReferenceNames("var_usage.IncrCounter", "var_usage.Counter"),
				index.NewReferenceNames("var_usage.IncrCounter", "var_usage.Counter"),
				index.NewReferenceNames("var_usage.Counter", "builtin.int"),
			),
		),
		Entry("has var passed as func arg", "var_func_arg",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "var_func_arg.GlobalVal", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "var_func_arg.consume", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "var_func_arg.Use", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("var_func_arg.Use", "var_func_arg.consume"),
				index.NewReferenceNames("var_func_arg.Use", "var_func_arg.GlobalVal"),
				index.NewReferenceNames("var_func_arg.GlobalVal", "builtin.int"),
			),
		),
		Entry("has init function modifying globals", "var_init",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "var_init.Ready", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "var_init.init", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("var_init.init", "var_init.Ready"),
				index.NewReferenceNames("var_init.Ready", "builtin.bool"),
			),
		),
		Entry("has multiple vars in one spec", "var_multi",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "var_multi.X", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "var_multi.Y", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
			),
			append(
				builtinReferences,
				// Single builtin ref per ValueSpec (X and Y share one spec "var X, Y int")
				index.NewReferenceNames("var_multi.X", "builtin.int"),
			),
		),
		Entry("skips blank identifier var", "var_blank",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "var_blank.Iface).M", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "var_blank.Impl).M", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "var_blank.Iface", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "var_blank.Impl", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
			),
			append(
				builtinReferences,
				// TYPE->FUNCTION: interface method
				index.NewReferenceNames("var_blank.Iface", "var_blank.Iface).M"),
				// TYPE->FUNCTION: receiver method
				index.NewReferenceNames("var_blank.Impl", "var_blank.Impl).M"),
				// Interface satisfaction: Iface.M -> Impl.M
				index.NewReferenceNames("var_blank.Iface).M", "var_blank.Impl).M"),
			),
		),
		Entry("has pointer operations on globals", "var_pointer_ops",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "var_pointer_ops.GlobalInt", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "var_pointer_ops.TakeAddr", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "var_pointer_ops.Deref", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("var_pointer_ops.TakeAddr", "var_pointer_ops.GlobalInt"),
				index.NewReferenceNames("var_pointer_ops.GlobalInt", "builtin.int"),
			),
		),
		Entry("has cross-package var references", "var_cross_pkg/app",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "var_cross_pkg/config.Debug", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "var_cross_pkg/app.Check", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("var_cross_pkg/app.Check", "var_cross_pkg/config.Debug"),
				index.NewReferenceNames("var_cross_pkg/config.Debug", "builtin.bool"),
				// Module import: app imports config
				index.NewReferenceNames("var_cross_pkg/app", "var_cross_pkg/config"),
			),
		),
		Entry("has simple const declarations", "const_simple",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "const_simple.ExportedInt", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "const_simple.unexportedStr", index.ScopeLocal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "const_simple.WithInit", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("const_simple.ExportedInt", "builtin.int"),
				index.NewReferenceNames("const_simple.unexportedStr", "builtin.string"),
			),
		),
		Entry("has block const declarations", "const_block",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "const_block.A", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "const_block.B", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "const_block.C", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("const_block.A", "builtin.int"),
				index.NewReferenceNames("const_block.B", "builtin.string"),
			),
		),
		Entry("has typed const declarations", "const_typed",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "const_typed.Mode", index.ScopeGlobal, index.SymbolTypeType, nil, nil),
				index.NewSymbol(3, 3, "const_typed.GlobalMode", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "const_typed.AltMode", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("const_typed.GlobalMode", "const_typed.Mode"),
			),
		),
		Entry("has const usage in functions", "const_usage",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "const_usage.MaxRetries", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "const_usage.ReadMax", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "const_usage.UseMax", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "const_usage.DoubleCheck", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("const_usage.ReadMax", "const_usage.MaxRetries"),
				index.NewReferenceNames("const_usage.UseMax", "const_usage.MaxRetries"),
				index.NewReferenceNames("const_usage.DoubleCheck", "const_usage.MaxRetries"),
				index.NewReferenceNames("const_usage.DoubleCheck", "const_usage.MaxRetries"),
				index.NewReferenceNames("const_usage.MaxRetries", "builtin.int"),
			),
		),
		Entry("has const passed as func arg", "const_func_arg",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "const_func_arg.GlobalVal", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "const_func_arg.consume", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "const_func_arg.Use", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("const_func_arg.Use", "const_func_arg.consume"),
				index.NewReferenceNames("const_func_arg.Use", "const_func_arg.GlobalVal"),
				index.NewReferenceNames("const_func_arg.GlobalVal", "builtin.int"),
			),
		),
		Entry("has init function using const", "const_init",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "const_init.DefaultName", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "const_init.Name", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "const_init.init", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("const_init.init", "const_init.Name"),
				index.NewReferenceNames("const_init.init", "const_init.DefaultName"),
				index.NewReferenceNames("const_init.DefaultName", "builtin.string"),
				index.NewReferenceNames("const_init.Name", "builtin.string"),
			),
		),
		Entry("has cross-package const references", "const_cross_pkg/app",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "const_cross_pkg/config.Debug", index.ScopeGlobal, index.SymbolTypeData, nil, nil),
				index.NewSymbol(3, 3, "const_cross_pkg/app.Check", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("const_cross_pkg/app.Check", "const_cross_pkg/config.Debug"),
				index.NewReferenceNames("const_cross_pkg/config.Debug", "builtin.bool"),
				// Module import: app imports config
				index.NewReferenceNames("const_cross_pkg/app", "const_cross_pkg/config"),
			),
		),
		Entry("checks module imports", "module_imports/consumer",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "module_imports/consumer.UseProvider", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "module_imports/consumer.UseBuiltinAndUnsafe", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "module_imports/provider.Hello", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				// Function call reference: UseProvider calls Hello
				index.NewReferenceNames("module_imports/consumer.UseProvider", "module_imports/provider.Hello"),
				// UseBuiltinAndUnsafe calls builtin functions (make, len)
				index.NewReferenceNames("module_imports/consumer.UseBuiltinAndUnsafe", "builtin.make"),
				index.NewReferenceNames("module_imports/consumer.UseBuiltinAndUnsafe", "builtin.len"),
				// Module import references: consumer imports provider and unsafe
				index.NewReferenceNames("module_imports/consumer", "module_imports/provider"),
				index.NewReferenceNames("module_imports/consumer", "unsafe"),
			),
		),
	)
})

var _ = Describe("ProtoIndex", func() {
	It("indexes a project and can resolve file IDs", func() {
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred(), "Failed to get current working directory")
		projectPath := filepath.Join(cwd, "test", "src", "mock1")
		projectFile := filepath.Join(projectPath, "mock.go")
		builtinPath := filepath.Join(runtime.GOROOT(), "src", "builtin", "builtin.go")

		idx, err := index.NewProtoIndex(
			index.WithProject("project_one"),
		)
		Expect(err).ToNot(HaveOccurred(), "Failed to create index")
		defer idx.Close()

		pkgParser := parser.NewParser(projectPath, idx)
		defer pkgParser.Close()

		err = pkgParser.Load()
		Expect(err).ToNot(HaveOccurred(), "Failed to load parser")

		err = pkgParser.AddPackages()
		Expect(err).ToNot(HaveOccurred(), "Failed to add packages to parser")

		err = idx.ResolveReferences()
		Expect(err).ToNot(HaveOccurred(), "Failed to resolve references")

		err = idx.Wait()
		Expect(err).ToNot(HaveOccurred(), "Failed to wait for index to finish")

		_, err = idx.FindFileId(projectFile)
		Expect(err).ToNot(HaveOccurred(), "Failed to find project file ID")
		_, err = idx.FindFileId(builtinPath)
		Expect(err).ToNot(HaveOccurred(), "Failed to find builtin file ID")
	})
})
