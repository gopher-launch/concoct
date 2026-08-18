// Package execution resolves, records, supervises, and reconciles one workflow action.
package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gopher-launch/concoct/internal/adapter"
	"github.com/gopher-launch/concoct/internal/config"
	"github.com/gopher-launch/concoct/internal/gitrepo"
	"github.com/gopher-launch/concoct/internal/integration"
	"github.com/gopher-launch/concoct/internal/orchestration"
	"github.com/gopher-launch/concoct/internal/planning"
	"github.com/gopher-launch/concoct/internal/prompt"
	"github.com/gopher-launch/concoct/internal/runstate"
	"github.com/gopher-launch/concoct/internal/workflow"
)

const terminationGrace = 2 * time.Second

type Options struct {
	DryRun           bool
	Override         config.Overrides
	SelectedPlan     string
	LocalIntegration bool
	AttemptID        string
	// PredecessorInvocationID links this observation to a prior authorized
	// action when a run gate supplied that correlation. It grants neither retry
	// nor replay authority.
	PredecessorInvocationID string
	// beforeLaunch is a test seam for changing covered evidence after the
	// invocation record is created but before the final authorization check.
	beforeLaunch func(string, Prepared) error
}

type Prepared struct {
	Resolution              orchestration.Resolution
	Action                  orchestration.Action
	Prompt                  []byte
	Composition             prompt.Composition
	Settings                config.Resolved
	Invocation              adapter.Invocation
	Schema                  []byte
	ConfigDigest            string
	Override                config.Overrides
	Direct                  bool
	Plan                    *planning.Session
	InitialHead             string
	InitialFiles            map[string]string
	UserConfig              string
	PredecessorInvocationID string
}

const supervisionAppendix = `

## Executable supervision boundary

This prompt's role instructions above remain the unchanged semantic authority.
Author and verify the complete role-owned candidate, but do not write Git
metadata, create a commit, or invoke the final Concoct completion command. The
outer Concoct executable will validate the candidate through the canonical
completion boundary and create any required transition commit. Return a
completed structured outcome only when the candidate is ready for that exact
validation; blockers, decisions, and failures must retain their honest
non-completion outcome.
`

type Metadata struct {
	InvocationID            string `json:"invocation_id"`
	PredecessorInvocationID string `json:"predecessor_invocation_id,omitempty"`
	Action                  string `json:"action"`
	Role                    string `json:"role"`
	Adapter                 string `json:"adapter"`
	AdapterVersion          string `json:"adapter_version,omitempty"`
	Model                   string `json:"model,omitempty"`
	ModelSource             string `json:"model_source"`
	Reasoning               string `json:"reasoning"`
	ReasoningSource         string `json:"reasoning_source"`
	Timeout                 string `json:"timeout"`
	TimeoutSource           string `json:"timeout_source"`
	Safety                  string `json:"safety"`
	Command                 string `json:"command"`
	PromptFile              string `json:"prompt_file,omitempty"`
	StartedAt               string `json:"started_at"`
	FinishedAt              string `json:"finished_at,omitempty"`
	Duration                string `json:"duration,omitempty"`
	PromptBytes             int    `json:"prompt_bytes"`
	CompositionFile         string `json:"composition_file,omitempty"`
	ConfigDigest            string `json:"config_digest"`
}

type Reconciliation struct {
	InvocationDisposition string            `json:"invocation_disposition"`
	ResultAccepted        bool              `json:"result_accepted"`
	OutcomeClass          string            `json:"outcome_class,omitempty"`
	ResultError           string            `json:"result_error,omitempty"`
	ObservedState         workflow.State    `json:"observed_state"`
	ObservedNext          string            `json:"observed_next"`
	RetrySafe             bool              `json:"retry_safe"`
	FinishedAt            string            `json:"finished_at"`
	CostDisposition       string            `json:"cost_disposition"`
	FailedInvariant       string            `json:"failed_invariant,omitempty"`
	ArtifactReusability   map[string]string `json:"artifact_reusability,omitempty"`
	Recovery              string            `json:"recovery,omitempty"`
}

type Result struct {
	Prepared       Prepared
	RecordPath     string
	Reconciliation Reconciliation
	Outcome        orchestration.Outcome
	Facts          orchestration.DurableFacts
	Measurement    adapter.EventEvidence
}

type retentionRecord struct {
	path   string
	closed time.Time
	size   int64
}

