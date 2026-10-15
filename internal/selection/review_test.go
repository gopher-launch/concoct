package selection

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/review"
)

type fakeReviewer struct {
	starts int
	cwd    string
	run    func(string, codexapp.ToolHandler) (codexapp.TurnOutcome, error)
}

func (a *fakeReviewer) StartThread(_ context.Context, cwd string, settings codexapp.RoleSettings, tools []codexapp.ToolSpec) (codexapp.Thread, error) {
	a.starts++
	a.cwd = cwd
	if settings.Role != "iragent" || settings.Effort != "high" || settings.Sandbox != "danger-full-access" || len(tools) != 1 || tools[0].Name != "save_assessment" {
		return codexapp.Thread{}, errors.New("reviewer settings or host tool differ")
	}
	return codexapp.Thread{ID: "reviewer-thread"}, nil
}
func (a *fakeReviewer) RunTurn(_ context.Context, _ string, prompt string, _ codexapp.RoleSettings, handler codexapp.ToolHandler, _ func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	return a.run(prompt, handler)
}
func (a *fakeReviewer) Idle(context.Context, string) error { return nil }

func reviewCheckpoint(t *testing.T, handler codexapp.ToolHandler, content string) {
	t.Helper()
	args, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		t.Fatal(err)
	}
	response, err := handler(context.Background(), codexapp.ToolCall{Role: "iragent", ThreadID: "reviewer-thread", TurnID: "review-turn", CallID: "assessment-one", Tool: "save_assessment", Arguments: args})
	if err != nil || !response.Success || response.Commit == nil {
		t.Fatalf("review callback: %+v, %v", response, err)
	}
	if err := response.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticInitialReviewVerdicts(t *testing.T) {
	for _, tc := range []struct{ disposition, status, phase string }{{"approved", "approved", "ready-for-acceptance"}, {"changes_requested", "recovery_required", "changes-requested"}} {
		t.Run(tc.disposition, func(t *testing.T) {
			repo, external, policy := testRepo(t)
			base, _ := gitOutput(repo, "rev-parse", "HEAD")
			reviewer := &fakeReviewer{}
			reviewer.run = func(prompt string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
				store, err := activework.Open(repo, external)
				if err != nil {
					t.Fatal(err)
				}
				snapshot, err := store.Status()
				if err != nil || snapshot.Candidate == nil || snapshot.Review == nil || snapshot.Review.ThreadID != "reviewer-thread" {
					t.Fatalf("review not bound before turn: %+v, %v", snapshot, err)
				}
				if !strings.Contains(prompt, snapshot.Candidate.Commit) || !strings.Contains(prompt, snapshot.Review.ScopeSHA256) || !strings.Contains(prompt, "Preserve the task boundary") || !strings.Contains(prompt, "All packages passed") {
					t.Fatal("review prompt lacks candidate, scope or engineering evidence")
				}
				if head, _ := gitOutput(reviewer.cwd, "rev-parse", "HEAD"); head != snapshot.Candidate.Commit {
					t.Fatal("reviewer checkout not pinned to candidate")
				}
				if _, err := gitOutput(reviewer.cwd, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil {
					t.Fatal("reviewer checkout is not detached")
				}
				reviewCheckpoint(t, handler, "Checked exact candidate source and criterion; findings: none.")
				return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "completed", FinalText: `{"disposition":"` + tc.disposition + `","summary":"independent assessment complete"}`}, nil
			}
			result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true), Reviewer: reviewer})
			if err != nil || result.Status != tc.status || result.Snapshot.Phase != tc.phase || result.Snapshot.Review == nil || result.Snapshot.Review.Verdict != tc.disposition || result.Snapshot.Review.Number != 1 || reviewer.starts != 1 {
				t.Fatalf("automatic independent review: %+v, %v", result, err)
			}
			if len(result.Snapshot.Dispatch.Assessments) != 1 || result.Snapshot.Dispatch.Assessments[0].Candidate != result.Snapshot.Candidate.Commit {
				t.Fatalf("review assessment consumption: %+v", result.Snapshot.Dispatch)
			}
			files, err := filepath.Glob(filepath.Join(external, "repos", "*", "runs", result.RunID, "evidence", "review-*.md"))
			if err != nil || len(files) != 1 || !strings.HasSuffix(files[0], "review-01.md") {
				t.Fatalf("review file numbering: %v, %v", files, err)
			}
			content, err := os.ReadFile(files[0])
			if err != nil || !strings.Contains(string(content), result.Snapshot.Candidate.Commit) || !strings.Contains(string(content), result.Snapshot.Review.ScopeSHA256) || !strings.Contains(string(content), "Verdict: "+tc.disposition) {
				t.Fatalf("review binding or verdict missing: %q, %v", content, err)
			}
			store, _ := activework.Open(repo, external)
			owner, err := store.Acquire()
			if err != nil {
				t.Fatal(err)
			}
			if err := owner.SaveReview(1, "rewrite finalized assessment"); err == nil {
				t.Fatal("finalized assessment was mutable")
			}
			owner.Close()
			if head, _ := gitOutput(repo, "rev-parse", "HEAD"); head != base {
				t.Fatal("parent changed during review")
			}
			if err := requireClean(repo); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReviewInterruptionRetainsPendingNumberAndCheckpoint(t *testing.T) {
	repo, external, policy := testRepo(t)
	reviewer := &fakeReviewer{}
	reviewer.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		reviewCheckpoint(t, handler, "Assessment in progress; candidate evidence inspected.")
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "interrupted"}, errors.New("provider interrupted")
	}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true), Reviewer: reviewer})
	if err == nil || result.Status != "recovery_required" || result.Snapshot.Review == nil || result.Snapshot.Review.Number != 1 || result.Snapshot.Review.Verdict != "" || result.Snapshot.Hold != "recovery-required" {
		t.Fatalf("interrupted review was not retained: %+v, %v", result, err)
	}
	if len(result.Snapshot.Dispatch.Assessments) != 0 || len(result.Snapshot.Dispatch.Attempts) != 2 || result.Snapshot.Dispatch.Attempts[1].Status != "uncertain" {
		t.Fatalf("interrupted dispatch consumption: %+v", result.Snapshot.Dispatch)
	}
	store, _ := activework.Open(repo, external)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	content, err := owner.ReviewEvidence(1)
	if err != nil || !strings.Contains(content, "Assessment in progress") {
		t.Fatalf("pending assessment lost: %q, %v", content, err)
	}
	second, err := review.Run(context.Background(), review.Config{Agent: reviewer}, owner, store)
	if err != nil || second.Status != "recovery_required" || second.Snapshot.Review.Number != 1 || reviewer.starts != 1 {
		t.Fatalf("interrupted review created another assessment: %+v, %v", second, err)
	}
}

