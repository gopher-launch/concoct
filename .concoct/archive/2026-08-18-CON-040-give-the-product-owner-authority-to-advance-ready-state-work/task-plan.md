---
id: CON-040
title: Give the Product Owner authority to advance ready-state work
roadmap-id: CON-040
status: implementation-complete
remediates-review: review-02.md
created: 2026-08-18
updated: 2026-08-18
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-040-give-the-product-owner-authority-to-advance-read
  base: 2bbdd28cec958b613836ddb4161f12f92642a728
  status: active
capability-impact:
  type: update
  ids:
    - CAP-001
    - CAP-005
    - CAP-006
    - CAP-009
    - CAP-011
    - CAP-012
    - CAP-013
    - CAP-014
    - CAP-015
  rationale: Extends the durable workflow, ready-state status and prompts, Product Owner recommendation, archive reconciliation, structured action protocol, supervised execution, bounded runs, and cost attribution with an evidence-bound Product Owner decision that can be approved and applied exactly once.
---

# Task Plan

## Goal

Replace the read-only `product-owner-next` recommendation loop with a validated,
durable Product Owner decision boundary. From `ready`, a Product Owner must be
able to reconcile authoritative roadmap and accepted delivery evidence, select
exactly one next item when appropriate, retain the proposed decision for policy
approval, and advance the accepted selection directly to planning without a
second Product Owner invocation.

Make `status`, `next`, `why` or equivalent explanation output, `exec`, and `run`
derive their operator-visible decision and exact next action from the same
evidence. Preserve a separate read-only rendering path for manual inspection.

## Context

The current workflow resolves every clean `ready` state to
`product-owner-next`. Its orchestration contract permits only a roadmap-level
recommendation, considers `ready` both the starting and completed state, and
validates a structured command recommendation without applying it. Execution
special-cases that action so successful semantic output is accepted while
ordinary finalization and workflow transition checks are skipped. The run loop
can interpret a `plan <id>` recommendation transiently, but repository evidence
remains unchanged; later detection therefore returns to `concoct next`.

`concoct next` prompt rendering is intentionally read-only and presents
validated roadmap, dependency, prerequisite, capability, and archive evidence.
That remains useful for inspection, but it is not a sufficient autonomous
lifecycle boundary. Existing run approval records bind one gate to current
repository, workflow, configuration, and next-action evidence, providing a
foundation for approval without re-invocation, but they do not retain the
Product Owner's semantic decision or proposed mutations.

CON-040 supplies the missing boundary between semantic Product Owner judgment
and executable planning authority. It also repairs ready-to-ready reporting so
successful integration remains visible long enough for delivered roadmap and
capability evidence to be reconciled before subsequent selection.

## Why this matters

Today Concoct can pay for a correct Product Owner decision, accept its structured
result, and then discard its operational meaning. Repeating the command consumes
more model allocation without progress and may eventually trip duplicate or
run bounds. Durable, one-use decision evidence makes product authority real,
keeps human approvals meaningful, and lets every command report the same next
action from repository-backed state rather than conversation or private adapter
output.

## Current state

- `internal/workflow.ResolveAction` maps every `ready` report to the executable
  `product-owner-next` action and `concoct next`; no pending Product Owner
  decision participates in state or next-action derivation.
- `internal/workflow.InspectNextActionEvidence` validates deterministic
  read-only roadmap, capability, dependency, prerequisite, archive, and origin
  evidence. Structural planning eligibility is already shared with
  `ValidatePlanItem`.
- `internal/prompt` renders `next` as a read-only Product Owner recommendation
  and declares no authorized updates.
- `internal/orchestration` represents Product Owner output as a bounded
  `Recommendation` containing a kind, command, and reason. The
  `product-owner-next` spec completes in `ready`, permits no authoritative
  selection or reconciliation mutation, and treats a command string as the
  useful semantic result.
