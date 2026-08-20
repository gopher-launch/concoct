---
version: 1
project: concoct
updated: 2026-08-20
---

# Roadmap

## Purpose

This roadmap defines the planned evolution of Concoct.

Concoct is a lightweight, agent-neutral workflow coordinator that turns rough ideas into implementation-ready work and guides transitions between planning, implementation, review, and archival roles.

Core loop:

```text
human idea → concoct(**product-owner** → roadmap → **task-planner** → task-plan → **developer** → source code → **reviewer** → review → **archivist** → capabilities) → product
```

Concoct is strict about the integrity, provenance, and interpretation of
durable evidence. It is configurable about how work enters the system, which
activities are required, who performs them, and how accepted work reaches the
product. Automation should reduce context-creation and handoff effort without
hiding the applicable prompts, personas, evidence, decisions, or transitions.

This file records intended future work.

It is distinct from:

- `.concoct/capabilities.md`, which records what Concoct can do now;
- `.concoct/current/task-plan.md`, which records the currently active implementation task;
- `.concoct/archive/`, which preserves completed task history.

## Roadmap conventions

Each roadmap item has a stable identifier.

Statuses for outstanding work:

- `candidate` — accepted as a possible direction but not yet ordered for implementation;
- `planned` — ready to be turned into an active task plan;
- `active` — currently represented by `.concoct/current/task-plan.md`;
- `blocked` — cannot proceed until a dependency or decision is resolved;
- `deferred` — intentionally postponed;
- `cancelled` — no longer intended and awaiting removal after its rationale and
  identifier reservation are preserved.

Priorities:

- `critical`
- `high`
- `medium`
- `low`

`Depends on` records only unresolved delivery dependencies on other outstanding
roadmap items. `Capability prerequisites` records enduring reliance on accepted
current behavior in `.concoct/capabilities.md`. Satisfied sequencing constraints
and historical provenance do not belong in either field.

Delivered and cancelled items leave the active roadmap after their relationships
are reconciled. Their identifiers remain reserved and must not be reused.

Reserved historical identifiers: `CON-003`, `CON-004`, `CON-005`, `CON-006`,
`CON-007`, `CON-008`, `CON-009`, `CON-015`, `CON-018`, `CON-028`, `CON-029`,
`CON-010`, `CON-030`, `CON-031`, `CON-032`, `CON-035`, `CON-036`, `CON-037`,
`CON-038`, `CON-040`.
Accepted delivery evidence is preserved by the corresponding capability records
and archives; CON-004 was cancelled as redundant and has no delivery archive.

A roadmap item should describe a coherent outcome. Detailed implementation steps belong in the task plan created by:

```text
concoct plan <roadmap-id>
```

---

## CON-011 — Add workflow diagnostics and recovery

- Status: `candidate`
- Priority: `medium`
- Depends on: None
- Capability prerequisites: CAP-001, CAP-005, CAP-007, CAP-011
- Capability impact: adds maintenance and recovery support

### Outcome

Add commands that detect drift, incomplete transitions, and malformed artifacts.

### Proposed commands

```text
concoct doctor
concoct abandon
concoct recover
```

### Requirements

`doctor` should detect:

- missing canonical files;
- malformed front matter;
- invalid status transitions;
- orphaned review files;
- archive-reference drift;
- capability references to missing IDs;
- roadmap references to missing archives;
- stale generated prompts;
- inconsistent current-task state.

`abandon` should:

- require explicit confirmation;
- preserve the abandoned task in an archive-like record;
- explain whether roadmap status changes to `planned`, `deferred`, or `cancelled`.

`recover` should:

- reconstruct state from durable artifacts;
- never discard code or planning files silently;
- explain every proposed repair.

### Acceptance criteria

- Diagnostics are read-only by default.
- Repairs require explicit action.
- Recovery preserves project archaeology.
- Errors contain actionable next steps.

---

## CON-012 — Improve project archaeology and reporting

- Status: `candidate`
- Priority: `low`
- Depends on: None
- Capability prerequisites: CAP-001, CAP-011
- Capability impact: adds historical reporting

### Outcome

Make the archive useful for understanding how the project evolved without turning Concoct into a project-management platform.

### Possible capabilities

```text
concoct history
concoct show <roadmap-id>
concoct capability <capability-id>
```

### Requirements

- Read from Markdown artifacts rather than a separate database.
- Trace roadmap items to tasks, reviews, archives, and capabilities.
- Keep reports human-readable and script-friendly.
- Avoid introducing dashboards or remote services.

### Acceptance criteria

- A user can trace why a capability exists.
- A user can inspect the delivery history of a roadmap item.
- Historical reporting does not mutate project state.

---

## CON-013 — Add opt-in client project upgrades

- Status: `candidate`
- Priority: `medium`
- Depends on: CON-014
- Capability prerequisites: CAP-003, CAP-005
- Capability impact: adds safe lifecycle upgrades for Concoct-enabled projects

### Outcome

Allow a user to run:

