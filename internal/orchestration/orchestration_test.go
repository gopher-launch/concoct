package orchestration

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gopher-launch/concoct/internal/workflow"
)

func TestRegistryHasExplicitContractForEveryAction(t *testing.T) {
	for _, spec := range Registry() {
		if spec.Kind == "" || spec.Role == "" || spec.Gate == "" || spec.Authority == "" || len(spec.AllowedStates) == 0 || len(spec.CompletedStates) == 0 || len(spec.SupportedOutcomes) != 5 || len(spec.PermittedEffects) == 0 || len(spec.Preconditions) == 0 || spec.CompletionValidator == nil || len(spec.Intervention.RequiredFor) == 0 || spec.Intervention.Kind == "" || spec.Intervention.Next == "" {
			t.Fatalf("incomplete contract: %#v", spec)
		}
		if err := spec.CompletionValidator(workflow.Report{State: spec.CompletedStates[0]}); err != nil {
			t.Fatalf("%s completion validator rejected registered completion state: %v", spec.Kind, err)
		}
		if err := spec.CompletionValidator(workflow.Report{State: workflow.Invalid}); err == nil {
			t.Fatalf("%s completion validator accepted invalid state", spec.Kind)
		}
		if !containsOutcome(spec.Intervention.RequiredFor, Blocked) {
			t.Fatalf("%s does not define blocked intervention behavior", spec.Kind)
		}
	}
}

func TestAuthorizeReadyPreservesProductOwnerSelectionBoundary(t *testing.T) {
	root := fixture(t, workflow.Ready)
	if report := workflow.Detect(root); report.State != workflow.Ready {
		t.Fatalf("fixture report = %#v", report)
	}
	action, err := Authorize(root, "product-owner-next", "attempt-1")
	if err != nil {
		t.Fatal(err)
	}
	if action.Correlation.Role != "product-owner" || action.Correlation.TaskID != "" || !action.Executable {
		t.Fatalf("action = %#v", action)
	}
	if _, err := Authorize(root, "development", "attempt-1"); err == nil {
		t.Fatal("development authorized in ready state")
	}
}

func TestValidateOutcomeRejectsMismatchedAndContradictedCompletion(t *testing.T) {
	root := fixture(t, workflow.Planned)
	action, err := Authorize(root, "development", "attempt-1")
	if err != nil {
		t.Fatal(err)
	}
	outcome := Outcome{ProtocolVersion: ProtocolVersion, Correlation: action.Correlation, Class: Completed, Summary: "implemented"}
	if _, err := ValidateOutcome(root, action, outcome); err == nil {
		t.Fatal("unchanged state accepted as completion")
	}
	outcome.Correlation.InvocationID = "other"
	if _, err := ValidateOutcome(root, action, outcome); err == nil {
		t.Fatal("mismatched correlation accepted")
	}
}

func TestValidateOutcomeAcceptsOnlyObservedRegisteredPostcondition(t *testing.T) {
	root := fixture(t, workflow.Planned)
	action, err := Authorize(root, "development", "attempt-1")
	if err != nil {
		t.Fatal(err)
	}
	task := filepath.Join(root, ".concoct/current/task-plan.md")
	data, err := os.ReadFile(task)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "status: planned", "status: implementation-complete", 1))
	if err := os.WriteFile(task, data, 0o644); err != nil {
		t.Fatal(err)
	}
	outcome := Outcome{ProtocolVersion: ProtocolVersion, Correlation: action.Correlation, Class: Completed, Summary: "implemented"}
	facts, err := ValidateOutcome(root, action, outcome)
	if err != nil || facts.Class != Completed {
		t.Fatalf("facts=%#v err=%v", facts, err)
	}
}

func TestAtomicResultIsSingleAndBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	outcome := Outcome{ProtocolVersion: ProtocolVersion, Class: Blocked, Summary: "needs a decision"}
	if err := WriteAtomicResult(path, outcome); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadResult(path); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomicResult(path, outcome); err == nil {
		t.Fatal("duplicate result accepted")
	}
}

func TestAtomicResultConcurrentDeliveryPreservesFirstResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	first := Outcome{ProtocolVersion: ProtocolVersion, Class: Blocked, Summary: "first"}
	second := Outcome{ProtocolVersion: ProtocolVersion, Class: Blocked, Summary: "second"}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, outcome := range []Outcome{first, second} {
		wg.Add(1)
		go func(outcome Outcome) {
			defer wg.Done()
			<-start
			errs <- WriteAtomicResult(path, outcome)
		}(outcome)
	}
	close(start)
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful deliveries = %d, want 1", successes)
	}
	stored, err := ReadResult(path)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Summary != "first" && stored.Summary != "second" {
		t.Fatalf("stored outcome = %#v", stored)
	}
	if err := WriteAtomicResult(path, Outcome{ProtocolVersion: ProtocolVersion, Class: Blocked, Summary: "third"}); err == nil {
		t.Fatal("existing concurrent result was overwritten")
	}
}

func TestValidateOutcomeRequiresRegisteredIntervention(t *testing.T) {
	root := fixture(t, workflow.Planned)
	action, err := Authorize(root, "development", "attempt-1")
	if err != nil {
		t.Fatal(err)
	}
	outcome := Outcome{ProtocolVersion: ProtocolVersion, Correlation: action.Correlation, Class: Blocked, Summary: "blocked"}
	if _, err := ValidateOutcome(root, action, outcome); err == nil {
		t.Fatal("missing registered intervention accepted")
	}
	spec, _ := Find(action.Kind)
	outcome.Intervention = Intervention{Kind: spec.Intervention.Kind, Next: spec.Intervention.Next}
	if _, err := ValidateOutcome(root, action, outcome); err != nil {
		t.Fatalf("registered intervention rejected: %v", err)
	}
}

func fixture(t *testing.T, state workflow.State) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{".concoct/current", ".concoct/archive"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, data string) {
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("AGENTS.md", "---\ninstruction-layer: project-guidance\n---\n# Agents\n")
	write(".concoct/policy.md", "---\ninstruction-layer: policy\nrequired-phases:\n  - product-ownership\n  - task-planning\n  - development\n  - independent-review\n  - archival\n  - integration\napproval-gates:\n  - reviewer-approval-before-archive\n  - archive-before-integration\ngit-strategy: task-branch-with-squash-integration\n---\n# Policy\n")
	roadmapStatus := "active"
	if state == workflow.Ready {
		roadmapStatus = "planned"
	}
	write(".concoct/roadmap.md", "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n## APP-001 — Demo\n- Status: `"+roadmapStatus+"`\n")
	write(".concoct/capabilities.md", "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Capabilities\n")
	if state != workflow.Ready {
		status := string(state)
		if state == workflow.Complete {
			status = "implementation-complete"
		}
		write(".concoct/current/task-plan.md", "---\nid: APP-001\ntitle: Demo\nroadmap-id: APP-001\nstatus: "+status+"\ncreated: 2026-01-01\nupdated: 2026-01-01\ncapability-impact:\n  type: none\n  rationale: none\n---\n# Task\n")
		write(".concoct/current/notes.md", "# Notes\n")
	} else {
		write(".concoct/current/task-plan.md", "# task-plan.md\n")
		write(".concoct/current/notes.md", "# notes.md\n_Add decisions here._\n_Record meaningful verification results here._\n")
	}
	return root
}
