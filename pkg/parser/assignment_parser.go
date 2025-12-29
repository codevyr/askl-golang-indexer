package parser

import (
	"fmt"
	"go/ast"
	"go/types"
	"iter"
	"log"
	"os"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"golang.org/x/tools/go/packages"
)

type AssignmentParser struct {
	filepath string
	fileId   index.FileId
	moduleId index.ModuleId
	ast      *ast.File
	pkg      *packages.Package
	index    index.Index
}

var _ Parsable = &AssignmentParser{}

func getFileContents(filepath string) ([]byte, error) {
	// Read the file contents
	contents, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file contents: %w", err)
	}
	return contents, nil
}

func NewAssignmentParser(parser *ParsingStage, pkg *packages.Package, filepath string, ast *ast.File, index index.Index) (*AssignmentParser, error) {
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

	return &AssignmentParser{
		filepath: filepath,
		ast:      ast,
		pkg:      pkg,
		index:    index,
		fileId:   fileId,
		moduleId: moduleId,
	}, nil
}

func getMethodName(method *types.Func) string {
	if method == nil {
		return "<nil>"
	}
	if method.Pkg() != nil {
		return method.FullName()
	}

	// If the method is not associated with a package, most likely it's a
	// built-in function. So we add the package name as "builtin" to the method name.

	methodName := "(builtin." + method.FullName()[1:]

	return methodName
}

func (f *AssignmentParser) createInterfaceReferences(lhsMethods, rhsMethods iter.Seq[*types.Func]) error {
	// Extract method names from the both lhs and rhs methods
	lhsMethodNames := []*types.Func{}
	for lhsMethod := range lhsMethods {
		lhsMethodNames = append(lhsMethodNames, lhsMethod)
	}
	rhsMethodNames := []*types.Func{}
	for rhsMethod := range rhsMethods {
		rhsMethodNames = append(rhsMethodNames, rhsMethod)
	}

	for _, lhsMethod := range lhsMethodNames {
		lhsMethodName := getMethodName(lhsMethod)
		for _, rhsMethod := range rhsMethodNames {
			if lhsMethodName == rhsMethod.FullName() {
				// assignment to the same type, skip
				continue
			}

			if lhsMethod.Name() == rhsMethod.Name() {
				fullName := lhsMethodName
				symbolScope := GetSymbolScope(lhsMethod.Name())
				start := f.pkg.Fset.Position(lhsMethod.Pos())
				end := f.pkg.Fset.Position(lhsMethod.Pos())

				declIds, err := f.index.FindDeclarationId(fullName, symbolScope, index.SymbolTypeDeclaration)
				if err != nil {
					return fmt.Errorf("failed to find symbol: %w", err)
				}

				// We can have multiple declarations for an anonymous interface
				// containing the same methods
				for _, declId := range declIds {
					f.index.AddReference(declId, f.pkg.Fset.Position(rhsMethod.Pos()), rhsMethod.FullName(), start, end)
				}
			}
		}
	}

	return nil
}

