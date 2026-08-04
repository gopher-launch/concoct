package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/project"
)

func TestGitArchiveCompletionIntegratesExactArchivedHead(t *testing.T) {
	root, _ := prepareGitArchiveCandidate(t)
	var out bytes.Buffer
	if err := Run([]string{"archive", "--complete"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	archiveHead := gitOutput(t, root, "rev-parse", "HEAD")
	if !strings.Contains(out.String(), "Git archive commit: "+archiveHead) {
		t.Fatalf("output = %s", out.String())
	}
	if err := Run([]string{"archive", "--complete"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("exact archive retry failed: %v", err)
	}
	if got := gitOutput(t, root, "rev-parse", "HEAD"); got != archiveHead {
		t.Fatal("archive retry created a duplicate commit")
	}
	if err := Run([]string{"integrate"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if branch := gitOutput(t, root, "branch", "--show-current"); branch != "main" {
		t.Fatalf("branch = %s", branch)
	}
}

func TestGitArchiveCleanRetryRejectsInvalidCommittedTransition(t *testing.T) {
	tests := []struct {
		name, want string
		corrupt    func(*testing.T, string)
	}{
		{name: "subject prefix", want: "not the exact archive transition commit", corrupt: func(t *testing.T, root string) {
			runGit(t, root, "commit", "--amend", "-qm", "concoct: archive APP-001 extra")
		}},
		{name: "unrelated roadmap edit", want: "committed archive roadmap transition is invalid", corrupt: func(t *testing.T, root string) {
			path := filepath.Join(root, ".concoct/roadmap.md")
			data, _ := os.ReadFile(path)
			writeArchiveTestFile(t, path, strings.Replace(string(data), "# Roadmap", "# Changed roadmap", 1))
			runGit(t, root, "add", path)
			runGit(t, root, "commit", "--amend", "--no-edit", "-q")
		}},
		{name: "capability prose edit", want: "committed archive capability transition is invalid", corrupt: func(t *testing.T, root string) {
			path := filepath.Join(root, ".concoct/capabilities.md")
			data, _ := os.ReadFile(path)
			writeArchiveTestFile(t, path, strings.Replace(string(data), "# Capabilities", "# Changed capabilities", 1))
			runGit(t, root, "add", path)
			runGit(t, root, "commit", "--amend", "--no-edit", "-q")
		}},
		{name: "non archived state", want: "requires git.status archived", corrupt: func(t *testing.T, root string) {
			for _, path := range []string{filepath.Join(root, ".concoct/current/task-plan.md"), filepath.Join(root, ".concoct/archive", time.Now().Format("2006-01-02")+"-APP-001-transition", "task-plan.md")} {
				data, _ := os.ReadFile(path)
				writeArchiveTestFile(t, path, strings.Replace(string(data), "  status: archived", "  status: active", 1))
			}
			runGit(t, root, "add", "-A")
			runGit(t, root, "commit", "--amend", "--no-edit", "-q")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := prepareGitArchiveCandidate(t)
			t.Setenv("CONCOCT_CALLER_DIR", root)
			if err := Run([]string{"archive", "--complete"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			tt.corrupt(t, root)
			err := Run([]string{"archive", "--complete"}, &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestGitArchiveCompletionRefusesUnsafeBoundary(t *testing.T) {
	tests := []struct {
		name, want string
		breakIt    func(*testing.T, string)
	}{
		{name: "wrong branch", want: "checkout drift", breakIt: func(t *testing.T, root string) {
			runGit(t, root, "checkout", "-qb", "wrong-archive-branch")
		}},
		{name: "detached head", want: "checkout drift", breakIt: func(t *testing.T, root string) {
			runGit(t, root, "checkout", "--detach", "-q")
		}},
		{name: "operation in progress", want: "operation is in progress", breakIt: func(t *testing.T, root string) {
			head := gitOutput(t, root, "rev-parse", "HEAD")
			writeArchiveTestFile(t, filepath.Join(root, ".git/MERGE_HEAD"), head+"\n")
		}},
		{name: "invalid base", want: "recorded Git base is not an ancestor", breakIt: func(t *testing.T, root string) {
			path := filepath.Join(root, ".concoct/current/task-plan.md")
			data, _ := os.ReadFile(path)
			base := gitOutput(t, root, "merge-base", "HEAD", "main")
			data = bytes.Replace(data, []byte(base), []byte(strings.Repeat("0", 40)), 1)
			writeArchiveTestFile(t, path, string(data))
			archive := filepath.Join(root, ".concoct/archive", time.Now().Format("2006-01-02")+"-APP-001-transition", "task-plan.md")
			writeArchiveTestFile(t, archive, string(data))
		}},
		{name: "forbidden path", want: "forbidden path implementation.txt", breakIt: func(t *testing.T, root string) {
			writeArchiveTestFile(t, filepath.Join(root, "implementation.txt"), "Archivist mutation\n")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := prepareGitArchiveCandidate(t)
			t.Setenv("CONCOCT_CALLER_DIR", root)
			tt.breakIt(t, root)
			err := Run([]string{"archive", "--complete"}, &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func prepareGitArchiveCandidate(t *testing.T) (string, string) {
	t.Helper()
	root := transitionProject(t, true)
	t.Setenv("CONCOCT_CALLER_DIR", root)
	writeTransitionTask(t, root, "implementation-complete")
	writeArchiveTestFile(t, filepath.Join(root, ".concoct/current/review-01.md"), "---\ntask-id: APP-001\nreview: 1\nstatus: approved\ncreated: 2026-07-31\npersona: reviewer\n---\n# Review\n\n## Outcome\n\n`approved`\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-qm", "concoct: record review-01.md for APP-001")
	taskPath := filepath.Join(root, ".concoct/current/task-plan.md")
	taskData, _ := os.ReadFile(taskPath)
	writeArchiveTestFile(t, taskPath, strings.Replace(string(taskData), "  status: active\n", "  archive-commit: self\n  status: archived\n", 1))
	date := time.Now().Format("2006-01-02")
	archiveRel := ".concoct/archive/" + date + "-APP-001-transition"
	archiveDir := filepath.Join(root, filepath.FromSlash(archiveRel))
	for _, name := range []string{"task-plan.md", "notes.md", "review-01.md"} {
		data, err := os.ReadFile(filepath.Join(root, ".concoct/current", name))
		if err != nil {
			t.Fatal(err)
		}
		writeArchiveTestFile(t, filepath.Join(archiveDir, name), string(data))
	}
	summary := "---\ntask-id: APP-001\nroadmap-id: APP-001\nstatus: archived\narchived: " + date + "\nreview: review-01.md\ndelivery: pending-integration\ncapability-impact:\n  type: none\n  ids: []\n---\n# Summary\n\n## Delivered outcome\nDone.\n\n## Key decisions\nExplicit.\n\n## Files and areas changed\nFiles.\n\n## Verification\nPassed.\n\n## Review outcome\nApproved.\n\n## Capability changes\nNone.\n\n## Skipped work\nNone.\n\n## Follow-up work\nNone.\n"
	writeArchiveTestFile(t, filepath.Join(archiveDir, "summary.md"), summary)
	roadPath := filepath.Join(root, ".concoct/roadmap.md")
	roadData, _ := os.ReadFile(roadPath)
	writeArchiveTestFile(t, roadPath, string(roadData)+"- Archive: `"+archiveRel+"/`\n")
	return root, archiveRel
}

func writeArchiveTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

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

func TestNonGitCompletionUsesArchivistHandoffWhenReviewIsNotRequired(t *testing.T) {
	root := transitionProject(t, true)
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(root, ".concoct/policy.md")
	policy, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	policy = []byte(strings.Replace(strings.Replace(string(policy), "  - independent-review\n  - archival\n  - integration\n", "  - archival\n  - integration\nnot-required-reasons:\n  - independent-review: repository accepts developer verification\n", 1), "  - reviewer-approval-before-archive\n", "", 1))
	if err := os.WriteFile(policyPath, policy, 0o644); err != nil {
		t.Fatal(err)
	}
	writeTransitionTaskWithBase(t, root, "implementation-complete", "", false)
	if err := os.WriteFile(filepath.Join(root, ".concoct/current/notes.md"), []byte(archivistHandoff), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONCOCT_CALLER_DIR", root)
	var out bytes.Buffer
	if err := Run([]string{"code", "--complete"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Next: concoct archive") {
		t.Fatalf("output = %q", out.String())
	}
	if err := Run([]string{"review", "--reserve"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "not required") {
		t.Fatalf("review reservation error = %v", err)
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

const archivistHandoff = `# Notes

## Handoff to archivist

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
### Suggested archive focus
Policy evidence.`

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
