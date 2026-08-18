package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectStates(t *testing.T) {
	tests := []struct {
		name, taskStatus, reviewStatus, extra string
		want                                  State
	}{
		{"ready", "", "", "", Ready},
		{"planned", "planned", "", "", Planned},
		{"in progress", "implementation-in-progress", "", "", InProgress},
		{"complete", "implementation-complete", "", "", Complete},
		{"changes requested", "implementation-complete", "changes-requested", "", ChangesRequested},
		{"approved", "implementation-complete", "approved", "", Approved},
		{"blocked", "implementation-complete", "blocked", "", Blocked},
		{"remediation in progress", "implementation-in-progress", "changes-requested", "remediates-review: review-01.md\n", InProgress},
		{"remediation complete", "implementation-complete", "changes-requested", "remediates-review: review-01.md\n", Complete},
		{"blocked route code", "implementation-in-progress", "blocked", "blocked-review-resolution:\n  review: review-01.md\n  route: code\n  recorded-by: developer\n  evidence:\n    - evidence.md\n", InProgress},
		{"blocked route review", "implementation-complete", "blocked", "blocked-review-resolution:\n  review: review-01.md\n  route: review\n  recorded-by: task-planner\n  evidence:\n    - evidence.md\n", Complete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := fixture(t, tt.taskStatus, tt.reviewStatus, tt.extra)
			r := Detect(root)
			if r.State != tt.want {
				t.Fatalf("state = %s, want %s; diagnostics: %v", r.State, tt.want, r.Diagnostics)
			}
		})
	}
}

func TestReadyReportMakesAbsentTaskAndGitMetadataExplicit(t *testing.T) {
	root := fixture(t, "", "", "")
	report := Detect(root)
	if report.State != Ready {
		t.Fatalf("state = %s; diagnostics: %v", report.State, report.Diagnostics)
	}
	text := report.String()
	for _, want := range []string{"Active task: inactive", "Git task metadata: not applicable", "Next: concoct next"} {
		if !strings.Contains(text, want) {
			t.Fatalf("ready report missing %q:\n%s", want, text)
		}
	}
}

func TestResolveActionClassifiesExecutableAndHumanGatedStates(t *testing.T) {
	tests := []struct {
		state      State
		activities []PolicyActivity
		kind       string
		executable bool
	}{
		{Ready, nil, "product-owner-next", true},
		{Planned, nil, "development", true},
		{InProgress, nil, "development", true},
		{Complete, nil, "independent-review", true},
		{Complete, []PolicyActivity{{Activity: "independent-review", Disposition: "not-required"}}, "archival", true},
		{ChangesRequested, nil, "development", true},
		{Approved, nil, "archival", true},
		{Archived, nil, "integration", true},
		{Blocked, nil, "", false},
		{Integrating, nil, "", false},
		{Invalid, nil, "", false},
	}
	for _, test := range tests {
		resolved := ResolveAction(Report{State: test.state, PolicyActivities: test.activities})
		if resolved.ActionKind != test.kind || resolved.Executable != test.executable || resolved.Command == "" {
			t.Errorf("state %s resolved to %#v", test.state, resolved)
		}
	}
}

func TestDetectArchivedGitTask(t *testing.T) {
	extra := "git:\n  enabled: true\n  trunk: trunk\n  task-branch: concoct/app-001-demo\n  base: abc123\n  archive-commit: def456\n  status: archived\n"
	root := fixture(t, "implementation-complete", "approved", extra)
	got := Detect(root)
	if got.State != Archived || got.Next != "concoct integrate" {
		t.Fatalf("got %#v", got)
	}
}

func TestDetectInterruptedDeliveryIsNotReady(t *testing.T) {
	root := fixture(t, "", "", "")
	write(t, filepath.Join(root, ".git/concoct/integrations/APP-001.yaml"), "phase: delivered\n")
	got := Detect(root)
	if got.State != Integrated || got.Next != "concoct integrate --continue" {
		t.Fatalf("got %#v", got)
	}
}

func TestDetectInvalidEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, root string)
	}{
		{"missing notes", func(t *testing.T, r string) { os.Remove(filepath.Join(r, ".concoct/current/notes.md")) }},
		{"roadmap mismatch", func(t *testing.T, r string) {
			write(t, filepath.Join(r, ".concoct/current/task-plan.md"), task("OTHER-001", "planned", ""))
		}},
		{"unknown status", func(t *testing.T, r string) {
			write(t, filepath.Join(r, ".concoct/current/task-plan.md"), task("APP-001", "mystery", ""))
		}},
		{"unknown roadmap status", func(t *testing.T, r string) {
			write(t, filepath.Join(r, ".concoct/roadmap.md"), roadmap("mystery"))
		}},
		{"missing task title", func(t *testing.T, r string) {
			path := filepath.Join(r, ".concoct/current/task-plan.md")
			data, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(data), "title: Demo task\n", "", 1))
		}},
		{"missing task created", func(t *testing.T, r string) {
			path := filepath.Join(r, ".concoct/current/task-plan.md")
			data, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(data), "created: 2026-01-01\n", "", 1))
		}},
		{"missing task updated", func(t *testing.T, r string) {
			path := filepath.Join(r, ".concoct/current/task-plan.md")
			data, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(data), "updated: 2026-01-01\n", "", 1))
		}},
		{"missing impact rationale", func(t *testing.T, r string) {
			path := filepath.Join(r, ".concoct/current/task-plan.md")
			data, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(data), "  rationale: Demo impact\n", "", 1))
		}},
		{"missing review created", func(t *testing.T, r string) {
			path := filepath.Join(r, ".concoct/current/review-01.md")
			data, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(data), "created: 2026-01-01\n", "", 1))
		}},
		{"missing review persona", func(t *testing.T, r string) {
			path := filepath.Join(r, ".concoct/current/review-01.md")
			data, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(data), "persona: reviewer\n", "", 1))
		}},
		{"review gap", func(t *testing.T, r string) {
			os.Rename(filepath.Join(r, ".concoct/current/review-01.md"), filepath.Join(r, ".concoct/current/review-02.md"))
		}},
		{"stale remediation", func(t *testing.T, r string) {
			write(t, filepath.Join(r, ".concoct/current/task-plan.md"), task("APP-001", "implementation-in-progress", "remediates-review: review-02.md\n"))
		}},
		{"unsafe blocker evidence", func(t *testing.T, r string) {
			write(t, filepath.Join(r, ".concoct/current/task-plan.md"), task("APP-001", "implementation-in-progress", "blocked-review-resolution:\n  review: review-01.md\n  route: code\n  recorded-by: developer\n  evidence: [../outside]\n"))
		}},
		{"delivered active task", func(t *testing.T, r string) { write(t, filepath.Join(r, ".concoct/roadmap.md"), roadmap("delivered")) }},
		{"malformed task metadata", func(t *testing.T, r string) {
			write(t, filepath.Join(r, ".concoct/current/task-plan.md"), "---\nid: [\n---\n")
		}},
		{"multiple review outcomes", func(t *testing.T, r string) {
			path := filepath.Join(r, ".concoct/current/review-01.md")
			data, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(data), "`changes-requested`", "`changes-requested` and `approved`", 1))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := fixture(t, "implementation-complete", "changes-requested", "")
			tt.mutate(t, root)
			got := Detect(root)
			if got.State != Invalid || len(got.Diagnostics) == 0 {
				t.Fatalf("got %#v", got)
			}
		})
	}
}

func TestLaterReviewSupersedesRecoveryMetadata(t *testing.T) {
	tests := []struct {
		name, firstOutcome, extra string
	}{
		{"remediation", "changes-requested", "remediates-review: review-01.md\n"},
		{"blocked resolution", "blocked", "blocked-review-resolution:\n  review: review-01.md\n  route: review\n  recorded-by: developer\n  evidence:\n    - evidence.md\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := fixture(t, "implementation-complete", tt.firstOutcome, tt.extra)
			write(t, filepath.Join(root, ".concoct/current/review-02.md"), reviewFile(2, "approved"))
			got := Detect(root)
			if got.State != Approved {
				t.Fatalf("state = %s, want %s; diagnostics: %v", got.State, Approved, got.Diagnostics)
			}
		})
	}
}

func TestHistoricalRemediationDoesNotBypassLatestBlockedResolution(t *testing.T) {
	root := fixture(t, "implementation-in-progress", "changes-requested", "remediates-review: review-01.md\nblocked-review-resolution:\n  review: review-02.md\n  route: code\n  recorded-by: developer\n  evidence:\n    - evidence.md\n")
	write(t, filepath.Join(root, ".concoct/current/review-02.md"), reviewFile(2, "blocked"))

	got := Detect(root)
	if got.State != InProgress {
		t.Fatalf("state = %s, want %s; diagnostics: %v", got.State, InProgress, got.Diagnostics)
	}
}

