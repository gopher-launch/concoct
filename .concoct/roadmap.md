---
version: 1
project: concoct
updated: 2026-08-12
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
`CON-010`, `CON-030`, `CON-031`, `CON-032`, `CON-035`.
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

- Status: `candidate`
- Priority: `high`
- Depends on: CON-033
- Capability prerequisites: CAP-001, CAP-005, CAP-007, CAP-009
- Capability impact: makes lifecycle orchestration durable across interruption, failure, intervention, and environment restart

### Outcome

Make lifecycle runs inspectable, recoverable, and resumable without replaying
completed actions, losing intervention context, overwriting user changes, or
trusting stale agent results.

### Rationale

Long-running agent workflows will be interrupted. Correct orchestration must
reconcile persisted attempt evidence with current workflow, repository, and Git
state before it can decide whether an action completed, can be retried, or
requires intervention.

### Requirements

- Give each run and action attempt a stable identity and persist only the state required to reconstruct policy, validated actions, the current attempt, stop reason, unresolved intervention, and last validated state.
- Distinguish running, completed, cancelled, interrupted, blocked, awaiting-approval, and failed run states.
- Detect abandoned attempts and reconcile recorded run evidence with current workflow and Git state before continuing.
- Never replay proven-complete actions or accept results from stale or superseded attempts.
- Treat uncertain completion conservatively and require explicit blocker or decision resolution when needed.
- Detect and preserve user changes made while paused; refuse continuation when external changes invalidate the prior recommendation.
- Support inspection, safe continuation, and abandonment without discarding the underlying task state.
- Separate recoverable local execution records from permanent task history and deliberately promote only material decisions and outcomes.
- Reuse existing workflow and integration recovery semantics rather than creating a competing state authority.

### Product decisions before planning

- Define which run data is repository-resident, local-only, portable, and retained for how long.
- Decide how to detect an agent process that may remain alive after coordinator disconnection.
- Define evidence sufficient to classify uncertain completion and whether a retry creates a new attempt or action instance.
- Define how resolved decisions enter durable project history and whether abandonment releases any workflow or repository lease.

### Acceptance criteria

- Interruption between actions resumes at the next valid action without replay.
- Interruption during an action does not assume success or failure before reconciliation.
- Stale outcomes and externally invalidated recommendations cannot be applied.
- Resolved gates, blockers, and decisions can continue the same run with retained context.
- A user can inspect and safely abandon an incomplete run without corrupting task state.

---

## CON-037 — Measure and reduce agent invocation cost

- Status: `planned`
- Priority: `critical`
- Depends on: None
- Capability prerequisites: CAP-005, CAP-006, CAP-010, CAP-013, CAP-014
- Capability impact: makes agent execution cost observable and establishes evidence-backed reductions required for practical daily orchestration

### Outcome

Measure the complete prompt composition and reported token usage of every supported
agent invocation, establish reproducible per-role baselines, and deliver verified
initial reductions without weakening workflow authority, role independence, safety,
or structured-result validation.

### Rationale

The first real `concoct run` completed an end-to-end lifecycle but consumed an
unsustainable amount of the user's ChatGPT Plus usage allocation. Four explicitly
observed invocations alone reported 181,120 tokens, including a 76,142-token Developer
remediation and a 45,838-token Archivist invocation. The complete lifecycle consumed
materially more.

At the current rate, more than half of the available usage allocation can be consumed
in less than two days. This prevents Concoct from serving as the intended daily
execution engine.

Likely contributors include repeated executable-owned instructions, broad roadmap and
capability context, duplicated instruction layers, repository rediscovery, large tool
outputs, uniformly expensive execution profiles, and repeated agent invocations caused
by failed mechanical transitions. These are hypotheses rather than measurements.
Optimization must begin with reliable attribution, then make reductions against fixed
baselines while preserving behaviour.

The complete originating evidence is recorded in:

```text
.concoct/reports/con-033-first-run-findings.md
```

### Requirements

#### Capture authoritative invocation usage

- Run the Codex adapter with machine-readable execution events and retain the final
  structured result independently from progress and diagnostic output.
- Capture the usage reported by Codex for every completed or failed turn, including
  input tokens, cached input tokens, output tokens, reasoning-output tokens when
  supplied, and any total reported by the installed adapter version.
- Treat adapter-reported usage as attribution evidence rather than reconstructing an
  incompatible total when fields are unavailable or change across versions.
