package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/dispatch"
	"github.com/gopher-launch/concoct/internal/selection"
)

type request struct {
	Command, Repository, Root, Codex, Task                     string
	JSON, Quiet                                                bool
	Answer, Text, Choice, AnswerFile                           string
	Authorize, Approve, Extend                                 string
	Defer, Abandon, Resume, Reason                             string
	ReviewLimit, DispatchLimit, RecoveryLimit, SelfReviewLimit int
	ReagentModel, IragentModel, ReagentEffort, IragentEffort   string
	Gate                                                       string
}

type outcome struct {
	Disposition, Summary, Next string
	Snapshot                   activework.Snapshot
	Acceptance                 *activework.Acceptance
	Completion                 *activework.CompletionResult
	Approval                   *approvalView
}

type approvalView struct {
	DiffCommand           string                        `json:"diff_command"`
	ReconciliationCommand string                        `json:"reconciliation_command"`
	TaskEvidence          string                        `json:"task_evidence"`
	ReviewEvidence        string                        `json:"review_evidence"`
	Progress              string                        `json:"progress"`
	SelfReview            string                        `json:"self_review"`
	Assessment            string                        `json:"assessment"`
	Criteria              []selection.CriterionEvidence `json:"criteria"`
	Checks                []selection.CheckEvidence     `json:"checks"`
	ReconciledPaths       []string                      `json:"reconciled_paths"`
}

type operations interface {
	Do(context.Context, request, func(codexapp.Event)) (outcome, error)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, ops operations) int {
	if len(args) == 0 {
		usage(stdout)
		return 0
	}
	if len(args) == 1 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		usage(stdout)
		return 0
	}
	if len(args) == 1 && (args[0] == "version" || args[0] == "-v" || args[0] == "--version") {
		fmt.Fprintln(stdout, version)
		return 0
	}
	req, err := parseRequest(args)
	if err != nil {
		if slices.Contains(args, "--json") {
			writePublicJSON(stdout, map[string]string{"type": "result", "disposition": "prerequisite-error", "summary": err.Error()})
		} else {
			fmt.Fprintln(stderr, "concoct:", err)
		}
		return 2
	}
	stream := newMessageStream(stdout, req.JSON, req.Quiet)
	roleForThread := func(threadID string) string {
		if store, err := activework.Open(req.Repository, req.Root); err == nil {
			if snapshot, err := store.Status(); err == nil {
				for role, id := range snapshot.RoleIDs {
					if id == threadID {
						return role
					}
				}
			}
		}
		return "agent"
	}
	result, err := ops.Do(ctx, req, func(event codexapp.Event) { stream.event(event, roleForThread) })
	stream.finish()
	if result.Disposition == "" {
		result.Disposition = "prerequisite_error"
	}
	if errors.Is(err, activework.ErrOwned) {
		result.Disposition = "busy"
	}
	if err != nil {
		result.Summary = err.Error()
	}
	if result.Next == "" {
		result.Next = nextCommand(result)
	}
	result.Next = contextualCommand(result.Next, req)
	if req.JSON {
		response := struct {
			Type        string        `json:"type"`
			Disposition string        `json:"disposition"`
			Summary     string        `json:"summary"`
			Next        string        `json:"next"`
			Status      statusView    `json:"status"`
			Approval    *approvalView `json:"approval,omitempty"`
			Agent       *agentReport  `json:"agent-reported,omitempty"`
		}{"result", publicName(result.Disposition), result.Summary, result.Next, view(result.Snapshot, req), result.Approval, stream.report}
		writePublicJSON(stdout, response)
	} else {
		if stream.report != nil {
			if stream.report.Error != "" {
				fmt.Fprintf(stdout, "Agent result: %s (%s turn %s)\n", stream.report.Error, stream.report.Role, stream.report.TurnID)
			} else {
				fmt.Fprintf(stdout, "Agent reported (%s, %s): %s\n", stream.report.Role, stream.report.Disposition, stream.report.Summary)
			}
		}
		fmt.Fprintf(stdout, "\nConcoct verified: %s\n", publicName(result.Disposition))
		if result.Summary != "" {
			fmt.Fprintln(stdout, result.Summary)
		}
		printStatus(stdout, view(result.Snapshot, req))
		if result.Approval != nil {
			printApproval(stdout, *result.Approval)
		}
		if result.Next != "" {
			fmt.Fprintln(stdout, "Next:", result.Next)
		}
	}
	return exitCode(result.Disposition)
}

