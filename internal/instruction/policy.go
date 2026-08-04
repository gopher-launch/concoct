package instruction

import (
	"fmt"
	"sort"
	"strings"
)

// Activity is a finite lifecycle activity governed by project policy.
type Activity string

const (
	ProductOwnership  Activity = "product-ownership"
	TaskPlanning      Activity = "task-planning"
	Development       Activity = "development"
	IndependentReview Activity = "independent-review"
	Archival          Activity = "archival"
	Integration       Activity = "integration"
)

// Requirement records whether an activity is selected by the policy. Runtime
// applicability (for example integration in a non-Git repository) is resolved
// separately by workflow evidence.
type Requirement string

const (
	Required    Requirement = "required"
	NotRequired Requirement = "not-required"
)

// Policy is the closed, validated policy selected by a project.
type Policy struct {
	Requirements  map[Activity]Requirement
	Reasons       map[Activity]string
	ApprovalGates map[string]bool
	GitStrategy   string
}

var knownActivities = map[string]Activity{
	"product-ownership": ProductOwnership, "task-planning": TaskPlanning,
	"development": Development, "independent-review": IndependentReview,
	"archival": Archival, "integration": Integration,
}

var knownGates = map[string]bool{
	"reviewer-approval-before-archive": true,
	"archive-before-integration":       true,
}

// ParsePolicy validates the policy-owned declarations after declaration
// ownership has been established. Omitting an activity is an explicit
// not-required selection; no runtime artifact absence is interpreted as a
// skip.
func ParsePolicy(decl declarationSet) (Policy, error) {
	p := Policy{Requirements: map[Activity]Requirement{}, Reasons: map[Activity]string{}, ApprovalGates: map[string]bool{}, GitStrategy: decl.scalar["git-strategy"]}
	if p.GitStrategy != "task-branch-with-squash-integration" {
		return Policy{}, fmt.Errorf("git-strategy %q is unsupported; the only managed strategy is task-branch-with-squash-integration", p.GitStrategy)
	}
	seen := map[Activity]bool{}
	for _, raw := range decl.lists["required-phases"] {
		activity, ok := knownActivities[raw]
		if !ok {
			return Policy{}, fmt.Errorf("required-phases contains unknown activity %q; use one of %s", raw, strings.Join(activityNames(), ", "))
		}
		if seen[activity] {
			return Policy{}, fmt.Errorf("required-phases contains duplicate activity %q", raw)
		}
		seen[activity] = true
		p.Requirements[activity] = Required
	}
	for _, activity := range knownActivities {
		if !seen[activity] {
			p.Requirements[activity] = NotRequired
		}
	}
	for _, raw := range decl.lists["not-required-reasons"] {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return Policy{}, fmt.Errorf("not-required-reasons entry %q must use activity: reason", raw)
		}
		activity, ok := knownActivities[strings.TrimSpace(parts[0])]
		if !ok {
			return Policy{}, fmt.Errorf("not-required-reasons names unknown activity %q", strings.TrimSpace(parts[0]))
		}
		reason := strings.TrimSpace(parts[1])
		if reason == "" {
			return Policy{}, fmt.Errorf("not-required-reasons for %q requires a reason", activity)
		}
		if p.Required(activity) {
			return Policy{}, fmt.Errorf("not-required-reasons for %q conflicts with required-phases", activity)
		}
		if _, exists := p.Reasons[activity]; exists {
			return Policy{}, fmt.Errorf("not-required-reasons duplicates activity %q", activity)
		}
		p.Reasons[activity] = reason
	}
	for activity, requirement := range p.Requirements {
		if requirement == NotRequired && strings.TrimSpace(p.Reasons[activity]) == "" {
			return Policy{}, fmt.Errorf("required omission %q needs a not-required-reasons entry", activity)
		}
	}
	for _, activity := range []Activity{ProductOwnership, TaskPlanning, Development, Archival, Integration} {
		if p.Requirements[activity] != Required {
			return Policy{}, fmt.Errorf("supported lifecycle requires integration and required-phases must include product-ownership, task-planning, development, archival, and integration; only independent-review may be omitted")
		}
	}
	seenGates := map[string]bool{}
	for _, gate := range decl.lists["approval-gates"] {
		if !knownGates[gate] {
			return Policy{}, fmt.Errorf("approval-gates contains unknown gate %q", gate)
		}
		if seenGates[gate] {
			return Policy{}, fmt.Errorf("approval-gates contains duplicate gate %q", gate)
		}
		seenGates[gate] = true
		p.ApprovalGates[gate] = true
	}
	if p.ApprovalGates["reviewer-approval-before-archive"] && p.Requirements[IndependentReview] != Required {
		return Policy{}, fmt.Errorf("approval-gate reviewer-approval-before-archive requires independent-review in required-phases")
	}
	if p.Requirements[IndependentReview] == Required && !p.ApprovalGates["reviewer-approval-before-archive"] {
		return Policy{}, fmt.Errorf("required independent-review requires approval-gate reviewer-approval-before-archive")
	}
	if !p.ApprovalGates["archive-before-integration"] {
		return Policy{}, fmt.Errorf("required integration requires approval-gate archive-before-integration")
	}
	return p, nil
}

func activityNames() []string {
	out := make([]string, 0, len(knownActivities))
	for name := range knownActivities {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (p Policy) Required(activity Activity) bool { return p.Requirements[activity] == Required }
func (p Policy) Gate(name string) bool           { return p.ApprovalGates[name] }

// IsDefault reports whether a policy reproduces Concoct's shipped lifecycle.
func (p Policy) IsDefault() bool {
	for _, activity := range []Activity{ProductOwnership, TaskPlanning, Development, IndependentReview, Archival} {
		if !p.Required(activity) {
			return false
		}
	}
	return p.Required(Integration) && p.Gate("reviewer-approval-before-archive") && p.Gate("archive-before-integration") && p.GitStrategy == "task-branch-with-squash-integration"
}
