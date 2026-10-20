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

type turnReader interface {
	ReadTurn(context.Context, string, string) (codexapp.ObservedTurn, error)
}

// Continue inspects one active run under its execution lock. It never starts
// selection when another run is active and never replays an uncertain turn.
func Continue(ctx context.Context, cfg Config) (Result, error) {
	var result Result
	store, err := activework.Open(cfg.Repository, cfg.Root)
	if err != nil {
		return result, err
	}
	owner, err := store.Acquire()
	if err != nil {
		return result, err
	}
	defer owner.Close()
	snapshot, err := store.Status()
	if err != nil {
		return result, err
	}
	if !snapshot.Active {
		return result, fmt.Errorf("no active run to continue")
	}
	result = Result{RunID: snapshot.RunID, TaskID: snapshot.TaskID, Workspace: filepath.Join(store.Root, "workspaces", snapshot.RunID), Snapshot: snapshot}
	if snapshot.Integration != nil {
		result.Status = "integration_pending"
		if snapshot.Integration.Status == "integrated" {
			result.Status = "integrated"
		}
		if snapshot.Completion != nil {
			result.Status = "maintenance_pending"
		}
		return result, nil
	}
	if err := verifyParentUnchanged(store, parentBinding{Branch: snapshot.ParentBranch, Commit: snapshot.ParentCommit}); err != nil {
		return holdContinuation(owner, store, result, "recovery-required", "parent baseline changed: "+err.Error())
	}
	if snapshot.Stop != nil && snapshot.Stop.Status != "stopped_safely" {
		result.Status = "stopped_abnormally"
		return result, nil
	}
	if snapshot.Hold == "operator-decision" {
		result.Status = "decision_required"
		return result, nil
	}
	if snapshot.Hold == "dispatch-authorization" {
		if snapshot.Dispatch == nil || snapshot.Dispatch.Pending == nil || !snapshot.Dispatch.Pending.Authorized {
			result.Status = "decision_required"
			return result, nil
		}
		if err := owner.UpdateOperational(activework.Operational{Phase: snapshot.Phase, RoleIDs: snapshot.RoleIDs, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters}); err != nil {
			return result, err
		}
		snapshot, err = store.Status()
		if err != nil {
			return result, err
		}
		result.Snapshot = snapshot
	}
	if snapshot.Hold == "limit-exhausted" {
		if snapshot.Dispatch == nil || len(snapshot.Dispatch.Attempts) >= snapshot.Dispatch.Limits.Dispatches || len(snapshot.Dispatch.Assessments) >= snapshot.Dispatch.Limits.Assessments {
			result.Status = "decision_required"
			return result, nil
		}
		if err := owner.UpdateOperational(activework.Operational{Phase: snapshot.Phase, RoleIDs: snapshot.RoleIDs, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters}); err != nil {
			return result, err
		}
		snapshot, err = store.Status()
		if err != nil {
			return result, err
		}
		result.Snapshot = snapshot
	}
	if snapshot.Dispatch == nil {
		return holdContinuation(owner, store, result, "recovery-required", "active run has no recorded dispatch policy")
	}
	var pending *dispatch.Attempt
	for i := range snapshot.Dispatch.Attempts {
		a := &snapshot.Dispatch.Attempts[i]
		if a.Status == "reserved" || a.Status == "uncertain" {
			pending = a
			break
		}
	}
	if pending != nil {
		if !pending.TurnRequested {
			if err := owner.FinishDispatch(pending.ID, "failed", nil); err != nil {
				return result, err
			}
			return continueAfterResolved(ctx, cfg, owner, store, result, pending.ID, true)
		}
		if pending.ThreadID == "" || pending.TurnID == "" {
			return holdContinuation(owner, store, result, "recovery-required", "turn/start acknowledgment is uncertain for attempt "+pending.ID+"; inspect provider before another dispatch")
		}
		observed, err := observePending(ctx, cfg, snapshot, *pending)
		if err != nil {
			return holdContinuation(owner, store, result, "recovery-required", "exact provider turn cannot be observed for "+pending.ID+": "+err.Error())
		}
		if observed.Status == "completed" {
			return applyCompleted(ctx, cfg, owner, store, result, *pending, observed)
		}
		if observed.Status == "interrupted" || observed.Status == "failed" {
			if snapshot.Stop == nil || snapshot.Stop.Status != "stopped_safely" || snapshot.Stop.AttemptID != pending.ID {
				return holdContinuation(owner, store, result, "abnormal-stop", "provider turn ended without verified owned-process cessation for "+pending.ID)
			}
			if err := owner.MarkDispatchUncertain(pending.ID); err != nil {
				return result, err
			}
			if err := owner.ReconcileDispatch(pending.ID, "exact provider turn "+observed.Status+"; owned processes verified stopped"); err != nil {
				return result, err
			}
			if err := owner.CompleteStopReconciliation(pending.ID); err != nil {
				return result, err
			}
			return continueAfterResolved(ctx, cfg, owner, store, result, pending.ID, true)
		}
		return holdContinuation(owner, store, result, "recovery-required", "provider turn "+pending.TurnID+" is still active or uncertain; stop it before continuation")
	}
	// A completed dispatch may have reached the provider before the controller
	// finalized its candidate or assessment. Apply its recorded outcome once.
	if len(snapshot.Dispatch.Attempts) > 0 {
		last := snapshot.Dispatch.Attempts[len(snapshot.Dispatch.Attempts)-1]
		if last.Status == "completed" && last.FinalStatus == "completed" && last.TurnID != "" && !last.Applied {
			return applyCompleted(ctx, cfg, owner, store, result, last, codexapp.ObservedTurn{ID: last.TurnID, Status: "completed", FinalText: last.FinalText})
		}
	}
	if snapshot.Question != nil && snapshot.Question.AnswerMode != "" {
		if snapshot.Hold == "recovery-required" {
			ready, err := retryAnsweredFailedLaunch(ctx, cfg, owner, snapshot)
			if err != nil {
				return result, err
			}
			if !ready {
				result.Status = "recovery_required"
				return result, nil
			}
			snapshot, err = store.Status()
			if err != nil {
				return result, err
			}
			result.Snapshot = snapshot
		}
		if snapshot.Hold == "" {
			return routeAnswered(ctx, cfg, owner, store, result, snapshot)
		}
	}
	return continueAfterResolved(ctx, cfg, owner, store, result, "", false)
}