```text
concoct upgrade
```

to deliberately bring an existing Concoct-enabled project onto a supported newer Concoct contract without silently replacing project-owned content or losing workflow history.

### Rationale

Concoct installs durable workflow files into client repositories. As that installed contract evolves, users need a low-friction maintenance path that keeps client projects current while respecting local changes and making every upgrade an explicit choice.

### Requirements

- Make upgrades opt-in and show the proposed target before changing the project.
- Identify the project's installed Concoct version or contract level and report when its origin cannot be determined reliably.
- Preview the affected files, migrations, conflicts, and preserved local changes.
- Distinguish Concoct-owned state from conventional project files and locally customized guidance.
- Never silently overwrite conflicting client changes; stop with actionable choices or preserve them for explicit reconciliation.
- Preserve active task state, roadmap and capability records, reviews, archives, and repository history across supported upgrades.
- Leave the project recoverable when an upgrade cannot complete, and report what changed and what remains unresolved.
- Explain when a source or target version is unsupported and recommend a safe next action.

### Product decisions before planning

- Define how installed projects record their Concoct version or contract level.
- Define the authoritative source of upgrade content and how users select or constrain a target release.
- Define ownership and merge policy for conventional files that Concoct installs but projects are expected to customize, including `AGENTS.md` and tool adapters.
- Decide whether preview is the default behavior or whether execution instead requires an explicit confirmation or apply option.

### Acceptance criteria

- An eligible client project can preview an upgrade without mutation.
- Applying an upgrade requires explicit user intent and reports the selected source and target.
- An unmodified supported installation upgrades to the expected contract while preserving project workflow data.
- Locally modified or ambiguous files are never overwritten without an explicit resolution.
- Failed or unsupported upgrades leave the project usable and provide actionable recovery guidance.
- A completed upgrade reports every changed, preserved, skipped, and conflicted artifact.

---

## CON-014 — Add explicit, optional client overlays

- Status: `candidate`
- Priority: `high`
- Depends on: None
- Capability prerequisites: CAP-003, CAP-004, CAP-006
- Capability impact: adds a supported customization layer for client-specific workflow guidance

### Outcome

Allow a Concoct-enabled project to explicitly declare project-owned overlays
that augment or replace supported portions of the versioned built-in workflow
content, using deterministic composition, source attribution, and compatibility
validation.

Overlays extend the project-guidance and workflow-policy layers established by
the current workflow contract. They are not the boundary between Concoct
protocol and project-owned truth, and they cannot weaken protocol invariants.

### Rationale

Concoct's shared workflow contract must remain reusable and portable, while client projects need durable guidance for their own domain, organization, and working practices. A first-class overlay boundary lets clients specialize the installed workflow without forking the base contract or making ordinary local edits indistinguishable from supported customization.

### Requirements

- Keep overlays optional; projects without an overlay retain the standard Concoct behavior.
- Require overlays to be explicitly selected or declared rather than inferred from incidental files.
- Reference embedded resources by stable logical identifier rather than their internal source path.
- Default to augmentation and permit replacement only for resource types explicitly declared replaceable.
- Record the Concoct compatibility range expected by an overlay.
- Support client-specific instructions, skills, and prompts, plus augmentation of base personas.
- Define deterministic composition and precedence so humans and agents can inspect the effective guidance and understand its origin.
- Preserve the agent-neutral base contract and keep overlay content distinct from Concoct-owned templates and workflow state.
- Preserve overlay files as project-owned content across executable upgrades.
- Report whether each effective instruction came from an embedded default, project guidance, or an overlay.
- Never interpret legacy materialized defaults as overlays merely because the files exist.
- Validate overlay references and incompatible customizations with clear, actionable errors.
- Detect overlay incompatibility before rendering a partial or internally inconsistent prompt.
- Ensure generated or rendered role guidance consistently includes applicable overlays without requiring a specific agent integration.
- Make the overlay boundary available to lifecycle operations so upgrades can preserve client-owned customization without treating it as an ambiguous edit to the base installation.

### Product decisions before planning

- Decide whether a project may compose multiple overlays or selects exactly one.
- Define whether overlays are project-local only or may also come from reusable external packages, and how their identity is recorded durably.
- Define which customization types may replace base guidance and which may only augment it.
- Define whether overlays apply during initialization, to existing enabled projects, or both.

### Acceptance criteria

- A project can use Concoct with no overlay and receive the standard installed and rendered workflow contract.
- A project can explicitly enable an overlay containing each supported customization type.
- Applicable client instructions, skills, prompts, and persona augmentations appear in the effective workflow guidance with deterministic precedence.
- A user can identify which effective guidance came from the base contract and which came from an overlay.
- Missing, malformed, or incompatible overlays fail clearly without partially changing project workflow state.
- Overlay behavior remains portable across the prompt-only workflow and does not require a direct agent execution adapter.
- Upgrade planning can distinguish and preserve declared overlay content from locally modified base files.

---

## CON-016 — Adopt an existing repository

