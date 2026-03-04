package parser

import (
	"fmt"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/logging"
	"golang.org/x/tools/go/packages"
)

type AssignmentStage struct {
	parser          *Parser
	pkg             *packages.Package
	index           index.Index
	continueOnError bool
}

var _ Parsable = &AssignmentStage{}

func NewAssignmentStage(parser *Parser, pkg *packages.Package, index index.Index, continueOnError bool) Parsable {
	return &AssignmentStage{
		parser:          parser,
		pkg:             pkg,
		index:           index,
		continueOnError: continueOnError,
	}
}

func (p *AssignmentStage) Parse(parser *ParsingStage) error {
	if len(p.pkg.CompiledGoFiles) != len(p.pkg.Syntax) {
		logging.Errorf("%v %v", p.pkg.CompiledGoFiles, p.pkg.Syntax)
		return fmt.Errorf("not all files in a package have been parsed")
	}

	logging.Infof("Parsing package %s (%s) with %d files", p.pkg.Name, p.pkg.PkgPath, len(p.pkg.CompiledGoFiles))
	for i, file := range p.pkg.CompiledGoFiles {
		fileParser, err := NewAssignmentParser(parser, p.pkg, file, p.pkg.Syntax[i], p.index)
		if err != nil {
			return err
		}

		if err := parser.Parse(fileParser); err != nil {
			return err
		}
	}

	for _, importedPkg := range p.pkg.Imports {
		err := parser.Parse(NewAssignmentStage(p.parser, importedPkg, p.index, p.continueOnError))
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *AssignmentStage) GetId() (string, bool) {
	return p.pkg.ID, true
}
