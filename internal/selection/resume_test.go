package selection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/stop"
)

type stoppedAgent struct {
	*fakeAgent
	cancel                context.CancelFunc
	turns, resumes, stops int
}

type stoppedReviewer struct {
	*fakeReviewer
	cancel         context.CancelFunc
	turns, resumes int
}

type observedReviewer struct {
	*fakeReviewer
	turns, reads int
	busy         bool
	idleChecks   int
}

type observedReagent struct {
	*fakeAgent
	items    []codexapp.DynamicToolItem
	observed *codexapp.ObservedTurn
	final    string
	reads    int
}

func (a *observedReagent) RunTurn(ctx context.Context, id, prompt string, settings codexapp.RoleSettings, handler codexapp.ToolHandler, onEvent func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	outcome, err := a.fakeAgent.RunTurn(ctx, id, prompt, settings, handler, onEvent)
	if err != nil {
		return outcome, err
	}
	if a.final != "" {
		outcome.FinalText = a.final
	}
	return outcome, errors.New("callback response acknowledgment unavailable")
}

func (a *observedReagent) ReadTurn(_ context.Context, threadID, turnID string) (codexapp.ObservedTurn, error) {
	a.reads++
	if threadID != "fresh-thread" || turnID != "one-turn" {
		return codexapp.ObservedTurn{}, errors.New("wrong provider turn")
	}
	if a.observed != nil {
		return *a.observed, nil
	}
	return codexapp.ObservedTurn{ID: turnID, Status: "completed", FinalText: `{"disposition":"ready_for_review","summary":"ready for independent review"}`, DynamicTools: a.items, ItemsKnown: true}, nil
}

func TestCompletedReagentRecoveryFailureReasonsPreserveBinding(t *testing.T) {
	validFinal := `{"disposition":"ready_for_review","summary":"ready for independent review"}`
	for _, tc := range []struct {
		name, id, status, final, reason string
		withItems                       bool
	}{
		{"identity mismatch", "other-turn", "completed", validFinal, "observed turn identity changed", true},
		{"nonterminal", "one-turn", "inProgress", validFinal, "still active or uncertain", true},
		{"missing final text", "one-turn", "completed", "", "no final-result text", true},
		{"malformed structured result", "one-turn", "completed", "{bad", "completed reagent result is invalid", true},
		{"missing callback item", "one-turn", "completed", validFinal, "callback evidence", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, external, policy := testRepo(t)
			ok := true
			items := []codexapp.DynamicToolItem{{ID: "claim-ready", Tool: "claim_task", Status: "completed", Success: &ok}, {ID: "checkpoint-ready", Tool: "save_checkpoint", Status: "completed", Success: &ok}}
			agent := &observedReagent{fakeAgent: readyAgent(t, repo, external, readyEvidence(), false, true), items: items}
			if tc.name == "malformed structured result" {
				agent.final = tc.final
			}
			first, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent})
			if err == nil || first.Status != "recovery_required" || len(first.Snapshot.Dispatch.Attempts) != 1 {
				t.Fatalf("initial hold: %+v %v", first, err)
			}
			before := first.Snapshot
			observed := codexapp.ObservedTurn{ID: tc.id, Status: tc.status, FinalText: tc.final, ItemsKnown: true}
			if tc.withItems {
				observed.DynamicTools = items
			}
			agent.observed = &observed
			result, err := Continue(context.Background(), Config{Repository: repo, Root: external, Agent: agent})
			if err != nil || result.Status != "recovery_required" || result.Snapshot.Recovery == nil || !strings.Contains(result.Snapshot.Recovery.Reason, tc.reason) {
				t.Fatalf("wrong recovery reason: result=%+v err=%v", result, err)
			}
			if agent.turns != 1 || agent.reads != 1 || result.Snapshot.TaskID != before.TaskID || result.Snapshot.TaskBranch != before.TaskBranch || result.Snapshot.Candidate != nil || len(result.Snapshot.Dispatch.Attempts) != 1 || result.Snapshot.Dispatch.Attempts[0].ID != before.Dispatch.Attempts[0].ID || result.Snapshot.Dispatch.Limits != before.Dispatch.Limits || !reflect.DeepEqual(result.Snapshot.RoleSettings, before.RoleSettings) || !reflect.DeepEqual(result.Snapshot.Counters, before.Counters) {
				t.Fatalf("recovery replayed or changed binding: before=%+v after=%+v turns=%d reads=%d", before, result.Snapshot, agent.turns, agent.reads)
			}
		})
	}
}