- Status: `candidate`
- Priority: `high`
- Depends on: None
- Capability prerequisites: CAP-001, CAP-003, CAP-005
- Capability impact: adds safe Concoct onboarding for brownfield repositories

### Outcome

Inspect an existing repository and produce an auditable, versionable adoption
proposal before Concoct creates or changes files. Approved application must
preserve repository-owned truth and validate that the inspected state has not
materially drifted.

### Requirements

- Follow an explicit `inspect → report → configure → approve → apply → validate` lifecycle.
- Discover repository boundaries, instructions, product and architecture documentation, verification commands, compatibility constraints, delivery practices, and existing planning sources.
- Report proposed artifacts, preserved files, conflicts, uncertainty, and questions requiring human resolution.
- Treat existing `AGENTS.md` and equivalent guidance as project-owned.
- Never manufacture historical archives or silently promote discovered claims into accepted capabilities.
- Keep inspection non-mutating, make repeated inspection safe, and detect drift before apply.

### Acceptance criteria

- A mature repository can receive a non-mutating adoption report without first becoming Concoct-shaped.
- Every proposed create, modify, reference, and preserve action is inspectable before approval.
- Apply requires explicit approval, refuses stale proposals safely, and leaves project-owned truth intact.
- Validation explains the resulting effective workflow and any unresolved uncertainty.

---

## CON-019 — Support multiple task origins

- Status: `candidate`
- Priority: `high`
- Depends on: None
- Capability prerequisites: CAP-001, CAP-005
- Capability impact: allows repository work that does not originate in the product roadmap

### Outcome

Give every task explicit provenance while treating roadmap work as one origin
alongside issues, incidents, maintenance, security, dependency changes,
investigations, experiments, review findings, and external changes.

### Requirements

- Record a typed origin and an optional external reference in durable task metadata.
- Preserve `concoct plan <roadmap-id>` as the roadmap-origin path.
- Support creation of non-roadmap tasks without manufacturing strategy entries.
- Classify capability impact as add, change, remove, none, or unknown during planning.
- Require unknown capability impact to be resolved before acceptance.

### Acceptance criteria

- Roadmap and non-roadmap tasks enter the same evidence model with inspectable provenance.
- Origin-specific validation fails clearly without imposing irrelevant roadmap requirements.
- Status, prompts, archives, and reports retain the original provenance.
- Accepted non-roadmap work reconciles capability truth when affected.

---

## CON-020 — Make Git lifecycle strategy-selectable

- Status: `candidate`
- Priority: `high`
- Depends on: None
- Capability prerequisites: CAP-007
- Capability impact: generalizes Git integration while preserving task-branch behavior as the default managed strategy

### Outcome

Expose the accepted task-branch lifecycle through a stable Git strategy boundary
and support current-branch, externally managed, and non-Git lifecycles with
strategy-appropriate evidence and recovery.

### Requirements

- Define where work is isolated, which repository state is authoritative, who integrates, and what proves completion for every strategy.
- Retain CAP-007 task-branch isolation and squash integration as the default managed strategy.
- Let external strategies record trustworthy integration evidence without making Concoct perform the integration.
- Define interruption, resume, drift, and reconciliation behavior for each strategy.
- Never claim integration or completion from ambient branch state alone.

### Acceptance criteria

- Each supported strategy has deterministic starting, archival, integration, recovery, and completion states.
- Existing task-branch projects retain their accepted behavior by default.
- External integration can be proven without granting Concoct control of the provider operation.
- Strategy changes cannot reinterpret existing task evidence silently.

---

## CON-021 — Represent provisional product knowledge

- Status: `candidate`
- Priority: `medium`
- Depends on: CON-016
- Capability prerequisites: CAP-001
- Capability impact: distinguishes accepted product truth from repository discoveries under evaluation

### Outcome

Record adoption and investigation findings with status, confidence, and cited
evidence without prematurely promoting them into canonical capabilities.

### Requirements

- Keep `.concoct/capabilities.md` as simple accepted current truth.
- Store proposed, disputed, obsolete, and unknown claims in a distinct discovery artifact.
- Distinguish verified, documented, and inferred confidence and retain evidence references.
- Make promotion to accepted capability truth explicit and reviewable.

### Acceptance criteria

- Repository inspection can preserve useful uncertain findings without changing capability truth.
- Users can trace each discovery claim to evidence and see its acceptance status.
- Disputed or obsolete findings cannot be consumed as accepted capabilities.
- Promotion preserves provenance and requires an explicit acceptance boundary.

---

## CON-022 — Plan from repository evidence

- Status: `candidate`
- Priority: `high`
- Depends on: CON-016, CON-021
- Capability prerequisites: CAP-001, CAP-006
- Capability impact: makes brownfield task planning evidence-aware

### Outcome

Assemble a bounded, task-relevant repository evidence package before planning
work in an existing project, showing what was examined, why it matters, and
what remains uncertain.

### Requirements