func TestReviewSourceDriftAndGenuineBlockerStayDistinct(t *testing.T) {
	for _, tc := range []struct{ name, disposition string }{{"review checkout drift", "approved"}, {"task checkout drift", "approved"}, {"new task source", "approved"}, {"new ignored task source", "approved"}, {"blocker", "decision_required"}} {
		t.Run(tc.name, func(t *testing.T) {
			repo, external, policy := testRepo(t)
			reviewer := &fakeReviewer{}
			reviewer.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
				reviewCheckpoint(t, handler, "Inspected candidate; need to report findings.")
				if tc.name == "review checkout drift" {
					if err := os.WriteFile(filepath.Join(reviewer.cwd, "source.txt"), []byte("reviewer edit\n"), 0600); err != nil {
						t.Fatal(err)
					}
				} else if tc.name == "task checkout drift" || tc.name == "new task source" || tc.name == "new ignored task source" {
					store, err := activework.Open(repo, external)
					if err != nil {
						t.Fatal(err)
					}
					snapshot, err := store.Status()
					if err != nil {
						t.Fatal(err)
					}
					name, content := "feature.go", "package changed\n"
					if tc.name == "new task source" {
						name, content = "newsource.go", "package feature\n"
					} else if tc.name == "new ignored task source" {
						name, content = "late.out", "ignored deliverable source\n"
					}
					if err := os.WriteFile(filepath.Join(external, "workspaces", snapshot.RunID, name), []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
				}
				return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "completed", FinalText: `{"disposition":"` + tc.disposition + `","summary":"external decision required"}`}, nil
			}
			result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true), Reviewer: reviewer})
			if tc.name != "blocker" {
				if err == nil || result.Status != "recovery_required" || result.Snapshot.Hold != "recovery-required" {
					t.Fatalf("source drift approved: %+v, %v", result, err)
				}
			} else if err != nil || result.Status != "decision_required" || result.Snapshot.Hold != "operator-decision" {
				t.Fatalf("genuine blocker conflated with verdict: %+v, %v", result, err)
			}
			if result.Snapshot.Review == nil || result.Snapshot.Review.Verdict != "" || len(result.Snapshot.Dispatch.Assessments) != 0 {
				t.Fatalf("blocked review finalized: %+v", result.Snapshot)
			}
		})
	}
}

