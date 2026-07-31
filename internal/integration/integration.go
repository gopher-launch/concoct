package integration

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gopher-launch/concoct/internal/gitrepo"
	"github.com/gopher-launch/concoct/internal/workflow"
	"gopkg.in/yaml.v3"
)

type recovery struct {
	TaskID             string `yaml:"task-id"`
	Trunk              string `yaml:"trunk"`
	TaskBranch         string `yaml:"task-branch"`
	ArchiveCommit      string `yaml:"archive-commit"`
	PreIntegrationHead string `yaml:"pre-integration-head"`
	IntegrationCommit  string `yaml:"integration-commit,omitempty"`
	DeliveryCommit     string `yaml:"delivery-commit,omitempty"`
	Phase              string `yaml:"phase"`
}

// Run performs an integration transaction. Input is used only for the default
// push confirmation when the recorded trunk has a matching upstream.
func Run(root, mode string, input io.Reader, output io.Writer) error {
	g, ok, err := gitrepo.Open(root)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("integration requires a Git-backed task")
	}
	if mode == "continue" || mode == "abort" {
		path, r, err := findRecovery(root)
		if err != nil {
			return err
		}
		if mode == "abort" {
			return abort(g, r, path)
		}
		return resume(g, r, path, input, output)
	}
	if mode != "" {
		return fmt.Errorf("unknown integration mode %q", mode)
	}
	c, err := workflow.InspectGitContext(root)
	if err != nil {
		return err
	}
	if !c.Enabled || !c.Archived || c.ArchiveCommit == "" {
		return fmt.Errorf("integration requires enabled git metadata with status archived and archive-commit")
	}
	path := filepath.Join(root, ".git", "concoct", "integrations", c.ID+".yaml")
	return start(g, c, path, input, output)
}

func start(g *gitrepo.Repository, c workflow.GitContext, path string, input io.Reader, output io.Writer) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("integration recovery already exists; use --continue or --abort")
	}
	if err := g.Clean(); err != nil {
		return err
	}
	if g.OperationInProgress() {
		return fmt.Errorf("an unrelated Git operation is in progress")
	}
	branch, err := g.Branch()
	if err != nil {
		return fmt.Errorf("detached HEAD is unsafe: %w", err)
	}
	if branch != c.TaskBranch {
		return fmt.Errorf("checkout drift: expected task branch %s, found %s", c.TaskBranch, branch)
	}
	head, err := g.Head()
	if err != nil {
		return err
	}
	ancestor, err := g.IsAncestor(c.ArchiveCommit, head)
	if err != nil || !ancestor {
		return fmt.Errorf("archive-commit must be present on the checked-out task branch")
	}
	pre, err := g.Ref(c.Trunk)
	if err != nil {
		return fmt.Errorf("recorded trunk %s is unavailable: %w", c.Trunk, err)
	}
	r := recovery{TaskID: c.ID, Trunk: c.Trunk, TaskBranch: c.TaskBranch, ArchiveCommit: c.ArchiveCommit, PreIntegrationHead: pre, Phase: "prepared"}
	if err := writeRecovery(path, r, true); err != nil {
		return err
	}
	if err := g.Checkout(c.Trunk); err != nil {
		return err
	}
	if err := g.MergeSquash(c.ArchiveCommit); err != nil {
		return fmt.Errorf("squash integration conflicted; recovery evidence preserved; resolve and stage conflicts then use --continue, or use --abort: %w", err)
	}
	r.Phase = "squashed"
	if err := writeRecovery(path, r, false); err != nil {
		return err
	}
	return finish(g, &r, path, input, output)
}

func resume(g *gitrepo.Repository, r recovery, path string, input io.Reader, output io.Writer) error {
	branch, err := g.Branch()
	if err != nil {
		return fmt.Errorf("detached HEAD is unsafe: %w", err)
	}
	if r.Phase == "prepared" && branch == r.TaskBranch {
		if err := validatePreparedTaskBranch(g, r); err != nil {
			return err
		}
		if err := g.Checkout(r.Trunk); err != nil {
			return err
		}
		branch = r.Trunk
	}
	if branch != r.Trunk {
		return fmt.Errorf("checkout drift: --continue requires recorded trunk %s", r.Trunk)
	}
	if err := validateTrunkHead(g, r); err != nil {
		return err
	}
	if r.Phase == "integrated" || r.Phase == "delivered" {
		if g.OperationInProgress() {
			return fmt.Errorf("an unrelated Git operation is in progress")
		}
		if err := g.Clean(); err != nil {
			return fmt.Errorf("recovery has unexpected worktree changes: %w", err)
		}
	}
	if r.Phase == "prepared" {
		unmerged, err := g.HasUnmerged()
		if err != nil {
			return err
		}
		if unmerged {
			return fmt.Errorf("unresolved conflicts remain")
		}
		staged, err := g.Staged()
		if err != nil {
			return err
		}
		if !staged {
			if err := g.Clean(); err != nil {
				return fmt.Errorf("prepared recovery has unexpected worktree changes: %w", err)
			}
			if g.OperationInProgress() {
				return fmt.Errorf("an unrelated Git operation is in progress")
			}
			if err := g.MergeSquash(r.ArchiveCommit); err != nil {
				return fmt.Errorf("squash integration conflicted; recovery evidence preserved; resolve and stage conflicts then use --continue, or use --abort: %w", err)
			}
		}
		if err := validateStagedRecovery(g); err != nil {
			return err
		}
		r.Phase = "squashed"
		if err := writeRecovery(path, r, false); err != nil {
			return err
		}
	}
	return finish(g, &r, path, input, output)
}

