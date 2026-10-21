package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

func TestRoleLabeledProseAndStructuredSummary(t *testing.T) {
	var out bytes.Buffer
	s := newMessageStream(&out, false, false)
	role := func(id string) string {
		if id == "review" {
			return "iragent"
		}
		return "reagent"
	}
	emit := func(kind, id, turn string, body any) {
		raw, _ := json.Marshal(body)
		s.event(codexapp.Event{Kind: kind, ThreadID: id, TurnID: turn, Raw: raw}, role)
	}
	emit("turn/started", "writer", "one", map[string]any{})
	emit("item/agentMessage/delta", "writer", "one", map[string]any{"itemId": "a", "delta": "Selected task."})
	emit("item/completed", "writer", "one", map[string]any{"item": map[string]any{"id": "a", "type": "agentMessage", "text": "Selected task."}})
	emit("item/agentMessage/delta", "writer", "one", map[string]any{"itemId": "b", "delta": "Committed candidate."})
	emit("item/completed", "writer", "one", map[string]any{"item": map[string]any{"id": "b", "type": "agentMessage", "text": "Committed candidate."}})
	emit("turn/started", "review", "two", map[string]any{})
	emit("item/agentMessage/delta", "review", "two", map[string]any{"itemId": "c", "delta": "Reviewing now."})
	emit("item/completed", "review", "two", map[string]any{"item": map[string]any{"id": "c", "type": "agentMessage", "text": "Reviewing now."}})
	emit("item/completed", "review", "two", map[string]any{"item": map[string]any{"id": "final", "type": "agentMessage", "phase": "final_answer", "text": "{\"disposition\":\"approved\",\"summary\":\"Checked candidate\",\"task_id\":null,\"candidate\":null,\"findings\":null,\"evidence\":null,\"question\":null}"}})
	s.finish()
	text := out.String()
	for _, want := range []string{"[reagent] Selected task.\n\n[reagent] Committed candidate.", "[iragent] Reviewing now."} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if strings.Contains(text, "\"disposition\"") || s.report == nil || s.report.Summary != "Checked candidate" {
		t.Fatalf("structured result leaked or lost: %q %+v", text, s.report)
	}
}

func TestCommentaryEnvelopeRendersSummary(t *testing.T) {
	var out bytes.Buffer
	s := newMessageStream(&out, false, false)
	role := func(string) string { return "reagent" }
	raw := []byte(`{"item":{"id":"summary","type":"agentMessage","phase":null,"text":"{\"summary\":\"Checking the build\"}"}}`)
	s.event(codexapp.Event{Kind: "item/completed", ThreadID: "writer", TurnID: "one", Raw: raw}, role)
	s.finish()
	if !strings.Contains(out.String(), "Checking the build") || strings.Contains(out.String(), "\"summary\"") {
		t.Fatalf("envelope output: %q", out.String())
	}
}