func observePending(ctx context.Context, cfg Config, snapshot activework.Snapshot, attempt dispatch.Attempt) (codexapp.ObservedTurn, error) {
	if snapshot.Stop != nil && snapshot.Stop.Status == "stopped_safely" && snapshot.Stop.AttemptID == attempt.ID && snapshot.Stop.ObservedStatus != "" && snapshot.Stop.ObservedStatus != "completed" {
		return codexapp.ObservedTurn{ID: attempt.TurnID, Status: snapshot.Stop.ObservedStatus, FinalText: snapshot.Stop.ObservedFinalText}, nil
	}
	var agent any = cfg.Agent
	if attempt.Binding.Role == dispatch.Iragent {
		agent = reviewerFor(cfg)
	}
	reader, ok := agent.(turnReader)
	if !ok {
		return codexapp.ObservedTurn{}, fmt.Errorf("bound role cannot inspect provider turns")
	}
	observed, err := reader.ReadTurn(ctx, attempt.ThreadID, attempt.TurnID)
	if err != nil {
		return observed, err
	}
	if observed.ID != attempt.TurnID {
		return observed, fmt.Errorf("observed turn identity changed")
	}
	return observed, nil
}

func reviewerFor(cfg Config) review.Agent {
	if cfg.Reviewer != nil {
		return cfg.Reviewer
	}
	switch agent := cfg.Agent.(type) {
	case CodexAgent:
		return agent
	case *CodexAgent:
		return agent
	}
	return nil
}

