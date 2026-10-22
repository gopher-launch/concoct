# Concoct

Concoct coordinates one bounded roadmap task through reagent implementation, independent iragent review, correction when needed, and operator-authorized local integration. It never pushes or publishes.

## First use

Build with Go 1.27.2 or newer on Linux, with Codex CLI 0.147.0 available as `codex` or through `--codex PATH`. Start in a clean Git checkout on a named parent branch. Supply `AGENTS.md`, a bounded `roadmap.md` with ready work and `capabilities.md` describing delivered behavior. Run `concoct run`. Reagent selects one task, creates its branch in this checkout, implements and commits; Concoct automatically asks a separate iragent to assess that exact candidate in the same checkout.

If iragent requests changes, Concoct sends its assessment and findings to reagent, then requests another independent assessment of the corrected commit. An approval displays the task, exact reviewed commit, role summaries and external review location. When an invocation remains, inspect the candidate and evidence before explicitly authorizing local integration with `concoct run --approve COMMIT`. Concoct passes that exact authorization to reagent, which checks Git state, integrates locally and reports its result. Concoct does not decide whether the engineering or review narrative is correct.

The default role model is the provider recommendation; reagent uses medium effort and iragent high effort. Fresh runs can set `--reagent-model`, `--iragent-model`, `--reagent-effort`, `--iragent-effort`, `--dispatch-limit`, `--review-limit`, `--turn-timeout` and `--stall-timeout`. Limits use Go durations such as `20m`. A new run defaults to twelve agent invocations and three completed independent assessments. Consumption continues across correction, replacement sessions and integration. No integration starts without exact operator approval.

```sh
go test ./...
go vet ./...
go build -o /tmp/concoct ./cmd/concoct
/tmp/concoct help
/tmp/concoct version
/tmp/concoct status
/tmp/concoct run
/tmp/concoct run --approve REVIEWED_COMMIT
```

`--repo PATH` selects the checkout and `--root PATH` selects an external Concoct storage root. `status` reads retained state without contacting Codex. Agent prose streams with role labels and blank lines between messages. `--quiet` suppresses progress; `--json` emits JSON. Structured result summaries and Concoct's coordination status are separate. A blocker displays a question; `concoct run --answer TEXT` sends the answer to its requesting role. Completed handoff boundaries can continue in a later `run`. A malformed result, execution failure, timeout or limit exhaustion retains source and external state and reports the issue. An unrecorded in-flight turn is reported as uncertain on the next `run`; it is not dispatched again automatically. Task and review evidence files must be readable and outside the project before their paths are used in a handoff or displayed as an assessment. No old-run migration or automatic historical callback recovery is provided.

## Construction

Use ordinary manually directed Codex sessions for one bounded roadmap task at a time. Keep manual task and review records under `$HOME/.concoct/manual/<task-id>/` outside this repository. Independent review and explicit operator acceptance of the exact candidate precede local integration. CON-042 is frozen reference material, not an active implementation branch.