func parseRequest(args []string) (request, error) {
	var r request
	var err error
	if len(args) == 0 || (args[0] != "run" && args[0] != "status") {
		return r, fmt.Errorf("unsupported command %q; use run, status, help or version", args[0])
	}
	r.Command = args[0]
	seen := map[string]bool{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			if r.Command != "status" || r.Task != "" {
				return r, fmt.Errorf("unexpected argument %q", arg)
			}
			r.Task = arg
			continue
		}
		name := strings.TrimPrefix(arg, "--")
		if seen[name] {
			return r, fmt.Errorf("duplicate --%s", name)
		}
		seen[name] = true
		if name == "json" || name == "quiet" {
			if name == "json" {
				r.JSON = true
			} else {
				r.Quiet = true
			}
			continue
		}
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return r, fmt.Errorf("--%s requires a value", name)
		}
		i++
		value := args[i]
		switch name {
		case "repo":
			r.Repository = value
		case "root":
			r.Root = value
		case "codex":
			r.Codex = value
		case "answer":
			r.Answer = value
		case "text":
			r.Text = value
		case "choice":
			r.Choice = value
		case "answer-file":
			r.AnswerFile = value
		case "authorize":
			r.Authorize = value
		case "approve":
			r.Approve = value
		case "extend":
			r.Extend = value
		case "defer":
			r.Defer = value
		case "abandon":
			r.Abandon = value
		case "resume":
			r.Resume = value
		case "reason":
			r.Reason = value
		case "review-limit":
			r.ReviewLimit, err = positiveLimit(value)
			if err != nil {
				return r, fmt.Errorf("invalid review limit")
			}
		case "dispatch-limit":
			r.DispatchLimit, err = positiveLimit(value)
			if err != nil {
				return r, fmt.Errorf("invalid dispatch limit")
			}
		case "recovery-limit":
			r.RecoveryLimit, err = positiveLimit(value)
			if err != nil {
				return r, fmt.Errorf("invalid recovery limit")
			}
		case "self-review-limit":
			r.SelfReviewLimit, err = positiveLimit(value)
			if err != nil {
				return r, fmt.Errorf("invalid self-review limit")
			}
		case "reagent-model":
			r.ReagentModel = value
		case "iragent-model":
			r.IragentModel = value
		case "reagent-effort":
			r.ReagentEffort = value
		case "iragent-effort":
			r.IragentEffort = value
		case "gate":
			r.Gate = value
		default:
			return r, fmt.Errorf("unsupported option --%s", name)
		}
	}
	if r.Command == "status" {
		for _, name := range []string{"codex", "answer", "text", "choice", "answer-file", "authorize", "approve", "extend", "defer", "abandon", "resume", "reason", "review-limit", "dispatch-limit", "recovery-limit", "self-review-limit", "reagent-model", "iragent-model", "reagent-effort", "iragent-effort", "gate"} {
			if seen[name] {
				return r, fmt.Errorf("status does not accept --%s", name)
			}
		}
		if r.Repository == "" {
			r.Repository = "."
		}
		return r, nil
	}
	actions := 0
	for _, value := range []string{r.Answer, r.Authorize, r.Approve, r.Extend, r.Defer, r.Abandon, r.Resume} {
		if value != "" {
			actions++
		}
	}
	if actions > 1 {
		return r, fmt.Errorf("choose one run intervention")
	}
	answerForms := 0
	for _, value := range []string{r.Text, r.Choice, r.AnswerFile} {
		if value != "" {
			answerForms++
		}
	}
	if r.Answer != "" && answerForms != 1 || r.Answer == "" && answerForms != 0 {
		return r, fmt.Errorf("--answer requires exactly one of --text, --choice or --answer-file")
	}
	if (r.Defer != "" || r.Abandon != "") != (r.Reason != "") {
		return r, fmt.Errorf("--defer or --abandon requires --reason")
	}
	limits := r.ReviewLimit != 0 || r.DispatchLimit != 0 || r.RecoveryLimit != 0 || r.SelfReviewLimit != 0
	if r.Extend != "" && !limits || r.Extend == "" && limits {
		return r, fmt.Errorf("--extend requires a task ID and at least one limit")
	}
	policy := r.ReagentModel != "" || r.IragentModel != "" || r.ReagentEffort != "" || r.IragentEffort != "" || r.Gate != ""
	if actions != 0 && policy {
		return r, fmt.Errorf("role and gate choices apply only to a fresh run")
	}
	if _, err := parseGates(r.Gate); err != nil {
		return r, err
	}
	if r.Repository == "" {
		r.Repository = "."
	}
	return r, nil
}

