package turnstop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

func TestConfiguredStopAndStallWatchdog(t *testing.T) {
	ctx, cancel, event, _ := Context(context.Background(), 0, 80*time.Millisecond, func(value codexapp.Event) bool { return value.Kind == "quota-stop" })
	defer cancel()
	event(codexapp.Event{Kind: "progress"})
	select {
	case <-ctx.Done():
		t.Fatal("progress did not reset stall clock")
	case <-time.After(30 * time.Millisecond):
	}
	event(codexapp.Event{Kind: "quota-stop"})
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("configured stop did not cancel turn")
	}
	other, stop, _, finish := Context(context.Background(), 30*time.Millisecond, 0, nil)
	defer stop()
	select {
	case <-other.Done():
	case <-time.After(time.Second):
		t.Fatal("elapsed watchdog did not cancel turn")
	}
	if !errors.Is(context.Cause(other), ErrElapsedLimit) {
		t.Fatalf("elapsed limit reason = %v", context.Cause(other))
	}
	if !errors.Is(finish(), ErrElapsedLimit) {
		t.Fatal("completed execution did not retain elapsed limit")
	}
	stalled, cancelStalled, _, finishStalled := Context(context.Background(), 0, 30*time.Millisecond, nil)
	defer cancelStalled()
	select {
	case <-stalled.Done():
	case <-time.After(time.Second):
		t.Fatal("stall watchdog did not cancel active turn")
	}
	if !errors.Is(finishStalled(), ErrStallLimit) {
		t.Fatalf("stall limit reason = %v", context.Cause(stalled))
	}
}

func TestCompletedTurnDeadlineDoesNotExpireDuringPersistence(t *testing.T) {
	for _, tc := range []struct {
		name           string
		elapsed, stall time.Duration
	}{
		{name: "elapsed", elapsed: 40 * time.Millisecond},
		{name: "stall", stall: 40 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel, _, finish := Context(context.Background(), tc.elapsed, tc.stall, nil)
			defer cancel()
			if reason := finish(); reason != nil {
				t.Fatalf("completed turn was stopped: %v", reason)
			}
			// Dispatch persistence happens after RunTurn returns and can cross
			// the old deadline without changing the completed turn's outcome.
			time.Sleep(80 * time.Millisecond)
			if ctx.Err() != nil {
				t.Fatalf("watchdog remained armed during persistence: %v", context.Cause(ctx))
			}
		})
	}
}
