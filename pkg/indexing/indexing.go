package indexing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/logging"
	"github.com/planetA/askl-golang-indexer/pkg/parser"
)

type Options struct {
	ProjectName     string
	IndexPath       string
	ContinueOnError bool
	ParseTypes      bool
	IncludeGitFiles bool
}

func ResolvePackagePaths(flagPaths []string, args []string) ([]string, error) {
	inputs := append([]string{}, flagPaths...)
	inputs = append(inputs, args...)
	if len(inputs) == 0 {
		inputs = []string{"."}
	}

	return expandPackagePaths(inputs)
}

func ResolveRootPath(rootFlag string, modulePaths []string) (string, error) {
	if rootFlag != "" {
		absRoot, err := filepath.Abs(rootFlag)
		if err != nil {
			return "", fmt.Errorf("resolve root path: %w", err)
		}
		info, err := os.Stat(absRoot)
		if err != nil {
			return "", fmt.Errorf("stat root path %q: %w", absRoot, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("root path %q is not a directory", absRoot)
		}
		return filepath.Clean(absRoot), nil
	}

	return commonAncestor(modulePaths)
}

func ParseModules(modulePaths []string, rootPath string, opts Options) error {
	idx, err := index.NewProtoIndex(
		index.WithProject(opts.ProjectName),
		index.WithRootPath(rootPath),
	)
	if err != nil {
		return err
	}
	defer idx.Close()

	byRoot, order, err := groupPackagePathsByModuleRoot(modulePaths)
	if err != nil {
		return err
	}

	for _, moduleRoot := range order {
		groupPaths := byRoot[moduleRoot]
		for _, modulePath := range groupPaths {
			module, err := getModulePath(modulePath)
			if err != nil {
				return err
			}

			logging.Infof("Module path: %v Package path %v", module.Module.Mod.Path, modulePath)
		}

		pkgParser := parser.NewParserWithPaths(groupPaths, idx,
			parser.WithContinueOnError(opts.ContinueOnError),
			parser.WithParseTypes(opts.ParseTypes),
		)

		err = pkgParser.Load()
		if err != nil {
			pkgParser.Close()
			return fmt.Errorf("failed to load parser: %w", err)
		}

		err = pkgParser.AddPackages()
		pkgParser.Close()
		if err != nil {
			return err
		}
	}

	if opts.IncludeGitFiles {
		if err := addGitTrackedFiles(idx, rootPath); err != nil {
			return err
		}
	}

	logging.Info("Parsing files done")

	err = idx.ResolveReferences()
	if err != nil {
		return err
	}

	err = idx.Wait()
	if err != nil {
		return err
	}

	payload, err := idx.Marshal()
	if err != nil {
		return err
	}

	err = os.WriteFile(opts.IndexPath, payload, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write index to %s: %w", opts.IndexPath, err)
	}

	return nil
}

func expandPackagePaths(inputs []string) ([]string, error) {
	resolved := []string{}
	seen := make(map[string]struct{})

	for _, input := range inputs {
		if hasGlobMeta(input) {
			matches, err := filepath.Glob(input)
			if err != nil {
				return nil, fmt.Errorf("invalid glob %q: %w", input, err)
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("glob %q matched no paths", input)
			}
			added := false
			for _, match := range matches {
				absMatch, err := filepath.Abs(match)
				if err != nil {
					return nil, fmt.Errorf("resolve path %q: %w", match, err)
				}
				info, err := os.Stat(absMatch)
				if err != nil {
					return nil, fmt.Errorf("stat path %q: %w", absMatch, err)
				}
				if !info.IsDir() {
					continue
				}
				if err := addPackageDir(absMatch, &resolved, seen); err != nil {
					return nil, err
				}
				added = true
			}
			if !added {
				return nil, fmt.Errorf("glob %q matched no directories", input)
			}
			continue
		}

		if err := addPackagePath(input, &resolved, seen); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}

func addPackagePath(input string, resolved *[]string, seen map[string]struct{}) error {
	absPath, err := filepath.Abs(input)
	if err != nil {
		return fmt.Errorf("resolve path %q: %w", input, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat path %q: %w", absPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", absPath)
	}

	return addPackageDir(absPath, resolved, seen)
}

func addPackageDir(absPath string, resolved *[]string, seen map[string]struct{}) error {
	absPath = filepath.Clean(absPath)
	if _, ok := seen[absPath]; ok {
		return nil
	}

	seen[absPath] = struct{}{}
	*resolved = append(*resolved, absPath)
	return nil
}

func hasGlobMeta(value string) bool {
	return strings.ContainsAny(value, "*?[")
}

func getModulePath(packagePath string) (*modfile.File, error) {
	goModPath, err := findGoModPath(packagePath)
	if err != nil {
		return nil, err
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

func findGoModPath(packagePath string) (string, error) {
	curPath := packagePath
	for {
		goModPath := filepath.Join(curPath, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("failed to open mod file: %w", err)
		}

		parent := filepath.Dir(curPath)
		if parent == curPath {
			return "", fmt.Errorf("could not find mod path neither in %v, nor in parent directories", packagePath)
		}
		curPath = parent
	}
}

func groupPackagePathsByModuleRoot(modulePaths []string) (map[string][]string, []string, error) {
	byRoot := make(map[string][]string)
	order := []string{}

	for _, modulePath := range modulePaths {
		goModPath, err := findGoModPath(modulePath)
		if err != nil {
			return nil, nil, err
		}
		moduleRoot := filepath.Dir(goModPath)
		if _, ok := byRoot[moduleRoot]; !ok {
			order = append(order, moduleRoot)
		}
		byRoot[moduleRoot] = append(byRoot[moduleRoot], modulePath)
	}

	return byRoot, order, nil
}

func commonAncestor(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("no module paths provided")
	}

	ancestor := paths[0]
	for _, candidate := range paths[1:] {
		for !isWithinDir(candidate, ancestor) {
			parent := filepath.Dir(ancestor)
			if parent == ancestor {
				return "", fmt.Errorf("no common ancestor for %q and %q", ancestor, candidate)
			}
			ancestor = parent
		}
	}

	return ancestor, nil
}

func isWithinDir(candidate, dir string) bool {
	rel, err := filepath.Rel(dir, candidate)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != ".."
}
