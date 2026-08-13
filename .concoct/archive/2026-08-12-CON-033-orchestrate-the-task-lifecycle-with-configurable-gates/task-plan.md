---
id: CON-033
title: Orchestrate the task lifecycle with configurable gates
roadmap-id: CON-033
status: implementation-complete
remediates-review: review-02.md
created: 2026-08-11
updated: 2026-08-12
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-033-orchestrate-the-task-lifecycle-with-configurable
  base: b822466e9c969576cc5365460de6c84d2e8b4733
  status: active
capability-impact:
  type: add
  ids:
    - CAP-014
  rationale: Adds bounded repeated execution of authorized workflow actions with configurable approval gates, progress detection, reviewer isolation, and safe intervention reporting.
---

# Task Plan

## Goal

Implement `concoct run` as a bounded lifecycle coordinator that repeatedly
re-resolves and executes the next authorized action until the selected task is
locally integrated or the run reaches a required approval gate, blocker,
decision, unsafe state, failure, cancellation, or configured bound.

The command must remove mechanical prompt and command shuttling while retaining
the existing workflow, policy, role, completion, Git, and adapter authorities.

## Context

Concoct already has the required single-action foundations:

- `internal/workflow` derives canonical state and a typed next action from
  durable artifacts and effective policy.
- `internal/orchestration` authorizes one correlated structured action and
  validates one bounded outcome against fresh repository evidence.
- `internal/execution` prepares, launches, records, and reconciles one adapter
  or direct-integration invocation.
- `internal/prompt`, the role completion boundaries, and
  `internal/integration` preserve manual prompt parity and canonical state
  transitions.
- `internal/config` resolves strict project, user, role, and invocation
  settings for one-shot execution.

`concoct exec` deliberately stops after one action. CON-033 composes that
accepted boundary into one in-process run; it does not replace it, parse its
human-readable output, recursively invoke the CLI, or add another workflow
state machine. CON-034 retains ownership of crash-resumable runs, abandoned
process reconciliation, and durable multi-invocation recovery.

## Why this matters

The current lifecycle requires a user to alternate role prompt commands and
completion commands even when the next action is mechanically determined.
`concoct run` should let the user focus on the meaningful decisions—selecting
work, accepting the plan, and authorizing integration—while Concoct performs
the safe, observable transitions between those decisions.

## Current state

- `concoct exec` resolves and runs at most one action. Accepted non-completion
  outcomes are reconciled but surfaced as an execution error, and the public
  result does not expose the Product Owner recommendation needed by a loop.
- Ready state authorizes `product-owner-next`. A valid `plan` recommendation is
  checked for current eligibility but remains advisory and is not retained.
- Manual `concoct plan <roadmap-id>` validates eligibility, creates the
  deterministic Git task branch, renders the planner prompt with exact Git
  metadata, and leaves the Planner to author and commit active artifacts.
  This setup is currently embedded in CLI prompt dispatch rather than exposed
  as a reusable planning operation.
- Workflow action specs already cover task planning, development, review,
  archival, and integration, but ordinary ready-state resolution cannot select
  task planning without an approved Product Owner proposal.
- The one-shot executor already starts a fresh ephemeral adapter process for
  every role action and rechecks configuration and authorization immediately
  before launch.
- Project configuration has strict `exec` and Git sections but no finite run
  policy, approval-gate model, action/cycle limits, or invocation-only
  restrictions.
- Direct integration may follow configured auto-push behavior. An orchestrated
  run needs a local-only integration path because run approval must never imply
  remote-push authority.
- Private ignored invocation records exist under
  `.concoct/runtime/invocations/`; there is no pending-gate record or permanent
  lifecycle-run history.
- Two consecutive real prompt-backed actions demonstrated that the built-in
  Codex adapter's `workspace-write` sandbox can author role-owned worktree
  files but cannot create `.git/index.lock`. Developer and Reviewer work was
  complete at the artifact level, but their required Git-backed completion
  commits could not be created inside the adapter process.
- Post-run reconciliation currently conflates expected action effects with
  unrelated drift. It re-resolves configuration after a Developer may have
  legitimately changed the project configuration, and it requires every
  non-completion outcome to retain the exact pre-launch repository digest.
  The failed Reviewer completion therefore produced valid
  `review-changes-requested` evidence but was rejected as stale.

## Target state

