package parser

import (
	"fmt"
	"go/ast"
	"go/types"
	"iter"
	"log"

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

func NewAssignmentParser(parser *ParsingStage, pkg *packages.Package, filepath string, ast *ast.File, index index.Index) (*AssignmentParser, error) {
	moduleId, err := index.AddModule(pkg.PkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create module: %w", err)
	}
	fileId, err := index.AddFile(moduleId, pkg.Dir, filepath)
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

func (f *AssignmentParser) createInterfaceReferences(lhsMethods, rhsMethods iter.Seq[*types.Func]) {
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
		log.Printf("Processing left-hand side method: %s", lhsMethod.Name())
		log.Printf("Left-hand side method full name: %s", lhsMethod.FullName())
		for _, rhsMethod := range rhsMethodNames {
			if lhsMethod.FullName() == rhsMethod.FullName() {
				// assignment to the same type, skip
				continue
			}

			log.Printf("Processing right-hand side method: %s", rhsMethod.Name())
			if lhsMethod.Name() == rhsMethod.Name() {
				fullName := lhsMethod.FullName()
				symbolScope := GetSymbolScope(lhsMethod.Name())
				start := f.pkg.Fset.Position(lhsMethod.Pos())
				end := f.pkg.Fset.Position(lhsMethod.Pos())

				log.Print("Adding symbol for interface method ", fullName, " scope ", symbolScope, " start ", start, " end ", end)

				_, declId, err := f.index.FindSymbol(f.moduleId, f.fileId, fullName, symbolScope, index.SymbolTypeDeclaration)
				if err != nil {
					log.Fatalf("Failed to find symbol: %s", err)
				}

				log.Printf("Connecting interface method %s to implementation %s", lhsMethod.Name(), rhsMethod.Name())
				f.index.AddReference(declId, f.pkg.Fset.Position(lhsMethod.Pos()), rhsMethod.FullName(), start, end)
			}
		}
	}
}

// Map rhs return types to lhs function return types. If rhs is a single
// expression, check the type of the expression and try to extract the return
// types.
func (f *AssignmentParser) extractReturnType(rhs []ast.Expr, position, total int) (types.Type, error) {
	if len(rhs) == 0 {
		return nil, fmt.Errorf("no right-hand side expressions found")
	}
	if len(rhs) > 1 && len(rhs) != total {
		return nil, fmt.Errorf("mismatched number of right-hand side expressions: %d vs %d", len(rhs), total)
	}
	if total > 1 && len(rhs) == 1 {
		switch nestedRhs := rhs[0].(type) {
		case *ast.CallExpr:
			var ok bool
			var sig *types.Signature
			if sig, ok = f.pkg.TypesInfo.TypeOf(nestedRhs.Fun).Underlying().(*types.Signature); !ok {
				return nil, fmt.Errorf("function being called is not a valid type: %T", nestedRhs.Fun)
			}

			funcResults := sig.Results()
			if funcResults == nil || funcResults.Len() <= position {
				return nil, fmt.Errorf("function has no return values or not enough return values: %d vs %d", funcResults.Len(), position)
			}
			targetType := funcResults.At(position).Type()
			if targetType == nil {
				log.Printf("Return type at index %d is nil", position)
				return nil, fmt.Errorf("return type at index %d is nil", position)
			}

			return targetType, nil
		default:
			log.Printf("Found unhandled nested right-hand side expression: %T", nestedRhs)
			return nil, fmt.Errorf("unhandled nested right-hand side expression: %T", nestedRhs)
		}
	}

	log.Printf("Extracting return type from right-hand side expressions at position %d/%d", position, total)

	currentRhs := rhs[position]
	switch rhs := currentRhs.(type) {
	case *ast.CallExpr: // If the right-hand side is a call expression, we need to extract the return type
		var ok bool
		var sig *types.Signature
		if sig, ok = f.pkg.TypesInfo.TypeOf(rhs.Fun).Underlying().(*types.Signature); !ok {
			return nil, fmt.Errorf("function being called is not a valid type: %T", rhs.Fun)
		}

		funcResults := sig.Results()
		if funcResults == nil || funcResults.Len() <= position {
			return nil, fmt.Errorf("function has no return values or not enough return values: %d vs %d", funcResults.Len(), position)
		}

		targetType := funcResults.At(position).Type()
		if targetType == nil {
			log.Printf("Return type at index %d is nil", position)
			return nil, fmt.Errorf("return type at index %d is nil", position)
		}

		switch targetType.(type) {
		case *types.Named:
			log.Printf("Return type is a named type: %s", targetType)
			return targetType, nil
		default:
			log.Printf("Return type is not a named type: %s", targetType)
			return targetType, nil
		}
	case *ast.Ident:
		return nil, fmt.Errorf("Unimplemented: identifier on right-hand side for interface assignment: %s", rhs.Name)
	case *ast.CompositeLit:

		log.Printf("Found composite literal on right-hand side: %T", rhs)
		rhsType := f.pkg.TypesInfo.TypeOf(rhs)
		if rhsType == nil {
			log.Printf("Composite literal has no type information")
			return nil, fmt.Errorf("Composite literal has no type information")
		}

		if rhsType.Underlying() != nil {
			log.Printf("Underlying type of named type: %s", rhsType.Underlying())
		}

		return rhsType, nil
	case *ast.BasicLit:
		log.Printf("Found basic literal on right-hand side: %s", rhs.Value)
		return nil, fmt.Errorf("Unimplemented: basic literal on right-hand side for interface assignment: %s", rhs.Value)
	default:
		log.Printf("Found unhandled right-hand side expression: %T", rhs)
		return nil, fmt.Errorf("Unimplemented: unhandled right-hand side expression for interface assignment: %T", rhs)
	}
}

func (f *AssignmentParser) connectInterfaceToImplementation(lhs *types.Interface, lhsIdx int, lhsLen int, allRhs []ast.Expr) error {
	log.Printf("Connecting interface %s at index %d with %d right-hand side expressions", lhs, lhsIdx, len(allRhs))
	rhsType, err := f.extractReturnType(allRhs, lhsIdx, lhsLen)
	if err != nil {
		log.Printf("Error extracting return type: %v", err)
		return fmt.Errorf("failed to extract return type: %w", err)
	}

	if namedType, ok := rhsType.(*types.Named); !ok {
		log.Printf("Right-hand side type is not a named type: %T", rhsType)
		return fmt.Errorf("right-hand side type is not a named type: %T", rhsType)
	} else {
		f.createInterfaceReferences(lhs.Methods(), namedType.Methods())
	}

	return nil
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
		log.Printf("Found assign statement left-hand side: %T", lhs)
		lhsType := f.pkg.TypesInfo.TypeOf(lhs)
		if lhsType == nil {
			log.Printf("Left-hand side has no type information")
			continue // Skip if no type information is available
		}

		log.Println("Type of left-hand side:", lhsType)
		log.Printf("Value of left-hand side: %T", lhsType)

		varType, ok := lhsType.Underlying().(*types.Interface)
		if !ok {
			log.Printf("Lhs is of type: %s", lhsType)
			continue // Skip non-interface types
		}
		log.Printf("Lhs is an interface with methods: %v", varType.Methods())

		err := f.connectInterfaceToImplementation(varType, i, len(as.Lhs), as.Rhs)
		if err != nil {
			return false, fmt.Errorf("failed to connect interface %s to implementation: %s", varType, err)
		}
	}

	return true, nil
}

