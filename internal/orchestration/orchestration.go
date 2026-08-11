// Package orchestration defines the executable-owned protocol boundary between
// Concoct and a future agent adapter. It deliberately does not start processes
// or mutate workflow artifacts.
package orchestration

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/workflow"
)

const ProtocolVersion = "v1"

type OutcomeClass string

const (
	Completed         OutcomeClass = "completed"
	Blocked           OutcomeClass = "blocked"
	DecisionRequired  OutcomeClass = "decision-required"
	FailedRecoverable OutcomeClass = "failed-recoverable"
	FailedTerminal    OutcomeClass = "failed-terminal"
)

// Evidence is a bounded, content-free description of the repository evidence
// against which authorization was issued. Its digest detects stale outcomes.
type Evidence struct {
	Digest string         `json:"digest"`
	State  workflow.State `json:"state"`
}

// Correlation identifies one authorization attempt. All fields must be echoed
// unchanged by an outcome; an adapter must never synthesize them.
type Correlation struct {
	InvocationID string `json:"invocation_id"`
	ActionID     string `json:"action_id"`
	TaskID       string `json:"task_id,omitempty"`
	AttemptID    string `json:"attempt_id"`
	Role         string `json:"role"`
}

// Action is the transport-neutral JSON envelope supplied to an adapter.
type Action struct {
	ProtocolVersion string      `json:"protocol_version"`
	Correlation     Correlation `json:"correlation"`
	Kind            string      `json:"kind"`
	Gate            string      `json:"gate"`
	Executable      bool        `json:"executable"`
	Explanation     string      `json:"explanation"`
	Evidence        Evidence    `json:"evidence"`
}

// Intervention is deliberately bounded: it carries a category and a concise
// next step, never raw logs, prompts, environment data, or file contents.
type Intervention struct {
	Kind string `json:"kind,omitempty"`
	Next string `json:"next,omitempty"`
}

// Diagnostic is safe for bounded durable history. Values are limited by
// ValidateOutcome and are not a general log transport.
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Outcome is an adapter's claim. It is not transition authority: callers must
// validate it against the action and current repository evidence.
type Outcome struct {
	ProtocolVersion string       `json:"protocol_version"`
	Correlation     Correlation  `json:"correlation"`
	Class           OutcomeClass `json:"class"`
	Summary         string       `json:"summary"`
	Artifacts       []string     `json:"artifacts,omitempty"`
	Intervention    Intervention `json:"intervention,omitempty"`
	Diagnostics     []Diagnostic `json:"diagnostics,omitempty"`
}

// DurableFacts is the only representation suitable for durable task history.
type DurableFacts struct {
	ActionID, InvocationID, AttemptID, Role, Kind, Summary string
	Class                                                  OutcomeClass
	Artifacts                                              []string
	Intervention                                           Intervention
}

// Spec defines the authority and observable completion requirements for one
// existing workflow operation. Completion remains validated by workflow state,
// rather than output text or an adapter exit status.
type Spec struct {
	Kind, Role, Gate, Explanation string
	// Authority identifies the workflow authority that grants this action.
	Authority         string
	AllowedStates     []workflow.State
	CompletedStates   []workflow.State
	SupportedOutcomes []OutcomeClass
	PermittedEffects  []string
	// Preconditions and CompletionValidator make the executable checks
	// inspectable instead of leaving callers to infer them from state lists.
	Preconditions       []string
	CompletionValidator func(workflow.Report) error
	Intervention        InterventionPolicy
}

// InterventionPolicy defines the bounded route required for a non-completed
// result. It intentionally contains no raw adapter output.
type InterventionPolicy struct {
	RequiredFor []OutcomeClass
	Kind        string
	Next        string
}

func spec(kind, role, gate, explanation string, allowed, completed []workflow.State, effects []string, intervention InterventionPolicy) Spec {
	return Spec{
		Kind: kind, Role: role, Gate: gate, Explanation: explanation,
		Authority: "executable workflow state and policy", AllowedStates: allowed,
		CompletedStates:   completed,
		SupportedOutcomes: []OutcomeClass{Completed, Blocked, DecisionRequired, FailedRecoverable, FailedTerminal},
		PermittedEffects:  effects,
		Preconditions:     []string{"observed workflow state is authorized for this action", "policy and workflow evidence remain operational"},
		CompletionValidator: func(report workflow.Report) error {
			if !containsState(completed, report.State) {
				return fmt.Errorf("completed outcome contradicts observed workflow state %s", report.State)
			}
			return nil
		},
		Intervention: intervention,
	}
}

var nonCompletionIntervention = []OutcomeClass{Blocked, DecisionRequired, FailedRecoverable, FailedTerminal}