func TestNewTaskSourceBeforeReviewDispatchIsRejected(t *testing.T) {
	repo, external, policy := testRepo(t)
	ready, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true)})
	if err != nil || ready.Status != "ready_for_review" || ready.Snapshot.Candidate == nil {
		t.Fatalf("candidate freeze: %+v, %v", ready, err)
	}
	if err := os.WriteFile(filepath.Join(ready.Workspace, "late.go"), []byte("package feature\n"), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := activework.Open(repo, external)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	reviewer := &fakeReviewer{}
	result, err := review.Run(context.Background(), review.Config{Agent: reviewer}, owner, store)
	if err == nil || result.Status != "recovery_required" || result.Snapshot.Review == nil || result.Snapshot.Review.Verdict != "" || reviewer.starts != 0 || len(result.Snapshot.Dispatch.Attempts) != 1 {
		t.Fatalf("new source passed pre-dispatch gate: %+v, %v", result, err)
	}
}

func TestReviewGateStopsBeforeReviewerThread(t *testing.T) {
	repo, external, policy := testRepo(t)
	policy.Gates.Activities = map[dispatch.Activity]bool{dispatch.Review: true}
	reviewer := &fakeReviewer{}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true), Reviewer: reviewer})
	if err != nil || result.Status != "authorization_required" || result.Snapshot.Review == nil || result.Snapshot.Review.Number != 1 || reviewer.starts != 0 {
		t.Fatalf("review gate was bypassed: %+v, %v", result, err)
	}
	if result.Snapshot.Dispatch.Pending == nil || result.Snapshot.Dispatch.Pending.Binding.Candidate != result.Snapshot.Candidate.Commit {
		t.Fatalf("review authorization lacks candidate binding: %+v", result.Snapshot.Dispatch)
	}
	store, err := activework.Open(repo, external)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if err := owner.AuthorizeDispatch(result.Snapshot.Dispatch.Pending.ID); err != nil {
		t.Fatal(err)
	}
	reviewer.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		reviewCheckpoint(t, handler, "Approved after the configured dispatch authorization.")
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "completed", FinalText: `{"disposition":"approved","summary":"reviewed"}`}, nil
	}
	resumed, err := review.Run(context.Background(), review.Config{Agent: reviewer}, owner, store)
	if err != nil || resumed.Status != "approved" || resumed.Snapshot.Review.Number != 1 || reviewer.starts != 1 || len(resumed.Snapshot.Dispatch.Assessments) != 1 {
		t.Fatalf("authorized review did not reuse pending binding: %+v, %v", resumed, err)
	}
}

func TestCompletedReviewWithoutAssessmentCannotApprove(t *testing.T) {
	repo, external, policy := testRepo(t)
	reviewer := &fakeReviewer{}
	reviewer.run = func(_ string, _ codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "completed", FinalText: `{"disposition":"approved","summary":"claims approval"}`}, nil
	}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true), Reviewer: reviewer})
	if err == nil || result.Status != "recovery_required" || result.Snapshot.Hold != "recovery-required" || result.Snapshot.Review.Verdict != "" || len(result.Snapshot.Dispatch.Assessments) != 0 {
		t.Fatalf("provider completion implied review approval: %+v, %v", result, err)
	}
}

func TestReviewCheckpointMustBelongToCompletedTurn(t *testing.T) {
	repo, external, policy := testRepo(t)
	reviewer := &fakeReviewer{}
	reviewer.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		reviewCheckpoint(t, handler, "Assessment belongs to the earlier turn.")
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "different-turn", Status: "completed", FinalText: `{"disposition":"approved","summary":"claims approval"}`}, nil
	}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true), Reviewer: reviewer})
	if err == nil || result.Status != "recovery_required" || result.Snapshot.Review.Verdict != "" || len(result.Snapshot.Dispatch.Assessments) != 0 {
		t.Fatalf("stale review checkpoint approved: %+v, %v", result, err)
	}
}
