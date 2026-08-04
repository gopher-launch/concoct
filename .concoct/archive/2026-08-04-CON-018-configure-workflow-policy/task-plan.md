---
id: CON-018
title: Configure workflow policy
roadmap-id: CON-018
status: implementation-complete
remediates-review: review-01.md
created: 2026-08-04
updated: 2026-08-04
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-018-configure-workflow-policy
  base: f2b8cee2ec4b411b10164c36a4e3dc576867b029
  archive-commit: self
  status: archived
capability-impact:
  type: update
  ids:
    - CAP-001
    - CAP-005
    - CAP-006
    - CAP-007
    - CAP-008
    - CAP-009
    - CAP-010
    - CAP-011
  rationale: Makes the accepted lifecycle configurable through a typed policy whose resolved activity requirements govern state, recommendations, prompts, completion gates, archival, and Git applicability while preserving the current default behavior.
---

# Task Plan

## Goal

Implement a small, typed workflow-policy model that lets a repository select
supported lifecycle requirements and records an explicit resolved disposition
for every governed activity, while preserving protocol invariants and the
current Concoct lifecycle as the default.

## Context

CON-017 separated executable-owned protocol, project-selected policy, and
repository-owned guidance, but policy front matter is currently checked only
for declaration ownership and non-empty `required-phases`, `approval-gates`,
and `git-strategy`. Workflow detection, recommendations, prompts, completion,
archival, and integration still encode the default lifecycle directly.

CON-018 makes the policy layer operational through a finite set of known
activities and dispositions. It must not become an arbitrary workflow graph or
weaken evidence integrity, immutable reviews, artifact ownership, capability
reconciliation, or invalid-state refusal.

## Why this matters

Repositories differ in which lifecycle activities apply and how completion is
proven. Typed, inspectable policy lets Concoct represent those differences
honestly instead of requiring forks or treating missing evidence as an implicit
skip. It is also foundational for adoption, task origins, profiles, workflow
explanation, and later orchestration.

## Current state

- `internal/instruction.Compose` parses a narrow declaration syntax, validates
  layer ownership and protocol controls, and preserves source bytes, but returns
  no typed effective policy.
- `.concoct/policy.md` and its template declare the current phases, approval
  gates, and task-branch Git strategy as untyped names.
- `internal/workflow` hard-codes state, next actions, review precedence,
  archival readiness, and Git applicability.
- `internal/prompt` reuses workflow state but hard-codes role modes, inputs,
  authorized updates, expected outcomes, and next transitions.
- Developer, Reviewer, Archivist, and integration completion boundaries enforce
  the current gates independently.
- Status and prompts do not expose a normalized requirement and disposition for
  every governed activity.
- Existing tests comprehensively cover the accepted default lifecycle and form
  the compatibility baseline.

## Target state

- One strict typed model parses a closed set of supported activities,
  requirement modes, conditions, gates, and the existing managed Git strategy.
- Every governed activity resolves visibly to exactly `completed`,
  `not-required`, `not-applicable`, `externally-satisfied`, or `blocked`.
- Any permitted skip or external satisfaction retains an explicit reason and,
  where applicable, safe durable evidence with ownership and provenance.
- Workflow state, command eligibility, recommendations, role prompts,
  completion gates, archival readiness, and Git applicability consume the same
  effective policy and resolution result.
- The checked-in default policy reproduces the exact lifecycle currently
  covered by tests.
- At least two supported policy selections produce observably different valid
  requirements without changing protocol or project guidance.

## Design constraints

- Protocol controls are unconditional: durable evidence establishes state,
  completed reviews remain immutable, artifact ownership is enforced,
  capability impact is resolved before acceptance, archival remains factual,
  and contradictory evidence stays invalid.
- Use one policy and resolution authority across composition, workflow, prompts,
  transitions, archival, and integration; do not introduce a second state
  machine.
- Keep the model finite and declarative. Reject unknown or impossible values
  with source-aware diagnostics; do not evaluate arbitrary expressions or
  user-defined graphs.
- Conditional activities resolve only from deterministic repository or task
  context defined by the product, never free-form prose or ambient state.
- Missing artifacts never authorize a skip. Non-required and externally
  satisfied dispositions require explicit durable reasons; external evidence
  must resolve safely.
- Policy stays project-owned in `.concoct/policy.md`; task-specific resolution
  evidence belongs in canonical active-task evidence with role ownership.
- Preserve the existing default Git/non-Git behavior, prompt determinism,
  executable-owned guidance boundary, and source/template relationships.

## Non-goals

- No arbitrary workflow graph, custom activity, or expression language.
- No alternate Git strategy from CON-020; task-branch squash integration remains
  the only managed Git strategy.
- No profiles, multiple task origins, brownfield adoption, or client-upgrade
  framework from CON-023, CON-019, CON-016, or CON-013.
- No new explanation command from CON-025 beyond policy detail required in
  existing status and prompts.
