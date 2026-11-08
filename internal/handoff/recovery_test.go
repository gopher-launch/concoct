package handoff

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/stop"
)

type recoveryRunner struct {
	repo, evidenceDir string
	models            []codexapp.Model
	disposition       string
	prompts           []string
	threads           []string
	resumed           []string
	settings          []codexapp.RoleSettings
	fail              bool
	startErr          error
	resumeOK          bool
}

func (r *recoveryRunner) Models(context.Context) ([]codexapp.Model, error) { return r.models, nil }
func (r *recoveryRunner) StartThread(_ context.Context, cwd string, settings codexapp.RoleSettings, _ []codexapp.ToolSpec) (codexapp.Thread, error) {
	if cwd != r.repo {
		return codexapp.Thread{}, errors.New("wrong repository")
	}
	if r.startErr != nil {
		return codexapp.Thread{}, r.startErr
	}
	r.settings = append(r.settings, settings)
	id := "replacement-" + settings.Role
	r.threads = append(r.threads, id)
	return codexapp.Thread{ID: id}, nil
}
func (r *recoveryRunner) ResumeThread(_ context.Context, id string, _ bool, settings codexapp.RoleSettings) (codexapp.Thread, error) {
	r.resumed = append(r.resumed, id)
	if r.resumeOK {
		r.settings = append(r.settings, settings)
		return codexapp.Thread{ID: id}, nil
	}
	return codexapp.Thread{}, errors.New("saved session unavailable")
}
func (r *recoveryRunner) RunRoleTurn(_ context.Context, id, prompt string, settings codexapp.RoleSettings, _ func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	r.prompts = append(r.prompts, prompt)
	if r.fail {
		return codexapp.TurnOutcome{ThreadID: id, TurnID: "replacement-turn", Status: "failed"}, errors.New("capacity unavailable")
	}
	result := map[string]any{"disposition": r.disposition, "summary": "replacement result", "task_id": nil, "candidate": nil, "findings": nil, "evidence": nil, "question": map[string]any{"text": "inspect preserved work?"}}
	if settings.Role == "iragent" {
		state, err := read(r.evidenceDir)
		if err != nil {
			return codexapp.TurnOutcome{}, err
		}
		path := reviewPath(r.evidenceDir, state.Roadmap, state.TaskID, state.Assessments+1)
		writeEvidence(path)
		result["disposition"], result["task_id"], result["candidate"], result["evidence"], result["question"] = "approved", state.TaskID, state.Candidate, path, nil
	}
	body, _ := json.Marshal(result)
	return codexapp.TurnOutcome{ThreadID: id, TurnID: "replacement-turn", Status: "completed", FinalText: string(body)}, nil
}
func (r *recoveryRunner) Stop(context.Context, string, string) stop.Result {
	return stop.Result{Status: "stopped_safely"}
}
func (r *recoveryRunner) Close() error { return nil }
func (r *recoveryRunner) RecoveryProcesses() []stop.Identity {
	return []stop.Identity{{PID: 999999, Start: 1}}
}

func recoveryModels() []codexapp.Model {
	models := model()
	models[0].IsDefault = false
	alt := codexapp.Model{Model: "replacement-model", IsDefault: true}
	for _, effort := range []string{"medium", "high"} {
		alt.SupportedReasoningEfforts = append(alt.SupportedReasoningEfforts, struct {
			ReasoningEffort string `json:"reasoningEffort"`
		}{effort})
	}
	return append(models, alt)
}

