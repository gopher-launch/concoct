package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gopher-launch/concoct/internal/buildinfo"
	"github.com/gopher-launch/concoct/internal/contract"
	"github.com/gopher-launch/concoct/internal/defaults"
	"github.com/gopher-launch/concoct/internal/execution"
	"github.com/gopher-launch/concoct/internal/gitrepo"
	"github.com/gopher-launch/concoct/internal/integration"
	"github.com/gopher-launch/concoct/internal/planning"
	"github.com/gopher-launch/concoct/internal/project"
	"github.com/gopher-launch/concoct/internal/prompt"
	"github.com/gopher-launch/concoct/internal/runloop"
	"github.com/gopher-launch/concoct/internal/workflow"
)

const usage = `Usage:
  concoct init <project>
  concoct version
  concoct defaults list
  concoct defaults show <logical-id>
  concoct status
  concoct exec [--dry-run] [--adapter <name>] [--model <model>] [--reasoning <level>] [--timeout <duration>]
  concoct exec inspect [--full-raw|--json] [<invocation-id>]
  concoct run [--approve <gate>] [--gate <gate>] [--max-actions <count>] [--max-cycles <count>]
              [--adapter <name>] [--model <model>] [--reasoning <level>] [--timeout <duration>]
  concoct next [--output <path>]
  concoct roadmap [--output <path>]
  concoct plan <roadmap-id> [--output <path>]
  concoct code [--output <path>]
  concoct code --complete
  concoct review [--output <path>]
  concoct review --reserve
  concoct review --complete
  concoct archive [--output <path>]
  concoct archive --complete
  concoct archive --complete --override-authority <authority> --override-reason <reason>
  concoct integrate [--continue|--abort]
  concoct help
`

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usage)
		return nil
	}
	switch args[0] {
	case "version":
		if len(args) != 1 {
			return fmt.Errorf("version accepts no arguments")
		}
		fmt.Fprint(stdout, buildinfo.Current().String())
		return nil
	case "defaults":
		if len(args) == 2 && args[1] == "list" {
			for _, r := range defaults.List() {
				fmt.Fprintf(stdout, "%s\t%s\t%s\n", r.ID, r.Kind, defaults.Provenance())
			}
			return nil
		}
		if len(args) == 3 && args[1] == "show" {
			data, err := defaults.Read(args[2], "defaults show")
			if err != nil {
				return err
			}
			_, err = stdout.Write(data)
			return err
		}
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("defaults requires `list` or `show <logical-id>`")
	case "init":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			fmt.Fprint(stderr, usage)
			return fmt.Errorf("init requires exactly one non-empty project target")
		}
		base, err := callerDir()
		if err != nil {
			return err
		}
		return project.Initialize(base, args[1], stdout)
	case "status":
		if len(args) != 1 {
			fmt.Fprint(stderr, usage)
			return fmt.Errorf("status accepts no positional arguments")
		}
		base, err := callerDir()
		if err != nil {
			return err
		}
		root, err := project.Discover(base)
		if err != nil {
			return err
		}
		if _, err := contract.CheckRead(root); err != nil {
			fmt.Fprint(stdout, contract.Describe(root))
			return nil
		}
		report := workflow.Detect(root)
		fmt.Fprint(stdout, report.String())
		if report.OperationalError != nil {
			return report.OperationalError
		}
		return nil
	case "why":
		base, err := callerDir()
		if err != nil {
			return err
		}
		root, err := project.Discover(base)
		if err != nil {
			return err
		}
		fmt.Fprint(stdout, contract.Describe(root))
		return nil
	case "exec":
		return runExec(args[1:], stdout, stderr)
	case "run":
		return runLifecycle(args[1:], stdout, stderr)
	case "next", "roadmap", "plan", "code", "review", "archive":
		if args[0] == "archive" && len(args) >= 2 && args[1] == "--complete" {
			return runArchiveTransition(args[2:], stdout, stderr)
		}
		if (args[0] == "code" || args[0] == "review") && len(args) == 2 && args[1] == "--complete" {
			return runRoleTransition(args[0], "complete", stdout)
		}
		if args[0] == "review" && len(args) == 2 && args[1] == "--reserve" {
			return runRoleTransition(args[0], "reserve", stdout)
		}
		return runPrompt(args, stdout, stderr)
	case "integrate":
		mode := ""
		if len(args) == 2 && (args[1] == "--continue" || args[1] == "--abort") {
			mode = strings.TrimPrefix(args[1], "--")
		} else if len(args) != 1 {
			fmt.Fprint(stderr, usage)
			return fmt.Errorf("integrate accepts only --continue or --abort")
		}
		base, err := callerDir()
		if err != nil {
			return err
		}
		root, err := project.Discover(base)
		if err != nil {
			return err
		}
		if err := contract.CheckMutate(root); err != nil {
			return err
		}
		if err := integration.Run(root, mode, os.Stdin, stdout); err != nil {
			return err
		}
		fmt.Fprint(stdout, workflow.Detect(root).String())
		return nil
	default:
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runLifecycle(args []string, stdout, stderr io.Writer) error {
	base, err := callerDir()
	if err != nil {
		return err
	}
	root, err := project.Discover(base)
	if err != nil {
		return err
	}
	options := runloop.Options{Input: os.Stdin, Progress: stderr}
	seen := map[string]bool{}
	for len(args) > 0 {
		flag := args[0]
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("%s requires one non-empty value", flag)
		}
		value := strings.TrimSpace(args[1])
		switch flag {
		case "--gate":
			options.Policy.Gates = append(options.Policy.Gates, value)
		case "--approve", "--max-actions", "--max-cycles", "--adapter", "--model", "--reasoning", "--timeout":
			if seen[flag] {
				return fmt.Errorf("%s may be specified only once", flag)
			}
			seen[flag] = true
			switch flag {
			case "--approve":
				options.Approve = value
			case "--max-actions", "--max-cycles":
				n, parseErr := strconv.Atoi(value)
				if parseErr != nil || n < 1 {
					return fmt.Errorf("%s requires a positive integer", flag)
				}
				if flag == "--max-actions" {
					options.Policy.MaxActions = n
				} else {
					options.Policy.MaxCycles = n
				}
			case "--adapter":
				options.Execution.Adapter = value
			case "--model":
				options.Execution.Model = value
			case "--reasoning":
				options.Execution.Reasoning = value
			case "--timeout":
				options.Execution.Timeout = value
			}
		default:
			fmt.Fprint(stderr, usage)
			return fmt.Errorf("unknown run option %q", flag)
		}
		args = args[2:]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	summary, runErr := runloop.Run(ctx, root, options)
	fmt.Fprint(stdout, summary.String())
	return runErr
}

