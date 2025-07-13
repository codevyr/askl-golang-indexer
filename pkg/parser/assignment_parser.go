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
	for lhsMethod := range lhsMethods {
		log.Printf("Processing left-hand side method: %s", lhsMethod.Name())
		for rhsMethod := range rhsMethods {
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
	if len(allRhs) == lhsLen {
		rhs := allRhs[lhsIdx]
		log.Printf("Found assign statement right-hand side: %T", rhs)
		switch rhs := rhs.(type) {
		case *ast.CallExpr:
			log.Printf("Found call expression on right-hand side: %T", rhs)
		case *ast.Ident:
			log.Printf("Found identifier on right-hand side: %s", rhs.Name)
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
		}
	} else if len(allRhs) == 1 {
		log.Printf("Found single right-hand side expression: %T", allRhs[0])
	} else {
		log.Printf("Found %d right-hand side expressions, expected %d", len(allRhs), lhsLen)
		return fmt.Errorf("mismatched number of left-hand side and right-hand side expressions: %d vs %d", lhsLen, len(allRhs))
	}
	log.Printf("Connecting interface %s to implementation at index %d", lhs, lhsIdx)
	return nil
}

func (f *AssignmentParser) assignStmtParser(parser *ParsingStage, as *ast.AssignStmt) bool {
	if len(as.Lhs) == 0 {
		log.Println("Skipping assign statement with no left-hand side")
		return false
	}
	if len(as.Rhs) == 0 {
		log.Println("Skipping assign statement with no right-hand side")
		return false
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
			if ok {
				log.Printf("Identifier %s is an interface with methods: %v", objVar.Name(), varType.Methods())
			} else {
				log.Printf("Identifier %s is of type: %s", objVar.Name(), objVar.Type())
			}

			err := f.connectInterfaceToImplementation(varType, i, len(as.Lhs), as.Rhs)
			if err != nil {
				log.Printf("Failed to connect interface %s to implementation: %s", varType, err)
			}
		}
	}

	return true
}

func (f *AssignmentParser) Parse(parser *ParsingStage) error {

	ast.Inspect(f.ast, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			return f.assignStmtParser(parser, n)
		default:
			return true // continue traversing
		}
	})

	return nil
}

func (p *AssignmentParser) GetId() (string, bool) {
	return p.filepath, true
}