- No direct agent execution or orchestration from CON-010, CON-032, or CON-033.
- No weakening of review, capability, archive, or Git recovery guarantees.

## Working assumptions

- The governed registry derives from the accepted lifecycle: product ownership,
  task planning, development, independent review, archival, and integration,
  plus only the gates needed to represent their completion conditions.
- Exact Go package names and YAML shape are technical choices, provided the
  schema is closed, human-readable, strictly decoded, and fully documented.
- Configured requirement modes and evaluated dispositions are distinct types.
- Non-Git operation is the initial deterministic `not-applicable` case;
  supported fixtures can demonstrate other dispositions without implementing
  alternate Git strategies.
- Capability limitations are compatible: semantic role judgment remains human
  or agent work, policy validation stays structural and evidence-based, prompts
  remain guidance, and Git conflict meaning stays human-attested.
- No new capability ID is expected; accepted work updates the existing
  capabilities materially affected by policy-aware behavior.

## Risks and open questions

- A separate policy resolver could diverge from workflow detection; resolution
  must occur before transition selection and flow through every consumer.
- Optional activity support could normalize incomplete work unless every
  non-completed disposition has explicit owned evidence and negative coverage.
- Task-scoped disposition metadata affects completion path allowlists, archive
  schemas, and integration recovery and therefore needs end-to-end tests.
- Policy context can make golden prompts noisy or nondeterministic unless it has
  one compact stable rendering.
- The current line-oriented parser may be unsafe for nested policy data; strict
  YAML decoding with known fields is preferable if nesting is required.
- No product decision blocks planning. The finite boundary, disposition
  vocabulary, evidence requirement, invariant protections, default behavior,
  and adjacent exclusions are explicit.

## Implementation phases

### Phase 1 — Define and validate the typed contract

Status: `complete`

- Inventory hard-coded activities, gates, next-action decisions, and Git
  applicability across instruction, workflow, prompt, transition, archive, and
  integration packages.
- Introduce a closed typed policy parser and normalized effective-policy model
  with source-aware, all-or-nothing validation.
- Define per-activity resolutions including reason and durable evidence rules.
- Update the default policy template to express the accepted lifecycle.

The supported selectable boundary is now explicit: product ownership, task
planning, development, archival, and integration remain required by the
current command/state contract; independent review may be required or omitted
with a reason. Review and integration gates must exactly support those choices,
and unsupported omissions fail during composition rather than reaching a
transition that cannot honor them.

### Phase 2 — Resolve policy against durable evidence

Status: `complete`

- Add one deterministic resolver combining effective policy with repository
  type, active task, reviews, archive state, and integration evidence.
- Extend task evidence only as needed for authorized non-required or externally
  satisfied dispositions, reasons, and safe repository-relative references.
- Reject missing, contradictory, stale, unsafe, unsupported, or owner-invalid
  evidence rather than inferring a skip.
- Preserve review precedence, remediation, blocker recovery, archival truth,
  capability impact, and interrupted Git recovery.

Implemented so far: the resolver produces deterministic activity dispositions
and validates task-scoped externally satisfied independent-review evidence,
including durable reason, recorder, and repository-relative file checks that
reject symbolic links in every path component. Invalid task or evidence states
never render external satisfaction. Other activities reject
external-satisfaction claims because their canonical transitions remain
required. Interrupted integration recovery deliberately remains available only
to preserve or unwind an already-recorded transaction; it cannot start a new
transition and has explicit regression coverage under invalid current policy.

### Phase 3 — Drive behavior from resolved policy

Status: `complete`

- Make state detection, command eligibility, and next-action selection consume
  resolved requirements while retaining one state authority.
- Expose each activity's requirement, disposition, reason/evidence, and blocker
  through `status` and applicable role prompts in deterministic order.
- Derive prompt modes, authorized updates, follow-ups, completion gates,
  archival readiness, and Git applicability from the same result.
- Keep alternate Git strategies outside this task.

Implemented so far: status, archive prompt eligibility, and archive completion
honor an externally satisfied or explicitly non-required independent review.
Developer prompts and completion now select a fresh Reviewer or Archivist
handoff from that same resolution, and direct integration enforces the composed
policy before mutation. Status and non-default role prompts attribute every
activity requirement to `.concoct/policy.md`. The final branch and recovery
audit found no unresolved duplicated eligibility path.

The review prompt and exclusive review reservation now refuse when the resolved
next action is archival, and the Developer prompt presents that same next
action. `code --complete` validates the corresponding policy-selected outgoing
handoff rather than always requiring Reviewer evidence.

Direct managed integration startup now rejects a policy that does not require
integration; existing `--continue` and `--abort` recovery remain available to
preserve and resolve already-created recovery evidence.

### Phase 4 — Prove compatibility and supported variation

Status: `complete`

