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

var builtinSymbols = []*index.Symbol{
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

func sortedSymbols(symbols []index.Symbol) []*index.Symbol {
	sorted := make([]*index.Symbol, len(symbols))
	for i := range symbols {
		sorted[i] = &symbols[i]
	}
	slices.SortFunc(sorted, func(a, b *index.Symbol) int {
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
	DescribeTable("Parsing a package", func(testDir string, expectedSymbols []*index.Symbol, expectedReferences []*index.ReferenceNames) {
		cwd, err := os.Getwd()
		Expect(err).ToNot(HaveOccurred(), "Failed to get current working directory")
		pkgPath := fmt.Sprintf("%s/test/src/%s", cwd, testDir)
		Expect(pkgPath).ToNot(BeEmpty(), "Package path should not be empty")

		parser := parser.NewParser(pkgPath, idx)
		defer parser.Close()

		err = parser.AddPackages()
		Expect(err).ToNot(HaveOccurred(), "Failed to add packages to parser")

		parser.Wait()

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
				index.NewReferenceNames("interface_call.MockImpl).MockFunction", "builtin.print"),
			),
		),
	)
})
