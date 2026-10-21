// Package review runs a bound independent assessment of a frozen candidate.
package review

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/turnstop"
)

type Agent interface {
	StartThread(context.Context, string, codexapp.RoleSettings, []codexapp.ToolSpec) (codexapp.Thread, error)
	RunTurn(context.Context, string, string, codexapp.RoleSettings, codexapp.ToolHandler, func(codexapp.Event)) (codexapp.TurnOutcome, error)
	Idle(context.Context, string) error
}

type resumer interface {
	ResumeThread(context.Context, string, bool, codexapp.RoleSettings) (codexapp.Thread, error)
}

type Config struct {
	Agent           Agent
	OnEvent         func(codexapp.Event)
	StopTrigger     func(codexapp.Event) bool
	RecoveryEpisode string // nonempty only for a reconciled interrupted assessment
}

type ownedQuieter interface {
	OwnedQuiet(context.Context) error
}

type Result struct {
	Status, Checkout string
	Snapshot         activework.Snapshot
}

// Run is called with the active execution owner, including directly after
// candidate freeze. The dispatch reservation precedes provider thread start.
func Run(ctx context.Context, cfg Config, owner *activework.Owner, store *activework.Store) (Result, error) {
	var result Result
	if cfg.Agent == nil {
		return result, fmt.Errorf("independent reviewer agent is required")
	}
	snapshot, err := store.Status()
	if err != nil {
		return result, err
	}
	if !snapshot.Active || snapshot.Candidate == nil || snapshot.TaskID == "" || snapshot.Dispatch == nil {
		return result, fmt.Errorf("review requires an active frozen candidate and dispatch policy")
	}
	if snapshot.Phase != "ready-for-review" && snapshot.Phase != "reviewing" {
		return result, fmt.Errorf("run is not awaiting independent review")
	}
	attemptID, err := randomID()
	if err != nil {
		return result, err
	}
	binding, err := owner.BeginReview(attemptID)
	if err != nil {
		return result, err
	}
	snapshot, err = store.Status()
	if err != nil {
		return result, err
	}
	result.Checkout = filepath.Join(store.Root, "workspaces", snapshot.RunID+"-review")
	if binding.Verdict != "" {
		return result, fmt.Errorf("review %02d is already finalized", binding.Number)
	}
	if snapshot.Hold != "" && snapshot.Hold != "dispatch-authorization" {
		result.Status = "decision_required"
		if snapshot.Hold == "recovery-required" {
			result.Status = "recovery_required"
		}
		result.Snapshot = snapshot
		return result, nil
	}
	if err := verifyCandidate(store, snapshot, result.Checkout, false); err != nil {
		return holdRecovery(owner, store, &result, "frozen candidate changed before review", err)
	}
	if prior := findAttempt(snapshot.Dispatch, binding.AttemptID); prior != nil {
		// A previously dispatched turn may have run. Its existing review number,
		// checkpoint and thread are retained for the recovery controller.
		if snapshot.Hold == "recovery-required" {
			result.Status, result.Snapshot = "recovery_required", snapshot
			return result, nil
		}
		return holdRecovery(owner, store, &result, "prior review dispatch requires reconciliation", fmt.Errorf("review attempt %s is %s", prior.ID, prior.Status))
	}
	activity := dispatch.Review
	if cfg.RecoveryEpisode != "" {
		activity = dispatch.Recovery
	}
	decision, err := owner.ReserveDispatch(binding.AttemptID, dispatch.Binding{TaskID: snapshot.TaskID, Role: dispatch.Iragent, Activity: activity, Candidate: binding.Candidate, Episode: cfg.RecoveryEpisode}, time.Now().UTC())
	if err != nil {
		if strings.Contains(err.Error(), "allowance exhausted") {
			return holdLimit(owner, store, &result, err)
		}
		return holdRecovery(owner, store, &result, "review dispatch requires reconciliation", err)
	}
	if !decision.Dispatch {
		if err := owner.UpdateOperational(activework.Operational{Phase: "reviewing", Hold: "dispatch-authorization", PendingRequest: "authorize independent review of " + binding.Candidate, RoleIDs: snapshot.RoleIDs, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters}); err != nil {
			return result, err
		}
		result.Status = "authorization_required"
		result.Snapshot, err = store.Status()
		return result, err
	}
	if snapshot.Hold == "dispatch-authorization" {
		if err := owner.UpdateOperational(activework.Operational{Phase: "reviewing", RoleIDs: snapshot.RoleIDs, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters}); err != nil {
			return result, err
		}
	}
	settings := snapshot.Dispatch.Profiles[dispatch.Iragent]
	if settings.Role != "iragent" || settings.Model == "" || settings.Effort == "" || settings.Sandbox != "danger-full-access" || settings.ApprovalPolicy != "never" {
		return failLaunch(owner, store, &result, binding.AttemptID, fmt.Errorf("reviewer settings are not explicitly resolved for full access"))
	}
	priorID := snapshot.RoleIDs["iragent"]
	hadTurn := reviewerHadTurn(snapshot, priorID)
	var thread codexapp.Thread
	lost := false
	if binding.Number > 1 {
		if err := cfg.Agent.Idle(ctx, priorID); err != nil {
			if !codexapp.IsMissingRollout(err) {
				return failLaunch(owner, store, &result, binding.AttemptID, fmt.Errorf("prior reviewer is not idle: %w", err))
			}
			// A missing rollout may make the provider's per-thread idle query
			// unavailable. Confirm exact loss and owned-process quiescence before
			// changing the review checkout.
			resuming, ok := cfg.Agent.(resumer)
			if !ok || priorID == "" {
				return failLaunch(owner, store, &result, binding.AttemptID, fmt.Errorf("prior reviewer is not idle: %w", err))
			}
			_, resumeErr := resuming.ResumeThread(ctx, priorID, hadTurn, settings)
			if resumeErr == nil || !canReplaceReviewer(ctx, cfg.Agent, snapshot, priorID, hadTurn, resumeErr) {
				return failLaunch(owner, store, &result, binding.AttemptID, fmt.Errorf("prior reviewer is not idle: %w; resume: %v", err, resumeErr))
			}
			lost = true
		}
		head, headErr := gitOutput(result.Checkout, "rev-parse", "HEAD")
		if headErr != nil {
			return failLaunch(owner, store, &result, binding.AttemptID, headErr)
		}
		if head != binding.Candidate {
			if err := advanceCheckout(store, snapshot, result.Checkout); err != nil {
				return failLaunch(owner, store, &result, binding.AttemptID, err)
			}
		}
	} else if err := prepareCheckout(store, snapshot, result.Checkout); err != nil {
		return failLaunch(owner, store, &result, binding.AttemptID, err)
	}
	if err := verifyCandidate(store, snapshot, result.Checkout, true); err != nil {
		return failLaunch(owner, store, &result, binding.AttemptID, err)
	}
	if binding.ThreadID == "" && binding.Number == 1 {
		thread, err = cfg.Agent.StartThread(ctx, result.Checkout, settings, toolSpecs())
	} else if lost {
		thread, err = cfg.Agent.StartThread(ctx, result.Checkout, settings, toolSpecs())
		if err == nil {
			err = owner.ReplaceLostReviewThread(binding.Number, priorID, thread.ID)
		}
	} else {
		resuming, ok := cfg.Agent.(resumer)
		if !ok || priorID == "" {
			return failLaunch(owner, store, &result, binding.AttemptID, fmt.Errorf("existing reviewer thread cannot be resumed"))
		}
		replaced := false
		thread, err = resuming.ResumeThread(ctx, priorID, hadTurn, settings)
		if err != nil && canReplaceReviewer(ctx, cfg.Agent, snapshot, priorID, hadTurn, err) {
			thread, err = cfg.Agent.StartThread(ctx, result.Checkout, settings, toolSpecs())
			if err == nil {
				err = owner.ReplaceLostReviewThread(binding.Number, priorID, thread.ID)
				replaced = err == nil
			}
		}
		if err == nil && thread.ID != priorID && !replaced {
			err = fmt.Errorf("resumed reviewer identity changed")
		}
	}
	if err != nil {
		return failLaunch(owner, store, &result, binding.AttemptID, err)
	}
	if thread.ID == "" || thread.ID == snapshot.RoleIDs["reagent"] {
		return failLaunch(owner, store, &result, binding.AttemptID, fmt.Errorf("reviewer must have a distinct thread"))
	}
	if err := owner.BindReviewThread(binding.Number, thread.ID); err != nil {
		return failLaunch(owner, store, &result, binding.AttemptID, err)
	}
	if err := owner.BindDispatchThread(binding.AttemptID, thread.ID); err != nil {
		return failLaunch(owner, store, &result, binding.AttemptID, err)
	}
	task, err := owner.TaskEvidence()
	if err != nil {
		return failLaunch(owner, store, &result, binding.AttemptID, err)
	}
	session := &hostSession{owner: owner, review: binding, threadID: thread.ID}
	reviewPrompt := prompt(snapshot, binding, task, result.Checkout)
	if binding.ThreadID != "" {
		if pending, readErr := owner.ReviewEvidence(binding.Number); readErr == nil {
			reviewPrompt += "\n\nPending assessment from this same review number:\n" + pending
		}
	}
	if binding.Number > 1 {
		previous, priorErr := owner.ReviewEvidence(binding.Number - 1)
		if priorErr != nil {
			return failLaunch(owner, store, &result, binding.AttemptID, priorErr)
		}
		reviewPrompt += fmt.Sprintf("\n\nPrior completed assessment and findings:\n%s\nCorrected candidate is %s; inspect its exact diff from prior candidate and reassess all criteria.\n", previous, binding.Candidate)
	}
	if err := owner.MarkTurnRequested(binding.AttemptID); err != nil {
		return result, err
	}
	turnCtx, cancelTurn, stopEvent, finishTurn := turnstop.Context(ctx, snapshot.Dispatch.WatchdogElapsed, snapshot.Dispatch.WatchdogStall, cfg.StopTrigger)
	defer cancelTurn()
	var observeErr error
	outcome, turnErr := cfg.Agent.RunTurn(turnCtx, thread.ID, reviewPrompt, settings, session.handle, func(event codexapp.Event) {
		stopEvent(event)
		if event.Kind == "turn/started" && event.TurnID != "" && observeErr == nil {
			observeErr = owner.ObserveDispatchTurn(binding.AttemptID, event.TurnID)
			if observeErr != nil {
				cancelTurn()
			}
		}
		if cfg.OnEvent != nil {
			cfg.OnEvent(event)
		}
	})
	stopReason := finishTurn()
	if stopReason == nil && outcome.TurnID == "" && errors.Is(turnErr, codexapp.ErrInvalidResponseSchema) {
		if err := owner.FailResponseSchema(binding.AttemptID, turnErr.Error()); err != nil {
			return result, err
		}
		result.Status = "startup_failed"
		result.Snapshot, err = store.Status()
		return result, err
	}
	if outcome.TurnID != "" && observeErr == nil {
		observeErr = owner.ObserveDispatchTurn(binding.AttemptID, outcome.TurnID)
	}
	if outcome.TurnID != "" && observeErr == nil {
		observeErr = owner.RecordDispatchCallbacks(binding.AttemptID, thread.ID, outcome.TurnID, outcome.Callbacks)
	}
	if outcome.TurnID != "" && outcome.Status != "" && observeErr == nil {
		if outcome.ThreadID != "" && outcome.ThreadID != thread.ID {
			observeErr = fmt.Errorf("provider outcome thread differs from reviewer binding")
		} else {
			observeErr = owner.RecordDispatchOutcome(binding.AttemptID, thread.ID, outcome.TurnID, outcome.Status, outcome.FinalText)
		}
	}
	if observeErr != nil && turnErr == nil {
		turnErr = observeErr
	}
	finish := "completed"
	if stopReason != nil || turnErr != nil || outcome.Status != "completed" {
		finish = "uncertain"
	}
	if err := owner.FinishDispatch(binding.AttemptID, finish, dispatch.Receipt(outcome.Usage)); err != nil {
		return result, err
	}
	if stopReason != nil {
		result.Status, err = turnstop.Handle(owner, cfg.Agent, "iragent", thread.ID, outcome.TurnID, binding.AttemptID, stopReason)
		result.Snapshot, _ = store.Status()
		return result, err
	}
	if turnErr != nil || outcome.Status != "completed" {
		if turnErr == nil {
			turnErr = fmt.Errorf("provider reported %s: %s", outcome.Status, outcome.ProviderError)
		}
		return holdRecovery(owner, store, &result, "review turn interrupted or uncertain", turnErr)
	}
	if outcome.ThreadID != thread.ID || outcome.TurnID == "" {
		return holdRecovery(owner, store, &result, "review turn identity is invalid", fmt.Errorf("review turn identity differs from reviewer thread"))
	}
	if err := cfg.Agent.Idle(ctx, thread.ID); err != nil {
		return holdRecovery(owner, store, &result, "review tool work is not idle", err)
	}
	if err := verifyCandidate(store, snapshot, result.Checkout, true); err != nil {
		return holdRecovery(owner, store, &result, "reviewed source changed", err)
	}
	verdict, err := codexapp.ParseRoleResult(outcome.FinalText)
	if err != nil {
		return holdRecovery(owner, store, &result, "invalid reviewer result", err)
	}
	if verdict.TaskID != "" && verdict.TaskID != snapshot.TaskID || verdict.Candidate != "" && verdict.Candidate != binding.Candidate {
		return holdRecovery(owner, store, &result, "reviewer result binding differs from task or candidate", nil)
	}
	if verdict.Disposition == "decision_required" || verdict.Disposition == "blocker" {
		if verdict.Question != nil {
			if verdict.Question.SourceChange {
				if err := session.verifyPendingEvidence(outcome.TurnID); err != nil {
					return holdRecovery(owner, store, &result, "source-changing reviewer question lacks candidate-bound findings", err)
				}
			}
			if _, err := owner.AskRoleQuestion("iragent", binding.AttemptID, verdict.Question); err != nil {
				return holdRecovery(owner, store, &result, "reviewer blocker lacks usable question context", err)
			}
		} else {
			if err := owner.HoldReview(verdict.Summary); err != nil {
				return result, err
			}
			if err := owner.MarkDispatchApplied(binding.AttemptID); err != nil {
				return result, err
			}
		}
		result.Status = "decision_required"
		result.Snapshot, err = store.Status()
		return result, err
	}
	if verdict.Disposition != "approved" && verdict.Disposition != "changes_requested" {
		return holdRecovery(owner, store, &result, "invalid reviewer disposition", fmt.Errorf("reviewer returned %q", verdict.Disposition))
	}
	if err := session.saveResult(verdict, outcome.TurnID); err != nil {
		return holdRecovery(owner, store, &result, "reviewer result could not be recorded", err)
	}
	if err := session.finalizeEvidence(verdict.Disposition, outcome.TurnID); err != nil {
		return holdRecovery(owner, store, &result, "review evidence could not be finalized", err)
	}
	if err := owner.FinishAssessment(binding.Number, verdict.Disposition, outcome.TurnID); err != nil {
		return holdRecovery(owner, store, &result, "review verdict could not be recorded", err)
	}
	if err := owner.MarkDispatchApplied(binding.AttemptID); err != nil {
		return result, err
	}
	result.Status = verdict.Disposition
	result.Snapshot, err = store.Status()
	return result, err
}

