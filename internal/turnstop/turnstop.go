package turnstop

import (
	"context"
	"errors"
	"sync"
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
func Context(parent context.Context, elapsed, stall time.Duration, trigger func(codexapp.Event) bool) (context.Context, context.CancelFunc, func(codexapp.Event), func() error) {
	ctx, cancelCause := context.WithCancelCause(parent)
	cancel := func() { cancelCause(context.Canceled) }
	var mu sync.Mutex
	var finished bool
	var timeoutAt time.Time
	var elapsedTimer, stallTimer *time.Timer
	started := time.Now()
	elapsedDeadline := started.Add(elapsed)
	stallDeadline := started.Add(stall)
	if elapsed > 0 {
		elapsedTimer = time.AfterFunc(elapsed, func() {
			mu.Lock()
			defer mu.Unlock()
			if !finished && !time.Now().Before(elapsedDeadline) {
				if ctx.Err() == nil {
					timeoutAt = time.Now()
				}
				cancelCause(ErrElapsedLimit)
			}
		})
	}
	if stall > 0 {
		stallTimer = time.AfterFunc(stall, func() {
			mu.Lock()
			defer mu.Unlock()
			if !finished && !time.Now().Before(stallDeadline) {
				if ctx.Err() == nil {
					timeoutAt = time.Now()
				}
				cancelCause(ErrStallLimit)
			}
		})
	}
	stop := func() {
		mu.Lock()
		if elapsedTimer != nil {
			elapsedTimer.Stop()
		}
		if stallTimer != nil {
			stallTimer.Stop()
		}
		cancel()
		mu.Unlock()
	}
	onEvent := func(event codexapp.Event) {
		mu.Lock()
		if stallTimer != nil && !finished {
			stallDeadline = time.Now().Add(stall)
			stallTimer.Reset(stall)
		}
		mu.Unlock()
		if trigger != nil && trigger(event) {
			cancel()
		}
	}
	// Finish is called immediately when RunTurn returns. Its timestamp freezes
	// the active execution window, even if a timer callback is queued.
	finish := func() error {
		returned := time.Now()
		mu.Lock()
		defer mu.Unlock()
		finished = true
		if elapsedTimer != nil {
			elapsedTimer.Stop()
		}
		if stallTimer != nil {
			stallTimer.Stop()
		}
		cause := context.Cause(ctx)
		if elapsed > 0 && !returned.Before(elapsedDeadline) {
			return ErrElapsedLimit
		}
		if stall > 0 && !returned.Before(stallDeadline) {
			return ErrStallLimit
		}
		if (errors.Is(cause, ErrElapsedLimit) || errors.Is(cause, ErrStallLimit)) && timeoutAt.After(returned) {
			return nil // A queued timer fired after Finish captured the return time.
		}
		return cause
	}
	return ctx, stop, onEvent, finish
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