func TestDiscoverableStatusDoesNotMutate(t *testing.T) {
	root := fixture(t, "planned", "", "")
	before := snapshot(t, root)
	_ = Detect(root)
	after := snapshot(t, root)
	if before != after {
		t.Fatal("Detect modified project files")
	}
}

func TestPolicyExternalSatisfactionRequiresSafeDurableEvidence(t *testing.T) {
	valid := "policy-activity-evidence:\n  - activity: independent-review\n    disposition: externally-satisfied\n    reason: external audit accepted the change\n    recorded-by: developer\n    evidence:\n      - evidence.md\n"
	root := fixture(t, "implementation-complete", "", valid)
	report := Detect(root)
	if report.State != Complete || report.Next != "concoct archive" {
		t.Fatalf("state = %s: %v", report.State, report.Diagnostics)
	}
	if !strings.Contains(report.String(), "Policy independent-review: externally-satisfied") || !strings.Contains(report.String(), "source .concoct/policy.md") {
		t.Fatalf("report = %s", report.String())
	}

	for name, extra := range map[string]string{
		"missing reason":  strings.Replace(valid, "reason: external audit accepted the change\n", "", 1),
		"unsafe evidence": strings.Replace(valid, "evidence.md", "../outside", 1),
	} {
		t.Run(name, func(t *testing.T) {
			got := Detect(fixture(t, "implementation-complete", "", extra))
			if got.State != Invalid || len(got.Diagnostics) == 0 {
				t.Fatalf("report = %#v", got)
			}
			if strings.Contains(got.String(), "externally-satisfied") {
				t.Fatalf("invalid evidence was rendered as satisfied:\n%s", got.String())
			}
		})
	}
}

func TestPolicyExternalSatisfactionRejectsSymlinkedParent(t *testing.T) {
	extra := "policy-activity-evidence:\n  - activity: independent-review\n    disposition: externally-satisfied\n    reason: external audit accepted the change\n    recorded-by: developer\n    evidence:\n      - linked/audit.md\n"
	root := fixture(t, "implementation-complete", "", extra)
	outside := t.TempDir()
	write(t, filepath.Join(outside, "audit.md"), "accepted\n")
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	got := Detect(root)
	if got.State != Invalid || !strings.Contains(strings.Join(got.Diagnostics, " "), "non-symlink") {
		t.Fatalf("report = %#v", got)
	}
}

func TestPolicyExternalSatisfactionAllowsBenignDoubleDotFilename(t *testing.T) {
	extra := "policy-activity-evidence:\n  - activity: independent-review\n    disposition: externally-satisfied\n    reason: external audit accepted the change\n    recorded-by: developer\n    evidence:\n      - audit..md\n"
	root := fixture(t, "implementation-complete", "", extra)
	write(t, filepath.Join(root, "audit..md"), "accepted\n")
	got := Detect(root)
	if got.State != Complete || got.Next != "concoct archive" {
		t.Fatalf("report = %#v", got)
	}
}

