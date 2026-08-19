package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/orchestration"
	"github.com/gopher-launch/concoct/internal/project"
	"github.com/gopher-launch/concoct/internal/runstate"
	"github.com/gopher-launch/concoct/internal/workflow"
)

func TestPromptStdoutAndFileOutputAreIdenticalAndNonDestructive(t *testing.T) {
	parent := t.TempDir()
	var initOutput bytes.Buffer
	if err := project.Initialize(parent, "demo", &initOutput); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	nested := filepath.Join(root, "doc", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONCOCT_CALLER_DIR", nested)

	before := workflowSnapshot(t, root)
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"next"}, &stdout, &stderr); err != nil {
		t.Fatalf("render stdout: %v (%s)", err, stderr.String())
	}
	output := filepath.Join(t.TempDir(), "next-prompt.md")
	if err := Run([]string{"next", "--output", output}, &bytes.Buffer{}, &stderr); err != nil {
		t.Fatal(err)
	}
	fileBytes, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), fileBytes) {
		t.Fatal("stdout and file output differ")
	}
	if before != workflowSnapshot(t, root) {
		t.Fatal("prompt rendering changed workflow artifacts")
	}
	if err := Run([]string{"next", "--output", output}, &bytes.Buffer{}, &stderr); err == nil || !strings.Contains(err.Error(), "without overwriting") {
		t.Fatalf("existing output error = %v", err)
	}
	if got, _ := os.ReadFile(output); !bytes.Equal(got, fileBytes) {
		t.Fatal("existing output was modified")
	}
}

func TestPromptArgumentValidation(t *testing.T) {
	tests := [][]string{{"plan"}, {"next", "extra"}, {"code", "extra"}, {"review", "--output"}, {"roadmap", "--output", "a", "--output", "b"}}
	for _, args := range tests {
		if err := Run(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Errorf("Run(%v) succeeded", args)
		}
	}
}

func TestExecDryRunIsInspectablyNonMutating(t *testing.T) {
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CONCOCT_CALLER_DIR", root)
	before := workflowSnapshot(t, root)
	var out bytes.Buffer
	if err := Run([]string{"exec", "--dry-run", "--reasoning", "high", "--timeout", "5m"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Action: product-owner-next", "Adapter: codex", "Reasoning: high (invocation override)", "workspace-write", "no process started"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("dry-run output lacks %q:\n%s", want, out.String())
		}
	}
	if before != workflowSnapshot(t, root) {
		t.Fatal("dry-run changed workflow evidence")
	}
	if _, err := os.Stat(filepath.Join(root, ".concoct/runtime")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created runtime: %v", err)
	}
}

func TestExecDisplaysTurnFailedDiagnosticAndCorrectiveAction(t *testing.T) {
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	bin := t.TempDir()
	script := "#!/bin/sh\nif [ \"${1:-}\" = \"--version\" ]; then echo codex-test; exit 0; fi\nprintf '%s\\n' '{\"type\":\"turn.started\"}' '{\"type\":\"turn.failed\",\"error\":{\"type\":\"invalid_json_schema\",\"message\":\"mutations must be required\"}}'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CONCOCT_CALLER_DIR", root)
	var out bytes.Buffer
	err := Run([]string{"exec"}, &out, &bytes.Buffer{})
	if err == nil {
		t.Fatal("failed Codex turn unexpectedly succeeded")
	}
	for _, want := range []string{"Codex failure: invalid_json_schema: mutations must be required", "Corrective action: correct the reported schema or configuration defect", "Retry safe from original evidence: false"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("exec output lacks %q: %s", want, out.String())
		}
	}
}