- Record the acting role, action, adapter, model, reasoning effort, start time,
  duration, disposition, retry or predecessor relationship, and invocation identity
  alongside usage.
- Preserve partial usage and execution evidence when an invocation fails, times out,
  or is cancelled and the adapter emitted it before termination.
- Do not confuse cached input with free input. Report it separately so both gross
  context size and effective cache reuse remain visible.

#### Attribute prompt composition

- Measure the exact rendered prompt byte count supplied to the adapter.
- Attribute prompt bytes to stable semantic components, including executable protocol,
  persona, handoff, supervision appendix, project guidance, policy, roadmap context,
  capability context, task plan, notes, reviews, archive evidence, and other selected
  task inputs.
- Define component boundaries during prompt composition rather than attempting to
  reverse-engineer them from the final concatenated prompt.
- Record whether each component was included in full, selected, summarized, or absent.
- Detect duplicated byte-identical or normalized-identical instruction material and
  report its contribution without silently removing it.
- Never retain secrets, unrestricted environment content, authentication material, or
  otherwise prohibited data merely for cost attribution.

#### Provide useful inspection and summaries

- Extend retained invocation inspection to show usage totals, prompt size and
  composition, execution profile, duration, outcome, and comparison with a prior or
  baseline invocation when available.
- Include a concise per-action usage line in `concoct exec` and `concoct run` summaries.
- Include aggregate run totals by role and action while avoiding double-counting the
  same adapter-reported usage event.
- Distinguish agent cost from mechanical Concoct actions that invoke no model.
- Make retained measurement data bounded by existing invocation retention policy and
  keep it local and ignored by Git unless deliberately exported.
- Provide an explicit diagnostic/export view suitable for comparing controlled
  baselines without exposing full prompts or sensitive repository content by default.

#### Establish reproducible baselines

- Define controlled fixtures for Product Owner, Task Planner, initial Developer,
  remediation Developer, Reviewer, and Archivist invocations.
- Hold repository state, prompt inputs, adapter version, model, reasoning effort, and
  expected result contract constant within each comparison.
- Record prompt composition and adapter-reported usage for every baseline.
- Permit baseline collection without mutating a real active project or consuming a
  production workflow transition.
- Clearly distinguish deterministic local measurements such as prompt bytes from
  externally variable measurements such as model token use, cache hits, duration, and
  reasoning output.
- Produce a baseline report identifying the largest prompt components, roles, repeated
  context, and avoidable re-invocation patterns.

#### Deliver evidence-backed initial reductions

- Use the baseline evidence to implement at least one material reduction in repeated
  prompt or context cost during this item; do not stop after adding instrumentation.
- Prefer removal of proven duplication, selection of task-relevant records, compact
  machine requirements, and elimination of unnecessary agent invocations before
  lowering model capability or reasoning effort.
- Include only roadmap items, capability records, reviews, archive evidence, and other
  context that the role contract actually requires, while retaining explicit
  provenance for selected or summarized inputs.
- Avoid asking an agent to rediscover deterministic workflow facts or perform
  executable-owned mechanical transitions that Concoct can resolve safely itself.
- Preserve byte-identical manual and automated semantic instructions for the same role
  and state, subject to the same measured composition rules.
- Record every removed, selected, summarized, or relocated instruction component and
  demonstrate why the resulting role still receives sufficient authority, context,
  constraints, and completion requirements.
- Do not weaken Product Owner authority, plan acceptance, Reviewer independence,
  capability-impact validation, archive validation, permission controls, or human
  approval gates to reduce usage.

#### Improve progress handling needed for measurement

- Consume structured Codex execution events rather than treating raw combined output
  as the primary progress protocol.
- Retain structured events, final result, and stderr diagnostics as distinct bounded
  artifacts.
- Render a compact continuous progress view from selected semantic events so reaching
  a display limit does not make an active invocation appear frozen.
- Keep display truncation independent from diagnostic retention limits and continue
  processing later events after earlier display content has been discarded.
- Preserve enough raw structured evidence to diagnose usage and execution behaviour
  without flooding the terminal.

#### Protect compatibility and failure behaviour

- Detect when an installed Codex version lacks required structured events or usage
  fields and fail clearly or use an explicitly documented degraded mode.
- Do not parse human-readable progress text as authoritative usage when structured
  usage is available.
