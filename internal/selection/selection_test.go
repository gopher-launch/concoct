package selection

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
)

const testRoadmap = "## APP-001 — Delivered\n- Kind: task\n- Status: delivered\n- Priority: high\n\n## APP-002 — Ready\n- Kind: task\n- Status: ready\n- Priority: high\n- Depends on: APP-001\n\n**Objective:** implement the ready task.\n\n**Acceptance criteria:**\n\n- Preserve the task boundary.\n\n**Verification:** run tests.\n\n## APP-003 — Also ready\n- Kind: task\n- Status: ready\n- Priority: medium\n- Depends on: APP-001\n\n**Objective:** implement another task.\n\n**Acceptance criteria:**\n\n- Keep it separate.\n"

type fakeAgent struct {
	starts, turns int
	run           func(string, codexapp.ToolHandler) (codexapp.TurnOutcome, error)
	idle          func() error
	event         *codexapp.Event
}

func (a *fakeAgent) StartThread(_ context.Context, cwd string, _ codexapp.RoleSettings, tools []codexapp.ToolSpec) (codexapp.Thread, error) {
	a.starts++
	if len(tools) != 2 {
		panic("missing host tools")
	}
	return codexapp.Thread{ID: "fresh-thread"}, nil
}
func (a *fakeAgent) RunTurn(_ context.Context, _ string, prompt string, _ codexapp.RoleSettings, handler codexapp.ToolHandler, onEvent func(codexapp.Event)) (codexapp.TurnOutcome, error) {
	a.turns++
	if a.event != nil && onEvent != nil {
		onEvent(*a.event)
	}
	return a.run(prompt, handler)
}
func (a *fakeAgent) Idle(context.Context, string) error {
	if a.idle != nil {
		return a.idle()
	}
	return nil
}