- `concoct run` can start from ready state or any ordinary actionable task
  state. It re-detects workflow and repository state, resolves effective run
  policy, authorizes one fresh action, executes it through the existing
  adapter/direct-operation boundary, validates observed progress, and repeats.
- In ready state, the run performs only the Product Owner decision action. A
  valid plan recommendation is retained as a bounded proposal tied to the exact
  action and evidence fingerprint, then the run stops at the invariant `next`
  approval gate.
- `concoct run --approve next` accepts only that still-current proposal,
  establishes the selected planning context and deterministic task branch, and
  continues through the Task Planner action. Successful planning stops at the
  effective plan-acceptance gate in the same invocation.
- `concoct run --approve plan` accepts only the exact current task and plan
  content, consumes the approval when the protected action begins, and then
  continues through development, policy-required independent review,
  productive remediation cycles, and archival until another gate or
  intervention applies.
- `concoct run --approve integration` validates the current archived task,
  reviewed revision, branch, target trunk, and repository evidence before
  performing local integration. The run never pushes a remote; existing manual
  integration behavior remains available separately.
- Required stops, project-selected gates, and invocation-added restrictions
  compose into one finite effective run policy. Invocation flags may add gates
  and lower bounds, never remove a project gate or weaken an invariant stop.
- Every stop prints a bounded run summary containing attempted actions,
  accepted outcomes, progress/cycle counts, current workflow and repository
  state, gate or intervention reason, next recommendation, and an exact safe
  continuation command when one exists.
- Private local pending-gate evidence survives the separate approval command
  without becoming workflow truth or a general execution-history product.
  It is invalidated by relevant evidence drift and consumed atomically when its
  protected action begins.
- For supervised prompt-backed actions, the sandboxed adapter authors and
  verifies only the role-owned candidate. The outer Concoct executable then
  validates the candidate through the canonical role completion authority and
  creates the Git transition commit before reporting the handoff. `exec` stops
  after that committed action; `run` may re-resolve and continue. Manual role
  commands retain their explicit completion workflow.
- Supervised Task Planner, Developer, Reviewer, and Archivist actions can
  complete autonomously without granting the adapter Git-metadata access or a
  sandbox bypass. Integration remains a direct executable operation under its
  existing transaction and approval boundaries.

## Design constraints

- Preserve `workflow.Detect`, `workflow.ResolveAction`, registered action
  specs, role completion commands, archive completion, and integration as the
  canonical authorities. The run coordinator may compose them but must not
  infer transitions from command strings, adapter logs, process status, or a
  claimed outcome alone.
- Re-resolve workflow state, action kind, role, adapter, execution profile,
  policy, repository identity, and authorization before every action. Do not
  reuse an earlier action envelope when an action kind recurs.
- Factor reusable single-action execution so the coordinator receives an
  accepted structured outcome, bounded recommendation, and observed state
  without scraping private record files or weakening the existing `exec` CLI's
  one-shot error and reporting contract.
- Factor the manual planning setup into one shared operation used by both
  `plan` and `run`: eligibility validation, clean Git checks, deterministic
  branch creation, exact trunk/branch/base context, prompt rendering, and safe
  rollback before agent work starts. An approved selection is the only
  additional authority that may choose the task-planning action.
- Keep orchestration policy a finite typed configuration concern, separate from
  workflow-state definitions and adapter profiles. Built-in defaults require
  plan acceptance and local-integration approval; ready-state proposal
  acceptance and all blocker/decision/safety stops are invariant. Project
  configuration may select supported optional gates and lower the hard action
  and cycle maxima; invocation flags may only be more restrictive.
- Define gates against a specific forthcoming action or transition. A pending
  gate record must include the gate, action/task/attempt identity, relevant
  evidence fingerprint, and the minimum bounded proposal or target data needed
  to validate approval. It must be private, no-replace/atomic, size-bounded,
  and Git-ignored.
- Treat approval as one-use authority. Reject missing, mismatched, stale,
  ambiguous, already-consumed, wrong-gate, or evidence-invalidated approval
  before branch, workflow, Git, adapter, or integration mutation.
- A plan gate binds to the exact selected task and planned content before the
  first protected development action. A material external edit while awaiting
  acceptance restores the gate. Normal Developer-owned progress updates after
  the accepted action begins do not create a second competing approval model.