// Map rhs return types to lhs function return types. If rhs is a single
// expression, check the type of the expression and try to extract the return
// types.
func (f *AssignmentParser) extractReturnType(returnExpr []ast.Expr, position, total int) (types.Type, error) {
	if len(returnExpr) == 0 {
		return nil, fmt.Errorf("no right-hand side expressions found")
	}
	if len(returnExpr) > 1 && len(returnExpr) != total {
		return nil, fmt.Errorf("mismatched number of right-hand side expressions: %d vs %d", len(returnExpr), total)
	}
	if total > 1 && len(returnExpr) == 1 {
		switch nestedRhs := returnExpr[0].(type) {
		case *ast.CallExpr:
			// Use our helper function to get the return type
			targetType, err := f.getCallExprReturnType(nestedRhs, position)
			if err != nil {
				return nil, fmt.Errorf("failed to get return type from nested call expression: %w", err)
			}

			return targetType, nil
		case *ast.TypeAssertExpr:
			nestedRhsType := f.pkg.TypesInfo.TypeOf(nestedRhs.Type)
			return nestedRhsType, nil
		case *ast.IndexExpr:
			nestedRhsType := f.pkg.TypesInfo.TypeOf(nestedRhs.X)
			return nestedRhsType, nil
		case *ast.UnaryExpr:
			nestedRhsType := f.pkg.TypesInfo.TypeOf(nestedRhs.X)
			return nestedRhsType, nil
		case *ast.CompositeLit:
			nestedRhsType := f.pkg.TypesInfo.TypeOf(nestedRhs)
			return nestedRhsType, nil
		case *ast.FuncLit:
			nestedRhsType := f.pkg.TypesInfo.TypeOf(nestedRhs)
			return nestedRhsType, nil
		case *ast.SelectorExpr:
			nestedRhsType := f.pkg.TypesInfo.TypeOf(nestedRhs)
			return nestedRhsType, nil
		case *ast.Ident:
			nestedRhsType := f.pkg.TypesInfo.TypeOf(nestedRhs)
			return nestedRhsType, nil
		case *ast.ParenExpr:
			nestedRhsSlice := []ast.Expr{nestedRhs.X}
			return f.extractReturnType(nestedRhsSlice, position, total)
		default:
			// Print position of the nested right-hand side expression
			pos := f.pkg.Fset.Position(nestedRhs.Pos())
			log.Printf("Nested right-hand side expression at position %s: %T", pos, nestedRhs)
			return nil, fmt.Errorf("unhandled nested right-hand side expression: %T", nestedRhs)
		}
	}

	currentRhs := returnExpr[position]
	// Get the type of the function being called
	rhsType := f.pkg.TypesInfo.TypeOf(currentRhs)
	if rhsType == nil {
		return nil, fmt.Errorf("no type information for function in call expression")
	}

	if pointerType, ok := rhsType.(*types.Pointer); ok {
		rhsType = pointerType.Elem()
	}

	return rhsType, nil
}

func unwrapType(t types.Type) types.Type {
	for {
		switch actualType := t.(type) {
		case *types.Alias:
			t = actualType.Rhs()
		case *types.TypeParam:
			t = actualType.Constraint()
		case *types.Pointer:
			t = actualType.Elem()
		default:
			return t
		}
	}
}

func (f *AssignmentParser) connectInterfaceToImplementation(lhs *types.Interface, lhsIdx int, lhsLen int, allRhs []ast.Expr) error {
	rhsType, err := f.extractReturnType(allRhs, lhsIdx, lhsLen)
	if err != nil {
		log.Printf("Error extracting return type: %v", err)
		return fmt.Errorf("failed to extract return type: %w", err)
	}

	rhsType = unwrapType(rhsType)

	switch rhsType := rhsType.(type) {
	case *types.Named:
		return f.createInterfaceReferences(lhs.Methods(), rhsType.Methods())
	case *types.Basic, *types.Struct, *types.Slice, *types.Array, *types.Map, *types.Chan:
		return nil
	case *types.Signature:
		// If the right-hand side is a function signature, we can try to connect
		// it to the left-hand side interface
		return nil
	case *types.Interface:
		// If the right-hand side is an interface type, we can try to connect it to the left-hand side interface
		if lhs == nil {
			log.Printf("Left-hand side interface is nil, cannot connect to right-hand side interface")
			return fmt.Errorf("left-hand side interface is nil, cannot connect to right-hand side interface")
		}
		// Create references between the methods of the left-hand side interface and the right-hand side interface
		return f.createInterfaceReferences(lhs.Methods(), rhsType.Methods())
	default:
		log.Printf("Right-hand side type is not a named type: %T", rhsType)
		return fmt.Errorf("right-hand side type is not a named type: %T", rhsType)
	}
}

func (f *AssignmentParser) assignStmtParser(parser *ParsingStage, as *ast.AssignStmt) (bool, error) {
	if len(as.Lhs) == 0 {
		log.Println("Skipping assign statement with no left-hand side")
		return false, nil
	}
	if len(as.Rhs) == 0 {
		log.Println("Skipping assign statement with no right-hand side")
		return false, nil
	}

	for i, lhs := range as.Lhs {
		lhsType := f.pkg.TypesInfo.TypeOf(lhs)
		if lhsType == nil {
			continue // Skip if no type information is available
		}

		varType, ok := lhsType.Underlying().(*types.Interface)
		if !ok {
			continue // Skip non-interface types
		}

		err := f.connectInterfaceToImplementation(varType, i, len(as.Lhs), as.Rhs)
		if err != nil {
			// Print the location of the assignment
			pos := f.pkg.Fset.Position(lhs.Pos())
			log.Printf("Error connecting interface %s at position %s: %v", varType, pos, err)
			return false, fmt.Errorf("failed to connect interface %s to implementation: %s", varType, err)
		}
	}

	return true, nil
}