- Preserve the full existing suite and add a default-policy compatibility matrix
  for states, commands, prompts, completion, archival, and Git/non-Git behavior.
- Add at least two distinct supported policy fixtures and end-to-end cases.
- Cover every disposition plus failures for silent absence, missing reasons,
  unsafe or missing external evidence, unknown values, impossible combinations,
  invariant conflicts, and partial transitions.
- Add parser/resolver, workflow, prompt, CLI, archive, integration,
  initialization, and recovery regression coverage.

Implemented so far: parser, status, prompt, archive, integration fixture, and
safe external-evidence regression coverage, including negative cases for
unsupported omissions, missing gates, and unsupported external satisfaction,
plus clean initialization smoke tests. Default and explicitly non-required
review selections now have CLI completion coverage with different outgoing
handoffs and next actions. Evidence coverage includes traversal, symlinked-parent,
invalid-state suppression, and benign filename cases. The final matrix reaches
and validates all five dispositions, and interrupted recovery is covered under
both normal and invalid-current-guidance conditions.

### Phase 5 — Document and prepare review

Status: `complete`

- Update README and normative instruction-layer, workflow, command, and state
  documentation with schema, resolution semantics, default behavior,
  ownership, supported variation, and exclusions.
- Update executable-owned personas/handoffs and adapters only where observable
  policy guidance changes.
- Run repository verification, inspect golden changes, search for stale
  hard-coded lifecycle claims, and prepare a focused Reviewer handoff.

Implemented so far: instruction-layer and state-machine documentation and
root/template default policy alignment. The instruction-layer reference now
defines the supported selectable boundary and external-review evidence scope;
the command and state references describe policy-selected Developer handoffs.
README, workflow, and the root/template Codex adapters are reconciled; the
stale-claim audit, full verification, fresh initialization, and final Reviewer
handoff are complete.

## Acceptance criteria

1. `.concoct/policy.md` parses into one closed typed policy; unknown fields,
   activities, modes, conditions, gates, strategies, duplicates, and
   invariant-weakening combinations fail before partial state or output.
2. Every governed activity exposes one configured requirement and resolves to
   exactly `completed`, `not-required`, `not-applicable`,
   `externally-satisfied`, or `blocked`, with source attribution.
3. Authorized skips have durable reasons, external satisfaction has safe
   resolvable evidence and provenance, and artifact absence never authorizes a
   skip.
4. State, command eligibility, recommendations, prompts, completion, archival,
   and Git applicability agree because they consume one resolution result.
5. The default policy reproduces all accepted states, role boundaries, gates,
   task-branch squash behavior, diagnostics, and recovery semantics.
6. Two repositories can select different supported policies and receive
   observably different valid requirements, dispositions, prompts, and next
   actions without changing protocol.
7. Completed reviews remain immutable, contradictory evidence remains invalid,
   capability impact is resolved before acceptance, archival remains factual,
   and artifact ownership is enforced at every mutation boundary.
8. Status and applicable role prompts expose requirements, dispositions,
   reasons/evidence, and actionable blockers deterministically.
9. Invalid policy or evidence fails with diagnostics naming the source,
   activity/field, conflict, and safe corrective action, without partial
   mutation or prompt output.
10. Alternate Git strategies, profiles, origins, adoption, upgrades, custom
    graphs, and direct agent execution remain outside delivered behavior.

## Verification

- Run `gofmt` and focused typed-policy/resolution tests.
- Run focused workflow, prompt, CLI transition, archive, integration, and
  initialization tests for the default and supported variants.
- Run `go test -count=1 ./...`, `go vet ./...`, and
  `go build ./cmd/concoct`.
- Run `bash -n cmd/concoct/concoct.sh` and verify executable mode.
- Initialize a temporary project and verify project-owned output, omitted
  built-ins, policy schema, Git staging/no commit, bootstrap, ready state, and
  default prompts.
- Exercise two policy selections end to end, including non-Git applicability
  and explicit externally satisfied evidence.
- Inspect prompt golden changes for deterministic policy context; compare
  intended source/template counterparts.
- Search active code and documentation for stale hard-coded policy claims.
- Run `git diff --check` and inspect the complete task diff.

## Capability impact

Expected impact is `update` for CAP-001, CAP-005, CAP-006, CAP-007, CAP-008,
CAP-009, CAP-010, and CAP-011. Workflow contract, status, rendering, Git
applicability, planning, recommendation, completion, and archival become
policy-aware while the current default and protocol guarantees remain intact.
No new capability ID is expected.

## Handoff expectations

The Developer should first inventory hard-coded lifecycle decisions and set the
task to `implementation-in-progress`. Keep one typed policy/resolution authority
and the existing command surface. Before review, record the final schema,
activity registry, ownership rules, supported variants, files changed, checks,
golden changes, risks, skipped work, and capability impact in `notes.md`; set
status to `implementation-complete`; and recommend `concoct code --complete`.
