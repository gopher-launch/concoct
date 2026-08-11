---
id: CON-010
title: Execute one recommended action through an agent adapter
roadmap-id: CON-010
status: implementation-complete
created: 2026-08-11
updated: 2026-08-11
remediates-review: review-01.md
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-010-execute-one-recommended-action-through-an-agent
  base: c30e7718462b3dba1affe6aa6ce924786b5312db
  archive-commit: self
  status: archived
capability-impact:
  type: add
  ids:
    - CAP-013
  rationale: Adds one-shot execution of an authorized workflow recommendation through a configured agent adapter, with retained inspection evidence and observed-state outcome validation.
---

# Task Plan

## Goal

Implement `concoct exec` so Concoct can resolve the one action currently
recommended by its existing workflow authority, run that action once through a
configured agent adapter, validate the correlated structured outcome against
the resulting repository state, and return control with an inspectable summary
and next recommendation.

## Context

Concoct already derives workflow state and a next command, renders
version-matched role prompts, validates role completion and Git transitions,
and defines a versioned action/outcome protocol in `internal/orchestration`.
No current command combines those capabilities into a supervised adapter run.

CON-010 supplies that one-shot execution boundary. It is intentionally not a
lifecycle loop: one invocation authorizes at most one action, starts at most one
adapter process, reconciles the result once, and then stops. CON-033 and
CON-034 retain ownership of repeated execution, durable run recovery, and
configurable orchestration gates.

## Why this matters

Today users must export or copy a rendered prompt into an agent, wait for the
role work, invoke the applicable completion boundary, and inspect the resulting
state themselves. A guarded one-shot command removes that mechanical handoff
without weakening role ownership, policy decisions, review independence, Git
safety, or the rule that durable repository evidence outranks an agent claim.

## Current state

- `internal/workflow.Report.Next` is the authoritative state-derived next
  command outside ready-state semantic selection. `InspectNextActionEvidence`
  and the `next` renderer provide the ready-state Product Owner decision input.
- `internal/prompt.Render` produces the exact effective instructions for
  Product Owner, Task Planner, Developer, Reviewer, and Archivist work.
- `internal/orchestration` defines action correlation, action kinds, supported
  outcome classes, bounded durable facts, observed-state validators, and an
  atomic no-replace result boundary, but deliberately launches nothing.
- `internal/cli` dispatches prompt, completion, archive, and integration
  commands. It has no `exec`, adapter, runtime-record, cancellation, timeout,
  inspection, retention, or execution-profile surface.
- `.concoct/config.yaml` is optional and currently read only for Git auto-push;
  there is no shared strict configuration model or user-level execution
  configuration.
- Generated projects do not yet establish the ignored
  `.concoct/runtime/invocations/` boundary required for local execution records.

## Target state

- `concoct exec` resolves exactly one executable action from the same workflow
  report, policy resolution, and ready-state evidence used by `status`, `next`,
  and the manual role commands. It refuses informational, ambiguous, blocked,
  invalid, recovery-choice, or explicitly human-gated recommendations.
- `concoct exec --dry-run` performs the complete authorization, prompt,
  adapter, profile, command, timeout, and storage resolution without starting
  a process or creating an invocation record.
- An executable-owned adapter registry remains separate from workflow action
  selection. The initial `codex` adapter runs non-interactively in the exact
  repository and task context and normalizes its result into the existing
  orchestration protocol.
- The selected action receives byte-identical role guidance to the
  corresponding manual prompt command. Direct workflow operations without a
  role prompt retain their existing executable authority and safety behavior;
  the adapter does not duplicate or bypass transition logic.
- Every real attempt has a unique private runtime directory containing the
  exact prompt, action envelope, sanitized resolved command/profile metadata,
  one normalized result when delivered, bounded redacted logs, and a
  reconciliation record. Runtime evidence is ignored by Git and is not itself
  workflow authority.
- Cancellation, startup failure, timeout, abnormal exit, missing or malformed
  output, stale correlation, and postcondition mismatch terminate the one-shot
  attempt without falsely accepting completion. Actual repository state is
  always re-detected and reported.