var registry = []Spec{
	spec("product-owner-next", "product-owner", "decision", "Assess ready-state evidence; selecting work still requires Product Owner judgment.", []workflow.State{workflow.Ready}, []workflow.State{workflow.Ready}, []string{"roadmap-only product decision"}, InterventionPolicy{nonCompletionIntervention, "human-product-decision", "Product Owner decides or clarifies the next work item"}),
	spec("roadmap-intake", "product-owner", "decision", "Evaluate human product input and update only the roadmap.", []workflow.State{workflow.Ready}, []workflow.State{workflow.Ready}, []string{".concoct/roadmap.md"}, InterventionPolicy{nonCompletionIntervention, "human-product-decision", "Product Owner clarifies product direction"}),
	spec("task-planning", "task-planner", "planning", "Create the active task plan and notes for an eligible item.", []workflow.State{workflow.Ready}, []workflow.State{workflow.Planned}, []string{".concoct/current/task-plan.md", ".concoct/current/notes.md", "selected roadmap status"}, InterventionPolicy{nonCompletionIntervention, "planner-or-product-owner", "Task Planner resolves planning evidence or escalates product scope"}),
	spec("development", "developer", "implementation", "Implement or remediate the active task without changing review, roadmap, or capability ownership.", []workflow.State{workflow.Planned, workflow.InProgress, workflow.ChangesRequested}, []workflow.State{workflow.Complete}, []string{"task source, tests, documentation", ".concoct/current/task-plan.md", ".concoct/current/notes.md"}, InterventionPolicy{nonCompletionIntervention, "developer-remediation", "Developer resolves implementation evidence or escalates scope"}),
	spec("independent-review", "reviewer", "review", "Independently review the implementation and record one reserved review outcome.", []workflow.State{workflow.Complete}, []workflow.State{workflow.Approved, workflow.ChangesRequested, workflow.Blocked}, []string{"one new .concoct/current/review-NN.md"}, InterventionPolicy{nonCompletionIntervention, "review-routing", "Reviewer routes findings or blocker to the responsible role"}),
	spec("archival", "archivist", "archive", "Archive accepted work and reconcile accepted capability evidence.", []workflow.State{workflow.Approved}, []workflow.State{workflow.Archived, workflow.Ready}, []string{"archive candidate", "capabilities", "roadmap delivery evidence"}, InterventionPolicy{nonCompletionIntervention, "archive-routing", "Archivist routes missing approval or archival evidence"}),
	spec("integration", "integrator", "integration", "Squash the archived Git task branch onto its recorded trunk.", []workflow.State{workflow.Archived}, []workflow.State{workflow.Integrated, workflow.Ready}, []string{"recorded Git integration only"}, InterventionPolicy{nonCompletionIntervention, "integration-routing", "Integrator resolves delivery or Git evidence"}),
}

func Registry() []Spec { return append([]Spec(nil), registry...) }

func Find(kind string) (Spec, bool) {
	for _, spec := range registry {
		if spec.Kind == kind {
			return spec, true
		}
	}
	return Spec{}, false
}

// Authorize derives action data and the human-readable explanation from the
// same workflow snapshot. Ready state intentionally authorizes only a Product
// Owner decision, never an autonomous roadmap-item selection.
func Authorize(root, kind, attemptID string) (Action, error) {
	spec, ok := Find(kind)
	if !ok {
		return Action{}, fmt.Errorf("unsupported action kind %q", kind)
	}
	evidence, report, err := Snapshot(root)
	if err != nil {
		return Action{}, err
	}
	if !containsState(spec.AllowedStates, report.State) {
		return Action{}, fmt.Errorf("action %s is not authorized in workflow state %s", kind, report.State)
	}
	if len(spec.Preconditions) == 0 || spec.Authority == "" {
		return Action{}, fmt.Errorf("action %s has an incomplete executable contract", kind)
	}
	if strings.TrimSpace(attemptID) == "" {
		return Action{}, errors.New("attempt id is required")
	}
	invocation, err := randomID()
	if err != nil {
		return Action{}, err
	}
	actionID, err := randomID()
	if err != nil {
		return Action{}, err
	}
	return Action{ProtocolVersion: ProtocolVersion, Correlation: Correlation{InvocationID: invocation, ActionID: actionID, TaskID: report.RoadmapItem, AttemptID: attemptID, Role: spec.Role}, Kind: spec.Kind, Gate: spec.Gate, Executable: true, Explanation: spec.Explanation, Evidence: evidence}, nil
}

// Snapshot includes only hashes and bounded state metadata; it never retains
// raw project content. It is safe to compare but not intended as task history.
func Snapshot(root string) (Evidence, workflow.Report, error) {
	report := workflow.Detect(root)
	if report.OperationalError != nil {
		return Evidence{}, report, report.OperationalError
	}
	paths := []string{".concoct/roadmap.md", ".concoct/capabilities.md", ".concoct/current/task-plan.md", ".concoct/current/notes.md"}
	var parts []string
	for _, rel := range paths {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err == nil {
			parts = append(parts, rel+":"+hash(data))
		} else if !os.IsNotExist(err) {
			return Evidence{}, report, err
		}
	}
	parts = append(parts, "state:"+string(report.State), "task:"+report.RoadmapItem, "review:"+report.LatestReview)
	sort.Strings(parts)
	return Evidence{Digest: hash([]byte(strings.Join(parts, "\n"))), State: report.State}, report, nil
}

