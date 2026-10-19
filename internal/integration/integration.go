// Package integration performs local, exact-candidate delivery after an
// operator supplies the durable acceptance ID. Completion remains separate.
package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/review"
)

type IdleProvider interface {
	Idle(context.Context, string) error
}

type Config struct {
	Repository, Root  string
	Reagent, Reviewer IdleProvider
	// After is a deterministic interruption seam for local Git transition tests.
	After func(string) error
}

type Result struct {
	Status, Detail string
	Acceptance     activework.Acceptance
}

// Offer records a stable, candidate-bound ID for operator inspection. It does
// not approve or integrate the candidate.
func Offer(cfg Config) (activework.Acceptance, error) {
	store, err := activework.Open(cfg.Repository, cfg.Root)
	if err != nil {
		return activework.Acceptance{}, err
	}
	owner, err := store.Acquire()
	if err != nil {
		return activework.Acceptance{}, err
	}
	defer owner.Close()
	snapshot, err := store.Status()
	if err != nil {
		return activework.Acceptance{}, err
	}
	if snapshot.Integration == nil {
		if err := review.VerifyFrozen(store, snapshot); err != nil {
			return activework.Acceptance{}, err
		}
	}
	return owner.IssueAcceptance()
}

// Apply either finishes the exact accepted transition or reports what actual
// Git state shows. It never substitutes a candidate or rewrites source.
func Apply(ctx context.Context, cfg Config, id string) (Result, error) {
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
	if !snapshot.Active || snapshot.Acceptance == nil || snapshot.Acceptance.ID != id {
		return Result{}, fmt.Errorf("acceptance ID is missing or does not match the active run")
	}
	acceptance, err := owner.IssueAcceptance() // recheck scope and review evidence
	if err != nil {
		if snapshot.Integration != nil {
			state, detail := inspectTarget(store.Checkout, *snapshot.Acceptance)
			return Result{Status: classify(state), Detail: detail + "; acceptance evidence requires reconciliation", Acceptance: *snapshot.Acceptance}, err
		}
		return Result{Status: "not_integrated", Acceptance: *snapshot.Acceptance}, err
	}
	result := Result{Status: "not_integrated", Acceptance: acceptance}
	if snapshot.Integration == nil {
		if err := review.VerifyFrozen(store, snapshot); err != nil {
			result.Status = "not_integrated"
			return result, fmt.Errorf("candidate or checkout drift: %w", err)
		}
		state, detail := inspectTarget(store.Checkout, acceptance)
		if state != "baseline" {
			return Result{Status: classify(state), Detail: detail, Acceptance: acceptance}, fmt.Errorf("parent is not at the clean expected baseline: %s", detail)
		}
		if err := idle(ctx, cfg, snapshot); err != nil {
			return result, err
		}
		if _, err := owner.BeginIntegration(id); err != nil {
			return result, err
		}
		if err := after(cfg, "intent"); err != nil {
			return interrupted(store.Checkout, acceptance, err)
		}
	} else if snapshot.Integration.AcceptanceID != id {
		return result, fmt.Errorf("integration intent belongs to another acceptance")
	}
	// An intent survives every crash point. A fully integrated clean target is
	// conclusive even if the old task checkout later changes; no Git mutation
	// or provider connection is needed to record that success.
	snapshot, err = store.Status()
	if err != nil {
		return result, err
	}
	state, detail := inspectTarget(store.Checkout, acceptance)
	result.Status, result.Detail = classify(state), detail
	if state == "integrated" {
		if err := verifyAcceptedObject(store.Checkout, acceptance); err != nil {
			return Result{Status: "conflicting", Detail: err.Error(), Acceptance: acceptance}, err
		}
		if err := owner.MarkIntegrated(id); err != nil {
			return Result{Status: "integrated", Detail: "Git integration succeeded; record update pending", Acceptance: acceptance}, err
		}
		return Result{Status: "integrated", Detail: "exact candidate and clean target verified", Acceptance: acceptance}, nil
	}
	if state != "baseline" && state != "prepared" && state != "ref-advanced" {
		return Result{Status: "conflicting", Detail: detail, Acceptance: acceptance}, fmt.Errorf("integration state requires reconciliation: %s", detail)
	}
	if err := verifyCandidateSource(store, snapshot, acceptance); err != nil {
		return Result{Status: "conflicting", Detail: err.Error(), Acceptance: acceptance}, err
	}
	if err := idle(ctx, cfg, snapshot); err != nil {
		return result, err
	}
	switch state {
	case "baseline":
		if _, err := git(store.Checkout, "switch", "--no-overwrite-ignore", "--detach", acceptance.Candidate); err != nil {
			return interrupted(store.Checkout, acceptance, err)
		}
		if err := after(cfg, "checkout-prepared"); err != nil {
			return interrupted(store.Checkout, acceptance, err)
		}
		fallthrough
	case "prepared":
		state, detail = inspectTarget(store.Checkout, acceptance)
		if state != "prepared" {
			return Result{Status: classify(state), Detail: detail, Acceptance: acceptance}, fmt.Errorf("prepared checkout changed: %s", detail)
		}
		if _, err := git(store.Checkout, "update-ref", "refs/heads/"+acceptance.TargetBranch, acceptance.Candidate, acceptance.ExpectedParent); err != nil {
			return interrupted(store.Checkout, acceptance, err)
		}
		if err := after(cfg, "ref-advanced"); err != nil {
			return interrupted(store.Checkout, acceptance, err)
		}
		fallthrough
	case "ref-advanced":
		state, detail = inspectTarget(store.Checkout, acceptance)
		if state != "ref-advanced" {
			return Result{Status: classify(state), Detail: detail, Acceptance: acceptance}, fmt.Errorf("parent ref or checkout changed: %s", detail)
		}
		if _, err := git(store.Checkout, "switch", "--no-overwrite-ignore", acceptance.TargetBranch); err != nil {
			return interrupted(store.Checkout, acceptance, err)
		}
		if err := after(cfg, "checkout-attached"); err != nil {
			return interrupted(store.Checkout, acceptance, err)
		}
		fallthrough
	case "integrated":
		state, detail = inspectTarget(store.Checkout, acceptance)
		if state != "integrated" {
			return Result{Status: classify(state), Detail: detail, Acceptance: acceptance}, fmt.Errorf("integration cannot be confirmed: %s", detail)
		}
		if err := owner.MarkIntegrated(id); err != nil {
			return Result{Status: "integrated", Detail: "Git integration succeeded; record update pending", Acceptance: acceptance}, err
		}
		return Result{Status: "integrated", Detail: "exact candidate and clean target verified", Acceptance: acceptance}, nil
	default:
		return Result{Status: classify(state), Detail: detail, Acceptance: acceptance}, fmt.Errorf("integration state requires reconciliation: %s", detail)
	}
}