- `concoct exec inspect [<invocation-id>]` reads retained attempt material
  without regenerating it and reports the retained prompt, quoted/redacted
  command, structured result, bounded logs, reconciliation, and observed state.

## Design constraints

- Preserve one authority for workflow state, next-action selection, policy
  gates, and transition validation. Do not parse the human-readable `Next`
  string as the primary decision model or introduce an adapter-specific state
  machine.
- Preserve ready-state Product Owner judgment. In ready state, the one
  executable action is the Product Owner `next` pass; its structured result may
  recommend a concrete follow-up, but `exec` must return rather than
  automatically planning or starting a second role.
- Extend the structured outcome only where needed to represent and validate a
  bounded next recommendation from a decision action. Keep protocol `v1`
  backward-compatible, action-specific, and agent-neutral.
- For role actions, render through the existing prompt package and prove the
  retained bytes equal manual rendering for the same authorized snapshot. Do
  not maintain separate agent-only personas or handoffs.
- Keep Codex command construction in the Codex adapter. The workflow and common
  orchestration packages may refer only to adapter-neutral roles, settings,
  actions, outcomes, and process lifecycle events.
- Invoke Codex through supported non-interactive controls: send the prompt via
  standard input, set the repository explicitly, request schema-constrained
  final output, and map model and reasoning choices to adapter-owned flags or
  configuration. Human-readable progress and exit status remain diagnostics.
- Never pass a bypass flag, suppress Codex safety rules, inherit or persist an
  unrestricted environment, expose authentication material in command
  metadata, or turn `exec` flags into permission escalation. Resolve and show
  the effective sandbox/permission posture before launch.
- Treat `concoct exec` as authorization to launch only the displayed action.
  Existing integration, push, approval, intervention, and policy gates remain
  authoritative; a required human decision stops execution or returns control.
- Use private permissions for runtime directories and files, create result
  material without replacement, and make retry create a fresh authorization
  from current evidence rather than replaying an envelope.
- Retain exact prompts and structured records, but bound diagnostic streams and
  sanitize command metadata. Never retain environment contents, credentials,
  authorization headers, or unrestricted tool output.
- Parse project and user configuration strictly, reject unknown or unsupported
  adapter/profile values before process start, and attribute every resolved
  value to its source. Reuse the existing project configuration path and use
  the platform user-config directory for the matching user-level schema.
- Preserve all current manual commands and output contracts. `exec` is an
  additional path, not a replacement for prompt inspection/export or explicit
  role completion.

## Non-goals

- No automatic loop across multiple roles or repeated execution until a
  terminal state.
- No resumable or interactive agent session, remote/cloud agent transport, or
  automatic external-adapter discovery.
- No autonomous roadmap ranking or task selection beyond the Product Owner
  action already represented by `concoct next`.
- No bypass of review, archival, integration, push, permissions, or human
  intervention policy.
- No replay of a retained action envelope, workflow mutation inferred from
  process success, or acceptance based only on adapter output.
- No general execution-history product, crash-resumable coordinator, shared
  locking system, concurrent task runner, or stale-process recovery protocol.
- No broad configuration/profile framework from CON-023 and no unrelated CLI,
  workflow, or documentation refactor.
- No additional built-in adapter until another adapter supplies a concrete
  transport and trust requirement.

## Working assumptions and decisions

- `CAP-013` is the next available capability identifier for the new observable
  direct-execution behavior. CAP-012 remains the underlying validation
  boundary; compatible protocol refinements made for execution do not replace
  its accepted meaning.
- The shared recommendation resolver should return typed action identity,
  role, manual-command source, executability, and refusal reason. Status text,
  prompt rendering, authorization, dry-run, and execution consume this result.
- Ready-state `product-owner-next` is a valid one-shot agent action even when
  its successful decision does not mutate repository artifacts. Its outcome
  must carry a validated bounded recommendation; state-changing actions still
  require their registered observable postcondition.