func failLaunch(owner *activework.Owner, store *activework.Store, result *Result, attemptID string, cause error) (Result, error) {
	if err := owner.FinishDispatch(attemptID, "failed", nil); err != nil {
		return *result, fmt.Errorf("%w; finish review dispatch: %v", cause, err)
	}
	return holdRecovery(owner, store, result, "review launch failed", cause)
}

func findAttempt(policy *dispatch.State, id string) *dispatch.Attempt {
	for i := range policy.Attempts {
		if policy.Attempts[i].ID == id {
			return &policy.Attempts[i]
		}
	}
	return nil
}

func reviewerHadTurn(snapshot activework.Snapshot, threadID string) bool {
	if snapshot.Dispatch == nil || threadID == "" {
		return false
	}
	for _, attempt := range snapshot.Dispatch.Attempts {
		if attempt.ThreadID == threadID && attempt.Binding.Role == dispatch.Iragent && attempt.TurnRequested {
			return true
		}
	}
	return false
}

// Missing rollout is evidence of session loss, not of process cessation.
// Previously dispatched work must be resolved and the dedicated provider's
// owned execution children quiet before a replacement thread is launched.
func canReplaceReviewer(ctx context.Context, agent Agent, snapshot activework.Snapshot, threadID string, hadTurn bool, resumeErr error) bool {
	if threadID == "" || snapshot.Dispatch == nil || !(codexapp.IsMissingRollout(resumeErr) || !hadTurn && codexapp.IsProvisionalThread(resumeErr)) {
		return false
	}
	for _, attempt := range snapshot.Dispatch.Attempts {
		if attempt.ThreadID != threadID || attempt.Binding.Role != dispatch.Iragent {
			continue
		}
		if attempt.Status == "reserved" || attempt.Status == "uncertain" {
			return false
		}
		if !attempt.TurnRequested {
			continue
		}
		if attempt.Status == "completed" && attempt.Applied {
			continue
		}
		stopped := false
		for _, record := range snapshot.StopHistory {
			if record.AttemptID == attempt.ID && record.Status == "stopped_safely" {
				stopped = true
			}
		}
		if attempt.Status != "reconciled" || !stopped {
			return false
		}
	}
	if !hadTurn {
		return true // metadata-only thread: no provider turn was requested
	}
	quieter, ok := agent.(ownedQuieter)
	return ok && quieter.OwnedQuiet(ctx) == nil
}