func seedStoppedState(t *testing.T, repo, root, phase, step string) (string, State) {
	t.Helper()
	dir, err := storage(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	base := gitTest(repo, "rev-parse", "HEAD")
	s := State{
		Repository: repo, Roadmap: filepath.Join(repo, "roadmap.md"), Parent: "main", Base: base,
		Phase: phase, PendingStep: step, Attempts: 1, MaxAttempts: 12, MaxAssessments: 3,
		Profiles: map[dispatch.Role]codexapp.RoleSettings{
			dispatch.Reagent: {Role: "reagent", Model: "test-model", Effort: "medium", Sandbox: "danger-full-access", ApprovalPolicy: "never"},
			dispatch.Iragent: {Role: "iragent", Model: "test-model", Effort: "high", Sandbox: "danger-full-access", ApprovalPolicy: "never"},
		},
		Threads: map[string]string{string(recoveryRole(step)): "failed-thread"}, RoleTurns: map[string]bool{},
	}
	before, err := checkpoint(s, dir)
	if err != nil {
		t.Fatal(err)
	}
	s.InFlight = &InFlight{Role: recoveryRole(step), ThreadID: "failed-thread", Step: step, TurnID: "failed-turn", Checkpoint: before, Terminated: true}
	return dir, s
}

func TestRecoverPreservesReagentWorkAndStartsFreshSession(t *testing.T) {
	repo, root := testRepo(t), t.TempDir()
	dir, s := seedStoppedState(t, repo, root, "execution_failed", "engineering")
	if err := os.WriteFile(filepath.Join(repo, "roadmap.md"), []byte("# roadmap\n\ntracked work in progress\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "unfinished.txt"), []byte("preserve me\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	runner := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels(), disposition: "decision_required"}
	recovered, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true, ReagentModel: "replacement-model", ReagentEffort: "high"}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err != nil || recovered.Phase != "blocked" || recovered.Attempts != 2 || len(recovered.Recoveries) != 1 || len(runner.threads) != 1 {
		t.Fatalf("recovery: %+v %v threads=%v", recovered, err, runner.threads)
	}
	if len(runner.resumed) != 1 || runner.resumed[0] != "failed-thread" {
		t.Fatalf("saved session was not attempted before fallback: %+v", runner.resumed)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "unfinished.txt")); err != nil || string(body) != "preserve me\n" {
		t.Fatalf("preserved work changed: %q %v", body, err)
	}
	if !strings.Contains(runner.prompts[0], "unfinished.txt") || !strings.Contains(runner.prompts[0], "roadmap.md") || !strings.Contains(runner.prompts[0], "explicit operator-authorized recovery") || strings.Contains(runner.prompts[0], "reset --hard") {
		t.Fatalf("recovery prompt omitted preserved changes: %s", runner.prompts[0])
	}
	audit := recovered.Recoveries[0]
	if audit.ReplacementThread != "replacement-reagent" || audit.ReplacementProfile.Model != "replacement-model" || audit.PriorProfile.Model != "test-model" {
		t.Fatalf("recovery audit/profile: %+v", audit)
	}
}

func TestRecoverOlderRecordResumesSavedSessionWithoutDiagnostics(t *testing.T) {
	repo, root := testRepo(t), t.TempDir()
	gitTest(repo, "switch", "-qc", "task/CURU-001")
	if err := os.WriteFile(filepath.Join(repo, "unfinished.txt"), []byte("preserve dirty work\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dir, s := seedStoppedState(t, repo, root, "execution_failed", "engineering")
	s.PendingStep = ""
	s.TaskID = "CURU-001"
	s.Branch = "task/CURU-001"
	s.Assessments = 1
	s.InFlight = &InFlight{Role: dispatch.Reagent, ThreadID: "failed-thread", Processes: []stop.Identity{{PID: 999999}}}
	s.Threads[string(dispatch.Reagent)] = "failed-thread"
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}

	plain, err := Run(context.Background(), Options{Repository: repo, Root: root}, func(context.Context, string, io.Writer) (Runner, error) {
		t.Fatal("plain run dispatched an agent")
		return nil, nil
	})
	if err != nil || plain.Phase != "execution_failed" || plain.PendingStep != "engineering" || plain.Attempts != 1 || plain.Assessments != 1 {
		t.Fatalf("plain run changed stopped record: %+v %v", plain, err)
	}
	status, err := Status(Options{Repository: repo, Root: root})
	if err != nil || status.PendingStep != "engineering" {
		t.Fatalf("status did not infer pending step: %+v %v", status, err)
	}

	runner := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels(), disposition: "decision_required", resumeOK: true}
	recovered, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err != nil || recovered.Phase != "blocked" || recovered.Attempts != 2 || recovered.Assessments != 1 || len(runner.resumed) != 1 || len(runner.threads) != 0 {
		t.Fatalf("older-record recovery: %+v %v resumed=%v starts=%v", recovered, err, runner.resumed, runner.threads)
	}
	if recovered.Recoveries[0].ReplacementThread != "failed-thread" || !strings.Contains(runner.prompts[0], "No pre-turn checkpoint was recorded") || !strings.Contains(runner.prompts[0], "unfinished.txt") {
		t.Fatalf("recovery did not retain session/current context: audit=%+v prompt=%s", recovered.Recoveries[0], runner.prompts[0])
	}
	if body, err := os.ReadFile(filepath.Join(repo, "unfinished.txt")); err != nil || string(body) != "preserve dirty work\n" {
		t.Fatalf("dirty work changed: %q %v", body, err)
	}
}

