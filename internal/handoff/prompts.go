package handoff

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func evidenceDirectory(root, selectedRoadmap, task string) string {
	return filepath.Join(root, "evidence", "roadmap-"+shaPath(selectedRoadmap), task)
}

func taskEvidencePath(root, selectedRoadmap, task string) string {
	return filepath.Join(evidenceDirectory(root, selectedRoadmap, task), "task.md")
}

func reviewPath(root, selectedRoadmap, task string, number int) string {
	return filepath.Join(evidenceDirectory(root, selectedRoadmap, task), fmt.Sprintf("review-%02d.md", number))
}

func promptForStep(s State, root, step string) (string, error) {
	roadmapMarkdown, err := os.ReadFile(s.Roadmap)
	if err != nil {
		return "", fmt.Errorf("read selected roadmap %q: %w", s.Roadmap, err)
	}
	context := fmt.Sprintf("Selected roadmap: %s\nApplicable capabilities: none found\n", s.Roadmap)
	if s.Capabilities != "" {
		context = fmt.Sprintf("Selected roadmap: %s\nApplicable capabilities: %s\n", s.Roadmap, s.Capabilities)
	}
	context += "Authoritative selected roadmap contents follow between markers.\n<selected-roadmap>\n" + string(roadmapMarkdown) + "\n</selected-roadmap>\n"
	var prompt string
	switch step {
	case "engineering":
		prompt = ReagentPrompt(s, root, context)
	case "correcting":
		prompt = CorrectionPrompt(s, root, context)
	case "reviewing":
		prompt = IragentPrompt(s, root, context)
	case "integrating":
		if s.ActiveRecovery != nil {
			prompt, err = RecoveryIntegrationPrompt(s, context)
		} else {
			prompt = IntegrationPrompt(s, context)
		}
	default:
		return "", fmt.Errorf("unsupported prompt step %q", step)
	}
	if s.Answer != "" && s.Question != nil {
		prompt += fmt.Sprintf("\nOperator answer to your question %q: %q\nContinue the same step using this answer.\n", s.Question.Text, s.Answer)
	}
	if s.ActiveRecovery != nil {
		r := s.ActiveRecovery
		prompt += fmt.Sprintf(`
This is an explicit operator-authorized recovery of a stopped %s turn. Start from the preserved checkout and evidence; do not reset, clean, discard, overwrite or infer a prior result. The pending workflow step is %s. The failed invocation remains consumed. Produce the ordinary structured result for this step before the workflow advances.

Prior diagnostic: %s
Prior failure: %s
Prior thread: %s
Base commit: %s
Candidate commit: %s
Last completed result: %s
Task evidence: %s
Review evidence: %s
Observed changes since the pre-turn checkpoint:
%s
`, r.Role, r.Step, r.Diagnostic, r.PriorError, r.PriorThread, s.Base, s.Candidate, recoveryLastResult(s), s.Evidence, s.ReviewEvidence, r.Changes)
		if r.ArchivedEvidence != "" {
			prompt += fmt.Sprintf("Partial review prose was retained separately at %s. It is not an assessment or verdict. Write the replacement assessment to the ordinary review path named above.\n", r.ArchivedEvidence)
		}
		for _, archive := range r.AdditionalArchives {
			prompt += fmt.Sprintf("Additional partial review prose observed before retry was retained separately at %s. It is not an assessment or verdict.\n", archive)
		}
	}
	return prompt, nil
}

func recoveryLastResult(s State) string {
	if s.LastResult == nil {
		return "none"
	}
	return fmt.Sprintf("%s by %s: %s", s.LastResult.Disposition, s.LastRole, s.LastResult.Summary)
}

