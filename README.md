# Concoct

Concoct coordinates one bounded roadmap task through reagent implementation, independent iragent review, correction when needed, and operator-authorized local integration. It never pushes or publishes.

## First use

Build with Go 1.27.2 or newer on Linux, with Codex CLI 0.147.0 available as `codex` or through `--codex PATH`. Start in a clean Git checkout on a named parent branch. Supply `AGENTS.md`, a bounded `roadmap.md` with ready work and `capabilities.md` describing delivered behavior. Run `concoct run`. Reagent selects one task, creates its branch in this checkout, implements and commits; Concoct automatically asks a separate iragent to assess that exact candidate in the same checkout.

For a repository with multiple roadmaps, select the authoritative backlog for fresh work with `concoct run --roadmap roadmap.md` or, for example, `concoct run --roadmap deploy/roadmap.md`. Relative paths resolve from the selected repository; absolute paths are accepted. Concoct validates the roadmap before dispatch, records its resolved path with active work, supplies its contents throughout implementation and review, and rejects attempts to switch roadmaps while that work remains active. A `capabilities.md` beside the selected roadmap applies when present; otherwise the repository-root capability document applies. Task and review evidence is stored under a compact namespace derived from the canonical roadmap path, so the same task ID in another roadmap remains distinct. Run summaries and `status` show the recorded roadmap.

If iragent requests changes, Concoct sends its assessment and findings to reagent, then requests another independent assessment of the corrected commit. An approval displays the task, exact reviewed commit, role summaries and external review location. When an invocation remains, inspect the candidate and evidence before explicitly authorizing local integration with `concoct run --approve COMMIT`. Concoct passes that exact authorization to reagent, which checks Git state, integrates locally and reports its result. A successful integration is retained in an external completion record and active state is released. `concoct status` shows the latest completed handoff; a later plain `concoct run` can start the next ready task from the current clean named checkout, including a descendant commit added by the operator after integration. Concoct does not decide whether the engineering or review narrative is correct.

The default role model is the provider recommendation; reagent uses medium effort and iragent high effort. Fresh runs can set `--reagent-model`, `--iragent-model`, `--reagent-effort`, `--iragent-effort`, `--dispatch-limit`, `--review-limit`, `--turn-timeout` and `--stall-timeout`. Timeouts use Go durations such as `20m`. A new run defaults to twelve agent invocations and three completed independent assessments. Consumption continues across correction, replacement sessions and integration. No integration starts without exact operator approval.

If an agent or provider stops before Concoct accepts a valid result, a plain `concoct run` remains stopped. Inspect `concoct status`, the reported provider diagnostic, the checkout and external evidence, then explicitly request a replacement turn:

```sh
/tmp/concoct run --repo /path/to/checkout --root /path/to/external-root --recover
```

Recovery runs under the normal execution lock. Before every replacement dispatch it reports the failed role, pending step, diagnostic, consumed limits and freshly observed source/evidence changes; verifies recorded provider and owned-process identities have ended; and starts a fresh session with a recovery-specific prompt. The durable state remains stopped at `recovering` through provider, profile and fresh-session setup; only the atomic replacement in-flight record restores the ordinary workflow step. Commits, tracked and untracked changes, diagnostics and evidence are preserved. Partial review prose found during preparation or a retry is archived separately and never counted as an assessment. Review recovery revalidates the exact clean candidate on every explicit retry. Integration recovery accepts only safely reconcilable, unchanged parent/task refs and gives one exclusive instruction: guarded fast-forward when the parent remains at base, or verification without ref updates when it already names the authorized commit.

The failed invocation remains consumed, and an assessment is consumed only after its valid result is verified. A replacement failure can be recovered again within the retained limits. Add only the replacement role's model/effort flags to `--recover` when an override is needed; Concoct validates and records that change without altering the other role. If an execution allowance is already exhausted, the requested override is retained with the recovery audit. Authorizing a higher limit leaves the task stopped at `recovering`; a subsequent explicit `run --recover` refreshes its bindings, presents the recovery summary and validates the retained override before dispatch. Recovery is refused if prior processes remain live or cannot be verified, the pending step is ambiguous, the exact review candidate is dirty or different, or integration refs conflict. Older records without a pre-turn source/process checkpoint remain stopped for manual inspection rather than being guessed into a recovery.

If Concoct stops at an execution limit, its summary shows every exhausted consumed/total allowance and prints a context-preserving continuation command. The suggested values are new total ceilings, not additional units. For example, after three completed assessments, authorizing a review total of five preserves `3/5` and leaves two assessments available:

```sh
/tmp/concoct run --repo /path/to/checkout --root /path/to/external-root --approve-review-limit-increase 5
```

Use `--approve-invocation-limit-increase TOTAL` for the invocation ceiling; both increase flags may be supplied together. Each total must be strictly greater than its recorded ceiling. Concoct validates the entire request before recording either change, preserves completed work and consumed counts, and continues the retained step only if no other allowance or workflow prerequisite still blocks it. These authorization flags do not change active role settings or replace exact-commit integration approval.

```sh
go test ./...
go vet ./...
go build -o /tmp/concoct ./cmd/concoct
/tmp/concoct help
/tmp/concoct version
/tmp/concoct status
/tmp/concoct run
/tmp/concoct run --recover
/tmp/concoct run --approve REVIEWED_COMMIT
```

Development builds obtain the Git revision from Go's embedded VCS build metadata and report the first 12 revision characters, for example `0.6.0-dev+5745faded2b5`. A build from a modified checkout adds `.dirty`. If Go cannot provide revision metadata, `concoct version` reports `0.6.0-dev+unknown`; release version strings remain unchanged. Ordinary `go build` and `go install` commands embed the metadata without custom linker flags, provided Go can identify the containing Git checkout.

`--repo PATH` selects the checkout and `--root PATH` selects an external Concoct storage root. `status` reads active state or the latest external completion record without contacting Codex. Continuation commands retain the recorded roadmap as well as the invocation's executable, repository and explicit external root. Agent prose streams with role labels and blank lines between messages; structured commentary displays its human summary. `--quiet` suppresses progress while keeping result summaries; `--json` emits JSON. Completed agent results and Concoct's coordination status have distinct labels. A blocker displays a question; `concoct run --answer TEXT` sends the answer to its requesting role. Completed handoff boundaries can continue in a later `run`. Only an accepted `integrated` result releases active work; failed integration, blockers, invalid results and uncertain execution remain stopped on that task. Completion-record maintenance is retryable and never consumes an agent invocation. A malformed result, execution failure, timeout or limit exhaustion retains source and external state and reports the issue. An unrecorded in-flight turn is reported as uncertain on the next plain `run`; only explicit `--recover` can dispatch a checked replacement. Each provider execution writes stderr to an external `provider-*.log` file beside Concoct's state; `run` and `status` show its path, and the file can be read with `cat PATH`. Error-looking log lines alone do not mark a successful turn as failed. Task and review evidence files must be readable and outside the project before their paths are used in a handoff or displayed as an assessment. No old-run migration or automatic historical callback recovery is provided.

## Construction

Use ordinary manually directed Codex sessions for one bounded roadmap task at a time. Keep manual task and review records under `$HOME/.concoct/manual/<task-id>/` outside this repository. Independent review and explicit operator acceptance of the exact candidate precede local integration. CON-042 is frozen reference material, not an active implementation branch.
