package handoff

import (
	"fmt"
	"path/filepath"
)

// These templates give engineering judgment to the agents. Concoct routes
// their completed structured results without reading or grading evidence.
func ReagentPrompt(s State, root string) string {
	return fmt.Sprintf(`Act as reagent for one bounded roadmap task in %s. Read AGENTS.md, roadmap.md, capabilities.md, relevant design sections and current source. For new work, inspect repository identity, branch, commit and status. Require a clean parent at %s (%s), then select one ready task whose dependencies are delivered. Create and switch to its task branch in this checkout. If continuing, inspect the existing branch and evidence; do not create another branch. Work only on that task.

Keep a concise active task checkpoint outside the product repository under %s/evidence/<task-id>/task.md. Record repository, branch, base commit, scope, decisions, checks, candidate commit and next action. Use ordinary tools. Implement, validate, self-review (at most three passes per candidate), reconcile roadmap and capabilities for the intended post-integration state, and commit the finished candidate. Leave its branch clean. Do not use historical task archives or the old Concoct workflow. Do not merge, push, publish or start another task. Stream concise prose as you work.

Return one structured result. For readiness use disposition ready_for_review, the selected task_id, exact candidate commit and a brief summary. For no suitable task use no_suitable_work with a concrete summary. For a genuine blocker use decision_required with an explicit question. Set evidence to the task.md path. Null unused fields. The result is routing data; maintain engineering detail in task.md.
`, s.Repository, s.Parent, s.Base, root)
}

func IragentPrompt(s State, root string) string {
	reviewEvidence := filepath.Join(root, "evidence", s.TaskID, "review-01.md")
	return fmt.Sprintf(`Act as independent iragent for task %s. Review exact candidate commit %s on branch %s in %s against its roadmap scope, acceptance criteria, relevant design, repository instructions, current source and tests. Read the reagent checkpoint at %s. This is the same checkout used by reagent. Do not edit source, documentation, branch or candidate while reviewing. Independently run appropriate read-only checks and inspect the diff from base %s. State actionable findings with evidence; treat summaries as claims to verify.

Write your assessment outside the product repository to %s. Include task ID, exact candidate commit, verdict, findings, checks and remaining risks. Stream concise progress. Return one structured result with task_id %s, candidate %s, disposition approved or changes_requested, a brief summary, findings and evidence path. If a genuine missing decision blocks review, return decision_required with an explicit question. Null unused fields. Do not integrate, push or publish.
`, s.TaskID, s.Candidate, s.Branch, s.Repository, s.Evidence, s.Base, reviewEvidence, s.TaskID, s.Candidate)
}
