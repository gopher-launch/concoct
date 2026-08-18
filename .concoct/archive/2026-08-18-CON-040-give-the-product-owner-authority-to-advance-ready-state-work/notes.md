# Notes

## Planning summary

CON-040 is `ready` for implementation. The roadmap outcome includes the required
product decisions, semantic decision kinds, approval semantics, mutation limits,
scope boundaries, and observable acceptance criteria. No conflicting active task
existed before planning; the Concoct command created and checked out the recorded
task branch from the clean `main` base.

The implementation should treat Product Owner output as a durable semantic
decision, not as a command recommendation. One authority must connect supervised
capture, optional approval, exact application, planning continuation, status and
explanation output, run reuse, invalidation, and cost disposition.

## Confirmed findings

- Recorded Git context is `main`, task branch
  `concoct/con-040-give-the-product-owner-authority-to-advance-read`, base
  `2bbdd28cec958b613836ddb4161f12f92642a728`; repository inspection confirmed
  the task branch is checked out and clean before planning writes.
- `.concoct/current/` contained only `.gitkeep`, so no active task was
  overwritten.
- `internal/workflow.ResolveAction` unconditionally maps `ready` to
  `product-owner-next` / `concoct next`. Workflow reports have no Product Owner
  decision or ready-to-ready completion fields.
- `internal/workflow.InspectNextActionEvidence` already centralizes validated,
  deterministic roadmap/capability/dependency/prerequisite evidence, while
  `ValidatePlanItem` supplies the structural planning gate. These should remain
  core inputs rather than being duplicated.
- The orchestration registry defines `product-owner-next` as a decision that
  starts and completes in `ready`. Its permitted effect is only a roadmap-level
  product decision, and its bounded `Recommendation` contains kind, command,
  and reason.
- Execution and reconciliation special-case `product-owner-next` to accept a
  recommendation without an ordinary durable workflow postcondition.
- The run loop can transiently turn Product Owner output into planning context,
  and `internal/runstate` can bind a pending approval gate to current evidence,
  but neither makes the semantic decision reusable across command exit.
- Prompt rendering explicitly labels `next` as read-only and authorizes no
  updates. This is compatible with retaining an inspection mode, but not with
  using the same boundary for autonomous lifecycle work.
- Integration and initialization tests currently assert `Next: concoct next`;
  workflow, prompt golden, orchestration, execution, run-loop, CLI, and
  integration tests all encode parts of the behavior CON-040 changes.

## Prerequisite compatibility

- `CAP-001`: compatible. Its limitation assigns semantic judgment to roles and
  structural enforcement to the CLI; CON-040 preserves that division while
  making accepted judgment durable.
- `CAP-005`: compatible. Read-only `status` remains read-only; it will inspect a
  richer durable decision and report a shared next action without applying it.
- `CAP-006`: compatible. Prompt rendering remains guidance rather than proof of
  completion; a distinct supervised boundary supplies mutation authority.
- `CAP-009`: compatible but intentionally changed. Its read-only recommendation
  and no-selection limitation is the primary behavior CON-040 supersedes for
  autonomous execution while preserving manual inspection.
- `CAP-011`: compatible. Semantic capability wording remains an Archivist or
  Product Owner responsibility, and Product Owner changes are restricted to
  reconciliation of accepted archive/delivery evidence.
- `CAP-012`: compatible but intentionally extended. Its current protocol is not
  persisted history and forbids autonomous ready selection; the new bounded
  decision evidence adds the explicit accepted authority rather than trusting a
  structured claim alone.
- `CAP-013`: compatible. One-shot execution remains bounded and non-resumable;
  only the accepted Product Owner decision survives as reusable state.
- `CAP-014`: compatible. Runs remain finite and non-crash-resumable; they gain a
  deterministic continuation across an approval boundary, not general durable
  run recovery.
- `CAP-015`: compatible. Optional/version-sensitive usage does not affect
  decision correctness; cost disposition can follow durable proposal and
  application evidence without inventing unavailable usage.

No documented prerequisite limitation blocks the selected outcome.

## Planning decisions

- Declare an `update` impact across CAP-001, CAP-005, CAP-006, CAP-009, CAP-011,
  CAP-012, CAP-013, CAP-014, and CAP-015 because the observable contract spans
  durable workflow truth, status/prompt behavior, Product Owner recommendation,
  reconciliation, structured outcomes, execution, runs, and measurement.