func applyCompleted(ctx context.Context, cfg Config, owner *activework.Owner, store *activework.Store, result Result, attempt dispatch.Attempt, observed codexapp.ObservedTurn) (Result, error) {
	snapshot, err := store.Status()
	if err != nil {
		return result, err
	}
	if !observed.ItemsKnown {
		var agent any = cfg.Agent
		if attempt.Binding.Role == dispatch.Iragent {
			agent = reviewerFor(cfg)
		}
		if _, ok := agent.(turnReader); ok {
			observed, err = observePending(ctx, cfg, snapshot, attempt)
			if err != nil {
				return holdContinuation(owner, store, result, "recovery-required", "exact completed turn items cannot be observed: "+err.Error())
			}
			if !observed.ItemsKnown {
				return holdContinuation(owner, store, result, "recovery-required", "exact completed turn item list is unavailable")
			}
		}
	}
	if observed.ID != attempt.TurnID {
		return holdContinuation(owner, store, result, "recovery-required", "provider turn identity differs from recorded attempt")
	}
	if observed.Status != "completed" {
		return holdContinuation(owner, store, result, "recovery-required", "provider turn is not completed: "+observed.Status)
	}
	if strings.TrimSpace(observed.FinalText) == "" {
		return holdContinuation(owner, store, result, "recovery-required", "completed provider turn has no final-result text")
	}
	if attempt.FinalText != "" && attempt.FinalText != observed.FinalText {
		return holdContinuation(owner, store, result, "recovery-required", "provider result differs from recorded outcome")
	}
	if err := verifyObservedCallbacks(owner, attempt, observed); err != nil {
		return holdContinuation(owner, store, result, "recovery-required", "callback evidence for completed turn is insufficient: "+err.Error())
	}
	if snapshot.Hold == "recovery-required" {
		if err := owner.UpdateOperational(activework.Operational{Phase: snapshot.Phase, RoleIDs: snapshot.RoleIDs, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters}); err != nil {
			return result, err
		}
	}
	if attempt.Binding.Role == dispatch.Iragent {
		assessment, err := review.ReconcileCompleted(ctx, reviewerFor(cfg), owner, store, observed)
		if err != nil {
			return holdContinuation(owner, store, result, "recovery-required", "review completion cannot be reconciled: "+err.Error())
		}
		if snapshot.Stop != nil {
			if err := owner.CompleteStopReconciliation(attempt.ID); err != nil {
				return result, err
			}
		}
		result.Status, result.Snapshot = assessment.Status, assessment.Snapshot
		if assessment.Status == "changes_requested" {
			return correctionLoop(ctx, cfg, owner, store, reviewerFor(cfg), result)
		}
		return result, nil
	}
	if err := owner.ReconcileCompletedDispatch(attempt.ID, attempt.ThreadID, attempt.TurnID, "provider completed exact reagent turn"); err != nil {
		return result, err
	}
	verdict, err := codexapp.ParseRoleResult(observed.FinalText)
	if err != nil {
		return holdContinuation(owner, store, result, "recovery-required", "completed reagent result is invalid: "+err.Error())
	}
	if snapshot.Stop != nil {
		if err := owner.CompleteStopReconciliation(attempt.ID); err != nil {
			return result, err
		}
	}
	applied, err := applyReagentVerdict(ctx, cfg, owner, store, result, attempt.ID, attempt.ThreadID, attempt.TurnID, verdict)
	if err == nil && applied.Status != "recovery_required" && applied.Status != "stopped_abnormally" && applied.Status != "no_suitable_work" {
		err = owner.MarkDispatchApplied(attempt.ID)
	}
	return applied, err
}