func TestPendingRecoveryStepInfersInitialEngineeringBeforeTaskSelection(t *testing.T) {
	step, err := pendingRecoveryStep(State{Phase: "execution_failed", Threads: map[string]string{"reagent": "saved-thread"}})
	if err != nil || step != "engineering" {
		t.Fatalf("initial engineering inference: %q %v", step, err)
	}
}

func TestReviewRecoveryArchivesPartialAndCountsOnlyReplacementResult(t *testing.T) {
	repo, root := testRepo(t), t.TempDir()
	gitTest(repo, "switch", "-qc", "task/APP-002")
	if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("candidate\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(repo, "add", "feature.txt")
	gitTest(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "candidate")
	candidate := gitTest(repo, "rev-parse", "HEAD")
	dir, s := seedStoppedState(t, repo, root, "execution_failed", "reviewing")
	s.Branch, s.TaskID, s.Candidate = "task/APP-002", "APP-002", candidate
	s.Assessments = 2
	s.InFlight.Checkpoint, _ = checkpoint(s, dir)
	writeEvidence(taskEvidencePath(dir, s.Roadmap, s.TaskID))
	writeEvidence(reviewPath(dir, s.Roadmap, s.TaskID, 1))
	writeEvidence(reviewPath(dir, s.Roadmap, s.TaskID, 2))
	partial := reviewPath(dir, s.Roadmap, s.TaskID, 3)
	writeEvidence(partial)
	if err := os.WriteFile(partial, []byte("unfinished review\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	runner := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels()}
	recovered, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true, IragentModel: "replacement-model", IragentEffort: "medium"}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err != nil || recovered.Phase != "awaiting_approval" || recovered.Assessments != 3 || recovered.Attempts != 2 || recovered.Verdict != "approved" {
		t.Fatalf("review recovery: %+v %v", recovered, err)
	}
	archive := recovered.Recoveries[0].ArchivedEvidence
	if archive == "" || filepath.Base(archive) == filepath.Base(partial) {
		t.Fatalf("partial review was not archived: %+v", recovered.Recoveries[0])
	}
	if body, err := os.ReadFile(archive); err != nil || string(body) != "unfinished review\n" {
		t.Fatalf("archived partial review: %q %v", body, err)
	}
	if _, err := os.Stat(partial); err != nil {
		t.Fatalf("replacement review missing: %v", err)
	}
}

func TestRecoverPreservesCommitCreatedByFailedReagent(t *testing.T) {
	repo, root := testRepo(t), t.TempDir()
	dir, s := seedStoppedState(t, repo, root, "execution_uncertain", "engineering")
	gitTest(repo, "switch", "-qc", "task/APP-002")
	if err := os.WriteFile(filepath.Join(repo, "committed.txt"), []byte("preserved commit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(repo, "add", "committed.txt")
	gitTest(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "partial reagent work")
	committed := gitTest(repo, "rev-parse", "HEAD")
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	runner := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels(), disposition: "decision_required"}
	recovered, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err != nil || recovered.Phase != "blocked" || gitTest(repo, "rev-parse", "HEAD") != committed {
		t.Fatalf("committed work recovery: %+v %v", recovered, err)
	}
	if !strings.Contains(recovered.Recoveries[0].Changes, "HEAD changed") || !strings.Contains(recovered.Recoveries[0].Changes, "task/APP-002") {
		t.Fatalf("committed change not recorded: %+v", recovered.Recoveries[0])
	}
}

func TestRecoverRefusesLivePriorProcessAndPlainRunStaysStopped(t *testing.T) {
	repo, root := testRepo(t), t.TempDir()
	dir, s := seedStoppedState(t, repo, root, "execution_uncertain", "engineering")
	probe, err := stop.NewProbe(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	s.InFlight.Terminated = false
	s.InFlight.Processes = []stop.Identity{probe.Root()}
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	opens := 0
	open := func(context.Context, string, io.Writer) (Runner, error) { opens++; return &recoveryRunner{}, nil }
	stopped, err := Run(context.Background(), Options{Repository: repo, Root: root}, open)
	if err != nil || stopped.Phase != "execution_uncertain" || opens != 0 {
		t.Fatalf("plain run dispatched: %+v %v opens=%d", stopped, err, opens)
	}
	stopped, err = Run(context.Background(), Options{Repository: repo, Root: root, Recover: true}, open)
	if err == nil || !strings.Contains(err.Error(), "remains active") || stopped.Phase != "execution_uncertain" || opens != 0 {
		t.Fatalf("live recovery accepted: %+v %v opens=%d", stopped, err, opens)
	}
}

func TestRepeatedRecoveryRetainsFailedInvocations(t *testing.T) {
	repo, root := testRepo(t), t.TempDir()
	dir, s := seedStoppedState(t, repo, root, "execution_failed", "engineering")
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	failing := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels(), fail: true}
	first, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true}, func(context.Context, string, io.Writer) (Runner, error) { return failing, nil })
	if err == nil || first.Phase != "execution_failed" || first.Attempts != 2 || len(first.Recoveries) != 1 {
		t.Fatalf("first replacement failure: %+v %v", first, err)
	}
	secondRunner := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels(), disposition: "decision_required"}
	second, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true}, func(context.Context, string, io.Writer) (Runner, error) { return secondRunner, nil })
	if err != nil || second.Phase != "blocked" || second.Attempts != 3 || len(second.Recoveries) != 2 {
		t.Fatalf("second replacement: %+v %v", second, err)
	}
}