- Keep exact storage format and path as a Developer design decision, but require
  an executable-owned, bounded, cross-command record with explicit authority,
  retention, invalidation, and Git-cleanliness semantics.
- Require record-aware roadmap/capability mutation validation. A raw unconstrained
  patch or command string is not sufficient evidence of Product Owner intent.
- Treat selection application and subsequent task planning as separate gates but
  a continuous derived lifecycle: an accepted selection must not route through a
  second `product-owner-next` action.
- Preserve manual read-only Product Owner inspection and align its semantic
  decision vocabulary with supervised execution.
- Use isolated test repositories for CON-039 candidate/planned scenarios so
  regression evidence does not depend on the live roadmap's future status.

## Risks and investigation focus

- Decide whether retained decisions are a workflow substate or typed auxiliary
  evidence. Avoid introducing a second state machine that can disagree with
  canonical artifacts.
- Define an atomic apply sequence that remains diagnosable after filesystem,
  process, or Git failure without granting CON-034-style general run recovery.
- Bound proposed Markdown changes while retaining enough exact content to apply
  an approved decision without model reinvocation.
- Include every semantically relevant input in drift detection without making
  harmless unrelated repository changes invalidate the decision unnecessarily.
- Make ready-to-ready completion evidence self-clearing after reconciliation or
  supersession so old success does not permanently dominate current status.
- Verify whether the structured protocol can evolve compatibly within `v1` or
  needs a new version; document adapter compatibility either way.

## Relevant history

- CON-028 deliberately delivered read-only `concoct next` and made it the sole
  ready recommendation. CON-040 changes that accepted boundary because live
  supervised execution exposed the resulting no-progress loop.
- CON-032 established that structured results are claims, not transition
  authority, and explicitly prohibited autonomous ready-state selection.
  CON-040 must add executable validation and durable authority rather than weaken
  that rule.
- CON-010 and CON-033 established one-shot supervision, bounded lifecycle runs,
  evidence-bound one-use approvals, and observed-state precedence. Reuse those
  mechanisms and retain their non-resumable limits.
- CON-036 hardened capability comparison around parsed record boundaries. Its
  approach is relevant to constrained Product Owner capability reconciliation.
- CON-037 and CON-038 established accepted/wasted invocation attribution,
  duplicate/budget protection, bounded private evidence, and prompt-context
  discipline. Product Owner reuse must integrate with those measurements.
- CON-039 owns general supervised Developer/Reviewer/Archivist candidate
  recovery. Do not absorb that scope while preserving invalid Product Owner
  candidates sufficiently for diagnosis and authorized repair.

## Development progress

### Decision and persistence contract

- Added `orchestration.ProductDecision` as a versioned, bounded semantic
  vocabulary: `select`, `reconcile-and-select`, `reconcile`,
  `human-decision-required`, and `no-action`. A command is intentionally not
  part of this contract.
- Added a private `.concoct/runtime/product-owner-decision.json` record through
  `runstate`. It binds the exact ready-state evidence and Product Owner action
  correlation to a decision and is create-only, mode-checked, size-bounded,
  and ignored by both the source and generated-project Git ignores.
- The record supports explicit proposed, approved, applied, and invalidated
  lifecycle labels, but no caller consumes them yet. That prevents an
  incomplete storage addition from accidentally authorizing roadmap mutation
  or planning.

### Verification

- Passed: `GOCACHE=/tmp/concoct-go-cache go test ./internal/orchestration ./internal/runstate`.
- Passed: `git diff --check`.

### Attempt: focused tests using the shared Go build cache

- Tried: `go test ./internal/orchestration ./internal/runstate`.
- Error/result: Go could not write `/home/cthain/.cache/go-build/...` because
  that cache is read-only in this execution environment.
- Why it failed: the repository sandbox permits writes only to the workspace
  and temporary directories.
- Next approach: use the equivalent writable temporary cache, which passed.

## Development continuation

### Implemented in this continuation

- Extended the Product Owner structured outcome with a versioned semantic
  `product_decision`; completed ready-state actions now reject command-only
  recommendations.
- Wired accepted `select` decisions into the bounded private decision record.
  The run coordinator verifies the record's evidence before reuse, creates the
  existing `next` gate from the stored decision, consumes it once, marks the
  decision applied, and invokes Task Planner work directly without a second
  Product Owner invocation.