func (f *AssignmentParser) callExprParser(parser *ParsingStage, call *ast.CallExpr) (bool, error) {
	if call == nil {
		return false, nil
	}

	// Get the function being called
	fun := call.Fun
	if fun == nil {
		return false, nil
	}

	// Get the type of the function being called
	funType := f.pkg.TypesInfo.TypeOf(fun)
	if funType == nil {
		log.Println("Skipping call expression with no type information for function")
		return false, nil
	}

	sig, ok := funType.Underlying().(*types.Signature)
	if !ok {
		return false, nil
	}

	if sig.Params() == nil || sig.Params().Len() == 0 {
		// No return values to process
		return false, nil
	}

	nParams := sig.Params().Len()
	if sig.Variadic() && len(call.Args) >= nParams-1 {
		nParams = len(call.Args)
	}

	// Iterate over the return values of the function signature
	for i := 0; i < sig.Params().Len(); i++ {
		param := sig.Params().At(i)
		if param == nil {
			continue
		}

		paramType := param.Type()
		if paramType == nil {
			continue
		}

		varType, ok := paramType.Underlying().(*types.Interface)
		if !ok {
			continue // Skip non-interface types
		}

		err := f.connectInterfaceToImplementation(varType, i, nParams, call.Args)
		if err != nil {
			// Print the location of the call expression
			pos := f.pkg.Fset.Position(call.Pos())
			log.Printf("Error connecting interface %s at position %s: %v", varType, pos, err)
			return false, fmt.Errorf("failed to connect interface %s to implementation: %s", varType, err)
		}
	}

	return true, nil
}

func (f *AssignmentParser) valueSpecParser(parser *ParsingStage, vs *ast.ValueSpec) (bool, error) {
	if len(vs.Names) == 0 {
		return false, nil
	}
	if len(vs.Values) == 0 {
		return false, nil
	}

	for i, name := range vs.Names {
		if name == nil {
			continue
		}

		lhsType := f.pkg.TypesInfo.TypeOf(name)
		if lhsType == nil {
			continue // Skip if no type information is available
		}

		varType, ok := lhsType.Underlying().(*types.Interface)
		if !ok {
			continue // Skip non-interface types
		}

		err := f.connectInterfaceToImplementation(varType, i, len(vs.Names), vs.Values)
		if err != nil {
			// Print the location of the assignment
			pos := f.pkg.Fset.Position(name.Pos())
			log.Printf("Error connecting interface %s at position %s: %v", varType, pos, err)
			return false, fmt.Errorf("failed to connect interface %s to implementation: %s", varType, err)
		}
	}

	return true, nil
}

func (f *AssignmentParser) returnStmtParser(parser *ParsingStage, fnType *ast.FuncType, rs *ast.ReturnStmt) (bool, error) {
	if len(rs.Results) == 0 {
		return false, nil
	}

	lhs := fnType.Results
	if lhs == nil {
		log.Println("Function has no return values")
		return false, nil
	}

	total := 0
	for _, lhsItem := range lhs.List {
		total += max(len(lhsItem.Names), 1)
	}

	idx := 0
	for _, lhsItem := range lhs.List {
		countItems := max(len(lhsItem.Names), 1)
		for range countItems {
			if lhsItem == nil {
				continue
			}

			if lhsItem.Type == nil {
				continue
			}

			// Get the types.Type for this field's type directly
			lhsTypeObj := f.pkg.TypesInfo.TypeOf(lhsItem.Type)
			if lhsTypeObj == nil {
				continue
			}

			// Check if it's an interface type (could be direct or underlying)
			var ifaceType *types.Interface
			if iface, ok := lhsTypeObj.(*types.Interface); ok {
				// Direct interface type
				ifaceType = iface
			} else if named, ok := lhsTypeObj.(*types.Named); ok {
				// Named type - check if underlying type is interface
				if iface, ok := named.Underlying().(*types.Interface); ok {
					ifaceType = iface
				}
			}

			if ifaceType != nil {
				err := f.connectInterfaceToImplementation(ifaceType, idx, total, rs.Results)
				if err != nil {
					pos := f.pkg.Fset.Position(lhs.Pos())
					log.Printf("Error connecting interface %s at position %s: %v", ifaceType, pos, err)
					return false, fmt.Errorf("failed to connect interface %s to implementation: %s", ifaceType, err)
				}
			}

			idx++
		}
	}

	return true, nil
}

