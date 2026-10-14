package selection

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gopher-launch/concoct/internal/activework"
)

type parentBinding struct {
	Branch string
	Commit string
}

func gitOutput(checkout string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", checkout}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func inspectParent(checkout string) (parentBinding, error) {
	branch, err := gitOutput(checkout, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch == "" {
		return parentBinding{}, fmt.Errorf("prerequisite: current checkout needs a named parent branch")
	}
	commit, err := gitOutput(checkout, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return parentBinding{}, err
	}
	if err := requireClean(checkout); err != nil {
		return parentBinding{}, fmt.Errorf("prerequisite: parent branch %s: %w", branch, err)
	}
	return parentBinding{Branch: branch, Commit: commit}, nil
}

func requireClean(checkout string) error {
	status, err := gitOutput(checkout, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("checkout has changes: %s", status)
	}
	return nil
}

func prepareWorkspace(store *activework.Store, runID, baseline string) (string, error) {
	workspace := filepath.Join(store.Root, "workspaces", runID)
	if _, err := os.Lstat(workspace); err == nil {
		return "", fmt.Errorf("workspace %q already exists; reconcile the active run", workspace)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(workspace), 0700); err != nil {
		return "", err
	}
	if _, err := gitOutput(store.Checkout, "worktree", "add", "--detach", workspace, baseline); err != nil {
		return "", err
	}
	return workspace, nil
}

func verifyParentUnchanged(store *activework.Store, parent parentBinding) error {
	current, err := gitOutput(store.Checkout, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || current != parent.Branch {
		return fmt.Errorf("parent checkout is no longer on branch %s", parent.Branch)
	}
	actual, err := gitOutput(store.Checkout, "rev-parse", "--verify", "refs/heads/"+parent.Branch)
	if err != nil || actual != parent.Commit {
		return fmt.Errorf("parent branch %s moved from %s", parent.Branch, parent.Commit)
	}
	return requireClean(store.Checkout)
}

func verifyClaimWorkspace(store *activework.Store, workspace string, parent parentBinding, taskBranch string) error {
	if err := verifyParentUnchanged(store, parent); err != nil {
		return err
	}
	common, err := gitOutput(workspace, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil || filepath.Clean(common) != store.CommonDir {
		return fmt.Errorf("task checkout does not belong to the bound repository")
	}
	branch, err := gitOutput(workspace, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch != taskBranch {
		return fmt.Errorf("task checkout is not on proposed branch %q", taskBranch)
	}
	commit, err := gitOutput(workspace, "rev-parse", "--verify", "HEAD")
	if err != nil || commit != parent.Commit {
		return fmt.Errorf("task branch does not start at parent baseline %s", parent.Commit)
	}
	if err := requireClean(workspace); err != nil {
		return fmt.Errorf("task checkout changed before claim: %w", err)
	}
	return nil
}

func removeCleanWorkspace(store *activework.Store, workspace string) error {
	if branch, err := gitOutput(workspace, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil && branch != "" {
		return fmt.Errorf("unclaimed selection left branch %q", branch)
	}
	if err := requireClean(workspace); err != nil {
		return err
	}
	_, err := gitOutput(store.Checkout, "worktree", "remove", workspace)
	return err
}