func runExec(args []string, stdout, stderr io.Writer) error {
	base, err := callerDir()
	if err != nil {
		return err
	}
	root, err := project.Discover(base)
	if err != nil {
		return err
	}
	if len(args) > 0 && args[0] == "inspect" {
		args = args[1:]
		fullRaw := false
		metricsJSON := false
		if len(args) > 0 && (args[0] == "--full-raw" || args[0] == "--json") {
			fullRaw, metricsJSON, args = args[0] == "--full-raw", args[0] == "--json", args[1:]
		}
		if len(args) > 1 {
			fmt.Fprint(stderr, usage)
			return fmt.Errorf("exec inspect accepts --full-raw or --json and at most one invocation id")
		}
		if _, err := contract.CheckRead(root); err != nil {
			return err
		}
		id := ""
		if len(args) == 1 {
			id = args[0]
		}
		var content string
		if metricsJSON {
			data, err := execution.Metrics(root, id)
			if err != nil {
				return err
			}
			content = string(data) + "\n"
		} else {
			var err error
			content, err = execution.Inspect(root, id, fullRaw)
			if err != nil {
				return err
			}
		}
		_, err = io.WriteString(stdout, content)
		return err
	}
	if err := contract.CheckMutate(root); err != nil {
		return err
	}
	options := execution.Options{}
	seen := map[string]bool{}
	for len(args) > 0 {
		switch args[0] {
		case "--dry-run":
			if options.DryRun {
				return fmt.Errorf("--dry-run may be specified only once")
			}
			options.DryRun = true
			args = args[1:]
		case "--adapter", "--model", "--reasoning", "--timeout":
			if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
				return fmt.Errorf("%s requires one non-empty value", args[0])
			}
			if seen[args[0]] {
				return fmt.Errorf("%s may be specified only once", args[0])
			}
			seen[args[0]] = true
			value := strings.TrimSpace(args[1])
			switch args[0] {
			case "--adapter":
				options.Override.Adapter = value
			case "--model":
				options.Override.Model = value
			case "--reasoning":
				options.Override.Reasoning = value
			case "--timeout":
				options.Override.Timeout = value
			}
			args = args[2:]
		default:
			fmt.Fprint(stderr, usage)
			return fmt.Errorf("unknown exec option %q", args[0])
		}
	}
	if options.DryRun {
		prepared, err := execution.Prepare(root, options)
		if err != nil {
			return err
		}
		fmt.Fprint(stdout, execution.Describe(prepared))
		fmt.Fprintln(stdout, "Dry run: no process started and no workflow, Git, or runtime evidence written.")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	result, runErr := execution.Run(ctx, root, options, os.Stdin, stderr)
	if result.RecordPath != "" {
		fmt.Fprintf(stdout, "Invocation: %s\n", filepath.Base(result.RecordPath))
	}
	if result.Reconciliation.FinishedAt != "" {
		fmt.Fprintf(stdout, "Adapter disposition: %s\nResult accepted: %t\nObserved phase: %s\nNext: %s\nRetry safe from original evidence: %t\n", result.Reconciliation.InvocationDisposition, result.Reconciliation.ResultAccepted, result.Reconciliation.ObservedState, result.Reconciliation.ObservedNext, result.Reconciliation.RetrySafe)
		if result.Prepared.Direct {
			fmt.Fprintln(stdout, "Agent usage: no-agent mechanical action")
		} else {
			fmt.Fprintf(stdout, "Prompt bytes: %d\nAgent usage: %s\n", result.Prepared.Composition.ByteCount(), result.Measurement.UsageSummary())
		}
	}
	return runErr
}

