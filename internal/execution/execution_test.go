package execution

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/config"
	"github.com/gopher-launch/concoct/internal/prompt"
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

func installFakeCodex(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body+"\n"), 0o755); err != nil {
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