func Prepare(root string, options Options) (prepared Prepared, resultErr error) {
	if options.DryRun && options.SelectedPlan != "" {
		return Prepared{}, fmt.Errorf("selected planning cannot be prepared as a dry run")
	}
	var session *planning.Session
	defer func() {
		if resultErr != nil && session != nil {
			_ = session.Rollback()
		}
	}()
	resolution, err := orchestration.Resolve(root)
	if options.SelectedPlan != "" {
		session, err = planning.Start(root, options.SelectedPlan)
		resolution = orchestration.Resolution{Kind: "task-planning", Role: "task-planner", PromptCommand: "plan", Command: "concoct plan " + options.SelectedPlan, Executable: true}
	}
	if err != nil {
		return Prepared{}, err
	}
	if !resolution.Executable {
		return Prepared{Resolution: resolution}, fmt.Errorf("execution refused: %s", resolution.Refusal)
	}
	if options.SelectedPlan == "" && resolution.Kind == "product-owner-next" {
		if decision, loadErr := runstate.LoadDecision(root); loadErr == nil && (decision.Status == "proposed" || decision.Status == "approved") {
			continuation := "concoct run --approve next"
			if decision.Status == "approved" && decision.Decision.Selection != "" {
				continuation = "concoct plan " + decision.Decision.Selection
			}
			return Prepared{Resolution: orchestration.Resolution{Command: continuation, Refusal: "a retained Product Owner decision already controls the ready-state continuation"}}, fmt.Errorf("execution refused: retained Product Owner decision; run %s", continuation)
		} else if loadErr != nil && !os.IsNotExist(loadErr) {
			return Prepared{}, fmt.Errorf("load retained Product Owner decision: %w", loadErr)
		}
	}
	attemptID := options.AttemptID
	if attemptID == "" {
		attemptID, err = orchestration.NewID()
		if err != nil {
			return Prepared{}, err
		}
	}
	var action orchestration.Action
	if options.SelectedPlan != "" {
		action, err = orchestration.AuthorizePlanning(root, options.SelectedPlan, attemptID)
	} else {
		action, err = orchestration.Authorize(root, resolution.Kind, attemptID)
	}
	if err != nil {
		return Prepared{}, err
	}

	prepared = Prepared{Resolution: resolution, Action: action, Override: options.Override, Direct: resolution.Kind == "integration", Plan: session, PredecessorInvocationID: options.PredecessorInvocationID}
	if session != nil {
		prepared.Prompt, err = session.Render()
		if err == nil {
			prepared.Composition.Append("task-planning-session", "planning.Session.Render", prompt.InclusionFull, prepared.Prompt)
		}
	} else if !prepared.Direct {
		prepared.Composition, err = prompt.RenderComposition(root, prompt.Request{Command: resolution.PromptCommand})
		if err != nil {
			return Prepared{}, err
		}
		prepared.Prompt = prepared.Composition.Bytes()
	}
	if err == nil && !prepared.Direct && resolution.Kind != "product-owner-next" {
		prepared.Composition.Append("executable-supervision", "built-in:supervision-appendix", prompt.InclusionFull, []byte(supervisionAppendix))
		prepared.Prompt = prepared.Composition.Bytes()
	}
	spec, ok := adapter.Find(firstNonEmpty(options.Override.Adapter, "codex"))
	if !ok && options.Override.Adapter != "" {
		return Prepared{}, fmt.Errorf("unsupported adapter %q", options.Override.Adapter)
	}
	if !ok {
		spec, _ = adapter.Find("codex")
	}
	prepared.Settings, err = config.Resolve(root, resolution.Role, options.Override, spec.Defaults)
	if err != nil {
		return Prepared{}, err
	}
	if selected, found := adapter.Find(prepared.Settings.Adapter.Value); found {
		if selected.Name != spec.Name {
			prepared.Settings, err = config.Resolve(root, resolution.Role, options.Override, selected.Defaults)
			if err != nil {
				return Prepared{}, err
			}
		}
	} else {
		return Prepared{}, fmt.Errorf("unsupported adapter %q", prepared.Settings.Adapter.Value)
	}
	prepared.ConfigDigest, err = resolvedConfigDigest(prepared.Settings)
	if err != nil {
		return Prepared{}, err
	}
	prepared.UserConfig, err = userConfigDigest()
	if err != nil {
		return Prepared{}, err
	}
	if prepared.Direct {
		return prepared, nil
	}
	if repo, ok, openErr := gitrepo.Open(root); openErr != nil {
		return Prepared{}, openErr
	} else if ok {
		prepared.InitialHead, err = repo.Head()
		if err != nil {
			// A freshly initialized repository may have an unborn branch. There
			// is no commit identity to protect until planning creates its base.
			if _, statusErr := repo.StatusEntries(); statusErr != nil {
				return Prepared{}, err
			}
		}
	} else {
		prepared.InitialFiles, err = snapshotFiles(root)
		if err != nil {
			return Prepared{}, err
		}
	}

	base := filepath.Join(root, ".concoct", "runtime", "invocations", action.Correlation.InvocationID)
	prepared.Schema, err = adapter.Schema(action)
	if err != nil {
		return Prepared{}, err
	}
	compositionBytes, marshalErr := json.Marshal(prepared.Composition)
	if marshalErr != nil {
		return Prepared{}, marshalErr
	}
	reserved := int64(len(prepared.Prompt)+len(prepared.Schema)+len(compositionBytes)) + 2*prepared.Settings.Retention.MaxLogBytes + 128*1024
	if reserved > prepared.Settings.Retention.MaxTotal {
		return Prepared{}, fmt.Errorf("resolved prompt and bounded invocation record require %d bytes, exceeding max-total-bytes %d", reserved, prepared.Settings.Retention.MaxTotal)
	}
	prepared.Invocation, err = adapter.Resolve(root, action, prepared.Settings, filepath.Join(base, "outcome-schema.json"), filepath.Join(base, "adapter-result.json"))
	if err != nil {
		return Prepared{}, err
	}
	return prepared, nil
}

func Describe(prepared Prepared) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Action: %s\nRole: %s\nWorkflow command: %s\n", prepared.Resolution.Kind, prepared.Resolution.Role, prepared.Resolution.Command)
	if prepared.Direct {
		fmt.Fprintln(&b, "Adapter: none (direct executable authority)")
		fmt.Fprintln(&b, "Safety posture: existing integration checks, recovery boundaries, and push policy")
		fmt.Fprintln(&b, "Command: concoct integrate")
		return b.String()
	}
	fmt.Fprintf(&b, "Adapter: %s (%s)\n", prepared.Settings.Adapter.Value, prepared.Settings.Adapter.Source)
	fmt.Fprintf(&b, "Model: %s (%s)\n", displayDefault(prepared.Settings.Model.Value), prepared.Settings.Model.Source)
	fmt.Fprintf(&b, "Reasoning: %s (%s)\n", prepared.Settings.Reasoning.Value, prepared.Settings.Reasoning.Source)
	fmt.Fprintf(&b, "Timeout: %s (%s)\n", prepared.Settings.Timeout, prepared.Settings.TimeoutSource)
	if budget := prepared.Settings.Budget; budget.WarnElapsed > 0 {
		fmt.Fprintf(&b, "Warning budgets: elapsed=%s activity=%d command-output-bytes=%d input-tokens=%d output-tokens=%d (%s)\n", budget.WarnElapsed, budget.WarnActivity.Value, budget.WarnCommandOutput.Value, budget.WarnInputTokens.Value, budget.WarnOutputTokens.Value, budget.WarnElapsedSource)
	}
	if budget := prepared.Settings.Budget; budget.HardElapsed > 0 || budget.HardActivity.Value > 0 || budget.HardCommandOutput.Value > 0 {
		fmt.Fprintf(&b, "Hard budgets (live enforceable): elapsed=%s activity=%d command-output-bytes=%d\n", budget.HardElapsed, budget.HardActivity.Value, budget.HardCommandOutput.Value)
	}
	fmt.Fprintf(&b, "Safety posture: %s\nCommand: %s\nPrompt: stdin (%d exact rendered bytes)\n", prepared.Invocation.Safety, adapter.DisplayCommand(prepared.Invocation), len(prepared.Prompt))
	return b.String()
}

func Run(ctx context.Context, root string, options Options, input io.Reader, progress io.Writer) (Result, error) {
	result, err := run(ctx, root, options, input, progress)
	if err == nil && result.Reconciliation.ResultAccepted && result.Reconciliation.OutcomeClass != string(orchestration.Completed) {
		return result, fmt.Errorf("action returned accepted non-completion outcome %s", result.Reconciliation.OutcomeClass)
	}
	return result, err
}

// RunAccepted exposes an accepted structured non-completion outcome to the
// lifecycle coordinator while preserving Run's one-shot refusal contract.
func RunAccepted(ctx context.Context, root string, options Options, input io.Reader, progress io.Writer) (Result, error) {
	return run(ctx, root, options, input, progress)
}