func randomID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

func holdRecovery(owner *activework.Owner, store *activework.Store, result *Result, reason string, cause error) (Result, error) {
	snapshot, err := store.Status()
	if err != nil {
		return *result, err
	}
	attemptID := ""
	if snapshot.Review != nil {
		attemptID = snapshot.Review.AttemptID
	}
	message := reason
	if cause != nil {
		message += ": " + cause.Error()
	}
	if err := owner.HoldRecovery(activework.RecoveryIssue{AttemptID: attemptID, Role: "iragent", Stage: "review", Operation: "assessment", Reason: message, EvidenceRef: "active review attempt " + attemptID}); err != nil {
		return *result, err
	}
	result.Status = "recovery_required"
	result.Snapshot, err = store.Status()
	if cause == nil {
		cause = fmt.Errorf("%s", reason)
	}
	return *result, cause
}

func holdLimit(owner *activework.Owner, store *activework.Store, result *Result, cause error) (Result, error) {
	snapshot, err := store.Status()
	if err != nil {
		return *result, err
	}
	action := cause.Error()
	if snapshot.Dispatch != nil {
		if strings.Contains(action, "dispatch allowance") {
			action = fmt.Sprintf("extend dispatch allowance to %d to assess %s candidate %s", len(snapshot.Dispatch.Attempts)+1, snapshot.TaskID, snapshot.Candidate.Commit)
		} else if strings.Contains(action, "assessment allowance") {
			action = fmt.Sprintf("extend assessment allowance to %d to assess %s candidate %s", len(snapshot.Dispatch.Assessments)+1, snapshot.TaskID, snapshot.Candidate.Commit)
		}
	}
	if err := owner.UpdateOperational(activework.Operational{Phase: "reviewing", Hold: "limit-exhausted", PendingRequest: action, RoleIDs: snapshot.RoleIDs, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters}); err != nil {
		return *result, err
	}
	result.Status = "decision_required"
	result.Snapshot, err = store.Status()
	return *result, err
}