- An integration gate binds to the approved archived task revision, archival
  commit, task branch, integration trunk, and relevant Git state. Run-specific
  integration must disable push even when `git.auto-push` is true; no run flag
  may imply remote authority.
- Required independent review must use a fresh reviewer adapter invocation with
  the Reviewer prompt and completion boundary, sharing no conversational
  session with the Developer. Policy-valid non-required or externally
  satisfied review must retain its existing archival route.
- Track progress with the typed action plus a relevant-state fingerprint that
  includes workflow state, material artifact/Git identity, review evidence,
  and pending intervention. Repeated action kinds are allowed only when the
  fingerprint advances.
- Enforce hard per-invocation maxima of 20 actions and three completed
  code/review cycles. A review outcome of `changes-requested` is productive
  lifecycle progress, not an execution retry. The initial release performs no
  automatic retry of recoverable execution failures.
- Cancellation and any accepted non-completion outcome stop promptly after
  reconciliation and preserve valid canonical state. The coordinator must not
  roll back agent-authored work or fabricate completion.
- Preserve exact pre-launch configuration and evidence checks immediately
  before starting an adapter. After launch, distinguish role-authorized action
  effects from unrelated or forbidden drift rather than requiring the entire
  pre-launch digest to remain unchanged.
- A supervised adapter receives the exact manual role prompt as an unchanged
  core plus a bounded executable-owned supervision appendix. The appendix must
  state that the adapter authors and verifies the candidate but does not write
  Git metadata or invoke the final commit boundary. It must not weaken the
  selected persona, artifact ownership, review independence, or completion
  requirements.
- The outer executable may finalize only a schema-valid, correlated completed
  candidate whose observed changes satisfy the action's role-owned effect and
  postcondition validators. It must invoke reusable planning, Developer,
  Reviewer, or archival completion logic directly rather than recursively
  invoking the CLI or parsing adapter prose.
- Blocked, decision-required, failed, cancelled, or malformed outcomes must not
  be silently upgraded or committed. Preserve any valid partial work, identify
  whether replay is unsafe, reject forbidden effects, and return an exact
  recovery action without fabricating completion.
- Keep manual `next`, `plan`, prompt, completion, `exec`, archive, and integrate
  commands supported and behaviorally compatible.
- Preserve project-contract compatibility checks before every new mutation or
  output boundary.

## Non-goals

- No crash-resumable coordinator, stale-process lease, replay after restart,
  uncertain-completion recovery, run abandonment command, or permanent run
  history; those remain CON-034.
- No concurrent, stacked, or worktree-based tasks; CON-024 retains that scope.
- No autonomous roadmap ranking, summary parsing, or acceptance of a Product
  Owner recommendation without the explicit `next` gate.
- No arbitrary workflow graph, plugin-defined action type, unrestricted policy
  language, general approve-all/force/no-gates option, or permission bypass.
- No automatic retry of adapter or direct-operation failures in the initial
  release.
- No remote push from `concoct run`, even when existing manual integration is
  configured to push automatically.
- No replacement of one-shot `exec`, manual prompts, role completion
  boundaries, immutable reviews, or archival evidence.
- No broad task-profile, Git-strategy, multiple-origin, diagnostics, or history
  work from CON-011, CON-012, CON-019, CON-020, or CON-023.

## Working assumptions and decisions

- `CAP-014` is the next available capability identifier and will describe the
  new observable repeated-execution behavior. CAP-013 remains the one-shot
  primitive and CAP-012 remains the structured action/outcome boundary.
- Run policy belongs in the existing strict configuration layer under a
  dedicated `run` section, not in adapter role profiles and not as a second
  lifecycle definition. Absence resolves to the default plan and integration
  gates and hard 20-action/three-cycle bounds.
- The public surface should support `concoct run`,
  `concoct run --approve <current-gate>`, invocation-added gate restrictions,
  and lower action/cycle bounds. Exact flag spelling beyond the roadmap's
  required `--approve next|plan|integration` forms may follow existing CLI
  conventions, but supported gate names and precedence must remain finite and
  documented.
- The invariant `next` gate is the only route from a Product Owner proposal to
  selected planning. A manually selected task created with `concoct plan`
  enters `run` at the plan gate when that gate is effective.
- Private pending-gate state should live below `.concoct/runtime/` with the
  same restrictive permissions and non-authoritative status as invocation
  records. Only the currently pending gate needs cross-invocation retention;
  completed action history remains in existing invocation records and the
  current command's bounded summary.
