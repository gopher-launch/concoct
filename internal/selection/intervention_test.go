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
)

func blockerResult(sourceChange bool) string {
	result, _ := json.Marshal(codexapp.RoleResult{Disposition: "decision_required", Summary: "operator input needed", Question: &codexapp.RoleQuestion{
		Objective: "Complete APP-002", Evidence: "Tried both documented inputs; neither names a mode.", Missing: "Choose the mode.", SafeWork: "Tests can be read.", NextStep: "Resume requester after an answer.", SourceChange: sourceChange,
		Options: []codexapp.RoleQuestionOption{{ID: "safe", Description: "Use safe mode", Tradeoff: "Slower execution"}},
	}})
	return string(result)
}

func claimedBlocker(t *testing.T, repo, external string, agent *fakeAgent) {
	t.Helper()
	agent.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		store, _ := activework.Open(repo, external)
		snapshot, _ := store.Status()
		workspace := filepath.Join(external, "workspaces", snapshot.RunID)
		if _, err := gitOutput(workspace, "switch", "-qc", "work/app-002"); err != nil {
			t.Fatal(err)
		}
		if err := invoke(t, handler, "claim", "claim_task", claimArgs("APP-002", "work/app-002")); err != nil {
			t.Fatal(err)
		}
		if err := invoke(t, handler, "checkpoint", "save_checkpoint", map[string]string{"content": "Blocked after investigating both documented inputs; waiting for operator mode choice."}); err != nil {
			t.Fatal(err)
		}
		return codexapp.TurnOutcome{ThreadID: "fresh-thread", TurnID: "blocker-turn", Status: "completed", FinalText: blockerResult(false)}, nil
	}
}

type failedResumeAgent struct{ *fakeAgent }

func (*failedResumeAgent) ResumeThread(context.Context, string, bool, codexapp.RoleSettings) (codexapp.Thread, error) {
	return codexapp.Thread{}, errors.New("runner unavailable")
}

func TestReagentQuestionAnswerDurableBeforeFailedResume(t *testing.T) {
	repo, external, policy := testRepo(t)
	agent := &fakeAgent{}
	claimedBlocker(t, repo, external, agent)
	cfg := Config{Repository: repo, Root: external, Policy: policy, Agent: agent}
	result, err := Execute(context.Background(), cfg)
	if err != nil || result.Status != "decision_required" || result.Snapshot.Question == nil || result.Snapshot.Question.Requester != "reagent" {
		t.Fatalf("blocker: %+v, %v", result, err)
	}
	q := result.Snapshot.Question
	if q.Objective == "" || q.Evidence == "" || len(q.Options) != 1 {
		t.Fatalf("question context lost: %+v", q)
	}
	beforeTurns := agent.turns
	continued, err := Continue(context.Background(), cfg)
	if err != nil || continued.Status != "decision_required" || agent.turns != beforeTurns {
		t.Fatalf("unanswered blocker dispatched: %+v, %v", continued, err)
	}
	store, _ := activework.Open(repo, external)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.AnswerQuestion("wrong-id", "choice", "safe"); err == nil {
		t.Fatal("wrong question accepted")
	}
	if err := owner.AnswerQuestion(q.ID, "choice", "invalid"); err == nil {
		t.Fatal("unknown choice accepted")
	}
	owner.Close()
	cfg.Agent = &failedResumeAgent{agent}
	failed, err := Answer(context.Background(), cfg, AnswerInput{QuestionID: q.ID, ChoiceID: "safe"})
	if err != nil || failed.Status != "recovery_required" {
		t.Fatalf("expected held failed requester resume: %+v, %v", failed, err)
	}
	snapshot, err := store.Status()
	if err != nil || snapshot.Question == nil || snapshot.Question.AnswerMode != "choice" || snapshot.Question.AnswerText != "safe" {
		t.Fatalf("answer not durable: %+v, %v", snapshot, err)
	}
	files, _ := filepath.Glob(filepath.Join(external, "repos", "*", "runs", result.RunID, "evidence", "task.md"))
	if len(files) != 1 {
		t.Fatalf("task evidence missing: %v", files)
	}
	body, _ := os.ReadFile(files[0])
	if !strings.Contains(string(body), "Answer to "+q.ID+": choice: safe") {
		t.Fatal("answer evidence missing before failed launch")
	}
	owner, err = store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.AnswerQuestion(q.ID, "text", "retry"); err == nil {
		t.Fatal("stale answer accepted")
	}
	owner.Close()
	failedID := snapshot.Dispatch.Attempts[len(snapshot.Dispatch.Attempts)-1].ID
	retrying := &continuingAgent{fakeAgent: agent}
	agent.idle = func() error { return errors.New("requester still busy") }
	cfg.Agent = retrying
	stalled, err := Continue(context.Background(), cfg)
	if err != nil || stalled.Status != "recovery_required" || len(stalled.Snapshot.Dispatch.Attempts) != len(snapshot.Dispatch.Attempts) {
		t.Fatalf("non-idle requester was retried: %+v, %v", stalled, err)
	}
	agent.idle = nil
	agent.run = func(prompt string, _ codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		if !strings.Contains(prompt, "Operator answer to "+q.ID) || !strings.Contains(prompt, "safe") {
			t.Fatal("retry did not receive the durable answer")
		}
		return codexapp.TurnOutcome{ThreadID: "fresh-thread", TurnID: "retry-turn", Status: "completed", FinalText: `{"disposition":"engineering","summary":"verified answer"}`}, nil
	}
	retried, err := Continue(context.Background(), cfg)
	if err != nil || retried.Status != "engineering" || retried.Snapshot.Question != nil || len(retried.Snapshot.Dispatch.Attempts) != len(snapshot.Dispatch.Attempts)+1 || retrying.resumes != 1 {
		t.Fatalf("answered requester did not retry: %+v, %v", retried, err)
	}
	last := retried.Snapshot.Dispatch.Attempts[len(retried.Snapshot.Dispatch.Attempts)-1]
	if last.Binding.Activity != dispatch.Recovery || last.Binding.Episode != failedID {
		t.Fatalf("retry did not preserve bounded recovery episode: %+v", last)
	}
}

