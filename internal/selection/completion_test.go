package selection

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/integration"
)

func integratedCompletionFixture(t *testing.T) (string, *activework.Store, activework.Snapshot, integration.Config) {
	t.Helper()
	repo, _, cfg, _ := approvedIntegration(t)
	acceptance, err := integration.Offer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	result, err := integration.Apply(context.Background(), cfg, acceptance.ID)
	if err != nil || result.Status != "integrated" {
		t.Fatalf("integrate fixture: %+v, %v", result, err)
	}
	store, err := activework.Open(repo, cfg.Root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Status()
	if err != nil || !snapshot.Active {
		t.Fatalf("integrated snapshot: %+v, %v", snapshot, err)
	}
	return repo, store, snapshot, cfg
}

func clearFixtureOutputs(t *testing.T, store *activework.Store, runID string) {
	t.Helper()
	for _, name := range []string{"concoct"} {
		if err := os.Remove(filepath.Join(store.Root, "workspaces", runID, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCompletionArchivesAndRetiresWithoutProvider(t *testing.T) {
	repo, store, snapshot, cfg := integratedCompletionFixture(t)
	clearFixtureOutputs(t, store, snapshot.RunID)
	reagent, reviewer := cfg.Reagent.(*fakeAgent), cfg.Reviewer.(*fakeReviewer)
	turns, reviews := reagent.turns, reviewer.starts
	result, err := store.Complete(nil)
	if err != nil || result.Status != "completed" || result.Facts.Candidate != snapshot.Candidate.Commit || result.Facts.TaskID != snapshot.TaskID {
		t.Fatalf("completion: %+v, %v", result, err)
	}
	if reagent.turns != turns || reviewer.starts != reviews {
		t.Fatal("completion invoked provider")
	}
	if status, err := store.Status(); err != nil || status.Active {
		t.Fatalf("active binding remained: %+v, %v", status, err)
	}
	for _, name := range []string{"completion.json", "evidence/task.md", "evidence/review-01.md"} {
		if _, err := os.Stat(filepath.Join(result.Archive, name)); err != nil {
			t.Fatalf("missing archive %s: %v", name, err)
		}
	}
	for _, suffix := range []string{"", "-review"} {
		if _, err := os.Stat(filepath.Join(store.Root, "workspaces", snapshot.RunID+suffix)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("workspace %q retained: %v", suffix, err)
		}
	}
	if head, err := gitOutput(repo, "rev-parse", "HEAD"); err != nil || head != snapshot.Candidate.Commit {
		t.Fatalf("integration was replayed or changed: %s, %v", head, err)
	}
	if _, err := store.Complete(nil); !errors.Is(err, activework.ErrNoActive) {
		t.Fatalf("completed run replayed: %v", err)
	}
}

func TestCompletionInterruptedMaintenanceRetriesOnly(t *testing.T) {
	for _, stage := range []string{"completion-recorded", "completion-archived", "completion-cleaned"} {
		t.Run(stage, func(t *testing.T) {
			repo, store, snapshot, cfg := integratedCompletionFixture(t)
			clearFixtureOutputs(t, store, snapshot.RunID)
			before, _ := gitOutput(repo, "rev-parse", "HEAD")
			result, err := store.Complete(func(got string) error {
				if got == stage {
					return errors.New("simulated maintenance interruption")
				}
				return nil
			})
			if err == nil || result.Status != "maintenance_pending" {
				t.Fatalf("interrupted completion: %+v, %v", result, err)
			}
			status, err := store.Status()
			if err != nil || !status.Active || status.Completion == nil {
				t.Fatalf("completion marker missing: %+v, %v", status, err)
			}
			continued, err := Continue(context.Background(), Config{Repository: repo, Root: cfg.Root})
			if err != nil || continued.Status != "maintenance_pending" {
				t.Fatalf("continuation should request maintenance: %+v, %v", continued, err)
			}
			result, err = store.Complete(nil)
			if err != nil || result.Status != "completed" || result.Facts.At != status.Completion.At {
				t.Fatalf("maintenance retry: %+v, %v", result, err)
			}
			if after, _ := gitOutput(repo, "rev-parse", "HEAD"); after != before {
				t.Fatal("maintenance retry changed integrated commit")
			}
		})
	}
}

func TestCompletionCleanupFailureRetainsEvidenceAndPruningExcludesActive(t *testing.T) {
	_, store, snapshot, _ := integratedCompletionFixture(t)
	result, err := store.Complete(nil) // ignored generated files prevent safe worktree removal
	if err == nil || result.Status != "maintenance_pending" {
		t.Fatalf("unsafe cleanup was not held: %+v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(result.Archive, "completion.json")); err != nil {
		t.Fatalf("archive missing after cleanup failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.Root, "workspaces", snapshot.RunID)); err != nil {
		t.Fatalf("recoverable workspace discarded: %v", err)
	}
	if removed, err := store.PruneArchives(time.Now().Add(time.Hour), nil); err != nil || len(removed) != 0 {
		t.Fatalf("active archive pruned: %v, %v", removed, err)
	}
	clearFixtureOutputs(t, store, snapshot.RunID)
	result, err = store.Complete(nil)
	if err != nil || result.Status != "completed" {
		t.Fatalf("cleanup retry: %+v, %v", result, err)
	}
	if removed, err := store.PruneArchives(time.Now().Add(time.Hour), []string{snapshot.TaskID}); err != nil || len(removed) != 0 {
		t.Fatalf("supplied task archive pruned: %v, %v", removed, err)
	}
	if removed, err := store.PruneArchives(time.Now().Add(time.Hour), []string{snapshot.RunID}); err != nil || len(removed) != 0 {
		t.Fatalf("supplied run archive pruned: %v, %v", removed, err)
	}
	if removed, err := store.PruneArchives(result.Facts.At, nil); err != nil || len(removed) != 0 {
		t.Fatalf("archive pruned before cutoff: %v, %v", removed, err)
	}
	if removed, err := store.PruneArchives(time.Now().Add(time.Hour), nil); err != nil || len(removed) != 1 || removed[0] != snapshot.TaskID+"/"+snapshot.RunID {
		t.Fatalf("explicit pruning: %v, %v", removed, err)
	}
}

func TestCompletionArchiveFailureRetriesWithoutIntegration(t *testing.T) {
	repo, store, snapshot, _ := integratedCompletionFixture(t)
	clearFixtureOutputs(t, store, snapshot.RunID)
	archive := filepath.Join(store.Root, "repos")
	entries, err := os.ReadDir(archive)
	if err != nil || len(entries) != 1 {
		t.Fatalf("repository record: %v, %v", entries, err)
	}
	archive = filepath.Join(archive, entries[0].Name(), "archive", snapshot.TaskID, snapshot.RunID)
	if err := os.MkdirAll(filepath.Join(archive, "evidence"), 0700); err != nil {
		t.Fatal(err)
	}
	conflict := filepath.Join(archive, "evidence", "task.md")
	if err := os.WriteFile(conflict, []byte("external archive conflict"), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := gitOutput(repo, "rev-parse", "HEAD")
	result, err := store.Complete(nil)
	if err == nil || result.Status != "maintenance_pending" {
		t.Fatalf("archive conflict was not held: %+v, %v", result, err)
	}
	if body, _ := os.ReadFile(conflict); string(body) != "external archive conflict" {
		t.Fatal("conflicting archive data overwritten")
	}
	if err := os.Remove(conflict); err != nil {
		t.Fatal(err)
	}
	result, err = store.Complete(nil)
	if err != nil || result.Status != "completed" {
		t.Fatalf("archive maintenance retry: %+v, %v", result, err)
	}
	if after, _ := gitOutput(repo, "rev-parse", "HEAD"); after != before {
		t.Fatal("archive retry changed integrated commit")
	}
}

func TestCompletionReconcilesIntegratedCheckoutWithPendingRecord(t *testing.T) {
	repo, external, cfg, _ := approvedIntegration(t)
	acceptance, err := integration.Offer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.After = func(stage string) error {
		if stage == "checkout-attached" {
			return errors.New("interrupted before integration record")
		}
		return nil
	}
	result, err := integration.Apply(context.Background(), cfg, acceptance.ID)
	if err == nil || result.Status != "integrated" {
		t.Fatalf("fixture did not stop after Git integration: %+v, %v", result, err)
	}
	store, err := activework.Open(repo, external)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Status()
	if err != nil || snapshot.Integration == nil || snapshot.Integration.Status != "intent" {
		t.Fatalf("unexpected integration record: %+v, %v", snapshot, err)
	}
	clearFixtureOutputs(t, store, snapshot.RunID)
	completed, err := store.Complete(nil)
	if err != nil || completed.Status != "completed" || completed.Facts.Candidate != acceptance.Candidate {
		t.Fatalf("integrated checkout was not reconciled: %+v, %v", completed, err)
	}
}

func TestCompletionRejectsUnintegratedCandidate(t *testing.T) {
	repo, external, cfg, snapshot := approvedIntegration(t)
	acceptance, err := integration.Offer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	store, err := activework.Open(repo, external)
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.Complete(nil)
	if err == nil || result.Status != "not_integrated" {
		t.Fatalf("unintegrated candidate completed: %+v, %v", result, err)
	}
	if status, err := store.Status(); err != nil || !status.Active || status.RunID != snapshot.RunID {
		t.Fatalf("active candidate was retired: %+v, %v", status, err)
	}
	if head, _ := gitOutput(repo, "rev-parse", "HEAD"); head != acceptance.ExpectedParent {
		t.Fatal("completion mutated target before acceptance")
	}
}

func TestCompletionArchiveIsNotFreshSelectionContext(t *testing.T) {
	repo, store, snapshot, _ := integratedCompletionFixture(t)
	clearFixtureOutputs(t, store, snapshot.RunID)
	result, err := store.Complete(nil)
	if err != nil {
		t.Fatal(err)
	}
	// An archived narrative must not become input to an unrelated task.
	file := filepath.Join(result.Archive, "evidence", "task.md")
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	const priorNarrative = "PRIOR_ARCHIVE_NARRATIVE_ONLY"
	if err := os.WriteFile(file, append(body, []byte(priorNarrative)...), 0600); err != nil {
		t.Fatal(err)
	}
	project, err := readProject(repo)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := inspectParent(repo)
	if err != nil {
		t.Fatal(err)
	}
	prompt := selectionPrompt(project, parent, filepath.Join(store.Root, "workspaces", "future-run"), nil, nil)
	if strings.Contains(prompt, priorNarrative) {
		t.Fatal("fresh selection received archived narrative")
	}
}
