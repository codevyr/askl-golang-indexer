package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/logging"
)

type Parsable interface {
	Parse(parser *ParsingStage) error
	GetId() (string, bool)
}

type ParserConstructor func(*Parser, *packages.Package, index.Index, bool) Parsable

type PackageParser struct {
	parser          *Parser
	pkg             *packages.Package
	index           index.Index
	continueOnError bool
}

var _ Parsable = &PackageParser{}

func NewPackageParser(p *Parser, pkg *packages.Package, index index.Index, continueOnError bool) Parsable {
	return &PackageParser{
		parser:          p,
		pkg:             pkg,
		index:           index,
		continueOnError: continueOnError,
	}
}

func (p *PackageParser) Parse(parser *ParsingStage) error {
	if len(p.pkg.CompiledGoFiles) != len(p.pkg.Syntax) {
		logging.Errorf("%v %v", p.pkg.CompiledGoFiles, p.pkg.Syntax)
		return fmt.Errorf("not all files in a package have been parsed")
	}

	logging.Infof("Parsing package %s (%s) with %d files", p.pkg.Name, p.pkg.PkgPath, len(p.pkg.CompiledGoFiles))
	for i, file := range p.pkg.CompiledGoFiles {
		fileParser, err := NewFileParser(parser, p.pkg, file, p.pkg.Syntax[i], p.index)
		if err != nil {
			return err
		}

		if err := parser.Parse(fileParser); err != nil {
			return err
		}
	}

	for _, importedPkg := range p.pkg.Imports {
		err := parser.Parse(NewPackageParser(p.parser, importedPkg, p.index, p.continueOnError))
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PackageParser) GetId() (string, bool) {
	return p.pkg.ID, true
}

func GetSymbolScope(name string) index.SymbolScope {
	var first rune
	for _, c := range name {
		first = c
		break
	}

	if unicode.IsUpper(first) && unicode.IsLetter(first) {
		return index.ScopeGlobal
	}
	return index.ScopeLocal
}

type builtinPkgs struct {
	pkgs []*packages.Package
}

func (b *builtinPkgs) Load(cfg *packages.Config) error {
	pkgNames := []string{"builtin", "unsafe"}
	var pkgs []*packages.Package
	pkgs, err := packages.Load(cfg, pkgNames...)
	if err != nil {
		return fmt.Errorf("failed to load a package: %w", err)
	}
	if len(pkgs) != 2 {
		return fmt.Errorf("expected %d builtin packages, got %d", len(pkgNames), len(pkgs))
	}

	b.pkgs = pkgs

	return nil
}

func (b *builtinPkgs) Apply(f func(*packages.Package) error) error {
	for _, pkg := range b.pkgs {
		if err := f(pkg); err != nil {
			return fmt.Errorf("failed to apply function to builtin package %s: %w", pkg.PkgPath, err)
		}
	}
	return nil
}

func (b *builtinPkgs) Lookup(name string) (types.Object, token.Position) {
	for _, pkg := range b.pkgs {
		obj := pkg.Types.Scope().Lookup(name)
		if obj == nil {
			continue
		}

		pos := pkg.Fset.Position(obj.Pos())
		return obj, pos
	}

	return nil, token.Position{}
}

func (b *builtinPkgs) LookupPosition(name string) (token.Position, bool) {
	for _, pkg := range b.pkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || fn.Name == nil {
					continue
				}
				if fn.Name.Name == name {
					pos := pkg.Fset.Position(fn.Name.Pos())
					if pos.IsValid() {
						return pos, true
					}
				}
			}
		}
	}

	return token.Position{}, false
}