func TestCompletedReagentRecoveryRequiresSuccessfulBoundItemsAndReceipts(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mutate      func([]codexapp.DynamicToolItem) []codexapp.DynamicToolItem
		recoverable bool
	}{
		{"successful legacy record", func(v []codexapp.DynamicToolItem) []codexapp.DynamicToolItem { return v }, true},
		{"duplicate completed item", func(v []codexapp.DynamicToolItem) []codexapp.DynamicToolItem { return append(v, v[1]) }, true},
		{"missing checkpoint item", func(v []codexapp.DynamicToolItem) []codexapp.DynamicToolItem { return v[:1] }, false},
		{"failed checkpoint item", func(v []codexapp.DynamicToolItem) []codexapp.DynamicToolItem { v[1].Status = "failed"; return v }, false},
		{"mismatched call ID", func(v []codexapp.DynamicToolItem) []codexapp.DynamicToolItem { v[1].ID = "other"; return v }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, external, policy := testRepo(t)
			ok := true
			items := []codexapp.DynamicToolItem{{ID: "claim-ready", Tool: "claim_task", Status: "completed", Success: &ok}, {ID: "checkpoint-ready", Tool: "save_checkpoint", Status: "completed", Success: &ok}}
			agent := &observedReagent{fakeAgent: readyAgent(t, repo, external, readyEvidence(), false, true), items: tc.mutate(items)}
			first, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent})
			if err == nil || first.Status != "recovery_required" || first.Snapshot.Recovery == nil || len(first.Snapshot.Dispatch.Attempts) != 1 {
				t.Fatalf("initial hold: %+v %v", first, err)
			}
			result, err := Continue(context.Background(), Config{Repository: repo, Root: external, Agent: agent})
			if err != nil || agent.turns != 1 || agent.reads != 1 || len(result.Snapshot.Dispatch.Attempts) != 1 {
				t.Fatalf("reconciliation replayed or failed: %+v %v", result, err)
			}
			if tc.recoverable {
				if result.Status != "ready_for_review" || result.Snapshot.Candidate == nil {
					t.Fatalf("valid completion held: %+v", result)
				}
			} else if result.Status != "recovery_required" || result.Snapshot.Candidate != nil || result.Snapshot.Recovery == nil || !strings.Contains(result.Snapshot.Recovery.Reason, "callback evidence") {
				t.Fatalf("invalid completion advanced: %+v", result)
			}
		})
	}
}

func (a *observedReviewer) Idle(_ context.Context, id string) error {
	a.idleChecks++
	if id != "reviewer-thread" || a.busy {
		return errors.New("reviewer background work remains active")
	}
	return nil
}

func (a *observedReviewer) RunTurn(_ context.Context, id, prompt string, settings codexapp.RoleSettings, handler codexapp.ToolHandler, onEvent func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	a.turns++
	body, _ := json.Marshal(map[string]string{"content": "Exact candidate passes all criteria."})
	response, err := handler(context.Background(), codexapp.ToolCall{Role: "iragent", ThreadID: id, TurnID: "observed-review-turn", CallID: "observed-assessment", Tool: "save_assessment", Arguments: body})
	if err != nil {
		return codexapp.TurnOutcome{}, err
	}
	if err := response.Commit(context.Background()); err != nil {
		return codexapp.TurnOutcome{}, err
	}
	return codexapp.TurnOutcome{ThreadID: id, TurnID: "observed-review-turn", Status: "completed", FinalText: `{"disposition":"approved","summary":"checked"}`}, errors.New("response connection lost after provider completion")
}
func (a *observedReviewer) ReadTurn(_ context.Context, threadID, turnID string) (codexapp.ObservedTurn, error) {
	a.reads++
	if threadID != "reviewer-thread" || turnID != "observed-review-turn" {
		return codexapp.ObservedTurn{}, errors.New("unexpected turn")
	}
	ok := true
	return codexapp.ObservedTurn{ID: turnID, Status: "completed", FinalText: `{"disposition":"approved","summary":"checked"}`, DynamicTools: []codexapp.DynamicToolItem{{ID: "observed-assessment", Tool: "save_assessment", Status: "completed", Success: &ok}}, ItemsKnown: true}, nil
}

