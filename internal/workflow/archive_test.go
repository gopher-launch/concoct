package workflow

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/gitrepo"
)

func TestArchivedGitTaskPlanAppliesOnlyCompletionOwnedMetadata(t *testing.T) {
	accepted := []byte("---\nid: APP-001\ntitle: Exact archive transition\nroadmap-id: APP-001\nstatus: implementation-complete\ncreated: 2026-01-01\nupdated: 2026-01-02\ngit:\n  enabled: true\n  trunk: main\n  task-branch: concoct/app-001-exact\n  base: abc123\n  status: active\ncapability-impact:\n  type: none\n  ids: []\n  rationale: Internal transition fix.\n---\n# Task\n\nPreserve this body byte-for-byte.\n")
	want := bytes.Replace(accepted, []byte("  status: active\n"), []byte("  status: archived\n  archive-commit: self\n"), 1)

	got, err := archivedGitTaskPlan(accepted)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("archival metadata transition changed unrelated task bytes\nwant:\n%s\ngot:\n%s", want, got)
	}
	if _, err := archivedGitTaskPlan(got); err == nil || !strings.Contains(err.Error(), "active and omit archive-commit") {
		t.Fatalf("already archived evidence was accepted: %v", err)
	}
}

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

func TestCapabilityLedgerSeparatesRecordsFromInterRecordFormatting(t *testing.T) {
	base := "---\nversion: 1\n---\n# Capabilities\n\nIntro.\n\n## CAP-001 — One\n- Status: `active`\nBody\n\n## CAP-002 — Two\n- Status: `active`\nBody\n"
	appendRecord := base + "\n\n## CAP-003 — Three\n- Status: `active`\nBody\n"
	insertRecord := strings.Replace(base, "\n## CAP-002", "\n\n## CAP-003 — Three\n- Status: `active`\nBody\n\n## CAP-002", 1)
	removedRecord := strings.Replace(base, "\n## CAP-002 — Two\n- Status: `active`\nBody\n", "\n\n", 1)
	separatorOnly := strings.Replace(base, "Body\n\n## CAP-002", "Body\n\n\n\t\n## CAP-002", 1)
	crlf := strings.ReplaceAll(base, "\n", "\r\n")
	withoutFinalNewline := strings.TrimSuffix(base, "\n")

	for name, tt := range map[string]struct {
		before, after, impact string
		ids                   []string
	}{
		"append":         {base, appendRecord, "add", []string{"CAP-003"}},
		"insertion":      {base, insertRecord, "add", []string{"CAP-003"}},
		"removal":        {base, removedRecord, "remove", []string{"CAP-002"}},
		"separator-only": {base, separatorOnly, "none", nil},
		"line-endings":   {base, crlf, "none", nil},
		"final-newline":  {base, withoutFinalNewline, "none", nil},
	} {
		t.Run(name, func(t *testing.T) {
			beforeLedger, beforeDiags := parseCapabilityLedger(tt.before)
			afterLedger, afterDiags := parseCapabilityLedger(tt.after)
			if len(beforeDiags) != 0 || len(afterDiags) != 0 {
				t.Fatalf("ledger diagnostics: before=%v after=%v", beforeDiags, afterDiags)
			}
			task := taskMeta{Impact: impact{Type: tt.impact, IDs: tt.ids}}
			if err := validateCapabilitySectionChanges(beforeLedger.sections(), afterLedger.sections(), task); err != nil {
				t.Fatal(err)
			}
			declared := map[string]bool{}
			for _, id := range tt.ids {
				declared[id] = true
			}
			if !bytes.Equal(beforeLedger.undeclaredBytes(declared), afterLedger.undeclaredBytes(declared)) {
				t.Fatal("inter-record formatting or declared record affected protected content")
			}
		})
	}
}

func TestCapabilityLedgerRejectsMalformedAndProtectsMeaningfulContent(t *testing.T) {
	base := "# Capabilities\n\n## CAP-001 — One\n- Status: `active`\nBody\n\n## CAP-002 — Two\n- Status: `active`\nBody\n"
	for name, candidate := range map[string]string{
		"heading":    strings.Replace(base, "## CAP-001 — One", "## CAP-001 - One", 1),
		"duplicate":  base + "\n## CAP-001 — Duplicate\n- Status: `active`\n",
		"body-space": strings.Replace(base, "\nBody\n\n## CAP-002", "\nBody \n\n## CAP-002", 1),
		"ordering":   strings.Replace(base, "## CAP-001 — One\n- Status: `active`\nBody\n\n## CAP-002 — Two\n- Status: `active`\nBody", "## CAP-002 — Two\n- Status: `active`\nBody\n\n## CAP-001 — One\n- Status: `active`\nBody", 1),
	} {
		t.Run(name, func(t *testing.T) {
			candidateLedger, diagnostics := parseCapabilityLedger(candidate)
			if name == "heading" || name == "duplicate" {
				if len(diagnostics) == 0 {
					t.Fatal("malformed ledger was accepted")
				}
				return
			}
			baseLedger, _ := parseCapabilityLedger(base)
			if bytes.Equal(baseLedger.undeclaredBytes(nil), candidateLedger.undeclaredBytes(nil)) {
				t.Fatal("meaningful undeclared ledger change was ignored")
			}
		})
	}
}

