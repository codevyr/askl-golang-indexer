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
	filepath        string
	fileId          index.FileId
	moduleId        index.ModuleId
	ast             *ast.File
	pkg             *packages.Package
	index           index.Index
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
		filepath:        filepath,
		ast:             ast,
		pkg:             pkg,
		index:           idx,
		fileId:          fileId,
		moduleId:        moduleId,
		anonFuncByPos:   make(map[int]string),
		varToFuncLitPos: make(map[token.Pos]int),
	}, nil
}

// span converts an AST position range to the byte offsets an instance's finer
// spans are expressed in.  Both ends must be valid: a declaration with no body
// has no content span, and "absent" is a meaning the wire format carries.
//
// Callers must nil-check the node first -- ast.Node methods have pointer
// receivers, so a nil *ast.BlockStmt panics rather than reporting an invalid
// position.
func (f *FileParser) span(pos, end token.Pos) *index.Span {
	if !pos.IsValid() || !end.IsValid() {
		return nil
	}
	return index.NewSpan(f.pkg.Fset.Position(pos).Offset, f.pkg.Fset.Position(end).Offset)
}

// addDocInstance creates a documentation instance for a symbol if it has a doc comment.
func (f *FileParser) addDocInstance(symbolName string, scope index.SymbolScope,
	symbolType index.SymbolType, doc *ast.CommentGroup) {
	if doc == nil {
		return
	}
	start := f.pkg.Fset.Position(doc.Pos())
	end := f.pkg.Fset.Position(doc.End())
	// No finer spans: a doc comment has no identifier and no body, and a
	// content span equal to the extent would say nothing.
	if _, _, err := f.index.AddSymbol(f.moduleId, f.fileId, symbolName, scope, symbolType,
		index.InstanceTypeDocumentation, start, end, index.Spans{}, index.NoParent); err != nil {
		logging.Errorf("Failed to add documentation instance for %s: %v", symbolName, err)
	}
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

		// The extent is `Name(args) result`, so the signature span would just
		// repeat it; only the identifier is worth separating.
		_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeField, index.InstanceTypeDeclaration, start, end,
			index.Spans{Name: f.span(methodName.Pos(), methodName.End())}, index.NoParent)
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

// nameToken is the identifier that names what `e` denotes: `f` in `f`, `Read`
// in `r.Read` and in `pkg.Read`.  nil where `e` is not a name.
func nameToken(e ast.Expr) *ast.Ident {
	switch e := e.(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e.Sel
	}
	return nil
}

// calleeToken is the token a call calls: the name in `f(x)` and `r.Read(p)`,
// and the generic function's name in `pick[int](x)`, which is a call of `pick`
// through its instantiation.  nil where no token is the callee: `fns[i](x)`
// calls an element, `mk()(x)` a result.
func (f *FileParser) calleeToken(fun ast.Expr) *ast.Ident {
	fun = ast.Unparen(fun)
	var instantiated ast.Expr
	switch e := fun.(type) {
	case *ast.IndexExpr:
		instantiated = e.X
	case *ast.IndexListExpr:
		instantiated = e.X
	default:
		return nameToken(fun)
	}
	if name := nameToken(instantiated); name != nil {
		if _, isFunc := f.pkg.TypesInfo.Uses[name].(*types.Func); isFunc {
			return name
		}
	}
	return nil
}

