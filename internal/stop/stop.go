package stop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

type Control interface {
	Interrupt(context.Context, string, string) error
	ReadTurn(context.Context, string, string) (codexapp.ObservedTurn, error)
	BackgroundTerminals(context.Context, string) ([]codexapp.BackgroundTerminal, error)
	TerminateBackgroundTerminal(context.Context, string, string) (bool, error)
	CleanBackgroundTerminals(context.Context, string) error
}

type Ownership interface {
	Capture() ([]Identity, error)
	Quiet(context.Context, time.Duration) error
}

type Result struct {
	Status  string // stopped_safely or stopped_abnormally
	Turn    codexapp.ObservedTurn
	Owned   []Identity
	Handles []string
	Problem string
}

// Run requests provider interruption and cleanup, then verifies the recorded
// host process identities have ceased. The provider handle is never a PID.
// Failed controls still lead to best-effort cleanup but never a safe verdict.
func Run(ctx context.Context, control Control, ownership Ownership, threadID, turnID string, grace time.Duration) Result {
	result := Result{Status: "stopped_abnormally"}
	var problems []string
	if control == nil || ownership == nil || threadID == "" {
		result.Problem = "owned provider control, process probe and thread ID are required"
		return result
	}
	owned, err := ownership.Capture()
	if err != nil {
		problems = append(problems, "capture owned processes: "+err.Error())
	} else {
		result.Owned = owned
	}
	if turnID == "" {
		problems = append(problems, "turn identity is unknown")
	} else {
		if err := control.Interrupt(ctx, threadID, turnID); err != nil {
			problems = append(problems, "interrupt: "+err.Error())
		}
		// The terminal receipt may arrive after interrupt acknowledgment.
		deadline := time.NewTimer(5 * time.Second)
		ticker := time.NewTicker(50 * time.Millisecond)
		terminal := false
		for !terminal {
			observed, err := control.ReadTurn(ctx, threadID, turnID)
			if err == nil && (observed.Status == "completed" || observed.Status == "interrupted" || observed.Status == "failed") {
				result.Turn, terminal = observed, true
				break
			}
			select {
			case <-ctx.Done():
				terminal = true
				problems = append(problems, "terminal observation: "+ctx.Err().Error())
			case <-deadline.C:
				terminal = true
				problems = append(problems, "terminal observation timed out or remained uncertain")
			case <-ticker.C:
			}
		}
		deadline.Stop()
		ticker.Stop()
	}
	terminals, err := control.BackgroundTerminals(ctx, threadID)
	if err != nil {
		problems = append(problems, "list terminals: "+err.Error())
	} else {
		if latest, err := ownership.Capture(); err == nil {
			result.Owned = latest
		} else {
			problems = append(problems, "capture after interrupt: "+err.Error())
		}
		for _, terminal := range terminals {
			result.Handles = append(result.Handles, terminal.ProcessID)
			terminated, err := control.TerminateBackgroundTerminal(ctx, threadID, terminal.ProcessID)
			if err != nil || !terminated {
				problems = append(problems, fmt.Sprintf("terminate provider handle %s: acknowledged=%t error=%v", terminal.ProcessID, terminated, err))
			}
		}
	}
	if err := control.CleanBackgroundTerminals(ctx, threadID); err != nil {
		problems = append(problems, "clean terminals: "+err.Error())
	}
	remaining, err := control.BackgroundTerminals(ctx, threadID)
	if err != nil {
		problems = append(problems, "relist terminals: "+err.Error())
	} else if len(remaining) != 0 {
		problems = append(problems, fmt.Sprintf("%d background terminals remain", len(remaining)))
	}
	if err := ownership.Quiet(ctx, grace); err != nil {
		problems = append(problems, "owned process cessation: "+err.Error())
	}
	if len(problems) == 0 {
		result.Status = "stopped_safely"
	} else {
		result.Problem = strings.Join(problems, "; ")
	}
	return result
}