func verifyObservedCallbacks(owner *activework.Owner, attempt dispatch.Attempt, observed codexapp.ObservedTurn) error {
	items := make(map[string]codexapp.DynamicToolItem)
	verifiedTools := make(map[string]bool)
	for _, item := range observed.DynamicTools {
		if item.ID == "" || item.Tool == "" {
			return fmt.Errorf("dynamic tool item identity is missing")
		}
		if prior := items[item.ID]; prior.ID != "" {
			if prior.Tool != item.Tool || prior.Namespace != item.Namespace || prior.Status != item.Status || (prior.Success == nil) != (item.Success == nil) || (prior.Success != nil && *prior.Success != *item.Success) {
				return fmt.Errorf("conflicting dynamic tool item %s", item.ID)
			}
			continue
		}
		items[item.ID] = item
		kind, err := callbackReceiptKind(attempt.Binding.Role, item.Tool)
		if err != nil {
			return err
		}
		if item.Status != "completed" || item.Success == nil || !*item.Success {
			return fmt.Errorf("%s call %s did not complete successfully", item.Tool, item.ID)
		}
		found, err := owner.VerifiedCallReceipt(kind, item.ID, attempt.TurnID)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("%s call %s lacks durable host receipt", item.Tool, item.ID)
		}
		verifiedTools[item.Tool] = true
	}
	// Codex can omit completed dynamic-tool items from thread/read. A live
	// acknowledgment retained on this attempt is an independent proof path,
	// but it must still match the bound host receipt and any returned item.
	callbacks := make(map[string]codexapp.CallbackEvidence)
	requests := make(map[string]bool)
	for _, callback := range attempt.Callbacks {
		if callback.ThreadID != attempt.ThreadID || callback.TurnID != attempt.TurnID || callback.RequestID == "" || callback.CallID == "" || callback.Tool == "" {
			return fmt.Errorf("callback binding differs from dispatch")
		}
		if _, exists := callbacks[callback.CallID]; exists || requests[callback.RequestID] {
			return fmt.Errorf("conflicting callback call or request %s", callback.CallID)
		}
		callbacks[callback.CallID] = callback
		requests[callback.RequestID] = true
		kind, err := callbackReceiptKind(attempt.Binding.Role, callback.Tool)
		if err != nil {
			return err
		}
		item, ok := items[callback.CallID]
		if ok && (item.Tool != callback.Tool || item.Namespace != callback.Namespace) {
			return fmt.Errorf("callback call %s conflicts with provider item", callback.CallID)
		}
		if ok {
			// A completed historical item and host receipt can verify a call even
			// when the live client missed its completion event.
			if callback.ItemID != "" && callback.ItemID != callback.CallID {
				return fmt.Errorf("callback call %s conflicts with provider item identity", callback.CallID)
			}
			if !callback.ReceiptCommitted || callback.Failure != "" {
				return fmt.Errorf("callback call %s conflicts with completed provider item", callback.CallID)
			}
		} else if callback.ItemID != callback.CallID || !callback.Done || !callback.ReceiptCommitted || !callback.ResponseSent || !callback.ProviderCompleted || !callback.Success || callback.Failure != "" {
			return fmt.Errorf("callback call %s lacks verified live acknowledgment", callback.CallID)
		}
		found, err := owner.VerifiedCallReceipt(kind, callback.CallID, attempt.TurnID)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("callback call %s lacks durable host receipt", callback.CallID)
		}
		verifiedTools[callback.Tool] = true
	}
	if len(callbacks) != 0 {
		for id := range items {
			if _, ok := callbacks[id]; !ok {
				return fmt.Errorf("provider item %s lacks recorded callback", id)
			}
		}
	}
	if observed.ItemsKnown {
		if verdict, err := codexapp.ParseRoleResult(observed.FinalText); err == nil {
			if verdict.Disposition == "ready_for_review" && !verifiedTools["save_checkpoint"] {
				return fmt.Errorf("ready result lacks verified save_checkpoint acknowledgment")
			}
			if verdict.Disposition == "ready_for_review" && attempt.Binding.Activity == dispatch.Selection && !verifiedTools["claim_task"] {
				return fmt.Errorf("selected task lacks verified claim_task acknowledgment")
			}
			if (verdict.Disposition == "approved" || verdict.Disposition == "changes_requested") && !verifiedTools["save_assessment"] {
				return fmt.Errorf("review result lacks verified save_assessment acknowledgment")
			}
		}
	}
	return nil
}

func callbackReceiptKind(role dispatch.Role, tool string) (string, error) {
	switch tool {
	case "claim_task":
		if role == dispatch.Reagent {
			return "claim", nil
		}
	case "save_checkpoint":
		if role == dispatch.Reagent {
			return "checkpoint", nil
		}
	case "save_assessment":
		if role == dispatch.Iragent {
			return "assessment", nil
		}
	}
	return "", fmt.Errorf("host operation %s differs from bound role or is unsupported", tool)
}