func finish(g *gitrepo.Repository, r *recovery, path string, input io.Reader, output io.Writer) error {
	if err := validateTrunkHead(g, *r); err != nil {
		return err
	}
	if r.IntegrationCommit == "" {
		if err := validateStagedRecovery(g); err != nil {
			return err
		}
		staged, err := g.Staged()
		if err != nil {
			return err
		}
		if !staged {
			return fmt.Errorf("integration result is not staged; use --abort or restore the staged squash result")
		}
		if err := g.Commit("concoct: integrate " + r.TaskID); err != nil {
			return err
		}
		r.IntegrationCommit, err = g.Head()
		if err != nil {
			return err
		}
		r.Phase = "integrated"
		if err := writeRecovery(path, *r, false); err != nil {
			return err
		}
	}
	if r.DeliveryCommit == "" {
		if err := reconcile(g.Root, r.TaskID); err != nil {
			return fmt.Errorf("integration commit created but final bookkeeping failed; recovery preserved: %w", err)
		}
		if err := g.AddAll(); err != nil {
			return err
		}
		staged, err := g.Staged()
		if err != nil {
			return err
		}
		if staged {
			if err := g.Commit("concoct: deliver " + r.TaskID); err != nil {
				return fmt.Errorf("commit final bookkeeping: %w", err)
			}
		}
		r.DeliveryCommit, err = g.Head()
		if err != nil {
			return err
		}
		r.Phase = "delivered"
		if err := writeRecovery(path, *r, false); err != nil {
			return err
		}
	}
	if err := maybePush(g, r.Trunk, input, output); err != nil {
		return err
	}
	exists, err := g.BranchExists(r.TaskBranch)
	if err != nil {
		return err
	}
	if exists {
		if err := g.DeleteBranch(r.TaskBranch); err != nil {
			return fmt.Errorf("delivery committed but task branch cleanup failed: %w", err)
		}
	}
	if err := g.Clean(); err != nil {
		return fmt.Errorf("delivery cleanup did not leave a clean trunk: %w", err)
	}
	return os.Remove(path)
}

func abort(g *gitrepo.Repository, r recovery, path string) error {
	branch, err := g.Branch()
	if err != nil {
		return fmt.Errorf("detached HEAD is unsafe: %w", err)
	}
	if r.Phase == "prepared" && branch == r.TaskBranch {
		if err := validatePreparedTaskBranch(g, r); err != nil {
			return err
		}
		return os.Remove(path)
	}
	if branch != r.Trunk {
		return fmt.Errorf("checkout drift: --abort requires recorded trunk %s", r.Trunk)
	}
	if err := validateTrunkHead(g, r); err != nil {
		return err
	}
	if err := validateAbortWorktree(g, r); err != nil {
		return err
	}
	if err := g.ResetHard(r.PreIntegrationHead); err != nil {
		return err
	}
	if err := g.Checkout(r.TaskBranch); err != nil {
		return err
	}
	return os.Remove(path)
}

func validatePreparedTaskBranch(g *gitrepo.Repository, r recovery) error {
	if g.OperationInProgress() {
		return fmt.Errorf("an unrelated Git operation is in progress")
	}
	if err := g.Clean(); err != nil {
		return fmt.Errorf("prepared recovery on task branch is unsafe: %w", err)
	}
	head, err := g.Head()
	if err != nil {
		return err
	}
	ref, err := g.Ref(r.TaskBranch)
	if err != nil || head != ref {
		return fmt.Errorf("prepared recovery task branch HEAD changed unexpectedly")
	}
	ancestor, err := g.IsAncestor(r.ArchiveCommit, head)
	if err != nil || !ancestor {
		return fmt.Errorf("prepared recovery archive commit is not present on the task branch")
	}
	trunk, err := g.Ref(r.Trunk)
	if err != nil || trunk != r.PreIntegrationHead {
		return fmt.Errorf("recorded trunk advanced after integration recovery was prepared; refusing to overwrite unrelated work")
	}
	return nil
}

func validateTrunkHead(g *gitrepo.Repository, r recovery) error {
	want := r.PreIntegrationHead
	if r.DeliveryCommit != "" || r.Phase == "delivered" {
		want = r.DeliveryCommit
	} else if r.IntegrationCommit != "" || r.Phase == "integrated" {
		want = r.IntegrationCommit
	}
	if want == "" {
		return fmt.Errorf("recovery phase %s is missing its recorded commit", r.Phase)
	}
	head, err := g.Head()
	if err != nil {
		return err
	}
	if head != want {
		return fmt.Errorf("recorded trunk HEAD changed unexpectedly: expected %s, found %s; refusing to overwrite unrelated work", want, head)
	}
	return nil
}

