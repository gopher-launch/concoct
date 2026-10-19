# Concoct

Concoct runs one bounded roadmap task at a time with a reagent and an independent iragent. It retains active work outside the project, presents an exact reviewed candidate for operator acceptance, integrates locally only with that acceptance ID, then archives completion evidence. The first supported runner is Linux Codex CLI 0.147.0. The real external-project demonstration remains CON-056, after corrective tasks CON-058 and CON-059.

## Build

Requires Go 1.27.2 or newer.

```sh
go test ./...
go vet ./...
go build -o /tmp/concoct ./cmd/concoct
/tmp/concoct help
/tmp/concoct version
```

Run in a Git checkout with `AGENTS.md`, `roadmap.md` and `capabilities.md`. A fresh run requires a clean named parent branch and a ready task with delivered dependencies. Concoct stores run records, task/review evidence and worktrees under `$HOME/.concoct` by default; `--root PATH` selects one external root. It does not install templates, prompts or notes into the project. The source tree and current project documents define implemented behavior.

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

Agent messages stream as reported. The final `Concoct:` line is the verified controller disposition. `--quiet` suppresses streamed messages but keeps the final result. `--json` writes JSON lines: `agent_progress` events followed by one `result` object. Neither mode turns an agent statement into verified completion. No-work is a normal result. Exit codes are 0 for completed, idle or no-work; 10 for a decision; 11 for authorization; 12 for acceptance; 13 for independent-review pause; 14 for safely stopped; 15 for abnormal stop; 16 for busy; 17 for maintenance; 18 for other retained work; and 2 for invalid arguments or prerequisites. Returned IDs and the exact next command guide follow-up actions.

The read-only `internal/roadmap` parser accepts `## ID — title` entries with contiguous `Kind`, `Status`, `Priority`, optional `Parent` and `Depends on` bullets. Objective and acceptance remain prose. Supported kinds are `epic`, `feature`, `task`; statuses are `planned`, `ready`, `delivered`; priorities are `high`, `medium`, `low`. Completed IDs can resolve through concise `capabilities.md` entries without reading archived narratives.

## Construction

Use ordinary manually directed Codex sessions for one bounded roadmap task at a time. Record construction evidence outside this repository under `$HOME/.concoct/manual/<task-id>/`; independent review and explicit operator acceptance precede local integration. Historical Git, including CON-042, is reference material for selective harvesting. CON-059 is the next ready task; CON-056 remains planned until the corrections are delivered.
