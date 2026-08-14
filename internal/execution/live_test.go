package execution

import (
	"bytes"
	"context"
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