- A code/review cycle is counted when an independently executed Reviewer action
  completes with an observed review outcome. Approval on the third review is
  permitted; a fourth Reviewer action in the same run is not.
- The coordinator ends successfully after validated local integration even
  though canonical cleanup normally returns workflow detection to `ready`.
  Its final summary must identify the completed integration action and the
  resulting ready state rather than confusing ready with an unstarted task.
- Before an adapter launches, planning setup may roll back a newly created
  unchanged task branch on failure. Once agent-authored or canonical evidence
  may exist, preserve it and stop with an intervention instead of deleting it.
- The human decision on 2026-08-12 authorizes executable-owned finalization for
  supervised actions. The adapter remains responsible for semantic role work;
  Concoct assumes only deterministic validation and commit authority after a
  completed candidate. This is a technical refinement of the already required
  autonomous Git-backed lifecycle, not authorization for broader agent or
  remote access.

## Risks and open questions

- Planning is the only action whose current manual command creates Git context
  before role execution. Reuse and rollback must avoid branch collisions,
  selection bypass, or loss of partial Planner work.
- The current executor treats accepted blockers and decisions as errors and
  omits recommendations from durable facts. Refactoring must preserve existing
  one-shot behavior while giving the loop a typed, non-stringly result.
- A pending approval that is checked too early could authorize changed policy,
  plan, review, or Git evidence. Validation and consumption must occur directly
  before the protected operation, with no side effect on mismatch.
- Over-broad fingerprints can report false progress from irrelevant runtime
  files; narrow fingerprints can miss meaningful plan, review, policy, config,
  or Git drift. Tests need both positive productive cycles and negative
  no-progress cases.
- A same-process coordinator can accidentally retain stale role/configuration
  decisions or blur reviewer independence. Every iteration must prepare a new
  action and fresh ephemeral adapter invocation.
- Local integration currently owns optional push. Separating the run's
  local-only authority must not regress manual integration recovery,
  confirmation, or auto-push behavior.
- The line between the minimal pending-gate record required here and CON-034's
  durable run recovery must remain explicit. Pending approval supports the
  documented continuation commands; it does not claim to reconcile an action
  interrupted in flight.
- Supervised finalization must not turn a structured claim into authority. A
  candidate may be committed only after role-specific scope, artifact,
  workflow, Git, and postcondition validation succeeds in the outer process.
- Post-launch mutation attribution cannot rely on a whole-repository digest
  alone. The implementation must reject forbidden paths and contradictory
  states while allowing the exact effects the authorized role was launched to
  produce on the repository's exclusive task branch.

No unresolved product decision blocks implementation. The roadmap defines the
mandatory gates, approval scope, bounds, initial no-retry posture, reviewer
independence, progress rules, stop reporting, and no-push boundary.

## Implementation phases

### Phase 1 — Expose reusable action and planning primitives

Status: `complete`

- Refactor the one-shot execution path to return a typed reconciled result that
  includes accepted outcome class, bounded Product Owner recommendation,
  durable facts, observed state, and progress evidence. Retain existing
  `concoct exec` output, one-action limit, and refusal semantics.
- Extract the current plan eligibility, task-branch setup, exact Git prompt
  context, and pre-launch rollback into a reusable operation shared by manual
  `plan` and the approved-selection run path.
- Extend authorization only as needed to carry a validated selected roadmap ID
  into `task-planning`; do not make ready-state task selection generally
  executable or bypass Product Owner approval.

### Phase 2 — Define effective run policy and pending gates

Status: `complete`

- Add strict finite run configuration with default project gates, supported
  optional action gates, hard action/cycle maxima, and invocation-only
  restrictive overrides. Reject unknown values, duplicate/conflicting gates,
  raised bounds, and permissive overrides before creating runtime evidence.
- Add private, bounded, atomic pending-gate records for Product Owner
  proposals, plan acceptance, configured action gates, and integration
  approval. Bind records to exact action/task/attempt and relevant evidence,
  validate them immediately before use, and consume them once.
- Extend source and initialized-project ignore rules for the new local runtime
  path without installing mutable run history or exposing private records as
  workflow artifacts.

### Phase 3 — Implement the bounded lifecycle coordinator

Status: `complete`

- Add an orchestration-loop package that repeatedly detects state, resolves
  policy and action, checks gates, prepares one fresh action, invokes the
  existing executor, validates postconditions, records in-memory progress, and
  decides whether to continue or stop.
