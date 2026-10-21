package turnstop

import (
	"context"
	"errors"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/stop"
)

var (
	ErrElapsedLimit = errors.New("agent turn time limit exceeded")
	ErrStallLimit   = errors.New("agent inactivity time limit exceeded")
)

type Stopper interface {
	StopTurn(context.Context, string, string) stop.Result
}

// Context applies configured elapsed/stall limits and an optional explicit
// event trigger to the same cancellation path as foreground Ctrl-C.
func Context(parent context.Context, elapsed, stall time.Duration, trigger func(codexapp.Event) bool) (context.Context, context.CancelFunc, func(codexapp.Event)) {
	ctx, cancelCause := context.WithCancelCause(parent)
	cancel := func() { cancelCause(context.Canceled) }
	var elapsedTimer, stallTimer *time.Timer
	if elapsed > 0 {
		elapsedTimer = time.AfterFunc(elapsed, func() { cancelCause(ErrElapsedLimit) })
	}
	if stall > 0 {
		stallTimer = time.AfterFunc(stall, func() { cancelCause(ErrStallLimit) })
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
func Handle(owner *activework.Owner, agent any, role, threadID, turnID, attemptID string, reason ...error) (string, error) {
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
	if len(reason) != 0 && reason[0] != nil && (errors.Is(reason[0], ErrElapsedLimit) || errors.Is(reason[0], ErrStallLimit)) {
		if result.Problem == "" {
			result.Problem = reason[0].Error()
		} else {
			result.Problem = reason[0].Error() + "; " + result.Problem
		}
	}
	if err := owner.FinishStop(result); err != nil {
		return "stopped_abnormally", err
	}
	return result.Status, nil
}
