package selection

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/review"
	"github.com/gopher-launch/concoct/internal/roadmap"
	"github.com/gopher-launch/concoct/internal/turnstop"
)

// Config contains explicit controller inputs; it never inherits an ambient
// agent thread or inferred dispatch profile.
type Config struct {
	Repository  string
	Root        string
	Policy      dispatch.State
	Agent       Agent
	Reviewer    review.Agent
	OnEvent     func(codexapp.Event)
	StopTrigger func(codexapp.Event) bool
	// GeneratedExclusions are explicit paths or patterns for build outputs.
	GeneratedExclusions []string
	RequiredChecks      []RequiredCheck
}

type Result struct {
	Status, RunID, Workspace, TaskID, Summary string
	Snapshot                                  activework.Snapshot
}

// Execute starts one fresh reagent selection and lets the same turn continue
// engineering after a host-acknowledged claim.
func Execute(ctx context.Context, cfg Config) (Result, error) {
	var result Result
	store, err := activework.Open(cfg.Repository, cfg.Root)
	if err != nil {
		return result, err
	}
	snapshot, err := store.Status()
	if err != nil {
		return result, err
	}
	if snapshot.Active || snapshot.PendingStart {
		return Result{Status: "active", RunID: snapshot.RunID, Snapshot: snapshot}, nil
	}
	owner, err := store.Acquire()
	if err != nil {
		return result, err
	}
	defer owner.Close()
	snapshot, err = store.Status()
	if err != nil {
		return result, err
	}
	if snapshot.Active || snapshot.PendingStart {
		return Result{Status: "active", RunID: snapshot.RunID, Snapshot: snapshot}, nil
	}
	parent, err := inspectParent(store.Checkout)
	if err != nil {
		return result, err
	}
	branchList, err := gitOutput(store.Checkout, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return result, err
	}
	existingBranches := make(map[string]bool)
	for _, branch := range strings.Split(branchList, "\n") {
		existingBranches[branch] = true
	}
	if cfg.Agent == nil {
		return result, fmt.Errorf("reagent agent is required")
	}
	if err := cfg.Policy.Validate(); err != nil {
		return result, err
	}
	project, err := readProject(store.Checkout)
	if err != nil {
		return result, err
	}
	deferred, err := store.UnavailableTaskIDs()
	if err != nil {
		return result, err
	}
	runID, err := owner.BeginSelection(activework.SelectionSpec{ParentBranch: parent.Branch, ParentCommit: parent.Commit})
	if err != nil {
		return result, err
	}
	result.RunID = runID
	if err := owner.ConfigureDispatch(cfg.Policy); err != nil {
		return result, err
	}
	workspace, err := prepareWorkspace(store, runID, parent.Commit)
	if err != nil {
		return result, err
	}
	result.Workspace = workspace
	settings := cfg.Policy.Profiles[dispatch.Reagent]
	attemptID, err := randomID()
	if err != nil {
		return result, err
	}
	decision, err := owner.ReserveDispatch(attemptID, dispatch.Binding{Role: dispatch.Reagent, Activity: dispatch.Selection}, time.Now().UTC())
	if err != nil {
		return result, err
	}
	if !decision.Dispatch {
		result.Status = "authorization_required"
		result.Snapshot, _ = store.Status()
		return result, nil
	}
	thread, err := cfg.Agent.StartThread(ctx, workspace, settings, toolSpecs())
	if err != nil {
		return failedSelectionLaunch(owner, store, result, attemptID, err)
	}
	if thread.ID == "" {
		return failedSelectionLaunch(owner, store, result, attemptID, fmt.Errorf("reagent thread has no identity"))
	}
	if err := owner.UpdateOperational(activework.Operational{Phase: "selecting", RoleIDs: map[string]string{"reagent": thread.ID}, RoleSettings: map[string]string{"reagent-model": settings.Model, "reagent-effort": settings.Effort}}); err != nil {
		return result, err
	}
	if err := owner.BindDispatchThread(attemptID, thread.ID); err != nil {
		return result, err
	}
	session := &hostSession{owner: owner, store: store, project: project, workspace: workspace, parent: parent, existingBranches: existingBranches, threadID: thread.ID}
	if err := owner.MarkTurnRequested(attemptID); err != nil {
		return result, err
	}
	turnCtx, cancelTurn, stopEvent, finishTurn := turnstop.Context(ctx, cfg.Policy.WatchdogElapsed, cfg.Policy.WatchdogStall, cfg.StopTrigger)
	defer cancelTurn()
	var observeErr error
	outcome, turnErr := cfg.Agent.RunTurn(turnCtx, thread.ID, selectionPrompt(project, parent, workspace, cfg.RequiredChecks, cfg.GeneratedExclusions, deferred), settings, session.handle, func(event codexapp.Event) {
		stopEvent(event)
		if event.Kind == "turn/started" && event.TurnID != "" && observeErr == nil {
			observeErr = owner.ObserveDispatchTurn(attemptID, event.TurnID)
			if observeErr != nil {
				cancelTurn()
			}
		}
		if cfg.OnEvent != nil {
			cfg.OnEvent(event)
		}
	})
	stopReason := finishTurn()
	if outcome.TurnID != "" && observeErr == nil {
		observeErr = owner.ObserveDispatchTurn(attemptID, outcome.TurnID)
	}
	if outcome.TurnID != "" && observeErr == nil {
		observeErr = owner.RecordDispatchCallbacks(attemptID, thread.ID, outcome.TurnID, outcome.Callbacks)
	}
	if outcome.TurnID != "" && outcome.Status != "" && observeErr == nil {
		if outcome.ThreadID != "" && outcome.ThreadID != thread.ID {
			observeErr = fmt.Errorf("provider outcome thread differs from reagent binding")
		} else {
			observeErr = owner.RecordDispatchOutcome(attemptID, thread.ID, outcome.TurnID, outcome.Status, outcome.FinalText)
		}
	}
	if observeErr != nil && turnErr == nil {
		turnErr = observeErr
	}
	finish := "completed"
	if stopReason != nil || turnErr != nil || outcome.Status != "completed" {
		finish = "uncertain"
	}
	tokens := dispatch.Receipt(outcome.Usage)
	if err := owner.FinishDispatch(attemptID, finish, tokens); err != nil {
		return result, err
	}
	if stopReason != nil {
		result.Status, err = turnstop.Handle(owner, cfg.Agent, "reagent", thread.ID, outcome.TurnID, attemptID, stopReason)
		result.Snapshot, _ = store.Status()
		return result, err
	}
	if turnErr != nil {
		if holdErr := retainRecovery(owner, store, &result, "uncertain-turn", attemptID, turnErr); holdErr != nil {
			return result, fmt.Errorf("%w; retain recovery: %v", turnErr, holdErr)
		}
		return result, turnErr
	}
	if outcome.Status != "completed" {
		if holdErr := retainRecovery(owner, store, &result, "incomplete-turn", attemptID, fmt.Errorf("provider turn ended %s: %s", outcome.Status, outcome.ProviderError)); holdErr != nil {
			return result, holdErr
		}
		return result, fmt.Errorf("reagent turn ended %s: %s", outcome.Status, outcome.ProviderError)
	}
	verdict, err := codexapp.ParseRoleResult(outcome.FinalText)
	if err != nil {
		if holdErr := retainRecovery(owner, store, &result, "invalid-role-result", attemptID, err); holdErr != nil {
			return result, fmt.Errorf("%w; retain recovery: %v", err, holdErr)
		}
		return result, err
	}
	result.Summary = verdict.Summary
	result.TaskID = session.taskID
	if verdict.TaskID != "" && verdict.TaskID != session.taskID {
		return invalidCompletedResult(owner, store, &result, "invalid-task-binding", attemptID, fmt.Errorf("reagent result task %q differs from claimed task %q", verdict.TaskID, session.taskID))
	}
	if session.taskID == "" {
		switch verdict.Disposition {
		case "no_suitable_work":
			if err := verifyParentUnchanged(store, parent); err != nil {
				return result, err
			}
			if err := removeCleanWorkspace(store, workspace); err != nil {
				return result, err
			}
			if outcome.TurnID != "" {
				if err := owner.MarkDispatchApplied(attemptID); err != nil {
					return result, err
				}
			}
			if err := owner.EndUnclaimedSelection(verdict.Summary); err != nil {
				return result, err
			}
			result.Status = verdict.Disposition
		case "decision_required":
			if verdict.Question != nil {
				if _, err := owner.AskRoleQuestion("reagent", attemptID, verdict.Question); err != nil {
					return result, err
				}
			} else {
				if err := owner.UpdateOperational(activework.Operational{Phase: "selecting", Hold: "operator-decision", PendingRequest: verdict.Summary, RoleIDs: map[string]string{"reagent": thread.ID}}); err != nil {
					return result, err
				}
			}
			result.Status = verdict.Disposition
		default:
			return result, fmt.Errorf("unclaimed selection returned %q", verdict.Disposition)
		}
	} else {
		if verdict.Disposition != "engineering" && verdict.Disposition != "decision_required" && verdict.Disposition != "ready_for_review" {
			return invalidCompletedResult(owner, store, &result, "invalid-disposition", attemptID, fmt.Errorf("claimed task returned %q", verdict.Disposition))
		}
		if !session.checkpointSaved {
			return invalidCompletedResult(owner, store, &result, "missing-checkpoint", attemptID, fmt.Errorf("claimed task returned without an engineering checkpoint"))
		}
		if err := verifyParentUnchanged(store, parent); err != nil {
			return result, err
		}
		if verdict.Disposition == "ready_for_review" {
			if _, err := freezeReady(ctx, cfg, owner, store, session, parent, outcome); err != nil {
				return invalidCompletedResult(owner, store, &result, "invalid-ready-candidate", attemptID, fmt.Errorf("candidate not ready: %w", err))
			}
			if outcome.TurnID != "" {
				if err := owner.MarkDispatchApplied(attemptID); err != nil {
					return result, err
				}
			}
			result.Status = verdict.Disposition
			result.Snapshot, err = store.Status()
			reviewer := cfg.Reviewer
			if reviewer == nil {
				switch agent := cfg.Agent.(type) {
				case CodexAgent:
					reviewer = agent
				case *CodexAgent:
					reviewer = agent
				}
			}
			if err == nil && reviewer != nil {
				assessment, reviewErr := review.Run(ctx, review.Config{Agent: reviewer, OnEvent: cfg.OnEvent, StopTrigger: cfg.StopTrigger}, owner, store)
				result.Status = assessment.Status
				if assessment.Snapshot.Active {
					result.Snapshot = assessment.Snapshot
				} else {
					result.Snapshot, _ = store.Status()
				}
				if reviewErr == nil && result.Status == "changes_requested" {
					return correctionLoop(ctx, cfg, owner, store, reviewer, result)
				}
				return result, reviewErr
			}
			return result, err
		}
		branch, err := gitOutput(workspace, "symbolic-ref", "--quiet", "--short", "HEAD")
		if err != nil || branch != session.taskBranch {
			return result, fmt.Errorf("claimed task checkout left branch %s", session.taskBranch)
		}
		if verdict.Disposition == "decision_required" {
			if verdict.Question != nil {
				if _, err := owner.AskRoleQuestion("reagent", attemptID, verdict.Question); err != nil {
					return result, err
				}
			} else {
				if err := owner.UpdateOperational(activework.Operational{Phase: "engineering", Hold: "operator-decision", PendingRequest: verdict.Summary, RoleIDs: map[string]string{"reagent": thread.ID}}); err != nil {
					return result, err
				}
			}
		}
		result.Status = verdict.Disposition
	}
	if result.Status != "no_suitable_work" && outcome.TurnID != "" && outcome.Status == "completed" {
		if err := owner.MarkDispatchApplied(attemptID); err != nil {
			return result, err
		}
	}
	result.Snapshot, err = store.Status()
	return result, err
}

