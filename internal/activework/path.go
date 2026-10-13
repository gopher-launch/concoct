package activework

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Store identifies one repository's external active-work location. Open is
// read-only; it creates no directories or target-repository files.
type Store struct {
	Root       string
	Repository string
	Checkout   string
	CommonDir  string
	dir        string
}

// Open uses $HOME/.concoct unless rootOverride names one explicit location.
// Symlinks are resolved before checking that storage is outside the repository.
func Open(repository, rootOverride string) (*Store, error) {
	checkout, err := canonical(repository)
	if err != nil {
		return nil, fmt.Errorf("repository path: %w", err)
	}
	rootBytes, err := exec.Command("git", "-C", checkout, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, fmt.Errorf("repository path %q is not a Git checkout: %w", checkout, err)
	}
	checkout, err = canonical(strings.TrimSpace(string(rootBytes)))
	if err != nil {
		return nil, fmt.Errorf("repository root: %w", err)
	}
	commonBytes, err := exec.Command("git", "-C", checkout, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return nil, fmt.Errorf("Git common directory: %w", err)
	}
	common, err := canonical(strings.TrimSpace(string(commonBytes)))
	if err != nil {
		return nil, fmt.Errorf("Git common directory: %w", err)
	}
	repo := checkout
	if filepath.Base(common) == ".git" {
		repo = filepath.Dir(common)
	} else {
		// A separate Git directory has no stable checkout path to infer.
		// Bind every worktree to the same canonical common directory.
		repo = common
	}
	root := rootOverride
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home directory: %w", err)
		}
		root = filepath.Join(home, ".concoct")
	}
	root, err = canonical(root)
	if err != nil {
		return nil, fmt.Errorf("active-work root: %w", err)
	}
	if within(repo, root) || within(checkout, root) {
		return nil, fmt.Errorf("active-work root %q is inside target repository checkout", root)
	}
	worktreeBytes, err := exec.Command("git", "-C", checkout, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("Git worktree list: %w", err)
	}
	for _, line := range strings.Split(string(worktreeBytes), "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		worktree, err := canonical(strings.TrimPrefix(line, "worktree "))
		if err != nil {
			return nil, fmt.Errorf("Git worktree path: %w", err)
		}
		if within(worktree, root) {
			return nil, fmt.Errorf("active-work root %q is inside target repository checkout %q", root, worktree)
		}
	}
	digest := sha256.Sum256([]byte(common))
	return &Store{Root: root, Repository: repo, Checkout: checkout, CommonDir: common, dir: filepath.Join(root, "repos", hex.EncodeToString(digest[:16]))}, nil
}

func canonical(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(abs)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		if _, statErr := os.Lstat(abs); statErr == nil {
			return "", fmt.Errorf("unresolvable symlink %q", abs)
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", err
		}
		suffix = append(suffix, filepath.Base(abs))
		abs = parent
	}
}

func within(parent, path string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}
