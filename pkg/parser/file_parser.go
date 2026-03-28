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
	anonFuncByPos   map[int]string    // FuncLit start offset -> full name
	varToFuncLitPos map[token.Pos]int // var def token.Pos -> FuncLit start offset
	// varToFuncLitPos is file-scoped and never cleared between functions.
	// This is safe because keys are token.Pos (unique per declaration site in the file),
	// so entries from different functions never collide.
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
		objFunc, ok := obj.(*types.Func)
		if !ok {
			return false, fmt.Errorf("expected *types.Func for interface method %s, got %T", methodName, obj)
		}
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
			var recurse bool
			recurse, err = f.typeSpecParser(parser, typeSpec, parentFullName)
			if err != nil {
				return false
			}
			return recurse
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

		// Handle identifier references to package-level variables, constants, and
		// variables holding anonymous functions (FuncLit vars).  The CallExpr
		// handler below skips *types.Var, so all var/const ident refs are resolved here.
		if ident, ok := n.(*ast.Ident); ok {
			if obj := f.pkg.TypesInfo.ObjectOf(ident); obj != nil {
				if varObj, isVar := obj.(*types.Var); isVar {
					// Check if this var holds a FuncLit (anonymous function)
					handled := false
					if len(f.varToFuncLitPos) > 0 {
						if funcLitOffset, ok := f.varToFuncLitPos[obj.Pos()]; ok {
							handled = true
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
					// If not a FuncLit var, check for package-level variable reference
					if !handled && !varObj.IsField() {
						err = f.addPackageLevelDataRef(obj, ident)
						if err != nil {
							return false
						}
					}
				} else if _, isConst := obj.(*types.Const); isConst {
					err = f.addPackageLevelDataRef(obj, ident)
					if err != nil {
						return false
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
				logging.Debugf("Unimplemented: %T %s %s", fun, start, end)
				return true
			case *ast.CallExpr:
				logging.Debugf("Unimplemented: %T %s %s", fun, start, end)
				return true
			case *ast.TypeAssertExpr:
				logging.Debugf("Unimplemented: %T %s %s", fun, start, end)
				return true
			case *ast.IndexExpr:
				logging.Debugf("Unimplemented: %T %s %s", fun, start, end)
				return true
			case *ast.IndexListExpr:
				logging.Debugf("Unimplemented: %T %s %s", fun, start, end)
				return true
			case *ast.ChanType:
				logging.Debugf("Unimplemented: %T %s %s", fun, start, end)
				return true
			case *ast.InterfaceType:
				recurse, err = f.addInterfaceMethods(fun)
				if err != nil {
					logging.Errorf("Failed to add interface methods: %v", err)
					return false
				}
				return recurse
			case *ast.MapType:
				logging.Debugf("Unimplemented: %T %s %s", fun, start, end)
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
		objFunc, ok := obj.(*types.Func)
		if !ok {
			return false, fmt.Errorf("expected *types.Func for function %s, got %T", fn.Name.Name, obj)
		}
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

	// Record deferred TYPE->FUNCTION reference for methods with receivers
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recvType := fn.Recv.List[0].Type
		// Unwrap *ast.StarExpr, *ast.IndexExpr, *ast.IndexListExpr
		for {
			switch t := recvType.(type) {
			case *ast.StarExpr:
				recvType = t.X
				continue
			case *ast.IndexExpr:
				recvType = t.X
				continue
			case *ast.IndexListExpr:
				recvType = t.X
				continue
			}
			break
		}
		var recvIdent *ast.Ident
		switch t := recvType.(type) {
		case *ast.Ident:
			recvIdent = t
		case *ast.SelectorExpr:
			recvIdent = t.Sel
		}
		if recvIdent != nil {
			if recvObj, ok := f.pkg.TypesInfo.Uses[recvIdent]; ok {
				if tn, ok := recvObj.(*types.TypeName); ok {
					methodPos := f.pkg.Fset.Position(fn.Name.Pos())
					parser.parser.addReceiverRef(methodReceiverRef{
						typeName:       tn,
						methodFullName: fullName,
						methodPos:      methodPos,
						pkg:            f.pkg,
					})
				}
			}
		}
	}

	// Process anonymous interfaces in function signature (params, results, receiver)
	// since we return false to prevent the outer inspector from descending.
	if fn.Type != nil {
		var inspectErr error
		ast.Inspect(fn.Type, func(n ast.Node) bool {
			if inspectErr != nil {
				return false
			}
			if interfaceType, ok := n.(*ast.InterfaceType); ok {
				_, inspectErr = f.addInterfaceMethods(interfaceType)
				if inspectErr != nil {
					return false
				}
				return true
			}
			return true
		})
		if inspectErr != nil {
			return false, inspectErr
		}
	}

	err = f.functionBodyParser(parser, fn.Body, fullName)
	if err != nil {
		return false, err
	}
	return false, nil // Don't let outer inspector descend — functionBodyParser handles the body
}

// addTypeRefs adds type references, deferring any whose target file isn't indexed yet.
func (f *FileParser) addTypeRefs(parser *ParsingStage, refs []typeRef) {
	for _, ref := range refs {
		useStart := f.pkg.Fset.Position(ref.usePos)
		useEnd := f.pkg.Fset.Position(ref.useEnd)

		defPos := f.pkg.Fset.Position(ref.typeName.Pos())
		if _, findErr := f.index.FindFileId(defPos.Filename); findErr != nil {
			// Target file not indexed yet — defer for post-stage-1 resolution
			parser.parser.addDeferredTypeRef(deferredTypeRef{
				fromFileId: f.fileId,
				typeName:   ref.typeName,
				useStart:   useStart,
				useEnd:     useEnd,
				pkg:        f.pkg,
			})
			continue
		}
		fullName := typeFullName(ref.typeName)
		if addErr := f.index.AddReference(f.fileId, defPos, fullName, useStart, useEnd); addErr != nil {
			logging.Errorf("Failed to add type reference: %v", addErr)
		}
	}
}

// addBuiltinTypeRefs walks type expressions and emits references to builtin
// types (int, string, bool, etc.) that are used in the expressions. This is
// called explicitly for struct field types so that structs using builtins
// have outgoing references to those types.
func (f *FileParser) addBuiltinTypeRefs(parser *ParsingStage, exprs ...ast.Expr) {
	seen := make(map[string]struct{})

	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		if e == nil {
			return
		}
		switch n := e.(type) {
		case *ast.Ident:
			obj, ok := f.pkg.TypesInfo.Uses[n]
			if !ok {
				return
			}
			tn, ok := obj.(*types.TypeName)
			if !ok || tn.Pkg() != nil {
				return // not a builtin type
			}
			if _, dup := seen[tn.Name()]; dup {
				return
			}
			seen[tn.Name()] = struct{}{}

			fullName := "builtin." + tn.Name()
			_, defPos := parser.parser.builtin.Lookup(tn.Name())
			if !defPos.IsValid() {
				return
			}
			useStart := f.pkg.Fset.Position(n.Pos())
			useEnd := f.pkg.Fset.Position(n.End())
			if addErr := f.index.AddReference(f.fileId, defPos, fullName, useStart, useEnd); addErr != nil {
				logging.Errorf("Failed to add builtin type reference: %v", addErr)
			}
		case *ast.StarExpr:
			walk(n.X)
		case *ast.ArrayType:
			walk(n.Elt)
		case *ast.MapType:
			walk(n.Key)
			walk(n.Value)
		case *ast.ChanType:
			walk(n.Value)
		case *ast.FuncType:
			if n.Params != nil {
				for _, field := range n.Params.List {
					walk(field.Type)
				}
			}
			if n.Results != nil {
				for _, field := range n.Results.List {
					walk(field.Type)
				}
			}
		case *ast.StructType:
			if n.Fields != nil {
				for _, field := range n.Fields.List {
					walk(field.Type)
				}
			}
		}
	}

	for _, expr := range exprs {
		walk(expr)
	}
}

func (f *FileParser) typeSpecParser(parser *ParsingStage, ts *ast.TypeSpec, parentFullName string) (bool, error) {
	if ts.Name == nil {
		logging.Warn("Skipping type spec with no name")
		return false, nil
	}

	// 3a. Create TYPE symbol
	var fullName string
	obj, ok := f.pkg.TypesInfo.Defs[ts.Name]
	if ok && obj != nil {
		tn, ok := obj.(*types.TypeName)
		if !ok {
			return false, fmt.Errorf("expected *types.TypeName for type %s, got %T", ts.Name.Name, obj)
		}
		// Package-level vs local type
		if tn.Parent() == tn.Pkg().Scope() {
			fullName = typeFullName(tn)
		} else {
			fullName = parentFullName + "." + tn.Name()
		}
	} else {
		// Fallback for compiler-provided packages (builtin/unsafe)
		if f.pkg.Types != nil {
			scopeObj := f.pkg.Types.Scope().Lookup(ts.Name.Name)
			if scopeObj != nil {
				if tn, ok := scopeObj.(*types.TypeName); ok {
					fullName = typeFullName(tn)
				} else {
					fullName = f.pkg.PkgPath + "." + ts.Name.Name
				}
			} else {
				logging.Debugf("Type %s not found in package scope %s, skipping", ts.Name.Name, f.pkg.PkgPath)
				return true, nil
			}
		} else {
			logging.Debugf("No type info for type %s, skipping", ts.Name.Name)
			return true, nil
		}
	}

	symbolScope := GetSymbolScope(ts.Name.Name)
	start := f.pkg.Fset.Position(ts.Pos())
	end := f.pkg.Fset.Position(ts.End())

	_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeType, start, end)
	if err != nil {
		return false, fmt.Errorf("failed to add TYPE symbol %s: %w", fullName, err)
	}

	// 3d. Walk TypeParams for generic constraint references
	if ts.TypeParams != nil {
		constraintExprs := make([]ast.Expr, 0, len(ts.TypeParams.List))
		for _, field := range ts.TypeParams.List {
			constraintExprs = append(constraintExprs, field.Type)
		}
		refs := extractNamedTypes(f.pkg, constraintExprs...)
		f.addTypeRefs(parser, refs)
	}

	switch iface := ts.Type.(type) {
	case *ast.InterfaceType:
		// 3b. For interfaces — add methods and create TYPE->FUNCTION refs
		if iface.Methods != nil {
			var embeddedExprs []ast.Expr
			typeNameStart := f.pkg.Fset.Position(ts.Name.Pos())
			typeNameEnd := f.pkg.Fset.Position(ts.Name.End())
			for _, method := range iface.Methods.List {
				if len(method.Names) == 0 {
					// Embedded type — collect for batched TYPE->TYPE refs
					embeddedExprs = append(embeddedExprs, method.Type)
					continue
				}
				methodName := method.Names[0]
				methodObj, ok := f.pkg.TypesInfo.Defs[methodName]
				if !ok {
					return false, fmt.Errorf("expected to find definition %s", methodName)
				}
				objFunc, ok := methodObj.(*types.Func)
				if !ok {
					return false, fmt.Errorf("expected *types.Func for interface method %s, got %T", methodName, methodObj)
				}
				methodFullName := objFunc.FullName()
				methodScope := GetSymbolScope(methodName.Name)
				methodStart := f.pkg.Fset.Position(method.Pos())
				methodEnd := f.pkg.Fset.Position(method.End())

				_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, methodFullName, methodScope, index.SymbolTypeFunction, methodStart, methodEnd)
				if err != nil {
					return false, fmt.Errorf("failed to add interface method symbol %s: %w", methodFullName, err)
				}

				// TYPE->FUNCTION reference: from_offset at the type name position
				if addErr := f.index.AddReference(f.fileId, methodStart, methodFullName, typeNameStart, typeNameEnd); addErr != nil {
					logging.Errorf("Failed to add TYPE->FUNCTION reference: %v", addErr)
				}
			}
			if len(embeddedExprs) > 0 {
				f.addTypeRefs(parser, extractNamedTypes(f.pkg, embeddedExprs...))
			}
		}
		return false, nil // Don't descend — prevents double addInterfaceMethods

	default:
		// 3c. For non-interfaces — walk type expression
		refs := extractNamedTypes(f.pkg, ts.Type)
		f.addTypeRefs(parser, refs)
		// Also emit references to builtin types used in struct fields
		if structType, ok := ts.Type.(*ast.StructType); ok && structType.Fields != nil {
			fieldExprs := make([]ast.Expr, 0, len(structType.Fields.List))
			for _, field := range structType.Fields.List {
				fieldExprs = append(fieldExprs, field.Type)
			}
			f.addBuiltinTypeRefs(parser, fieldExprs...)
		}
		return true, nil // Let inspector descend for anonymous interfaces in struct fields
	}
}

// isPackageLevelValue reports whether obj is a package-level variable or
// constant (i.e. its parent scope is the package scope).
func isPackageLevelValue(obj types.Object, pkgScope *types.Scope) bool {
	switch obj.(type) {
	case *types.Var, *types.Const:
		return obj.Parent() == pkgScope
	}
	return false
}

// addPackageLevelDataRef emits a reference from the current file to a
// package-level variable or constant identified by obj, at the use-site ident.
func (f *FileParser) addPackageLevelDataRef(obj types.Object, ident *ast.Ident) error {
	if obj.Pkg() == nil || obj.Parent() == nil || obj.Parent() != obj.Pkg().Scope() {
		return nil
	}
	fullName := obj.Pkg().Path() + "." + obj.Name()
	defPos := f.pkg.Fset.Position(obj.Pos())
	useStart := f.pkg.Fset.Position(ident.Pos())
	useEnd := f.pkg.Fset.Position(ident.End())
	return f.index.AddReference(f.fileId, defPos, fullName, useStart, useEnd)
}

// valueDeclParser extracts package-level variable or constant declarations as
// DATA symbols.  It handles both token.VAR and token.CONST GenDecls.
func (f *FileParser) valueDeclParser(parser *ParsingStage, gd *ast.GenDecl) error {
	pkgScope := f.pkg.Types.Scope()
	for _, spec := range gd.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		hasPackageLevelValue := false
		for _, name := range vs.Names {
			if name.Name == "_" {
				continue // skip blank identifier
			}
			obj, ok := f.pkg.TypesInfo.Defs[name]
			if !ok || obj == nil {
				continue
			}
			if !isPackageLevelValue(obj, pkgScope) {
				continue
			}
			fullName := f.pkg.PkgPath + "." + obj.Name()
			symbolScope := GetSymbolScope(name.Name)
			start := f.pkg.Fset.Position(vs.Pos())
			end := f.pkg.Fset.Position(vs.End())
			_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeData, start, end)
			if err != nil {
				return fmt.Errorf("failed to add DATA symbol %s: %w", fullName, err)
			}
			hasPackageLevelValue = true
		}
		// If this ValueSpec has an explicit type annotation and at least one
		// package-level value was added, emit DATA→TYPE references once per spec.
		if hasPackageLevelValue && vs.Type != nil {
			refs := extractNamedTypes(f.pkg, vs.Type)
			f.addTypeRefs(parser, refs)
			f.addBuiltinTypeRefs(parser, vs.Type)
		}
	}
	return nil
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
		case *ast.GenDecl:
			if n.Tok == token.VAR || n.Tok == token.CONST {
				err = f.valueDeclParser(parser, n)
				if err != nil {
					logging.Errorf("Failed to parse %s declaration: %v", n.Tok, err)
					return false
				}
			}
			return true // Descend to discover FuncLit in var initializers, TypeSpec, etc.
		case *ast.TypeSpec:
			var recurse bool
			recurse, err = f.typeSpecParser(parser, n, f.pkg.PkgPath)
			if err != nil {
				logging.Errorf("Failed to parse type spec %s: %v", n.Name.Name, err)
				return false
			}
			return recurse
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
