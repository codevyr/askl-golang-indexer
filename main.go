package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"strings"

	"golang.org/x/mod/modfile"
)

func getModulePath(packagePath string) (string, error) {
	goModPath := path.Join(packagePath, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("could not read go.mod file: %v", err)
	}

	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return "", fmt.Errorf("could not parse go.mod file: %v", err)
	}

	return modFile.Module.Mod.Path, nil
}

type File struct {
	Module string
	Name   string
}

func isSubmodule(module, submodule string) bool {
	_, err := getSubmodule(module, submodule)
	return err == nil
}

func getSubmodule(module, submodule string) (string, error) {
	if module == submodule {
		// Module is its own submodule
		return submodule, nil
	}

	if strings.HasPrefix(submodule, module+"/") {
		return strings.TrimPrefix(submodule, module+"/"), nil
	}

	return "", fmt.Errorf("not a submodule")
}

func parseDirectory(modulePath, module, submodule string, visitedModules map[string]bool) ([]File, error) {
	importPath := path.Join(modulePath, submodule)

	fset := token.NewFileSet() // positions are relative to fset
	pkgs, err := parser.ParseDir(fset, importPath, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	var files []File
	newImports := make(map[string]bool)
	for pkgName, pkg := range pkgs {
		fmt.Println("Package:", pkgName)
		for fileName, file := range pkg.Files {
			fmt.Println("Processing file:", fileName)
			newFile := File{
				Module: module,
				Name:   fileName,
			}
			files = append(files, newFile)
			ast.Inspect(file, func(n ast.Node) bool {
				importSpec, ok := n.(*ast.ImportSpec)
				if ok {
					importPath := strings.TrimSuffix(strings.TrimPrefix(importSpec.Path.Value, "\""), "\"")
					if isSubmodule(module, importPath) {
						newImports[importPath] = true
					}
				}
				return true
			})
		}
	}

	for newImport := range newImports {
		submodule, err := getSubmodule(module, newImport)
		if err != nil {
			return nil, err
		}
		if visitedModules[newImport] {
			continue
		}

		visitedModules[newImport] = true
		newFiles, err := parseDirectory(modulePath, module, submodule, visitedModules)
		if err != nil {
			return nil, fmt.Errorf("failed to parse directory %s (%s): %v", importPath, newImport, err)
		}

		files = append(files, newFiles...)
	}

	return files, nil
}

func main() {
	packagePath := flag.String("path", ".", "Path to the Go package")
	flag.Parse()

	module, err := getModulePath(*packagePath)
	if err != nil {
		log.Fatalf("Could not get the module path: %v", err)
	}

	visitedModules := make(map[string]bool)
	files, err := parseDirectory(*packagePath, module, "", visitedModules)
	if err != nil {
		log.Fatalf("failed to parse directory: %v", err)
	}

	count := 0
	fm := make(map[string]int)
	for _, f := range files {
		fm[f.Name] = fm[f.Name] + 1
		count = count + 1
		fmt.Println(f.Name)
	}
	// Output results
	fmt.Printf("Analysis complete\n")
	fmt.Println(count)
}