func TestReviewerQuestionRoutesToReviewerAfterAnswer(t *testing.T) {
	repo, external, policy := testRepo(t)
	reviewer := &continuingReviewer{fakeReviewer: &fakeReviewer{}}
	calls := 0
	reviewer.run = func(prompt string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		calls++
		if calls == 1 {
			return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "completed", FinalText: blockerResult(false)}, nil
		}
		if !strings.Contains(prompt, "Answer to") || !strings.Contains(prompt, "safe") {
			t.Fatal("answered question absent from reviewer context")
		}
		body, _ := json.Marshal(map[string]string{"content": "Mode chosen; candidate verified."})
		response, err := handler(context.Background(), codexapp.ToolCall{Role: "iragent", ThreadID: "reviewer-thread", TurnID: "review-turn-2", CallID: "assessment-two", Tool: "save_assessment", Arguments: body})
		if err != nil || !response.Success || response.Commit == nil {
			t.Fatalf("review assessment: %+v, %v", response, err)
		}
		if err := response.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn-2", Status: "completed", FinalText: `{"disposition":"approved","summary":"verified"}`}, nil
	}
	cfg := Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true), Reviewer: reviewer}
	result, err := Execute(context.Background(), cfg)
	if err != nil || result.Status != "decision_required" || result.Snapshot.Question == nil || result.Snapshot.Question.Requester != "iragent" {
		t.Fatalf("review blocker: %+v, %v", result, err)
	}
	q := result.Snapshot.Question
	answerFile := filepath.Join(t.TempDir(), "answer.txt")
	if err := os.WriteFile(answerFile, []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err = Answer(context.Background(), cfg, AnswerInput{QuestionID: q.ID, File: answerFile})
	if err != nil || result.Status != "approved" || result.Snapshot.Question != nil || calls != 2 {
		t.Fatalf("reviewer answer route: %+v, %v, calls=%d", result, err, calls)
	}
}

func TestAnsweredLaunchRetryObeysRecoveryLimit(t *testing.T) {
	repo, external, policy := testRepo(t)
	policy.Limits.Recovery = 1
	agent := &fakeAgent{}
	claimedBlocker(t, repo, external, agent)
	cfg := Config{Repository: repo, Root: external, Policy: policy, Agent: agent}
	result, err := Execute(context.Background(), cfg)
	if err != nil || result.Snapshot.Question == nil {
		t.Fatalf("setup: %+v, %v", result, err)
	}
	cfg.Agent = &failedResumeAgent{agent}
	result, err = Answer(context.Background(), cfg, AnswerInput{QuestionID: result.Snapshot.Question.ID, Text: "safe"})
	if err != nil || result.Status != "recovery_required" {
		t.Fatalf("initial failure: %+v, %v", result, err)
	}
	result, err = Continue(context.Background(), cfg)
	if err != nil || result.Status != "recovery_required" || len(result.Snapshot.Dispatch.Attempts) != 3 {
		t.Fatalf("bounded recovery attempt: %+v, %v", result, err)
	}
	result, err = Continue(context.Background(), cfg)
	if err != nil || result.Status != "recovery_required" || len(result.Snapshot.Dispatch.Attempts) != 3 || result.Snapshot.Question == nil || result.Snapshot.Question.AnswerText != "safe" {
		t.Fatalf("recovery limit bypassed or answer lost: %+v, %v", result, err)
	}
}

