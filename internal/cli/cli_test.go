package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/project"
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