func prompt(snapshot activework.Snapshot, binding activework.PendingReview, task, checkout string) string {
	return fmt.Sprintf("You are iragent, an independent reviewer. Follow AGENTS.md and current repository sources. Review the agreed task and frozen candidate without editing source, branch, index or untracked files. Your separate checkout is %s, pinned to candidate %s with tree %s. The parent baseline is %s; the agreed scope SHA-256 is %s. Judge the implementation, checks, documentation and any prior findings yourself. Inspect source and relevant design; do not read unrelated archives or run a separate task. Stream concise progress. Return one JSON result with disposition approved, changes_requested, or decision_required, summary, task_id %s, candidate %s, and findings as an array of specific actionable strings when changes are requested. A decision_required result includes a question object with objective, evidence, missing, safe_work, next_step, optional stable-ID options/tradeoffs, and source_change or scope_change flags when applicable. If a source-changing decision needs retained findings, save them with save_assessment. A source-changing answer stays within the agreed scope unless separately authorized. Do not integrate, push or publish.\n\nActive task and verification record:\n%s", checkout, binding.Candidate, snapshot.Candidate.Tree, snapshot.ParentCommit, binding.ScopeSHA256, snapshot.TaskID, binding.Candidate, task)
}

// ReconcileCompleted finalizes one provider-observed completed assessment
// only when its exact callback receipt and frozen source still match.
func ReconcileCompleted(ctx context.Context, agent Agent, owner *activework.Owner, store *activework.Store, observed codexapp.ObservedTurn) (Result, error) {
	var result Result
	snapshot, err := store.Status()
	if err != nil {
		return result, err
	}
	if snapshot.Review == nil || snapshot.Review.Verdict != "" || snapshot.Candidate == nil || snapshot.Review.Candidate != snapshot.Candidate.Commit || observed.ID == "" || observed.Status != "completed" {
		return result, fmt.Errorf("no pending exact assessment to reconcile")
	}
	result.Checkout = filepath.Join(store.Root, "workspaces", snapshot.RunID+"-review")
	if agent == nil || snapshot.Review.ThreadID == "" {
		return result, fmt.Errorf("bound reviewer is unavailable for idle verification")
	}
	if err := agent.Idle(ctx, snapshot.Review.ThreadID); err != nil {
		return result, fmt.Errorf("reviewer tool work is not idle: %w", err)
	}
	if err := verifyCandidate(store, snapshot, result.Checkout, true); err != nil {
		return result, err
	}
	verdict, err := codexapp.ParseRoleResult(observed.FinalText)
	if err != nil || (verdict.Disposition != "approved" && verdict.Disposition != "changes_requested") {
		return result, fmt.Errorf("completed reviewer has no actionable verdict: %v", err)
	}
	if verdict.TaskID != "" && verdict.TaskID != snapshot.TaskID || verdict.Candidate != "" && verdict.Candidate != snapshot.Review.Candidate {
		return result, fmt.Errorf("reviewer result binding differs from task or candidate")
	}
	session := &hostSession{owner: owner, review: *snapshot.Review, threadID: snapshot.Review.ThreadID}
	if err := owner.ReconcileCompletedDispatch(snapshot.Review.AttemptID, snapshot.Review.ThreadID, observed.ID, "provider completed exact review turn"); err != nil {
		return result, err
	}
	if err := session.saveResult(verdict, observed.ID); err != nil {
		return result, err
	}
	content, err := owner.ReviewEvidence(snapshot.Review.Number)
	if err != nil {
		return result, err
	}
	if !strings.HasSuffix(content, fmt.Sprintf("\nVerdict: %s\nReviewer turn: %s\n", verdict.Disposition, observed.ID)) {
		if err := session.finalizeEvidence(verdict.Disposition, observed.ID); err != nil {
			return result, err
		}
	}
	if err := owner.FinishAssessment(snapshot.Review.Number, verdict.Disposition, observed.ID); err != nil {
		return result, err
	}
	if err := owner.MarkDispatchApplied(snapshot.Review.AttemptID); err != nil {
		return result, err
	}
	result.Status = verdict.Disposition
	result.Snapshot, err = store.Status()
	return result, err
}