// storageToken is the token of the variable that `target`, the place an
// assignment stores into, is PART OF.
//
// A FIELD OR AN ARRAY ELEMENT OF A VARIABLE IS INSIDE IT; WHAT A POINTER, A
// SLICE OR A MAP LEADS TO IS SOMEWHERE ELSE.  `cfg.Debug = true` and
// `sizes[0] = 1` (an array) change `cfg` and `sizes`, so the token is written.
// `names["a"] = 1` and `p.next = q` change something the variable only leads
// to: the variable is read to find the place, and nothing writes it.
func (f *FileParser) storageToken(target ast.Expr) *ast.Ident {
	for {
		switch e := ast.Unparen(target).(type) {
		case *ast.Ident:
			return e
		case *ast.SelectorExpr:
			selection, selects := f.pkg.TypesInfo.Selections[e]
			if !selects {
				return e.Sel // a qualified identifier: `pkg.Var`
			}
			if selection.Kind() != types.FieldVal || selection.Indirect() {
				return nil
			}
			target = e.X
		case *ast.IndexExpr:
			of := f.pkg.TypesInfo.TypeOf(e.X)
			if of == nil {
				return nil
			}
			if _, isArray := of.Underlying().(*types.Array); !isArray {
				return nil
			}
			target = e.X
		default:
			return nil
		}
	}
}

// functionBodyParser finds the references and the function literals in a
// function's body.
func (f *FileParser) functionBodyParser(parser *ParsingStage, body *ast.BlockStmt, parentFullName string) error {
	if body == nil {
		return nil
	}
	return f.codeParser(parser, body, parentFullName, index.NoParent)
}

