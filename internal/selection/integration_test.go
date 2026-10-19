package selection

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/integration"
)

func approvedIntegration(t *testing.T) (string, string, integration.Config, activework.Snapshot) {
	t.Helper()
	repo, external, policy := testRepo(t)
	reagent := readyAgent(t, repo, external, readyEvidence(), false, true)
	reviewer := &fakeReviewer{}
	reviewer.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		reviewCheckpoint(t, handler, "Exact source and acceptance criteria verified.")
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "completed", FinalText: `{"disposition":"approved","summary":"approved"}`}, nil
	}
	result, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: reagent, Reviewer: reviewer})
	if err != nil || result.Status != "approved" || result.Snapshot.Review == nil {
		t.Fatalf("approved fixture: %+v, %v", result, err)
	}
	return repo, external, integration.Config{Repository: repo, Root: external, Reagent: reagent, Reviewer: reviewer}, result.Snapshot
}

func TestExactAcceptanceFastForwardAndReplay(t *testing.T) {
	repo, _, cfg, snapshot := approvedIntegration(t)
	acceptance, err := integration.Offer(cfg)
	if err != nil || acceptance.TaskID != snapshot.TaskID || acceptance.Candidate != snapshot.Candidate.Commit || acceptance.CandidateTree != snapshot.Candidate.Tree || acceptance.ScopeSHA256 != snapshot.Review.ScopeSHA256 || acceptance.ExpectedParent != snapshot.ParentCommit || acceptance.ReviewNumber != snapshot.Review.Number || acceptance.ReviewAttemptID != snapshot.Review.AttemptID {
		t.Fatalf("acceptance binding: %+v, %v", acceptance, err)
	}
	again, err := integration.Offer(cfg)
	if err != nil || again != acceptance {
		t.Fatalf("acceptance ID was reissued: %+v, %v", again, err)
	}
	if _, err := integration.Apply(context.Background(), cfg, snapshot.TaskID); err == nil {
		t.Fatal("task ID approved substitute work")
	}
	if head, _ := gitOutput(repo, "rev-parse", "HEAD"); head != snapshot.ParentCommit {
		t.Fatal("wrong ID changed target")
	}
	result, err := integration.Apply(context.Background(), cfg, acceptance.ID)
	if err != nil || result.Status != "integrated" {
		t.Fatalf("integration: %+v, %v", result, err)
	}
	if head, _ := gitOutput(repo, "rev-parse", "HEAD"); head != acceptance.Candidate {
		t.Fatalf("wrong integrated commit: %s", head)
	}
	if tree, _ := gitOutput(repo, "rev-parse", "HEAD^{tree}"); tree != acceptance.CandidateTree {
		t.Fatal("integrated tree differs")
	}
	if branch, _ := gitOutput(repo, "branch", "--show-current"); branch != acceptance.TargetBranch {
		t.Fatal("parent checkout was not reattached")
	}
	if err := requireClean(repo); err != nil {
		t.Fatal(err)
	}
	continued, err := Continue(context.Background(), Config{Repository: repo, Root: cfg.Root})
	if err != nil || continued.Status != "integrated" || continued.Snapshot.Hold != "" {
		t.Fatalf("controller mistook integrated parent for drift: %+v, %v", continued, err)
	}
	store, _ := activework.Open(repo, cfg.Root)
	status, err := store.Status()
	if err != nil || status.Integration == nil || status.Integration.Status != "integrated" {
		t.Fatalf("completion handoff missing: %+v, %v", status, err)
	}
	result, err = integration.Apply(context.Background(), cfg, acceptance.ID)
	if err != nil || result.Status != "integrated" {
		t.Fatalf("successful integration replayed: %+v, %v", result, err)
	}
	cfg.Reagent, cfg.Reviewer = nil, nil
	result, err = integration.Apply(context.Background(), cfg, acceptance.ID)
	if err != nil || result.Status != "integrated" {
		t.Fatalf("recorded integration requires provider: %+v, %v", result, err)
	}
	if remote, _ := gitOutput(repo, "remote"); remote != "" {
		t.Fatalf("test unexpectedly has publication target: %s", remote)
	}
}

