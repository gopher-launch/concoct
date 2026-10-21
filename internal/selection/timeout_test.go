package selection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/stop"
)

type failedLaunchAgent struct{ *fakeAgent }

func (a *failedLaunchAgent) StartThread(context.Context, string, codexapp.RoleSettings, []codexapp.ToolSpec) (codexapp.Thread, error) {
	return codexapp.Thread{}, errors.New("provider executable unavailable")
}

func TestProviderLaunchFailureKeepsActiveWorkAndExplainsFailure(t *testing.T) {
	repo, external, policy := testRepo(t)
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: &failedLaunchAgent{fakeAgent: &fakeAgent{}}})
	if err == nil || result.Status != "recovery_required" || result.Snapshot.Recovery == nil || !strings.Contains(result.Snapshot.PendingRequest, "provider executable unavailable") || result.Snapshot.Dispatch.Attempts[0].Status != "failed" {
		t.Fatalf("failed launch was not retained and explained: %+v, %v", result, err)
	}
}

type timedAgent struct {
	*fakeAgent
	stopped bool
}

func (a *timedAgent) RunTurn(ctx context.Context, threadID, _ string, _ codexapp.RoleSettings, _ codexapp.ToolHandler, onEvent func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	onEvent(codexapp.Event{Kind: "turn/started", ThreadID: threadID, TurnID: "timed-turn"})
	<-ctx.Done()
	return codexapp.TurnOutcome{ThreadID: threadID, TurnID: "timed-turn"}, ctx.Err()
}

func (a *timedAgent) StopTurn(context.Context, string, string) stop.Result {
	a.stopped = true
	return stop.Result{Status: "stopped_safely"}
}

func TestTurnTimeoutStopsExecutionAndReportsLimit(t *testing.T) {
	repo, external, policy := testRepo(t)
	policy.WatchdogElapsed = 10 * time.Millisecond
	agent := &timedAgent{fakeAgent: &fakeAgent{}}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent})
	if err != nil || result.Status != "stopped_safely" || !agent.stopped || result.Snapshot.Stop == nil || !strings.Contains(result.Snapshot.PendingRequest, "agent turn time limit exceeded") {
		t.Fatalf("timeout did not stop and explain execution: %+v, %v", result, err)
	}
}
