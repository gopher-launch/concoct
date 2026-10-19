package activework

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CompletionResult distinguishes a verified integration from unfinished
// archive or workspace maintenance. Completion never invokes a provider.
type CompletionResult struct {
	Status, Archive string
	Facts           Completion
}

// Complete finalizes the integrated active run under the repository lock.
// A retry after the durable completion marker only resumes maintenance.
func (s *Store) Complete(after func(string) error) (CompletionResult, error) {
	owner, err := s.Acquire()
	if err != nil {
		return CompletionResult{}, err
	}
	defer owner.Close()
	owner.hook = after
	return owner.complete()
}

func (o *Owner) complete() (CompletionResult, error) {
	st, err := o.currentState()
	if err != nil {
		return CompletionResult{}, err
	}
	if st.TaskID == "" || st.Acceptance == nil || st.Integration == nil {
		return CompletionResult{Status: "not_integrated"}, fmt.Errorf("no accepted integration to complete")
	}
	if st.Completion == nil {
		if err := verifyIntegratedCheckout(o.store.Checkout, st); err != nil {
			return CompletionResult{Status: "not_integrated"}, err
		}
		if st.Integration.Status == "intent" {
			if err := o.MarkIntegrated(st.Acceptance.ID); err != nil {
				return CompletionResult{Status: "maintenance_pending"}, err
			}
			st, err = o.currentState()
			if err != nil {
				return CompletionResult{Status: "maintenance_pending"}, err
			}
		}
		if st.Hold != "" || st.Stop != nil || st.Question != nil || st.Dispatch == nil || st.Dispatch.Pending != nil {
			return CompletionResult{Status: "not_integrated"}, fmt.Errorf("integration has unresolved execution state")
		}
		for _, attempt := range st.Dispatch.Attempts {
			if attempt.Status == "reserved" || attempt.Status == "uncertain" {
				return CompletionResult{Status: "not_integrated"}, fmt.Errorf("integration has unresolved dispatch")
			}
		}
		st.Completion = &Completion{RunID: st.RunID, TaskID: st.TaskID, Candidate: st.Acceptance.Candidate, CandidateTree: st.Acceptance.CandidateTree, TargetBranch: st.ParentBranch, At: time.Now().UTC()}
		st.Revision++
		if err := writeJSON(o.store.statePath(st.RunID), st); err != nil {
			return CompletionResult{Status: "maintenance_pending"}, err
		}
	}
	result := CompletionResult{Status: "maintenance_pending", Facts: *st.Completion, Archive: filepath.Join(o.store.dir, "archive", st.TaskID, st.RunID)}
	if err := o.after("completion-recorded"); err != nil {
		return result, err
	}
	if err := o.archiveCompletion(st, result.Archive); err != nil {
		return result, err
	}
	if err := o.after("completion-archived"); err != nil {
		return result, err
	}
	for _, suffix := range []string{"-review", ""} {
		if err := o.removeCompletionWorkspace(st, suffix); err != nil {
			return result, err
		}
	}
	if err := o.after("completion-cleaned"); err != nil {
		return result, err
	}
	if err := removeSync(filepath.Join(o.store.dir, "active.json")); err != nil {
		return result, err
	}
	result.Status = "completed"
	return result, nil
}

func verifyIntegratedCheckout(checkout string, st state) error {
	a := st.Acceptance
	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"symbolic-ref", "--quiet", "--short", "HEAD"}, a.TargetBranch},
		{[]string{"rev-parse", "--verify", "HEAD"}, a.Candidate},
		{[]string{"rev-parse", "--verify", "refs/heads/" + a.TargetBranch}, a.Candidate},
		{[]string{"rev-parse", "--verify", "HEAD^{tree}"}, a.CandidateTree},
		{[]string{"status", "--porcelain=v1", "--untracked-files=all"}, ""},
	} {
		got, err := completionGit(checkout, check.args...)
		if err != nil || got != check.want {
			return fmt.Errorf("integrated target differs from accepted clean commit: git %s = %q: %v", strings.Join(check.args, " "), got, err)
		}
	}
	return nil
}

