package parser_test

import (
	"fmt"
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
			index.WithIndexPath(":memory:"),
			index.WithProject("test_project"),
			index.WithRecreate(true),
		)
		Expect(err).ToNot(HaveOccurred(), "Failed to create index")
	})
	AfterEach(func() {
		err := idx.Close()
		Expect(err).ToNot(HaveOccurred(), "Failed to close index")
	})
	Describe("Parsing a package", func() {
		It("should parse the package and its files", func() {
			cwd, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred(), "Failed to get current working directory")
			pkgPath := fmt.Sprintf("%s/test/src/mock1", cwd)

			parser := parser.NewParser(pkgPath, idx)
			defer parser.Close()

			err = parser.AddPackages()
			Expect(err).ToNot(HaveOccurred(), "Failed to add packages to parser")

			parser.Wait()

			err = idx.ResolveReferences()
			Expect(err).ToNot(HaveOccurred(), "Failed to resolve references")

			symbols, err := idx.GetAllSymbols()
			Expect(err).ToNot(HaveOccurred(), "Failed to get symbols from index")
			Expect(symbols).ToNot(BeEmpty(), "Expected symbols to be indexed, but found none")
		})
	})

})