func (f *AssignmentParser) functionBodyParser(parser *ParsingStage, fnType *ast.FuncType, fnBody *ast.BlockStmt) (bool, error) {

	if fnBody == nil {
		return false, nil
	}

	var err error
	ok := true
	ast.Inspect(fnBody, func(n ast.Node) bool {
		if err != nil {
			log.Printf("Error encountered during parsing, stopping traversal: %v", err)
			return false // stop traversing on error
		}

		switch n := n.(type) {
		case *ast.FuncLit:
			ok, err = f.functionBodyParser(parser, n.Type, n.Body) // Recursively parse nested function bodies
			if err != nil {
				log.Printf("Error parsing nested function body: %v", err)
				return false // stop traversing on error
			}
			return false // stop traversing, we are only interested in the current function body
		case *ast.ReturnStmt:
			var ok bool
			ok, err = f.returnStmtParser(parser, fnType, n)
			if err != nil {
				log.Printf("Error parsing assign statement: %v", err)
				return false // stop traversing on error
			}
			return ok // continue traversing
		default:
			return true // continue traversing
		}
	})

	if err != nil {
		log.Printf("Error parsing function body: %v", err)
		// If we encountered an error, we should not continue parsing
		return false, fmt.Errorf("failed to parse function body: %w", err)
	}

	return ok, err
}

func (f *AssignmentParser) Parse(parser *ParsingStage) (err error) {
	// decide if stop traversing on error
	errorExit := parser.parser.continueOnError

	ast.Inspect(f.ast, func(n ast.Node) bool {
		if err != nil {
			log.Printf("Error encountered during parsing, stopping traversal (%v): %v", errorExit, err)
			return errorExit
		}

		var ok bool
		switch n := n.(type) {
		case *ast.AssignStmt:
			ok, err = f.assignStmtParser(parser, n)
			if err != nil {
				log.Printf("Error parsing assign statement: %v", err)
				return errorExit
			}
			return ok
		case *ast.ValueSpec:
			ok, err = f.valueSpecParser(parser, n)
			if err != nil {
				log.Printf("Error parsing value spec: %v", err)
				return errorExit
			}
			return ok
		case *ast.CallExpr:
			ok, err = f.callExprParser(parser, n)
			if err != nil {
				log.Printf("Error parsing call expression: %v", err)
				return errorExit
			}
			return ok
		case *ast.FuncDecl:
			ok, err = f.functionBodyParser(parser, n.Type, n.Body)
			if err != nil {
				log.Printf("Error parsing function body: %v", err)
				return errorExit
			}
			return ok
		default:
			return true
		}
	})

	if errorExit {
		log.Printf("Finished parsing file %s with error: %v", f.filepath, err)
		return nil
	}

	return err
}

func (p *AssignmentParser) GetId() (string, bool) {
	return p.filepath, true
}

// getCallExprReturnType determines the return type of a call expression at a specific position
func (f *AssignmentParser) getCallExprReturnType(callExpr *ast.CallExpr, returnIndex int) (types.Type, error) {
	// Get the type of the function being called
	funType := f.pkg.TypesInfo.TypeOf(callExpr.Fun)
	if funType == nil {
		return nil, fmt.Errorf("no type information for function in call expression")
	}

	// The function type should be a signature
	sig, ok := funType.Underlying().(*types.Signature)
	if !ok {
		return nil, fmt.Errorf("function being called is not a valid signature: %T", funType)
	}

	// Get the results (return types)
	results := sig.Results()
	if results == nil {
		return nil, fmt.Errorf("function has no return values")
	}

	if returnIndex >= results.Len() {
		return nil, fmt.Errorf("return index %d out of range, function has %d return values", returnIndex, results.Len())
	}

	returnType := results.At(returnIndex).Type()
	if returnType == nil {
		return nil, fmt.Errorf("return type at index %d is nil", returnIndex)
	}

	return returnType, nil
}
