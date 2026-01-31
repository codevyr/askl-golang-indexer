package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"golang.org/x/tools/go/packages"
)

type Parser struct {
	builtin builtinPkgs

	pkgs []*packages.Package

	stages []*ParsingStage

	packagePath string
	index       index.Index

	parseTypes      bool
	continueOnError bool
}

type option func(*Parser)

func WithParseTypes(parseTypes bool) option {
	return func(p *Parser) {
		p.parseTypes = parseTypes
	}
}

func WithContinueOnError(continueOnError bool) option {
	return func(p *Parser) {
		p.continueOnError = continueOnError
	}
}

func NewParser(packagePath string, index index.Index, options ...option) *Parser {
	p := &Parser{
		packagePath: packagePath,
		index:       index,
		stages:      []*ParsingStage{},
	}

	for _, opt := range options {
		opt(p)
	}

	p.stages = append(p.stages,
		NewParsingStage(p, "PackageParser", NewPackageParser),
		NewParsingStage(p, "AssignmentParser", NewAssignmentStage),
	)

	return p
}

func (p *Parser) Load() error {

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.LoadImports | packages.LoadAllSyntax,
		Dir:  p.packagePath,
		// BuildFlags: []string{"-tags=cgo"},
		// Dir, Env, or other settings can be specified if needed
	}

	err := p.builtin.Load(cfg)
	if err != nil {
		return fmt.Errorf("failed to load builtin packages: %w", err)
	}

	p.pkgs, err = packages.Load(cfg, p.packagePath, "builtin", "unsafe")
	if err != nil {
		return fmt.Errorf("failed to load a package: %w", err)
	}

	return nil
}

func (p *Parser) AddPackages() error {

	for _, stage := range p.stages {
		log.Printf("Running stage: %s", stage.StageName)
		p.builtin.Apply(func(pkg *packages.Package) error {
			item := stage.StageConstructor(p, pkg, p.index, p.continueOnError)
			err := stage.Parse(item)
			if err != nil {
				return fmt.Errorf("failed to parse builtin package with stage %s: %w", stage.StageName, err)
			}

			return nil
		})

		for _, pkg := range p.pkgs {
			log.Printf("Parsing package %s with stage %s", pkg.PkgPath, stage.StageName)
			item := stage.StageConstructor(p, pkg, p.index, p.continueOnError)
			err := stage.Parse(item)
			if err != nil {
				return fmt.Errorf("failed to parse package %s with stage %s: %w", pkg.PkgPath, stage.StageName, err)
			}
		}
		err := stage.Wait() // Wait for all parsing to finish
		if err != nil {
			return fmt.Errorf("failed to wait for stage %s: %w", stage.StageName, err)
		}
		log.Printf("Finished stage: %s", stage.StageName)
	}

	return nil
}

func (p *Parser) Close() {
	for i, _ := range p.stages {
		p.stages[i].Close()
	}
}

func (p *Parser) positionForObject(obj types.Object) (token.Position, bool) {
	if obj == nil {
		return token.Position{}, false
	}
	if obj.Pkg() == nil {
		return p.builtin.LookupPosition(obj.Name())
	}

	pkg := p.packageByPath(obj.Pkg().Path())
	if pkg == nil {
		return token.Position{}, false
	}

	if obj.Pos().IsValid() {
		pos := pkg.Fset.Position(obj.Pos())
		if pos.IsValid() {
			return pos, true
		}
	}

	return lookupFuncPosition(pkg, obj.Name())
}

func (p *Parser) packageByPath(path string) *packages.Package {
	for _, pkg := range p.pkgs {
		if pkg.Types != nil && pkg.Types.Path() == path {
			return pkg
		}
	}
	for _, pkg := range p.builtin.pkgs {
		if pkg.Types != nil && pkg.Types.Path() == path {
			return pkg
		}
	}
	return nil
}

func lookupFuncPosition(pkg *packages.Package, name string) (token.Position, bool) {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil {
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
	return token.Position{}, false
}