func failedSelectionLaunch(owner *activework.Owner, store *activework.Store, result Result, attemptID string, cause error) (Result, error) {
	if err := owner.FinishDispatch(attemptID, "failed", nil); err != nil {
		return result, fmt.Errorf("reagent launch failed: %w; record attempt: %v", cause, err)
	}
	if err := retainRecovery(owner, store, &result, "reagent-launch", attemptID, cause); err != nil {
		return result, fmt.Errorf("reagent launch failed: %w; retain work: %v", cause, err)
	}
	return result, fmt.Errorf("reagent launch failed: %w", cause)
}

func invalidCompletedResult(owner *activework.Owner, store *activework.Store, result *Result, reason, attemptID string, cause error) (Result, error) {
	if err := retainRecovery(owner, store, result, reason, attemptID, cause); err != nil {
		return *result, fmt.Errorf("%w; retain recovery: %v", cause, err)
	}
	return *result, cause
}

func retainRecovery(owner *activework.Owner, store *activework.Store, result *Result, reason, attemptID string, cause error) error {
	snapshot, err := store.Status()
	if err != nil {
		return err
	}
	if !snapshot.Active {
		return fmt.Errorf("active run disappeared before recovery hold")
	}
	issue := recoveryIssue(snapshot, reason, attemptID, cause)
	if err := owner.HoldRecovery(issue); err != nil {
		return err
	}
	result.Status = "recovery_required"
	result.TaskID = snapshot.TaskID
	result.Snapshot, err = store.Status()
	return err
}