func idle(ctx context.Context, cfg Config, snapshot activework.Snapshot) error {
	if snapshot.Stop != nil || snapshot.Hold != "" || snapshot.FreezeIntent != nil || snapshot.Dispatch == nil || snapshot.Dispatch.Pending != nil {
		return fmt.Errorf("execution has an unresolved stop, hold, freeze or authorization")
	}
	for _, attempt := range snapshot.Dispatch.Attempts {
		if attempt.Status == "reserved" || attempt.Status == "uncertain" {
			return fmt.Errorf("execution attempt %s is unresolved", attempt.ID)
		}
	}
	for _, role := range []struct {
		name     string
		provider IdleProvider
	}{{"reagent", cfg.Reagent}, {"iragent", cfg.Reviewer}} {
		thread := snapshot.RoleIDs[role.name]
		if thread == "" || role.provider == nil {
			return fmt.Errorf("%s idle verification is unavailable", role.name)
		}
		if err := role.provider.Idle(ctx, thread); err != nil {
			return fmt.Errorf("%s is not idle: %w", role.name, err)
		}
	}
	return nil
}

func after(cfg Config, stage string) error {
	if cfg.After != nil {
		return cfg.After(stage)
	}
	return nil
}

func interrupted(checkout string, a activework.Acceptance, cause error) (Result, error) {
	state, detail := inspectTarget(checkout, a)
	return Result{Status: classify(state), Detail: detail, Acceptance: a}, cause
}

func classify(state string) string {
	switch state {
	case "baseline", "prepared":
		return "not_integrated"
	case "integrated":
		return "integrated"
	case "ref-advanced":
		return "uncertain"
	default:
		return "conflicting"
	}
}