// These templates give engineering judgment to the agents. Concoct routes
// their completed structured results without reading or grading evidence.
func ReagentPrompt(s State, root, roadmapContext string) string {
	return fmt.Sprintf(`Act as reagent for one bounded roadmap task in %s. Follow applicable repository and directory-specific instructions, read relevant design sections and current source, and use the selected roadmap and capability document identified below. Treat the supplied selected-roadmap contents as the authoritative backlog for task selection; do not select from another roadmap. For new work, inspect repository identity, branch, commit and status. Require a clean parent at %s (%s), then select one ready task whose dependencies are delivered. Create and switch to its task branch in this checkout. If continuing, inspect the existing branch and evidence; do not create another branch. Work only on that task.

%s

Keep a concise active task checkpoint outside the product repository under %s/<task-id>/task.md. This namespace is bound to the selected roadmap; use it exactly so an identical task ID from another roadmap retains separate evidence. Before editing, retain the selected task's original scope and acceptance criteria there so review can assess them after the roadmap entry is removed. Record the selected roadmap, repository, branch, base commit, decisions, checks, candidate commit and next action. Use ordinary tools. Implement, validate and self-review (at most three passes per candidate). Reconcile the selected roadmap and the applicable capability document identified above for the intended post-integration state: summarize delivered behavior in that capability document when one applies, remove the completed task from the selected roadmap, remove its satisfied dependency references, and promote newly eligible tasks when appropriate. Do not edit another roadmap in its place. Preserve unfinished task IDs and scope. Commit the finished candidate and leave its branch clean. Do not use historical task archives or the old Concoct workflow. Do not merge, push, publish or start another task. Stream concise prose as you work.

Return one structured result. For readiness use disposition ready_for_review, the selected task_id, exact candidate commit and a brief summary. For no suitable task use no_suitable_work with a concrete summary. For a genuine blocker use decision_required with an explicit question. Set evidence to the task.md path. Null unused fields. The result is routing data; maintain engineering detail in task.md.
`, s.Repository, s.Parent, s.Base, roadmapContext, filepath.Join(root, "evidence", "roadmap-"+shaPath(s.Roadmap)))
}

func IragentPrompt(s State, root, roadmapContext string) string {
	reviewEvidence := reviewPath(root, s.Roadmap, s.TaskID, s.Assessments+1)
	return fmt.Sprintf(`Act as independent iragent for task %s in selected roadmap %s. Review exact candidate commit %s on branch %s in %s against the original task scope and acceptance criteria retained in the reagent checkpoint at %s, plus relevant design, applicable repository and directory-specific instructions, current source and tests. The selected candidate roadmap may have removed the completed task. This is the same checkout used by reagent. Do not edit source, documentation, branch or candidate while reviewing. Independently run appropriate read-only checks and inspect the diff from base %s. State actionable findings with evidence; treat summaries as claims to verify.

%s

Write your assessment outside the product repository to %s. Include task ID, exact candidate commit, verdict, findings, checks and remaining risks. Stream concise progress. Return one structured result with task_id %s, candidate %s, disposition approved or changes_requested, a brief summary, findings and evidence path. If a genuine missing decision blocks review, return decision_required with an explicit question. Null unused fields. Do not integrate, push or publish.
`, s.TaskID, s.Roadmap, s.Candidate, s.Branch, s.Repository, s.Evidence, s.Base, roadmapContext, reviewEvidence, s.TaskID, s.Candidate)
}

func CorrectionPrompt(s State, root, roadmapContext string) string {
	return fmt.Sprintf(`Continue as reagent for task %s in selected roadmap %s, in %s on branch %s, based on %s. The independent iragent assessed candidate %s and requested changes. Read the current source, task evidence %s and assessment %s. Follow applicable repository and directory-specific instructions. Address the findings within the task scope; judge their merit yourself and explain any disputed finding with evidence. Findings and summary are agent narrative, not Concoct judgments.

%s

Review summary: %s
Findings:
%s

Update the external task evidence, affected checks and documentation. Self-review at most three passes for this candidate cycle. Commit a distinct corrected candidate on the same clean task branch. Return ready_for_review with task_id %s, exact new candidate commit, evidence path and summary. For a genuine missing decision, return decision_required with a concise question. Do not merge, push, publish or start another task.
`, s.TaskID, s.Roadmap, s.Repository, s.Branch, s.Base, s.Candidate, s.Evidence, s.ReviewEvidence, roadmapContext, s.ReviewSummary, strings.Join(s.Findings, "\n"), s.TaskID)
}

