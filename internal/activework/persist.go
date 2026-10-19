package activework

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func (s *Store) runDir(id string) string    { return filepath.Join(s.dir, "runs", id) }
func (s *Store) statePath(id string) string { return filepath.Join(s.runDir(id), "state.json") }
func validRunID(id string) bool             { return len(id) == 32 && strings.Trim(id, "0123456789abcdef") == "" }

func (s *Store) validateState(st state, id string) error {
	if st.Version != 1 || st.RunID != id || st.Repository != s.Repository || st.CommonDir != s.CommonDir || !validRunID(id) {
		return fmt.Errorf("active state has wrong version or repository binding")
	}
	if !revisionPattern.MatchString(st.ParentCommit) || st.ParentBranch == "" || st.Digests == nil || st.FinalReviews == nil {
		return fmt.Errorf("active state has malformed task binding")
	}
	if st.TaskID == "" {
		if st.TaskBranch != "" || st.Phase != "selecting" || st.Digests["evidence/task.md"] != "" {
			return fmt.Errorf("selecting run has invented task binding")
		}
	} else if !taskPattern.MatchString(st.TaskID) || st.TaskBranch == "" || st.Digests["evidence/task.md"] == "" {
		return fmt.Errorf("active state has malformed task binding")
	}
	if st.Candidate != nil {
		candidate := st.Candidate
		if st.TaskID == "" || st.Phase == "selecting" || candidate.BaselineCommit != st.ParentCommit || candidate.Commit == st.ParentCommit || !revisionPattern.MatchString(candidate.BaselineTree) || !revisionPattern.MatchString(candidate.Commit) || !revisionPattern.MatchString(candidate.Tree) || !isDigest(candidate.DiffSHA256) || !isDigest(candidate.CheckpointSHA256) || candidate.TurnID == "" || len(candidate.ChangedPaths) == 0 {
			return fmt.Errorf("active state has malformed candidate identity")
		}
		for index, path := range candidate.ChangedPaths {
			if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") || (index > 0 && candidate.ChangedPaths[index-1] >= path) {
				return fmt.Errorf("active state has malformed candidate path")
			}
		}
		for index, path := range candidate.GeneratedOutputs {
			if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") || (index > 0 && candidate.GeneratedOutputs[index-1] >= path) || slices.Contains(candidate.ChangedPaths, path) {
				return fmt.Errorf("active state has malformed generated output path")
			}
		}
	}
	if st.FreezeIntent != nil {
		intent := st.FreezeIntent
		if st.TaskID == "" || (st.Phase != "engineering" && st.Phase != "correcting") || intent.TurnID == "" || !isDigest(intent.CheckpointSHA256) || len(intent.ChangedPaths) == 0 || (st.Candidate != nil && intent.PriorCommit != st.Candidate.Commit) || (st.Candidate == nil && intent.PriorCommit != "") {
			return fmt.Errorf("active state has malformed candidate freeze intent")
		}
	}
	if st.Review != nil {
		review := st.Review
		if st.Candidate == nil || st.Dispatch == nil || review.Number < 1 || review.Number > st.Dispatch.Limits.Assessments || (review.Verdict == "" && review.Candidate != st.Candidate.Commit) || (st.Phase == "changes-requested" && review.Candidate != st.Candidate.Commit) || !isDigest(review.ScopeSHA256) || !namePattern.MatchString(review.AttemptID) || len(review.ThreadID) > 256 || len(review.TurnID) > 256 {
			return fmt.Errorf("active state has malformed review binding")
		}
		if review.Verdict != "" && review.Verdict != "approved" && review.Verdict != "changes_requested" {
			return fmt.Errorf("active state has invalid review verdict")
		}
		if (review.Verdict != "") != st.FinalReviews[review.Number] {
			return fmt.Errorf("active review finalization differs from verdict")
		}
	}
	if st.Acceptance != nil {
		a := st.Acceptance
		if !namePattern.MatchString(a.ID) || a.TaskID != st.TaskID || a.TargetBranch != st.ParentBranch || a.ExpectedParent != st.ParentCommit || st.Candidate == nil || a.Candidate != st.Candidate.Commit || a.CandidateTree != st.Candidate.Tree || st.Review == nil || st.Review.Verdict != "approved" || a.ReviewNumber != st.Review.Number || a.ReviewAttemptID != st.Review.AttemptID || a.ReviewTurnID != st.Review.TurnID || a.ScopeSHA256 != st.Review.ScopeSHA256 || !isDigest(a.ReviewSHA256) {
			return fmt.Errorf("active acceptance differs from reviewed candidate or target")
		}
	}
	if st.Integration != nil {
		if st.Acceptance == nil || st.Integration.AcceptanceID != st.Acceptance.ID || (st.Integration.Status != "intent" && st.Integration.Status != "integrated") {
			return fmt.Errorf("active integration lacks exact acceptance intent")
		}
	}
	if st.Completion != nil {
		c := st.Completion
		if st.Integration == nil || st.Integration.Status != "integrated" || st.Acceptance == nil || c.RunID != st.RunID || c.TaskID != st.TaskID || c.Candidate != st.Acceptance.Candidate || c.CandidateTree != st.Acceptance.CandidateTree || c.TargetBranch != st.ParentBranch || c.At.IsZero() {
			return fmt.Errorf("active completion lacks integrated binding")
		}
	}
	if st.Stop != nil {
		stop := st.Stop
		if st.Dispatch == nil || (stop.Role != "reagent" && stop.Role != "iragent") || stop.ThreadID == "" || st.RoleIDs[stop.Role] != stop.ThreadID || stop.AttemptID == "" || (stop.Status != "requested" && stop.Status != "stopped_safely" && stop.Status != "stopped_abnormally") || len(stop.Problem) > 1024 || len(stop.Owned) > 1024 || len(stop.Handles) > 1024 {
			return fmt.Errorf("active state has malformed stop binding")
		}
		found := false
		for _, attempt := range st.Dispatch.Attempts {
			if attempt.ID == stop.AttemptID && string(attempt.Binding.Role) == stop.Role {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("stop lacks a bound dispatch")
		}
	}
	if len(st.StopHistory) > 0 && (st.Dispatch == nil || len(st.StopHistory) > st.Dispatch.Limits.Dispatches) {
		return fmt.Errorf("active stop history exceeds dispatch bound")
	}
	for _, stopped := range st.StopHistory {
		if stopped.Status != "stopped_safely" || stopped.AttemptID == "" || stopped.ThreadID == "" || (stopped.Role != "reagent" && stopped.Role != "iragent") {
			return fmt.Errorf("active stop history has invalid evidence")
		}
		bound := false
		for _, attempt := range st.Dispatch.Attempts {
			if attempt.ID == stopped.AttemptID && attempt.ThreadID == stopped.ThreadID && string(attempt.Binding.Role) == stopped.Role && (attempt.Status == "completed" || attempt.Status == "reconciled" || attempt.Status == "failed") {
				bound = true
			}
		}
		if !bound {
			return fmt.Errorf("active stop history lacks a reconciled dispatch")
		}
	}
	if st.Dispatch != nil {
		if err := st.Dispatch.Validate(); err != nil {
			return fmt.Errorf("active dispatch state: %w", err)
		}
	}
	if err := validateQuestionState(st.Question, st); err != nil {
		return err
	}
	return nil
}

