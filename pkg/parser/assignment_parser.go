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

func (f *AssignmentParser) connectInterfaceToImplementation(lhs *types.Interface, lhsIdx int, lhsLen int, allRhs []ast.Expr) error {
	log.Printf("Connecting interface %s at index %d with %d right-hand side expressions", lhs, lhsIdx, len(allRhs))
	if len(allRhs) == lhsLen {
		rhs := allRhs[lhsIdx]
		log.Printf("Found assign statement right-hand side: %T", rhs)
		switch rhs := rhs.(type) {
		case *ast.CallExpr:
			log.Printf("Found call expression on right-hand side: %T", rhs)
			return fmt.Errorf("Unimplemented: call expression on right-hand side for interface assignment: %T", rhs)
		case *ast.Ident:
			log.Printf("Found identifier on right-hand side: %s", rhs.Name)
			return fmt.Errorf("Unimplemented: identifier on right-hand side for interface assignment: %s", rhs.Name)
		case *ast.CompositeLit:

			log.Printf("Found composite literal on right-hand side: %T", rhs)
			rhsType := f.pkg.TypesInfo.TypeOf(rhs)
			if rhsType != nil {
				log.Printf("Type of composite literal: %s %T", rhsType, rhsType)
				if namedType, ok := rhsType.(*types.Named); ok {
					log.Printf("Composite literal is of named type: %s", namedType.Obj().Name())
					if namedType.Underlying() != nil {
						log.Printf("Underlying type of named type: %s", namedType.Underlying())
					}
					log.Printf("Named type has methods: %v", namedType.Methods())
					f.createInterfaceReferences(lhs.Methods(), namedType.Methods())
				}
			} else {
				log.Printf("Composite literal has no type information")
			}
		case *ast.BasicLit:
			log.Printf("Found basic literal on right-hand side: %s", rhs.Value)
			return fmt.Errorf("Unimplemented: basic literal on right-hand side for interface assignment: %s", rhs.Value)
		default:
			log.Printf("Found unhandled right-hand side expression: %T", rhs)
			return fmt.Errorf("Unimplemented: unhandled right-hand side expression for interface assignment: %T", rhs)
		}
	} else if len(allRhs) == 1 {
		log.Printf("Found single right-hand side expression: %T", allRhs[0])
		switch rhs := allRhs[0].(type) {
		case *ast.CallExpr:
			log.Printf("Found call expression on right-hand side: %T", rhs)

			// Extract the function being called
			if sig, ok := f.pkg.TypesInfo.TypeOf(rhs.Fun).Underlying().(*types.Signature); ok {
				log.Printf("Function being called: %s", sig)
				funcResults := sig.Results()
				if funcResults != nil && funcResults.Len() > lhsIdx {
					log.Printf("Function returns: %s", funcResults)
					if funcResults.At(lhsIdx).Type() != nil {
						log.Printf("Return type at index %d: %s", lhsIdx, funcResults.At(lhsIdx).Type())
						if namedType, ok := funcResults.At(lhsIdx).Type().(*types.Named); ok {
							log.Printf("Return type is a named type: %s", namedType.Obj().Name())
							if namedType.Underlying() != nil {
								log.Printf("Underlying type of named type: %s", namedType.Underlying())
								f.createInterfaceReferences(lhs.Methods(), namedType.Underlying().(*types.Interface).Methods())
							} else {
								log.Printf("Named type has no underlying type")
							}
						} else {
							log.Printf("Return type is not a named type: %s", funcResults.At(lhsIdx).Type())
						}
					} else {
						log.Printf("Return type at index %d is nil", lhsIdx)
					}
				} else {
					log.Printf("Function has no return values")
				}
			} else {
				log.Printf("Function being called is not a valid type: %T", rhs.Fun)
			}
		default:
			return fmt.Errorf("Unimplemented: single right-hand side expression for interface assignment: %T", allRhs[0])
		}

	} else {
		log.Printf("Found %d right-hand side expressions, expected %d", len(allRhs), lhsLen)
		return fmt.Errorf("mismatched number of left-hand side and right-hand side expressions: %d vs %d", lhsLen, len(allRhs))
	}
	log.Printf("Connecting interface %s to implementation at index %d", lhs, lhsIdx)
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
		var objVar *types.Var
		switch lhs := lhs.(type) {
		case *ast.Ident:
			log.Printf("Assigning to identifier: %s", lhs.Name)

			if obj, ok := f.pkg.TypesInfo.Defs[lhs]; ok {
				objVar = obj.(*types.Var)
				log.Printf("Full name of identifier: %s", objVar)
			} else if obj, ok := f.pkg.TypesInfo.Uses[lhs]; ok {
				objVar = obj.(*types.Var)
				log.Printf("Full name of identifier: %s", objVar)
			} else {
				log.Panicf("Expected to find definition for identifier %s", lhs.Name)
			}

			varType, ok := objVar.Type().Underlying().(*types.Interface)
			if !ok {
				log.Printf("Identifier %s is of type: %s", objVar.Name(), objVar.Type())
				continue // Skip non-interface types
			}
			log.Printf("Identifier %s is an interface with methods: %v", objVar.Name(), varType.Methods())

			// Print position information
			start := f.pkg.Fset.Position(lhs.Pos())
			end := f.pkg.Fset.Position(lhs.End())
			log.Printf("Identifier %s position: start %s, end %s", lhs.Name, start, end)

			err := f.connectInterfaceToImplementation(varType, i, len(as.Lhs), as.Rhs)
			if err != nil {
				return false, fmt.Errorf("Failed to connect interface %s to implementation: %s", varType, err)
			}
		}
	}

	return true, nil
}

func (f *AssignmentParser) returnStmtParser(parser *ParsingStage, fn *ast.FuncDecl, rs *ast.ReturnStmt) (bool, error) {
	if len(rs.Results) == 0 {
		log.Println("Skipping return statement with no results")
		return false, nil
	}

	lhs := fn.Type.Results
	if lhs == nil {
		log.Println("Function has no return values")
		return false, nil
	}

	if len(rs.Results) != len(lhs.List) {
		log.Printf("Return statement has %d results, expected %d", len(rs.Results), len(lhs.List))
		return false, fmt.Errorf("mismatched number of return values: %d vs %d", len(rs.Results), len(lhs.List))
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
			err := f.connectInterfaceToImplementation(ifaceType, i, len(rs.Results), rs.Results)
			if err != nil {
				return false, fmt.Errorf("Failed to connect interface %s to implementation: %s", ifaceType, err)
			}
		} else {
			continue
		}
	}

	return true, nil
}

func (f *AssignmentParser) functionBodyParser(parser *ParsingStage, fn *ast.FuncDecl) (ok bool, err error) {

	if fn.Body == nil {
		return false, nil
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			return false // stop traversing, we are only interested in the current function body
		case *ast.ReturnStmt:
			var ok bool
			ok, err = f.returnStmtParser(parser, fn, n)
			if err != nil {
				log.Printf("Error parsing assign statement: %v", err)
				return false // stop traversing on error
			}
			return ok // continue traversing
		default:
			return true // continue traversing
		}
	})

	return true, nil
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
			ok, err = f.functionBodyParser(parser, n)
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