- Added atomic lifecycle replacement and explicit stale-decision invalidation
  for the private decision record. The record remains Git-ignored runtime
  state; canonical task/roadmap changes still occur only through planning.
- Updated the command reference to distinguish semantic decisions from
  command-only recommendations.

### Verification

- Passed: `GOCACHE=/tmp/concoct-go-cache go test ./internal/orchestration ./internal/adapter ./internal/runstate ./internal/execution ./internal/runloop`.
- Passed after restoring prompt-golden parity: `GOCACHE=/tmp/concoct-go-cache go test ./...`.
- Passed: `git diff --check`.

### Remaining work and risk

- `reconcile` and `reconcile-and-select` are retained and reported but are not
  yet backed by the constrained canonical roadmap/capability mutation
  transaction required by Phase 2. They deliberately stop rather than infer or
  apply a Markdown mutation.
- `status`, `why`, and `exec inspect` do not yet render the retained decision;
  ready-state report enrichment and integration reconciliation remain pending.
- Manual prompt wording still describes the legacy recommendation vocabulary.
  It must be updated together with its deterministic golden fixtures once the
  full reconciliation application contract is implemented.

### Continued implementation: operator visibility and shared vocabulary

- `concoct status` now reads the bounded private Product Owner decision record
  without mutating it and reports its semantic kind, lifecycle status,
  selection (when present), and rationale. A malformed private record is
  reported as unavailable rather than silently treated as authoritative.
- `concoct exec inspect` now includes the retained Product Owner decision for
  an invocation inspection. This preserves the existing private-record
  boundary while making proposal/application correlation inspectable.
- The manual `concoct next` prompt, executable-owned Product Owner persona,
  and deterministic golden fixtures now use the same semantic vocabulary as
  supervised execution: `select`, `reconcile-and-select`, `reconcile`,
  `human-decision-required`, and `no-action`. Manual rendering remains
  explicitly read-only: it neither retains nor applies a decision.

### Verification

- Passed: `GOCACHE=/tmp/concoct-go-cache go test ./...`.
- Passed: `git diff --check`.
- Passed: `bash -n cmd/concoct/concoct.sh` and initialization into
  `/tmp/concoct-init.gZUc8M/generated-project`, including generated Git,
  bootstrap prompt, nested Codex adapter, and exclusion of executable-owned
  persona files. The wrapper's first invocation failed only because its Go
  build used the read-only default cache; rerunning the identical command with
  `GOCACHE=/tmp/concoct-go-cache` succeeded.

### Remaining implementation boundary

- `reconcile` and `reconcile-and-select` still require their approved bounded
  canonical roadmap/capability mutation transaction. Their current retention
  and reporting deliberately stop before mutation; this remains the blocker to
  completing Phases 2–4 and to claiming the task complete. No unsafe raw
  Markdown patch protocol was introduced merely to close that gap.

### Continued implementation: constrained reconciliation application

- Added a bounded `ProductMutation` contract to a retained semantic decision.
  A decision can carry at most eight exact replacements, each naming one
  roadmap or capability record and binding the complete prior record by its
  SHA-256 digest. This is record-scoped evidence, not a raw Markdown patch.
- `concoct run --approve next` now has a reconciliation path. It checks the
  existing evidence-bound gate, rejects changed/missing/ambiguous records,
  applies the retained record replacements, and either finishes a
  reconciliation-only decision or validates the selected item before entering
  Task Planner work. Reconciliation and selection use the same one-use
  approval; no extra Product Owner invocation is introduced.
- Status exposes the exact approval or planning continuation for retained
  reconciliation decisions. The command reference now documents the retained
  mutation boundary and drift behavior.

### Verification

- Passed: `GOCACHE=/tmp/concoct-go-cache go test ./...`.
- Passed: `GOCACHE=/tmp/concoct-go-cache go vet ./...` and
  `GOCACHE=/tmp/concoct-go-cache go test -race ./...`.
- Passed with writable copied module cache: native and Windows builds, shell
  syntax and wrapper mode checks, and fresh-project initialization in
  `/tmp/concoct-init.bdvsfn/generated-project` (Git/bootstrap/Codex adapter
  present; executable-owned persona directory absent).
- Passed: `git diff --check`.
- The first build attempt used the read-only default Go module cache. It
  compiled but could not write module-cache stat metadata; rerunning against a
  temporary writable copy passed.

### Remaining implementation boundary

