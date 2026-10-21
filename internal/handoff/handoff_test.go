package handoff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
			writeEvidence(filepath.Join(f.evidenceDir, "evidence", "APP-002", "task.md"))
		}
	} else if f.evidenceDir != "" && !f.skipReviewEvidence {
		writeEvidence(filepath.Join(f.evidenceDir, "evidence", "APP-002", "review-01.md"))
	}
	result := f.output[f.turns-1]
	return codexapp.TurnOutcome{ThreadID: id, TurnID: s.Role + "-turn", Status: "completed", FinalText: result}, nil
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
func TestSelectionToIndependentReview(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	fake := &fakeRunner{repo: repo, models: model(), output: []string{result("ready_for_review", "Candidate ready", "APP-002", "CANDIDATE"), result("approved", "Looks good", "APP-002", "CANDIDATE")}}
	fake.evidenceDir, _ = storage(repo, root)
	open := func(context.Context, string) (Runner, error) { return fake, nil }
	s, err := Run(context.Background(), Options{Repository: repo, Root: root, OnEvent: func(role string, e codexapp.Event) {}}, open)
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != "reviewed" || s.Verdict != "approved" || s.Attempts != 2 || fake.turns != 2 {
		t.Fatalf("wrong handoff: %+v", s)
	}
	if !strings.Contains(fake.prompts[0], "roadmap.md") || !strings.Contains(fake.prompts[1], s.Candidate) || !strings.Contains(fake.prompts[1], s.Evidence) {
		t.Fatalf("wrong prompts: %#v", fake.prompts)
	}
	if s.Branch != "task/APP-002" || s.Candidate != gitTest(repo, "rev-parse", "HEAD") {
		t.Fatalf("wrong candidate binding: %+v", s)
	}
}
func TestInvocationLimitStopsBeforeReview(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	fake := &fakeRunner{repo: repo, models: model(), output: []string{result("ready_for_review", "Candidate ready", "APP-002", "CANDIDATE")}}
	fake.evidenceDir, _ = storage(repo, root)
	s, err := Run(context.Background(), Options{Repository: repo, Root: root, MaxAttempts: 1}, func(context.Context, string) (Runner, error) { return fake, nil })
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
	s, err := Run(context.Background(), Options{Repository: repo, Root: t.TempDir()}, func(context.Context, string) (Runner, error) { return fake, nil })
	if err == nil || !strings.Contains(err.Error(), "invalid structured") || s.Phase != "invalid_result" || s.Attempts != 1 {
		t.Fatalf("malformed result: %+v %v", s, err)
	}
}
func TestTimeoutStopsTurn(t *testing.T) {
	repo := testRepo(t)
	fake := &fakeRunner{repo: repo, models: model(), fail: errors.New("wait")}
	s, err := Run(context.Background(), Options{Repository: repo, Root: t.TempDir(), TurnTimeout: 20 * time.Millisecond}, func(context.Context, string) (Runner, error) { return fake, nil })
	if err == nil || fake.stopCalls != 1 || s.Attempts != 1 || s.Phase != "execution_failed" {
		t.Fatalf("timeout: %+v %v", s, err)
	}
}

func TestExecutionFailureRetainsDiagnostic(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	fake := &fakeRunner{repo: repo, models: model(), immediate: fmt.Errorf("%w: provider rejected detailed schema", codexapp.ErrInvalidResponseSchema)}
	s, err := Run(context.Background(), Options{Repository: repo, Root: root}, func(context.Context, string) (Runner, error) { return fake, nil })
	if err == nil || strings.Contains(err.Error(), "provider rejected detailed") || !strings.Contains(err.Error(), "invalid response schema") || s.Attempts != 1 {
		t.Fatalf("failure classification: %+v %v", s, err)
	}
	dir, e := storage(repo, root)
	if e != nil {
		t.Fatal(e)
	}
	log, e := os.ReadFile(filepath.Join(dir, "diagnostic.log"))
	if e != nil || !strings.Contains(string(log), "provider rejected detailed schema") {
		t.Fatalf("missing private diagnostic: %s %v", log, e)
	}
}
func TestStallTimeoutStopsTurn(t *testing.T) {
	repo := testRepo(t)
	fake := &fakeRunner{repo: repo, models: model(), fail: errors.New("wait")}
	s, err := Run(context.Background(), Options{Repository: repo, Root: t.TempDir(), StallTimeout: 20 * time.Millisecond}, func(context.Context, string) (Runner, error) { return fake, nil })
	if err == nil || fake.stopCalls != 1 || s.Attempts != 1 {
		t.Fatalf("stall: %+v %v", s, err)
	}
}

func TestNoSuitableWorkDoesNotLaunchReview(t *testing.T) {
	repo := testRepo(t)
	fake := &fakeRunner{repo: repo, models: model(), output: []string{result("no_suitable_work", "Nothing ready", "", "")}}
	s, err := Run(context.Background(), Options{Repository: repo, Root: t.TempDir()}, func(context.Context, string) (Runner, error) { return fake, nil })
	if err != nil || s.Phase != "no_suitable_work" || fake.turns != 1 {
		t.Fatalf("no-work routing: %+v %v", s, err)
	}
}
func TestWrongCandidateCannotReachReview(t *testing.T) {
	repo := testRepo(t)
	fake := &fakeRunner{repo: repo, models: model(), output: []string{result("ready_for_review", "Ready", "APP-002", "wrong")}}
	s, err := Run(context.Background(), Options{Repository: repo, Root: t.TempDir()}, func(context.Context, string) (Runner, error) { return fake, nil })
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
			s, err := Run(context.Background(), Options{Repository: repo, Root: root}, func(context.Context, string) (Runner, error) { return fake, nil })
			if err == nil || !strings.Contains(err.Error(), tc.wantError) || s.Phase != "invalid_result" || fake.turns != tc.wantTurns || s.Verdict != "" || (tc.skipTask && s.Evidence != "") || (tc.skipReview && s.ReviewEvidence != "") {
				t.Fatalf("evidence handoff: %+v %v turns=%d", s, err, fake.turns)
			}
		})
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
			s, err := Run(context.Background(), Options{Repository: repo, Root: root}, func(context.Context, string) (Runner, error) { return fake, nil })
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
			open := func(context.Context, string) (Runner, error) { opens++; return fake, nil }
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
