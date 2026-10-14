package selection

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
)

func readyEvidence() ReadyCheckpoint {
	return ReadyCheckpoint{
		TaskID: "APP-002", Plan: "Implement the ready task.", Progress: "Added feature.go and reconciled the delivery docs.",
		Criteria: []CriterionEvidence{{Criterion: "Preserve the task boundary.", Result: "passed", Evidence: "feature.go and the task-specific probe pass."}},
		Checks: []CheckEvidence{
			{Command: "gofmt -w feature.go", Source: "AGENTS.md", Result: "passed", Evidence: "gofmt -l feature.go produced no output."},
			{Command: "go test ./...", Source: "AGENTS.md", Result: "passed", Evidence: "All packages passed."},
			{Command: "go vet ./...", Source: "AGENTS.md", Result: "passed", Evidence: "No diagnostics."},
			{Command: "go build ./cmd/concoct", Source: "AGENTS.md", Result: "passed", Evidence: "Build exited zero."},
			{Command: "git diff --check", Source: "AGENTS.md", Result: "passed", Evidence: "No whitespace errors."},
		},
		SelfReview: "Checked scope, source and docs.", NextAction: "Independent review.",
		Deliverables: []string{"feature.go", "feature.out"}, GeneratedOutputs: []string{"concoct"},
	}
}

func readyAgent(t *testing.T, repo, external string, evidence ReadyCheckpoint, ordinaryCommit, reconcile bool) *fakeAgent {
	t.Helper()
	agent := &fakeAgent{}
	agent.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		store, err := activework.Open(repo, external)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := store.Status()
		if err != nil {
			t.Fatal(err)
		}
		workspace := filepath.Join(external, "workspaces", snapshot.RunID)
		if _, err := gitOutput(workspace, "switch", "-qc", "work/app-002"); err != nil {
			t.Fatal(err)
		}
		if err := invoke(t, handler, "claim-ready", "claim_task", claimArgs("APP-002", "work/app-002")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workspace, "feature.go"), []byte("package feature\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workspace, "feature.out"), []byte("deliverable source output\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workspace, "concoct"), []byte("generated binary"), 0600); err != nil {
			t.Fatal(err)
		}
		if reconcile {
			roadmapText, err := os.ReadFile(filepath.Join(workspace, "roadmap.md"))
			if err != nil {
				t.Fatal(err)
			}
			updated := strings.Replace(string(roadmapText), "## APP-002 — Ready\n- Kind: task\n- Status: ready", "## APP-002 — Ready\n- Kind: task\n- Status: delivered", 1)
			if err := os.WriteFile(filepath.Join(workspace, "roadmap.md"), []byte(updated), 0600); err != nil {
				t.Fatal(err)
			}
			file, err := os.OpenFile(filepath.Join(workspace, "capabilities.md"), os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.WriteString("\n## APP-002 — Implemented ready task\n"); err != nil {
				t.Fatal(err)
			}
			file.Close()
		}
		if ordinaryCommit {
			if _, err := gitOutput(workspace, "add", "-f", "feature.go", "feature.out", "roadmap.md", "capabilities.md"); err != nil {
				t.Fatal(err)
			}
			if _, err := gitOutput(workspace, "-c", "user.name=Agent", "-c", "user.email=agent@example.com", "commit", "-qm", "agent implementation"); err != nil {
				t.Fatal(err)
			}
		}
		checkpoint, err := json.Marshal(evidence)
		if err != nil {
			t.Fatal(err)
		}
		if err := invoke(t, handler, "checkpoint-ready", "save_checkpoint", map[string]string{"content": string(checkpoint)}); err != nil {
			t.Fatal(err)
		}
		return codexapp.TurnOutcome{ThreadID: "fresh-thread", TurnID: "one-turn", Status: "completed", FinalText: "{\"disposition\":\"ready_for_review\",\"summary\":\"ready for independent review\"}"}, nil
	}
	return agent
}