func run(ctx context.Context, root string, options Options, input io.Reader, progress io.Writer) (Result, error) {
	prepared, err := Prepare(root, options)
	if err != nil {
		return Result{}, err
	}
	if options.DryRun {
		return Result{Prepared: prepared}, nil
	}
	record, err := createRecord(root, prepared)
	if err != nil {
		if prepared.Plan != nil {
			_ = prepared.Plan.Rollback()
		}
		return Result{}, err
	}
	result := Result{Prepared: prepared, RecordPath: record}
	disposition := "failed"
	var runErr error
	if options.beforeLaunch != nil {
		runErr = options.beforeLaunch(root, prepared)
	}
	if runErr == nil {
		runErr = authorizationStable(root, options.Override, prepared)
	}
	if runErr != nil {
		disposition = "authorization-changed"
		if prepared.Plan != nil {
			_ = prepared.Plan.Rollback()
		}
	} else if prepared.Direct {
		disposition, runErr = runDirect(ctx, root, record, prepared, options.LocalIntegration, input, progress)
		if runErr == nil && disposition == "completed" {
			outcome := orchestration.Outcome{ProtocolVersion: orchestration.ProtocolVersion, Correlation: prepared.Action.Correlation, Class: orchestration.Completed, Summary: "direct integration completed"}
			runErr = orchestration.WriteAtomicResult(filepath.Join(record, "result.json"), outcome)
		}
	} else {
		disposition, result.Measurement, runErr = runAdapter(ctx, record, prepared, progress)
		if disposition == "startup-failed" && prepared.Plan != nil {
			_ = prepared.Plan.Rollback()
		}
		if runErr == nil && disposition == "completed" {
			outcome, candidateErr := orchestration.ReadResult(filepath.Join(record, "result.json"))
			if candidateErr == nil {
				candidateErr = orchestration.ValidateCandidate(prepared.Action, outcome)
			}
			if candidateErr == nil {
				candidateErr = validateSupervisedEffects(root, prepared)
			}
			if candidateErr == nil && outcome.Class == orchestration.Completed && prepared.Resolution.Kind != "product-owner-next" {
				candidateErr = finalizeSupervised(root, prepared)
			}
			if candidateErr != nil {
				disposition, runErr = "finalization-failed", candidateErr
			}
		}
	}
	reconcileErr := reconcile(root, record, prepared, disposition, runErr, &result)
	if pruneErr := Prune(root, prepared.Settings.Retention, prepared.Action.Correlation.InvocationID); pruneErr != nil && reconcileErr == nil {
		reconcileErr = pruneErr
	}
	if runErr != nil && reconcileErr != nil {
		return result, fmt.Errorf("%v; reconciliation: %w", runErr, reconcileErr)
	}
	if runErr != nil {
		return result, runErr
	}
	if reconcileErr != nil {
		return result, reconcileErr
	}
	return result, nil
}

func runDirect(ctx context.Context, root, record string, prepared Prepared, localOnly bool, input io.Reader, progress io.Writer) (string, error) {
	stdoutFile, err := privateFile(filepath.Join(record, "stdout.log"))
	if err != nil {
		return "startup-failed", err
	}
	defer stdoutFile.Close()
	stderrFile, err := privateFile(filepath.Join(record, "stderr.log"))
	if err != nil {
		return "startup-failed", err
	}
	defer stderrFile.Close()
	output := newBoundedRedactedWriter(stdoutFile, &lockedWriter{writer: progress}, prepared.Settings.Retention.MaxLogBytes)
	timerCtx, cancel := context.WithTimeout(ctx, prepared.Settings.Timeout)
	defer cancel()
	err = integration.RunContextOptions(timerCtx, root, "", input, output, integration.Options{LocalOnly: localOnly})
	output.finish()
	if err != nil {
		if errors.Is(timerCtx.Err(), context.DeadlineExceeded) {
			return "timed-out", fmt.Errorf("direct integration timed out: %w", err)
		}
		if errors.Is(timerCtx.Err(), context.Canceled) {
			return "cancelled", fmt.Errorf("direct integration cancelled: %w", err)
		}
		return "failed", err
	}
	return "completed", nil
}