- Select evidence according to the requested change and repository domain rather than dumping the repository.
- Include relevant structure, interfaces, compatibility guarantees, tests, CI, migrations, consumers, and documentation.
- Distinguish current inspection from adoption-baseline knowledge that may have gone stale.
- Preserve an inventory of included, excluded, and unresolved evidence for planner and reviewer use.

### Acceptance criteria

- A planner can trace material assumptions to bounded repository evidence.
- The evidence package is inspectable, deterministic for unchanged inputs, and proportionate to the task.
- Stale baseline claims and unresolved conflicts remain visible.
- Historical Concoct reporting remains separately scoped to CON-012.

---

## CON-023 — Support task profiles

- Status: `candidate`
- Priority: `medium`
- Depends on: CON-019
- Capability prerequisites: CAP-001
- Capability impact: adds reusable, inspectable policy presets for common kinds of work

### Outcome

Allow projects to name policy selections for feature, maintenance, incident,
experiment, and other recurring work without creating separate hard-coded
workflow engines.

### Requirements

- Resolve a profile into ordinary visible workflow policy for the selected task.
- Keep profile source, selected values, and overrides inspectable.
- Validate profiles against protocol invariants and the project's supported policy schema.
- Allow explicit per-task choices without requiring a profile.

### Acceptance criteria

- Selecting a profile produces the same observable policy as selecting its values directly.
- Prompts, status, and archives identify the selected profile and resolved rules.
- Missing or incompatible profiles fail without partial task creation.
- Profiles cannot hide skipped activities or weaken protocol invariants.

---

## CON-024 — Support concurrent and interrupting work

- Status: `candidate`
- Priority: `medium`
- Depends on: CON-019, CON-020
- Capability prerequisites: CAP-001, CAP-005, CAP-007
- Capability impact: removes the single-active-task repository constraint

### Outcome

Allow multiple durable tasks to coexist and be selected unambiguously across
branches, worktrees, contributors, interruptions, and external reviews.

### Requirements

- Give each task a stable identity and isolated artifact location.
- Resolve the applicable task from explicit selection or trustworthy repository context.
- Prevent branches, worktrees, reviews, and integration evidence from being attributed to the wrong task.
- Support pausing blocked work while maintenance or an incident proceeds.
- Preserve the current single-task model as a compatible default or migration source.

### Acceptance criteria

- Two tasks can coexist without ambiguous ownership of plans, notes, reviews, or Git evidence.
- Task selection and valid transitions are deterministic in supported contexts.
- Interrupting work does not rewrite or invalidate the paused task's evidence.
- Status reports ambiguity instead of guessing when no unique task can be resolved.

---

## CON-025 — Explain effective workflow and state

- Status: `candidate`
- Priority: `medium`
- Depends on: CON-020, CON-023
- Capability prerequisites: CAP-005, CAP-006
- Capability impact: makes configured workflow behavior and state interpretation directly inspectable

### Outcome

Explain the effective workflow, task rules, state evidence, valid transitions,
and blocked transitions together with the configuration source of each rule.

### Requirements

- Report active profile, resolved phase requirements, Git strategy, and task origin.
- Cite the evidence used to determine current state.
- Explain valid and blocked transitions and their reasons.
- Attribute effective values to defaults, project policy, profiles, or task-specific choices.
- Keep explanation read-only and script-friendly.

### Acceptance criteria

- A user can determine why Concoct selected a state or refused a transition without reading implementation code.
- Explanations agree with status and prompt selection for the same repository state.
- Defaulted and explicitly configured behavior are distinguishable.
- Invalid evidence is reported rather than normalized away.

---

## CON-026 — Reconcile externally performed work

- Status: `candidate`
- Priority: `medium`
- Depends on: CON-019, CON-020, CON-022
- Capability prerequisites: CAP-001, CAP-005
- Capability impact: establishes trustworthy Concoct state for work begun or completed outside its managed lifecycle

### Outcome

Inspect existing branches, commits, pull requests, emergency fixes, automation,
or externally merged contributions and propose the evidence and remaining work
needed to bring them into Concoct's durable model.

### Requirements

- Inspect and propose before mutating workflow truth.
- Support retrospective task records, proposed capability changes, missing review work, integration evidence, and unresolved provenance questions.
- Refuse acceptance when trustworthy state cannot be established.
- Distinguish reconciliation from adoption of the repository itself.
- Preserve external identifiers and evidence sources.

### Acceptance criteria

- Existing work can enter Concoct without being falsely represented as having followed the managed lifecycle.
- The proposal identifies verified evidence, gaps, required decisions, and the path to acceptance.
- No external work is marked accepted solely because a branch or commit exists.
- Applied reconciliation remains traceable through task, review, archive, capability, and integration records as applicable.

---

## CON-027 — Track and resolve product bugs

- Status: `candidate`
- Priority: `high`
- Depends on: CON-019
- Capability prerequisites: CAP-001, CAP-005, CAP-006, CAP-011
- Capability impact: adds a durable bug register and bug-origin task lifecycle without treating defects as product-roadmap work