// RouteAnsweredChange turns a completed reviewer source-change blocker into
// candidate-bound findings after the operator answer is durably recorded.
// The reagent then makes the change and a fresh assessment reviews its tree.
func RouteAnsweredChange(ctx context.Context, agent Agent, owner *activework.Owner, store *activework.Store, questionID string) (Result, error) {
	var result Result
	snapshot, err := store.Status()
	if err != nil {
		return result, err
	}
	q := snapshot.Question
	if q == nil || q.ID != questionID || q.Requester != "iragent" || !q.SourceChange || q.ScopeChange || q.AnswerMode == "" || snapshot.Hold != "" || snapshot.Review == nil || snapshot.Review.Verdict != "" || snapshot.Review.AttemptID != q.OriginAttemptID || agent == nil {
		return result, fmt.Errorf("answered source-change question is not bound to a pending review")
	}
	var attempt *dispatch.Attempt
	for i := range snapshot.Dispatch.Attempts {
		if snapshot.Dispatch.Attempts[i].ID == q.OriginAttemptID {
			attempt = &snapshot.Dispatch.Attempts[i]
		}
	}
	if attempt == nil || attempt.Status != "completed" || !attempt.Applied || attempt.FinalStatus != "completed" || attempt.TurnID == "" || attempt.ThreadID != snapshot.Review.ThreadID {
		return result, fmt.Errorf("reviewer source-change turn lacks completed identity")
	}
	verdict, err := codexapp.ParseRoleResult(attempt.FinalText)
	if err != nil || verdict.Question == nil || !verdict.Question.SourceChange || (verdict.Disposition != "decision_required" && verdict.Disposition != "blocker") {
		return result, fmt.Errorf("reviewer did not request a source-changing decision")
	}
	if err := agent.Idle(ctx, attempt.ThreadID); err != nil {
		return result, fmt.Errorf("reviewer execution is not idle: %w", err)
	}
	result.Checkout = filepath.Join(store.Root, "workspaces", snapshot.RunID+"-review")
	if err := verifyCandidate(store, snapshot, result.Checkout, true); err != nil {
		return result, err
	}
	content, err := owner.ReviewEvidence(snapshot.Review.Number)
	if err != nil {
		return result, err
	}
	session := &hostSession{owner: owner, review: *snapshot.Review, threadID: attempt.ThreadID, turnID: attempt.TurnID, saved: true}
	if !strings.HasPrefix(content, session.header()) {
		return result, fmt.Errorf("pending assessment belongs to another candidate or scope")
	}
	body := strings.TrimSpace(strings.TrimPrefix(content, session.header()))
	receipted, err := owner.VerifiedTurnReceipt("assessment", attempt.TurnID, body)
	if err != nil || !receipted {
		return result, fmt.Errorf("reviewer source-change findings lack exact receipt: %v", err)
	}
	if err := session.finalizeEvidence("changes_requested", attempt.TurnID); err != nil {
		return result, err
	}
	if err := owner.FinishAssessment(snapshot.Review.Number, "changes_requested", attempt.TurnID); err != nil {
		return result, err
	}
	result.Status = "changes_requested"
	result.Snapshot, err = store.Status()
	return result, err
}