- The new replacement transaction is record-scoped and drift-safe, but it is
  sequential across the two canonical files rather than a journaled,
  crash-recoverable multi-file transaction. It also needs end-to-end offline
  adapter fixtures for `reconcile-and-select`, record authorization rules
  beyond heading/digest binding, and ready-state/integration reconciliation
  reporting before this task can honestly be marked complete.

### Continued implementation: exact-once selection application

- Refined the retained `select` lifecycle so `approved` means that the
  evidence-bound `next` gate has been consumed, while `applied` is written only
  after a validated Task Planner transition reaches `planned`.
- A planner startup failure or rejected post-launch candidate now leaves the
  decision `approved`. A subsequent run invokes Task Planner directly for that
  exact selection; it neither calls Product Owner again nor requests a second
  selection approval.
- This avoids falsely consuming product authority when planning did not create
  durable workflow progress, while retaining the one-use selection approval.

### Verification

- Passed: `GOCACHE=/tmp/concoct-go-cache go test ./internal/runloop -run
  'TestPlanningStartupRollbackAndPostLaunchPreservation' -count=1`.
- Passed: `GOCACHE=/tmp/concoct-go-cache go test ./...`.
- Passed: `git diff --check`.

## Handoff

- Current state: planning artifacts authored for CON-040 on the recorded task
  branch.
- Work completed: roadmap readiness, prerequisite compatibility, relevant
  archive history, workflow/prompt/orchestration/execution/run/integration
  surfaces, scope, risks, acceptance criteria, and verification were inspected
  and translated into an implementation-ready plan.
- Work remaining: implement the five phases in `task-plan.md`; no source or test
  changes were made during planning.
- Expected next role: Developer.
- Recommended next command after planning completion validation: `concoct code`.

## Development completion

### Decision and reconciliation completion

- The Product Owner adapter schema now exposes the bounded `mutations` field
  required by semantic reconciliation decisions. Each replacement is limited to
  one roadmap or capability record, its SHA-256-bound prior bytes, and bounded
  replacement bytes; the schema cannot express a general patch.
- Reconciliation now validates every retained replacement in memory before it
  writes either canonical file. This prevents a later invalid replacement from
  leaving an earlier roadmap or capability replacement behind. Writes use the
  existing atomic per-file primitive and roll back earlier writes on a later
  write failure. A process crash between two filesystem renames remains the
  documented residual risk; the bounded decision and digest mismatch make it
  diagnosable rather than replayable.

### Attempt: combined final verification command

- Tried: one shell command that ran all checks and removed a temporary Windows
  executable on success.
- Error/result: the execution policy rejected the command before it ran because
  its `rm -f` cleanup form is not permitted.
- Why it failed: environment command safety policy, not repository behavior.
- Next approach: rerun the same checks without cleanup; all checks completed.

## Development continuation

### Implemented so far

- Implemented the retained Product Owner decision boundary across semantic
  output, private persistence, exact-once approval/application, selection
  continuation, bounded reconciliation, status and inspection visibility, and
  read-only manual prompt vocabulary.
- Added the final schema-to-application reconciliation path so supervised
  Product Owner output can carry constrained record replacements and rejected
  multi-record candidates do not mutate canonical workflow truth.

### Key decisions

- Product judgment remains a versioned semantic decision, never a command
  string. Runtime evidence is private and non-canonical; roadmap and capability
  files become authoritative only after validated application.
- The private decision record is a ready-state continuation rather than a
  competing workflow state. Approval is one-use, and a successful planning
  transition—not gate consumption alone—marks a selection applied.
- Reconciliation is record-scoped, SHA-256-bound, and bounded to eight
  replacements. Per-file atomic replacement plus compensating rollback protects
  ordinary write failures without claiming cross-file crash recovery.

### Files changed

- Workflow implementation and tests: `internal/orchestration/`,
  `internal/adapter/`, `internal/execution/`, `internal/runstate/`,
  `internal/runloop/`, and `internal/cli/`.
- Manual prompt/persona fixtures and Git ignores: `templates/`,
  `internal/prompt/testdata/`, and `.gitignore` counterparts.
- Operator documentation: `README.md`, `doc/command-reference.md`,
  `doc/state-machine.md`, and `doc/workflow.md`.
- Active implementation evidence: `.concoct/current/task-plan.md` and this
  notes file.

### Verification

