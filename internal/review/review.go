// Package review runs the first independent assessment of a frozen candidate.
// Correction routing and later review cycles belong to the bounded loop task.
package review

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
)

type Agent interface {
	StartThread(context.Context, string, codexapp.RoleSettings, []codexapp.ToolSpec) (codexapp.Thread, error)
	RunTurn(context.Context, string, string, codexapp.RoleSettings, codexapp.ToolHandler, func(codexapp.Event)) (codexapp.TurnOutcome, error)
	Idle(context.Context, string) error
}

type Config struct {
	Agent   Agent
	OnEvent func(codexapp.Event)
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
	decision, err := owner.ReserveDispatch(binding.AttemptID, dispatch.Binding{TaskID: snapshot.TaskID, Role: dispatch.Iragent, Activity: dispatch.Review, Candidate: binding.Candidate}, time.Now().UTC())
	if err != nil {
		return result, err
	}
	if !decision.Dispatch {
		if err := owner.UpdateOperational(activework.Operational{Phase: "reviewing", Hold: "dispatch-authorization", PendingRequest: "authorize initial independent review of " + binding.Candidate, RoleIDs: snapshot.RoleIDs, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters}); err != nil {
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
	if err := prepareCheckout(store, snapshot, result.Checkout); err != nil {
		return failLaunch(owner, store, &result, binding.AttemptID, err)
	}
	if err := verifyCandidate(store, snapshot, result.Checkout, true); err != nil {
		return failLaunch(owner, store, &result, binding.AttemptID, err)
	}
	settings := snapshot.Dispatch.Profiles[dispatch.Iragent]
	if settings.Role != "iragent" || settings.Model == "" || settings.Effort == "" || settings.Sandbox != "danger-full-access" || settings.ApprovalPolicy != "never" {
		return failLaunch(owner, store, &result, binding.AttemptID, fmt.Errorf("reviewer settings are not explicitly resolved for full access"))
	}
	thread, err := cfg.Agent.StartThread(ctx, result.Checkout, settings, toolSpecs())
	if err != nil {
		return failLaunch(owner, store, &result, binding.AttemptID, err)
	}
	if thread.ID == "" || thread.ID == snapshot.RoleIDs["reagent"] {
		return failLaunch(owner, store, &result, binding.AttemptID, fmt.Errorf("reviewer must have a distinct thread"))
	}
	if err := owner.BindReviewThread(binding.Number, thread.ID); err != nil {
		return failLaunch(owner, store, &result, binding.AttemptID, err)
	}
	task, err := owner.TaskEvidence()
	if err != nil {
		return failLaunch(owner, store, &result, binding.AttemptID, err)
	}
	session := &hostSession{owner: owner, review: binding, threadID: thread.ID}
	outcome, turnErr := cfg.Agent.RunTurn(ctx, thread.ID, prompt(snapshot, binding, task, result.Checkout), settings, session.handle, cfg.OnEvent)
	finish := "completed"
	if turnErr != nil || outcome.Status != "completed" {
		finish = "uncertain"
	}
	if err := owner.FinishDispatch(binding.AttemptID, finish, dispatch.Receipt(outcome.Usage)); err != nil {
		return result, err
	}
	if turnErr != nil || outcome.Status != "completed" {
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
	if verdict.Disposition == "decision_required" || verdict.Disposition == "blocker" {
		if err := owner.HoldReview(verdict.Summary); err != nil {
			return result, err
		}
		result.Status = "decision_required"
		result.Snapshot, err = store.Status()
		return result, err
	}
	if verdict.Disposition != "approved" && verdict.Disposition != "changes_requested" {
		return holdRecovery(owner, store, &result, "invalid reviewer disposition", fmt.Errorf("reviewer returned %q", verdict.Disposition))
	}
	if !session.saved {
		return holdRecovery(owner, store, &result, "reviewer returned no assessment checkpoint", fmt.Errorf("completed reviewer verdict has no assessment"))
	}
	if err := session.finalizeEvidence(verdict.Disposition, outcome.TurnID); err != nil {
		return holdRecovery(owner, store, &result, "review evidence could not be finalized", err)
	}
	if err := owner.FinishAssessment(binding.Number, verdict.Disposition, outcome.TurnID); err != nil {
		return holdRecovery(owner, store, &result, "review verdict could not be recorded", err)
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
	if err := owner.UpdateOperational(activework.Operational{Phase: "reviewing", Hold: "recovery-required", PendingRequest: reason, RoleIDs: snapshot.RoleIDs, RoleSettings: snapshot.RoleSettings, Counters: snapshot.Counters}); err != nil {
		return *result, err
	}
	result.Status = "recovery_required"
	result.Snapshot, err = store.Status()
	if cause == nil {
		cause = fmt.Errorf("%s", reason)
	}
	return *result, cause
}

func prompt(snapshot activework.Snapshot, binding activework.PendingReview, task, checkout string) string {
	return fmt.Sprintf("You are iragent, an independent reviewer. Follow AGENTS.md and current repository sources. Review the agreed task and frozen candidate without editing source, branch, index or untracked files. Your separate checkout is %s, pinned to candidate %s with tree %s. The parent baseline is %s; the agreed scope SHA-256 is %s. Assess all acceptance criteria and candidate verification evidence from the active task below. Inspect source and relevant design; do not read unrelated archives or run a separate task. Stream concise progress. Save a pending assessment with save_assessment before returning a structured JSON result. Include specific findings, evidence and checks. Return disposition approved only if the exact candidate meets the scope, changes_requested for actionable source findings, or decision_required for a genuine blocker. Provider completion alone does not approve the candidate. Do not integrate, push or publish.\n\nActive task and verification record:\n%s", checkout, binding.Candidate, snapshot.Candidate.Tree, snapshot.ParentCommit, binding.ScopeSHA256, task)
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
	if call.Role != "iragent" || call.ThreadID != s.threadID || call.TurnID == "" || call.CallID == "" || call.Tool != "save_assessment" {
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
