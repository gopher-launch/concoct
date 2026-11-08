package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/handoff"
)

func TestApprovalAndAnswerOptions(t *testing.T) {
	for _, tc := range []struct {
		args                     []string
		wantApproval, wantAnswer string
		wantReviewLimit          int
	}{{[]string{"run", "--approve", "abc"}, "abc", "", 0}, {[]string{"run", "--answer", "Use safe mode"}, "", "Use safe mode", 0}, {[]string{"run", "--review-limit", "2"}, "", "", 2}} {
		o, command, _, _, err := parse(tc.args)
		if err != nil || command != "run" || o.Approve != tc.wantApproval || o.Answer != tc.wantAnswer || o.MaxAssessments != tc.wantReviewLimit {
			t.Fatalf("parse %v: %+v %v", tc.args, o, err)
		}
	}
	for _, args := range [][]string{{"run", "--review-limit", "2x"}, {"status", "--approve", "abc"}, {"status", "--answer", "yes"}} {
		if _, _, _, _, err := parse(args); err == nil {
			t.Fatalf("accepted invalid args %v", args)
		}
	}
	o, _, _, _, err := parse([]string{"run", "--approve-review-limit-increase", "5", "--approve-invocation-limit-increase", "14"})
	if err != nil || o.ApproveReviewLimitIncrease != 5 || o.ApproveInvocationLimitIncrease != 14 {
		t.Fatalf("parse limit increases: %+v %v", o, err)
	}
	o, _, _, _, err = parse([]string{"run", "--roadmap", "deploy/roadmap.md"})
	if err != nil || o.Roadmap != "deploy/roadmap.md" {
		t.Fatalf("parse roadmap: %+v %v", o, err)
	}
	if _, _, _, _, err := parse([]string{"status", "--roadmap", "roadmap.md"}); err == nil {
		t.Fatal("status accepted run-only roadmap selection")
	}
	if _, _, _, _, err := parse([]string{"run", "--roadmap", ""}); err == nil {
		t.Fatal("accepted empty roadmap path")
	}
	o, command, _, _, err := parse([]string{"run", "--recover", "--iragent-model", "replacement"})
	if err != nil || command != "run" || !o.Recover || o.IragentModel != "replacement" {
		t.Fatalf("parse recovery: %+v %s %v", o, command, err)
	}
	if _, _, _, _, err := parse([]string{"status", "--recover"}); err == nil {
		t.Fatal("status accepted recovery")
	}
	for _, args := range [][]string{{"run", "--approve-review-limit-increase", "0"}, {"run", "--approve-invocation-limit-increase", "bad"}, {"status", "--approve-review-limit-increase", "5"}} {
		if _, _, _, _, err := parse(args); err == nil {
			t.Fatalf("accepted invalid increase args %v", args)
		}
	}
}

func TestRecoveryPresentationIncludesContextualCommand(t *testing.T) {
	var out bytes.Buffer
	s := handoff.State{
		Phase: "execution_failed", Repository: "/tmp/repo with space", Roadmap: "/tmp/repo with space/roadmap.md",
		PendingStep: "reviewing", Diagnostics: "/tmp/provider.log", Attempts: 2, MaxAttempts: 12, Assessments: 0, MaxAssessments: 3,
		InFlight: &handoff.InFlight{Role: "iragent", ThreadID: "failed-thread", Step: "reviewing"},
	}
	displayWithContext(&out, s, false, commandContext{executable: "/tmp/concoct tool", options: handoff.Options{Root: "/tmp/root;literal"}})
	for _, want := range []string{"Failed role: iragent", "Pending step: reviewing", "Provider diagnostics: /tmp/provider.log", "Invocations: 2/12", "complete recorded process identity is live or cannot be inspected", "Missing or incomplete legacy process identities are treated as missing diagnostics", "'/tmp/concoct tool' run --repo '/tmp/repo with space' --root '/tmp/root;literal' --roadmap '/tmp/repo with space/roadmap.md' --recover"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "prior owned processes remain uncertain") {
		t.Fatalf("legacy diagnostic gaps were presented as a recovery refusal: %s", out.String())
	}
	out.Reset()
	s.Phase = "recovering"
	s.InFlight = nil
	s.ActiveRecovery = &handoff.Recovery{Role: "iragent", Step: "reviewing", Diagnostic: "/tmp/prior-provider.log"}
	displayWithContext(&out, s, false, commandContext{executable: "/tmp/concoct tool", options: handoff.Options{Root: "/tmp/root;literal"}})
	for _, want := range []string{"Concoct: recovering", "Failed role: iragent", "Pending step: reviewing", "Prior provider diagnostics: /tmp/prior-provider.log", "--recover"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("recovering status missing %q in %s", want, out.String())
		}
	}
}

func TestRecoveryDispatchPresentationIsReadableAndJSONSafe(t *testing.T) {
	s := handoff.State{Attempts: 2, MaxAttempts: 12, Assessments: 1, MaxAssessments: 3, ActiveRecovery: &handoff.Recovery{Role: "iragent", Step: "reviewing", Diagnostic: "/tmp/prior.log", PriorError: "capacity unavailable", Changes: "external evidence: review-03.md (new)", ArchivedEvidence: "/tmp/review-03-partial.md"}}
	var out bytes.Buffer
	displayRecoveryDispatch(&out, s, false)
	for _, want := range []string{"Recovering failed iragent step reviewing", "2/12 invocations", "1/3 independent assessments", "/tmp/prior.log", "capacity unavailable", "review-03-partial.md"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in recovery dispatch %s", want, out.String())
		}
	}
	out.Reset()
	displayRecoveryDispatch(&out, s, true)
	var record map[string]any
	if err := json.Unmarshal(out.Bytes(), &record); err != nil || record["type"] != "recovery-dispatch" || record["role"] != "iragent" {
		t.Fatalf("JSON recovery dispatch: %#v %v output=%q", record, err, out.String())
	}
}

