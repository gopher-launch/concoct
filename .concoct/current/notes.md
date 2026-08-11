# Notes

## Planning summary

CON-032 is ready for implementation on the recorded Git task branch. It adds a
new executable-owned, agent-neutral action/outcome contract (proposed CAP-012)
without executing agents or changing the current manual workflow. The roadmap's
product decisions resolve the protocol transport, correlation, authority,
precedence, and diagnostic-retention boundaries.

## Confirmed findings

- Git identity is trunk `main`, task branch
  `concoct/con-032-define-structured-orchestration-actions-and-outc`, and base
  `95cc94cca997aa5c0c9f1710a1e7c1e3f63190ed`.
- Before planning, `.concoct/current/` contained only `.gitkeep`; no active task
  or review conflicted with CON-032.
- `internal/prompt` provides deterministic manual role prompts, and
  `internal/workflow` owns durable state/transition validation. Neither has a
  common authorized-action or structured-outcome model.
- `internal/cli` has no agent launcher or outcome transport; standard command
  output is human-readable and cannot currently establish a workflow result.
- Existing thin adapters are instruction-only. CAP-001, CAP-004, CAP-005,
  CAP-006, CAP-007, and CAP-009 limitations are compatible with this contract
  work because it does not automate role judgment, mutate unsupported projects,
  or add concurrent Git lifecycle behavior.

## Product decisions

- The authoritative protocol is versioned JSON with transport-independent
  semantics. Process adapters receive an invocation-specific temporary result
  path and write exactly one result atomically; stdout/stderr are diagnostic
  only.
- Correlation binds invocation, action, task, attempt, and role identities.
  Missing, malformed, duplicate, stale, or mismatched outcomes are protocol
  failures and cannot advance workflow state.
- The action registry is executable-owned and defines authority, gates,
  preconditions, effects, postconditions, outcomes, interventions, and
  completion validation. Workflow transitions remain owned by the workflow
  model.
- Observed artifact, workflow, and repository state plus executable authority
  outrank a structured claim; process exit status and human-readable output do
  not independently prove completion.
- Manual workflows remain supported without synthetic results. Automated
  orchestration requires the structured protocol; raw envelopes/logs stay
  ephemeral and only bounded sanitized facts may enter durable history.

## Risks

- Contract coverage must not accidentally turn rendering or invocation health
  into completion authority.
- Evidence snapshots and diagnostics require careful normalization to remain
  useful without retaining secrets or unbounded content.
- Future transport adapters must normalize to this protocol without creating a
  second semantic authority.

## Relevant history

- CON-006 established deterministic, non-mutating prompt rendering.
- CON-008 and CON-009 established explicit transition boundaries and
  evidence-backed ready-state recommendations.
- CON-015, CON-017, CON-018, CON-030, and CON-031 supply the Git, instruction,
  policy, embedded-resource, and compatibility boundaries this task must
  preserve.

## Handoff to developer

### Current state

Planning is complete; the task is ready for implementation on the recorded
branch.

### Completed

- Validated roadmap scope, capability-prerequisite limitations, repository
  state, prior delivery evidence, and exact Git identity.
- Created the active task plan and identified CAP-012 as the expected new
  accepted capability after archival.

### Remaining

- Implement the versioned action/outcome contract, registry, evidence binding,
  transport normalization boundary, tests, and documentation described in the
  task plan.

### Known risks

- Preserve manual workflow behavior and Product Owner selection authority.
- Do not add direct agent execution, runtime-specific parsing, lifecycle loops,
  or persistent raw execution logs.

### Commands run

- `concoct status`
- Repository, roadmap, capability, archive-summary, CLI, prompt, workflow,
  documentation, and Git-identity inspection.

### Suggested next step

Run `concoct code`, then implement Phase 1 while keeping subsequent phase
statuses and findings current.

## Implementation findings

- Added `internal/orchestration`, an executable-owned, transport-neutral v1
  JSON contract with action/outcome envelopes, correlation, bounded diagnostics,
  durable facts, a registry, and no agent-launching dependency.
- The registry explicitly covers Product Owner next-action and roadmap intake,
  task planning, development, independent review, archival, and integration.
  Each entry owns a role, gate, authorized starting states, permitted effects,
  supported outcome classes, and observed completion states.
- Authorization snapshots hashes of bounded workflow inputs and state rather
  than raw content. Ready state permits only a Product Owner decision action;
  it does not carry an autonomous roadmap selection.
- `ValidateOutcome` requires exact invocation/action/task/attempt/role
  correlation, supported protocol and outcome class, bounded fields, fresh
  evidence for non-completed results, and a registered observed postcondition
  for completion. Exit status and human-readable output are intentionally not
  inputs to validation.