- Implement ready-state proposal, approved planning, plan acceptance,
  development, independent review, changes-requested remediation, archival,
  and archived-task integration paths using canonical role transitions.
- Detect repeated action/fingerprint pairs, claimed completion without
  validated advancement, unexpected state, exhausted action/cycle bounds, and
  every accepted intervention class deterministically.
- Ensure Reviewer actions use independent ephemeral invocations and policy
  routes remain correct when review is non-required or externally satisfied.

### Phase 4 — Separate local integration and complete stop behavior

Status: `complete`

- Introduce an explicit local-only integration option below the CLI so the run
  path can retain all branch, recovery, conflict, and cleanup validation while
  suppressing push. Preserve existing manual and one-shot integration push
  behavior.
- Make integration approval validate the reviewed archive revision, archival
  commit, task branch, trunk, and repository evidence at the final launch
  boundary.
- Define bounded summaries and exact continuations for gates, blockers,
  decisions, recoverable/terminal failures, cancellation, invalid or recovery
  states, exhausted bounds, no-progress detection, and successful local
  integration.

### Phase 5 — Add CLI and documentation surfaces

Status: `complete`

- Add `run`, approval, additional-gate, and lower-bound argument parsing with
  strict validation and compatibility preflight before any record, branch,
  prompt, or Git mutation.
- Document default and effective gates, approval binding and invalidation,
  progress/cycle accounting, reviewer isolation, no-retry behavior, private
  pending evidence, all stop reasons, exact continuation commands, manual
  fallbacks, and the local-integration/no-push boundary.
- Update README, command reference, state-machine/workflow documentation,
  built-in workflow guidance where required, configuration examples, and
  source/template ignore counterparts without implying CON-034 recovery.

### Phase 6 — Verify and prepare independent review

Status: `complete`

- Add focused policy, pending-gate, coordinator, execution, planning, CLI, and
  integration tests plus real-repository/controllable-adapter lifecycle tests.
- Exercise happy paths from ready and preplanned state; plan and integration
  gates; more-restrictive invocation policy; required, non-required, and
  externally satisfied review; changes-requested cycles; every stop class;
  drift and stale approval; cancellation; no progress; bounds; branch safety;
  manual compatibility; and remote-push suppression.
- Run the full repository verification and generated-project initialization
  checks, update phase statuses and durable findings, and add a fresh Reviewer
  handoff.

The initial implementation, verification, and handoff were completed before
independent review. Review 01 reopened acceptance with three major findings,
and the observed supervised-completion failures add the authorized technical
refinement below. Phase 7 now owns the remaining work.

### Phase 7 — Remediate Review 01 and supervise role finalization

Status: `complete`

- Resolve all three findings in `review-01.md`: make optional action approvals
  apply to one occurrence only, reject symlinked/non-directory pending-state
  parents before mutation, and implement the complete coordinator lifecycle and
  failure matrix required by this plan.
- Refactor prompt-backed execution so the sandboxed adapter authors and
  verifies a completed role candidate while the outer executable invokes the
  canonical planning, Developer, Reviewer, or archival completion primitive
  and creates the Git transition commit before handoff. Keep `exec` one-shot,
  allow `run` to continue after committed progress, preserve fresh Reviewer
  processes, and do not grant Git or sandbox bypass authority to adapters.
- Separate immutable pre-launch authorization checks from post-launch effect
  validation. Accept only role-authorized completed effects; stop safely on
  partial non-completion, forbidden drift, commit failure, cancellation, or
  contradictory state, with truthful retry safety and exact recovery.
- Update manual and supervised prompt contracts, command/workflow/state
  documentation, structured execution records, and tests to describe which
  process owns semantic work, validation, commits, and handoffs.

### Phase 8 — Complete Review 02 boundary verification

Status: `complete`

- Add coordinator-level coverage for policy-valid non-required and externally
  satisfied review routing, repeated action/evidence no-progress termination,
  planning startup rollback versus post-launch preservation, and integration
  conflict recovery summaries with exact continuations.
- Prove at byte level that every supervised Planner, Developer, Reviewer, and
  Archivist prompt is its independently rendered manual core followed only by
  the fixed executable-owned supervision appendix.
- Correct any production behavior exposed by the missing matrix, rerun the
  complete verification set, and record the Review 02 disposition and fresh
  Reviewer handoff.

