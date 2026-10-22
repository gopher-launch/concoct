package handoff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/stop"
)

// State contains coordination facts. Engineering and review prose belongs in
// task.md and numbered review files, which agents maintain with ordinary tools.
type State struct {
	Repository       string                                  `json:"repository"`
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
	Attempts         int                                     `json:"attempts"`
	MaxAttempts      int                                     `json:"max_attempts"`
	TurnTimeout      time.Duration                           `json:"turn_timeout,omitempty"`
	StallTimeout     time.Duration                           `json:"stall_timeout,omitempty"`
	LastResult       *codexapp.RoleResult                    `json:"last_result,omitempty"`
	LastRole         dispatch.Role                           `json:"last_role,omitempty"`
	LastError        string                                  `json:"last_error,omitempty"`
	Diagnostics      string                                  `json:"diagnostics,omitempty"`
	Updated          time.Time                               `json:"updated"`
}

// InFlight is saved with the invocation count before dispatch. A later run
// cannot assume an unrecorded provider outcome means the turn never ran.
type InFlight struct {
	Role     dispatch.Role `json:"role"`
	ThreadID string        `json:"thread_id"`
}

type Options struct {
	Repository, Root, Codex                                  string
	Approve, Answer                                          string
	ReagentModel, IragentModel, ReagentEffort, IragentEffort string
	MaxAttempts, MaxAssessments                              int
	TurnTimeout, StallTimeout                                time.Duration
	OnEvent                                                  func(string, codexapp.Event)
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
func statePath(dir string) string { return filepath.Join(dir, "state.json") }
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
	if o.Approve != "" && o.Answer != "" {
		return s, fmt.Errorf("approval and blocker answer cannot be combined")
	}
	if s.Phase == "no_suitable_work" {
		s = State{}
	}
	if s.Phase != "" && (o.ReagentModel != "" || o.IragentModel != "" || o.ReagentEffort != "" || o.IragentEffort != "" || o.MaxAttempts > 0 || o.MaxAssessments > 0 || o.TurnTimeout > 0 || o.StallTimeout > 0) {
		return s, fmt.Errorf("role settings and limits cannot change an active run")
	}
	if s.InFlight != nil && (s.Phase == "engineering" || s.Phase == "reviewing" || s.Phase == "correcting" || s.Phase == "integrating") {
		s.Phase = "execution_uncertain"
		s.LastError = fmt.Sprintf("%s turn in thread %s has no recorded outcome; inspect source and external evidence before any further dispatch", s.InFlight.Role, s.InFlight.ThreadID)
		if err := save(dir, s); err != nil {
			return s, err
		}
		return s, nil
	}
	if s.MaxAssessments == 0 {
		s.MaxAssessments = 3
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
		s.Phase = "limit_exhausted"
		s.LastError = "approved candidate retained, but invocation limit leaves no integration turn"
		if err := save(dir, s); err != nil {
			return s, err
		}
	}
	switch s.Phase {
	case "integrated", "integration_failed", "execution_failed", "execution_uncertain", "invalid_result", "source_changed", "limit_exhausted":
		if o.Approve != "" || o.Answer != "" {
			return s, fmt.Errorf("active run is stopped at %s", s.Phase)
		}
		return s, nil
	}
	if s.Phase == "" {
		if o.Approve != "" || o.Answer != "" {
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
		s = State{Repository: repo, Parent: branch, Base: base, Phase: "engineering", Profiles: map[dispatch.Role]codexapp.RoleSettings{}, Threads: map[string]string{}, RoleTurns: map[string]bool{}, MaxAttempts: 12, MaxAssessments: 3, TurnTimeout: o.TurnTimeout, StallTimeout: o.StallTimeout}
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
	if s.Phase != "engineering" && s.Phase != "reviewing" && s.Phase != "correcting" && s.Phase != "integrating" {
		return s, fmt.Errorf("unsupported active phase %q", s.Phase)
	}
	if s.Attempts >= s.MaxAttempts || (s.Assessments >= s.MaxAssessments && (s.Phase == "reviewing" || s.Phase == "correcting")) {
		s.Phase = "limit_exhausted"
		s.LastError = "invocation or assessment limit exhausted"
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
		return s, err
	}
	defer runner.Close()
	if len(s.Profiles) == 0 {
		models, err := runner.Models(ctx)
		if err != nil {
			return s, err
		}
		s.Profiles, err = dispatch.ResolveProfiles(models, dispatch.ProfileSources{Invocation: map[dispatch.Role]dispatch.Choice{dispatch.Reagent: {Model: o.ReagentModel, Effort: o.ReagentEffort}, dispatch.Iragent: {Model: o.IragentModel, Effort: o.IragentEffort}}})
		if err != nil {
			return s, err
		}
		if err = save(dir, s); err != nil {
			return s, err
		}
	}
	for {
		if s.Attempts >= s.MaxAttempts || (s.Assessments >= s.MaxAssessments && (s.Phase == "reviewing" || s.Phase == "correcting")) {
			s.Phase = "limit_exhausted"
			s.LastError = "invocation or assessment limit exhausted"
			_ = save(dir, s)
			return s, nil
		}
		step := s.Phase
		role := dispatch.Reagent
		if step == "reviewing" {
			role = dispatch.Iragent
		}
		s, err = turn(ctx, runner, dir, s, role, step, o)
		if err != nil {
			return s, err
		}
		if s.Phase != "reviewing" && s.Phase != "correcting" {
			return s, nil
		}
	}
}

func turn(ctx context.Context, runner Runner, dir string, s State, role dispatch.Role, step string, o Options) (State, error) {
	if s.Attempts >= s.MaxAttempts {
		s.Phase = "limit_exhausted"
		s.LastError = "invocation limit exhausted"
		_ = save(dir, s)
		return s, nil
	}
	settings := s.Profiles[role]
	var thread codexapp.Thread
	var err error
	if id := s.Threads[string(role)]; id != "" {
		thread, err = runner.ResumeThread(ctx, id, s.RoleTurns[string(role)], settings)
	}
	if s.Threads[string(role)] == "" || err != nil {
		thread, err = runner.StartThread(ctx, s.Repository, settings, nil)
	}
	if err != nil {
		return s, err
	}
	s.Threads[string(role)] = thread.ID
	s.Attempts++
	s.InFlight = &InFlight{Role: role, ThreadID: thread.ID}
	if err = save(dir, s); err != nil {
		return s, err
	}
	prompt := promptForStep(s, dir, step)
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
	outcome, runErr := runner.RunRoleTurn(turnCtx, thread.ID, prompt, settings, func(event codexapp.Event) {
		if event.TurnID != "" {
			turnID = event.TurnID
		}
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
	if runErr != nil {
		s.Phase = "execution_failed"
		_ = recordDiagnostic(s.Diagnostics, string(role), runErr)
		if turnID == "" {
			turnID = outcome.TurnID
		}
		if errors.Is(turnCtx.Err(), context.DeadlineExceeded) || errors.Is(turnCtx.Err(), context.Canceled) {
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 12*time.Second)
			stopped := runner.Stop(stopCtx, thread.ID, turnID)
			stopCancel()
			s.LastError = fmt.Sprintf("%s execution stopped: %s", role, stopped.Status)
			if stopped.Problem != "" {
				s.LastError += ": " + stopped.Problem
			}
		} else if errors.Is(runErr, codexapp.ErrInvalidResponseSchema) {
			s.LastError = "Concoct supplied an invalid response schema; correct Concoct's code before retrying"
		} else {
			s.LastError = fmt.Sprintf("%s execution failed; inspect diagnostics", role)
		}
		_ = save(dir, s)
		return s, fmt.Errorf("%s", s.LastError)
	}
	s.InFlight = nil
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
			evidence := filepath.Join(dir, "evidence", s.TaskID, "task.md")
			if result.Evidence != "" {
				evidence = result.Evidence
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
		reviewEvidence := reviewPath(dir, s.TaskID, s.Assessments+1)
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
				previous, err := os.Stat(reviewPath(dir, s.TaskID, number))
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
				s.Phase = "limit_exhausted"
				s.LastError = "approved candidate retained, but invocation limit leaves no integration turn"
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
