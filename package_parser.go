package main

import (
	"fmt"
	"log"
	"sync"

	"golang.org/x/tools/go/packages"
)

type PackageParser struct {
	parsedPackaged map[string]bool
	packageChannel chan *packages.Package
	wg             sync.WaitGroup
}

func NewPackageParser() *PackageParser {
	c := make(chan *packages.Package)

	p := &PackageParser{
		parsedPackaged: make(map[string]bool),
		packageChannel: c,
	}

	go p.loop()

	return p
}

func (p *PackageParser) Wait() {
	p.wg.Wait()
}

func (p *PackageParser) Close() {
	close(p.packageChannel)
}

func (p *PackageParser) Parse(pkg *packages.Package) error {

	p.wg.Add(1)
	p.packageChannel <- pkg

	return nil
}

func (p *PackageParser) doParse(pkg *packages.Package) error {
	defer p.wg.Done()

	if _, ok := p.parsedPackaged[pkg.ID]; ok {
		return nil
	}

	p.parsedPackaged[pkg.ID] = true
	fmt.Println("Package Name:", pkg.Name)
	for _, file := range pkg.GoFiles {
		err := parseFile(pkg, file)
		if err != nil {
			return err
		}
	}

	p.wg.Add(1)
	go p.parseImports(pkg)
	return nil
}

func (p *PackageParser) loop() {
	for pkg := range p.packageChannel {
		err := p.doParse(pkg)
		if err != nil {
			log.Fatalf("failed to parse package: %v", err)
		}

	}
}

func (p *PackageParser) parseImports(pkg *packages.Package) error {
	defer p.wg.Done()

	for _, importedPkg := range pkg.Imports {
		err := p.Parse(importedPkg)
		if err != nil {
			return err
		}
	}

	return nil
}