func TestIntegrationPreservesIgnoredParentFileOnRetry(t *testing.T) {
	repo, external, policy := testRepo(t)
	ignore := filepath.Join(repo, ".gitignore")
	if err := os.WriteFile(ignore, []byte("/concoct\n*.out\nfeature.go\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(repo, "add", ".gitignore"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "ignore future candidate path"); err != nil {
		t.Fatal(err)
	}
	reagent := readyAgent(t, repo, external, readyEvidence(), false, true)
	originalRun := reagent.run
	reagent.run = func(prompt string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		outcome, err := originalRun(prompt, handler)
		if err != nil {
			return outcome, err
		}
		store, err := activework.Open(repo, external)
		if err != nil {
			return outcome, err
		}
		snapshot, err := store.Status()
		if err != nil {
			return outcome, err
		}
		workspace := filepath.Join(external, "workspaces", snapshot.RunID)
		_, err = gitOutput(workspace, "add", "-f", "feature.go")
		return outcome, err
	}
	reviewer := &fakeReviewer{}
	reviewer.run = func(_ string, handler codexapp.ToolHandler) (codexapp.TurnOutcome, error) {
		reviewCheckpoint(t, handler, "Exact source and acceptance criteria verified.")
		return codexapp.TurnOutcome{ThreadID: "reviewer-thread", TurnID: "review-turn", Status: "completed", FinalText: `{"disposition":"approved","summary":"approved"}`}, nil
	}
	selected, err := Execute(context.Background(), Config{Repository: repo, Root: external, Policy: policy, Agent: reagent, Reviewer: reviewer})
	if err != nil || selected.Status != "approved" {
		t.Fatalf("approved fixture: %+v, %v", selected, err)
	}
	cfg := integration.Config{Repository: repo, Root: external, Reagent: reagent, Reviewer: reviewer}
	acceptance, err := integration.Offer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	parentFile := filepath.Join(repo, "feature.go")
	const externalData = "external data\n"
	if err := os.WriteFile(parentFile, []byte(externalData), 0600); err != nil {
		t.Fatal(err)
	}
	if status, err := gitOutput(repo, "status", "--porcelain=v1", "--untracked-files=all"); err != nil || status != "" {
		t.Fatalf("parent must look clean to the existing guard: %q, %v", status, err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		result, err := integration.Apply(context.Background(), cfg, acceptance.ID)
		if err == nil || result.Status != "not_integrated" {
			t.Fatalf("attempt %d did not hold the ignored collision: %+v, %v", attempt, result, err)
		}
		if data, err := os.ReadFile(parentFile); err != nil || string(data) != externalData {
			t.Fatalf("attempt %d changed external file: %q, %v", attempt, data, err)
		}
		if target, err := gitOutput(repo, "rev-parse", "refs/heads/main"); err != nil || target != acceptance.ExpectedParent {
			t.Fatalf("attempt %d advanced target: %s, %v", attempt, target, err)
		}
		if branch, err := gitOutput(repo, "branch", "--show-current"); err != nil || branch != "main" {
			t.Fatalf("attempt %d changed parent checkout: %s, %v", attempt, branch, err)
		}
		if status, err := gitOutput(repo, "status", "--porcelain=v1", "--untracked-files=all"); err != nil || status != "" {
			t.Fatalf("attempt %d left partial checkout changes: %q, %v", attempt, status, err)
		}
	}
}

func TestIntegrationRejectsTargetCandidateAndDirtyDrift(t *testing.T) {
	for _, drift := range []string{"target", "candidate", "candidate-ref", "dirty", "review", "scope", "assessment", "idle"} {
		t.Run(drift, func(t *testing.T) {
			repo, external, cfg, snapshot := approvedIntegration(t)
			acceptance, err := integration.Offer(cfg)
			if err != nil {
				t.Fatal(err)
			}
			switch drift {
			case "target":
				if _, err := gitOutput(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "outside target change"); err != nil {
					t.Fatal(err)
				}
			case "candidate":
				workspace := filepath.Join(external, "workspaces", snapshot.RunID)
				if err := os.WriteFile(filepath.Join(workspace, "source.txt"), []byte("outside change\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "candidate-ref":
				competing, err := gitOutput(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit-tree", snapshot.Candidate.Tree, "-p", snapshot.Candidate.Commit, "-m", "outside candidate change")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := gitOutput(repo, "update-ref", "refs/heads/"+snapshot.TaskBranch, competing, snapshot.Candidate.Commit); err != nil {
					t.Fatal(err)
				}
			case "dirty":
				if err := os.WriteFile(filepath.Join(repo, "outside.txt"), []byte("outside change\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "review":
				checkout := filepath.Join(external, "workspaces", snapshot.RunID+"-review")
				if err := os.WriteFile(filepath.Join(checkout, "source.txt"), []byte("review changed\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "scope", "assessment":
				name := "task.md"
				if drift == "assessment" {
					name = "review-01.md"
				}
				files, err := filepath.Glob(filepath.Join(external, "repos", "*", "runs", snapshot.RunID, "evidence", name))
				if err != nil || len(files) != 1 {
					t.Fatalf("bound evidence: %v, %v", files, err)
				}
				file, err := os.OpenFile(files[0], os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.WriteString("external alteration\n"); err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			case "idle":
				cfg.Reagent = idleFailure{err: errors.New("background process active")}
			}
			if result, err := integration.Apply(context.Background(), cfg, acceptance.ID); err == nil {
				t.Fatalf("%s drift integrated: %+v", drift, result)
			}
			if got, _ := gitOutput(repo, "rev-parse", "refs/heads/main"); got == acceptance.Candidate {
				t.Fatalf("%s drift advanced target", drift)
			}
		})
	}
}

type idleFailure struct{ err error }

func (i idleFailure) Idle(context.Context, string) error { return i.err }

func TestIntegrationInterruptedTransitions(t *testing.T) {
	for _, stage := range []struct{ name, status string }{{"intent", "not_integrated"}, {"checkout-prepared", "not_integrated"}, {"ref-advanced", "uncertain"}, {"checkout-attached", "integrated"}} {
		t.Run(stage.name, func(t *testing.T) {
			repo, _, cfg, _ := approvedIntegration(t)
			acceptance, err := integration.Offer(cfg)
			if err != nil {
				t.Fatal(err)
			}
			cfg.After = func(got string) error {
				if got == stage.name {
					return errors.New("simulated interruption")
				}
				return nil
			}
			first, err := integration.Apply(context.Background(), cfg, acceptance.ID)
			if err == nil || first.Status != stage.status {
				t.Fatalf("interruption classification: %+v, %v", first, err)
			}
			cfg.After = nil
			if stage.name == "checkout-attached" {
				cfg.Reagent, cfg.Reviewer = nil, nil
				store, _ := activework.Open(repo, cfg.Root)
				status, _ := store.Status()
				workspace := filepath.Join(cfg.Root, "workspaces", status.RunID)
				if err := os.WriteFile(filepath.Join(workspace, "source.txt"), []byte("later task checkout change\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := integration.Apply(context.Background(), cfg, acceptance.ID)
			if err != nil || result.Status != "integrated" {
				t.Fatalf("interrupted integration did not reconcile: %+v, %v", result, err)
			}
			if err := requireClean(repo); err != nil {
				t.Fatal(err)
			}
			if head, _ := gitOutput(repo, "rev-parse", "HEAD"); head != acceptance.Candidate {
				t.Fatal("reconciled target differs")
			}
		})
	}
}

func TestIntegrationConcurrentTargetRefChangeIsHeld(t *testing.T) {
	repo, _, cfg, snapshot := approvedIntegration(t)
	acceptance, err := integration.Offer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	competing, err := gitOutput(repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit-tree", snapshot.Candidate.BaselineTree, "-p", snapshot.ParentCommit, "-m", "external ref change")
	if err != nil {
		t.Fatal(err)
	}
	cfg.After = func(stage string) error {
		if stage == "checkout-prepared" {
			_, err := gitOutput(repo, "update-ref", "refs/heads/main", competing, snapshot.ParentCommit)
			return err
		}
		return nil
	}
	result, err := integration.Apply(context.Background(), cfg, acceptance.ID)
	if err == nil || result.Status != "conflicting" || !strings.Contains(result.Detail, competing) {
		t.Fatalf("concurrent ref change was not held: %+v, %v", result, err)
	}
	if target, _ := gitOutput(repo, "rev-parse", "refs/heads/main"); target != competing {
		t.Fatalf("external target overwritten: %s", target)
	}
}

func TestIntegrationDoesNotOverwriteInterruptedCheckoutChange(t *testing.T) {
	repo, _, cfg, _ := approvedIntegration(t)
	acceptance, err := integration.Offer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.After = func(stage string) error {
		if stage == "ref-advanced" {
			return errors.New("simulated interruption")
		}
		return nil
	}
	result, err := integration.Apply(context.Background(), cfg, acceptance.ID)
	if err == nil || result.Status != "uncertain" {
		t.Fatalf("expected detached advanced ref: %+v, %v", result, err)
	}
	if err := os.WriteFile(filepath.Join(repo, "outside.txt"), []byte("external work\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg.After = nil
	result, err = integration.Apply(context.Background(), cfg, acceptance.ID)
	if err == nil || result.Status != "conflicting" {
		t.Fatalf("dirty interrupted checkout was overwritten: %+v, %v", result, err)
	}
	if body, err := os.ReadFile(filepath.Join(repo, "outside.txt")); err != nil || string(body) != "external work\n" {
		t.Fatalf("external work changed: %q, %v", body, err)
	}
}