func TestObservedCompletedReviewFinalizedOnceWithoutReplay(t *testing.T) {
	repo, external, policy := testRepo(t)
	reviewer := &observedReviewer{fakeReviewer: &fakeReviewer{}}
	first, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true), Reviewer: reviewer})
	if err == nil || first.Status != "recovery_required" || len(first.Snapshot.Dispatch.Assessments) != 0 {
		t.Fatalf("uncertain response: %+v, %v", first, err)
	}
	result, err := Continue(context.Background(), Config{Repository: repo, Root: external, Reviewer: reviewer})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "approved" || len(result.Snapshot.Dispatch.Assessments) != 1 || reviewer.turns != 1 || reviewer.reads != 1 || result.Snapshot.Review.TurnID != "observed-review-turn" {
		t.Fatalf("completed turn reconciliation: %+v", result)
	}
	second, err := Continue(context.Background(), Config{Repository: repo, Root: external, Reviewer: reviewer})
	if err != nil || second.Status != "approved" || len(second.Snapshot.Dispatch.Assessments) != 1 || reviewer.turns != 1 {
		t.Fatalf("completed turn was replayed: %+v, %v", second, err)
	}
}

func TestObservedCompletedReviewWaitsForReviewerIdle(t *testing.T) {
	repo, external, policy := testRepo(t)
	reviewer := &observedReviewer{fakeReviewer: &fakeReviewer{}, busy: true}
	first, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true), Reviewer: reviewer})
	if err == nil || first.Status != "recovery_required" {
		t.Fatalf("lost response: %+v, %v", first, err)
	}
	blocked, err := Continue(context.Background(), Config{Repository: repo, Root: external, Reviewer: reviewer})
	if err != nil || blocked.Status != "recovery_required" || blocked.Snapshot.Review.Verdict != "" || len(blocked.Snapshot.Dispatch.Assessments) != 0 || reviewer.turns != 1 {
		t.Fatalf("busy reviewer was finalized or replayed: %+v, %v", blocked, err)
	}
	reviewer.busy = false
	completed, err := Continue(context.Background(), Config{Repository: repo, Root: external, Reviewer: reviewer})
	if err != nil || completed.Status != "approved" || len(completed.Snapshot.Dispatch.Assessments) != 1 || reviewer.turns != 1 || reviewer.idleChecks < 2 {
		t.Fatalf("quiescent reviewer did not finalize once: %+v, %v", completed, err)
	}
}

type zeroTurnReviewer struct {
	*fakeReviewer
	resumes, turns int
}

func (a *zeroTurnReviewer) ResumeThread(_ context.Context, id string, hadTurn bool, settings codexapp.RoleSettings) (codexapp.Thread, error) {
	a.resumes++
	if id != "reviewer-thread" || hadTurn || settings.Role != "iragent" || settings.Effort != "high" {
		return codexapp.Thread{}, errors.New("zero-turn reviewer binding changed")
	}
	return codexapp.Thread{}, codexapp.ErrProvisionalThread
}

func (a *zeroTurnReviewer) StartThread(ctx context.Context, cwd string, settings codexapp.RoleSettings, tools []codexapp.ToolSpec) (codexapp.Thread, error) {
	if _, err := a.fakeReviewer.StartThread(ctx, cwd, settings, tools); err != nil {
		return codexapp.Thread{}, err
	}
	return codexapp.Thread{ID: "replacement-reviewer"}, nil
}