- `internal/execution` explicitly excludes `product-owner-next` from normal
  completion/finalization behavior and retains the result only in private
  invocation evidence.
- `internal/runloop` interprets a Product Owner recommendation only within the
  current process. Pending gate storage in `internal/runstate` binds approval to
  evidence, but does not persist the proposed Product Owner decision and
  mutations as reusable workflow state.
- Integration returns to `ready` and recommends `concoct next`, while the
  preceding completion can be obscured by fresh policy eligibility and
  no-active-task diagnostics.
- Existing tests encode the read-only behavior in workflow, prompt,
  orchestration, execution, run-loop, CLI, initialization, and integration
  fixtures.

## Target state

- Product Owner supervised output uses a versioned semantic decision with
  exactly one of `select`, `reconcile-and-select`, `reconcile`,
  `human-decision-required`, or `no-action`; a display command is derived from
  the decision and is never its sole durable meaning.
- An executable-owned, bounded decision record retains the selected item,
  rationale, proposed roadmap/capability changes, source-evidence digest,
  invocation/outcome correlation, approval state, and intended next transition.
  Its location, schema, lifecycle, privacy, and Git treatment are explicit and
  validated.
- Product Owner candidates can propose only authorized roadmap changes and
  capability reconciliation supported by accepted archive, delivery, and
  integration evidence. Executable validation rejects malformed, unrelated, or
  unsupported mutations without silently changing canonical truth.
- The configured `next` gate protects application of one retained decision.
  Approval validates current evidence and applies that exact candidate without
  another model call. A no-gate policy may apply it immediately.
- Applying `select` or `reconcile-and-select` consumes the decision once and
  authorizes the exact planning action. A separately configured planning gate
  still applies. Drift, activation, supersession, or changed prerequisites
  invalidates stale evidence with an actionable reason.
- Candidate roadmap items are visible to Product Owner judgment. The item
  selected for planning still passes the shared structural eligibility contract
  after any authorized promotion or refinement.
- `status`, `next`, explanation output, `exec`, and `run` use one typed
  next-action authority that accounts for pending, approved, applied,
  invalidated, and completed Product Owner decisions.
- Manual `concoct next` or an explicitly named inspection mode remains
  deterministic and non-mutating; autonomous execution uses the supervised
  decision boundary.
- Ready-to-ready reconciliation preserves the just-completed transition and
  authoritative delivery evidence until the Product Owner has reconciled it.
  Clean `ready` output describes absent task phases as inactive or not
  applicable.
- Invocation measurement follows a Product Owner decision through proposal,
  approval, application, invalidation, and reuse, counting accepted cost only
  when the action leaves valid durable progress, intervention, or mutation.

## Design constraints

- Product Owner judgment owns product direction; deterministic code validates,
  retains, approves, applies, or invalidates that judgment but does not rank or
  select roadmap work itself.
- Durable workflow artifacts and executable-owned local records remain distinct.
  Any private decision record must survive command exit and be discoverable by
  all relevant commands, while semantic roadmap/capability changes remain
  repository evidence when applied.
- Capability changes require explicit accepted archive or delivery provenance.
  Product Owner reconciliation cannot manufacture acceptance, edit historical
  archives, or bypass Reviewer, Archivist, or Integrator authority.
- Stable roadmap/capability identifiers, supported status transitions,
  dependency/prerequisite references, selected-item uniqueness, reserved IDs,
  and supported work origins must fail closed before canonical mutations.
- Evidence binding must include all inputs whose change can alter the Product
  Owner decision or planning eligibility, without retaining unbounded prompts,
  model output, or repository content.
- Application must be transactional: canonical mutations, decision state, gate
  consumption, and planning authorization cannot leave an ambiguous partially
  applied state. Preserve and diagnose rejected supervised candidates.
- Reuse existing workflow detection, structural planning validation, action
  correlation, approval binding, adapter supervision, Git branch creation, and
  cost measurement rather than introducing competing authorities.
