---
version: 1
status: normative-design
roadmap-id: CON-003
updated: 2026-07-29
---

# Concoct command reference

## Git integration commands

`concoct integrate` verifies the clean recorded task branch and archival commit,
preserves the trunk head locally, and squash-integrates into the exact recorded
trunk. `--continue` requires resolved, staged human-selected results. `--abort`
restores the exact preserved trunk head and checks out the task branch. A
matching trunk upstream prompts before push; `git.auto-push: true` in
`.concoct/config.yaml` opts in to automatic push. Missing and non-matching
upstreams remain local success.

## Contract status

This reference defines the implemented command surface and its durable workflow
contract. Role commands render guidance; explicit code/review completion,
Git-backed `plan`, and `integrate` perform the documented mutations.

Workflow state names and transition validity are defined in [state-machine.md](state-machine.md). Project-relative paths in this reference are resolved from the Concoct-enabled project root.

## Shared command behavior

Every command first locates the project or target, parses the artifacts it needs, validates the detected state, and fails without workflow mutation when evidence is malformed, contradictory, or outside the command's valid starting states.

Project-aware workflow commands first validate `.concoct/project.yaml` before
creating output files, task branches, reviews, archives, commits, or integration
state. Missing records are legacy/unversioned and unsupported records are
read-only. `status` and `why` provide reduced compatibility diagnostics without
parsing workflow evidence in those cases. `version` and `defaults` are always
project-independent.

## `concoct version`

Reports the executable product version, source revision, modified state, and
classification. An official release requires a complete valid leading-`v`
SemVer tag, an exact revision, clean source, and release build metadata;
otherwise the binary is labeled `development`. This command has no project
inputs and makes no changes.

Role commands render deterministic, inspectable handoff content. Rendering does not launch an agent and does not prove that role work completed. The `Resulting state` entries below mean the state after the selected role successfully performs the rendered handoff and persists its authorized outputs. Unless an explicit output option is later defined, prompts are written to standard output and are not committed artifacts.

Every role prompt includes the selected persona, exact required context, allowed writes, current state, expected outcome, verification requirements, outgoing handoff, and recommended next transition. Each role command therefore carries both incoming context and outgoing handoff; no separate `handoff` command is part of the happy path.

## Command completeness matrix

| Command | Purpose | Inputs | Start states | Reads | Persona | Writes/effects | Prompt | Result | Failures | Next |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `init` | Bootstrap a project | Project target | `uninitialized` target | Template source and target parent | None | Creates project contract and Git repository | Bootstrap guidance | `ready` | Yes | `roadmap` |
| `status` | Report state | Project root discovery | Any initialized valid or invalid state | State-bearing artifacts | None | None | None | Unchanged | Yes | State-dependent |
| `roadmap` | Prepare Product Owner work | Optional human input supplied outside the command contract | `ready` | Guidance, capabilities, roadmap, archive summaries | Product Owner | CLI: none; role: roadmap only | Product Owner handoff | `ready` | Yes | `plan <id>` or `roadmap` |
| `plan` | Prepare one active task | Roadmap ID | `ready` | Guidance, capabilities, roadmap, history, repository evidence | Task Planner | Git CLI: safe task branch; role: task plan, notes, selected roadmap status | Planner handoff | `planned` | Yes | `code` |
| `code` | Prepare implementation or remediation | Active task | `planned`, `implementation-in-progress`, `review-changes-requested` | Guidance, capabilities, task, notes, applicable reviews, repository | Developer | CLI: none; role: scoped implementation, tests/docs, plan, notes | Developer handoff | `implementation-complete` via in-progress | Yes | `review` |
| `review` | Prepare independent review | Reviewable active task | `implementation-complete` when independent review resolves required | Guidance, capabilities, task, notes, all reviews, diff and relevant files | Reviewer | CLI: none; role: next review only | Reviewer handoff | Outcome-bearing review state | Yes | Outcome-dependent |
| `archive` | Prepare accepted archival | Approved active task or policy-valid externally satisfied review | `review-approved`, or `implementation-complete` when review resolves satisfied | All canonical task, review, product, and implementation evidence | Archivist | Archive and capabilities; Git tasks preserve pending delivery/current evidence | Archivist handoff | Git: `archived`; non-Git: `ready` | Yes | Git: `integrate`; non-Git: `roadmap` or `plan <id>` |
| `integrate` | Integrate accepted Git task | Archived Git task | `archived`, `integrating`, `integrated` | Recorded Git and recovery evidence | None | Squash integration, recovery, delivery bookkeeping, cleanup | None | `ready`, or recoverable `integrating`/`archived` | Yes | State-dependent |