### Outcome

Allow projects to record, triage, resolve, verify, and close observed
contradictions in accepted or intended product behavior while keeping defects
distinct from roadmap direction, capability truth, and active-task review
findings.

A defect discovered before task acceptance remains an obligation of the active
task. A defect discovered after acceptance becomes a separately tracked bug.

### Rationale

Roadmap items describe intended product evolution, so using them as a defect
register obscures both strategy and current product truth. A dedicated bug
lifecycle preserves durable evidence of observed failures and their disposition
while allowing remediation to use the ordinary reviewed task lifecycle with
explicit bug provenance.

### Requirements

- Introduce `.concoct/bugs.md` as the authoritative human-readable register for
  independently tracked product defects.
- Give each bug a stable unique identifier, lifecycle status, severity,
  expected and observed behavior, evidence, affected capabilities or intended
  behavior, and durable resolution references.
- Support the primary lifecycle `reported → confirmed → planned → in-progress
  → resolved → verified → closed`, plus reasoned alternate dispositions for
  duplicate, not-a-bug, cannot-reproduce, deferred, and superseded reports.
- Keep pre-acceptance defects and review findings within their active task;
  create a separate bug only after acceptance or when the defect is otherwise
  independent of active-task obligations.
- Allow a confirmed bug to originate or become associated with an ordinary
  CON-019 task without manufacturing a roadmap item, and maintain deterministic
  bidirectional references between the bug and repair task.
- Carry repair work through the configured planning, implementation, review,
  archive, and delivery lifecycle while retaining links among the bug, task,
  reviews, archive, capability reconciliation, and delivery evidence.
- Classify each report during triage as a failure to conform to an accepted
  capability, an incorrect or incomplete capability contract, intended but not
  yet accepted behavior, or not a product defect.
- Require repair tasks to classify capability impact as none, clarification,
  change, or unknown; permit unknown during triage and planning but resolve it
  before acceptance.
- Do not change capability truth merely because a bug is reported. Restore
  conformance without manufacturing a new capability when the accepted
  contract is already correct; route genuine product evolution through an
  explicit roadmap decision and accepted capability reconciliation.
- Provide supported operations to report, inspect, list and filter, triage,
  reprioritize, associate repair work, record resolution and independent
  verification, close, reopen, and find bugs by affected capability.
- Make status and role prompts expose applicable bug state, invalid evidence,
  and valid next actions without silently synchronizing divergent bug and task
  states or treating implementation, review, archive, or integration as closure.
- Preserve unresolved, deferred, duplicate, rejected, superseded, closed, and
  reopened bug history rather than deleting or rewriting prior evidence.

### State integrity

- Confirmed bugs require durable supporting evidence.
- Planned and in-progress bugs reference a valid repair task, and a bug-origin
  repair task references a valid bug.
- Resolved means implementation claims the defect is corrected; verified
  requires independent evidence against the originally reported behavior.
- Verification requires accepted repair work, and closure requires complete
  resolution, verification, provenance, and capability-impact evidence.
- Duplicate reports identify their canonical bug, and reopened bugs preserve
  earlier resolution and verification history.
- Contradictory or incomplete evidence produces an invalid state rather than an
  inferred or arbitrarily selected result.
- Bug state remains derivable from durable artifacts without conversation
  history.

### Initial validation

- Seed the first accepted post-CON-015 output-path defect as a confirmed bug
  record: ignored and untracked in-repository prompt output should be allowed,
  while tracked or unignored output remains rejected and external output
  remains supported.
- Validate the artifact and state model during CON-027 with representative
  evidence for bidirectional task references, capability-impact resolution,
  invalid states, closure, and reopening; do not fabricate those later states
  on the seeded real bug.
- Do not claim that CON-027 itself completed a bug-origin repair lifecycle. Its
  accepted delivery makes that lifecycle available; the seeded bug's repair
  must then proceed as a separately evidenced bug-origin task.

### Documentation

- Explain the boundaries among bugs, roadmap items, capabilities, tasks, and
  active-task review findings.
- Document lifecycle states and dispositions, triage and capability
  responsibilities, repair-task creation and resumption, the distinction
  between resolution and verification, and preservation of historical evidence.

### Acceptance criteria

- A post-acceptance defect can be recorded and triaged without adding product
  evolution to the roadmap or changing accepted capability truth.
- A pre-acceptance defect remains visibly owned by its active task and cannot be
  closed through the separate bug lifecycle to bypass task review.
- A confirmed bug can originate or associate with an ordinary repair task, and
  both artifacts retain deterministic provenance from report through review,
  archive, delivery, capability reconciliation, and verified closure.
- Bug closure requires observable verification against the reported behavior;
  implementation completion, review approval, or integration alone is not
  treated as verification unless the required evidence is present.
- Capability-conformance repairs do not create spurious capabilities, while
  changed expected behavior cannot be closed as a bug without an explicit
  product decision and accepted capability reconciliation.