// codeParser finds the references and the function literals in a piece of
// code: a function's body, or the initialiser of a package-level declaration.
//
// `literalParent` states where a function literal found here belongs in the
// containment tree (see funcLitParser).
func (f *FileParser) codeParser(parser *ParsingStage, code ast.Node, parentFullName string,
	literalParent index.SymbolInstanceId) (err error) {
	// WHAT A TOKEN DOES IS SAID BY THE CONSTRUCT AROUND IT.  ast.Inspect visits
	// a construct before its parts, so by the time the identifier handler below
	// reaches `inner` in `inner(1)` or `Count` in `Count++`, the call or the
	// statement around it has said what it is there.
	//
	// The tokens that are the callee of a call.
	callees := map[*ast.Ident]struct{}{}
	// The tokens of the variables an assignment stores into, with what the
	// assignment does to them: `=` writes; `+=` and `++` read what they write.
	stored := map[*ast.Ident]index.Roles{}
	stores := func(target ast.Expr, roles index.Roles) {
		if target == nil {
			return
		}
		if name := f.storageToken(target); name != nil {
			stored[name] |= roles
		}
	}

	ast.Inspect(code, func(n ast.Node) bool {
		if err != nil {
			return false
		}

		switch node := n.(type) {
		case *ast.CallExpr:
			if callee := f.calleeToken(node.Fun); callee != nil {
				callees[callee] = struct{}{}
			}
		case *ast.AssignStmt:
			roles := index.RoleWrite
			if node.Tok != token.ASSIGN && node.Tok != token.DEFINE {
				roles |= index.RoleTake
			}
			for _, target := range node.Lhs {
				stores(target, roles)
			}
		case *ast.IncDecStmt:
			stores(node.X, index.RoleWrite|index.RoleTake)
		case *ast.RangeStmt:
			// `for Count = range xs`.  With `:=` the names are new, and
			// nobody's.
			stores(node.Key, index.RoleWrite)
			stores(node.Value, index.RoleWrite)
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
			err = f.funcLitParser(parser, funcLit, parentFullName, literalParent)
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

		// An identifier that names a function, a package-level variable or
		// constant, or a variable holding a function literal.
		if ident, ok := n.(*ast.Ident); ok {
			_, called := callees[ident]
			storing, isStored := stored[ident]

			// A FUNCTION NAMED AND NOT CALLED IS TAKEN: passed, assigned,
			// returned, stored in a literal, or a method value (`r.Read`) or
			// expression (`T.Read`).  The callee of a call is the call
			// handler's below.  `Uses`, not ObjectOf: the name of an interface
			// method where it is declared is no use of it.
			if fn, isFunc := f.pkg.TypesInfo.Uses[ident].(*types.Func); isFunc {
				if !called {
					err = f.addFunctionValueRef(fn, ident)
					if err != nil {
						return false
					}
				}
				return true
			}

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
								// THE VARIABLE STANDS FOR THE FUNCTION IT NAMES
								// where it is used: `inner(1)` calls it and
								// `apply(inner)` takes it.  Where the variable
								// is assigned (`inner = other`) the token is
								// the variable itself, and of the function
								// nothing is stated.
								roles := index.RoleTake
								if isStored {
									roles = index.NoRole
								} else if called {
									roles = index.RoleCall
								}
								err = f.index.AddReference(f.fileId, varPos, funcName, start, end, nil, roles, index.NoRelation)
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
						// PACKAGE-LEVEL DATA IS WRITTEN, CALLED OR READ, AND
						// THE CONSTRUCT AROUND THE TOKEN SAYS WHICH.  Anything
						// that is neither a store nor a call takes its value
						// or its address.
						roles := index.RoleTake
						if isStored {
							roles = storing
						} else if called {
							roles = index.RoleCall
						}
						err = f.addPackageLevelDataRef(obj, ident, roles)
						if err != nil {
							return false
						}
					}
				} else if _, isConst := obj.(*types.Const); isConst {
					// A constant can only be read.
					err = f.addPackageLevelDataRef(obj, ident, index.RoleTake)
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
			case *ast.ParenExpr, *ast.IndexExpr, *ast.IndexListExpr:
				// `(f)(x)` calls `f`, and `pick[int](x)` calls `pick`; an
				// element (`fns[i](x)`) or a conversion (`(*T)(x)`) has no
				// callee token.
				ident = f.calleeToken(fun)
				if ident == nil {
					logging.Debugf("Unimplemented: %T %s %s", fun, start, end)
					return true
				}
			case *ast.CallExpr:
				logging.Debugf("Unimplemented: %T %s %s", fun, start, end)
				return true
			case *ast.TypeAssertExpr:
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

			// The REFERENCING TOKEN is the identifier that names the symbol --
			// `Target` in `a.Target(x)`, `Bump` in `mk().Bump()` -- and the
			// whole call is the construct around it.  Both used to be the call
			// expression, which made every Go reference span its own arguments;
			// the wire format reserves from_offset_* for the token and `full`
			// for the construct, as the C indexer already does (`printk` inside
			// `printk(foo)`).
			//
			// `ident`, not callExpr.Fun: for a selector, Fun is the ENTIRE
			// receiver expression, so a chain like `mk().Bump()` or
			// `reg["k"].Bump()` would give a token spanning other calls --
			// each of them a separate reference nested inside this one's
			// "name".
			tokStart := f.pkg.Fset.Position(ident.Pos())
			tokEnd := f.pkg.Fset.Position(ident.End())
			// The token is the callee of a call, whatever it names: a
			// function, a method, an interface's method, a builtin.
			err = f.index.AddReference(f.fileId, pos, call, tokStart, tokEnd,
				index.NewSpan(start.Offset, end.Offset), index.RoleCall, index.NoRelation)
			if err != nil {
				return false
			}
		}
		return true
	})

	return
}

// addFunctionValueRef emits a reference from the token `ident` to the function
// it names, stating that the function is taken there and not called.
func (f *FileParser) addFunctionValueRef(fn *types.Func, ident *ast.Ident) error {
	// A function of the universe (`error.Error`) has no position; a call of
	// one produces no reference either.
	if !fn.Pos().IsValid() {
		return nil
	}
	return f.index.AddReference(f.fileId, f.pkg.Fset.Position(fn.Pos()), fn.Origin().FullName(),
		f.pkg.Fset.Position(ident.Pos()), f.pkg.Fset.Position(ident.End()),
		nil, index.RoleTake, index.NoRelation)
}

