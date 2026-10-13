package activework

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testStore(t *testing.T) (*Store, string) {
	t.Helper()
	parent := t.TempDir()
	repo := filepath.Join(parent, "project")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "--initial-branch=main", repo)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	cmd = exec.Command("git", "-C", repo, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "base")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	store, err := Open(repo, filepath.Join(parent, "external"))
	if err != nil {
		t.Fatal(err)
	}
	return store, repo
}

func spec(t *testing.T, repo string) StartSpec {
	t.Helper()
	commit, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return StartSpec{TaskID: "APP-001", ParentBranch: "main", ParentCommit: strings.TrimSpace(string(commit)), TaskBranch: "work/app-001", Objective: "Build the useful thing.", Acceptance: "It works.", SelectionRationale: "Ready and highest priority."}
}

func TestOpenCanonicalRootsAndReadOnlyStatus(t *testing.T) {
	store, repo := testStore(t)
	if _, err := os.Stat(store.Root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Open created root: %v", err)
	}
	snap, err := store.Status()
	if err != nil || snap.Active || snap.PendingStart || snap.ObservedAt.IsZero() {
		t.Fatalf("empty status = %#v, %v", snap, err)
	}
	if _, err := os.Stat(store.Root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Status created root: %v", err)
	}
	if _, err := Open(repo, filepath.Join(repo, ".concoct")); err == nil || !strings.Contains(err.Error(), "inside target repository") {
		t.Fatalf("inside-root error = %v", err)
	}
	link := filepath.Join(filepath.Dir(repo), "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(repo, filepath.Join(link, "state")); err == nil || !strings.Contains(err.Error(), "inside target repository") {
		t.Fatalf("symlink-root error = %v", err)
	}
	oldHome := t.TempDir()
	t.Setenv("HOME", oldHome)
	defaultStore, err := Open(repo, "")
	if err != nil || defaultStore.Root != filepath.Join(oldHome, ".concoct") {
		t.Fatalf("default root = %#v, %v", defaultStore, err)
	}
}

func TestCommonGitDirectorySharesOwnershipAcrossWorktrees(t *testing.T) {
	store, repo := testStore(t)
	sibling := filepath.Join(filepath.Dir(repo), "sibling")
	cmd := exec.Command("git", "-C", repo, "worktree", "add", "-b", "task", sibling, "HEAD")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v: %s", err, output)
	}
	other, err := Open(sibling, store.Root)
	if err != nil {
		t.Fatal(err)
	}
	if store.dir != other.dir || store.CommonDir != other.CommonDir {
		t.Fatalf("same Git repository got separate stores: %#v %#v", store, other)
	}
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := other.Acquire(); !errors.Is(err, ErrOwned) {
		t.Fatalf("sibling acquired concurrent ownership: %v", err)
	}
	if _, err := owner.Start(spec(t, repo)); err != nil {
		t.Fatal(err)
	}
	snapshot, err := other.Status()
	if err != nil || !snapshot.Active || snapshot.TaskID != "APP-001" {
		t.Fatalf("sibling status = %#v, %v", snapshot, err)
	}
	if _, err := Open(sibling, filepath.Join(sibling, ".concoct")); err == nil || !strings.Contains(err.Error(), "inside target repository") {
		t.Fatalf("sibling-inside-root error = %v", err)
	}
	if _, err := Open(repo, filepath.Join(sibling, ".concoct")); err == nil || !strings.Contains(err.Error(), "inside target repository") {
		t.Fatalf("other-worktree-inside-root error = %v", err)
	}
}