- The initial adapter is selected as `codex` unless a supported command,
  project, or user setting overrides it. Precedence is: one-invocation flags,
  project role settings, user role settings, adapter role defaults, then the
  adapter general default.
- The public one-invocation surface may include `--adapter`, `--model`,
  `--reasoning`, and `--timeout`; these values affect only the current run and
  never rewrite configuration.
- The default timeout is 30 minutes. After cancellation or timeout, request
  graceful termination, close result acceptance, use a short bounded grace
  period, then force termination if required before reconciliation.
- Retention defaults are 20 completed attempts and 14 days. The implementation
  must also choose and document conservative finite per-log and total-byte
  defaults, retain the current attempt while it is active, and prune only
  completed attempts after reconciliation.
- Current Codex CLI behavior supports the required non-interactive foundation:
  `codex exec`, explicit working directory and sandbox, `--model`,
  `model_reasoning_effort`, `--output-schema`, final-output capture, ephemeral
  sessions, progress output, and standard-input prompts. Availability and
  supported configured values must still be checked before launch.

## Risks and open questions

- Recommendation drift is the primary authority risk: the action must be
  authorized and launched from one evidence snapshot, and a changed snapshot
  must invalidate late or mismatched completion claims.
- Ready-state recommendations do not necessarily mutate artifacts. Their
  validated result contract must stay distinct from transition completion so
  the existing no-observed-change rejection is not weakened for other actions.
- The current action snapshot hashes core task inputs but not every review,
  policy, configuration, Git, or archive input that can affect authorization.
  Execution authorization must cover all material evidence or revalidate it
  immediately before launch and reconciliation.
- A role agent may successfully author work but fail before emitting a result,
  or emit a failure after durable state advanced. Reconciliation must report
  observed state truth without fabricating either failure rollback or success.
- Cancellation and timeout can race with result publication or leave child
  processes alive. The process boundary needs deterministic result closure,
  bounded termination, and platform-appropriate process-tree tests.
- Retained logs or command details may contain sensitive material. Redaction,
  allowlisted metadata, private permissions, size bounds, and negative tests
  are required; raw environment capture is prohibited.
- Configuration precedence can diverge between dry-run and execution unless
  both consume one immutable resolved invocation description.
- Runtime retention and inspection must tolerate partial records caused by
  crashes without treating them as workflow evidence or blocking manual work.
- No unresolved product decision blocks planning. Adapter choice, profile
  precedence, one-shot behavior, intervention boundaries, timeout, retention,
  inspection, and manual fallback are defined by the roadmap.

## Implementation phases

### Phase 1 — Resolve one authorized execution

Status: `complete`

- Extract a typed recommendation/action resolution from the existing workflow,
  policy, and ready-state authorities and use it for status-facing guidance,
  orchestration authorization, dry-run, and real execution.
- Classify actionable role work, direct executable operations, ambiguous or
  recovery-choice states, blockers, informational outcomes, and required human
  decisions without stringly typed command parsing.
- Reconcile the action/outcome contract for decision actions, full material
  evidence freshness, and one-shot next-recommendation reporting while
  retaining action-specific observed-state completion validation.

### Phase 2 — Add adapter and profile resolution

Status: `complete`

- Define an adapter-neutral execution interface and executable-owned registry,
  then implement the initial `codex` adapter behind it.
- Define strict project and user configuration for adapter selection,
  role-specific execution settings, timeout, and retention. Apply and expose
  the required precedence and source attribution.
- Build one inspectable resolved invocation shared by dry-run and launch.
  Validate executable availability, model/reasoning values, timeout, working
  directory, prompt source, output schema, and safety posture before starting.

### Phase 3 — Supervise one invocation safely

Status: `complete`

- Create a private ignored invocation directory and retain the exact prompt,
  action envelope, sanitized resolved metadata, result target, bounded logs,
  and lifecycle timestamps/disposition.
- Run the adapter once in the exact repository/task context, stream bounded
  progress, normalize schema-constrained output, and publish at most one
  correlated result through the atomic no-replace boundary.