- `WriteAtomicResult`/`ReadResult` establish the future process-adapter result
  boundary: one non-overwriting JSON result written by same-directory atomic
  rename. Raw result envelopes and logs are not persisted by this package.

## Verification results

- `gofmt -w internal/orchestration/*.go`
- `go test ./internal/orchestration`
- `go test ./...`
- Remaining final checks: `go vet ./...`, `go build ./cmd/concoct`, shell
  syntax, prompt rendering, and final diff inspection.

## Review 01 remediation

- Finding 1 — fixed. `WriteAtomicResult` now publishes its same-directory
  temporary file with `os.Link`, whose creation fails when the destination
  already exists. This is an atomic no-replace publication boundary, so two
  concurrent writers cannot overwrite the first delivered result. A concurrent
  delivery test proves exactly one writer succeeds and a subsequent writer is
  rejected.
- Finding 2 — fixed. Every `Spec` now explicitly defines executable authority,
  inspectable preconditions, a bounded non-completed intervention policy, and a
  completion validator. Authorization rejects incomplete contracts; outcome
  reconciliation invokes the completion validator and requires the registered
  intervention route for blocked, decision-required, and failure outcomes.
  Registry tests assert these definitions and exercise each validator; outcome
  tests prove a missing intervention is rejected.

## Final verification

- `gofmt -w internal/orchestration/*.go`
- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed.
- `git diff --check` — passed.
- Prompt-render commands correctly refused to run while this Git task branch
  was intentionally dirty; `concoct code --complete` is the required clean
  transition before the next review prompt can be rendered.

## Handoff to reviewer

### Implemented

- Versioned agent-neutral action/outcome contract and action registry in
  `internal/orchestration`.
- Snapshot-bound authorization, strict correlation and postcondition validation,
  bounded durable facts, and atomic single-result transport helpers.
- Normative structured-orchestration documentation in
  `doc/command-reference.md`.

### Verification

- Focused orchestration tests cover registry completeness, ready-state Product
  Owner selection boundary, mismatched/unchanged completion rejection,
  registered postcondition acceptance, and atomic duplicate-result rejection.
- `go test ./...` passed before the final non-test checks recorded below.

### Known risks

- This deliberately supplies contract and validation only; no CLI command
  authorizes an adapter run or invokes an agent yet. CON-010 must wire this
  package into a process or native adapter without treating its transport as
  workflow authority.

### Capability impact

- Proposed CAP-012 adds a validated, inspectable protocol boundary while
  preserving existing manual prompts and explicit completion commands.

### Suggested review focus

- Confirm every completion path remains driven by observed workflow state, that
  ready-state Product Owner choice is not automated, and that no raw adapter
  outputs have crossed into durable workflow history.

## Handoff to reviewer

### Implemented

- Remediated review-01's atomic-delivery race with atomic no-replace result
  publication; concurrent duplicate deliveries cannot replace an accepted
  result.
- Completed the explicit registry contract: authority, preconditions,
  permitted effects, postconditions via completion validators, supported
  outcomes, and intervention behavior are now defined and enforced.
- Extended focused tests and the normative command reference accordingly.

### Key decisions

- Hard-link publication is used instead of rename because rename can replace an
  existing destination; a same-filesystem link atomically fails if the result
  target already exists.
- Non-completed claims must use their action's registered bounded intervention
  route. Completion still depends on observed workflow state, never adapter
  output or process status.

### Files changed

- `internal/orchestration/orchestration.go`
- `internal/orchestration/orchestration_test.go`
- `doc/command-reference.md`

### Verification

- `go test -count=1 ./...`, `go vet ./...`, `go build ./cmd/concoct`,
  `bash -n cmd/concoct/concoct.sh`, and `git diff --check` all passed.

### Known risks

- This remains contract and validation work only: no CLI command authorizes an
  adapter run or invokes an agent. CON-010 must preserve this no-replace result
  boundary when it wires a process or native adapter.

### Skipped or unresolved work

- None within CON-032 scope. Prompt rendering after remediation is deferred
  until the required Git completion transition makes the task branch clean.

### Capability impact

- Proposed CAP-012 adds an inspectable, race-safe structured outcome boundary
  while preserving manual prompts, explicit completion commands, and role
  ownership.

### Suggested review focus

- Verify atomic no-replace behavior under concurrent result delivery and ensure
  every registry action's explicit authority, precondition, intervention, and
  completion-validator semantics remain tied to observed workflow evidence.
