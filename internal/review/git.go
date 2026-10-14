package review

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gopher-launch/concoct/internal/activework"
)

func gitBytes(checkout string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", checkout}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func gitOutput(checkout string, args ...string) (string, error) {
	output, err := gitBytes(checkout, args...)
	return strings.TrimSpace(string(output)), err
}

func prepareCheckout(store *activework.Store, snapshot activework.Snapshot, checkout string) error {
	if _, err := os.Lstat(checkout); err == nil {
		return nil // an existing checkout must pass the full identity check
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(checkout), 0700); err != nil {
		return err
	}
	_, err := gitOutput(store.Checkout, "worktree", "add", "--detach", checkout, snapshot.Candidate.Commit)
	return err
}

func verifyCandidate(store *activework.Store, snapshot activework.Snapshot, checkout string, reviewExists bool) error {
	candidate := snapshot.Candidate
	if candidate == nil || candidate.BaselineCommit != snapshot.ParentCommit {
		return fmt.Errorf("frozen candidate binding is absent or changed")
	}
	parentBranch, err := gitOutput(store.Checkout, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || parentBranch != snapshot.ParentBranch {
		return fmt.Errorf("parent checkout left branch %q", snapshot.ParentBranch)
	}
	parent, err := gitOutput(store.Checkout, "rev-parse", "refs/heads/"+snapshot.ParentBranch)
	if err != nil || parent != snapshot.ParentCommit {
		return fmt.Errorf("parent baseline changed before review")
	}
	if status, err := gitOutput(store.Checkout, "status", "--porcelain=v1", "--untracked-files=all"); err != nil {
		return err
	} else if status != "" {
		return fmt.Errorf("parent checkout changed before review: %s", status)
	}
	task, err := gitOutput(store.Checkout, "rev-parse", "refs/heads/"+snapshot.TaskBranch)
	if err != nil || task != candidate.Commit {
		return fmt.Errorf("task branch moved from frozen candidate")
	}
	taskCheckout := filepath.Join(store.Root, "workspaces", snapshot.RunID)
	branch, err := gitOutput(taskCheckout, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch != snapshot.TaskBranch {
		return fmt.Errorf("task checkout left frozen branch")
	}
	head, err := gitOutput(taskCheckout, "rev-parse", "HEAD")
	if err != nil || head != candidate.Commit {
		return fmt.Errorf("task checkout left frozen commit")
	}
	if _, err := gitOutput(taskCheckout, "diff", "--quiet"); err != nil {
		return fmt.Errorf("task checkout source changed after freeze: %w", err)
	}
	if _, err := gitOutput(taskCheckout, "diff", "--cached", "--quiet"); err != nil {
		return fmt.Errorf("task checkout index changed after freeze: %w", err)
	}
	tree, err := gitOutput(store.Checkout, "rev-parse", candidate.Commit+"^{tree}")
	if err != nil || tree != candidate.Tree {
		return fmt.Errorf("candidate tree differs from frozen identity")
	}
	diff, err := gitBytes(store.Checkout, "diff", "--binary", "--no-ext-diff", "--no-renames", snapshot.ParentCommit, candidate.Commit)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(diff)
	if hex.EncodeToString(sum[:]) != candidate.DiffSHA256 {
		return fmt.Errorf("candidate diff digest differs from frozen identity")
	}
	if !reviewExists {
		return nil
	}
	common, err := gitOutput(checkout, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil || filepath.Clean(common) != store.CommonDir {
		return fmt.Errorf("review checkout belongs to another repository")
	}
	if branch, err := gitOutput(checkout, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil || branch != "" {
		return fmt.Errorf("review checkout is not detached")
	}
	head, err = gitOutput(checkout, "rev-parse", "HEAD")
	if err != nil || head != candidate.Commit {
		return fmt.Errorf("review checkout no longer points to candidate")
	}
	if status, err := gitOutput(checkout, "status", "--porcelain=v1", "--untracked-files=all"); err != nil {
		return err
	} else if status != "" {
		return fmt.Errorf("review checkout source changed: %s", status)
	}
	return nil
}
