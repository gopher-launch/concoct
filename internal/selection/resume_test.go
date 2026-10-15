package selection

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
	return codexapp.ObservedTurn{ID: turnID, Status: "completed", FinalText: `{"disposition":"approved","summary":"checked"}`}, nil
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