func TestRecoveryValidatesOverrideBeforeReplacementDispatch(t *testing.T) {
	repo, root := testRepo(t), t.TempDir()
	dir, s := seedStoppedState(t, repo, root, "execution_failed", "engineering")
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	runner := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels(), disposition: "decision_required"}
	state, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true, ReagentModel: "missing-model"}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err == nil || !strings.Contains(err.Error(), "unavailable") || len(runner.threads) != 0 || state.Attempts != 1 || len(state.Recoveries) != 1 {
		t.Fatalf("invalid override dispatched: %+v %v threads=%v", state, err, runner.threads)
	}
	status, err := Status(Options{Repository: repo, Root: root})
	if err != nil || status.Phase != "recovering" || status.ActiveRecovery == nil || status.PendingStep != "engineering" || !strings.Contains(status.LastError, "missing-model") {
		t.Fatalf("invalid override lost stopped recovery state: %+v %v", status, err)
	}
	state, err = Run(context.Background(), Options{Repository: repo, Root: root, Recover: true, ReagentModel: "replacement-model", ReagentEffort: "high"}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err != nil || state.Phase != "blocked" || state.Attempts != 2 || len(state.Recoveries) != 1 || len(runner.threads) != 1 {
		t.Fatalf("validated retry duplicated recovery: %+v %v threads=%v", state, err, runner.threads)
	}
}

