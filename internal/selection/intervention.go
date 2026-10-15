package selection

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/review"
)

type AnswerInput struct {
	QuestionID string
	ChoiceID   string
	Text       string
	File       string
}

// Answer persists one exact answer under the execution lock before any role
// is resumed. Failure after that write leaves the answer available on retry.
func Answer(ctx context.Context, cfg Config, input AnswerInput) (Result, error) {
	store, err := activework.Open(cfg.Repository, cfg.Root)
	if err != nil {
		return Result{}, err
	}
	owner, err := store.Acquire()
	if err != nil {
		return Result{}, err
	}
	mode, value := "", ""
	for _, candidate := range []struct{ mode, value string }{{"choice", input.ChoiceID}, {"text", input.Text}, {"file", input.File}} {
		if candidate.value != "" {
			if mode != "" {
				owner.Close()
				return Result{}, fmt.Errorf("provide exactly one answer form")
			}
			mode, value = candidate.mode, candidate.value
		}
	}
	if mode == "file" {
		body, readErr := os.ReadFile(value)
		if readErr != nil {
			owner.Close()
			return Result{}, readErr
		}
		value = string(body)
	}
	if err := owner.AnswerQuestion(input.QuestionID, mode, value); err != nil {
		owner.Close()
		return Result{}, err
	}
	if err := owner.Close(); err != nil {
		return Result{}, err
	}
	return Continue(ctx, cfg)
}

func routeAnswered(ctx context.Context, cfg Config, owner *activework.Owner, store *activework.Store, result Result, snapshot activework.Snapshot) (Result, error) {
	q := snapshot.Question
	if q == nil || q.AnswerMode == "" || snapshot.Hold != "" {
		return result, fmt.Errorf("no answered question is ready to route")
	}
	var err error
	switch q.Requester {
	case "reagent":
		result, err = runReagentContinuation(ctx, cfg, owner, store, result, "", false)
	case "iragent":
		reviewer := reviewerFor(cfg)
		if reviewer == nil {
			return holdContinuation(owner, store, result, "recovery-required", "requesting reviewer provider is unavailable")
		}
		if q.SourceChange {
			if snapshot.Review == nil {
				return holdContinuation(owner, store, result, "recovery-required", "source-change answer lost review binding")
			}
			if snapshot.Review.Verdict == "" {
				if _, err := review.RouteAnsweredChange(ctx, reviewer, owner, store, q.ID); err != nil {
					return holdContinuation(owner, store, result, "recovery-required", "answered reviewer change cannot be routed: "+err.Error())
				}
			}
			latest, err := store.Status()
			if err != nil {
				return result, err
			}
			if latest.Review == nil || latest.Review.Verdict != "changes_requested" {
				return holdContinuation(owner, store, result, "recovery-required", "source-changing review did not retain findings")
			}
			result, err = correctionLoop(ctx, cfg, owner, store, reviewer, Result{Status: "changes_requested", RunID: latest.RunID, TaskID: latest.TaskID, Workspace: result.Workspace, Snapshot: latest})
		} else {
			if snapshot.Review == nil || snapshot.Review.Verdict != "" {
				return holdContinuation(owner, store, result, "recovery-required", "requesting review is no longer pending")
			}
			if snapshot.Review.AttemptID == q.OriginAttemptID {
				nextID, err := randomID()
				if err != nil {
					return result, err
				}
				if err := owner.RenewAnsweredReviewAttempt(q.ID, nextID); err != nil {
					return result, err
				}
			}
			assessment, reviewErr := review.Run(ctx, review.Config{Agent: reviewer, OnEvent: cfg.OnEvent, StopTrigger: cfg.StopTrigger}, owner, store)
			result.Status, result.Snapshot, err = assessment.Status, assessment.Snapshot, reviewErr
			if err == nil && result.Status == "changes_requested" {
				result, err = correctionLoop(ctx, cfg, owner, store, reviewer, result)
			}
		}
	default:
		return result, fmt.Errorf("question has unknown requester")
	}
	if err == nil && result.Status != "recovery_required" && result.Status != "decision_required" && result.Status != "stopped_abnormally" && result.Status != "authorization_required" {
		if err := owner.ClearAnsweredQuestion(q.ID); err != nil {
			return result, err
		}
		result.Snapshot, err = store.Status()
	}
	return result, err
}

func retire(ctx context.Context, cfg Config, taskID, kind, reason string) (Result, error) {
	store, err := activework.Open(cfg.Repository, cfg.Root)
	if err != nil {
		return Result{}, err
	}
	owner, err := store.Acquire()
	if err != nil {
		return Result{}, err
	}
	defer owner.Close()
	snapshot, err := store.Status()
	if err != nil {
		return Result{}, err
	}
	if !snapshot.Active || snapshot.TaskID != taskID {
		return Result{}, fmt.Errorf("task is not the active bound work")
	}
	if snapshot.Stop != nil || snapshot.Dispatch == nil {
		return Result{}, fmt.Errorf("execution has unresolved stop or dispatch state")
	}
	for _, attempt := range snapshot.Dispatch.Attempts {
		if attempt.Status == "reserved" || attempt.Status == "uncertain" {
			return Result{}, fmt.Errorf("execution attempt %s is unresolved", attempt.ID)
		}
	}
	for _, role := range []string{"reagent", "iragent"} {
		threadID := snapshot.RoleIDs[role]
		if threadID == "" {
			continue
		}
		if role == "reagent" {
			if cfg.Agent == nil {
				return Result{}, fmt.Errorf("bound reagent is unavailable for idle verification")
			}
			if err := cfg.Agent.Idle(ctx, threadID); err != nil {
				return Result{}, fmt.Errorf("reagent is not idle: %w", err)
			}
		} else {
			reviewer := reviewerFor(cfg)
			if reviewer == nil {
				return Result{}, fmt.Errorf("bound reviewer is unavailable for idle verification")
			}
			if err := reviewer.Idle(ctx, threadID); err != nil {
				return Result{}, fmt.Errorf("reviewer is not idle: %w", err)
			}
		}
	}
	if _, err := owner.Retire(taskID, kind, reason); err != nil {
		return Result{}, err
	}
	return Result{Status: kind, RunID: snapshot.RunID, TaskID: taskID, Workspace: filepath.Join(store.Root, "workspaces", snapshot.RunID)}, nil
}

func Defer(ctx context.Context, cfg Config, taskID, reason string) (Result, error) {
	return retire(ctx, cfg, taskID, "deferred", reason)
}

func Abandon(ctx context.Context, cfg Config, taskID, reason string) (Result, error) {
	return retire(ctx, cfg, taskID, "abandoned", reason)
}

func ResumeDeferred(ctx context.Context, cfg Config, taskID string) (Result, error) {
	store, err := activework.Open(cfg.Repository, cfg.Root)
	if err != nil {
		return Result{}, err
	}
	owner, err := store.Acquire()
	if err != nil {
		return Result{}, err
	}
	if _, err := owner.Reactivate(taskID); err != nil {
		owner.Close()
		return Result{}, err
	}
	if err := owner.Close(); err != nil {
		return Result{}, err
	}
	return Continue(ctx, cfg)
}
