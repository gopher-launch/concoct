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
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/stop"
)

type fakeRunner struct {
	repo               string
	turns              int
	prompts            []string
	output             []string
	stopCalls          int
	fail               error
	immediate          error
	models             []codexapp.Model
	evidenceDir        string
	skipTaskEvidence   bool
	skipReviewEvidence bool
	beforeTurn         func(string)
}

func (f *fakeRunner) Models(context.Context) ([]codexapp.Model, error) { return f.models, nil }
func (f *fakeRunner) StartThread(_ context.Context, cwd string, s codexapp.RoleSettings, tools []codexapp.ToolSpec) (codexapp.Thread, error) {
	if cwd != f.repo || len(tools) != 0 {
		return codexapp.Thread{}, errors.New("wrong thread inputs")
	}
	return codexapp.Thread{ID: s.Role + "-thread"}, nil
}
func (f *fakeRunner) ResumeThread(_ context.Context, id string, _ bool, _ codexapp.RoleSettings) (codexapp.Thread, error) {
	return codexapp.Thread{ID: id}, nil
}
func (f *fakeRunner) RunRoleTurn(ctx context.Context, id, prompt string, s codexapp.RoleSettings, emit func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	f.prompts = append(f.prompts, prompt)
	f.turns++
	emit(codexapp.Event{Kind: "turn/started", ThreadID: id, TurnID: s.Role + "-turn"})
	if f.beforeTurn != nil {
		f.beforeTurn(s.Role)
	}
	if f.immediate != nil {
		return codexapp.TurnOutcome{ThreadID: id, TurnID: s.Role + "-turn"}, f.immediate
	}
	if f.fail != nil {
		<-ctx.Done()
		return codexapp.TurnOutcome{ThreadID: id, TurnID: s.Role + "-turn"}, ctx.Err()
	}
	if s.Role == "reagent" {
		gitTest(f.repo, "switch", "-qc", "task/APP-002")
		os.WriteFile(filepath.Join(f.repo, "feature.txt"), []byte("done\n"), 0600)
		gitTest(f.repo, "add", "feature.txt")
		gitTest(f.repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "implement")
		head := gitTest(f.repo, "rev-parse", "HEAD")
		for i := range f.output {
			f.output[i] = strings.ReplaceAll(f.output[i], "CANDIDATE", head)
		}
		if f.evidenceDir != "" && !f.skipTaskEvidence {
			writeEvidence(taskEvidencePath(f.evidenceDir, selectedRoadmapInPrompt(prompt), "APP-002"))
		}
	} else if f.evidenceDir != "" && !f.skipReviewEvidence {
		writeEvidence(reviewPath(f.evidenceDir, selectedRoadmapInPrompt(prompt), "APP-002", 1))
	}
	result := f.output[f.turns-1]
	return codexapp.TurnOutcome{ThreadID: id, TurnID: s.Role + "-turn", Status: "completed", FinalText: result}, nil
}

func selectedRoadmapInPrompt(prompt string) string {
	const prefix = "Selected roadmap: "
	for _, line := range strings.Split(prompt, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	panic("prompt omitted selected roadmap")
}
func (f *fakeRunner) Stop(context.Context, string, string) stop.Result {
	f.stopCalls++
	return stop.Result{Status: "stopped_safely"}
}
func (f *fakeRunner) Close() error { return nil }
func writeEvidence(path string) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, []byte("# Agent evidence\n"), 0600); err != nil {
		panic(err)
	}
}
func gitTest(repo string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
	if err != nil {
		panic(string(out) + err.Error())
	}
	return strings.TrimSpace(string(out))
}
func testRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitTest(repo, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(repo, "roadmap.md"), []byte("# roadmap\n"), 0600)
	gitTest(repo, "add", ".")
	gitTest(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "base")
	return repo
}
func model() []codexapp.Model {
	m := codexapp.Model{Model: "test-model", IsDefault: true}
	m.SupportedReasoningEfforts = append(m.SupportedReasoningEfforts, struct {
		ReasoningEffort string `json:"reasoningEffort"`
	}{"medium"}, struct {
		ReasoningEffort string `json:"reasoningEffort"`
	}{"high"})
	return []codexapp.Model{m}
}
func result(disposition, summary, task, candidate string) string {
	data, _ := json.Marshal(map[string]any{"disposition": disposition, "summary": summary, "task_id": task, "candidate": candidate, "findings": nil, "evidence": nil, "question": nil})
	return string(data)
}