func validateAbortWorktree(g *gitrepo.Repository, r recovery) error {
	if r.Phase == "integrated" || r.Phase == "delivered" {
		if g.OperationInProgress() {
			return fmt.Errorf("an unrelated Git operation is in progress")
		}
		if err := g.Clean(); err != nil {
			return fmt.Errorf("--abort refuses to discard worktree changes: %w", err)
		}
		return nil
	}
	status, err := g.Status()
	if err != nil {
		return err
	}
	entries, err := g.StatusEntries()
	if err != nil {
		return err
	}
	transactionPaths, err := g.ChangedPaths(r.PreIntegrationHead, r.ArchiveCommit)
	if err != nil {
		return fmt.Errorf("determine transaction-owned integration paths: %w", err)
	}
	for _, entry := range entries {
		transactionOwned := true
		for _, path := range entry.Paths {
			if _, ok := transactionPaths[path]; !ok {
				transactionOwned = false
			}
		}
		if entry.Code == "??" || !transactionOwned || (entry.Code[1] != ' ' && !isUnmergedStatus(entry.Code)) {
			return fmt.Errorf("--abort refuses unexpected worktree changes; preserve or remove them before retrying:\n%s", status)
		}
	}
	return nil
}

func validateStagedRecovery(g *gitrepo.Repository) error {
	status, err := g.Status()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "?? ") || len(line) < 2 || line[1] != ' ' {
			return fmt.Errorf("integration recovery contains unexpected unstaged, unmerged, or untracked changes:\n%s", status)
		}
	}
	return nil
}

func isUnmergedStatus(status string) bool {
	switch status {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return true
	default:
		return false
	}
}

func maybePush(g *gitrepo.Repository, trunk string, input io.Reader, output io.Writer) error {
	upstream, ok, err := g.Upstream()
	if err != nil || !ok {
		return err
	}
	if !strings.HasSuffix(upstream, "/"+trunk) {
		return nil
	}
	auto, err := autoPush(g.Root)
	if err != nil {
		return err
	}
	if !auto {
		fmt.Fprintf(output, "Push integrated trunk %s to %s? [y/N] ", trunk, upstream)
		line, _ := bufio.NewReader(input).ReadString('\n')
		if strings.ToLower(strings.TrimSpace(line)) != "y" && strings.ToLower(strings.TrimSpace(line)) != "yes" {
			return nil
		}
	}
	return g.Push()
}

func autoPush(root string) (bool, error) {
	b, err := os.ReadFile(filepath.Join(root, ".concoct", "config.yaml"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var c struct {
		Git struct {
			AutoPush bool `yaml:"auto-push"`
		} `yaml:"git"`
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return false, fmt.Errorf("parse .concoct/config.yaml: %w", err)
	}
	return c.Git.AutoPush, nil
}

func findRecovery(root string) (string, recovery, error) {
	paths, err := filepath.Glob(filepath.Join(root, ".git", "concoct", "integrations", "*.yaml"))
	if err != nil {
		return "", recovery{}, err
	}
	if len(paths) != 1 {
		return "", recovery{}, fmt.Errorf("expected exactly one integration recovery record, found %d", len(paths))
	}
	r, err := readRecovery(paths[0])
	return paths[0], r, err
}
func writeRecovery(path string, r recovery, exclusive bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	flag := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if exclusive {
		flag |= os.O_EXCL
	}
	f, err := os.OpenFile(path, flag, 0o600)
	if err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
func readRecovery(path string) (recovery, error) {
	var r recovery
	b, err := os.ReadFile(path)
	if err == nil {
		err = yaml.Unmarshal(b, &r)
	}
	return r, err
}

func reconcile(root, id string) error {
	road := filepath.Join(root, ".concoct", "roadmap.md")
	b, err := os.ReadFile(road)
	if err != nil {
		return err
	}
	heading := regexp.MustCompile(`(?m)^## ` + regexp.QuoteMeta(id) + `\s+—`)
	loc := heading.FindIndex(b)
	if loc == nil {
		return fmt.Errorf("roadmap item %s missing", id)
	}
	end := len(b)
	if next := regexp.MustCompile(`(?m)^## [A-Z][A-Z0-9-]*-[0-9]+\s+—`).FindIndex(b[loc[1]:]); next != nil {
		end = loc[1] + next[0]
	}
	section := string(b[loc[0]:end])
	if strings.Contains(section, "- Status: `active`") {
		updated := strings.Replace(section, "- Status: `active`", "- Status: `delivered`", 1)
		b = append(append([]byte{}, b[:loc[0]]...), append([]byte(updated), b[end:]...)...)
		if err := os.WriteFile(road, b, 0o644); err != nil {
			return err
		}
	} else if !strings.Contains(section, "- Status: `delivered`") {
		return fmt.Errorf("roadmap item %s is neither active nor delivered", id)
	}
	cur := filepath.Join(root, ".concoct", "current")
	entries, err := os.ReadDir(cur)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() != ".gitkeep" && e.Name() != "bootstrap-prompt.md" {
			if err := os.RemoveAll(filepath.Join(cur, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