func continueAfterResolved(ctx context.Context, cfg Config, owner *activework.Owner, store *activework.Store, result Result, episode string, recovering bool) (Result, error) {
	snapshot, err := store.Status()
	if err != nil {
		return result, err
	}
	result.Snapshot = snapshot
	if snapshot.Hold != "" {
		if snapshot.Hold != "recovery-required" || snapshot.Phase != "reviewing" || snapshot.Review == nil || snapshot.Review.Verdict != "" || snapshot.Dispatch == nil {
			result.Status = "decision_required"
			if snapshot.Hold == "recovery-required" {
				result.Status = "recovery_required"
			}
			return result, nil
		}
		priorResolved := false
		for _, attempt := range snapshot.Dispatch.Attempts {
			if attempt.ID == snapshot.Review.AttemptID && (attempt.Status == "failed" || attempt.Status == "reconciled") {
				priorResolved = true
				episode = attempt.ID
				if attempt.Binding.Activity == dispatch.Recovery {
					episode = attempt.Binding.Episode
				}
			}
		}
		if !priorResolved {
			result.Status = "recovery_required"
			return result, nil
		}
		recovering = true
	}
	switch snapshot.Phase {
	case "ready-for-acceptance":
		result.Status = "approved"
		return result, nil
	case "ready-for-review":
		reviewer := reviewerFor(cfg)
		if reviewer == nil {
			result.Status = "ready_for_review"
			return result, nil
		}
		assessment, err := review.Run(ctx, review.Config{Agent: reviewer, OnEvent: cfg.OnEvent, StopTrigger: cfg.StopTrigger}, owner, store)
		result.Status, result.Snapshot = assessment.Status, assessment.Snapshot
		if err != nil {
			return result, err
		}
		if result.Status == "changes_requested" {
			return correctionLoop(ctx, cfg, owner, store, reviewer, result)
		}
		return result, nil
	case "changes-requested":
		return correctionLoop(ctx, cfg, owner, store, reviewerFor(cfg), Result{Status: "changes_requested", RunID: snapshot.RunID, TaskID: snapshot.TaskID, Workspace: result.Workspace, Snapshot: snapshot})
	case "reviewing":
		if snapshot.Review == nil || snapshot.Review.Verdict != "" {
			return holdContinuation(owner, store, result, "recovery-required", "review binding requires reconciliation")
		}
		if snapshot.Dispatch.Pending != nil && snapshot.Dispatch.Pending.Binding.Activity == dispatch.Review {
			reviewer := reviewerFor(cfg)
			if reviewer == nil {
				return holdContinuation(owner, store, result, "recovery-required", "reviewer provider is unavailable")
			}
			assessment, err := review.Run(ctx, review.Config{Agent: reviewer, OnEvent: cfg.OnEvent, StopTrigger: cfg.StopTrigger}, owner, store)
			result.Status, result.Snapshot = assessment.Status, assessment.Snapshot
			if err != nil {
				return result, err
			}
			if result.Status == "changes_requested" {
				return correctionLoop(ctx, cfg, owner, store, reviewer, result)
			}
			return result, nil
		}
		if !recovering && (snapshot.Dispatch.Pending == nil || snapshot.Dispatch.Pending.Binding.Activity != dispatch.Recovery) {
			return holdContinuation(owner, store, result, "recovery-required", "pending assessment needs exact attempt reconciliation")
		}
		if snapshot.Dispatch.Pending == nil {
			id, err := randomID()
			if err != nil {
				return result, err
			}
			if err := owner.RenewReviewAttempt(id); err != nil {
				return result, err
			}
		} else {
			episode = snapshot.Dispatch.Pending.Binding.Episode
		}
		reviewer := reviewerFor(cfg)
		if reviewer == nil {
			return holdContinuation(owner, store, result, "recovery-required", "reviewer provider is unavailable")
		}
		assessment, err := review.Run(ctx, review.Config{Agent: reviewer, OnEvent: cfg.OnEvent, StopTrigger: cfg.StopTrigger, RecoveryEpisode: episode}, owner, store)
		result.Status, result.Snapshot = assessment.Status, assessment.Snapshot
		if err != nil {
			return result, err
		}
		if result.Status == "changes_requested" {
			return correctionLoop(ctx, cfg, owner, store, reviewer, result)
		}
		return result, nil
	case "correcting":
		if !recovering && snapshot.Review != nil && snapshot.Review.Verdict == "changes_requested" && snapshot.Candidate != nil && snapshot.Review.Candidate == snapshot.Candidate.Commit && (snapshot.Dispatch.Pending == nil || snapshot.Dispatch.Pending.Binding.Activity == dispatch.Correction) {
			return correctionLoop(ctx, cfg, owner, store, reviewerFor(cfg), Result{Status: "changes_requested", RunID: snapshot.RunID, TaskID: snapshot.TaskID, Workspace: result.Workspace, Snapshot: snapshot})
		}
		return runReagentContinuation(ctx, cfg, owner, store, result, episode, recovering)
	case "engineering", "selecting":
		return runReagentContinuation(ctx, cfg, owner, store, result, episode, recovering)
	default:
		return holdContinuation(owner, store, result, "recovery-required", "unknown active phase "+snapshot.Phase)
	}
}

func holdContinuation(owner *activework.Owner, store *activework.Store, result Result, hold, request string) (Result, error) {
	snapshot, err := store.Status()
	if err != nil {
		return result, err
	}
	if len(request) > 256 {
		request = request[:256]
	}
	var holdErr error
	if hold == "recovery-required" {
		attemptID := ""
		if snapshot.Dispatch != nil && len(snapshot.Dispatch.Attempts) != 0 {
			attemptID = snapshot.Dispatch.Attempts[len(snapshot.Dispatch.Attempts)-1].ID
		}
		issue := recoveryIssue(snapshot, "reconciliation", attemptID, fmt.Errorf("%s", request))
		issue.Reason = request
		holdErr = owner.HoldRecovery(issue)
	} else {
		holdErr = owner.UpdateOperational(activework.Operational{Phase: snapshot.Phase, Hold: hold, PendingRequest: request, RoleIDs: snapshot.RoleIDs, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters})
	}
	if holdErr != nil {
		return result, holdErr
	}
	result.Status = "recovery_required"
	if hold == "abnormal-stop" {
		result.Status = "stopped_abnormally"
	} else if hold == "dispatch-authorization" {
		result.Status = "authorization_required"
	}
	result.Snapshot, err = store.Status()
	return result, err
}