func resultWithEvidence(disposition, summary, task, candidate, evidence string) string {
	data, _ := json.Marshal(map[string]any{"disposition": disposition, "summary": summary, "task_id": task, "candidate": candidate, "findings": nil, "evidence": evidence, "question": nil})
	return string(data)
}
func TestSelectionToIndependentReview(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	fake := &fakeRunner{repo: repo, models: model(), output: []string{result("ready_for_review", "Candidate ready", "APP-002", "CANDIDATE"), result("approved", "Looks good", "APP-002", "CANDIDATE")}}
	fake.evidenceDir, _ = storage(repo, root)
	open := func(context.Context, string, io.Writer) (Runner, error) { return fake, nil }
	s, err := Run(context.Background(), Options{Repository: repo, Root: root, OnEvent: func(role string, e codexapp.Event) {}}, open)
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != "awaiting_approval" || s.Verdict != "approved" || s.ApprovedCommit != s.Candidate || s.Attempts != 2 || fake.turns != 2 {
		t.Fatalf("wrong handoff: %+v", s)
	}
	if !strings.Contains(fake.prompts[0], "roadmap.md") || !strings.Contains(fake.prompts[1], s.Candidate) || !strings.Contains(fake.prompts[1], s.Evidence) {
		t.Fatalf("wrong prompts: %#v", fake.prompts)
	}
	if !strings.Contains(fake.prompts[0], "original scope and acceptance criteria") || !strings.Contains(fake.prompts[1], "original task scope and acceptance criteria retained") {
		t.Fatalf("original task scope not retained for independent review: %#v", fake.prompts)
	}
	if s.Branch != "task/APP-002" || s.Candidate != gitTest(repo, "rev-parse", "HEAD") {
		t.Fatalf("wrong candidate binding: %+v", s)
	}
	if s.Roadmap != filepath.Join(repo, "roadmap.md") {
		t.Fatalf("default roadmap was not recorded: %+v", s)
	}
}