func runAdapter(ctx context.Context, record string, prepared Prepared, progress io.Writer) (string, adapter.EventEvidence, error) {
	eventsFile, err := privateFile(filepath.Join(record, "stdout.log"))
	if err != nil {
		return "startup-failed", adapter.EventEvidence{}, err
	}
	defer eventsFile.Close()
	var rawEvents io.Writer = io.Discard
	if prepared.Settings.Retention.RawEvents {
		rawEvents = eventsFile
	}
	stderrFile, err := privateFile(filepath.Join(record, "stderr.log"))
	if err != nil {
		return "startup-failed", adapter.EventEvidence{}, err
	}
	defer stderrFile.Close()
	max := prepared.Settings.Retention.MaxLogBytes
	safeProgress := &lockedWriter{writer: progress}
	stdout := newStructuredCapture(rawEvents, safeProgress, max)
	stderr := newBoundedRedactedWriter(stderrFile, safeProgress, max)
	if version, err := adapter.Version(prepared.Invocation.Executable); err == nil {
		if err := updateMetadataVersion(filepath.Join(record, "metadata.json"), version); err != nil {
			return "startup-failed", stdout.close(), err
		}
	}

	cmd := exec.Command(prepared.Invocation.Executable, prepared.Invocation.Args...)
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = prepared.Invocation.Root, prepared.Invocation.Environment, bytes.NewReader(prepared.Prompt), stdout, stderr
	configureProcess(cmd)
	if err := cmd.Start(); err != nil {
		return "startup-failed", stdout.close(), fmt.Errorf("start %s adapter: %w", prepared.Invocation.Adapter, err)
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	timerCtx, cancel := context.WithTimeout(ctx, prepared.Settings.Timeout)
	defer cancel()
	budgetTicker := time.NewTicker(25 * time.Millisecond)
	defer budgetTicker.Stop()
	started := time.Now()
	warnings := warningTracker{}
	closeMeasurement := func() adapter.EventEvidence {
		measurement := stdout.close()
		warnings.observe(measurement, prepared.Settings.Budget, time.Since(started))
		warnings.appendTo(&measurement)
		appendTerminalTokenWarnings(&measurement, prepared.Settings.Budget)
		return measurement
	}
	stopForBudget := func(event adapter.BudgetEvent) (string, adapter.EventEvidence, error) {
		terminateProcess(cmd)
		select {
		case <-wait:
		case <-time.After(terminationGrace):
			killProcess(cmd)
			<-wait
		}
		measurement := closeMeasurement()
		measurement.Budgets = append(measurement.Budgets, event)
		stderr.finish()
		return "budget-exhausted", measurement, fmt.Errorf("%s adapter exhausted hard %s budget: observed %d %s (limit %d)", prepared.Invocation.Adapter, event.Dimension, event.Observed, event.Unit, event.Limit)
	}
	select {
	case err := <-wait:
		measurement := closeMeasurement()
		stderr.finish()
		if err != nil {
			return "nonzero-exit", measurement, fmt.Errorf("%s adapter exited unsuccessfully: %w", prepared.Invocation.Adapter, err)
		}
	case <-timerCtx.Done():
		disposition := "cancelled"
		if errors.Is(timerCtx.Err(), context.DeadlineExceeded) {
			disposition = "timed-out"
		}
		terminateProcess(cmd)
		select {
		case <-wait:
		case <-time.After(terminationGrace):
			killProcess(cmd)
			<-wait
		}
		measurement := closeMeasurement()
		stderr.finish()
		return disposition, measurement, fmt.Errorf("%s adapter %s", prepared.Invocation.Adapter, strings.ReplaceAll(disposition, "-", " "))
	case <-budgetTicker.C:
		for {
			observed := stdout.evidence()
			elapsed := time.Since(started)
			warnings.observe(observed, prepared.Settings.Budget, elapsed)
			if limit := prepared.Settings.Budget.HardElapsed; limit > 0 && elapsed >= limit {
				return stopForBudget(adapter.BudgetEvent{Dimension: "elapsed", Kind: "hard", Limit: limit.Milliseconds(), Observed: elapsed.Milliseconds(), Unit: "milliseconds", Source: prepared.Settings.Budget.HardElapsedSource, Evaluation: "live", Enforceable: true})
			}
			if limit := prepared.Settings.Budget.HardActivity; limit.Value > 0 && observed.ActivityEvents >= limit.Value {
				return stopForBudget(adapter.BudgetEvent{Dimension: "activity", Kind: "hard", Limit: limit.Value, Observed: observed.ActivityEvents, Unit: "events", Source: limit.Source, Evaluation: "live", Enforceable: true})
			}
			if limit := prepared.Settings.Budget.HardCommandOutput; limit.Value > 0 && observed.Commands.OutputBytes >= limit.Value {
				return stopForBudget(adapter.BudgetEvent{Dimension: "command-output", Kind: "hard", Limit: limit.Value, Observed: observed.Commands.OutputBytes, Unit: "bytes", Source: limit.Source, Evaluation: "live", Enforceable: true})
			}
			select {
			case err := <-wait:
				measurement := closeMeasurement()
				stderr.finish()
				if err != nil {
					return "nonzero-exit", measurement, fmt.Errorf("%s adapter exited unsuccessfully: %w", prepared.Invocation.Adapter, err)
				}
				goto completed
			case <-timerCtx.Done():
				terminateProcess(cmd)
				select {
				case <-wait:
				case <-time.After(terminationGrace):
					killProcess(cmd)
					<-wait
				}
				measurement := closeMeasurement()
				stderr.finish()
				disposition := "cancelled"
				if errors.Is(timerCtx.Err(), context.DeadlineExceeded) {
					disposition = "timed-out"
				}
				return disposition, measurement, fmt.Errorf("%s adapter %s", prepared.Invocation.Adapter, strings.ReplaceAll(disposition, "-", " "))
			case <-budgetTicker.C:
			}
		}
	}
completed:
	measurement := closeMeasurement()
	if event := exceededHardBudget(measurement, prepared.Settings.Budget, time.Since(started)); event != nil {
		measurement.Budgets = append(measurement.Budgets, *event)
		return "budget-exhausted", measurement, fmt.Errorf("%s adapter exhausted hard %s budget: observed %d %s (limit %d)", prepared.Invocation.Adapter, event.Dimension, event.Observed, event.Unit, event.Limit)
	}
	candidate := filepath.Join(record, "adapter-result.json")
	info, statErr := os.Lstat(candidate)
	if statErr == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
		return "malformed-result", measurement, fmt.Errorf("adapter result must be a regular non-symlink file")
	}
	if statErr != nil && !os.IsNotExist(statErr) {
		return "malformed-result", measurement, statErr
	}
	if err := os.Chmod(candidate, 0o600); err != nil && !os.IsNotExist(err) {
		return "malformed-result", measurement, fmt.Errorf("secure adapter result: %w", err)
	}
	outcome, err := readCandidate(candidate)
	if err != nil {
		return "missing-or-malformed-result", measurement, err
	}
	if err := orchestration.WriteAtomicResult(filepath.Join(record, "result.json"), outcome); err != nil {
		return "duplicate-result", measurement, err
	}
	return "completed", measurement, nil
}

func exceededHardBudget(measurement adapter.EventEvidence, budget config.Budget, elapsed time.Duration) *adapter.BudgetEvent {
	if limit := budget.HardElapsed; limit > 0 && elapsed >= limit {
		return &adapter.BudgetEvent{Dimension: "elapsed", Kind: "hard", Limit: limit.Milliseconds(), Observed: elapsed.Milliseconds(), Unit: "milliseconds", Source: budget.HardElapsedSource, Evaluation: "live", Enforceable: true}
	}
	if limit := budget.HardActivity; limit.Value > 0 && measurement.ActivityEvents >= limit.Value {
		return &adapter.BudgetEvent{Dimension: "activity", Kind: "hard", Limit: limit.Value, Observed: measurement.ActivityEvents, Unit: "events", Source: limit.Source, Evaluation: "live", Enforceable: true}
	}
	if limit := budget.HardCommandOutput; limit.Value > 0 && measurement.Commands.OutputBytes >= limit.Value {
		return &adapter.BudgetEvent{Dimension: "command-output", Kind: "hard", Limit: limit.Value, Observed: measurement.Commands.OutputBytes, Unit: "bytes", Source: limit.Source, Evaluation: "live", Enforceable: true}
	}
	return nil
}

// warningTracker records each live-observable threshold once. It is deliberately
// kept outside the decoder so warnings survive adapter failure, cancellation,
// timeout, and hard-budget termination just as decoded activity does.
type warningTracker struct {
	events map[string]adapter.BudgetEvent
}

func (w *warningTracker) observe(measurement adapter.EventEvidence, budget config.Budget, elapsed time.Duration) {
	w.append("elapsed", budget.WarnElapsed.Milliseconds(), elapsed.Milliseconds(), "milliseconds", budget.WarnElapsedSource, true)
	w.append("activity", budget.WarnActivity.Value, measurement.ActivityEvents, "events", budget.WarnActivity.Source, true)
	w.append("command-output", budget.WarnCommandOutput.Value, measurement.Commands.OutputBytes, "bytes", budget.WarnCommandOutput.Source, true)
}

func (w *warningTracker) append(dimension string, limit, observed int64, unit, source string, enforceable bool) {
	if limit <= 0 || observed < limit {
		return
	}
	if w.events == nil {
		w.events = map[string]adapter.BudgetEvent{}
	}
	if _, exists := w.events[dimension]; !exists {
		w.events[dimension] = adapter.BudgetEvent{Dimension: dimension, Kind: "warning", Limit: limit, Observed: observed, Unit: unit, Source: source, Evaluation: "live", Enforceable: enforceable}
	}
}

func (w *warningTracker) appendTo(measurement *adapter.EventEvidence) {
	for _, dimension := range []string{"elapsed", "activity", "command-output"} {
		if event, exists := w.events[dimension]; exists {
			measurement.Budgets = append(measurement.Budgets, event)
		}
	}
}

