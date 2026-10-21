package selection

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/integration"
)

type continuingAgent struct {
	*fakeAgent
	resumes int
}

func (a *continuingAgent) ResumeThread(_ context.Context, id string, hadTurn bool, settings codexapp.RoleSettings) (codexapp.Thread, error) {
	if id != "fresh-thread" || !hadTurn || settings.Role != "reagent" {
		panic("reagent binding changed")
	}
	a.resumes++
	return codexapp.Thread{ID: id}, nil
}

type continuingReviewer struct {
	*fakeReviewer
	resumes int
}

func (a *continuingReviewer) ResumeThread(_ context.Context, id string, hadTurn bool, settings codexapp.RoleSettings) (codexapp.Thread, error) {
	if id != "reviewer-thread" || !hadTurn || settings.Role != "iragent" {
		panic("reviewer binding changed")
	}
	a.resumes++
	return codexapp.Thread{ID: id}, nil
}

func TestCorrectedCandidateReturnsToSameReviewer(t *testing.T) {
	repo, external, policy := testRepo(t)
	first := readyAgent(t, repo, external, readyEvidence(), false, true)
	agent := &continuingAgent{fakeAgent: first}
	reviewer := &continuingReviewer{fakeReviewer: &fakeReviewer{}}
	var firstCandidate string
	reviewer.run = func(prompt string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		store, err := activework.Open(repo, external)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := store.Status()
		if err != nil {
			t.Fatal(err)
		}
		if firstCandidate == "" {
			firstCandidate = snapshot.Candidate.Commit
			first.run = func(prompt string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
				if !strings.Contains(prompt, firstCandidate) || !strings.Contains(prompt, "Please correct feature.go") || strings.Contains(prompt, "findings: none") {
					t.Fatal("prior findings absent from correction prompt")
				}
				workspace := filepath.Join(external, "workspaces", snapshot.RunID)
				if err := os.WriteFile(filepath.Join(workspace, "feature.go"), []byte("package feature\n\nfunc Corrected() bool { return true }\n"), 0600); err != nil {
					t.Fatal(err)
				}
				evidence := readyEvidence()
				evidence.Progress = "Corrected feature.go after independent findings."
				evidence.SelfReview = "Rechecked corrected feature and unchanged task scope."
				body, _ := json.Marshal(evidence)
				if err := invoke(t, handler, "checkpoint-correction", "save_checkpoint", map[string]string{"content": string(body)}); err != nil {
					t.Fatal(err)
				}
				return codexapp.TurnOutcome{ThreadID: "fresh-thread", TurnID: "correction-turn", Status: "completed", FinalText: `{"disposition":"ready_for_review","summary":"corrected"}`}, nil
			}
		} else {
			if snapshot.Candidate.Commit == firstCandidate || snapshot.Review.Number != 2 || !strings.Contains(prompt, firstCandidate) || !strings.Contains(prompt, "Please correct feature.go") {
				t.Fatal("reassessment lacks exact corrected binding or findings")
			}
			if head, _ := gitOutput(reviewer.cwd, "rev-parse", "HEAD"); head != snapshot.Candidate.Commit {
				t.Fatal("review checkout was not advanced")
			}
		}
		turn := "review-turn-1"
		verdict := "changes_requested"
		if snapshot.Review.Number == 2 {
			turn, verdict = "review-turn-2", "approved"
		}
		final := `{"disposition":"` + verdict + `","summary":"checked","task_id":"APP-002","candidate":"` + snapshot.Candidate.Commit + `","findings":null,"question":null}`
		if verdict == "changes_requested" {
			args, _ := json.Marshal(map[string]string{"content": "findings: none"})
			response, err := handler(context.Background(), codexapp.ToolCall{Role: "iragent", ThreadID: "reviewer-thread", TurnID: turn, CallID: "assessment-one", Tool: "save_assessment", Arguments: args})
			if err != nil || !response.Success || response.Commit == nil {
				t.Fatalf("review callback: %+v, %v", response, err)
			}
			if err := response.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			final = `{"disposition":"changes_requested","summary":"checked","task_id":"APP-002","candidate":"` + snapshot.Candidate.Commit + `","findings":["Please correct feature.go"],"question":null}`
		}
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: turn, Status: "completed", FinalText: final}, nil
	}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent, Reviewer: reviewer})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "approved" || result.Snapshot.Review.Number != 2 || result.Snapshot.Candidate.Commit == firstCandidate || len(result.Snapshot.Dispatch.Assessments) != 2 || agent.starts != 1 || agent.resumes != 1 || reviewer.starts != 1 || reviewer.resumes != 1 {
		t.Fatalf("bounded cycle: %+v, reagent starts/resumes %d/%d, reviewer starts/resumes %d/%d", result, agent.starts, agent.resumes, reviewer.starts, reviewer.resumes)
	}
	if head, _ := gitOutput(repo, "rev-parse", "HEAD"); head != result.Snapshot.ParentCommit {
		t.Fatal("parent moved")
	}
	store, err := activework.Open(repo, external)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.FinishAssessment(1, "approved", "stale-turn"); err == nil {
		t.Fatal("stale approval was accepted")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	approvalConfig := integration.Config{Repository: repo, Root: external, Reagent: agent, Reviewer: reviewer}
	acceptance, err := integration.Offer(approvalConfig)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := integration.Apply(context.Background(), approvalConfig, acceptance.ID); err != nil {
		t.Fatal(err)
	}
	if head, _ := gitOutput(repo, "rev-parse", "HEAD"); head != result.Snapshot.Candidate.Commit {
		t.Fatal("operator-authorized integration did not retain reviewed candidate")
	}
}

