package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/review"
	"github.com/gopher-launch/concoct/internal/selection"
)

type fakeOperations struct {
	calls   int
	request request
	out     outcome
	event   *codexapp.Event
}

func TestFreshRunTimeLimits(t *testing.T) {
	req, err := parseRequest([]string{"run", "--turn-timeout", "2m", "--stall-timeout", "30s"})
	if err != nil || req.TurnTimeout != 2*time.Minute || req.StallTimeout != 30*time.Second {
		t.Fatalf("time limits: %+v, %v", req, err)
	}
	for _, args := range [][]string{{"run", "--turn-timeout", "0s"}, {"run", "--stall-timeout", "25h"}, {"status", "--turn-timeout", "1m"}, {"run", "--approve", "a-1", "--turn-timeout", "1m"}} {
		if _, err := parseRequest(args); err == nil {
			t.Fatalf("invalid time limit options accepted: %v", args)
		}
	}
	var output bytes.Buffer
	printStatus(&output, view(activework.Snapshot{Active: true, TaskID: "APP-002", Dispatch: &dispatch.State{WatchdogElapsed: 2 * time.Minute, WatchdogStall: 30 * time.Second}}, request{}))
	if !strings.Contains(output.String(), "Time limits: turn 2m0s; inactivity 30s") {
		t.Fatalf("time limits hidden from operator: %s", output.String())
	}
}

func TestRecoveryStatusShowsRetainedReasonBeforeReadyPhase(t *testing.T) {
	snapshot := activework.Snapshot{Active: true, TaskID: "APP-002", Phase: "ready-for-review", Hold: "recovery-required", PendingRequest: "reconcile callback", Recovery: &activework.RecoveryIssue{AttemptID: "attempt-1", Stage: "provider-completion", Operation: "save_checkpoint", Reason: "matching completed item missing", EvidenceRef: "turn turn-1; call call-1", AgentResult: "ready for review"}}
	if got := disposition(snapshot); got != "recovery_required" {
		t.Fatalf("status disposition = %s", got)
	}
	status := view(snapshot, request{})
	var output bytes.Buffer
	printStatus(&output, status)
	for _, fragment := range []string{"matching completed item missing", "paused: previous execution needs inspection", "ready for review", "turn turn-1"} {
		if !strings.Contains(output.String(), fragment) {
			t.Fatalf("missing %q in status: %s", fragment, output.String())
		}
	}
}

func (f *fakeOperations) Do(_ context.Context, r request, stream func(codexapp.Event)) (outcome, error) {
	f.calls++
	f.request = r
	if f.event != nil {
		stream(codexapp.Event{Kind: "turn/started", ThreadID: "thread-1"})
		stream(*f.event)
	}
	return f.out, nil
}

func TestCLIInterventionRoutingAndInvalidOptions(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		field func(request) string
	}{
		{[]string{"run", "--answer", "q-1", "--text", "evidence"}, func(r request) string { return r.Answer + ":" + r.Text }},
		{[]string{"run", "--answer", "q-1", "--choice", "option-a"}, func(r request) string { return r.Answer + ":" + r.Choice }},
		{[]string{"run", "--answer", "q-1", "--answer-file", "answer.txt"}, func(r request) string { return r.Answer + ":" + r.AnswerFile }},
		{[]string{"run", "--authorize", "dispatch-1"}, func(r request) string { return r.Authorize }},
		{[]string{"run", "--approve", "accept-1"}, func(r request) string { return r.Approve }},
		{[]string{"run", "--extend", "APP-002", "--review-limit", "4"}, func(r request) string { return r.Extend }},
		{[]string{"run", "--defer", "APP-002", "--reason", "paused"}, func(r request) string { return r.Defer + ":" + r.Reason }},
		{[]string{"run", "--abandon", "APP-002", "--reason", "stopped"}, func(r request) string { return r.Abandon + ":" + r.Reason }},
		{[]string{"run", "--resume", "APP-002"}, func(r request) string { return r.Resume }},
	} {
		f := &fakeOperations{out: outcome{Disposition: "completed"}}
		var out, errs bytes.Buffer
		if code := run(context.Background(), tc.args, &out, &errs, f); code != 0 || f.calls != 1 || tc.field(f.request) == "" {
			t.Fatalf("%v: code %d, request %+v, err %s", tc.args, code, f.request, errs.String())
		}
	}
	for _, args := range [][]string{
		{"run", "--answer", "q"}, {"run", "--answer", "q", "--text", "x", "--choice", "a"},
		{"run", "--approve", "a", "--authorize", "b"}, {"run", "--extend", "APP-002"},
		{"run", "--review-limit", "4"}, {"run", "--review-limit", "4x", "--extend", "APP-002"},
		{"run", "--defer", "APP-002"}, {"run", "--gate", "unrecognized"},
		{"status", "--approve", "a"}, {"status", "APP-002", "APP-003"},
		{"init"}, {"run", "--obsolete", "x"},
	} {
		f := &fakeOperations{}
		var out, errs bytes.Buffer
		if code := run(context.Background(), args, &out, &errs, f); code != 2 || f.calls != 0 || errs.Len() == 0 {
			t.Fatalf("invalid %v: code %d, calls %d, err %s", args, code, f.calls, errs.String())
		}
	}
}

