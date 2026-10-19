package selection

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/review"
	"github.com/gopher-launch/concoct/internal/turnstop"
)

type threadResumer interface {
	ResumeThread(context.Context, string, bool, codexapp.RoleSettings) (codexapp.Thread, error)
}

// correctionLoop routes completed findings to the bound reagent and returns
// every revised candidate to the same reviewer thread. A hold retains all
// source and evidence for an explicit continuation.
func correctionLoop(ctx context.Context, cfg Config, owner *activework.Owner, store *activework.Store, reviewer review.Agent, result Result) (Result, error) {
	for result.Status == "changes_requested" {
		prior := result.Snapshot
		if prior.Review == nil || prior.Candidate == nil || prior.Dispatch == nil {
			return result, fmt.Errorf("completed review binding is missing")
		}
		if err := review.VerifyFrozen(store, prior); err != nil {
			return correctionHold(owner, store, result, "recovery-required", "reviewed candidate changed before correction: "+err.Error())
		}
		if prior.Review.Number >= prior.Dispatch.Limits.Assessments {
			return correctionHold(owner, store, result, "limit-exhausted", fmt.Sprintf("independent assessment allowance exhausted (%d/%d); extend assessments to %d to continue %s at %s", prior.Review.Number, prior.Dispatch.Limits.Assessments, prior.Review.Number+1, prior.TaskID, prior.Candidate.Commit))
		}
		resumer, ok := cfg.Agent.(threadResumer)
		if !ok || prior.RoleIDs["reagent"] == "" {
			return correctionHold(owner, store, result, "recovery-required", "bound reagent thread cannot be resumed")
		}
		findings, err := owner.ReviewEvidence(prior.Review.Number)
		if err != nil {
			return result, err
		}
		task, err := owner.TaskEvidence()
		if err != nil {
			return result, err
		}
		if err := owner.BeginCorrection(); err != nil {
			return result, err
		}
		attemptID, err := randomID()
		if err != nil {
			return result, err
		}
		if prior.Dispatch.Pending != nil {
			if prior.Dispatch.Pending.Binding.Activity != dispatch.Correction || prior.Dispatch.Pending.Binding.Candidate != prior.Candidate.Commit || !prior.Dispatch.Pending.Authorized {
				return correctionHold(owner, store, result, "dispatch-authorization", "authorize existing bound correction dispatch")
			}
			attemptID = prior.Dispatch.Pending.ID
		}
		decision, err := owner.ReserveDispatch(attemptID, dispatch.Binding{TaskID: prior.TaskID, Role: dispatch.Reagent, Activity: dispatch.Correction, Candidate: prior.Candidate.Commit}, time.Now().UTC())
		if err != nil {
			if !strings.Contains(err.Error(), "allowance exhausted") {
				return correctionHold(owner, store, result, "recovery-required", "correction dispatch requires reconciliation: "+err.Error())
			}
			return correctionHold(owner, store, result, "limit-exhausted", fmt.Sprintf("%v; extend dispatch allowance to continue %s at %s", err, prior.TaskID, prior.Candidate.Commit))
		}
		if !decision.Dispatch {
			return correctionHold(owner, store, result, "dispatch-authorization", fmt.Sprintf("authorize bound correction dispatch %s for %s at %s", attemptID, prior.TaskID, prior.Candidate.Commit))
		}
		settings := prior.Dispatch.Profiles[dispatch.Reagent]
		if err := cfg.Agent.Idle(ctx, prior.RoleIDs["reagent"]); err != nil {
			return correctionUncertain(owner, store, result, attemptID, err)
		}
		thread, err := resumer.ResumeThread(ctx, prior.RoleIDs["reagent"], true, settings)
		if err != nil || thread.ID != prior.RoleIDs["reagent"] {
			return correctionUncertain(owner, store, result, attemptID, fmt.Errorf("reagent thread resume changed identity: %v", err))
		}
		if err := owner.BindDispatchThread(attemptID, thread.ID); err != nil {
			return result, err
		}
		project, err := readProject(store.Checkout)
		if err != nil {
			return correctionUncertain(owner, store, result, attemptID, err)
		}
		workspace := filepath.Join(store.Root, "workspaces", prior.RunID)
		session := &hostSession{owner: owner, store: store, project: project, workspace: workspace, parent: parentBinding{Branch: prior.ParentBranch, Commit: prior.ParentCommit}, threadID: thread.ID, taskID: prior.TaskID, taskBranch: prior.TaskBranch}
		if err := owner.MarkTurnRequested(attemptID); err != nil {
			return result, err
		}
		prompt := fmt.Sprintf("You are the bound reagent continuing %s on the existing task branch in %s. Address the completed independent assessment below within the agreed task scope. Preserve the parent baseline %s and review findings. Correct source, validate affected behavior, self-review within the recorded limit, reconcile product documentation for the intended integrated state, and save a new ready checkpoint with save_checkpoint. For a genuine blocker return decision_required with a question object containing objective, evidence, missing, safe_work, next_step and optional stable-ID options/tradeoffs. Return ready_for_review only after a completed correction. Do not integrate, push, publish or start another task.\n\nPrior candidate: %s\nPrior assessment:\n%s\nActive task:\n%s", prior.TaskID, workspace, prior.ParentCommit, prior.Candidate.Commit, findings, task)
		turnCtx, cancelTurn, stopEvent := turnstop.Context(ctx, prior.Dispatch.WatchdogElapsed, prior.Dispatch.WatchdogStall, cfg.StopTrigger)
		var observeErr error
		outcome, turnErr := cfg.Agent.RunTurn(turnCtx, thread.ID, prompt, settings, session.handle, func(event codexapp.Event) {
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
		if turnErr != nil || outcome.Status != "completed" {
			finish = "uncertain"
		}
		if err := owner.FinishDispatch(attemptID, finish, dispatch.Receipt(outcome.Usage)); err != nil {
			return result, err
		}
		if turnCtx.Err() != nil && turnErr != nil {
			result.Status, err = turnstop.Handle(owner, cfg.Agent, "reagent", thread.ID, outcome.TurnID, attemptID)
			result.Snapshot, _ = store.Status()
			cancelTurn()
			return result, err
		}
		cancelTurn()
		if turnErr != nil || outcome.Status != "completed" || outcome.ThreadID != thread.ID || outcome.TurnID == "" {
			return correctionHold(owner, store, result, "recovery-required", "correction turn interrupted or has invalid identity; reconcile recorded attempt "+attemptID)
		}
		verdict, err := codexapp.ParseRoleResult(outcome.FinalText)
		if err != nil {
			return correctionHold(owner, store, result, "recovery-required", "correction returned invalid structured result; inspect attempt "+attemptID)
		}
		if verdict.Disposition == "decision_required" || verdict.Disposition == "blocker" {
			if verdict.Question != nil {
				if _, err := owner.AskRoleQuestion("reagent", attemptID, verdict.Question); err != nil {
					return result, err
				}
				result.Status = "decision_required"
				result.Snapshot, err = store.Status()
				return result, err
			}
			return correctionHold(owner, store, result, "operator-decision", verdict.Summary)
		}
		if verdict.Disposition != "ready_for_review" || !session.checkpointSaved {
			return correctionHold(owner, store, result, "recovery-required", "correction did not provide a ready checkpoint; inspect attempt "+attemptID)
		}
		candidate, err := freezeReady(ctx, cfg, owner, store, session, project, session.parent, outcome)
		if err != nil {
			return correctionHold(owner, store, result, "recovery-required", "corrected candidate could not be frozen: "+err.Error())
		}
		if candidate.Commit == prior.Candidate.Commit {
			return correctionHold(owner, store, result, "recovery-required", "corrected candidate did not change")
		}
		if err := owner.MarkDispatchApplied(attemptID); err != nil {
			return result, err
		}
		assessment, err := review.Run(ctx, review.Config{Agent: reviewer, OnEvent: cfg.OnEvent, StopTrigger: cfg.StopTrigger}, owner, store)
		result.Status, result.Snapshot = assessment.Status, assessment.Snapshot
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func correctionUncertain(owner *activework.Owner, store *activework.Store, result Result, attemptID string, cause error) (Result, error) {
	if err := owner.FinishDispatch(attemptID, "uncertain", nil); err != nil {
		return result, err
	}
	hold, err := correctionHold(owner, store, result, "recovery-required", "correction dispatch "+attemptID+" needs reconciliation: "+cause.Error())
	return hold, err
}

func correctionHold(owner *activework.Owner, store *activework.Store, result Result, hold, action string) (Result, error) {
	snapshot, err := store.Status()
	if err != nil {
		return result, err
	}
	if len(action) > 256 {
		action = action[:256]
	}
	phase := snapshot.Phase
	if phase != "changes-requested" {
		phase = "correcting"
	}
	if err := owner.UpdateOperational(activework.Operational{Phase: phase, Hold: hold, PendingRequest: strings.TrimSpace(action), RoleIDs: snapshot.RoleIDs, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters}); err != nil {
		return result, err
	}
	result.Status = "decision_required"
	if hold == "recovery-required" {
		result.Status = "recovery_required"
	}
	result.Snapshot, err = store.Status()
	return result, err
}