### Phase 9 — Remediate the archival completion boundary

Status: `complete`

- Preserve the archive candidate's `task-plan.md` as byte-identical accepted
  evidence instead of requiring the Archivist to pre-apply archival Git
  metadata.
- Make `concoct archive --complete` derive, atomically apply, commit, and
  validate the current-only `git.status: archived` and
  `git.archive-commit: self` transition against immutable pre-archive evidence.
- Preserve forbidden-path and exact-transition checks while adding rollback,
  interrupted-transition recovery, clean committed retry, supervised lifecycle,
  and integration regression coverage.

## Acceptance criteria

1. `concoct run` executes consecutive authorized actions through the existing
   structured protocol and canonical completion boundaries without recursive
   CLI invocation, prompt-file shuttling, or a second workflow state machine.
2. In ready state, one fresh Product Owner action yields either a validated
   bounded proposal or an explicit intervention. A plan proposal is persisted
   with its evidence fingerprint and always stops at `next`; no roadmap item is
   selected or planned before `concoct run --approve next`.
3. `--approve next` accepts only the exact current eligible proposal. Evidence
   drift, a changed/ineligible item, missing or malformed pending state, wrong
   gate, or reused approval refuses before branch or artifact mutation and
   requires a fresh Product Owner action.
4. Valid next approval establishes the deterministic planning context and
   invokes the Task Planner with the same effective prompt and exact Git
   metadata as manual planning. Successful planning reaches `planned` and
   stops at the effective plan gate in the same invocation.
5. A manually planned task also stops at the effective plan gate. Plan
   approval is bound to the exact task and plan content, consumed only when its
   protected action begins, and invalidated by material drift before launch.
6. With gates satisfied, a happy-path task proceeds through development,
   required independent review, archival, and approved local integration in
   one run, ending with validated integration completion and canonical ready
   state.
7. Default run policy requires plan and integration approval; ready-state
   proposal approval and blocker/decision/safety stops are invariant. Project
   configuration can select only supported optional gates and lower bounds;
   invocation options can add gates or lower bounds but cannot remove or raise
   effective project requirements.
8. Approval is scoped to only the currently pending gate, task, action,
   attempt, and relevant evidence. No force, no-gates, approve-all, or
   pre-authorization of a later gate is accepted.
9. Required review uses a fresh Reviewer adapter process and Reviewer prompt,
   never a continued Developer session. Non-required and valid externally
   satisfied review follow their accepted policy routes without fabricating a
   review artifact.
10. A `changes-requested` review followed by scoped remediation is treated as
    productive progress. Repeated Developer and Reviewer action kinds continue
    only when material workflow/review/Git fingerprints advance, and all prior
    reviews remain immutable.
11. Each run attempts at most 20 actions and at most three completed
    code/review cycles, or stricter effective limits. The boundary action is
    never started after its limit is reached, and the summary reports consumed
    and remaining bounds.
12. The initial release never automatically retries a recoverable execution
    failure. It stops after reconciliation with a safe fresh-run/manual
    continuation when current evidence permits one.
13. Repeating the same action with the same relevant-state fingerprint,
    accepted completion without observed progress, exhausted bounds, or an
    unexpected state stops deterministically rather than looping.
14. Blocked, decision-required, invalid, integration-recovery, ambiguous,
    terminal-failure, cancellation, and unsafe states stop even under the most
    permissive optional-gate configuration and cannot be coerced by flags.
15. Every stop summary identifies attempted actions and accepted outcomes,
    workflow/repository state, pending gate or intervention, bound usage,
    progress evidence, next recommendation, and the exact safe continuation
    command when one is available.
16. `--approve integration` is rejected on reviewed-revision, archive,
    branch, trunk, policy, configuration, or repository drift. A valid approval
    performs local integration with existing safety/recovery semantics.
17. `concoct run` performs no remote push, including when
    `git.auto-push: true`; plan or integration approval never implies remote
    authority. Existing manual integration and `exec` behavior remain
    compatible.
18. Pending-gate files are private, bounded, atomic, Git-ignored, non-
    authoritative, and safely replaced or invalidated without changing
    canonical workflow state. Ordinary one-shot invocation retention remains
    intact.
19. `concoct exec`, all manual prompt/completion commands, workflow status,
    review immutability, archive validation, integration recovery, project
    compatibility checks, and generated-project initialization retain their
    accepted behavior.