func toolSpecs() []codexapp.ToolSpec {
	return []codexapp.ToolSpec{{Type: "function", Name: "save_assessment", Description: "Save the pending assessment for this exact candidate without editing source.", InputSchema: json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`)}}
}

type hostSession struct {
	mu       sync.Mutex
	owner    *activework.Owner
	review   activework.PendingReview
	threadID string
	turnID   string
	saved    bool
}

func (s *hostSession) handle(_ context.Context, call codexapp.ToolCall) (codexapp.PreparedToolResponse, error) {
	if call.Role != "iragent" || call.ThreadID != s.threadID || call.TurnID == "" || call.CallID == "" || call.Namespace != "" || call.Tool != "save_assessment" {
		return codexapp.PreparedToolResponse{}, fmt.Errorf("callback does not belong to pending iragent assessment")
	}
	var args struct {
		Content string `json:"content"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(call.Arguments)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return codexapp.PreparedToolResponse{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return codexapp.PreparedToolResponse{}, fmt.Errorf("assessment callback has trailing JSON")
	}
	return codexapp.PreparedToolResponse{Success: true, Text: "Pending assessment saved.", Commit: func(context.Context) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.turnID != "" && s.turnID != call.TurnID {
			return fmt.Errorf("assessment callback belongs to another reviewer turn")
		}
		if len(call.CallID) > 256 {
			return fmt.Errorf("callback ID is too long")
		}
		receipt, err := json.Marshal(struct{ CallID, TurnID, Content string }{call.CallID, call.TurnID, args.Content})
		if err != nil {
			return err
		}
		key := sha256.Sum256([]byte(call.CallID))
		name := "assessment-" + hex.EncodeToString(key[:12])
		exists, err := s.owner.CheckReceipt(name, receipt)
		if err != nil {
			return err
		}
		if exists {
			s.turnID = call.TurnID
			s.saved = true
			return nil
		}
		if err := s.owner.SaveReview(s.review.Number, s.header()+strings.TrimSpace(args.Content)+"\n"); err != nil {
			return err
		}
		if err := s.owner.RecordReceipt(name, receipt); err != nil {
			return err
		}
		s.turnID = call.TurnID
		s.saved = true
		return nil
	}}, nil
}

func (s *hostSession) header() string {
	return fmt.Sprintf("# Independent review %02d\n\nCandidate: %s\nAgreed scope SHA-256: %s\n\n## Assessment\n\n", s.review.Number, s.review.Candidate, s.review.ScopeSHA256)
}

func (s *hostSession) saveResult(verdict codexapp.RoleResult, turnID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saved && s.turnID != turnID {
		return fmt.Errorf("pending assessment belongs to another reviewer turn")
	}
	if verdict.Disposition == "changes_requested" && len(verdict.Findings) == 0 {
		return fmt.Errorf("changes requested without findings")
	}
	body := strings.TrimSpace(verdict.Summary)
	for _, finding := range verdict.Findings {
		if strings.TrimSpace(finding) == "" {
			return fmt.Errorf("empty review finding")
		}
		body += "\n- " + strings.TrimSpace(finding)
	}
	if prior, err := s.owner.ReviewEvidence(s.review.Number); err == nil {
		if !strings.HasPrefix(prior, s.header()) {
			return fmt.Errorf("pending assessment belongs to another candidate or scope")
		}
		priorBody := strings.TrimSpace(strings.TrimPrefix(prior, s.header()))
		finalized := false
		if suffix := fmt.Sprintf("\n\nVerdict: %s\nReviewer turn: %s", verdict.Disposition, turnID); strings.HasSuffix(priorBody, suffix) {
			priorBody = strings.TrimSuffix(priorBody, suffix)
			finalized = true
		}
		if priorBody != body {
			if finalized {
				return fmt.Errorf("finalized assessment differs from reviewer result")
			}
			verified, err := s.owner.VerifiedTurnReceipt("assessment", turnID, priorBody)
			if err != nil || !verified {
				return fmt.Errorf("pending assessment lacks exact reviewer receipt: %v", err)
			}
			if err := s.owner.SaveReview(s.review.Number, s.header()+body+"\n"); err != nil {
				return err
			}
		}
		s.saved, s.turnID = true, turnID
		return nil
	}
	if err := s.owner.SaveReview(s.review.Number, s.header()+body+"\n"); err != nil {
		return err
	}
	s.saved, s.turnID = true, turnID
	return nil
}

func (s *hostSession) finalizeEvidence(verdict, turnID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.saved || s.turnID != turnID {
		return fmt.Errorf("no pending assessment")
	}
	content, err := s.owner.ReviewEvidence(s.review.Number)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(content, s.header()) {
		return fmt.Errorf("pending assessment belongs to another candidate or scope")
	}
	return s.owner.SaveReview(s.review.Number, strings.TrimRight(content, "\n")+fmt.Sprintf("\n\nVerdict: %s\nReviewer turn: %s\n", verdict, turnID))
}

func (s *hostSession) verifyPendingEvidence(turnID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.saved || s.turnID != turnID {
		return fmt.Errorf("no assessment saved by the requesting reviewer turn")
	}
	content, err := s.owner.ReviewEvidence(s.review.Number)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(content, s.header()) {
		return fmt.Errorf("pending assessment belongs to another candidate or scope")
	}
	body := strings.TrimSpace(strings.TrimPrefix(content, s.header()))
	if body == "" {
		return fmt.Errorf("pending assessment contains no findings")
	}
	receipted, err := s.owner.VerifiedTurnReceipt("assessment", turnID, body)
	if err != nil {
		return err
	}
	if !receipted {
		return fmt.Errorf("pending assessment lacks exact reviewer callback receipt")
	}
	return nil
}