- Passed: `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./...`.
- Passed: `GOCACHE=/tmp/concoct-go-cache go vet ./...`.
- Passed: `GOCACHE=/tmp/concoct-go-cache go test -race ./...`.
- Passed: native `go build ./cmd/concoct` and Windows
  `GOOS=windows GOARCH=amd64 go build -o /tmp/concoct-windows-CON-040.exe ./cmd/concoct`.
- Passed: `bash -n cmd/concoct/concoct.sh`, executable-mode check, fresh
  initialization in `/tmp/concoct-init.*` with Git, bootstrap prompt, current
  directory, and Codex adapter present and executable-owned persona files
  absent.
- Passed: `git diff --check`.
- The build emitted harmless Go module-cache stat warnings because the default
  module cache is read-only; compilation and both builds completed using the
  writable temporary Go build cache.

### Known risks

- Reconciliation uses per-file atomic writes with rollback, not a
  crash-recoverable multi-file journal. A process crash between canonical file
  replacements can require forward reconciliation; retained evidence will not
  replay automatically after drift.

### Skipped or unresolved work

- No live adapter invocation was run; offline fixtures cover the acceptance
  paths and the task explicitly makes live exercise optional.
- General retained-candidate recovery and crash-resumable coordination remain
  outside CON-040's approved non-goals.

### Capability impact

- Update pending independent review for CAP-001, CAP-005, CAP-006, CAP-009,
  CAP-011, CAP-012, CAP-013, CAP-014, and CAP-015. Do not update the capability
  ledger until accepted archival.

### Remaining implementation focus

- Implement record-level authorization for every permitted roadmap and
  capability status transition, including accepted archive/delivery provenance.
- Add the approved offline fixtures for reconcile-and-select, reconciliation
  only, human-decision-required, no-action, drift, supersession, duplicate
  invocation, and post-integration reconciliation before asking for review.

## Development continuation: reconciliation approval boundary

- Fixed the retained reconciliation continuation so its originating Product
  Owner invocation immediately creates the evidence-bound `next` approval
  gate. Previously, the decision was safely retained but the first run stopped
  without a gate; an operator had to issue a second `concoct run` just to
  discover the reusable proposal.
- Added an offline run-loop regression fixture. It verifies that one completed
  `reconcile` Product Owner invocation leaves a proposed private decision and
  a `next` gate targeting `reconciliation`, without another adapter call.

### Verification

- Passed: `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./internal/runloop ./internal/runstate ./internal/orchestration ./internal/execution`.
- Passed: `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./...`.
- Passed: `GOCACHE=/tmp/concoct-go-cache go vet ./...`.
- Passed: `git diff --check`.

### Remaining implementation boundary

- The task remains `implementation-in-progress`. Record-level authorization
  and provenance validation for canonical mutations, the remaining offline
  acceptance fixtures, and ready-to-ready integration reconciliation reporting
  are not yet complete; a Reviewer handoff would therefore be premature.

## Development continuation: canonical reconciliation authorization

- Added a shared workflow validator that evaluates the complete in-memory
  roadmap and capability candidates before `runstate` writes either canonical
  file. It rejects record addition/removal, duplicate changes, unsupported
  roadmap transitions, and capability edits without newly cited accepted
  delivery provenance.
- The only promotion without delivery evidence is `candidate` to `planned`.
  Delivered roadmap changes and capability reconciliation require an existing
  `.concoct/archive/.../summary.md` with `status: delivered` and
  `delivery: complete`; Product Owner output cannot manufacture that evidence.
- Direct `concoct exec` now refuses to invoke Product Owner again while a
  proposed or approved retained decision exists, directing the operator to the
  exact `run --approve next` or planner continuation instead.

### Verification

- Passed: `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./internal/execution ./internal/workflow ./internal/runstate ./internal/runloop`.
- Passed: `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./...`.
- Passed: `git diff --check`.

### Remaining implementation boundary

- The task remains `implementation-in-progress`: the full offline
  reconcile-and-select and post-integration reporting fixtures are still
  outstanding, as is the planned ready-to-ready completion presentation. The
  new validation closes the record-authorization gap but does not claim
  crash-resumable multi-file application.

## Development completion: ready-state reporting and generated bootstrap

- Clean ready reporting now explicitly states `Active task: inactive` and
  `Git task metadata: not applicable`. This prevents an absent task from being
  misread as blocked or as a non-Git active task after delivery cleanup.
