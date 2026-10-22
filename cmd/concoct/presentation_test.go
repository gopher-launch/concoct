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
	if strings.Contains(text, "\"disposition\"") || !strings.Contains(text, "[iragent result] approved: Checked candidate") {
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

func TestFragmentedCommentaryAndCodeExample(t *testing.T) {
	var out bytes.Buffer
	s := newMessageStream(&out, false, false)
	role := func(string) string { return "reagent" }
	emit := func(kind, item, phase, value string) {
		var body any
		if kind == "item/agentMessage/delta" {
			body = map[string]any{"itemId": item, "delta": value}
		} else {
			body = map[string]any{"item": map[string]any{"id": item, "type": "agentMessage", "phase": phase, "text": value}}
		}
		raw, _ := json.Marshal(body)
		s.event(codexapp.Event{Kind: kind, ThreadID: "writer", TurnID: "one", Raw: raw}, role)
	}
	envelope := `{"disposition":"decision_required","summary":"No response is needed.","question":{"text":"placeholder"},"task_id":null,"candidate":null,"findings":null,"evidence":null}`
	emit("item/started", "structured", "commentary", "")
	emit("item/agentMessage/delta", "structured", "", envelope[:26])
	emit("item/agentMessage/delta", "structured", "", envelope[26:])
	if out.Len() != 0 {
		t.Fatalf("structured fragment leaked before completion: %q", out.String())
	}
	emit("item/completed", "structured", "commentary", envelope)
	emit("item/agentMessage/delta", "plain", "", "Running checks ")
	emit("item/agentMessage/delta", "plain", "", "now.")
	emit("item/completed", "plain", "", "Running checks now.")
	code := "```json\n{\"disposition\":\"example\"}\n```"
	emit("item/completed", "code", "commentary", code)
	emit("item/completed", "object", "", `{"example":true}`)
	s.finish()
	text := out.String()
	for _, want := range []string{"[reagent] No response is needed.", "[reagent] Running checks now.", code, `{"example":true}`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	for _, leaked := range []string{`"question"`, `"task_id"`, `"decision_required"`, "placeholder"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("intermediate control field leaked: %q", text)
		}
	}
}

func TestAbsentPhaseIntermediateDoesNotBecomeResultPresentation(t *testing.T) {
	var out bytes.Buffer
	s := newMessageStream(&out, false, false)
	role := func(string) string { return "iragent" }
	intermediate := `{"disposition":"approved","summary":"Still checking files","task_id":null,"candidate":null,"findings":null,"evidence":null,"question":null}`
	for _, item := range []struct{ phase, text string }{{"", intermediate}, {"final_answer", `{"disposition":"changes_requested","summary":"Found a defect","task_id":null,"candidate":null,"findings":null,"evidence":null,"question":null}`}} {
		raw, _ := json.Marshal(map[string]any{"item": map[string]any{"id": item.phase + item.text[:1], "type": "agentMessage", "phase": item.phase, "text": item.text}})
		s.event(codexapp.Event{Kind: "item/completed", ThreadID: "review", TurnID: "one", Raw: raw}, role)
	}
	s.finish()
	text := out.String()
	if !strings.Contains(text, "Still checking files") || !strings.Contains(text, "[iragent result] changes-requested: Found a defect") || strings.Contains(text, "[iragent result] approved") {
		t.Fatalf("intermediate disposition was presented as final: %q", text)
	}
}

func TestFailedProviderTurnDoesNotPresentProvisionalApproval(t *testing.T) {
	var out bytes.Buffer
	s := newMessageStream(&out, false, false)
	role := func(string) string { return "iragent" }
	final := []byte(`{"item":{"id":"final","type":"agentMessage","phase":"final_answer","text":"{\"disposition\":\"approved\",\"summary\":\"Provisional\"}"}}`)
	s.event(codexapp.Event{Kind: "item/completed", ThreadID: "review", TurnID: "one", Raw: final}, role)
	s.event(codexapp.Event{Kind: "turn/completed", ThreadID: "review", TurnID: "one", Raw: []byte(`{"turn":{"status":"failed"}}`)}, role)
	s.finish()
	if strings.Contains(out.String(), "approved") || strings.Contains(out.String(), "Provisional") {
		t.Fatalf("failed turn presented approval: %q", out.String())
	}
}

func TestExplicitJSONPresentationKeepsProgressAndResult(t *testing.T) {
	var out bytes.Buffer
	s := newMessageStream(&out, true, false)
	role := func(string) string { return "iragent" }
	for _, item := range []struct{ id, phase, text string }{
		{"comment", "commentary", `{"disposition":"decision_required","summary":"Checking evidence","question":null}`},
		{"final", "final_answer", `{"disposition":"approved","summary":"Checked evidence"}`},
	} {
		raw, _ := json.Marshal(map[string]any{"item": map[string]any{"id": item.id, "type": "agentMessage", "phase": item.phase, "text": item.text}})
		s.event(codexapp.Event{Kind: "item/completed", ThreadID: "review", TurnID: "one", Raw: raw}, role)
	}
	s.event(codexapp.Event{Kind: "turn/completed", ThreadID: "review", TurnID: "one", Raw: []byte(`{"turn":{"status":"completed"}}`)}, role)
	s.finish()
	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected progress and result JSON lines: %s", out.String())
	}
	var progress, result map[string]any
	if json.Unmarshal(lines[0], &progress) != nil || json.Unmarshal(lines[1], &result) != nil || progress["type"] != "agent-progress" || progress["text"] != "Checking evidence" || result["type"] != "agent-result" || result["summary"] != "Checked evidence" {
		t.Fatalf("JSON presentation lost source separation: %s", out.String())
	}
}
