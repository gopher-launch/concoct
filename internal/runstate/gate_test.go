package runstate

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/orchestration"
	"github.com/gopher-launch/concoct/internal/workflow"
)

func TestGateIsPrivateAtomicAndOneUse(t *testing.T) {
	root := t.TempDir()
	evidence := orchestration.Evidence{Digest: "digest", State: workflow.Planned}
	gate, err := New("plan", "development", "APP-001", "", evidence, orchestration.Correlation{}, "config")
	if err != nil {
		t.Fatal(err)
	}
	if err := Create(root, gate); err != nil {
		t.Fatal(err)
	}
	if err := Create(root, gate); err == nil {
		t.Fatal("duplicate gate replaced existing record")
	}
	info, err := os.Stat(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("gate mode = %o", info.Mode().Perm())
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCurrent(loaded, "plan", evidence, "config"); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- Consume(root, loaded)
		}()
	}
	wg.Wait()
	close(errs)
	succeeded := 0
	for err := range errs {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful consumes = %d", succeeded)
	}
	if _, err := os.Stat(Path(root)); !os.IsNotExist(err) {
		t.Fatalf("consumed gate remains: %v", err)
	}
}

func TestGateRejectsStaleMalformedAndPublicEvidence(t *testing.T) {
	root := t.TempDir()
	evidence := orchestration.Evidence{Digest: "digest", State: workflow.Archived}
	gate, err := New("integration", "integration", "APP-001", "", evidence, orchestration.Correlation{}, "config")
	if err != nil {
		t.Fatal(err)
	}
	if err := Create(root, gate); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCurrent(gate, "integration", orchestration.Evidence{Digest: "changed", State: workflow.Archived}, "config"); err == nil {
		t.Fatal("stale evidence accepted")
	}
	if err := ValidateCurrent(gate, "integration", evidence, "changed-config"); err == nil {
		t.Fatal("configuration drift accepted")
	}
	if err := os.Chmod(Path(root), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("public gate permissions accepted")
	}
	if err := os.Chmod(Path(root), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root), []byte("{\"version\":1,\"unknown\":true}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("malformed gate accepted")
	}
	large := make([]byte, maxRecordBytes+1)
	if err := os.WriteFile(filepath.Clean(Path(root)), large, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("oversized gate accepted")
	}
}

func TestDecisionRecordIsPrivateBoundedAndCannotReplaceExistingCandidate(t *testing.T) {
	root := t.TempDir()
	evidence := orchestration.Evidence{Digest: "evidence", State: workflow.Ready}
	correlation := orchestration.Correlation{InvocationID: "invocation", ActionID: "action", AttemptID: "attempt", Role: "product-owner"}
	record, err := NewDecision(orchestration.ProductDecision{Version: 1, Kind: orchestration.DecisionSelect, Selection: "APP-001", Rationale: "ready to plan"}, evidence, correlation)
	if err != nil {
		t.Fatal(err)
	}
	if err := CreateDecision(root, record); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadDecision(root)
	if err != nil || loaded.Decision.Selection != "APP-001" || loaded.Status != "proposed" {
		t.Fatalf("loaded=%#v err=%v", loaded, err)
	}
	if err := CreateDecision(root, record); err == nil {
		t.Fatal("existing decision was replaced")
	}
	info, err := os.Stat(DecisionPath(root))
	if err != nil || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("private decision permissions=%v err=%v", info.Mode(), err)
	}
}