func positiveLimit(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > 1000 {
		return 0, fmt.Errorf("invalid positive limit")
	}
	return n, nil
}

// statusView omits provider final text and active narratives from machine output.
type statusView struct {
	ObservedAt      time.Time                 `json:"observed_at"`
	Active          bool                      `json:"active"`
	PendingStart    bool                      `json:"pending_start,omitempty"`
	TaskID          string                    `json:"task_id,omitempty"`
	RunID           string                    `json:"run_id,omitempty"`
	Phase           string                    `json:"phase,omitempty"`
	Hold            string                    `json:"hold,omitempty"`
	PendingRequest  string                    `json:"pending_request,omitempty"`
	Recovery        *activework.RecoveryIssue `json:"recovery,omitempty"`
	Candidate       string                    `json:"candidate,omitempty"`
	ReviewVerdict   string                    `json:"review_verdict,omitempty"`
	QuestionID      string                    `json:"question_id,omitempty"`
	Question        *questionView             `json:"question,omitempty"`
	AuthorizationID string                    `json:"authorization_id,omitempty"`
	AcceptanceID    string                    `json:"acceptance_id,omitempty"`
	TargetBranch    string                    `json:"target_branch,omitempty"`
	TargetCommit    string                    `json:"target_commit,omitempty"`
	Profiles        map[string]string         `json:"profiles,omitempty"`
	Attempts        int                       `json:"attempts"`
	DispatchLimit   int                       `json:"dispatch_limit"`
	Assessments     int                       `json:"assessments"`
	AssessmentLimit int                       `json:"assessment_limit"`
	RecoveryLimit   int                       `json:"recovery_limit"`
	ObservedTokens  *int64                    `json:"observed_tokens,omitempty"`
	ReviewTurnSeen  bool                      `json:"review_turn_observed"`
	QuotaAge        string                    `json:"quota_age"`
	QuotaWindows    map[string]quotaWindow    `json:"quota_windows,omitempty"`
}

type questionView struct {
	ID           string               `json:"id"`
	Requester    string               `json:"requester"`
	Objective    string               `json:"objective"`
	Evidence     string               `json:"evidence"`
	Missing      string               `json:"missing"`
	SafeWork     string               `json:"safe_work"`
	NextStep     string               `json:"next_step"`
	Options      []questionOptionView `json:"options,omitempty"`
	SourceChange bool                 `json:"source_change"`
	ScopeChange  bool                 `json:"scope_change"`
}

type questionOptionView struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Tradeoff    string `json:"tradeoff"`
	Command     string `json:"command"`
}

type quotaWindow struct {
	UsedPercent      float64   `json:"used_percent"`
	RemainingPercent float64   `json:"remaining_percent"`
	ResetAt          time.Time `json:"reset_at,omitempty"`
}