- Treat malformed, duplicate, contradictory, or out-of-order terminal usage events as
  diagnostic failures rather than silently producing misleading totals.
- Preserve the accepted structured outcome and workflow state even when usage
  attribution or progress rendering encounters a recoverable error.
- Keep adapter-specific event and usage semantics outside the workflow engine.

### Product decisions

- Treat cost observability as part of the adapter execution boundary. The workflow
  engine knows roles and actions; the adapter translates its native event stream into
  normalized optional usage and progress evidence.
- Use adapter-reported token fields as the authoritative observed values and retain
  their original meanings. Do not collapse input, cached input, output, and reasoning
  output into an unexplained single number.
- Record exact prompt bytes and semantic component sizes for deterministic local
  attribution. Token estimates may supplement these measurements but must be labelled
  as estimates unless reported by the adapter.
- Keep detailed invocation measurements in ignored, bounded runtime state. Durable
  project history records only material optimization decisions and benchmark results,
  not full prompts, raw event streams, or per-turn repository content.
- Baselines are execution-profile-specific. Comparisons must name adapter version,
  model, reasoning effort, fixture identity, and relevant cache state rather than
  presenting one universal token number.
- Require this item to ship at least one measured reduction. Instrumentation alone is
  insufficient because current usage is already a critical blocker.
- Define a material initial reduction as at least a 30 percent decrease in exact
  rendered prompt bytes for one or more dominant fixed-role baselines, or an equivalent
  elimination of a redundant full agent invocation, with no regression in required
  inputs, structured outcomes, workflow validation, or fixture acceptance behaviour.
- Do not make automatic model downgrades part of the initial reduction. Model and
  reasoning-profile experiments may be reported for later product decisions, but
  correctness-preserving context and orchestration improvements come first.
- Use structured events for both measurement and compact progress rendering, but keep
  the retained event log, terminal presentation, and authoritative final result as
  separate concerns.
- The first-run findings report is source evidence for this investigation; CON-037
  does not absorb unrelated orchestration-boundary defects merely because those
  defects caused expensive retries. It measures their cost and may eliminate only
  redundant invocations whose safe removal is within this item's execution scope.

### Documentation

- Document each captured usage field and its adapter-specific meaning.
- Document prompt-component categories and how exact byte attribution is calculated.
- Explain the difference among prompt bytes, input tokens, cached input tokens, output
  tokens, reasoning-output tokens, and subscription usage allocation.
- Document baseline collection, comparison requirements, degraded adapter behaviour,
  retention, export, and privacy boundaries.
- Document the compact progress view and where complete structured events, diagnostics,
  and final results are retained.
- Publish the measured baseline, implemented reductions, resulting measurements, and
  remaining dominant cost drivers.

### Acceptance criteria

- Every supported real Codex invocation records its role, action, adapter version,
  model, reasoning effort, duration, exact prompt bytes, prompt-component breakdown,
  disposition, and all usage fields reported by Codex.
- A completed `concoct run` summary reports per-action and aggregate usage without
  double-counting, and mechanical actions with no model invocation report zero agent
  usage explicitly or remain clearly excluded.
- `concoct exec inspect` or its documented equivalent shows normalized usage and prompt
  composition for a retained invocation without displaying the full prompt by default.
- Product Owner, Task Planner, initial Developer, remediation Developer, Reviewer, and
  Archivist controlled baselines can be reproduced and compared under named fixed
  execution profiles.
- The baseline report identifies the largest prompt components and repeated context for
  each role and distinguishes exact local measurements from variable adapter usage.
- At least one dominant fixed-role baseline shows a 30 percent or greater reduction in
  exact rendered prompt bytes, or one provably redundant complete agent invocation is
  eliminated, without weakening required context, gates, authority, role independence,
  structured-result validation, or fixture outcomes.
- Before-and-after evidence names every changed prompt component or eliminated
  invocation and explains why the reduction preserves correctness.
- Structured progress continues throughout a long invocation even after earlier
  display history is discarded; full retained events and diagnostics remain available
  within configured bounds.
- Failed, cancelled, and timed-out invocations preserve any available usage and partial
  event evidence without applying stale results or corrupting workflow state.
- Missing or incompatible Codex event and usage support produces a clear diagnostic or
  documented degraded mode rather than fabricated measurements.
- Full automated tests use deterministic fixtures and do not require billable live
  model invocations; an explicitly invoked integration test may validate current Codex
  event compatibility.
