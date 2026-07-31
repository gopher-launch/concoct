package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gopher-launch/concoct/internal/workflow"
)

type Request struct {
	Command                          string
	RoadmapID                        string
	GitTrunk, GitTaskBranch, GitBase string
}

type roleSpec struct {
	persona, source, mode, outcome, next string
	reads, writes                        []string
}

func Render(root string, request Request) ([]byte, error) {
	context, err := workflow.InspectPromptContext(root)
	if err != nil {
		return nil, err
	}
	spec, err := selectRole(root, request, context)
	if err != nil {
		return nil, err
	}
	var eligibility workflow.PlanEligibility
	var nextEvidence workflow.NextActionEvidence
	if request.Command == "plan" {
		eligibility, err = workflow.InspectPlanEligibility(root, request.RoadmapID)
		if err != nil {
			return nil, err
		}
		for _, prerequisite := range eligibility.Prerequisites {
			spec.reads = append(spec.reads, prerequisite.Archives...)
		}
	} else if request.Command == "next" {
		nextEvidence, err = workflow.InspectNextActionEvidence(root)
		if err != nil {
			return nil, err
		}
		for _, capability := range nextEvidence.Capabilities {
			spec.reads = append(spec.reads, capability.Archives...)
		}
		for _, item := range nextEvidence.RoadmapItems {
			if item.Archive != "" {
				spec.reads = append(spec.reads, item.Archive)
			}
		}
	}
	archives, err := archiveInputs(root, request.RoadmapID, spec.reads)
	if err != nil {
		return nil, err
	}
	spec.reads = append(spec.reads, archives...)
	spec.reads = uniqueSorted(spec.reads)
	spec.writes = uniqueSorted(spec.writes)
	source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(spec.source)))
	if err != nil {
		return nil, fmt.Errorf("read prompt asset %s: %w", spec.source, err)
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "# Concoct %s prompt\n\n", strings.Title(request.Command)) //nolint:staticcheck
	fmt.Fprintf(&b, "- Persona: `%s`\n- Workflow state: `%s`\n- Mode: `%s`\n", spec.persona, context.Report.State, spec.mode)
	if request.RoadmapID != "" {
		fmt.Fprintf(&b, "- Roadmap item: `%s`\n", request.RoadmapID)
	}
	if request.GitTaskBranch != "" {
		fmt.Fprintf(&b, "- Git trunk: `%s`\n- Git task branch: `%s`\n- Git task base: `%s`\n", request.GitTrunk, request.GitTaskBranch, request.GitBase)
	}
	if context.Report.LatestReview != "" {
		fmt.Fprintf(&b, "- Latest review: `%s` (`%s`)\n", context.Report.LatestReview, context.Report.ReviewOutcome)
	}
	if request.Command == "review" {
		fmt.Fprintf(&b, "- Next review artifact: `%s`\n", context.NextReview)
	}
	if request.Command == "plan" {
		fmt.Fprintln(&b, "\n## Accepted capability prerequisites")
		if len(eligibility.Prerequisites) == 0 {
			fmt.Fprintln(&b, "\n- None declared.")
		}
		for _, prerequisite := range eligibility.Prerequisites {
			fmt.Fprintf(&b, "\n- `%s` — `%s` accepted truth", prerequisite.ID, prerequisite.Status)
			if prerequisite.Limitations == "" {
				fmt.Fprint(&b, "; no documented limitations")
			} else {
				fmt.Fprint(&b, "; documented limitations must be inspected for compatibility")
			}
		}
		fmt.Fprintln(&b, "\n\nIdentity and accepted status were validated structurally. The Task Planner must inspect each referenced capability record and decide whether its documented limitations are compatible with the selected outcome.")
	}
	if request.Command == "next" {
		fmt.Fprintln(&b, "\n## Authoritative next-action evidence")
		fmt.Fprintln(&b, "\n### Roadmap items")
		if len(nextEvidence.RoadmapItems) == 0 {
			fmt.Fprintln(&b, "\n- None recorded.")
		}
		for _, item := range nextEvidence.RoadmapItems {
			fmt.Fprintf(&b, "\n- `%s` — %s; status `%s`; priority `%s`; structurally plannable: `%t`", item.ID, item.Title, item.Status, valueOrNone(item.Priority), item.Eligible)
			fmt.Fprintf(&b, "; dependencies: %s; capability prerequisites: %s", listOrNone(item.Dependencies), listOrNone(item.Prerequisites))
			if item.Blocker != "" {
				fmt.Fprintf(&b, "; blocker: %s", item.Blocker)
			}
			if item.Archive != "" {
				fmt.Fprintf(&b, "; archive provenance: `%s`", item.Archive)
			}
		}
		fmt.Fprintln(&b, "\n\n### Accepted capability truth")
		if len(nextEvidence.Capabilities) == 0 {
			fmt.Fprintln(&b, "\n- None recorded.")
		}
		for _, capability := range nextEvidence.Capabilities {
			fmt.Fprintf(&b, "\n- `%s` — status `%s`; limitations: `%s`; archive provenance: %s", capability.ID, capability.Status, valueOrNone(capability.Limitations), listOrNone(capability.Archives))
		}
		fmt.Fprintln(&b, "\n\n### Supported work origins")
		for _, origin := range nextEvidence.SupportedOrigins {
			fmt.Fprintf(&b, "\n- %s", origin)
		}
		fmt.Fprintln(&b, "\n\nOrdering is deterministic presentation only. The CLI has not selected work; priority and semantic limitation compatibility remain Product Owner judgment.")
	}
	fmt.Fprintln(&b, "\n## Exact inputs to read")
	for _, path := range spec.reads {
		fmt.Fprintf(&b, "\n- `%s`", path)
	}
	fmt.Fprintln(&b, "\n\n## Authorized updates")
	for _, path := range spec.writes {
		fmt.Fprintf(&b, "\n- `%s`", path)
	}
	if request.Command == "plan" && request.GitTaskBranch != "" {
		fmt.Fprintln(&b, "\n\nThe command created and checked out the recorded task branch after validating the clean source trunk and base. Persist these exact values in task-plan Git metadata. It made no workflow-artifact updates.")
	} else {
		fmt.Fprintf(&b, "\n\nThe `concoct %s` command itself made none of these updates. Do not modify any completed review, archive artifact, or capability ledger unless explicitly listed above.\n", request.Command)
	}
	fmt.Fprintf(&b, "\n## Expected outcome\n\n%s\n\n## Validation and completion\n\nValidate repository assumptions before writing, stay within the selected persona's ownership, run relevant documented checks, preserve durable decisions and results, and provide a complete outgoing handoff. Rendered output is guidance only and does not establish completed role work or change workflow state.\n", spec.outcome)
	fmt.Fprintf(&b, "\n## Recommended next transition\n\n%s\n\n## Canonical handoff instructions\n\n", spec.next)
	b.Write(bytes.TrimSpace(source))
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func selectRole(root string, request Request, c workflow.PromptContext) (roleSpec, error) {
	base := []string{"AGENTS.md", ".concoct/capabilities.md"}
	switch request.Command {
	case "next":
		if c.Report.State != workflow.Ready {
			return roleSpec{}, wrongState(request.Command, c.Report.State, "ready")
		}
		return roleSpec{"product-owner", ".concoct/prompts/roadmap/next-action-recommendation.md", "next-action-recommendation", "Recommend exactly one supported next action from authoritative evidence without selecting work or mutating workflow artifacts.", "one exact follow-up command when applicable: `concoct plan <roadmap-id>` or `concoct roadmap`; otherwise name the blocker or report no actionable recorded work", append(base, ".concoct/personas/product-owner.md", ".concoct/roadmap.md"), []string{"none (read-only recommendation)"}}, nil
	case "roadmap":
		if c.Report.State != workflow.Ready {
			return roleSpec{}, wrongState(request.Command, c.Report.State, "ready")
		}
		return roleSpec{"product-owner", ".concoct/prompts/roadmap/human-roadmap-input.md", "roadmap-intake", "Evaluate product input and update only the roadmap when sufficiently understood; do not create an active task.", "`concoct plan <roadmap-id>` when ready, otherwise `concoct roadmap`", append(base, ".concoct/personas/product-owner.md", ".concoct/roadmap.md"), []string{".concoct/roadmap.md"}}, nil
	case "plan":
		if c.Report.State != workflow.Ready {
			return roleSpec{}, wrongState(request.Command, c.Report.State, "ready")
		}
		if err := workflow.ValidatePlanItem(root, request.RoadmapID); err != nil {
			return roleSpec{}, err
		}
		return roleSpec{"task-planner", ".concoct/prompts/handoffs/product-owner-to-task-planner.md", "task-planning", "Create an implementation-ready task plan and durable notes for the selected eligible roadmap item, without implementing code.", "`concoct code`", append(base, ".concoct/personas/task-planner.md", ".concoct/roadmap.md"), []string{".concoct/current/task-plan.md", ".concoct/current/notes.md", ".concoct/roadmap.md (selected item status only after both active artifacts validate)"}}, nil
	case "code":
		mode, source := "implementation", ".concoct/prompts/handoffs/task-planner-to-developer.md"
		if c.Report.State == workflow.ChangesRequested {
			mode, source = "review-remediation", ".concoct/prompts/handoffs/reviewer-to-developer.md"
		} else if c.Report.State == workflow.InProgress {
			mode = "implementation-continuation"
			if c.ResolutionRoute == "code" {
				mode = "blocked-review-recovery-to-code"
			} else if c.RemediatesReview != "" {
				mode, source = "review-remediation", ".concoct/prompts/handoffs/reviewer-to-developer.md"
			}
		} else if c.Report.State != workflow.Planned {
			return roleSpec{}, wrongState(request.Command, c.Report.State, "planned, implementation-in-progress, or review-changes-requested")
		}
		reads := append(base, ".concoct/personas/developer.md", ".concoct/current/task-plan.md", ".concoct/current/notes.md")
		reads = append(reads, c.ReviewFiles...)
		return roleSpec{"developer", source, mode, "Implement or remediate the active task, record verification and decisions, set honest task status, and leave a fresh reviewer handoff. Completed reviews are append-only.", "`concoct review` after completion, or `concoct code` while work remains in progress", reads, []string{"task-scoped source, tests, and documentation", ".concoct/current/task-plan.md", ".concoct/current/notes.md"}}, nil
	case "review":
		if c.Report.State != workflow.Complete {
			return roleSpec{}, wrongState(request.Command, c.Report.State, "implementation-complete")
		}
		mode := "independent-review"
		if c.ResolutionRoute == "review" {
			mode = "blocked-review-recovery-to-review"
		} else if len(c.ReviewFiles) > 0 {
			mode = "post-remediation-review"
		}
		reads := append(base, ".concoct/personas/reviewer.md", ".concoct/current/task-plan.md", ".concoct/current/notes.md", "complete Git diff and relevant source, tests, and documentation")
		reads = append(reads, c.ReviewFiles...)
		return roleSpec{"reviewer", ".concoct/prompts/handoffs/developer-to-reviewer.md", mode, "Independently assess the implementation and create exactly the named next sequential review with one outcome: approved, changes-requested, or blocked. Do not implement fixes.", "approved → `concoct archive`; changes requested → `concoct code`; blocked → responsible role or human", reads, []string{c.NextReview}}, nil
	case "archive":
		if c.Report.State != workflow.Approved {
			return roleSpec{}, wrongState(request.Command, c.Report.State, "review-approved")
		}
		reads := append(base, ".concoct/personas/archivist.md", ".concoct/roadmap.md", ".concoct/current/task-plan.md", ".concoct/current/notes.md", "complete implementation and Git evidence")
		reads = append(reads, c.ReviewFiles...)
		return roleSpec{"archivist", ".concoct/prompts/handoffs/reviewer-to-archivist.md", "archival", "Archive approved evidence and reconcile capability and pending roadmap evidence. For a Git-backed task, commit the archive on the task branch, record git.archive-commit and git.status archived, and do not clear current state or mark delivery.", "Git-backed task → `concoct integrate`; non-Git task → `concoct next`", reads, []string{".concoct/archive/<dated-task>/", ".concoct/capabilities.md", ".concoct/roadmap.md (pending delivery evidence only)", ".concoct/current/task-plan.md", ".concoct/current/notes.md"}}, nil
	default:
		return roleSpec{}, fmt.Errorf("unsupported prompt command %q", request.Command)
	}
}

func valueOrNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return strings.TrimSpace(value)
}
func listOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func wrongState(command string, got workflow.State, want string) error {
	return fmt.Errorf("concoct %s is not valid in state %s; expected %s; run concoct status", command, got, want)
}

func archiveInputs(root, id string, reads []string) ([]string, error) {
	data := id + " " + strings.Join(reads, " ")
	if task, err := os.ReadFile(filepath.Join(root, ".concoct", "current", "task-plan.md")); err == nil {
		data += " " + string(task)
	}
	ids := regexp.MustCompile(`[A-Z][A-Z0-9-]*-[0-9]+`).FindAllString(data, -1)
	wanted := map[string]bool{}
	for _, value := range ids {
		wanted[value] = true
	}
	paths, err := filepath.Glob(filepath.Join(root, ".concoct", "archive", "*", "summary.md"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}
		if id == "" && len(wanted) == 0 {
			out = append(out, filepath.ToSlash(rel))
			continue
		}
		for value := range wanted {
			if strings.Contains(filepath.ToSlash(rel), value) {
				out = append(out, filepath.ToSlash(rel))
				break
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func uniqueSorted(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
