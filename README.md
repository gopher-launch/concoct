# Concoct

Concoct coordinates one bounded roadmap task through reagent implementation and an independent iragent assessment. This slice stops after displaying the review; correction routing and operator-approved integration are planned under CON-065. It never pushes or publishes.

## First use

Build with Go 1.27.2 or newer on Linux, with Codex CLI 0.147.0 available as `codex` or through `--codex PATH`. Start in a clean Git checkout on a named parent branch. Supply `AGENTS.md`, a bounded `roadmap.md` with ready work and `capabilities.md` describing delivered behavior. Run `concoct run`. Reagent selects one task, creates its branch in this checkout, implements and commits; Concoct then automatically asks a separate iragent to assess that exact candidate in the same checkout. Agent task and review Markdown live outside the project. Concoct does not decide whether the engineering or review narrative is correct.

The default role model is the provider recommendation; reagent uses medium effort and iragent high effort. Fresh runs can set `--reagent-model`, `--iragent-model`, `--reagent-effort`, `--iragent-effort`, `--dispatch-limit`, `--turn-timeout` and `--stall-timeout`. Limits use Go durations such as `20m`. A new run defaults to twelve agent invocations. A completed review leaves the task branch and candidate untouched for operator inspection.

```sh
go test ./...
go vet ./...
go build -o /tmp/concoct ./cmd/concoct
/tmp/concoct help
/tmp/concoct version
/tmp/concoct status
/tmp/concoct run
```

`--repo PATH` selects the checkout and `--root PATH` selects an external Concoct storage root. `status` reads retained state without contacting Codex. Agent prose streams with role labels and blank lines between messages. `--quiet` suppresses progress; `--json` emits JSON. Structured result summaries and the final Concoct coordination status are separate. A malformed result, execution failure, timeout, explicit blocker or invocation exhaustion retains source and external state and reports the issue. An unrecorded in-flight turn is reported as uncertain on the next `run`; it is not dispatched again automatically. Task and review evidence files must be readable and outside the project before their paths are used in a handoff or displayed as an assessment. No old-run migration or automatic historical callback recovery is provided in this slice.

## Construction

Use ordinary manually directed Codex sessions for one bounded roadmap task at a time. Keep manual task and review records under `$HOME/.concoct/manual/<task-id>/` outside this repository. Independent review and explicit operator acceptance of the exact candidate precede later local integration. CON-042 is frozen reference material, not an active implementation branch.