func TestRecoverySetupRetryPreservesOriginalProfileAudit(t *testing.T) {
	repo, root := testRepo(t), t.TempDir()
	dir, s := seedStoppedState(t, repo, root, "execution_failed", "engineering")
	originalReagent := s.Profiles[dispatch.Reagent]
	originalIragent := s.Profiles[dispatch.Iragent]
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	runner := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels(), disposition: "decision_required", startErr: errors.New("start thread failed")}
	failed, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true, ReagentModel: "replacement-model", ReagentEffort: "high"}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err == nil || failed.Phase != "recovering" || failed.ActiveRecovery == nil || failed.Profiles[dispatch.Reagent].Model != "replacement-model" {
		t.Fatalf("initial setup failure: %+v %v", failed, err)
	}

	runner.startErr = nil
	recovered, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err != nil || recovered.Phase != "blocked" || recovered.Attempts != 2 || len(recovered.Recoveries) != 1 {
		t.Fatalf("successful setup retry: %+v %v", recovered, err)
	}
	audit := recovered.Recoveries[0]
	if audit.PriorProfile != originalReagent || audit.ReplacementProfile.Model != "replacement-model" || audit.ReplacementProfile.Effort != "high" {
		t.Fatalf("retry corrupted profile audit: %+v", audit)
	}
	if recovered.Profiles[dispatch.Iragent] != originalIragent {
		t.Fatalf("retry changed other role: got %+v want %+v", recovered.Profiles[dispatch.Iragent], originalIragent)
	}
}

func TestRecoverySetupFailuresRemainDurablyStopped(t *testing.T) {
	for _, tc := range []struct {
		name string
		open func(*recoveryRunner) func(context.Context, string, io.Writer) (Runner, error)
	}{
		{"provider open", func(_ *recoveryRunner) func(context.Context, string, io.Writer) (Runner, error) {
			return func(context.Context, string, io.Writer) (Runner, error) {
				return nil, errors.New("provider unavailable")
			}
		}},
		{"fresh session", func(runner *recoveryRunner) func(context.Context, string, io.Writer) (Runner, error) {
			runner.startErr = errors.New("start thread failed")
			return func(context.Context, string, io.Writer) (Runner, error) { return runner, nil }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, root := testRepo(t), t.TempDir()
			dir, s := seedStoppedState(t, repo, root, "execution_failed", "engineering")
			if err := save(dir, s); err != nil {
				t.Fatal(err)
			}
			runner := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels(), disposition: "decision_required"}
			state, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true}, tc.open(runner))
			if err == nil || state.Phase != "recovering" || state.ActiveRecovery == nil || state.Attempts != 1 || len(state.Recoveries) != 1 {
				t.Fatalf("setup failure state: %+v %v", state, err)
			}
			status, statusErr := Status(Options{Repository: repo, Root: root})
			if statusErr != nil || status.Phase != "recovering" || status.ActiveRecovery == nil || status.PendingStep != "engineering" || status.Attempts != 1 || !strings.Contains(status.LastError, err.Error()) {
				t.Fatalf("durable setup failure: %+v %v", status, statusErr)
			}
			opens := 0
			plain, plainErr := Run(context.Background(), Options{Repository: repo, Root: root}, func(context.Context, string, io.Writer) (Runner, error) {
				opens++
				return runner, nil
			})
			if plainErr != nil || plain.Phase != "recovering" || opens != 0 {
				t.Fatalf("plain run dispatched prepared recovery: %+v %v opens=%d", plain, plainErr, opens)
			}
		})
	}
}

