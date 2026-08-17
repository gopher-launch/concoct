package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/adapter"
	"github.com/gopher-launch/concoct/internal/config"
	"github.com/gopher-launch/concoct/internal/defaults"
	"github.com/gopher-launch/concoct/internal/prompt"
	"github.com/gopher-launch/concoct/internal/workflow"
)

func TestPrepareDryRunPreservesPromptBytesAndRuntime(t *testing.T) {
	root := readyFixture(t)
	installFakeCodex(t, "exit 0")
	prepared, err := Prepare(root, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	manual, err := prompt.Render(root, prompt.Request{Command: "next"})
	if err != nil {
		t.Fatal(err)
	}
	if string(prepared.Prompt) != string(manual) {
		t.Fatal("execution prompt differs from manual rendering")
	}
	if _, err := os.Stat(filepath.Join(root, ".concoct", "runtime")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created runtime evidence: %v", err)
	}
	if !strings.Contains(Describe(prepared), "workspace-write") || !strings.Contains(Describe(prepared), "stdin") {
		t.Fatal("dry-run omitted safety or prompt posture")
	}
}

func TestEverySupervisedRoleReceivesExactManualPromptPlusFixedAppendix(t *testing.T) {
	t.Run("task-planner", func(t *testing.T) {
		root := planningPromptFixture(t)
		prepared, err := Prepare(root, Options{SelectedPlan: "APP-001"})
		if err != nil {
			t.Fatal(err)
		}
		defer prepared.Plan.Rollback()
		manual, err := prepared.Plan.Render()
		if err != nil {
			t.Fatal(err)
		}
		assertSupervisedPrompt(t, prepared.Prompt, manual)
	})

	for _, test := range []struct {
		name, state, command string
	}{
		{name: "developer", state: "planned", command: "code"},
		{name: "reviewer", state: "implementation-complete", command: "review"},
		{name: "archivist", state: "approved", command: "archive"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := supervisedPromptFixture(t, test.state)
			prepared, err := Prepare(root, Options{})
			if err != nil {
				t.Fatal(err)
			}
			manual, err := prompt.Render(root, prompt.Request{Command: test.command})
			if err != nil {
				t.Fatal(err)
			}
			assertSupervisedPrompt(t, prepared.Prompt, manual)
		})
	}
}

func TestLiveRoleBenchmarkContractsAreConcrete(t *testing.T) {
	root := supervisedPromptFixture(t, "planned")
	plan, err := os.ReadFile(filepath.Join(root, ".concoct/current/task-plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(plan, []byte("benchmark-result.txt")) || !bytes.Contains(plan, []byte("## Acceptance criteria")) {
		t.Fatalf("Developer benchmark task is not concrete: %s", plan)
	}
	for _, test := range []struct{ resource, contract string }{
		{"persona-reviewer", "must match. The `## Outcome` section"},
		{"persona-archivist", "The parser resolves that directory to `summary.md`"},
	} {
		body, err := defaults.Read(test.resource, "benchmark contract test")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte(test.contract)) {
			t.Fatalf("%s lacks completion contract %q", test.resource, test.contract)
		}
	}
}

func TestLiveRoleBenchmarkFixturesReachCompletionOffline(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		wantState workflow.State
		mutations string
	}{
		{
			name:      "developer",
			state:     "planned",
			wantState: workflow.Complete,
			mutations: `
printf 'benchmark-complete\n' > benchmark-result.txt
sed -i '0,/status: planned/s//status: implementation-complete/' .concoct/current/task-plan.md
cat >> .concoct/current/notes.md <<'EOF'

## Handoff to reviewer

### Implemented

Created the exact benchmark result.

### Verification

Exact content checked.

### Known risks

None.

### Capability impact

None.

### Suggested review focus

Exact file content and scope.
EOF
`,
		},
		{
			name:      "reviewer",
			state:     "implementation-complete",
			wantState: workflow.Approved,
			mutations: `
cat > .concoct/current/review-01.md <<'EOF'
---
task-id: APP-001
review: 1
status: approved
created: 2026-08-14
persona: reviewer
---
# Review 01

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

` + "`approved`" + `

## Summary

The exact fixture result and scope satisfy the task.
EOF
`,
		},
		{
			name:      "archivist",
			state:     "approved",
			wantState: workflow.Archived,
			mutations: `
archive=".concoct/archive/$(date +%F)-APP-001-demo"
mkdir -p "$archive"
cp .concoct/current/task-plan.md "$archive/task-plan.md"
cp .concoct/current/notes.md "$archive/notes.md"
cp .concoct/current/review-01.md "$archive/review-01.md"
cat > "$archive/summary.md" <<EOF
---
task-id: APP-001
roadmap-id: APP-001
status: archived
archived: $(date +%F)
review: review-01.md
delivery: pending-integration
capability-impact:
  type: none
---
# Summary

## Delivered outcome

Created the exact benchmark result.

## Key decisions

Kept the fixture deliberately narrow.

## Files and areas changed

Benchmark result and workflow evidence.

## Verification

Exact content checked.

## Review outcome

Approved by review-01.md.

## Capability changes

None.

## Skipped work

None.

## Follow-up work

None.
EOF
awk -v archive="$archive" '{print; if ($0 == "- Status: ` + "`active`" + `") print "- Archive: ` + "`" + `" archive "/` + "`" + `"}' .concoct/roadmap.md > .concoct/roadmap.md.tmp
mv .concoct/roadmap.md.tmp .concoct/roadmap.md
`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := supervisedPromptFixture(t, test.state)
			installFakeCodex(t, supervisedOutcomeScript(test.mutations, "completed", "", ""))
			result, err := Run(context.Background(), root, Options{}, strings.NewReader(""), &strings.Builder{})
			if err != nil {
				t.Fatalf("offline fixture did not complete: %v", err)
			}
			if !result.Reconciliation.ResultAccepted || result.Reconciliation.ObservedState != test.wantState {
				t.Fatalf("reconciliation = %#v, want accepted state %s", result.Reconciliation, test.wantState)
			}
		})
	}
}

func assertSupervisedPrompt(t *testing.T, got, manual []byte) {
	t.Helper()
	want := append(append([]byte(nil), manual...), []byte(supervisionAppendix)...)
	if !bytes.Equal(got, want) {
		t.Fatalf("supervised prompt is not the byte-identical manual core plus the fixed appendix: got=%d bytes want=%d bytes", len(got), len(want))
	}
	if !bytes.HasPrefix(got, manual) || bytes.Count(got, []byte("## Executable supervision boundary")) != 1 {
		t.Fatal("supervised prompt core or appendix boundary is ambiguous")
	}
}

