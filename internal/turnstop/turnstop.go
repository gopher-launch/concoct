package turnstop

import (
	"context"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/stop"
)

type Stopper interface {
	StopTurn(context.Context, string, string) stop.Result
}

// Context applies configured elapsed/stall limits and an optional explicit
// event trigger to the same cancellation path as foreground Ctrl-C.
func Context(parent context.Context, elapsed, stall time.Duration, trigger func(codexapp.Event) bool) (context.Context, context.CancelFunc, func(codexapp.Event)) {
	ctx, cancel := context.WithCancel(parent)
	var elapsedTimer, stallTimer *time.Timer
	if elapsed > 0 {
		elapsedTimer = time.AfterFunc(elapsed, cancel)
	}
	if stall > 0 {
		stallTimer = time.AfterFunc(stall, cancel)
	}
	stop := func() {
		if elapsedTimer != nil {
			elapsedTimer.Stop()
		}
		if stallTimer != nil {
			stallTimer.Stop()
		}
		cancel()
	}
	onEvent := func(event codexapp.Event) {
		if stallTimer != nil {
			stallTimer.Reset(stall)
		}
		if trigger != nil && trigger(event) {
			cancel()
		}
	}
	return ctx, stop, onEvent
}

// Handle runs after the dispatch is marked uncertain. A canceled turn is
// never assumed absent; the stop record is persisted before provider calls.
func Handle(owner *activework.Owner, agent any, role, threadID, turnID, attemptID string) (string, error) {
	if err := owner.BeginStop(role, threadID, turnID, attemptID); err != nil {
		return "stopped_abnormally", err
	}
	var result stop.Result
	if stopper, ok := agent.(Stopper); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		result = stopper.StopTurn(ctx, threadID, turnID)
	} else {
		result = stop.Result{Status: "stopped_abnormally", Problem: "owned provider stop controls are unavailable"}
	}
	if err := owner.FinishStop(result); err != nil {
		return "stopped_abnormally", err
	}
	return result.Status, nil
}