// ValidateOutcome reconciles a claim with executable authority and observed
// state. A process exit code is deliberately absent: invocation health cannot
// establish a workflow transition.
func ValidateOutcome(root string, action Action, outcome Outcome) (DurableFacts, error) {
	if action.ProtocolVersion != ProtocolVersion || outcome.ProtocolVersion != ProtocolVersion {
		return DurableFacts{}, fmt.Errorf("unsupported protocol version")
	}
	spec, ok := Find(action.Kind)
	if !ok {
		return DurableFacts{}, fmt.Errorf("unsupported action kind %q", action.Kind)
	}
	if action.Correlation.Role != spec.Role || !sameCorrelation(action.Correlation, outcome.Correlation) {
		return DurableFacts{}, errors.New("outcome correlation does not match authorized action")
	}
	if !containsOutcome(spec.SupportedOutcomes, outcome.Class) {
		return DurableFacts{}, fmt.Errorf("outcome class %q is not supported for %s", outcome.Class, action.Kind)
	}
	if err := bounded(outcome); err != nil {
		return DurableFacts{}, err
	}
	current, report, err := Snapshot(root)
	if err != nil {
		return DurableFacts{}, err
	}
	if outcome.Class != Completed && current.Digest != action.Evidence.Digest {
		return DurableFacts{}, errors.New("outcome is stale: repository evidence changed after authorization")
	}
	if current.Digest == action.Evidence.Digest && outcome.Class == Completed {
		return DurableFacts{}, errors.New("completed outcome has no observed workflow or artifact change")
	}
	if outcome.Class == Completed {
		if spec.CompletionValidator == nil {
			return DurableFacts{}, fmt.Errorf("action %s has no completion validator", action.Kind)
		}
		if err := spec.CompletionValidator(report); err != nil {
			return DurableFacts{}, err
		}
	}
	if containsOutcome(spec.Intervention.RequiredFor, outcome.Class) && (outcome.Intervention.Kind != spec.Intervention.Kind || outcome.Intervention.Next != spec.Intervention.Next) {
		return DurableFacts{}, fmt.Errorf("outcome class %q requires intervention %q", outcome.Class, spec.Intervention.Kind)
	}
	return DurableFacts{ActionID: action.Correlation.ActionID, InvocationID: action.Correlation.InvocationID, AttemptID: action.Correlation.AttemptID, Role: action.Correlation.Role, Kind: action.Kind, Class: outcome.Class, Summary: outcome.Summary, Artifacts: append([]string(nil), outcome.Artifacts...), Intervention: outcome.Intervention}, nil
}

// WriteAtomicResult enforces the process-adapter single-result contract. The
// target must not exist. Publishing a same-directory temporary file with a hard
// link is an atomic no-replace operation: exactly one concurrent writer can
// create the destination.
func WriteAtomicResult(path string, outcome Outcome) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("result already exists: duplicate delivery")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := bounded(outcome); err != nil {
		return err
	}
	data, err := json.Marshal(outcome)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".concoct-result-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0o600)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Link(name, path); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("result already exists: duplicate delivery")
		}
		return err
	}
	return os.Remove(name)
}

func ReadResult(path string) (Outcome, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Outcome{}, err
	}
	var outcome Outcome
	if err := json.Unmarshal(data, &outcome); err != nil {
		return Outcome{}, fmt.Errorf("malformed result: %w", err)
	}
	if err := bounded(outcome); err != nil {
		return Outcome{}, err
	}
	return outcome, nil
}

func sameCorrelation(a, b Correlation) bool {
	return a.InvocationID == b.InvocationID && a.ActionID == b.ActionID && a.TaskID == b.TaskID && a.AttemptID == b.AttemptID && a.Role == b.Role
}
func containsState(haystack []workflow.State, needle workflow.State) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
func containsOutcome(haystack []OutcomeClass, needle OutcomeClass) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func randomID() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func bounded(o Outcome) error {
	if o.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported protocol version %q", o.ProtocolVersion)
	}
	if strings.TrimSpace(o.Summary) == "" || len(o.Summary) > 1024 {
		return errors.New("outcome summary must be 1..1024 bytes")
	}
	if len(o.Artifacts) > 32 || len(o.Diagnostics) > 16 {
		return errors.New("outcome contains too many bounded fields")
	}
	for _, value := range append(append([]string{}, o.Artifacts...), o.Intervention.Kind, o.Intervention.Next) {
		if len(value) > 512 || strings.Contains(value, "\x00") {
			return errors.New("outcome contains invalid bounded field")
		}
	}
	for _, diagnostic := range o.Diagnostics {
		if len(diagnostic.Code) > 64 || len(diagnostic.Message) > 512 {
			return errors.New("outcome diagnostic exceeds bounds")
		}
	}
	return nil
}

// Now is a seam for future adapters that need timestamps without adding them to
// the authority contract; raw invocation records remain ephemeral.
func Now() time.Time { return time.Now().UTC() }
