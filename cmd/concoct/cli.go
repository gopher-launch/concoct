package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/handoff"
)

type opener func(context.Context, string) (handoff.Runner, error)

func liveOpen(ctx context.Context, executable string) (handoff.Runner, error) {
	return handoff.Launch(ctx, executable)
}
func run(ctx context.Context, args []string, stdout, stderr io.Writer, open opener) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		usage(stdout)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintln(stdout, version)
		return 0
	}
	o, command, jsonMode, quiet, err := parse(args)
	if err != nil {
		fmt.Fprintln(stderr, "concoct:", err)
		return 2
	}
	if command == "status" {
		state, err := handoff.Status(o)
		if err != nil {
			fmt.Fprintln(stderr, "concoct:", err)
			return 2
		}
		display(stdout, state, jsonMode)
		return 0
	}
	stream := newMessageStream(stdout, jsonMode, quiet)
	o.OnEvent = func(role string, event codexapp.Event) { stream.event(event, func(string) string { return role }) }
	state, err := handoff.Run(ctx, o, open)
	stream.finish()
	if err != nil {
		fmt.Fprintln(stderr, "concoct:", err)
	}
	display(stdout, state, jsonMode)
	if err != nil {
		return 2
	}
	switch state.Phase {
	case "awaiting_approval":
		return 12
	case "blocked", "limit_exhausted":
		return 10
	case "execution_uncertain", "execution_failed", "invalid_result", "source_changed", "integration_failed":
		return 18
	default:
		return 0
	}
}
func parse(args []string) (handoff.Options, string, bool, bool, error) {
	var o handoff.Options
	o.Repository = "."
	command := args[0]
	if command != "run" && command != "status" {
		return o, "", false, false, fmt.Errorf("unknown command %q", command)
	}
	jsonMode, quiet := false, false
	seen := map[string]bool{}
	var err error
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			return o, "", false, false, fmt.Errorf("unexpected argument %q", arg)
		}
		key := strings.TrimPrefix(arg, "--")
		if seen[key] {
			return o, "", false, false, fmt.Errorf("duplicate --%s", key)
		}
		seen[key] = true
		if key == "json" {
			jsonMode = true
			continue
		}
		if key == "quiet" {
			quiet = true
			continue
		}
		if i+1 >= len(args) {
			return o, "", false, false, fmt.Errorf("--%s requires value", key)
		}
		i++
		value := args[i]
		switch key {
		case "repo":
			o.Repository = value
		case "root":
			o.Root = value
		case "codex":
			o.Codex = value
		case "approve":
			o.Approve = value
		case "answer":
			o.Answer = value
		case "reagent-model":
			o.ReagentModel = value
		case "iragent-model":
			o.IragentModel = value
		case "reagent-effort":
			o.ReagentEffort = value
		case "iragent-effort":
			o.IragentEffort = value
		case "dispatch-limit":
			o.MaxAttempts, err = strconv.Atoi(value)
			if err != nil || o.MaxAttempts <= 0 {
				return o, "", false, false, fmt.Errorf("invalid dispatch limit")
			}
		case "review-limit":
			o.MaxAssessments, err = strconv.Atoi(value)
			if err != nil || o.MaxAssessments <= 0 {
				return o, "", false, false, fmt.Errorf("invalid review limit")
			}
		case "stall-timeout":
			d, err := time.ParseDuration(value)
			if err != nil || d <= 0 {
				return o, "", false, false, fmt.Errorf("invalid stall timeout")
			}
			o.StallTimeout = d
		case "turn-timeout":
			d, err := time.ParseDuration(value)
			if err != nil || d <= 0 {
				return o, "", false, false, fmt.Errorf("invalid turn timeout")
			}
			o.TurnTimeout = d
		default:
			return o, "", false, false, fmt.Errorf("unknown option --%s", key)
		}
	}
	if command == "status" && (o.Codex != "" || o.Approve != "" || o.Answer != "" || o.ReagentModel != "" || o.IragentModel != "" || o.ReagentEffort != "" || o.IragentEffort != "" || o.MaxAttempts > 0 || o.MaxAssessments > 0 || o.TurnTimeout > 0 || o.StallTimeout > 0) {
		return o, "", false, false, fmt.Errorf("execution options require run")
	}
	return o, command, jsonMode, quiet, nil
}
func display(w io.Writer, s handoff.State, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(s)
		return
	}
	if s.Phase == "" {
		fmt.Fprintln(w, "No active work.")
		return
	}
	fmt.Fprintf(w, "Concoct: %s\n", strings.ReplaceAll(s.Phase, "_", "-"))
	if s.TaskID != "" {
		fmt.Fprintf(w, "Task: %s\n", s.TaskID)
	}
	if s.Candidate != "" {
		fmt.Fprintf(w, "Candidate: %s\n", s.Candidate)
	}
	if s.Verdict != "" {
		fmt.Fprintf(w, "Independent assessment: %s\n", strings.ReplaceAll(s.Verdict, "_", "-"))
	}
	if s.ReagentSummary != "" {
		fmt.Fprintln(w, "Reagent summary:", s.ReagentSummary)
	}
	if s.ReviewSummary != "" {
		fmt.Fprintln(w, "Iragent summary:", s.ReviewSummary)
	}
	for _, finding := range s.Findings {
		fmt.Fprintln(w, "Review finding:", finding)
	}
	if s.Evidence != "" {
		fmt.Fprintln(w, "Task evidence:", s.Evidence)
	}
	if s.ReviewEvidence != "" {
		fmt.Fprintln(w, "Review evidence:", s.ReviewEvidence)
	}
	if s.Summary != "" && s.Summary != s.ReagentSummary && s.Summary != s.ReviewSummary {
		fmt.Fprintln(w, s.Summary)
	}
	if s.Question != nil {
		fmt.Fprintln(w, "Question:", s.Question.Text)
	}
	if s.LastError != "" {
		fmt.Fprintln(w, "Execution issue:", s.LastError)
	}
	fmt.Fprintf(w, "Invocations: %d/%d\n", s.Attempts, s.MaxAttempts)
	if s.MaxAssessments > 0 {
		fmt.Fprintf(w, "Independent assessments: %d/%d\n", s.Assessments, s.MaxAssessments)
	}
	if s.Phase == "awaiting_approval" {
		fmt.Fprintf(w, "To authorize local integration of this exact commit: concoct run --approve %s\n", s.ApprovedCommit)
	}
	if s.Phase == "blocked" {
		fmt.Fprintln(w, "To answer: concoct run --answer TEXT")
	}
}
func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage: concoct run [--repo PATH] [--root PATH] [--codex PATH] [--reagent-model MODEL] [--iragent-model MODEL] [--reagent-effort EFFORT] [--iragent-effort EFFORT] [--dispatch-limit N] [--review-limit N] [--turn-timeout DURATION] [--stall-timeout DURATION] [--json] [--quiet]\n       concoct run --answer TEXT [--repo PATH] [--root PATH]\n       concoct run --approve COMMIT [--repo PATH] [--root PATH]\n       concoct status [--repo PATH] [--root PATH] [--json]\n       concoct help | version")
}
