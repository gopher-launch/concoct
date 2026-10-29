package handoff

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/stop"
)

type limitRunner struct {
	prompt string
	turns  int
}

func (r *limitRunner) Models(context.Context) ([]codexapp.Model, error) { return nil, nil }
func (r *limitRunner) StartThread(context.Context, string, codexapp.RoleSettings, []codexapp.ToolSpec) (codexapp.Thread, error) {
	return codexapp.Thread{ID: "reagent-limit-thread"}, nil
}
func (r *limitRunner) ResumeThread(context.Context, string, bool, codexapp.RoleSettings) (codexapp.Thread, error) {
	return codexapp.Thread{ID: "reagent-limit-thread"}, nil
}
func (r *limitRunner) RunRoleTurn(_ context.Context, id, prompt string, _ codexapp.RoleSettings, _ func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	r.turns++
	r.prompt = prompt
	result, _ := json.Marshal(map[string]any{
		"disposition": "decision_required", "summary": "Need a decision", "task_id": nil,
		"candidate": nil, "findings": nil, "evidence": nil,
		"question": map[string]any{"text": "Proceed?"},
	})
	return codexapp.TurnOutcome{ThreadID: id, TurnID: "limit-turn", Status: "completed", FinalText: string(result)}, nil
}
func (r *limitRunner) Stop(context.Context, string, string) stop.Result {
	return stop.Result{Status: "stopped_safely"}
}
func (r *limitRunner) Close() error { return nil }

func limitState(t *testing.T, root string, attempts, maxAttempts, assessments, maxAssessments int) (string, string, State) {
	t.Helper()
	repo := testRepo(t)
	base := gitTest(repo, "rev-parse", "HEAD")
	gitTest(repo, "switch", "-qc", "task/APP-002")
	if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("candidate\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(repo, "add", "feature.txt")
	gitTest(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "candidate")
	candidate := gitTest(repo, "rev-parse", "HEAD")
	dir, err := storage(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	evidence := filepath.Join(dir, "evidence", "APP-002", "task.md")
	review := reviewPath(dir, "APP-002", assessments)
	writeEvidence(evidence)
	writeEvidence(review)
	s := State{
		Repository: repo, Parent: "main", Base: base, Branch: "task/APP-002", TaskID: "APP-002", Candidate: candidate,
		Phase: "limit_exhausted", LimitPhase: "correcting", Evidence: evidence, ReviewEvidence: review,
		Verdict: "changes_requested", ReviewSummary: "Fix the boundary", Findings: []string{"retain prior counts"},
		Attempts: attempts, MaxAttempts: maxAttempts, Assessments: assessments, MaxAssessments: maxAssessments,
		Profiles: map[dispatch.Role]codexapp.RoleSettings{
			dispatch.Reagent: {Role: "reagent", Model: "test", Effort: "medium", Sandbox: "danger-full-access", ApprovalPolicy: "never"},
		},
		Threads: map[string]string{"reagent": "retained-reagent-thread"}, RoleTurns: map[string]bool{"reagent": true},
	}
	setLimitExhausted(&s, "correcting")
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	return repo, dir, s
}

func TestReviewLimitIncreaseContinuesRetainedCorrection(t *testing.T) {
	root := t.TempDir()
	repo, dir, _ := limitState(t, root, 7, 12, 3, 3)
	runner := &limitRunner{}
	opened := false
	s, err := Run(context.Background(), Options{Repository: repo, Root: root, ApproveReviewLimitIncrease: 5}, func(context.Context, string, io.Writer) (Runner, error) {
		opened = true
		stored, readErr := read(dir)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if stored.Assessments != 3 || stored.MaxAssessments != 5 || stored.Attempts != 7 || stored.MaxAttempts != 12 || len(stored.LimitIncreases) != 1 || stored.LimitIncreases[0].PreviousTotal != 3 || stored.LimitIncreases[0].AuthorizedTotal != 5 || stored.LimitIncreases[0].AuthorizedAt.IsZero() || stored.Phase != "correcting" {
			t.Fatalf("increase not persisted before provider open: %+v", stored)
		}
		return runner, nil
	})
	if err != nil || !opened || runner.turns != 1 || s.Phase != "blocked" || s.Assessments != 3 || s.MaxAssessments != 5 || s.Attempts != 8 || !strings.Contains(runner.prompt, "retain prior counts") {
		t.Fatalf("review continuation: %+v err=%v turns=%d prompt=%q", s, err, runner.turns, runner.prompt)
	}

	reopened, err := Run(context.Background(), Options{Repository: repo, Root: root}, func(context.Context, string, io.Writer) (Runner, error) {
		t.Fatal("blocked continuation opened provider")
		return nil, nil
	})
	if err != nil || reopened.MaxAssessments != 5 || reopened.Assessments != 3 || reopened.Attempts != 8 || runner.turns != 1 {
		t.Fatalf("reopen changed consumption: %+v %v", reopened, err)
	}
}

func TestLimitIncreaseValidationIsAtomic(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{"non-increasing review", Options{ApproveReviewLimitIncrease: 3}},
		{"non-increasing invocation", Options{ApproveInvocationLimitIncrease: 12}},
		{"combined invalid", Options{ApproveReviewLimitIncrease: 5, ApproveInvocationLimitIncrease: 12}},
		{"startup setting change", Options{ApproveReviewLimitIncrease: 5, MaxAttempts: 20}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			repo, dir, before := limitState(t, root, 12, 12, 3, 3)
			tc.opts.Repository, tc.opts.Root = repo, root
			opens := 0
			_, err := Run(context.Background(), tc.opts, func(context.Context, string, io.Writer) (Runner, error) { opens++; return &limitRunner{}, nil })
			after, readErr := read(dir)
			if err == nil || readErr != nil || opens != 0 || after.MaxAttempts != before.MaxAttempts || after.MaxAssessments != before.MaxAssessments || len(after.LimitIncreases) != 0 {
				t.Fatalf("non-atomic rejection: after=%+v runErr=%v readErr=%v opens=%d", after, err, readErr, opens)
			}
		})
	}
}

