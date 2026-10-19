package dispatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
)

type Activity string

const (
	Selection   Activity = "selection"
	Engineering Activity = "engineering"
	Review      Activity = "review"
	Correction  Activity = "correction"
	Recovery    Activity = "recovery"
)

type Binding struct {
	TaskID    string   `json:"task_id"`
	Role      Role     `json:"role"`
	Activity  Activity `json:"activity"`
	Candidate string   `json:"candidate,omitempty"`
	Episode   string   `json:"episode,omitempty"`
}

func (b Binding) validate() error {
	if b.Role != Reagent && b.Role != Iragent {
		return fmt.Errorf("invalid dispatch role %q", b.Role)
	}
	switch b.Activity {
	case Selection:
		if b.Role != Reagent || b.TaskID != "" || b.Candidate != "" || b.Episode != "" {
			return fmt.Errorf("selection must be an unclaimed reagent dispatch")
		}
	case Engineering:
		if b.Role != Reagent || b.TaskID == "" || b.Candidate != "" || b.Episode != "" {
			return fmt.Errorf("invalid engineering binding")
		}
	case Review:
		if b.Role != Iragent || b.TaskID == "" || b.Candidate == "" || b.Episode != "" {
			return fmt.Errorf("review requires task and candidate binding")
		}
	case Correction:
		if b.Role != Reagent || b.TaskID == "" || b.Candidate == "" || b.Episode != "" {
			return fmt.Errorf("correction requires the reviewed candidate")
		}
	case Recovery:
		if b.Episode == "" || (b.TaskID == "" && (b.Role != Reagent || b.Candidate != "")) {
			return fmt.Errorf("recovery requires a failure episode and a claimed task or unclaimed reagent selection")
		}
	default:
		return fmt.Errorf("invalid dispatch activity %q", b.Activity)
	}
	if len(b.TaskID) > 64 || len(b.Candidate) > 128 || len(b.Episode) > 128 {
		return fmt.Errorf("dispatch binding exceeds size limit")
	}
	return nil
}

type Limits struct {
	Dispatches  int `json:"dispatches"`
	SelfReviews int `json:"self_reviews"` // in-turn instruction, not a dispatch counter
	Assessments int `json:"assessments"`
	Recovery    int `json:"recovery"`
}

func DefaultLimits() Limits {
	return Limits{Dispatches: 12, SelfReviews: 3, Assessments: 3, Recovery: 2}
}

// ResolveLimits applies explicit task values over project values over the
// starting policy. A zero override leaves the lower-priority value intact.
func ResolveLimits(project, task Limits) (Limits, error) {
	limits := DefaultLimits()
	for _, source := range []Limits{project, task} {
		if source.Dispatches != 0 {
			limits.Dispatches = source.Dispatches
		}
		if source.SelfReviews != 0 {
			limits.SelfReviews = source.SelfReviews
		}
		if source.Assessments != 0 {
			limits.Assessments = source.Assessments
		}
		if source.Recovery != 0 {
			limits.Recovery = source.Recovery
		}
	}
	return limits, limits.validate()
}

func (l Limits) validate() error {
	if l.Dispatches < 1 || l.SelfReviews < 1 || l.Assessments < 1 || l.Recovery < 1 || l.Dispatches > 1000 || l.Assessments > 1000 || l.Recovery > 1000 || l.SelfReviews > 1000 {
		return fmt.Errorf("invalid dispatch limits")
	}
	return nil
}

type GatePolicy struct {
	Activities map[Activity]bool `json:"activities,omitempty"`
}

func (g GatePolicy) requires(activity Activity) bool { return g.Activities[activity] }

type QuotaPolicy struct {
	WarnUsedPercent float64       `json:"warn_used_percent,omitempty"`
	HoldUsedPercent float64       `json:"hold_used_percent,omitempty"`
	MaxAge          time.Duration `json:"max_age,omitempty"`
	StrictUnknown   bool          `json:"strict_unknown,omitempty"`
}