The detailed contracts below expand every matrix cell.

## `concoct init <project>`

### Purpose

Create a new Concoct-enabled project from the installed template while preserving conventional root files, nested files, and dotfiles.

### Required inputs

- Exactly one non-empty `<project>` target name or path.
- A valid destination parent and a resolvable template distribution.
- A target path that does not already exist.

### Valid starting states

The resolved target must be `uninitialized`. Initialization never overlays an existing path or repairs an initialized project.

### Files read

- The distributed project template, including root adapters and `.concoct/` content.
- Any packaged bootstrap-prompt source required by the distribution.
- The destination parent for safety and writability checks.

It does not read a target roadmap or active task because the target does not yet exist.

### Selected persona

None. Initialization is a bootstrap operation, not a workflow role.

### Files created or updated

- The new project directory and the project-owned outputs from the distributed
  template.
- `AGENTS.md` and conventional tool adapters at the project root.
- `.concoct/policy.md`, `.concoct/capabilities.md`, `.concoct/roadmap.md`,
  current and archive lifecycle directories, and the ready-state convention.
- A Git repository without an automatically created commit.
- Bootstrap guidance when the distribution defines it.

The executable supplies immutable built-in protocol, personas, and handoffs at
runtime; initialization does not install mutable copies of those resources.

Whether generated files are staged is an implementation decision reserved for the CLI-foundation task; this contract requires the choice to be deliberate and reported, not a particular choice.

Creation is transactional from the user's perspective. On failure, report the exact partial target. Remove it automatically only when the command can prove it created the target during this invocation and removal is safe; otherwise preserve it for inspection and provide cleanup guidance.

### Prompt produced

Human-readable bootstrap guidance that identifies the `ready` state and recommends `concoct next`. No workflow persona is selected.

### Resulting state

`ready` after the copied artifacts, nested content, dotfiles, Git repository, and bootstrap guidance validate successfully.

### Failure conditions

- Missing or extra required arguments.
- Empty or unsafe normalized project name.
- Existing target path.
- Missing or invalid template distribution.
- Unwritable parent or filesystem failure.
- Copy, Git initialization, or post-copy validation failure.

Every error identifies whether a partial target exists and how to inspect or safely remove it.

### Recommended next commands

- `concoct next` for one evidence-backed Product Owner recommendation.
- `concoct status` to inspect the initialized project.

## `concoct status`

### Purpose

Report repository-backed workflow state and the next valid action without mutation.

### Required inputs

No positional arguments. The command discovers the project root from the current path or an implementation-defined global project option.

### Valid starting states

Every initialized normative state, including `invalid`. Outside a Concoct-enabled project it reports `uninitialized` or a project-discovery error without mutation.

### Files read

- `AGENTS.md` for project identity where needed.
- `.concoct/capabilities.md`.
- `.concoct/roadmap.md`.
- `.concoct/current/task-plan.md` and `notes.md` when populated.
- All `.concoct/current/review-NN.md` files.
- Relevant archive references when needed to diagnose partial archival state.

### Selected persona

None. Status is a read-only reporting operation.

### Files created or updated

None.

### Prompt produced

None. It prints a structured human-readable report containing, when applicable: project name, active roadmap item, normative phase, task status, latest review, review outcome, capability impact, every policy activity's requirement, disposition, reason, and source, diagnostics, and recommended next action. Invalid task evidence is diagnosed and is never echoed as an accepted disposition.

### Resulting state

Unchanged.

### Failure conditions

- Project root cannot be found.
- Required artifacts cannot be read or parsed.
- Evidence is invalid or ambiguous.

