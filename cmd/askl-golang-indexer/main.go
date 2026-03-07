package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/planetA/askl-golang-indexer/pkg/indexing"
	"github.com/planetA/askl-golang-indexer/pkg/logging"
)

type Flags struct {
	packagePaths    []string
	packageName     string
	indexPath       string
	projectName     string
	continueOnError bool
	parseTypes      bool
	includeGitFiles bool
	logLevel        string
	rootPath        string
}

func main() {
	var flags Flags

	if err := logging.Configure("error"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

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
			&cli.StringSliceFlag{
				Name:        "path",
				Usage:       "`PATH` to a Go package (repeatable, glob supported)",
				Destination: &flags.packagePaths,
			},
			&cli.StringFlag{
				Name:        "root",
				Usage:       "`PATH` to use as the project root",
				Destination: &flags.rootPath,
			},
			&cli.StringFlag{
				Name:        "index",
				Value:       "index.pb",
				Usage:       "`INDEX` file where to store the resulting protobuf index",
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
			&cli.BoolFlag{
				Name:        "include-git-files",
				Value:       false,
				Usage:       "Include all git-tracked files at HEAD in the project files list",
				Destination: &flags.includeGitFiles,
			},
			&cli.StringFlag{
				Name:        "log-level",
				Value:       "error",
				Usage:       "Logging level (`debug`, `info`, `warn`, `error`)",
				Destination: &flags.logLevel,
			},
		},
		Action: func(_ context.Context, command *cli.Command) error {
			if err := logging.Configure(flags.logLevel); err != nil {
				return err
			}

			modulePaths, err := indexing.ResolvePackagePaths(flags.packagePaths, command.Args().Slice())
			if err != nil {
				return err
			}

			rootPath, err := indexing.ResolveRootPath(flags.rootPath, modulePaths)
			if err != nil {
				return err
			}

			opts := indexing.Options{
				ProjectName:     flags.projectName,
				IndexPath:       flags.indexPath,
				ContinueOnError: flags.continueOnError,
				ParseTypes:      flags.parseTypes,
				IncludeGitFiles: flags.includeGitFiles,
			}
			err = indexing.ParseModules(modulePaths, rootPath, opts)
			if err != nil {
				logging.Fatalf("Indexing failed: %v", err)
			}
			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		logging.Fatal(err)
	}
}
