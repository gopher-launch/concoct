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
	fail              bool
	startErr          error
}

func (r *recoveryRunner) Models(context.Context) ([]codexapp.Model, error) { return r.models, nil }
func (r *recoveryRunner) StartThread(_ context.Context, cwd string, settings codexapp.RoleSettings, _ []codexapp.ToolSpec) (codexapp.Thread, error) {
	if cwd != r.repo {
		return codexapp.Thread{}, errors.New("wrong repository")
	}
	if r.startErr != nil {
		return codexapp.Thread{}, r.startErr
	}
	id := "replacement-" + settings.Role
	r.threads = append(r.threads, id)
	return codexapp.Thread{ID: id}, nil
}
func (r *recoveryRunner) ResumeThread(context.Context, string, bool, codexapp.RoleSettings) (codexapp.Thread, error) {
	return codexapp.Thread{}, errors.New("recovery must start a fresh thread")
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