func testRepo(t *testing.T) (string, string, dispatch.State) {
	t.Helper()
	root := t.TempDir()
	repo, external := filepath.Join(root, "repo"), filepath.Join(root, "external")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(repo, "init", "-q", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"AGENTS.md": "Follow the current task only. For Go changes run `gofmt`, `go test ./...`, `go vet ./...`, `go build ./cmd/concoct`, and `git diff --check`. Run focused smoke checks for CLI behavior.\n", "roadmap.md": testRoadmap, "capabilities.md": "# Delivered capabilities\n\n## APP-001 — Delivered\n", "source.txt": "baseline\n", ".gitignore": "/concoct\n*.out\n"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "baseline"}} {
		if _, err := gitOutput(repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	profiles := map[dispatch.Role]codexapp.RoleSettings{
		dispatch.Reagent: {Role: "reagent", Model: "test", Effort: "medium", Sandbox: "danger-full-access", ApprovalPolicy: "never"},
		dispatch.Iragent: {Role: "iragent", Model: "test", Effort: "high", Sandbox: "danger-full-access", ApprovalPolicy: "never"},
	}
	policy, err := dispatch.NewState(profiles, dispatch.DefaultLimits(), dispatch.GatePolicy{}, dispatch.QuotaPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	return repo, external, policy
}

func invoke(t *testing.T, handler codexapp.ToolHandler, callID, tool string, args any) error {
	t.Helper()
	data, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	response, err := handler(context.Background(), codexapp.ToolCall{Role: "reagent", ThreadID: "fresh-thread", TurnID: "one-turn", CallID: callID, Tool: tool, Arguments: data})
	if err != nil {
		return err
	}
	if !response.Success || response.Commit == nil {
		t.Fatalf("invalid tool response: %+v", response)
	}
	return response.Commit(context.Background())
}

func claimArgs(id, branch string) map[string]string {
	objective, acceptance := "implement the ready task.", "- Preserve the task boundary."
	if id == "APP-003" {
		objective, acceptance = "implement another task.", "- Keep it separate."
	}
	return map[string]string{"task_id": id, "task_branch": branch, "selection_rationale": "highest suitable priority", "objective": objective, "acceptance": acceptance}
}

func TestClaimAndEngineeringInOneTurn(t *testing.T) {
	repo, external, policy := testRepo(t)
	base, _ := gitOutput(repo, "rev-parse", "HEAD")
	agent := &fakeAgent{}
	agent.run = func(prompt string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		if !strings.Contains(prompt, "APP-002") || !strings.Contains(prompt, "AGENTS.md") || !strings.Contains(prompt, base) {
			t.Fatal("missing current selection context")
		}
		store, err := activework.Open(repo, external)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := store.Status()
		if err != nil || snapshot.Dispatch == nil || len(snapshot.Dispatch.Attempts) != 1 || snapshot.Dispatch.Attempts[0].Status != "reserved" {
			t.Fatalf("dispatch not reserved before turn: %+v, %v", snapshot, err)
		}
		workspace := filepath.Join(external, "workspaces", snapshot.RunID)
		if _, err := gitOutput(workspace, "switch", "-qc", "work/app-002"); err != nil {
			t.Fatal(err)
		}
		if err := invoke(t, handler, "claim-one", "claim_task", claimArgs("APP-002", "work/app-002")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workspace, "source.txt"), []byte("engineered\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := invoke(t, handler, "checkpoint-one", "save_checkpoint", map[string]string{"content": "Implemented source change; next: test and commit."}); err != nil {
			t.Fatal(err)
		}
		if err := invoke(t, handler, "claim-two", "claim_task", claimArgs("APP-003", "work/app-002")); err == nil {
			t.Fatal("second different claim accepted")
		}
		if err := invoke(t, handler, "claim-retry", "claim_task", claimArgs("APP-002", "work/app-002")); err != nil {
			t.Fatalf("same binding retry: %v", err)
		}
		return codexapp.TurnOutcome{Status: "completed", FinalText: "{\"disposition\":\"engineering\",\"summary\":\"implementation underway\"}"}, nil
	}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "engineering" || result.Snapshot.Hold != "" || result.TaskID != "APP-002" || result.Snapshot.TaskBranch != "work/app-002" || agent.starts != 1 || agent.turns != 1 {
		t.Fatalf("unexpected result: %+v, starts %d turns %d", result, agent.starts, agent.turns)
	}
	taskFiles, err := filepath.Glob(filepath.Join(external, "repos", "*", "runs", result.RunID, "evidence", "task.md"))
	if err != nil || len(taskFiles) != 1 {
		t.Fatalf("task evidence: %v, %v", taskFiles, err)
	}
	taskText, err := os.ReadFile(taskFiles[0])
	if err != nil || !strings.Contains(string(taskText), "Implemented source change") || !strings.Contains(string(taskText), "- Preserve the task boundary.") {
		t.Fatalf("checkpoint or agreed scope absent: %s, %v", taskText, err)
	}
	if got, _ := gitOutput(repo, "rev-parse", "HEAD"); got != base {
		t.Fatalf("parent moved: %s", got)
	}
	if err := requireClean(repo); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "source.txt"), []byte("parent later dirty\n"), 0600); err != nil {
		t.Fatal(err)
	}
	otherAgent := &fakeAgent{}
	active, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: otherAgent})
	if err != nil || active.Status != "active" || active.RunID != result.RunID || otherAgent.starts != 0 {
		t.Fatalf("existing active work was not checked first: %+v, %v", active, err)
	}
}

func TestDirtyParentRejectsBeforeAgent(t *testing.T) {
	repo, external, policy := testRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "source.txt"), []byte("dirty\n"), 0600); err != nil {
		t.Fatal(err)
	}
	agent := &fakeAgent{}
	if _, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent}); err == nil || !strings.Contains(err.Error(), "changes") {
		t.Fatalf("expected dirty prerequisite, got %v", err)
	}
	if agent.starts != 0 || agent.turns != 0 {
		t.Fatal("agent dispatched for dirty parent")
	}
}

func TestDirtyPreclaimAndNormalNoWork(t *testing.T) {
	repo, external, policy := testRepo(t)
	agent := &fakeAgent{}
	agent.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		store, err := activework.Open(repo, external)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, _ := store.Status()
		workspace := filepath.Join(external, "workspaces", snapshot.RunID)
		if _, err := gitOutput(workspace, "switch", "-qc", "work/app-002"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workspace, "source.txt"), []byte("premature\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := invoke(t, handler, "claim", "claim_task", claimArgs("APP-002", "work/app-002")); err == nil {
			t.Fatal("dirty preclaim accepted")
		}
		if err := os.WriteFile(filepath.Join(workspace, "source.txt"), []byte("baseline\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := gitOutput(workspace, "switch", "--detach"); err != nil {
			t.Fatal(err)
		}
		if _, err := gitOutput(repo, "branch", "-D", "work/app-002"); err != nil {
			t.Fatal(err)
		}
		return codexapp.TurnOutcome{Status: "completed", FinalText: "{\"disposition\":\"no_suitable_work\",\"summary\":\"No suitable ready task after inspection\"}"}, nil
	}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent})
	if err != nil || result.Status != "no_suitable_work" {
		t.Fatalf("no-work result: %+v, %v", result, err)
	}
	store, err := activework.Open(repo, external)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Status()
	if err != nil || snapshot.Active {
		t.Fatalf("active work not retired: %+v, %v", snapshot, err)
	}
	if _, err := os.Stat(result.Workspace); !os.IsNotExist(err) {
		t.Fatalf("no-work checkout remains: %v", err)
	}
}

