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
contract. Role commands render guidance; `exec` supervises one authorized
action and `run` composes fresh authorized actions within finite gates and
bounds. Explicit role completion, shared Git-backed planning setup, archival
completion, and integration remain the mutation authorities.

Workflow state names and transition validity are defined in [state-machine.md](state-machine.md). Project-relative paths in this reference are resolved from the Concoct-enabled project root.

## Shared command behavior

Every command first locates the project or target, parses the artifacts it needs, validates the detected state, and fails without workflow mutation when evidence is malformed, contradictory, or outside the command's valid starting states.

## Structured orchestration contract

Concoct has an executable-owned, transport-neutral JSON protocol (`v1`) for one
adapter to receive one authorized action and return one claimed outcome.
`concoct exec` supervises one use of this boundary; manual prompts and explicit
completion commands remain fully supported.

An action carries an unpredictable invocation and action identity, task and
attempt correlation, selected role, action kind, gate, human-readable
explanation, and a bounded digest of the workflow evidence from which it was
authorized. The registry covers Product Owner next-action and roadmap work,
Task Planner work, development, independent review, archival, and Git
integration. Every entry declares its role, executable authority, explicit
preconditions, permitted effects, supported outcome classes, bounded
intervention route, and completion validator with observable completion states.

Adapters report exactly one `completed`, `blocked`, `decision-required`,
`failed-recoverable`, or `failed-terminal` outcome. The result must echo all
correlation fields and include a concise summary; optional artifact references,
intervention guidance, and diagnostics are bounded and sanitized. For a
process adapter, Concoct provides an invocation-specific result boundary and
normalizes at most one result with an atomic no-replace operation. Standard
output, standard error, process exit status, prompts, logs, and runtime records
are diagnostics, never workflow evidence or durable task history.

Concoct validates a claimed result against the action registry, correlation,
evidence freshness, current artifact state, workflow state, and repository
state. Executable authority and observed state win over an agent claim. A
`completed` result is rejected unless an allowed observable postcondition is
actually present; malformed, duplicate, stale, mismatched, unsupported, or
contradicted results cannot advance the workflow. Ready-state evidence can
authorize only a Product Owner decision: it never autonomously selects a
roadmap item. A successful ready-state decision instead carries one bounded
`plan`, `roadmap`, `blocker`, or `no-action` recommendation and stops.

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
| `init` | Bootstrap a project | Project target | `uninitialized` target | Template source and target parent | None | Creates project contract and Git repository | Bootstrap guidance | `ready` | Yes | `next` |
| `status` | Report state | Project root discovery | Any initialized valid or invalid state | State-bearing artifacts | None | None | None | Unchanged | Yes | State-dependent |
| `exec` | Execute one recommendation | Optional one-run profile overrides | One unambiguous executable state recommendation | Complete action evidence and exact manual prompt core | State-selected role, or direct integration | One adapter process at most; private ignored runtime record | Manual prompt core plus fixed supervision appendix | Claimed outcome finalized and reconciled with observed state | Yes | Reported and never auto-run |
| `run` | Coordinate one bounded lifecycle | Optional current-gate approval, added gates, lower bounds, and profile overrides | `ready` or an ordinary actionable task state | Fresh action, workflow, policy, configuration, Git, and pending-gate evidence per iteration | Fresh state-selected role per action | Consecutive canonical role transitions; one private pending gate; local-only integration | Manual prompt core plus fixed supervision appendix per role | Integrated/ready or bounded gate/intervention summary | Yes | Exact safe continuation when available |
| `roadmap` | Prepare Product Owner work | Optional human input supplied outside the command contract | `ready` | Guidance, capabilities, roadmap, archive summaries | Product Owner | CLI: none; role: roadmap only | Product Owner handoff | `ready` | Yes | `plan <id>` or `roadmap` |
| `plan` | Prepare one active task | Roadmap ID | `ready` | Guidance, capabilities, roadmap, history, repository evidence | Task Planner | Git CLI: safe task branch; role: task plan, notes, selected roadmap status | Planner handoff | `planned` | Yes | `code` |
| `code` | Prepare implementation or remediation | Active task | `planned`, `implementation-in-progress`, `review-changes-requested` | Guidance, capabilities, task, notes, applicable reviews, repository | Developer | CLI: none; role: scoped implementation, tests/docs, plan, notes | Developer handoff | `implementation-complete` via in-progress | Yes | `review` |
| `review` | Prepare independent review | Reviewable active task | `implementation-complete` when independent review resolves required | Guidance, capabilities, task, notes, all reviews, diff and relevant files | Reviewer | CLI: none; role: next review only | Reviewer handoff | Outcome-bearing review state | Yes | Outcome-dependent |
| `archive` | Prepare accepted archival | Approved active task or policy-valid externally satisfied review | `review-approved`, or `implementation-complete` when review resolves satisfied | All canonical task, review, product, and implementation evidence | Archivist | Archive and capabilities; Git tasks preserve pending delivery/current evidence | Archivist handoff | Git: `archived`; non-Git: `ready` | Yes | Git: `integrate`; non-Git: `next` |
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