func (o *Owner) requireOwner() error {
	if o == nil || o.lock == nil {
		return fmt.Errorf("execution owner is closed")
	}
	return nil
}

func (o *Owner) after(stage string) error {
	if o.hook != nil {
		return o.hook(stage)
	}
	return nil
}

func (o *Owner) recover() error {
	if err := o.recoverReactivation(); err != nil {
		return err
	}
	var intent startIntent
	err := readJSON(filepath.Join(o.store.dir, "start.json"), &intent)
	if err == nil {
		if err := o.finishStart(intent); err != nil {
			return fmt.Errorf("recover start: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var active pointer
	err = readJSON(filepath.Join(o.store.dir, "active.json"), &active)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !validRunID(active.RunID) {
		return fmt.Errorf("invalid active run ID")
	}
	var st state
	if err := readJSON(o.store.statePath(active.RunID), &st); err != nil {
		return err
	}
	if err := o.store.validateState(st, active.RunID); err != nil {
		return err
	}
	if st.Pending != nil {
		if err := o.recoverPending(st); err != nil {
			return err
		}
	}
	return o.recoverInterventions()
}

func (o *Owner) finishStart(intent startIntent) error {
	st := intent.State
	if err := o.store.validateState(st, st.RunID); err != nil {
		return err
	}
	if len(intent.Task) > maxEvidence || (st.TaskID != "" && st.Digests["evidence/task.md"] != digest([]byte(intent.Task))) || (st.TaskID == "" && intent.Task != "") {
		return fmt.Errorf("start intent task digest mismatch")
	}
	runDir := o.store.runDir(st.RunID)
	if err := ensureDir(runDir); err != nil {
		return err
	}
	if st.TaskID != "" {
		taskPath := filepath.Join(runDir, "evidence", "task.md")
		if err := writeOrCheck(taskPath, []byte(intent.Task)); err != nil {
			return err
		}
	}
	if err := o.after("start-task"); err != nil {
		return err
	}
	statePath := o.store.statePath(st.RunID)
	if err := writeOrCheckJSON(statePath, st); err != nil {
		return err
	}
	if err := o.after("start-state"); err != nil {
		return err
	}
	activePath := filepath.Join(o.store.dir, "active.json")
	if err := writeOrCheckJSON(activePath, pointer{RunID: st.RunID}); err != nil {
		return err
	}
	if err := o.after("start-active"); err != nil {
		return err
	}
	if err := removeSync(filepath.Join(o.store.dir, "start.json")); err != nil {
		return err
	}
	return nil
}

func writeOrCheck(path string, content []byte) error {
	previous, err := readRegular(path)
	if err == nil {
		if digest(previous) != digest(content) {
			return fmt.Errorf("existing %s differs from recoverable intent", path)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return atomicWrite(path, content)
}

func writeOrCheckJSON(path string, value any) error {
	content, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeOrCheck(path, content)
}

func (o *Owner) currentState() (state, error) {
	if err := o.requireOwner(); err != nil {
		return state{}, err
	}
	var active pointer
	if err := readJSON(filepath.Join(o.store.dir, "active.json"), &active); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state{}, ErrNoActive
		}
		return state{}, err
	}
	if !validRunID(active.RunID) {
		return state{}, fmt.Errorf("invalid active run ID")
	}
	var st state
	if err := readJSON(o.store.statePath(active.RunID), &st); err != nil {
		return state{}, err
	}
	if err := o.store.validateState(st, active.RunID); err != nil {
		return state{}, err
	}
	if st.Pending != nil {
		return state{}, fmt.Errorf("pending write requires recovery")
	}
	return st, nil
}

func (o *Owner) readBoundFile(st state, relative string) ([]byte, error) {
	path, err := o.boundPath(st, relative)
	if err != nil {
		return nil, err
	}
	content, err := readRegular(path)
	if err != nil {
		return nil, err
	}
	if digest(content) != st.Digests[relative] {
		return nil, fmt.Errorf("evidence digest mismatch for %s", relative)
	}
	return content, nil
}

func (o *Owner) boundPath(st state, relative string) (string, error) {
	if filepath.IsAbs(relative) || filepath.Clean(relative) != relative || strings.HasPrefix(relative, "..") || !(strings.HasPrefix(relative, "evidence/") || strings.HasPrefix(relative, "receipts/")) {
		return "", fmt.Errorf("invalid active evidence path %q", relative)
	}
	return filepath.Join(o.store.runDir(st.RunID), relative), nil
}

func (o *Owner) transitionFile(st state, relative, content string) error {
	return o.transitionFileClaim(st, relative, content, "", "")
}

func (o *Owner) transitionFileClaim(st state, relative, content, claimTaskID, claimTaskBranch string) error {
	if len(content) > maxEvidence {
		return fmt.Errorf("evidence exceeds size limit")
	}
	path, err := o.boundPath(st, relative)
	if err != nil {
		return err
	}
	oldDigest := st.Digests[relative]
	previous, err := readRegular(path)
	if err == nil {
		if digest(previous) != oldDigest {
			return fmt.Errorf("evidence digest mismatch for %s", relative)
		}
	} else if !errors.Is(err, os.ErrNotExist) || oldDigest != "" {
		return fmt.Errorf("prior evidence for %s: %w", relative, err)
	}
	newDigest := digest([]byte(content))
	if oldDigest == newDigest {
		return nil
	}
	st.Pending = &pendingWrite{Path: relative, OldDigest: oldDigest, NewDigest: newDigest, Body: content, ClaimTaskID: claimTaskID, ClaimTaskBranch: claimTaskBranch}
	if err := writeJSON(o.store.statePath(st.RunID), st); err != nil {
		return err
	}
	if err := o.after("write-intent"); err != nil {
		return err
	}
	return o.recoverPending(st)
}

func (o *Owner) recoverPending(st state) error {
	pending := st.Pending
	if pending == nil {
		return nil
	}
	if digest([]byte(pending.Body)) != pending.NewDigest || st.Digests[pending.Path] != pending.OldDigest {
		return fmt.Errorf("invalid pending write digest")
	}
	path, err := o.boundPath(st, pending.Path)
	if err != nil {
		return err
	}
	current, err := readRegular(path)
	currentDigest := ""
	if err == nil {
		currentDigest = digest(current)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	switch currentDigest {
	case pending.OldDigest:
		if err := atomicWrite(path, []byte(pending.Body)); err != nil {
			return err
		}
	case pending.NewDigest:
		// The evidence rename completed before interruption.
	default:
		return fmt.Errorf("pending write %s conflicts with changed evidence", pending.Path)
	}
	if err := o.after("write-evidence"); err != nil {
		return err
	}
	st.Digests[pending.Path] = pending.NewDigest
	if pending.ClaimTaskID != "" {
		if st.TaskID != "" || st.Phase != "selecting" || pending.Path != "evidence/task.md" || !taskPattern.MatchString(pending.ClaimTaskID) || pending.ClaimTaskBranch == "" {
			return fmt.Errorf("invalid pending claim binding")
		}
		st.TaskID, st.TaskBranch, st.Phase = pending.ClaimTaskID, pending.ClaimTaskBranch, "engineering"
	}
	st.Pending = nil
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

func readJSON(path string, value any) error {
	content, err := readRegular(path)
	if err != nil {
		return err
	}
	if len(content) > maxEvidence*4 {
		return fmt.Errorf("oversized record %s", path)
	}
	if err := json.Unmarshal(content, value); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func writeJSON(path string, value any) error {
	content, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return atomicWrite(path, content)
}

func readRegular(path string) ([]byte, error) {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("record parent %s is not a directory", dir)
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("record %s is not a regular file", path)
	}
	return os.ReadFile(path)
}

func ensureDir(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", path)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(path)
	if parent != path {
		if err := ensureDir(parent); err != nil {
			return err
		}
	}
	if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return syncDir(parent)
}

func atomicWrite(path string, content []byte) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func removeSync(path string) error {
	if err := os.Remove(path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