func recoveryIssue(snapshot activework.Snapshot, stage, attemptID string, cause error) activework.RecoveryIssue {
	issue := activework.RecoveryIssue{AttemptID: attemptID, Stage: stage, Operation: "provider turn", Reason: stage, EvidenceRef: "runs/" + snapshot.RunID + "/state.json dispatch attempt " + attemptID}
	if cause != nil {
		issue.Reason = stage + ": " + cause.Error()
	}
	if snapshot.Dispatch != nil {
		for _, attempt := range snapshot.Dispatch.Attempts {
			if attempt.ID != attemptID {
				continue
			}
			issue.Role = string(attempt.Binding.Role)
			if attempt.TurnID != "" {
				issue.EvidenceRef += "; turn " + attempt.TurnID
			}
			if verdict, err := codexapp.ParseRoleResult(attempt.FinalText); err == nil {
				issue.AgentResult = verdict.Summary
			} else {
				issue.AgentResult = strings.TrimSpace(attempt.FinalText)
			}
			for _, callback := range attempt.Callbacks {
				if callback.Success {
					continue
				}
				issue.Operation = callback.Tool
				issue.EvidenceRef += "; call " + callback.CallID + "; request " + callback.RequestID
				kind := ""
				switch callback.Tool {
				case "claim_task":
					kind = "claim"
				case "save_checkpoint":
					kind = "checkpoint"
				case "save_assessment":
					kind = "assessment"
				}
				if kind != "" {
					hash := sha256.Sum256([]byte(callback.CallID))
					issue.EvidenceRef += "; receipts/" + kind + "-" + hex.EncodeToString(hash[:12]) + ".json"
				}
				if !callback.ReceiptCommitted {
					issue.Stage = "host-receipt"
				} else if !callback.ResponseSent {
					issue.Stage = "response-transport"
				} else {
					issue.Stage = "provider-completion"
				}
				if callback.Failure != "" {
					issue.Reason = callback.Failure
				}
				break
			}
			break
		}
	}
	if len(issue.Reason) > 2048 {
		issue.Reason = issue.Reason[:2048]
	}
	if len(issue.AgentResult) > 512 {
		issue.AgentResult = issue.AgentResult[:512]
	}
	return issue
}