func (a *zeroTurnReviewer) RunTurn(_ context.Context, id, _ string, _ codexapp.RoleSettings, handler codexapp.ToolHandler, _ func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	a.turns++
	if id != "replacement-reviewer" {
		return codexapp.TurnOutcome{}, errors.New("wrong reviewer thread dispatched")
	}
	body, _ := json.Marshal(map[string]string{"content": "Reconstructed reviewer assessed exact candidate."})
	response, err := handler(context.Background(), codexapp.ToolCall{Role: "iragent", ThreadID: id, TurnID: "recovered-review-turn", CallID: "recovered-assessment", Tool: "save_assessment", Arguments: body})
	if err != nil {
		return codexapp.TurnOutcome{}, err
	}
	if err := response.Commit(context.Background()); err != nil {
		return codexapp.TurnOutcome{}, err
	}
	return codexapp.TurnOutcome{ThreadID: id, TurnID: "recovered-review-turn", Status: "completed", FinalText: `{"disposition":"approved","summary":"checked"}`}, nil
}

func TestZeroTurnReviewerRecreatedWithSameAssessment(t *testing.T) {
	repo, external, policy := testRepo(t)
	first, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true)})
	if err != nil || first.Status != "ready_for_review" || first.Snapshot.Candidate == nil {
		t.Fatalf("candidate setup: %+v, %v", first, err)
	}
	store, err := activework.Open(repo, external)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	binding, err := owner.BeginReview("zero-turn-review")
	if err != nil {
		t.Fatal(err)
	}
	_, err = owner.ReserveDispatch(binding.AttemptID, dispatch.Binding{TaskID: first.TaskID, Role: dispatch.Iragent, Activity: dispatch.Review, Candidate: first.Snapshot.Candidate.Commit}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.BindReviewThread(binding.Number, "reviewer-thread"); err != nil {
		t.Fatal(err)
	}
	if err := owner.BindDispatchThread(binding.AttemptID, "reviewer-thread"); err != nil {
		t.Fatal(err)
	}
	owner.Close() // crash before turn/start intent
	reviewer := &zeroTurnReviewer{fakeReviewer: &fakeReviewer{}}
	result, err := Continue(context.Background(), Config{Repository: repo, Root: external, Reviewer: reviewer})
	if err != nil || result.Status != "approved" || result.Snapshot.Candidate.Commit != first.Snapshot.Candidate.Commit || result.Snapshot.Review.Number != 1 || result.Snapshot.Review.ThreadID != "replacement-reviewer" || reviewer.starts != 1 || reviewer.resumes != 1 || reviewer.turns != 1 || len(result.Snapshot.Dispatch.Assessments) != 1 {
		t.Fatalf("zero-turn reviewer recovery: %+v, %v", result, err)
	}
	attempts := result.Snapshot.Dispatch.Attempts
	if len(attempts) != 3 || attempts[1].Status != "failed" || attempts[2].Binding.Activity != dispatch.Recovery || attempts[2].Binding.Episode != attempts[1].ID {
		t.Fatalf("zero-turn recovery reset dispatch history: %+v", attempts)
	}
}

type lostCorrectionReviewer struct {
	*fakeReviewer
	reagent      *fakeAgent
	turns        int
	quietCalls   int
	idleCalls    int
	quietBlocked bool
}

func (a *lostCorrectionReviewer) StartThread(ctx context.Context, cwd string, settings codexapp.RoleSettings, tools []codexapp.ToolSpec) (codexapp.Thread, error) {
	if _, err := a.fakeReviewer.StartThread(ctx, cwd, settings, tools); err != nil {
		return codexapp.Thread{}, err
	}
	return codexapp.Thread{ID: fmt.Sprintf("reviewer-%d", a.starts)}, nil
}

func (a *lostCorrectionReviewer) ResumeThread(_ context.Context, id string, hadTurn bool, settings codexapp.RoleSettings) (codexapp.Thread, error) {
	if id != "reviewer-1" || !hadTurn || settings.Role != "iragent" {
		return codexapp.Thread{}, errors.New("lost reviewer binding changed")
	}
	return codexapp.Thread{}, codexapp.ErrMissingRollout
}

func (a *lostCorrectionReviewer) Idle(_ context.Context, id string) error {
	if id == "reviewer-1" {
		a.idleCalls++
		if a.idleCalls > 1 {
			return codexapp.ErrMissingRollout
		}
	}
	return nil
}

func (a *lostCorrectionReviewer) OwnedQuiet(context.Context) error {
	a.quietCalls++
	if a.quietBlocked {
		return errors.New("owned reviewer process still running")
	}
	return nil
}

