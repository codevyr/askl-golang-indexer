package indexing

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/planetA/askl-golang-indexer/pkg/indexpb"
)

func TestExpandPackagePathsGlob(t *testing.T) {
	root := t.TempDir()
	cmdDir := filepath.Join(root, "cmd")
	kubelet := filepath.Join(cmdDir, "kubelet")
	kubectl := filepath.Join(cmdDir, "kubectl")

	for _, dir := range []string{kubelet, kubectl} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create dir %s: %v", dir, err)
		}
	}

	glob := filepath.Join(cmdDir, "*")
	paths, err := expandPackagePaths([]string{glob})
	if err != nil {
		t.Fatalf("expandPackagePaths failed: %v", err)
	}

	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
	}

	expected := map[string]struct{}{
		filepath.Clean(kubelet): {},
		filepath.Clean(kubectl): {},
	}
	for _, path := range paths {
		if _, ok := expected[path]; !ok {
			t.Fatalf("unexpected path %q", path)
		}
	}
}

func TestCommonAncestor(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "cmd", "kubelet")
	right := filepath.Join(root, "cmd", "kubectl")

	for _, dir := range []string{left, right} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create dir %s: %v", dir, err)
		}
	}

	ancestor, err := commonAncestor([]string{left, right})
	if err != nil {
		t.Fatalf("commonAncestor failed: %v", err)
	}

	expected := filepath.Join(root, "cmd")
	if ancestor != expected {
		t.Fatalf("expected %q, got %q", expected, ancestor)
	}
}

func TestGroupPackagePathsByModuleRoot(t *testing.T) {
	repoRoot := repoRoot(t)
	fixtureRoot := filepath.Join(repoRoot, "pkg", "parser", "test", "src", "multi_module")

	modOneCmd := mustAbs(t, filepath.Join(fixtureRoot, "mod-one", "cmd", "tool"))
	modTwoCmd := mustAbs(t, filepath.Join(fixtureRoot, "mod-two", "cmd", "app"))
	modOnePkg := mustAbs(t, filepath.Join(fixtureRoot, "mod-one", "pkg", "foo"))

	byRoot, order, err := groupPackagePathsByModuleRoot([]string{modOneCmd, modTwoCmd, modOnePkg})
	if err != nil {
		t.Fatalf("groupPackagePathsByModuleRoot failed: %v", err)
	}

	expectedRoots := []string{
		mustAbs(t, filepath.Join(fixtureRoot, "mod-one")),
		mustAbs(t, filepath.Join(fixtureRoot, "mod-two")),
	}
	if len(order) != len(expectedRoots) {
		t.Fatalf("expected %d roots, got %d: %v", len(expectedRoots), len(order), order)
	}
	for i, root := range expectedRoots {
		if order[i] != root {
			t.Fatalf("expected root %q at index %d, got %q", root, i, order[i])
		}
	}

	modOneGroup := byRoot[expectedRoots[0]]
	if len(modOneGroup) != 2 {
		t.Fatalf("expected 2 paths for mod-one, got %d: %v", len(modOneGroup), modOneGroup)
	}
	if modOneGroup[0] != modOneCmd || modOneGroup[1] != modOnePkg {
		t.Fatalf("unexpected mod-one group order: %v", modOneGroup)
	}

	modTwoGroup := byRoot[expectedRoots[1]]
	if len(modTwoGroup) != 1 {
		t.Fatalf("expected 1 path for mod-two, got %d: %v", len(modTwoGroup), modTwoGroup)
	}
	if modTwoGroup[0] != modTwoCmd {
		t.Fatalf("unexpected mod-two group: %v", modTwoGroup)
	}

	indexPath := filepath.Join(t.TempDir(), "index.pb")
	opts := Options{
		ProjectName: "test_project",
		IndexPath:   indexPath,
	}

	rootPath := mustAbs(t, fixtureRoot)
	err = ParseModules([]string{modOneCmd, modTwoCmd, modOnePkg}, rootPath, opts)
	if err != nil {
		t.Fatalf("ParseModules failed: %v", err)
	}

	project := readIndexProject(t, indexPath)
	files := filePathSet(project)
	expectedFiles := []string{
		filepath.Join(modOneCmd, "main.go"),
		filepath.Join(modOnePkg, "foo.go"),
		filepath.Join(modTwoCmd, "main.go"),
	}
	for _, expected := range expectedFiles {
		if _, ok := files[expected]; !ok {
			t.Fatalf("expected file %q to be indexed", expected)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	return filepath.Clean(filepath.Join(cwd, "..", ".."))
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs path %q: %v", path, err)
	}
	return absPath
}

func readIndexProject(t *testing.T, indexPath string) *indexpb.Project {
	t.Helper()
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read index file: %v", err)
	}

	project := &indexpb.Project{}
	if err := proto.Unmarshal(data, project); err != nil {
		t.Fatalf("unmarshal index: %v", err)
	}
	return project
}