20. A successful supervised Task Planner, Developer, Reviewer, or Archivist
    action completes its role-owned work in the sandbox and is validated and
    committed by the outer Concoct executable before `exec` returns or `run`
    resolves the next action. The adapter never requires `.git` write access.
21. Executable-owned finalization calls the same reusable role completion
    authority as the manual workflow. It validates exact role scope, required
    artifacts, workflow and Git context, fresh handoff or review evidence, and
    observed postconditions before committing; adapter text, exit zero, and a
    structured claim alone remain insufficient.
22. The supervised prompt contains the byte-identical manual role prompt as
    its core plus a fixed, bounded, executable-owned appendix that transfers
    only final validation and commit ownership to Concoct. Manual prompts and
    explicit manual completion commands remain supported and unchanged.
23. Pre-launch configuration and evidence drift still prevents launch. After
    launch, authorized role effects—including a task-required configuration
    edit—do not become false stale outcomes. Forbidden or contradictory
    changes are rejected, and non-completion after partial progress is
    preserved and reported as unsafe to replay without being mistaken for a
    clean retry.
24. A real-Git supervised lifecycle can plan, develop, independently review,
    remediate, archive, and locally integrate without manual commits when all
    gates are approved and no genuine intervention occurs. `exec` remains
    bounded to one committed action, and `run` remains bounded by its effective
    action and cycle limits.

## Verification

- Run `gofmt` on all changed Go files.
- Add focused tests for typed run-policy parsing and precedence, finite gate
  validation, lower-only invocation bounds, default policy, and strict unknown
  field/value failures.
- Add pending-record tests for permissions, atomic creation/consumption,
  bounded content, every approval binding, stale evidence, duplicate/replayed
  approval, malformed/partial records, and Git-ignore behavior.
- Add coordinator tests with a controllable fake adapter for every actionable
  start state, Product Owner proposal classes, manual and approved planning,
  happy-path continuation, review dispositions, productive remediation, fresh
  role/profile resolution, accepted non-completion outcomes, process failures,
  cancellation, no progress, and action/cycle exhaustion.
- Use real temporary Git repositories to verify exact planning branch/base
  context, pre-launch rollback, preservation after possible agent mutation,
  archival-to-integration flow, approval drift rejection, integration conflict
  recovery stop, cleanup, and absence of remote push with both auto-push
  settings.
- Prove each supervised role action receives the exact manual prompt bytes as
  an unchanged core plus only the fixed supervision appendix, and that every
  Reviewer execution uses a distinct ephemeral invocation from the preceding
  Developer.
- Add real-Git execution tests in which the adapter can write role artifacts
  but cannot write `.git`; prove the outer executable validates and commits
  successful planning, development, review, and archival candidates and that
  `exec` stops only after its one committed action while `run` continues.
- Cover a Developer action that legitimately changes project configuration,
  expected post-launch evidence changes, forbidden-path drift, partial
  non-completion, outer commit failure, clean/idempotent completion retry, and
  truthful `retry_safe` reporting without granting an adapter sandbox bypass.
- Run focused race tests for pending-gate no-replace/consume behavior and any
  shared coordinator state.
- Run `go test -count=1 ./...`, relevant `go test -race` packages,
  `go vet ./...`, and `go build ./cmd/concoct`.
- Run `bash -n cmd/concoct/concoct.sh` and confirm the wrapper remains
  executable.
- Initialize a project under a temporary parent outside this repository;
  confirm dotfiles, nested project-owned templates, planning directories,
  contract/bootstrap evidence, Git staging, no generated commit, runtime
  ignore coverage, and absence of installed built-in protocol/persona/prompt
  files.
- Run `git diff --check`, compare source/template counterparts, search for
  stale one-shot-only or auto-push claims, and inspect the complete diff for
  leaked private run records or broadened authority.

## Handoff expectations

The Developer should record the exact run-policy schema and precedence,
pending-gate format and lifetime, selected planning reuse boundary, progress
fingerprint, cycle accounting, structured execution-result refactor,
local-only integration mechanism, stop/continuation matrix, and verification
results in `notes.md`. Before review, all phase statuses must be honest and the
notes must contain a fresh `## Handoff to reviewer` with suggested focus on
approval freshness, Product Owner selection authority, loop termination,
reviewer isolation, planning/Git rollback, and remote-push suppression.
