package parser

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/planetA/askl-golang-indexer/pkg/index"
)

func TestPackageParser(t *testing.T) {
	// Create an index
	idx, err := index.NewSqlIndex(
		index.WithIndexPath(":memory:"),
		index.WithProject("test_project"),
		index.WithRecreate(true),
	)
	assert.NoError(t, err, "Failed to create index")

	cwd, err := os.Getwd()
	assert.NoError(t, err, "Failed to get current working directory")
	pkgPath := fmt.Sprintf("%s/test/src/mock1", cwd)
	// Create a parser
	parser := NewParser(pkgPath, idx)
	defer parser.Close()

	err = parser.AddPackages()
	assert.NoError(t, err, "Failed to add packages to parser")

	parser.Wait()

	err = idx.ResolveReferences()
	assert.NoError(t, err, "Failed to resolve references")

	// Verify the index contains the expected data
	symbols, err := idx.GetAllSymbols()
	assert.NoError(t, err, "Failed to get symbols from index")
	if len(symbols) == 0 {
		t.Errorf("Expected symbols to be indexed, but found none")
	}

	// Additional checks can be added here to verify specific symbols, declarations, or references
}
