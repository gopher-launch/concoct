package project

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/instruction"
	"github.com/gopher-launch/concoct/internal/workflow"
)

func TestInitializeEndToEnd(t *testing.T) {
	parent := t.TempDir()
	var out bytes.Buffer
	if err := Initialize(parent, "demo", &out); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	for _, path := range []string{"AGENTS.md", ".aider.conf.yml", ".codex/skills/concoct/SKILL.md", ".concoct/protocol.md", ".concoct/policy.md", ".concoct/personas/developer.md", ".concoct/prompts/handoffs/task-planner-to-developer.md", ".concoct/current/bootstrap-prompt.md"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}
	if err := filepath.Walk(filepath.Join("..", "..", "templates"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(filepath.Join("..", "..", "templates"), path)
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("template path not copied: %s", rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := workflow.Detect(root); got.State != workflow.Ready {
		t.Fatalf("state %s: %v", got.State, got.Diagnostics)
	}
	nested := filepath.Join(root, "some", "nested", "directory")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if got, err := Discover(nested); err != nil || got != root {
		t.Fatalf("Discover = %q, %v", got, err)
	}
	status := git(t, root, "status", "--short")
	if !strings.Contains(status, "A  .aider.conf.yml") || !strings.Contains(status, "A  .concoct/current/bootstrap-prompt.md") {
		t.Fatalf("files not staged:\n%s", status)
	}
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = root
	if err := cmd.Run(); err == nil {
		t.Fatal("init unexpectedly created a commit")
	}
	if !strings.Contains(out.String(), "staged (no commit created)") {
		t.Fatalf("output omitted staging decision: %s", out.String())
	}
	if strings.Count(out.String(), "Next: concoct next") != 1 || strings.Contains(out.String(), "concoct roadmap or concoct plan") {
		t.Fatalf("initialization output does not recommend exactly concoct next: %s", out.String())
	}
	bootstrap, err := os.ReadFile(filepath.Join(root, ".concoct/current/bootstrap-prompt.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(bootstrap), "Recommended next command: `concoct next`") != 1 || strings.Contains(string(bootstrap), "concoct roadmap or concoct plan") {
		t.Fatalf("bootstrap does not recommend exactly concoct next: %s", bootstrap)
	}
}

func TestInitializedGuidanceComposesWithoutMutation(t *testing.T) {
	parent := t.TempDir()
	if err := Initialize(parent, "layered", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "layered")
	path := filepath.Join(root, "AGENTS.md")
	custom := []byte("---\ninstruction-layer: project-guidance\nstrengthen-controls:\n  - evidence-integrity\n---\n# Custom project guidance\nexact bytes\n")
	if err := os.WriteFile(path, custom, 0644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if _, err := instruction.Compose(root); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("composition changed project-owned guidance")
	}
}

func TestInitializeRefusesExistingAndUnsafeTargets(t *testing.T) {
	parent := t.TempDir()
	existing := filepath.Join(parent, "existing")
	os.Mkdir(existing, 0755)
	for _, target := range []string{"existing", ".", "..", ""} {
		if err := Initialize(parent, target, &bytes.Buffer{}); err == nil {
			t.Errorf("target %q unexpectedly accepted", target)
		}
	}
}
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}
