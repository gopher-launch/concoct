package runstate

import (
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
