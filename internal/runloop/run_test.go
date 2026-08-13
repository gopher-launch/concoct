package runloop

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/config"
	"github.com/gopher-launch/concoct/internal/execution"
	"github.com/gopher-launch/concoct/internal/orchestration"
	"github.com/gopher-launch/concoct/internal/project"
	"github.com/gopher-launch/concoct/internal/runstate"
	"github.com/gopher-launch/concoct/internal/workflow"
)

func TestReadyRunPersistsProposalAndRejectsDriftedApproval(t *testing.T) {
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	roadmap := filepath.Join(root, ".concoct", "roadmap.md")
	file, err := os.OpenFile(roadmap, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\n## APP-001 — Proposed work\n\n- Status: `planned`\n- Depends on: `none`\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	installRunCodex(t, "completed", "plan", "concoct plan APP-001", "selected eligible work", "", "")

	summary, err := Run(context.Background(), root, Options{Policy: config.RunOverrides{MaxActions: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Gate != "next" || summary.Actions != 1 || summary.ActionLimit != 1 || len(summary.Steps) != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	gate, err := runstate.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if gate.Selection != "APP-001" || gate.Action != "task-planning" {
		t.Fatalf("gate = %#v", gate)
	}
	branchBefore := gitOutput(t, root, "branch", "--show-current")
	file, err = os.OpenFile(roadmap, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString("\nMaterial drift.\n")
	_ = file.Close()
	if _, err := Run(context.Background(), root, Options{Approve: "next"}); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("drifted approval error = %v", err)
	}
	if branch := gitOutput(t, root, "branch", "--show-current"); branch != branchBefore {
		t.Fatalf("stale approval changed branch from %s to %s", branchBefore, branch)
	}
	if _, err := os.Stat(runstate.Path(root)); !os.IsNotExist(err) {
		t.Fatalf("stale gate was not invalidated: %v", err)
	}
}

func TestReadyRunStopsOnAcceptedInterventionWithoutRetry(t *testing.T) {
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	installRunCodex(t, "failed-recoverable", "", "", "", "human-product-decision", "Product Owner decides or clarifies the next work item")
	summary, err := Run(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Actions != 1 || len(summary.Steps) != 1 || summary.Steps[0].Outcome != "failed-recoverable" || !strings.Contains(summary.Stop, "accepted failed-recoverable") {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestCoordinatorStopsForEveryNonCompletionClassAndProcessFailure(t *testing.T) {
	for _, test := range []struct {
		class string
		kind  string
		next  string
	}{
		{"blocked", "human-product-decision", "Product Owner decides or clarifies the next work item"},
		{"decision-required", "human-product-decision", "Product Owner decides or clarifies the next work item"},
		{"failed-recoverable", "human-product-decision", "Product Owner decides or clarifies the next work item"},
		{"failed-terminal", "human-product-decision", "Product Owner decides or clarifies the next work item"},
	} {
		t.Run(test.class, func(t *testing.T) {
			parent := t.TempDir()
			if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(parent, "demo")
			installRunCodex(t, test.class, "", "", "", test.kind, test.next)
			summary, err := Run(context.Background(), root, Options{})
			if err != nil || summary.Actions != 1 || !strings.Contains(summary.Stop, "accepted "+test.class) || summary.Recommendation != test.next {
				t.Fatalf("summary=%#v err=%v", summary, err)
			}
		})
	}
	t.Run("process failure", func(t *testing.T) {
		parent := t.TempDir()
		if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(parent, "demo")
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "codex"), []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		summary, err := Run(context.Background(), root, Options{})
		if err == nil || summary.Actions != 1 || summary.Recommendation != "concoct run" {
			t.Fatalf("summary=%#v err=%v", summary, err)
		}
	})
	t.Run("cancelled before action", func(t *testing.T) {
		parent := t.TempDir()
		if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		summary, err := Run(ctx, filepath.Join(parent, "demo"), Options{})
		if err == nil || summary.Actions != 0 || summary.Stop != "run cancelled" {
			t.Fatalf("summary=%#v err=%v", summary, err)
		}
	})
}

func TestApprovedSelectionUsesPlanningBranchAndStopsAtPlanGate(t *testing.T) {
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	gitOutput(t, root, "config", "user.email", "test@example.com")
	gitOutput(t, root, "config", "user.name", "Test")
	roadmap := filepath.Join(root, ".concoct", "roadmap.md")
	file, err := os.OpenFile(roadmap, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString("\n## APP-001 — Proposed work\n\n- Status: `planned`\n- Depends on: `none`\n")
	_ = file.Close()
	gitOutput(t, root, "add", "-A")
	gitOutput(t, root, "commit", "-qm", "bootstrap")
	base := gitOutput(t, root, "rev-parse", "HEAD")
	trunk := gitOutput(t, root, "branch", "--show-current")
	installPlanningCodex(t)

	first, err := Run(context.Background(), root, Options{})
	if err != nil || first.Gate != "next" {
		t.Fatalf("proposal summary=%#v err=%v", first, err)
	}
	second, err := Run(context.Background(), root, Options{Approve: "next"})
	if err != nil {
		t.Fatal(err)
	}
	if second.Actions != 1 || second.Steps[0].Action != "task-planning" || second.Gate != "plan" || second.State != "planned" {
		t.Fatalf("planning summary = %#v", second)
	}
	if branch := gitOutput(t, root, "branch", "--show-current"); branch != "concoct/app-001-proposed-work" {
		t.Fatalf("task branch = %s", branch)
	}
	plan, err := os.ReadFile(filepath.Join(root, ".concoct", "current", "task-plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"trunk: " + trunk, "task-branch: concoct/app-001-proposed-work", "base: " + base} {
		if !strings.Contains(string(plan), want) {
			t.Fatalf("plan lacks %q:\n%s", want, plan)
		}
	}
}

func TestApprovedPlanContinuesIntoIndependentFreshReview(t *testing.T) {
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".concoct", "roadmap.md"), []byte("---\nversion: 1\nproject: demo\nupdated: 2026-08-11\n---\n# Roadmap\n\n## APP-001 — Demo\n\n- Status: `active`\n- Depends on: `none`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := "---\nid: APP-001\ntitle: Demo\nroadmap-id: APP-001\nstatus: planned\ncreated: 2026-08-11\nupdated: 2026-08-11\ncapability-impact:\n  type: none\n  rationale: No capability impact.\n---\n# Task Plan\n"
	if err := os.WriteFile(filepath.Join(root, ".concoct", "current", "task-plan.md"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".concoct", "current", "notes.md"), []byte("# Notes\n\nPlanned.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gateSummary, err := Run(context.Background(), root, Options{})
	if err != nil || gateSummary.Gate != "plan" {
		t.Fatalf("plan gate summary=%#v err=%v", gateSummary, err)
	}
	installDevelopmentReviewCodex(t)
	summary, err := Run(context.Background(), root, Options{Approve: "plan"})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Actions != 2 || len(summary.Steps) != 2 || summary.Steps[0].Action != "development" || summary.Steps[1].Action != "independent-review" {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.Steps[0].Invocation == summary.Steps[1].Invocation || summary.Steps[1].Outcome != "failed-recoverable" {
		t.Fatalf("review was not a fresh stopped invocation: %#v", summary.Steps)
	}
}

func TestRepeatedDevelopmentAndReviewOccurrencesRequireFreshGates(t *testing.T) {
	root := nonGitPlannedFixture(t)
	installChangesRequestedCodex(t)

	first, err := Run(context.Background(), root, Options{Policy: config.RunOverrides{Gates: []string{"development", "review"}}})
	if err != nil || first.Gate != "plan" {
		t.Fatalf("first gate=%#v err=%v", first, err)
	}
	if _, err := Run(context.Background(), root, Options{Approve: "plan", Policy: config.RunOverrides{Gates: []string{"development", "review"}}}); err != nil {
		t.Fatal(err)
	}
	developmentOne, err := runstate.Load(root)
	if err != nil || developmentOne.Name != "development" {
		t.Fatalf("first development gate=%#v err=%v", developmentOne, err)
	}
	if _, err := Run(context.Background(), root, Options{Approve: "development", Policy: config.RunOverrides{Gates: []string{"development", "review"}}}); err != nil {
		t.Fatal(err)
	}
	reviewOne, err := runstate.Load(root)
	if err != nil || reviewOne.Name != "review" {
		t.Fatalf("first review gate=%#v err=%v", reviewOne, err)
	}
	if _, err := Run(context.Background(), root, Options{Approve: "review", Policy: config.RunOverrides{Gates: []string{"development", "review"}}}); err != nil {
		t.Fatal(err)
	}
	developmentTwo, err := runstate.Load(root)
	if err != nil || developmentTwo.Name != "development" || developmentTwo.AttemptID == developmentOne.AttemptID {
		t.Fatalf("second development gate=%#v err=%v", developmentTwo, err)
	}
	if _, err := Run(context.Background(), root, Options{Approve: "development", Policy: config.RunOverrides{Gates: []string{"development", "review"}}}); err != nil {
		t.Fatal(err)
	}
	reviewTwo, err := runstate.Load(root)
	if err != nil || reviewTwo.Name != "review" || reviewTwo.AttemptID == reviewOne.AttemptID {
		t.Fatalf("second review gate=%#v err=%v", reviewTwo, err)
	}
}

func TestApprovedActionGateIsClearedBeforeSameKindRecursInOneRun(t *testing.T) {
	t.Run("development", func(t *testing.T) {
		root := nonGitPlannedFixture(t)
		installChangesRequestedCodex(t)
		if _, err := Run(context.Background(), root, Options{Policy: config.RunOverrides{Gates: []string{"development"}}}); err != nil {
			t.Fatal(err)
		}
		if _, err := Run(context.Background(), root, Options{Approve: "plan", Policy: config.RunOverrides{Gates: []string{"development"}}}); err != nil {
			t.Fatal(err)
		}
		summary, err := Run(context.Background(), root, Options{Approve: "development", Policy: config.RunOverrides{Gates: []string{"development"}}})
		if err != nil || summary.Gate != "development" || summary.Actions != 2 || summary.Steps[1].Action != "independent-review" {
			t.Fatalf("summary=%#v err=%v", summary, err)
		}
	})
	t.Run("review", func(t *testing.T) {
		root := nonGitPlannedFixture(t)
		installChangesRequestedCodex(t)
		if _, err := Run(context.Background(), root, Options{Policy: config.RunOverrides{Gates: []string{"review"}}}); err != nil {
			t.Fatal(err)
		}
		first, err := Run(context.Background(), root, Options{Approve: "plan", Policy: config.RunOverrides{Gates: []string{"review"}}})
		if err != nil || first.Gate != "review" {
			t.Fatalf("first=%#v err=%v", first, err)
		}
		summary, err := Run(context.Background(), root, Options{Approve: "review", Policy: config.RunOverrides{Gates: []string{"review"}}})
		if err != nil || summary.Gate != "review" || summary.Actions != 2 || summary.Steps[1].Action != "development" {
			t.Fatalf("summary=%#v err=%v", summary, err)
		}
	})
}

func TestActionAndCycleBoundsStopBeforeProtectedAction(t *testing.T) {
	t.Run("action", func(t *testing.T) {
		root := nonGitPlannedFixture(t)
		installChangesRequestedCodex(t)
		if _, err := Run(context.Background(), root, Options{Policy: config.RunOverrides{MaxActions: 1}}); err != nil {
			t.Fatal(err)
		}
		summary, err := Run(context.Background(), root, Options{Approve: "plan", Policy: config.RunOverrides{MaxActions: 1}})
		if err != nil || summary.Actions != 1 || !strings.Contains(summary.Stop, "action bound exhausted") {
			t.Fatalf("summary=%#v err=%v", summary, err)
		}
	})
	t.Run("cycle", func(t *testing.T) {
		root := nonGitPlannedFixture(t)
		installChangesRequestedCodex(t)
		if _, err := Run(context.Background(), root, Options{Policy: config.RunOverrides{MaxCycles: 1}}); err != nil {
			t.Fatal(err)
		}
		summary, err := Run(context.Background(), root, Options{Approve: "plan", Policy: config.RunOverrides{MaxCycles: 1}})
		if err != nil || summary.Cycles != 1 || !strings.Contains(summary.Stop, "review cycle bound exhausted") {
			t.Fatalf("summary=%#v err=%v", summary, err)
		}
	})
}

func TestCoordinatorStopsOnRepeatedActionAndEvidenceFingerprint(t *testing.T) {
	root := nonGitPlannedFixture(t)
	calls := 0
	runner := func(_ context.Context, _ string, _ execution.Options, _ io.Reader, _ io.Writer) (execution.Result, error) {
		calls++
		return execution.Result{
			Prepared: execution.Prepared{
				Resolution: orchestration.Resolution{Kind: "development", Role: "developer"},
				Action:     orchestration.Action{Correlation: orchestration.Correlation{InvocationID: "fake-invocation"}},
			},
			Reconciliation: execution.Reconciliation{ResultAccepted: true, OutcomeClass: "completed", ObservedState: workflow.Planned},
			Facts:          orchestration.DurableFacts{Class: orchestration.Completed},
		}, nil
	}

	if _, err := Run(context.Background(), root, Options{}); err != nil {
		t.Fatal(err)
	}
	summary, err := run(context.Background(), root, Options{Approve: "plan"}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || summary.Actions != 1 || summary.Stop != "no progress: repeated action and evidence fingerprint" || summary.Recommendation != "concoct code" {
		t.Fatalf("calls=%d summary=%#v", calls, summary)
	}
}

func TestCoordinatorRoutesPolicySatisfiedReviewDirectlyToArchive(t *testing.T) {
	for _, disposition := range []string{"not-required", "externally-satisfied"} {
		t.Run(disposition, func(t *testing.T) {
			root := nonGitCompleteFixture(t, disposition)
			summary, err := Run(context.Background(), root, Options{Policy: config.RunOverrides{Gates: []string{"archive"}}})
			if err != nil {
				t.Fatal(err)
			}
			if summary.Gate != "archive" || summary.Actions != 0 || summary.State != workflow.Complete || summary.Recommendation != "concoct run --approve archive" {
				t.Fatalf("summary=%#v", summary)
			}
			if _, err := os.Stat(filepath.Join(root, ".concoct", "current", "review-01.md")); !os.IsNotExist(err) {
				t.Fatalf("policy route fabricated review evidence: %v", err)
			}
		})
	}
}

func TestPlanningStartupRollbackAndPostLaunchPreservation(t *testing.T) {
	t.Run("startup failure rolls back untouched branch", func(t *testing.T) {
		root := planningProposalFixture(t)
		installRunCodex(t, "completed", "plan", "concoct plan APP-001", "selected eligible work", "", "")
		if summary, err := Run(context.Background(), root, Options{}); err != nil || summary.Gate != "next" {
			t.Fatalf("proposal summary=%#v err=%v", summary, err)
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "codex"), []byte("#!/missing-concoct-test-interpreter\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		summary, err := Run(context.Background(), root, Options{Approve: "next"})
		if err == nil || summary.Actions != 1 || summary.Recommendation != "concoct plan" {
			t.Fatalf("summary=%#v err=%v", summary, err)
		}
		if branch := gitOutput(t, root, "branch", "--show-current"); branch == "concoct/app-001-proposed-work" {
			t.Fatalf("startup failure retained unused planning branch %s", branch)
		}
		if branches := gitOutput(t, root, "branch", "--list", "concoct/app-001-proposed-work"); branches != "" {
			t.Fatalf("startup failure retained task branch: %s", branches)
		}
	})

	t.Run("post-launch mutation is preserved", func(t *testing.T) {
		root := planningProposalFixture(t)
		installRunCodex(t, "completed", "plan", "concoct plan APP-001", "selected eligible work", "", "")
		if summary, err := Run(context.Background(), root, Options{}); err != nil || summary.Gate != "next" {
			t.Fatalf("proposal summary=%#v err=%v", summary, err)
		}
		dir := t.TempDir()
		body := "#!/bin/sh\nprintf 'planner partial work\\n' > .concoct/current/notes.md\nexit 7\n"
		if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		summary, err := Run(context.Background(), root, Options{Approve: "next"})
		if err == nil || summary.Actions != 1 || summary.Recommendation != "concoct plan" {
			t.Fatalf("summary=%#v err=%v", summary, err)
		}
		if branch := gitOutput(t, root, "branch", "--show-current"); branch != "concoct/app-001-proposed-work" {
			t.Fatalf("post-launch work was moved off task branch: %s", branch)
		}
		data, readErr := os.ReadFile(filepath.Join(root, ".concoct", "current", "notes.md"))
		if readErr != nil || string(data) != "planner partial work\n" {
			t.Fatalf("partial planner work was not preserved: %q err=%v", data, readErr)
		}
	})
}

func TestRealGitLifecycleReachesLocalIntegrationWithoutPush(t *testing.T) {
	root, remote := gitPlannedRunFixture(t)
	installHappyLifecycleCodex(t)
	if _, err := Run(context.Background(), root, Options{}); err != nil {
		t.Fatal(err)
	}
	beforeRemote := gitOutput(t, remote, "rev-parse", "refs/heads/main")
	first, err := Run(context.Background(), root, Options{Approve: "plan"})
	if err != nil || first.Gate != "integration" || first.Actions != 3 || first.Cycles != 1 {
		roadmap, _ := os.ReadFile(filepath.Join(root, ".concoct", "roadmap.md"))
		t.Logf("roadmap after archival candidate:\n%s", roadmap)
		t.Fatalf("pre-integration summary=%#v err=%v", first, err)
	}
	second, err := Run(context.Background(), root, Options{Approve: "integration"})
	if err != nil || !second.Completed || second.State != "ready" || second.Actions != 1 {
		t.Fatalf("integration summary=%#v err=%v", second, err)
	}
	if afterRemote := gitOutput(t, remote, "rev-parse", "refs/heads/main"); afterRemote != beforeRemote {
		t.Fatalf("run pushed remote main from %s to %s", beforeRemote, afterRemote)
	}
	if branch := gitOutput(t, root, "branch", "--show-current"); branch != "main" {
		t.Fatalf("final branch = %s", branch)
	}
}

func TestIntegrationConflictStopsInRecoveryWithExactContinuation(t *testing.T) {
	root, _ := gitPlannedRunFixture(t)
	gitOutput(t, root, "checkout", "main")
	if err := os.WriteFile(filepath.Join(root, "feature.txt"), []byte("trunk implementation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "add", "feature.txt")
	gitOutput(t, root, "commit", "-qm", "conflicting trunk implementation")
	gitOutput(t, root, "checkout", "concoct/app-001-demo")
	installHappyLifecycleCodex(t)

	if summary, err := Run(context.Background(), root, Options{}); err != nil || summary.Gate != "plan" {
		t.Fatalf("plan gate summary=%#v err=%v", summary, err)
	}
	if summary, err := Run(context.Background(), root, Options{Approve: "plan"}); err != nil || summary.Gate != "integration" {
		t.Fatalf("integration gate summary=%#v err=%v", summary, err)
	}
	summary, err := Run(context.Background(), root, Options{Approve: "integration"})
	if err == nil || summary.State != workflow.Integrating || summary.Actions != 1 || !strings.Contains(summary.Stop, "conflicted") || summary.Recommendation != "concoct integrate --continue, or concoct integrate --abort" {
		t.Fatalf("summary=%#v err=%v", summary, err)
	}
	if matches, globErr := filepath.Glob(filepath.Join(root, ".git", "concoct", "integrations", "*.yaml")); globErr != nil || len(matches) != 1 {
		t.Fatalf("integration recovery evidence=%v err=%v", matches, globErr)
	}
}

func gitPlannedRunFixture(t *testing.T) (string, string) {
	t.Helper()
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	gitOutput(t, root, "config", "user.email", "test@example.com")
	gitOutput(t, root, "config", "user.name", "Test")
	roadmap := filepath.Join(root, ".concoct", "roadmap.md")
	if err := os.WriteFile(roadmap, []byte("---\nversion: 1\nproject: demo\nupdated: 2026-08-12\n---\n# Roadmap\n\n## APP-001 — Demo\n\n- Status: `planned`\n- Depends on: `none`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "add", "-A")
	gitOutput(t, root, "commit", "-qm", "base")
	base := gitOutput(t, root, "rev-parse", "HEAD")
	gitOutput(t, root, "checkout", "-qb", "concoct/app-001-demo")
	if err := os.WriteFile(roadmap, []byte("---\nversion: 1\nproject: demo\nupdated: 2026-08-12\n---\n# Roadmap\n\n## APP-001 — Demo\n\n- Status: `active`\n- Depends on: `none`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := "---\nid: APP-001\ntitle: Demo\nroadmap-id: APP-001\nstatus: planned\ncreated: 2026-08-12\nupdated: 2026-08-12\ngit:\n  enabled: true\n  trunk: main\n  task-branch: concoct/app-001-demo\n  base: " + base + "\n  status: active\ncapability-impact:\n  type: none\n  rationale: No capability impact.\n---\n# Task Plan\n"
	if err := os.WriteFile(filepath.Join(root, ".concoct", "current", "task-plan.md"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".concoct", "current", "notes.md"), []byte("# Notes\n\nPlanned.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "add", "-A")
	gitOutput(t, root, "commit", "-qm", "concoct: plan APP-001")
	remote := filepath.Join(t.TempDir(), "remote.git")
	gitOutput(t, "", "init", "--bare", "-q", remote)
	gitOutput(t, root, "remote", "add", "origin", remote)
	gitOutput(t, root, "checkout", "main")
	gitOutput(t, root, "push", "-qu", "origin", "main")
	gitOutput(t, root, "checkout", "concoct/app-001-demo")
	return root, remote
}

func installHappyLifecycleCodex(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	body := `#!/bin/sh
set -eu
schema=""
output=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-schema) schema="$2"; shift 2 ;;
    --output-last-message) output="$2"; shift 2 ;;
    *) shift ;;
  esac
done
cat > /dev/null
value() {
  awk -v key="\"$1\"" '$0 ~ key { found=1 } found && /"const":/ { line=$0; sub(/^.*"const": "/, "", line); sub(/".*$/, "", line); print line; exit }' "$schema"
}
role=$(value role)
case "$role" in
  developer)
    sed -i '0,/status: planned/s//status: implementation-complete/' .concoct/current/task-plan.md
    printf '\n## Handoff to reviewer\n\n### Implemented\n\nDemo.\n\n### Verification\n\nChecked.\n\n### Known risks\n\nNone.\n\n### Capability impact\n\nNone.\n\n### Suggested review focus\n\nDemo.\n' >> .concoct/current/notes.md
    printf 'implemented\n' > feature.txt
    ;;
  reviewer)
    printf '%s\n' '---' 'task-id: APP-001' 'review: 1' 'status: approved' 'created: 2026-08-12' 'persona: reviewer' '---' '' '# Review 01' '' '<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->' '' '## Outcome' '' '` + "`approved`" + `' > .concoct/current/review-01.md
    ;;
  archivist)
    today=$(date +%F)
    rel=".concoct/archive/${today}-APP-001-demo"
    tick=$(printf '\140')
    printf '%s\n' '---' 'version: 1' 'project: demo' "updated: $today" '---' '# Roadmap' '' '## APP-001 — Demo' '' "- Status: ${tick}active${tick}" "- Archive: ${tick}${rel}/${tick}" "- Depends on: ${tick}none${tick}" > .concoct/roadmap.md
    mkdir -p "$rel"
    cp .concoct/current/task-plan.md "$rel/task-plan.md"
    cp .concoct/current/notes.md "$rel/notes.md"
    cp .concoct/current/review-01.md "$rel/review-01.md"
    printf '%s\n' '---' 'task-id: APP-001' 'roadmap-id: APP-001' 'status: archived' "archived: $today" 'review: review-01.md' 'delivery: pending-integration' 'capability-impact:' '  type: none' '---' '# Summary' '' '## Delivered outcome' 'Done.' '' '## Key decisions' 'Explicit.' '' '## Files and areas changed' 'Files.' '' '## Verification' 'Passed.' '' '## Review outcome' 'Approved.' '' '## Capability changes' 'None.' '' '## Skipped work' 'None.' '' '## Follow-up work' 'None.' > "$rel/summary.md"
    ;;
esac
printf '{"protocol_version":"v1","correlation":{"invocation_id":"%s","action_id":"%s","task_id":"%s","attempt_id":"%s","role":"%s"},"class":"completed","summary":"completed","artifacts":[],"intervention":{"kind":"","next":""},"diagnostics":[],"recommendation":{"kind":"","command":"","reason":""}}\n' "$(value invocation_id)" "$(value action_id)" "$(value task_id)" "$(value attempt_id)" "$role" > "$output"
`
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func nonGitPlannedFixture(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	roadmap := "---\nversion: 1\nproject: demo\nupdated: 2026-08-11\n---\n# Roadmap\n\n## APP-001 — Demo\n\n- Status: `active`\n- Depends on: `none`\n"
	plan := "---\nid: APP-001\ntitle: Demo\nroadmap-id: APP-001\nstatus: planned\ncreated: 2026-08-11\nupdated: 2026-08-11\ncapability-impact:\n  type: none\n  rationale: No capability impact.\n---\n# Task Plan\n"
	if err := os.WriteFile(filepath.Join(root, ".concoct", "roadmap.md"), []byte(roadmap), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".concoct", "current", "task-plan.md"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".concoct", "current", "notes.md"), []byte("# Notes\n\nPlanned.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func nonGitCompleteFixture(t *testing.T, disposition string) string {
	t.Helper()
	root := nonGitPlannedFixture(t)
	planPath := filepath.Join(root, ".concoct", "current", "task-plan.md")
	plan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	plan = []byte(strings.Replace(string(plan), "status: planned", "status: implementation-complete", 1))
	switch disposition {
	case "not-required":
		policyPath := filepath.Join(root, ".concoct", "policy.md")
		policy, readErr := os.ReadFile(policyPath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		policy = []byte(strings.Replace(strings.Replace(string(policy), "  - independent-review\n  - archival\n  - integration\n", "  - archival\n  - integration\nnot-required-reasons:\n  - independent-review: repository accepts developer verification\n", 1), "  - reviewer-approval-before-archive\n", "", 1))
		if err := os.WriteFile(policyPath, policy, 0o644); err != nil {
			t.Fatal(err)
		}
	case "externally-satisfied":
		if err := os.WriteFile(filepath.Join(root, "audit.md"), []byte("external review accepted\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		evidence := "policy-activity-evidence:\n  - activity: independent-review\n    disposition: externally-satisfied\n    reason: external audit accepted the change\n    recorded-by: developer\n    evidence:\n      - audit.md\n"
		plan = []byte(strings.Replace(string(plan), "capability-impact:\n", evidence+"capability-impact:\n", 1))
	default:
		t.Fatalf("unsupported review disposition %q", disposition)
	}
	if err := os.WriteFile(planPath, plan, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func planningProposalFixture(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	gitOutput(t, root, "config", "user.email", "test@example.com")
	gitOutput(t, root, "config", "user.name", "Test")
	roadmap := "---\nversion: 1\nproject: demo\nupdated: 2026-08-12\n---\n# Roadmap\n\n## APP-001 — Proposed work\n\n- Status: `planned`\n- Depends on: `none`\n"
	if err := os.WriteFile(filepath.Join(root, ".concoct", "roadmap.md"), []byte(roadmap), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "add", "-A")
	gitOutput(t, root, "commit", "-qm", "bootstrap")
	return root
}

func installChangesRequestedCodex(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	body := `#!/bin/sh
set -eu
schema=""
output=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-schema) schema="$2"; shift 2 ;;
    --output-last-message) output="$2"; shift 2 ;;
    *) shift ;;
  esac
done
cat > /dev/null
value() {
  awk -v key="\"$1\"" '$0 ~ key { found=1 } found && /"const":/ { line=$0; sub(/^.*"const": "/, "", line); sub(/".*$/, "", line); print line; exit }' "$schema"
}
role=$(value role)
if [ "$role" = "developer" ]; then
  if ls .concoct/current/review-*.md >/dev/null 2>&1; then
    sed -i '/^status: implementation-complete/a remediates-review: review-01.md' .concoct/current/task-plan.md
  else
    sed -i '0,/status: planned/s//status: implementation-complete/' .concoct/current/task-plan.md
  fi
  printf '\n## Handoff to reviewer\n\n### Implemented\n\nDemo %s.\n\n### Verification\n\nChecked.\n\n### Known risks\n\nNone.\n\n### Capability impact\n\nNone.\n\n### Suggested review focus\n\nDemo.\n' "$(date +%s%N)" >> .concoct/current/notes.md
else
  count=$(find .concoct/current -name 'review-*.md' | wc -l | tr -d ' ')
  number=$(printf '%02d' $((count + 1)))
  printf '%s\n' '---' 'task-id: APP-001' "review: $((count + 1))" 'status: changes-requested' 'created: 2026-08-12' 'persona: reviewer' '---' '' "# Review $number" '' '<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->' '' '## Outcome' '' '` + "`changes-requested`" + `' > ".concoct/current/review-$number.md"
fi
printf '{"protocol_version":"v1","correlation":{"invocation_id":"%s","action_id":"%s","task_id":"%s","attempt_id":"%s","role":"%s"},"class":"completed","summary":"completed","artifacts":[],"intervention":{"kind":"","next":""},"diagnostics":[],"recommendation":{"kind":"","command":"","reason":""}}\n' "$(value invocation_id)" "$(value action_id)" "$(value task_id)" "$(value attempt_id)" "$role" > "$output"
`
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func installRunCodex(t *testing.T, class, kind, command, reason, interventionKind, interventionNext string) {
	t.Helper()
	dir := t.TempDir()
	body := `#!/bin/sh
set -eu
schema=""
output=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-schema) schema="$2"; shift 2 ;;
    --output-last-message) output="$2"; shift 2 ;;
    *) shift ;;
  esac
done
value() {
  awk -v key="\"$1\"" '$0 ~ key { found=1 } found && /"const":/ { line=$0; sub(/^.*"const": "/, "", line); sub(/".*$/, "", line); print line; exit }' "$schema"
}
printf '{"protocol_version":"v1","correlation":{"invocation_id":"%s","action_id":"%s","task_id":"%s","attempt_id":"%s","role":"%s"},"class":"%s","summary":"proposal","artifacts":[],"intervention":{"kind":"%s","next":"%s"},"diagnostics":[],"recommendation":{"kind":"%s","command":"%s","reason":"%s"}}\n' "$(value invocation_id)" "$(value action_id)" "$(value task_id)" "$(value attempt_id)" "$(value role)" ` + quote(class) + ` ` + quote(interventionKind) + ` ` + quote(interventionNext) + ` ` + quote(kind) + ` ` + quote(command) + ` ` + quote(reason) + ` > "$output"
`
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func installPlanningCodex(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	body := `#!/bin/sh
set -eu
schema=""
output=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-schema) schema="$2"; shift 2 ;;
    --output-last-message) output="$2"; shift 2 ;;
    *) shift ;;
  esac
done
prompt=$(mktemp)
cat > "$prompt"
value() {
  awk -v key="\"$1\"" '$0 ~ key { found=1 } found && /"const":/ { line=$0; sub(/^.*"const": "/, "", line); sub(/".*$/, "", line); print line; exit }' "$schema"
}
role=$(value role)
kind="plan"
command="concoct plan APP-001"
reason="selected eligible work"
summary="proposal"
if [ "$role" = "task-planner" ]; then
  trunk=$(grep '^- Git trunk:' "$prompt" | cut -d: -f2- | tr -d ' \140')
  branch=$(grep '^- Git task branch:' "$prompt" | cut -d: -f2- | tr -d ' \140')
  base=$(grep '^- Git task base:' "$prompt" | cut -d: -f2- | tr -d ' \140')
  sed -i '/^## APP-001 /,/^## / s/- Status: .planned./- Status: \x60active\x60/' .concoct/roadmap.md
  printf '%s\n' '---' 'id: APP-001' 'title: Proposed work' 'roadmap-id: APP-001' 'status: planned' 'created: 2026-08-11' 'updated: 2026-08-11' 'git:' '  enabled: true' "  trunk: $trunk" "  task-branch: $branch" "  base: $base" '  status: active' 'capability-impact:' '  type: none' '  rationale: No capability impact.' '---' '# Task Plan' > .concoct/current/task-plan.md
  printf '# Notes\n\nPlanning complete.\n' > .concoct/current/notes.md
  kind=""
  command=""
  reason=""
  summary="planning complete"
fi
printf '{"protocol_version":"v1","correlation":{"invocation_id":"%s","action_id":"%s","task_id":"%s","attempt_id":"%s","role":"%s"},"class":"completed","summary":"%s","artifacts":[],"intervention":{"kind":"","next":""},"diagnostics":[],"recommendation":{"kind":"%s","command":"%s","reason":"%s"}}\n' "$(value invocation_id)" "$(value action_id)" "$(value task_id)" "$(value attempt_id)" "$role" "$summary" "$kind" "$command" "$reason" > "$output"
rm -f "$prompt"
`
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func installDevelopmentReviewCodex(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	body := `#!/bin/sh
set -eu
schema=""
output=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-schema) schema="$2"; shift 2 ;;
    --output-last-message) output="$2"; shift 2 ;;
    *) shift ;;
  esac
done
cat > /dev/null
value() {
  awk -v key="\"$1\"" '$0 ~ key { found=1 } found && /"const":/ { line=$0; sub(/^.*"const": "/, "", line); sub(/".*$/, "", line); print line; exit }' "$schema"
}
role=$(value role)
class="completed"
summary="development complete"
intervention=""
next=""
if [ "$role" = "developer" ]; then
  sed -i '0,/status: planned/s//status: implementation-complete/' .concoct/current/task-plan.md
  printf '\n## Handoff to reviewer\n\n### Implemented\n\nDemo.\n\n### Verification\n\nChecked.\n\n### Known risks\n\nNone.\n\n### Capability impact\n\nNone.\n\n### Suggested review focus\n\nDemo.\n' >> .concoct/current/notes.md
else
  class="failed-recoverable"
  summary="review service unavailable"
  intervention="review-routing"
  next="Reviewer routes findings or blocker to the responsible role"
fi
printf '{"protocol_version":"v1","correlation":{"invocation_id":"%s","action_id":"%s","task_id":"%s","attempt_id":"%s","role":"%s"},"class":"%s","summary":"%s","artifacts":[],"intervention":{"kind":"%s","next":"%s"},"diagnostics":[],"recommendation":{"kind":"","command":"","reason":""}}\n' "$(value invocation_id)" "$(value action_id)" "$(value task_id)" "$(value attempt_id)" "$role" "$class" "$summary" "$intervention" "$next" > "$output"
`
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func quote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
