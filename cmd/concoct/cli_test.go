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
		{[]string{"--json"}, `"type":"agent_progress"`, true},
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
		if mode.progress && strings.Index(text, mode.marker) > strings.Index(text, "ready_for_acceptance") {
			t.Fatalf("agent progress was delayed after verified result: %s", text)
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
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result["disposition"] != "prerequisite_error" {
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
	if code := run(context.Background(), []string{"status", "APP-002", "--json", "--repo", repo, "--root", root}, &out, &errs, liveOperations{}); code != 18 || !strings.Contains(out.String(), `"task_id":"APP-002"`) {
		t.Fatalf("status under lock: %d %s %s", code, out.String(), errs.String())
	}
	if code := run(context.Background(), []string{"status", "APP-003", "--repo", repo, "--root", root}, &out, &errs, liveOperations{}); code != 2 {
		t.Fatalf("stale task accepted: %d", code)
	}
	if code := run(context.Background(), []string{"run", "--approve", "wrong", "--repo", repo, "--root", root}, &out, &errs, liveOperations{}); code != 2 {
		t.Fatalf("stale acceptance accepted: %d", code)
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
	return codexapp.TurnOutcome{ThreadID: "fresh-thread", TurnID: "turn-1", Status: "completed", FinalText: `{"disposition":"no_suitable_work","summary":"No work selected."}`}, nil
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
			if code != 0 || fake.turns != 1 || !strings.Contains(out.String(), "No task chosen.") || !strings.Contains(out.String(), "no_suitable_work") {
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