func TestPreexistingBranchCannotBeClaimed(t *testing.T) {
	repo, external, policy := testRepo(t)
	if _, err := gitOutput(repo, "branch", "work/app-002"); err != nil {
		t.Fatal(err)
	}
	agent := &fakeAgent{}
	agent.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		store, err := activework.Open(repo, external)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, _ := store.Status()
		workspace := filepath.Join(external, "workspaces", snapshot.RunID)
		if _, err := gitOutput(workspace, "switch", "work/app-002"); err != nil {
			t.Fatal(err)
		}
		if err := invoke(t, handler, "existing-branch", "claim_task", claimArgs("APP-002", "work/app-002")); err == nil || !strings.Contains(err.Error(), "existed before") {
			t.Fatalf("preexisting branch claim: %v", err)
		}
		if _, err := gitOutput(workspace, "switch", "--detach"); err != nil {
			t.Fatal(err)
		}
		return codexapp.TurnOutcome{Status: "completed", FinalText: "{\"disposition\":\"no_suitable_work\",\"summary\":\"Existing branch prevents a fresh task claim\"}"}, nil
	}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent})
	if err != nil || result.Status != "no_suitable_work" {
		t.Fatalf("result: %+v, %v", result, err)
	}
}

func TestDurableClaimWithoutReceiptCanBeAcknowledgedOnRetry(t *testing.T) {
	repo, external, _ := testRepo(t)
	store, err := activework.Open(repo, external)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := inspectParent(repo)
	if err != nil {
		t.Fatal(err)
	}
	project, err := readProject(repo)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	runID, err := owner.BeginSelection(activework.SelectionSpec{ParentBranch: parent.Branch, ParentCommit: parent.Commit})
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := prepareWorkspace(store, runID, parent.Commit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(workspace, "switch", "-qc", "work/app-002"); err != nil {
		t.Fatal(err)
	}
	args := claimArguments{TaskID: "APP-002", TaskBranch: "work/app-002", SelectionRationale: "highest suitable priority", Objective: "implement the ready task.", Acceptance: "- Preserve the task boundary."}
	spec := activework.StartSpec{TaskID: args.TaskID, TaskBranch: args.TaskBranch, ParentBranch: parent.Branch, ParentCommit: parent.Commit, SelectionRationale: args.SelectionRationale, Objective: args.Objective, Acceptance: args.Acceptance}
	if err := owner.Claim(spec); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner, err = store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	session := &hostSession{owner: owner, store: store, project: project, workspace: workspace, parent: parent, existingBranches: map[string]bool{"main": true}}
	changed := args
	changed.SelectionRationale = "different rationale"
	if err := session.claim("original-call", changed); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("changed durable claim accepted: %v", err)
	}
	for _, callID := range []string{"original-call", "new-call"} {
		if err := session.claim(callID, args); err != nil {
			t.Fatalf("retry %s: %v", callID, err)
		}
		receipt, name, err := callReceipt("claim", callID, args)
		if err != nil {
			t.Fatal(err)
		}
		found, err := owner.CheckReceipt(name, receipt)
		if err != nil || !found {
			t.Fatalf("receipt %s missing: %v", callID, err)
		}
	}
	if err := os.WriteFile(filepath.Join(workspace, "source.txt"), []byte("edited before a fresh acknowledgment\n"), 0600); err != nil {
		t.Fatal(err)
	}
	freshSession := &hostSession{owner: owner, store: store, project: project, workspace: workspace, parent: parent, existingBranches: map[string]bool{"main": true}}
	if err := freshSession.claim("original-call", args); err == nil || !strings.Contains(err.Error(), "changed before claim") {
		t.Fatalf("dirty checkout accepted on receipt retry: %v", err)
	}
	if session.taskID != "APP-002" || session.taskBranch != "work/app-002" {
		t.Fatalf("binding changed: %+v", session)
	}
	snapshot, err := store.Status()
	if err != nil || snapshot.TaskID != "APP-002" || snapshot.TaskBranch != "work/app-002" {
		t.Fatalf("durable binding changed: %+v, %v", snapshot, err)
	}
}