func TestPreparedReviewRecoveryRevalidatesBeforeEveryRetry(t *testing.T) {
	for _, tc := range []struct {
		name   string
		first  func(*recoveryRunner) (Options, func(context.Context, string, io.Writer) (Runner, error))
		mutate func(*testing.T, string)
	}{
		{
			name: "provider open then tracked change",
			first: func(_ *recoveryRunner) (Options, func(context.Context, string, io.Writer) (Runner, error)) {
				return Options{}, func(context.Context, string, io.Writer) (Runner, error) {
					return nil, errors.New("provider unavailable")
				}
			},
			mutate: func(t *testing.T, repo string) {
				if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("dirty candidate\n"), 0600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "override validation then untracked change",
			first: func(r *recoveryRunner) (Options, func(context.Context, string, io.Writer) (Runner, error)) {
				return Options{IragentModel: "missing-model"}, func(context.Context, string, io.Writer) (Runner, error) { return r, nil }
			},
			mutate: func(t *testing.T, repo string) {
				if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("preserve\n"), 0600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "fresh session then branch change",
			first: func(r *recoveryRunner) (Options, func(context.Context, string, io.Writer) (Runner, error)) {
				r.startErr = errors.New("start thread failed")
				return Options{}, func(context.Context, string, io.Writer) (Runner, error) { return r, nil }
			},
			mutate: func(t *testing.T, repo string) { gitTest(repo, "switch", "-q", "main") },
		},
		{
			name: "provider open then head change",
			first: func(_ *recoveryRunner) (Options, func(context.Context, string, io.Writer) (Runner, error)) {
				return Options{}, func(context.Context, string, io.Writer) (Runner, error) {
					return nil, errors.New("provider unavailable")
				}
			},
			mutate: func(t *testing.T, repo string) {
				if err := os.WriteFile(filepath.Join(repo, "later.txt"), []byte("later commit\n"), 0600); err != nil {
					t.Fatal(err)
				}
				gitTest(repo, "add", "later.txt")
				gitTest(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "later")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, root := testRepo(t), t.TempDir()
			gitTest(repo, "switch", "-qc", "task/APP-002")
			if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("candidate\n"), 0600); err != nil {
				t.Fatal(err)
			}
			gitTest(repo, "add", "feature.txt")
			gitTest(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "candidate")
			candidate := gitTest(repo, "rev-parse", "HEAD")
			dir, s := seedStoppedState(t, repo, root, "execution_failed", "reviewing")
			s.Branch, s.TaskID, s.Candidate = "task/APP-002", "APP-002", candidate
			s.InFlight.Checkpoint, _ = checkpoint(s, dir)
			if err := save(dir, s); err != nil {
				t.Fatal(err)
			}
			runner := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels()}
			opts, open := tc.first(runner)
			opts.Repository, opts.Root, opts.Recover = repo, root, true
			state, err := Run(context.Background(), opts, open)
			if err == nil || state.Phase != "recovering" || state.Attempts != 1 {
				t.Fatalf("initial setup failure: %+v %v", state, err)
			}
			tc.mutate(t, repo)
			partial := reviewPath(dir, s.Roadmap, s.TaskID, 1)
			writeEvidence(partial)
			opens := 0
			state, err = Run(context.Background(), Options{Repository: repo, Root: root, Recover: true}, func(context.Context, string, io.Writer) (Runner, error) {
				opens++
				return runner, nil
			})
			if err == nil || !strings.Contains(err.Error(), "exact candidate in a clean checkout") || state.Phase != "recovering" || state.Attempts != 1 || opens != 0 {
				t.Fatalf("mutated retry dispatched: %+v %v opens=%d", state, err, opens)
			}
			if len(state.ActiveRecovery.AdditionalArchives) != 1 {
				t.Fatalf("retry partial review not archived: %+v", state.ActiveRecovery)
			}
			if _, err := os.Stat(state.ActiveRecovery.AdditionalArchives[0]); err != nil {
				t.Fatalf("retry archive missing: %v", err)
			}
		})
	}
}