func (f *AssignmentParser) returnStmtParser(parser *ParsingStage, fnType *ast.FuncType, rs *ast.ReturnStmt) (bool, error) {
	if len(rs.Results) == 0 {
		log.Println("Skipping return statement with no results")
		return false, nil
	}

	lhs := fnType.Results
	if lhs == nil {
		log.Println("Function has no return values")
		return false, nil
	}

	for i, lhsItem := range lhs.List {
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
			err := f.connectInterfaceToImplementation(ifaceType, i, len(lhs.List), rs.Results)
			if err != nil {
				return false, fmt.Errorf("Failed to connect interface %s to implementation: %s", ifaceType, err)
			}
		} else {
			continue
		}
	}

	return true, nil
}

func (f *AssignmentParser) functionBodyParser(parser *ParsingStage, fnType *ast.FuncType, fnBody *ast.BlockStmt) (ok bool, err error) {

	if fnBody == nil {
		return false, nil
	}

	ast.Inspect(fnBody, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			log.Printf("Found nested function declaration")
			f.functionBodyParser(parser, n.Type, n.Body) // Recursively parse nested function bodies
			return false                                 // stop traversing, we are only interested in the current function body
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

	ok = true

	return
}

func (f *AssignmentParser) Parse(parser *ParsingStage) (err error) {

	ast.Inspect(f.ast, func(n ast.Node) bool {
		var ok bool
		switch n := n.(type) {
		case *ast.AssignStmt:
			ok, err = f.assignStmtParser(parser, n)
			if err != nil {
				log.Printf("Error parsing assign statement: %v", err)
				return false // stop traversing on error
			}
			return ok // continue traversing
		case *ast.FuncDecl:
			ok, err = f.functionBodyParser(parser, n.Type, n.Body)
			if err != nil {
				log.Printf("Error parsing function body: %v", err)
				return false // stop traversing on error
			}
			return ok // continue traversing
		default:
			return true // continue traversing
		}
	})

	return
}

func (p *AssignmentParser) GetId() (string, bool) {
	return p.filepath, true
}
