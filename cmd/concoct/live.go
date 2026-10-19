package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/integration"
	"github.com/gopher-launch/concoct/internal/review"
	"github.com/gopher-launch/concoct/internal/selection"
)

type provider interface {
	Agent() selection.Agent
	Reviewer() review.Agent
	PrepareIdle(context.Context, activework.Snapshot) error
	Models(context.Context) ([]codexapp.Model, error)
	RateLimits(context.Context) (json.RawMessage, error)
	Close() error
}

type codexProvider struct {
	process *codexapp.Process
	agent   selection.CodexAgent
}

func (p codexProvider) Agent() selection.Agent { return p.agent }
func (p codexProvider) Reviewer() review.Agent { return p.agent }
func (p codexProvider) PrepareIdle(ctx context.Context, snapshot activework.Snapshot) error {
	if snapshot.Dispatch == nil {
		return fmt.Errorf("active dispatch settings are unavailable")
	}
	for _, role := range []dispatch.Role{dispatch.Reagent, dispatch.Iragent} {
		id := snapshot.RoleIDs[string(role)]
		if id == "" {
			continue
		}
		settings, ok := snapshot.Dispatch.Profiles[role]
		if !ok {
			return fmt.Errorf("%s settings are unavailable", role)
		}
		hadTurn := false
		for _, attempt := range snapshot.Dispatch.Attempts {
			if attempt.Binding.Role == role && attempt.ThreadID == id && attempt.TurnID != "" {
				hadTurn = true
				break
			}
		}
		if _, err := p.agent.ResumeThread(ctx, id, hadTurn, settings); err != nil {
			return fmt.Errorf("resume %s for idle verification: %w", role, err)
		}
	}
	return nil
}
func (p codexProvider) Models(ctx context.Context) ([]codexapp.Model, error) {
	return p.process.Client.Models(ctx)
}
func (p codexProvider) RateLimits(ctx context.Context) (json.RawMessage, error) {
	return p.process.Client.RateLimits(ctx)
}
func (p codexProvider) Close() error { return p.process.Close() }

type liveOperations struct {
	Open func(context.Context, string) (provider, error)
}

func (l liveOperations) open(ctx context.Context, executable string) (provider, error) {
	if l.Open != nil {
		return l.Open(ctx, executable)
	}
	process, err := codexapp.Launch(ctx, executable, io.Discard)
	if err != nil {
		return nil, err
	}
	agent, err := selection.NewCodexAgent(process)
	if err != nil {
		process.Close()
		return nil, err
	}
	return codexProvider{process: process, agent: agent}, nil
}