func (a *lostCorrectionReviewer) RunTurn(_ context.Context, id, prompt string, _ codexapp.RoleSettings, handler codexapp.ToolHandler, _ func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	a.turns++
	turn, verdict := "review-turn-1", "changes_requested"
	if a.turns == 1 {
		a.reagent.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
			workspace := strings.TrimSuffix(a.cwd, "-review")
			// The task checkout is the sibling of the review checkout.
			if err := os.WriteFile(filepath.Join(workspace, "feature.go"), []byte("package feature\n\nfunc Corrected() bool { return true }\n"), 0600); err != nil {
				return codexapp.TurnOutcome{}, err
			}
			evidence := readyEvidence()
			evidence.Progress = "Corrected feature after review."
			evidence.SelfReview = "Checked corrected source and scope."
			body, _ := json.Marshal(evidence)
			args, _ := json.Marshal(map[string]string{"content": string(body)})
			response, err := handler(context.Background(), codexapp.ToolCall{Role: "reagent", ThreadID: "fresh-thread", TurnID: "correction-turn", CallID: "correction-checkpoint", Tool: "save_checkpoint", Arguments: args})
			if err != nil {
				return codexapp.TurnOutcome{}, err
			}
			if err := response.Commit(context.Background()); err != nil {
				return codexapp.TurnOutcome{}, err
			}
			return codexapp.TurnOutcome{ThreadID: "fresh-thread", TurnID: "correction-turn", Status: "completed", FinalText: `{"disposition":"ready_for_review","summary":"corrected"}`}, nil
		}
	} else {
		turn, verdict = "review-turn-2", "approved"
		if id != "reviewer-2" || !strings.Contains(prompt, "Fix feature.go") {
			return codexapp.TurnOutcome{}, errors.New("replacement reviewer lost correction evidence")
		}
	}
	body, _ := json.Marshal(map[string]string{"content": "Fix feature.go"})
	response, err := handler(context.Background(), codexapp.ToolCall{Role: "iragent", ThreadID: id, TurnID: turn, CallID: "assessment-" + turn, Tool: "save_assessment", Arguments: body})
	if err != nil {
		return codexapp.TurnOutcome{}, err
	}
	if err := response.Commit(context.Background()); err != nil {
		return codexapp.TurnOutcome{}, err
	}
	return codexapp.TurnOutcome{ThreadID: id, TurnID: turn, Status: "completed", FinalText: `{"disposition":"` + verdict + `","summary":"checked"}`}, nil
}

func TestLostReviewerCorrectionUsesSameAssessmentSequence(t *testing.T) {
	repo, external, policy := testRepo(t)
	base := readyAgent(t, repo, external, readyEvidence(), false, true)
	agent := &continuingAgent{fakeAgent: base}
	reviewer := &lostCorrectionReviewer{fakeReviewer: &fakeReviewer{}, reagent: base, quietBlocked: true}
	blocked, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent, Reviewer: reviewer})
	if err == nil || blocked.Status != "recovery_required" || blocked.Snapshot.Review.Number != 2 || blocked.Snapshot.Review.Verdict != "" || reviewer.starts != 1 {
		t.Fatalf("unquiet lost reviewer advanced: %+v, %v", blocked, err)
	}
	reviewer.quietBlocked = false
	result, err := Continue(context.Background(), Config{Repository: repo, Root: external, Reviewer: reviewer})
	if err != nil || result.Status != "approved" || result.Snapshot.Review.Number != 2 || result.Snapshot.Review.ThreadID != "reviewer-2" || len(result.Snapshot.Dispatch.Assessments) != 2 || reviewer.starts != 2 || reviewer.turns != 2 || reviewer.quietCalls == 0 {
		t.Fatalf("lost reviewer correction: %+v, %v", result, err)
	}
	attempts := result.Snapshot.Dispatch.Attempts
	if len(attempts) != 5 || attempts[4].Binding.Activity != dispatch.Recovery || attempts[4].Binding.Episode != attempts[3].ID || attempts[3].Status != "failed" {
		t.Fatalf("lost reviewer recovery reset dispatch history: %+v", attempts)
	}
}