func TestOneShotRunRetainsPrivateSanitizedEvidenceAndAcceptsDecision(t *testing.T) {
	root := readyFixture(t)
	installFakeCodex(t, `
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

invocation=$(value invocation_id)
action=$(value action_id)
task=$(value task_id)
attempt=$(value attempt_id)
role=$(value role)
printf 'OPENAI_API_KEY=sk-supersecret123456789\n' >&2
printf '{"protocol_version":"v1","correlation":{"invocation_id":"%s","action_id":"%s","task_id":"%s","attempt_id":"%s","role":"%s"},"class":"completed","summary":"no actionable work","artifacts":[],"intervention":{"kind":"","next":""},"diagnostics":[],"recommendation":{"kind":"no-action","command":"","reason":"no eligible work"}}\n' "$invocation" "$action" "$task" "$attempt" "$role" > "$output"
`)
	result, err := Run(context.Background(), root, Options{}, strings.NewReader(""), &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Reconciliation.ResultAccepted || result.Reconciliation.ObservedState != "ready" {
		t.Fatalf("reconciliation = %#v", result.Reconciliation)
	}
	for _, name := range []string{"prompt.md", "action.json", "metadata.json", "outcome-schema.json", "adapter-result.json", "result.json", "stdout.log", "stderr.log", "reconciliation.json"} {
		path := filepath.Join(result.RecordPath, name)
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("%s: %v", name, statErr)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o", name, info.Mode().Perm())
		}
	}
	log, _ := os.ReadFile(filepath.Join(result.RecordPath, "stderr.log"))
	if strings.Contains(string(log), "supersecret") || !strings.Contains(string(log), "[REDACTED]") {
		t.Fatalf("log was not redacted: %s", log)
	}
	inspection, err := Inspect(root, result.Prepared.Action.Correlation.InvocationID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspection, "## Prompt") || !strings.Contains(inspection, "no actionable work") {
		t.Fatal("inspection did not use retained attempt material")
	}
}

func TestInvocationMetadataRecordsKnownPredecessorOnly(t *testing.T) {
	root := readyFixture(t)
	installFakeCodex(t, "exit 7")
	result, err := Run(context.Background(), root, Options{PredecessorInvocationID: "prior-invocation"}, strings.NewReader(""), io.Discard)
	if err == nil {
		t.Fatal("failed adapter unexpectedly succeeded")
	}
	data, err := os.ReadFile(filepath.Join(result.RecordPath, "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"predecessor_invocation_id": "prior-invocation"`) {
		t.Fatalf("metadata lacks predecessor: %s", data)
	}
}

func TestCancellationClosesAttemptAndStopsResultAcceptance(t *testing.T) {
	root := readyFixture(t)
	installFakeCodex(t, "sleep 10 &\nwait")
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	result, err := Run(ctx, root, Options{Override: config.Overrides{Timeout: "2s"}}, strings.NewReader(""), &strings.Builder{})
	if err == nil {
		t.Fatal("cancelled adapter accepted")
	}
	if result.Reconciliation.InvocationDisposition != "cancelled" || result.Reconciliation.ResultAccepted {
		t.Fatalf("reconciliation = %#v", result.Reconciliation)
	}
	if _, statErr := os.Stat(filepath.Join(result.RecordPath, "reconciliation.json")); statErr != nil {
		t.Fatal(statErr)
	}
}

func TestAdapterFailureModesCloseInspectably(t *testing.T) {
	tests := []struct{ name, body, disposition string }{
		{"nonzero", "exit 7", "nonzero-exit"},
		{"missing result", "exit 0", "missing-or-malformed-result"},
		{"malformed result", `output=""; while [ "$#" -gt 0 ]; do if [ "$1" = "--output-last-message" ]; then output="$2"; shift 2; else shift; fi; done; printf '{' > "$output"`, "missing-or-malformed-result"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := readyFixture(t)
			installFakeCodex(t, test.body)
			result, err := Run(context.Background(), root, Options{}, strings.NewReader(""), &strings.Builder{})
			if err == nil {
				t.Fatal("failed adapter accepted")
			}
			if result.Reconciliation.InvocationDisposition != test.disposition || result.Reconciliation.ResultAccepted {
				t.Fatalf("reconciliation = %#v", result.Reconciliation)
			}
			if _, statErr := os.Stat(filepath.Join(result.RecordPath, "reconciliation.json")); statErr != nil {
				t.Fatal(statErr)
			}
		})
	}
}

func TestTimeoutAndConfigurationDriftAreRejected(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		root := readyFixture(t)
		installFakeCodex(t, "sleep 10")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		result, err := Run(ctx, root, Options{Override: config.Overrides{Timeout: "2s"}}, strings.NewReader(""), &strings.Builder{})
		if err == nil || result.Reconciliation.InvocationDisposition != "timed-out" {
			t.Fatalf("result=%#v err=%v", result, err)
		}
	})
	t.Run("user config drift", func(t *testing.T) {
		root := readyFixture(t)
		installFakeCodex(t, "exit 0")
		prepared, err := Prepare(root, Options{})
		if err != nil {
			t.Fatal(err)
		}
		userPath, err := config.UserPath()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(userPath, []byte("exec:\n  roles:\n    product-owner:\n      reasoning: low\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := configurationStable(root, config.Overrides{}, prepared); err == nil {
			t.Fatal("changed user configuration remained authorized")
		}
	})
}