func view(s activework.Snapshot, req request) statusView {
	v := statusView{ObservedAt: s.ObservedAt, Active: s.Active, PendingStart: s.PendingStart, TaskID: s.TaskID, RunID: s.RunID, Phase: publicName(s.Phase), Hold: publicName(s.Hold), PendingRequest: s.PendingRequest, QuotaAge: "unknown", Profiles: map[string]string{}}
	if s.Recovery != nil {
		issue := *s.Recovery
		issue.Stage = publicName(issue.Stage)
		v.Recovery = &issue
	}
	if s.Candidate != nil {
		v.Candidate = s.Candidate.Commit
	}
	if s.Review != nil {
		v.ReviewVerdict = publicName(s.Review.Verdict)
		if s.Candidate != nil && s.Review.Candidate == s.Candidate.Commit && s.Dispatch != nil {
			for _, attempt := range s.Dispatch.Attempts {
				if attempt.Binding.Role == dispatch.Iragent && attempt.Binding.TaskID == s.TaskID && attempt.Binding.Candidate == s.Candidate.Commit && attempt.TurnID != "" {
					v.ReviewTurnSeen = true
					break
				}
			}
		}
	}
	if s.Question != nil && s.Question.AnswerMode == "" {
		v.QuestionID = s.Question.ID
		v.Question = &questionView{ID: s.Question.ID, Requester: s.Question.Requester, Objective: s.Question.Objective, Evidence: s.Question.Evidence, Missing: s.Question.Missing, SafeWork: s.Question.SafeWork, NextStep: s.Question.NextStep, SourceChange: s.Question.SourceChange, ScopeChange: s.Question.ScopeChange}
		for _, option := range s.Question.Options {
			v.Question.Options = append(v.Question.Options, questionOptionView{ID: option.ID, Description: option.Description, Tradeoff: option.Tradeoff, Command: contextualCommand("concoct run --answer "+s.Question.ID+" --choice "+option.ID, req)})
		}
	}
	if s.Acceptance != nil {
		v.AcceptanceID, v.TargetBranch, v.TargetCommit = s.Acceptance.ID, s.Acceptance.TargetBranch, s.Acceptance.Candidate
	} else if s.Active {
		v.TargetBranch = s.ParentBranch
		v.TargetCommit = s.ParentCommit
	}
	if s.Dispatch != nil {
		v.Attempts, v.DispatchLimit = len(s.Dispatch.Attempts), s.Dispatch.Limits.Dispatches
		v.Assessments, v.AssessmentLimit, v.RecoveryLimit = len(s.Dispatch.Assessments), s.Dispatch.Limits.Assessments, s.Dispatch.Limits.Recovery
		if s.Dispatch.Pending != nil {
			v.AuthorizationID = s.Dispatch.Pending.ID
		}
		for role, profile := range s.Dispatch.Profiles {
			v.Profiles[string(role)] = fmt.Sprintf("%s/%s sandbox=%s approval=%s", profile.Model, profile.Effort, profile.Sandbox, profile.ApprovalPolicy)
		}
		for _, attempt := range s.Dispatch.Attempts {
			if attempt.Tokens != nil {
				if v.ObservedTokens == nil {
					v.ObservedTokens = new(int64)
				}
				*v.ObservedTokens += attempt.Tokens.TotalTokens
			}
		}
		if s.Dispatch.Quota != nil {
			if age, ok := s.Dispatch.Quota.Age(time.Now().UTC()); ok {
				v.QuotaAge = age.Round(time.Second).String()
			}
			v.QuotaWindows = make(map[string]quotaWindow, len(s.Dispatch.Quota.Windows))
			for name, window := range s.Dispatch.Quota.Windows {
				v.QuotaWindows[name] = quotaWindow{UsedPercent: window.UsedPercent, RemainingPercent: 100 - window.UsedPercent, ResetAt: window.ResetAt}
			}
		}
	}
	return v
}