func TestInvalidTaskNeverRendersExternalEvidenceAsSatisfied(t *testing.T) {
	extra := "policy-activity-evidence:\n  - activity: independent-review\n    disposition: externally-satisfied\n    reason: external audit accepted the change\n    recorded-by: developer\n    evidence:\n      - evidence.md\n"
	root := fixture(t, "implementation-complete", "", extra)
	path := filepath.Join(root, ".concoct/current/task-plan.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, strings.Replace(string(data), "title: Demo task", "title:", 1))
	got := Detect(root)
	if got.State != Invalid || strings.Contains(got.String(), "externally-satisfied") {
		t.Fatalf("report = %#v\n%s", got, got.String())
	}
}

func TestPolicyExternalSatisfactionRejectsUnsupportedActivity(t *testing.T) {
	extra := "policy-activity-evidence:\n  - activity: development\n    disposition: externally-satisfied\n    reason: another system built the change\n    recorded-by: developer\n    evidence:\n      - evidence.md\n"
	got := Detect(fixture(t, "implementation-complete", "", extra))
	if got.State != Invalid || !strings.Contains(strings.Join(got.Diagnostics, " "), "only for independent-review") {
		t.Fatalf("report = %#v", got)
	}
}

func TestPolicyDispositionMatrix(t *testing.T) {
	reports := []Report{
		Detect(fixture(t, "planned", "", "")),
		Detect(fixture(t, "implementation-complete", "approved", "")),
		Detect(fixture(t, "implementation-complete", "", "policy-activity-evidence:\n  - activity: independent-review\n    disposition: externally-satisfied\n    reason: external audit accepted the change\n    recorded-by: developer\n    evidence:\n      - evidence.md\n")),
	}
	nonRequiredRoot := fixture(t, "implementation-complete", "", "")
	policyPath := filepath.Join(nonRequiredRoot, ".concoct/policy.md")
	policy, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	policy = []byte(strings.Replace(strings.Replace(string(policy), "  - independent-review\n  - archival\n  - integration\n", "  - archival\n  - integration\nnot-required-reasons:\n  - independent-review: repository accepts developer verification\n", 1), "  - reviewer-approval-before-archive\n", "", 1))
	write(t, policyPath, string(policy))
	reports = append(reports, Detect(nonRequiredRoot))

	wantDispositions := map[string]bool{"completed": true, "not-required": true, "not-applicable": true, "externally-satisfied": true, "blocked": true}
	seen := map[string]bool{}
	for _, report := range reports {
		if report.State == Invalid {
			t.Fatalf("matrix fixture invalid: %#v", report)
		}
		if len(report.PolicyActivities) != 6 {
			t.Fatalf("activity count = %d", len(report.PolicyActivities))
		}
		for _, activity := range report.PolicyActivities {
			if !wantDispositions[activity.Disposition] {
				t.Fatalf("unknown disposition %#v", activity)
			}
			if activity.Requirement != "required" && activity.Requirement != "not-required" {
				t.Fatalf("unknown requirement %#v", activity)
			}
			if activity.Source != ".concoct/policy.md" || activity.Reason == "" {
				t.Fatalf("incomplete attribution %#v", activity)
			}
			seen[activity.Disposition] = true
		}
	}
	for disposition := range wantDispositions {
		if !seen[disposition] {
			t.Errorf("disposition %s was not resolved", disposition)
		}
	}
}

func TestGitTaskRejectsPolicyThatOmitsIntegration(t *testing.T) {
	root := fixture(t, "implementation-complete", "approved", "git:\n  enabled: true\n  trunk: trunk\n  task-branch: concoct/app-001-demo\n  base: abc123\n")
	policyPath := filepath.Join(root, ".concoct/policy.md")
	data, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "  - integration\n", "not-required-reasons:\n  - integration: non-Git delivery only\n", 1))
	if err := os.WriteFile(policyPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	got := Detect(root)
	if got.State != Invalid || !strings.Contains(strings.Join(got.Diagnostics, " "), "requires integration") {
		t.Fatalf("report = %#v", got)
	}
}