- Listing and filtering can identify open bugs and all bugs affecting a
  selected capability without requiring archive or conversation reconstruction.
- Reopening and alternate dispositions preserve reasons, links, and prior
  evidence with clear, valid next actions.
- Malformed, contradictory, missing, or stale bug and task references fail
  clearly without partially mutating bug, task, roadmap, or capability state.

---

## CON-034 — Make lifecycle runs durable and resumable

- Status: `cancelled`
- Priority: `high`
- Depends on: CON-033
- Capability prerequisites: CAP-001, CAP-005, CAP-007, CAP-009
- Capability impact: None; its durable-session, restart-reconciliation, checkpoint, and abandonment scope is absorbed by CON-039

### Outcome

Retain this identifier as the superseded proposal for durable lifecycle runs.
CON-039 now establishes the durable governed work session as the principal
execution unit and absorbs every remaining requirement from this item. No
separate durable-run authority or history model should be implemented.

### Rationale

A run record beside a work-session record would create competing execution
state. The governed-session architecture resolves the same interruption,
reconciliation, stale-result, intervention, and abandonment problems while also
separating lifecycle, assurance, authorization, and cost state.

### Historical disposition

- Stable run and attempt identity, restart reconciliation, safe continuation,
  unresolved intervention, checkpointing, and abandonment move to CON-039.
- Future cross-machine or shared-store portability must extend the CON-039
  `SessionStore` contract rather than revive a separate run subsystem.
- Accepted lifecycle and integration evidence remains canonical; session state
  must reconcile with it rather than replace it.

---

## CON-039 — Establish durable governed work sessions and recoverable execution

- Status: `planned`
- Priority: `critical`
- Depends on: None
- Capability prerequisites: CAP-001, CAP-005, CAP-007, CAP-010, CAP-011, CAP-012, CAP-013, CAP-014, CAP-015
- Capability impact: replaces role-per-invocation orchestration with a portable, recoverable, policy-governed work-session foundation and cost-aware primary-agent execution

### Outcome

Make a durable governed work session the principal execution unit. Concoct
computes eligible work, records an operator or policy-admissible agent selection,
transactionally claims one item, supervises a resumable primary agent across
ordinary lifecycle stages, owns every deterministic mechanism, preserves
recoverable candidates and checkpoints, records assurance truthfully, governs
cost, and requires exact separate grants for integration and publication.

Lifecycle stages remain canonical repository evidence but no longer imply one
role, one model context, or one invocation. The ordinary primary agent may plan,
implement, validate, self-review, correct, and prepare archival meaning in one
continuing context. Same-context review is useful assurance evidence but never
independent review.

### Rationale

Strict outer validation, exact Git transitions, evidence-bound decisions, and
integration recovery are valuable, but ordinary agent candidates can still become
stranded between authorship and deterministic finalization. `concoct run` retains
no durable session identity, cycle count, budget, work ownership, or restart state.
It launches a fresh ephemeral Codex process for every lifecycle action.

CON-037 through CON-041 show that repeated tool/edit exchanges and cached
conversation-prefix replay dominate cost. Static prompt reduction and per-invocation
budgets can measure or contain that cost but cannot remove the amplification.
Concoct must reduce model exchanges by owning mechanical operations, reuse primary
semantic context across stages, bound tool results, and deliberately compact or
checkpoint context before replay becomes excessive.

The CON-041 free-range session demonstrated that one capable primary agent can
carry ordinary work quickly and self-correct. It also demonstrated that repository
instructions do not confer integration or publication authority and that
same-context critique cannot be represented as independent review.

### Requirements

#### Separate state planes

- Keep lifecycle, session, assurance, authorization, and cost state separate.
- Continue deriving lifecycle truth from validated task, review, archive, policy,
  capability, roadmap, and Git evidence.
- Persist a stable work-session identity, selected item, repository baseline,
  current stage, attempts, checkpoints, stop reason, unresolved intervention,
  cost, assurance requirements, and authority references.
- Reconcile session expectations with canonical evidence after every restart; a
  session record must never become a competing lifecycle authority.

#### Portable state and target-tree neutrality

- Define an execution-neutral `SessionStore` contract and keep operational state
  outside the target project tree by default.
- Permit project-managed, external-state, portable, and temporary/no-attribution
  modes without making Git a session-storage requirement.
- A no-attribution workflow may leave local audit history outside the target tree
  while pushing no Concoct metadata, artifacts, branch names, or commit attribution
  to the target origin.
- Start with a simple local compare-and-swap store while keeping future shared or
  cross-clone stores implementable behind the same contract.

#### Work selection and transactional claim

- Compute a typed eligible work set from roadmap state, readiness, priority,
  dependencies, capabilities, blockers, ownership, repository capacity, project
  policy, item policy, agent capability, risk, and budget.
- Allow operator selection and policy-bounded agent selection through one shared
  path. Agent selection requires a uniquely dominant eligible item, complete
  product intent, no material ambiguity, bounded rationale, and immediate
  executable revalidation.