Invalid state is a successful diagnostic outcome only when the report can reliably explain the contradictions; operational failures still return an error. Diagnostics name affected files and fields and never repair them automatically.

### Recommended next commands

- `ready`: `concoct next`.
- `planned` or `implementation-in-progress`: `concoct code`.
- `implementation-complete`: `concoct review`, or `concoct archive` when independent review is explicitly non-required or externally satisfied.
- `review-changes-requested`: `concoct code`.
- `review-approved`: `concoct archive`.
- `review-blocked`: responsible-role or human handoff from the review.
- `invalid`: the reported non-destructive recovery action, then `concoct status`.

## `concoct next`

### Purpose

Render a deterministic, read-only Product Owner prompt that recommends one
valid next project action without selecting work or mutating lifecycle evidence.

### Valid starting states

`ready` only. Invalid canonical evidence is diagnosed and no prompt is rendered.

### Evidence and output

The command validates and presents roadmap items, priority, dependencies,
capability prerequisites and limitations, relevant archive provenance, and the
currently supported human-product-input and roadmap-maintenance origins in
stable order. Existing planning eligibility remains authoritative. The prompt
requires exactly one outcome: plan an eligible item, perform supported roadmap
or product intake, resolve a named blocker or inconsistency, or report no
actionable recorded work.

The command writes deterministic bytes to stdout or a create-only `--output`
path and changes no workflow artifact or Git state. Presentation order is not
automatic selection; Product Owner judgment supplies the recommendation.

### Recommended next commands

- `concoct plan <roadmap-id>` for one selected eligible item.
- `concoct roadmap` for supported input or reconciliation.
- The named recovery action for a blocker, or no command when no work exists.

## `concoct roadmap`

### Purpose

Render the complete Product Owner handoff for evaluating human input and maintaining intended future product work.

### Required inputs

No positional arguments. Human product input may be supplied alongside the rendered prompt when used; it is not inferred from repository state.

### Valid starting states

`ready`. The initial contract does not allow roadmap intake to mutate product direction while an active task exists; status or the current role transition should resolve that task first.

### Files read

- `AGENTS.md`.
- The Product Owner persona rendered from the executable.
- `.concoct/capabilities.md`.
- `.concoct/roadmap.md`.
- Relevant `.concoct/archive/**/summary.md` files and project documentation selected deterministically from the input context.

### Selected persona

Product Owner.

### Files created or updated

The command itself updates none. During the role session, the Product Owner may update only `.concoct/roadmap.md` when input is sufficiently understood. It does not create an active task or change capability truth.

### Prompt produced

A Product Owner prompt that includes the exact read set, roadmap ownership, evaluation criteria, prohibition on active-task creation, and the outgoing recommendation to plan an eligible item or continue roadmap work.

### Resulting state

`ready`. An eligible planned roadmap item corresponds to the compact `roadmapped` journey label but does not create a distinct active state.

### Failure conditions

- State is not `ready`.
- Product Owner persona, capabilities, or roadmap is missing or malformed.
- Archive or capability references needed for the supplied input are contradictory.

The error recommends completing the active transition or repairing the named canonical artifact; it does not suggest deleting the task.

### Recommended next commands

- `concoct plan <roadmap-id>` when an item is eligible.
- `concoct roadmap` when product direction still needs work.

## `concoct plan <roadmap-id>`

### Purpose

Render the Task Planner handoff that turns one eligible roadmap item into an implementation-ready active task.

### Required inputs

- Exactly one stable `<roadmap-id>`.
- A matching roadmap item whose status is eligible for planning, whose
  outstanding delivery dependencies are satisfied or explicitly handled, and
  whose capability prerequisites resolve to accepted current truth.

### Valid starting states

`ready`. A populated active task is a conflict and is never overwritten.

### Files read

- `AGENTS.md`.
- The Task Planner persona rendered from the executable.
- `.concoct/capabilities.md`.
- `.concoct/roadmap.md` and the selected item.
- Relevant archive summaries and artifacts.
- Relevant source, tests, and documentation needed to validate roadmap assumptions.

