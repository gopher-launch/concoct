// Package runloop composes fresh one-shot actions into one bounded lifecycle run.
package runloop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/gopher-launch/concoct/internal/adapter"
	"github.com/gopher-launch/concoct/internal/config"
	"github.com/gopher-launch/concoct/internal/contract"
	"github.com/gopher-launch/concoct/internal/execution"
	"github.com/gopher-launch/concoct/internal/gitrepo"
	"github.com/gopher-launch/concoct/internal/orchestration"
	"github.com/gopher-launch/concoct/internal/runstate"
	"github.com/gopher-launch/concoct/internal/workflow"
)

type Options struct {
	Approve   string
	Policy    config.RunOverrides
	Execution config.Overrides
	Input     io.Reader
	Progress  io.Writer
}

type actionRunner func(context.Context, string, execution.Options, io.Reader, io.Writer) (execution.Result, error)

type Step struct {
	Action, Role, Outcome, State, Invocation, Progress, Disposition string
	PromptBytes                                                     int
	Measurement                                                     adapter.EventEvidence
	Accepted                                                        bool
}

type Summary struct {
	Steps                      []Step
	State                      workflow.State
	Stop, Gate, Recommendation string
	Repository                 string
	Actions, ActionLimit       int
	Cycles, CycleLimit         int
	Completed                  bool
}

func (s Summary) String() string {
	var b strings.Builder
	fmt.Fprintln(&b, "Run summary:")
	for i, step := range s.Steps {
		fmt.Fprintf(&b, "  %d. %s (%s): %s -> %s", i+1, step.Action, step.Role, step.Outcome, step.State)
		if step.Invocation != "" {
			fmt.Fprintf(&b, " [%s]", step.Invocation)
		}
		if step.Progress != "" {
			fmt.Fprintf(&b, " {%s}", step.Progress)
		}
		if step.PromptBytes > 0 {
			cost := "wasted"
			if step.Accepted {
				cost = "accepted"
			}
			fmt.Fprintf(&b, " {prompt-bytes=%d; cost=%s; disposition=%s; activity=%d; command-output-bytes=%d; %s}", step.PromptBytes, cost, step.Disposition, step.Measurement.ActivityEvents, step.Measurement.Commands.OutputBytes, step.Measurement.UsageSummary())
		} else if step.Action == "integration" {
			fmt.Fprint(&b, " {no-agent mechanical action}")
		}
		b.WriteByte('\n')
	}
	if len(s.Steps) == 0 {
		fmt.Fprintln(&b, "  No actions attempted.")
	}
	if aggregates := s.usageAggregates(); len(aggregates) > 0 {
		fmt.Fprintln(&b, "Agent usage aggregates (native fields; reported attempts only):")
		keys := make([]string, 0, len(aggregates))
		for key := range aggregates {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "  %s: %s\n", key, aggregates[key].String())
		}
	}
	fmt.Fprintf(&b, "Bounds: actions %d/%d (remaining %d); review cycles %d/%d (remaining %d)\n", s.Actions, s.ActionLimit, max(0, s.ActionLimit-s.Actions), s.Cycles, s.CycleLimit, max(0, s.CycleLimit-s.Cycles))
	fmt.Fprintf(&b, "Workflow state: %s\n", s.State)
	if s.Repository != "" {
		fmt.Fprintf(&b, "Repository state: %s\n", s.Repository)
	}
	if s.Gate != "" {
		fmt.Fprintf(&b, "Pending gate: %s\n", s.Gate)
	}
	if s.Stop != "" {
		fmt.Fprintf(&b, "Stop: %s\n", s.Stop)
	}
	if s.Recommendation != "" {
		fmt.Fprintf(&b, "Next: %s\n", s.Recommendation)
	}
	return b.String()
}

type usageAggregate struct {
	attempts int
	fields   [5]usageField
}

type usageField struct {
	total    int64
	reported int
}

func (a *usageAggregate) add(usage adapter.Usage) {
	a.attempts++
	for index, value := range []*int64{usage.Input, usage.CachedInput, usage.Output, usage.ReasoningOutput, usage.Total} {
		if value != nil {
			a.fields[index].total += *value
			a.fields[index].reported++
		}
	}
}

func (a usageAggregate) String() string {
	names := []string{"input", "cached-input", "output", "reasoning-output", "total"}
	var values []string
	for index, field := range a.fields {
		if field.reported > 0 {
			values = append(values, fmt.Sprintf("%s=%d (%d/%d attempts)", names[index], field.total, field.reported, a.attempts))
		}
	}
	return strings.Join(values, " ")
}