func TestReadyFreezeCapturesUntrackedSourceAndOrdinaryCommit(t *testing.T) {
	for _, ordinaryCommit := range []bool{false, true} {
		t.Run(map[bool]string{false: "mechanical", true: "agent-commit"}[ordinaryCommit], func(t *testing.T) {
			repo, external, policy := testRepo(t)
			base, _ := gitOutput(repo, "rev-parse", "HEAD")
			agent := readyAgent(t, repo, external, readyEvidence(), ordinaryCommit, true)
			seen := false
			agent.event = &codexapp.Event{Kind: "progress"}
			result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent, OnEvent: func(event codexapp.Event) { seen = event.Kind == "progress" }})
			if err != nil {
				t.Fatal(err)
			}
			if !seen || result.Status != "ready_for_review" || result.Snapshot.Candidate == nil || result.Snapshot.Phase != "ready-for-review" {
				t.Fatalf("missing progress or candidate: %+v", result)
			}
			candidate := result.Snapshot.Candidate
			if candidate.BaselineCommit != base || candidate.Commit == base || candidate.TurnID != "one-turn" || !slices.Contains(candidate.ChangedPaths, "feature.go") || !slices.Contains(candidate.ChangedPaths, "feature.out") || slices.Contains(candidate.ChangedPaths, "concoct") {
				t.Fatalf("candidate identity or paths: %+v", candidate)
			}
			baseTree, _ := gitOutput(repo, "rev-parse", base+"^{tree}")
			candidateTree, _ := gitOutput(result.Workspace, "rev-parse", candidate.Commit+"^{tree}")
			diff, err := gitBytes(result.Workspace, "diff", "--binary", "--no-ext-diff", "--no-renames", base, candidate.Commit)
			if err != nil || candidate.BaselineTree != baseTree || candidate.Tree != candidateTree || candidate.DiffSHA256 != sha256Hex(diff) {
				t.Fatalf("candidate source identity mismatch: %+v, %v", candidate, err)
			}
			got, err := gitOutput(result.Workspace, "rev-parse", "HEAD")
			if err != nil || got != candidate.Commit {
				t.Fatalf("candidate branch: %s, %v", got, err)
			}
			if _, err := gitOutput(result.Workspace, "cat-file", "-e", candidate.Commit+":feature.go"); err != nil {
				t.Fatalf("untracked source absent from commit: %v", err)
			}
			if _, err := gitOutput(result.Workspace, "cat-file", "-e", candidate.Commit+":feature.out"); err != nil {
				t.Fatalf("ignored deliverable absent from commit: %v", err)
			}
			if _, err := gitOutput(result.Workspace, "cat-file", "-e", candidate.Commit+":concoct"); err == nil {
				t.Fatal("explicit generated output entered candidate")
			}
			if ordinaryCommit {
				subject, _ := gitOutput(result.Workspace, "log", "-1", "--format=%s")
				if subject != "agent implementation" {
					t.Fatalf("ordinary agent commit was replaced: %s", subject)
				}
			} else {
				subject, _ := gitOutput(result.Workspace, "log", "-1", "--format=%s")
				if subject != "Freeze APP-002 candidate" {
					t.Fatalf("mechanical finalization missing: %s", subject)
				}
			}
			if got, _ := gitOutput(repo, "rev-parse", "HEAD"); got != base {
				t.Fatal("parent branch moved")
			}
			if err := requireClean(repo); err != nil {
				t.Fatal(err)
			}
			store, err := activework.Open(repo, external)
			if err != nil {
				t.Fatal(err)
			}
			owner, err := store.Acquire()
			if err != nil {
				t.Fatal(err)
			}
			if err := owner.SaveCheckpoint("changed after freeze"); err == nil {
				t.Fatal("frozen checkpoint was mutable")
			}
			owner.Close()
		})
	}
}

func TestReadyRejectsIncompleteChecksAndReconciliation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		evidence  ReadyCheckpoint
		reconcile bool
		want      string
		extra     []RequiredCheck
	}{
		{name: "missing mandatory check", evidence: func() ReadyCheckpoint { e := readyEvidence(); e.Checks = e.Checks[:3]; return e }(), reconcile: true, want: "mandatory check"},
		{name: "configured check", evidence: readyEvidence(), reconcile: true, want: "mandatory check", extra: []RequiredCheck{{Command: "make lint", Source: "AGENTS.md"}}},
		{name: "unaccepted unavailable check", evidence: func() ReadyCheckpoint {
			e := readyEvidence()
			e.Checks[2].Result = "unavailable"
			e.Checks[2].Decision = "proposed waiver"
			return e
		}(), reconcile: true, want: "operator acceptance"},
		{name: "missing docs", evidence: readyEvidence(), reconcile: false, want: "does not mark"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, external, policy := testRepo(t)
			agent := readyAgent(t, repo, external, tc.evidence, false, tc.reconcile)
			result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent, RequiredChecks: tc.extra})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %s, got %+v, %v", tc.want, result, err)
			}
			if result.Snapshot.Candidate != nil || result.Snapshot.Phase != "engineering" {
				t.Fatalf("invalid readiness froze candidate: %+v", result.Snapshot)
			}
			if _, err := os.Stat(filepath.Join(result.Workspace, "feature.go")); err != nil {
				t.Fatalf("unfinished work not retained: %v", err)
			}
		})
	}
}

