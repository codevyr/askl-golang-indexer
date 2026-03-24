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
	// unsafe package functions (manually parsed from compiler-provided package)
	index.NewSymbol(2, 2, "unsafe.Sizeof", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.Offsetof", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.Alignof", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.Add", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.Slice", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.SliceData", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.String", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(2, 2, "unsafe.StringData", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	// cmp package functions
	index.NewSymbol(3, 3, "cmp.Compare", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(3, 3, "cmp.Less", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(3, 3, "cmp.Or", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
	index.NewSymbol(3, 3, "cmp.isNaN", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
}

var builtinReferences = []*index.ReferenceNames{
	index.NewReferenceNames("cmp.Less", "cmp.isNaN"),
	index.NewReferenceNames("cmp.Less", "cmp.isNaN"),
	index.NewReferenceNames("cmp.Compare", "cmp.isNaN"),
	index.NewReferenceNames("cmp.Compare", "cmp.isNaN"),
	// Module import: builtin imports cmp
	index.NewReferenceNames("builtin", "cmp"),
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
			),
			append(
				builtinReferences,
				index.NewReferenceNames("generic_instantiation/app.Call", "generic_instantiation/app.Doer).Foo"),
				index.NewReferenceNames("generic_instantiation/app.Doer).Foo", "generic_instantiation/lib.Box[T]).Foo"),
				// Module import: app imports lib
				index.NewReferenceNames("generic_instantiation/app", "generic_instantiation/lib"),
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
			),
			append(
				builtinReferences,
				index.NewReferenceNames("(builtin.error).Error", "PathError).Error"),
			),
		),
		Entry("is an interface call", "interface_call",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call.CallInterface", "interface_call.Mock).MockFunction"),
				index.NewReferenceNames("interface_call.Mock).MockFunction", "interface_call.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call.MockImpl).MockFunction", "builtin.print"),
			),
		),
		Entry("is an interface call (2)", "interface_call2",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call2.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call2.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call2.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call2.CallInterface", "interface_call2.Mock).MockFunction"),
				index.NewReferenceNames("interface_call2.Mock).MockFunction", "interface_call2.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call2.MockImpl).MockFunction", "builtin.print"),
			),
		),
		Entry("is an interface call (3)", "interface_call3",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call3.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call3.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call3.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call3.CallInterface", "interface_call3.Mock).MockFunction"),
				index.NewReferenceNames("interface_call3.Mock).MockFunction", "interface_call3.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call3.MockImpl).MockFunction", "builtin.print"),
			),
		),
		Entry("is an interface call (4)", "interface_call4",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call4.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call4.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call4.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call4.foo", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call4.CallInterface", "interface_call4.Mock).MockFunction"),
				index.NewReferenceNames("interface_call4.CallInterface", "interface_call4.foo"),
				index.NewReferenceNames("interface_call4.Mock).MockFunction", "interface_call4.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call4.MockImpl).MockFunction", "builtin.print"),
			),
		),
		Entry("is an interface call", "interface_call5",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call5.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call5.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call5.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call5.CallInterface:<anon219>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call5.CallInterface", "interface_call5.Mock).MockFunction"),
				index.NewReferenceNames("interface_call5.CallInterface", "interface_call5.CallInterface:<anon219>"),
				index.NewReferenceNames("interface_call5.Mock).MockFunction", "interface_call5.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call5.MockImpl).MockFunction", "builtin.print"),
			),
		),
		Entry("is an interface call", "interface_call6",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call6.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.CallInterface:<anon219>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.CallInterface:<anon219>:<anon249>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.CallInterface:<anon219>:<anon249>:<anon280>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.CallInterface:<anon219>:<anon249>:<anon280>:<anon312>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call6.CallInterface", "interface_call6.Mock).MockFunction"),
				index.NewReferenceNames("interface_call6.CallInterface", "interface_call6.CallInterface:<anon219>"),
				index.NewReferenceNames("interface_call6.Mock).MockFunction", "interface_call6.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call6.MockImpl).MockFunction", "builtin.print"),
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
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_values.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_values.MockFunction", "return_values.Foo"),
				index.NewReferenceNames("error).Error", "return_values.wrapErrors).Error"),
			),
		),
		Entry("return different values", "return_values2",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_values2.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_values2.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_values2.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_values2.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_values2.MockFunction", "return_values2.Foo"),
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
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_alias.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_alias.MockFunction", "return_alias.Foo"),
				index.NewReferenceNames("error).Error", "return_alias.wrapErrors).Error"),
			),
		),
		Entry("return another type alias", "return_alias2",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_alias2.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_alias2.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_alias2.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_alias2.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_alias2.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_alias2.MockFunction", "return_alias2.Foo"),
			),
		),
		Entry("return interface", "return_interface",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_interface.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_interface.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_interface.Foo:<anon298>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_interface.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_interface.MockFunction", "return_interface.Foo"),
			),
		),
		Entry("return type params", "type_params",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "type_params.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_params.isNaN", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_params.cmpLess", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "type_params.insertionSortOrdered", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
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
			),
		),
		Entry("returns type assert", "return_type_assert",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_type_assert.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_type_assert.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_type_assert.Token).GetText", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_type_assert.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_type_assert.MockFunction", "return_type_assert.Foo"),
				index.NewReferenceNames("return_type_assert.Foo", "return_type_assert.Token).GetText"),
			),
		),
		Entry("returns type assert", "return_index_expression",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_index_expression.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_index_expression.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_index_expression.Token).GetText", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_index_expression.TokenImpl).GetText", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_index_expression.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_index_expression.MockFunction", "return_index_expression.Foo"),
				index.NewReferenceNames("return_index_expression.Token).GetText", "return_index_expression.TokenImpl).GetText"),
			),
		),
		Entry("returns grouped variables", "return_grouped",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_grouped.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_grouped.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_grouped.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_grouped.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_grouped.MockFunction", "return_grouped.Foo"),
			),
		),
		Entry("has a nested function", "return_nested",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_nested.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "return_nested.sysDialer).dialUnix", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "sysDialer).dialUnix:<anon157>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_nested.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_nested.MockFunction", "return_nested.sysDialer).dialUnix"),
				index.NewReferenceNames("sysDialer).dialUnix", "sysDialer).dialUnix:<anon157>"),
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
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_pointer.transport).SupportsUnixFDs", "return_pointer.Conn).SupportsUnixFDs"),
				index.NewReferenceNames("return_pointer.newNonceTcpTransport", "NewConn"),
				index.NewReferenceNames("return_pointer.MockFunction", "return_pointer.newNonceTcpTransport"),
				index.NewReferenceNames("return_pointer.MockFunction", "builtin.print"),
			),
		),
		Entry("assigns anonymous interfaces", "assign_interface",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "assign_interface.MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "assign_interface.Types).FindExtensionByName", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "(interface).FindExtensionByName", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "(interface).FindExtensionByName", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("(interface).FindExtensionByName", "assign_interface.Types).FindExtensionByName"),
			),
		),
		Entry("assign a func to any", "assign_func",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "assign_func.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "assign_func.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "assign_func.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "MockImpl).MockFunction:<anon191>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("assign_func.CallInterface", "assign_func.MockImpl).MockFunction"),
				index.NewReferenceNames("assign_func.CallInterface", "builtin.panic"),
			),
		),
		Entry("has unary expression", "assign_unary",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "assign_unary.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "assign_unary.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "assign_unary.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("assign_unary.MockImpl).MockFunction", "builtin.make"),
				index.NewReferenceNames("assign_unary.MockImpl).MockFunction", "assign_unary.Mock).MockFunction"),
				index.NewReferenceNames("assign_unary.CallInterface", "assign_unary.MockImpl).MockFunction"),
				index.NewReferenceNames("assign_unary.CallInterface", "builtin.panic"),
			),
		),
		Entry("is an indirect interface call", "interface_indirect1",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_indirect1.Mock2).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_indirect1.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_indirect1.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_indirect1.CallInterface:<anon280>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_indirect1.Mock2).MockFunction", "interface_indirect1.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_indirect1.MockImpl).MockFunction", "builtin.print"),
				index.NewReferenceNames("interface_indirect1.CallInterface", "builtin.print"),
				index.NewReferenceNames("interface_indirect1.CallInterface", "interface_indirect1.CallInterface:<anon280>"),
			),
		),
		Entry("is an interface mapping through func parameter", "interface_call7",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call7.Mock2).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call7.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call7.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call7.CallInterface:<anon254>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call7.Mock2).MockFunction", "interface_call7.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call7.MockImpl).MockFunction", "builtin.print"),
				index.NewReferenceNames("interface_call7.CallInterface", "builtin.print"),
				index.NewReferenceNames("interface_call7.CallInterface", "interface_call7.CallInterface:<anon254>"),
			),
		),
		Entry("is an interface mapping through func parameter (2)", "interface_call8",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call8.Mock2).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call8.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "interface_call8.MockImpl).CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "MockImpl).CallInterface:<anon269>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call8.Mock2).MockFunction", "interface_call8.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call8.MockImpl).MockFunction", "builtin.print"),
				index.NewReferenceNames("interface_call8.MockImpl).CallInterface", "builtin.print"),
				index.NewReferenceNames("MockImpl).CallInterface", "MockImpl).CallInterface:<anon269>"),
			),
		),
		Entry("checks correctness of callExprParser", "call_expr_parser",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "call_expr_parser.Writer).Write", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "call_expr_parser.Fprintln", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "call_expr_parser.File).Write", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "call_expr_parser.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("call_expr_parser.Writer).Write", "call_expr_parser.File).Write"),
				index.NewReferenceNames("call_expr_parser.Fprintln", "call_expr_parser.Writer).Write"),
				index.NewReferenceNames("call_expr_parser.CallInterface", "builtin.print"),
				index.NewReferenceNames("call_expr_parser.CallInterface", "call_expr_parser.Fprintln"),
			),
		),
		Entry("checks passing func as parameter", "func_as_param",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "func_as_param.FooType).Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "func_as_param.CallInterface", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "func_as_param.CallInterface:<anon307>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("func_as_param.CallInterface", "builtin.print"),
			),
		),
		Entry("checks generic functions", "generic_functions",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "generic_functions.Foo", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "generic_functions.NewPodSetReducer", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "generic_functions.PodSetReducer[R]).Search", index.ScopeGlobal, index.SymbolTypeFunction, nil, nil),
				index.NewSymbol(3, 3, "generic_functions.Foo:<anon300>", index.ScopeLocal, index.SymbolTypeFunction, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("Foo", "generic_functions.NewPodSetReducer"),
				index.NewReferenceNames("Foo", "PodSetReducer[R]).Search"),
				index.NewReferenceNames("Foo", "builtin.print"),
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
			),
			append(
				builtinReferences,
				index.NewReferenceNames("ClearUnknown", "(interface).Has"),
				index.NewReferenceNames("ClearUnknown", "builtin.print"),
				index.NewReferenceNames("(interface).Has", "fieldNum).Has"),
				index.NewReferenceNames("MockFunction", "ClearUnknown"),
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
			),
			append(
				builtinReferences,
				index.NewReferenceNames("local_type_in_closure.Outer", "local_type_in_closure.Outer:<anon56>"),
				index.NewReferenceNames("Outer:<anon56>", "canceler).Cancel"),
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