func appendTerminalTokenWarnings(measurement *adapter.EventEvidence, budget config.Budget) {
	appendWarning := func(dimension string, limit, observed int64, unit, source string) {
		if limit > 0 && observed >= limit {
			measurement.Budgets = append(measurement.Budgets, adapter.BudgetEvent{Dimension: dimension, Kind: "warning", Limit: limit, Observed: observed, Unit: unit, Source: source, Evaluation: "terminal-only", Enforceable: false})
		}
	}
	if measurement.Usage.Input != nil {
		appendWarning("input", budget.WarnInputTokens.Value, *measurement.Usage.Input, "tokens", budget.WarnInputTokens.Source)
	}
	if measurement.Usage.Output != nil {
		appendWarning("output", budget.WarnOutputTokens.Value, *measurement.Usage.Output, "tokens", budget.WarnOutputTokens.Source)
	}
}

// structuredCapture independently bounds retained raw JSONL while keeping the
// decoder fed with every byte.  Display history is deliberately not a parser
// input: a long stream may truncate retained/displayed progress without losing
// a later terminal usage event or result reconciliation.
type structuredCapture struct {
	mu                sync.Mutex
	raw               io.Writer
	progress          io.Writer
	max               int64
	written           int64
	truncated         bool
	pending           []byte
	decoder           adapter.StreamDecoder
	lastActivity      string
	progressWritten   int64
	progressTruncated bool
	closed            bool
}

func newStructuredCapture(raw, progress io.Writer, max int64) *structuredCapture {
	return &structuredCapture{raw: raw, progress: progress, max: max, decoder: adapter.NewStreamDecoder(max)}
}

func (c *structuredCapture) Write(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	if _, err := c.decoder.Write(data); err != nil {
		return 0, err
	}
	c.pending = append(c.pending, data...)
	for {
		index := bytes.IndexByte(c.pending, '\n')
		if index < 0 {
			break
		}
		if err := c.writeRaw(c.pending[:index+1]); err != nil {
			return 0, err
		}
		c.pending = c.pending[index+1:]
	}
	if int64(len(c.pending)) >= c.max {
		if err := c.writeRaw(c.pending); err != nil {
			return 0, err
		}
		c.pending = nil
	}
	evidence := c.decoder.Evidence()
	if evidence.CurrentActivity != "" && evidence.CurrentActivity != c.lastActivity {
		c.writeProgress("activity=" + evidence.CurrentActivity)
		c.lastActivity = evidence.CurrentActivity
	}
	return len(data), nil
}

func (c *structuredCapture) evidence() adapter.EventEvidence {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.decoder.Evidence()
}

func (c *structuredCapture) writeRaw(data []byte) error {
	remaining := c.max - c.written
	if remaining <= 0 {
		c.truncated = true
		return nil
	}
	clean := []byte(redact(string(data)))
	if int64(len(clean)) > remaining {
		clean = clean[:remaining]
		c.truncated = true
	}
	if len(clean) == 0 {
		return nil
	}
	n, err := c.raw.Write(clean)
	c.written += int64(n)
	return err
}

func (c *structuredCapture) close() adapter.EventEvidence {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return c.decoder.Close()
	}
	c.closed = true
	if len(c.pending) > 0 {
		_ = c.writeRaw(c.pending)
		c.pending = nil
	}
	evidence := c.decoder.Close()
	if c.truncated {
		c.decoder.AddDiagnostic("raw structured event retention truncated")
		evidence = c.decoder.Close()
		_, _ = c.raw.Write([]byte("\n[concoct: structured event log truncated]\n"))
	}
	if c.progressTruncated && c.progress != nil {
		_, _ = c.progress.Write([]byte("[concoct: structured progress truncated; latest activity=" + redact(evidence.CurrentActivity) + "]\n"))
	}
	return evidence
}

// writeProgress applies the same redaction boundary as logs and independently
// bounds terminal display. It never influences event decoding or retained
// measurement, so later usage/result evidence survives display truncation.
func (c *structuredCapture) writeProgress(value string) {
	if c.progress == nil {
		return
	}
	data := []byte(redact(value + "\n"))
	if c.progressWritten+int64(len(data)) > c.max {
		c.progressTruncated = true
		return
	}
	_, _ = c.progress.Write(data)
	c.progressWritten += int64(len(data))
}