- Atomically claim the selected item for one session using exact policy,
  repository, roadmap, capability, configuration, and ownership evidence.
- Recover every claim crash point without selecting different work or producing
  duplicate ownership. Keep the initial lease local and simple without preventing
  a future shared-store implementation.

#### Executable-owned mechanics

- Move every deterministic operation into the Concoct executable: eligibility,
  branch or worktree setup, metadata projection, review reservation, evidence
  snapshots, diff summaries, bounded validation execution, lifecycle commits,
  archive copying and structure, recovery, cost accounting, grants, integration,
  publication, and cleanup.
- Ask agents only for semantic product judgment, planning, implementation,
  debugging, review judgment, correction, and archival/capability meaning.
- Never ask an agent to manufacture executable-owned provenance, exact paths,
  status projections, commit boundaries, or mechanically derivable ledger edits.

#### Recoverable session attempts

- Record a bounded baseline/candidate/current manifest for each attempt, including
  authorized effects, process identity, invocation linkage, diagnostics, cost, and
  terminal disposition.
- Treat structured output as a candidate declaration, not proof of completion.
- Preserve rejected or interrupted candidates and distinguish them from later,
  unrelated, stale, superseded, or ambiguously overlapping changes.
- Support inspect, deterministic-finalize, semantic-continue, checkpoint, and
  explicit operator abandonment without replaying proven-complete model work.
- Prepare executable prerequisites before invocation, including exclusive review
  evidence, and make deterministic repair or finalization non-billable.
- Persist review/remediation counts and all session bounds across coordinator
  processes so restart cannot evade policy.

#### Resumable primary-agent execution and context cost

- Introduce an adapter capability for starting, continuing, compacting,
  checkpointing, and closing one primary agent context.
- Permit that primary context to carry planning, implementation, validation,
  same-context self-review, correction, and archival preparation across lifecycle
  stages while the executable changes authorized effect scopes mechanically.
- Reduce conversation amplification with bounded tool results, executable-generated
  stage and recovery briefs, repeated-operation detection, and deliberate context
  compaction or rollover at stage and cost thresholds.
- Benchmark equivalent workloads; resumability alone is not evidence of lower
  cost and must not permit an indefinitely growing prefix.

#### Truthful assurance and selectable risk policy

- Record self-review, isolated-agent review, external review, deterministic checks,
  candidate revision, context identities, findings, and dispositions as separate
  assurance facts.
- Never describe same-context self-review as independent, including after an
  operator relaxes the required assurance level.
- Provide built-in `relaxed`, `standard`, and `strict` risk profiles covering at
  least security/authorization, destructive work, persistence/migrations,
  recovery/concurrency, public APIs, release/supply chain, external side effects,
  billing/cost, privacy/compliance, broad diffs, weak verification, and agent
  uncertainty. Default to `standard`.
- Require a fresh executor-created review context when resolved risk policy
  requires isolated review. Legacy approved reviews before CON-041 remain
  grandfathered; CON-041 remains truthfully same-context reviewed and
  operator-accepted.

#### Actors, grants, overrides, and publication

- Attribute material events to authenticated operator, primary agent,
  isolated-review agent, executable, or external-system contexts independently of
  Git authorship.
- Keep grant creation in the operator control plane; an agent may request but may
  not mint operator authority or relabel its actor identity.
- Permit an authenticated and authorized operator to relax project or item policy
  at any point through an exact, scoped, reasoned, expiring, auditable override.
- Preserve evidence truth under every override: policy may be relaxed, but actor,
  assurance, result, and provenance facts may not be falsified.
- Remove automatic push. Require separate, short-lived, one-use grants for local
  integration and external publication.
- Bind integration to the exact task, archive revision, trunk, and pre-integration
  head. Bind publication to the exact local revision, remote name and URL identity,
  destination ref, expected remote head where available, and non-force behavior.

#### Cost governance

- Govern session cost independently from cost efficiency using available native
  input, cached-input, output, reasoning, elapsed, activity, and command-output
  measurements, with optional credit or currency providers when authoritative.
- Reserve sufficient budget for required assurance before implementation spends
  the remaining allocation.
- On exhaustion, create a durable checkpoint and return control without blind
  retry, silent assurance reduction, or candidate loss.
- Allow an agent to request additional session budget with bounded rationale. A
  session top-up requires an exact policy-controlled operator grant and does not
  itself purchase external account credits or change an account plan.

### Product decisions before planning

- The operator has approved the complete requirements and authority boundaries in
  this record. No additional pre-planning product decision remains.
- An operator override may relax policy but may not falsify evidence or actor and
  assurance provenance.
- The initial store and work claim are local and KISS; their interfaces must not
  prevent later portability or coordination across clones.
- Ordinary session authority may be granted once at launch. Integration and
  publication remain separate just-in-time grants.
- The implementation should use Go and the Codex app-server or equivalent supported
  resumable protocol rather than introducing a JavaScript/TypeScript runtime.