// A distinct recovery attempt is reserved before the role is resumed. Its
// prompt contains only the active task, reviews and current source checkout.
func runReagentContinuation(ctx context.Context, cfg Config, owner *activework.Owner, store *activework.Store, result Result, episode string, recovering bool) (Result, error) {
	snapshot, err := store.Status()
	if err != nil {
		return result, err
	}
	if cfg.Agent == nil || snapshot.Dispatch == nil {
		return holdContinuation(owner, store, result, "recovery-required", "recorded reagent provider is unavailable")
	}
	parent := parentBinding{Branch: snapshot.ParentBranch, Commit: snapshot.ParentCommit}
	if err := verifyParentUnchanged(store, parent); err != nil {
		return holdContinuation(owner, store, result, "recovery-required", err.Error())
	}
	project, err := readProject(store.Checkout)
	if err != nil {
		return result, err
	}
	workspace := result.Workspace
	if err := verifyResumeWorkspace(store, snapshot, workspace); err != nil {
		return holdContinuation(owner, store, result, "recovery-required", err.Error())
	}
	activity := dispatch.Engineering
	if snapshot.TaskID == "" {
		activity = dispatch.Selection
	} else if snapshot.Phase == "correcting" && snapshot.Candidate != nil {
		activity = dispatch.Correction
	}
	if recovering {
		activity = dispatch.Recovery
	}
	if snapshot.Dispatch.Pending != nil && snapshot.Dispatch.Pending.Authorized {
		activity = snapshot.Dispatch.Pending.Binding.Activity
		episode = snapshot.Dispatch.Pending.Binding.Episode
		recovering = activity == dispatch.Recovery
	}
	attemptID, err := randomID()
	if err != nil {
		return result, err
	}
	binding := dispatch.Binding{TaskID: snapshot.TaskID, Role: dispatch.Reagent, Activity: activity, Episode: episode}
	if (recovering || activity == dispatch.Correction) && snapshot.Candidate != nil {
		binding.Candidate = snapshot.Candidate.Commit
	}
	if snapshot.Dispatch.Pending != nil {
		if !snapshot.Dispatch.Pending.Authorized || snapshot.Dispatch.Pending.Binding != binding {
			return holdContinuation(owner, store, result, "dispatch-authorization", "pending reagent authorization differs from continuation")
		}
		attemptID = snapshot.Dispatch.Pending.ID
	}
	decision, err := owner.ReserveDispatch(attemptID, binding, time.Now().UTC())
	if err != nil {
		return holdContinuation(owner, store, result, "recovery-required", "recovery allowance or dispatch prerequisite: "+err.Error())
	}
	if !decision.Dispatch {
		return holdContinuation(owner, store, result, "dispatch-authorization", "authorize bound reagent continuation "+attemptID)
	}
	settings := snapshot.Dispatch.Profiles[dispatch.Reagent]
	threadID := snapshot.RoleIDs["reagent"]
	hadTurn := false
	for _, attempt := range snapshot.Dispatch.Attempts {
		if attempt.ThreadID == threadID && attempt.TurnRequested {
			hadTurn = true
		}
	}
	var thread codexapp.Thread
	if threadID != "" {
		resumer, ok := cfg.Agent.(threadResumer)
		if !ok {
			if err := owner.FinishDispatch(attemptID, "failed", nil); err != nil {
				return result, err
			}
			return holdContinuation(owner, store, result, "recovery-required", "reagent thread could not be reconstructed: provider cannot resume bound thread")
		}
		thread, err = resumer.ResumeThread(ctx, threadID, hadTurn, settings)
		if err != nil && (codexapp.IsProvisionalThread(err) && !hadTurn || codexapp.IsMissingRollout(err) && len(snapshot.StopHistory) > 0) {
			thread, err = cfg.Agent.StartThread(ctx, workspace, settings, toolSpecs())
			if err == nil {
				roles := snapshot.RoleIDs
				roles["reagent"] = thread.ID
				err = owner.UpdateOperational(activework.Operational{Phase: snapshot.Phase, RoleIDs: roles, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters})
			}
		}
		if err == nil && thread.ID != threadID && snapshot.RoleIDs["reagent"] != thread.ID {
			err = fmt.Errorf("reagent thread identity changed without verified loss")
		}
	} else {
		thread, err = cfg.Agent.StartThread(ctx, workspace, settings, toolSpecs())
		if err == nil {
			roles := snapshot.RoleIDs
			roles["reagent"] = thread.ID
			err = owner.UpdateOperational(activework.Operational{Phase: snapshot.Phase, RoleIDs: roles, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters})
		}
	}
	if err != nil {
		if finishErr := owner.FinishDispatch(attemptID, "failed", nil); finishErr != nil {
			return result, fmt.Errorf("reagent resume failed: %w; finish dispatch: %v", err, finishErr)
		}
		return holdContinuation(owner, store, result, "recovery-required", "reagent thread could not be reconstructed: "+err.Error())
	}
	if err := owner.BindDispatchThread(attemptID, thread.ID); err != nil {
		return result, err
	}
	branchesText, err := gitOutput(store.Checkout, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return result, err
	}
	branches := make(map[string]bool)
	for _, branch := range strings.Split(branchesText, "\n") {
		branches[branch] = true
	}
	session := &hostSession{owner: owner, store: store, project: project, workspace: workspace, parent: parent, existingBranches: branches, threadID: thread.ID, taskID: snapshot.TaskID, taskBranch: snapshot.TaskBranch}
	deferred, err := store.UnavailableTaskIDs()
	if err != nil {
		return result, err
	}
	prompt := selectionPrompt(project, parent, workspace, cfg.RequiredChecks, cfg.GeneratedExclusions, deferred)
	if snapshot.TaskID != "" {
		task, err := owner.TaskEvidence()
		if err != nil {
			return result, err
		}
		prompt = fmt.Sprintf("Continue the bound reagent task %s in %s using current source and active evidence. Preserve parent %s and task branch %s. Reconcile any interrupted work before editing or running commands; never replay an uncertain command. Validate and save_checkpoint before a ready result. For a genuine blocker return decision_required with a question object containing objective, evidence, missing, safe_work, next_step and optional stable-ID options/tradeoffs. Do not select another task, read archives, integrate or publish.\n\n%s", snapshot.TaskID, workspace, parent.Commit, snapshot.TaskBranch, task)
		if snapshot.Review != nil && snapshot.Review.Verdict == "changes_requested" {
			findings, err := owner.ReviewEvidence(snapshot.Review.Number)
			if err != nil {
				return result, err
			}
			prompt += "\n\nCompleted review findings:\n" + findings
		}
	}
	if snapshot.Question != nil && snapshot.Question.AnswerMode != "" {
		prompt += fmt.Sprintf("\n\nOperator answer to %s (%s): %s\nVerify any material condition or external action yourself. This answer grants no unrelated dispatch authorization and does not change the agreed scope.\n", snapshot.Question.ID, snapshot.Question.AnswerMode, snapshot.Question.AnswerText)
	}
	if err := owner.MarkTurnRequested(attemptID); err != nil {
		return result, err
	}
	turnCtx, cancelTurn, stopEvent := turnstop.Context(ctx, snapshot.Dispatch.WatchdogElapsed, snapshot.Dispatch.WatchdogStall, cfg.StopTrigger)
	defer cancelTurn()
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
		observeErr = owner.RecordDispatchOutcome(attemptID, thread.ID, outcome.TurnID, outcome.Status, outcome.FinalText)
	}
	if observeErr != nil && turnErr == nil {
		turnErr = observeErr
	}
	status := "completed"
	if turnErr != nil || outcome.Status != "completed" {
		status = "uncertain"
	}
	if err := owner.FinishDispatch(attemptID, status, dispatch.Receipt(outcome.Usage)); err != nil {
		return result, err
	}
	if turnCtx.Err() != nil && turnErr != nil {
		result.Status, err = turnstop.Handle(owner, cfg.Agent, "reagent", thread.ID, outcome.TurnID, attemptID)
		result.Snapshot, _ = store.Status()
		return result, err
	}
	if turnErr != nil || outcome.Status != "completed" {
		return holdContinuation(owner, store, result, "recovery-required", "reagent continuation uncertain in attempt "+attemptID)
	}
	verdict, err := codexapp.ParseRoleResult(outcome.FinalText)
	if err != nil {
		return holdContinuation(owner, store, result, "recovery-required", "invalid reagent continuation result")
	}
	applied, err := applyReagentVerdict(ctx, cfg, owner, store, result, attemptID, thread.ID, outcome.TurnID, verdict)
	if err == nil && applied.Status != "recovery_required" && applied.Status != "stopped_abnormally" && applied.Status != "no_suitable_work" && outcome.TurnID != "" {
		err = owner.MarkDispatchApplied(attemptID)
	}
	return applied, err
}