The command structurally validates that every declared capability prerequisite
resolves uniquely to an active capability record. The rendered prompt names the
accepted prerequisites, includes their available archive-summary provenance,
and tells the Task Planner to assess documented limitations for semantic
compatibility with the selected outcome.

### Selected persona

Task Planner.

### Files created or updated

For a safe Git project, the command records the exact source branch and HEAD,
creates and checks out the deterministic collision-free task branch, and emits
those values. Rendering/output failure rolls the checkout and new branch back.
For non-Git projects it remains non-mutating. During successful planning, the
Task Planner creates or replaces only empty placeholders with:

- `.concoct/current/task-plan.md`;
- `.concoct/current/notes.md`.

The plan preserves the roadmap ID, declares valid task status and capability impact, and has observable acceptance criteria. The selected roadmap item moves to `active` only after both active artifacts validate. No other roadmap item changes.

### Prompt produced

A Task Planner prompt containing the selected roadmap item, repository evidence to inspect, readiness test, permitted artifacts, prohibition on implementation and product invention, and outgoing Developer handoff requirements.

### Resulting state

`planned` after both active artifacts and the roadmap status agree. If the planner finds unresolved product ambiguity or an unsatisfied dependency, no active task is established and state remains `ready`.

### Failure conditions

- Missing, extra, or syntactically invalid roadmap ID.
- Unknown, delivered, cancelled, deferred, blocked, already active, or otherwise ineligible item.
- Unsatisfied delivery dependency; malformed, duplicate, missing, or inactive
  capability prerequisite; or a documented capability limitation the Task
  Planner determines is incompatible with the outcome.
- Existing populated active task or current reviews.
- Malformed capability, roadmap, archive, or placeholder evidence.
- Planner outcome is not implementation-ready.

Errors identify the item status or active conflict and recommend Product Owner work, dependency delivery, or completion of the existing task. They never overwrite active artifacts.

### Recommended next commands

- `concoct code` after successful planning.
- `concoct roadmap` when product clarification or eligibility work is required.

## `concoct code`

### Purpose

Render the Developer handoff for initial implementation, continuation, or remediation of review findings.

### Required inputs

No positional arguments. A valid active task, notes, and all context required by the detected mode must exist.

### Valid starting states

- `planned` for initial implementation.
- `implementation-in-progress` to resume interrupted or explicitly incomplete work.
- `review-changes-requested` for remediation.

`review-blocked` is not directly accepted. After the responsible handoff, `code` becomes valid when task-plan metadata contains a valid `blocked-review-resolution` for the exact latest blocked review with route `code`, the cited durable evidence resolves, and task status is `implementation-in-progress` as defined by the state machine.

### Files read

- `AGENTS.md`.
- The Developer persona rendered from the executable.
- `.concoct/capabilities.md`.
- `.concoct/current/task-plan.md` and `notes.md`.
- The latest review in remediation mode and prior reviews as needed for finding history.
- Relevant source, tests, documentation, and referenced archives.

### Selected persona

Developer. In `review-changes-requested`, the prompt explicitly selects review-remediation mode.

### Files created or updated

Plain `concoct code` updates nothing. During the role session, the Developer may update source, tests, task-required documentation, the task plan's technical details and phase statuses, and notes. The Developer must not update roadmap, capabilities, archived artifacts, or completed reviews. `concoct code --complete` validates that both task plan and notes changed and requires a complete fresh outgoing handoff. When independent review remains unresolved, this is `## Handoff to reviewer` with `### Suggested review focus`. When policy resolves review as explicitly non-required or externally satisfied, it is `## Handoff to archivist` with `### Suggested archive focus`. The final selected handoff section must differ from the version committed at `HEAD`; unrelated notes edits cannot reuse stale evidence. Completion also rejects forbidden workflow paths and validates the resulting state before committing one Git-backed transition. A clean retry reuses that exact completion commit. Non-Git tasks have no committed baseline to compare, so their artifact-level rule requires the selected final handoff section and all required headings in the current notes.

