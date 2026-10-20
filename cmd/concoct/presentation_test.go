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
	stream.event(messageEvent("turn/started", "reviewer", "two", `{}`), role)
	stream.event(messageEvent("item/agentMessage/delta", "reviewer", "two", `{"itemId":"review-note","delta":"reviewing"}`), role)
	stream.finish()
	got := output.String()
	for _, want := range []string{"[reagent] first\nline\n\n[reagent] second", "[iragent] reviewing"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "ready_for_review") || stream.report == nil || stream.report.Summary != "done" || stream.report.Disposition != "ready-for-review" {
		t.Fatalf("structured final was printed or lost: %q %#v", got, stream.report)
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