func TestReserveReviewRefusesExternallySatisfiedReview(t *testing.T) {
	extra := "policy-activity-evidence:\n  - activity: independent-review\n    disposition: externally-satisfied\n    reason: external audit accepted the change\n    recorded-by: developer\n    evidence:\n      - evidence.md\n"
	root := fixture(t, "implementation-complete", "", extra)
	if _, err := ReserveReview(root); err == nil || !strings.Contains(err.Error(), "not required") {
		t.Fatalf("reserve error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".concoct/current/review-01.md")); !os.IsNotExist(err) {
		t.Fatalf("reservation created review: %v", err)
	}
}

func TestExternalReviewSatisfactionRejectsReviewEvidence(t *testing.T) {
	extra := "policy-activity-evidence:\n  - activity: independent-review\n    disposition: externally-satisfied\n    reason: external audit accepted the change\n    recorded-by: developer\n    evidence:\n      - evidence.md\n"
	got := Detect(fixture(t, "implementation-complete", "approved", extra))
	if got.State != Invalid || !strings.Contains(strings.Join(got.Diagnostics, " "), "cannot coexist") {
		t.Fatalf("report = %#v", got)
	}
}

func TestNotRequiredReviewRejectsReviewEvidence(t *testing.T) {
	root := fixture(t, "implementation-complete", "approved", "")
	path := filepath.Join(root, ".concoct/policy.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(strings.Replace(string(data), "  - independent-review\n  - archival\n  - integration\n", "  - archival\n  - integration\nnot-required-reasons:\n  - independent-review: external audit is required\n", 1), "  - reviewer-approval-before-archive\n", "", 1))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	got := Detect(root)
	if got.State != Invalid || !strings.Contains(strings.Join(got.Diagnostics, " "), "cannot coexist") {
		t.Fatalf("report = %#v", got)
	}
}

func TestInspectNextActionEvidenceUsesPlanEligibility(t *testing.T) {
	root := fixture(t, "", "", "")
	write(t, filepath.Join(root, ".concoct/roadmap.md"), "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n## APP-001 — Eligible\n- Status: `planned`\n- Priority: `high`\n- Depends on: `none`\n## APP-002 — Blocked\n- Status: `planned`\n- Priority: `critical`\n- Depends on: APP-003\n")
	evidence, err := InspectNextActionEvidence(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence.RoadmapItems) != 2 || !evidence.RoadmapItems[0].Eligible || evidence.RoadmapItems[1].Eligible || !strings.Contains(evidence.RoadmapItems[1].Blocker, "unsatisfied dependency APP-003") {
		t.Fatalf("evidence = %#v", evidence)
	}
	if evidence.RoadmapItems[0].Priority != "high" || len(evidence.SupportedOrigins) != 2 {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestValidateReconciliationCandidateRestrictsTransitionsAndRequiresAcceptedEvidence(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".concoct/roadmap.md"), "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n\n## CON-101 — Example\n- Status: `candidate`\n")
	write(t, filepath.Join(root, ".concoct/capabilities.md"), "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Capabilities\n\n## CAP-101 — Example\n- Status: `active`\n")
	roadmap, err := os.ReadFile(filepath.Join(root, ".concoct/roadmap.md"))
	if err != nil {
		t.Fatal(err)
	}
	capabilities, err := os.ReadFile(filepath.Join(root, ".concoct/capabilities.md"))
	if err != nil {
		t.Fatal(err)
	}
	promoted := []byte(strings.Replace(string(roadmap), "`candidate`", "`planned`", 1))
	if err := ValidateReconciliationCandidate(root, promoted, capabilities, []ReconciliationChange{{Target: "roadmap", ID: "CON-101"}}); err != nil {
		t.Fatalf("candidate promotion rejected: %v", err)
	}
	if err := ValidateReconciliationCandidate(root, []byte(strings.Replace(string(roadmap), "`candidate`", "`blocked`", 1)), capabilities, []ReconciliationChange{{Target: "roadmap", ID: "CON-101"}}); err == nil || !strings.Contains(err.Error(), "unauthorized roadmap status") {
		t.Fatalf("unauthorized transition error = %v", err)
	}
	changedCapability := []byte(strings.Replace(string(capabilities), "- Status: `active`", "- Status: `active`\n- Archive: `.concoct/archive/2026-01-01-CON-101/`", 1))
	if err := ValidateReconciliationCandidate(root, roadmap, changedCapability, []ReconciliationChange{{Target: "capabilities", ID: "CAP-101"}}); err == nil || !strings.Contains(err.Error(), "archive provenance") {
		t.Fatalf("missing provenance error = %v", err)
	}
	write(t, filepath.Join(root, ".concoct/archive/2026-01-01-CON-101/summary.md"), "---\nstatus: delivered\ndelivery: complete\n---\n# Summary\n")
	if err := ValidateReconciliationCandidate(root, roadmap, changedCapability, []ReconciliationChange{{Target: "capabilities", ID: "CAP-101"}}); err != nil {
		t.Fatalf("accepted capability reconciliation rejected: %v", err)
	}
}

func TestInspectPlanEligibilityValidatesCapabilityPrerequisites(t *testing.T) {
	capability := func(status string) string {
		return fmt.Sprintf("---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Capabilities\n## CAP-001 — Foundation\n- Status: `%s`\n- Archive: `.concoct/archive/2026-01-01-CAP-001-foundation/`\n### Limitations\n\n- Planner must inspect this limitation.\n", status)
	}
	setup := func(t *testing.T, prerequisite, capabilities string) string {
		root := fixture(t, "", "", "")
		write(t, filepath.Join(root, ".concoct/roadmap.md"), fmt.Sprintf("---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n## APP-001 — Demo\n- Status: `planned`\n- Depends on: `none`\n- Capability prerequisites: %s\n", prerequisite))
		write(t, filepath.Join(root, ".concoct/capabilities.md"), capabilities)
		return root
	}

	t.Run("accepted with limitation context", func(t *testing.T) {
		got, err := InspectPlanEligibility(setup(t, "CAP-001", capability("active")), "APP-001")
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Prerequisites) != 1 || !strings.Contains(got.Prerequisites[0].Limitations, "inspect this limitation") || len(got.Prerequisites[0].Archives) != 1 {
			t.Fatalf("eligibility = %#v", got)
		}
	})

	tests := []struct {
		name, prerequisite, capabilities string
		want                             []string
	}{
		{"missing", "CAP-002", capability("active"), []string{"CAP-002 is missing"}},
		{"inactive", "CAP-001", capability("limited"), []string{"status limited"}},
		{"duplicate declaration", "CAP-001, CAP-001", capability("active"), []string{"duplicate capability prerequisite CAP-001"}},
		{"malformed declaration", "cap-one", capability("active"), []string{"malformed Capability prerequisite cap-one"}},
		{"duplicate record", "CAP-001", capability("active") + "\n## CAP-001 — Again\n- Status: `active`\n", []string{"roadmap item APP-001", "duplicate capability CAP-001", "correct .concoct/capabilities.md before retrying planning"}},
		{"missing record status", "CAP-001", strings.Replace(capability("active"), "- Status: `active`\n", "", 1), []string{"roadmap item APP-001", "CAP-001 missing Status", "correct .concoct/capabilities.md before retrying planning"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := InspectPlanEligibility(setup(t, tt.prerequisite, tt.capabilities), "APP-001")
			if err == nil {
				t.Fatal("expected eligibility error")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error = %v, want fragment %q", err, want)
				}
			}
		})
	}
}