- New projects now bootstrap through the supervised `concoct run` Product
  Owner decision boundary. `concoct next` remains explicitly described as
  read-only inspection, so generated guidance no longer routes selection
  through the legacy recommendation loop.

### Attempt: stale-contract search

- Tried: a shell search containing Markdown backticks in the search pattern.
- Result: the shell interpreted the backticked token as a command and printed
  a harmless command-not-found diagnostic before continuing.
- Cause: shell command substitution in an unquoted pattern, not repository
  behavior.
- Next approach: use fixed-string quoted searches; the relevant bootstrap
  wording was then found, corrected, and covered by initialization tests.

## Handoff to reviewer

### Implemented

- Product Owner semantic decision capture, private evidence-bound retention,
  one-use approval, direct selected-plan continuation, constrained
  reconciliation application, invalidation, operator inspection, and shared
  manual inspection vocabulary.
- Ready reporting now explicitly marks no active task and Git task metadata as
  inactive/not applicable. Generated bootstrap guidance starts supervised
  Product Owner work with `concoct run` and keeps `concoct next` read-only.

### Key decisions

- Product decisions remain versioned semantic records rather than command
  strings. Runtime evidence is private and non-canonical; canonical roadmap
  and capability edits require bounded, record-scoped, provenance-validated
  replacements.
- A retained Product Owner decision is a ready-state continuation, not an
  active task. A selected item is applied only after Task Planner makes a
  validated planned transition.

### Files changed

- Product Owner decision implementation and coverage: `internal/orchestration/`,
  `internal/execution/`, `internal/runstate/`, `internal/runloop/`,
  `internal/workflow/`, `internal/cli/`, and associated tests.
- Prompt/template and operator contract surfaces: `templates/`, `.gitignore`,
  `README.md`, `doc/command-reference.md`, `doc/state-machine.md`,
  `doc/workflow.md`, and `internal/project/`.
- Active task evidence: `.concoct/current/task-plan.md` and this notes file.

### Verification

- Passed: `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./...`.
- Passed: `GOCACHE=/tmp/concoct-go-cache go vet ./...` and
  `GOCACHE=/tmp/concoct-go-cache go test -race ./...`.
- Passed: native and Windows `go build`, `bash -n cmd/concoct/concoct.sh`,
  executable-mode check, `git diff --check`, and fresh external
  initialization with Git, bootstrap prompt, Codex adapter, planning
  directories, and no executable-owned persona directory.

### Known risks

- Reconciliation uses per-file atomic writes with compensating rollback; a
  process crash between canonical file renames is diagnosable by retained
  evidence and digest mismatch but is not crash-resumable.

### Skipped or unresolved work

- No live adapter invocation was run; offline fixtures cover the required
  outcomes. General retained-candidate recovery and crash-resumable
  coordination remain approved non-goals.

### Capability impact

- Pending independent review: CAP-001, CAP-005, CAP-006, CAP-009, CAP-011,
  CAP-012, CAP-013, CAP-014, and CAP-015. The capability ledger remains
  unchanged until accepted archival.

### Suggested review focus

- Verify evidence binding and one-use behavior across `select` and
  `reconcile-and-select`, record-scoped provenance validation, stale-decision
  refusal, and the distinction between read-only `next` and supervised `run`.
- Confirm generated bootstrap and clean-ready output accurately present the
  new Product Owner boundary without treating absent task metadata as active.

## Review remediation — review-01

### Finding disposition

- Finding 1 — fixed. `status` now permits a retained Product Owner decision to
  replace its next action only while the workflow remains `ready`. An applied
  selection therefore remains visible as private decision evidence but cannot
  mask the canonical continuation of the active planned task.

### Verification

- Passed focused regression coverage: `GOCACHE=/tmp/concoct-go-cache go test
  ./internal/cli ./internal/runloop`.
- Passed complete regression suite: `GOCACHE=/tmp/concoct-go-cache go test
  -count=1 ./...`.
- Passed static analysis: `GOCACHE=/tmp/concoct-go-cache go vet ./...`.
- Passed: `git diff --check`.

## Handoff to reviewer

### Implemented

- Corrected status rendering so an applied Product Owner `select` decision
  defers to an active planned task's canonical `concoct code` continuation.
- Added regression coverage for status after the selection-to-planned path.

### Key decisions

- Retained Product Owner records remain operator-visible private evidence, but
  workflow state is authoritative once planning has created an active task.

### Files changed

