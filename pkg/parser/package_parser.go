package parser

import (
	"fmt"
	"go/ast"
	"go/types"
	"log"
	"sync"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/planetA/askl-golang-indexer/pkg/index"
)

type Parsable interface {
	Parse(parser *Parser) error
	GetId() (string, bool)
}

type PackageParser struct {
	pkg   *packages.Package
	index index.Index
}

var _ Parsable = &PackageParser{}

func NewPackageParser(pkg *packages.Package, index index.Index) *PackageParser {
	return &PackageParser{
		pkg:   pkg,
		index: index,
	}
}

func (p *PackageParser) Parse(parser *Parser) error {
	if len(p.pkg.CompiledGoFiles) != len(p.pkg.Syntax) {
		log.Println(p.pkg.CompiledGoFiles, p.pkg.Syntax)
		return fmt.Errorf("not all files in a package have been parsed")
	}

	log.Printf("Parsing package %s (%s) with %d files", p.pkg.Name, p.pkg.PkgPath, len(p.pkg.CompiledGoFiles))
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
		err := parser.Parse(NewPackageParser(importedPkg, p.index))
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *PackageParser) GetId() (string, bool) {
	return p.pkg.ID, true
}

type FileParser struct {
	parser   *Parser
	filepath string
	fileId   index.FileId
	moduleId index.ModuleId
	ast      *ast.File
	pkg      *packages.Package
	index    index.Index
}

var _ Parsable = &FileParser{}

func NewFileParser(parser *Parser, pkg *packages.Package, filepath string, ast *ast.File, index index.Index) (*FileParser, error) {
	moduleId, err := index.AddModule(pkg.PkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create module: %w", err)
	}
	fileId, err := index.AddFile(moduleId, pkg.Dir, filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return &FileParser{
		parser:   parser,
		filepath: filepath,
		ast:      ast,
		pkg:      pkg,
		index:    index,
		fileId:   fileId,
		moduleId: moduleId,
	}, nil
}

// Find function calls in a given FuncDecl
func (f *FileParser) callExprParser(fn *ast.FuncDecl, declId index.DeclarationId) {
	if fn.Body == nil {
		return
	}

	// Traverse the function body
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			start := f.pkg.Fset.Position(n.Pos())
			end := f.pkg.Fset.Position(n.End())

			// Check if the function call has an identifier (direct function call)
			var call string
			var ident *ast.Ident
			switch fun := callExpr.Fun.(type) {
			case *ast.Ident:
				ident = fun
				call = fun.Name
			case *ast.SelectorExpr:
				ident = fun.Sel
			case *ast.FuncLit:
				log.Println("Unimplemented:", start, end)
				return true
			case *ast.ParenExpr:
				log.Println("Unimplemented")
				return true
			case *ast.CallExpr:
				log.Println("Unimplemented")
				return true
			case *ast.TypeAssertExpr:
				log.Println("Unimplemented")
				return true
			case *ast.IndexExpr:
				log.Println("Unimplemented")
				return true
			case *ast.IndexListExpr:
				log.Println("Unimplemented")
				return true
			case *ast.ChanType:
				log.Println("Unimplemented")
				return true
			case *ast.InterfaceType:
				log.Println("Unimplemented")
				return true
			case *ast.MapType:
				log.Println("Unimplemented")
				return true
			case *ast.ArrayType:
				// We do not care about array initialization
				return true
			default:
				log.Fatalf("Unknown call expression type %T %s %s", fun, start, end)
			}
			obj := f.pkg.TypesInfo.ObjectOf(ident)
			if !obj.Pos().IsValid() {
				log.Println("Unimplemented built in support:", call, start, end)
				return true
			}
			switch obj := obj.(type) {
			case *types.Func:
				call = obj.FullName()
				sig, ok := obj.Type().(*types.Signature)
				if !ok {
					log.Fatalf("Function %s has no signature", call)
				}
				if sig.Recv() != nil {
					if _, ok := sig.Recv().Type().Underlying().(*types.Interface); ok {
						log.Println("Unimplemented abstract interface:", obj.String(), start, end)
						// Method in an interface, so no actual body
						return true
					}
					if sig.Recv() != sig.Recv().Origin() {
						log.Println("Unimplemented generic interface:", obj.String(), start, end)
						return true
					}
				}
			case *types.TypeName:
				log.Println("Unimplemented:", obj.String(), start, end)
				return true
			case *types.Var:
				log.Println("Unimplemented:", obj.String(), start, end)
				return true
			case *types.Builtin:
				log.Println("Unimplemented:", obj.String(), start, end)
				return true
			default:
				log.Panicf("Unimplemented %+T", obj)
			}
			pos := f.pkg.Fset.Position(obj.Pos())

			f.index.AddReference(declId, pos, call, start, end)
		}
		return true
	})
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

