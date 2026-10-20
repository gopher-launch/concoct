# Concoct

Concoct runs one bounded roadmap task at a time with a reagent and an independent iragent. It retains active work outside the project, presents an exact reviewed candidate for operator acceptance, integrates locally only with that acceptance ID, then archives completion evidence. The first supported runner is Linux Codex CLI 0.147.0.

## First use

Build with Go 1.27.2 or newer on Linux, with Codex CLI 0.147.0 available as `codex` or via `--codex PATH`. Start in a clean, initialized Git project on a named parent branch. Supply `AGENTS.md`, a bounded `roadmap.md` with one ready task whose dependencies are delivered, and `capabilities.md` describing delivered behavior. Run `concoct status` to inspect any retained work, then `concoct run` to select and engineer one task. The first run takes role model/effort choices from `--reagent-model`, `--iragent-model`, `--reagent-effort` and `--iragent-effort` when supplied; otherwise it uses the provider-recommended model with medium reagent and high iragent effort. Those settings stay bound to active work.

During `run`, ordinary agent messages stream before Concoct's verified disposition. If Concoct needs input, authorization or recovery, use the displayed exact command or inspect with `concoct status [TASK]`. A successful independent review shows the candidate commit, acceptance ID, exact Git diff command, task evidence and review evidence. Inspect those materials before running the displayed `concoct run --approve ACCEPTANCE`; this locally integrates that exact candidate. If completion maintenance is pending, plain `concoct run` retries it without another agent turn. No command pushes or publishes.

Run records, evidence and workspaces live under `$HOME/.concoct` by default, outside the project. `--repo PATH` selects a project and `--root PATH` selects another external storage root. `status` is read-only and works while `run` owns the execution lock.

## Build

Requires Go 1.27.2 or newer.

```sh
go test ./...
go vet ./...
go build -o /tmp/concoct ./cmd/concoct
/tmp/concoct help
/tmp/concoct version
```

Concoct does not install templates, prompts or notes into the project. The source tree and current project documents define implemented behavior.

## Operator commands

```sh
concoct run
concoct status [TASK] [--json]
concoct run --answer QUESTION --text TEXT
concoct run --answer QUESTION --choice OPTION
concoct run --answer QUESTION --answer-file PATH
concoct run --authorize DISPATCH
concoct run --approve ACCEPTANCE
concoct run --extend TASK --dispatch-limit N
concoct run --extend TASK --review-limit N
concoct run --extend TASK --recovery-limit N
concoct run --defer TASK --reason TEXT
concoct run --abandon TASK --reason TEXT
concoct run --resume TASK
```

`run` resumes active work; otherwise it selects one eligible task. It starts a fresh reagent thread, freezes its verified candidate, and launches a separate reviewer. A reviewer approval returns a candidate and acceptance ID, a command for its exact base-to-candidate diff, the external task and finalized review evidence paths, and the recorded checks, criteria, findings and documentation reconciliation. Inspect these materials, then use the displayed exact `--approve` command to integrate locally. Approval does not push or publish. A completion or workspace cleanup problem returns `maintenance_pending`; plain `run` retries maintenance without another agent turn. Ctrl-C requests graceful interruption and retains recoverable state.

`status` is read-only, works while execution holds the repository lock, and does not contact Codex. Output includes the phase, hold, candidate, role model/effort/full-access settings, observed token receipts, quota observation age and windows, consumed bounds, pending stable IDs and next command. A question includes the requesting role, evidence, missing action, safe work, next step and any option IDs with tradeoffs. An exhausted limit shows the pending request and task-bound extension command. Acceptance status includes the same bound inspection materials as `run`. A task argument must match the active task. `--repo PATH` selects the checkout and `--root PATH` selects external storage; both also apply to `run`. On a fresh `run`, `--reagent-model`, `--iragent-model`, `--reagent-effort`, `--iragent-effort` and `--gate selection,engineering,review,correction,recovery` set explicit invocation choices. The provider's recommended model is the default. The role and gate choices are retained with active work and cannot be changed by an ordinary resume.

Agent messages stream by item, with a blank line between messages. A structured final agent result appears as an `Agent reported` summary; the separate `Concoct verified:` line is the controller disposition. Malformed final results are identified without printing raw control JSON. `--quiet` suppresses streamed messages but keeps the agent result and final status. `--json` writes JSON lines: `agent-progress` events followed by one `result` object with optional `agent-reported`. Public Concoct JSON field names and disposition/event values use kebab-case (for example, `task-id`, `observed-tokens`, `ready-for-acceptance`); this changes the CLI output contract from underscore names without changing older durable records or provider-owned messages. A missing `observed-tokens` field means no token receipt; zero means a measured zero. Neither mode turns an agent statement into verified completion.

Human timestamps use the local timezone, show the zone and numeric offset, and honor `TZ` (for example `TZ=UTC concoct status` or `TZ=America/Vancouver concoct status`). JSON timestamps and durable records remain UTC instants. No-work is a normal result. Exit codes are 0 for completed, idle or no-work; 10 for a decision; 11 for authorization; 12 for acceptance; 13 for independent-review pause; 14 for safely stopped; 15 for abnormal stop; 16 for busy; 17 for maintenance; 18 for other retained work; and 2 for invalid arguments or prerequisites. Returned IDs and the exact next command guide follow-up actions.

The read-only `internal/roadmap` parser accepts `## ID — title` entries with contiguous `Kind`, `Status`, `Priority`, optional `Parent` and `Depends on` bullets. Objective and acceptance remain prose. Supported kinds are `epic`, `feature`, `task`; statuses are `planned`, `ready`, `delivered`; priorities are `high`, `medium`, `low`. Completed IDs can resolve through concise `capabilities.md` entries without reading archived narratives.

## Construction

Use ordinary manually directed Codex sessions for one bounded roadmap task at a time. Record construction evidence outside this repository under `$HOME/.concoct/manual/<task-id>/`; independent review and explicit operator acceptance precede local integration. Historical Git, including CON-042, is reference material for selective harvesting.