- Implement cancellation, timeout, graceful termination, bounded forced
  termination, startup/exit/missing-result handling, late-result closure, and
  unconditional post-run state reconciliation.

### Phase 4 — Reconcile, inspect, and retain

Status: `complete`

- Validate outcomes through orchestration authority and observed repository,
  workflow, policy, Git, and action-specific postconditions. Report both the
  adapter disposition and actual resulting state without treating runtime
  evidence as lifecycle truth.
- Implement `concoct exec inspect [<invocation-id>]` from retained bytes only,
  including safe handling of missing, partial, malformed, expired, and unknown
  records.
- Apply age, count, per-log, and total-size retention after attempts close;
  preserve active attempts and prune completed records deterministically.

### Phase 5 — Complete the CLI and compatibility surface

Status: `complete`

- Add `exec`, `exec --dry-run`, inspection, one-invocation overrides, useful
  output, help, error, and exit behavior while keeping manual prompt commands
  unchanged.
- Establish `.concoct/runtime/invocations/` as ignored local state in this
  repository and newly initialized projects without committing invocation
  contents.
- Update README, command/state/workflow documentation, configuration examples,
  and executable-owned guidance needed for agents to emit the structured
  result and invoke existing completion boundaries correctly.

### Phase 6 — Verify and prepare independent review

Status: `complete`

- Add focused package tests plus real-process and real-repository CLI tests for
  every acceptance and failure path, including a controllable fake adapter.
- Prove prompt-byte parity, profile precedence/source attribution, dry-run
  non-mutation, secret redaction, private file modes, bounded retention,
  cancellation/timeout races, duplicate/late results, observed-state
  precedence, and manual-workflow compatibility.
- Run the full repository checks, update durable implementation findings and
  phase statuses, and add a fresh Reviewer handoff.

## Review-01 remediation

Status: `complete`

- Permit archival orchestration from `implementation-complete` only when the
  composed policy explicitly makes independent review `not-required` or valid
  task evidence makes it `externally-satisfied`; retain Reviewer selection and
  reject direct archival authorization for ordinary unapproved evidence.
- Re-snapshot the complete authorized repository evidence immediately before
  either an adapter or direct integration starts, independently of the existing
  configuration digest check.
- Apply the resolved cancellation and timeout context to direct integration,
  including Git commands and push confirmation, while recording completed
  local mutations at recoverable transaction phases before returning an
  interruption.
- Add orchestration, CLI, execution, integration, and Git-context coverage for
  all three findings and rerun the complete verification matrix.

## Acceptance criteria

1. From every valid workflow state, `concoct exec --dry-run` either displays one
   fully resolved executable action or refuses with one evidence-backed reason;
   it starts no process and changes no workflow, Git, or runtime evidence.
2. Real `concoct exec` authorizes and launches at most one adapter process and
   returns after one action outcome. It never automatically starts the next
   role, retries an old envelope, or loops the lifecycle.
3. Action selection comes from one typed authority shared with current status,
   policy, `next`, and manual command eligibility. Invalid, blocked,
   informational, ambiguous, recovery-choice, and human-gated states cannot be
   coerced into execution by flags.
4. Ready-state execution performs only the Product Owner decision action. A
   completed result carries one bounded valid recommendation and does not
   automatically run planning or require a fabricated artifact mutation.
5. Every prompt-backed action receives bytes identical to the corresponding
   manual prompt rendered from the same authorized evidence and executable
   version. Manual stdout and create-only prompt export remain supported.
6. The adapter registry is executable-owned and independent from workflow
   selection. The built-in `codex` adapter runs non-interactively in the exact
   project root, consumes the prompt without exposing it in retained command
   arguments, and returns a schema-constrained correlated outcome.
7. Adapter and role settings resolve in the required precedence order. Dry-run
   shows the adapter, role, model, reasoning, timeout, safety posture, command,
   and value provenance; invalid configuration or unavailable capabilities fail
   before launch.
8. `exec` never supplies permission/sandbox bypasses, weakens agent rules,
   silently grants broader access, or bypasses existing review, archive,
   integration, push, or intervention gates. The exact launch posture is shown
   before the process starts.