func TestSelectedRoadmapPersistsAcrossWorkflow(t *testing.T) {
	repo := testRepo(t)
	rootContents := "## APP-002 — Root task with same ID\n\n- Kind: task\n- Status: ready\n- Priority: high\n- Parent: none\n- Depends on: none\n\n**Objective:** root-marker\n"
	if err := os.WriteFile(filepath.Join(repo, "roadmap.md"), []byte(rootContents), 0600); err != nil {
		t.Fatal(err)
	}
	selectedDir := filepath.Join(repo, "deploy plans")
	if err := os.MkdirAll(selectedDir, 0700); err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(selectedDir, "release roadmap.md")
	contents := "## APP-002 — Deployment task\n\n- Kind: task\n- Status: ready\n- Priority: high\n- Parent: none\n- Depends on: none\n\n**Objective:** deploy-marker\n"
	if err := os.WriteFile(selected, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	capabilities := filepath.Join(selectedDir, "capabilities.md")
	if err := os.WriteFile(capabilities, []byte("# Deployment capabilities\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(repo, "add", ".")
	gitTest(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "deployment roadmap")

	root := t.TempDir()
	dir, err := storage(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	runner := &flowRunner{repo: repo, root: dir}
	relative := filepath.Join("deploy plans", "release roadmap.md")
	s, err := Run(context.Background(), Options{Repository: repo, Root: root, Roadmap: relative}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err != nil || s.Phase != "awaiting_approval" || s.Roadmap != selected || s.Capabilities != capabilities {
		t.Fatalf("selected roadmap state: %+v %v", s, err)
	}
	if len(runner.prompts) != 4 {
		t.Fatalf("workflow prompt count: %d", len(runner.prompts))
	}
	for i, prompt := range runner.prompts {
		for _, want := range []string{selected, capabilities, "deploy-marker"} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("prompt %d lacks %q: %s", i, want, prompt)
			}
		}
		if strings.Contains(prompt, "root-marker") {
			t.Fatalf("prompt %d mixed the root roadmap into selected context: %s", i, prompt)
		}
	}
	reopened, err := Run(context.Background(), Options{Repository: repo, Root: root}, func(context.Context, string, io.Writer) (Runner, error) {
		t.Fatal("approval wait opened provider")
		return nil, nil
	})
	if err != nil || reopened.Phase != "awaiting_approval" || reopened.Roadmap != selected || reopened.Candidate != s.Candidate {
		t.Fatalf("continuation without roadmap lost recorded context: %+v %v", reopened, err)
	}

	turns := runner.turns
	if _, err := Run(context.Background(), Options{Repository: repo, Root: root, Roadmap: "roadmap.md"}, func(context.Context, string, io.Writer) (Runner, error) {
		t.Fatal("roadmap mismatch opened provider")
		return nil, nil
	}); err == nil || !strings.Contains(err.Error(), "cannot switch") || runner.turns != turns {
		t.Fatalf("roadmap switch was not rejected before dispatch: %v turns=%d", err, runner.turns)
	}

	s, err = Run(context.Background(), Options{Repository: repo, Root: root, Roadmap: selected, Approve: s.Candidate}, func(context.Context, string, io.Writer) (Runner, error) { return runner, nil })
	if err != nil || s.Phase != "completed" || s.Roadmap != selected || len(runner.prompts) != 5 || !strings.Contains(runner.prompts[4], "deploy-marker") {
		t.Fatalf("integration lost roadmap context: %+v %v prompts=%d", s, err, len(runner.prompts))
	}
	status, err := Status(Options{Repository: repo, Root: root})
	if err != nil || status.Roadmap != selected {
		t.Fatalf("status lost roadmap: %+v %v", status, err)
	}
}

func TestInvalidSelectedRoadmapDoesNotCreateActiveWork(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func(*testing.T, string) string
		want string
	}{
		{"missing", func(_ *testing.T, repo string) string { return filepath.Join(repo, "missing.md") }, "resolve roadmap"},
		{"directory", func(t *testing.T, repo string) string {
			path := filepath.Join(repo, "roadmap-dir")
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			return path
		}, "not a regular file"},
		{"unreadable", func(t *testing.T, repo string) string {
			path := filepath.Join(repo, "unreadable.md")
			if err := os.WriteFile(path, []byte("# Roadmap\n"), 0000); err != nil {
				t.Fatal(err)
			}
			return path
		}, "not readable"},
		{"invalid metadata", func(t *testing.T, repo string) string {
			path := filepath.Join(repo, "invalid.md")
			if err := os.WriteFile(path, []byte("## APP-002 — Broken\n\n- Kind: task\n- Status: unknown\n- Priority: high\n- Parent: none\n- Depends on: none\n"), 0600); err != nil {
				t.Fatal(err)
			}
			return path
		}, "invalid roadmap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testRepo(t)
			root := t.TempDir()
			selected := tc.make(t, repo)
			opened := false
			s, err := Run(context.Background(), Options{Repository: repo, Root: root, Roadmap: selected}, func(context.Context, string, io.Writer) (Runner, error) {
				opened = true
				return nil, nil
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) || opened || s.Phase != "" {
				t.Fatalf("invalid roadmap result: %+v err=%v opened=%v", s, err, opened)
			}
			dir, storageErr := storage(repo, root)
			if storageErr != nil {
				t.Fatal(storageErr)
			}
			if _, statErr := os.Stat(statePath(dir)); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("invalid roadmap created active state: %v", statErr)
			}
		})
	}
}
func TestInvocationLimitStopsBeforeReview(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	fake := &fakeRunner{repo: repo, models: model(), output: []string{result("ready_for_review", "Candidate ready", "APP-002", "CANDIDATE")}}
	fake.evidenceDir, _ = storage(repo, root)
	s, err := Run(context.Background(), Options{Repository: repo, Root: root, MaxAttempts: 1}, func(context.Context, string, io.Writer) (Runner, error) { return fake, nil })
	if err != nil {
		t.Fatal(err)
	}
	if fake.turns != 1 || s.Phase != "limit_exhausted" {
		t.Fatalf("limit failed: %+v", s)
	}
}
func TestMalformedResultKeepsWork(t *testing.T) {
	repo := testRepo(t)
	fake := &fakeRunner{repo: repo, models: model(), output: []string{`{"summary":"missing disposition"}`}}
	s, err := Run(context.Background(), Options{Repository: repo, Root: t.TempDir()}, func(context.Context, string, io.Writer) (Runner, error) { return fake, nil })
	if err == nil || !strings.Contains(err.Error(), "invalid structured") || s.Phase != "invalid_result" || s.Attempts != 1 {
		t.Fatalf("malformed result: %+v %v", s, err)
	}
}
func TestTimeoutStopsTurn(t *testing.T) {
	repo := testRepo(t)
	fake := &fakeRunner{repo: repo, models: model(), fail: errors.New("wait")}
	s, err := Run(context.Background(), Options{Repository: repo, Root: t.TempDir(), TurnTimeout: 20 * time.Millisecond}, func(context.Context, string, io.Writer) (Runner, error) { return fake, nil })
	if err == nil || fake.stopCalls != 1 || s.Attempts != 1 || s.Phase != "execution_failed" {
		t.Fatalf("timeout: %+v %v", s, err)
	}
}

func TestExecutionFailureRetainsDiagnostic(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	fake := &fakeRunner{repo: repo, models: model(), immediate: fmt.Errorf("%w: provider rejected detailed schema", codexapp.ErrInvalidResponseSchema)}
	s, err := Run(context.Background(), Options{Repository: repo, Root: root}, func(context.Context, string, io.Writer) (Runner, error) { return fake, nil })
	if err == nil || strings.Contains(err.Error(), "provider rejected detailed") || !strings.Contains(err.Error(), "invalid response schema") || s.Attempts != 1 {
		t.Fatalf("failure classification: %+v %v", s, err)
	}
	log, e := os.ReadFile(s.Diagnostics)
	if e != nil || !strings.Contains(string(log), "provider rejected detailed schema") {
		t.Fatalf("missing private diagnostic: %s %v", log, e)
	}
}

func TestProviderDiagnosticsDoNotTurnSuccessfulExecutionIntoFailure(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	fake := &fakeRunner{repo: repo, models: model(), output: []string{result("no_suitable_work", "Nothing ready", "", "")}}
	var diagnostics io.Writer
	fake.beforeTurn = func(string) {
		_, _ = io.WriteString(diagnostics, "model-cache error: recovered\nrecoverable tool failure: retried\n")
	}
	s, err := Run(context.Background(), Options{Repository: repo, Root: root}, func(_ context.Context, _ string, log io.Writer) (Runner, error) {
		diagnostics = log
		_, _ = io.WriteString(log, "app-server startup diagnostic\n")
		return fake, nil
	})
	if err != nil || s.Phase != "no_suitable_work" || s.LastError != "" || s.Diagnostics == "" {
		t.Fatalf("recoverable diagnostics changed outcome: %+v %v", s, err)
	}
	log, err := os.ReadFile(s.Diagnostics)
	if err != nil || !strings.Contains(string(log), "app-server startup diagnostic") || !strings.Contains(string(log), "model-cache error") || !strings.Contains(string(log), "recoverable tool failure") {
		t.Fatalf("provider diagnostics missing: %s %v", log, err)
	}
}
func TestStallTimeoutStopsTurn(t *testing.T) {
	repo := testRepo(t)
	fake := &fakeRunner{repo: repo, models: model(), fail: errors.New("wait")}
	s, err := Run(context.Background(), Options{Repository: repo, Root: t.TempDir(), StallTimeout: 20 * time.Millisecond}, func(context.Context, string, io.Writer) (Runner, error) { return fake, nil })
	if err == nil || fake.stopCalls != 1 || s.Attempts != 1 {
		t.Fatalf("stall: %+v %v", s, err)
	}
}

func TestNoSuitableWorkDoesNotLaunchReview(t *testing.T) {
	repo := testRepo(t)
	fake := &fakeRunner{repo: repo, models: model(), output: []string{result("no_suitable_work", "Nothing ready", "", "")}}
	s, err := Run(context.Background(), Options{Repository: repo, Root: t.TempDir()}, func(context.Context, string, io.Writer) (Runner, error) { return fake, nil })
	if err != nil || s.Phase != "no_suitable_work" || fake.turns != 1 {
		t.Fatalf("no-work routing: %+v %v", s, err)
	}
}
func TestWrongCandidateCannotReachReview(t *testing.T) {
	repo := testRepo(t)
	fake := &fakeRunner{repo: repo, models: model(), output: []string{result("ready_for_review", "Ready", "APP-002", "wrong")}}
	s, err := Run(context.Background(), Options{Repository: repo, Root: t.TempDir()}, func(context.Context, string, io.Writer) (Runner, error) { return fake, nil })
	if err == nil || s.Phase != "source_changed" || fake.turns != 1 {
		t.Fatalf("candidate binding: %+v %v", s, err)
	}
}

func TestMissingAgentEvidenceStopsHandoff(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		skipTask, skipReview bool
		wantTurns            int
		wantError            string
	}{
		{"task", true, false, 1, "task evidence"},
		{"review", false, true, 2, "review evidence"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testRepo(t)
			root := t.TempDir()
			dir, err := storage(repo, root)
			if err != nil {
				t.Fatal(err)
			}
			fake := &fakeRunner{repo: repo, evidenceDir: dir, skipTaskEvidence: tc.skipTask, skipReviewEvidence: tc.skipReview, models: model(), output: []string{result("ready_for_review", "Ready", "APP-002", "CANDIDATE"), result("approved", "Approved", "APP-002", "CANDIDATE")}}
			s, err := Run(context.Background(), Options{Repository: repo, Root: root}, func(context.Context, string, io.Writer) (Runner, error) { return fake, nil })
			if err == nil || !strings.Contains(err.Error(), tc.wantError) || s.Phase != "invalid_result" || fake.turns != tc.wantTurns || s.Verdict != "" || (tc.skipTask && s.Evidence != "") || (tc.skipReview && s.ReviewEvidence != "") {
				t.Fatalf("evidence handoff: %+v %v turns=%d", s, err, fake.turns)
			}
		})
	}
}