type projectSnapshot struct {
	Agents, Roadmap, Capabilities string
	Document                      *roadmap.Document
}

func readProject(checkout string) (projectSnapshot, error) {
	var project projectSnapshot
	for _, item := range []struct {
		name string
		out  *string
	}{{"AGENTS.md", &project.Agents}, {"roadmap.md", &project.Roadmap}, {"capabilities.md", &project.Capabilities}} {
		data, err := os.ReadFile(filepath.Join(checkout, item.name))
		if err != nil {
			return project, fmt.Errorf("read %s: %w", item.name, err)
		}
		*item.out = string(data)
	}
	doc, err := roadmap.Parse([]byte(project.Roadmap), []byte(project.Capabilities))
	if err != nil {
		return project, err
	}
	project.Document = doc
	return project, nil
}

func selectionPrompt(project projectSnapshot, parent parentBinding, workspace string, required []RequiredCheck, exclusions []string, excluded ...map[string]bool) string {
	var ready strings.Builder
	for _, entry := range project.Document.EligibleTasks() {
		if len(excluded) > 0 && excluded[0][entry.ID] {
			continue
		}
		fmt.Fprintf(&ready, "\n%s | priority %s | %s\n%s\n", entry.ID, entry.Priority, entry.Title, entry.Body)
	}
	var checks strings.Builder
	for _, check := range required {
		fmt.Fprintf(&checks, "\n%s (source: %s)", check.Command, check.Source)
	}
	return fmt.Sprintf("You are reagent. Follow AGENTS.md and current repository sources. Select one suitable ready task, using the roadmap entry by task ID; explain any higher-priority skip and request an operator decision for a material departure. Do not read historical task archives or reuse another task's context. The parent branch is %s at %s. Your task checkout is %s. Before changing source, create one task branch here from the baseline, call claim_task with task_id, task_branch and selection_rationale, and wait for acknowledgment. Continue engineering, testing and self-review in this same turn, with at most three self-review passes per candidate cycle. Save a concise checkpoint after selection, material decisions and meaningful progress. Judge criterion coverage, check sufficiency, findings resolution and documentation yourself. Reconcile roadmap.md and capabilities.md for the intended integrated state. Do not integrate, push or publish. If no suitable task exists, leave the detached checkout clean and return JSON with disposition no_suitable_work and a reason. For a genuine missing decision, return decision_required with a question object containing objective, evidence, missing, safe_work, next_step and optional options. To request independent review, save a final checkpoint containing the work, checks, self-review and next action; if excluding generated output, use a JSON checkpoint with GeneratedOutputs as exact paths and Deliverables for explicitly included ignored files. Return one JSON result with disposition ready_for_review, task_id and summary. Otherwise return engineering with task_id and summary. Do not invent work or expand into siblings.\n\nAGENTS.md:\n%s\n\nroadmap.md:\n%s\n\ncapabilities.md:\n%s\n\nEligible tasks:\n%s\nConfigured checks for reagent judgment:%s\nConfigured generated-output exclusions: %v\nRelevant design is at doc/implementation-design.md in this checkout; read the sections for the selected task. Current source and build configuration define implemented behavior.", parent.Branch, parent.Commit, workspace, project.Agents, project.Roadmap, project.Capabilities, ready.String(), checks.String(), exclusions)
}

func randomID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}