func TestHardElapsedBudgetStopsWithDistinctDisposition(t *testing.T) {
	root := readyFixture(t)
	if err := os.WriteFile(filepath.Join(root, ".concoct", "config.yaml"), []byte("exec:\n  roles:\n    product-owner:\n      budget:\n        hard-elapsed: 40ms\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	installFakeCodex(t, "sleep 10")
	result, err := Run(context.Background(), root, Options{Override: config.Overrides{Timeout: "2s"}}, strings.NewReader(""), &strings.Builder{})
	if err == nil || result.Reconciliation.InvocationDisposition != "budget-exhausted" || result.Reconciliation.ResultAccepted {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(result.Measurement.Budgets) == 0 || result.Measurement.Budgets[0].Dimension != "elapsed" || !result.Measurement.Budgets[0].Enforceable {
		t.Fatalf("budget evidence = %#v", result.Measurement.Budgets)
	}
}

func TestEventDrivenHardBudgetsRejectLateCandidateAndPreserveMeasurement(t *testing.T) {
	// The candidate is deliberately written after the threshold event but before
	// the adapter is stopped. This exercises the stop-decision precedence that
	// prevents a racing structured result from advancing workflow state.
	tests := []struct {
		name, budget, event, dimension, unit string
	}{
		{
			name:      "activity",
			budget:    "hard-activity: 1",
			event:     `{"type":"item.completed","item":{"type":"command_execution","command":"go test ./...","aggregated_output":"ok"}}`,
			dimension: "activity",
			unit:      "events",
		},
		{
			name:      "command output",
			budget:    "hard-command-output-bytes: 1",
			event:     `{"type":"item.completed","item":{"type":"command_execution","command":"go test ./...","aggregated_output":"output"}}`,
			dimension: "command-output",
			unit:      "bytes",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := readyFixture(t)
			if err := os.WriteFile(filepath.Join(root, ".concoct", "config.yaml"), []byte("exec:\n  roles:\n    product-owner:\n      budget:\n        "+test.budget+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			installFakeCodex(t, lateCandidateAfterEventScript(test.event))

			result, err := Run(context.Background(), root, Options{Override: config.Overrides{Timeout: "2s"}}, strings.NewReader(""), &strings.Builder{})
			if err == nil {
				t.Fatal("expected hard budget exhaustion")
			}
			if result.Reconciliation.InvocationDisposition != "budget-exhausted" || result.Reconciliation.ResultAccepted {
				t.Fatalf("reconciliation=%#v err=%v", result.Reconciliation, err)
			}
			if result.Reconciliation.ArtifactReusability["late-structured-outcome"] != "rejected" {
				t.Fatalf("late candidate reuse guidance = %#v", result.Reconciliation.ArtifactReusability)
			}
			if _, err := os.Stat(filepath.Join(result.RecordPath, "adapter-result.json")); err != nil {
				t.Fatalf("late candidate was not preserved: %v", err)
			}
			if _, err := os.Stat(filepath.Join(result.RecordPath, "result.json")); !os.IsNotExist(err) {
				t.Fatalf("late candidate was accepted into result evidence: %v", err)
			}
			report := workflow.Detect(root)
			if report.State != workflow.Ready {
				t.Fatalf("workflow state advanced to %s: %#v", report.State, report)
			}
			if result.Measurement.ActivityEvents == 0 || result.Measurement.Commands.OutputBytes == 0 {
				t.Fatalf("partial measurement was not preserved: %#v", result.Measurement)
			}
			var event *adapter.BudgetEvent
			for index := range result.Measurement.Budgets {
				candidate := &result.Measurement.Budgets[index]
				if candidate.Kind == "hard" && candidate.Dimension == test.dimension {
					event = candidate
					break
				}
			}
			if event == nil || event.Source != "project role configuration" || event.Observed < event.Limit || event.Unit != test.unit || event.Evaluation != "live" || !event.Enforceable {
				t.Fatalf("hard budget evidence = %#v", result.Measurement.Budgets)
			}
		})
	}
}

func lateCandidateAfterEventScript(event string) string {
	return `output=""
schema=""
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
printf '%s\n' '` + event + `'
printf '{"protocol_version":"v1","correlation":{"invocation_id":"%s","action_id":"%s","task_id":"%s","attempt_id":"%s","role":"%s"},"class":"completed","summary":"no actionable work","artifacts":[],"intervention":{"kind":"","next":""},"diagnostics":[],"recommendation":{"kind":"no-action","command":"","reason":"no eligible work"}}\n' "$(value invocation_id)" "$(value action_id)" "$(value task_id)" "$(value attempt_id)" "$(value role)" > "$output"
sleep 10`
}

func TestWarningBudgetsAreRecordedOnceAcrossTerminalDispositions(t *testing.T) {
	tests := []struct {
		name, budget, body, disposition string
	}{
		{
			name:   "completed",
			budget: "warn-elapsed: 1ms\n        warn-activity: 1\n        warn-command-output-bytes: 1",
			body: `output=""
	schema=""
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
printf '%s\n' '{"type":"item.completed","item":{"type":"command_execution","command":"go test ./...","aggregated_output":"ok"}}'
sleep 0.04
printf '{"protocol_version":"v1","correlation":{"invocation_id":"%s","action_id":"%s","task_id":"%s","attempt_id":"%s","role":"%s"},"class":"completed","summary":"no actionable work","artifacts":[],"intervention":{"kind":"","next":""},"diagnostics":[],"recommendation":{"kind":"no-action","command":"","reason":"no eligible work"}}\n' "$(value invocation_id)" "$(value action_id)" "$(value task_id)" "$(value attempt_id)" "$(value role)" > "$output"`,
			disposition: "completed",
		},
		{
			name:        "nonzero exit",
			budget:      "warn-elapsed: 1ms\n        warn-activity: 1\n        warn-command-output-bytes: 1",
			body:        "printf '%s\\n' '{\"type\":\"item.completed\",\"item\":{\"type\":\"command_execution\",\"command\":\"go test ./...\",\"aggregated_output\":\"ok\"}}'\nsleep 0.04\nexit 1",
			disposition: "nonzero-exit",
		},
		{
			name:        "hard stopped",
			budget:      "warn-elapsed: 1ms\n        warn-activity: 1\n        warn-command-output-bytes: 1\n        hard-elapsed: 40ms",
			body:        "printf '%s\\n' '{\"type\":\"item.completed\",\"item\":{\"type\":\"command_execution\",\"command\":\"go test ./...\",\"aggregated_output\":\"ok\"}}'\nsleep 10",
			disposition: "budget-exhausted",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := readyFixture(t)
			if err := os.WriteFile(filepath.Join(root, ".concoct", "config.yaml"), []byte("exec:\n  roles:\n    product-owner:\n      budget:\n        "+test.budget+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			installFakeCodex(t, test.body)
			result, err := Run(context.Background(), root, Options{Override: config.Overrides{Timeout: "2s"}}, strings.NewReader(""), &strings.Builder{})
			if test.disposition == "completed" && err != nil {
				t.Fatal(err)
			}
			if test.disposition != "completed" && err == nil {
				t.Fatal("expected execution failure")
			}
			if result.Reconciliation.InvocationDisposition != test.disposition {
				t.Fatalf("disposition=%q err=%v", result.Reconciliation.InvocationDisposition, err)
			}
			seen := map[string]adapter.BudgetEvent{}
			for _, event := range result.Measurement.Budgets {
				if event.Kind == "warning" {
					if _, duplicate := seen[event.Dimension]; duplicate {
						t.Fatalf("duplicate warning events: %#v", result.Measurement.Budgets)
					}
					seen[event.Dimension] = event
				}
			}
			for _, dimension := range []string{"elapsed", "activity", "command-output"} {
				event, ok := seen[dimension]
				if !ok || event.Evaluation != "live" || !event.Enforceable {
					t.Fatalf("%s warning=%#v all=%#v", dimension, event, result.Measurement.Budgets)
				}
			}
		})
	}
}

func TestRepositoryDriftAfterPreparePreventsAdapterLaunch(t *testing.T) {
	root := readyFixture(t)
	marker := filepath.Join(t.TempDir(), "launched")
	installFakeCodex(t, "touch \""+marker+"\"")
	options := Options{beforeLaunch: func(root string, _ Prepared) error {
		path := filepath.Join(root, "AGENTS.md")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(data, []byte("\nchanged after prepare\n")...), 0o644)
	}}
	result, err := Run(context.Background(), root, options, strings.NewReader(""), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "no action was launched") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if result.Reconciliation.InvocationDisposition != "authorization-changed" || result.Reconciliation.ResultAccepted {
		t.Fatalf("reconciliation = %#v", result.Reconciliation)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("adapter process launched after evidence drift: %v", statErr)
	}
}

func TestRepositoryDriftAfterPreparePreventsDirectIntegration(t *testing.T) {
	root := archivedFixture(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	options := Options{beforeLaunch: func(root string, _ Prepared) error {
		path := filepath.Join(root, "AGENTS.md")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(data, []byte("\nchanged after prepare\n")...), 0o644)
	}}
	result, err := Run(context.Background(), root, options, strings.NewReader(""), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "no action was launched") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if result.Reconciliation.InvocationDisposition != "authorization-changed" {
		t.Fatalf("reconciliation = %#v", result.Reconciliation)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".git/concoct/integrations/APP-001.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("direct integration started after evidence drift: %v", statErr)
	}
}

func TestSupervisedDeveloperCandidateIsValidatedAndCommittedByOuterExecutor(t *testing.T) {
	root := plannedGitFixture(t)
	installFakeCodex(t, supervisedOutcomeScript(`
sed -i '0,/status: planned/s//status: implementation-complete/' .concoct/current/task-plan.md
printf '\n## Handoff to reviewer\n\n### Implemented\n\nChanged source.\n\n### Verification\n\nChecked.\n\n### Known risks\n\nNone.\n\n### Capability impact\n\nNone.\n\n### Suggested review focus\n\nSource.\n' >> .concoct/current/notes.md
printf 'implemented\n' > feature.txt
printf 'run:\n  max-actions: 10\n' > .concoct/config.yaml
`, "completed", "", ""))
	before := executionGit(t, root, "rev-parse", "HEAD")
	result, err := Run(context.Background(), root, Options{}, strings.NewReader(""), &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	after := executionGit(t, root, "rev-parse", "HEAD")
	if before == after || executionGit(t, root, "log", "-1", "--format=%s") != "concoct: complete APP-001 implementation" {
		t.Fatalf("outer completion did not create the Developer transition: before=%s after=%s", before, after)
	}
	if status := executionGit(t, root, "status", "--short"); status != "" {
		t.Fatalf("worktree not clean: %s", status)
	}
	retry, err := workflow.CompleteDeveloper(root)
	if err != nil || !retry.Committed || retry.Commit != after {
		t.Fatalf("clean completion retry=%#v err=%v", retry, err)
	}
	prompt, err := os.ReadFile(filepath.Join(result.RecordPath, "prompt.md"))
	if err != nil || !strings.Contains(string(prompt), "do not write Git") || !strings.Contains(string(prompt), "# Developer") {
		t.Fatalf("supervision appendix/core missing: err=%v", err)
	}
}

func TestSupervisedPartialWorkIsPreservedButForbiddenEffectsAreRejected(t *testing.T) {
	t.Run("partial", func(t *testing.T) {
		root := plannedGitFixture(t)
		installFakeCodex(t, supervisedOutcomeScript("printf 'partial\\n' > feature.txt", "failed-recoverable", "developer-remediation", "Developer resolves implementation evidence or escalates scope"))
		before := executionGit(t, root, "rev-parse", "HEAD")
		result, err := RunAccepted(context.Background(), root, Options{}, strings.NewReader(""), &strings.Builder{})
		if err != nil || !result.Reconciliation.ResultAccepted || result.Reconciliation.RetrySafe {
			t.Fatalf("result=%#v err=%v", result, err)
		}
		if executionGit(t, root, "rev-parse", "HEAD") != before {
			t.Fatal("partial outcome was committed")
		}
		if _, err := os.Stat(filepath.Join(root, "feature.txt")); err != nil {
			t.Fatal("valid partial work was not preserved")
		}
	})
	t.Run("forbidden", func(t *testing.T) {
		root := plannedGitFixture(t)
		installFakeCodex(t, supervisedOutcomeScript("printf '\\nforbidden\\n' >> .concoct/capabilities.md", "failed-recoverable", "developer-remediation", "Developer resolves implementation evidence or escalates scope"))
		result, err := RunAccepted(context.Background(), root, Options{}, strings.NewReader(""), &strings.Builder{})
		if err == nil || result.Reconciliation.ResultAccepted || !strings.Contains(err.Error(), "forbidden workflow path") {
			t.Fatalf("result=%#v err=%v", result, err)
		}
	})
	t.Run("forbidden non-Git", func(t *testing.T) {
		root := plannedNonGitFixture(t)
		installFakeCodex(t, supervisedOutcomeScript("printf '\\nforbidden\\n' >> .concoct/capabilities.md", "failed-recoverable", "developer-remediation", "Developer resolves implementation evidence or escalates scope"))
		result, err := RunAccepted(context.Background(), root, Options{}, strings.NewReader(""), &strings.Builder{})
		if err == nil || result.Reconciliation.ResultAccepted || !strings.Contains(err.Error(), "forbidden workflow path") {
			t.Fatalf("result accepted=%v disposition=%s err=%v", result.Reconciliation.ResultAccepted, result.Reconciliation.InvocationDisposition, err)
		}
	})
	t.Run("adapter commit", func(t *testing.T) {
		root := plannedGitFixture(t)
		installFakeCodex(t, supervisedOutcomeScript("printf 'committed\\n' > feature.txt\ngit add feature.txt\ngit commit -qm 'unauthorized adapter commit'", "completed", "", ""))
		result, err := RunAccepted(context.Background(), root, Options{}, strings.NewReader(""), &strings.Builder{})
		if err == nil || result.Reconciliation.ResultAccepted || !strings.Contains(err.Error(), "created or changed a Git commit") {
			t.Fatalf("result accepted=%v disposition=%s err=%v", result.Reconciliation.ResultAccepted, result.Reconciliation.InvocationDisposition, err)
		}
	})
	t.Run("outer commit failure", func(t *testing.T) {
		root := plannedGitFixture(t)
		realGit, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		installFakeCodex(t, supervisedOutcomeScript(`
sed -i '0,/status: planned/s//status: implementation-complete/' .concoct/current/task-plan.md
printf '\n## Handoff to reviewer\n\n### Implemented\n\nChanged.\n\n### Verification\n\nChecked.\n\n### Known risks\n\nNone.\n\n### Capability impact\n\nNone.\n\n### Suggested review focus\n\nSource.\n' >> .concoct/current/notes.md
printf 'implemented\n' > feature.txt
`, "completed", "", ""))
		adapterDir := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))[0]
		wrapper := "#!/bin/sh\nif [ \"$1\" = commit ]; then exit 9; fi\nexec " + shellQuote(realGit) + " \"$@\"\n"
		if err := os.WriteFile(filepath.Join(adapterDir, "git"), []byte(wrapper), 0o755); err != nil {
			t.Fatal(err)
		}
		before := executionGit(t, root, "rev-parse", "HEAD")
		result, runErr := RunAccepted(context.Background(), root, Options{}, strings.NewReader(""), &strings.Builder{})
		if runErr == nil || result.Reconciliation.ResultAccepted || !strings.Contains(runErr.Error(), "git commit") {
			t.Fatalf("result accepted=%v disposition=%s err=%v", result.Reconciliation.ResultAccepted, result.Reconciliation.InvocationDisposition, runErr)
		}
		if after := executionGit(t, root, "rev-parse", "HEAD"); after != before {
			t.Fatalf("failed outer commit moved HEAD from %s to %s", before, after)
		}
	})
}

func plannedGitFixture(t *testing.T) string {
	t.Helper()
	root := readyFixture(t)
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".concoct/roadmap.md", "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n## APP-001 — Demo\n- Status: `planned`\n- Depends on: `none`\n")
	executionGit(t, root, "init", "-q", "-b", "main")
	executionGit(t, root, "config", "user.email", "test@example.com")
	executionGit(t, root, "config", "user.name", "Test")
	executionGit(t, root, "add", "-A")
	executionGit(t, root, "commit", "-qm", "base")
	base := executionGit(t, root, "rev-parse", "HEAD")
	executionGit(t, root, "checkout", "-qb", "concoct/app-001-demo")
	write(".concoct/roadmap.md", "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n## APP-001 — Demo\n- Status: `active`\n- Depends on: `none`\n")
	write(".concoct/current/task-plan.md", "---\nid: APP-001\ntitle: Demo\nroadmap-id: APP-001\nstatus: planned\ncreated: 2026-01-01\nupdated: 2026-01-01\ngit:\n  enabled: true\n  trunk: main\n  task-branch: concoct/app-001-demo\n  base: "+base+"\n  status: active\ncapability-impact:\n  type: none\n  rationale: This fixture exercises workflow completion without changing product capability truth.\n---\n# Task Plan\n\n## Goal\n\nCreate `benchmark-result.txt` containing exactly `benchmark-complete` followed by a newline.\n\n## Constraints\n\nDo not change any other product file. Workflow evidence changes required by the Developer contract are allowed.\n\n## Acceptance criteria\n\n- `benchmark-result.txt` exists at the repository root.\n- Its complete byte content is `benchmark-complete\\n`.\n\n## Verification\n\nRun `test \"$(cat benchmark-result.txt)\" = benchmark-complete`.\n")
	write(".concoct/current/notes.md", "# Notes\n\nPlanned.\n")
	executionGit(t, root, "add", "-A")
	executionGit(t, root, "commit", "-qm", "concoct: plan APP-001")
	return root
}

func plannedNonGitFixture(t *testing.T) string {
	t.Helper()
	root := readyFixture(t)
	if err := os.WriteFile(filepath.Join(root, ".concoct", "roadmap.md"), []byte("---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n## APP-001 — Demo\n- Status: `active`\n- Depends on: `none`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := "---\nid: APP-001\ntitle: Demo\nroadmap-id: APP-001\nstatus: planned\ncreated: 2026-01-01\nupdated: 2026-01-01\ncapability-impact:\n  type: none\n  rationale: No impact.\n---\n# Task Plan\n"
	if err := os.WriteFile(filepath.Join(root, ".concoct", "current", "task-plan.md"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".concoct", "current", "notes.md"), []byte("# Notes\n\nPlanned.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func supervisedOutcomeScript(mutations, class, intervention, next string) string {
	return `schema=""
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

` + mutations + `
printf '{"protocol_version":"v1","correlation":{"invocation_id":"%s","action_id":"%s","task_id":"%s","attempt_id":"%s","role":"%s"},"class":"%s","summary":"candidate","artifacts":[],"intervention":{"kind":"%s","next":"%s"},"diagnostics":[],"recommendation":{"kind":"","command":"","reason":""}}\n' "$(value invocation_id)" "$(value action_id)" "$(value task_id)" "$(value attempt_id)" "$(value role)" ` + shellQuote(class) + ` ` + shellQuote(intervention) + ` ` + shellQuote(next) + ` > "$output"`
}

func TestLiveBenchmarkMetricsSurviveRejectedOutcome(t *testing.T) {
	root := plannedGitFixture(t)
	installFakeCodex(t, supervisedOutcomeScript(`
printf '{"type":"item.completed","item":{"type":"command_execution","command":"go test ./...","aggregated_output":"failed","exit_code":1,"status":"failed"}}\n'
printf '{"type":"turn.completed","usage":{"input_tokens":120,"cached_input_tokens":80,"output_tokens":9,"total_tokens":129}}\n'
`, "failed-recoverable", "developer-remediation", "Developer resolves implementation evidence or escalates scope"))
	result, runErr := Run(context.Background(), root, Options{}, strings.NewReader(""), &strings.Builder{})
	if runErr == nil || !strings.Contains(runErr.Error(), "accepted non-completion outcome") {
		t.Fatalf("expected rejected completion outcome, got %v", runErr)
	}
	evidenceDir := t.TempDir()
	metrics, err := retainLiveBenchmarkMetrics(evidenceDir, "developer", root, result)
	if err != nil {
		t.Fatalf("retain rejected benchmark metrics: %v", err)
	}
	for _, want := range []string{`"input_tokens": 120`, `"activity_events": 2`, `"result_accepted": true`, `"outcome_class": "failed-recoverable"`} {
		if !bytes.Contains(metrics, []byte(want)) {
			t.Fatalf("retained metrics lack %s: %s", want, metrics)
		}
	}
	if _, err := os.Stat(filepath.Join(evidenceDir, "developer-metrics.json")); err != nil {
		t.Fatalf("rejected benchmark metrics were not durably exported: %v", err)
	}
	if _, err := retainLiveBenchmarkMetrics(evidenceDir, "developer", root, result); err == nil || !strings.Contains(err.Error(), "without overwriting") {
		t.Fatalf("expected create-only evidence protection, got %v", err)
	}
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

func TestDirectIntegrationCancellationAndTimeoutCloseInspectably(t *testing.T) {
	for _, test := range []struct {
		name    string
		timeout string
		cancel  bool
		want    string
	}{
		{"cancelled", "5s", true, "cancelled"},
		{"timed out", "1s", false, "timed-out"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := archivedFixture(t)
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			remote := filepath.Join(t.TempDir(), "remote.git")
			executionCommand(t, "", "git", "init", "--bare", "-q", remote)
			executionGit(t, root, "remote", "add", "origin", remote)
			executionGit(t, root, "checkout", "main")
			executionGit(t, root, "push", "-qu", "origin", "main")
			executionGit(t, root, "checkout", "concoct/app-001-demo")

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader := &executionBlockingReader{started: make(chan struct{})}
			done := make(chan struct {
				result Result
				err    error
			}, 1)
			go func() {
				result, err := Run(ctx, root, Options{Override: config.Overrides{Timeout: test.timeout}}, reader, &strings.Builder{})
				done <- struct {
					result Result
					err    error
				}{result, err}
			}()
			select {
			case <-reader.started:
				if test.cancel {
					cancel()
				}
			case got := <-done:
				t.Fatalf("direct integration returned before push confirmation: result=%#v err=%v", got.result, got.err)
			case <-time.After(5 * time.Second):
				t.Fatal("direct integration did not reach push confirmation")
			}
			select {
			case got := <-done:
				if got.err == nil || got.result.Reconciliation.InvocationDisposition != test.want || got.result.Reconciliation.ResultAccepted {
					t.Fatalf("result=%#v err=%v", got.result, got.err)
				}
				if _, err := os.Stat(filepath.Join(got.result.RecordPath, "reconciliation.json")); err != nil {
					t.Fatalf("missing reconciliation: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("interrupted direct integration did not return")
			}
			if _, err := os.Stat(filepath.Join(root, ".git/concoct/integrations/APP-001.yaml")); err != nil {
				t.Fatalf("interrupted direct integration lost recovery evidence: %v", err)
			}
		})
	}
}

func TestInspectToleratesPartialRecordWithoutRegeneration(t *testing.T) {
	root := t.TempDir()
	id := strings.Repeat("a", 36)
	record := filepath.Join(root, ".concoct/runtime/invocations", id)
	if err := os.MkdirAll(record, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(record, "action.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Inspect(root, id)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "unavailable (partial or not produced)") || !strings.Contains(got, "## Action\n{}") {
		t.Fatalf("inspection = %s", got)
	}
}

func TestMetricsExportExcludesPromptAndRawEvents(t *testing.T) {
	root := t.TempDir()
	id := strings.Repeat("b", 36)
	record := filepath.Join(root, ".concoct/runtime/invocations", id)
	if err := os.MkdirAll(record, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"metadata.json":           `{"prompt_bytes":12}`,
		"prompt-composition.json": `{"components":[{"category":"persona","bytes":12}]}`,
		"measurement.json":        `{"usage":{"total_tokens":3},"progress":["OPENAI_API_KEY=sk-secret repository content"],"diagnostics":["repository content"]}`,
		"reconciliation.json":     `{"invocation_disposition":"completed"}`,
		"prompt.md":               "private prompt",
		"stdout.log":              "private event",
	} {
		if err := os.WriteFile(filepath.Join(record, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := Metrics(root, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private prompt", "private event", "prompt.md", "stdout.log", "sk-secret", "repository content"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("metrics export leaked %q: %s", forbidden, data)
		}
	}
	if !strings.Contains(string(data), `"prompt_bytes": 12`) || !strings.Contains(string(data), `"total_tokens": 3`) {
		t.Fatalf("metrics export omitted evidence: %s", data)
	}
	inspection, err := Inspect(root, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"sk-secret", "repository content"} {
		if strings.Contains(inspection, forbidden) {
			t.Fatalf("metrics-first inspection leaked %q: %s", forbidden, inspection)
		}
	}
}

func TestRedactionSpansWriterChunksAndLogBoundIsFinite(t *testing.T) {
	var retained, displayed strings.Builder
	w := newBoundedRedactedWriter(&retained, &displayed, 64)
	if _, err := w.Write([]byte("OPENAI_API_KEY=sk-super")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("secret123456\nmore output that is deliberately much longer than the configured retained bound\n")); err != nil {
		t.Fatal(err)
	}
	w.finish()
	if strings.Contains(retained.String(), "supersecret") || !strings.Contains(retained.String(), "[REDACTED]") {
		t.Fatalf("retained log = %q", retained.String())
	}
	if len(retained.String()) > 96 {
		t.Fatalf("bounded log grew unexpectedly: %d bytes", len(retained.String()))
	}
	if retained.String() != displayed.String() {
		t.Fatal("displayed and retained sanitized streams differ")
	}
}

func TestStructuredCaptureDecodesIncrementallyAndRedactsRawEvents(t *testing.T) {
	var retained, displayed strings.Builder
	capture := newStructuredCapture(&retained, &displayed, 128)
	if _, err := capture.Write([]byte(`{"type":"item.started","message":"OPENAI_API_KEY=sk-super`)); err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Write([]byte("secret123456789\"}\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := capture.Write([]byte(`{"type":"item.completed","usage":{"total_tokens":9}}`)); err != nil {
		t.Fatal(err)
	}
	evidence := capture.close()
	if evidence.Usage.Total == nil || *evidence.Usage.Total != 9 {
		t.Fatalf("late evidence = %#v", evidence)
	}
	if strings.Contains(retained.String(), "supersecret") || !strings.Contains(retained.String(), "[REDACTED]") {
		t.Fatalf("structured record = %q", retained.String())
	}
	if strings.Contains(displayed.String(), "supersecret") || strings.Contains(displayed.String(), "OPENAI_API_KEY") || !strings.Contains(displayed.String(), "activity=unknown") {
		t.Fatalf("unsafe structured progress = %q", displayed.String())
	}
	if len(displayed.String()) > 96 {
		t.Fatalf("unbounded structured progress = %d bytes", len(displayed.String()))
	}
}

func TestStructuredCaptureBoundsManyProgressEventsWithoutLosingUsage(t *testing.T) {
	var retained, displayed strings.Builder
	capture := newStructuredCapture(&retained, &displayed, 96)
	for i := 0; i < 100; i++ {
		if _, err := capture.Write([]byte(`{"type":"item.started","message":"very long repository content OPENAI_API_KEY=sk-secret"}` + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := capture.Write([]byte(`{"type":"turn.completed","usage":{"total_tokens":12}}`)); err != nil {
		t.Fatal(err)
	}
	evidence := capture.close()
	measurement, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Usage.Total == nil || *evidence.Usage.Total != 12 || !evidence.ProgressTruncated {
		t.Fatalf("evidence = %#v", evidence)
	}
	if strings.Contains(string(measurement), "sk-secret") || strings.Contains(string(measurement), "repository content") || len(measurement) > 2048 {
		t.Fatalf("unsafe or unbounded measurement = %q", measurement)
	}
	if strings.Contains(displayed.String(), "sk-secret") || len(displayed.String()) > 128 {
		t.Fatalf("unsafe or unbounded display = %q", displayed.String())
	}
}

func TestStructuredCaptureBoundsMalformedAndOversizedEvidence(t *testing.T) {
	var retained, displayed strings.Builder
	capture := newStructuredCapture(&retained, &displayed, 128)
	for i := 0; i < 100; i++ {
		if _, err := capture.Write([]byte("not-json\n")); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 20; i++ {
		if _, err := capture.Write([]byte(strings.Repeat("x", 32))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := capture.Write([]byte("\n" + `{"type":"turn.completed","usage":{"total_tokens":29}}`)); err != nil {
		t.Fatal(err)
	}
	evidence := capture.close()
	measurement, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Usage.Total == nil || *evidence.Usage.Total != 29 || !evidence.DiagnosticsTruncated || !evidence.EventTruncated {
		t.Fatalf("evidence = %#v", evidence)
	}
	if len(measurement) > 2048 {
		t.Fatalf("measurement = %d bytes: %s", len(measurement), measurement)
	}
	if len(retained.String()) > 192 {
		t.Fatalf("retained raw evidence = %d bytes", len(retained.String()))
	}
}

func TestStructuredProgressReportsLatestSemanticActivityAfterRollover(t *testing.T) {
	var retained, displayed strings.Builder
	capture := newStructuredCapture(&retained, &displayed, 96)
	for i := 0; i < 30; i++ {
		item := "agent_message"
		if i%2 == 1 {
			item = "file_change"
		}
		if _, err := capture.Write([]byte(`{"type":"item.completed","item":{"type":"` + item + `"}}` + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	evidence := capture.close()
	if !strings.Contains(displayed.String(), "structured progress truncated; latest activity=edit") || evidence.CurrentActivity != "edit" {
		t.Fatalf("display=%q evidence=%#v", displayed.String(), evidence)
	}
	if strings.Contains(displayed.String(), "item.completed") {
		t.Fatalf("generic lifecycle label leaked into semantic display: %q", displayed.String())
	}
}

func TestPrunePreservesActiveAndBoundsCompletedRecords(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, ".concoct/runtime/invocations")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two", "active"} {
		path := filepath.Join(base, id)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if id != "active" {
			if err := os.WriteFile(filepath.Join(path, "reconciliation.json"), []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Millisecond)
		}
	}
	retention := config.Retention{MaxCompleted: 1, MaxAge: 24 * time.Hour, MaxLogBytes: 1024, MaxTotal: 1024 * 1024}
	if err := Prune(root, retention, "active"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, "active")); err != nil {
		t.Fatal("active record was pruned")
	}
	remaining := 0
	for _, id := range []string{"one", "two"} {
		if _, err := os.Stat(filepath.Join(base, id)); err == nil {
			remaining++
		}
	}
	if remaining != 1 {
		t.Fatalf("completed records remaining = %d, want configured bound 1", remaining)
	}
}

func TestPruneEnforcesAgeAndTotalWhilePreservingCurrent(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, ".concoct/runtime/invocations")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"old", "other", "current"} {
		path := filepath.Join(base, id)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		reconciliation := filepath.Join(path, "reconciliation.json")
		if err := os.WriteFile(reconciliation, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "payload"), []byte(strings.Repeat("x", 128)), 0o600); err != nil {
			t.Fatal(err)
		}
		if id == "old" {
			past := time.Now().Add(-48 * time.Hour)
			if err := os.Chtimes(reconciliation, past, past); err != nil {
				t.Fatal(err)
			}
		}
		time.Sleep(time.Millisecond)
	}
	retention := config.Retention{MaxCompleted: 10, MaxAge: 24 * time.Hour, MaxLogBytes: 64, MaxTotal: 180}
	if err := Prune(root, retention, "current"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"old", "other"} {
		if _, err := os.Stat(filepath.Join(base, id)); !os.IsNotExist(err) {
			t.Fatalf("%s was not pruned: %v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(base, "current")); err != nil {
		t.Fatal("current closed record was pruned")
	}
}

func readyFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{".concoct/current", ".concoct/archive"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("AGENTS.md", "---\ninstruction-layer: project-guidance\n---\n# Agents\n")
	write(".gitignore", ".concoct/runtime/invocations/\n")
	write(".concoct/policy.md", "---\ninstruction-layer: policy\nrequired-phases:\n  - product-ownership\n  - task-planning\n  - development\n  - independent-review\n  - archival\n  - integration\napproval-gates:\n  - reviewer-approval-before-archive\n  - archive-before-integration\ngit-strategy: task-branch-with-squash-integration\n---\n# Policy\n")
	write(".concoct/roadmap.md", "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n")
	write(".concoct/capabilities.md", "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Capabilities\n")
	write(".concoct/current/task-plan.md", "# task-plan.md\n")
	write(".concoct/current/notes.md", "# notes.md\n_Add decisions here._\n_Record meaningful verification results here._\n")
	return root
}

func planningPromptFixture(t *testing.T) string {
	t.Helper()
	root := readyFixture(t)
	roadmap := "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n\n## APP-001 — Demo\n\n- Status: `planned`\n- Depends on: `none`\n"
	if err := os.WriteFile(filepath.Join(root, ".concoct", "roadmap.md"), []byte(roadmap), 0o644); err != nil {
		t.Fatal(err)
	}
	executionGit(t, root, "init", "-q", "-b", "main")
	executionGit(t, root, "config", "user.email", "test@example.com")
	executionGit(t, root, "config", "user.name", "Test")
	executionGit(t, root, "add", "-A")
	executionGit(t, root, "commit", "-qm", "base")
	return root
}

func supervisedPromptFixture(t *testing.T, state string) string {
	t.Helper()
	root := plannedGitFixture(t)
	if state == "planned" {
		return root
	}
	planPath := filepath.Join(root, ".concoct", "current", "task-plan.md")
	plan, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	plan = []byte(strings.Replace(string(plan), "status: planned", "status: implementation-complete", 1))
	if err := os.WriteFile(planPath, plan, 0o644); err != nil {
		t.Fatal(err)
	}
	notesPath := filepath.Join(root, ".concoct", "current", "notes.md")
	notes, err := os.ReadFile(notesPath)
	if err != nil {
		t.Fatal(err)
	}
	handoff := "\n## Handoff to reviewer\n\n### Implemented\n\nDemo.\n\n### Verification\n\nPassed.\n\n### Known risks\n\nNone.\n\n### Capability impact\n\nNone.\n\n### Suggested review focus\n\nDemo.\n"
	if err := os.WriteFile(notesPath, append(notes, []byte(handoff)...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "benchmark-result.txt"), []byte("benchmark-complete\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	executionGit(t, root, "add", "-A")
	executionGit(t, root, "commit", "-qm", "concoct: complete APP-001 implementation")
	reservation := "---\ntask-id: APP-001\nreview: 1\nstatus: reserved\ncreated: 2026-08-14\npersona: reviewer\n---\n\n# Review 01\n\n<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->\n"
	if err := os.WriteFile(filepath.Join(root, ".concoct", "current", "review-01.md"), []byte(reservation), 0o644); err != nil {
		t.Fatal(err)
	}
	executionGit(t, root, "add", "-A")
	executionGit(t, root, "commit", "-qm", "benchmark review reservation")
	if state == "implementation-complete" {
		return root
	}
	if state != "approved" {
		t.Fatalf("unsupported supervised prompt state %q", state)
	}
	review := "---\ntask-id: APP-001\nreview: 1\nstatus: approved\ncreated: 2026-01-01\npersona: reviewer\n---\n# Review 01\n\n<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->\n\n## Outcome\n\n`approved`\n"
	if err := os.WriteFile(filepath.Join(root, ".concoct", "current", "review-01.md"), []byte(review), 0o644); err != nil {
		t.Fatal(err)
	}
	executionGit(t, root, "add", "-A")
	executionGit(t, root, "commit", "-qm", "concoct: review APP-001 approved")
	return root
}

func installFakeCodex(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "codex")
	script := "#!/bin/sh\nset -eu\nif [ \"${1:-}\" = \"--version\" ]; then\n  printf 'codex test adapter 0.0.0\\n'\n  exit 0\nfi\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func archivedFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{".concoct/current", ".concoct/archive/2026-01-01-APP-001-demo"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("AGENTS.md", "---\ninstruction-layer: project-guidance\n---\n# Agents\n")
	write(".gitignore", ".concoct/runtime/invocations/\n")
	write(".concoct/policy.md", "---\ninstruction-layer: policy\nrequired-phases:\n  - product-ownership\n  - task-planning\n  - development\n  - independent-review\n  - archival\n  - integration\napproval-gates:\n  - reviewer-approval-before-archive\n  - archive-before-integration\ngit-strategy: task-branch-with-squash-integration\n---\n# Policy\n")
	write(".concoct/capabilities.md", "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Capabilities\n")
	write(".concoct/roadmap.md", "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n## APP-001 — Demo\n- Status: `active`\n")
	executionGit(t, root, "init", "-q", "-b", "main")
	executionGit(t, root, "config", "user.email", "test@example.com")
	executionGit(t, root, "config", "user.name", "Test")
	executionGit(t, root, "add", "-A")
	executionGit(t, root, "commit", "-qm", "base")
	base := executionGit(t, root, "rev-parse", "HEAD")
	executionGit(t, root, "checkout", "-qb", "concoct/app-001-demo")
	plan := func(archive string) string {
		return "---\nid: APP-001\ntitle: Demo\nroadmap-id: APP-001\nstatus: implementation-complete\ncreated: 2026-01-01\nupdated: 2026-01-01\ncapability-impact:\n  type: none\n  rationale: No impact\ngit:\n  enabled: true\n  trunk: main\n  task-branch: concoct/app-001-demo\n  base: " + base + "\n  archive-commit: " + archive + "\n  status: archived\n---\n# Task\n"
	}
	write(".concoct/current/task-plan.md", plan(""))
	write(".concoct/current/notes.md", "# Notes\n")
	write(".concoct/current/review-01.md", "---\ntask-id: APP-001\nreview: 1\nstatus: approved\ncreated: 2026-01-01\npersona: reviewer\n---\n# Review\n## Outcome\n`approved`\n")
	write(".concoct/archive/2026-01-01-APP-001-demo/summary.md", "# Delivered\n")
	executionGit(t, root, "add", "-A")
	executionGit(t, root, "commit", "-qm", "archive")
	archive := executionGit(t, root, "rev-parse", "HEAD")
	write(".concoct/current/task-plan.md", plan(archive))
	executionGit(t, root, "add", "-A")
	executionGit(t, root, "commit", "-qm", "record archive commit")
	return root
}

func executionGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	return executionCommand(t, root, "git", args...)
}

func executionCommand(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v: %s", name, strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

type executionBlockingReader struct {
	started chan struct{}
	once    sync.Once
}

func (r *executionBlockingReader) Read([]byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	select {}
}
