package activework

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	maxEvidence     = 64 << 10
	checkpointStart = "<!-- engineering-checkpoint:start -->"
	checkpointEnd   = "<!-- engineering-checkpoint:end -->"
	decisionsStart  = "<!-- operator-decisions:start -->"
	decisionsEnd    = "<!-- operator-decisions:end -->"
)

var (
	ErrNoActive     = errors.New("no active run")
	namePattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	taskPattern     = regexp.MustCompile(`^[A-Z][A-Z0-9]*-[A-Z]*[0-9]{3,}$`)
	revisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type StartSpec struct {
	TaskID, ParentBranch, ParentCommit, TaskBranch string
	Objective, Acceptance, SelectionRationale      string
}

type SelectionSpec struct {
	ParentBranch, ParentCommit string
}

type Operational struct {
	Phase, Hold, PendingRequest string
	RoleIDs, RoleSettings       map[string]string
	Counters                    map[string]int
}

type Snapshot struct {
	ObservedAt     time.Time
	Active         bool
	PendingStart   bool
	RunID          string
	Repository     string
	CommonDir      string
	TaskID         string
	ParentBranch   string
	ParentCommit   string
	TaskBranch     string
	Phase          string
	Hold           string
	PendingRequest string
	Revision       uint64
	PendingWrite   bool
	RoleIDs        map[string]string
	RoleSettings   map[string]string
	Counters       map[string]int
}

type pointer struct {
	RunID string `json:"run_id"`
}

type pendingWrite struct {
	Path, OldDigest, NewDigest, Body string
	ClaimTaskID, ClaimTaskBranch     string
}

type state struct {
	Version                                        int
	RunID, Repository, CommonDir                   string
	TaskID, ParentBranch, ParentCommit, TaskBranch string
	Phase, Hold, PendingRequest                    string
	RoleIDs, RoleSettings                          map[string]string
	Counters                                       map[string]int
	Digests                                        map[string]string
	FinalReviews                                   map[int]bool
	Revision                                       uint64
	Pending                                        *pendingWrite
}

type startIntent struct {
	State state
	Task  string
}

// Status only reads atomic records. It takes no execution lock and makes no
// filesystem changes, so it remains available while an owner is running.
func (s *Store) Status() (Snapshot, error) {
	snapshot := Snapshot{ObservedAt: time.Now().UTC()}
	var active pointer
	if err := readJSON(filepath.Join(s.dir, "active.json"), &active); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return snapshot, err
		}
		_, pendingErr := os.Stat(filepath.Join(s.dir, "start.json"))
		if pendingErr == nil {
			snapshot.PendingStart = true
		}
		if pendingErr != nil && !errors.Is(pendingErr, os.ErrNotExist) {
			return snapshot, pendingErr
		}
		return snapshot, nil
	}
	if !validRunID(active.RunID) {
		return snapshot, fmt.Errorf("invalid active run ID")
	}
	var st state
	if err := readJSON(s.statePath(active.RunID), &st); err != nil {
		return snapshot, err
	}
	if err := s.validateState(st, active.RunID); err != nil {
		return snapshot, err
	}
	snapshot.Active = true
	snapshot.RunID, snapshot.TaskID = st.RunID, st.TaskID
	snapshot.Repository, snapshot.CommonDir = st.Repository, st.CommonDir
	snapshot.ParentBranch, snapshot.ParentCommit, snapshot.TaskBranch = st.ParentBranch, st.ParentCommit, st.TaskBranch
	snapshot.Phase, snapshot.Hold, snapshot.PendingRequest = st.Phase, st.Hold, st.PendingRequest
	snapshot.Revision, snapshot.PendingWrite = st.Revision, st.Pending != nil
	snapshot.RoleIDs, snapshot.RoleSettings, snapshot.Counters = cloneStrings(st.RoleIDs), cloneStrings(st.RoleSettings), cloneInts(st.Counters)
	return snapshot, nil
}