func printStatus(w io.Writer, s statusView) {
	if !s.ObservedAt.IsZero() {
		fmt.Fprintln(w, "Observed:", localTime(s.ObservedAt))
	}
	if !s.Active {
		if s.PendingStart {
			fmt.Fprintln(w, "Run start requires recovery.")
		}
		return
	}
	if s.TaskID == "" {
		fmt.Fprintf(w, "Selecting a task; phase %s", publicName(s.Phase))
	} else {
		fmt.Fprintf(w, "Task %s; phase %s", s.TaskID, publicName(s.Phase))
	}
	if s.Hold != "" {
		fmt.Fprintf(w, "; hold %s", publicName(s.Hold))
	}
	fmt.Fprintln(w)
	if s.PendingRequest != "" {
		fmt.Fprintln(w, "Pending action:", s.PendingRequest)
	}
	if s.Recovery != nil {
		label := s.Recovery.Stage
		if s.Recovery.Operation != "" {
			label += " " + s.Recovery.Operation
		}
		if s.Recovery.AgentResult != "" {
			fmt.Fprintln(w, "Agent reported:", s.Recovery.AgentResult)
		}
		fmt.Fprintln(w, "Controller verification: incomplete; independent review turn observed:", s.ReviewTurnSeen)
		if label != "" {
			fmt.Fprintf(w, "Recovery at %s: %s\n", publicName(label), s.Recovery.Reason)
		} else {
			fmt.Fprintf(w, "Recovery: %s\n", s.Recovery.Reason)
		}
		if s.Recovery.EvidenceRef != "" {
			fmt.Fprintln(w, "Evidence:", s.Recovery.EvidenceRef)
		}
		fmt.Fprintln(w, "Next run reconciles retained evidence and source before any candidate or review step; missing proof keeps this hold.")
	}
	if s.Candidate != "" {
		fmt.Fprintln(w, "Candidate:", s.Candidate)
	}
	if s.TargetCommit != "" {
		fmt.Fprintf(w, "Target: %s -> %s\n", s.TargetBranch, s.TargetCommit)
	}
	if len(s.Profiles) > 0 {
		fmt.Fprintf(w, "Roles: reagent %s; iragent %s\n", s.Profiles[string(dispatch.Reagent)], s.Profiles[string(dispatch.Iragent)])
	}
	fmt.Fprintf(w, "Bounds: attempts %d/%d; assessments %d/%d; recovery limit %d\n", s.Attempts, s.DispatchLimit, s.Assessments, s.AssessmentLimit, s.RecoveryLimit)
	if s.ObservedTokens == nil {
		fmt.Fprintf(w, "Observed tokens: unavailable; quota observation age: %s\n", s.QuotaAge)
	} else {
		fmt.Fprintf(w, "Observed tokens: %d; quota observation age: %s\n", *s.ObservedTokens, s.QuotaAge)
	}
	if len(s.QuotaWindows) > 0 {
		names := make([]string, 0, len(s.QuotaWindows))
		for name := range s.QuotaWindows {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			window := s.QuotaWindows[name]
			fmt.Fprintf(w, "Quota %s: %.1f%% used, %.1f%% remaining", name, window.UsedPercent, window.RemainingPercent)
			if !window.ResetAt.IsZero() {
				fmt.Fprintf(w, "; resets %s", localTime(window.ResetAt))
			}
			fmt.Fprintln(w)
		}
	}
	if s.QuestionID != "" {
		q := s.Question
		fmt.Fprintf(w, "Question %s from %s\nObjective: %s\nEvidence: %s\nMissing: %s\nSafe work: %s\nNext step: %s\n", q.ID, q.Requester, q.Objective, q.Evidence, q.Missing, q.SafeWork, q.NextStep)
		for _, option := range q.Options {
			fmt.Fprintf(w, "Option %s: %s (tradeoff: %s)\n", option.ID, option.Description, option.Tradeoff)
			fmt.Fprintln(w, "  Choose:", option.Command)
		}
	}
	if s.AuthorizationID != "" {
		fmt.Fprintln(w, "Authorization:", s.AuthorizationID)
	}
	if s.AcceptanceID != "" {
		fmt.Fprintln(w, "Acceptance:", s.AcceptanceID)
	}
}

func localTime(value time.Time) string {
	location := time.Local
	if zone := os.Getenv("TZ"); zone != "" {
		if loaded, err := time.LoadLocation(zone); err == nil {
			location = loaded
		}
	}
	return value.In(location).Format("2006-01-02 15:04:05 MST -07:00")
}

func printApproval(w io.Writer, a approvalView) {
	fmt.Fprintln(w, "Recorded candidate evidence:", a.TaskEvidence)
	fmt.Fprintln(w, "Final independent assessment:", a.ReviewEvidence)
	fmt.Fprintln(w, "Source diff:", a.DiffCommand)
	fmt.Fprintln(w, "Roadmap/capability diff:", a.ReconciliationCommand)
	fmt.Fprintln(w, "Reconciled paths:", strings.Join(a.ReconciledPaths, ", "))
	fmt.Fprintln(w, "Engineering progress:", a.Progress)
	fmt.Fprintln(w, "Self-review:", a.SelfReview)
	for _, criterion := range a.Criteria {
		fmt.Fprintf(w, "Criterion %s: %s — %s\n", criterion.Result, criterion.Criterion, criterion.Evidence)
	}
	for _, check := range a.Checks {
		fmt.Fprintf(w, "Check %s: %s — %s\n", check.Result, check.Command, check.Evidence)
	}
	fmt.Fprintln(w, "Final review verdict: approved")
	fmt.Fprintln(w, "Review assessment:", a.Assessment)
}