func TestOneIncreaseDoesNotBypassOtherExhaustedLimit(t *testing.T) {
	root := t.TempDir()
	repo, _, _ := limitState(t, root, 12, 12, 3, 3)
	opens := 0
	s, err := Run(context.Background(), Options{Repository: repo, Root: root, ApproveReviewLimitIncrease: 5}, func(context.Context, string, io.Writer) (Runner, error) {
		opens++
		return &limitRunner{}, nil
	})
	if err != nil || opens != 0 || s.Phase != "limit_exhausted" || s.MaxAssessments != 5 || s.Assessments != 3 || s.MaxAttempts != 12 || s.Attempts != 12 || !strings.Contains(s.LastError, "invocation limit exhausted") {
		t.Fatalf("other exhausted limit bypassed: %+v %v opens=%d", s, err, opens)
	}
}

func TestCombinedLimitIncreaseAndLegacyPhaseContinuation(t *testing.T) {
	root := t.TempDir()
	repo, dir, s := limitState(t, root, 12, 12, 3, 3)
	s.LimitPhase = "" // representative record written before the pending phase was retained
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	runner := &limitRunner{}
	continued, err := Run(context.Background(), Options{Repository: repo, Root: root, ApproveReviewLimitIncrease: 5, ApproveInvocationLimitIncrease: 14}, func(context.Context, string, io.Writer) (Runner, error) {
		return runner, nil
	})
	if err != nil || runner.turns != 1 || continued.MaxAssessments != 5 || continued.MaxAttempts != 14 || continued.Assessments != 3 || continued.Attempts != 13 || len(continued.LimitIncreases) != 2 {
		t.Fatalf("combined legacy continuation: %+v %v", continued, err)
	}
}

func TestInvocationLimitIncreaseContinuesWhenReviewAllowanceRemains(t *testing.T) {
	root := t.TempDir()
	repo, _, _ := limitState(t, root, 12, 12, 2, 3)
	runner := &limitRunner{}
	s, err := Run(context.Background(), Options{Repository: repo, Root: root, ApproveInvocationLimitIncrease: 14}, func(context.Context, string, io.Writer) (Runner, error) {
		return runner, nil
	})
	if err != nil || runner.turns != 1 || s.MaxAttempts != 14 || s.Attempts != 13 || s.MaxAssessments != 3 || s.Assessments != 2 || len(s.LimitIncreases) != 1 || s.LimitIncreases[0].Allowance != "invocation" {
		t.Fatalf("invocation continuation: %+v %v", s, err)
	}
}

func TestInvocationIncreaseDoesNotReplaceExactCommitApproval(t *testing.T) {
	root := t.TempDir()
	repo, dir, s := limitState(t, root, 2, 2, 1, 3)
	s.Verdict = "approved"
	s.ApprovedCommit = s.Candidate
	s.Findings = nil
	setLimitExhausted(&s, "awaiting_approval")
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	opens := 0
	continued, err := Run(context.Background(), Options{Repository: repo, Root: root, ApproveInvocationLimitIncrease: 3}, func(context.Context, string, io.Writer) (Runner, error) {
		opens++
		return &limitRunner{}, nil
	})
	if err != nil || opens != 0 || continued.Phase != "awaiting_approval" || continued.AuthorizedCommit != "" || continued.ApprovedCommit != continued.Candidate || continued.Attempts != 2 || continued.MaxAttempts != 3 {
		t.Fatalf("increase bypassed exact approval: %+v %v opens=%d", continued, err, opens)
	}
}

func TestInvalidIncreaseDoesNotAdvanceAwaitingApprovalState(t *testing.T) {
	root := t.TempDir()
	repo, dir, s := limitState(t, root, 2, 2, 1, 3)
	s.Phase = "awaiting_approval"
	s.LimitPhase = ""
	s.Verdict = "approved"
	s.ApprovedCommit = s.Candidate
	s.LastError = ""
	if err := save(dir, s); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), Options{Repository: repo, Root: root, ApproveInvocationLimitIncrease: 2}, func(context.Context, string, io.Writer) (Runner, error) {
		t.Fatal("invalid request opened provider")
		return nil, nil
	})
	stored, readErr := read(dir)
	if err == nil || readErr != nil || stored.Phase != "awaiting_approval" || stored.LimitPhase != "" || len(stored.LimitIncreases) != 0 {
		t.Fatalf("invalid request changed state: %+v runErr=%v readErr=%v", stored, err, readErr)
	}
}

func TestLimitIncreaseRequiresActiveWork(t *testing.T) {
	repo := testRepo(t)
	root := t.TempDir()
	opens := 0
	_, err := Run(context.Background(), Options{Repository: repo, Root: root, ApproveReviewLimitIncrease: 4}, func(context.Context, string, io.Writer) (Runner, error) {
		opens++
		return &limitRunner{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "no active work") || opens != 0 {
		t.Fatalf("missing active work accepted: %v opens=%d", err, opens)
	}
}
