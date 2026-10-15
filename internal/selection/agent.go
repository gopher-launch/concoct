package selection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/stop"
)

// Agent is one reagent thread and turn. Fake agents can exercise the complete
// host claim/checkpoint sequence without spending a provider turn.
type Agent interface {
	StartThread(context.Context, string, codexapp.RoleSettings, []codexapp.ToolSpec) (codexapp.Thread, error)
	RunTurn(context.Context, string, string, codexapp.RoleSettings, codexapp.ToolHandler, func(codexapp.Event)) (codexapp.TurnOutcome, error)
	Idle(context.Context, string) error
}

// CodexAgent adapts the verified app-server client to the selection controller.
type CodexAgent struct {
	Client *codexapp.Client
	Owned  *stop.Probe // exact descendants of a dedicated Concoct app-server
}

func NewCodexAgent(process *codexapp.Process) (CodexAgent, error) {
	if process == nil || process.PID() == 0 {
		return CodexAgent{}, fmt.Errorf("dedicated provider process is required")
	}
	probe, err := stop.NewProbe(process.PID())
	if err != nil {
		return CodexAgent{}, err
	}
	return CodexAgent{Client: process.Client, Owned: probe}, nil
}

func (a CodexAgent) ResumeThread(ctx context.Context, id string, hadTurn bool, settings codexapp.RoleSettings) (codexapp.Thread, error) {
	return a.Client.ResumeThread(ctx, id, hadTurn, settings)
}

func (a CodexAgent) ReadTurn(ctx context.Context, threadID, turnID string) (codexapp.ObservedTurn, error) {
	return a.Client.ReadTurn(ctx, threadID, turnID)
}

func (a CodexAgent) StartThread(ctx context.Context, cwd string, settings codexapp.RoleSettings, tools []codexapp.ToolSpec) (codexapp.Thread, error) {
	return a.Client.StartThread(ctx, cwd, settings, tools)
}

func (a CodexAgent) RunTurn(ctx context.Context, threadID, prompt string, settings codexapp.RoleSettings, handler codexapp.ToolHandler, onEvent func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	if a.Client == nil || a.Owned == nil {
		return codexapp.TurnOutcome{}, fmt.Errorf("dedicated provider and owned-process probe are required")
	}
	a.Client.SetToolHandler(handler)
	if _, err := a.Owned.Capture(); err != nil {
		return codexapp.TurnOutcome{}, err
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, _ = a.Owned.Capture()
			}
		}
	}()
	outcome, err := a.Client.RunTurn(ctx, threadID, prompt, settings, json.RawMessage(nil), onEvent)
	close(done)
	_, _ = a.Owned.Capture()
	return outcome, err
}

func (a CodexAgent) StopTurn(ctx context.Context, threadID, turnID string) stop.Result {
	return stop.Run(ctx, a.Client, a.Owned, threadID, turnID, 2*time.Second)
}

func (a CodexAgent) OwnedQuiet(ctx context.Context) error {
	if a.Owned == nil {
		return fmt.Errorf("owned-process probe is unavailable")
	}
	quietCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return a.Owned.Quiet(quietCtx, 2*time.Second)
}

// Idle requires both the completed turn's resolved callbacks and an empty
// provider background-terminal list before source can be frozen.
func (a CodexAgent) Idle(ctx context.Context, threadID string) error {
	if a.Client == nil || a.Owned == nil {
		return fmt.Errorf("dedicated provider and owned-process probe are required")
	}
	terminals, err := a.Client.BackgroundTerminals(ctx, threadID)
	if err != nil {
		return err
	}
	if len(terminals) != 0 {
		return fmt.Errorf("role thread still has %d background terminals", len(terminals))
	}
	return a.OwnedQuiet(ctx)
}

func toolSpecs() []codexapp.ToolSpec {
	return []codexapp.ToolSpec{
		{Type: "function", Name: "claim_task", Description: "Bind one eligible task after creating its branch from the baseline and before editing source. Continue engineering after acknowledgment.", InputSchema: json.RawMessage(`{"type":"object","properties":{"task_id":{"type":"string"},"task_branch":{"type":"string"},"selection_rationale":{"type":"string"},"objective":{"type":"string"},"acceptance":{"type":"string"}},"required":["task_id","task_branch","selection_rationale","objective","acceptance"],"additionalProperties":false}`)},
		{Type: "function", Name: "save_checkpoint", Description: "Save current engineering progress and next action after an acknowledged claim, without changing the agreed task scope.", InputSchema: json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`)},
	}
}