func TestCorrectionDispatchExhaustionRetainsFindings(t *testing.T) {
	repo, external, policy := testRepo(t)
	policy.Limits.Dispatches = 2 // selection and first review consume the allowance
	agent := &continuingAgent{fakeAgent: readyAgent(t, repo, external, readyEvidence(), false, true)}
	reviewer := &continuingReviewer{fakeReviewer: &fakeReviewer{}}
	reviewer.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		reviewCheckpoint(t, handler, "Fix the feature implementation.")
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "completed", FinalText: `{"disposition":"changes_requested","summary":"source correction needed","findings":["Fix the feature implementation."]}`}, nil
	}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent, Reviewer: reviewer})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "decision_required" || result.Snapshot.Hold != "limit-exhausted" || result.Snapshot.Candidate == nil || result.Snapshot.Review == nil || result.Snapshot.Review.Verdict != "changes_requested" || !strings.Contains(result.Snapshot.PendingRequest, "extend dispatch allowance") || agent.resumes != 0 {
		t.Fatalf("exhausted correction lost binding: %+v", result)
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
	findings, err := owner.ReviewEvidence(1)
	if err != nil || !strings.Contains(findings, "Fix the feature implementation") {
		t.Fatalf("findings were lost: %q, %v", findings, err)
	}
}

func TestAssessmentExhaustionRetainsReviewedCandidate(t *testing.T) {
	repo, external, policy := testRepo(t)
	policy.Limits.Assessments = 1
	agent := &continuingAgent{fakeAgent: readyAgent(t, repo, external, readyEvidence(), false, true)}
	reviewer := &continuingReviewer{fakeReviewer: &fakeReviewer{}}
	reviewer.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		reviewCheckpoint(t, handler, "Correct feature.go before approval.")
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "completed", FinalText: `{"disposition":"changes_requested","summary":"correction required","findings":["Correct feature.go before approval."]}`}, nil
	}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent, Reviewer: reviewer})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "decision_required" || result.Snapshot.Hold != "limit-exhausted" || result.Snapshot.Review.Number != 1 || len(result.Snapshot.Dispatch.Assessments) != 1 || !strings.Contains(result.Snapshot.PendingRequest, "extend assessments to 2") || agent.resumes != 0 {
		t.Fatalf("assessment exhaustion: %+v", result)
	}
}
