package handoff

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	roadmapdoc "github.com/gopher-launch/concoct/internal/roadmap"
	"github.com/gopher-launch/concoct/internal/stop"
)

// State contains coordination facts. Engineering and review prose belongs in
// task.md and numbered review files, which agents maintain with ordinary tools.
type State struct {
	Repository       string                                  `json:"repository"`
	Roadmap          string                                  `json:"roadmap,omitempty"`
	Capabilities     string                                  `json:"capabilities,omitempty"`
	Parent           string                                  `json:"parent"`
	Base             string                                  `json:"base"`
	Branch           string                                  `json:"branch,omitempty"`
	TaskID           string                                  `json:"task_id,omitempty"`
	Candidate        string                                  `json:"candidate,omitempty"`
	Phase            string                                  `json:"phase"`
	Evidence         string                                  `json:"evidence,omitempty"`
	ReviewEvidence   string                                  `json:"review_evidence,omitempty"`
	Verdict          string                                  `json:"verdict,omitempty"`
	Summary          string                                  `json:"summary,omitempty"`
	ReagentSummary   string                                  `json:"reagent_summary,omitempty"`
	ReviewSummary    string                                  `json:"review_summary,omitempty"`
	Findings         []string                                `json:"findings,omitempty"`
	Assessments      int                                     `json:"assessments"`
	MaxAssessments   int                                     `json:"max_assessments"`
	LimitPhase       string                                  `json:"limit_phase,omitempty"`
	LimitIncreases   []LimitIncrease                         `json:"limit_increases,omitempty"`
	ApprovedCommit   string                                  `json:"approved_commit,omitempty"`
	AuthorizedCommit string                                  `json:"authorized_commit,omitempty"`
	Question         *codexapp.RoleQuestion                  `json:"question,omitempty"`
	QuestionRole     dispatch.Role                           `json:"question_role,omitempty"`
	BlockedPhase     string                                  `json:"blocked_phase,omitempty"`
	Answer           string                                  `json:"answer,omitempty"`
	Profiles         map[dispatch.Role]codexapp.RoleSettings `json:"profiles"`
	Threads          map[string]string                       `json:"threads,omitempty"`
	RoleTurns        map[string]bool                         `json:"role_turns,omitempty"`
	InFlight         *InFlight                               `json:"in_flight,omitempty"`
	PendingStep      string                                  `json:"pending_step,omitempty"`
	Recoveries       []Recovery                              `json:"recoveries,omitempty"`
	ActiveRecovery   *Recovery                               `json:"active_recovery,omitempty"`
	Attempts         int                                     `json:"attempts"`
	MaxAttempts      int                                     `json:"max_attempts"`
	TurnTimeout      time.Duration                           `json:"turn_timeout,omitempty"`
	StallTimeout     time.Duration                           `json:"stall_timeout,omitempty"`
	LastResult       *codexapp.RoleResult                    `json:"last_result,omitempty"`
	LastRole         dispatch.Role                           `json:"last_role,omitempty"`
	LastError        string                                  `json:"last_error,omitempty"`
	Diagnostics      string                                  `json:"diagnostics,omitempty"`
	CompletionRecord string                                  `json:"completion_record,omitempty"`
	CompletedAt      time.Time                               `json:"completed_at,omitempty"`
	Updated          time.Time                               `json:"updated"`
}

// LimitIncrease records an operator-authorized change to a total execution
// ceiling. Consumption remains in State.Attempts and State.Assessments.
type LimitIncrease struct {
	Allowance       string    `json:"allowance"`
	PreviousTotal   int       `json:"previous_total"`
	AuthorizedTotal int       `json:"authorized_total"`
	AuthorizedAt    time.Time `json:"authorized_at"`
}

// InFlight is saved with the invocation count before dispatch. A later run
// cannot assume an unrecorded provider outcome means the turn never ran.
type InFlight struct {
	Role       dispatch.Role   `json:"role"`
	ThreadID   string          `json:"thread_id"`
	Step       string          `json:"step,omitempty"`
	TurnID     string          `json:"turn_id,omitempty"`
	Processes  []stop.Identity `json:"processes,omitempty"`
	Checkpoint Checkpoint      `json:"checkpoint,omitempty"`
	Terminated bool            `json:"terminated,omitempty"`
}

type Checkpoint struct {
	Branch           string             `json:"branch,omitempty"`
	Head             string             `json:"head,omitempty"`
	Worktree         string             `json:"worktree,omitempty"`
	GitVisibleSHA256 string             `json:"git_visible_sha256,omitempty"`
	Evidence         []EvidenceSnapshot `json:"evidence,omitempty"`
}

type EvidenceSnapshot struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Recovery struct {
	RequestedAt        time.Time             `json:"requested_at"`
	PriorPhase         string                `json:"prior_phase"`
	Role               dispatch.Role         `json:"role"`
	Step               string                `json:"step"`
	Diagnostic         string                `json:"diagnostic,omitempty"`
	PriorError         string                `json:"prior_error,omitempty"`
	Before             Checkpoint            `json:"before"`
	Observed           Checkpoint            `json:"observed"`
	Changes            string                `json:"changes"`
	ArchivedEvidence   string                `json:"archived_evidence,omitempty"`
	AdditionalArchives []string              `json:"additional_archives,omitempty"`
	RequestedModel     string                `json:"requested_model,omitempty"`
	RequestedEffort    string                `json:"requested_effort,omitempty"`
	PriorThread        string                `json:"prior_thread,omitempty"`
	ReplacementThread  string                `json:"replacement_thread,omitempty"`
	PriorProfile       codexapp.RoleSettings `json:"prior_profile"`
	ReplacementProfile codexapp.RoleSettings `json:"replacement_profile"`
	DispatchAttempt    int                   `json:"dispatch_attempt,omitempty"`
	IntegrationParent  string                `json:"integration_parent,omitempty"`
}

type Options struct {
	Repository, Root, Codex, Roadmap                           string
	Approve, Answer                                            string
	ReagentModel, IragentModel, ReagentEffort, IragentEffort   string
	MaxAttempts, MaxAssessments                                int
	ApproveInvocationLimitIncrease, ApproveReviewLimitIncrease int
	TurnTimeout, StallTimeout                                  time.Duration
	Recover                                                    bool
	OnEvent                                                    func(string, codexapp.Event)
	OnResult                                                   func(string, codexapp.RoleResult)
	OnRecovery                                                 func(State)
}

type processRecorder interface {
	RecoveryProcesses() []stop.Identity
}

type Runner interface {
	Models(context.Context) ([]codexapp.Model, error)
	StartThread(context.Context, string, codexapp.RoleSettings, []codexapp.ToolSpec) (codexapp.Thread, error)
	ResumeThread(context.Context, string, bool, codexapp.RoleSettings) (codexapp.Thread, error)
	RunRoleTurn(context.Context, string, string, codexapp.RoleSettings, func(codexapp.Event)) (codexapp.TurnOutcome, error)
	Stop(context.Context, string, string) stop.Result
	Close() error
}

type Provider struct {
	process *codexapp.Process
	probe   *stop.Probe
	known   map[string]bool
}