func fixture(t *testing.T, status, reviewStatus, extra string) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "AGENTS.md"), "---\ninstruction-layer: project-guidance\n---\n# Agents\n")
	write(t, filepath.Join(root, ".concoct/policy.md"), "---\ninstruction-layer: policy\nrequired-phases:\n  - product-ownership\n  - task-planning\n  - development\n  - independent-review\n  - archival\n  - integration\napproval-gates:\n  - reviewer-approval-before-archive\n  - archive-before-integration\ngit-strategy: task-branch-with-squash-integration\n---\n# Policy\n")
	write(t, filepath.Join(root, ".concoct/roadmap.md"), roadmap(map[bool]string{true: "active", false: "planned"}[status != ""]))
	write(t, filepath.Join(root, ".concoct/capabilities.md"), "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Capabilities\n")
	if status != "" {
		write(t, filepath.Join(root, ".concoct/current/task-plan.md"), task("APP-001", status, extra))
		notes := "# Notes\n\nImplementation evidence and handoff to reviewer. Blocker disposition.\n"
		if extra != "" && strings.Contains(extra, "remediates") {
			notes += "Finding 1 fixed.\n"
		}
		write(t, filepath.Join(root, ".concoct/current/notes.md"), notes)
	} else {
		write(t, filepath.Join(root, ".concoct/current/task-plan.md"), "# task-plan.md\n")
		write(t, filepath.Join(root, ".concoct/current/notes.md"), "# notes.md\n_Add decisions here._\n_Record meaningful verification results here._\n")
	}
	if reviewStatus != "" {
		write(t, filepath.Join(root, ".concoct/current/review-01.md"), reviewFile(1, reviewStatus))
	}
	write(t, filepath.Join(root, "evidence.md"), "resolved\n")
	return root
}
func roadmap(status string) string {
	return fmt.Sprintf("---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n## APP-001 — Demo\n- Status: `%s`\n", status)
}
func task(id, status, extra string) string {
	return fmt.Sprintf("---\nid: %s\ntitle: Demo task\nroadmap-id: %s\nstatus: %s\ncreated: 2026-01-01\nupdated: 2026-01-01\n%scapability-impact:\n  type: add\n  ids: [CAP-001]\n  rationale: Demo impact\n---\n# Task\n", id, id, status, extra)
}
func reviewFile(number int, status string) string {
	return fmt.Sprintf("---\ntask-id: APP-001\nreview: %d\nstatus: %s\ncreated: 2026-01-01\npersona: reviewer\n---\n# Review\n## Outcome\n\n`%s`\n\n## Findings\n\n### Finding 1 — example\n", number, status, status)
}
func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().IsRegular() {
			data, _ := os.ReadFile(path)
			fmt.Fprintf(&b, "%s:%x\n", path, data)
		}
		return nil
	})
	return b.String()
}