func TestReviewerSourceChangeAnswerRoutesThroughReagent(t *testing.T) {
	repo, external, policy := testRepo(t)
	reviewer := &continuingReviewer{fakeReviewer: &fakeReviewer{}}
	reviewer.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		reviewCheckpoint(t, handler, "Candidate needs operator-selected behavior before approval.")
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "completed", FinalText: blockerResult(true)}, nil
	}
	agent := readyAgent(t, repo, external, readyEvidence(), false, true)
	cfg := Config{Repository: repo, Root: external, Policy: policy, Agent: agent, Reviewer: reviewer}
	result, err := Execute(context.Background(), cfg)
	if err != nil || result.Status != "decision_required" || result.Snapshot.Question == nil || !result.Snapshot.Question.SourceChange {
		t.Fatalf("source-change blocker: %+v, %v", result, err)
	}
	cfg.Agent = &failedResumeAgent{agent}
	_, _ = Answer(context.Background(), cfg, AnswerInput{QuestionID: result.Snapshot.Question.ID, Text: "Use the selected behavior within APP-002 scope."})
	store, _ := activework.Open(repo, external)
	snapshot, err := store.Status()
	if err != nil || snapshot.Review == nil || snapshot.Review.Verdict != "changes_requested" || len(snapshot.Dispatch.Assessments) != 1 || snapshot.Question == nil || snapshot.Question.AnswerMode != "text" {
		t.Fatalf("answer did not route through reagent: %+v, %v", snapshot, err)
	}
}

func TestSourceChangeQuestionRequiresReceiptedAssessment(t *testing.T) {
	repo, external, policy := testRepo(t)
	reviewer := &fakeReviewer{}
	reviewer.run = func(_ string, _ codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "completed", FinalText: blockerResult(true)}, nil
	}
	cfg := Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true), Reviewer: reviewer}
	result, err := Execute(context.Background(), cfg)
	if err == nil || result.Status != "recovery_required" || result.Snapshot.Question != nil || result.Snapshot.Hold != "recovery-required" || result.Snapshot.Review == nil || result.Snapshot.Review.Verdict != "" {
		t.Fatalf("unreceipted source-changing question accepted: %+v, %v", result, err)
	}
	store, _ := activework.Open(repo, external)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.AnswerQuestion("missing-question", "text", "change source"); err == nil {
		t.Fatal("answer accepted without routable question")
	}
	owner.Close()
}

func TestQuestionAnswerDoesNotAuthorizeNextDispatch(t *testing.T) {
	repo, external, policy := testRepo(t)
	policy.Gates.Activities = map[dispatch.Activity]bool{dispatch.Engineering: true}
	agent := &fakeAgent{}
	claimedBlocker(t, repo, external, agent)
	cfg := Config{Repository: repo, Root: external, Policy: policy, Agent: agent}
	result, err := Execute(context.Background(), cfg)
	if err != nil || result.Snapshot.Question == nil {
		t.Fatalf("question setup: %+v, %v", result, err)
	}
	result, err = Answer(context.Background(), cfg, AnswerInput{QuestionID: result.Snapshot.Question.ID, Text: "Use safe mode"})
	if err != nil || result.Status != "authorization_required" || result.Snapshot.Dispatch.Pending == nil || result.Snapshot.Dispatch.Pending.Authorized || agent.turns != 1 {
		t.Fatalf("answer authorized unrelated dispatch: %+v, %v", result, err)
	}
}

func TestDeferReactivationAndTerminalAbandon(t *testing.T) {
	repo, external, policy := testRepo(t)
	agent := &fakeAgent{}
	claimedBlocker(t, repo, external, agent)
	cfg := Config{Repository: repo, Root: external, Policy: policy, Agent: agent}
	result, err := Execute(context.Background(), cfg)
	if err != nil || result.Status != "decision_required" {
		t.Fatalf("initial blocker: %+v, %v", result, err)
	}
	initial := result.Snapshot
	agent.idle = func() error { return errors.New("background command active") }
	if _, err := Defer(context.Background(), cfg, "APP-002", "awaiting input"); err == nil {
		t.Fatal("non-idle task deferred")
	}
	agent.idle = nil
	result, err = Defer(context.Background(), cfg, "APP-002", "awaiting input")
	if err != nil || result.Status != "deferred" {
		t.Fatalf("defer: %+v, %v", result, err)
	}
	store, _ := activework.Open(repo, external)
	status, _ := store.Status()
	if status.Active {
		t.Fatal("deferred task still active")
	}
	ids, err := store.DeferredTaskIDs()
	if err != nil || !ids["APP-002"] {
		t.Fatalf("deferred task not excluded: %v, %v", ids, err)
	}
	result, err = ResumeDeferred(context.Background(), cfg, "APP-002")
	if err != nil || result.Status != "decision_required" || result.RunID != initial.RunID || result.Snapshot.Question.ID != initial.Question.ID || len(result.Snapshot.Dispatch.Attempts) != len(initial.Dispatch.Attempts) {
		t.Fatalf("reactivation binding: %+v, %v", result, err)
	}
	ids, err = store.DeferredTaskIDs()
	if err != nil || ids["APP-002"] {
		t.Fatalf("deferred marker remained: %v, %v", ids, err)
	}
	result, err = Abandon(context.Background(), cfg, "APP-002", "operator ended task")
	if err != nil || result.Status != "abandoned" {
		t.Fatalf("abandon: %+v, %v", result, err)
	}
	if _, err := ResumeDeferred(context.Background(), cfg, "APP-002"); err == nil {
		t.Fatal("abandoned task reactivated")
	}
	ids, err = store.UnavailableTaskIDs()
	if err != nil || !ids["APP-002"] {
		t.Fatalf("abandoned task selectable: %v, %v", ids, err)
	}
}
