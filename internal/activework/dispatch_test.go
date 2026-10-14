package activework

import (
	"strings"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
)

func TestDispatchReservationAndGateSurviveOwnerRestart(t *testing.T) {
	store, repo := testStore(t)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Start(spec(t, repo)); err != nil {
		t.Fatal(err)
	}
	profiles := map[dispatch.Role]codexapp.RoleSettings{
		dispatch.Reagent: {Role: "reagent", Model: "recommended", Effort: "medium", Sandbox: "danger-full-access", ApprovalPolicy: "never"},
		dispatch.Iragent: {Role: "iragent", Model: "recommended", Effort: "high", Sandbox: "danger-full-access", ApprovalPolicy: "never"},
	}
	policy, err := dispatch.NewState(profiles, dispatch.DefaultLimits(), dispatch.GatePolicy{Activities: map[dispatch.Activity]bool{dispatch.Review: true}}, dispatch.QuotaPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.ConfigureDispatch(policy); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	engineering := dispatch.Binding{TaskID: "APP-001", Role: dispatch.Reagent, Activity: dispatch.Engineering}
	decision, err := owner.ReserveDispatch("engineering-1", engineering, now)
	if err != nil || !decision.Dispatch {
		t.Fatalf("engineering reserve: %#v, %v", decision, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Status()
	if err != nil || snapshot.Dispatch == nil || len(snapshot.Dispatch.Attempts) != 1 || snapshot.Dispatch.Attempts[0].Status != "reserved" {
		t.Fatalf("durable reservation: %#v, %v", snapshot.Dispatch, err)
	}
	owner, err = store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := owner.ReserveDispatch("engineering-2", engineering, now); err != dispatch.ErrUnresolvedAttempt {
		t.Fatalf("duplicate provider dispatch allowed: %v", err)
	}
	if err := owner.FinishDispatch("engineering-1", "completed", nil); err != nil {
		t.Fatal(err)
	}
	review := dispatch.Binding{TaskID: "APP-001", Role: dispatch.Iragent, Activity: dispatch.Review, Candidate: "abc123"}
	decision, err = owner.ReserveDispatch("review-1", review, now)
	if err != nil || decision.Hold == nil {
		t.Fatalf("review gate: %#v, %v", decision, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner, err = store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if retry, err := owner.ReserveDispatch("review-1", review, now); err != nil || retry.Hold == nil || retry.Dispatch {
		t.Fatalf("restart bypassed gate: %#v, %v", retry, err)
	}
	if err := owner.AuthorizeDispatch("review-1"); err != nil {
		t.Fatal(err)
	}
	decision, err = owner.ReserveDispatch("review-1", review, now)
	if err != nil || !decision.Dispatch {
		t.Fatalf("authorized review: %#v, %v", decision, err)
	}
	if err := owner.FinishDispatch("review-1", "completed", nil); err != nil {
		t.Fatal(err)
	}
	if err := owner.RecordAssessment("assessment-1", "review-1"); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Status()
	if err != nil || snapshot.Dispatch == nil || len(snapshot.Dispatch.Attempts) != 2 || len(snapshot.Dispatch.Assessments) != 1 || snapshot.Dispatch.Pending != nil || snapshot.Dispatch.Profiles[dispatch.Iragent].Effort != "high" {
		t.Fatalf("durable consumption/settings: %#v, %v", snapshot.Dispatch, err)
	}
	if _, err := owner.ReserveDispatch("other-task", dispatch.Binding{TaskID: "APP-002", Role: dispatch.Reagent, Activity: dispatch.Engineering}, now); err == nil {
		t.Fatal("wrong task binding accepted")
	}
}

func TestSelectionConsumptionTransfersOnClaim(t *testing.T) {
	store, repo := testStore(t)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	task := spec(t, repo)
	if _, err := owner.BeginSelection(SelectionSpec{ParentBranch: task.ParentBranch, ParentCommit: task.ParentCommit}); err != nil {
		t.Fatal(err)
	}
	policy, err := dispatch.NewState(map[dispatch.Role]codexapp.RoleSettings{
		dispatch.Reagent: {Role: "reagent", Model: "recommended", Effort: "medium", Sandbox: "danger-full-access", ApprovalPolicy: "never"},
		dispatch.Iragent: {Role: "iragent", Model: "recommended", Effort: "high", Sandbox: "danger-full-access", ApprovalPolicy: "never"},
	}, dispatch.DefaultLimits(), dispatch.GatePolicy{}, dispatch.QuotaPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.ConfigureDispatch(policy); err != nil {
		t.Fatal(err)
	}
	decision, err := owner.ReserveDispatch("selection-1", dispatch.Binding{Role: dispatch.Reagent, Activity: dispatch.Selection}, time.Now().UTC())
	if err != nil || !decision.Dispatch {
		t.Fatalf("selection reservation: %#v, %v", decision, err)
	}
	if err := owner.FinishDispatch("selection-1", "completed", nil); err != nil {
		t.Fatal(err)
	}
	if err := owner.Claim(task); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Status()
	if err != nil || snapshot.TaskID != "APP-001" || snapshot.Dispatch == nil || len(snapshot.Dispatch.Attempts) != 1 || snapshot.Dispatch.Attempts[0].Binding.Activity != dispatch.Selection {
		t.Fatalf("selection consumption lost on claim: %#v, %v", snapshot, err)
	}
}

func TestRecoveryAllowanceSurvivesEpisodeSwitchAndRestart(t *testing.T) {
	store, repo := testStore(t)
	owner, err := store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Start(spec(t, repo)); err != nil {
		t.Fatal(err)
	}
	policy, err := dispatch.NewState(map[dispatch.Role]codexapp.RoleSettings{
		dispatch.Reagent: {Role: "reagent", Model: "recommended", Effort: "medium", Sandbox: "danger-full-access", ApprovalPolicy: "never"},
		dispatch.Iragent: {Role: "iragent", Model: "recommended", Effort: "high", Sandbox: "danger-full-access", ApprovalPolicy: "never"},
	}, dispatch.DefaultLimits(), dispatch.GatePolicy{}, dispatch.QuotaPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.ConfigureDispatch(policy); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, step := range []struct{ id, episode string }{{"a-1", "A"}, {"a-2", "A"}, {"b-1", "B"}} {
		binding := dispatch.Binding{TaskID: "APP-001", Role: dispatch.Reagent, Activity: dispatch.Recovery, Episode: step.episode}
		decision, err := owner.ReserveDispatch(step.id, binding, now)
		if err != nil || !decision.Dispatch {
			t.Fatalf("reserve %s: %#v, %v", step.id, decision, err)
		}
		if err := owner.FinishDispatch(step.id, "failed", nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner, err = store.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	snapshot, err := store.Status()
	if err != nil || snapshot.Dispatch == nil || len(snapshot.Dispatch.Attempts) != 3 {
		t.Fatalf("reopened recovery history: %#v, %v", snapshot.Dispatch, err)
	}
	backToA := dispatch.Binding{TaskID: "APP-001", Role: dispatch.Reagent, Activity: dispatch.Recovery, Episode: "A"}
	forged, err := snapshot.Dispatch.Clone()
	if err != nil {
		t.Fatal(err)
	}
	forged.Attempts = append(forged.Attempts, dispatch.Attempt{ID: "a-3", Binding: backToA, ReservedAt: now, Status: "failed"})
	if err := forged.Validate(); err == nil || !strings.Contains(err.Error(), "recovery allowance exceeded") {
		t.Fatalf("persisted over-limit episode validated: %v", err)
	}
	if _, err := owner.ReserveDispatch("a-3", backToA, now); err == nil || !strings.Contains(err.Error(), "recovery allowance exhausted") {
		t.Fatalf("episode A allowance reset after B/restart: %v", err)
	}
	if err := owner.ExtendDispatchLimits(dispatch.Limits{Dispatches: 12, SelfReviews: 3, Assessments: 3, Recovery: 3}); err != nil {
		t.Fatal(err)
	}
	decision, err := owner.ReserveDispatch("a-3", backToA, now)
	if err != nil || !decision.Dispatch {
		t.Fatalf("extension did not retain earlier consumption: %#v, %v", decision, err)
	}
}
