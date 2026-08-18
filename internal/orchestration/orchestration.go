// Package orchestration defines the executable-owned protocol boundary between
// Concoct and agent adapters. It deliberately does not start processes
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
	"os/exec"
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
	ProtocolVersion string         `json:"protocol_version"`
	Correlation     Correlation    `json:"correlation"`
	Class           OutcomeClass   `json:"class"`
	Summary         string         `json:"summary"`
	Artifacts       []string       `json:"artifacts,omitempty"`
	Intervention    Intervention   `json:"intervention,omitempty"`
	Diagnostics     []Diagnostic   `json:"diagnostics,omitempty"`
	Recommendation  Recommendation `json:"recommendation,omitempty"`
	// ProductDecision is required for a completed Product Owner action.  It is
	// intentionally separate from Recommendation so an operator command can
	// never be the sole retained meaning of product judgment.
	ProductDecision ProductDecision `json:"product_decision,omitempty"`
}

// Recommendation is the bounded result of a decision action. It is separate
// from lifecycle completion: Product Owner judgment may recommend one manual
// follow-up without mutating workflow artifacts or starting that follow-up.
type Recommendation struct {
	Kind    string `json:"kind,omitempty"`
	Command string `json:"command,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// ProductDecision is the bounded semantic result of a Product Owner action.
// It deliberately records intent separately from the command displayed to an
// operator. Applying a decision remains executable authority; this value is a
// validated candidate rather than a patch or a general-purpose editor.
type ProductDecision struct {
	Version            int    `json:"version"`
	Kind               string `json:"kind"`
	Selection          string `json:"selection,omitempty"`
	Rationale          string `json:"rationale"`
	RoadmapDigest      string `json:"roadmap_digest,omitempty"`
	CapabilityDigest   string `json:"capability_digest,omitempty"`
	CompletionEvidence string `json:"completion_evidence,omitempty"`
	// Mutations are exact, record-scoped replacements. They are deliberately
	// not a patch language: each binds one canonical record to its prior digest.
	Mutations []ProductMutation `json:"mutations,omitempty"`
}

type ProductMutation struct {
	Target       string `json:"target,omitempty"`
	ID           string `json:"id,omitempty"`
	BeforeDigest string `json:"before_digest,omitempty"`
	Before       string `json:"before,omitempty"`
	After        string `json:"after,omitempty"`
}

const (
	DecisionSelect             = "select"
	DecisionReconcileAndSelect = "reconcile-and-select"
	DecisionReconcile          = "reconcile"
	DecisionHumanRequired      = "human-decision-required"
	DecisionNoAction           = "no-action"
)

// ValidateProductDecision enforces the versioned semantic vocabulary before a
// candidate can be retained. Detailed mutation validation is intentionally a
// later authority: digest fields bind the candidate to the exact inputs they
// describe without retaining arbitrary Markdown or adapter output.
func ValidateProductDecision(decision ProductDecision) error {
	if decision.Version != 1 {
		return fmt.Errorf("unsupported Product Owner decision version %d", decision.Version)
	}
	if len(decision.Kind) == 0 || len(decision.Kind) > 64 || len(decision.Rationale) == 0 || len(decision.Rationale) > 512 {
		return errors.New("Product Owner decision kind and rationale must be bounded non-empty fields")
	}
	if len(decision.Selection) > 128 || strings.Contains(decision.Selection, "\x00") {
		return errors.New("Product Owner decision contains an invalid selection")
	}
	for _, value := range []string{decision.Rationale, decision.RoadmapDigest, decision.CapabilityDigest, decision.CompletionEvidence} {
		if strings.Contains(value, "\x00") || len(value) > 512 {
			return errors.New("Product Owner decision contains an invalid bounded field")
		}
	}
	if len(decision.Mutations) > 8 {
		return errors.New("Product Owner decision contains too many record mutations")
	}
	seen := map[string]bool{}
	for _, mutation := range decision.Mutations {
		if (mutation.Target != "roadmap" && mutation.Target != "capabilities") || mutation.ID == "" || len(mutation.ID) > 128 || len(mutation.BeforeDigest) != 64 || len(mutation.Before) == 0 || len(mutation.After) == 0 || len(mutation.Before) > 8192 || len(mutation.After) > 8192 || strings.Contains(mutation.Before, "\x00") || strings.Contains(mutation.After, "\x00") || seen[mutation.Target+":"+mutation.ID] {
			return errors.New("Product Owner decision contains an invalid record-scoped mutation")
		}
		seen[mutation.Target+":"+mutation.ID] = true
	}
	switch decision.Kind {
	case DecisionSelect:
		if decision.Selection == "" || decision.RoadmapDigest != "" || decision.CapabilityDigest != "" || decision.CompletionEvidence != "" {
			return errors.New("select decision requires only a selected item and rationale")
		}
	case DecisionReconcileAndSelect:
		if decision.Selection == "" || (decision.RoadmapDigest == "" && decision.CapabilityDigest == "" && decision.CompletionEvidence == "") {
			return errors.New("reconcile-and-select decision requires a selected item and bounded reconciliation evidence")
		}
	case DecisionReconcile:
		if decision.Selection != "" || (decision.RoadmapDigest == "" && decision.CapabilityDigest == "" && decision.CompletionEvidence == "") {
			return errors.New("reconcile decision requires bounded reconciliation evidence and no selection")
		}
	case DecisionHumanRequired, DecisionNoAction:
		if decision.Selection != "" || decision.RoadmapDigest != "" || decision.CapabilityDigest != "" || decision.CompletionEvidence != "" {
			return fmt.Errorf("%s decision must not propose a selection or reconciliation", decision.Kind)
		}
	default:
		return fmt.Errorf("unsupported Product Owner decision kind %q", decision.Kind)
	}
	return nil
}

// DurableFacts is the only representation suitable for durable task history.
type DurableFacts struct {
	ActionID, InvocationID, AttemptID, Role, Kind, Summary string
	Class                                                  OutcomeClass
	Artifacts                                              []string
	Intervention                                           Intervention
	Recommendation                                         Recommendation
	ProductDecision                                        ProductDecision
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
	spec("archival", "archivist", "archive", "Archive accepted work and reconcile accepted capability evidence.", []workflow.State{workflow.Complete, workflow.Approved}, []workflow.State{workflow.Archived, workflow.Ready}, []string{"archive candidate", "capabilities", "roadmap delivery evidence"}, InterventionPolicy{nonCompletionIntervention, "archive-routing", "Archivist routes missing approval or archival evidence"}),
	spec("integration", "integrator", "integration", "Squash the archived Git task branch onto its recorded trunk.", []workflow.State{workflow.Archived}, []workflow.State{workflow.Integrated, workflow.Ready}, []string{"recorded Git integration only"}, InterventionPolicy{nonCompletionIntervention, "integration-routing", "Integrator resolves delivery or Git evidence"}),
}

// Resolution is the typed, state-derived recommendation consumed by status,
// dry-run, and execution. Command is retained only for display and manual
// parity; action selection never parses it.
type Resolution struct {
	Kind, Role, PromptCommand, Command, Refusal string
	Executable                                  bool
}

// Resolve selects at most one action from validated workflow and policy
// evidence. Recovery-choice and human-routed states deliberately refuse.
func Resolve(root string) (Resolution, error) {
	report := workflow.Detect(root)
	if report.OperationalError != nil {
		return Resolution{}, report.OperationalError
	}
	if report.State == workflow.Invalid {
		return Resolution{Refusal: "invalid workflow state: " + strings.Join(report.Diagnostics, "; ")}, nil
	}
	authority := workflow.ResolveAction(report)
	resolved := Resolution{Kind: authority.ActionKind, Role: authority.Role, PromptCommand: authority.PromptCommand, Command: authority.Command, Refusal: authority.Refusal, Executable: authority.Executable}
	if !resolved.Executable {
		return resolved, nil
	}
	spec, ok := Find(resolved.Kind)
	if !ok || !containsState(spec.AllowedStates, report.State) {
		return Resolution{}, fmt.Errorf("resolved action %s has no matching executable contract for state %s", resolved.Kind, report.State)
	}
	if resolved.Role != spec.Role {
		return Resolution{}, fmt.Errorf("resolved action %s role %s disagrees with executable contract role %s", resolved.Kind, resolved.Role, spec.Role)
	}
	return resolved, nil
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
	authority := workflow.ResolveAction(report)
	if !authority.Executable || authority.ActionKind != kind {
		return Action{}, fmt.Errorf("action %s is not the policy-authorized recommendation in workflow state %s", kind, report.State)
	}
	if len(spec.Preconditions) == 0 || spec.Authority == "" {
		return Action{}, fmt.Errorf("action %s has an incomplete executable contract", kind)
	}
	if strings.TrimSpace(attemptID) == "" {
		return Action{}, errors.New("attempt id is required")
	}
	return newAction(spec, report.RoadmapItem, attemptID, evidence)
}

// AuthorizePlanning is the narrow additional authority used after a validated
// Product Owner selection. Ordinary ready-state resolution remains limited to
// product-owner-next.
func AuthorizePlanning(root, itemID, attemptID string) (Action, error) {
	if err := workflow.ValidatePlanItem(root, itemID); err != nil {
		return Action{}, fmt.Errorf("selected planning item is not eligible: %w", err)
	}
	spec, _ := Find("task-planning")
	evidence, report, err := Snapshot(root)
	if err != nil {
		return Action{}, err
	}
	if report.State != workflow.Ready {
		return Action{}, fmt.Errorf("task planning is not authorized in workflow state %s", report.State)
	}
	return newAction(spec, itemID, attemptID, evidence)
}

func newAction(spec Spec, taskID, attemptID string, evidence Evidence) (Action, error) {
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
	return Action{ProtocolVersion: ProtocolVersion, Correlation: Correlation{InvocationID: invocation, ActionID: actionID, TaskID: taskID, AttemptID: attemptID, Role: spec.Role}, Kind: spec.Kind, Gate: spec.Gate, Executable: true, Explanation: spec.Explanation, Evidence: evidence}, nil
}

// Snapshot includes only hashes and bounded state metadata; it never retains
// raw project content. It is safe to compare but not intended as task history.
func Snapshot(root string) (Evidence, workflow.Report, error) {
	report := workflow.Detect(root)
	if report.OperationalError != nil {
		return Evidence{}, report, report.OperationalError
	}
	paths := []string{"AGENTS.md", ".concoct/policy.md", ".concoct/project.yaml", ".concoct/config.yaml", ".concoct/roadmap.md", ".concoct/capabilities.md"}
	var parts []string
	for _, rel := range paths {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err == nil {
			parts = append(parts, rel+":"+hash(data))
		} else if !os.IsNotExist(err) {
			return Evidence{}, report, err
		}
	}
	for _, pattern := range []string{".concoct/current/*", ".concoct/archive/*/summary.md"} {
		matches, globErr := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if globErr != nil {
			return Evidence{}, report, globErr
		}
		for _, path := range matches {
			info, statErr := os.Lstat(path)
			if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return Evidence{}, report, readErr
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return Evidence{}, report, relErr
			}
			parts = append(parts, filepath.ToSlash(rel)+":"+hash(data))
		}
	}
	if gitEvidence, gitErr := repositoryEvidence(root); gitErr != nil {
		return Evidence{}, report, gitErr
	} else if gitEvidence != "" {
		parts = append(parts, gitEvidence)
	}
	parts = append(parts, "state:"+string(report.State), "task:"+report.RoadmapItem, "review:"+report.LatestReview)
	sort.Strings(parts)
	return Evidence{Digest: hash([]byte(strings.Join(parts, "\n"))), State: report.State}, report, nil
}

// ValidateOutcome reconciles a claim with executable authority and observed
// state. A process exit code is deliberately absent: invocation health cannot
// establish a workflow transition.
func ValidateOutcome(root string, action Action, outcome Outcome) (DurableFacts, error) {
	return validateOutcome(root, action, outcome, false)
}

// ValidateSupervisedOutcome permits role-owned partial repository effects for
// accepted non-completion outcomes after the executor has validated their
// changed paths. Ordinary callers retain the stricter unchanged-evidence rule.
func ValidateSupervisedOutcome(root string, action Action, outcome Outcome) (DurableFacts, error) {
	return validateOutcome(root, action, outcome, true)
}

func validateOutcome(root string, action Action, outcome Outcome, allowPartialEffects bool) (DurableFacts, error) {
	if err := ValidateCandidate(action, outcome); err != nil {
		return DurableFacts{}, err
	}
	spec, _ := Find(action.Kind)
	current, report, err := Snapshot(root)
	if err != nil {
		return DurableFacts{}, err
	}
	if outcome.Class == Completed {
		if action.Kind == "product-owner-next" {
			if current.Digest != action.Evidence.Digest {
				return DurableFacts{}, errors.New("Product Owner recommendation changed workflow or repository evidence")
			}
			if err := validateProductOwnerDecision(root, outcome.ProductDecision); err != nil {
				return DurableFacts{}, err
			}
		} else if current.Digest == action.Evidence.Digest {
			return DurableFacts{}, errors.New("completed outcome has no observed workflow or artifact change")
		}
		if spec.CompletionValidator == nil {
			return DurableFacts{}, fmt.Errorf("action %s has no completion validator", action.Kind)
		}
		if err := spec.CompletionValidator(report); err != nil {
			return DurableFacts{}, err
		}
	} else if current.Digest != action.Evidence.Digest && !allowPartialEffects {
		return DurableFacts{}, errors.New("outcome is stale: repository evidence changed after authorization")
	}
	return DurableFacts{ActionID: action.Correlation.ActionID, InvocationID: action.Correlation.InvocationID, AttemptID: action.Correlation.AttemptID, Role: action.Correlation.Role, Kind: action.Kind, Class: outcome.Class, Summary: outcome.Summary, Artifacts: append([]string(nil), outcome.Artifacts...), Intervention: outcome.Intervention, Recommendation: outcome.Recommendation, ProductDecision: outcome.ProductDecision}, nil
}

// ValidateCandidate validates the bounded structured claim before any
// supervised completion authority is invoked. Repository effects are checked
// separately by the executor against the role-owned candidate boundary.
func ValidateCandidate(action Action, outcome Outcome) error {
	if action.ProtocolVersion != ProtocolVersion || outcome.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported protocol version")
	}
	spec, ok := Find(action.Kind)
	if !ok {
		return fmt.Errorf("unsupported action kind %q", action.Kind)
	}
	if action.Correlation.Role != spec.Role || !sameCorrelation(action.Correlation, outcome.Correlation) {
		return errors.New("outcome correlation does not match authorized action")
	}
	if !containsOutcome(spec.SupportedOutcomes, outcome.Class) {
		return fmt.Errorf("outcome class %q is not supported for %s", outcome.Class, action.Kind)
	}
	if err := bounded(outcome); err != nil {
		return err
	}
	if containsOutcome(spec.Intervention.RequiredFor, outcome.Class) && (outcome.Intervention.Kind != spec.Intervention.Kind || outcome.Intervention.Next != spec.Intervention.Next) {
		return fmt.Errorf("outcome class %q requires intervention %q", outcome.Class, spec.Intervention.Kind)
	}
	return nil
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
func NewID() (string, error)  { return randomID() }
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
	for _, value := range []string{o.Recommendation.Kind, o.Recommendation.Command, o.Recommendation.Reason} {
		if len(value) > 512 || strings.Contains(value, "\x00") {
			return errors.New("outcome recommendation exceeds bounds")
		}
	}
	if o.ProductDecision.Version != 0 {
		if err := ValidateProductDecision(o.ProductDecision); err != nil {
			return err
		}
	}
	return nil
}

func validateProductOwnerDecision(root string, decision ProductDecision) error {
	if err := ValidateProductDecision(decision); err != nil {
		return fmt.Errorf("invalid Product Owner semantic decision: %w", err)
	}
	// A selection that needs no reconciliation must already be structurally
	// plannable. Reconciliation decisions retain their evidence for a later
	// bounded application transaction and therefore cannot claim eligibility
	// merely by displaying a command.
	if decision.Kind == DecisionSelect {
		if err := workflow.ValidatePlanItem(root, decision.Selection); err != nil {
			return fmt.Errorf("selected Product Owner item is not eligible: %w", err)
		}
	}
	return nil
}

func validateRecommendation(root string, recommendation Recommendation) error {
	switch recommendation.Kind {
	case "plan":
		parts := strings.Fields(recommendation.Command)
		if len(parts) != 3 || parts[0] != "concoct" || parts[1] != "plan" {
			return errors.New("plan recommendation must use `concoct plan <roadmap-id>`")
		}
		if err := workflow.ValidatePlanItem(root, parts[2]); err != nil {
			return fmt.Errorf("plan recommendation is not currently eligible: %w", err)
		}
	case "roadmap":
		if recommendation.Command != "concoct roadmap" {
			return errors.New("roadmap recommendation must use `concoct roadmap`")
		}
	case "blocker", "no-action":
		if recommendation.Command != "" || strings.TrimSpace(recommendation.Reason) == "" {
			return fmt.Errorf("%s recommendation requires a reason and no command", recommendation.Kind)
		}
	default:
		return fmt.Errorf("unsupported Product Owner recommendation kind %q", recommendation.Kind)
	}
	return nil
}

func repositoryEvidence(root string) (string, error) {
	gitDir := filepath.Join(root, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	read := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
		return string(out), nil
	}
	head, err := read("rev-parse", "--verify", "HEAD")
	if err != nil {
		head = "unborn\n"
	}
	branch, err := read("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	status, err := read("status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return "", err
	}
	worktreeDiff, err := read("diff", "--binary", "--no-ext-diff")
	if err != nil {
		return "", err
	}
	indexDiff, err := read("diff", "--cached", "--binary", "--no-ext-diff")
	if err != nil {
		return "", err
	}
	untracked, err := read("ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}
	var untrackedParts []string
	for _, rel := range strings.Split(untracked, "\x00") {
		if rel == "" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", readErr
		}
		untrackedParts = append(untrackedParts, rel+":"+hash(data))
	}
	sort.Strings(untrackedParts)
	return "git:" + hash([]byte(head+"\x00"+branch+"\x00"+status+"\x00"+worktreeDiff+"\x00"+indexDiff+"\x00"+strings.Join(untrackedParts, "\n"))), nil
}

// Now is a seam for future adapters that need timestamps without adding them to
// the authority contract; raw invocation records remain ephemeral.
func Now() time.Time { return time.Now().UTC() }