func runArchiveTransition(args []string, stdout, stderr io.Writer) error {
	override := workflow.ArchiveOverride{}
	for len(args) > 0 {
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return fmt.Errorf("archive completion override flags require non-empty values")
		}
		switch args[0] {
		case "--override-authority":
			override.Authority = strings.TrimSpace(args[1])
		case "--override-reason":
			override.Reason = strings.TrimSpace(args[1])
		default:
			fmt.Fprint(stderr, usage)
			return fmt.Errorf("unknown archive completion option %q", args[0])
		}
		args = args[2:]
	}
	if (override.Authority == "") != (override.Reason == "") {
		return fmt.Errorf("unapproved archival requires both --override-authority and --override-reason")
	}
	base, err := callerDir()
	if err != nil {
		return err
	}
	root, err := project.Discover(base)
	if err != nil {
		return err
	}
	if err := contract.CheckMutate(root); err != nil {
		return err
	}
	result, err := workflow.CompleteArchive(root, override)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, result.Message)
	if result.Committed {
		fmt.Fprintf(stdout, "Commit: %s\n", result.Commit)
	}
	fmt.Fprint(stdout, workflow.Detect(root).String())
	return nil
}

func runRoleTransition(command, action string, stdout io.Writer) error {
	base, err := callerDir()
	if err != nil {
		return err
	}
	root, err := project.Discover(base)
	if err != nil {
		return err
	}
	if err := contract.CheckMutate(root); err != nil {
		return err
	}
	var result workflow.TransitionResult
	if command == "code" {
		result, err = workflow.CompleteDeveloper(root)
	} else if action == "reserve" {
		result, err = workflow.ReserveReview(root)
	} else {
		result, err = workflow.CompleteReview(root)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, result.Message)
	if result.Committed {
		fmt.Fprintf(stdout, "Commit: %s\n", result.Commit)
	}
	fmt.Fprint(stdout, workflow.Detect(root).String())
	return nil
}

