package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopher-launch/concoct/internal/defaults"
)

func TestRenderRolesAndModesDeterministically(t *testing.T) {
	tests := []struct {
		name, command, status, review, extra, want, golden string
	}{
		{"next", "next", "", "", "", "Mode: `next-action-recommendation`", "next.golden"},
		{"roadmap", "roadmap", "", "", "", "Persona: `product-owner`", "roadmap.golden"},
		{"plan", "plan", "", "", "", "Persona: `task-planner`", "plan.golden"},
		{"code initial", "code", "planned", "", "", "Mode: `implementation`", "code-initial.golden"},
		{"code continuation", "code", "implementation-in-progress", "", "", "Mode: `implementation-continuation`", "code-continuation.golden"},
		{"code remediation", "code", "implementation-complete", "changes-requested", "", "Mode: `review-remediation`", "code-remediation.golden"},
		{"review initial", "review", "implementation-complete", "", "", "Next review artifact: `.concoct/current/review-01.md`", "review-initial.golden"},
		{"archive", "archive", "implementation-complete", "approved", "", "Persona: `archivist`", "archive.golden"},
		{"review after remediation", "review", "implementation-complete", "changes-requested", "remediates-review: review-01.md\n", "Mode: `post-remediation-review`", "review-after-remediation.golden"},
		{"blocked code recovery", "code", "implementation-in-progress", "blocked", resolution("code", "developer"), "Mode: `blocked-review-recovery-to-code`", "code-blocked-recovery.golden"},
		{"blocked review recovery", "review", "implementation-complete", "blocked", resolution("review", "task-planner"), "Mode: `blocked-review-recovery-to-review`", "review-blocked-recovery.golden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := fixture(t, tt.status, tt.review, tt.extra)
			request := Request{Command: tt.command}
			if tt.command == "plan" {
				request.RoadmapID = "APP-002"
			}
			first, err := Render(root, request)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Render(root, request)
			if err != nil {
				t.Fatal(err)
			}
			if string(first) != string(second) {
				t.Fatal("rendering changed without repository changes")
			}
			composition, err := RenderComposition(root, request)
			if err != nil {
				t.Fatal(err)
			}
			if !composition.ConservesBytes() || !bytes.Equal(composition.Bytes(), first) {
				t.Fatalf("composition changed %s prompt: %#v", tt.name, composition.Components)
			}
			t.Logf("controlled baseline %s: %d prompt bytes", tt.name, composition.ByteCount())
			if !hasComponent(composition, "persona") || !hasComponent(composition, "handoff") || !hasComponent(composition, "input-reference") {
				t.Fatalf("composition is missing required semantic categories for %s: %#v", tt.name, composition.Components)
			}
			goldenPath := filepath.Join("testdata", tt.golden)
			golden, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatal(err)
			}
			if want := currentGolden(t, golden, personaForCommand(tt.command)); !bytes.Equal(first, want) {
				t.Fatalf("rendered prompt differs from %s\n--- want ---\n%s\n--- got ---\n%s", goldenPath, want, first)
			}
			if !strings.Contains(string(first), tt.want) {
				t.Fatalf("output does not contain %q:\n%s", tt.want, first)
			}
			for _, required := range []string{"## Exact inputs to read", "## Authorized updates", "## Expected outcome", "## Validation and completion", "## Recommended next transition", "## Canonical handoff instructions"} {
				if !strings.Contains(string(first), required) {
					t.Errorf("missing section %s", required)
				}
			}
		})
	}
}

func hasComponent(composition Composition, category string) bool {
	for _, component := range composition.Components {
		if component.Category == category {
			return true
		}
	}
	return false
}