func (s Summary) usageAggregates() map[string]usageAggregate {
	out := map[string]usageAggregate{}
	for _, step := range s.Steps {
		if step.PromptBytes == 0 || !step.Measurement.HasUsage() {
			continue
		}
		cost := "wasted"
		if step.Accepted {
			cost = "accepted"
		}
		for _, key := range []string{"role=" + step.Role, "action=" + step.Action, "cost=" + cost} {
			aggregate := out[key]
			aggregate.add(step.Measurement.Usage)
			out[key] = aggregate
		}
	}
	return out
}

// Run re-detects and re-authorizes every action. Expected gates and accepted
// interventions are represented in Summary; unsafe preparation/execution
// failures are also returned as errors.
func Run(ctx context.Context, root string, options Options) (Summary, error) {
	return run(ctx, root, options, execution.RunAccepted)
}

func run(ctx context.Context, root string, options Options, runAccepted actionRunner) (Summary, error) {
	if options.Approve != "" && !map[string]bool{"next": true, "plan": true, "development": true, "review": true, "archive": true, "integration": true}[options.Approve] {
		return Summary{}, fmt.Errorf("unsupported approval gate %q", options.Approve)
	}
	if options.Input == nil {
		options.Input = strings.NewReader("")
	}
	if options.Progress == nil {
		options.Progress = io.Discard
	}
	if err := contract.CheckMutate(root); err != nil {
		return Summary{}, err
	}
	policy, err := config.ResolveRun(root, options.Policy)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{ActionLimit: policy.MaxActions, CycleLimit: policy.MaxCycles}
	consumedNext := false
	nextAttempt := ""
	nextPredecessor := ""
	satisfied := map[string]bool{}

	var approval *runstate.Gate
	if gate, loadErr := runstate.Load(root); loadErr == nil {
		evidence, _, snapErr := orchestration.Snapshot(root)
		if snapErr != nil {
			return summary, snapErr
		}
		summary.Repository = repositoryState(root)
		configDigest, digestErr := config.EvidenceDigest(root)
		if digestErr != nil {
			return summary, digestErr
		}
		if options.Approve == "" {
			if staleErr := runstate.ValidateCurrent(gate, gate.Name, evidence, configDigest); staleErr != nil {
				if invalidationErr := runstate.Consume(root, gate); invalidationErr != nil {
					return summary, fmt.Errorf("%v; invalidate stale gate: %w", staleErr, invalidationErr)
				}
				summary.State = evidence.State
				summary.Stop = staleErr.Error()
				summary.Recommendation = "concoct run"
				return summary, staleErr
			}
			summary.State, summary.Gate = evidence.State, gate.Name
			summary.Repository = repositoryState(root)
			summary.Stop = "approval is required before the protected action"
			summary.Recommendation = "concoct run --approve " + gate.Name
			return summary, nil
		}
		if err := runstate.ValidateCurrent(gate, options.Approve, evidence, configDigest); err != nil {
			summary.State = evidence.State
			if gate.Name == options.Approve && (gate.Evidence != evidence.Digest || gate.State != evidence.State || gate.ConfigDigest != configDigest) {
				if invalidationErr := runstate.Consume(root, gate); invalidationErr != nil {
					return summary, fmt.Errorf("%v; invalidate stale gate: %w", err, invalidationErr)
				}
				summary.Recommendation = "concoct run"
			}
			return summary, err
		}
		approval = &gate
		for _, prerequisite := range gate.Prerequisites {
			satisfied[prerequisite] = true
		}
	} else if !isNotExist(loadErr) {
		return summary, loadErr
	} else if options.Approve != "" {
		return summary, fmt.Errorf("--approve %s requires a current pending gate", options.Approve)
	}

	seen := map[string]bool{}
	selectedPlan := ""
	for {
		if err := ctx.Err(); err != nil {
			summary.Stop, summary.Recommendation = "run cancelled", "concoct run"
			return finish(root, summary), err
		}
		if err := contract.CheckMutate(root); err != nil {
			return finish(root, summary), err
		}
		currentPolicy, err := config.ResolveRun(root, options.Policy)
		if err != nil {
			return finish(root, summary), err
		}
		if currentPolicy.MaxActions < summary.ActionLimit {
			summary.ActionLimit = currentPolicy.MaxActions
		}
		if currentPolicy.MaxCycles < summary.CycleLimit {
			summary.CycleLimit = currentPolicy.MaxCycles
		}
		for gate := range currentPolicy.Gates {
			policy.Gates[gate] = true
		}

		report := workflow.Detect(root)
		summary.State = report.State
		summary.Repository = repositoryState(root)
		if report.OperationalError != nil {
			return summary, report.OperationalError
		}
		if report.State == workflow.Invalid {
			summary.Stop = "invalid workflow state: " + strings.Join(report.Diagnostics, "; ")
			return summary, fmt.Errorf("%s", summary.Stop)
		}

		if approval != nil && approval.Name == "next" && !consumedNext {
			if report.State != workflow.Ready || approval.Action != "task-planning" {
				return summary, fmt.Errorf("pending next approval no longer targets ready-state task planning")
			}
			if err := workflow.ValidatePlanItem(root, approval.Selection); err != nil {
				return summary, fmt.Errorf("pending next selection is no longer eligible: %w", err)
			}
			if err := consume(root, approval, "task-planning", approval.Selection); err != nil {
				return summary, err
			}
			consumedNext = true
			nextAttempt = approval.AttemptID
			nextPredecessor = approval.InvocationID
			selectedPlan = approval.Selection
			approval = nil
		}

		resolution, err := orchestration.Resolve(root)
		if err != nil {
			return summary, err
		}
		if selectedPlan != "" {
			resolution = orchestration.Resolution{Kind: "task-planning", Role: "task-planner", Command: "concoct plan " + selectedPlan, Executable: true}
		}
		if !resolution.Executable {
			summary.Stop = resolution.Refusal
			summary.Recommendation = resolution.Command
			return summary, nil
		}
		before, _, err := orchestration.Snapshot(root)
		if err != nil {
			return summary, err
		}
		fingerprint := resolution.Kind + ":" + before.Digest
		if seen[fingerprint] {
			summary.Stop = "no progress: repeated action and evidence fingerprint"
			summary.Recommendation = resolution.Command
			return summary, nil
		}

		gates := requiredGates(report.State, resolution.Kind, policy)
		if approval != nil && !contains(gates, approval.Name) {
			gates = append([]string{approval.Name}, gates...)
		}
		for _, gateName := range gates {
			if satisfied[gateName] {
				continue
			}
			if approval != nil {
				if approval.Name != gateName {
					return summary, fmt.Errorf("approved gate %s does not authorize current gate %s", approval.Name, gateName)
				}
				if err := consume(root, approval, resolution.Kind, report.RoadmapItem); err != nil {
					return summary, err
				}
				nextAttempt = approval.AttemptID
				nextPredecessor = approval.InvocationID
				satisfied[gateName] = true
				approval = nil
				continue
			}
			evidence, _, err := orchestration.Snapshot(root)
			if err != nil {
				return summary, err
			}
			configDigest, err := config.EvidenceDigest(root)
			if err != nil {
				return summary, err
			}
			gate, err := runstate.New(gateName, resolution.Kind, report.RoadmapItem, "", evidence, orchestration.Correlation{}, configDigest)
			if err != nil {
				return summary, err
			}
			for _, prior := range gates {
				if satisfied[prior] {
					gate.Prerequisites = append(gate.Prerequisites, prior)
				}
			}
			if err := runstate.Create(root, gate); err != nil {
				return summary, err
			}
			summary.Gate = gateName
			summary.Stop = "approval is required before " + resolution.Kind
			summary.Recommendation = "concoct run --approve " + gateName
			return summary, nil
		}

		if summary.Actions >= summary.ActionLimit {
			summary.Stop = "action bound exhausted before " + resolution.Kind
			summary.Recommendation = "concoct run"
			return summary, nil
		}
		if resolution.Kind == "independent-review" && summary.Cycles >= summary.CycleLimit {
			summary.Stop = "review cycle bound exhausted before independent-review"
			summary.Recommendation = "concoct review"
			return summary, nil
		}
		seen[fingerprint] = true

		execOptions := execution.Options{Override: options.Execution, SelectedPlan: selectedPlan, LocalIntegration: resolution.Kind == "integration", PredecessorInvocationID: nextPredecessor}
		execOptions.AttemptID = nextAttempt
		nextAttempt = ""
		nextPredecessor = ""
		// Every satisfied gate protects only this action occurrence. Recurring
		// Developer or Reviewer actions must stop for fresh approval.
		satisfied = map[string]bool{}
		result, runErr := runAccepted(ctx, root, execOptions, options.Input, options.Progress)
		selectedPlan = ""
		summary.Actions++
		outcome := result.Reconciliation.OutcomeClass
		if outcome == "" {
			outcome = result.Reconciliation.InvocationDisposition
		}
		progressEvidence := shortDigest(before.Digest)
		if after, _, snapshotErr := orchestration.Snapshot(root); snapshotErr == nil {
			progressEvidence += " -> " + shortDigest(after.Digest)
		}
		summary.Steps = append(summary.Steps, Step{Action: resolution.Kind, Role: resolution.Role, Outcome: outcome, State: string(result.Reconciliation.ObservedState), Invocation: result.Prepared.Action.Correlation.InvocationID, Progress: progressEvidence, Disposition: result.Reconciliation.InvocationDisposition, PromptBytes: result.Prepared.Composition.ByteCount(), Measurement: result.Measurement, Accepted: result.Reconciliation.ResultAccepted})
		if resolution.Kind == "independent-review" && result.Reconciliation.ResultAccepted && result.Facts.Class == orchestration.Completed {
			summary.Cycles++
		}
		if runErr != nil {
			summary.Stop = runErr.Error()
			summary.Recommendation = safeContinuation(resolution.Kind)
			return finish(root, summary), runErr
		}
		if !result.Reconciliation.ResultAccepted {
			summary.Stop = "action produced no accepted structured result"
			summary.Recommendation = safeContinuation(resolution.Kind)
			return finish(root, summary), fmt.Errorf("%s", summary.Stop)
		}
		if result.Facts.Class != orchestration.Completed {
			summary.Stop = fmt.Sprintf("accepted %s outcome: %s", result.Facts.Class, result.Facts.Summary)
			summary.Recommendation = result.Facts.Intervention.Next
			return finish(root, summary), nil
		}

		if resolution.Kind == "product-owner-next" {
			recommendation := result.Facts.Recommendation
			switch recommendation.Kind {
			case "plan":
				parts := strings.Fields(recommendation.Command)
				selection := parts[len(parts)-1]
				evidence, _, err := orchestration.Snapshot(root)
				if err != nil {
					return summary, err
				}
				configDigest, err := config.EvidenceDigest(root)
				if err != nil {
					return summary, err
				}
				gate, err := runstate.New("next", "task-planning", selection, selection, evidence, result.Prepared.Action.Correlation, configDigest)
				if err != nil {
					return summary, err
				}
				if err := runstate.Create(root, gate); err != nil {
					return summary, err
				}
				summary.Gate = "next"
				summary.Stop = "Product Owner proposal requires explicit selection approval"
				summary.Recommendation = "concoct run --approve next"
				return finish(root, summary), nil
			case "roadmap":
				summary.Stop, summary.Recommendation = recommendation.Reason, recommendation.Command
				return finish(root, summary), nil
			case "blocker", "no-action":
				summary.Stop = recommendation.Reason
				return finish(root, summary), nil
			}
		}
		if resolution.Kind == "integration" {
			final := workflow.Detect(root)
			if final.State == workflow.Ready {
				summary.State, summary.Completed = final.State, true
				summary.Stop = "local integration completed"
				return summary, nil
			}
		}
		if resolution.Kind == "archival" {
			final := workflow.Detect(root)
			if final.State == workflow.Ready {
				summary.State, summary.Completed = final.State, true
				summary.Stop = "non-Git archival completed"
				return summary, nil
			}
		}
	}
}