func (o *Owner) Start(spec StartSpec) (string, error) {
	if err := o.requireOwner(); err != nil {
		return "", err
	}
	if err := o.validateStart(spec); err != nil {
		return "", err
	}
	task := renderTask(spec)
	st := state{Version: 1, Repository: o.store.Repository, CommonDir: o.store.CommonDir,
		TaskID: spec.TaskID, ParentBranch: spec.ParentBranch, ParentCommit: spec.ParentCommit, TaskBranch: spec.TaskBranch,
		Phase: "engineering", RoleIDs: map[string]string{}, RoleSettings: map[string]string{}, Counters: map[string]int{},
		Digests: map[string]string{"evidence/task.md": digest([]byte(task))}, FinalReviews: map[int]bool{}}
	return o.begin(st, task)
}

// BeginSelection retains parent and recovery facts before a task has been
// claimed. It writes no invented task.md. Claim binds one actual task later.
func (o *Owner) BeginSelection(spec SelectionSpec) (string, error) {
	if err := o.requireOwner(); err != nil {
		return "", err
	}
	if err := o.validateParent(spec.ParentBranch, spec.ParentCommit); err != nil {
		return "", err
	}
	st := state{Version: 1, Repository: o.store.Repository, CommonDir: o.store.CommonDir,
		ParentBranch: spec.ParentBranch, ParentCommit: spec.ParentCommit, Phase: "selecting",
		RoleIDs: map[string]string{}, RoleSettings: map[string]string{}, Counters: map[string]int{}, Digests: map[string]string{}, FinalReviews: map[int]bool{}}
	return o.begin(st, "")
}