## `concoct exec`

### Purpose and authorization

`concoct exec` resolves the one typed action currently authorized by workflow
state and policy. Ready state authorizes only the Product Owner `next` decision;
implementation states authorize Developer work; implementation-complete
authorizes Reviewer or Archivist work according to policy; approved authorizes
Archivist work; archived authorizes the existing direct integration operation.
Invalid, blocked, informational, recovery-choice, and human-gated states refuse
with one evidence-backed reason. Flags cannot coerce a different action.

`--dry-run` resolves the action, exact prompt, adapter, role, model, reasoning,
timeout, value provenance, command, and safety posture. It verifies adapter
availability but starts no process and creates no workflow, Git, or runtime
evidence. Real execution launches at most one adapter process and returns after
one outcome; it never retries or starts the reported next command.

The built-in `codex` adapter runs non-interactively in the exact project root,
receives the byte-identical manual role prompt as the leading semantic core on
standard input followed by a fixed executable supervision appendix, uses the
`workspace-write` sandbox without bypass flags, ignores Codex user configuration
while retaining project rules and authentication, and receives an invocation-
specific output schema containing the authorized correlation. The adapter
inherits only an allowlist of operational and authentication variables; the
environment is never retained. The appendix directs supervised role agents to
author and verify candidates without writing Git metadata or invoking final
completion. The outer executable accepts only a correlated completed candidate
with role-owned effects, calls the existing completion authority, and creates
the transition commit. Valid non-completion work is preserved but not committed;
forbidden effects and adapter-created commits are rejected. Direct Git
integration uses the existing integration authority instead of manufacturing
an agent-only prompt.

### Configuration

The command accepts `--adapter`, `--model`, `--reasoning`, and `--timeout` for
one invocation. Profile values resolve in this order: invocation override,
project role configuration, user role configuration, adapter role default,
then adapter default. Adapter selection uses the invocation override, project
configuration, user configuration, then the built-in `codex` default.

Project configuration is `.concoct/config.yaml`. User configuration is
`concoct/config.yaml` below the platform user-configuration directory. Both use
strict YAML; unknown fields, roles, adapters, reasoning values, unsafe model
values, invalid durations, and invalid retention bounds fail before launch.
The `exec.roles.<role>` mapping accepts `adapter`, `model`, `reasoning`, and
`timeout`. `exec.retention` accepts `max-completed`, `max-age`,
`max-log-bytes`, and `max-total-bytes`. Defaults are 20 completed attempts, 14
days, 256 KiB per log, and 20 MiB total retained bytes. Existing
`git.auto-push` behavior remains supported.

### Runtime records, failure, and inspection

Every launched attempt creates a mode-0700 directory below the Git-ignored
`.concoct/runtime/invocations/`. Mode-0600 files retain the exact prompt,
action, schema, sanitized resolved metadata (including the adapter's
self-reported version when its post-authorization probe succeeds), bounded
redacted structured stdout, bounded redacted stderr, normalized result when present,
prompt composition, native usage evidence, and final reconciliation. Runtime
records do not establish lifecycle completion.
When an action was authorized through a gate created by a prior invocation, its
metadata also records that predecessor invocation ID. This is observational
linkage only; it grants no replay or retry authority.

Prompt composition is an ordered, byte-conserving manifest: every emitted byte
belongs to a generated-context, policy disposition, capability or roadmap
evidence, persona, instruction-provenance, input-reference, authorized-update,
completion-contract, or handoff component. Each component names its source,
inclusion mode, exact byte count, and bounded exact/whitespace-normalized
duplicate group when applicable. A path reference is recorded as an input
reference; it does not claim that the file's contents were embedded.

The Codex adapter requests JSONL events. Structured events are decoded while
the process runs, independently of retained-output and display limits: a late
terminal usage event remains attributable even when earlier progress has
exhausted the retained structured-event budget. Progress is an allowlisted,
bounded lifecycle-label representation; event payload text is never copied to
progress display or measurement. `input_tokens`,
`cached_input_tokens`, `output_tokens`, `reasoning_output_tokens`, and
`total_tokens` are adapter-reported optional native fields. Reported zero is
distinct from unavailable; cached input remains separate; Concoct never
constructs a total or treats display text as usage authority. Diagnostic count
and bytes are independently capped. An unterminated event is capped at the
same configured evidence-byte ceiling; an oversized event is discarded until
the next JSONL boundary, with `event_truncated`, `diagnostics_truncated`, and
`degraded` evidence recording loss where applicable. Parsing then resumes, so
later valid usage remains observable. Malformed, duplicate, contradictory,
missing, or out-of-order usage events become measurement diagnostics. Events
after a terminal `turn.*` event mark
the measurement `status` as `degraded`; valid usage seen before that point is
preserved as partial observation and does not change structured workflow-result
acceptance.

