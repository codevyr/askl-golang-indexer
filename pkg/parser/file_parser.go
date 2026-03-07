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

type FileParser struct {
	filepath string
	fileId   index.FileId
	moduleId index.ModuleId
	ast      *ast.File
	pkg      *packages.Package
	index    index.Index
}

var _ Parsable = &FileParser{}

func NewFileParser(parser *ParsingStage, pkg *packages.Package, rootPath string, filepath string, ast *ast.File, idx index.Index) (*FileParser, error) {
	moduleId, err := idx.AddModule(pkg.PkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create module: %w", err)
	}

	contents, err := getFileContents(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file contents: %w", err)
	}

	// Use rootPath as baseDir if provided, otherwise fall back to pkg.Dir
	baseDir := rootPath
	if baseDir == "" {
		baseDir = pkg.Dir
	}

	fileId, err := idx.AddFile(&moduleId, baseDir, filepath, index.GoFileType, contents)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return &FileParser{
		filepath: filepath,
		ast:      ast,
		pkg:      pkg,
		index:    idx,
		fileId:   fileId,
		moduleId: moduleId,
	}, nil
}

func (f *FileParser) addInterfaceMethods(interfaceType *ast.InterfaceType) (bool, error) {
	for _, method := range interfaceType.Methods.List {
		if len(method.Names) == 0 {
			pos := f.pkg.Fset.Position(method.Pos())
			logging.Warnf("Skipping interface method with no name at %s", pos)
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
		if err != nil {
			return false
		}

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
				logging.Debugf("Unimplemented: %s %s", start, end)
				return true
			case *ast.ParenExpr:
				logging.Debug("Unimplemented")
				return true
			case *ast.CallExpr:
				logging.Debug("Unimplemented")
				return true
			case *ast.TypeAssertExpr:
				logging.Debug("Unimplemented")
				return true
			case *ast.IndexExpr:
				logging.Debug("Unimplemented")
				return true
			case *ast.IndexListExpr:
				logging.Debug("Unimplemented")
				return true
			case *ast.ChanType:
				logging.Debug("Unimplemented")
				return true
			case *ast.InterfaceType:
				recurse, err = f.addInterfaceMethods(fun)
				if err != nil {
					logging.Errorf("Failed to add interface methods: %v", err)
					return false
				}
				return recurse
			case *ast.MapType:
				logging.Debug("Unimplemented")
				return true
			case *ast.ArrayType:
				// We do not care about array initialization
				return true
			default:
				logging.Fatalf("Unknown call expression type %T %s %s", fun, start, end)
			}
			var pos token.Position
			obj := f.pkg.TypesInfo.ObjectOf(ident)
			if obj == nil {
				logging.Debugf("Unimplemented: %s call= %+v obj= %+v %s %s %T", ident, call, obj, start, end, callExpr.Fun)
				return true
			}
			if !obj.Pos().IsValid() {
				typeValue, ok := f.pkg.TypesInfo.Types[callExpr.Fun]
				if !ok {
					logging.Fatalf("Failed to find type for %s in %s", ident.Name, f.filepath)
				}
				if typeValue.IsType() {
					// Type conversions are not references.
					return true
				}
				if typeValue.IsBuiltin() {
					obj, pos = parser.parser.builtin.Lookup(ident.Name)
					call = obj.Id()
				} else {
					return true
				}
			} else {
				switch obj := obj.(type) {
				case *types.Func:
					call = obj.Origin().FullName()
				case *types.TypeName:
					logging.Debugf("Unimplemented: %s %s %s", obj.String(), start, end)
					return true
				case *types.Var:
					logging.Debugf("Unimplemented: %s %s %s", obj.String(), start, end)
					return true
				case *types.Builtin:
					logging.Debugf("Unimplemented: %s %s %s", obj.String(), start, end)
					return true
				default:
					logging.Panicf("Unimplemented %+T", obj)
				}
				pos = f.pkg.Fset.Position(obj.Pos())
			}

			if !pos.IsValid() {
				fallbackPos, ok := parser.parser.positionForObject(obj)
				if !ok {
					return true
				}
				pos = fallbackPos
			}

			err = f.index.AddReference(f.fileId, pos, call, start, end)
			if err != nil {
				return false
			}
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

	err = f.functionBodyParser(parser, fn, declId)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (f *FileParser) typeSpecParser(parser *ParsingStage, ts *ast.TypeSpec) bool {
	if ts.Name == nil {
		logging.Warn("Skipping type spec with no name")
		return false
	}
	name := ts.Name.Name

	switch ts := ts.Type.(type) {
	case *ast.InterfaceType:
		if ts.Methods == nil {
			logging.Debugf("Skipping empty interface %s", name)
			return false
		}

		for _, method := range ts.Methods.List {
			if len(method.Names) == 0 {
				logging.Warnf("Skipping interface method with no name in %s", name)
				continue
			}
			methodName := method.Names[0]
			obj, ok := f.pkg.TypesInfo.Defs[methodName]
			if !ok {
				logging.Panicf("Expected to find definition %s", methodName)
			}
			objFunc := obj.(*types.Func)
			fullName := objFunc.FullName()
			symbolScope := GetSymbolScope(methodName.Name)
			start := f.pkg.Fset.Position(method.Pos())
			end := f.pkg.Fset.Position(method.End())

			_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeDeclaration, start, end)
			if err != nil {
				logging.Fatalf("Failed to add symbol: %s", err)
			}

		}

		return true // We do not handle interfaces yet
	default:
		return true
	}
}

func (f *FileParser) Parse(parser *ParsingStage) (err error) {

	ast.Inspect(f.ast, func(n ast.Node) bool {
		if err != nil {
			return false
		}

		var recurse bool
		switch n := n.(type) {
		case *ast.FuncLit:
			// Print the function literal
			logging.Debugf("Found function literal at %s: %T %T", f.pkg.Fset.Position(n.Pos()), n, n.Type)
			return true // continue traversing
		case *ast.FuncType:
			logging.Debugf("Found function type object at %s: %T", f.pkg.Fset.Position(n.Pos()), n)
			return true // continue traversing
		case *ast.FuncDecl:
			recurse, err = f.funcDeclParser(parser, n)
			if err != nil {
				logging.Errorf("Failed to parse function declaration %s: %v", n.Name.Name, err)
				return false // stop traversing
			}
			return recurse
		case *ast.InterfaceType:
			recurse, err = f.addInterfaceMethods(n)
			if err != nil {
				logging.Errorf("Failed to add interface methods: %v", err)
				return false
			}
			return recurse
		case *ast.TypeSpec:
			return f.typeSpecParser(parser, n)
		default:
			return true // continue traversing
		}
	})

	return
}

func (p *FileParser) GetId() (string, bool) {
	return p.filepath, true
}
