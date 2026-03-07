package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/logging"
	"golang.org/x/tools/go/packages"
)

type Parser struct {
	builtin builtinPkgs

	pkgs []*packages.Package

	stages []*ParsingStage

	packagePath  string
	packagePaths []string
	rootPath     string
	index        index.Index

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

func WithRootPath(rootPath string) option {
	return func(p *Parser) {
		p.rootPath = rootPath
	}
}

func NewParser(packagePath string, index index.Index, options ...option) *Parser {
	return NewParserWithPaths([]string{packagePath}, index, options...)
}

func NewParserWithPaths(packagePaths []string, index index.Index, options ...option) *Parser {
	p := &Parser{
		index:        index,
		stages:       []*ParsingStage{},
		packagePaths: append([]string{}, packagePaths...),
	}
	if len(packagePaths) > 0 {
		p.packagePath = packagePaths[0]
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

	if len(p.packagePaths) == 0 {
		return fmt.Errorf("no package paths provided")
	}

	p.pkgs, err = packages.Load(cfg, p.packagePaths...)
	if err != nil {
		return fmt.Errorf("failed to load a package: %w", err)
	}

	return nil
}

func (p *Parser) AddPackages() error {
	uniquePkgs := uniquePackages(p.pkgs)

	for _, stage := range p.stages {
		logging.Infof("Running stage: %s", stage.StageName)
		p.builtin.Apply(func(pkg *packages.Package) error {
			item := stage.StageConstructor(p, pkg, p.index, p.continueOnError)
			err := stage.Parse(item)
			if err != nil {
				return fmt.Errorf("failed to parse builtin package with stage %s: %w", stage.StageName, err)
			}

			return nil
		})

		for _, pkg := range uniquePkgs {
			logging.Infof("Parsing package %s with stage %s", pkg.PkgPath, stage.StageName)
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
		logging.Infof("Finished stage: %s", stage.StageName)
	}

	return nil
}

func uniquePackages(pkgs []*packages.Package) []*packages.Package {
	seen := make(map[string]struct{}, len(pkgs))
	out := make([]*packages.Package, 0, len(pkgs))
	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		key := pkg.ID
		if key == "" {
			key = pkg.PkgPath
		}
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, pkg)
	}
	return out
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