func nextCommand(o outcome) string {
	s := o.Snapshot
	switch o.Disposition {
	case "idle":
		return "concoct run"
	case "ready_for_acceptance":
		if o.Acceptance != nil {
			return "concoct run --approve " + o.Acceptance.ID
		}
		if s.Acceptance != nil {
			return "concoct run --approve " + s.Acceptance.ID
		}
		return "concoct run"
	case "authorization_required":
		if s.Dispatch != nil && s.Dispatch.Pending != nil {
			return "concoct run --authorize " + s.Dispatch.Pending.ID
		}
	case "decision_required":
		if s.Hold == "limit-exhausted" && s.Dispatch != nil && s.TaskID != "" {
			var limits []string
			if len(s.Dispatch.Attempts) >= s.Dispatch.Limits.Dispatches {
				limits = append(limits, fmt.Sprintf("--dispatch-limit %d", len(s.Dispatch.Attempts)+1))
			}
			if len(s.Dispatch.Assessments) >= s.Dispatch.Limits.Assessments {
				limits = append(limits, fmt.Sprintf("--review-limit %d", len(s.Dispatch.Assessments)+1))
			}
			if len(limits) != 0 {
				return "concoct run --extend " + s.TaskID + " " + strings.Join(limits, " ")
			}
		}
		if s.Question != nil && s.Question.AnswerMode == "" {
			return "concoct run --answer " + s.Question.ID + " --text <answer>"
		}
	case "maintenance_pending", "in_progress", "stopped_safely", "recovery_required":
		return "concoct run"
	case "ready_for_independent_review":
		return "concoct run"
	}
	return ""
}

func contextualCommand(command string, req request) string {
	if command == "" {
		return ""
	}
	if req.Repository != "" && req.Repository != "." {
		command += " --repo " + shellQuote(req.Repository)
	}
	if req.Root != "" {
		command += " --root " + shellQuote(req.Root)
	}
	if req.Codex != "" {
		command += " --codex " + shellQuote(req.Codex)
	}
	return command
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

func exitCode(disposition string) int {
	switch disposition {
	case "completed", "no_suitable_work", "idle":
		return 0
	case "decision_required":
		return 10
	case "authorization_required":
		return 11
	case "ready_for_acceptance":
		return 12
	case "ready_for_independent_review":
		return 13
	case "stopped_safely":
		return 14
	case "stopped_abnormally":
		return 15
	case "busy":
		return 16
	case "maintenance_pending":
		return 17
	case "in_progress", "recovery_required", "deferred", "abandoned":
		return 18
	default:
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Concoct 0.5 development CLI")
	fmt.Fprintln(w, "Usage: concoct status [TASK] [--json] [--repo PATH] [--root PATH]")
	fmt.Fprintln(w, "       concoct run [--json|--quiet] [--repo PATH] [--root PATH] [--codex PATH]")
	fmt.Fprintln(w, "       concoct run --answer QUESTION (--text TEXT|--choice OPTION|--answer-file PATH)")
	fmt.Fprintln(w, "       concoct run --authorize DISPATCH | --approve ACCEPTANCE")
	fmt.Fprintln(w, "       concoct run --extend TASK (--dispatch-limit N|--review-limit N|--recovery-limit N)")
	fmt.Fprintln(w, "       concoct run --defer TASK --reason TEXT | --abandon TASK --reason TEXT | --resume TASK")
	fmt.Fprintln(w, "Fresh run options: --reagent-model MODEL --iragent-model MODEL --reagent-effort EFFORT --iragent-effort EFFORT --gate ACTIVITY[,ACTIVITY]")
	fmt.Fprintln(w, "Use help for this summary. Workflow commands integrate locally; they never push or publish.")
}