func (p QuotaPolicy) validate() error {
	if math.IsNaN(p.WarnUsedPercent) || math.IsInf(p.WarnUsedPercent, 0) || math.IsNaN(p.HoldUsedPercent) || math.IsInf(p.HoldUsedPercent, 0) || p.WarnUsedPercent < 0 || p.WarnUsedPercent > 100 || p.HoldUsedPercent < 0 || p.HoldUsedPercent > 100 || p.MaxAge < 0 {
		return fmt.Errorf("invalid quota policy")
	}
	return nil
}

type Window struct {
	UsedPercent float64   `json:"used_percent"`
	ResetAt     time.Time `json:"reset_at"`
}

type QuotaObservation struct {
	ObservedAt time.Time         `json:"observed_at"`
	Windows    map[string]Window `json:"windows"`
}

// Age is computed when displayed; an observation never stores an age that
// would silently become stale after restart.
func (q QuotaObservation) Age(now time.Time) (time.Duration, bool) {
	if q.ObservedAt.IsZero() || now.Before(q.ObservedAt) {
		return 0, false
	}
	return now.Sub(q.ObservedAt), true
}

type TokenReceipt struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

func Receipt(usage *codexapp.Usage) *TokenReceipt {
	if usage == nil {
		return nil
	}
	return &TokenReceipt{usage.InputTokens, usage.CachedInputTokens, usage.OutputTokens, usage.ReasoningOutputTokens, usage.TotalTokens}
}

type Attempt struct {
	ID            string                      `json:"id"`
	Binding       Binding                     `json:"binding"`
	ThreadID      string                      `json:"thread_id,omitempty"`
	TurnID        string                      `json:"turn_id,omitempty"`
	TurnRequested bool                        `json:"turn_requested,omitempty"`
	FinalStatus   string                      `json:"final_status,omitempty"`
	FinalText     string                      `json:"final_text,omitempty"`
	Callbacks     []codexapp.CallbackEvidence `json:"callbacks,omitempty"`
	Applied       bool                        `json:"applied,omitempty"`
	ReservedAt    time.Time                   `json:"reserved_at"`
	Status        string                      `json:"status"` // reserved, uncertain, completed, failed, reconciled
	Tokens        *TokenReceipt               `json:"tokens,omitempty"`
	Resolution    string                      `json:"resolution,omitempty"`
}

// RecordCallbacks retains the transport evidence even when the provider turn
// completed but callback acknowledgment remained uncertain.
func (s *State) RecordCallbacks(id, threadID, turnID string, callbacks []codexapp.CallbackEvidence) error {
	if len(callbacks) > 64 || threadID == "" || turnID == "" {
		return fmt.Errorf("invalid callback evidence")
	}
	for i := range s.Attempts {
		a := &s.Attempts[i]
		if a.ID != id {
			continue
		}
		if a.ThreadID != threadID || (a.TurnID != "" && a.TurnID != turnID) {
			return fmt.Errorf("callback evidence differs from dispatch")
		}
		if len(a.Callbacks) != 0 && !reflect.DeepEqual(a.Callbacks, callbacks) {
			return fmt.Errorf("callback evidence changed")
		}
		a.Callbacks = append([]codexapp.CallbackEvidence(nil), callbacks...)
		return nil
	}
	return fmt.Errorf("unknown dispatch attempt %q", id)
}

