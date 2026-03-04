package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/planetA/askl-golang-indexer/pkg/index"
	"github.com/planetA/askl-golang-indexer/pkg/logging"
)

type gitRepoInfo struct {
	repo     *git.Repository
	worktree *git.Worktree
	root     string
}

func openRepository(repoPath string) (*gitRepoInfo, error) {
	repo, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("open git repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("open git worktree: %w", err)
	}

	root := worktree.Filesystem.Root()
	if root == "" {
		return nil, fmt.Errorf("git worktree root not found")
	}

	root, err = filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve git worktree root: %w", err)
	}

	return &gitRepoInfo{repo: repo, worktree: worktree, root: root}, nil
}

func warnIfDirty(worktree *git.Worktree) error {
	status, err := worktree.Status()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if !status.IsClean() {
		logging.Warn("git worktree has uncommitted changes")
	}
	return nil
}

func listHeadFiles(repo *git.Repository) ([]string, error) {
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD tree: %w", err)
	}

	var files []string
	if err := walkTree(tree, "", &files); err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

func walkTree(tree *object.Tree, prefix string, files *[]string) error {
	for _, entry := range tree.Entries {
		entryPath := entry.Name
		if prefix != "" {
			entryPath = path.Join(prefix, entry.Name)
		}

		switch entry.Mode {
		case filemode.Dir:
			subtree, err := tree.Tree(entry.Name)
			if err != nil {
				return fmt.Errorf("open tree %s: %w", entryPath, err)
			}
			if err := walkTree(subtree, entryPath, files); err != nil {
				return err
			}
		case filemode.Submodule:
			return fmt.Errorf("git submodule detected at %s", entryPath)
		default:
			*files = append(*files, entryPath)
		}
	}
	return nil
}

func addGitTrackedFiles(idx index.Index, repoPath string) error {
	repoInfo, err := openRepository(repoPath)
	if err != nil {
		return err
	}

	if err := warnIfDirty(repoInfo.worktree); err != nil {
		return err
	}

	relativePaths, err := listHeadFiles(repoInfo.repo)
	if err != nil {
		return err
	}

	for _, relPath := range relativePaths {
		absPath := filepath.Join(repoInfo.root, filepath.FromSlash(relPath))
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("tracked file missing on disk: %s: %w", absPath, err)
		}
		if info.IsDir() {
			continue
		}

		contents, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("read tracked file %s: %w", absPath, err)
		}

		filetype := index.GuessFileType(absPath, contents)
		if _, err := idx.AddFile(nil, repoInfo.root, absPath, filetype, contents); err != nil {
			return err
		}
	}

	return nil
}
