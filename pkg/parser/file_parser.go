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
	filepath         string
	fileId           index.FileId
	moduleId         index.ModuleId
	ast              *ast.File
	pkg              *packages.Package
	index            index.Index
	anonFuncByPos    map[int]string      // FuncLit start offset -> full name
	varToFuncLitPos  map[token.Pos]int   // var def token.Pos -> FuncLit start offset (file-level for closure capture)
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
		filepath:      filepath,
		ast:           ast,
		pkg:           pkg,
		index:         idx,
		fileId:        fileId,
		moduleId:      moduleId,
		anonFuncByPos:   make(map[int]string),
		varToFuncLitPos: make(map[token.Pos]int),
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

		_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeFunction, start, end)
		if err != nil {
			return false, fmt.Errorf("failed to add symbol: %w", err)
		}
	}

	return true, nil
}

// recordFuncLitVar records a mapping from a variable's definition position to a FuncLit's
// start offset, so later Ident references to that variable resolve to the anonymous function.
func (f *FileParser) recordFuncLitVar(ident *ast.Ident, fl *ast.FuncLit) {
	if obj := f.pkg.TypesInfo.ObjectOf(ident); obj != nil {
		funcLitPos := f.pkg.Fset.Position(fl.Pos())
		f.varToFuncLitPos[obj.Pos()] = funcLitPos.Offset
	}
}

// Find function calls and anonymous functions in a given function body
func (f *FileParser) functionBodyParser(parser *ParsingStage, body *ast.BlockStmt, parentFullName string) (err error) {
	if body == nil {
		return
	}

	ast.Inspect(body, func(n ast.Node) bool {
		if err != nil {
			return false
		}

		// Track variable assignments to FuncLit for resolving calls like inner(1)
		switch node := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range node.Rhs {
				if fl, ok := rhs.(*ast.FuncLit); ok && i < len(node.Lhs) {
					if ident, ok := node.Lhs[i].(*ast.Ident); ok {
						f.recordFuncLitVar(ident, fl)
					}
				}
			}
			return true
		case *ast.ValueSpec:
			for i, val := range node.Values {
				if fl, ok := val.(*ast.FuncLit); ok && i < len(node.Names) {
					f.recordFuncLitVar(node.Names[i], fl)
				}
			}
			return true
		}

		// Handle anonymous function literals (nested functions)
		if funcLit, ok := n.(*ast.FuncLit); ok {
			err = f.funcLitParser(parser, funcLit, parentFullName)
			if err != nil {
				return false
			}
			return false // Don't descend - funcLitParser handles the body
		}

		// Handle type declarations inside function bodies (e.g., local interfaces)
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			return f.typeSpecParser(parser, typeSpec)
		}

		// Handle inline anonymous interfaces (e.g., type assertions like .(interface{ Method() }))
		if interfaceType, ok := n.(*ast.InterfaceType); ok {
			var recurse bool
			recurse, err = f.addInterfaceMethods(interfaceType)
			if err != nil {
				return false
			}
			return recurse
		}

		// Handle references to variables holding anonymous functions (e.g., passed as callbacks).
		// This also covers direct calls like inner(1) — the CallExpr handler skips *types.Var.
		if ident, ok := n.(*ast.Ident); ok && len(f.varToFuncLitPos) > 0 {
			if obj := f.pkg.TypesInfo.ObjectOf(ident); obj != nil {
				if _, isVar := obj.(*types.Var); isVar {
					if funcLitOffset, ok := f.varToFuncLitPos[obj.Pos()]; ok {
						if funcName, ok := f.anonFuncByPos[funcLitOffset]; ok {
							varPos := f.pkg.Fset.Position(obj.Pos())
							start := f.pkg.Fset.Position(ident.Pos())
							end := f.pkg.Fset.Position(ident.End())
							err = f.index.AddReference(f.fileId, varPos, funcName, start, end)
							if err != nil {
								return false
							}
						} else {
							logging.Debugf("var %s maps to FuncLit at offset %d but no anon func registered", obj.String(), funcLitOffset)
						}
					}
				}
			}
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
				// Will be handled by top-level FuncLit case when Inspect descends
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
					pos = f.pkg.Fset.Position(obj.Pos())
				case *types.TypeName:
					logging.Debugf("Unimplemented: %s %s %s", obj.String(), start, end)
					return true
				case *types.Var:
					// Variable calls to anonymous functions are handled by the Ident handler
					logging.Debugf("Unimplemented: %s %s %s", obj.String(), start, end)
					return true
				case *types.Builtin:
					logging.Debugf("Unimplemented: %s %s %s", obj.String(), start, end)
					return true
				default:
					logging.Panicf("Unimplemented %+T", obj)
				}
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

func (f *FileParser) funcLitParser(parser *ParsingStage, fn *ast.FuncLit, parentFullName string) error {
	start := f.pkg.Fset.Position(fn.Pos())
	end := f.pkg.Fset.Position(fn.End())

	name := fmt.Sprintf("%s:<anon%d>", parentFullName, start.Offset)

	_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, name, index.ScopeLocal, index.SymbolTypeFunction, start, end)
	if err != nil {
		return fmt.Errorf("failed to add anonymous function symbol: %w", err)
	}

	// Record the mapping for reference resolution
	f.anonFuncByPos[start.Offset] = name

	// Recursively parse the function body for call references and deeper nesting
	return f.functionBodyParser(parser, fn.Body, name)
}

