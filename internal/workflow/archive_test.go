package workflow

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompleteArchiveNonGitReconcilesAndClearsLast(t *testing.T) {
	root := fixture(t, "implementation-complete", "approved", "")
	archiveRel := authorArchiveFixture(t, root, false, "", "")
	result, err := CompleteArchive(root, ArchiveOverride{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, archiveRel) {
		t.Fatalf("result = %#v", result)
	}
	if got := Detect(root); got.State != Ready {
		t.Fatalf("state = %s; diagnostics: %v", got.State, got.Diagnostics)
	}
	if _, err := os.Stat(filepath.Join(root, archiveRel, "summary.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".concoct/current/task-plan.md")); !os.IsNotExist(err) {
		t.Fatal("current task was not cleared last")
	}
}

func TestCompleteArchiveRequiresExplicitMatchingOverride(t *testing.T) {
	root := fixture(t, "implementation-complete", "blocked", "")
	authorArchiveFixture(t, root, true, "release-manager", "accepted known risk")
	if _, err := CompleteArchive(root, ArchiveOverride{}); err == nil || !strings.Contains(err.Error(), "explicit") {
		t.Fatalf("error = %v", err)
	}
	if _, err := CompleteArchive(root, ArchiveOverride{Authority: "release-manager", Reason: "different"}); err == nil || !strings.Contains(err.Error(), "exactly match") {
		t.Fatalf("error = %v", err)
	}
	if _, err := CompleteArchive(root, ArchiveOverride{Authority: "release-manager", Reason: "accepted known risk"}); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteArchiveRejectsGitPolicyWithoutIntegration(t *testing.T) {
	root := fixture(t, "implementation-complete", "approved", "git:\n  enabled: true\n  trunk: trunk\n  task-branch: concoct/app-001-demo\n  base: abc123\n")
	path := filepath.Join(root, ".concoct/policy.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "  - integration\n", "not-required-reasons:\n  - integration: non-Git delivery only\n", 1))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteArchive(root, ArchiveOverride{}); err == nil || !strings.Contains(err.Error(), "requires integration") {
		t.Fatalf("error = %v", err)
	}
}

func TestCompleteArchiveRejectsInvalidExternalReviewEvidenceBeforeMutation(t *testing.T) {
	valid := "policy-activity-evidence:\n  - activity: independent-review\n    disposition: externally-satisfied\n    reason: external audit accepted the change\n    recorded-by: developer\n    evidence:\n      - evidence.md\n"
	tests := []struct {
		name, evidence, want string
		keepReview           bool
	}{
		{name: "missing reason", evidence: strings.Replace(valid, "    reason: external audit accepted the change\n", "", 1), want: "requires a durable reason"},
		{name: "unsafe path", evidence: strings.Replace(valid, "evidence.md", "../outside.md", 1), want: "unsafe evidence path"},
		{name: "unauthorized recorder", evidence: strings.Replace(valid, "recorded-by: developer", "recorded-by: reviewer", 1), want: "unauthorized recorded-by"},
		{name: "immutable review contradiction", evidence: valid, keepReview: true, want: "cannot coexist with immutable review evidence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := fixture(t, "implementation-complete", "approved", "")
			archiveRel := authorArchiveFixture(t, root, false, "", "")
			if !tt.keepReview {
				if err := os.Remove(filepath.Join(root, ".concoct/current/review-01.md")); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(root, archiveRel, "review-01.md")); err != nil {
					t.Fatal(err)
				}
				summaryPath := filepath.Join(root, archiveRel, "summary.md")
				summary, err := os.ReadFile(summaryPath)
				if err != nil {
					t.Fatal(err)
				}
				write(t, summaryPath, strings.Replace(string(summary), "review: review-01.md\n", "review:\n", 1))
			}
			taskPath := filepath.Join(root, ".concoct/current/task-plan.md")
			taskData, err := os.ReadFile(taskPath)
			if err != nil {
				t.Fatal(err)
			}
			write(t, taskPath, strings.Replace(string(taskData), "capability-impact:\n", tt.evidence+"capability-impact:\n", 1))
			before := snapshot(t, root)
			if _, err := CompleteArchive(root, ArchiveOverride{}); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if after := snapshot(t, root); after != before {
				t.Fatal("archive completion mutated repository before rejecting invalid policy evidence")
			}
		})
	}
}

func TestCompleteArchiveNonGitRejectsContradictoryLifecycleBeforeCleanup(t *testing.T) {
	for _, replacement := range []string{"status: archived", "delivery: pending-integration", "delivery:"} {
		t.Run(strings.ReplaceAll(replacement, ":", "-"), func(t *testing.T) {
			root := fixture(t, "implementation-complete", "approved", "")
			archiveRel := authorArchiveFixture(t, root, false, "", "")
			path := filepath.Join(root, archiveRel, "summary.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(replacement, "status:") {
				data = []byte(strings.Replace(string(data), "status: delivered", replacement, 1))
			} else {
				data = []byte(strings.Replace(string(data), "delivery: complete", replacement, 1))
			}
			write(t, path, string(data))
			if _, err := CompleteArchive(root, ArchiveOverride{}); err == nil || !strings.Contains(err.Error(), "non-Git archive summary") {
				t.Fatalf("error = %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, ".concoct/current/task-plan.md")); err != nil {
				t.Fatalf("current evidence was removed: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, archiveRel, "summary.md")); err != nil {
				t.Fatalf("delivery evidence was removed: %v", err)
			}
		})
	}
}

func TestCompleteArchiveRequiresExactDeterministicPath(t *testing.T) {
	root := fixture(t, "implementation-complete", "approved", "")
	archiveRel := authorArchiveFixture(t, root, false, "", "")
	wrong := strings.TrimSuffix(archiveRel, "demo-task") + "wrong-slug"
	if err := os.Rename(filepath.Join(root, archiveRel), filepath.Join(root, wrong)); err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteArchive(root, ArchiveOverride{}); err == nil || !strings.Contains(err.Error(), "exact deterministic path") {
		t.Fatalf("error = %v", err)
	}
}

func TestCompleteArchiveRejectsExtraReviewCopy(t *testing.T) {
	root := fixture(t, "implementation-complete", "approved", "")
	archiveRel := authorArchiveFixture(t, root, false, "", "")
	write(t, filepath.Join(root, archiveRel, "review-02.md"), reviewFile(2, "approved"))
	if _, err := CompleteArchive(root, ArchiveOverride{}); err == nil || !strings.Contains(err.Error(), "exactly the 1 accepted") {
		t.Fatalf("error = %v", err)
	}
}

func TestCompleteArchiveRejectsIncompleteCandidateWithoutCleanup(t *testing.T) {
	tests := []struct {
		name, want string
		mutate     func(*testing.T, string, string)
	}{
		{name: "missing review", want: "review-01.md is missing", mutate: func(t *testing.T, root, rel string) {
			if err := os.Remove(filepath.Join(root, rel, "review-01.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing summary section", want: "requires non-empty section", mutate: func(t *testing.T, root, rel string) {
			path := filepath.Join(root, rel, "summary.md")
			data, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(data), "## Verification\nPassed.", "## Verification\n", 1))
		}},
		{name: "capability provenance", want: "must cite archive provenance", mutate: func(t *testing.T, root, rel string) {
			path := filepath.Join(root, ".concoct/capabilities.md")
			data, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(data), rel+"/", ".concoct/archive/wrong/", 1))
		}},
		{name: "roadmap cross reference", want: "Archive must reference", mutate: func(t *testing.T, root, rel string) {
			path := filepath.Join(root, ".concoct/roadmap.md")
			data, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(data), rel+"/", ".concoct/archive/wrong/", 1))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := fixture(t, "implementation-complete", "approved", "")
			rel := authorArchiveFixture(t, root, false, "", "")
			tt.mutate(t, root, rel)
			_, err := CompleteArchive(root, ArchiveOverride{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if _, statErr := os.Stat(filepath.Join(root, ".concoct/current/task-plan.md")); statErr != nil {
				t.Fatalf("current evidence was removed: %v", statErr)
			}
		})
	}
}

func TestCapabilityUndeclaredBytesPreservesNonRecordContentAndOrder(t *testing.T) {
	declared := map[string]bool{"CAP-002": true}
	base := "---\nversion: 1\n---\n# Capabilities\n\nIntro.\n\n## CAP-001 — One\nA\n\n## CAP-002 — Two\nB\n\n## CAP-003 — Three\nC\n"
	allowed := strings.Replace(base, "## CAP-002 — Two\nB\n\n", "## CAP-002 — Two changed\nDifferent.\n\n", 1)
	if !bytes.Equal(capabilityUndeclaredBytes(base, declared), capabilityUndeclaredBytes(allowed, declared)) {
		t.Fatal("declared record edit changed preservation projection")
	}
	for name, changed := range map[string]string{
		"front-matter": strings.Replace(base, "version: 1", "version: 2", 1),
		"intro":        strings.Replace(base, "Intro.", "Changed intro.", 1),
		"record":       strings.Replace(base, "## CAP-001 — One", "## CAP-001 — Changed", 1),
		"ordering":     strings.Replace(base, "## CAP-001 — One\nA\n\n## CAP-002 — Two\nB\n\n## CAP-003 — Three\nC", "## CAP-003 — Three\nC\n\n## CAP-002 — Two\nB\n\n## CAP-001 — One\nA", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if string(capabilityUndeclaredBytes(base, declared)) == string(capabilityUndeclaredBytes(changed, declared)) {
				t.Fatal("unrelated ledger content was not detected")
			}
		})
	}
}

func TestValidateCapabilitySectionChangesAllImpactTypes(t *testing.T) {
	old := map[string]string{"CAP-001": "old", "CAP-002": "stable"}
	tests := []struct {
		name, impact string
		ids          []string
		after        map[string]string
		wantErr      bool
	}{
		{"add", "add", []string{"CAP-003"}, map[string]string{"CAP-001": "old", "CAP-002": "stable", "CAP-003": "new"}, false},
		{"update", "update", []string{"CAP-001"}, map[string]string{"CAP-001": "changed", "CAP-002": "stable"}, false},
		{"remove", "remove", []string{"CAP-001"}, map[string]string{"CAP-002": "stable"}, false},
		{"none", "none", nil, map[string]string{"CAP-001": "old", "CAP-002": "stable"}, false},
		{"undeclared", "update", []string{"CAP-001"}, map[string]string{"CAP-001": "changed", "CAP-002": "also changed"}, true},
		{"add-existing", "add", []string{"CAP-001"}, map[string]string{"CAP-001": "changed", "CAP-002": "stable"}, true},
		{"update-missing", "update", []string{"CAP-003"}, map[string]string{"CAP-001": "old", "CAP-002": "stable"}, true},
		{"remove-retained", "remove", []string{"CAP-001"}, map[string]string{"CAP-001": "old", "CAP-002": "stable"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := taskMeta{Impact: impact{Type: tt.impact, IDs: tt.ids}}
			err := validateCapabilitySectionChanges(old, tt.after, task)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizedRoadmapForArchiveAllowsOnlySelectedFields(t *testing.T) {
	base := "---\nversion: 1\n---\n# Roadmap\n## APP-001 — One\n- Status: `active`\nText.\n## APP-002 — Two\n- Status: `planned`\nOther.\n"
	allowed := strings.Replace(base, "- Status: `active`", "- Status: `active`\n- Archive: `.concoct/archive/example/`", 1)
	want, _ := normalizedRoadmapForArchive(base, "APP-001")
	got, _ := normalizedRoadmapForArchive(allowed, "APP-001")
	if got != want {
		t.Fatal("selected archival fields were not normalized")
	}
	for name, changed := range map[string]string{
		"front-matter":   strings.Replace(base, "version: 1", "version: 2", 1),
		"selected-prose": strings.Replace(base, "Text.", "Changed.", 1),
		"other-item":     strings.Replace(base, "Other.", "Changed other.", 1),
	} {
		t.Run(name, func(t *testing.T) {
			got, _ := normalizedRoadmapForArchive(changed, "APP-001")
			if got == want {
				t.Fatal("unrelated roadmap content was not detected")
			}
		})
	}
}

func authorArchiveFixture(t *testing.T, root string, override bool, authority, reason string) string {
	t.Helper()
	date := time.Now().Format("2006-01-02")
	rel := ".concoct/archive/" + date + "-APP-001-demo-task"
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"task-plan.md", "notes.md", "review-01.md"} {
		b, err := os.ReadFile(filepath.Join(root, ".concoct/current", name))
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(dir, name), string(b))
	}
	overrideYAML := ""
	review := "review: review-01.md\n"
	if override {
		review = ""
		overrideYAML = "override:\n  authority: " + authority + "\n  reason: " + reason + "\n"
	}
	summary := "---\ntask-id: APP-001\nroadmap-id: APP-001\nstatus: delivered\narchived: " + date + "\n" + review + "delivery: complete\n" + overrideYAML + "capability-impact:\n  type: add\n  ids: [CAP-001]\n---\n# Summary\n\n## Delivered outcome\nDone.\n\n## Key decisions\nExplicit.\n\n## Files and areas changed\nFiles.\n\n## Verification\nPassed.\n\n## Review outcome\nRecorded.\n\n## Capability changes\nAdded.\n\n## Skipped work\nNone.\n\n## Follow-up work\nNone.\n"
	write(t, filepath.Join(dir, "summary.md"), summary)
	write(t, filepath.Join(root, ".concoct/capabilities.md"), "---\nversion: 1\nproject: demo\nupdated: "+date+"\n---\n# Capabilities\n## CAP-001 — Demo\n- Status: `active`\n- Archive: `"+rel+"/`\n")
	write(t, filepath.Join(root, ".concoct/roadmap.md"), "---\nversion: 1\nproject: demo\nupdated: "+date+"\n---\n# Roadmap\n## APP-001 — Demo\n- Status: `delivered`\n- Archive: `"+rel+"/`\n")
	return rel
}