func TestTaskEvidenceMustMatchSelectedRoadmapNamespace(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	dir, err := storage(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	wrong := filepath.Join(dir, "evidence", "APP-002", "task.md")
	fake := &fakeRunner{
		repo: repo, evidenceDir: dir, models: model(),
		output: []string{resultWithEvidence("ready_for_review", "Ready", "APP-002", "CANDIDATE", wrong)},
	}
	s, err := Run(context.Background(), Options{Repository: repo, Root: root}, func(context.Context, string, io.Writer) (Runner, error) { return fake, nil })
	want := taskEvidencePath(dir, filepath.Join(repo, "roadmap.md"), "APP-002")
	if err == nil || !strings.Contains(err.Error(), "task evidence must be the selected roadmap task file "+want) || s.Phase != "invalid_result" || fake.turns != 1 || s.Evidence != "" {
		t.Fatalf("wrong roadmap evidence accepted: %+v %v", s, err)
	}
}

func TestReviewRequiresExplicitExactIdentity(t *testing.T) {
	for _, tc := range []struct{ name, task, candidate string }{
		{"missing task", "", "CANDIDATE"}, {"missing candidate", "APP-002", ""},
		{"wrong task", "OTHER-002", "CANDIDATE"}, {"wrong candidate", "APP-002", "wrong"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := testRepo(t)
			root := t.TempDir()
			dir, err := storage(repo, root)
			if err != nil {
				t.Fatal(err)
			}
			fake := &fakeRunner{repo: repo, evidenceDir: dir, models: model(), output: []string{result("ready_for_review", "Ready", "APP-002", "CANDIDATE"), result("approved", "Approved", tc.task, tc.candidate)}}
			s, err := Run(context.Background(), Options{Repository: repo, Root: root}, func(context.Context, string, io.Writer) (Runner, error) { return fake, nil })
			if err == nil || !strings.Contains(err.Error(), "review result identity") || s.Phase != "invalid_result" || s.Verdict != "" || fake.turns != 2 {
				t.Fatalf("identity binding: %+v %v", s, err)
			}
		})
	}
}