func TestCLIStreamsProgressBeforeVerifiedResult(t *testing.T) {
	event := codexapp.Event{Kind: "item/agentMessage/delta", Raw: json.RawMessage(`{"delta":"agent says ready"}`)}
	for _, mode := range []struct {
		flags    []string
		marker   string
		progress bool
	}{
		{nil, "agent says ready", true},
		{[]string{"--json"}, `"type":"agent-progress"`, true},
		{[]string{"--quiet"}, "agent says ready", false},
	} {
		f := &fakeOperations{out: outcome{Disposition: "ready_for_acceptance", Acceptance: &activework.Acceptance{ID: "accept-123"}}, event: &event}
		var out, errs bytes.Buffer
		args := append([]string{"run"}, mode.flags...)
		if code := run(context.Background(), args, &out, &errs, f); code != 12 {
			t.Fatalf("%v code %d", args, code)
		}
		text := out.String()
		if strings.Contains(text, mode.marker) != mode.progress || !strings.Contains(text, "accept-123") {
			t.Fatalf("%v output: %s", args, text)
		}
		if mode.progress && strings.Index(text, mode.marker) > strings.Index(text, "ready-for-acceptance") {
			t.Fatalf("agent progress was delayed after verified result: %s", text)
		}
	}
}

func TestCLIQuestionContextInHumanQuietAndJSON(t *testing.T) {
	question := &activework.Question{ID: "question-1", Requester: "iragent", Objective: "Assess APP-002", Evidence: "Two input modes are possible.", Missing: "Choose a mode.", SafeWork: "Inspect tests.", NextStep: "Resume review after answer.", Options: []activework.QuestionOption{{ID: "safe", Description: "Safe mode", Tradeoff: "Slower"}}}
	snapshot := activework.Snapshot{Active: true, TaskID: "APP-002", Phase: "reviewing", Hold: "operator-decision", PendingRequest: "answer question-1 for iragent", Question: question}
	for _, flags := range [][]string{nil, {"--quiet"}, {"--json"}} {
		f := &fakeOperations{out: outcome{Disposition: "decision_required", Snapshot: snapshot}}
		var out, errs bytes.Buffer
		if code := run(context.Background(), append([]string{"run"}, flags...), &out, &errs, f); code != 10 {
			t.Fatalf("question exit %v: %d %s", flags, code, errs.String())
		}
		for _, want := range []string{"question-1", "iragent", "Assess APP-002", "Two input modes", "Choose a mode", "Inspect tests", "Resume review", "safe", "Slower", "--answer question-1"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%v missing %q: %s", flags, want, out.String())
			}
		}
	}
}