// inspectTarget is read-only. A partially updated or externally changed
// checkout is conflicting; the controller does not overwrite it on retry.
func inspectTarget(checkout string, a activework.Acceptance) (string, string) {
	target, err := git(checkout, "rev-parse", "--verify", "refs/heads/"+a.TargetBranch)
	if err != nil {
		return "conflicting", err.Error()
	}
	branch, branchErr := git(checkout, "symbolic-ref", "--quiet", "--short", "HEAD")
	if branchErr != nil {
		branch = ""
	}
	head, err := git(checkout, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "conflicting", err.Error()
	}
	clean, err := git(checkout, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return "conflicting", err.Error()
	}
	if clean != "" {
		return "conflicting", "parent checkout has changes: " + clean
	}
	switch {
	case target == a.ExpectedParent && branch == a.TargetBranch && head == target:
		return "baseline", "target remains at expected baseline"
	case target == a.ExpectedParent && branch == "" && head == a.Candidate:
		return "prepared", "candidate tree prepared; target ref unchanged"
	case target == a.Candidate && branch == "" && head == a.Candidate:
		return "ref-advanced", "target ref advanced; checkout remains detached"
	case target == a.Candidate && branch == a.TargetBranch && head == a.Candidate:
		return "integrated", "target and checkout match accepted candidate"
	default:
		return "conflicting", fmt.Sprintf("target %s, checkout branch %q at %s", target, branch, head)
	}
}

func verifyCandidateSource(store *activework.Store, s activework.Snapshot, a activework.Acceptance) error {
	if s.Candidate == nil || s.Review == nil || s.Review.Verdict != "approved" || s.Candidate.Commit != a.Candidate || s.Candidate.Tree != a.CandidateTree || s.Review.Candidate != a.Candidate || s.Review.ScopeSHA256 != a.ScopeSHA256 {
		return fmt.Errorf("reviewed candidate or scope binding changed")
	}
	if err := verifyAcceptedObject(store.Checkout, a); err != nil {
		return err
	}
	branch, err := git(store.Checkout, "rev-parse", "refs/heads/"+s.TaskBranch)
	if err != nil || branch != a.Candidate {
		return fmt.Errorf("task branch moved from accepted candidate")
	}
	taskCheckout := filepath.Join(store.Root, "workspaces", s.RunID)
	if err := sameRepository(taskCheckout, store.CommonDir); err != nil {
		return fmt.Errorf("task checkout: %w", err)
	}
	if b, err := git(taskCheckout, "symbolic-ref", "--quiet", "--short", "HEAD"); err != nil || b != s.TaskBranch {
		return fmt.Errorf("task checkout left accepted branch")
	}
	if h, err := git(taskCheckout, "rev-parse", "HEAD"); err != nil || h != a.Candidate {
		return fmt.Errorf("task checkout left accepted commit")
	}
	if _, err := git(taskCheckout, "diff", "--quiet"); err != nil {
		return fmt.Errorf("task checkout source changed")
	}
	if _, err := git(taskCheckout, "diff", "--cached", "--quiet"); err != nil {
		return fmt.Errorf("task checkout index changed")
	}
	others, err := gitBytes(taskCheckout, "ls-files", "--others", "-z")
	if err != nil {
		return err
	}
	for _, path := range bytes.Split(others, []byte{0}) {
		if len(path) > 0 && !slices.Contains(s.Candidate.GeneratedOutputs, string(path)) {
			return fmt.Errorf("task checkout gained untracked source %q", path)
		}
	}
	reviewCheckout := filepath.Join(store.Root, "workspaces", s.RunID+"-review")
	if err := sameRepository(reviewCheckout, store.CommonDir); err != nil {
		return fmt.Errorf("review checkout: %w", err)
	}
	if b, err := git(reviewCheckout, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil || b != "" {
		return fmt.Errorf("review checkout is not detached")
	}
	if h, err := git(reviewCheckout, "rev-parse", "HEAD"); err != nil || h != a.Candidate {
		return fmt.Errorf("review checkout left accepted commit")
	}
	if clean, err := git(reviewCheckout, "status", "--porcelain=v1", "--untracked-files=all"); err != nil || clean != "" {
		return fmt.Errorf("review checkout changed: %s: %v", clean, err)
	}
	diff, err := gitBytes(store.Checkout, "diff", "--binary", "--no-ext-diff", "--no-renames", a.ExpectedParent, a.Candidate)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(diff)
	if hex.EncodeToString(sum[:]) != s.Candidate.DiffSHA256 {
		return fmt.Errorf("accepted candidate diff changed")
	}
	return nil
}

func verifyAcceptedObject(checkout string, a activework.Acceptance) error {
	tree, err := git(checkout, "rev-parse", a.Candidate+"^{tree}")
	if err != nil || tree != a.CandidateTree {
		return fmt.Errorf("accepted commit or tree identity changed")
	}
	return nil
}

func sameRepository(checkout, commonDir string) error {
	path, err := git(checkout, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if filepath.Clean(resolved) != commonDir {
		return fmt.Errorf("checkout belongs to another repository")
	}
	return nil
}

func git(checkout string, args ...string) (string, error) {
	output, err := gitBytes(checkout, args...)
	return strings.TrimSpace(string(output)), err
}
func gitBytes(checkout string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", checkout}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