func IntegrationPrompt(s State, roadmapContext string) string {
	return fmt.Sprintf(`Continue as reagent for task %s in selected roadmap %s. The operator explicitly approved integration of exact independently approved commit %s after assessment %s. Read the current source, task evidence %s and review evidence %s. Follow applicable repository and directory-specific instructions. Work in %s. Check the task branch %s is clean at that commit, the approved commit and review match, and parent branch %s still points to base %s. If any check fails, stop and report integration_failed with the reason; do not substitute another commit or invent a merge.

%s

Perform local fast-forward integration of that exact commit onto %s using an expected-old-ref guard, then switch to the parent checkout and verify it is clean at the approved commit. Record the action in external task evidence. Do not push or publish. Return integrated with task_id %s, candidate %s, evidence path and a clear summary only after completing those checks. If a genuine missing operator decision blocks safe progress, return decision_required with a concise question. Concoct will report your result without performing its own Git transaction.
`, s.TaskID, s.Roadmap, s.AuthorizedCommit, s.ReviewEvidence, s.Evidence, s.ReviewEvidence, s.Repository, s.Branch, s.Parent, s.Base, roadmapContext, s.Parent, s.TaskID, s.AuthorizedCommit)
}

func RecoveryIntegrationPrompt(s State, roadmapContext string) (string, error) {
	if s.ActiveRecovery == nil || s.ActiveRecovery.Step != "integrating" {
		return "", fmt.Errorf("integration recovery requires an active integrating recovery")
	}
	intro := fmt.Sprintf(`Continue as reagent for task %s in selected roadmap %s. The operator explicitly requested recovery of an interrupted integration for exact independently approved commit %s after assessment %s. Read the current source, task evidence %s and review evidence %s. Follow applicable repository and directory-specific instructions. Work in %s. The host has reconciled task branch %s to the authorized commit and classified the parent ref before dispatch. Independently recheck every binding before any Git action; never infer success from unfinished prose.

%s
`, s.TaskID, s.Roadmap, s.AuthorizedCommit, s.ReviewEvidence, s.Evidence, s.ReviewEvidence, s.Repository, s.Branch, roadmapContext)
	var action string
	switch s.ActiveRecovery.IntegrationParent {
	case "base":
		action = fmt.Sprintf(`The parent branch %s was observed at recorded base %s. Require it still to equal that base, require task branch %s still to equal authorized commit %s, and require the task checkout to be clean. If any binding fails, stop and report integration_failed; do not substitute another commit or invent a merge. Otherwise perform the local fast-forward of that exact commit using an expected-old-ref guard, switch to the parent checkout and verify it is clean at the authorized commit.`, s.Parent, s.Base, s.Branch, s.AuthorizedCommit)
	case "authorized":
		action = fmt.Sprintf(`The parent branch %s was already observed at authorized commit %s. Do not fast-forward, update or move either branch ref. Require both parent branch %s and task branch %s still to equal that exact commit, then switch to or confirm the parent checkout and verify it is clean at the authorized commit. If any binding fails, stop and report integration_failed.`, s.Parent, s.AuthorizedCommit, s.Parent, s.Branch)
	default:
		return "", fmt.Errorf("integration recovery lacks a reconciled parent classification")
	}
	return intro + "\n" + action + fmt.Sprintf(` Record the verified action in external task evidence. Do not push or publish. Return integrated with task_id %s, candidate %s, evidence path and a clear summary only after completing the applicable checks. If a genuine missing operator decision blocks safe progress, return decision_required with a concise question. Concoct will report your result without performing its own Git transaction.
`, s.TaskID, s.AuthorizedCommit), nil
}
