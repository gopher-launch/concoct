package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
)

type presentationOperations struct {
	events []codexapp.Event
	result outcome
}

func (p presentationOperations) Do(_ context.Context, _ request, onEvent func(codexapp.Event)) (outcome, error) {
	for _, event := range p.events {
		onEvent(event)
	}
	return p.result, nil
}

func messageEvent(kind, thread, turn, raw string) codexapp.Event {
	return codexapp.Event{Kind: kind, ThreadID: thread, TurnID: turn, Raw: json.RawMessage(raw)}
}

func TestMessageItemsStreamBeforeCompletionAndKeepControlSeparate(t *testing.T) {
	var output bytes.Buffer
	stream := newMessageStream(&output, false, false)
	role := func(thread string) string {
		if thread == "reviewer" {
			return "iragent"
		}
		return "reagent"
	}
	stream.event(messageEvent("turn/started", "writer", "one", `{}`), role)
	stream.event(messageEvent("item/started", "writer", "one", `{"item":{"id":"a","type":"agentMessage","phase":"commentary"}}`), role)
	stream.event(messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"a","delta":"first\nline"}`), role)
	if got := output.String(); !strings.Contains(got, "first\nline") {
		t.Fatalf("ordinary delta was buffered: %q", got)
	}
	stream.event(messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"a","delta":"\n"}`), role)
	stream.event(messageEvent("item/completed", "writer", "one", `{"item":{"id":"a","type":"agentMessage","phase":"commentary","text":"first\nline\n"}}`), role)
	stream.event(messageEvent("item/started", "writer", "one", `{"item":{"id":"empty","type":"agentMessage","phase":"commentary"}}`), role)
	stream.event(messageEvent("item/completed", "writer", "one", `{"item":{"id":"empty","type":"agentMessage","phase":"commentary","text":""}}`), role)
	stream.event(messageEvent("item/started", "writer", "one", `{"item":{"id":"b","type":"agentMessage","phase":"commentary"}}`), role)
	stream.event(messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"b","delta":"second"}`), role)
	stream.event(messageEvent("item/started", "writer", "one", `{"item":{"id":"final","type":"agentMessage","phase":"final"}}`), role)
	stream.event(messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"final","delta":"{\"disposition\":\"ready_"}`), role)
	stream.event(messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"final","delta":"for_review\",\"summary\":\"done\"}"}`), role)
	stream.event(messageEvent("item/completed", "writer", "one", `{"item":{"id":"final","type":"agentMessage","phase":"final","text":"{\"disposition\":\"ready_for_review\",\"summary\":\"done\"}"}}`), role)
	if stream.report == nil || stream.report.Summary != "done" || stream.report.Disposition != "ready-for-review" {
		t.Fatalf("structured final was lost: %#v", stream.report)
	}
	stream.event(messageEvent("turn/started", "reviewer", "two", `{}`), role)
	if stream.report != nil {
		t.Fatalf("prior role result was retained for new turn: %#v", stream.report)
	}
	stream.event(messageEvent("item/agentMessage/delta", "reviewer", "two", `{"itemId":"review-note","delta":"reviewing"}`), role)
	stream.finish()
	got := output.String()
	for _, want := range []string{"[reagent] first\nline\n\n[reagent] second", "[iragent] reviewing"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "ready_for_review") {
		t.Fatalf("structured final was printed: %q", got)
	}
}

func TestMalformedFinalAndJSONQuietPresentation(t *testing.T) {
	for _, mode := range []struct{ json, quiet bool }{{true, false}, {false, true}} {
		var output bytes.Buffer
		stream := newMessageStream(&output, mode.json, mode.quiet)
		role := func(string) string { return "reagent" }
		stream.event(messageEvent("turn/started", "writer", "one", `{}`), role)
		stream.event(messageEvent("item/started", "writer", "one", `{"item":{"id":"final","type":"agentMessage","phase":"final"}}`), role)
		stream.event(messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"final","delta":"{bad"}`), role)
		stream.event(messageEvent("item/completed", "writer", "one", `{"item":{"id":"final","type":"agentMessage","phase":"final","text":"{bad"}}`), role)
		stream.finish()
		if output.Len() != 0 || stream.report == nil || stream.report.Error == "" {
			t.Fatalf("mode %+v leaked control JSON or lost diagnostic: %q %#v", mode, output.String(), stream.report)
		}
	}
}

