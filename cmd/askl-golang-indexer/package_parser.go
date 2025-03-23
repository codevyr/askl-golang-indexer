package main

import (
	"fmt"
	"go/ast"
	"log"
	"sync"
	"unicode"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"golang.org/x/tools/go/packages"
)

type Parsable interface {
	Parse(parser *Parser) error
	GetId() (string, bool)
}

type PackageParser struct {
	pkg   *packages.Package
	index *index.Index
}

var _ Parsable = &PackageParser{}

func NewPackageParser(pkg *packages.Package, index *index.Index) *PackageParser {
	return &PackageParser{
		pkg:   pkg,
		index: index,
	}
}

func (p *PackageParser) Parse(parser *Parser) error {
	fmt.Println("Package Name:", p.pkg.Name)

	if len(p.pkg.CompiledGoFiles) != len(p.pkg.Syntax) {
		log.Println(p.pkg.CompiledGoFiles, p.pkg.Syntax)
		return fmt.Errorf("not all files in a package have been parsed")
	}

	for i, file := range p.pkg.CompiledGoFiles {
		fileParser, err := NewFileParser(p.pkg, file, p.pkg.Syntax[i], p.index)
		if err != nil {
			return err
		}

		if err := parser.Parse(fileParser); err != nil {
			return err
		}
	}

	for _, importedPkg := range p.pkg.Imports {
		err := parser.Parse(NewPackageParser(importedPkg, p.index))
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
	ast      *ast.File
	pkg      *packages.Package
	index    *index.Index
}

var _ Parsable = &FileParser{}

func NewFileParser(pkg *packages.Package, filepath string, ast *ast.File, index *index.Index) (*FileParser, error) {
	fileId, err := index.AddFile(pkg.Dir, filepath)
	if err != nil {
		return nil, err
	}

	return &FileParser{
		filepath: filepath,
		ast:      ast,
		pkg:      pkg,
		index:    index,
		fileId:   fileId,
	}, nil
}

// Find function calls in a given FuncDecl
func (f *FileParser) callExprParser(fn *ast.FuncDecl, declId index.DeclarationId) {
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
				call = fmt.Sprintf("%s.%s", fun.X, fun.Sel.Name)
			case *ast.FuncLit:
				log.Println("Unimplemented")
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
			case *ast.ArrayType:
				// We do not care about array initialization
				return true
			default:
				log.Fatalf("Unknown call expression type %T %s %s", fun, start, end)
			}
			obj, ok := f.pkg.TypesInfo.Uses[ident]
			if !ok {
				log.Fatalf("Failed to resolve identifier: %s", ident.Name)
			}
			pos := f.pkg.Fset.Position(obj.Pos())

			log.Printf(">>>>> %s:%s:%s", obj.Pkg(), pos, call)
			f.index.AddReference(declId, call, start, end)
		}
		return true
	})
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

func (f *FileParser) funcDeclParser(n ast.Node) bool {
	// Check if the node is a function declaration
	if fn, ok := n.(*ast.FuncDecl); ok {
		var fullName string
		if fn.Recv != nil {
			f := fn.Recv.List[0]
			if f.Names != nil {
				fullName = fmt.Sprintf("%s.%s", getReceiverType(f.Type), fn.Name.Name)
			}
		} else {
			fullName = fn.Name.Name
		}

		symbolScope := GetSymbolScope(fn.Name.Name)

		start := f.pkg.Fset.Position(n.Pos())
		end := f.pkg.Fset.Position(n.End())

		_, declId, err := f.index.AddSymbol(f.fileId, fullName, symbolScope, start, end)
		if err != nil {
			log.Fatalf("Failed to add symbol: %s", err)
		}

		f.callExprParser(fn, declId)
	}
	return true
}

func (f *FileParser) Parse(parser *Parser) error {

	ast.Inspect(f.ast, func(n ast.Node) bool {
		return f.funcDeclParser(n)
	})

	return nil
}

// getReceiverType extracts and formats the receiver type
func getReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident: // Example: func (p Person) Method() {}
		return t.Name
	case *ast.StarExpr: // Example: func (p *Person) Method() {}
		return "*" + getReceiverType(t.X)
	case *ast.IndexExpr: // Generic type (Go 1.18+), Example: func (p MyStruct[T]) Method() {}
		return getReceiverType(t.X) + "[...]"
	case *ast.IndexListExpr: // Generic type (multiple parameters)
		return getReceiverType(t.X) + "[...]"
	}
	return "unknown"
}

func (p *FileParser) GetId() (string, bool) {
	return p.filepath, true
}

type Parser struct {
	parsedPackaged map[string]bool
	channel        chan Parsable
	wg             sync.WaitGroup
}

func NewParser() *Parser {
	c := make(chan Parsable)

	p := &Parser{
		parsedPackaged: make(map[string]bool),
		channel:        c,
	}

	go p.loop()

	return p
}

func (p *Parser) Wait() {
	p.wg.Wait()
}

func (p *Parser) Close() {
	close(p.channel)
}

func (p *Parser) Parse(item Parsable) error {

	p.wg.Add(1)
	go func() { p.channel <- item }()

	return nil
}

func (p *Parser) doParse(item Parsable) error {

	if id, ok := item.GetId(); ok {
		if _, ok := p.parsedPackaged[id]; ok {
			p.wg.Done()

			return nil
		}

		p.parsedPackaged[id] = true
	}

	go func() {
		defer p.wg.Done()
		err := item.Parse(p)
		if err != nil {
			log.Fatalf("failed to parse: %s", err)
		}
	}()

	return nil
}

func (p *Parser) loop() {
	for item := range p.channel {
		err := p.doParse(item)
		if err != nil {
			log.Fatalf("failed to parse package: %v", err)
		}
	}
}