func Launch(ctx context.Context, executable string, diagnostics io.Writer) (*Provider, error) {
	p, err := codexapp.Launch(ctx, executable, diagnostics)
	if err != nil {
		return nil, err
	}
	probe, err := stop.NewProbe(p.PID())
	if err != nil {
		p.Close()
		return nil, err
	}
	return &Provider{process: p, probe: probe, known: make(map[string]bool)}, nil
}
func (p *Provider) Models(ctx context.Context) ([]codexapp.Model, error) {
	return p.process.Models(ctx)
}
func (p *Provider) StartThread(ctx context.Context, cwd string, s codexapp.RoleSettings, t []codexapp.ToolSpec) (codexapp.Thread, error) {
	thread, err := p.process.StartThread(ctx, cwd, s, t)
	if err == nil {
		p.known[thread.ID] = true
	}
	return thread, err
}
func (p *Provider) ResumeThread(ctx context.Context, id string, hadTurn bool, s codexapp.RoleSettings) (codexapp.Thread, error) {
	if p.known[id] {
		return codexapp.Thread{ID: id}, nil
	}
	thread, err := p.process.ResumeThread(ctx, id, hadTurn, s)
	if err == nil {
		p.known[id] = true
	}
	return thread, err
}
func (p *Provider) RunRoleTurn(ctx context.Context, id, prompt string, s codexapp.RoleSettings, event func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, _ = p.probe.Capture()
			}
		}
	}()
	outcome, err := p.process.RunRoleTurn(ctx, id, prompt, s, event)
	close(done)
	_, _ = p.probe.Capture()
	return outcome, err
}
func (p *Provider) Stop(ctx context.Context, id, turn string) stop.Result {
	result := stop.Run(ctx, p.process.Client, p.probe, id, turn, 2*time.Second)
	if result.Status != "stopped_safely" {
		fallbackCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := p.probe.TerminateOwned(fallbackCtx); err != nil {
			result.Problem += "; host termination: " + err.Error()
		} else {
			result.Problem += "; owned children terminated by host"
		}
	}
	return result
}
func (p *Provider) Close() error { return p.process.Close() }
func (p *Provider) RecoveryProcesses() []stop.Identity {
	ids := []stop.Identity{p.probe.Root()}
	if owned, err := p.probe.Capture(); err == nil {
		ids = append(ids, owned...)
	}
	return ids
}