9. A completed claim is accepted only when correlation, evidence freshness,
   action contract, and actual action-specific postconditions agree. Exit zero,
   final text, or a result file alone cannot advance or prove workflow state.
10. Cancellation, timeout, startup failure, nonzero exit, missing/malformed or
    duplicate result, stale/mismatched correlation, and postcondition mismatch
    all close inspectably, stop further execution, reconcile actual state, and
    report whether a fresh retry from current evidence is safe.
11. Each launched attempt has a unique private record beneath
    `.concoct/runtime/invocations/` containing the exact retained prompt,
    sanitized resolved metadata, bounded logs, structured result when present,
    and reconciliation. These records are Git-ignored and never establish
    workflow completion.
12. `concoct exec inspect` selects the most recent retained attempt, an explicit
    identifier selects only that attempt, and inspection uses retained material
    rather than current prompt/config regeneration.
13. Retention enforces configurable count, age, per-log, and total-size bounds,
    defaults to at most 20 completed attempts or 14 days, never prunes an active
    attempt, and handles partial records without blocking manual workflow.
14. Sanitized metadata and retained logs exclude environment contents,
    credentials, authorization material, prompt arguments, and tested common
    secret forms. Runtime files and directories use private permissions.
15. Existing prompt rendering, role completion, archival, Git integration,
    compatibility checks, initialization, and default-policy tests continue to
    pass unchanged except for deliberate `exec`, configuration, and ignored
    runtime additions.
16. User and normative documentation describes one-shot authorization,
    configuration and precedence, dry-run, inspection, retention, failures,
    manual fallback, and the distinction between adapter claims and observed
    workflow truth.

## Verification

- Run `gofmt` on all changed Go files.
- Add table-driven tests for recommendation-to-action resolution across every
  workflow state and policy disposition, including ready-state Product Owner
  judgment and every refusal class.
- Add adapter/configuration tests for registry completeness, strict parsing,
  project/user/role/default precedence, one-invocation overrides, source
  attribution, unsupported values, safe command quoting, and no secret or
  prompt leakage.
- Add process-supervision tests with a fake adapter for normal completion,
  startup failure, cancellation, timeout, graceful and forced termination,
  child-process cleanup, progress bounds, nonzero exit, missing/malformed,
  duplicate/late/stale/mismatched results, and durable-state/result
  disagreement.
- Add runtime tests for private modes, ignored paths, exact retained prompt,
  inspection from retained bytes, partial records, latest selection, and age,
  count, log, and total-size pruning.
- Add CLI and real-repository tests proving one-shot behavior, prompt parity,
  result reconciliation, state/next reporting, retry reauthorization, and
  preservation of all manual role and Git paths.
- Run `go test -count=1 ./...` and a focused race run for orchestration,
  execution, and runtime-record packages.
- Run `go vet ./...`, `go build ./cmd/concoct`, and
  `bash -n cmd/concoct/concoct.sh`; confirm the wrapper remains executable.
- Run `./cmd/concoct/concoct.sh init <temporary-project-path>` outside this
  repository. Confirm project-owned dotfiles/templates and planning directories
  are copied, built-in protocol/persona/prompt files are not installed, Git is
  initialized, bootstrap guidance exists, and runtime invocation paths are
  ignored but absent until first use.
- Run `git diff --check` and search active docs, templates, and defaults for
  stale claims that direct execution is unavailable or that an adapter claim,
  process status, or rendered prompt proves completion.

## Handoff expectations

The Developer should record the final recommendation/action model, adapter and
configuration contracts, Codex command mapping, process lifecycle and result
closure rules, runtime schema and retention choices, evidence-snapshot changes,
security decisions, files changed, and exact verification results in
`notes.md`. Before review, update phase statuses honestly and add a fresh
`## Handoff to reviewer` with focus on one-shot boundaries, authority and
postcondition precedence, prompt parity, cancellation races, secret handling,
configuration provenance, runtime retention, and manual-workflow regression.