func consume(root string, gate *runstate.Gate, action, task string) error {
	evidence, _, err := orchestration.Snapshot(root)
	if err != nil {
		return err
	}
	configDigest, err := config.EvidenceDigest(root)
	if err != nil {
		return err
	}
	if err := runstate.ValidateCurrent(*gate, gate.Name, evidence, configDigest); err != nil {
		return err
	}
	if gate.Action != action || (gate.TaskID != "" && gate.TaskID != task) {
		return fmt.Errorf("pending %s approval targets %s/%s, not %s/%s", gate.Name, gate.Action, gate.TaskID, action, task)
	}
	return runstate.Consume(root, *gate)
}

func requiredGates(state workflow.State, action string, policy config.RunPolicy) []string {
	var candidates []string
	if state == workflow.Planned && policy.Requires("plan") {
		candidates = append(candidates, "plan")
	}
	name := map[string]string{"development": "development", "independent-review": "review", "archival": "archive", "integration": "integration"}[action]
	if name != "" && policy.Requires(name) {
		candidates = append(candidates, name)
	}
	return candidates
}

func safeContinuation(action string) string {
	return map[string]string{"product-owner-next": "concoct run", "task-planning": "concoct plan", "development": "concoct code", "independent-review": "concoct review", "archival": "concoct archive", "integration": "concoct integrate"}[action]
}

func finish(root string, summary Summary) Summary {
	report := workflow.Detect(root)
	summary.State = report.State
	summary.Repository = repositoryState(root)
	if report.State == workflow.Integrating {
		summary.Recommendation = "concoct integrate --continue, or concoct integrate --abort"
	}
	return summary
}

func repositoryState(root string) string {
	repo, ok, err := gitrepo.Open(root)
	if err != nil || !ok {
		return "non-Git"
	}
	branch, branchErr := repo.Branch()
	head, headErr := repo.Head()
	status, statusErr := repo.Status()
	if branchErr != nil || headErr != nil || statusErr != nil {
		return "Git evidence unavailable"
	}
	if len(head) > 12 {
		head = head[:12]
	}
	clean := "clean"
	if status != "" {
		clean = "dirty"
	}
	return fmt.Sprintf("branch %s at %s (%s)", branch, head, clean)
}

func isNotExist(err error) bool { return errors.Is(err, os.ErrNotExist) }

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func shortDigest(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