func TestReadyRequiresIdleToolsAndResolvedCallbacks(t *testing.T) {
	for _, tc := range []struct {
		name string
		idle bool
	}{
		{"active tool", false}, {"uncertain callback", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, external, policy := testRepo(t)
			agent := readyAgent(t, repo, external, readyEvidence(), false, true)
			if !tc.idle {
				agent.idle = func() error { return errors.New("background terminal active") }
			} else {
				original := agent.run
				agent.run = func(prompt string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
					outcome, _ := original(prompt, handler)
					return outcome, codexapp.ErrUncertainCallback
				}
			}
			result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent})
			if err == nil || result.Snapshot.Candidate != nil {
				t.Fatalf("uncertain work froze candidate: %+v, %v", result, err)
			}
			store, _ := activework.Open(repo, external)
			snapshot, statusErr := store.Status()
			if statusErr != nil || !snapshot.Active || snapshot.Candidate != nil {
				t.Fatalf("active work not retained: %+v, %v", snapshot, statusErr)
			}
		})
	}
}

func TestMalformedReadyResultRetainsBoundedRecovery(t *testing.T) {
	repo, external, policy := testRepo(t)
	agent := readyAgent(t, repo, external, readyEvidence(), false, true)
	original := agent.run
	agent.run = func(prompt string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		outcome, err := original(prompt, handler)
		outcome.FinalText = "ready"
		return outcome, err
	}
	seen := false
	agent.event = &codexapp.Event{Kind: "progress"}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent, OnEvent: func(event codexapp.Event) { seen = event.Kind == "progress" }})
	if err == nil || !seen || result.Status != "recovery_required" || result.Snapshot.Hold != "recovery-required" || result.Snapshot.Candidate != nil {
		t.Fatalf("malformed result advanced or hid progress: %+v, %v", result, err)
	}
	if result.Snapshot.Dispatch == nil || len(result.Snapshot.Dispatch.Attempts) != 1 || result.Snapshot.Dispatch.Attempts[0].Status != "completed" {
		t.Fatalf("recovery lost bounded dispatch consumption: %+v", result.Snapshot.Dispatch)
	}
}

func TestExplicitlyAcceptedUnavailableCheck(t *testing.T) {
	repo, external, policy := testRepo(t)
	evidence := readyEvidence()
	evidence.Checks[2].Result = "unavailable"
	evidence.Checks[2].Evidence = "Local vet command unavailable in this fixture."
	evidence.Checks[2].Decision = "Operator accepted the unavailable vet check for this candidate."
	agent := readyAgent(t, repo, external, evidence, false, true)
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: agent, AcceptedUnavailable: []string{"go vet ./..."}})
	if err != nil || result.Status != "ready_for_review" || result.Snapshot.Candidate == nil {
		t.Fatalf("explicit unavailable-check acceptance: %+v, %v", result, err)
	}
}

func TestMechanicalCommitRejectsFilteredBytes(t *testing.T) {
	repo, _, _ := testRepo(t)
	if _, err := gitOutput(repo, "config", "core.autocrlf", "true"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "feature.go"), []byte("package feature\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := stageSource(repo, []string{"feature.go"}, []string{"feature.go"}); err == nil || !strings.Contains(err.Error(), "differs from verified worktree") {
		t.Fatalf("filtered bytes entered candidate: %v", err)
	}
}

func TestSelectionPromptIncludesConfiguredFreezeChecks(t *testing.T) {
	repo, _, _ := testRepo(t)
	project, err := readProject(repo)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := inspectParent(repo)
	if err != nil {
		t.Fatal(err)
	}
	prompt := selectionPrompt(project, parent, repo, []RequiredCheck{{Command: "make lint", Source: "Makefile"}}, []string{"build/"})
	if !strings.Contains(prompt, "make lint (source: Makefile)") || !strings.Contains(prompt, "build/") || !strings.Contains(prompt, "GeneratedOutputs") {
		t.Fatal("configured readiness or generated-output contract absent from reagent context")
	}
}
