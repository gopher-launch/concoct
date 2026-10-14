package dispatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

func catalog() []codexapp.Model {
	models := []codexapp.Model{
		{Model: "recommended", IsDefault: true},
		{Model: "older"},
	}
	for i := range models {
		for _, effort := range []string{"low", "medium", "high"} {
			models[i].SupportedReasoningEfforts = append(models[i].SupportedReasoningEfforts, struct {
				ReasoningEffort string `json:"reasoningEffort"`
			}{ReasoningEffort: effort})
		}
	}
	return models
}

func profiles(t *testing.T) map[Role]codexapp.RoleSettings {
	t.Helper()
	resolved, err := ResolveProfiles(catalog(), ProfileSources{})
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func policy(t *testing.T, gates GatePolicy, quota QuotaPolicy) State {
	t.Helper()
	state, err := NewState(profiles(t), DefaultLimits(), gates, quota)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestProfilePrecedenceAndUnsupportedPairs(t *testing.T) {
	defaults := profiles(t)
	if defaults[Reagent].Model != "recommended" || defaults[Reagent].Effort != "medium" || defaults[Iragent].Effort != "high" || defaults[Reagent].Sandbox != "danger-full-access" {
		t.Fatalf("role defaults: %#v", defaults)
	}
	resolved, err := ResolveProfiles(catalog(), ProfileSources{
		User:       map[Role]Choice{Reagent: {Model: "older", Effort: "low"}},
		Project:    map[Role]Choice{Reagent: {Effort: "high"}},
		Invocation: map[Role]Choice{Reagent: {Model: "recommended"}},
	})
	if err != nil || resolved[Reagent].Model != "recommended" || resolved[Reagent].Effort != "high" {
		t.Fatalf("precedence: %#v, %v", resolved, err)
	}
	if _, err := ResolveProfiles(catalog(), ProfileSources{Invocation: map[Role]Choice{Iragent: {Effort: "ultra"}}}); err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("unsupported effort: %v", err)
	}
	if _, err := ResolveProfiles(nil, ProfileSources{}); err == nil {
		t.Fatal("accepted missing provider recommendation")
	}
}

func TestDispatchRoutesGatesAndNoReplay(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	s := policy(t, GatePolicy{Activities: map[Activity]bool{Review: true}}, QuotaPolicy{})
	routes := []Binding{
		{Role: Reagent, Activity: Selection},
		{TaskID: "CON-046", Role: Reagent, Activity: Engineering},
		{TaskID: "CON-046", Role: Iragent, Activity: Review, Candidate: "candidate-a"},
		{TaskID: "CON-046", Role: Reagent, Activity: Correction, Candidate: "candidate-a"},
	}
	for i, binding := range routes {
		id := fmt.Sprintf("turn-%d", i)
		decision, err := s.Reserve(id, binding, now)
		if err != nil {
			t.Fatal(err)
		}
		if binding.Activity == Review {
			if decision.Hold == nil || decision.Dispatch || decision.Hold.Binding.Candidate != "candidate-a" {
				t.Fatalf("review gate: %#v", decision)
			}
			if retry, err := s.Reserve(id, binding, now); err != nil || retry.Hold == nil {
				t.Fatalf("plain rerun bypassed gate: %#v, %v", retry, err)
			}
			if changed, err := s.Reserve("other", Binding{TaskID: "CON-046", Role: Iragent, Activity: Review, Candidate: "candidate-b"}, now); err != nil || changed.Hold == nil || changed.Hold.ID != id {
				t.Fatalf("candidate change bypassed gate: %#v, %v", changed, err)
			}
			if err := s.Authorize("wrong"); err == nil {
				t.Fatal("wrong grant accepted")
			}
			if err := s.Authorize(id); err != nil {
				t.Fatal(err)
			}
			decision, err = s.Reserve(id, binding, now)
			if err != nil {
				t.Fatal(err)
			}
		}
		if !decision.Dispatch || decision.Attempt == nil || decision.Attempt.Binding != binding {
			t.Fatalf("dispatch route %s: %#v", binding.Activity, decision)
		}
		if i == 0 {
			if _, err := s.Reserve("next", routes[1], now); !errors.Is(err, ErrUnresolvedAttempt) {
				t.Fatalf("unresolved attempt: %v", err)
			}
		}
		if err := s.Finish(id, "completed", nil); err != nil {
			t.Fatal(err)
		}
		if retry, err := s.Reserve(id, binding, now); err != nil || retry.Dispatch {
			t.Fatalf("completed dispatch was replayable: %#v, %v", retry, err)
		}
	}
	if len(s.Attempts) != 4 {
		t.Fatalf("consumption reset or duplicated: %d", len(s.Attempts))
	}
	if err := s.RecordAssessment("review-1", "turn-2"); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordAssessment("review-2", "turn-2"); err == nil {
		t.Fatal("one completed review counted twice")
	}
}

func TestLimitsRecoveryAndUnknownTurn(t *testing.T) {
	now := time.Now().UTC()
	unclaimed := policy(t, GatePolicy{}, QuotaPolicy{})
	if decision, err := unclaimed.Reserve("selection-recovery", Binding{Role: Reagent, Activity: Recovery, Episode: "selection-outage"}, now); err != nil || !decision.Dispatch {
		t.Fatalf("unclaimed selection recovery: %#v, %v", decision, err)
	}
	s := policy(t, GatePolicy{}, QuotaPolicy{})
	if s.Limits != (Limits{12, 3, 3, 2}) || s.WatchdogElapsed != 0 || s.WatchdogStall != 0 {
		t.Fatalf("starting policy: %#v", s)
	}
	if err := s.Extend(Limits{Dispatches: 13, SelfReviews: 4, Assessments: 4, Recovery: 3}); err != nil {
		t.Fatal(err)
	}
	if err := s.Extend(DefaultLimits()); err == nil {
		t.Fatal("limit contraction accepted")
	}
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("recovery-%d", i)
		binding := Binding{TaskID: "CON-046", Role: Reagent, Activity: Recovery, Episode: "outage-1"}
		decision, err := s.Reserve(id, binding, now)
		if err != nil || !decision.Dispatch {
			t.Fatalf("recovery reserve: %#v, %v", decision, err)
		}
		if i == 0 {
			if err := s.Finish(id, "uncertain", nil); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Reserve("premature", binding, now); !errors.Is(err, ErrUnresolvedAttempt) {
				t.Fatalf("uncertain dispatch replay: %v", err)
			}
			if err := s.Reconcile(id, "provider metadata and workspace inspected"); err != nil {
				t.Fatal(err)
			}
		} else if err := s.Finish(id, "failed", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Reserve("excess-recovery", Binding{TaskID: "CON-046", Role: Reagent, Activity: Recovery, Episode: "outage-1"}, now); err == nil {
		t.Fatal("recovery episode limit bypassed")
	}
	if decision, err := s.Reserve("new-episode", Binding{TaskID: "CON-046", Role: Reagent, Activity: Recovery, Episode: "outage-2"}, now); err != nil || !decision.Dispatch || len(s.Attempts) != 4 {
		t.Fatalf("new episode reset total consumption: %#v, %v", decision, err)
	}
	if err := s.Finish("new-episode", "completed", Receipt(&codexapp.Usage{InputTokens: 2, TotalTokens: 3})); err != nil || s.Attempts[3].Tokens.TotalTokens != 3 {
		t.Fatalf("token receipt: %v, %#v", err, s.Attempts[3].Tokens)
	}
	short, err := ResolveLimits(Limits{Dispatches: 2}, Limits{Recovery: 1})
	if err != nil || short.Dispatches != 2 || short.Recovery != 1 || short.Assessments != 3 {
		t.Fatalf("limit resolution: %#v, %v", short, err)
	}
}

func TestDefaultGrantAndExhaustedBounds(t *testing.T) {
	now := time.Now().UTC()
	s := policy(t, GatePolicy{}, QuotaPolicy{})
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("review-turn-%d", i)
		b := Binding{TaskID: "CON-046", Role: Iragent, Activity: Review, Candidate: fmt.Sprintf("candidate-%d", i)}
		decision, err := s.Reserve(id, b, now)
		if err != nil || !decision.Dispatch || decision.Hold != nil {
			t.Fatalf("default iragent gate at review %d: %#v, %v", i, decision, err)
		}
		if err := s.Finish(id, "completed", nil); err != nil {
			t.Fatal(err)
		}
		if err := s.RecordAssessment(fmt.Sprintf("assessment-%d", i), id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Reserve("fourth-review", Binding{TaskID: "CON-046", Role: Iragent, Activity: Review, Candidate: "candidate-4"}, now); err == nil || !strings.Contains(err.Error(), "assessment allowance") {
		t.Fatalf("fourth completed assessment allowed: %v", err)
	}
	for i := 3; i < 12; i++ {
		id := fmt.Sprintf("engineering-%d", i)
		decision, err := s.Reserve(id, Binding{TaskID: "CON-046", Role: Reagent, Activity: Engineering}, now)
		if err != nil || !decision.Dispatch {
			t.Fatalf("dispatch %d: %#v, %v", i, decision, err)
		}
		if err := s.Finish(id, "failed", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Reserve("thirteenth", Binding{TaskID: "CON-046", Role: Reagent, Activity: Engineering}, now); err == nil || !strings.Contains(err.Error(), "dispatch allowance") {
		t.Fatalf("thirteenth dispatch allowed: %v", err)
	}
	if err := s.Extend(Limits{Dispatches: 13, SelfReviews: 3, Assessments: 3, Recovery: 2}); err != nil {
		t.Fatal(err)
	}
	if decision, err := s.Reserve("thirteenth", Binding{TaskID: "CON-046", Role: Reagent, Activity: Engineering}, now); err != nil || !decision.Dispatch || len(s.Attempts) != 13 {
		t.Fatalf("extension reset consumption: %#v, %v", decision, err)
	}
}

func TestQuotaWindowsStalenessAndAuthorization(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	raw := json.RawMessage(`{"rateLimits":{"primary":{"usedPercent":99,"resetsAt":1792000000}},"rateLimitsByLimitId":{"codex":{"primary":{"usedPercent":82,"resetsAt":1792000000},"secondary":{"usedPercent":35,"resetsAt":1793000000}}}}`)
	quota, err := ParseQuota(raw, now)
	if err != nil || len(quota.Windows) != 2 || quota.Windows["codex/primary"].UsedPercent != 82 || quota.Windows["codex/secondary"].ResetAt.IsZero() {
		t.Fatalf("quota parsing: %#v, %v", quota, err)
	}
	if age, ok := quota.Age(now.Add(2 * time.Minute)); !ok || age != 2*time.Minute {
		t.Fatalf("quota observation age: %v, %v", age, ok)
	}
	s := policy(t, GatePolicy{}, QuotaPolicy{WarnUsedPercent: 80, HoldUsedPercent: 90, MaxAge: time.Minute})
	if err := s.ObserveQuota(quota, now); err != nil {
		t.Fatal(err)
	}
	b := Binding{Role: Reagent, Activity: Selection}
	decision, err := s.Reserve("warn", b, now)
	if err != nil || !decision.Dispatch || !strings.Contains(decision.Warning, "warning") {
		t.Fatalf("quota warning: %#v, %v", decision, err)
	}
	if err := s.Finish("warn", "completed", nil); err != nil {
		t.Fatal(err)
	}
	quota.Windows["codex/primary"] = Window{UsedPercent: 95, ResetAt: quota.Windows["codex/primary"].ResetAt}
	if err := s.ObserveQuota(quota, now); err != nil {
		t.Fatal(err)
	}
	decision, err = s.Reserve("quota-hold", b, now)
	if err != nil || decision.Hold == nil || decision.Dispatch {
		t.Fatalf("quota hold: %#v, %v", decision, err)
	}
	if err := s.Authorize("quota-hold"); err != nil {
		t.Fatal(err)
	}
	decision, err = s.Reserve("quota-hold", b, now)
	if err != nil || !decision.Dispatch {
		t.Fatalf("quota override: %#v, %v", decision, err)
	}
	if err := s.Finish("quota-hold", "completed", nil); err != nil {
		t.Fatal(err)
	}
	unknown := policy(t, GatePolicy{}, QuotaPolicy{StrictUnknown: true, MaxAge: time.Minute})
	if decision, err := unknown.Reserve("unknown", b, now); err != nil || decision.Hold == nil {
		t.Fatalf("strict unknown fallback: %#v, %v", decision, err)
	}
	if err := unknown.ObserveQuota(quota, now.Add(-2*time.Minute)); err == nil {
		t.Fatal("future-dated observation accepted")
	}
	if err := unknown.ObserveQuota(quota, now); err != nil {
		t.Fatal(err)
	}
	if decision, err := unknown.Reserve("stale", b, now.Add(2*time.Minute)); err != nil || decision.Hold == nil || decision.Hold.ID != "unknown" {
		t.Fatalf("pending gate changed on stale telemetry: %#v, %v", decision, err)
	}
	stale := policy(t, GatePolicy{}, QuotaPolicy{StrictUnknown: true, MaxAge: time.Minute})
	if err := stale.ObserveQuota(quota, now); err != nil {
		t.Fatal(err)
	}
	if decision, err := stale.Reserve("stale", b, now.Add(2*time.Minute)); err != nil || decision.Hold == nil || !strings.Contains(decision.Hold.Reason, "stale") {
		t.Fatalf("stale observation allowed dispatch: %#v, %v", decision, err)
	}
	defaultUnknown := policy(t, GatePolicy{}, QuotaPolicy{})
	if decision, err := defaultUnknown.Reserve("fallback", b, now); err != nil || !decision.Dispatch || !strings.Contains(decision.Warning, "unknown") {
		t.Fatalf("default unknown fallback: %#v, %v", decision, err)
	}
	reset := policy(t, GatePolicy{}, QuotaPolicy{StrictUnknown: true})
	if err := reset.ObserveQuota(QuotaObservation{ObservedAt: now, Windows: map[string]Window{"primary": {UsedPercent: 95, ResetAt: now.Add(-time.Second)}}}, now); err != nil {
		t.Fatal(err)
	}
	if decision, err := reset.Reserve("reset", b, now); err != nil || decision.Hold == nil || !strings.Contains(decision.Hold.Reason, "reset") {
		t.Fatalf("reset window treated as current: %#v, %v", decision, err)
	}
}