type Authorization struct {
	ID         string    `json:"id"`
	Binding    Binding   `json:"binding"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
	Authorized bool      `json:"authorized"`
}

type Assessment struct {
	ID        string `json:"id"`
	AttemptID string `json:"attempt_id"`
	Candidate string `json:"candidate"`
}

type State struct {
	Profiles        map[Role]codexapp.RoleSettings `json:"profiles"`
	Limits          Limits                         `json:"limits"`
	Gates           GatePolicy                     `json:"gates"`
	QuotaPolicy     QuotaPolicy                    `json:"quota_policy"`
	Quota           *QuotaObservation              `json:"quota,omitempty"`
	Attempts        []Attempt                      `json:"attempts"`
	Assessments     []Assessment                   `json:"assessments"`
	Pending         *Authorization                 `json:"pending,omitempty"`
	WatchdogElapsed time.Duration                  `json:"watchdog_elapsed,omitempty"`
	WatchdogStall   time.Duration                  `json:"watchdog_stall,omitempty"`
}

func (s State) Clone() (State, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return State{}, err
	}
	var copy State
	err = json.Unmarshal(data, &copy)
	return copy, err
}

func (s State) Validate() error {
	if _, err := NewState(s.Profiles, s.Limits, s.Gates, s.QuotaPolicy); err != nil {
		return err
	}
	if s.WatchdogElapsed < 0 || s.WatchdogStall < 0 || len(s.Attempts) > s.Limits.Dispatches || len(s.Assessments) > s.Limits.Assessments {
		return fmt.Errorf("invalid persisted dispatch bounds")
	}
	ids := make(map[string]bool, len(s.Attempts))
	recoveryByEpisode := make(map[string]int)
	unresolved := 0
	for _, attempt := range s.Attempts {
		if attempt.ID == "" || ids[attempt.ID] || attempt.ReservedAt.IsZero() {
			return fmt.Errorf("invalid persisted dispatch attempt")
		}
		if len(attempt.ThreadID) > 256 || len(attempt.TurnID) > 256 || (attempt.TurnID != "" && attempt.ThreadID == "") || (attempt.TurnRequested && attempt.ThreadID == "") || len(attempt.FinalText) > 65536 || len(attempt.FinalStatus) > 32 {
			return fmt.Errorf("invalid persisted provider turn binding")
		}
		if len(attempt.Callbacks) > 64 {
			return fmt.Errorf("too many persisted callbacks")
		}
		for _, callback := range attempt.Callbacks {
			if callback.ThreadID != attempt.ThreadID || callback.TurnID != attempt.TurnID || callback.CallID == "" || callback.Tool == "" || len(callback.RequestID) > 256 || len(callback.Failure) > 1024 {
				return fmt.Errorf("invalid persisted callback binding")
			}
		}
		if attempt.Applied && (attempt.Status != "completed" || attempt.FinalStatus != "completed") {
			return fmt.Errorf("applied dispatch lacks completed result")
		}
		ids[attempt.ID] = true
		if err := attempt.Binding.validate(); err != nil {
			return err
		}
		if attempt.Binding.Activity == Recovery {
			recoveryByEpisode[attempt.Binding.Episode]++
			if recoveryByEpisode[attempt.Binding.Episode] > s.Limits.Recovery {
				return fmt.Errorf("recovery allowance exceeded for episode %q", attempt.Binding.Episode)
			}
		}
		if attempt.Tokens != nil && !attempt.Tokens.valid() {
			return fmt.Errorf("invalid persisted token receipt")
		}
		switch attempt.Status {
		case "reserved", "uncertain":
			unresolved++
		case "completed", "failed", "reconciled":
		default:
			return fmt.Errorf("invalid persisted dispatch status")
		}
	}
	if unresolved > 1 {
		return fmt.Errorf("multiple unresolved dispatches")
	}
	if unresolved > 0 && s.Pending != nil {
		return fmt.Errorf("authorization pending beside unresolved dispatch")
	}
	assessments := make(map[string]bool, len(s.Assessments))
	assessedAttempts := make(map[string]bool, len(s.Assessments))
	for _, assessment := range s.Assessments {
		if assessment.ID == "" || assessments[assessment.ID] || assessedAttempts[assessment.AttemptID] {
			return fmt.Errorf("duplicate or invalid assessment")
		}
		assessments[assessment.ID] = true
		assessedAttempts[assessment.AttemptID] = true
		found := false
		for _, attempt := range s.Attempts {
			if attempt.ID == assessment.AttemptID && (attempt.Binding.Activity == Review || (attempt.Binding.Activity == Recovery && attempt.Binding.Role == Iragent)) && attempt.Binding.Candidate == assessment.Candidate && attempt.Status == "completed" {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("assessment lacks completed review dispatch")
		}
	}
	if s.Pending != nil {
		if s.Pending.ID == "" || ids[s.Pending.ID] || s.Pending.CreatedAt.IsZero() {
			return fmt.Errorf("invalid pending authorization")
		}
		if err := s.Pending.Binding.validate(); err != nil {
			return err
		}
	}
	if s.Quota != nil {
		if s.Quota.ObservedAt.IsZero() {
			return fmt.Errorf("invalid quota observation")
		}
		for name, window := range s.Quota.Windows {
			if name == "" || len(name) > 64 || math.IsNaN(window.UsedPercent) || math.IsInf(window.UsedPercent, 0) || window.UsedPercent < 0 || window.UsedPercent > 100 {
				return fmt.Errorf("invalid quota window")
			}
		}
	}
	return nil
}

func (s *State) BindThread(id, threadID string) error {
	if threadID == "" || len(threadID) > 256 {
		return fmt.Errorf("invalid dispatch thread identity")
	}
	for i := range s.Attempts {
		a := &s.Attempts[i]
		if a.ID != id {
			continue
		}
		if a.Status != "reserved" || (a.ThreadID != "" && a.ThreadID != threadID) {
			return fmt.Errorf("dispatch thread binding changed")
		}
		a.ThreadID = threadID
		return nil
	}
	return fmt.Errorf("unknown dispatch attempt %q", id)
}

func (s *State) ObserveTurn(id, turnID string) error {
	if turnID == "" || len(turnID) > 256 {
		return fmt.Errorf("invalid dispatch turn identity")
	}
	for i := range s.Attempts {
		a := &s.Attempts[i]
		if a.ID != id {
			continue
		}
		if a.ThreadID == "" || (a.Status != "reserved" && a.Status != "uncertain") || (a.TurnID != "" && a.TurnID != turnID) {
			return fmt.Errorf("dispatch turn binding changed")
		}
		a.TurnID = turnID
		return nil
	}
	return fmt.Errorf("unknown dispatch attempt %q", id)
}

func (s *State) MarkTurnRequested(id string) error {
	for i := range s.Attempts {
		a := &s.Attempts[i]
		if a.ID != id {
			continue
		}
		if a.ThreadID == "" || a.Status != "reserved" || a.TurnRequested {
			return fmt.Errorf("dispatch turn request is not ready")
		}
		a.TurnRequested = true
		return nil
	}
	return fmt.Errorf("unknown dispatch attempt %q", id)
}

func (s *State) RecordOutcome(id, threadID, turnID, status, finalText string) error {
	if threadID == "" || turnID == "" || (status != "completed" && status != "interrupted" && status != "failed") || len(finalText) > 65536 {
		return fmt.Errorf("invalid provider outcome")
	}
	for i := range s.Attempts {
		a := &s.Attempts[i]
		if a.ID != id {
			continue
		}
		if a.ThreadID != threadID || !a.TurnRequested || (a.TurnID != "" && a.TurnID != turnID) {
			return fmt.Errorf("provider outcome differs from dispatch")
		}
		if a.FinalStatus != "" && (a.FinalStatus != status || a.FinalText != finalText) {
			return fmt.Errorf("provider outcome changed")
		}
		a.TurnID, a.FinalStatus, a.FinalText = turnID, status, finalText
		return nil
	}
	return fmt.Errorf("unknown dispatch attempt %q", id)
}

func (s *State) MarkApplied(id string) error {
	for i := range s.Attempts {
		a := &s.Attempts[i]
		if a.ID != id {
			continue
		}
		if a.Applied {
			return nil
		}
		if a.Status != "completed" || a.FinalStatus != "completed" {
			return fmt.Errorf("dispatch result is not completed")
		}
		a.Applied = true
		return nil
	}
	return fmt.Errorf("unknown dispatch attempt %q", id)
}

func NewState(profiles map[Role]codexapp.RoleSettings, limits Limits, gates GatePolicy, quota QuotaPolicy) (State, error) {
	if err := limits.validate(); err != nil {
		return State{}, err
	}
	if err := quota.validate(); err != nil {
		return State{}, err
	}
	for activity, gated := range gates.Activities {
		if gated && activity != Selection && activity != Engineering && activity != Review && activity != Correction && activity != Recovery {
			return State{}, fmt.Errorf("invalid gated activity %q", activity)
		}
	}
	for _, role := range []Role{Reagent, Iragent} {
		p, ok := profiles[role]
		if !ok || p.Role != string(role) || p.Model == "" || p.Effort == "" || p.Sandbox != "danger-full-access" || p.ApprovalPolicy != "never" {
			return State{}, fmt.Errorf("missing or invalid %s profile", role)
		}
	}
	copyProfiles := make(map[Role]codexapp.RoleSettings, 2)
	for role, value := range profiles {
		copyProfiles[role] = value
	}
	copyGates := GatePolicy{Activities: make(map[Activity]bool, len(gates.Activities))}
	for activity, enabled := range gates.Activities {
		copyGates.Activities[activity] = enabled
	}
	return State{Profiles: copyProfiles, Limits: limits, Gates: copyGates, QuotaPolicy: quota, Attempts: []Attempt{}, Assessments: []Assessment{}}, nil
}

func (s *State) Extend(next Limits) error {
	if err := next.validate(); err != nil {
		return err
	}
	if next.Dispatches < s.Limits.Dispatches || next.SelfReviews < s.Limits.SelfReviews || next.Assessments < s.Limits.Assessments || next.Recovery < s.Limits.Recovery {
		return fmt.Errorf("limits may only be extended")
	}
	s.Limits = next
	return nil
}

func (s *State) ObserveQuota(value QuotaObservation, now time.Time) error {
	if value.ObservedAt.IsZero() || value.ObservedAt.After(now) {
		return fmt.Errorf("invalid quota observation time or windows")
	}
	if s.Quota != nil && value.ObservedAt.Before(s.Quota.ObservedAt) {
		return fmt.Errorf("quota observation predates retained snapshot")
	}
	for name, window := range value.Windows {
		if name == "" || len(name) > 64 || math.IsNaN(window.UsedPercent) || math.IsInf(window.UsedPercent, 0) || window.UsedPercent < 0 || window.UsedPercent > 100 {
			return fmt.Errorf("invalid quota window %q", name)
		}
	}
	copyWindows := make(map[string]Window, len(value.Windows))
	for name, window := range value.Windows {
		copyWindows[name] = window
	}
	s.Quota = &QuotaObservation{ObservedAt: value.ObservedAt, Windows: copyWindows}
	return nil
}

func (s *State) quotaDecision(now time.Time) (hold, warning string) {
	if s.Quota == nil || len(s.Quota.Windows) == 0 || s.Quota.ObservedAt.After(now) || (s.QuotaPolicy.MaxAge > 0 && now.Sub(s.Quota.ObservedAt) > s.QuotaPolicy.MaxAge) {
		if s.QuotaPolicy.StrictUnknown {
			return "quota telemetry unknown or stale", ""
		}
		return "", "quota telemetry unknown or stale"
	}
	names := make([]string, 0, len(s.Quota.Windows))
	for name := range s.Quota.Windows {
		names = append(names, name)
	}
	sort.Strings(names)
	resetUnknown := false
	for _, name := range names {
		window := s.Quota.Windows[name]
		if !window.ResetAt.IsZero() && !now.Before(window.ResetAt) {
			resetUnknown = true
			continue
		}
		if s.QuotaPolicy.HoldUsedPercent > 0 && window.UsedPercent >= s.QuotaPolicy.HoldUsedPercent {
			return fmt.Sprintf("quota window %s reached dispatch hold threshold", name), ""
		}
		if s.QuotaPolicy.WarnUsedPercent > 0 && window.UsedPercent >= s.QuotaPolicy.WarnUsedPercent {
			warning = fmt.Sprintf("quota window %s reached warning threshold; observations may lag", name)
		}
	}
	if resetUnknown {
		if s.QuotaPolicy.StrictUnknown {
			return "quota window reset requires a fresh observation", ""
		}
		return "", "quota window reset requires a fresh observation"
	}
	return "", warning
}

type Decision struct {
	Attempt  *Attempt
	Hold     *Authorization
	Warning  string
	Dispatch bool // true only for a newly persisted reservation
}

var ErrUnresolvedAttempt = errors.New("previous dispatch remains unresolved")

// Reserve is called before provider turn/start. The caller persists the changed
// state before sending that request. Repeating an ID returns the same attempt.
func (s *State) Reserve(id string, binding Binding, now time.Time) (Decision, error) {
	if id == "" || len(id) > 128 || strings.ContainsAny(id, " \t\r\n\x00") || now.IsZero() {
		return Decision{}, fmt.Errorf("invalid dispatch reservation ID or time")
	}
	if err := binding.validate(); err != nil {
		return Decision{}, err
	}
	if _, ok := s.Profiles[binding.Role]; !ok {
		return Decision{}, fmt.Errorf("missing recorded role settings")
	}
	for i := range s.Attempts {
		attempt := &s.Attempts[i]
		if attempt.ID == id {
			if attempt.Binding != binding {
				return Decision{}, fmt.Errorf("dispatch ID has different binding")
			}
			return Decision{Attempt: attempt}, nil // observation only; never replay
		}
		if attempt.Status == "reserved" || attempt.Status == "uncertain" {
			return Decision{}, ErrUnresolvedAttempt
		}
	}
	if len(s.Attempts) >= s.Limits.Dispatches {
		return Decision{}, fmt.Errorf("dispatch allowance exhausted (%d/%d)", len(s.Attempts), s.Limits.Dispatches)
	}
	if binding.Activity == Review && len(s.Assessments) >= s.Limits.Assessments {
		return Decision{}, fmt.Errorf("independent assessment allowance exhausted")
	}
	if binding.Activity == Recovery {
		used := 0
		for _, attempt := range s.Attempts {
			if attempt.Binding.Activity == Recovery && attempt.Binding.Episode == binding.Episode {
				used++
			}
		}
		if used >= s.Limits.Recovery {
			return Decision{}, fmt.Errorf("recovery allowance exhausted for episode %q", binding.Episode)
		}
	}
	hold, warning := s.quotaDecision(now)
	if s.Gates.requires(binding.Activity) && hold == "" {
		hold = "configured dispatch authorization required"
	}
	if s.Pending != nil {
		if s.Pending.ID != id || s.Pending.Binding != binding {
			return Decision{Hold: s.Pending}, nil
		}
		if !s.Pending.Authorized {
			return Decision{Hold: s.Pending}, nil
		}
		// An explicit grant permits the bound dispatch despite the same quota
		// observation. A later dispatch evaluates fresh policy again.
	} else if hold != "" {
		s.Pending = &Authorization{ID: id, Binding: binding, Reason: hold, CreatedAt: now}
		return Decision{Hold: s.Pending, Warning: warning}, nil
	}
	s.Pending = nil
	s.Attempts = append(s.Attempts, Attempt{ID: id, Binding: binding, ReservedAt: now, Status: "reserved"})
	return Decision{Attempt: &s.Attempts[len(s.Attempts)-1], Warning: warning, Dispatch: true}, nil
}

func (s *State) Authorize(id string) error {
	if s.Pending == nil || s.Pending.ID != id {
		return fmt.Errorf("authorization does not match pending dispatch")
	}
	s.Pending.Authorized = true
	return nil
}

func (s *State) Finish(id, status string, tokens *TokenReceipt) error {
	if status != "completed" && status != "failed" && status != "uncertain" {
		return fmt.Errorf("invalid dispatch terminal status")
	}
	for i := range s.Attempts {
		a := &s.Attempts[i]
		if a.ID != id {
			continue
		}
		if a.Status != "reserved" {
			return fmt.Errorf("dispatch already has outcome %q", a.Status)
		}
		if tokens != nil && !tokens.valid() {
			return fmt.Errorf("invalid token receipt")
		}
		a.Status, a.Tokens = status, tokens
		return nil
	}
	return fmt.Errorf("unknown dispatch attempt %q", id)
}

func (t TokenReceipt) valid() bool {
	return t.InputTokens >= 0 && t.CachedInputTokens >= 0 && t.OutputTokens >= 0 && t.ReasoningOutputTokens >= 0 && t.TotalTokens >= 0
}

func (s *State) Reconcile(id, evidence string) error {
	if strings.TrimSpace(evidence) == "" || len(evidence) > 1024 {
		return fmt.Errorf("reconciliation requires bounded evidence")
	}
	for i := range s.Attempts {
		if s.Attempts[i].ID == id && s.Attempts[i].Status == "uncertain" {
			s.Attempts[i].Status = "reconciled"
			s.Attempts[i].Resolution = evidence
			return nil
		}
	}
	return fmt.Errorf("no uncertain dispatch %q", id)
}

func (s *State) MarkUncertain(id string) error {
	for i := range s.Attempts {
		a := &s.Attempts[i]
		if a.ID != id {
			continue
		}
		if a.Status == "uncertain" {
			return nil
		}
		if a.Status != "reserved" {
			return fmt.Errorf("dispatch is not unresolved")
		}
		a.Status = "uncertain"
		return nil
	}
	return fmt.Errorf("unknown dispatch attempt %q", id)
}

// ReconcileCompleted consumes one provider-observed completed turn. The
// caller must verify its exact thread/turn, callback receipts and source.
func (s *State) ReconcileCompleted(id, threadID, turnID, evidence string) error {
	if evidence == "" || len(evidence) > 1024 || threadID == "" || turnID == "" {
		return fmt.Errorf("completed reconciliation lacks exact evidence")
	}
	for i := range s.Attempts {
		a := &s.Attempts[i]
		if a.ID != id {
			continue
		}
		if a.ThreadID != threadID || a.TurnID != turnID || !a.TurnRequested {
			return fmt.Errorf("completed turn differs from reserved dispatch")
		}
		if a.Status == "completed" {
			return nil
		}
		if a.Status != "reserved" && a.Status != "uncertain" {
			return fmt.Errorf("dispatch cannot be completed twice")
		}
		a.Status, a.Resolution = "completed", evidence
		return nil
	}
	return fmt.Errorf("unknown dispatch attempt %q", id)
}

func (s *State) RecordAssessment(id, attemptID string) error {
	if id == "" || len(id) > 128 {
		return fmt.Errorf("invalid assessment ID")
	}
	for _, existing := range s.Assessments {
		if existing.ID == id {
			if existing.AttemptID == attemptID {
				return nil
			}
			return fmt.Errorf("assessment ID has different dispatch")
		}
		if existing.AttemptID == attemptID {
			return fmt.Errorf("review dispatch already has an assessment")
		}
	}
	if len(s.Assessments) >= s.Limits.Assessments {
		return fmt.Errorf("independent assessment allowance exhausted")
	}
	for _, attempt := range s.Attempts {
		if attempt.ID == attemptID && (attempt.Binding.Activity == Review || (attempt.Binding.Activity == Recovery && attempt.Binding.Role == Iragent)) && attempt.Status == "completed" {
			s.Assessments = append(s.Assessments, Assessment{ID: id, AttemptID: attemptID, Candidate: attempt.Binding.Candidate})
			return nil
		}
	}
	return fmt.Errorf("assessment requires a completed review dispatch")
}
