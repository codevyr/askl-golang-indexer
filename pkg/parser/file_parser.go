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

	contents, err := getFileContents(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file contents: %w", err)
	}

	fileId, err := index.AddFile(moduleId, pkg.Dir, filepath, contents)
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

func (f *FileParser) addInterfaceMethods(interfaceType *ast.InterfaceType) (bool, error) {
	for _, method := range interfaceType.Methods.List {
		if len(method.Names) == 0 {
			pos := f.pkg.Fset.Position(method.Pos())
			log.Printf("Skipping interface method with no name at %s", pos)
			continue
		}
		methodName := method.Names[0]
		obj, ok := f.pkg.TypesInfo.Defs[methodName]
		if !ok {
			return false, fmt.Errorf("expected to find definition %s", methodName)
		}
		objFunc := obj.(*types.Func)
		fullName := objFunc.FullName()
		symbolScope := GetSymbolScope(methodName.Name)
		start := f.pkg.Fset.Position(method.Pos())
		end := f.pkg.Fset.Position(method.End())

		_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeDeclaration, start, end)
		if err != nil {
			return false, fmt.Errorf("failed to add symbol: %w", err)
		}
	}

	return true, nil
}

// Find function calls in a given FuncDecl
func (f *FileParser) functionBodyParser(parser *ParsingStage, fn *ast.FuncDecl, declId index.DeclarationId) (err error) {
	if fn.Body == nil {
		return
	}

	// Traverse the function body
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		var recurse bool
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
				recurse, err = f.addInterfaceMethods(fun)
				if err != nil {
					log.Printf("Failed to add interface methods: %v", err)
					return false
				}
				return recurse
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
			if obj == nil {
				log.Printf("Unimplemented: %s call= %+v obj= %+v %s %s %T", ident, call, obj, start, end, callExpr.Fun)
				return true
			}
			if !obj.Pos().IsValid() {
				typeValue, ok := f.pkg.TypesInfo.Types[callExpr.Fun]
				if !ok {
					log.Fatalf("Failed to find type for %s in %s", ident.Name, f.filepath)
				}
				if typeValue.IsBuiltin() {
					obj, pos = parser.parser.builtin.Lookup(ident.Name)
					call = obj.Id()
				}
			} else {
				switch obj := obj.(type) {
				case *types.Func:
					call = obj.Origin().FullName()
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

			f.index.AddReference(f.fileId, pos, call, start, end)
		}
		return true
	})

	return
}

func (f *FileParser) funcDeclParser(parser *ParsingStage, fn *ast.FuncDecl) (bool, error) {
	// Check if the node is a function declaration
	obj, ok := f.pkg.TypesInfo.Defs[fn.Name]
	if !ok {
		return false, fmt.Errorf("expected to find definition %s", fn.Name)
	}
	objFunc := obj.(*types.Func)
	fullName := objFunc.FullName()

	symbolScope := GetSymbolScope(fn.Name.Name)

	start := f.pkg.Fset.Position(fn.Pos())
	end := f.pkg.Fset.Position(fn.End())
	_, declId, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeDefinition, start, end)
	if err != nil {
		return false, fmt.Errorf("failed to add symbol: %s", err)
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
		return true
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
			log.Printf("Found function type object at %s: %T", f.pkg.Fset.Position(n.Pos()), n)
			return true // continue traversing
		case *ast.FuncDecl:
			recurse, err = f.funcDeclParser(parser, n)
			if err != nil {
				log.Printf("Failed to parse function declaration %s: %v", n.Name.Name, err)
				return false // stop traversing
			}
			return recurse
		case *ast.InterfaceType:
			recurse, err = f.addInterfaceMethods(n)
			if err != nil {
				log.Printf("Failed to add interface methods: %v", err)
				return false
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
