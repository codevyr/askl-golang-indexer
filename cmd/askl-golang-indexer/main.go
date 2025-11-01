package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/urfave/cli/v3"
	"golang.org/x/mod/modfile"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/parser"
)

func getModulePath(packagePath string) (*modfile.File, error) {
	curPath := packagePath
	var goModPath string
	for {
		goModPath = path.Join(curPath, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			break
		} else if curPath == "/" && errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("could not find mod path neither in %v, nor in parent directories", packagePath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to open mod file: %w", err)
		}

		curPath = path.Dir(curPath)
	}

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

type ModuleType int

const (
	ModuleRoot ModuleType = iota
	ModuleDependency
)

func parseModule(flags Flags, packageType ModuleType) error {
	index, err := index.NewSqlIndex(
		index.WithIndexPath(flags.indexPath),
		index.WithRecreate(true),
		index.WithJournal(index.JournalModeOff),
		index.WithSynchronous(index.SynchronousModeOff),
		index.WithProject(flags.projectName),
	)
	if err != nil {
		return err
	}
	defer index.Close()

	module, err := getModulePath(flags.packagePath)
	if err != nil {
		return err
	}

	log.Printf("Module path: %v Package path %v", module.Module.Mod.Path, flags.packagePath)

	parser := parser.NewParser(flags.packagePath, index,
		parser.WithContinueOnError(flags.continueOnError),
		parser.WithParseTypes(flags.parseTypes),
	)
	defer parser.Close()

	err = parser.Load()
	if err != nil {
		return fmt.Errorf("failed to load parser: %w", err)
	}

	err = parser.AddPackages()
	if err != nil {
		return err
	}

	log.Println("Parsing files done")

	err = index.ResolveReferences()
	if err != nil {
		return err
	}

	return nil
}

type Flags struct {
	packagePath     string
	packageName     string
	indexPath       string
	projectName     string
	continueOnError bool
	parseTypes      bool
}

func main() {
	var flags Flags

	cmd := &cli.Command{
		Name:  "askl-golang-indexer",
		Usage: "Create askl index for a Go package",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "project",
				Value:       "main",
				Usage:       "`NAME` of the project",
				Destination: &flags.projectName,
			},
			&cli.StringFlag{
				Name:        "path",
				Value:       ".",
				Usage:       "`PATH` to the Go package",
				Destination: &flags.packagePath,
			},
			&cli.StringFlag{
				Name:        "index",
				Value:       "index.db",
				Usage:       "`INDEX` where to store the resulting index",
				Destination: &flags.indexPath,
			},
			&cli.StringFlag{
				Name:        "package",
				Value:       "main",
				Usage:       "What package `NAME` to parse",
				Destination: &flags.packageName,
			},
			&cli.BoolFlag{
				Name:        "continue-on-error",
				Value:       false,
				Usage:       "Continue parsing if an error is encountered",
				Destination: &flags.continueOnError,
			},
			&cli.BoolFlag{
				Name:        "parse-types",
				Value:       false,
				Usage:       "Parse type information",
				Destination: &flags.parseTypes,
			},
		},
		Action: func(context.Context, *cli.Command) error {
			err := parseModule(flags, ModuleRoot)
			if err != nil {
				log.Fatalf("Could not get the module path: %v", err)
			}
			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}

}
