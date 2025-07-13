package parser

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/planetA/askl-golang-indexer/pkg/index"
)

type Parsable interface {
	Parse(parser *ParsingStage) error
	GetId() (string, bool)
}

type ParserConstructor func(*Parser, *packages.Package, index.Index) Parsable

type PackageParser struct {
	parser *Parser
	pkg    *packages.Package
	index  index.Index
}

var _ Parsable = &PackageParser{}

func NewPackageParser(p *Parser, pkg *packages.Package, index index.Index) Parsable {
	return &PackageParser{
		parser: p,
		pkg:    pkg,
		index:  index,
	}
}

func (p *PackageParser) Parse(parser *ParsingStage) error {
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
		err := parser.Parse(NewPackageParser(p.parser, importedPkg, p.index))
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
	filepath string
	fileId   index.FileId
	moduleId index.ModuleId
	ast      *ast.File
	pkg      *packages.Package
	index    index.Index
}

var _ Parsable = &FileParser{}

func NewFileParser(parser *ParsingStage, pkg *packages.Package, filepath string, ast *ast.File, index index.Index) (*FileParser, error) {
	moduleId, err := index.AddModule(pkg.PkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create module: %w", err)
	}
	fileId, err := index.AddFile(moduleId, pkg.Dir, filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return &FileParser{
		filepath: filepath,
		ast:      ast,
		pkg:      pkg,
		index:    index,
		fileId:   fileId,
		moduleId: moduleId,
	}, nil
}

// Find function calls in a given FuncDecl
func (f *FileParser) functionBodyParser(parser *ParsingStage, fn *ast.FuncDecl, declId index.DeclarationId) (err error) {
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
			var pos token.Position
			obj := f.pkg.TypesInfo.ObjectOf(ident)
			if !obj.Pos().IsValid() {
				typeValue, ok := f.pkg.TypesInfo.Types[callExpr.Fun]
				if !ok {
					log.Fatalf("Failed to find type for %s in %s", ident.Name, f.filepath)
				}
				if typeValue.IsBuiltin() {
					obj = parser.parser.builtinPkg.Types.Scope().Lookup(ident.Name)
					pos = parser.parser.builtinPkg.Fset.Position(obj.Pos())
					call = obj.Id()
				}
			} else {
				switch obj := obj.(type) {
				case *types.Func:
					call = obj.FullName()
					sig, ok := obj.Type().(*types.Signature)
					if !ok {
						log.Fatalf("Function %s has no signature", call)
					}
					if sig.Recv() != nil {
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
				pos = f.pkg.Fset.Position(obj.Pos())
			}

			log.Printf("Found call %s", obj.Name())
			f.index.AddReference(declId, pos, call, start, end)
		}
		return true
	})

	return
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

func (f *FileParser) funcDeclParser(parser *ParsingStage, fn *ast.FuncDecl) (bool, error) {
	// Check if the node is a function declaration
	obj, ok := f.pkg.TypesInfo.Defs[fn.Name]
	if !ok {
		return false, fmt.Errorf("Expected to find definition %s", fn.Name)
	}
	objFunc := obj.(*types.Func)
	fullName := objFunc.FullName()

	symbolScope := GetSymbolScope(fn.Name.Name)

	start := f.pkg.Fset.Position(fn.Pos())
	end := f.pkg.Fset.Position(fn.End())
	_, declId, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeDefinition, start, end)
	if err != nil {
		return false, fmt.Errorf("Failed to add symbol: %s", err)
	}

	f.functionBodyParser(parser, fn, declId)
	return true, nil
}

func (f *FileParser) typeSpecParser(parser *ParsingStage, ts *ast.TypeSpec) bool {
	if ts.Name == nil {
		log.Println("Skipping type spec with no name")
		return false
	}
	name := ts.Name.Name

	switch ts := ts.Type.(type) {
	case *ast.InterfaceType:
		if ts.Methods == nil {
			log.Printf("Skipping empty interface %s", name)
			return false
		}

		for _, method := range ts.Methods.List {
			if len(method.Names) == 0 {
				log.Printf("Skipping interface method with no name in %s", name)
				continue
			}
			methodName := method.Names[0]
			obj, ok := f.pkg.TypesInfo.Defs[methodName]
			if !ok {
				log.Panicf("Expected to find definition %s", methodName)
			}
			objFunc := obj.(*types.Func)
			fullName := objFunc.FullName()
			symbolScope := GetSymbolScope(methodName.Name)
			start := f.pkg.Fset.Position(method.Pos())
			end := f.pkg.Fset.Position(method.End())

			_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeDeclaration, start, end)
			if err != nil {
				log.Fatalf("Failed to add symbol: %s", err)
			}

		}

		return true // We do not handle interfaces yet
	default:
		return false
	}
}

func (f *FileParser) Parse(parser *ParsingStage) (err error) {

	ast.Inspect(f.ast, func(n ast.Node) bool {
		var recurse bool
		switch n := n.(type) {
		case *ast.FuncLit:
			// Print the function literal
			log.Printf("Found function literal at %s: %T %T", f.pkg.Fset.Position(n.Pos()), n, n.Type)
			return true // continue traversing
		case *ast.FuncType:
			// log.Println("Unimplemented function type parsing", n)
			return true // continue traversing
		case *ast.FuncDecl:
			recurse, err = f.funcDeclParser(parser, n)
			if err != nil {
				log.Printf("Failed to parse function declaration %s: %v", n.Name.Name, err)
				return false // stop traversing
			}
			return recurse
		case *ast.TypeSpec:
			return f.typeSpecParser(parser, n)
		default:
			return true // continue traversing
		}
	})

	return nil
}

func (p *FileParser) GetId() (string, bool) {
	return p.filepath, true
}

type Parser struct {
	builtinPkg *packages.Package

	pkgs []*packages.Package

	stages []*ParsingStage

	packagePath string
	index       index.Index
}

func NewParser(packagePath string, index index.Index) *Parser {
	p := &Parser{
		packagePath: packagePath,
		index:       index,
		stages:      []*ParsingStage{},
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
		// Dir, Env, or other settings can be specified if needed
	}

	var pkgs []*packages.Package
	pkgs, err := packages.Load(cfg, "builtin")
	if err != nil {
		return fmt.Errorf("failed to load a package: %w", err)
	}
	if len(pkgs) != 1 {
		return fmt.Errorf("expected one builtin package, got %d", len(pkgs))
	}
	p.builtinPkg = pkgs[0]

	p.pkgs, err = packages.Load(cfg, p.packagePath)
	if err != nil {
		return fmt.Errorf("failed to load a package: %w", err)
	}

	return nil
}

func (p *Parser) AddPackages() error {

	for _, stage := range p.stages {
		log.Printf("Running stage: %s", stage.StageName)
		item := stage.StageConstructor(p, p.builtinPkg, p.index)
		err := stage.Parse(item)
		if err != nil {
			return fmt.Errorf("failed to parse builtin package with stage %s: %w", stage.StageName, err)
		}

		for _, pkg := range p.pkgs {
			log.Printf("Parsing package %s with stage %s", pkg.PkgPath, stage.StageName)
			item := stage.StageConstructor(p, pkg, p.index)
			err := stage.Parse(item)
			if err != nil {
				return fmt.Errorf("failed to parse package %s with stage %s: %w", pkg.PkgPath, stage.StageName, err)
			}
		}
		stage.Wait() // Wait for all parsing to finish
		log.Printf("Finished stage: %s", stage.StageName)
	}

	return nil
}

func (p *Parser) Close() {
	for i, _ := range p.stages {
		p.stages[i].Close()
	}
}