func (o *Owner) archiveCompletion(st state, archive string) error {
	if err := ensureDir(archive); err != nil {
		return err
	}
	paths := []string{"evidence/task.md"}
	for number, final := range st.FinalReviews {
		if final {
			paths = append(paths, fmt.Sprintf("evidence/review-%02d.md", number))
		}
	}
	sort.Strings(paths)
	for _, relative := range paths {
		content, err := o.readBoundFile(st, relative)
		if err != nil {
			return err
		}
		if err := writeOrCheck(filepath.Join(archive, relative), content); err != nil {
			return err
		}
	}
	body, err := json.MarshalIndent(st.Completion, "", "  ")
	if err != nil {
		return err
	}
	if err := writeOrCheck(filepath.Join(archive, "completion.json"), append(body, '\n')); err != nil {
		return err
	}
	return nil
}

func (o *Owner) removeCompletionWorkspace(st state, suffix string) error {
	path := filepath.Join(o.store.Root, "workspaces", st.RunID+suffix)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil // already removed by an interrupted maintenance pass
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace %s is not a directory", path)
	}
	common, err := completionGit(path, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	common, err = canonical(common)
	if err != nil || common != o.store.CommonDir {
		return fmt.Errorf("workspace %s has wrong repository binding", path)
	}
	if head, err := completionGit(path, "rev-parse", "HEAD"); err != nil || head != st.Acceptance.Candidate {
		return fmt.Errorf("workspace %s changed commit: %s: %v", path, head, err)
	}
	if suffix == "" {
		if branch, err := completionGit(path, "symbolic-ref", "--quiet", "--short", "HEAD"); err != nil || branch != st.TaskBranch {
			return fmt.Errorf("task workspace changed branch: %s: %v", branch, err)
		}
	} else if _, err := completionGit(path, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil {
		return fmt.Errorf("review workspace is no longer detached")
	}
	if status, err := completionGit(path, "status", "--porcelain=v1", "--ignored", "--untracked-files=all"); err != nil || status != "" {
		return fmt.Errorf("workspace %s has retained changes: %s: %v", path, status, err)
	}
	_, err = completionGit(o.store.Checkout, "worktree", "remove", path)
	return err
}

func completionGit(checkout string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", checkout}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

// PruneArchives removes completed archives older than before. The caller must
// explicitly request pruning; active and supplied task/run IDs are retained.
func (s *Store) PruneArchives(before time.Time, supplied []string) ([]string, error) {
	owner, err := s.Acquire()
	if err != nil {
		return nil, err
	}
	defer owner.Close()
	if before.IsZero() {
		return nil, fmt.Errorf("pruning requires a cutoff time")
	}
	keep := make(map[string]bool, len(supplied)+1)
	for _, id := range supplied {
		if !taskPattern.MatchString(id) && !validRunID(id) {
			return nil, fmt.Errorf("invalid supplied record ID %q", id)
		}
		keep[id] = true
	}
	var active pointer
	if err := readJSON(filepath.Join(s.dir, "active.json"), &active); err == nil {
		if !validRunID(active.RunID) {
			return nil, fmt.Errorf("invalid active run ID")
		}
		keep[active.RunID] = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	root := filepath.Join(s.dir, "archive")
	tasks, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, task := range tasks {
		if !task.IsDir() || !taskPattern.MatchString(task.Name()) {
			return removed, fmt.Errorf("invalid archive task entry %q", task.Name())
		}
		taskDir := filepath.Join(root, task.Name())
		runs, err := os.ReadDir(taskDir)
		if err != nil {
			return removed, err
		}
		for _, run := range runs {
			if !run.IsDir() || !validRunID(run.Name()) {
				return removed, fmt.Errorf("invalid archive run entry %q", run.Name())
			}
			if keep[task.Name()] || keep[run.Name()] {
				continue
			}
			path := filepath.Join(taskDir, run.Name())
			var facts Completion
			if err := readJSON(filepath.Join(path, "completion.json"), &facts); err != nil || facts.TaskID != task.Name() || facts.RunID != run.Name() || facts.At.IsZero() {
				return removed, fmt.Errorf("archive %s lacks complete facts: %v", path, err)
			}
			if !facts.At.Before(before) {
				continue
			}
			if err := os.RemoveAll(path); err != nil {
				return removed, err
			}
			if err := syncDir(taskDir); err != nil {
				return removed, err
			}
			removed = append(removed, task.Name()+"/"+run.Name())
		}
	}
	return removed, nil
}
