package main

import (
	"fmt"
	"go/ast"
	"log"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

type Parsable interface {
	Parse(parser *Parser) error
	GetId() (string, bool)
}

type PackageParser struct {
	pkg *packages.Package
}

var _ Parsable = &PackageParser{}

func NewPackageParser(pkg *packages.Package) *PackageParser {
	return &PackageParser{
		pkg: pkg,
	}
}

func (p *PackageParser) Parse(parser *Parser) error {
	fmt.Println("Package Name:", p.pkg.Name)

	if len(p.pkg.CompiledGoFiles) != len(p.pkg.Syntax) {
		log.Println(p.pkg.CompiledGoFiles, p.pkg.Syntax)
		return fmt.Errorf("not all files in a package have been parsed")
	}

	for i, file := range p.pkg.CompiledGoFiles {
		err := parser.Parse(NewFileParser(file, p.pkg.Syntax[i]))
		if err != nil {
			return err
		}
	}

	for _, importedPkg := range p.pkg.Imports {
		err := parser.Parse(NewPackageParser(importedPkg))
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
	ast      *ast.File
}

var _ Parsable = &FileParser{}

func NewFileParser(filepath string, ast *ast.File) *FileParser {
	return &FileParser{
		filepath: filepath,
		ast:      ast,
	}
}

func (f *FileParser) Parse(parser *Parser) error {
	fmt.Println("GoFiles:", f.filepath)

	ast.Inspect(f.ast, func(n ast.Node) bool {
		// Check if the node is a function declaration
		if fn, ok := n.(*ast.FuncDecl); ok {
			recv := make([]string, 0)
			if fn.Recv != nil {
				f := fn.Recv.List[0]
				if f.Names != nil {
					n := f.Names[0]
					recv = append(recv, n.Name)
					recv = append(recv, getReceiverType(f.Type))
				}
			}
			fmt.Println(" -", strings.Join(recv, " "), fn.Name.Name, f.filepath)
		}
		return true
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