func TestApplyDecisionRequiresExactBoundRecord(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".concoct"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := "## CON-101 — Example\n\n- Status: `candidate`\n"
	after := "## CON-101 — Example\n\n- Status: `planned`\n"
	if err := os.WriteFile(filepath.Join(root, ".concoct", "roadmap.md"), []byte("---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n\n"+before), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".concoct", "capabilities.md"), []byte("---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Capabilities\n\n## CAP-001 — Foundation\n- Status: `active`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(before))
	record, err := NewDecision(orchestration.ProductDecision{Version: 1, Kind: orchestration.DecisionReconcile, Rationale: "promote the accepted candidate", RoadmapDigest: "evidence", Mutations: []orchestration.ProductMutation{{Target: "roadmap", ID: "CON-101", BeforeDigest: fmt.Sprintf("%x", digest), Before: before, After: after}}}, orchestration.Evidence{Digest: "evidence", State: workflow.Ready}, orchestration.Correlation{InvocationID: "invocation", ActionID: "action", AttemptID: "attempt", Role: "product-owner"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyDecision(root, record); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".concoct", "roadmap.md"))
	if err != nil || !strings.Contains(string(data), "Status: `planned`") {
		t.Fatalf("data=%q err=%v", data, err)
	}
	if err := ApplyDecision(root, record); err == nil {
		t.Fatal("applied a stale decision twice")
	}
}

func TestApplyDecisionValidatesEveryRecordBeforeMutatingCanonicalFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".concoct"), 0o755); err != nil {
		t.Fatal(err)
	}
	roadmapBefore := "# Roadmap\n\n## CON-101 — Example\n\n- Status: `candidate`\n"
	capabilitiesBefore := "# Capabilities\n\n## CAP-101 — Example\n\n- Status: `active`\n"
	if err := os.WriteFile(filepath.Join(root, ".concoct", "roadmap.md"), []byte(roadmapBefore), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".concoct", "capabilities.md"), []byte(capabilitiesBefore), 0o644); err != nil {
		t.Fatal(err)
	}
	roadmapRecord := strings.TrimPrefix(roadmapBefore, "# Roadmap\n\n")
	capabilityRecord := strings.TrimPrefix(capabilitiesBefore, "# Capabilities\n\n")
	roadmapDigest := sha256.Sum256([]byte(roadmapRecord))
	record, err := NewDecision(orchestration.ProductDecision{Version: 1, Kind: orchestration.DecisionReconcile, Rationale: "reconcile accepted delivery", RoadmapDigest: "evidence", Mutations: []orchestration.ProductMutation{
		{Target: "roadmap", ID: "CON-101", BeforeDigest: fmt.Sprintf("%x", roadmapDigest), Before: roadmapRecord, After: strings.Replace(roadmapRecord, "candidate", "planned", 1)},
		{Target: "capabilities", ID: "CAP-101", BeforeDigest: strings.Repeat("0", 64), Before: capabilityRecord, After: strings.Replace(capabilityRecord, "active", "retired", 1)},
	}}, orchestration.Evidence{Digest: "evidence", State: workflow.Ready}, orchestration.Correlation{InvocationID: "invocation", ActionID: "action", AttemptID: "attempt", Role: "product-owner"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyDecision(root, record); err == nil {
		t.Fatal("invalid later record applied an earlier canonical replacement")
	}
	data, err := os.ReadFile(filepath.Join(root, ".concoct", "roadmap.md"))
	if err != nil || string(data) != roadmapBefore {
		t.Fatalf("roadmap changed despite rejected reconciliation: %q err=%v", data, err)
	}
}

func TestGateRejectsSymlinkedRuntimeWithoutTouchingExternalTarget(t *testing.T) {
	for _, operation := range []string{"create", "load", "consume"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".concoct"), 0o755); err != nil {
				t.Fatal(err)
			}
			external := t.TempDir()
			before, err := os.Stat(external)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(external, filepath.Join(root, ".concoct", "runtime")); err != nil {
				t.Fatal(err)
			}
			gate := Gate{Version: 1, Name: "plan", Action: "development", AttemptID: "attempt", Evidence: "evidence", ConfigDigest: "config", State: workflow.Planned, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			var operationErr error
			switch operation {
			case "create":
				operationErr = Create(root, gate)
			case "load":
				operationErr = func() error { _, err := Load(root); return err }()
			case "consume":
				operationErr = Consume(root, gate)
			}
			if operationErr == nil || !strings.Contains(operationErr.Error(), "non-symlink directory") {
				t.Fatalf("%s error = %v", operation, operationErr)
			}
			after, err := os.Stat(external)
			if err != nil {
				t.Fatal(err)
			}
			if after.Mode().Perm() != before.Mode().Perm() {
				t.Fatalf("external mode changed from %o to %o", before.Mode().Perm(), after.Mode().Perm())
			}
			entries, err := os.ReadDir(external)
			if err != nil || len(entries) != 0 {
				t.Fatalf("external target was mutated: entries=%v err=%v", entries, err)
			}
		})
	}
}