func TestStatusStructuredQuestionReadOnly(t *testing.T) {
	repo, root, store := cliRepo(t)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	base, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Start(activework.StartSpec{TaskID: "APP-002", ParentBranch: "main", ParentCommit: strings.TrimSpace(string(base)), TaskBranch: "work/app-002", Objective: "test", Acceptance: "test", SelectionRationale: "test"}); err != nil {
		t.Fatal(err)
	}
	profiles := map[dispatch.Role]codexapp.RoleSettings{}
	for _, role := range []dispatch.Role{dispatch.Reagent, dispatch.Iragent} {
		profiles[role] = codexapp.RoleSettings{Role: string(role), Model: "test", Effort: "medium", Sandbox: "danger-full-access", ApprovalPolicy: "never"}
	}
	policy, err := dispatch.NewState(profiles, dispatch.DefaultLimits(), dispatch.GatePolicy{}, dispatch.QuotaPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.ConfigureDispatch(policy); err != nil {
		t.Fatal(err)
	}
	if err := owner.UpdateOperational(activework.Operational{Phase: "engineering", RoleIDs: map[string]string{"reagent": "thread-1"}}); err != nil {
		t.Fatal(err)
	}
	if decision, err := owner.ReserveDispatch("engineering-1", dispatch.Binding{TaskID: "APP-002", Role: dispatch.Reagent, Activity: dispatch.Engineering}, time.Now().UTC()); err != nil || !decision.Dispatch {
		t.Fatalf("reserve: %+v %v", decision, err)
	}
	for _, step := range []func() error{
		func() error { return owner.BindDispatchThread("engineering-1", "thread-1") },
		func() error { return owner.MarkTurnRequested("engineering-1") },
		func() error { return owner.ObserveDispatchTurn("engineering-1", "turn-1") },
		func() error {
			return owner.RecordDispatchOutcome("engineering-1", "thread-1", "turn-1", "completed", "question")
		},
		func() error { return owner.FinishDispatch("engineering-1", "completed", nil) },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	question, err := owner.AskRoleQuestion("reagent", "engineering-1", &codexapp.RoleQuestion{Objective: "Choose mode", Evidence: "Both modes pass.", Missing: "Pick one.", SafeWork: "Read docs.", NextStep: "Resume with choice.", Options: []codexapp.RoleQuestionOption{{ID: "safe", Description: "Safe mode", Tradeoff: "Slower"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	for _, flags := range [][]string{nil, {"--quiet"}, {"--json"}} {
		var out, errs bytes.Buffer
		args := append([]string{"status", "APP-002", "--repo", repo, "--root", root}, flags...)
		if code := run(context.Background(), args, &out, &errs, liveOperations{}); code != 10 {
			t.Fatalf("question status %v: %d %s", flags, code, errs.String())
		}
		for _, want := range []string{question.ID, "Choose mode", "Both modes pass", "Pick one", "Read docs", "Resume with choice", "safe", "Slower", "--answer " + question.ID, "--choice safe --repo '" + repo + "'"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("question status %v missing %q: %s", flags, want, out.String())
			}
		}
	}
}

func TestCLIJSONInvalidOptionKeepsDisposition(t *testing.T) {
	f := &fakeOperations{}
	var out, errs bytes.Buffer
	if code := run(context.Background(), []string{"run", "--json", "--obsolete", "x"}, &out, &errs, f); code != 2 || f.calls != 0 {
		t.Fatalf("invalid JSON request: code %d calls %d", code, f.calls)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result["disposition"] != "prerequisite-error" {
		t.Fatalf("invalid request lost JSON disposition: %s, %v", out.String(), err)
	}
}

func TestCLINextCommandRetainsCheckoutAndRoot(t *testing.T) {
	f := &fakeOperations{out: outcome{Disposition: "ready_for_acceptance", Acceptance: &activework.Acceptance{ID: "accept-123"}}}
	var out, errs bytes.Buffer
	args := []string{"run", "--repo", "/tmp/my project", "--root", "/tmp/state", "--codex", "/tmp/codex"}
	if code := run(context.Background(), args, &out, &errs, f); code != 12 {
		t.Fatalf("next command result: %d %s", code, errs.String())
	}
	want := "concoct run --approve accept-123 --repo '/tmp/my project' --root '/tmp/state' --codex '/tmp/codex'"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("next command lost context: %s", out.String())
	}
}

func cliRepo(t *testing.T) (string, string, *activework.Store) {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "base"}} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	store, err := activework.Open(repo, filepath.Join(root, "external"))
	if err != nil {
		t.Fatal(err)
	}
	return repo, store.Root, store
}

func TestStatusReadOnlyDuringOwnerAndStaleIDs(t *testing.T) {
	repo, root, store := cliRepo(t)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	cmd := exec.Command("git", "-C", repo, "rev-parse", "HEAD")
	base, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	_, err = owner.Start(activework.StartSpec{TaskID: "APP-002", ParentBranch: "main", ParentCommit: strings.TrimSpace(string(base)), TaskBranch: "work/app-002", Objective: "test", Acceptance: "test", SelectionRationale: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := run(context.Background(), []string{"status", "APP-002", "--json", "--repo", repo, "--root", root}, &out, &errs, liveOperations{}); code != 18 || !strings.Contains(out.String(), `"task-id":"APP-002"`) {
		t.Fatalf("status under lock: %d %s %s", code, out.String(), errs.String())
	}
	if code := run(context.Background(), []string{"status", "APP-003", "--repo", repo, "--root", root}, &out, &errs, liveOperations{}); code != 2 {
		t.Fatalf("stale task accepted: %d", code)
	}
	if code := run(context.Background(), []string{"run", "--approve", "wrong", "--repo", repo, "--root", root}, &out, &errs, liveOperations{}); code != 2 {
		t.Fatalf("stale acceptance accepted: %d", code)
	}
}

func TestStatusLimitHoldHasExactExtension(t *testing.T) {
	repo, root, store := cliRepo(t)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	base, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Start(activework.StartSpec{TaskID: "APP-002", ParentBranch: "main", ParentCommit: strings.TrimSpace(string(base)), TaskBranch: "work/app-002", Objective: "test", Acceptance: "test", SelectionRationale: "test"}); err != nil {
		t.Fatal(err)
	}
	profiles := map[dispatch.Role]codexapp.RoleSettings{}
	for _, role := range []dispatch.Role{dispatch.Reagent, dispatch.Iragent} {
		profiles[role] = codexapp.RoleSettings{Role: string(role), Model: "test", Effort: "medium", Sandbox: "danger-full-access", ApprovalPolicy: "never"}
	}
	policy, err := dispatch.NewState(profiles, dispatch.Limits{Dispatches: 1, SelfReviews: 3, Assessments: 3, Recovery: 2}, dispatch.GatePolicy{}, dispatch.QuotaPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.ConfigureDispatch(policy); err != nil {
		t.Fatal(err)
	}
	if decision, err := owner.ReserveDispatch("engineering-1", dispatch.Binding{TaskID: "APP-002", Role: dispatch.Reagent, Activity: dispatch.Engineering}, time.Now().UTC()); err != nil || !decision.Dispatch {
		t.Fatalf("reserve: %+v %v", decision, err)
	}
	if err := owner.FinishDispatch("engineering-1", "completed", nil); err != nil {
		t.Fatal(err)
	}
	if err := owner.UpdateOperational(activework.Operational{Phase: "engineering", Hold: "limit-exhausted", PendingRequest: "extend dispatch allowance to 2"}); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{nil, {"--quiet"}, {"--json"}} {
		var out, errs bytes.Buffer
		args := append([]string{"status", "APP-002", "--repo", repo, "--root", root}, flags...)
		if code := run(context.Background(), args, &out, &errs, liveOperations{}); code != 10 {
			t.Fatalf("status %v: %d %s", flags, code, errs.String())
		}
		for _, want := range []string{"decision-required", "extend dispatch allowance to 2", "concoct run --extend APP-002 --dispatch-limit 2"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("status %v missing %q: %s", flags, want, out.String())
			}
		}
	}
	var heldOut, heldErr bytes.Buffer
	if code := run(context.Background(), []string{"run", "--repo", repo, "--root", root, "--codex", filepath.Join(t.TempDir(), "missing-codex")}, &heldOut, &heldErr, liveOperations{}); code != 10 || !strings.Contains(heldOut.String(), "--extend APP-002 --dispatch-limit 2") {
		t.Fatalf("plain run lost held decision or launched provider: %d %s %s", code, heldOut.String(), heldErr.String())
	}
}

func TestAuthorizationGateAndExactIDWithoutProvider(t *testing.T) {
	repo, root, store := cliRepo(t)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	baseBytes, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	_, err = owner.BeginSelection(activework.SelectionSpec{ParentBranch: "main", ParentCommit: strings.TrimSpace(string(baseBytes))})
	if err != nil {
		t.Fatal(err)
	}
	profiles := map[dispatch.Role]codexapp.RoleSettings{}
	for _, role := range []dispatch.Role{dispatch.Reagent, dispatch.Iragent} {
		profiles[role] = codexapp.RoleSettings{Role: string(role), Model: "test", Effort: "medium", Sandbox: "danger-full-access", ApprovalPolicy: "never"}
	}
	policy, err := dispatch.NewState(profiles, dispatch.DefaultLimits(), dispatch.GatePolicy{Activities: map[dispatch.Activity]bool{dispatch.Selection: true}}, dispatch.QuotaPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.ConfigureDispatch(policy); err != nil {
		t.Fatal(err)
	}
	decision, err := owner.ReserveDispatch("dispatch-1", dispatch.Binding{Role: dispatch.Reagent, Activity: dispatch.Selection}, time.Now().UTC())
	if err != nil || decision.Dispatch {
		t.Fatalf("gate: %+v %v", decision, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	args := []string{"run", "--repo", repo, "--root", root}
	if code := run(context.Background(), args, &out, &errs, liveOperations{}); code != 11 || !strings.Contains(out.String(), "dispatch-1") {
		t.Fatalf("gate status: %d %s", code, out.String())
	}
	if code := run(context.Background(), append(args, "--authorize", "wrong"), &out, &errs, liveOperations{}); code != 2 {
		t.Fatalf("wrong authorization accepted: %d", code)
	}
	if code := run(context.Background(), append(args, "--authorize", "dispatch-1", "--codex", filepath.Join(t.TempDir(), "missing-codex")), &out, &errs, liveOperations{}); code != 2 {
		t.Fatalf("expected unavailable runner after authorization: %d", code)
	}
	status, err := store.Status()
	if err != nil || status.Dispatch.Pending == nil || !status.Dispatch.Pending.Authorized {
		t.Fatalf("exact authorization not durable: %+v %v", status, err)
	}
}

type noWorkProvider struct{ starts, turns, closed int }

type schemaRejectProvider struct{ *noWorkProvider }

func (p *schemaRejectProvider) Agent() selection.Agent { return p }
func (p *schemaRejectProvider) Reviewer() review.Agent { return p }

func (p *schemaRejectProvider) RunTurn(context.Context, string, string, codexapp.RoleSettings, codexapp.ToolHandler, func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	p.turns++
	return codexapp.TurnOutcome{}, fmt.Errorf("%w: app-server turn/start: -32600 Invalid schema for response_format 'codex_output_schema': 'required' must include task_id; %sprovider diagnostic tail", codexapp.ErrInvalidResponseSchema, strings.Repeat("schema context; ", 25))
}

func TestCLIInvalidResponseSchemaIsDefiniteStartupFailure(t *testing.T) {
	repo, root, store := cliRepo(t)
	writeCLIProject(t, repo)
	fake := &schemaRejectProvider{noWorkProvider: &noWorkProvider{}}
	ops := liveOperations{Open: func(context.Context, string) (provider, error) { return fake, nil }}
	args := []string{"run", "--repo", repo, "--root", root}
	var out, errs bytes.Buffer
	if code := run(context.Background(), args, &out, &errs, ops); code != 2 || fake.turns != 1 {
		t.Fatalf("startup failure: code %d turns %d out %s err %s", code, fake.turns, out.String(), errs.String())
	}
	for _, want := range []string{"Concoct verified: startup-failed", "could not start the agent", "invalid response schema", "Correct Concoct's code", "paused: agent could not start", "Diagnostic:"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q: %s", want, out.String())
		}
	}
	for _, hidden := range []string{"response_format", "provider diagnostic tail", "Next: concoct run", "uncertain", "needs inspection"} {
		if strings.Contains(out.String(), hidden) || strings.Contains(errs.String(), hidden) {
			t.Fatalf("raw rejection or retry advice leaked: %s %s", out.String(), errs.String())
		}
	}
	snapshot, err := store.Status()
	if err != nil || snapshot.Recovery == nil || snapshot.Recovery.Stage != activework.InvalidResponseSchemaStage || len(snapshot.Dispatch.Attempts) != 1 || snapshot.Dispatch.Attempts[0].Status != "failed" || snapshot.Dispatch.Attempts[0].TurnID != "" {
		t.Fatalf("rejection was not bound as a failed pre-turn attempt: %+v, %v", snapshot, err)
	}
	files, err := filepath.Glob(filepath.Join(root, "repos", "*", snapshot.Recovery.EvidenceRef))
	if err != nil || len(files) != 1 {
		t.Fatalf("diagnostic file: %v, %v", files, err)
	}
	diagnostic, err := os.ReadFile(files[0])
	if err != nil || !strings.Contains(string(diagnostic), "provider diagnostic tail") || !strings.Contains(string(diagnostic), "required' must include task_id") {
		t.Fatalf("full provider error was not retained: %q, %v", diagnostic, err)
	}
	out.Reset()
	if code := run(context.Background(), args, &out, &errs, ops); code != 2 || fake.turns != 1 || !strings.Contains(out.String(), "Correct Concoct's code") || strings.Contains(out.String(), "Next: concoct run") {
		t.Fatalf("unchanged retry was recommended or dispatched: code %d turns %d out %s", code, fake.turns, out.String())
	}
	out.Reset()
	if code := run(context.Background(), []string{"status", "--json", "--repo", repo, "--root", root}, &out, &errs, ops); code != 2 || !strings.Contains(out.String(), `"disposition":"startup-failed"`) || !strings.Contains(out.String(), "Correct Concoct's code") || strings.Contains(out.String(), "response_format") || strings.Contains(out.String(), `"next":"concoct run`) {
		t.Fatalf("read-only status lost clear startup failure: code %d out %s", code, out.String())
	}
}

func (p *noWorkProvider) Agent() selection.Agent                                 { return p }
func (p *noWorkProvider) Reviewer() review.Agent                                 { return p }
func (p *noWorkProvider) PrepareIdle(context.Context, activework.Snapshot) error { return nil }
func (p *noWorkProvider) Models(context.Context) ([]codexapp.Model, error) {
	var models []codexapp.Model
	_ = json.Unmarshal([]byte(`[{"model":"test-model","isDefault":true,"supportedReasoningEfforts":[{"reasoningEffort":"medium"},{"reasoningEffort":"high"}]}]`), &models)
	return models, nil
}
func (p *noWorkProvider) RateLimits(context.Context) (json.RawMessage, error) {
	return nil, errors.New("quota unavailable")
}
func (p *noWorkProvider) Close() error { p.closed++; return nil }
func (p *noWorkProvider) StartThread(context.Context, string, codexapp.RoleSettings, []codexapp.ToolSpec) (codexapp.Thread, error) {
	p.starts++
	return codexapp.Thread{ID: "fresh-thread"}, nil
}
func (p *noWorkProvider) RunTurn(_ context.Context, _ string, _ string, _ codexapp.RoleSettings, _ codexapp.ToolHandler, onEvent func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	p.turns++
	if onEvent != nil {
		onEvent(codexapp.Event{Kind: "turn/started", ThreadID: "fresh-thread", TurnID: "turn-1"})
		onEvent(codexapp.Event{Kind: "item/agentMessage/delta", ThreadID: "fresh-thread", TurnID: "turn-1", Raw: json.RawMessage(`{"delta":"No task chosen."}`)})
	}
	return codexapp.TurnOutcome{ThreadID: "fresh-thread", TurnID: "turn-1", Status: "completed", FinalText: `{"disposition":"no_suitable_work","summary":"No work selected.","task_id":null,"candidate":null,"findings":null,"question":null}`}, nil
}
func (p *noWorkProvider) Idle(context.Context, string) error { return nil }

func writeCLIProject(t *testing.T, repo string) {
	t.Helper()
	files := map[string]string{
		"AGENTS.md":       "Work on one task.\n",
		"roadmap.md":      "## APP-001 — Delivered\n- Kind: task\n- Status: delivered\n- Priority: high\n\n## APP-002 — Ready\n- Kind: task\n- Status: ready\n- Priority: high\n- Depends on: APP-001\n\n**Objective:** build task.\n\n**Acceptance criteria:**\n\n- Build it.\n",
		"capabilities.md": "# Delivered capabilities\n\n## APP-001 — Delivered\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("git", "-C", repo, "add", ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	cmd = exec.Command("git", "-C", repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "project docs")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
}

func TestCLINominalNoWorkAndGateContinuation(t *testing.T) {
	for _, gated := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "gated"}[gated], func(t *testing.T) {
			repo, root, store := cliRepo(t)
			writeCLIProject(t, repo)
			fake := &noWorkProvider{}
			ops := liveOperations{Open: func(context.Context, string) (provider, error) { return fake, nil }}
			args := []string{"run", "--repo", repo, "--root", root}
			if gated {
				args = append(args, "--gate", "selection")
			}
			var out, errs bytes.Buffer
			code := run(context.Background(), args, &out, &errs, ops)
			if gated {
				if code != 11 || fake.turns != 0 {
					t.Fatalf("gate did not hold before agent turn: %d %s", code, out.String())
				}
				snapshot, err := store.Status()
				if err != nil || snapshot.Dispatch == nil || snapshot.Dispatch.Pending == nil {
					t.Fatalf("gate state: %+v %v", snapshot, err)
				}
				out.Reset()
				code = run(context.Background(), []string{"run", "--authorize", snapshot.Dispatch.Pending.ID, "--repo", repo, "--root", root}, &out, &errs, ops)
			}
			if code != 0 || fake.turns != 1 || !strings.Contains(out.String(), "No task chosen.") || !strings.Contains(out.String(), "no-suitable-work") {
				t.Fatalf("nominal no-work: code %d, turns %d, output %s, err %s", code, fake.turns, out.String(), errs.String())
			}
			if snapshot, err := store.Status(); err != nil || snapshot.Active {
				t.Fatalf("no-work kept active binding: %+v %v", snapshot, err)
			}
		})
	}
}

type deliveryProvider struct {
	repo, root, reagentCheckout, reviewCheckout string
	turns, prepared                             int
}

func (p *deliveryProvider) Agent() selection.Agent { return p }
func (p *deliveryProvider) Reviewer() review.Agent { return p }
func (p *deliveryProvider) PrepareIdle(_ context.Context, snapshot activework.Snapshot) error {
	if snapshot.RoleIDs["reagent"] == "" || snapshot.RoleIDs["iragent"] == "" {
		return fmt.Errorf("idle preparation lacks role bindings")
	}
	p.prepared++
	return nil
}
func (p *deliveryProvider) Models(context.Context) ([]codexapp.Model, error) {
	return (&noWorkProvider{}).Models(context.Background())
}
func (p *deliveryProvider) RateLimits(context.Context) (json.RawMessage, error) {
	return nil, errors.New("quota unavailable")
}
func (p *deliveryProvider) Close() error                       { return nil }
func (p *deliveryProvider) Idle(context.Context, string) error { return nil }
func (p *deliveryProvider) StartThread(_ context.Context, cwd string, settings codexapp.RoleSettings, _ []codexapp.ToolSpec) (codexapp.Thread, error) {
	if settings.Role == "reagent" {
		p.reagentCheckout = cwd
		return codexapp.Thread{ID: "fresh-thread"}, nil
	}
	p.reviewCheckout = cwd
	return codexapp.Thread{ID: "reviewer-thread"}, nil
}
func (p *deliveryProvider) RunTurn(ctx context.Context, threadID, _ string, settings codexapp.RoleSettings, handler codexapp.ToolHandler, onEvent func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	p.turns++
	if settings.Role == "iragent" {
		if onEvent != nil {
			onEvent(codexapp.Event{Kind: "turn/started", ThreadID: threadID, TurnID: "review-turn"})
		}
		if err := cliCallback(ctx, handler, "iragent", threadID, "review-turn", "save_assessment", map[string]string{"content": "Candidate source and criteria verified."}); err != nil {
			return codexapp.TurnOutcome{}, err
		}
		return codexapp.TurnOutcome{ThreadID: threadID, TurnID: "review-turn", Status: "completed", FinalText: `{"disposition":"approved","summary":"Candidate approved."}`}, nil
	}
	if onEvent != nil {
		onEvent(codexapp.Event{Kind: "turn/started", ThreadID: threadID, TurnID: "reagent-turn"})
	}
	cmd := exec.Command("git", "-C", p.reagentCheckout, "switch", "-qc", "work/app-002")
	if out, err := cmd.CombinedOutput(); err != nil {
		return codexapp.TurnOutcome{}, fmt.Errorf("task branch: %v: %s", err, out)
	}
	if err := cliCallback(ctx, handler, "reagent", threadID, "reagent-turn", "claim_task", map[string]string{"task_id": "APP-002", "task_branch": "work/app-002", "selection_rationale": "ready and highest priority", "objective": "build task.", "acceptance": "- Build it."}); err != nil {
		return codexapp.TurnOutcome{}, err
	}
	if err := os.WriteFile(filepath.Join(p.reagentCheckout, "feature.txt"), []byte("implemented\n"), 0600); err != nil {
		return codexapp.TurnOutcome{}, err
	}
	roadmap, err := os.ReadFile(filepath.Join(p.reagentCheckout, "roadmap.md"))
	if err != nil {
		return codexapp.TurnOutcome{}, err
	}
	if err := os.WriteFile(filepath.Join(p.reagentCheckout, "roadmap.md"), []byte(strings.Replace(string(roadmap), "- Status: ready", "- Status: delivered", 1)), 0600); err != nil {
		return codexapp.TurnOutcome{}, err
	}
	file, err := os.OpenFile(filepath.Join(p.reagentCheckout, "capabilities.md"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return codexapp.TurnOutcome{}, err
	}
	if _, err := file.WriteString("\n## APP-002 — Implemented task\n"); err != nil {
		file.Close()
		return codexapp.TurnOutcome{}, err
	}
	if err := file.Close(); err != nil {
		return codexapp.TurnOutcome{}, err
	}
	checkpoint := selection.ReadyCheckpoint{TaskID: "APP-002", Plan: "Implement task.", Progress: "Added feature and reconciled docs.", Criteria: []selection.CriterionEvidence{{Criterion: "Build it.", Result: "passed", Evidence: "Feature content checked."}}, SelfReview: "Reviewed source and docs.", NextAction: "Independent review.", Deliverables: []string{"feature.txt"}}
	body, err := json.Marshal(checkpoint)
	if err != nil {
		return codexapp.TurnOutcome{}, err
	}
	if err := cliCallback(ctx, handler, "reagent", threadID, "reagent-turn", "save_checkpoint", map[string]string{"content": string(body)}); err != nil {
		return codexapp.TurnOutcome{}, err
	}
	if onEvent != nil {
		onEvent(codexapp.Event{Kind: "item/agentMessage/delta", ThreadID: threadID, TurnID: "reagent-turn", Raw: json.RawMessage(`{"delta":"Task ready for review."}`)})
	}
	return codexapp.TurnOutcome{ThreadID: threadID, TurnID: "reagent-turn", Status: "completed", FinalText: `{"disposition":"ready_for_review","summary":"Candidate ready."}`}, nil
}
func cliCallback(ctx context.Context, handler codexapp.ToolHandler, role, thread, turn, tool string, args any) error {
	body, err := json.Marshal(args)
	if err != nil {
		return err
	}
	response, err := handler(ctx, codexapp.ToolCall{Role: role, ThreadID: thread, TurnID: turn, CallID: tool + "-1", Tool: tool, Arguments: body})
	if err != nil {
		return err
	}
	if !response.Success || response.Commit == nil {
		return fmt.Errorf("%s callback was not prepared", tool)
	}
	return response.Commit(ctx)
}

func TestCLIFullLocalDeliveryWithFakeProvider(t *testing.T) {
	repo, root, store := cliRepo(t)
	writeCLIProject(t, repo)
	fake := &deliveryProvider{repo: repo, root: root}
	ops := liveOperations{Open: func(context.Context, string) (provider, error) { return fake, nil }}
	var out, errs bytes.Buffer
	args := []string{"run", "--repo", repo, "--root", root}
	if code := run(context.Background(), args, &out, &errs, ops); code != 12 || fake.turns != 2 || !strings.Contains(out.String(), "Task ready for review.") {
		t.Fatalf("candidate flow: code %d, turns %d, out %s, err %s", code, fake.turns, out.String(), errs.String())
	}
	snapshot, err := store.Status()
	if err != nil || snapshot.Acceptance == nil || snapshot.Candidate == nil {
		t.Fatalf("candidate not offered: %+v %v", snapshot, err)
	}
	for _, want := range []string{"Source diff:", "Roadmap/capability diff:", "Final independent assessment:", "Candidate approved.", "Criterion passed", "Review assessment:", "Final review verdict: approved"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("acceptance result missing %q: %s", want, out.String())
		}
	}
	var statusOut bytes.Buffer
	if code := run(context.Background(), []string{"status", "APP-002", "--json", "--repo", repo, "--root", root}, &statusOut, &errs, ops); code != 12 {
		t.Fatalf("acceptance status: %d %s", code, statusOut.String())
	}
	for _, want := range []string{`"approval":`, `"diff-command":`, `"review-evidence":`, "Candidate approved.", `"reconciled-paths":`} {
		if !strings.Contains(statusOut.String(), want) {
			t.Errorf("acceptance JSON missing %q: %s", want, statusOut.String())
		}
	}
	var statusResult struct {
		Approval struct {
			ReviewEvidence string `json:"review-evidence"`
		} `json:"approval"`
	}
	if err := json.Unmarshal(statusOut.Bytes(), &statusResult); err != nil {
		t.Fatal(err)
	}
	reviewBytes, err := os.ReadFile(statusResult.Approval.ReviewEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statusResult.Approval.ReviewEvidence, append(append([]byte(nil), reviewBytes...), 'x'), 0600); err != nil {
		t.Fatal(err)
	}
	statusOut.Reset()
	if code := run(context.Background(), []string{"status", "APP-002", "--json", "--repo", repo, "--root", root}, &statusOut, &errs, ops); code != 2 || !strings.Contains(statusOut.String(), "evidence digest mismatch") || strings.Contains(statusOut.String(), `"approval":`) {
		t.Fatalf("tampered assessment remained approvable: %d %s", code, statusOut.String())
	}
	if err := os.WriteFile(statusResult.Approval.ReviewEvidence, reviewBytes, 0600); err != nil {
		t.Fatal(err)
	}
	candidate, id := snapshot.Candidate.Commit, snapshot.Acceptance.ID
	out.Reset()
	if code := run(context.Background(), append(args, "--approve", "wrong"), &out, &errs, ops); code != 2 || fake.turns != 2 {
		t.Fatalf("wrong acceptance changed candidate: %d %s", code, out.String())
	}
	out.Reset()
	if code := run(context.Background(), append(args, "--approve", id), &out, &errs, ops); code != 0 || !strings.Contains(out.String(), "completed") || fake.turns != 2 || fake.prepared != 1 {
		t.Fatalf("approved delivery: code %d, turns %d, out %s, err %s", code, fake.turns, out.String(), errs.String())
	}
	snapshot, err = store.Status()
	if err != nil || snapshot.Active {
		t.Fatalf("completion left active run: %+v %v", snapshot, err)
	}
	commitBytes, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(commitBytes)) != candidate {
		t.Fatalf("wrong integrated candidate: %s %v", commitBytes, err)
	}
	if _, err := os.Stat(filepath.Join(root, "workspaces")); err != nil {
		t.Fatal(err)
	}
}
