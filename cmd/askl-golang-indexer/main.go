package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/urfave/cli/v3"
	"golang.org/x/mod/modfile"

	"github.com/planetA/askl-golang-indexer/pkg/index"
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

type ModuleType int

const (
	ModuleRoot ModuleType = iota
	ModuleDependency
)

func parseModule(flags Flags, packageType ModuleType) error {
	index, err := index.NewIndex(
		index.WithIndexPath(flags.indexPath),
		index.WithRecreate(true),
		index.WithJournal(index.JournalModeOff),
		index.WithSynchronous(index.SynchronousModeOff),
	)
	if err != nil {
		return err
	}
	defer index.Close()

	module, err := getModulePath(flags.packagePath)
	if err != nil {
		return err
	}

	log.Printf("%v", module.Module.Mod.Path)

	parser := NewParser(module.Module.Mod.Path, flags.packagePath, index)
	defer parser.Close()

	err = parser.AddPackages()
	if err != nil {
		return err
	}

	parser.Wait()
	log.Println("Parsing files done")

	err = index.ResolveReferences()
	if err != nil {
		return err
	}

	return nil
}

type Flags struct {
	packagePath string
	packageName string
	indexPath   string
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
