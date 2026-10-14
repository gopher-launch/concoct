package selection

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

// Agent is one reagent thread and turn. Fake agents can exercise the complete
// host claim/checkpoint sequence without spending a provider turn.
type Agent interface {
	StartThread(context.Context, string, codexapp.RoleSettings, []codexapp.ToolSpec) (codexapp.Thread, error)
	RunTurn(context.Context, string, string, codexapp.RoleSettings, codexapp.ToolHandler, func(codexapp.Event)) (codexapp.TurnOutcome, error)
	Idle(context.Context, string) error
}

// CodexAgent adapts the verified app-server client to the selection controller.
type CodexAgent struct{ Client *codexapp.Client }

func (a CodexAgent) StartThread(ctx context.Context, cwd string, settings codexapp.RoleSettings, tools []codexapp.ToolSpec) (codexapp.Thread, error) {
	return a.Client.StartThread(ctx, cwd, settings, tools)
}

func (a CodexAgent) RunTurn(ctx context.Context, threadID, prompt string, settings codexapp.RoleSettings, handler codexapp.ToolHandler, onEvent func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	a.Client.SetToolHandler(handler)
	return a.Client.RunTurn(ctx, threadID, prompt, settings, json.RawMessage(nil), onEvent)
}

// Idle requires both the completed turn's resolved callbacks and an empty
// provider background-terminal list before source can be frozen.
func (a CodexAgent) Idle(ctx context.Context, threadID string) error {
	terminals, err := a.Client.BackgroundTerminals(ctx, threadID)
	if err != nil {
		return err
	}
	if len(terminals) != 0 {
		return fmt.Errorf("reagent still has %d background terminals", len(terminals))
	}
	return nil
}

func toolSpecs() []codexapp.ToolSpec {
	return []codexapp.ToolSpec{
		{Type: "function", Name: "claim_task", Description: "Bind one eligible task after creating its branch from the baseline and before editing source. Continue engineering after acknowledgment.", InputSchema: json.RawMessage(`{"type":"object","properties":{"task_id":{"type":"string"},"task_branch":{"type":"string"},"selection_rationale":{"type":"string"},"objective":{"type":"string"},"acceptance":{"type":"string"}},"required":["task_id","task_branch","selection_rationale","objective","acceptance"],"additionalProperties":false}`)},
		{Type: "function", Name: "save_checkpoint", Description: "Save current engineering progress and next action after an acknowledged claim, without changing the agreed task scope.", InputSchema: json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`)},
	}
}