- Manual prompt semantics and supervised prompt semantics must share the same
  Product Owner decision vocabulary even when their mutation authority differs.
- Keep source and embedded-template counterparts synchronized. Preserve CLI
  portability, deterministic output, privacy bounds, and current manual role
  commands.

## Non-goals

- No general retained-candidate repair for Developer, Reviewer, or Archivist
  finalization; that remains CON-039.
- No crash-resumable multi-action coordinator, permanent run history, or broad
  workflow lease; those remain CON-034.
- No deterministic or executable-owned product prioritization. Product
  prioritization remains Product Owner judgment, subject to configured human
  approval; Concoct validates, retains, applies, or rejects that judgment without
  substituting presentation order or hard-coded ranking.
- No capability mutation based on unreviewed implementation or unsupported
  prose inference.
- No bypass of review, archival, Git integration, or task-branch isolation.
- No remote push, pull-request, or external delivery automation.
- No general-purpose roadmap editor, project doctor, historical recovery, or
  arbitrary mutation protocol.

## Working assumptions

- The current private run-state area can be extended or complemented with a
  bounded Product Owner decision store, provided all commands use a common
  reader and the record is not mistaken for accepted product truth.
- The existing orchestration correlation and evidence digest can be evolved to
  bind a richer decision candidate; a protocol version change or explicitly
  compatible extension will be chosen based on fixture and adapter impact.
- Candidate promotion to `planned` can be validated using the current roadmap
  parser plus stricter record-level comparison and `ValidatePlanItem` after the
  proposed mutation is applied to an isolated candidate view.
- Ready-to-ready completion evidence can remain bounded and local while archive,
  capability, roadmap, and Git commits remain the permanent authorities.
- CON-039 may still be a useful fixture even if its live roadmap status changes;
  tests should construct isolated candidate and planned variants rather than
  depend on the repository's future state.

## Risks and open questions

- The decision record's exact location and commit/ignore boundary must balance
  cross-command durability, Git cleanliness, user inspection, and stale-record
  cleanup. The developer must document the chosen authority and retention model.
- Proposed decisions may remain bounded and local while awaiting approval. Once
  approval is consumed, the decision must be repository-backed or atomically
  reflected in canonical roadmap/task evidence. Interruption between approval
  and planning must resume the exact approved selection without Product Owner
  reinvocation or repeated approval.
- Roadmap and capability mutations are Markdown record edits. Candidate capture
  must avoid unbounded patches while still proving exact authorized changes and
  preventing unrelated edits.
- A decision may combine delivered-item reconciliation and next-item selection.
  Validation and rollback must make this one atomic semantic application rather
  than two independently replayable operations.
- Planning creates and checks out a task branch. The handoff between applied
  selection evidence, a planning approval gate, and `concoct plan <id>` must not
  consume authority early or strand an approved selection.
- `next` currently means both a manual read-only prompt and the autonomous ready
  action. CLI naming must remain understandable and backward compatible while
  exposing the new supervised behavior accurately.
- Existing status has no durable `pending-product-decision` state. The
  implementation must decide whether this is a workflow state, a ready substate,
  or typed next-action evidence without making canonical phase reporting
  contradictory.
- Ready-to-ready completion visibility needs a bounded clearing rule so stale
  integration success does not permanently shadow current product work.

## Implementation phases

### Phase 1 — Specify the decision and persistence contracts

Status: `complete`

- Trace current ready resolution, Product Owner rendering, structured outcome,
  execution special cases, run approval flow, planning authorization,
  integration completion, and measurement disposition end to end.
- Define the versioned Product Owner decision schema, permitted mutations,
  lifecycle states, source digest, approval/application semantics, retention,
  invalidation, and operator rendering.
- Define transactional boundaries for candidate validation, canonical mutation,
  gate consumption, planning authorization, and recovery from interruption.
- Add focused contract tests for semantic decisions and exact evidence binding
  before wiring CLI behavior.

