package parser_test

import (
	"fmt"
	"log"
	"os"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/parser"
)

var builtinSymbols = []*index.SymbolDecl{
	index.NewSymbol(1, 1, "builtin.append", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.cap", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.clear", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.close", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.complex", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.copy", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.delete", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.imag", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.len", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.make", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.max", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.min", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.new", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.panic", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.print", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.println", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.real", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "builtin.recover", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(1, 1, "(builtin.error).Error", index.ScopeGlobal, index.SymbolTypeDeclaration, nil, nil),
	index.NewSymbol(2, 2, "cmp.Compare", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(2, 2, "cmp.Less", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(2, 2, "cmp.Or", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
	index.NewSymbol(2, 2, "cmp.isNaN", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
}

var builtinReferences = []*index.ReferenceNames{
	index.NewReferenceNames("cmp.Less", "cmp.isNaN"),
	index.NewReferenceNames("cmp.Less", "cmp.isNaN"),
	index.NewReferenceNames("cmp.Compare", "cmp.isNaN"),
	index.NewReferenceNames("cmp.Compare", "cmp.isNaN"),
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
		idx, err = index.NewSqlIndex(
			index.WithIndexPath("file::memory:"),
			index.WithCache(index.CacheModeShared),
			index.WithProject("test_project"),
			index.WithRecreate(true),
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
				index.NewSymbol(3, 3, "mock1.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("mock1.MockFunction", "builtin.print"),
			),
		),
		Entry("is primitive type", "primitive_types",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "primitive_types.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("primitive_types.MockFunction", "builtin.print"),
				index.NewReferenceNames("primitive_types.MockFunction", "builtin.make"),
				index.NewReferenceNames("primitive_types.MockFunction", "builtin.append"),
			),
		),
		Entry("is unsafe", "unsafe",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "unsafe.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
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
			),
		),
		Entry("is an interface call", "interface_call",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDeclaration, nil, nil),
				index.NewSymbol(3, 3, "interface_call.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "interface_call.CallInterface", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call.CallInterface", "interface_call.Mock).MockFunction"),
				index.NewReferenceNames("interface_call.Mock).MockFunction", "interface_call.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call.MockImpl).MockFunction", "builtin.print"),
			),
		),
		Entry("is an interface call", "interface_call2",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call2.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDeclaration, nil, nil),
				index.NewSymbol(3, 3, "interface_call2.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "interface_call2.CallInterface", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call2.CallInterface", "interface_call2.Mock).MockFunction"),
				index.NewReferenceNames("interface_call2.Mock).MockFunction", "interface_call2.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call2.MockImpl).MockFunction", "builtin.print"),
			),
		),
		Entry("is an interface call", "interface_call3",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call3.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDeclaration, nil, nil),
				index.NewSymbol(3, 3, "interface_call3.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "interface_call3.CallInterface", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call3.CallInterface", "interface_call3.Mock).MockFunction"),
				index.NewReferenceNames("interface_call3.Mock).MockFunction", "interface_call3.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call3.MockImpl).MockFunction", "builtin.print"),
			),
		),
		Entry("is an interface call", "interface_call4",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call4.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDeclaration, nil, nil),
				index.NewSymbol(3, 3, "interface_call4.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "interface_call4.CallInterface", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "interface_call4.foo", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "interface_call5.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDeclaration, nil, nil),
				index.NewSymbol(3, 3, "interface_call5.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "interface_call5.CallInterface", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call5.CallInterface", "interface_call5.Mock).MockFunction"),
				index.NewReferenceNames("interface_call5.Mock).MockFunction", "interface_call5.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call5.MockImpl).MockFunction", "builtin.print"),
			),
		),
		Entry("is an interface call", "interface_call6",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "interface_call6.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDeclaration, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "interface_call6.CallInterface", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("interface_call6.CallInterface", "interface_call6.Mock).MockFunction"),
				index.NewReferenceNames("interface_call6.Mock).MockFunction", "interface_call6.MockImpl).MockFunction"),
				index.NewReferenceNames("interface_call6.MockImpl).MockFunction", "builtin.print"),
			),
		),
		Entry("return various values", "return_values",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_values.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_values.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_values.wrapErrors).Error", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_values.wrapErrors).Unwrap", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_values.Foo", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "return_values2.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_values2.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_values2.Foo", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "return_alias.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_alias.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_alias.wrapErrors).Error", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_alias.wrapErrors).Unwrap", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_alias.Foo", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "return_alias2.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_alias2.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_alias2.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_alias2.Foo", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "return_interface.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_interface.Foo", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "type_params.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "type_params.isNaN", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "type_params.cmpLess", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "type_params.insertionSortOrdered", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "return_type_assert.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_type_assert.Foo", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_type_assert.Token).GetText", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "return_index_expression.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_index_expression.Foo", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_index_expression.Token).GetText", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_index_expression.TokenImpl).GetText", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "return_grouped.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_grouped.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_grouped.Foo", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "return_nested.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_nested.sysDialer).dialUnix", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("return_nested.MockFunction", "builtin.print"),
				index.NewReferenceNames("return_nested.MockFunction", "return_nested.sysDialer).dialUnix"),
			),
		),
		Entry("returns pointer from a function call", "return_pointer",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "return_pointer.transport).SupportsUnixFDs", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_pointer.Conn).SupportsUnixFDs", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_pointer.newNonceTcpTransport", index.ScopeLocal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_pointer.NewConn", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "return_pointer.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "assign_interface.MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "assign_interface.Types).FindExtensionByName", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "(interface).FindExtensionByName", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "(interface).FindExtensionByName", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("(interface).FindExtensionByName", "assign_interface.Types).FindExtensionByName"),
				index.NewReferenceNames("(interface).FindExtensionByName", "assign_interface.Types).FindExtensionByName"),
			),
		),
		Entry("assign a func to any", "assign_func",
			append(
				builtinSymbols,
				index.NewSymbol(3, 3, "assign_func.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDeclaration, nil, nil),
				index.NewSymbol(3, 3, "assign_func.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "assign_func.CallInterface", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
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
				index.NewSymbol(3, 3, "assign_unary.Mock).MockFunction", index.ScopeGlobal, index.SymbolTypeDeclaration, nil, nil),
				index.NewSymbol(3, 3, "assign_unary.MockImpl).MockFunction", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
				index.NewSymbol(3, 3, "assign_unary.CallInterface", index.ScopeGlobal, index.SymbolTypeDefinition, nil, nil),
			),
			append(
				builtinReferences,
				index.NewReferenceNames("assign_unary.MockImpl).MockFunction", "builtin.make"),
				index.NewReferenceNames("assign_unary.MockImpl).MockFunction", "assign_unary.Mock).MockFunction"),
				index.NewReferenceNames("assign_unary.CallInterface", "assign_unary.MockImpl).MockFunction"),
				index.NewReferenceNames("assign_unary.CallInterface", "builtin.panic"),
			),
		),
	)
})