func TestInterruptedDispatchDoesNotReplay(t *testing.T) {
	for _, crashRole := range []string{"reagent", "iragent"} {
		t.Run(crashRole, func(t *testing.T) {
			repo := testRepo(t)
			root := t.TempDir()
			dir, err := storage(repo, root)
			if err != nil {
				t.Fatal(err)
			}
			fake := &fakeRunner{repo: repo, evidenceDir: dir, models: model(), output: []string{result("ready_for_review", "Ready", "APP-002", "CANDIDATE"), result("approved", "Approved", "APP-002", "CANDIDATE")}}
			fake.beforeTurn = func(role string) {
				if role == crashRole {
					stored, err := read(dir)
					if err != nil {
						t.Fatal(err)
					}
					if stored.InFlight == nil || stored.InFlight.Role != dispatch.Role(role) || stored.Attempts != fake.turns {
						t.Fatalf("dispatch not persisted before provider turn: %+v", stored)
					}
					panic("simulated process loss")
				}
			}
			opens := 0
			open := func(context.Context, string, io.Writer) (Runner, error) { opens++; return fake, nil }
			func() {
				defer func() {
					if value := recover(); value == nil {
						t.Fatal("expected simulated process loss")
					}
				}()
				_, _ = Run(context.Background(), Options{Repository: repo, Root: root}, open)
			}()
			before := fake.turns
			s, err := Run(context.Background(), Options{Repository: repo, Root: root}, open)
			if err != nil || s.Phase != "execution_uncertain" || s.InFlight == nil || s.Attempts != before || fake.turns != before || opens != 1 {
				t.Fatalf("uncertain turn replayed: %+v %v turns=%d opens=%d", s, err, fake.turns, opens)
			}
		})
	}
}