### Phase 2 — Build validated decision capture and application

Status: `complete`

- Extend Product Owner prompt and structured outcome contracts to require one
  semantic decision and bounded proposed changes rather than a command-only
  recommendation.
- Validate roadmap refinements/promotions and capability reconciliation against
  stable records, accepted provenance, scope, supported origins, and the
  post-mutation planning eligibility of any selected item.
- Persist valid pending/accepted decisions atomically with executable-owned
  correlation and evidence. Preserve invalid candidates and actionable
  diagnostics without applying unauthorized changes.
- Implement exact-once approval/application, drift and supersession detection,
  safe deterministic reuse, and explicit consumption/invalidation.

### Phase 3 — Unify ready-state derivation and lifecycle continuation

Status: `complete`

Progress: approval and application of a retained `select` decision now have
separate durable lifecycle states. Consuming the `next` gate marks the decision
`approved`; only a validated Task Planner transition marks it `applied`. An
interrupted or rejected planner attempt resumes the exact approved selection
without a second Product Owner invocation or selection approval.

- Make workflow/action resolution represent retained decisions and derive the
  exact Product Owner, approval, reconciliation, or planning continuation from
  one authority.
- Update `exec` and `run` so autonomous ready work captures or resumes a
  decision, applies the configured `next` gate to that retained decision, and
  proceeds directly to separately gated planning without re-invocation.
- Preserve a read-only Product Owner inspection/rendering path and align its
  semantic vocabulary with supervised execution.
- Carry integration completion evidence into one ready-state reconciliation
  cycle and correct inactive phase/Git reporting when no task exists.
- Update cost disposition and duplicate/bound checks so reuse is non-billable
  and semantic output without durable progress is not accepted cost.

### Phase 4 — Complete CLI, documentation, and compatibility behavior

Status: `complete`

Progress: retained Product Owner decisions are now visible in `status` and
`exec inspect`; the manual prompt/persona use the shared semantic vocabulary
and deterministic fixtures cover that read-only rendering. Reconciliation
application, corresponding operator effects, and the remaining compatibility
documentation still require completion.

- Expose pending, approved, applied, invalidated, reconciled, and no-action
  decisions with rationale, proposed/applied effects, provenance, and exact next
  commands in status, explanation, execution inspection, and run summaries.
- Update command reference, state machine, workflow guidance, README, built-in
  Product Owner persona/handoff/prompt resources, and template counterparts.
- Preserve manual command compatibility and clearly distinguish inspection from
  supervised mutation and approval.
- Add fixtures for planned selection, candidate reconcile-and-select,
  reconciliation-only, human decision, no action, invalid mutation, drift,
  supersession, duplicate invocation prevention, and post-integration
  reconciliation.

### Phase 5 — Verify and prepare independent review

Status: `complete`

- Run focused package tests throughout, then the complete Go test, race, vet,
  native build, and Windows build suites.
- Validate deterministic prompt/golden changes, private-record bounds,
  repository non-mutation on read-only/refusal paths, and source/template parity.
- Initialize a fresh project outside the repository and confirm selective
  template copying, Git/bootstrap behavior, ready reporting, and the new Product
  Owner flow.
- Run shell syntax validation, executable-mode checks, `git diff --check`, stale
  terminology/path searches, and focused final diff inspection.
- Update task status and durable notes with implementation decisions, test
  evidence, remaining risks, capability impact, and Reviewer handoff.

## Acceptance criteria

- In an isolated ready project where `CON-039` is a suitable candidate, one
  Product Owner invocation can propose authorized refinement/promotion and
  selection, retain the exact evidence-bound decision awaiting `next` approval,
  and make no canonical mutation before approval.
- Approving the retained `reconcile-and-select` decision makes no Product Owner
  invocation, applies only the validated proposed changes, consumes the decision
  once, and advances to the exact `concoct plan CON-039` continuation subject to
  the planning gate.