func TestMissingTurnAcknowledgmentDoesNotReplay(t *testing.T) {
	repo, external, policy := testRepo(t)
	agent := &fakeAgent{run: func(string, codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		return codexapp.TurnOutcome{}, codexapp.ErrUncertainDispatch
	}}
	first, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent})
	if err == nil || first.Status != "recovery_required" {
		t.Fatalf("missing ack: %+v, %v", first, err)
	}
	result, err := Continue(context.Background(), Config{Repository: repo, Root: external, Agent: agent})
	if err != nil || result.Status != "recovery_required" || !strings.Contains(result.Snapshot.PendingRequest, "turn/start acknowledgment is uncertain") || agent.turns != 1 || len(result.Snapshot.Dispatch.Attempts) != 1 {
		t.Fatalf("uncertain dispatch replayed: %+v, %v", result, err)
	}
}

func (a *stoppedReviewer) ResumeThread(_ context.Context, id string, hadTurn bool, settings codexapp.RoleSettings) (codexapp.Thread, error) {
	if id != "reviewer-thread" || !hadTurn || settings.Role != "iragent" || settings.Effort != "high" {
		return codexapp.Thread{}, errors.New("reviewer resume settings differ")
	}
	a.resumes++
	return codexapp.Thread{ID: id}, nil
}
func (a *stoppedReviewer) StopTurn(_ context.Context, threadID, turnID string) stop.Result {
	if threadID != "reviewer-thread" || turnID != "review-turn" {
		panic("wrong reviewer turn stopped")
	}
	return stop.Result{Status: "stopped_safely", Turn: codexapp.ObservedTurn{ID: turnID, Status: "interrupted"}}
}
func (a *stoppedReviewer) RunTurn(_ context.Context, id, prompt string, settings codexapp.RoleSettings, handler codexapp.ToolHandler, onEvent func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	a.turns++
	if a.turns == 1 {
		body, _ := json.Marshal(map[string]string{"content": "Pending assessment before interruption."})
		response, err := handler(context.Background(), codexapp.ToolCall{Role: "iragent", ThreadID: id, TurnID: "review-turn", CallID: "assessment-before-stop", Tool: "save_assessment", Arguments: body})
		if err != nil {
			return codexapp.TurnOutcome{}, err
		}
		if err := response.Commit(context.Background()); err != nil {
			return codexapp.TurnOutcome{}, err
		}
		a.cancel()
		return codexapp.TurnOutcome{ThreadID: id, TurnID: "review-turn", Status: "interrupted"}, context.Canceled
	}
	if !strings.Contains(prompt, "Pending assessment before interruption") {
		panic("review evidence was lost")
	}
	body, _ := json.Marshal(map[string]string{"content": "Reassessed exact candidate; no findings."})
	response, err := handler(context.Background(), codexapp.ToolCall{Role: "iragent", ThreadID: id, TurnID: "review-turn-2", CallID: "assessment-after-stop", Tool: "save_assessment", Arguments: body})
	if err != nil {
		return codexapp.TurnOutcome{}, err
	}
	if err := response.Commit(context.Background()); err != nil {
		return codexapp.TurnOutcome{}, err
	}
	return codexapp.TurnOutcome{ThreadID: id, TurnID: "review-turn-2", Status: "completed", FinalText: `{"disposition":"approved","summary":"candidate approved"}`}, nil
}