- `internal/cli/cli.go`
- `internal/cli/cli_test.go`
- `.concoct/current/notes.md`

### Verification

- `GOCACHE=/tmp/concoct-go-cache go test ./internal/cli ./internal/runloop`
- `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./...`
- `GOCACHE=/tmp/concoct-go-cache go vet ./...`
- `git diff --check`

### Known risks

- The existing documented reconciliation crash-window limitation remains
  unchanged; this remediation only changes read-only status rendering.

### Skipped or unresolved work

- None within review-01's requested scope.

### Capability impact

- Unchanged pending-review impact: CAP-001, CAP-005, CAP-006, CAP-009,
  CAP-011, CAP-012, CAP-013, CAP-014, and CAP-015.

### Suggested review focus

- Confirm status shows `concoct code`, not another planning command, after an
  applied selection has produced a planned task, while it still surfaces the
  retained Product Owner decision for inspection.

## Review remediation — review-02

### Finding disposition

- Finding 1 — fixed. A retained Product Owner decision may now override the
  rendered next action only while the detected workflow state is `ready`.
  Once a task is planned, the canonical active-task continuation remains
  authoritative for every semantic decision kind, while the retained record
  remains visible as private inspection evidence.

### Verification

- Passed focused regression coverage: `GOCACHE=/tmp/concoct-go-cache go test
  ./internal/cli -run
  'TestStatus(AppliedSelectionDefersToPlannedTaskContinuation|RetainedNonSelectionDecisionsDeferToPlannedTaskContinuation)$'
  -count=1`.
- Passed complete regression suite: `GOCACHE=/tmp/concoct-go-cache go test
  -count=1 ./...`.
- Passed static analysis: `GOCACHE=/tmp/concoct-go-cache go vet ./...`.
- Passed: `bash -n cmd/concoct/concoct.sh`, executable-mode check, and `git
  diff --check`.

## Handoff to reviewer

### Implemented

- Restricted retained Product Owner decision next-action rendering to the
  ready state, preserving planned-task continuation for all decision kinds.
- Added planned-task status regressions for `reconcile`,
  `reconcile-and-select`, `human-decision-required`, and `no-action`.

### Key decisions

- Private ready-state decision evidence remains visible in status after a task
  starts, but it cannot supersede canonical workflow progression.

### Files changed

- `internal/cli/cli.go`
- `internal/cli/cli_test.go`
- `.concoct/current/task-plan.md`
- `.concoct/current/notes.md`

### Verification

- `GOCACHE=/tmp/concoct-go-cache go test ./internal/cli -run
  'TestStatus(AppliedSelectionDefersToPlannedTaskContinuation|RetainedNonSelectionDecisionsDeferToPlannedTaskContinuation)$'
  -count=1`
- `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./...`
- `GOCACHE=/tmp/concoct-go-cache go vet ./...`
- `bash -n cmd/concoct/concoct.sh`, executable-mode check, and `git diff
  --check`

### Known risks

- The existing documented reconciliation crash-window limitation remains
  unchanged; this remediation changes only read-only status precedence.

### Skipped or unresolved work

- None within Review 02's requested scope.

### Capability impact

- Unchanged pending-review impact: CAP-001, CAP-005, CAP-006, CAP-009,
  CAP-011, CAP-012, CAP-013, CAP-014, and CAP-015.

### Suggested review focus

- Confirm every retained Product Owner decision remains visible on a planned
  task without replacing its canonical `concoct code` continuation.

## Archival handoff

- Archive candidate: `.concoct/archive/2026-08-18-CON-040-give-the-product-owner-authority-to-advance-ready-state-work/`
- Summary: approved CON-040 is ready for Git archival; Review 03 is the acceptance authority.
- Capability changes: update CAP-001, CAP-005, CAP-006, CAP-009, CAP-011, CAP-012, CAP-013, CAP-014, and CAP-015 with the delivered Product Owner decision, status precedence, reconciliation, orchestration, execution, run, and measurement behavior.
- Roadmap evidence: preserve CON-040 as `active` and add the archive directory reference; delivery remains pending integration.
- Checks: approved review records full tests, vet, shell syntax, executable-mode, and diff checks; archival validation must recheck exact copies, references, and scope.
- Risks: the documented reconciliation crash-window limitation remains; no unresolved task work remains.
- Pending delivery: Git archival and subsequent `concoct integrate` on the recorded task branch.
- Next action: run the supervised archival completion boundary, then integrate.