// funcLitParser adds a function literal's symbol and walks its body.
//
// `parent` states the literal's place in the containment tree, which matters
// only for an initialiser: a literal inside a function body is already inside
// that body's range, so index.NoParent derives the right answer there.
func (f *FileParser) funcLitParser(parser *ParsingStage, fn *ast.FuncLit, parentFullName string, parent index.SymbolInstanceId) error {
	start := f.pkg.Fset.Position(fn.Pos())
	end := f.pkg.Fset.Position(fn.End())

	name := fmt.Sprintf("%s:<anon%d>", parentFullName, start.Offset)

	// A literal has no identifier, hence no name span.
	spans := index.Spans{Signature: f.span(fn.Pos(), fn.Type.End())}
	if fn.Body != nil {
		spans.Content = f.span(fn.Body.Pos(), fn.Body.End())
	}
	_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, name, index.ScopeLocal, index.SymbolTypeFunction, index.InstanceTypeDefinition, start, end, spans, parent)
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
	// The signature span starts at the declaration, not the name, so it
	// carries `func` and the receiver -- which is the whole point for a method
	// whose signature spills over several lines.  A declaration with no body
	// (an assembly stub) has no content span.
	spans := index.Spans{
		Name:      f.span(fn.Name.Pos(), fn.Name.End()),
		Signature: f.span(fn.Pos(), fn.Type.End()),
	}
	if fn.Body != nil {
		spans.Content = f.span(fn.Body.Pos(), fn.Body.End())
	}
	_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeFunction, index.InstanceTypeDefinition, start, end, spans, index.NoParent)
	if err != nil {
		return false, fmt.Errorf("failed to add symbol: %s", err)
	}
	f.addDocInstance(fullName, symbolScope, index.SymbolTypeFunction, fn.Doc)

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
		if addErr := f.index.AddReference(f.fileId, defPos, fullName, useStart, useEnd, nil, index.NoRole, index.NoRelation); addErr != nil {
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
			if addErr := f.index.AddReference(f.fileId, defPos, fullName, useStart, useEnd, nil, index.NoRole, index.NoRelation); addErr != nil {
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

	// No signature span: for a type the notion is not well defined -- `type`
	// plus the name plus any type parameters is not what a reader means by one
	// -- and absence is how the format says so.
	spans := index.Spans{Name: f.span(ts.Name.Pos(), ts.Name.End())}
	if ts.Type != nil {
		spans.Content = f.span(ts.Type.Pos(), ts.Type.End())
	}
	_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeType, index.InstanceTypeDefinition, start, end, spans, index.NoParent)
	if err != nil {
		return false, fmt.Errorf("failed to add TYPE symbol %s: %w", fullName, err)
	}
	f.addDocInstance(fullName, symbolScope, index.SymbolTypeType, ts.Doc)

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
		// 3b. For interfaces — add methods as FIELD symbols and create TYPE->FIELD refs
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

				_, _, err := f.index.AddSymbol(f.moduleId, f.fileId, methodFullName, methodScope, index.SymbolTypeField, index.InstanceTypeDeclaration, methodStart, methodEnd,
					index.Spans{Name: f.span(methodName.Pos(), methodName.End())}, index.NoParent)
				if err != nil {
					return false, fmt.Errorf("failed to add interface method symbol %s: %w", methodFullName, err)
				}

				// TYPE->FIELD reference: from_offset at the type name position.
				// Membership, like TYPE->FUNCTION: it states nothing.
				if addErr := f.index.AddReference(f.fileId, methodStart, methodFullName, typeNameStart, typeNameEnd, nil, index.NoRole, index.NoRelation); addErr != nil {
					logging.Errorf("Failed to add TYPE->FIELD reference: %v", addErr)
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
// package-level variable or constant identified by obj, at the use-site ident,
// stating what the token does to it there.
func (f *FileParser) addPackageLevelDataRef(obj types.Object, ident *ast.Ident, roles index.Roles) error {
	if obj.Pkg() == nil || obj.Parent() == nil || obj.Parent() != obj.Pkg().Scope() {
		return nil
	}
	fullName := obj.Pkg().Path() + "." + obj.Name()
	defPos := f.pkg.Fset.Position(obj.Pos())
	useStart := f.pkg.Fset.Position(ident.Pos())
	useEnd := f.pkg.Fset.Position(ident.End())
	return f.index.AddReference(f.fileId, defPos, fullName, useStart, useEnd, nil, roles, index.NoRelation)
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
		// The initialisers a name of this spec claimed for itself, below.
		claimed := map[int]struct{}{}
		for nameIdx, name := range vs.Names {
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
			// A grouped spec declares several names over ONE range, and giving
			// each name all of it is not just imprecise -- it is wrong.  The
			// names then have equal extents, so containment cannot order them:
			// the numbering makes the earlier ones childless and hands the LAST
			// one every initialiser in the spec.  Measured on
			// `var First, Second = func(){...}, func(){...}`: `First { target }`
			// returned nothing and `Second { target }` returned both closures,
			// one of which is First's.
			//
			// So a multi-name spec WITH INITIALISERS gives each name its own
			// identifier and nothing else, and states the parentage separately
			// (below) so each initialiser still belongs to its own variable.
			//
			// Only with initialisers.  `var X, Y int` has nothing inside the
			// spec that belongs to one name in particular -- the shared type
			// annotation applies to both -- and narrowing there would put the
			// DATA->TYPE reference inside neither, losing `X -> int` rather
			// than merely moving it.  So those names keep the whole spec and
			// stay co-extensive, which costs nothing: the only thing inside is
			// the annotation they share.
			start := f.pkg.Fset.Position(vs.Pos())
			end := f.pkg.Fset.Position(vs.End())
			spans := index.Spans{Name: f.span(name.Pos(), name.End())}
			if len(vs.Names) > 1 && len(vs.Values) > 0 {
				start = f.pkg.Fset.Position(name.Pos())
				end = f.pkg.Fset.Position(name.End())
			} else if len(vs.Values) == len(vs.Names) {
				spans.Content = f.span(vs.Values[nameIdx].Pos(), vs.Values[nameIdx].End())
			}
			_, instanceID, err := f.index.AddSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeData, index.InstanceTypeDefinition, start, end, spans, index.NoParent)
			if err != nil {
				return fmt.Errorf("failed to add DATA symbol %s: %w", fullName, err)
			}
			// Claim this name's own initialiser before the generic walk gets
			// to it.  `Parse` would otherwise reach the literal as an
			// unattached node and attribute it to the package, which for a
			// grouped spec is the only chance anyone has to know whose it is:
			// `var First, Second = f1, f2` gives both names one range, so no
			// extent distinguishes f1 from f2.
			if len(vs.Values) == len(vs.Names) {
				claimed[nameIdx] = struct{}{}
				if err := f.codeParser(parser, vs.Values[nameIdx], fullName, instanceID); err != nil {
					return err
				}
			}

			doc := vs.Doc
			if doc == nil {
				doc = gd.Doc
			}
			f.addDocInstance(fullName, symbolScope, index.SymbolTypeData, doc)
			hasPackageLevelValue = true
		}
		// AN INITIALISER IS CODE.  `var handlers = map[string]Handler{"a": run}`
		// takes `run` and `var limit = compute()` calls `compute` exactly as a
		// function body would, and neither used to be read at all.  The
		// initialisers no name claimed (`var _ = register(run)`, and
		// `var a, b = pair()`, which is one value for two names) belong to the
		// package, as a function literal in them always did.
		for valueIdx, value := range vs.Values {
			if _, done := claimed[valueIdx]; done {
				continue
			}
			if err := f.codeParser(parser, value, f.pkg.PkgPath, index.NoParent); err != nil {
				return err
			}
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
				err = f.funcLitParser(parser, n, f.pkg.PkgPath, index.NoParent)
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