func TestValidateGitCapabilityDiffAllowsDeclaredAppendAcrossSeparator(t *testing.T) {
	root := t.TempDir()
	base := "# Capabilities\n\n## CAP-001 — One\n- Status: `active`\nBody\n"
	write(t, filepath.Join(root, ".concoct/capabilities.md"), base)
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "baseline")
	archiveRel := ".concoct/archive/2026-01-01-app-001"
	write(t, filepath.Join(root, ".concoct/capabilities.md"), base+"\n\n## CAP-002 — Two\n- Status: `active`\n- Archive: `"+archiveRel+"/`\nBody\n")
	repo, ok, err := gitrepo.Open(root)
	if err != nil || !ok {
		t.Fatalf("open repository: ok=%v err=%v", ok, err)
	}
	task := taskMeta{Impact: impact{Type: "add", IDs: []string{"CAP-002"}}}
	if err := validateGitCapabilityDiff(repo, "HEAD", task, archiveRel); err != nil {
		t.Fatalf("declared append was rejected: %v", err)
	}
}

func TestValidateGitCapabilityDiffRejectsBaselineMissingRequiredMetadata(t *testing.T) {
	root := t.TempDir()
	baseline := "# Capabilities\n\n## CAP-001 — Existing\nBody\n"
	write(t, filepath.Join(root, ".concoct/capabilities.md"), baseline)
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "baseline")

	archiveRel := ".concoct/archive/2026-01-01-app-001"
	candidate := "# Capabilities\n\n## CAP-001 — Existing\n- Status: `active`\nBody\n\n## CAP-002 — Added\n- Status: `active`\n- Archive: `" + archiveRel + "/`\nBody\n"
	write(t, filepath.Join(root, ".concoct/capabilities.md"), candidate)
	repo, ok, err := gitrepo.Open(root)
	if err != nil || !ok {
		t.Fatalf("open repository: ok=%v err=%v", ok, err)
	}
	task := taskMeta{Impact: impact{Type: "add", IDs: []string{"CAP-002"}}}
	err = validateGitCapabilityDiff(repo, "HEAD", task, archiveRel)
	if err == nil || !strings.Contains(err.Error(), "invalid baseline capabilities: .concoct/capabilities.md: CAP-001 missing Status") {
		t.Fatalf("error = %v, want invalid baseline missing-Status diagnostic", err)
	}
}

func TestCompleteGitArchiveAllowsDeclaredAppendAcrossSeparator(t *testing.T) {
	root := fixture(t, "implementation-complete", "approved", "")
	baseCapabilities := "---\nversion: 1\nproject: demo\n---\n# Capabilities\n\n## CAP-000 — Existing\n- Status: `active`\nBody\n"
	write(t, filepath.Join(root, ".concoct/capabilities.md"), baseCapabilities)
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "planning base")
	base := runGit(t, root, "rev-parse", "HEAD")
	taskBranch := "concoct/app-001-demo"
	runGit(t, root, "checkout", "-qb", taskBranch)

	taskPath := filepath.Join(root, ".concoct/current/task-plan.md")
	taskData, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	gitMeta := "git:\n  enabled: true\n  trunk: main\n  task-branch: " + taskBranch + "\n  base: " + base + "\n  status: active\n"
	write(t, taskPath, strings.Replace(string(taskData), "capability-impact:\n", gitMeta+"capability-impact:\n", 1))
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "accepted implementation")
	roadmapPath := filepath.Join(root, ".concoct/roadmap.md")
	baseRoadmap, err := os.ReadFile(roadmapPath)
	if err != nil {
		t.Fatal(err)
	}

	archiveRel := authorArchiveFixture(t, root, false, "", "")
	summaryPath := filepath.Join(root, filepath.FromSlash(archiveRel), "summary.md")
	summary, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	summary = []byte(strings.Replace(string(summary), "status: delivered", "status: archived", 1))
	summary = []byte(strings.Replace(string(summary), "delivery: complete", "delivery: pending-integration", 1))
	write(t, summaryPath, string(summary))

	write(t, filepath.Join(root, ".concoct/capabilities.md"), baseCapabilities+"\n\n## CAP-001 — Added\n- Status: `active`\n- Archive: `"+archiveRel+"/`\nBody\n")
	write(t, roadmapPath, string(baseRoadmap)+"- Archive: `"+archiveRel+"/`\n")

	result, err := CompleteArchive(root, ArchiveOverride{})
	if err != nil {
		t.Fatalf("complete Git archive with declared append: %v", err)
	}
	if !result.Committed || runGit(t, root, "status", "--short") != "" {
		t.Fatalf("archive result = %#v; status = %q", result, runGit(t, root, "status", "--short"))
	}
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
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