func TestRenderCompositionConservesExactPromptBytes(t *testing.T) {
	root := fixture(t, "planned", "", "")
	composition, err := RenderComposition(root, Request{Command: "code"})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Render(root, Request{Command: "code"})
	if err != nil {
		t.Fatal(err)
	}
	if !composition.ConservesBytes() || !bytes.Equal(composition.Bytes(), plain) {
		t.Fatalf("composition does not preserve render: bytes=%d components=%#v", len(composition.Bytes()), composition.Components)
	}
	if len(composition.Components) < 6 {
		t.Fatalf("component provenance = %#v", composition.Components)
	}
	want := map[string]string{
		"generated-context":      "prompt.Render",
		"persona":                "built-in:persona-developer",
		"instruction-provenance": "instruction.Compose",
		"input-reference":        "prompt.Render",
		"authorized-update":      "prompt.Render",
		"completion-contract":    "prompt.Render",
		"handoff":                "built-in:handoff-task-planner-to-developer",
	}
	seen := map[string]bool{}
	for _, component := range composition.Components {
		if component.Digest == "" || component.NormalizedDigest == "" {
			t.Fatalf("component lacks digest: %#v", component)
		}
		if source, ok := want[component.Category]; ok && component.Source == source {
			seen[component.Category] = true
		}
	}
	for category := range want {
		if !seen[category] {
			t.Errorf("missing %s component with expected provenance", category)
		}
	}
}

func TestCompositionIdentifiesExactAndNormalizedDuplicates(t *testing.T) {
	var composition Composition
	composition.Append("first", "fixture", InclusionFull, []byte("same\n"))
	composition.Append("exact", "fixture", InclusionFull, []byte("same\n"))
	composition.Append("normalized", "fixture", InclusionFull, []byte("same   \n"))
	if got := composition.Components[1].ExactDuplicateOf; got != 1 {
		t.Fatalf("exact duplicate = %d, want 1", got)
	}
	if got := composition.Components[2].NormalizedDuplicateOf; got != 1 {
		t.Fatalf("normalized duplicate = %d, want 1", got)
	}
}

func TestRenderIncludesEmbeddedPersonaForEveryRoleAndIgnoresLocalCopies(t *testing.T) {
	tests := []struct {
		command, status, review, persona, marker string
	}{
		{"next", "", "", "product-owner", "# Product Owner Persona"},
		{"roadmap", "", "", "product-owner", "# Product Owner Persona"},
		{"plan", "", "", "task-planner", "# Task Planner Persona"},
		{"code", "planned", "", "developer", "# Developer Persona"},
		{"review", "implementation-complete", "", "reviewer", "# Reviewer Persona"},
		{"archive", "implementation-complete", "approved", "archivist", "# Archivist Persona"},
	}
	for _, tt := range tests {
		t.Run(tt.command+"-"+tt.persona, func(t *testing.T) {
			root := fixture(t, tt.status, tt.review, "")
			write(t, filepath.Join(root, ".concoct", "personas", tt.persona+".md"), "# Shadow persona\n")
			request := Request{Command: tt.command}
			if tt.command == "plan" {
				request.RoadmapID = "APP-002"
			}
			got, err := Render(root, request)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{
				"## Selected built-in persona",
				"Source: `built-in:persona-" + tt.persona + "`",
				tt.marker,
				"- Layer `persona`; source `built-in:persona-" + tt.persona + "`",
			} {
				if !strings.Contains(string(got), want) {
					t.Errorf("rendered prompt missing %q", want)
				}
			}
			if strings.Contains(string(got), "# Shadow persona") {
				t.Fatal("repository-local persona shadowed executable-owned persona")
			}
		})
	}
}

func TestRenderRejectsWrongStateAndUnsatisfiedDependency(t *testing.T) {
	root := fixture(t, "planned", "", "")
	if _, err := Render(root, Request{Command: "next"}); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Fatalf("next wrong-state error = %v", err)
	}
	if _, err := Render(root, Request{Command: "roadmap"}); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Fatalf("wrong-state error = %v", err)
	}
	root = fixture(t, "", "", "")
	roadmapPath := filepath.Join(root, ".concoct", "roadmap.md")
	data, _ := os.ReadFile(roadmapPath)
	write(t, roadmapPath, strings.Replace(string(data), "APP-001 — Delivered\n- Status: `delivered`", "APP-001 — Delivered\n- Status: `planned`", 1))
	if _, err := Render(root, Request{Command: "plan", RoadmapID: "APP-002"}); err == nil || !strings.Contains(err.Error(), "unsatisfied dependency") {
		t.Fatalf("dependency error = %v", err)
	}
}