func git(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}
func canonical(repo string) (string, error) {
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func resolvePath(repo, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(repo, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func readDocument(path, label string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("%s %q: %w", label, path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s %q is not a regular file", label, path)
	}
	if info.Mode().Perm()&0444 == 0 {
		return nil, fmt.Errorf("%s %q is not readable", label, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s %q: %w", label, path, err)
	}
	return data, nil
}

func chooseCapabilities(repo, selectedRoadmap string) (string, error) {
	candidates := []string{filepath.Join(filepath.Dir(selectedRoadmap), "capabilities.md")}
	root := filepath.Join(repo, "capabilities.md")
	if candidates[0] != root {
		candidates = append(candidates, root)
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil {
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("capabilities %q is not a regular file", candidate)
			}
			resolved, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				return "", err
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("capabilities %q: %w", candidate, err)
		}
	}
	return "", nil
}

func validateRoadmap(path, capabilities string) error {
	roadmapMarkdown, err := readDocument(path, "roadmap")
	if err != nil {
		return err
	}
	var capabilitiesMarkdown []byte
	if capabilities != "" {
		capabilitiesMarkdown, err = readDocument(capabilities, "capabilities")
		if err != nil {
			return err
		}
	}
	if _, err := roadmapdoc.Parse(roadmapMarkdown, capabilitiesMarkdown); err != nil {
		return fmt.Errorf("invalid roadmap %q: %w", path, err)
	}
	return nil
}

func selectRoadmap(repo, requested string) (string, string, error) {
	if requested == "" {
		requested = "roadmap.md"
	}
	path, err := resolvePath(repo, requested)
	if err != nil {
		return "", "", fmt.Errorf("resolve roadmap %q: %w", requested, err)
	}
	capabilities, err := chooseCapabilities(repo, path)
	if err != nil {
		return "", "", err
	}
	if err := validateRoadmap(path, capabilities); err != nil {
		return "", "", err
	}
	return path, capabilities, nil
}
func storage(repo, root string) (string, error) {
	common, err := git(repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil {
		return "", err
	}
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".concoct")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	// A stable path key avoids name collisions without putting source in the record.
	sum := shaPath(common)
	return filepath.Join(root, "repos", sum), nil
}
func shaPath(value string) string { // path-only identity; not a content digest
	h := uint64(14695981039346656037)
	for _, c := range []byte(value) {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return fmt.Sprintf("%016x", h)
}
func statePath(dir string) string       { return filepath.Join(dir, "state.json") }
func completionsPath(dir string) string { return filepath.Join(dir, "completions") }
func read(dir string) (State, error) {
	data, err := os.ReadFile(statePath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err = json.Unmarshal(data, &s); err != nil {
		return State{}, err
	}
	return s, nil
}
func save(dir string, s State) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	s.Updated = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".state-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, statePath(dir)); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func Status(o Options) (State, error) {
	repo, err := canonical(o.Repository)
	if err != nil {
		return State{}, err
	}
	dir, err := storage(repo, o.Root)
	if err != nil {
		return State{}, err
	}
	s, err := read(dir)
	if err != nil {
		return s, err
	}
	if s.Repository != "" && s.Repository != repo {
		return State{}, fmt.Errorf("state belongs to %s", s.Repository)
	}
	if s.Phase == "" {
		return latestCompletion(dir, repo)
	}
	return s, nil
}

func checkpoint(s State, root string) (Checkpoint, error) {
	var c Checkpoint
	c.Branch, _ = git(s.Repository, "symbolic-ref", "--quiet", "--short", "HEAD")
	var err error
	c.Head, err = git(s.Repository, "rev-parse", "HEAD")
	if err != nil {
		return c, err
	}
	c.Worktree, err = git(s.Repository, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return c, err
	}
	c.GitVisibleSHA256, err = gitVisibleHash(s.Repository)
	if err != nil {
		return c, err
	}
	evidenceRoot := filepath.Join(root, "evidence", "roadmap-"+shaPath(s.Roadmap))
	if s.TaskID != "" {
		evidenceRoot = filepath.Join(evidenceRoot, s.TaskID)
	}
	err = filepath.WalkDir(evidenceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if errors.Is(walkErr, os.ErrNotExist) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		rel, err := filepath.Rel(evidenceRoot, path)
		if err != nil {
			return err
		}
		c.Evidence = append(c.Evidence, EvidenceSnapshot{Path: rel, Size: info.Size(), SHA256: fmt.Sprintf("%x", sum)})
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return c, err
	}
	sort.Slice(c.Evidence, func(i, j int) bool { return c.Evidence[i].Path < c.Evidence[j].Path })
	return c, nil
}

func gitVisibleHash(repo string) (string, error) {
	diff, err := exec.Command("git", "-C", repo, "diff", "--binary", "HEAD").Output()
	if err != nil {
		return "", err
	}
	untracked, err := exec.Command("git", "-C", repo, "ls-files", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	hash.Write(diff)
	for _, raw := range strings.Split(string(untracked), "\x00") {
		if raw == "" {
			continue
		}
		hash.Write([]byte("\x00" + raw + "\x00"))
		path := filepath.Join(repo, raw)
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			hash.Write(body)
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			hash.Write([]byte(target))
		} else {
			hash.Write([]byte(info.Mode().String()))
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func checkpointChanges(before, after Checkpoint) string {
	var changes []string
	if before.Branch != after.Branch {
		changes = append(changes, fmt.Sprintf("branch changed from %q to %q", before.Branch, after.Branch))
	}
	if before.Head != after.Head {
		changes = append(changes, fmt.Sprintf("HEAD changed from %s to %s", before.Head, after.Head))
	}
	if before.Worktree != after.Worktree || before.GitVisibleSHA256 != after.GitVisibleSHA256 {
		changes = append(changes, fmt.Sprintf("Git-visible worktree changed; current status:\n%s", after.Worktree))
	}
	beforeEvidence := make(map[string]EvidenceSnapshot, len(before.Evidence))
	for _, item := range before.Evidence {
		beforeEvidence[item.Path] = item
	}
	var evidence []string
	for _, item := range after.Evidence {
		prior, ok := beforeEvidence[item.Path]
		if !ok {
			evidence = append(evidence, item.Path+" (new)")
		} else if prior.SHA256 != item.SHA256 || prior.Size != item.Size {
			evidence = append(evidence, item.Path+" (changed)")
		}
		delete(beforeEvidence, item.Path)
	}
	for path := range beforeEvidence {
		evidence = append(evidence, path+" (missing)")
	}
	sort.Strings(evidence)
	if len(evidence) > 0 {
		changes = append(changes, "external evidence: "+strings.Join(evidence, ", "))
	}
	if len(changes) == 0 {
		return "No source or external evidence changes were observed."
	}
	return strings.Join(changes, "\n")
}

func pendingRecoveryStep(s State) (string, error) {
	if s.PendingStep != "" {
		return s.PendingStep, nil
	}
	if s.InFlight != nil && s.InFlight.Step != "" {
		return s.InFlight.Step, nil
	}
	// Compatibility for older records is deliberately based only on durable
	// workflow bindings, never on narrative or repository guesses.
	if s.AuthorizedCommit != "" && s.AuthorizedCommit == s.ApprovedCommit {
		return "integrating", nil
	}
	if s.Verdict == "changes_requested" && s.Candidate != "" {
		return "correcting", nil
	}
	if s.Candidate != "" && s.TaskID != "" && s.Verdict == "" {
		return "reviewing", nil
	}
	if s.Candidate == "" && s.Verdict == "" {
		return "engineering", nil
	}
	return "", fmt.Errorf("stopped state does not identify one pending workflow step")
}

func recoveryRole(step string) dispatch.Role {
	if step == "reviewing" {
		return dispatch.Iragent
	}
	return dispatch.Reagent
}

func validateRecoveryOverrideRole(s State, o Options) error {
	step, err := pendingRecoveryStep(s)
	if err != nil {
		return err
	}
	role := recoveryRole(step)
	if (role == dispatch.Reagent && (o.IragentModel != "" || o.IragentEffort != "")) || (role == dispatch.Iragent && (o.ReagentModel != "" || o.ReagentEffort != "")) {
		return fmt.Errorf("recovery overrides must target the replacement %s role", role)
	}
	return nil
}

func recordRecoveryOverride(s *State, o Options) {
	if s.ActiveRecovery == nil {
		return
	}
	if s.ActiveRecovery.Role == dispatch.Iragent {
		if o.IragentModel != "" {
			s.ActiveRecovery.RequestedModel = o.IragentModel
		}
		if o.IragentEffort != "" {
			s.ActiveRecovery.RequestedEffort = o.IragentEffort
		}
	} else {
		if o.ReagentModel != "" {
			s.ActiveRecovery.RequestedModel = o.ReagentModel
		}
		if o.ReagentEffort != "" {
			s.ActiveRecovery.RequestedEffort = o.ReagentEffort
		}
	}
	s.Recoveries[len(s.Recoveries)-1] = *s.ActiveRecovery
}

func integrationRecoveryParent(s State) (string, error) {
	parent, err := git(s.Repository, "rev-parse", s.Parent)
	if err != nil {
		return "", err
	}
	task, err := git(s.Repository, "rev-parse", s.Branch)
	if err != nil {
		return "", err
	}
	if task != s.AuthorizedCommit || (parent != s.Base && parent != s.AuthorizedCommit) {
		return "", fmt.Errorf("integration refs are not safely reconcilable: parent=%s task=%s expected parent %s or %s and task %s", parent, task, s.Base, s.AuthorizedCommit, s.AuthorizedCommit)
	}
	if parent == s.AuthorizedCommit {
		return "authorized", nil
	}
	return "base", nil
}

func archivePartialReview(dir string, s State, at time.Time) (string, string, error) {
	if s.TaskID == "" {
		return "", "", nil
	}
	partial := reviewPath(dir, s.Roadmap, s.TaskID, s.Assessments+1)
	info, err := os.Stat(partial)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("inspect partial review evidence: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("partial review evidence is not a regular file: %s", partial)
	}
	base := strings.TrimSuffix(partial, ".md") + "-partial-" + at.Format("20060102T150405.000000000Z")
	archive := base + ".md"
	for collision := 2; ; collision++ {
		if _, err := os.Lstat(archive); errors.Is(err, os.ErrNotExist) {
			break
		} else if err != nil {
			return "", "", fmt.Errorf("inspect partial review archive: %w", err)
		}
		archive = fmt.Sprintf("%s-%d.md", base, collision)
	}
	if err := os.Rename(partial, archive); err != nil {
		return "", "", fmt.Errorf("archive partial review evidence: %w", err)
	}
	return partial, archive, nil
}

func refreshPreparedRecovery(dir string, s State) (State, error) {
	if s.ActiveRecovery == nil || s.Phase != "recovering" {
		return s, fmt.Errorf("no prepared recovery to refresh")
	}
	partialOriginal, archive := "", ""
	var err error
	if s.ActiveRecovery.Step == "reviewing" {
		partialOriginal, archive, err = archivePartialReview(dir, s, time.Now().UTC())
		if err != nil {
			return s, err
		}
		if archive != "" {
			s.ActiveRecovery.AdditionalArchives = append(s.ActiveRecovery.AdditionalArchives, archive)
		}
	}
	observed, err := checkpoint(s, dir)
	if err != nil {
		if archive != "" {
			_ = os.Rename(archive, partialOriginal)
		}
		return s, fmt.Errorf("observe prepared recovery source and evidence: %w", err)
	}
	s.ActiveRecovery.Observed = observed
	s.ActiveRecovery.Changes = checkpointChanges(s.ActiveRecovery.Before, observed)
	var bindingErr error
	switch s.ActiveRecovery.Step {
	case "reviewing":
		if err := verifyCandidate(s); err != nil {
			bindingErr = fmt.Errorf("review recovery requires the exact candidate in a clean checkout: %w", err)
		}
	case "integrating":
		parent, err := integrationRecoveryParent(s)
		if err != nil {
			bindingErr = err
		} else if parent != s.ActiveRecovery.IntegrationParent {
			bindingErr = fmt.Errorf("integration refs changed after recovery preparation: observed %s parent, prepared for %s parent", parent, s.ActiveRecovery.IntegrationParent)
		}
	}
	s.Recoveries[len(s.Recoveries)-1] = *s.ActiveRecovery
	if bindingErr != nil {
		s.LastError = bindingErr.Error()
		if err := save(dir, s); err != nil {
			if archive != "" {
				_ = os.Rename(archive, partialOriginal)
			}
			return s, fmt.Errorf("%v; save stopped recovery: %w", bindingErr, err)
		}
		return s, bindingErr
	}
	s.LastError = fmt.Sprintf("recovery requested for failed %s step; replacement turn is not yet dispatched", s.ActiveRecovery.Step)
	if err := save(dir, s); err != nil {
		if archive != "" {
			_ = os.Rename(archive, partialOriginal)
		}
		return s, err
	}
	return s, nil
}

func prepareRecovery(dir string, s State) (State, error) {
	step, err := pendingRecoveryStep(s)
	if err != nil {
		return s, err
	}
	if s.InFlight == nil {
		return s, fmt.Errorf("stopped turn lacks its pre-turn checkpoint and process identities; inspect and resolve it manually")
	}
	if s.InFlight.Step != "" && s.InFlight.Step != step {
		return s, fmt.Errorf("stopped turn step %q conflicts with pending step %q", s.InFlight.Step, step)
	}
	if s.InFlight.Role != "" && s.InFlight.Role != recoveryRole(step) {
		return s, fmt.Errorf("stopped turn role %q conflicts with pending %s step", s.InFlight.Role, step)
	}
	if !s.InFlight.Terminated {
		inactive, err := stop.Inactive(s.InFlight.Processes)
		if err != nil {
			return s, fmt.Errorf("cannot verify prior owned processes: %w", err)
		}
		if !inactive {
			return s, fmt.Errorf("prior %s turn or an owned process remains active; recovery refused", s.InFlight.Role)
		}
	}
	observed, err := checkpoint(s, dir)
	if err != nil {
		return s, fmt.Errorf("observe recovery source and evidence: %w", err)
	}
	if step == "reviewing" {
		if err := verifyCandidate(s); err != nil {
			return s, fmt.Errorf("review recovery requires the exact candidate in a clean checkout: %w", err)
		}
	}
	recoveryParent := ""
	if step == "integrating" {
		recoveryParent, err = integrationRecoveryParent(s)
		if err != nil {
			return s, err
		}
	}
	recovery := Recovery{RequestedAt: time.Now().UTC(), PriorPhase: s.Phase, Role: recoveryRole(step), Step: step, Diagnostic: s.Diagnostics, PriorError: s.LastError, Before: s.InFlight.Checkpoint, Observed: observed, PriorThread: s.InFlight.ThreadID, IntegrationParent: recoveryParent}
	recovery.Changes = checkpointChanges(recovery.Before, recovery.Observed)
	partialOriginal := ""
	if step == "reviewing" && s.TaskID != "" {
		partialOriginal, recovery.ArchivedEvidence, err = archivePartialReview(dir, s, recovery.RequestedAt)
		if err != nil {
			return s, err
		}
	}
	s.Recoveries = append(s.Recoveries, recovery)
	s.ActiveRecovery = &s.Recoveries[len(s.Recoveries)-1]
	s.PendingStep = step
	s.Phase = "recovering"
	s.LastError = fmt.Sprintf("recovery requested for failed %s step; replacement turn is not yet dispatched", step)
	if s.Threads == nil {
		s.Threads = make(map[string]string)
	}
	if s.RoleTurns == nil {
		s.RoleTurns = make(map[string]bool)
	}
	s.Threads[string(recovery.Role)] = ""
	s.RoleTurns[string(recovery.Role)] = false
	s.InFlight = nil
	if err := save(dir, s); err != nil {
		if partialOriginal != "" {
			if rollbackErr := os.Rename(recovery.ArchivedEvidence, partialOriginal); rollbackErr != nil {
				return s, fmt.Errorf("save recovery: %v; restore partial review evidence: %w", err, rollbackErr)
			}
		}
		return s, err
	}
	return s, nil
}

func Run(ctx context.Context, o Options, open func(context.Context, string, io.Writer) (Runner, error)) (State, error) {
	repo, err := canonical(o.Repository)
	if err != nil {
		return State{}, err
	}
	dir, err := storage(repo, o.Root)
	if err != nil {
		return State{}, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return State{}, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "run.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return State{}, err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return State{}, fmt.Errorf("another Concoct run is active")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	s, err := read(dir)
	if err != nil {
		return s, err
	}
	if s.Repository != "" && s.Repository != repo {
		return s, fmt.Errorf("state belongs to %s", s.Repository)
	}
	hasLimitIncrease := o.ApproveInvocationLimitIncrease > 0 || o.ApproveReviewLimitIncrease > 0
	if o.Approve != "" && o.Answer != "" {
		return s, fmt.Errorf("approval and blocker answer cannot be combined")
	}
	if hasLimitIncrease && (o.Approve != "" || o.Answer != "") {
		return s, fmt.Errorf("limit increases cannot be combined with approval or blocker answer")
	}
	if o.Recover && (o.Approve != "" || o.Answer != "" || hasLimitIncrease) {
		return s, fmt.Errorf("recovery cannot be combined with approval, blocker answer or limit increase")
	}
	if hasLimitIncrease && (s.Phase == "" || s.Phase == "no_suitable_work" || s.Phase == "completed" || s.Phase == "integrated") {
		return s, fmt.Errorf("no active work has execution limits to increase")
	}
	if s.Phase == "integrated" {
		completed, finishErr := finishCompletion(dir, s)
		if finishErr != nil {
			s.LastError = "completion record maintenance failed: " + finishErr.Error()
			_ = save(dir, s)
			return s, fmt.Errorf("%s", s.LastError)
		}
		return completed, nil
	}
	if s.Phase == "no_suitable_work" {
		s = State{}
	}
	if s.Phase == "" {
		selected, capabilities, err := selectRoadmap(repo, o.Roadmap)
		if err != nil {
			return s, err
		}
		s.Roadmap, s.Capabilities = selected, capabilities
	} else {
		if s.Roadmap == "" {
			selected, capabilities, err := selectRoadmap(repo, "")
			if err != nil {
				return s, err
			}
			s.Roadmap, s.Capabilities = selected, capabilities
		}
		if o.Roadmap != "" {
			selected, err := resolvePath(repo, o.Roadmap)
			if err != nil {
				return s, fmt.Errorf("resolve roadmap %q: %w", o.Roadmap, err)
			}
			if selected != s.Roadmap {
				return s, fmt.Errorf("active work uses roadmap %s; cannot switch to %s", s.Roadmap, selected)
			}
		}
		if err := validateRoadmap(s.Roadmap, s.Capabilities); err != nil {
			return s, err
		}
	}
	if s.Phase != "" && !o.Recover && (o.ReagentModel != "" || o.IragentModel != "" || o.ReagentEffort != "" || o.IragentEffort != "" || o.MaxAttempts > 0 || o.MaxAssessments > 0 || o.TurnTimeout > 0 || o.StallTimeout > 0) {
		return s, fmt.Errorf("role settings and limits cannot change an active run")
	}
	if o.Recover && (o.MaxAttempts > 0 || o.MaxAssessments > 0 || o.TurnTimeout > 0 || o.StallTimeout > 0) {
		return s, fmt.Errorf("recovery may override only the replacement role model and effort")
	}
	if s.ActiveRecovery != nil && s.InFlight == nil && (s.Phase == "engineering" || s.Phase == "reviewing" || s.Phase == "correcting" || s.Phase == "integrating") {
		// Normalize records written by the initial CON-077 candidate before the
		// dedicated stopped recovery phase was introduced.
		s.Phase = "recovering"
		s.PendingStep = s.ActiveRecovery.Step
		s.LastError = fmt.Sprintf("recovery requested for failed %s step; replacement turn is not yet dispatched", s.ActiveRecovery.Step)
		if err := save(dir, s); err != nil {
			return s, err
		}
		if !o.Recover {
			return s, nil
		}
		if err := validateRecoveryOverrideRole(s, o); err != nil {
			return s, err
		}
	}
	if s.InFlight != nil && (s.Phase == "engineering" || s.Phase == "reviewing" || s.Phase == "correcting" || s.Phase == "integrating") {
		if s.PendingStep == "" {
			s.PendingStep = s.Phase
		}
		s.Phase = "execution_uncertain"
		s.LastError = fmt.Sprintf("%s turn in thread %s has no recorded outcome; inspect source and external evidence before any further dispatch", s.InFlight.Role, s.InFlight.ThreadID)
		if err := save(dir, s); err != nil {
			return s, err
		}
		if !o.Recover {
			return s, nil
		}
	}
	if s.MaxAssessments == 0 {
		s.MaxAssessments = 3
	}
	if hasLimitIncrease {
		if err := validateLimitIncreases(s, o); err != nil {
			return s, err
		}
	}
	// CON-064 records can continue from an exact completed review boundary.
	if s.Phase == "reviewed" {
		if s.Verdict == "approved" {
			s.ApprovedCommit = s.Candidate
			s.Phase = "awaiting_approval"
		} else if s.Verdict == "changes_requested" {
			if s.Assessments == 0 {
				s.Assessments = 1
			}
			s.Phase = "correcting"
		} else {
			return halt(dir, s, "invalid_result", fmt.Errorf("reviewed state lacks a verdict"))
		}
		if err := save(dir, s); err != nil {
			return s, err
		}
	}
	if s.Phase == "awaiting_approval" && s.Attempts >= s.MaxAttempts {
		setLimitExhausted(&s, "awaiting_approval")
		if err := save(dir, s); err != nil {
			return s, err
		}
	}
	if hasLimitIncrease {
		pending := ""
		if s.Phase == "limit_exhausted" {
			pending, err = pendingLimitPhase(s)
			if err != nil {
				return s, err
			}
		}
		authorizeLimitIncreases(&s, o)
		if s.Phase == "limit_exhausted" {
			next := pending
			if s.ActiveRecovery != nil && s.ActiveRecovery.Step == pending {
				next = "recovering"
			}
			s.Phase, s.LimitPhase, s.LastError = next, "", ""
			if limitsExhausted(s, pending) {
				setLimitExhausted(&s, pending)
			} else if next == "recovering" {
				s.LastError = fmt.Sprintf("recovery limit increase authorized for failed %s step; run --recover to revalidate and dispatch the replacement turn", pending)
			}
		}
		// Authorization is durable before any provider is opened or turn dispatched.
		if err := save(dir, s); err != nil {
			return s, err
		}
	}
	switch s.Phase {
	case "integration_failed", "execution_failed", "execution_uncertain", "invalid_result", "source_changed", "limit_exhausted", "recovering":
		if o.Recover {
			if s.Phase != "execution_failed" && s.Phase != "execution_uncertain" && s.Phase != "recovering" {
				return s, fmt.Errorf("--recover applies only to execution-failed or execution-uncertain turns")
			}
			if err := validateRecoveryOverrideRole(s, o); err != nil {
				return s, err
			}
			if s.Phase != "recovering" {
				s, err = prepareRecovery(dir, s)
				if err != nil {
					return s, err
				}
			}
			recordRecoveryOverride(&s, o)
			s, err = refreshPreparedRecovery(dir, s)
			if err != nil {
				return s, err
			}
			break
		}
		if o.Approve != "" || o.Answer != "" {
			return s, fmt.Errorf("active run is stopped at %s", s.Phase)
		}
		return s, nil
	}
	if s.Phase == "" {
		if o.Approve != "" || o.Answer != "" || hasLimitIncrease || o.Recover {
			return s, fmt.Errorf("no active work to approve or answer")
		}
		branch, err := git(repo, "symbolic-ref", "--quiet", "--short", "HEAD")
		if err != nil {
			return s, fmt.Errorf("new work requires a named branch: %w", err)
		}
		dirty, err := git(repo, "status", "--porcelain")
		if err != nil {
			return s, err
		}
		if dirty != "" {
			return s, fmt.Errorf("new work requires a clean checkout on %s", branch)
		}
		base, err := git(repo, "rev-parse", "HEAD")
		if err != nil {
			return s, err
		}
		s = State{Repository: repo, Roadmap: s.Roadmap, Capabilities: s.Capabilities, Parent: branch, Base: base, Phase: "engineering", Profiles: map[dispatch.Role]codexapp.RoleSettings{}, Threads: map[string]string{}, RoleTurns: map[string]bool{}, MaxAttempts: 12, MaxAssessments: 3, TurnTimeout: o.TurnTimeout, StallTimeout: o.StallTimeout}
		if o.MaxAttempts > 0 {
			s.MaxAttempts = o.MaxAttempts
		}
		if o.MaxAssessments > 0 {
			s.MaxAssessments = o.MaxAssessments
		}
		if err = save(dir, s); err != nil {
			return s, err
		}
	}
	if o.Approve != "" {
		if s.Phase != "awaiting_approval" || s.ApprovedCommit == "" || o.Approve != s.ApprovedCommit {
			return s, fmt.Errorf("approval does not match the exact independently approved candidate")
		}
		s.AuthorizedCommit = o.Approve
		s.Phase = "integrating"
		if err = save(dir, s); err != nil {
			return s, err
		}
	}
	if o.Answer != "" {
		if s.Phase != "blocked" || s.Question == nil || s.BlockedPhase == "" {
			return s, fmt.Errorf("no active blocker is awaiting an answer")
		}
		if strings.TrimSpace(o.Answer) == "" {
			return s, fmt.Errorf("answer must not be empty")
		}
		s.Answer = o.Answer
		s.Phase = s.BlockedPhase
		if err = save(dir, s); err != nil {
			return s, err
		}
	}
	if s.Phase == "blocked" || s.Phase == "awaiting_approval" {
		return s, nil
	}
	if s.Phase != "engineering" && s.Phase != "reviewing" && s.Phase != "correcting" && s.Phase != "integrating" && !(s.Phase == "recovering" && s.ActiveRecovery != nil) {
		return s, fmt.Errorf("unsupported active phase %q", s.Phase)
	}
	pendingPhase := s.Phase
	if s.Phase == "recovering" {
		pendingPhase = s.ActiveRecovery.Step
	}
	if limitsExhausted(s, pendingPhase) {
		setLimitExhausted(&s, pendingPhase)
		_ = save(dir, s)
		return s, nil
	}
	diagnosticFile, err := os.CreateTemp(dir, "provider-*.log")
	if err != nil {
		return s, err
	}
	diagnosticPath := diagnosticFile.Name()
	if err := diagnosticFile.Close(); err != nil {
		return s, err
	}
	diagnosticFile, err = os.OpenFile(diagnosticPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return s, err
	}
	defer diagnosticFile.Close()
	s.Diagnostics = diagnosticPath
	if err := save(dir, s); err != nil {
		return s, err
	}
	runner, err := open(ctx, o.Codex, diagnosticFile)
	if err != nil {
		_ = recordDiagnostic(s.Diagnostics, "provider", err)
		return recoverySetupError(dir, s, err)
	}
	defer runner.Close()
	if len(s.Profiles) == 0 {
		models, err := runner.Models(ctx)
		if err != nil {
			return recoverySetupError(dir, s, err)
		}
		s.Profiles, err = dispatch.ResolveProfiles(models, dispatch.ProfileSources{Invocation: map[dispatch.Role]dispatch.Choice{dispatch.Reagent: {Model: o.ReagentModel, Effort: o.ReagentEffort}, dispatch.Iragent: {Model: o.IragentModel, Effort: o.IragentEffort}}})
		if err != nil {
			return recoverySetupError(dir, s, err)
		}
		if err = save(dir, s); err != nil {
			return s, err
		}
	}
	if s.ActiveRecovery != nil {
		role := s.ActiveRecovery.Role
		if s.ActiveRecovery.RequestedModel != "" || s.ActiveRecovery.RequestedEffort != "" {
			models, err := runner.Models(ctx)
			if err != nil {
				return recoverySetupError(dir, s, err)
			}
			project := make(map[dispatch.Role]dispatch.Choice, len(s.Profiles))
			for configuredRole, profile := range s.Profiles {
				project[configuredRole] = dispatch.Choice{Model: profile.Model, Effort: profile.Effort}
			}
			resolved, err := dispatch.ResolveProfiles(models, dispatch.ProfileSources{Project: project, Invocation: map[dispatch.Role]dispatch.Choice{role: {Model: s.ActiveRecovery.RequestedModel, Effort: s.ActiveRecovery.RequestedEffort}}})
			if err != nil {
				return recoverySetupError(dir, s, err)
			}
			s.ActiveRecovery.PriorProfile = s.Profiles[role]
			s.Profiles[role] = resolved[role]
			s.ActiveRecovery.ReplacementProfile = resolved[role]
			s.Recoveries[len(s.Recoveries)-1] = *s.ActiveRecovery
			if err := save(dir, s); err != nil {
				return s, err
			}
		} else {
			s.ActiveRecovery.PriorProfile = s.Profiles[role]
			s.ActiveRecovery.ReplacementProfile = s.Profiles[role]
			s.Recoveries[len(s.Recoveries)-1] = *s.ActiveRecovery
			if err := save(dir, s); err != nil {
				return s, err
			}
		}
	}
	if o.Recover && o.OnRecovery != nil {
		o.OnRecovery(s)
	}
	for {
		step := s.Phase
		if s.Phase == "recovering" && s.ActiveRecovery != nil {
			step = s.ActiveRecovery.Step
		}
		if limitsExhausted(s, step) {
			setLimitExhausted(&s, step)
			_ = save(dir, s)
			return s, nil
		}
		role := dispatch.Reagent
		if step == "reviewing" {
			role = dispatch.Iragent
		}
		s, err = turn(ctx, runner, dir, s, role, step, o)
		if err != nil {
			return s, err
		}
		if o.OnResult != nil && s.LastResult != nil {
			o.OnResult(string(role), *s.LastResult)
		}
		if s.Phase == "integrated" {
			completed, finishErr := finishCompletion(dir, s)
			if finishErr != nil {
				s.LastError = "completion record maintenance failed: " + finishErr.Error()
				_ = save(dir, s)
				return s, fmt.Errorf("%s", s.LastError)
			}
			return completed, nil
		}
		if s.Phase != "reviewing" && s.Phase != "correcting" {
			return s, nil
		}
	}
}

func recordRecoveryProcesses(inFlight *InFlight, identities []stop.Identity) {
	seen := make(map[stop.Identity]bool, len(inFlight.Processes)+len(identities))
	for _, identity := range inFlight.Processes {
		seen[identity] = true
	}
	for _, identity := range identities {
		if identity.PID > 0 && identity.Start > 0 && !seen[identity] {
			inFlight.Processes = append(inFlight.Processes, identity)
			seen[identity] = true
		}
	}
}

func recoverySetupError(dir string, s State, err error) (State, error) {
	if s.ActiveRecovery == nil || s.InFlight != nil {
		return s, err
	}
	s.Phase = "recovering"
	s.PendingStep = s.ActiveRecovery.Step
	s.LastError = "recovery setup failed before replacement dispatch: " + err.Error()
	if saveErr := save(dir, s); saveErr != nil {
		return s, fmt.Errorf("%v; save stopped recovery state: %w", err, saveErr)
	}
	return s, err
}

func validateLimitIncreases(s State, o Options) error {
	if o.ApproveReviewLimitIncrease > 0 && o.ApproveReviewLimitIncrease <= s.MaxAssessments {
		return fmt.Errorf("review limit increase must be greater than recorded total %d", s.MaxAssessments)
	}
	if o.ApproveInvocationLimitIncrease > 0 && o.ApproveInvocationLimitIncrease <= s.MaxAttempts {
		return fmt.Errorf("invocation limit increase must be greater than recorded total %d", s.MaxAttempts)
	}
	return nil
}

func authorizeLimitIncreases(s *State, o Options) {
	now := time.Now().UTC()
	if o.ApproveReviewLimitIncrease > 0 {
		s.LimitIncreases = append(s.LimitIncreases, LimitIncrease{Allowance: "review", PreviousTotal: s.MaxAssessments, AuthorizedTotal: o.ApproveReviewLimitIncrease, AuthorizedAt: now})
		s.MaxAssessments = o.ApproveReviewLimitIncrease
	}
	if o.ApproveInvocationLimitIncrease > 0 {
		s.LimitIncreases = append(s.LimitIncreases, LimitIncrease{Allowance: "invocation", PreviousTotal: s.MaxAttempts, AuthorizedTotal: o.ApproveInvocationLimitIncrease, AuthorizedAt: now})
		s.MaxAttempts = o.ApproveInvocationLimitIncrease
	}
}

func limitsExhausted(s State, pending string) bool {
	return s.Attempts >= s.MaxAttempts || ((pending == "reviewing" || pending == "correcting") && s.Assessments >= s.MaxAssessments)
}

func setLimitExhausted(s *State, pending string) {
	s.Phase = "limit_exhausted"
	s.LimitPhase = pending
	issues := make([]string, 0, 2)
	if s.Attempts >= s.MaxAttempts {
		issues = append(issues, fmt.Sprintf("invocation limit exhausted (%d/%d consumed)", s.Attempts, s.MaxAttempts))
	}
	if (pending == "reviewing" || pending == "correcting") && s.Assessments >= s.MaxAssessments {
		issues = append(issues, fmt.Sprintf("review limit exhausted (%d/%d completed independent assessments)", s.Assessments, s.MaxAssessments))
	}
	if pending == "awaiting_approval" && len(issues) == 1 {
		s.LastError = "approved candidate retained, but " + issues[0] + " leaves no integration turn"
		return
	}
	s.LastError = strings.Join(issues, "; ")
}

// pendingLimitPhase supports records written before LimitPhase was retained.
// It relies only on structured workflow bindings and refuses ambiguous shapes.
func pendingLimitPhase(s State) (string, error) {
	if s.LimitPhase != "" {
		if s.LimitPhase != "engineering" && s.LimitPhase != "reviewing" && s.LimitPhase != "correcting" && s.LimitPhase != "integrating" && s.LimitPhase != "awaiting_approval" {
			return "", fmt.Errorf("limit-exhausted state has unsupported pending phase %q", s.LimitPhase)
		}
		return s.LimitPhase, nil
	}
	if s.AuthorizedCommit != "" && s.AuthorizedCommit == s.ApprovedCommit {
		return "integrating", nil
	}
	if s.Verdict == "approved" && s.ApprovedCommit != "" && s.ApprovedCommit == s.Candidate {
		return "awaiting_approval", nil
	}
	if s.Verdict == "changes_requested" && s.Candidate != "" {
		return "correcting", nil
	}
	if s.Candidate != "" && s.TaskID != "" {
		return "reviewing", nil
	}
	if s.TaskID == "" && s.Candidate == "" {
		return "engineering", nil
	}
	return "", fmt.Errorf("limit-exhausted state does not identify a pending workflow step")
}

func completionPath(dir string, s State) string {
	key := shaPath(s.Repository + "\x00" + s.TaskID + "\x00" + s.Candidate)
	return filepath.Join(completionsPath(dir), "completion-"+key+".json")
}

func finishCompletion(dir string, s State) (State, error) {
	if s.Phase != "integrated" || s.TaskID == "" || s.Candidate == "" || s.ApprovedCommit != s.Candidate || s.AuthorizedCommit != s.Candidate || s.LastResult == nil || s.LastResult.Disposition != "integrated" {
		return s, fmt.Errorf("integrated state lacks an accepted completion binding")
	}
	path := completionPath(dir, s)
	if data, err := os.ReadFile(path); err == nil {
		var recorded State
		if err := json.Unmarshal(data, &recorded); err != nil {
			return s, fmt.Errorf("read existing completion: %w", err)
		}
		if recorded.Phase != "completed" || recorded.Repository != s.Repository || recorded.TaskID != s.TaskID || recorded.Candidate != s.Candidate || recorded.CompletionRecord != path {
			return s, fmt.Errorf("existing completion record does not match integrated work")
		}
		s = recorded
	} else if !errors.Is(err, os.ErrNotExist) {
		return s, err
	} else {
		s.Phase = "completed"
		s.LastError = ""
		s.CompletionRecord = path
		s.CompletedAt = time.Now().UTC()
		s.Updated = s.CompletedAt
		if err := os.MkdirAll(completionsPath(dir), 0700); err != nil {
			return s, err
		}
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return s, err
		}
		if err := writeAtomic(path, append(data, '\n')); err != nil {
			return s, err
		}
	}
	if err := save(dir, State{}); err != nil {
		return s, fmt.Errorf("release active state: %w", err)
	}
	return s, nil
}

func latestCompletion(dir, repo string) (State, error) {
	entries, err := os.ReadDir(completionsPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var latest State
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "completion-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(completionsPath(dir), entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return State{}, err
		}
		var completed State
		if err := json.Unmarshal(data, &completed); err != nil {
			return State{}, fmt.Errorf("read completion %s: %w", path, err)
		}
		if completed.Phase != "completed" || completed.Repository != repo || completed.CompletionRecord != path || completed.CompletedAt.IsZero() {
			return State{}, fmt.Errorf("invalid completion record %s", path)
		}
		if latest.CompletedAt.Before(completed.CompletedAt) {
			latest = completed
		}
	}
	return latest, nil
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".write-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func turn(ctx context.Context, runner Runner, dir string, s State, role dispatch.Role, step string, o Options) (State, error) {
	if s.Attempts >= s.MaxAttempts {
		setLimitExhausted(&s, step)
		_ = save(dir, s)
		return s, nil
	}
	prompt, err := promptForStep(s, dir, step)
	if err != nil {
		return recoverySetupError(dir, s, err)
	}
	preTurn, err := checkpoint(s, dir)
	if err != nil {
		return recoverySetupError(dir, s, fmt.Errorf("record pre-turn checkpoint: %w", err))
	}
	settings := s.Profiles[role]
	var thread codexapp.Thread
	if id := s.Threads[string(role)]; id != "" {
		thread, err = runner.ResumeThread(ctx, id, s.RoleTurns[string(role)], settings)
	}
	if s.Threads[string(role)] == "" || err != nil {
		thread, err = runner.StartThread(ctx, s.Repository, settings, nil)
	}
	if err != nil {
		return recoverySetupError(dir, s, err)
	}
	s.Threads[string(role)] = thread.ID
	s.Attempts++
	s.PendingStep = step
	s.Phase = step
	s.LastError = ""
	s.InFlight = &InFlight{Role: role, ThreadID: thread.ID, Step: step, Checkpoint: preTurn}
	if recorder, ok := runner.(processRecorder); ok {
		recordRecoveryProcesses(s.InFlight, recorder.RecoveryProcesses())
	}
	if s.ActiveRecovery != nil {
		s.ActiveRecovery.ReplacementThread = thread.ID
		s.ActiveRecovery.DispatchAttempt = s.Attempts
		s.Recoveries[len(s.Recoveries)-1] = *s.ActiveRecovery
	}
	if err = save(dir, s); err != nil {
		return s, err
	}
	turnCtx := ctx
	var activity chan struct{}
	cancel := func() {}
	if s.TurnTimeout > 0 {
		turnCtx, cancel = context.WithTimeout(ctx, s.TurnTimeout)
	}
	defer cancel()
	if s.StallTimeout > 0 {
		var stallCancel context.CancelFunc
		turnCtx, stallCancel = context.WithCancel(turnCtx)
		defer stallCancel()
		activity = make(chan struct{}, 1)
		go func() {
			timer := time.NewTimer(s.StallTimeout)
			defer timer.Stop()
			for {
				select {
				case <-turnCtx.Done():
					return
				case <-activity:
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(s.StallTimeout)
				case <-timer.C:
					stallCancel()
					return
				}
			}
		}()
	}
	turnID := ""
	var inFlightMu sync.Mutex
	sampleDone := make(chan struct{})
	sampleStopped := make(chan struct{})
	if recorder, ok := runner.(processRecorder); ok {
		go func() {
			defer close(sampleStopped)
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-sampleDone:
					return
				case <-ticker.C:
					identities := recorder.RecoveryProcesses()
					inFlightMu.Lock()
					before := len(s.InFlight.Processes)
					recordRecoveryProcesses(s.InFlight, identities)
					if len(s.InFlight.Processes) != before {
						_ = save(dir, s)
					}
					inFlightMu.Unlock()
				}
			}
		}()
	} else {
		close(sampleStopped)
	}
	samplingStopped := false
	stopSampling := func() {
		if !samplingStopped {
			close(sampleDone)
			<-sampleStopped
			samplingStopped = true
		}
	}
	defer stopSampling()
	outcome, runErr := runner.RunRoleTurn(turnCtx, thread.ID, prompt, settings, func(event codexapp.Event) {
		inFlightMu.Lock()
		changed := false
		if event.TurnID != "" && event.TurnID != s.InFlight.TurnID {
			turnID = event.TurnID
			s.InFlight.TurnID = turnID
			changed = true
		}
		if recorder, ok := runner.(processRecorder); ok {
			before := len(s.InFlight.Processes)
			recordRecoveryProcesses(s.InFlight, recorder.RecoveryProcesses())
			changed = changed || len(s.InFlight.Processes) != before
		}
		if changed {
			_ = save(dir, s)
		}
		inFlightMu.Unlock()
		if activity != nil {
			select {
			case activity <- struct{}{}:
			default:
			}
		}
		if o.OnEvent != nil {
			o.OnEvent(string(role), event)
		}
	})
	stopSampling()
	if runErr != nil {
		s.Phase = "execution_failed"
		_ = recordDiagnostic(s.Diagnostics, string(role), runErr)
		if turnID == "" {
			turnID = outcome.TurnID
		}
		s.InFlight.TurnID = turnID
		if recorder, ok := runner.(processRecorder); ok {
			recordRecoveryProcesses(s.InFlight, recorder.RecoveryProcesses())
		}
		if errors.Is(turnCtx.Err(), context.DeadlineExceeded) || errors.Is(turnCtx.Err(), context.Canceled) {
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 12*time.Second)
			stopped := runner.Stop(stopCtx, thread.ID, turnID)
			stopCancel()
			s.LastError = fmt.Sprintf("%s execution stopped: %s", role, stopped.Status)
			if stopped.Problem != "" {
				s.LastError += ": " + stopped.Problem
			}
			s.InFlight.Terminated = stopped.Status == "stopped_safely"
		} else if errors.Is(runErr, codexapp.ErrInvalidResponseSchema) {
			s.LastError = "Concoct supplied an invalid response schema; correct Concoct's code before retrying"
		} else {
			s.LastError = fmt.Sprintf("%s execution failed; inspect diagnostics", role)
		}
		_ = save(dir, s)
		return s, fmt.Errorf("%s", s.LastError)
	}
	s.InFlight = nil
	s.PendingStep = ""
	s.ActiveRecovery = nil
	if s.RoleTurns == nil {
		s.RoleTurns = make(map[string]bool)
	}
	s.RoleTurns[string(role)] = true
	result, err := codexapp.ParseRoleResult(outcome.FinalText)
	if err != nil {
		return halt(dir, s, "invalid_result", fmt.Errorf("%s returned an invalid structured result", role))
	}
	return routeResult(dir, s, role, step, result)
}

func routeResult(dir string, s State, role dispatch.Role, step string, result codexapp.RoleResult) (State, error) {
	s.LastResult = &result
	s.LastRole = role
	s.Summary = result.Summary
	s.LastError = ""
	if result.Disposition == "decision_required" || result.Disposition == "blocker" {
		s.Phase = "blocked"
		s.Question = result.Question
		s.QuestionRole = role
		s.BlockedPhase = step
		s.Answer = ""
		if err := save(dir, s); err != nil {
			return s, err
		}
		return s, nil
	}
	s.Question, s.QuestionRole, s.BlockedPhase, s.Answer = nil, "", "", ""
	switch step {
	case "engineering", "correcting":
		switch result.Disposition {
		case "no_suitable_work":
			if step != "engineering" {
				return halt(dir, s, "invalid_result", fmt.Errorf("correction cannot report no suitable work"))
			}
			s.Phase = "no_suitable_work"
		case "ready_for_review":
			if result.TaskID == "" || result.Candidate == "" {
				return halt(dir, s, "invalid_result", fmt.Errorf("ready result requires task ID and candidate commit"))
			}
			if step == "correcting" && (result.TaskID != s.TaskID || result.Candidate == s.Candidate) {
				return halt(dir, s, "invalid_result", fmt.Errorf("correction requires same task and a distinct candidate"))
			}
			priorBranch := s.Branch
			s.TaskID = result.TaskID
			s.Candidate = result.Candidate
			if step == "engineering" {
				s.Branch, _ = git(s.Repository, "symbolic-ref", "--quiet", "--short", "HEAD")
			} else {
				s.Branch = priorBranch
			}
			if err := verifyCandidate(s); err != nil {
				return halt(dir, s, "source_changed", err)
			}
			evidence := taskEvidencePath(dir, s.Roadmap, s.TaskID)
			if result.Evidence != "" && result.Evidence != evidence {
				return halt(dir, s, "invalid_result", fmt.Errorf("task evidence must be the selected roadmap task file %s", evidence))
			}
			if err := verifyEvidence(s.Repository, evidence); err != nil {
				return halt(dir, s, "invalid_result", fmt.Errorf("task evidence: %w", err))
			}
			s.Evidence = evidence
			s.ReagentSummary = result.Summary
			s.ReviewEvidence, s.ReviewSummary, s.Findings, s.Verdict, s.ApprovedCommit = "", "", nil, "", ""
			s.Phase = "reviewing"
		default:
			return halt(dir, s, "invalid_result", fmt.Errorf("unexpected reagent disposition %q", result.Disposition))
		}
	case "reviewing":
		if result.TaskID != s.TaskID || result.Candidate != s.Candidate {
			return halt(dir, s, "invalid_result", fmt.Errorf("review result identity differs from candidate"))
		}
		if result.Disposition != "approved" && result.Disposition != "changes_requested" {
			return halt(dir, s, "invalid_result", fmt.Errorf("unexpected iragent disposition %q", result.Disposition))
		}
		reviewEvidence := reviewPath(dir, s.Roadmap, s.TaskID, s.Assessments+1)
		if result.Evidence != "" && result.Evidence != reviewEvidence {
			return halt(dir, s, "invalid_result", fmt.Errorf("review evidence must be the current assessment file %s", reviewEvidence))
		}
		if err := verifyCandidate(s); err != nil {
			return halt(dir, s, "source_changed", err)
		}
		if err := verifyEvidence(s.Repository, reviewEvidence); err != nil {
			return halt(dir, s, "invalid_result", fmt.Errorf("review evidence: %w", err))
		}
		if s.Assessments > 0 {
			current, err := os.Stat(reviewEvidence)
			if err != nil {
				return halt(dir, s, "invalid_result", fmt.Errorf("current review evidence: %w", err))
			}
			for number := 1; number <= s.Assessments; number++ {
				previous, err := os.Stat(reviewPath(dir, s.Roadmap, s.TaskID, number))
				if err != nil {
					return halt(dir, s, "invalid_result", fmt.Errorf("prior review evidence %02d: %w", number, err))
				}
				if os.SameFile(current, previous) {
					return halt(dir, s, "invalid_result", fmt.Errorf("current review evidence reuses assessment file %02d", number))
				}
			}
		}
		s.ReviewEvidence = reviewEvidence
		s.ReviewSummary = result.Summary
		s.Findings = append([]string(nil), result.Findings...)
		s.Assessments++
		s.Verdict = result.Disposition
		if result.Disposition == "approved" {
			s.ApprovedCommit = s.Candidate
			if s.Attempts >= s.MaxAttempts {
				setLimitExhausted(&s, "awaiting_approval")
			} else {
				s.Phase = "awaiting_approval"
			}
		} else {
			s.Phase = "correcting"
		}
	case "integrating":
		if result.TaskID != s.TaskID || result.Candidate != s.AuthorizedCommit || s.AuthorizedCommit == "" || s.AuthorizedCommit != s.ApprovedCommit {
			return halt(dir, s, "invalid_result", fmt.Errorf("integration result identity differs from authorized candidate"))
		}
		switch result.Disposition {
		case "integrated":
			s.Phase = "integrated"
		case "integration_failed":
			s.Phase = "integration_failed"
		default:
			return halt(dir, s, "invalid_result", fmt.Errorf("unexpected integration disposition %q", result.Disposition))
		}
	default:
		return halt(dir, s, "invalid_result", fmt.Errorf("unsupported step %q", step))
	}
	if err := save(dir, s); err != nil {
		return s, err
	}
	return s, nil
}

// Evidence is agent-owned Markdown. Only its external location and readable
// regular-file presence are checked; Concoct does not parse its narrative.
func verifyEvidence(repo, path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path %q is not absolute", path)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(repo, resolved)
	if err != nil {
		return err
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q is inside the product repository", path)
	}
	f, err := os.Open(resolved)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("path %q is not a regular file", path)
	}
	var first [1]byte
	if _, err := f.Read(first[:]); err != nil && err != io.EOF {
		return err
	}
	return nil
}

// verifyCandidate binds review to the commit the reagent reported. It does
// not inspect engineering merit or documentation content.
func verifyCandidate(s State) error {
	if s.Branch == "" || s.Branch == s.Parent {
		return fmt.Errorf("candidate requires a distinct task branch")
	}
	branch, err := git(s.Repository, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return err
	}
	if branch != s.Branch {
		return fmt.Errorf("checkout branch changed from %s to %s", s.Branch, branch)
	}
	head, err := git(s.Repository, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != s.Candidate {
		return fmt.Errorf("candidate %s is not checkout HEAD %s", s.Candidate, head)
	}
	if _, err := git(s.Repository, "merge-base", "--is-ancestor", s.Base, s.Candidate); err != nil {
		return fmt.Errorf("candidate is not based on %s: %w", s.Base, err)
	}
	dirty, err := git(s.Repository, "status", "--porcelain")
	if err != nil {
		return err
	}
	if dirty != "" {
		return fmt.Errorf("candidate checkout is dirty")
	}
	return nil
}

func recordDiagnostic(path, role string, err error) error {
	f, openErr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if openErr != nil {
		return openErr
	}
	defer f.Close()
	_, writeErr := fmt.Fprintf(f, "%s [%s] %v\n", time.Now().UTC().Format(time.RFC3339Nano), role, err)
	if writeErr != nil {
		return writeErr
	}
	return f.Sync()
}

func halt(dir string, s State, phase string, err error) (State, error) {
	s.Phase = phase
	s.LastError = err.Error()
	if saveErr := save(dir, s); saveErr != nil {
		return s, saveErr
	}
	return s, err
}