func TestNullPhaseFinalStaysOutOfProgressAndOrdinaryTextStreams(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		var output bytes.Buffer
		stream := newMessageStream(&output, jsonMode, false)
		role := func(string) string { return "reagent" }
		stream.event(messageEvent("turn/started", "writer", "one", `{}`), role)
		stream.event(messageEvent("item/started", "writer", "one", `{"item":{"id":"note","type":"agentMessage","phase":null}}`), role)
		stream.event(messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"note","delta":"ordinary "}`), role)
		if !strings.Contains(output.String(), "ordinary ") {
			t.Fatalf("null-phase ordinary text was delayed: %q", output.String())
		}
		stream.event(messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"note","delta":"message"}`), role)
		stream.event(messageEvent("item/completed", "writer", "one", `{"item":{"id":"note","type":"agentMessage","phase":null,"text":"ordinary message"}}`), role)
		stream.event(messageEvent("item/started", "writer", "one", `{"item":{"id":"control","type":"agentMessage","phase":null}}`), role)
		stream.event(messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"control","delta":"  {\"dispo"}`), role)
		stream.event(messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"control","delta":"sition\":\"ready_for_review\",\"summary\":\"done\"}"}`), role)
		if strings.Contains(output.String(), "dispo") || strings.Contains(output.String(), "ready_for_review") {
			t.Fatalf("null-phase control JSON leaked before completion: %q", output.String())
		}
		stream.event(messageEvent("item/completed", "writer", "one", `{"item":{"id":"control","type":"agentMessage","phase":null,"text":"  {\"disposition\":\"ready_for_review\",\"summary\":\"done\"}"}}`), role)
		if strings.Contains(output.String(), "disposition") || stream.report == nil || stream.report.Disposition != "ready-for-review" {
			t.Fatalf("null-phase final classification: %q %#v", output.String(), stream.report)
		}
		stream.event(messageEvent("item/started", "writer", "one", `{"item":{"id":"json-note","type":"agentMessage","phase":null}}`), role)
		stream.event(messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"json-note","delta":"{\"ordinary-json-key\":1}"}`), role)
		if strings.Contains(output.String(), "ordinary-json-key") {
			t.Fatalf("ambiguous JSON should await item completion: %q", output.String())
		}
		stream.event(messageEvent("item/completed", "writer", "one", `{"item":{"id":"json-note","type":"agentMessage","phase":null,"text":"{\"ordinary-json-key\":1}"}}`), role)
		if !strings.Contains(output.String(), "ordinary-json-key") {
			t.Fatalf("ordinary null-phase JSON was lost: %q", output.String())
		}
	}
}

func TestMalformedNullPhaseFinalsDoNotBecomeProgress(t *testing.T) {
	for _, tc := range []struct {
		name, first, last, completed string
	}{
		{"missing-disposition", `{"summary":`, `"done"}`, `{"summary":"done"}`},
		{"malformed-object", `{`, `bad`, `{bad`},
	} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "-human", true: "-json"}[jsonMode], func(t *testing.T) {
				events := []codexapp.Event{
					messageEvent("turn/started", "writer", "one", `{}`),
					messageEvent("item/started", "writer", "one", `{"item":{"id":"control","type":"agentMessage","phase":null}}`),
					messageEvent("item/agentMessage/delta", "writer", "one", string(mustJSON(t, map[string]string{"itemId": "control", "delta": tc.first}))),
					messageEvent("item/agentMessage/delta", "writer", "one", string(mustJSON(t, map[string]string{"itemId": "control", "delta": tc.last}))),
					messageEvent("item/completed", "writer", "one", string(mustJSON(t, map[string]any{"item": map[string]any{"id": "control", "type": "agentMessage", "phase": nil, "text": tc.completed}}))),
				}
				var out, errs bytes.Buffer
				args := []string{"run", "--root", t.TempDir()}
				if jsonMode {
					args = append(args, "--json")
				}
				if code := run(context.Background(), args, &out, &errs, presentationOperations{events: events, result: outcome{Disposition: "recovery_required", Summary: "verification incomplete"}}); code != 18 {
					t.Fatalf("exit %d: %s", code, errs.String())
				}
				got := out.String()
				if !strings.Contains(got, "malformed structured agent result") || strings.Contains(got, tc.completed) || strings.Contains(got, "agent-progress") {
					t.Fatalf("raw null-phase control result leaked: %s", got)
				}
				if jsonMode && !strings.Contains(got, `"agent-reported"`) || !jsonMode && !strings.Contains(got, "Agent result:") {
					t.Fatalf("missing human/JSON diagnostic: %s", got)
				}
			})
		}
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRunKeepsAgentReportSeparateInHumanJSONAndQuiet(t *testing.T) {
	events := []codexapp.Event{
		messageEvent("turn/started", "writer", "one", `{}`),
		messageEvent("item/started", "writer", "one", `{"item":{"id":"note","type":"agentMessage","phase":"commentary"}}`),
		messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"note","delta":"I finished"}`),
		messageEvent("item/started", "writer", "one", `{"item":{"id":"final","type":"agentMessage","phase":"final"}}`),
		messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"final","delta":"{\"disposition\":\"ready_for_review\""}`),
		messageEvent("item/agentMessage/delta", "writer", "one", `{"itemId":"final","delta":",\"summary\":\"ready\"}"}`),
		messageEvent("item/completed", "writer", "one", `{"item":{"id":"final","type":"agentMessage","phase":"final","text":"{\"disposition\":\"ready_for_review\",\"summary\":\"ready\"}"}}`),
	}
	operations := presentationOperations{events: events, result: outcome{Disposition: "recovery_required", Summary: "callback evidence missing"}}
	for _, tc := range []struct {
		flags        []string
		want, absent string
	}{
		{nil, "Agent reported (agent, ready-for-review): ready", `{"disposition"`},
		{[]string{"--json"}, `"agent-reported":{"disposition":"ready-for-review"`, `ready_for_review`},
		{[]string{"--quiet"}, "Agent reported (agent, ready-for-review): ready", "I finished"},
	} {
		var out, errs bytes.Buffer
		if code := run(context.Background(), append([]string{"run"}, tc.flags...), &out, &errs, operations); code != 18 {
			t.Fatalf("%v: exit %d: %s", tc.flags, code, errs.String())
		}
		if !strings.Contains(out.String(), tc.want) || strings.Contains(out.String(), tc.absent) || !strings.Contains(out.String(), "callback evidence missing") {
			t.Fatalf("%v: %s", tc.flags, out.String())
		}
		if len(tc.flags) == 0 && !strings.Contains(out.String(), "Concoct verified: recovery-required") {
			t.Fatalf("controller result missing: %s", out.String())
		}
	}
}

func TestPublicJSONNamesAndTokenReceiptDistinction(t *testing.T) {
	snapshot := activework.Snapshot{Active: true, TaskID: "APP-002", Phase: "ready_for_review", Dispatch: &dispatch.State{}}
	without := view(snapshot, request{})
	if without.ObservedTokens != nil {
		t.Fatal("missing receipt rendered as zero")
	}
	snapshot.Dispatch.Attempts = append(snapshot.Dispatch.Attempts, dispatch.Attempt{Tokens: &dispatch.TokenReceipt{}})
	with := view(snapshot, request{})
	if with.ObservedTokens == nil || *with.ObservedTokens != 0 {
		t.Fatal("measured zero receipt was lost")
	}
	var output bytes.Buffer
	writePublicJSON(&output, map[string]any{"status": with, "disposition": publicName("ready_for_acceptance"), "approval": map[string]any{"DiffCommand": "git diff"}, "large_count": int64(9007199254740993), "quota_windows": map[string]any{"provider_Window": map[string]any{"ResetAt": "later"}}})
	for _, want := range []string{`"observed-tokens":0`, `"phase":"ready-for-review"`, `"disposition":"ready-for-acceptance"`, `"diff-command":"git diff"`, `"large-count":9007199254740993`, `"provider_Window":{"reset-at":"later"}`} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing %q: %s", want, output.String())
		}
	}
	if strings.Contains(output.String(), "observed_tokens") || strings.Contains(output.String(), "DiffCommand") {
		t.Fatalf("legacy public names remain: %s", output.String())
	}
}

func TestReviewTurnVisibilityUsesCurrentCandidateDispatchEvidence(t *testing.T) {
	bound := dispatch.Binding{TaskID: "APP-002", Role: dispatch.Iragent, Activity: dispatch.Review, Candidate: "candidate-new"}
	snapshot := activework.Snapshot{
		Active: true, TaskID: "APP-002", Candidate: &activework.Candidate{Commit: "candidate-new"},
		Review:   &activework.PendingReview{Candidate: "candidate-new", AttemptID: "review-2"},
		Recovery: &activework.RecoveryIssue{Reason: "review turn incomplete"},
		Dispatch: &dispatch.State{Attempts: []dispatch.Attempt{{ID: "review-2", Binding: bound, TurnID: "observed-turn"}}},
	}
	if !view(snapshot, request{}).ReviewTurnSeen {
		t.Fatal("pending review's observed dispatch turn was hidden before finalization")
	}
	var out bytes.Buffer
	printStatus(&out, view(snapshot, request{}))
	if !strings.Contains(out.String(), "independent review turn observed: true") {
		t.Fatalf("recovery view hid observed review turn: %s", out.String())
	}
	out.Reset()
	writePublicJSON(&out, view(snapshot, request{}))
	if !strings.Contains(out.String(), `"review-turn-observed":true`) {
		t.Fatalf("JSON recovery view hid observed review turn: %s", out.String())
	}
	// A renewed attempt does not erase the observed turn on the same candidate.
	snapshot.Review.AttemptID = "review-retry"
	snapshot.Dispatch.Attempts = append(snapshot.Dispatch.Attempts, dispatch.Attempt{ID: "review-retry", Binding: dispatch.Binding{TaskID: "APP-002", Role: dispatch.Iragent, Activity: dispatch.Recovery, Candidate: "candidate-new"}})
	if !view(snapshot, request{}).ReviewTurnSeen {
		t.Fatal("same-candidate review history was hidden by a retry reservation")
	}
	// Correction freezes a distinct candidate. Its review has not begun.
	snapshot.Candidate.Commit = "candidate-corrected"
	snapshot.Review = &activework.PendingReview{Candidate: "candidate-corrected", AttemptID: "review-3"}
	if view(snapshot, request{}).ReviewTurnSeen {
		t.Fatal("prior candidate's review was attributed to corrected candidate")
	}
	out.Reset()
	writePublicJSON(&out, view(snapshot, request{}))
	if !strings.Contains(out.String(), `"review-turn-observed":false`) {
		t.Fatalf("JSON view attributed prior review to corrected candidate: %s", out.String())
	}
	snapshot.Dispatch.Attempts = append(snapshot.Dispatch.Attempts, dispatch.Attempt{ID: "review-3", Binding: dispatch.Binding{TaskID: "APP-002", Role: dispatch.Iragent, Activity: dispatch.Review, Candidate: "candidate-corrected"}, TurnID: "corrected-turn"})
	if !view(snapshot, request{}).ReviewTurnSeen {
		t.Fatal("corrected candidate's observed review turn was hidden")
	}
}

func TestSelectionRecoveryAndLocalTimestampViews(t *testing.T) {
	selection := view(activework.Snapshot{Active: true, Phase: "selecting", ObservedAt: time.Date(2024, 11, 3, 8, 30, 0, 0, time.UTC)}, request{})
	var out bytes.Buffer
	t.Setenv("TZ", "America/Vancouver")
	printStatus(&out, selection)
	if !strings.Contains(out.String(), "Selecting a task") || !strings.Contains(out.String(), "2024-11-03 01:30:00 PDT -07:00") || strings.Contains(out.String(), "Task ;") {
		t.Fatalf("selection view: %s", out.String())
	}
	if got := localTime(time.Date(2024, 11, 3, 9, 30, 0, 0, time.UTC)); got != "2024-11-03 01:30:00 PST -08:00" {
		t.Fatalf("DST boundary: %s", got)
	}
	t.Setenv("TZ", "UTC")
	if got := localTime(time.Date(2024, 11, 3, 9, 30, 0, 0, time.UTC)); got != "2024-11-03 09:30:00 UTC +00:00" {
		t.Fatalf("UTC choice: %s", got)
	}
	out.Reset()
	recovery := view(activework.Snapshot{Active: true, TaskID: "APP-002", Hold: "recovery-required", Recovery: &activework.RecoveryIssue{Reason: "receipt missing", AgentResult: "ready for review"}}, request{})
	printStatus(&out, recovery)
	for _, want := range []string{"Agent reported: ready for review", "Controller verification: incomplete", "independent review turn observed: false", "Recovery: receipt missing", "missing proof keeps this hold"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("recovery missing %q: %s", want, out.String())
		}
	}
}