func (f *FileParser) funcDeclParser(n ast.Node) bool {
	// Check if the node is a function declaration
	if fn, ok := n.(*ast.FuncDecl); ok {
		obj, ok := f.pkg.TypesInfo.Defs[fn.Name]
		if !ok {
			log.Panicf("Expected to find definition %s", fn.Name)
		}
		objFunc := obj.(*types.Func)
		fullName := objFunc.FullName()

		symbolScope := GetSymbolScope(fn.Name.Name)

		start := f.pkg.Fset.Position(fn.Pos())
		end := f.pkg.Fset.Position(n.End())
		_, declId, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, start, end)
		if err != nil {
			log.Fatalf("Failed to add symbol: %s", err)
		}

		f.callExprParser(fn, declId)
	}
	return true
}

func (f *FileParser) Parse(parser *Parser) error {

	ast.Inspect(f.ast, func(n ast.Node) bool {
		return f.funcDeclParser(n)
	})

	return nil
}

func (p *FileParser) GetId() (string, bool) {
	return p.filepath, true
}

type Parser struct {
	packagePath    string
	index          index.Index
	parsedPackaged map[string]bool
	channel        chan Parsable
	wg             sync.WaitGroup
}

func NewParser(packagePath string, index index.Index) *Parser {
	c := make(chan Parsable)

	p := &Parser{
		packagePath:    packagePath,
		index:          index,
		parsedPackaged: make(map[string]bool),
		channel:        c,
	}

	go p.loop()

	return p
}

func (p *Parser) AddPackages() error {

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.LoadImports | packages.LoadAllSyntax,
		Dir:  p.packagePath,
		// Dir, Env, or other settings can be specified if needed
	}

	pkgs, err := packages.Load(cfg, p.packagePath)
	if err != nil {
		return fmt.Errorf("failed to load a package: %w", err)
	}

	// pkgs now contains package metadata, ASTs, type info, etc.
	for _, pkg := range pkgs {
		log.Printf("Found package: '%+v' %v", pkg, pkg.Dir)
		err := p.Parse(NewPackageParser(pkg, p.index))
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) AddPackage(pkg *types.Package) error {

	return nil
}

func (p *Parser) Wait() {
	p.wg.Wait()
}

func (p *Parser) Close() {
	close(p.channel)
}

func (p *Parser) Parse(item Parsable) error {

	p.wg.Add(1)
	go func() { p.channel <- item }()

	return nil
}

func (p *Parser) doParse(item Parsable) error {

	if id, ok := item.GetId(); ok {
		if _, ok := p.parsedPackaged[id]; ok {
			p.wg.Done()

			return nil
		}

		p.parsedPackaged[id] = true
	}

	go func() {
		defer p.wg.Done()
		err := item.Parse(p)
		if err != nil {
			log.Fatalf("failed to parse: %s", err)
		}
	}()

	return nil
}

func (p *Parser) loop() {
	for item := range p.channel {
		err := p.doParse(item)
		if err != nil {
			log.Fatalf("failed to parse package: %v", err)
		}
	}
}