### Scope boundaries

- This item delivers one coherent governed-session vertical slice for a single
  locally claimed item. Parallel active tasks, distributed locks, and cross-clone
  coordination remain future extensions of the store/claim contracts.
- It does not add general-purpose project archaeology or historical repair from
  CON-011.
- It does not require Git for session persistence, remote account-credit purchase,
  or permanent retention of raw conversation transcripts.
- Manual role commands and current adapter behavior are not compatibility
  requirements before v1. Preserve only mechanisms that support the target design.
- Do not weaken strict lifecycle validation, archive truth, capability provenance,
  or safe integration recovery to make a session appear successful.

### Acceptance criteria

- A portable external session store creates, compares-and-swaps, loads, checkpoints,
  reconciles, and finalizes a session without writing Concoct state into the target
  directory; project-managed mode remains possible through the same contract.
- Eligible-set output is deterministic and explains every included and excluded
  item. Operator and admissible agent selection use one transactional work-claim
  path, and racing or interrupted claims produce exactly one owner.
- A work session survives process restart with its stage, attempts, intervention,
  assurance, cycle count, budget, and safe continuation intact without replaying
  proven-complete work.
- The executable prepares and owns all deterministic workflow evidence and
  transition operations. Agents are not required to synthesize reservation markers,
  exact metadata, commit boundaries, archive paths, or mechanical ledger edits.
- Rejected, interrupted, stale, superseded, disjoint, and ambiguously overlapping
  candidates reconcile correctly; ambiguity preserves all work and stops for the
  operator.
- One resumable primary Codex context can cross ordinary semantic stages with
  executable-controlled scopes. Stage briefs and compaction keep retained context
  bounded, and an equivalent-workload report distinguishes governance from actual
  cost reduction.
- Same-context review cannot satisfy isolated-review policy. A distinct
  executor-created context bound to the exact candidate revision can satisfy it.
- Operator overrides are exact and auditable while underlying evidence remains
  truthful. Agent contexts cannot mint grants or claim operator identity.
- Integration and publication require distinct exact grants. No configuration,
  repository instruction, role output, or integration grant can cause a push.
- Budget exhaustion checkpoints the session and does not retry or weaken assurance;
  an exact operator grant can add a bounded session budget and resume it.
- Deterministic tests cover every claim, attempt, checkpoint, compaction, assurance,
  override, grant, integration, publication, and abandonment crash boundary without
  live or billable model execution.

---

## CON-041 — Repair structured-output schema and terminal failure diagnostics

- Status: `delivered`
- Archive: `.concoct/archive/2026-08-18-CON-041-repair-structured-output-schema-and-terminal-failure-diagnostics/`
- Priority: `critical`
- Depends on: None
- Capability prerequisites: CAP-001, CAP-012, CAP-013, CAP-014, CAP-015
- Capability impact: corrects supervised structured-output contracts, failure evidence, and actionable retry guidance

### Outcome

Repair the post-CON-040 Product Owner invocation failure by making every generated
structured-output schema locally contract-valid, rejecting malformed results before
Codex invocation, and preserving bounded Codex terminal diagnostics through one-shot
and lifecycle-run reporting.

### Rationale

The retained post-CON-040 invocation demonstrates that the generated Product Owner
schema declared `mutations` but omitted it from the parent object's required fields.
Codex rejected the schema before inference, while reconciliation discarded the
structured terminal diagnostic and recommended an unchanged blind retry. This wastes
operator time and model capacity and obscures the exact configuration defect.

### Requirements

- Require every declared object property exactly once in `required`, require
  `additionalProperties: false`, and validate this contract recursively offline for
  every role/action output schema.
- Correct Product Owner outcomes so `mutations` is always present, including an empty
  array for decisions without mutations, and cover every supported decision kind.
- Reject malformed generated schemas and malformed or omitted mutations locally
  before launching Codex.
- Parse only bounded diagnostic type/message fields from `turn.failed`, retain them
  in measurement and reconciliation evidence, and display them in `exec` and `run`.
- Do not depend on stdout or stderr logs for JSONL-reported Codex failures and do not
  retain arbitrary raw event content.
- Classify pre-inference schema rejection as retryable only after the exact schema or
  configuration defect changes, and report that corrective action or blocker.

### Acceptance criteria

- The exact post-CON-040 Product Owner schema passes the recursive schema-contract
  validator and requires `product_decision.mutations`.
- Empty-mutation select, reconcile-and-select, reconcile,
  human-decision-required, and no-action results validate; omitted or malformed
  mutations fail before adapter launch.
- Every generated role/action schema is exercised by the offline validator with
  complete paths for missing and duplicate required properties.
- A `turn.started` → `turn.failed` → nonzero exit sequence with no adapter result
  and no usage retains and displays the bounded type/message diagnostic in both
  one-shot and run output.
- Focused and full tests, race tests, vet, native and Windows builds,
  source/template parity, fresh initialization, shell validation, and diff checks
  pass without any live or billable model invocation.
