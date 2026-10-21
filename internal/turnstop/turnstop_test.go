package turnstop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

func TestConfiguredStopAndStallWatchdog(t *testing.T) {
	ctx, cancel, event := Context(context.Background(), 0, 80*time.Millisecond, func(value codexapp.Event) bool { return value.Kind == "quota-stop" })
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
	other, stop, _ := Context(context.Background(), 30*time.Millisecond, 0, nil)
	defer stop()
	select {
	case <-other.Done():
	case <-time.After(time.Second):
		t.Fatal("elapsed watchdog did not cancel turn")
	}
	if !errors.Is(context.Cause(other), ErrElapsedLimit) {
		t.Fatalf("elapsed limit reason = %v", context.Cause(other))
	}
}
