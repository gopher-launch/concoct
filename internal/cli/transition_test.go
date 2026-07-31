package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/project"
)

func TestGitDeveloperAndReviewerCompletionLoop(t *testing.T) {
	root := transitionProject(t, true)
	t.Setenv("CONCOCT_CALLER_DIR", root)

	writeTransitionTask(t, root, "implementation-complete")
	if err := os.WriteFile(filepath.Join(root, ".concoct/current/notes.md"), []byte(reviewerHandoff+"\nInitial implementation complete.\n\n"+freshReviewerHandoff("Initial implementation.")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "implementation.txt"), []byte("done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Run([]string{"code", "--complete"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	first := gitOutput(t, root, "rev-parse", "HEAD")
	if !strings.Contains(out.String(), "implementation-complete") || gitOutput(t, root, "status", "--short") != "" {
		t.Fatalf("completion output/status = %q", out.String())
	}
	if err := Run([]string{"code", "--complete"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if got := gitOutput(t, root, "rev-parse", "HEAD"); got != first {
		t.Fatal("developer retry created a duplicate commit")
	}

	if err := Run([]string{"review", "--reserve"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	reviewPath := filepath.Join(root, ".concoct/current/review-01.md")
	data, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatal(err)
	}
	completed := strings.Replace(string(data), "status: reserved", "status: changes-requested", 1) + "\n## Findings\n\n### Finding 1\n\nFix it.\n\n## Outcome\n\n`changes-requested`\n"
	if err := os.WriteFile(reviewPath, []byte(completed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"review", "--complete"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "review-changes-requested") || gitOutput(t, root, "status", "--short") != "" {
		t.Fatalf("review output/status = %q", out.String())
	}

	writeTransitionTask(t, root, "implementation-complete")
	plan := filepath.Join(root, ".concoct/current/task-plan.md")
	b, _ := os.ReadFile(plan)
	b = bytes.Replace(b, []byte("updated: 2026-07-31\n"), []byte("updated: 2026-08-01\nremediates-review: review-01.md\n"), 1)
	if err := os.WriteFile(plan, b, 0o644); err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(root, ".concoct/current/notes.md")
	if err := os.WriteFile(notes, []byte(reviewerHandoff+"\nFixed finding 1.\n\n"+freshReviewerHandoff("Remediation complete.")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"code", "--complete"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"review", "--reserve"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".concoct/current/review-02.md")); err != nil {
		t.Fatal("second review was not numbered deterministically")
	}
}

func TestDeveloperCompletionRejectsUnrelatedNotesEditWithStaleHandoff(t *testing.T) {
	root := transitionProject(t, true)
	writeTransitionTask(t, root, "implementation-complete")
	notes := filepath.Join(root, ".concoct/current/notes.md")
	if err := os.WriteFile(notes, []byte("# Notes\n\nUnrelated note edit.\n\n"+strings.TrimPrefix(reviewerHandoff, "# Notes\n\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONCOCT_CALLER_DIR", root)
	err := Run([]string{"code", "--complete"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "handoff is unchanged from HEAD") {
		t.Fatalf("error = %v", err)
	}
}

func TestReviewReservationValidatesGitEntryBoundary(t *testing.T) {
	tests := []struct {
		name          string
		breakBoundary func(*testing.T, string)
		want          string
	}{
		{name: "valid clean path"},
		{name: "dirty worktree", breakBoundary: func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, want: "worktree is not clean"},
		{name: "wrong branch", breakBoundary: func(t *testing.T, root string) {
			runGit(t, root, "checkout", "-qb", "wrong-branch")
		}, want: "checkout drift"},
		{name: "detached head", breakBoundary: func(t *testing.T, root string) {
			runGit(t, root, "checkout", "--detach", "-q")
		}, want: "detached HEAD is unsafe"},
		{name: "operation in progress", breakBoundary: func(t *testing.T, root string) {
			head := gitOutput(t, root, "rev-parse", "HEAD")
			if err := os.WriteFile(filepath.Join(root, ".git/MERGE_HEAD"), []byte(head+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, want: "operation is in progress"},
		{name: "invalid recorded base", breakBoundary: func(t *testing.T, root string) {
			plan := filepath.Join(root, ".concoct/current/task-plan.md")
			data, err := os.ReadFile(plan)
			if err != nil {
				t.Fatal(err)
			}
			base := gitOutput(t, root, "merge-base", "HEAD", "main")
			data = bytes.Replace(data, []byte(base), []byte(strings.Repeat("0", 40)), 1)
			if err := os.WriteFile(plan, data, 0o644); err != nil {
				t.Fatal(err)
			}
			runGit(t, root, "add", plan)
			runGit(t, root, "commit", "-qm", "test invalid recorded base")
		}, want: "recorded Git base is not an ancestor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := transitionProject(t, true)
			writeTransitionTask(t, root, "implementation-complete")
			runGit(t, root, "add", "-A")
			runGit(t, root, "commit", "-qm", "concoct: complete APP-001 implementation")
			if tt.breakBoundary != nil {
				tt.breakBoundary(t, root)
			}
			t.Setenv("CONCOCT_CALLER_DIR", root)
			err := Run([]string{"review", "--reserve"}, &bytes.Buffer{}, &bytes.Buffer{})
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(root, ".concoct/current/review-01.md")); err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if _, statErr := os.Stat(filepath.Join(root, ".concoct/current/review-01.md")); !os.IsNotExist(statErr) {
				t.Fatalf("reservation was created at an invalid boundary: %v", statErr)
			}
		})
	}
}

func TestReviewCompletionRejectsDeveloperOwnedMutation(t *testing.T) {
	root := transitionProject(t, true)
	writeTransitionTask(t, root, "implementation-complete")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-qm", "concoct: complete APP-001 implementation")
	t.Setenv("CONCOCT_CALLER_DIR", root)
	if err := Run([]string{"review", "--reserve"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "implementation.txt"), []byte("reviewer edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"review", "--complete"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "only the one reserved") {
		t.Fatalf("error = %v", err)
	}
}

func TestNonGitCompletionUsesArtifactSemanticsWithoutCommit(t *testing.T) {
	root := transitionProject(t, true)
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	writeTransitionTaskWithBase(t, root, "implementation-complete", "", false)
	if err := os.WriteFile(filepath.Join(root, ".concoct/current/notes.md"), []byte(reviewerHandoff+"\nNon-Git completion.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONCOCT_CALLER_DIR", root)
	var out bytes.Buffer
	if err := Run([]string{"code", "--complete"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Commit:") {
		t.Fatal("non-Git completion fabricated a commit")
	}
	if err := Run([]string{"review", "--reserve"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".concoct/current/review-01.md")
	b, _ := os.ReadFile(path)
	completed := strings.Replace(string(b), "status: reserved", "status: approved", 1) + "\n## Outcome\n\n`approved`\n"
	if err := os.WriteFile(path, []byte(completed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"review", "--complete"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "review-approved") {
		t.Fatalf("output = %q", out.String())
	}
}

const reviewerHandoff = `# Notes

## Handoff to reviewer

### Implemented
Done.
### Key decisions
Kept narrow.
### Files changed
Task files.
### Verification
Tests passed.
### Known risks
None.
### Skipped or unresolved work
None.
### Capability impact
Adds coordination.
### Suggested review focus
Transitions.`

func freshReviewerHandoff(implemented string) string {
	return strings.Replace(reviewerHandoff, "# Notes\n\n", "", 1) + "\n" + implemented
}

func transitionProject(t *testing.T, gitBacked bool) string {
	t.Helper()
	parent := t.TempDir()
	if err := project.Initialize(parent, "demo", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "demo")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	road := filepath.Join(root, ".concoct/roadmap.md")
	f, _ := os.OpenFile(road, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("\n## APP-001 — Transition\n\n- Status: `active`\n- Depends on: None\n")
	_ = f.Close()
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-qm", "base")
	base := gitOutput(t, root, "rev-parse", "HEAD")
	runGit(t, root, "checkout", "-qb", "concoct/app-001-transition")
	writeTransitionTaskWithBase(t, root, "planned", base, gitBacked)
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-qm", "concoct: plan APP-001")
	return root
}

func writeTransitionTask(t *testing.T, root, status string) {
	t.Helper()
	base := gitOutput(t, root, "merge-base", "HEAD", "main")
	writeTransitionTaskWithBase(t, root, status, base, true)
}
func writeTransitionTaskWithBase(t *testing.T, root, status, base string, enabled bool) {
	t.Helper()
	gitBlock := ""
	if enabled {
		gitBlock = fmt.Sprintf("git:\n  enabled: true\n  trunk: main\n  task-branch: concoct/app-001-transition\n  base: %s\n  status: active\n", base)
	}
	task := fmt.Sprintf("---\nid: APP-001\ntitle: Transition\nroadmap-id: APP-001\nstatus: %s\ncreated: 2026-07-31\nupdated: 2026-07-31\n%scapability-impact:\n  type: none\n  rationale: No capability ledger change in fixture.\n---\n\n# Task Plan\n", status, gitBlock)
	if err := os.WriteFile(filepath.Join(root, ".concoct/current/task-plan.md"), []byte(task), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".concoct/current/notes.md"), []byte(reviewerHandoff), 0o644); err != nil {
		t.Fatal(err)
	}
}