func TestOwnershipStatusEvidenceAndFinalReview(t *testing.T) {
	store, repo := testStore(t)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := store.Acquire(); !errors.Is(err, ErrOwned) {
		t.Fatalf("second owner error = %v", err)
	}
	id, err := owner.Start(spec(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	snap, err := store.Status()
	if err != nil || !snap.Active || snap.RunID != id || snap.TaskID != "APP-001" || snap.Phase != "engineering" || snap.ObservedAt.Before(start.Add(-time.Second)) {
		t.Fatalf("status during lock = %#v, %v", snap, err)
	}
	if err := owner.SaveCheckpoint("Implemented and checked."); err != nil {
		t.Fatal(err)
	}
	if err := owner.RecordDecision("Operator accepted a small clarification."); err != nil {
		t.Fatal(err)
	}
	if err := owner.SaveReview(1, "Pending independent assessment."); err != nil {
		t.Fatal(err)
	}
	if err := owner.SaveReview(1, "Approved."); err != nil {
		t.Fatal(err)
	}
	if err := owner.FinalizeReview(1); err != nil {
		t.Fatal(err)
	}
	if err := owner.SaveReview(1, "Rewritten verdict."); err == nil || !strings.Contains(err.Error(), "finalized") {
		t.Fatalf("final review mutation error = %v", err)
	}
	if err := owner.RecordReceipt("dispatch-1", []byte(`{"result":"done"}`)); err != nil {
		t.Fatal(err)
	}
	if err := owner.RecordReceipt("dispatch-1", []byte(`{"result":"done"}`)); err != nil {
		t.Fatal(err)
	}
	if err := owner.RecordReceipt("dispatch-1", []byte(`{"result":"changed"}`)); err == nil {
		t.Fatal("conflicting receipt accepted")
	}
	receiptPath := filepath.Join(store.runDir(id), "receipts", "dispatch-1.json")
	if err := os.WriteFile(receiptPath, []byte(`{"result":"tampered"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := owner.RecordReceipt("dispatch-1", []byte(`{"result":"done"}`)); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("tampered receipt error = %v", err)
	}
	if err := owner.UpdateOperational(Operational{Phase: "awaiting-review", Hold: "", RoleIDs: map[string]string{"reagent": "thread-1"}, RoleSettings: map[string]string{"reagent-model": "example"}, Counters: map[string]int{"dispatches": 2}}); err != nil {
		t.Fatal(err)
	}
	snap, err = store.Status()
	if err != nil || snap.Phase != "awaiting-review" || snap.Counters["dispatches"] != 2 || snap.RoleIDs["reagent"] != "thread-1" {
		t.Fatalf("updated status = %#v, %v", snap, err)
	}
	task, err := os.ReadFile(filepath.Join(store.runDir(id), "evidence", "task.md"))
	if err != nil || !strings.Contains(string(task), "Build the useful thing.") || !strings.Contains(string(task), "Implemented and checked.") || !strings.Contains(string(task), "Operator accepted") {
		t.Fatalf("task evidence = %q, %v", task, err)
	}
	entries, err := os.ReadDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != ".git" {
		t.Fatalf("target repository gained files: %v", entries)
	}
}

func TestStatusReadsDuringAtomicCheckpointUpdates(t *testing.T) {
	store, repo := testStore(t)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := owner.Start(spec(t, repo)); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 1)
	go func() {
		for range 100 {
			snap, err := store.Status()
			if err != nil {
				results <- err
				return
			}
			if !snap.Active || snap.TaskID != "APP-001" {
				results <- errors.New("status lost active task")
				return
			}
		}
		results <- nil
	}()
	for i := range 20 {
		if err := owner.SaveCheckpoint("checkpoint " + string(rune('A'+i))); err != nil {
			t.Fatal(err)
		}
	}
	if err := <-results; err != nil {
		t.Fatal(err)
	}
}

func TestInterruptedStartAndTransitionRecovery(t *testing.T) {
	for _, stage := range []string{"start-intent", "start-task", "start-state", "start-active"} {
		t.Run(stage, func(t *testing.T) {
			store, repo := testStore(t)
			owner, err := store.Acquire()
			if err != nil {
				t.Fatal(err)
			}
			owner.hook = func(got string) error {
				if got == stage {
					return errors.New("injected stop")
				}
				return nil
			}
			if _, err := owner.Start(spec(t, repo)); err == nil {
				t.Fatal("start did not stop")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			owner, err = store.Acquire()
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			snap, err := store.Status()
			if err != nil || !snap.Active || snap.PendingStart {
				t.Fatalf("recovered start = %#v, %v", snap, err)
			}
		})
	}
	for _, stage := range []string{"write-intent", "write-evidence"} {
		t.Run(stage, func(t *testing.T) {
			store, repo := testStore(t)
			owner, err := store.Acquire()
			if err != nil {
				t.Fatal(err)
			}
			id, err := owner.Start(spec(t, repo))
			if err != nil {
				t.Fatal(err)
			}
			owner.hook = func(got string) error {
				if got == stage {
					return errors.New("injected stop")
				}
				return nil
			}
			if err := owner.SaveCheckpoint("Recovered checkpoint."); err == nil {
				t.Fatal("write did not stop")
			}
			snap, err := store.Status()
			if err != nil || !snap.PendingWrite {
				t.Fatalf("pending status = %#v, %v", snap, err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			owner, err = store.Acquire()
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			snap, err = store.Status()
			if err != nil || snap.PendingWrite || snap.Revision != 1 {
				t.Fatalf("recovered write = %#v, %v", snap, err)
			}
			body, err := os.ReadFile(filepath.Join(store.runDir(id), "evidence", "task.md"))
			if err != nil || strings.Count(string(body), "Recovered checkpoint.") != 1 {
				t.Fatalf("recovered task = %q, %v", body, err)
			}
		})
	}
}

func TestSelectionBeforeClaimAndInterruptedClaim(t *testing.T) {
	for _, stage := range []string{"write-intent", "write-evidence"} {
		t.Run(stage, func(t *testing.T) {
			store, repo := testStore(t)
			owner, err := store.Acquire()
			if err != nil {
				t.Fatal(err)
			}
			binding := spec(t, repo)
			id, err := owner.BeginSelection(SelectionSpec{ParentBranch: binding.ParentBranch, ParentCommit: binding.ParentCommit})
			if err != nil {
				t.Fatal(err)
			}
			snap, err := store.Status()
			if err != nil || !snap.Active || snap.Phase != "selecting" || snap.TaskID != "" {
				t.Fatalf("selection snapshot = %#v, %v", snap, err)
			}
			if err := owner.UpdateOperational(Operational{Phase: "engineering"}); err == nil {
				t.Fatal("unclaimed run advanced out of selecting phase")
			}
			if _, err := os.Stat(filepath.Join(store.runDir(id), "evidence", "task.md")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invented task evidence: %v", err)
			}
			wrong := binding
			wrong.ParentCommit = strings.Repeat("b", 40)
			if err := owner.Claim(wrong); err == nil {
				t.Fatal("wrong parent accepted")
			}
			owner.hook = func(got string) error {
				if got == stage {
					return errors.New("injected claim stop")
				}
				return nil
			}
			if err := owner.Claim(binding); err == nil {
				t.Fatal("claim did not stop")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			owner, err = store.Acquire()
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			snap, err = store.Status()
			if err != nil || snap.PendingWrite || snap.TaskID != binding.TaskID || snap.TaskBranch != binding.TaskBranch || snap.Phase != "engineering" {
				t.Fatalf("recovered claim = %#v, %v", snap, err)
			}
			if err := owner.Claim(binding); err == nil {
				t.Fatal("second claim accepted")
			}
		})
	}
}

func TestInvalidBindingsAndTamperedEvidence(t *testing.T) {
	store, repo := testStore(t)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	invalid := spec(t, repo)
	invalid.ParentCommit = "bad"
	if _, err := owner.Start(invalid); err == nil {
		t.Fatal("invalid commit accepted")
	}
	invalid = spec(t, repo)
	invalid.ParentCommit = strings.Repeat("b", 40)
	if _, err := owner.Start(invalid); err == nil || !strings.Contains(err.Error(), "not present") {
		t.Fatalf("missing commit error = %v", err)
	}
	invalid = spec(t, repo)
	invalid.ParentBranch = "missing"
	if _, err := owner.Start(invalid); err == nil || !strings.Contains(err.Error(), "does not point") {
		t.Fatalf("missing parent branch error = %v", err)
	}
	invalid = spec(t, repo)
	invalid.TaskBranch = "../escape"
	if _, err := owner.Start(invalid); err == nil {
		t.Fatal("invalid branch accepted")
	}
	id, err := owner.Start(spec(t, repo))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.runDir(id), "evidence", "task.md")
	if err := os.WriteFile(path, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := owner.SaveCheckpoint("should fail"); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("tampered evidence error = %v", err)
	}
	if err := owner.UpdateOperational(Operational{Phase: "engineering", Counters: map[string]int{"dispatches": -1}}); err == nil {
		t.Fatal("negative counter accepted")
	}
}

func TestSymlinkedStorePathRejected(t *testing.T) {
	store, repo := testStore(t)
	if err := os.Symlink(repo, store.Root); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Acquire(); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("symlinked root error = %v", err)
	}
}
