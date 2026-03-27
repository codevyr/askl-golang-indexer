package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sync"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/logging"
	"golang.org/x/tools/go/packages"
)

// methodReceiverRef records a deferred TYPE->FUNCTION reference for methods
// with receivers. Processing is deferred because during concurrent stage 1
// parsing, the type's file may not yet be indexed.
type methodReceiverRef struct {
	typeName       *types.TypeName
	methodFullName string
	methodPos      token.Position
	pkg            *packages.Package
}

// deferredTypeRef records a deferred TYPE->TYPE reference when the target
// type's file is not yet indexed during concurrent stage 1 parsing.
type deferredTypeRef struct {
	fromFileId index.FileId
	typeName   *types.TypeName
	useStart   token.Position
	useEnd     token.Position
	pkg        *packages.Package
}

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

	// Deferred method receiver references (TYPE->FUNCTION).
	// Collected during stage 1, resolved between stage 1 and stage 2.
	receiverRefsMu sync.Mutex
	receiverRefs   []methodReceiverRef

	// Deferred type-to-type references.
	// Collected during stage 1 when target file is not yet indexed.
	typeRefsMu sync.Mutex
	typeRefs   []deferredTypeRef
}

const stageNamePackageParser = "PackageParser"

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
		NewParsingStage(p, stageNamePackageParser, NewPackageParser),
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

		// After PackageParser (stage 1), resolve deferred refs (receivers + type-to-type)
		if stage.StageName == stageNamePackageParser {
			if err := p.resolveDeferredRefs(); err != nil {
				return fmt.Errorf("failed to resolve deferred refs: %w", err)
			}
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

// typeFullName returns the canonical fully-qualified name for a named type.
func typeFullName(tn *types.TypeName) string {
	return tn.Pkg().Path() + "." + tn.Name()
}

// addReceiverRef records a deferred method receiver reference for post-stage-1 processing.
func (p *Parser) addReceiverRef(ref methodReceiverRef) {
	p.receiverRefsMu.Lock()
	defer p.receiverRefsMu.Unlock()
	p.receiverRefs = append(p.receiverRefs, ref)
}

// addDeferredTypeRef records a deferred type-to-type reference for post-stage-1 processing.
func (p *Parser) addDeferredTypeRef(ref deferredTypeRef) {
	p.typeRefsMu.Lock()
	defer p.typeRefsMu.Unlock()
	p.typeRefs = append(p.typeRefs, ref)
}

// resolveDeferredRefs processes deferred references collected during concurrent
// stage 1 parsing: TYPE->FUNCTION (method receivers) and TYPE->TYPE (cross-file).
// Called after stage 1 completes, when all files are indexed.
func (p *Parser) resolveDeferredRefs() error {
	p.receiverRefsMu.Lock()
	refs := p.receiverRefs
	p.receiverRefs = nil
	p.receiverRefsMu.Unlock()

	for _, ref := range refs {
		typeDefPos := ref.pkg.Fset.Position(ref.typeName.Pos())
		typeFileId, err := p.index.FindFileId(typeDefPos.Filename)
		if err != nil {
			logging.Debugf("Skipping receiver ref for %s: type file not indexed: %v", ref.methodFullName, err)
			continue
		}
		typeFullName := typeFullName(ref.typeName)
		// Reference from TYPE to FUNCTION, placed at the type name position
		typeNameEnd := ref.pkg.Fset.Position(ref.typeName.Pos() + token.Pos(len(ref.typeName.Name())))
		err = p.index.AddReference(typeFileId, ref.methodPos, ref.methodFullName, typeDefPos, typeNameEnd)
		if err != nil {
			logging.Errorf("Failed to add receiver ref %s -> %s: %v", typeFullName, ref.methodFullName, err)
		}
	}

	// Resolve deferred type-to-type references
	p.typeRefsMu.Lock()
	typeRefs := p.typeRefs
	p.typeRefs = nil
	p.typeRefsMu.Unlock()

	for _, ref := range typeRefs {
		defPos := ref.pkg.Fset.Position(ref.typeName.Pos())
		if _, err := p.index.FindFileId(defPos.Filename); err != nil {
			logging.Debugf("Skipping deferred type ref to %s: file not indexed: %v", ref.typeName.Name(), err)
			continue
		}
		fullName := typeFullName(ref.typeName)
		if err := p.index.AddReference(ref.fromFileId, defPos, fullName, ref.useStart, ref.useEnd); err != nil {
			logging.Errorf("Failed to add deferred type ref -> %s: %v", fullName, err)
		}
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