Work begins by persisting task status `implementation-in-progress`. In remediation mode the task metadata also sets `remediates-review` to the exact latest `changes-requested` review. Successful completion records `implementation-complete`, verification evidence, and the outgoing handoff selected by the resolved review disposition. Remediation records each finding as fixed, partially fixed, disputed with evidence, obsolete, or blocked; a completed remediation without dispositions for every finding is invalid.

### Prompt produced

A Developer prompt with exact scope, mode, required reads, allowed writes, constraints and non-goals, verification, finding-disposition requirements when applicable, and the resolved outgoing Reviewer or Archivist handoff.

### Resulting state

`implementation-complete` after successful role completion. Interrupted or deliberately unfinished work remains `implementation-in-progress`. A newly discovered product ambiguity is recorded and escalated without being represented as complete.

### Failure conditions

- No valid active task or required notes.
- Starting state is not one of the allowed states.
- Task metadata and roadmap identity disagree.
- Remediation review is missing, malformed, not latest, or has a different outcome.
- Blocked-review resolution is missing, names a non-latest or non-blocked review, lacks resolvable evidence, selects a route other than `code`, or was recorded by an unauthorized role.
- Plan has unresolved product ambiguity that prevents implementation.

Errors name the missing evidence or correct next role. Prior reviews remain unchanged.

### Recommended next commands

- `concoct code --complete` after authoring a complete Developer transition.
- The resolved next command after completion: `concoct review` while review remains required, or `concoct archive` when review is explicitly non-required or externally satisfied.
- `concoct code` to resume persisted in-progress work.
- The responsible role or human handoff when work is blocked by scope, product intent, or authorization.

## `concoct review`

### Purpose

Render the independent Reviewer handoff and allocate the next append-only review sequence.

### Required inputs

No positional arguments. The active task must be implementation-complete with a Developer handoff and sufficient verification evidence for review.

### Valid starting states

`implementation-complete`, including completion following a remediation cycle or a valid `blocked-review-resolution` for the exact latest blocked review with route `review`. No existing review may determine another actionable state.

### Files read

- `AGENTS.md`.
- The Reviewer persona rendered from the executable.
- `.concoct/capabilities.md`.
- `.concoct/current/task-plan.md` and `notes.md`.
- All prior `.concoct/current/review-NN.md` files.
- The complete diff and relevant source, tests, documentation, and archive evidence.

### Selected persona

Reviewer.

### Files created or updated

Plain `concoct review` updates nothing and identifies the next sequence.
`concoct review --reserve` exclusively creates that exact path with recognizable
`reserved` metadata, but only after validating the recorded Git trunk and base,
attached task branch, clean worktree, and absence of another Git operation. A
reservation is incomplete and does not advance state.
The Reviewer replaces the reserved status with one supported outcome and adds
the required review evidence. `concoct review --complete` accepts only that one
review path, validates its matching task, number, persona, date, and exactly one
documented outcome, then commits one Git-backed review transition. Prior reviews
and Developer-owned files remain unchanged, and a clean retry reuses the exact
review commit.

Malformed or abandoned reservations are preserved for inspection. Restore the
generated reservation or complete it; Concoct never overwrites or renumbers it.

### Prompt produced

A Reviewer prompt containing the complete review inputs, acceptance contract, prior-finding context, required evidence quality, append-only output path, exactly-one-outcome rule, and outcome-specific handoffs.

### Resulting state

- `review-approved` for `approved`.
- `review-changes-requested` for `changes-requested`.
- `review-blocked` for `blocked`.

### Failure conditions

- Task is absent, in progress, or lacks a completion handoff.
- Task or notes are malformed.
- Review sequence is gapped, conflicting, malformed, or collision-prone.
- An existing latest review already determines the state.
- Blocked-review resolution is missing, names a non-latest or non-blocked review, lacks resolvable evidence or a fresh reviewer handoff, selects a route other than `review`, or was recorded by an unauthorized role.
- Required diff or repository evidence cannot be inspected sufficiently to perform review.

The error preserves all reviews and recommends implementation completion, sequence repair by the artifact owner, or restoration of missing evidence.

### Recommended next commands