func TestCompletedPresentationOffersNextRunAndHistory(t *testing.T) {
	var out bytes.Buffer
	display(&out, handoff.State{Phase: "completed", TaskID: "APP-002", Candidate: "abc123", CompletionRecord: "/tmp/completion.json", Attempts: 5, MaxAttempts: 12}, false)
	for _, want := range []string{"Concoct: completed", "Task: APP-002", "Completion record: /tmp/completion.json", "No active work.", "concoct run"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}
func TestApprovalAndBlockerPresentation(t *testing.T) {
	var out bytes.Buffer
	s := handoff.State{Phase: "awaiting_approval", Roadmap: "/tmp/roadmaps/deploy plan.md", TaskID: "APP-002", Candidate: "abc123", ApprovedCommit: "abc123", ReagentSummary: "implemented", ReviewSummary: "approved", ReviewEvidence: "/tmp/review-01.md", Attempts: 2, MaxAttempts: 12, Assessments: 1, MaxAssessments: 3}
	display(&out, s, false)
	for _, want := range []string{"Roadmap: /tmp/roadmaps/deploy plan.md", "Task: APP-002", "Candidate: abc123", "Reagent summary: implemented", "Iragent summary: approved", "Review evidence: /tmp/review-01.md", "--roadmap '/tmp/roadmaps/deploy plan.md' --approve abc123"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
	out.Reset()
	s.Phase = "blocked"
	s.Question = &codexapp.RoleQuestion{Text: "Choose mode?"}
	display(&out, s, false)
	if !strings.Contains(out.String(), "Question: Choose mode?") || !strings.Contains(out.String(), "--roadmap '/tmp/roadmaps/deploy plan.md' --answer TEXT") {
		t.Fatalf("blocker output: %s", out.String())
	}
	out.Reset()
	s.Phase = "limit_exhausted"
	s.Question = nil
	s.LastError = "approved candidate retained, but invocation limit leaves no integration turn"
	display(&out, s, false)
	if strings.Contains(out.String(), "concoct run --approve") || !strings.Contains(out.String(), "no integration turn") {
		t.Fatalf("exhausted approval output: %s", out.String())
	}
}

func TestDiagnosticsAndQuietSummaryPresentation(t *testing.T) {
	var out bytes.Buffer
	s := handoff.State{Phase: "no_suitable_work", Summary: "Nothing ready", LastRole: "reagent", Diagnostics: "/tmp/provider-123.log", MaxAttempts: 12, MaxAssessments: 3}
	display(&out, s, false)
	if !strings.Contains(out.String(), "reagent result: Nothing ready") || !strings.Contains(out.String(), "Provider diagnostics: /tmp/provider-123.log") {
		t.Fatalf("missing source or diagnostic location: %s", out.String())
	}
	out.Reset()
	stream := newMessageStream(&out, false, true)
	stream.event(codexapp.Event{Kind: "item/completed", ThreadID: "reagent", TurnID: "one", Raw: []byte(`{"item":{"id":"message","type":"agentMessage","phase":"commentary","text":"Working"}}`)}, func(string) string { return "reagent" })
	stream.finish()
	display(&out, s, false)
	if strings.Contains(out.String(), "Working") || !strings.Contains(out.String(), "reagent result: Nothing ready") {
		t.Fatalf("quiet hid result or showed progress: %s", out.String())
	}
}

func TestLimitExhaustedPresentationBuildsContextualAuthorizationCommand(t *testing.T) {
	var out bytes.Buffer
	s := handoff.State{
		Phase: "limit_exhausted", LimitPhase: "correcting", Repository: "/tmp/repo with space", Roadmap: "/tmp/repo with space/deploy plans/roadmap.md", Verdict: "changes_requested",
		Attempts: 12, MaxAttempts: 12, Assessments: 3, MaxAssessments: 3,
	}
	displayWithContext(&out, s, false, commandContext{executable: "/tmp/bin dir/concoct's", options: handoff.Options{Root: "/tmp/root;literal"}})
	for _, want := range []string{"--repo '/tmp/repo with space'", "--root '/tmp/root;literal'", "--roadmap '/tmp/repo with space/deploy plans/roadmap.md'", "--approve-invocation-limit-increase 13", "--approve-review-limit-increase 4", "new total ceilings"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
}

func TestShellCommandPreservesLiteralArguments(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "harmless command's")
	output := filepath.Join(dir, "arguments.txt")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGS_OUT\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	values := []string{"run", "--repo", "/tmp/repo with space", "--root", "/tmp/root;$(false)", "--approve-review-limit-increase", "5"}
	cmd := exec.Command("sh", "-c", shellCommand(executable, values...))
	cmd.Env = append(os.Environ(), "ARGS_OUT="+output)
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("execute generated command: %v: %s", err, data)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"); strings.Join(got, "\x00") != strings.Join(values, "\x00") {
		t.Fatalf("arguments changed: got %#v want %#v", got, values)
	}
}