func TestInterruptedAssessmentKeepsNumberAndConsumesOneVerdict(t *testing.T) {
	repo, external, policy := testRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	reviewer := &stoppedReviewer{fakeReviewer: &fakeReviewer{}, cancel: cancel}
	first, err := Execute(ctx, Config{Repository: repo, Root: external, Policy: policy, Agent: readyAgent(t, repo, external, readyEvidence(), false, true), Reviewer: reviewer})
	if err != nil || first.Status != "stopped_safely" || first.Snapshot.Review.Number != 1 || first.Snapshot.Review.Verdict != "" {
		t.Fatalf("interrupted review: %+v, %v", first, err)
	}
	result, err := Continue(context.Background(), Config{Repository: repo, Root: external, Reviewer: reviewer})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "approved" || result.Snapshot.Review.Number != 1 || result.Snapshot.Review.Verdict != "approved" || len(result.Snapshot.Dispatch.Assessments) != 1 || reviewer.starts != 1 || reviewer.resumes != 1 || reviewer.turns != 2 {
		t.Fatalf("review recovery: %+v", result)
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
	assessment, err := owner.ReviewEvidence(1)
	if err != nil || !strings.Contains(assessment, "Reviewer turn: review-turn-2") {
		t.Fatalf("review evidence: %q, %v", assessment, err)
	}
}

func (a *stoppedAgent) ResumeThread(_ context.Context, id string, hadTurn bool, settings codexapp.RoleSettings) (codexapp.Thread, error) {
	if id != "fresh-thread" || !hadTurn || settings.Role != "reagent" || settings.Effort != "medium" {
		return codexapp.Thread{}, errors.New("recorded resume settings differ")
	}
	a.resumes++
	return codexapp.Thread{ID: id}, nil
}
func (a *stoppedAgent) StopTurn(_ context.Context, threadID, turnID string) stop.Result {
	if threadID != "fresh-thread" || turnID != "interrupted-turn" {
		panic("stop did not use exact turn")
	}
	a.stops++
	return stop.Result{Status: "stopped_safely", Turn: codexapp.ObservedTurn{ID: turnID, Status: "interrupted"}}
}
func (a *stoppedAgent) RunTurn(ctx context.Context, id, prompt string, settings codexapp.RoleSettings, handler codexapp.ToolHandler, onEvent func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	a.turns++
	if a.turns == 1 {
		outcome, err := a.fakeAgent.RunTurn(ctx, id, prompt, settings, handler, onEvent)
		if err != nil {
			return outcome, err
		}
		a.cancel()
		return codexapp.TurnOutcome{ThreadID: id, TurnID: "interrupted-turn", Status: "interrupted"}, context.Canceled
	}
	if !strings.Contains(prompt, "APP-002") || !strings.Contains(prompt, "Reconcile any interrupted work") {
		panic("recovery prompt lost active task")
	}
	evidence, _ := json.Marshal(readyEvidence())
	args, _ := json.Marshal(map[string]string{"content": string(evidence)})
	response, err := handler(context.Background(), codexapp.ToolCall{Role: "reagent", ThreadID: id, TurnID: "resume-turn", CallID: "resume-checkpoint", Tool: "save_checkpoint", Arguments: args})
	if err != nil {
		return codexapp.TurnOutcome{}, err
	}
	if err := response.Commit(context.Background()); err != nil {
		return codexapp.TurnOutcome{}, err
	}
	return codexapp.TurnOutcome{ThreadID: id, TurnID: "resume-turn", Status: "completed", FinalText: `{"disposition":"ready_for_review","summary":"recovered and validated"}`}, nil
}

func TestSafeStopResumesBoundTaskWithoutReplay(t *testing.T) {
	repo, external, policy := testRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	base := readyAgent(t, repo, external, readyEvidence(), false, true)
	agent := &stoppedAgent{fakeAgent: base, cancel: cancel}
	first, err := Execute(ctx, Config{Repository: repo, Root: external, Policy: policy, Agent: agent})
	if err != nil || first.Status != "stopped_safely" || first.Snapshot.Stop == nil || agent.stops != 1 {
		t.Fatalf("initial stop: %+v, %v", first, err)
	}
	result, err := Continue(context.Background(), Config{Repository: repo, Root: external, Agent: agent})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ready_for_review" || result.Snapshot.Candidate == nil || result.Snapshot.Stop != nil || len(result.Snapshot.StopHistory) != 1 || agent.turns != 2 || agent.resumes != 1 || agent.starts != 1 {
		t.Fatalf("resume result: %+v; turns/resumes/starts %d/%d/%d", result, agent.turns, agent.resumes, agent.starts)
	}
	attempts := result.Snapshot.Dispatch.Attempts
	if len(attempts) != 2 || attempts[0].Status != "reconciled" || attempts[1].Status != "completed" || attempts[1].Binding.Activity != dispatch.Recovery || attempts[1].Binding.Episode != attempts[0].ID || attempts[1].ThreadID != "fresh-thread" || attempts[1].TurnID != "resume-turn" {
		t.Fatalf("recovery consumption: %+v", attempts)
	}
}