- When `CON-039` is already planned, one Product Owner invocation retains and
  applies a `select` decision without roadmap mutation; `status`, inspection,
  `exec`, and `run` all report or perform planning rather than returning to
  `concoct next`.
- `reconcile` applies only authorized roadmap/capability changes;
  `human-decision-required` names one unresolved product decision; `no-action`
  reports genuine absence of actionable or reconcilable work. None is reduced
  to advice to invoke another Product Owner command.
- Repeating an operation against unchanged pending or accepted evidence reuses
  the decision without a billable agent call. Applying it twice is impossible.
- Changes to relevant roadmap, capability, archive, integration, policy,
  configuration, active-task, prerequisite, or approval evidence invalidate or
  supersede the retained decision before mutation with a specific diagnostic.
- Unauthorized roadmap fields, unrelated records, invalid status changes,
  missing or false archive provenance, capability changes without accepted
  evidence, and ambiguous selected items are rejected without canonical
  roadmap/capability mutation.
- After integration, operator output preserves the completed transition and
  routes through Product Owner reconciliation; delivered evidence can be
  reconciled and subsequent work selected in one action where valid.
- A clean ready project reports absent task phases and Git task metadata as
  inactive/not applicable and does not claim blocked work or a non-Git active
  task.
- Manual Product Owner inspection remains deterministic and read-only. Its
  output uses the same decision kinds and authoritative input contract as the
  supervised action.
- Measurement and inspection evidence links the invocation to proposal,
  approval, application, invalidation, or reuse, and does not classify a
  command-only recommendation with no durable effect as accepted progress.
- Regression tests reproduce both 2026-08-18 failures: accepted planned-item
  selection reverting to `concoct next`, and accepted roadmap reconciliation
  being ignored by later state derivation.
- Reproduce the retained 2026-08-18 outcomes through offline adapter fixtures.
  No billable model invocation is required for CON-040 acceptance. Any
  optional live exercise requires separate operator authorization and is not a
  delivery gate.
- Existing manual task planning, development, review, archive, integration,
  policy gates, structured result validation, Git isolation, and capability
  reconciliation continue to pass their regression suites.
- An applicable `human-decision-required` or `no-action` decision remains
  visible and suppresses repeated Product Owner invocation until authoritative
  evidence changes, the human resolves the named decision, or the outcome is
  explicitly superseded.

## Verification

- Focused tests: `go test ./internal/workflow ./internal/prompt ./internal/orchestration ./internal/execution ./internal/runstate ./internal/runloop ./internal/cli ./internal/integration`
- Full tests: `go test -count=1 ./...`
- Race tests: `go test -race ./...`
- Static checks: `go vet ./...`
- Builds: `go build ./cmd/concoct` and `GOOS=windows GOARCH=amd64 go build -o <temporary-output> ./cmd/concoct`
- Shell and mode checks: `bash -n cmd/concoct/concoct.sh` and confirm the wrapper remains executable.
- Initialization: run `./cmd/concoct/concoct.sh init <temporary-project-path>` outside this repository; verify root files, dotfiles, nested project-owned templates, planning directories, Git initialization, bootstrap prompt, and exclusion of built-in protocol/persona/prompt files.
- Parity: compare changed source/template counterparts and update deterministic prompt golden fixtures deliberately.
- Hygiene: `git diff --check`, search for stale read-only Product Owner contracts and obsolete paths/branding, and inspect the complete task diff.

## Handoff expectations

The Developer should begin with the decision/persistence contract and record the
chosen authority, schema, transaction sequence, and invalidation inputs before
broad integration changes. The final Developer handoff must enumerate changed
workflow, orchestration, execution, run-state, CLI, documentation, and template
surfaces; map tests to each acceptance criterion; report the two reproduced
failures; distinguish automated from live checks; and call out any migration,
retention, privacy, or compatibility risk for independent review.