Cancellation and timeout terminate the adapter process group with bounded
grace before force. They also interrupt direct integration Git work and push
confirmation while preserving the existing recovery record at a safe local
transaction boundary. Both paths close result acceptance and still reconcile
actual state.
Startup failure, nonzero exit, missing or malformed output, duplicate delivery,
stale correlation, evidence drift, and postcondition mismatch all stop the
one-shot invocation and report adapter disposition separately from observed
state. A retry always authorizes a fresh envelope from current evidence.

`concoct exec inspect [<invocation-id>]` is metrics-first: it shows metadata,
component evidence, normalized native usage, result, and reconciliation but
not the prompt or raw logs. `--full-raw` explicitly includes the private prompt
and retained structured-event/stderr diagnostics for recovery. `--json` emits a
stable metrics-only JSON comparison record; it excludes prompt bytes, raw
events, diagnostics, and progress text. With no ID inspection selects the most recent attempt;
missing pieces are reported as partial rather than regenerated. Prompt bytes
are deterministic local attribution. Native token fields are only values
reported by Codex: cached input is never subtracted and missing totals are not
manufactured. The concise one-shot and run output reports the same distinction;
direct integration is identified as a no-agent mechanical action.

## `concoct run`

### Purpose and authorization

`concoct run` repeatedly re-detects state and prepares one fresh action through
the same structured executor until the selected task is locally delivered or a
gate, intervention, cancellation, unsafe state, failure, no-progress condition,
or bound stops the invocation. It never invokes `concoct exec` recursively and
does not infer completion from process output.

Ready state runs only Product Owner judgment. A valid `plan` recommendation is
stored as a bounded proposal and always stops at `next`; only
`--approve next` may establish its still-eligible planning context. The shared
planning operation gives manual and coordinated planning identical eligibility,
branch, base, and prompt behavior. Planned work stops at `plan` by default.
`--approve plan` authorizes only the exact pending plan. Archived work likewise
stops until `--approve integration`, after which integration is local-only.

Each approval names exactly one current gate and one forthcoming action
occurrence. Missing, wrong, replayed,
malformed, or evidence-stale approval refuses before the protected operation.
The private mode-0600 `.concoct/runtime/pending-gate.json` record is bounded,
atomic, Git-ignored, and consumed once; it is not workflow state or resumable
run history. If another configured gate protects that same action, the next
record carries the consumed prerequisite approval. It never carries authority
to a later remediation or review occurrence, which must stop for a fresh gate
and attempt correlation.

### Policy, bounds, and isolation

Built-in gates are `plan` and `integration`; `next` is invariant. Strict
`run.gates` configuration and repeated `--gate` flags may add only
`development`, `review`, or `archive`. Gates compose by union. `run.max-actions`
and `run.max-cycles`, plus matching invocation flags, may lower but never raise
the hard limits of 20 actions and three completed Reviewer actions. Unknown or
duplicate gates, unknown configuration, raised bounds, and non-positive bounds
fail before runtime evidence.

Every iteration freshly resolves workflow, policy, configuration, role,
adapter, prompt, and authorization. Every Reviewer receives an independent
ephemeral adapter invocation. A changes-requested outcome advances the material
fingerprint and may continue through remediation; repeating an action with the
same fingerprint stops. Recoverable execution failure is reported without an
automatic retry.

### Stop summary and compatibility

The bounded summary lists attempted actions and accepted outcomes, action and
cycle usage, final workflow state, pending gate or intervention, and an exact
continuation when one is safe. Cancellation, blocked or decision outcomes,
invalid and integration-recovery states, failures, and exhausted bounds always
stop. Manual prompts and completion commands, `exec`, `status`, review
immutability, archival validation, and manual integration remain available.
Manual integration retains its configured push behavior; `run` never pushes.

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
For Git tasks, the candidate task plan remains byte-identical to accepted
active evidence. Completion alone applies `git.status: archived` and
`git.archive-commit: self` to the current task before committing. The
non-recursive sentinel resolves to the exact archival HEAD; clean retries
reconstruct and validate that exact current-only metadata transition against
the immutable parent before reusing the commit.

Source, tests, accepted task history, completed reviews, and unrelated roadmap items are never rewritten by archival.

Capability reconciliation compares canonical `CAP-NNN` records by identifier.
Blank lines between records, LF versus CRLF line endings, and a final newline
are formatting-only; all record content, record order, and ledger-level content
remain protected. Malformed or duplicate capability records are rejected before
archive mutation.

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