func readCandidate(path string) (orchestration.Outcome, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return orchestration.Outcome{}, fmt.Errorf("adapter exited without a structured result")
	}
	if err != nil {
		return orchestration.Outcome{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var outcome orchestration.Outcome
	if err := dec.Decode(&outcome); err != nil {
		return outcome, fmt.Errorf("malformed adapter result: %w", err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return outcome, fmt.Errorf("malformed adapter result: trailing JSON data")
	}
	return outcome, nil
}

func reconcile(root, record string, prepared Prepared, disposition string, runErr error, target *Result) error {
	reconciliation := Reconciliation{InvocationDisposition: disposition, FinishedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	report := workflow.Detect(root)
	reconciliation.ObservedState, reconciliation.ObservedNext = report.State, report.Next
	var validationErr error
	if currentUserConfig, err := userConfigDigest(); err != nil {
		validationErr = err
	} else if currentUserConfig != prepared.UserConfig {
		validationErr = fmt.Errorf("user execution configuration changed after authorization")
	} else if prepared.Resolution.Kind != "development" {
		validationErr = configurationStable(root, prepared.Override, prepared)
	}
	if outcome, err := orchestration.ReadResult(filepath.Join(record, "result.json")); err == nil {
		target.Outcome = outcome
		reconciliation.OutcomeClass = string(outcome.Class)
		if validationErr == nil && runErr == nil {
			if prepared.Direct {
				target.Facts, validationErr = orchestration.ValidateOutcome(root, prepared.Action, outcome)
			} else {
				target.Facts, validationErr = orchestration.ValidateSupervisedOutcome(root, prepared.Action, outcome)
			}
		}
		if validationErr == nil && runErr == nil {
			reconciliation.ResultAccepted = true
			if prepared.Resolution.Kind == "product-owner-next" && outcome.Class == orchestration.Completed {
				decision, decisionErr := runstate.NewDecision(outcome.ProductDecision, prepared.Action.Evidence, prepared.Action.Correlation)
				if decisionErr == nil {
					decisionErr = runstate.CreateDecision(root, decision)
				}
				if decisionErr != nil {
					reconciliation.ResultAccepted = false
					validationErr = fmt.Errorf("retain Product Owner decision: %w", decisionErr)
				}
			}
		}
	} else if !os.IsNotExist(err) {
		validationErr = err
	}
	if validationErr != nil {
		reconciliation.ResultError = validationErr.Error()
	}
	if runErr != nil && reconciliation.ResultError == "" {
		reconciliation.ResultError = runErr.Error()
	}
	if reconciliation.ResultAccepted {
		reconciliation.CostDisposition = "accepted"
	} else {
		reconciliation.CostDisposition = "wasted"
	}
	if disposition == "finalization-failed" {
		reconciliation.FailedInvariant = reconciliation.ResultError
		reconciliation.ArtifactReusability = map[string]string{"role-owned-candidate": "preserved; inspect and repair the named invariant", "structured-outcome": "not reusable after repository evidence changes"}
		reconciliation.Recovery = "repair the preserved candidate manually, run the role's completion command, then authorize a fresh workflow action only if semantic work remains"
	} else if disposition == "budget-exhausted" {
		reconciliation.ArtifactReusability = map[string]string{"partial-measurement": "reusable", "late-structured-outcome": "rejected", "role-owned-candidate": "inspect before reuse; completeness is not established"}
		reconciliation.Recovery = "inspect preserved evidence and current workflow state before choosing a fresh retry or manual completion"
	}
	current, _, snapshotErr := orchestration.Snapshot(root)
	reconciliation.RetrySafe = snapshotErr == nil && current.Digest == prepared.Action.Evidence.Digest
	if !prepared.Direct {
		// Measurements are supplied by the live stream decoder.  Retained event
		// bytes are intentionally only diagnostic evidence, so their independent
		// bound must never redefine what was observed.
		if err := writeJSON(filepath.Join(record, "measurement.json"), target.Measurement); err != nil {
			return err
		}
	}
	if err := updateMetadataTiming(filepath.Join(record, "metadata.json"), reconciliation.FinishedAt); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(record, "reconciliation.json"), reconciliation); err != nil {
		return err
	}
	target.Reconciliation = reconciliation
	if report.OperationalError != nil {
		return report.OperationalError
	}
	if report.State == workflow.Invalid {
		return fmt.Errorf("post-run workflow state is invalid: %s", strings.Join(report.Diagnostics, "; "))
	}
	if validationErr != nil {
		return fmt.Errorf("structured result was not accepted: %w", validationErr)
	}
	if runErr == nil && !reconciliation.ResultAccepted {
		return fmt.Errorf("execution produced no accepted structured result")
	}
	return nil
}

// Metrics returns the privacy-preserving record intended for comparison. It
// never reads prompt bytes or raw adapter output.
func Metrics(root, id string) ([]byte, error) {
	base := filepath.Join(root, ".concoct", "runtime", "invocations")
	record, err := selectRecord(base, id)
	if err != nil {
		return nil, err
	}
	type metricRecord struct {
		Metadata       json.RawMessage `json:"metadata,omitempty"`
		Composition    json.RawMessage `json:"prompt_composition,omitempty"`
		Measurement    json.RawMessage `json:"measurement,omitempty"`
		Reconciliation json.RawMessage `json:"reconciliation,omitempty"`
	}
	var out metricRecord
	for _, item := range []struct {
		name string
		dst  *json.RawMessage
	}{
		{"metadata.json", &out.Metadata}, {"prompt-composition.json", &out.Composition}, {"measurement.json", &out.Measurement}, {"reconciliation.json", &out.Reconciliation},
	} {
		data, readErr := os.ReadFile(filepath.Join(record, item.name))
		if os.IsNotExist(readErr) {
			continue
		}
		if readErr != nil {
			return nil, readErr
		}
		if !json.Valid(data) {
			return nil, fmt.Errorf("retained %s is not valid JSON", item.name)
		}
		if item.name == "measurement.json" {
			data, readErr = compactMeasurement(data)
			if readErr != nil {
				return nil, readErr
			}
		}
		*item.dst = append((*item.dst)[:0], data...)
	}
	return json.MarshalIndent(out, "", "  ")
}

// compactMeasurement removes progress and diagnostic text from the
// metrics-first surfaces. New measurements contain only selected lifecycle
// labels, but this also prevents older retained records from leaking arbitrary
// adapter payloads through default inspection or comparison export.
func compactMeasurement(data []byte) ([]byte, error) {
	var evidence adapter.EventEvidence
	if err := json.Unmarshal(data, &evidence); err != nil {
		return nil, fmt.Errorf("decode retained measurement: %w", err)
	}
	evidence.Progress = nil
	evidence.Diagnostics = nil
	return json.MarshalIndent(evidence, "", "  ")
}

func updateMetadataTiming(path, finished string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("metadata is not a regular non-symlink file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}
	start, err := time.Parse(time.RFC3339Nano, metadata.StartedAt)
	if err != nil {
		return err
	}
	end, err := time.Parse(time.RFC3339Nano, finished)
	if err != nil {
		return err
	}
	metadata.FinishedAt, metadata.Duration = finished, end.Sub(start).String()
	data, err = json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func updateMetadataVersion(path, version string) error {
	if version == "" {
		return nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("metadata is not a regular non-symlink file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}
	metadata.AdapterVersion = version
	data, err = json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func validateSupervisedEffects(root string, prepared Prepared) error {
	if prepared.Resolution.Kind == "product-owner-next" {
		// Product Owner completion is separately required to preserve the exact
		// authorization evidence and never enters a Git finalization boundary.
		return nil
	}
	repo, ok, err := gitrepo.Open(root)
	if err != nil {
		return err
	}
	var paths []string
	if ok {
		head, headErr := repo.Head()
		if headErr != nil {
			return headErr
		}
		if prepared.InitialHead != "" && head != prepared.InitialHead {
			return fmt.Errorf("supervised %s action created or changed a Git commit; outer finalization refused", prepared.Resolution.Kind)
		}
		entries, statusErr := repo.StatusEntries()
		if statusErr != nil {
			return statusErr
		}
		for _, entry := range entries {
			paths = append(paths, entry.Paths...)
		}
	} else {
		current, snapshotErr := snapshotFiles(root)
		if snapshotErr != nil {
			return snapshotErr
		}
		paths = changedFiles(prepared.InitialFiles, current)
	}
	for _, raw := range paths {
		path := filepath.ToSlash(raw)
		switch prepared.Resolution.Kind {
		case "task-planning":
			if path != ".concoct/current/task-plan.md" && path != ".concoct/current/notes.md" && path != ".concoct/roadmap.md" {
				return fmt.Errorf("Task Planner action contains forbidden path %s", path)
			}
		case "development":
			if strings.HasPrefix(path, ".concoct/current/review-") || path == ".concoct/roadmap.md" || path == ".concoct/capabilities.md" || strings.HasPrefix(path, ".concoct/archive/") {
				return fmt.Errorf("Developer action contains forbidden workflow path %s", path)
			}
		case "independent-review":
			if !regexp.MustCompile(`^\.concoct/current/review-[0-9]{2}\.md$`).MatchString(path) {
				return fmt.Errorf("Reviewer action contains forbidden path %s", path)
			}
		case "archival":
			if !(strings.HasPrefix(path, ".concoct/archive/") || path == ".concoct/capabilities.md" || path == ".concoct/roadmap.md" || path == ".concoct/current/task-plan.md" || path == ".concoct/current/notes.md") {
				return fmt.Errorf("Archivist action contains forbidden path %s", path)
			}
		}
	}
	return nil
}

func snapshotFiles(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() && (rel == ".git" || rel == ".concoct/runtime") {
			return filepath.SkipDir
		}
		if info.IsDir() || rel == "." {
			return nil
		}
		var data []byte
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			data = []byte("symlink:" + target)
		} else if info.Mode().IsRegular() {
			data, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		} else {
			data = []byte(info.Mode().String())
		}
		sum := sha256.Sum256(data)
		files[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	return files, err
}

func changedFiles(before, after map[string]string) []string {
	changed := map[string]bool{}
	for path, digest := range before {
		if after[path] != digest {
			changed[path] = true
		}
	}
	for path, digest := range after {
		if before[path] != digest {
			changed[path] = true
		}
	}
	paths := make([]string, 0, len(changed))
	for path := range changed {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func userConfigDigest() (string, error) {
	path, err := config.UserPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func finalizeSupervised(root string, prepared Prepared) error {
	var err error
	switch prepared.Resolution.Kind {
	case "task-planning":
		if prepared.Plan == nil {
			return fmt.Errorf("supervised planning completion lacks its planning session")
		}
		_, err = prepared.Plan.Complete()
	case "development":
		_, err = workflow.CompleteDeveloper(root)
	case "independent-review":
		_, err = workflow.CompleteReview(root)
	case "archival":
		_, err = workflow.CompleteArchive(root, workflow.ArchiveOverride{})
	default:
		return fmt.Errorf("action %s has no supervised completion boundary", prepared.Resolution.Kind)
	}
	if err != nil {
		return fmt.Errorf("finalize supervised %s candidate: %w", prepared.Resolution.Kind, err)
	}
	return nil
}

func createRecord(root string, prepared Prepared) (string, error) {
	base := filepath.Join(root, ".concoct", "runtime", "invocations")
	for _, dir := range []string{filepath.Dir(base), base} {
		if err := ensurePrivateDir(dir); err != nil {
			return "", err
		}
	}
	record := filepath.Join(base, prepared.Action.Correlation.InvocationID)
	if err := os.Mkdir(record, 0o700); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(record, "action.json"), prepared.Action); err != nil {
		return "", err
	}
	if len(prepared.Prompt) > 0 {
		if err := writePrivate(filepath.Join(record, "prompt.md"), prepared.Prompt); err != nil {
			return "", err
		}
		if err := writePrivate(filepath.Join(record, "outcome-schema.json"), prepared.Schema); err != nil {
			return "", err
		}
		if err := writeJSON(filepath.Join(record, "prompt-composition.json"), prepared.Composition); err != nil {
			return "", err
		}
	}
	metadata := Metadata{
		InvocationID: prepared.Action.Correlation.InvocationID, PredecessorInvocationID: prepared.PredecessorInvocationID, Action: prepared.Action.Kind, Role: prepared.Resolution.Role,
		Adapter: prepared.Settings.Adapter.Value, AdapterVersion: prepared.Invocation.AdapterVersion, Model: prepared.Settings.Model.Value, ModelSource: prepared.Settings.Model.Source,
		Reasoning: prepared.Settings.Reasoning.Value, ReasoningSource: prepared.Settings.Reasoning.Source,
		Timeout: prepared.Settings.Timeout.String(), TimeoutSource: prepared.Settings.TimeoutSource,
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano), PromptFile: "prompt.md", PromptBytes: len(prepared.Prompt), CompositionFile: "prompt-composition.json", ConfigDigest: prepared.ConfigDigest,
	}
	if prepared.Direct {
		metadata.Adapter, metadata.Safety, metadata.Command, metadata.PromptFile = "direct", "existing executable integration authority", "concoct integrate", ""
	} else {
		metadata.Safety, metadata.Command = prepared.Invocation.Safety, adapter.DisplayCommand(prepared.Invocation)
	}
	if err := writeJSON(filepath.Join(record, "metadata.json"), metadata); err != nil {
		return "", err
	}
	return record, nil
}

func Inspect(root, id string, fullRaw ...bool) (string, error) {
	base := filepath.Join(root, ".concoct", "runtime", "invocations")
	record, err := selectRecord(base, id)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Invocation: %s\n", filepath.Base(record))
	items := []struct{ name, heading string }{{"metadata.json", "Metadata"}, {"action.json", "Action"}, {"prompt-composition.json", "Prompt composition"}, {"result.json", "Result"}, {"measurement.json", "Measurement"}, {"reconciliation.json", "Reconciliation"}}
	if len(fullRaw) > 0 && fullRaw[0] {
		items = append(items, struct{ name, heading string }{"prompt.md", "Prompt"}, struct{ name, heading string }{"stdout.log", "Structured events"}, struct{ name, heading string }{"stderr.log", "Stderr diagnostics"})
	}
	for _, item := range items {
		fmt.Fprintf(&b, "\n## %s\n", item.heading)
		path := filepath.Join(record, item.name)
		info, statErr := os.Lstat(path)
		if statErr == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
			return "", fmt.Errorf("retained %s is not a regular non-symlink file", item.name)
		}
		data, readErr := os.ReadFile(path)
		if os.IsNotExist(readErr) {
			fmt.Fprintln(&b, "unavailable (partial or not produced)")
			continue
		}
		if readErr != nil {
			return "", readErr
		}
		if item.name == "measurement.json" && !(len(fullRaw) > 0 && fullRaw[0]) {
			data, readErr = compactMeasurement(data)
			if readErr != nil {
				return "", readErr
			}
		}
		b.Write(data)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	if decision, decisionErr := runstate.LoadDecision(root); decisionErr == nil {
		data, marshalErr := json.MarshalIndent(decision, "", "  ")
		if marshalErr != nil {
			return "", marshalErr
		}
		fmt.Fprintln(&b, "\n## Retained Product Owner decision")
		b.Write(data)
		b.WriteByte('\n')
	} else if !os.IsNotExist(decisionErr) {
		fmt.Fprintf(&b, "\n## Retained Product Owner decision\n\nunavailable (%v)\n", decisionErr)
	}
	return b.String(), nil
}

func Prune(root string, retention config.Retention, currentID string) error {
	base := filepath.Join(root, ".concoct", "runtime", "invocations")
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var closed []retentionRecord
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(base, entry.Name())
		info, statErr := os.Lstat(filepath.Join(path, "reconciliation.json"))
		if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		size, sizeErr := dirSize(path)
		if sizeErr != nil {
			continue
		}
		closed = append(closed, retentionRecord{path, info.ModTime(), size})
	}
	sort.Slice(closed, func(i, j int) bool {
		if closed[i].closed.Equal(closed[j].closed) {
			return closed[i].path < closed[j].path
		}
		return closed[i].closed.Before(closed[j].closed)
	})
	now := time.Now()
	keep := closed[:0]
	for _, item := range closed {
		if now.Sub(item.closed) > retention.MaxAge && filepath.Base(item.path) != currentID {
			if err := os.RemoveAll(item.path); err != nil {
				return err
			}
			continue
		}
		keep = append(keep, item)
	}
	closed = keep
	for len(closed) > retention.MaxCompleted {
		index := oldestRemovable(closed, currentID)
		if index < 0 {
			break
		}
		if err := os.RemoveAll(closed[index].path); err != nil {
			return err
		}
		closed = append(closed[:index], closed[index+1:]...)
	}
	var total int64
	for _, item := range closed {
		total += item.size
	}
	for total > retention.MaxTotal && len(closed) > 0 {
		index := oldestRemovable(closed, currentID)
		if index < 0 {
			break
		}
		if err := os.RemoveAll(closed[index].path); err != nil {
			return err
		}
		total -= closed[index].size
		closed = append(closed[:index], closed[index+1:]...)
	}
	return nil
}

type boundedRedactedWriter struct {
	mu            sync.Mutex
	file, display io.Writer
	max, written  int64
	truncated     bool
	pending       []byte
}

type lockedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer == nil {
		return len(p), nil
	}
	return w.writer.Write(p)
}

func newBoundedRedactedWriter(file, display io.Writer, max int64) *boundedRedactedWriter {
	return &boundedRedactedWriter{file: file, display: display, max: max}
}
func (w *boundedRedactedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(w.pending, p...)
	for {
		index := bytes.IndexByte(w.pending, '\n')
		if index < 0 {
			break
		}
		if err := w.emit(w.pending[:index+1]); err != nil {
			return 0, err
		}
		w.pending = w.pending[index+1:]
	}
	if int64(len(w.pending)) >= w.max {
		if err := w.emit(w.pending); err != nil {
			return 0, err
		}
		w.pending = nil
	}
	return len(p), nil
}
func (w *boundedRedactedWriter) emit(p []byte) error {
	clean := redact(string(p))
	remaining := w.max - w.written
	if remaining <= 0 {
		w.truncated = true
		return nil
	}
	data := []byte(clean)
	if int64(len(data)) > remaining {
		data = data[:remaining]
		w.truncated = true
	}
	if _, err := w.file.Write(data); err != nil {
		return err
	}
	if w.display != nil {
		_, _ = w.display.Write(data)
	}
	w.written += int64(len(data))
	return nil
}
func (w *boundedRedactedWriter) finish() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) > 0 {
		_ = w.emit(w.pending)
		w.pending = nil
	}
	if w.truncated {
		marker := []byte("\n[concoct: log truncated]\n")
		_, _ = w.file.Write(marker)
		if w.display != nil {
			_, _ = w.display.Write(marker)
		}
	}
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization\s*:\s*(?:bearer|basic)\s+)\S+`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}\b`),
	regexp.MustCompile(`(?i)((?:api[_-]?key|access[_-]?token|password|secret)\s*[=:]\s*)[^\s,;]+`),
	regexp.MustCompile(`(?i)("(?:token|api_key|password|secret)"\s*:\s*")[^"]+`),
}

func redact(value string) string {
	for _, re := range secretPatterns {
		value = re.ReplaceAllString(value, `${1}[REDACTED]`)
	}
	return value
}

func selectRecord(base, id string) (string, error) {
	if id != "" {
		if !regexp.MustCompile(`^[a-f0-9]{36}$`).MatchString(id) {
			return "", fmt.Errorf("invalid invocation id %q", id)
		}
		path := filepath.Join(base, id)
		if info, err := os.Lstat(path); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("unknown invocation %s", id)
		}
		return path, nil
	}
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("no retained invocations")
	}
	if err != nil {
		return "", err
	}
	type candidate struct {
		path string
		when time.Time
	}
	var candidates []candidate
	for _, entry := range entries {
		if entry.IsDir() {
			path := filepath.Join(base, entry.Name())
			when := time.Time{}
			var metadata Metadata
			metadataPath := filepath.Join(path, "metadata.json")
			info, statErr := os.Lstat(metadataPath)
			if statErr == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
				if data, readErr := os.ReadFile(metadataPath); readErr == nil && json.Unmarshal(data, &metadata) == nil {
					when, _ = time.Parse(time.RFC3339Nano, metadata.StartedAt)
				}
			}
			if when.IsZero() {
				info, _ := entry.Info()
				when = info.ModTime()
			}
			candidates = append(candidates, candidate{path, when})
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no retained invocations")
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].when.Equal(candidates[j].when) {
			return candidates[i].path > candidates[j].path
		}
		return candidates[i].when.After(candidates[j].when)
	})
	return candidates[0].path, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writePrivate(path, append(data, '\n'))
}
func writePrivate(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
func privateFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}
func dirSize(root string) (int64, error) {
	var size int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
func displayDefault(value string) string {
	if value == "" {
		return "adapter default"
	}
	return value
}

func resolvedConfigDigest(settings config.Resolved) (string, error) {
	data, err := json.Marshal(struct {
		Adapter, Model, Reasoning config.Value
		Timeout                   string
		TimeoutSource             string
		Retention                 config.Retention
		RetentionSource           map[string]string
	}{settings.Adapter, settings.Model, settings.Reasoning, settings.Timeout.String(), settings.TimeoutSource, settings.Retention, settings.RetentionSource})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func configurationStable(root string, override config.Overrides, prepared Prepared) error {
	spec, ok := adapter.Find(prepared.Settings.Adapter.Value)
	if !ok {
		return fmt.Errorf("configured adapter %q is no longer available", prepared.Settings.Adapter.Value)
	}
	current, err := config.Resolve(root, prepared.Resolution.Role, override, spec.Defaults)
	if err != nil {
		return fmt.Errorf("re-resolve execution configuration: %w", err)
	}
	digest, err := resolvedConfigDigest(current)
	if err != nil {
		return err
	}
	if digest != prepared.ConfigDigest {
		return fmt.Errorf("execution configuration changed after authorization")
	}
	return nil
}

func authorizationStable(root string, override config.Overrides, prepared Prepared) error {
	if err := configurationStable(root, override, prepared); err != nil {
		return err
	}
	current, _, err := orchestration.Snapshot(root)
	if err != nil {
		return fmt.Errorf("re-snapshot authorized repository evidence: %w", err)
	}
	if current.Digest != prepared.Action.Evidence.Digest || current.State != prepared.Action.Evidence.State {
		return fmt.Errorf("repository evidence changed after authorization; no action was launched")
	}
	return nil
}

func ensurePrivateDir(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if err := os.Mkdir(path, 0o700); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("runtime path %s must be a non-symlink directory", path)
	}
	return os.Chmod(path, 0o700)
}

func oldestRemovable(records []retentionRecord, currentID string) int {
	for i, item := range records {
		if filepath.Base(item.path) != currentID {
			return i
		}
	}
	return -1
}
