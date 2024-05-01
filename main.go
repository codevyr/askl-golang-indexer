package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
)

func main() {
	packagePath := flag.String("path", ".", "Path to the Go package")
	flag.Parse()

	fset := token.NewFileSet() // positions are relative to fset

	pkgs, err := parser.ParseDir(fset, *packagePath, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	for pkgName, pkg := range pkgs {
		fmt.Println("Package:", pkgName)
		for fileName, file := range pkg.Files {
			fmt.Println("Processing file:", fileName)
			ast.Inspect(file, func(n ast.Node) bool {
				// Your AST inspection logic here
				return true
			})
		}
	}

	// Output results
	fmt.Println("Analysis complete")
}
