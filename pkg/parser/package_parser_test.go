package parser_test

import (
	"fmt"
	"log"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/parser"
)

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
	DescribeTable("Parsing a package", func(testDir string, expectedSymbols []*index.Symbol) {
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

		symbols, err := idx.GetAllSymbols()
		log.Println(symbols)
		Expect(err).ToNot(HaveOccurred(), "Failed to get symbols from index")
		Expect(symbols).ToNot(BeEmpty(), "Expected symbols to be indexed, but found none")

		for i, symbol := range symbols {
			Expect(symbol).To(index.RepresentSymbol(expectedSymbols[i]), "Symbol %v in index does not match expected symbol", i)
		}
	},
		Entry("trivial file", "mock1", []*index.Symbol{
			index.NewSymbol(1, 1, "mock1.MockFunction", index.ScopeGlobal, nil, nil),
		}),
	)
})
