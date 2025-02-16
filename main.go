package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"strings"

	"github.com/urfave/cli/v3"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

func getModulePath(packagePath string) (*modfile.File, error) {
	goModPath := path.Join(packagePath, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("could not read go.mod file: %v", err)
	}

	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, fmt.Errorf("could not parse go.mod file: %v", err)
	}

	return modFile, nil
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

type ModuleType int

const (
	ModuleRoot ModuleType = iota
	ModuleDependency
)

func parseModule(flags Flags, packageType ModuleType) error {
	module, err := getModulePath(flags.packagePath)
	if err != nil {
		return err
	}

	log.Printf("%v", module.Module.Mod.Path)
	modulePath := module.Module.Mod.Path

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.LoadImports | packages.LoadAllSyntax,
		Dir:  flags.packagePath,
		// Dir, Env, or other settings can be specified if needed
	}

	pkgs, err := packages.Load(cfg, modulePath)
	if err != nil {
		return fmt.Errorf("failed to load a package: %w", err)
	}

	parser := NewParser()
	defer parser.Close()

	// pkgs now contains package metadata, ASTs, type info, etc.
	for _, p := range pkgs {
		err := parser.Parse(NewPackageParser(p))
		if err != nil {
			return err
		}
	}

	// // Requirements
	// for _, r := range module.Require {
	// 	fmt.Printf("Require: %s@%s (Indirect=%v)\n", r.Mod.Path, r.Mod.Version, r.Indirect)
	// }

	// if packageType == ModuleRoot {
	// 	// Replacements
	// 	for _, rep := range module.Replace {
	// 		oldPath := rep.Old.Path
	// 		oldVer := rep.Old.Version
	// 		newPath := rep.New.Path
	// 		newVer := rep.New.Version
	// 		fmt.Printf("Replace: %s@%s => %s@%s\n", oldPath, oldVer, newPath, newVer)
	// 		panic("Unimplemented")
	// 	}

	// 	// We ignore excludes, because they only impact modules that use this module
	// }

	parser.Wait()

	return nil
}

type Flags struct {
	packagePath string
	packageName string
}

func main() {
	var flags Flags

	cmd := &cli.Command{
		Name:  "askl-golang-indexer",
		Usage: "Create askl index for a Go package",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "path",
				Value:       ".",
				Usage:       "`PATH` to the Go package",
				Destination: &flags.packagePath,
			},
			&cli.StringFlag{
				Name:        "package",
				Value:       "main",
				Usage:       "What package `NAME` to parse",
				Destination: &flags.packageName,
			},
		},
		Action: func(context.Context, *cli.Command) error {
			err := parseModule(flags, ModuleRoot)
			if err != nil {
				log.Fatalf("Could not get the module path: %v", err)
			}

			module := ""
			visitedModules := make(map[string]bool)
			files, err := parseDirectory(flags.packagePath, module, "", visitedModules)
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
			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}

}
