package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/workflow"
)

func TestLocalIntegrationAndCleanup(t *testing.T) {
	root, road, _ := setupArchived(t, false)
	var output bytes.Buffer
	if err := Run(root, "", &bytes.Buffer{}, &output); err != nil {
		t.Fatal(err)
	}
	if got := git(t, root, "branch", "--show-current"); got != "trunk" {
		t.Fatalf("branch = %s", got)
	}
	if out := git(t, root, "branch", "--list", "concoct/app-001-demo"); out != "" {
		t.Fatalf("task branch remains: %s", out)
	}
	roadData, _ := os.ReadFile(road)
	if !strings.Contains(string(roadData), "Status: `delivered`") {
		t.Fatal("roadmap not delivered")
	}
	if _, err := os.Stat(filepath.Join(root, ".concoct/current/task-plan.md")); !os.IsNotExist(err) {
		t.Fatal("current task not cleared")
	}
	output.WriteString(workflow.Detect(root).String())
	if strings.Count(output.String(), "Next: concoct next") != 1 || strings.Contains(output.String(), "concoct roadmap or concoct plan") {
		t.Fatalf("integration output does not recommend exactly concoct next: %s", output.String())
	}
}

func TestIntegrationRejectsPolicyOmission(t *testing.T) {
	root, _, _ := setupArchived(t, false)
	path := filepath.Join(root, ".concoct/policy.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "  - integration\n", "not-required-reasons:\n  - integration: non-Git delivery only\n", 1))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "", &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "requires integration") {
		t.Fatalf("error = %v", err)
	}
}

func TestAbortPreparedRecoveryDoesNotDependOnCurrentPolicyComposition(t *testing.T) {
	root, _, pre := setupArchived(t, false)
	archive := git(t, root, "rev-parse", "HEAD~1")
	policyPath := filepath.Join(root, ".concoct/policy.md")
	write(t, root, ".concoct/policy.md", "---\ninstruction-layer: policy\nrequired-phases: []\n---\n# Invalid current policy\n")
	git(t, root, "add", ".concoct/policy.md")
	git(t, root, "commit", "-qm", "corrupt current guidance after recovery preparation")
	path := filepath.Join(root, ".git", "concoct", "integrations", "APP-001.yaml")
	r := recovery{TaskID: "APP-001", Trunk: "trunk", TaskBranch: "concoct/app-001-demo", ArchiveCommit: archive, PreIntegrationHead: pre, Phase: "prepared"}
	if err := writeRecovery(path, r, true); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "abort", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("recovery record remains: %v", err)
	}
	if _, err := os.Stat(policyPath); err != nil {
		t.Fatalf("abort discarded current policy evidence: %v", err)
	}
}

