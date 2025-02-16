package main

import (
	"fmt"
	"go/ast"
	"log"
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

	return nil
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
