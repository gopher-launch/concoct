package roadmap

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestGroupedFixtureAndEligibility(t *testing.T) {
	roadmap := fixture(t, "testdata/grouped-roadmap.md")
	capabilities := fixture(t, "testdata/capabilities.md")
	doc, err := Parse(roadmap, capabilities)
	if err != nil {
		t.Fatal(err)
	}
	entries := doc.Entries()
	if len(entries) != 5 {
		t.Fatalf("got %d entries, want 5: %#v", len(entries), entries)
	}
	if entries[2].ID != "APP-002" || !strings.Contains(entries[2].Body, "**Acceptance criteria:**\n\n- Preserve this bullet.") {
		t.Fatalf("task prose not preserved: %#v", entries[2])
	}
	if got := ids(doc.EligibleTasks()); !reflect.DeepEqual(got, []string{"APP-002"}) {
		t.Fatalf("eligible tasks = %v, want APP-002", got)
	}
	entries[2].DependsOn[0] = "CHANGED-001"
	if got := doc.Entries()[2].DependsOn[0]; got != "APP-001" {
		t.Fatalf("returned entry mutated document: %s", got)
	}
}

func TestStandaloneAndRoadmapDeliveredDependency(t *testing.T) {
	roadmap := `## APP-001 — Already delivered
- Kind: task
- Status: delivered
- Priority: high

## APP-002 — Standalone ready
- Kind: task
- Status: ready
- Priority: medium
- Depends on: APP-001

Objective stays here.
`
	doc, err := Parse([]byte(roadmap), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(doc.EligibleTasks()); !reflect.DeepEqual(got, []string{"APP-002"}) {
		t.Fatalf("eligible tasks = %v", got)
	}
}

func TestOptionalParentAndSeveralDependencies(t *testing.T) {
	roadmap := `## APP-F001 — Standalone feature
- Kind: feature
- Status: planned
- Priority: high

## APP-001 — Delivered task
- Kind: task
- Status: delivered
- Priority: high

## APP-002 — Ready task
- Kind: task
- Status: ready
- Priority: high
- Parent: APP-F001
- Depends on: APP-001, APP-099
`
	capabilities := `## APP-099 — Earlier delivery
`
	doc, err := Parse([]byte(roadmap), []byte(capabilities))
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(doc.EligibleTasks()); !reflect.DeepEqual(got, []string{"APP-002"}) {
		t.Fatalf("eligible tasks = %v", got)
	}
}

func TestCurrentRepositoryRoadmap(t *testing.T) {
	_, err := Parse(fixture(t, "../../roadmap.md"), fixture(t, "../../capabilities.md"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestMalformedMetadataAndReferences(t *testing.T) {
	const base = `## APP-001 — A task
- Kind: task
- Status: ready
- Priority: high
- Parent: none
- Depends on: none
`
	cases := []struct {
		name, roadmap, capabilities, want string
	}{
		{"duplicate ID", base + base, "", "duplicate ID APP-001"},
		{"unknown parent", strings.Replace(base, "Parent: none", "Parent: APP-F999", 1), "", "unknown parent APP-F999"},
		{"wrong parent kind", base + strings.Replace(strings.Replace(base, "APP-001", "APP-002", 1), "Parent: none", "Parent: APP-001", 1), "", "must be a feature"},
		{"unknown dependency", strings.Replace(base, "Depends on: none", "Depends on: APP-999", 1), "", "unknown dependency APP-999"},
		{"non-task dependency", base + `
## APP-E001 — Epic
- Kind: epic
- Status: planned
- Priority: high
` + strings.Replace(strings.Replace(base, "APP-001", "APP-002", 1), "Depends on: none", "Depends on: APP-E001", 1), "", "is not a task"},
		{"missing kind", strings.Replace(base, "- Kind: task\n", "", 1), "", "missing Kind"},
		{"malformed bullet", strings.Replace(base, "- Kind: task", "- Kind task", 1), "", "malformed metadata bullet"},
		{"unsupported field", strings.Replace(base, "- Kind: task", "- Flavor: mild\n- Kind: task", 1), "", "unsupported metadata field"},
		{"unsupported status", strings.Replace(base, "Status: ready", "Status: unknowable", 1), "", "unsupported status"},
		{"duplicate field", strings.Replace(base, "- Kind: task", "- Kind: task\n- Kind: task", 1), "", "duplicate Kind"},
		{"malformed heading", strings.Replace(base, "APP-001 —", "APP-001 -", 1), "", "malformed entry heading"},
		{"lowercase heading ID", strings.Replace(base, "APP-001 —", "app-001 —", 1), "", `line 1: malformed ID "app-001"`},
		{"malformed dependency", strings.Replace(base, "Depends on: none", "Depends on: app-001", 1), "", "malformed dependency ID"},
		{"contradictory capability", base, "## APP-001 — Delivered\n", "recorded as delivered but roadmap status is ready"},
		{"duplicate capability", base, "## APP-999 — One\n\n## APP-999 — Two\n", "duplicate delivered ID"},
		{"unclosed fence", base + "\n```md\n## APP-999 — Hidden\n", "", "unclosed code fence"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.roadmap), []byte(tc.capabilities))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestSeparatedDependencyCannotMakeTaskEligible(t *testing.T) {
	roadmap := `## APP-001 — Prerequisite
- Kind: task
- Status: planned
- Priority: high

## APP-002 — Candidate
- Kind: task
- Status: ready
- Priority: high

- Depends on: APP-001
`
	_, err := Parse([]byte(roadmap), nil)
	if err == nil || !strings.Contains(err.Error(), "line 11: Depends on metadata for APP-002 is separated") {
		t.Fatalf("error = %v, want line-specific separated-dependency diagnostic", err)
	}
	withProse := strings.Replace(roadmap, "\n- Depends on: APP-001", "\n**Objective:** a paragraph precedes the misplaced field.\n\n- Depends on: APP-001", 1)
	_, err = Parse([]byte(withProse), nil)
	if err == nil || !strings.Contains(err.Error(), "Depends on metadata for APP-002 is separated") {
		t.Fatalf("error after prose = %v, want separated-dependency diagnostic", err)
	}
	withEmptyValue := strings.Replace(roadmap, "- Depends on: APP-001", "- Depends on:", 1)
	_, err = Parse([]byte(withEmptyValue), nil)
	if err == nil || !strings.Contains(err.Error(), "Depends on metadata for APP-002 is separated") {
		t.Fatalf("error for empty separated field = %v, want separated-dependency diagnostic", err)
	}
}

func TestOrdinaryHeadingsAndFencedMetadataRemainProse(t *testing.T) {
	roadmap := `## Phase-001 notes

## FAQ-notes

## APP-001 — Task
- Kind: task
- Status: ready
- Priority: high

**Objective:** describe metadata in prose.

~~~md
- Depends on: this is a fenced example, not task metadata.
~~~
`
	doc, err := Parse([]byte(roadmap), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Entries()) != 1 || !strings.Contains(doc.Entries()[0].Body, "- Depends on: this is a fenced example") {
		t.Fatalf("ordinary prose lost: %#v", doc.Entries())
	}
}

func TestDependencyCycles(t *testing.T) {
	roadmap := `## APP-001 — First
- Kind: task
- Status: ready
- Priority: high
- Depends on: APP-002

## APP-002 — Second
- Kind: task
- Status: planned
- Priority: high
- Depends on: APP-001
`
	_, err := Parse([]byte(roadmap), nil)
	if err == nil || !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("error = %v, want dependency cycle", err)
	}
}

func fixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func ids(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, entry := range entries {
		out[i] = entry.ID
	}
	return out
}