func (f *FileParser) funcDeclParser(parser *ParsingStage, fn *ast.FuncDecl) (bool, error) {
	// Check if the node is a function declaration
	var fullName string
	obj, ok := f.pkg.TypesInfo.Defs[fn.Name]
	if ok {
		objFunc := obj.(*types.Func)
		fullName = objFunc.FullName()
	} else {
		// For compiler-provided packages like "unsafe", TypesInfo.Defs is empty
		// Try to lookup the function in the package's type scope
		if f.pkg.Types != nil {
			scopeObj := f.pkg.Types.Scope().Lookup(fn.Name.Name)
			if scopeObj != nil {
				if objFunc, ok := scopeObj.(*types.Func); ok {
					fullName = objFunc.FullName()
				} else if objBuiltin, ok := scopeObj.(*types.Builtin); ok {
					// For builtin functions (like unsafe.Sizeof which is a Builtin type)
					fullName = f.pkg.PkgPath + "." + objBuiltin.Name()
				} else {
					// Fallback: construct the name manually
					fullName = f.pkg.PkgPath + "." + fn.Name.Name
				}
			} else {
				// Object not found in scope, skip this function
				logging.Debugf("Function %s not found in package scope %s, skipping", fn.Name.Name, f.pkg.PkgPath)
				return true, nil
			}
		} else {
			return false, fmt.Errorf("expected to find definition %s", fn.Name)
		}
	}

	symbolScope := GetSymbolScope(fn.Name.Name)

	start := f.pkg.Fset.Position(fn.Pos())
	end := f.pkg.Fset.Position(fn.End())
	_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeFunction, start, end)
	if err != nil {
		return false, fmt.Errorf("failed to add symbol: %s", err)
	}

	err = f.functionBodyParser(parser, fn.Body, fullName)
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

			_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeFunction, start, end)
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
	// Process imports to create module-to-module references
	f.parseImports()

	ast.Inspect(f.ast, func(n ast.Node) bool {
		if err != nil {
			return false
		}

		var recurse bool
		switch n := n.(type) {
		case *ast.FuncLit:
			// Process if not already handled by functionBodyParser (which handles
			// FuncLits inside FuncDecl bodies). Top-level FuncLits (e.g., var f = func(){})
			// are only discovered here.
			start := f.pkg.Fset.Position(n.Pos())
			if _, alreadyProcessed := f.anonFuncByPos[start.Offset]; !alreadyProcessed {
				err = f.funcLitParser(parser, n, f.pkg.PkgPath)
				if err != nil {
					logging.Errorf("Failed to parse function literal: %v", err)
					return false
				}
			}
			return false // Don't descend - funcLitParser handles body
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

// parseImports processes import declarations to create module-to-module references.
// Each import statement creates a reference from the current module to the imported module.
func (f *FileParser) parseImports() {
	for _, imp := range f.ast.Imports {
		if imp.Path == nil {
			continue
		}

		// Get the import path (remove quotes)
		importPath := imp.Path.Value
		if len(importPath) >= 2 && importPath[0] == '"' && importPath[len(importPath)-1] == '"' {
			importPath = importPath[1 : len(importPath)-1]
		}

		// Get the position of the import statement
		startPos := f.pkg.Fset.Position(imp.Pos())
		endPos := f.pkg.Fset.Position(imp.End())

		// Add module import reference
		// Note: If the target module isn't indexed, the reference will be skipped
		// during resolveModuleImports (with a debug log)
		err := f.index.AddModuleImport(f.moduleId, importPath, f.fileId, startPos.Offset, endPos.Offset)
		if err != nil {
			logging.Warnf("Failed to add module import %s -> %s: %v", f.pkg.PkgPath, importPath, err)
		}
	}
}