func verifyResumeWorkspace(store *activework.Store, snapshot activework.Snapshot, workspace string) error {
	common, err := gitOutput(workspace, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("task checkout is unavailable")
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil || filepath.Clean(common) != store.CommonDir {
		return fmt.Errorf("task checkout belongs to another repository")
	}
	if snapshot.TaskID != "" {
		branch, err := gitOutput(workspace, "symbolic-ref", "--quiet", "--short", "HEAD")
		if err != nil || branch != snapshot.TaskBranch {
			return fmt.Errorf("task checkout left its bound branch")
		}
		if _, err := gitOutput(workspace, "merge-base", "--is-ancestor", snapshot.ParentCommit, "HEAD"); err != nil {
			return fmt.Errorf("task branch no longer descends from baseline")
		}
		return nil
	}
	head, err := gitOutput(workspace, "rev-parse", "HEAD")
	if err != nil || head != snapshot.ParentCommit {
		return fmt.Errorf("unclaimed checkout left baseline")
	}
	if err := requireClean(workspace); err != nil {
		return fmt.Errorf("unclaimed checkout has source changes")
	}
	return nil
}

func applyReagentVerdict(ctx context.Context, cfg Config, owner *activework.Owner, store *activework.Store, result Result, attemptID, threadID, turnID string, verdict codexapp.RoleResult) (Result, error) {
	snapshot, err := store.Status()
	if err != nil {
		return result, err
	}
	result.TaskID, result.Summary = snapshot.TaskID, verdict.Summary
	if snapshot.TaskID == "" {
		if verdict.Disposition != "no_suitable_work" {
			return holdContinuation(owner, store, result, "recovery-required", "unclaimed completion needs task claim or no-work verdict")
		}
		if err := removeCleanWorkspace(store, result.Workspace); err != nil {
			return result, err
		}
		if err := owner.EndUnclaimedSelection(verdict.Summary); err != nil {
			return result, err
		}
		result.Status, result.Snapshot = "no_suitable_work", activework.Snapshot{}
		return result, nil
	}
	if verdict.Disposition == "decision_required" {
		if verdict.Question != nil {
			if _, err := owner.AskRoleQuestion("reagent", attemptID, verdict.Question); err != nil {
				return result, err
			}
			result.Status = "decision_required"
			result.Snapshot, err = store.Status()
			return result, err
		}
		return holdContinuation(owner, store, result, "operator-decision", verdict.Summary)
	}
	if verdict.Disposition == "engineering" {
		result.Status = "engineering"
		result.Snapshot = snapshot
		return result, nil
	}
	if verdict.Disposition != "ready_for_review" {
		return holdContinuation(owner, store, result, "recovery-required", "claimed completion has invalid disposition")
	}
	checkpoint, err := owner.Checkpoint()
	if err != nil {
		return result, err
	}
	bound, err := owner.VerifiedTurnReceipt("checkpoint", turnID, checkpoint)
	if err != nil || !bound {
		return holdContinuation(owner, store, result, "recovery-required", "ready checkpoint lacks exact completed-turn receipt")
	}
	if snapshot.Candidate == nil || snapshot.Phase == "engineering" || snapshot.Phase == "correcting" {
		project, err := readProject(store.Checkout)
		if err != nil {
			return result, err
		}
		session := &hostSession{owner: owner, store: store, project: project, workspace: result.Workspace, parent: parentBinding{Branch: snapshot.ParentBranch, Commit: snapshot.ParentCommit}, threadID: threadID, taskID: snapshot.TaskID, taskBranch: snapshot.TaskBranch, checkpointSaved: true}
		if _, err := freezeReady(ctx, cfg, owner, store, session, project, session.parent, codexapp.TurnOutcome{ThreadID: threadID, TurnID: turnID, Status: "completed"}); err != nil {
			return holdContinuation(owner, store, result, "recovery-required", "candidate freeze requires reconciliation: "+err.Error())
		}
	}
	reviewer := reviewerFor(cfg)
	if reviewer == nil {
		result.Status = "ready_for_review"
		result.Snapshot, _ = store.Status()
		return result, nil
	}
	assessment, err := review.Run(ctx, review.Config{Agent: reviewer, OnEvent: cfg.OnEvent, StopTrigger: cfg.StopTrigger}, owner, store)
	result.Status, result.Snapshot = assessment.Status, assessment.Snapshot
	if err != nil {
		return result, err
	}
	if result.Status == "changes_requested" {
		return correctionLoop(ctx, cfg, owner, store, reviewer, result)
	}
	return result, nil
}