func TestExecDryRunHonorsIndependentReviewPolicyRoutes(t *testing.T) {
	for _, disposition := range []string{"required", "not-required", "externally-satisfied"} {
		t.Run(disposition, func(t *testing.T) {
			root := transitionProject(t, false)
			writeTransitionTaskWithBase(t, root, "implementation-complete", "", false)
			if disposition == "not-required" {
				path := filepath.Join(root, ".concoct/policy.md")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				data = []byte(strings.Replace(strings.Replace(string(data), "  - independent-review\n  - archival\n  - integration\n", "  - archival\n  - integration\nnot-required-reasons:\n  - independent-review: repository accepts developer verification\n", 1), "  - reviewer-approval-before-archive\n", "", 1))
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if disposition == "externally-satisfied" {
				if err := os.WriteFile(filepath.Join(root, "audit.md"), []byte("accepted external review\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, ".concoct/current/task-plan.md")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				evidence := "policy-activity-evidence:\n  - activity: independent-review\n    disposition: externally-satisfied\n    reason: external audit accepted the change\n    recorded-by: developer\n    evidence:\n      - audit.md\n"
				data = []byte(strings.Replace(string(data), "capability-impact:\n", evidence+"capability-impact:\n", 1))
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			bin := t.TempDir()
			if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("CONCOCT_CALLER_DIR", root)
			var out bytes.Buffer
			if err := Run([]string{"exec", "--dry-run"}, &out, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			want := "Action: archival"
			if disposition == "required" {
				want = "Action: independent-review"
			}
			if !strings.Contains(out.String(), want) {
				t.Fatalf("dry-run output lacks %q:\n%s", want, out.String())
			}
		})
	}
}

func TestLegacyProjectRejectsWorkflowCommandBeforeOutput(t *testing.T) {
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	if err := os.Remove(filepath.Join(root, ".concoct", "project.yaml")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONCOCT_CALLER_DIR", root)
	output := filepath.Join(root, "should-not-exist.md")
	err := Run([]string{"next", "--output", output}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "legacy/unversioned") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("workflow command wrote output: %v", err)
	}
}

func TestMalformedProjectRecordsUseReducedStatusAndRejectWorkflowOutput(t *testing.T) {
	for name, record := range map[string]string{
		"incomplete":     "contract-version: 1\n",
		"empty":          "",
		"invalid":        "contract-version: [\n",
		"multi-document": "contract-version: 1\ncreated-with: {version: development, revision: unknown}\nlast-upgraded-with: {version: development, revision: unknown}\n---\ncontract-version: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(parent, "demo")
			if err := os.WriteFile(filepath.Join(root, ".concoct", "project.yaml"), []byte(record), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CONCOCT_CALLER_DIR", root)
			var status bytes.Buffer
			if err := Run([]string{"status"}, &status, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(status.String(), "Project contract: incompatible") {
				t.Fatalf("status = %q", status.String())
			}
			output := filepath.Join(root, "should-not-exist.md")
			err := Run([]string{"next", "--output", output}, &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), "malformed project contract") {
				t.Fatalf("next error = %v", err)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("workflow command wrote output: %v", err)
			}
		})
	}
}

func TestStatusAppliedSelectionDefersToPlannedTaskContinuation(t *testing.T) {
	root := transitionProject(t, true)
	decision, err := runstate.NewDecision(
		orchestration.ProductDecision{
			Version:   1,
			Kind:      orchestration.DecisionSelect,
			Selection: "APP-001",
			Rationale: "Plan the selected item.",
		},
		orchestration.Evidence{Digest: "ready-state-evidence", State: workflow.Ready},
		orchestration.Correlation{
			InvocationID: "invocation-1",
			ActionID:     "action-1",
			AttemptID:    "attempt-1",
			Role:         "product-owner",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	decision.Status = "applied"
	if err := runstate.CreateDecision(root, decision); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONCOCT_CALLER_DIR", root)

	var status bytes.Buffer
	if err := Run([]string{"status"}, &status, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status.String(), "Next: concoct code") {
		t.Fatalf("status did not retain planned continuation:\n%s", status.String())
	}
	if strings.Contains(status.String(), "Next: concoct plan APP-001") {
		t.Fatalf("status repeated applied selection:\n%s", status.String())
	}
}

func TestStatusRetainedNonSelectionDecisionsDeferToPlannedTaskContinuation(t *testing.T) {
	tests := []struct {
		name     string
		decision orchestration.ProductDecision
		status   string
	}{
		{
			name: "proposed reconcile",
			decision: orchestration.ProductDecision{
				Version: 1, Kind: orchestration.DecisionReconcile, Rationale: "Reconcile accepted delivery.", RoadmapDigest: "ready-state-evidence",
			},
			status: "proposed",
		},
		{
			name: "approved reconcile and select",
			decision: orchestration.ProductDecision{
				Version: 1, Kind: orchestration.DecisionReconcileAndSelect, Selection: "APP-001", Rationale: "Reconcile then plan the selected item.", RoadmapDigest: "ready-state-evidence",
			},
			status: "approved",
		},
		{
			name: "human decision required",
			decision: orchestration.ProductDecision{
				Version: 1, Kind: orchestration.DecisionHumanRequired, Rationale: "Choose between incompatible product outcomes.",
			},
			status: "proposed",
		},
		{
			name: "no action",
			decision: orchestration.ProductDecision{
				Version: 1, Kind: orchestration.DecisionNoAction, Rationale: "No actionable roadmap work remains.",
			},
			status: "proposed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := transitionProject(t, true)
			decision, err := runstate.NewDecision(tt.decision, orchestration.Evidence{Digest: "ready-state-evidence", State: workflow.Ready}, orchestration.Correlation{
				InvocationID: "invocation-1", ActionID: "action-1", AttemptID: "attempt-1", Role: "product-owner",
			})
			if err != nil {
				t.Fatal(err)
			}
			decision.Status = tt.status
			if err := runstate.CreateDecision(root, decision); err != nil {
				t.Fatal(err)
			}
			t.Setenv("CONCOCT_CALLER_DIR", root)

			var output bytes.Buffer
			if err := Run([]string{"status"}, &output, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "Next: concoct code") {
				t.Fatalf("status did not retain planned continuation:\n%s", output.String())
			}
		})
	}
}

func TestPlanCreatesDeterministicTaskBranchAndRefusesCollision(t *testing.T) {
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	road := filepath.Join(root, ".concoct/roadmap.md")
	f, err := os.OpenFile(road, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n## APP-001 — Branch Demo\n\n- Status: `planned`\n- Depends on: `none`\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-qm", "initial")
	trunk := gitOutput(t, root, "branch", "--show-current")
	t.Setenv("CONCOCT_CALLER_DIR", root)
	var out bytes.Buffer
	if err := Run([]string{"plan", "APP-001"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if branch := gitOutput(t, root, "branch", "--show-current"); branch != "concoct/app-001-branch-demo" {
		t.Fatalf("branch = %s", branch)
	}
	if !strings.Contains(out.String(), "Git trunk:") || !strings.Contains(out.String(), "Git task base:") {
		t.Fatal("prompt lacks recorded Git start")
	}
	runGit(t, root, "checkout", trunk)
	before := gitOutput(t, root, "rev-parse", "HEAD")
	if err := Run([]string{"plan", "APP-001"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("error = %v", err)
	}
	if gitOutput(t, root, "branch", "--show-current") != trunk || gitOutput(t, root, "rev-parse", "HEAD") != before {
		t.Fatal("collision changed checkout")
	}
}

func TestPlanRejectsCapabilityPrerequisiteBeforeBranchCreation(t *testing.T) {
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	road := filepath.Join(root, ".concoct/roadmap.md")
	f, err := os.OpenFile(road, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("\n## APP-404 — Missing Capability\n\n- Status: `planned`\n- Depends on: `none`\n- Capability prerequisites: CAP-404\n")
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-qm", "initial")
	trunk := gitOutput(t, root, "branch", "--show-current")
	head := gitOutput(t, root, "rev-parse", "HEAD")
	t.Setenv("CONCOCT_CALLER_DIR", root)
	err = Run([]string{"plan", "APP-404"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "CAP-404 is missing") {
		t.Fatalf("error = %v", err)
	}
	if gitOutput(t, root, "branch", "--show-current") != trunk || gitOutput(t, root, "rev-parse", "HEAD") != head {
		t.Fatal("prerequisite failure changed Git boundary")
	}
	if strings.Contains(gitOutput(t, root, "branch", "--format=%(refname:short)"), "concoct/app-404") {
		t.Fatal("prerequisite failure created task branch")
	}
}

func runGit(t *testing.T, root string, args ...string) { t.Helper(); _ = gitOutput(t, root, args...) }
func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func workflowSnapshot(t *testing.T, root string) string {
	t.Helper()
	var result strings.Builder
	base := filepath.Join(root, ".concoct")
	if err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			result.WriteString(path)
			result.WriteByte(':')
			result.Write(data)
			result.WriteByte('\n')
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result.String()
}
