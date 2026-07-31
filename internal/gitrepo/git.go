package gitrepo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Repository is the deliberately small Git boundary used by workflow code.
type Repository struct{ Root string }

type TaskStart struct{ Trunk, Branch, Base string }

type StatusEntry struct {
	Code  string
	Paths []string
}

func Open(root string) (*Repository, bool, error) {
	c := exec.Command("git", "rev-parse", "--show-toplevel")
	c.Dir = root
	out, err := c.CombinedOutput()
	if err != nil {
		return nil, false, nil
	}
	top, err := filepath.Abs(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, false, err
	}
	want, _ := filepath.Abs(root)
	if top != want {
		return nil, false, fmt.Errorf("Concoct project root %s is nested inside Git repository %s", want, top)
	}
	return &Repository{Root: top}, true, nil
}

func (r *Repository) run(args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Dir = r.Root
	out, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *Repository) Head() (string, error)              { return r.run("rev-parse", "HEAD") }
func (r *Repository) LastCommitSubject() (string, error) { return r.run("log", "-1", "--format=%s") }
func (r *Repository) FileAt(ref, path string) ([]byte, error) {
	c := exec.Command("git", "show", ref+":"+path)
	c.Dir = r.Root
	out, err := c.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w: %s", ref, path, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
func (r *Repository) Branch() (string, error) {
	return r.run("symbolic-ref", "--quiet", "--short", "HEAD")
}
func (r *Repository) Status() (string, error) { return r.run("status", "--short") }
func (r *Repository) StatusEntries() ([]StatusEntry, error) {
	c := exec.Command("git", "status", "--porcelain=v1", "-z")
	c.Dir = r.Root
	out, err := c.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git status --porcelain=v1 -z: %w: %s", err, strings.TrimSpace(string(out)))
	}
	fields := strings.Split(string(out), "\x00")
	entries := make([]StatusEntry, 0, len(fields))
	for i := 0; i < len(fields) && fields[i] != ""; i++ {
		if len(fields[i]) < 4 {
			return nil, fmt.Errorf("unexpected Git status entry %q", fields[i])
		}
		entry := StatusEntry{Code: fields[i][:2], Paths: []string{fields[i][3:]}}
		if entry.Code[0] == 'R' || entry.Code[0] == 'C' {
			i++
			if i >= len(fields) || fields[i] == "" {
				return nil, fmt.Errorf("Git status rename is missing its source path")
			}
			entry.Paths = append(entry.Paths, fields[i])
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
func (r *Repository) Clean() error {
	s, err := r.Status()
	if err != nil {
		return err
	}
	if s != "" {
		return fmt.Errorf("worktree is not clean; commit or stash changes before retrying:\n%s", s)
	}
	return nil
}
func (r *Repository) Ref(name string) (string, error) { return r.run("rev-parse", "--verify", name) }
func (r *Repository) BranchExists(name string) (bool, error) {
	c := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	c.Dir = r.Root
	err := c.Run()
	if err == nil {
		return true, nil
	}
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}
func (r *Repository) Upstream() (string, bool, error) {
	c := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	c.Dir = r.Root
	out, err := c.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "no upstream configured") || strings.Contains(string(out), "no upstream") {
			return "", false, nil
		}
		return "", false, nil
	}
	return strings.TrimSpace(string(out)), true, nil
}
func (r *Repository) Push() error { _, err := r.run("push"); return err }
func (r *Repository) IsAncestor(older, newer string) (bool, error) {
	c := exec.Command("git", "merge-base", "--is-ancestor", older, newer)
	c.Dir = r.Root
	err := c.Run()
	if err == nil {
		return true, nil
	}
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// ChangedPaths returns the paths changed by head since its merge base with base.
// This is the set of paths a squash integration of head is allowed to place in
// the index or worktree.
func (r *Repository) ChangedPaths(base, head string) (map[string]struct{}, error) {
	c := exec.Command("git", "diff", "--name-only", "--no-renames", "-z", base+"..."+head)
	c.Dir = r.Root
	out, err := c.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff transaction paths: %w: %s", err, strings.TrimSpace(string(out)))
	}
	paths := make(map[string]struct{})
	for _, path := range strings.Split(string(out), "\x00") {
		if path != "" {
			paths[path] = struct{}{}
		}
	}
	return paths, nil
}
func (r *Repository) Checkout(name string) error { _, err := r.run("checkout", name); return err }

// CreateTaskBranch validates all planning inputs before changing checkout.
func (r *Repository) CreateTaskBranch(id, title string) (TaskStart, error) {
	if err := r.Clean(); err != nil {
		return TaskStart{}, err
	}
	if r.OperationInProgress() {
		return TaskStart{}, fmt.Errorf("an unrelated Git operation is in progress")
	}
	trunk, err := r.Branch()
	if err != nil {
		return TaskStart{}, fmt.Errorf("detached HEAD is unsafe: %w", err)
	}
	base, err := r.Head()
	if err != nil {
		return TaskStart{}, err
	}
	branch := TaskBranch(id, title)
	exists, err := r.BranchExists(branch)
	if err != nil {
		return TaskStart{}, err
	}
	if exists {
		return TaskStart{}, fmt.Errorf("task branch collision: %s already exists; no checkout was changed", branch)
	}
	if _, err := r.run("checkout", "-b", branch, base); err != nil {
		return TaskStart{}, err
	}
	return TaskStart{Trunk: trunk, Branch: branch, Base: base}, nil
}
func (r *Repository) DeleteBranch(name string) error {
	_, err := r.run("branch", "-D", name)
	return err
}
func (r *Repository) AddAll() error { _, err := r.run("add", "-A"); return err }
func (r *Repository) MergeSquash(commit string) error {
	_, err := r.run("merge", "--squash", commit)
	return err
}
func (r *Repository) Commit(message string) error {
	_, err := r.run("commit", "-m", message)
	return err
}
func (r *Repository) ResetHard(commit string) error {
	_, err := r.run("reset", "--hard", commit)
	return err
}
func (r *Repository) HasUnmerged() (bool, error) {
	s, err := r.run("diff", "--name-only", "--diff-filter=U")
	return s != "", err
}
func (r *Repository) Staged() (bool, error) {
	c := exec.Command("git", "diff", "--cached", "--quiet")
	c.Dir = r.Root
	err := c.Run()
	if err == nil {
		return false, nil
	}
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
		return true, nil
	}
	return false, err
}
func (r *Repository) OperationInProgress() bool {
	gitDir, err := r.run("rev-parse", "--git-dir")
	if err != nil {
		return true
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(r.Root, gitDir)
	}
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(gitDir, name)); err == nil {
			return true
		}
	}
	return false
}

var unsafe = regexp.MustCompile(`[^a-z0-9]+`)

// TaskBranch is portable, deterministic, and bounded for readable refs.
func TaskBranch(id, title string) string {
	s := strings.ToLower(id + "-" + title)
	s = strings.Trim(unsafe.ReplaceAllString(s, "-"), "-")
	if len(s) > 56 {
		s = strings.TrimRight(s[:56], "-")
	}
	return "concoct/" + s
}