func (l liveOperations) Do(ctx context.Context, req request, onEvent func(codexapp.Event)) (outcome, error) {
	store, err := activework.Open(req.Repository, req.Root)
	if err != nil {
		return outcome{}, err
	}
	snapshot, err := store.Status()
	if err != nil {
		return outcome{}, err
	}
	if req.Command == "status" {
		if req.Task != "" && (!snapshot.Active || snapshot.TaskID != req.Task) {
			return outcome{Snapshot: snapshot}, fmt.Errorf("task %q is not active", req.Task)
		}
		result := outcome{Disposition: disposition(snapshot), Snapshot: snapshot}
		if result.Disposition == "ready_for_acceptance" && snapshot.Acceptance != nil {
			result.Approval, err = approvalMaterials(store, snapshot)
			if err != nil {
				return outcome{Snapshot: snapshot}, err
			}
		}
		return result, err
	}
	if snapshot.PendingStart {
		owner, err := store.Acquire()
		if err != nil {
			return outcome{Snapshot: snapshot}, err
		}
		if err := owner.Close(); err != nil {
			return outcome{Snapshot: snapshot}, err
		}
		snapshot, err = store.Status()
		if err != nil {
			return outcome{}, err
		}
	}
	if snapshot.Active && hasPolicyChoices(req) {
		return outcome{Snapshot: snapshot}, fmt.Errorf("role and gate choices cannot change an active run")
	}
	if req.Approve != "" {
		if !snapshot.Active || snapshot.Acceptance == nil || snapshot.Acceptance.ID != req.Approve {
			return outcome{Snapshot: snapshot}, fmt.Errorf("acceptance ID does not match the active reviewed candidate")
		}
		return l.approve(ctx, req, store, snapshot, onEvent)
	}
	if req.Extend != "" {
		if !snapshot.Active || snapshot.TaskID != req.Extend || snapshot.Dispatch == nil {
			return outcome{Snapshot: snapshot}, fmt.Errorf("extension task does not match active work")
		}
		limits := snapshot.Dispatch.Limits
		if req.DispatchLimit != 0 {
			limits.Dispatches = req.DispatchLimit
		}
		if req.ReviewLimit != 0 {
			limits.Assessments = req.ReviewLimit
		}
		if req.RecoveryLimit != 0 {
			limits.Recovery = req.RecoveryLimit
		}
		if req.SelfReviewLimit != 0 {
			limits.SelfReviews = req.SelfReviewLimit
		}
		owner, err := store.Acquire()
		if err != nil {
			return outcome{Snapshot: snapshot}, err
		}
		err = owner.ExtendDispatchLimits(limits)
		closeErr := owner.Close()
		if err != nil {
			return outcome{Snapshot: snapshot}, err
		}
		if closeErr != nil {
			return outcome{Snapshot: snapshot}, closeErr
		}
	}
	if req.Authorize != "" {
		if !snapshot.Active || snapshot.Dispatch == nil || snapshot.Dispatch.Pending == nil || snapshot.Dispatch.Pending.ID != req.Authorize {
			return outcome{Snapshot: snapshot}, fmt.Errorf("authorization ID does not match pending dispatch")
		}
		owner, err := store.Acquire()
		if err != nil {
			return outcome{Snapshot: snapshot}, err
		}
		err = owner.AuthorizeDispatch(req.Authorize)
		closeErr := owner.Close()
		if err != nil {
			return outcome{Snapshot: snapshot}, err
		}
		if closeErr != nil {
			return outcome{Snapshot: snapshot}, closeErr
		}
	}
	if req.Answer != "" && (!snapshot.Active || snapshot.Question == nil || snapshot.Question.ID != req.Answer) {
		return outcome{Snapshot: snapshot}, fmt.Errorf("question ID does not match active request")
	}
	if (req.Defer != "" || req.Abandon != "") && (!snapshot.Active || snapshot.TaskID != firstNonempty(req.Defer, req.Abandon)) {
		return outcome{Snapshot: snapshot}, fmt.Errorf("disposition task does not match active work")
	}
	if (req.Defer != "" || req.Abandon != "") && snapshot.Integration != nil {
		return outcome{Snapshot: snapshot}, fmt.Errorf("integrated work requires completion maintenance")
	}
	if req.Resume != "" && snapshot.Active {
		return outcome{Snapshot: snapshot}, fmt.Errorf("another run owns the repository")
	}
	if req.Resume != "" {
		deferred, err := store.DeferredTaskIDs()
		if err != nil {
			return outcome{Snapshot: snapshot}, err
		}
		if !deferred[req.Resume] {
			return outcome{Snapshot: snapshot}, fmt.Errorf("task %q is not deferred", req.Resume)
		}
	}
	if req.Answer == "" && req.Authorize == "" && req.Extend == "" && req.Defer == "" && req.Abandon == "" && req.Resume == "" {
		if snapshot.Active && snapshot.Completion != nil {
			return complete(store)
		}
		if snapshot.Active && snapshot.Integration != nil {
			return l.approve(ctx, req, store, snapshot, onEvent)
		}
		if snapshot.Active && snapshot.Hold == "" && snapshot.Review != nil && snapshot.Review.Verdict == "approved" {
			acceptance, err := integration.Offer(integration.Config{Repository: req.Repository, Root: req.Root})
			if err != nil {
				return outcome{Snapshot: snapshot}, err
			}
			fresh, err := store.Status()
			if err != nil {
				return outcome{}, err
			}
			return acceptanceResult(store, fresh, acceptance, "Reviewed candidate is ready for exact operator acceptance.")
		}
		if snapshot.Active && snapshot.Dispatch != nil && snapshot.Dispatch.Pending != nil && !snapshot.Dispatch.Pending.Authorized {
			return outcome{Disposition: "authorization_required", Snapshot: snapshot}, nil
		}
		if snapshot.Active && snapshot.Question != nil && snapshot.Question.AnswerMode == "" {
			return outcome{Disposition: "decision_required", Snapshot: snapshot}, nil
		}
		if snapshot.Active && (snapshot.Hold == "limit-exhausted" || snapshot.Hold == "operator-decision") {
			return outcome{Disposition: "decision_required", Snapshot: snapshot}, nil
		}
	}
	return l.agentOperation(ctx, req, store, snapshot, onEvent)
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
func hasPolicyChoices(r request) bool {
	return r.ReagentModel != "" || r.IragentModel != "" || r.ReagentEffort != "" || r.IragentEffort != "" || r.Gate != ""
}

func (l liveOperations) approve(ctx context.Context, req request, store *activework.Store, snapshot activework.Snapshot, onEvent func(codexapp.Event)) (outcome, error) {
	id := req.Approve
	if id == "" && snapshot.Acceptance != nil && snapshot.Integration != nil {
		id = snapshot.Acceptance.ID
	}
	if id == "" {
		return outcome{Snapshot: snapshot}, fmt.Errorf("no accepted integration intent")
	}
	cfg := integration.Config{Repository: req.Repository, Root: req.Root}
	if snapshot.Integration != nil {
		checked, checkErr := integration.Apply(ctx, cfg, id)
		if checkErr == nil && checked.Status == "integrated" {
			return complete(store)
		}
		if checked.Status == "conflicting" {
			fresh, _ := store.Status()
			return outcome{Disposition: checked.Status, Snapshot: fresh, Summary: checked.Detail}, checkErr
		}
	}
	if snapshot.Integration == nil || snapshot.Integration.Status != "integrated" {
		process, err := l.open(ctx, req.Codex)
		if err != nil {
			return outcome{Snapshot: snapshot}, err
		}
		defer process.Close()
		if err := process.PrepareIdle(ctx, snapshot); err != nil {
			return outcome{Snapshot: snapshot}, err
		}
		cfg.Reagent, cfg.Reviewer = process.Agent(), process.Reviewer()
	}
	integrated, err := integration.Apply(ctx, cfg, id)
	fresh, statusErr := store.Status()
	if statusErr == nil {
		snapshot = fresh
	}
	if err != nil {
		return outcome{Disposition: integrated.Status, Summary: integrated.Detail, Snapshot: snapshot}, err
	}
	if integrated.Status != "integrated" {
		return outcome{Disposition: integrated.Status, Summary: integrated.Detail, Snapshot: snapshot}, fmt.Errorf("integration did not reach the accepted target")
	}
	return complete(store)
}

func complete(store *activework.Store) (outcome, error) {
	result, err := store.Complete(nil)
	snapshot, statusErr := store.Status()
	if statusErr != nil && err == nil {
		err = statusErr
	}
	return outcome{Disposition: result.Status, Snapshot: snapshot, Completion: &result, Summary: fmt.Sprintf("Integrated %s on %s; external archive %s.", result.Facts.Candidate, result.Facts.TargetBranch, result.Archive)}, err
}

func (l liveOperations) agentOperation(ctx context.Context, req request, store *activework.Store, snapshot activework.Snapshot, onEvent func(codexapp.Event)) (outcome, error) {
	process, err := l.open(ctx, req.Codex)
	if err != nil {
		return outcome{Snapshot: snapshot}, err
	}
	defer process.Close()
	if req.Defer != "" || req.Abandon != "" {
		if err := process.PrepareIdle(ctx, snapshot); err != nil {
			return outcome{Snapshot: snapshot}, err
		}
	}
	cfg := selection.Config{Repository: req.Repository, Root: req.Root, Agent: process.Agent(), Reviewer: process.Reviewer(), OnEvent: onEvent}
	if !snapshot.Active && req.Resume == "" {
		models, err := process.Models(ctx)
		if err != nil {
			return outcome{}, err
		}
		profiles, err := dispatch.ResolveProfiles(models, dispatch.ProfileSources{Invocation: map[dispatch.Role]dispatch.Choice{
			dispatch.Reagent: {Model: req.ReagentModel, Effort: req.ReagentEffort},
			dispatch.Iragent: {Model: req.IragentModel, Effort: req.IragentEffort},
		}})
		if err != nil {
			return outcome{}, err
		}
		gates, err := parseGates(req.Gate)
		if err != nil {
			return outcome{}, err
		}
		cfg.Policy, err = dispatch.NewState(profiles, dispatch.DefaultLimits(), gates, dispatch.QuotaPolicy{})
		if err != nil {
			return outcome{}, err
		}
		if observation := readQuota(ctx, process); observation != nil {
			cfg.Policy.Quota = observation
		}
	} else if snapshot.Active {
		if observation := readQuota(ctx, process); observation != nil {
			recordQuota(store, *observation)
		}
	}
	var result selection.Result
	switch {
	case req.Answer != "":
		result, err = selection.Answer(ctx, cfg, selection.AnswerInput{QuestionID: req.Answer, ChoiceID: req.Choice, Text: req.Text, File: req.AnswerFile})
	case req.Defer != "":
		result, err = selection.Defer(ctx, cfg, req.Defer, req.Reason)
	case req.Abandon != "":
		result, err = selection.Abandon(ctx, cfg, req.Abandon, req.Reason)
	case req.Resume != "":
		result, err = selection.ResumeDeferred(ctx, cfg, req.Resume)
	case snapshot.Active:
		result, err = selection.Continue(ctx, cfg)
	default:
		result, err = selection.Execute(ctx, cfg)
	}
	fresh, statusErr := store.Status()
	if observation := readQuota(ctx, process); observation != nil {
		recordQuota(store, *observation)
		fresh, statusErr = store.Status()
	}
	if statusErr == nil {
		result.Snapshot = fresh
	}
	if err != nil {
		return outcome{Disposition: normalize(result.Status), Snapshot: result.Snapshot, Summary: result.Summary}, err
	}
	if result.Status == "approved" || result.Status == "ready_for_acceptance" {
		acceptance, err := integration.Offer(integration.Config{Repository: req.Repository, Root: req.Root})
		if err != nil {
			return outcome{Snapshot: result.Snapshot}, err
		}
		fresh, err := store.Status()
		if err != nil {
			return outcome{}, err
		}
		return acceptanceResult(store, fresh, acceptance, result.Summary)
	}
	returned := outcome{Disposition: normalize(result.Status), Snapshot: result.Snapshot, Summary: result.Summary}
	if result.Status == "deferred" {
		returned.Next = "concoct run --resume " + req.Defer
	}
	return returned, nil
}

func acceptanceResult(store *activework.Store, snapshot activework.Snapshot, acceptance activework.Acceptance, summary string) (outcome, error) {
	materials, err := approvalMaterials(store, snapshot)
	if err != nil {
		return outcome{Snapshot: snapshot}, err
	}
	return outcome{Disposition: "ready_for_acceptance", Snapshot: snapshot, Acceptance: &acceptance, Summary: summary, Approval: materials}, err
}

func approvalMaterials(store *activework.Store, snapshot activework.Snapshot) (*approvalView, error) {
	inspection, err := store.InspectAcceptance(snapshot)
	if err != nil {
		return nil, err
	}
	var checkpoint selection.ReadyCheckpoint
	if err := json.Unmarshal([]byte(inspection.Checkpoint), &checkpoint); err != nil {
		return nil, fmt.Errorf("candidate checkpoint: %w", err)
	}
	base, candidate := snapshot.ParentCommit, snapshot.Candidate.Commit
	git := "git -C " + shellQuote(store.Checkout) + " diff --no-ext-diff --binary " + base + " " + candidate + " --"
	var reconciled []string
	for _, path := range []string{"roadmap.md", "capabilities.md"} {
		for _, changed := range snapshot.Candidate.ChangedPaths {
			if changed == path {
				reconciled = append(reconciled, path)
			}
		}
	}
	return &approvalView{DiffCommand: git, ReconciliationCommand: git + " roadmap.md capabilities.md", TaskEvidence: inspection.TaskPath, ReviewEvidence: inspection.ReviewPath, Progress: checkpoint.Progress, SelfReview: checkpoint.SelfReview, Criteria: checkpoint.Criteria, Checks: checkpoint.Checks, Assessment: inspection.Assessment, ReconciledPaths: reconciled}, nil
}

func readQuota(ctx context.Context, process provider) *dispatch.QuotaObservation {
	raw, err := process.RateLimits(ctx)
	if err != nil {
		return nil
	}
	observation, err := dispatch.ParseQuota(raw, time.Now().UTC())
	if err != nil {
		return nil
	}
	return &observation
}

func recordQuota(store *activework.Store, observation dispatch.QuotaObservation) {
	snapshot, err := store.Status()
	if err != nil || !snapshot.Active || snapshot.Dispatch == nil {
		return
	}
	owner, err := store.Acquire()
	if err != nil {
		return
	}
	defer owner.Close()
	_ = owner.ObserveQuota(observation, time.Now().UTC())
}

func parseGates(value string) (dispatch.GatePolicy, error) {
	gates := dispatch.GatePolicy{Activities: map[dispatch.Activity]bool{}}
	if value == "" {
		return gates, nil
	}
	for _, part := range strings.Split(value, ",") {
		activity := dispatch.Activity(part)
		switch activity {
		case dispatch.Selection, dispatch.Engineering, dispatch.Review, dispatch.Correction, dispatch.Recovery:
			gates.Activities[activity] = true
		default:
			return dispatch.GatePolicy{}, fmt.Errorf("unsupported gate activity %q", part)
		}
	}
	return gates, nil
}

func disposition(s activework.Snapshot) string {
	if s.PendingStart {
		return "recovery_required"
	}
	if !s.Active {
		return "idle"
	}
	if s.Completion != nil {
		return "maintenance_pending"
	}
	if s.Integration != nil {
		if s.Integration.Status == "integrated" {
			return "maintenance_pending"
		}
		return "in_progress"
	}
	if s.Stop != nil {
		if s.Stop.Status == "stopped_safely" {
			return "stopped_safely"
		}
		return "stopped_abnormally"
	}
	if s.Hold == "recovery-required" {
		return "recovery_required"
	}
	if s.Dispatch != nil && s.Dispatch.Pending != nil && !s.Dispatch.Pending.Authorized {
		return "authorization_required"
	}
	if s.Question != nil && s.Question.AnswerMode == "" || s.Hold == "operator-decision" || s.Hold == "limit-exhausted" {
		return "decision_required"
	}
	if s.Review != nil && s.Review.Verdict == "approved" {
		return "ready_for_acceptance"
	}
	if s.Phase == "ready-for-review" {
		return "ready_for_independent_review"
	}
	return "in_progress"
}

func normalize(value string) string {
	switch value {
	case "approved":
		return "ready_for_acceptance"
	case "ready_for_review":
		return "ready_for_independent_review"
	case "engineering", "changes_requested", "active":
		return "in_progress"
	case "integrated":
		return "maintenance_pending"
	case "deferred", "abandoned", "decision_required", "authorization_required", "no_suitable_work", "ready_for_acceptance", "stopped_safely", "stopped_abnormally", "recovery_required", "maintenance_pending":
		return value
	default:
		return value
	}
}