func (o *Owner) begin(st state, task string) (string, error) {
	if _, err := os.Stat(filepath.Join(o.store.dir, "active.json")); err == nil {
		return "", fmt.Errorf("active run already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(o.store.dir, "start.json")); err == nil {
		return "", fmt.Errorf("incomplete run start requires recovery")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	id := hex.EncodeToString(bytes)
	st.RunID = id
	intent := startIntent{State: st, Task: task}
	if err := writeJSON(filepath.Join(o.store.dir, "start.json"), intent); err != nil {
		return "", err
	}
	if err := o.after("start-intent"); err != nil {
		return "", err
	}
	if err := o.finishStart(intent); err != nil {
		return "", err
	}
	return id, nil
}

// Claim is the host-owned transition from a selecting run to one task. Its
// binding and task.md are recovered together after interruption.
func (o *Owner) Claim(spec StartSpec) error {
	if err := o.validateStart(spec); err != nil {
		return err
	}
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Phase != "selecting" || st.TaskID != "" {
		return fmt.Errorf("run already has a task binding")
	}
	if st.ParentBranch != spec.ParentBranch || st.ParentCommit != spec.ParentCommit {
		return fmt.Errorf("claim parent differs from active run")
	}
	return o.transitionFileClaim(st, "evidence/task.md", renderTask(spec), spec.TaskID, spec.TaskBranch)
}

func (o *Owner) validateStart(spec StartSpec) error {
	if !taskPattern.MatchString(spec.TaskID) {
		return fmt.Errorf("invalid task ID %q", spec.TaskID)
	}
	if spec.TaskBranch == "" || len(spec.TaskBranch) > 200 || strings.ContainsAny(spec.TaskBranch, " \t\r\n\x00") || strings.HasPrefix(spec.TaskBranch, "-") || strings.Contains(spec.TaskBranch, "..") {
		return fmt.Errorf("invalid task branch binding %q", spec.TaskBranch)
	}
	if spec.ParentBranch == spec.TaskBranch {
		return fmt.Errorf("task branch must differ from parent branch")
	}
	if err := o.validateParent(spec.ParentBranch, spec.ParentCommit); err != nil {
		return err
	}
	output, err := exec.Command("git", "-C", o.store.Checkout, "check-ref-format", "--branch", spec.TaskBranch).Output()
	if err != nil || strings.TrimSpace(string(output)) != spec.TaskBranch {
		return fmt.Errorf("invalid Git branch binding %q", spec.TaskBranch)
	}
	for _, text := range []string{spec.Objective, spec.Acceptance, spec.SelectionRationale} {
		if strings.TrimSpace(text) == "" || len(text) > maxEvidence/4 || strings.Contains(text, "<!-- engineering-checkpoint:") || strings.Contains(text, "<!-- operator-decisions:") {
			return fmt.Errorf("empty, oversized, or reserved task scope content")
		}
	}
	return nil
}

func (o *Owner) validateParent(branch, commit string) error {
	if !revisionPattern.MatchString(commit) {
		return fmt.Errorf("invalid parent commit")
	}
	if branch == "" || len(branch) > 200 || strings.ContainsAny(branch, " \t\r\n\x00") || strings.HasPrefix(branch, "-") || strings.Contains(branch, "..") {
		return fmt.Errorf("invalid parent branch binding %q", branch)
	}
	output, err := exec.Command("git", "-C", o.store.Checkout, "check-ref-format", "--branch", branch).Output()
	if err != nil || strings.TrimSpace(string(output)) != branch {
		return fmt.Errorf("invalid Git branch binding %q", branch)
	}
	if err := exec.Command("git", "-C", o.store.Checkout, "cat-file", "-e", commit+"^{commit}").Run(); err != nil {
		return fmt.Errorf("parent commit %s is not present in the repository: %w", commit, err)
	}
	actual, err := exec.Command("git", "-C", o.store.Checkout, "rev-parse", "--verify", "refs/heads/"+branch).Output()
	if err != nil || strings.TrimSpace(string(actual)) != commit {
		return fmt.Errorf("parent branch %q does not point to commit %s", branch, commit)
	}
	return nil
}

func renderTask(spec StartSpec) string {
	return fmt.Sprintf("# %s active task\n\n## Agreed objective\n\n%s\n\n## Acceptance\n\n%s\n\n## Selection rationale\n\n%s\n\n## Engineering checkpoint\n\n%s\n%s\n\n## Operator decisions\n\n%s\n%s\n", spec.TaskID, spec.Objective, spec.Acceptance, spec.SelectionRationale, checkpointStart, checkpointEnd, decisionsStart, decisionsEnd)
}

func (o *Owner) SaveCheckpoint(content string) error {
	if err := validateContent(content); err != nil {
		return err
	}
	st, err := o.currentState()
	if err != nil {
		return err
	}
	old, err := o.readBoundFile(st, "evidence/task.md")
	if err != nil {
		return err
	}
	next, err := replaceRegion(string(old), checkpointStart, checkpointEnd, content)
	if err != nil {
		return err
	}
	return o.transitionFile(st, "evidence/task.md", next)
}

// RecordDecision is the host-owned path for appending an operator decision.
func (o *Owner) RecordDecision(content string) error {
	if err := validateContent(content); err != nil {
		return err
	}
	st, err := o.currentState()
	if err != nil {
		return err
	}
	old, err := o.readBoundFile(st, "evidence/task.md")
	if err != nil {
		return err
	}
	start := strings.Index(string(old), decisionsStart)
	end := strings.Index(string(old), decisionsEnd)
	if start < 0 || end < start {
		return fmt.Errorf("task decision markers are missing or reordered")
	}
	before := string(old)[:end]
	if len(before)+len(content)+len(string(old)[end:]) > maxEvidence {
		return fmt.Errorf("task evidence exceeds size limit")
	}
	return o.transitionFile(st, "evidence/task.md", before+strings.TrimSpace(content)+"\n"+string(old)[end:])
}

func (o *Owner) SaveReview(number int, content string) error {
	if number < 1 || number > 99 {
		return fmt.Errorf("invalid review number")
	}
	if err := validateContent(content); err != nil {
		return err
	}
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.FinalReviews[number] {
		return fmt.Errorf("review %02d is finalized", number)
	}
	return o.transitionFile(st, fmt.Sprintf("evidence/review-%02d.md", number), content)
}

func (o *Owner) FinalizeReview(number int) error {
	if number < 1 || number > 99 {
		return fmt.Errorf("invalid review number")
	}
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.FinalReviews[number] {
		return fmt.Errorf("review %02d is already finalized", number)
	}
	path := fmt.Sprintf("evidence/review-%02d.md", number)
	if st.Digests[path] == "" {
		return fmt.Errorf("review %02d has no content", number)
	}
	if _, err := o.readBoundFile(st, path); err != nil {
		return err
	}
	st.FinalReviews[number] = true
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

// RecordReceipt creates a bounded JSON receipt once. Repeating identical data
// is idempotent; a different payload for the same name is rejected.
func (o *Owner) RecordReceipt(name string, content []byte) error {
	if !namePattern.MatchString(name) || !json.Valid(content) || len(content) > maxEvidence/4 {
		return fmt.Errorf("invalid receipt name or JSON content")
	}
	st, err := o.currentState()
	if err != nil {
		return err
	}
	path := "receipts/" + name + ".json"
	if previous := st.Digests[path]; previous != "" {
		if _, err := o.readBoundFile(st, path); err != nil {
			return err
		}
		if previous == digest(content) {
			return nil
		}
		return fmt.Errorf("receipt %s already has different content", name)
	}
	return o.transitionFile(st, path, string(content))
}

func (o *Owner) UpdateOperational(value Operational) error {
	if len(value.Phase) > 64 || value.Phase == "" || len(value.Hold) > 256 || len(value.PendingRequest) > 256 || len(value.RoleIDs) > 4 || len(value.RoleSettings) > 16 || len(value.Counters) > 32 {
		return fmt.Errorf("invalid operational state")
	}
	for key, count := range value.Counters {
		if !namePattern.MatchString(key) || count < 0 {
			return fmt.Errorf("invalid counter %q", key)
		}
	}
	for key, val := range value.RoleIDs {
		if !namePattern.MatchString(key) || len(val) > 256 {
			return fmt.Errorf("invalid role identity %q", key)
		}
	}
	for key, val := range value.RoleSettings {
		if !namePattern.MatchString(key) || len(val) > 256 {
			return fmt.Errorf("invalid role setting %q", key)
		}
	}
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.TaskID == "" && value.Phase != "selecting" {
		return fmt.Errorf("unclaimed run must remain in selecting phase")
	}
	st.Phase, st.Hold, st.PendingRequest = value.Phase, value.Hold, value.PendingRequest
	st.RoleIDs, st.RoleSettings, st.Counters = cloneStrings(value.RoleIDs), cloneStrings(value.RoleSettings), cloneInts(value.Counters)
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

func validateContent(content string) error {
	if strings.TrimSpace(content) == "" || len(content) > maxEvidence/2 || strings.Contains(content, checkpointStart) || strings.Contains(content, checkpointEnd) || strings.Contains(content, decisionsStart) || strings.Contains(content, decisionsEnd) {
		return fmt.Errorf("empty, oversized, or reserved evidence content")
	}
	return nil
}

func replaceRegion(text, startMarker, endMarker, replacement string) (string, error) {
	start := strings.Index(text, startMarker)
	end := strings.Index(text, endMarker)
	if start < 0 || end < start || strings.Count(text, startMarker) != 1 || strings.Count(text, endMarker) != 1 {
		return "", fmt.Errorf("task evidence markers are missing or duplicated")
	}
	result := text[:start+len(startMarker)] + "\n" + strings.TrimSpace(replacement) + "\n" + text[end:]
	if len(result) > maxEvidence {
		return "", fmt.Errorf("task evidence exceeds size limit")
	}
	return result, nil
}

func cloneStrings(input map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
func cloneInts(input map[string]int) map[string]int {
	out := make(map[string]int, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