func TestRenderUsesResolvedExternalReviewSatisfaction(t *testing.T) {
	root := fixture(t, "implementation-complete", "", "")
	path := filepath.Join(root, ".concoct/current/task-plan.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	evidence := "policy-activity-evidence:\n  - activity: independent-review\n    disposition: externally-satisfied\n    reason: external audit accepted the change\n    recorded-by: developer\n    evidence:\n      - evidence.md\n"
	write(t, path, strings.Replace(string(data), "capability-impact:\n", evidence+"capability-impact:\n", 1))
	if _, err := Render(root, Request{Command: "review"}); err == nil || !strings.Contains(err.Error(), "not required") {
		t.Fatalf("review error = %v", err)
	}
	if _, err := Render(root, Request{Command: "archive"}); err != nil {
		t.Fatalf("archive render: %v", err)
	}
}

func TestDeveloperPromptUsesResolvedNonRequiredReviewHandoff(t *testing.T) {
	root := fixture(t, "implementation-in-progress", "", "")
	path := filepath.Join(root, ".concoct/policy.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(strings.Replace(string(data), "  - independent-review\n  - archival\n  - integration\n", "  - archival\n  - integration\nnot-required-reasons:\n  - independent-review: repository accepts developer verification\n", 1), "  - reviewer-approval-before-archive\n", "", 1))
	write(t, path, string(data))

	got, err := Render(root, Request{Command: "code"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Policy-specific Developer handoff", "`## Handoff to archivist`", "leave a fresh Archivist handoff", "`concoct archive` after completion validation succeeds"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("prompt missing %q:\n%s", want, got)
		}
	}
}

func TestRenderRejectsInvariantWeakeningWithoutPartialOutput(t *testing.T) {
	root := fixture(t, "", "", "")
	write(t, filepath.Join(root, "AGENTS.md"), "---\ninstruction-layer: project-guidance\nweaken-controls:\n  - evidence-integrity\n---\n# Agents\n")
	got, err := Render(root, Request{Command: "next"})
	if err == nil || !strings.Contains(err.Error(), "evidence-integrity") {
		t.Fatalf("weakening error = %v", err)
	}
	if got != nil {
		t.Fatalf("partial prompt returned: %q", got)
	}
}

func TestRenderRejectsPolicyProjectConflictWithoutPartialOutput(t *testing.T) {
	root := fixture(t, "", "", "")
	write(t, filepath.Join(root, "AGENTS.md"), "---\ninstruction-layer: project-guidance\ngit-strategy: direct-main\n---\n# Agents\n")
	got, err := Render(root, Request{Command: "next"})
	if err == nil {
		t.Fatal("expected policy/project conflict")
	}
	for _, want := range []string{"project-guidance", "AGENTS.md", "git-strategy", "policy layer", ".concoct/policy.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
	if got != nil {
		t.Fatalf("partial prompt returned: %q", got)
	}
}

func TestNextCoversNoActionableWork(t *testing.T) {
	root := fixture(t, "", "", "")
	write(t, filepath.Join(root, ".concoct/roadmap.md"), "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n")
	got := assertNextGolden(t, root, "next-no-work.golden")
	for _, want := range []string{"### Roadmap items\n\n- None recorded.", "report no actionable recorded work"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("prompt missing %q:\n%s", want, got)
		}
	}
}

func TestNextFullOutputOutcomeEvidence(t *testing.T) {
	tests := []struct {
		name, golden string
		prepare      func(*testing.T, string)
	}{
		{
			name:   "supported product input",
			golden: "next-product-input.golden",
			prepare: func(t *testing.T, root string) {
				roadmap := filepath.Join(root, ".concoct/roadmap.md")
				data, err := os.ReadFile(roadmap)
				if err != nil {
					t.Fatal(err)
				}
				write(t, roadmap, strings.Replace(string(data), "- Status: `planned`", "- Status: `candidate`", 1))
			},
		},
		{
			name:   "roadmap reconciliation",
			golden: "next-reconciliation.golden",
			prepare: func(t *testing.T, root string) {
				write(t, filepath.Join(root, ".concoct/roadmap.md"), "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n## APP-001 — Delivered\n- Status: `delivered`\n- Depends on: `none`\n")
			},
		},
		{
			name:   "specific blocker",
			golden: "next-blocker.golden",
			prepare: func(t *testing.T, root string) {
				roadmap := filepath.Join(root, ".concoct/roadmap.md")
				data, err := os.ReadFile(roadmap)
				if err != nil {
					t.Fatal(err)
				}
				write(t, roadmap, strings.Replace(string(data), "- Depends on: APP-001", "- Depends on: APP-999", 1))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := fixture(t, "", "", "")
			tt.prepare(t, root)
			assertNextGolden(t, root, tt.golden)
		})
	}
}

func TestNextRejectsInvalidCanonicalEvidenceWithoutMutation(t *testing.T) {
	root := fixture(t, "", "", "")
	roadmap := filepath.Join(root, ".concoct/roadmap.md")
	data, err := os.ReadFile(roadmap)
	if err != nil {
		t.Fatal(err)
	}
	write(t, roadmap, strings.Replace(string(data), "- Status: `planned`", "- Status: `mystery`", 1))
	before, err := os.ReadFile(roadmap)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Render(root, Request{Command: "next"}); err == nil || !strings.Contains(err.Error(), "invalid workflow state") {
		t.Fatalf("invalid evidence error = %v", err)
	}
	after, err := os.ReadFile(roadmap)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("next mutated invalid canonical evidence")
	}
}

func assertNextGolden(t *testing.T, root, name string) []byte {
	t.Helper()
	before := workflowFiles(t, root)
	first, err := Render(root, Request{Command: "next"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(root, Request{Command: "next"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("next output is not deterministic")
	}
	if after := workflowFiles(t, root); !bytes.Equal(before, after) {
		t.Fatal("next mutated workflow artifacts")
	}
	path := filepath.Join("testdata", name)
	golden, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := currentGolden(t, golden, "product-owner"); !bytes.Equal(first, want) {
		t.Fatalf("rendered prompt differs from %s\n--- want ---\n%s\n--- got ---\n%s", path, want, first)
	}
	return first
}

func currentGolden(t *testing.T, golden []byte, persona string) []byte {
	t.Helper()
	personaID := "persona-" + persona
	body, err := defaults.Read(personaID, "test")
	if err != nil {
		t.Fatal(err)
	}
	want := string(golden)
	for _, role := range []string{"product-owner", "task-planner", "developer", "reviewer", "archivist"} {
		old := "- `.concoct/personas/" + role + ".md`"
		label := strings.ReplaceAll(strings.Title(strings.ReplaceAll(role, "-", " ")), " ", " ")
		want = strings.ReplaceAll(want, old, "- the selected executable-owned "+label+" persona rendered in this prompt")
	}
	section := "\n## Selected built-in persona\n\nSource: `built-in:" + personaID + "` (executable-owned; repository-local persona files are not inputs).\n\n" + strings.TrimSpace(string(body)) + "\n"
	want = strings.Replace(want, "\n## Effective instruction sources", section+"\n## Effective instruction sources", 1)
	want = strings.Replace(want, "\n- Layer `task-context`; sources selected below for the active command", "\n- Layer `persona`; source `built-in:"+personaID+"`\n- Layer `task-context`; sources selected below for the active command", 1)
	return []byte(want)
}

func personaForCommand(command string) string {
	switch command {
	case "next", "roadmap":
		return "product-owner"
	case "plan":
		return "task-planner"
	case "code":
		return "developer"
	case "review":
		return "reviewer"
	case "archive":
		return "archivist"
	default:
		panic("unsupported command " + command)
	}
}

func workflowFiles(t *testing.T, root string) []byte {
	t.Helper()
	var snapshot bytes.Buffer
	for _, rel := range []string{".concoct/roadmap.md", ".concoct/capabilities.md", ".concoct/current/task-plan.md", ".concoct/current/notes.md"} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		snapshot.WriteString(rel)
		snapshot.Write(data)
	}
	return snapshot.Bytes()
}

func TestPlanIncludesAcceptedPrerequisiteContextAndArchive(t *testing.T) {
	root := fixture(t, "", "", "")
	road := filepath.Join(root, ".concoct/roadmap.md")
	data, err := os.ReadFile(road)
	if err != nil {
		t.Fatal(err)
	}
	write(t, road, strings.Replace(string(data), "- Depends on: APP-001", "- Depends on: APP-001\n- Capability prerequisites: CAP-001", 1))
	write(t, filepath.Join(root, ".concoct/capabilities.md"), "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Capabilities\n## CAP-001 — Delivered\n- Status: `active`\n- Archive: `.concoct/archive/2026-01-01-CAP-001-delivered/`\n### Limitations\n\n- Limited example.\n")
	write(t, filepath.Join(root, ".concoct/archive/2026-01-01-CAP-001-delivered/summary.md"), "# Summary\n")
	got, err := Render(root, Request{Command: "plan", RoadmapID: "APP-002"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"`CAP-001` — `active` accepted truth; documented limitations", ".concoct/archive/2026-01-01-CAP-001-delivered/summary.md", "Task Planner must inspect"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("prompt missing %q:\n%s", want, got)
		}
	}
}

func fixture(t *testing.T, status, reviewStatus, extra string) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "AGENTS.md"), "---\ninstruction-layer: project-guidance\n---\n# Agents\n")
	write(t, filepath.Join(root, ".concoct/protocol.md"), "---\ninstruction-layer: protocol\nprotected-controls:\n  - completed-review-immutability\n  - evidence-integrity\n  - invalid-state-refusal\n  - workflow-artifact-ownership\n---\n# Protocol\n")
	write(t, filepath.Join(root, ".concoct/policy.md"), "---\ninstruction-layer: policy\nrequired-phases:\n  - product-ownership\n  - task-planning\n  - development\n  - independent-review\n  - archival\n  - integration\napproval-gates:\n  - reviewer-approval-before-archive\n  - archive-before-integration\ngit-strategy: task-branch-with-squash-integration\n---\n# Policy\n")
	roadStatus := "planned"
	if status != "" {
		roadStatus = "active"
	}
	write(t, filepath.Join(root, ".concoct/roadmap.md"), fmt.Sprintf("---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Roadmap\n## APP-001 — Delivered\n- Status: `delivered`\n- Depends on: `none`\n## APP-002 — Demo\n- Status: `%s`\n- Depends on: APP-001\n", roadStatus))
	write(t, filepath.Join(root, ".concoct/capabilities.md"), "---\nversion: 1\nproject: demo\nupdated: 2026-01-01\n---\n# Capabilities\n")
	assets := map[string]string{
		".concoct/prompts/roadmap/human-roadmap-input.md": "# Product Owner handoff", ".concoct/prompts/roadmap/next-action-recommendation.md": "# Next action handoff", ".concoct/prompts/handoffs/product-owner-to-task-planner.md": "# Planner handoff", ".concoct/prompts/handoffs/task-planner-to-developer.md": "# Developer handoff", ".concoct/prompts/handoffs/reviewer-to-developer.md": "# Remediation handoff", ".concoct/prompts/handoffs/developer-to-reviewer.md": "# Reviewer handoff", ".concoct/prompts/handoffs/reviewer-to-archivist.md": "# Archivist handoff",
	}
	for path, body := range assets {
		write(t, filepath.Join(root, path), body)
	}
	if status == "" {
		write(t, filepath.Join(root, ".concoct/current/task-plan.md"), "# task-plan.md\n")
		write(t, filepath.Join(root, ".concoct/current/notes.md"), "# notes.md\n_Add decisions here._\n_Record meaningful verification results here._\n")
	} else {
		write(t, filepath.Join(root, ".concoct/current/task-plan.md"), fmt.Sprintf("---\nid: APP-002\ntitle: Demo task\nroadmap-id: APP-002\nstatus: %s\ncreated: 2026-01-01\nupdated: 2026-01-01\n%scapability-impact:\n  type: add\n  ids: [CAP-002]\n  rationale: Demo\n---\n# Task\n", status, extra))
		notes := "# Notes\n\nImplementation evidence and handoff to reviewer. Blocker disposition.\n"
		if strings.Contains(extra, "remediates") {
			notes += "Finding 1 fixed.\n"
		}
		write(t, filepath.Join(root, ".concoct/current/notes.md"), notes)
	}
	if reviewStatus != "" {
		write(t, filepath.Join(root, ".concoct/current/review-01.md"), fmt.Sprintf("---\ntask-id: APP-002\nreview: 1\nstatus: %s\ncreated: 2026-01-01\npersona: reviewer\n---\n# Review\n## Outcome\n\n`%s`\n\n## Findings\n\n### Finding 1 — Demo\n", reviewStatus, reviewStatus))
	}
	write(t, filepath.Join(root, "evidence.md"), "resolved\n")
	return root
}

func resolution(route, role string) string {
	return fmt.Sprintf("blocked-review-resolution:\n  review: review-01.md\n  route: %s\n  recorded-by: %s\n  evidence:\n    - evidence.md\n", route, role)
}
func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
