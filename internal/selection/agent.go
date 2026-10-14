package selection

import (
	"context"
	"encoding/json"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

// Agent is one reagent thread and turn. Fake agents can exercise the complete
// host claim/checkpoint sequence without spending a provider turn.
type Agent interface {
	StartThread(context.Context, string, codexapp.RoleSettings, []codexapp.ToolSpec) (codexapp.Thread, error)
	RunTurn(context.Context, string, string, codexapp.RoleSettings, codexapp.ToolHandler, func(codexapp.Event)) (codexapp.TurnOutcome, error)
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

func toolSpecs() []codexapp.ToolSpec {
	return []codexapp.ToolSpec{
		{Type: "function", Name: "claim_task", Description: "Bind one eligible task after creating its branch from the baseline and before editing source. Continue engineering after acknowledgment.", InputSchema: json.RawMessage(`{"type":"object","properties":{"task_id":{"type":"string"},"task_branch":{"type":"string"},"selection_rationale":{"type":"string"},"objective":{"type":"string"},"acceptance":{"type":"string"}},"required":["task_id","task_branch","selection_rationale","objective","acceptance"],"additionalProperties":false}`)},
		{Type: "function", Name: "save_checkpoint", Description: "Save current engineering progress and next action after an acknowledged claim, without changing the agreed task scope.", InputSchema: json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`)},
	}
}