- Approved: `concoct archive`.
- Changes requested: `concoct code`.
- Blocked: the responsible persona or human identified by the review. After owned resolution evidence and valid task-plan resolution metadata exist, run `concoct code` or `concoct review` according to the recorded route; there is no universal immediate command.

## `concoct archive`

### Purpose

Render the Archivist handoff and coordinate the acceptance boundary. Non-Git
tasks return directly to `ready`; Git-backed tasks stop at `archived` pending
integration.

### Required inputs

No positional arguments. Required evidence includes a valid active task and notes, the complete sequential review history, latest outcome `approved`, resolved capability impact, matching roadmap metadata, accepted implementation, verification evidence, and an unused dated archive destination.

### Valid starting states

`review-approved` for ordinary completion. The exceptional completion form
requires both `--override-authority <authority>` and `--override-reason
<reason>` and exactly matching non-empty `override` summary metadata.

### Files read

- `AGENTS.md`.
- The Archivist persona rendered from the executable.
- `.concoct/capabilities.md` and `.concoct/roadmap.md`.
- `.concoct/current/task-plan.md`, `notes.md`, and all review files.
- Latest approved review.
- Relevant implementation, tests, documentation, and verification evidence.
- Existing archive destinations and cross-references.

### Selected persona

Archivist.

### Files created or updated

After the rendered handoff is carried out, archival performs one ordered transaction:

1. create `.concoct/archive/YYYY-MM-DD-roadmap-id-short-task-name/`;
2. copy the accepted task plan, notes, every review, and other approved durable task artifacts;
3. create and validate `summary.md` with task identity, delivered outcome, decisions, files changed, checks, approving review, capability impact, skipped work, and follow-ups;
4. reconcile `.concoct/capabilities.md` from accepted behavior;
5. add archive/capability cross-references and validate reconciled records;
6. for Git-backed tasks, commit archive evidence on the task branch, record its
   hash and pending delivery, and preserve current/active evidence;
7. for non-Git tasks, mark delivery and clear current only after validation.

Non-Git summaries must record the exact lifecycle pair `status: delivered` and
`delivery: complete`; contradictory or missing lifecycle evidence is rejected
before current-task cleanup.

Plain `concoct archive` remains read-only prompt rendering. The Archivist
authors the candidate transaction and invokes `concoct archive --complete`.
Git metadata uses `archive-commit: self`, a non-recursive committed sentinel
that resolves to the exact archival HEAD; clean retries reuse that commit.

Source, tests, accepted task history, completed reviews, and unrelated roadmap items are never rewritten by archival.

### Prompt produced

An Archivist prompt with all preconditions, exact artifact ownership, transactional order, summary requirements, capability reconciliation rules, validation steps, and outgoing Product Owner or next-planning handoff.

### Resulting state

Git-backed: `archived` after the archival commit and pending-delivery evidence
validate. Non-Git: `ready` after delivery and current reset validate.

### Failure conditions

- Latest review is missing, malformed, `changes-requested`, or `blocked`.
- Task, roadmap, review, or capability identifiers disagree.
- Capability impact is absent, unresolved, unsupported, or contradicted by accepted behavior.
- Required artifacts, implementation, verification, or approval evidence is missing.
- Archive destination exists or is unsafe.
- An override is partial, does not exactly match durable summary evidence, or
  is supplied for an ordinarily approved task.
- Copy, summary creation, capability reconciliation, roadmap update, cross-reference validation, or current reset fails.

Before current reset, every failure preserves `.concoct/current/`. A failure after any durable archive write reports the repository as `invalid`, enumerates completed and pending transaction steps, and directs the Archivist to reconcile forward without deleting evidence.

### Recommended next commands

- Git-backed: `concoct integrate`.
- Non-Git: `concoct next`.

- `concoct next` to obtain the next evidence-backed recommendation.
- `concoct status` to confirm the returned `ready` state.

## Commands outside the initial surface

`concoct handoff`, `concoct abandon`, and `concoct doctor` are optional future ideas. No happy path, remediation loop, blocker route, invalid-state diagnostic, or archive recovery in this contract depends on them.