func TestRecoveryLimitIncreaseRetainsStoppedBoundaryAndOverride(t *testing.T) {
	repo, root := testRepo(t), t.TempDir()
	dir, s := seedStoppedState(t, repo, root, "execution_failed", "engineering")
	s.MaxAttempts, s.Attempts = 1, 1
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	opens := 0
	limited, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true, ReagentModel: "replacement-model", ReagentEffort: "high"}, func(context.Context, string, io.Writer) (Runner, error) {
		opens++
		return nil, errors.New("must not open")
	})
	if err != nil || limited.Phase != "limit_exhausted" || limited.ActiveRecovery == nil || limited.ActiveRecovery.RequestedModel != "replacement-model" || limited.ActiveRecovery.RequestedEffort != "high" || opens != 0 {
		t.Fatalf("limited recovery: %+v %v opens=%d", limited, err, opens)
	}
	authorized, err := Run(context.Background(), Options{Repository: repo, Root: root, ApproveInvocationLimitIncrease: 2}, func(context.Context, string, io.Writer) (Runner, error) {
		opens++
		return nil, errors.New("must not open")
	})
	if err != nil || authorized.Phase != "recovering" || authorized.Attempts != 1 || opens != 0 || !strings.Contains(authorized.LastError, "run --recover") {
		t.Fatalf("limit authorization dispatched recovery: %+v %v opens=%d", authorized, err, opens)
	}
	runner := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels(), disposition: "decision_required"}
	presented := 0
	recovered, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true, OnRecovery: func(State) { presented++ }}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err != nil || recovered.Phase != "blocked" || recovered.Attempts != 2 || len(runner.settings) != 1 || runner.settings[0].Model != "replacement-model" || runner.settings[0].Effort != "high" || presented != 1 {
		t.Fatalf("recovery after limit authorization: %+v %v settings=%+v presented=%d", recovered, err, runner.settings, presented)
	}
}

func TestIntegrationRecoveryReconcilesAlreadyMovedParentWithoutHostGitAction(t *testing.T) {
	repo, root := testRepo(t), t.TempDir()
	base := gitTest(repo, "rev-parse", "HEAD")
	gitTest(repo, "switch", "-qc", "task/APP-002")
	if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("candidate\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(repo, "add", "feature.txt")
	gitTest(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "candidate")
	candidate := gitTest(repo, "rev-parse", "HEAD")
	gitTest(repo, "update-ref", "refs/heads/main", candidate, base)
	dir, s := seedStoppedState(t, repo, root, "execution_uncertain", "integrating")
	s.Base, s.Branch, s.TaskID, s.Candidate = base, "task/APP-002", "APP-002", candidate
	s.ApprovedCommit, s.AuthorizedCommit = candidate, candidate
	s.InFlight.Checkpoint, _ = checkpoint(s, dir)
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	runner := &recoveryRunner{repo: repo, evidenceDir: dir, models: recoveryModels(), disposition: "decision_required"}
	recovered, err := Run(context.Background(), Options{Repository: repo, Root: root, Recover: true}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err != nil || recovered.Phase != "blocked" || gitTest(repo, "rev-parse", "main") != candidate || gitTest(repo, "rev-parse", "task/APP-002") != candidate {
		t.Fatalf("integration reconciliation: %+v %v", recovered, err)
	}
	prompt := runner.prompts[0]
	if !strings.Contains(prompt, "was already observed at authorized commit") || !strings.Contains(prompt, "Do not fast-forward") || strings.Contains(prompt, "still points to base") || strings.Contains(prompt, "Perform local fast-forward") {
		t.Fatalf("integration recovery prompt is contradictory: %s", prompt)
	}
}

func TestIntegrationRecoveryPromptAtBaseOnlyOrdersGuardedFastForward(t *testing.T) {
	s := State{TaskID: "APP-002", Roadmap: "/repo/roadmap.md", Repository: "/repo", Branch: "task/APP-002", Parent: "main", Base: "base", Candidate: "candidate", AuthorizedCommit: "candidate", ReviewEvidence: "/evidence/review.md", Evidence: "/evidence/task.md", ActiveRecovery: &Recovery{Step: "integrating", IntegrationParent: "base"}}
	prompt, err := RecoveryIntegrationPrompt(s, "roadmap context")
	if err != nil || !strings.Contains(prompt, "was observed at recorded base") || !strings.Contains(prompt, "perform the local fast-forward") || strings.Contains(prompt, "already observed at authorized commit") || strings.Contains(prompt, "Do not fast-forward") {
		t.Fatalf("base integration recovery prompt: %v %s", err, prompt)
	}
}