func filePathSet(project *indexpb.Project) map[string]struct{} {
	paths := make(map[string]struct{})
	for _, file := range project.GetObjects() {
		if file == nil {
			continue
		}
		path := filepath.Clean(file.GetFilesystemPath())
		if path == "" {
			continue
		}
		paths[path] = struct{}{}
	}
	return paths
}

// TestModulePathsAreUniqueAndRelativeToRoot verifies that when indexing multiple
// packages across different modules, the ModulePath values are:
// 1. Unique (no two files should have the same ModulePath)
// 2. Relative to the project's rootPath (not relative to each package's directory)
//
// This test currently FAILS because baseDir is set to pkg.Dir instead of rootPath,
// causing files like mod-one/cmd/tool/main.go and mod-two/cmd/app/main.go to both
// have ModulePath="/main.go" instead of their full paths relative to the root.
func TestModulePathsAreUniqueAndRelativeToRoot(t *testing.T) {
	repoRoot := repoRoot(t)
	fixtureRoot := filepath.Join(repoRoot, "pkg", "parser", "test", "src", "multi_module")

	modOneCmd := mustAbs(t, filepath.Join(fixtureRoot, "mod-one", "cmd", "tool"))
	modTwoCmd := mustAbs(t, filepath.Join(fixtureRoot, "mod-two", "cmd", "app"))

	indexPath := filepath.Join(t.TempDir(), "index.pb")
	opts := Options{
		ProjectName: "test_project",
		IndexPath:   indexPath,
	}

	rootPath := mustAbs(t, fixtureRoot)
	err := ParseModules([]string{modOneCmd, modTwoCmd}, rootPath, opts)
	if err != nil {
		t.Fatalf("ParseModules failed: %v", err)
	}

	project := readIndexProject(t, indexPath)

	// Verify that rootPath is set correctly on the project
	if project.GetRootPath() != rootPath {
		t.Errorf("expected RootPath=%q, got %q", rootPath, project.GetRootPath())
	}

	// Collect all ModulePath values and check for uniqueness
	modulePathCounts := make(map[string][]string) // modulePath -> list of filesystemPaths with that modulePath
	for _, file := range project.GetObjects() {
		if file == nil {
			continue
		}
		modulePath := file.GetModulePath()
		fsPath := file.GetFilesystemPath()
		modulePathCounts[modulePath] = append(modulePathCounts[modulePath], fsPath)
	}

	// Check that no ModulePath is shared by multiple files
	for modulePath, fsPaths := range modulePathCounts {
		if len(fsPaths) > 1 {
			t.Errorf("ModulePath %q is not unique, shared by %d files: %v", modulePath, len(fsPaths), fsPaths)
		}
	}

	// Verify that ModulePath values are relative to rootPath, not to each package's directory.
	// For example, mod-one/cmd/tool/main.go should have ModulePath="/mod-one/cmd/tool/main.go"
	// (relative to rootPath), not ModulePath="/main.go" (relative to pkg.Dir).
	expectedModulePaths := map[string]string{
		filepath.Join(modOneCmd, "main.go"): "/mod-one/cmd/tool/main.go",
		filepath.Join(modTwoCmd, "main.go"): "/mod-two/cmd/app/main.go",
	}

	for _, file := range project.GetObjects() {
		if file == nil {
			continue
		}
		fsPath := file.GetFilesystemPath()
		expectedModulePath, ok := expectedModulePaths[fsPath]
		if !ok {
			continue // skip files not in our expected set (e.g., builtin files)
		}

		actualModulePath := file.GetModulePath()
		if actualModulePath != expectedModulePath {
			t.Errorf("File %q has ModulePath=%q, expected %q (relative to rootPath)",
				fsPath, actualModulePath, expectedModulePath)
		}
	}
}