func runPrompt(args []string, stdout, stderr io.Writer) error {
	command := args[0]
	positional, output, err := parsePromptArgs(args[1:])
	if err != nil {
		fmt.Fprint(stderr, usage)
		return err
	}
	roadmapID := ""
	if command == "plan" {
		if len(positional) != 1 || strings.TrimSpace(positional[0]) == "" {
			fmt.Fprint(stderr, usage)
			return fmt.Errorf("plan requires exactly one non-empty roadmap id")
		}
		roadmapID = positional[0]
	} else if len(positional) != 0 {
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("%s accepts no positional arguments", command)
	}
	base, err := callerDir()
	if err != nil {
		return err
	}
	root, err := project.Discover(base)
	if err != nil {
		return err
	}
	// Rendering fully interprets project workflow evidence. Check mutation
	// compatibility too because plan can create a branch and --output can write.
	if err := contract.CheckMutate(root); err != nil {
		return err
	}
	request := prompt.Request{Command: command, RoadmapID: roadmapID}
	var session *planning.Session
	if command == "plan" {
		if output != "" {
			target := output
			if !filepath.IsAbs(target) {
				target = filepath.Join(base, target)
			}
			if _, statErr := os.Stat(target); statErr == nil {
				return fmt.Errorf("create output %s without overwriting: file exists", target)
			}
		}
		if _, ok, openErr := gitrepo.Open(root); openErr != nil {
			return openErr
		} else if ok {
			if output != "" {
				target := output
				if !filepath.IsAbs(target) {
					target = filepath.Join(base, target)
				}
				rel, relErr := filepath.Rel(root, target)
				if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return fmt.Errorf("Git-backed plan output must be outside the project so the new task branch remains clean")
				}
			}
		}
		session, err = planning.Start(root, roadmapID)
		if err != nil {
			return err
		}
		request = session.Request
	}
	rollback := func() {
		if session != nil {
			_ = session.Rollback()
		}
	}
	var content []byte
	if session != nil {
		content, err = session.Render()
	} else {
		content, err = prompt.Render(root, request)
	}
	if err != nil {
		rollback()
		return err
	}
	if output == "" {
		_, err = stdout.Write(content)
		if err != nil {
			rollback()
		}
		return err
	}
	if !filepath.IsAbs(output) {
		output = filepath.Join(base, output)
	}
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		rollback()
		return fmt.Errorf("create output %s without overwriting: %w", output, err)
	}
	wrote := false
	defer func() {
		if !wrote {
			_ = os.Remove(output)
		}
	}()
	if _, err = file.Write(content); err != nil {
		_ = file.Close()
		rollback()
		return fmt.Errorf("write output %s: %w", output, err)
	}
	if err = file.Close(); err != nil {
		rollback()
		return fmt.Errorf("close output %s: %w", output, err)
	}
	wrote = true
	return nil
}

func parsePromptArgs(args []string) ([]string, string, error) {
	var positional []string
	output := ""
	for i := 0; i < len(args); i++ {
		if args[i] != "--output" {
			positional = append(positional, args[i])
			continue
		}
		if output != "" || i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
			return nil, "", fmt.Errorf("--output requires exactly one non-empty path")
		}
		output = args[i+1]
		i++
	}
	return positional, output, nil
}

func callerDir() (string, error) {
	if dir := os.Getenv("CONCOCT_CALLER_DIR"); dir != "" {
		return filepath.Abs(dir)
	}
	return os.Getwd()
}