func TestConflictContinueAndAbortRetry(t *testing.T) {
	t.Run("continue", func(t *testing.T) {
		root, _, _ := setupArchived(t, true)
		err := Run(root, "", &bytes.Buffer{}, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "conflicted") {
			t.Fatalf("error = %v", err)
		}
		write(t, root, "shared.txt", "resolved\n")
		git(t, root, "add", "shared.txt")
		if err := Run(root, "continue", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		if got := git(t, root, "branch", "--show-current"); got != "trunk" {
			t.Fatalf("branch = %s", got)
		}
	})
	t.Run("abort and retry", func(t *testing.T) {
		root, _, pre := setupArchived(t, true)
		if err := Run(root, "", &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatal("expected conflict")
		}
		if err := Run(root, "abort", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		if got := git(t, root, "branch", "--show-current"); got != "concoct/app-001-demo" {
			t.Fatalf("branch = %s", got)
		}
		if got := git(t, root, "rev-parse", "trunk"); got != pre {
			t.Fatalf("trunk changed: %s != %s", got, pre)
		}
		git(t, root, "checkout", "trunk")
		write(t, root, "shared.txt", "task\n")
		git(t, root, "add", "shared.txt")
		git(t, root, "commit", "-qm", "resolve on trunk")
		git(t, root, "checkout", "concoct/app-001-demo")
		if err := Run(root, "", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestDirtyAndCheckoutDriftRefusedWithoutMutation(t *testing.T) {
	root, _, _ := setupArchived(t, false)
	before := git(t, root, "rev-parse", "trunk")
	write(t, root, "dirty.txt", "dirty\n")
	if err := Run(root, "", &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "not clean") {
		t.Fatalf("error = %v", err)
	}
	if got := git(t, root, "rev-parse", "trunk"); got != before {
		t.Fatal("dirty refusal mutated trunk")
	}
	if err := os.Remove(filepath.Join(root, "dirty.txt")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "checkout", "trunk")
	if err := Run(root, "", &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("checkout drift was accepted")
	}
	if got := git(t, root, "rev-parse", "trunk"); got != before {
		t.Fatal("checkout-drift refusal mutated trunk")
	}
}

func TestMatchingUpstreamPushPolicy(t *testing.T) {
	for _, tt := range []struct {
		name, answer string
		pushed       bool
	}{{"declined", "n\n", false}, {"confirmed", "yes\n", true}} {
		t.Run(tt.name, func(t *testing.T) {
			root, _, pre := setupArchived(t, false)
			remote := filepath.Join(t.TempDir(), "remote.git")
			command(t, "", "git", "init", "--bare", "-q", remote)
			git(t, root, "remote", "add", "origin", remote)
			git(t, root, "checkout", "trunk")
			git(t, root, "push", "-qu", "origin", "trunk")
			git(t, root, "checkout", "concoct/app-001-demo")
			var output bytes.Buffer
			if err := Run(root, "", strings.NewReader(tt.answer), &output); err != nil {
				t.Fatal(err)
			}
			remoteHead := command(t, "", "git", "--git-dir", remote, "rev-parse", "refs/heads/trunk")
			if tt.pushed && remoteHead == pre {
				t.Fatal("confirmed push did not update upstream")
			}
			if !tt.pushed && remoteHead != pre {
				t.Fatal("declined push updated upstream")
			}
			if !strings.Contains(output.String(), "Push integrated trunk") {
				t.Fatal("matching upstream did not prompt")
			}
		})
	}
}

func TestAutoPushConfiguration(t *testing.T) {
	root, _, _ := setupArchived(t, false)
	remote := filepath.Join(t.TempDir(), "remote.git")
	command(t, "", "git", "init", "--bare", "-q", remote)
	git(t, root, "remote", "add", "origin", remote)
	git(t, root, "checkout", "trunk")
	write(t, root, ".concoct/config.yaml", "git:\n  auto-push: true\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "enable auto push")
	pre := git(t, root, "rev-parse", "HEAD")
	git(t, root, "push", "-qu", "origin", "trunk")
	git(t, root, "checkout", "concoct/app-001-demo")
	var output bytes.Buffer
	if err := Run(root, "", &bytes.Buffer{}, &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "Push integrated trunk") {
		t.Fatal("auto-push prompted")
	}
	remoteHead := command(t, "", "git", "--git-dir", remote, "rev-parse", "refs/heads/trunk")
	if remoteHead == pre {
		t.Fatal("auto-push did not update upstream")
	}
}

func TestContinueAfterIntegrationCommitInterruption(t *testing.T) {
	root, road, pre := setupArchived(t, false)
	archive := git(t, root, "rev-parse", "concoct/app-001-demo~1")
	git(t, root, "checkout", "trunk")
	git(t, root, "merge", "--squash", archive)
	git(t, root, "commit", "-qm", "concoct: integrate APP-001")
	integrationCommit := git(t, root, "rev-parse", "HEAD")
	path := filepath.Join(root, ".git/concoct/integrations/APP-001.yaml")
	r := recovery{TaskID: "APP-001", Trunk: "trunk", TaskBranch: "concoct/app-001-demo", ArchiveCommit: archive, PreIntegrationHead: pre, IntegrationCommit: integrationCommit, Phase: "integrated"}
	if err := writeRecovery(path, r, true); err != nil {
		t.Fatal(err)
	}
	if err := Run(root, "continue", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(road)
	if !strings.Contains(string(b), "Status: `delivered`") {
		t.Fatal("bookkeeping did not resume")
	}
	if out := git(t, root, "branch", "--list", "concoct/app-001-demo"); out != "" {
		t.Fatal("cleanup did not resume")
	}
}

func TestPreparedRecoveryFromTaskBranch(t *testing.T) {
	t.Run("continue completes checkout and integration", func(t *testing.T) {
		root, road, pre := setupArchived(t, false)
		archive := git(t, root, "rev-parse", "concoct/app-001-demo~1")
		path := filepath.Join(root, ".git/concoct/integrations/APP-001.yaml")
		r := recovery{TaskID: "APP-001", Trunk: "trunk", TaskBranch: "concoct/app-001-demo", ArchiveCommit: archive, PreIntegrationHead: pre, Phase: "prepared"}
		if err := writeRecovery(path, r, true); err != nil {
			t.Fatal(err)
		}
		if err := Run(root, "continue", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(road)
		if !strings.Contains(string(b), "Status: `delivered`") {
			t.Fatal("prepared recovery did not finish delivery")
		}
	})

	t.Run("abort removes pre-mutation recovery", func(t *testing.T) {
		root, _, pre := setupArchived(t, false)
		archive := git(t, root, "rev-parse", "concoct/app-001-demo~1")
		path := filepath.Join(root, ".git/concoct/integrations/APP-001.yaml")
		r := recovery{TaskID: "APP-001", Trunk: "trunk", TaskBranch: "concoct/app-001-demo", ArchiveCommit: archive, PreIntegrationHead: pre, Phase: "prepared"}
		if err := writeRecovery(path, r, true); err != nil {
			t.Fatal(err)
		}
		if err := Run(root, "abort", &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("prepared recovery record remains after abort")
		}
	})
}

func TestRecoveryRefusesUnrelatedTrunkChanges(t *testing.T) {
	t.Run("dirty abort", func(t *testing.T) {
		root, _, _ := setupArchived(t, true)
		if err := Run(root, "", &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatal("expected conflict")
		}
		write(t, root, "unrelated.txt", "keep\n")
		if err := Run(root, "abort", &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "refuses unexpected worktree changes") {
			t.Fatalf("error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "unrelated.txt")); err != nil {
			t.Fatal("dirty abort discarded unrelated file")
		}
	})

	t.Run("staged new file before abort", func(t *testing.T) {
		root, _, _ := setupArchived(t, true)
		if err := Run(root, "", &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatal("expected conflict")
		}
		write(t, root, "unrelated.txt", "keep\n")
		git(t, root, "add", "unrelated.txt")
		if err := Run(root, "abort", &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "refuses unexpected worktree changes") {
			t.Fatalf("error = %v", err)
		}
		if got := git(t, root, "show", ":unrelated.txt"); got != "keep" {
			t.Fatalf("staged file changed: %q", got)
		}
	})

	t.Run("staged unrelated tracked modification before abort", func(t *testing.T) {
		root, _, _ := setupArchived(t, true)
		if err := Run(root, "", &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatal("expected conflict")
		}
		write(t, root, "AGENTS.md", "# Unrelated staged work\n")
		git(t, root, "add", "AGENTS.md")
		if err := Run(root, "abort", &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "refuses unexpected worktree changes") {
			t.Fatalf("error = %v", err)
		}
		if got := git(t, root, "show", ":AGENTS.md"); got != "# Unrelated staged work" {
			t.Fatalf("staged modification changed: %q", got)
		}
	})

	t.Run("unrelated commit before abort", func(t *testing.T) {
		root, _, pre := setupArchived(t, false)
		archive := git(t, root, "rev-parse", "concoct/app-001-demo~1")
		git(t, root, "checkout", "trunk")
		path := filepath.Join(root, ".git/concoct/integrations/APP-001.yaml")
		r := recovery{TaskID: "APP-001", Trunk: "trunk", TaskBranch: "concoct/app-001-demo", ArchiveCommit: archive, PreIntegrationHead: pre, Phase: "prepared"}
		if err := writeRecovery(path, r, true); err != nil {
			t.Fatal(err)
		}
		write(t, root, "unrelated.txt", "keep\n")
		git(t, root, "add", "unrelated.txt")
		git(t, root, "commit", "-qm", "unrelated trunk work")
		advanced := git(t, root, "rev-parse", "HEAD")
		if err := Run(root, "abort", &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "HEAD changed unexpectedly") {
			t.Fatalf("error = %v", err)
		}
		if got := git(t, root, "rev-parse", "HEAD"); got != advanced {
			t.Fatal("abort rewound unrelated trunk commit")
		}
	})

	t.Run("unexpected head before later continuation", func(t *testing.T) {
		root, _, pre := setupArchived(t, false)
		archive := git(t, root, "rev-parse", "concoct/app-001-demo~1")
		git(t, root, "checkout", "trunk")
		git(t, root, "merge", "--squash", archive)
		git(t, root, "commit", "-qm", "concoct: integrate APP-001")
		integrationCommit := git(t, root, "rev-parse", "HEAD")
		path := filepath.Join(root, ".git/concoct/integrations/APP-001.yaml")
		r := recovery{TaskID: "APP-001", Trunk: "trunk", TaskBranch: "concoct/app-001-demo", ArchiveCommit: archive, PreIntegrationHead: pre, IntegrationCommit: integrationCommit, Phase: "integrated"}
		if err := writeRecovery(path, r, true); err != nil {
			t.Fatal(err)
		}
		write(t, root, "unrelated.txt", "keep\n")
		git(t, root, "add", "unrelated.txt")
		git(t, root, "commit", "-qm", "unrelated trunk work")
		advanced := git(t, root, "rev-parse", "HEAD")
		if err := Run(root, "continue", &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "HEAD changed unexpectedly") {
			t.Fatalf("error = %v", err)
		}
		if got := git(t, root, "rev-parse", "HEAD"); got != advanced {
			t.Fatal("continue rewrote unexpected trunk head")
		}
	})
}

func setupArchived(t *testing.T, conflict bool) (root, road, pre string) {
	t.Helper()
	root = t.TempDir()
	write(t, root, "AGENTS.md", "---\ninstruction-layer: project-guidance\n---\n# Agents\n")
	write(t, root, ".concoct/policy.md", "---\ninstruction-layer: policy\nrequired-phases:\n  - product-ownership\n  - task-planning\n  - development\n  - independent-review\n  - archival\n  - integration\napproval-gates:\n  - reviewer-approval-before-archive\n  - archive-before-integration\ngit-strategy: task-branch-with-squash-integration\n---\n# Policy\n")
	write(t, root, ".concoct/capabilities.md", "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Capabilities\n")
	write(t, root, ".concoct/roadmap.md", "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n## APP-001 — Demo\n- Status: `planned`\n")
	git(t, root, "init", "-q", "-b", "trunk")
	git(t, root, "config", "user.email", "test@example.com")
	git(t, root, "config", "user.name", "Test")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "base")
	base := git(t, root, "rev-parse", "HEAD")
	if conflict {
		write(t, root, "shared.txt", "trunk\n")
		git(t, root, "add", "shared.txt")
		git(t, root, "commit", "-qm", "trunk change")
		pre = git(t, root, "rev-parse", "HEAD")
		git(t, root, "checkout", "-qb", "concoct/app-001-demo", base)
		write(t, root, "shared.txt", "task\n")
	} else {
		pre = base
		git(t, root, "checkout", "-qb", "concoct/app-001-demo")
	}
	road = filepath.Join(root, ".concoct/roadmap.md")
	b, _ := os.ReadFile(road)
	_ = os.WriteFile(road, []byte(strings.Replace(string(b), "`planned`", "`active`", 1)), 0o644)
	plan := func(archive string) string {
		return "---\nid: APP-001\ntitle: Demo\nroadmap-id: APP-001\nstatus: implementation-complete\ncreated: 2026-01-01\nupdated: 2026-01-01\ncapability-impact:\n  type: none\n  rationale: No impact\ngit:\n  enabled: true\n  trunk: trunk\n  task-branch: concoct/app-001-demo\n  base: " + base + "\n  archive-commit: " + archive + "\n  status: archived\n---\n# Task\n"
	}
	write(t, root, ".concoct/current/task-plan.md", plan(""))
	write(t, root, ".concoct/current/notes.md", "# Notes\n")
	write(t, root, ".concoct/current/review-01.md", "---\ntask-id: APP-001\nreview: 1\nstatus: approved\ncreated: 2026-01-01\npersona: reviewer\n---\n# Review\n## Outcome\n`approved`\n")
	write(t, root, ".concoct/archive/2026-01-01-APP-001-demo/summary.md", "# Delivered\n")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "archive")
	archive := git(t, root, "rev-parse", "HEAD")
	write(t, root, ".concoct/current/task-plan.md", plan(archive))
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "record archive commit")
	return root, road, pre
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func command(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	c := exec.Command(name, args...)
	if dir != "" {
		c.Dir = dir
	}
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v: %s", name, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
