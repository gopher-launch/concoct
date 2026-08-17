package execution

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/project"
)

// TestLiveCodexCompatibility is an opt-in compatibility and baseline harness.
// It uses the real execution boundary only in a freshly initialized temporary
// project, so it cannot advance or retain evidence in the caller's task.
//
// CONCOCT_LIVE_CODEX=1 CONCOCT_LIVE_CODEX_MODEL=gpt-5 \
// CONCOCT_LIVE_CODEX_REASONING=medium go test -v -run TestLiveCodexCompatibility ./internal/execution
func TestLiveCodexCompatibility(t *testing.T) {
	if os.Getenv("CONCOCT_LIVE_CODEX") != "1" {
		t.Skip("set CONCOCT_LIVE_CODEX=1 to run the isolated billable Codex compatibility harness")
	}
	model := strings.TrimSpace(os.Getenv("CONCOCT_LIVE_CODEX_MODEL"))
	reasoning := strings.TrimSpace(os.Getenv("CONCOCT_LIVE_CODEX_REASONING"))
	if model == "" || reasoning == "" {
		t.Fatal("CONCOCT_LIVE_CODEX_MODEL and CONCOCT_LIVE_CODEX_REASONING are required")
	}

	parent := t.TempDir()
	if err := project.Initialize(parent, "live-fixture", &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(parent, "live-fixture")
	config := "exec:\n  adapter: codex\n  roles:\n    product-owner:\n      model: " + model + "\n      reasoning: " + reasoning + "\n"
	if err := os.WriteFile(filepath.Join(root, ".concoct", "config.yaml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	result, err := Run(ctx, root, Options{}, strings.NewReader(""), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("live Codex compatibility execution failed: %v", err)
	}
	if result.Prepared.Action.Kind != "product-owner-next" || !result.Reconciliation.ResultAccepted {
		t.Fatalf("live result did not preserve the ready-state decision boundary: %#v", result)
	}
	if result.Prepared.Settings.Model.Value != model || result.Prepared.Settings.Reasoning.Value != reasoning {
		t.Fatalf("resolved live profile = model %q reasoning %q", result.Prepared.Settings.Model.Value, result.Prepared.Settings.Reasoning.Value)
	}
	metadata, readErr := os.ReadFile(filepath.Join(result.RecordPath, "metadata.json"))
	if readErr != nil || !strings.Contains(string(metadata), "adapter_version") {
		t.Fatalf("live measurement lacks adapter version: %v", readErr)
	}
	t.Logf("isolated fixture=%s cache-condition=no pre-seeded adapter cache prompt-bytes=%d usage=%s", root, result.Prepared.Composition.ByteCount(), result.Measurement.UsageSummary())
}

// TestLiveCodexRoleBenchmarks runs three independent, disposable lifecycle
// fixtures. It is intentionally billable, opt-in, and excluded from ordinary
// verification. Each role uses the same model/reasoning/sandbox/schema profile
// and starts without a pre-seeded adapter cache. Privacy-preserving metrics are
// exported create-only to CONCOCT_LIVE_CODEX_EVIDENCE_DIR before lifecycle
// success is evaluated, so blocked or finalization-rejected candidates retain
// their paid measurement evidence outside the disposable fixture.
func TestLiveCodexRoleBenchmarks(t *testing.T) {
	if os.Getenv("CONCOCT_LIVE_CODEX_ROLES") != "1" {
		t.Skip("set CONCOCT_LIVE_CODEX_ROLES=1 to run isolated billable Developer/Reviewer/Archivist benchmarks")
	}
	model := strings.TrimSpace(os.Getenv("CONCOCT_LIVE_CODEX_MODEL"))
	reasoning := strings.TrimSpace(os.Getenv("CONCOCT_LIVE_CODEX_REASONING"))
	if model == "" || reasoning == "" {
		t.Fatal("CONCOCT_LIVE_CODEX_MODEL and CONCOCT_LIVE_CODEX_REASONING are required")
	}
	evidenceDir := strings.TrimSpace(os.Getenv("CONCOCT_LIVE_CODEX_EVIDENCE_DIR"))
	if evidenceDir == "" || !filepath.IsAbs(evidenceDir) {
		t.Fatal("CONCOCT_LIVE_CODEX_EVIDENCE_DIR must name an absolute durable output directory")
	}
	if err := os.MkdirAll(evidenceDir, 0o700); err != nil {
		t.Fatalf("create live benchmark evidence directory: %v", err)
	}
	for _, fixture := range []struct{ role, state string }{{"developer", "planned"}, {"reviewer", "implementation-complete"}, {"archivist", "approved"}} {
		t.Run(fixture.role, func(t *testing.T) {
			root := supervisedPromptFixture(t, fixture.state)
			config := "exec:\n  adapter: codex\n  roles:\n    " + fixture.role + ":\n      model: " + model + "\n      reasoning: " + reasoning + "\n      timeout: 30m\n"
			if err := os.WriteFile(filepath.Join(root, ".concoct", "config.yaml"), []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}
			// Configuration is part of authorization evidence and must be clean.
			executionGit(t, root, "add", ".concoct/config.yaml")
			executionGit(t, root, "commit", "-qm", "benchmark profile")
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Minute)
			defer cancel()
			result, err := Run(ctx, root, Options{}, strings.NewReader(""), &bytes.Buffer{})
			metrics, metricsErr := retainLiveBenchmarkMetrics(evidenceDir, fixture.role, root, result)
			if metricsErr != nil {
				t.Fatalf("retain live %s benchmark metrics before lifecycle evaluation: %v", fixture.role, metricsErr)
			}
			t.Logf("role=%s benchmark-metrics=%s", fixture.role, metrics)
			if err != nil {
				t.Fatalf("live %s benchmark failed: %v", fixture.role, err)
			}
			if !result.Reconciliation.ResultAccepted {
				t.Fatalf("live %s result not accepted: %#v", fixture.role, result.Reconciliation)
			}
			metadata, readErr := os.ReadFile(filepath.Join(result.RecordPath, "metadata.json"))
			if readErr != nil || !strings.Contains(string(metadata), "adapter_version") {
				t.Fatalf("live %s lacks adapter version: %v", fixture.role, readErr)
			}
			t.Logf("role=%s fixture=%s model=%s reasoning=%s sandbox=workspace-write schema=structured-outcome cache-condition=no-preseeded-cache prompt-bytes=%d activity=%d command-output-bytes=%d usage=%s disposition=%s accepted=%t", fixture.role, root, model, reasoning, result.Prepared.Composition.ByteCount(), result.Measurement.ActivityEvents, result.Measurement.Commands.OutputBytes, result.Measurement.UsageSummary(), result.Reconciliation.InvocationDisposition, result.Reconciliation.ResultAccepted)
		})
	}
}

func retainLiveBenchmarkMetrics(evidenceDir, role, root string, result Result) ([]byte, error) {
	if result.RecordPath == "" {
		return nil, fmt.Errorf("execution returned no invocation record")
	}
	metrics, err := Metrics(root, filepath.Base(result.RecordPath))
	if err != nil {
		return nil, fmt.Errorf("export privacy-preserving invocation metrics: %w", err)
	}
	path := filepath.Join(evidenceDir, role+"-metrics.json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create %s without overwriting prior evidence: %w", path, err)
	}
	if _, err := file.Write(append(metrics, '\n')); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return metrics, nil
}
