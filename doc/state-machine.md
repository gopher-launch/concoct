---
version: 1
status: normative-design
roadmap-id: CON-003
updated: 2026-07-29
---

# Concoct workflow state machine

Git-backed approved tasks extend the terminal path:

```text
review-approved → archive → archived → integrate → integrated → ready
                                      ↘ integrating
                                        ├─ --continue → integrated → ready
                                        └─ --abort → archived
```

`ready` is reported only after integration, delivery bookkeeping, current
cleanup, task-branch deletion, and recovery cleanup succeed.

## Scope and status

This document is the normative contract for the implemented Concoct workflow
state and transitions. Artifact evidence remains authoritative; local Git
recovery evidence is authoritative only while integration is incomplete.

State is derived only from repository artifacts rooted at the project directory. Conversation history, an agent's claim, a generated prompt, and an unpersisted decision are not state evidence.

## Canonical evidence

State detection reads the smallest set of artifacts needed to establish a phase and validate its consistency:

- `AGENTS.md` identifies a project through its repository-owned conventional entry point; `.concoct/protocol.md` and `.concoct/policy.md` provide the owned workflow layers.
- `.concoct/capabilities.md` records accepted current behavior.
- `.concoct/roadmap.md` records roadmap items, their stable identifiers, and their statuses.
- `.concoct/current/task-plan.md` identifies the active task through `id`, `roadmap-id`, `status`, and `capability-impact` metadata. During remediation it also uses `remediates-review` to name the latest `changes-requested` review being addressed. After a blocked review, a `blocked-review-resolution` mapping names the exact blocked review, the authorized recorder, the resolution evidence, and whether the task returns to `code` or `review`.
- `.concoct/current/notes.md` is required durable task context, but narrative text in it does not override structured task or review state.
- `.concoct/current/review-NN.md` files record sequential review attempts. Each completed review has one outcome: `approved`, `changes-requested`, or `blocked`.
- `.concoct/archive/*/` records completed tasks. Archive contents support validation and history reporting but do not make an active task current.

Empty tracked placeholders do not count as active artifacts. A parser must distinguish a documented placeholder from a populated artifact and must reject partially populated task or review files as malformed.

## Normative states

The states below are mutually exclusive. `invalid` is a detected condition, not a workflow milestone.

| State | Observable evidence | Normal next action |
| --- | --- | --- |
| `uninitialized` | The target is absent, or it lacks the required Concoct project contract created by initialization. | `concoct init <project>` |
| `ready` | The project contract is valid; no populated active task or current review exists; roadmap and capability artifacts are readable and internally consistent. | `concoct next` |
| `planned` | A valid task plan and notes exist; the plan maps to one eligible roadmap item; task status is `planned`; no review exists. | `concoct code` |
| `implementation-in-progress` | The active task status is `implementation-in-progress`; required task artifacts are valid; after review, either `remediates-review` names the latest `changes-requested` review or a valid `blocked-review-resolution` names the latest `blocked` review and selects `code`. | `concoct code` to continue or resume |
| `implementation-complete` | The active task status is `implementation-complete`; required task artifacts are valid; either no review exists, `remediates-review` names the latest `changes-requested` review and notes contain its completed finding dispositions, or a valid `blocked-review-resolution` names the latest `blocked` review and selects `review`. | `concoct review` |
| `review-changes-requested` | The highest valid sequential review is complete with outcome `changes-requested` and matches the active task. | `concoct code` |
| `review-approved` | The highest valid sequential review is complete with outcome `approved`, matches the active task, and archive prerequisites including resolved capability impact are present. | `concoct archive` |
| `archived` | A Git-backed approved task has committed archive evidence and a recorded archival commit; delivery and current cleanup remain pending. | `concoct integrate` |
| `integrating` | Local recovery evidence preserves the task, trunk, archival commit, and pre-integration trunk head while a squash result is incomplete. | `concoct integrate --continue` or `--abort` |
| `integrated` | The squash commit exists and final delivery bookkeeping remains recoverable. | `concoct integrate --continue` |
| `review-blocked` | The highest valid sequential review is complete with outcome `blocked` and matches the active task. | Route the recorded blocker to its responsible role or human. |
| `invalid` | Required evidence is malformed, missing, contradictory, unsafe to interpret, or shows a partially completed transition. | Follow the diagnostic recovery instructions; do not mutate workflow state automatically. |

The task status vocabulary is `planned`, `implementation-in-progress`, and `implementation-complete`. Review outcomes are stored in review artifacts and do not overwrite task history.

## Roadmap vocabulary mapping

The roadmap's compact diagram is a user-journey projection, not an alternative detection algorithm:

| Compact label | Normative interpretation |
| --- | --- |
| `uninitialized` | `uninitialized` |
| `ready` | `ready` |
| `roadmapped` | Still `ready`, with at least one eligible roadmap item. Roadmap editing alone creates no active-task phase. |
| `planned` | `planned` |
| `implemented` | `implementation-complete`; `implementation-in-progress` is the durable intermediate state. |
| `reviewed` | One of `review-changes-requested`, `review-approved`, or `review-blocked`. |

This mapping preserves the intended journey while retaining enough evidence to recommend a deterministic next action.

## Detection and precedence

State detection follows this order:

1. Locate the project root and required Concoct contract. If it is absent, report `uninitialized`.
2. Parse and validate roadmap, capabilities, active task, notes, and review metadata that exist. Any unsafe or contradictory evidence yields `invalid`.
3. If no populated active task exists, require no current reviews and report `ready`.
4. Validate that task `id` and `roadmap-id` identify the same active roadmap item, required notes exist, and capability impact uses `add`, `update`, `remove`, or `none`.
5. Validate reviews as a contiguous sequence beginning with `review-01.md`. The highest sequence number is the latest review; filenames, internal review numbers, task identifiers, and outcomes must agree.
6. If a latest completed review exists, its outcome determines the review state unless valid remediation evidence names that exact `changes-requested` review or valid blocker-resolution evidence names that exact `blocked` review. A review may not coexist with task status `planned`; an `approved` review cannot be bypassed, and a blocked review can be superseded only by the resolution contract below.
7. Valid changes-requested remediation evidence, or valid blocked-review resolution evidence, plus the corresponding task status selects `implementation-in-progress` or `implementation-complete`. Otherwise, use task status only when no review exists.

Review precedence does not permit stale approval to survive remediation. Before starting remediation, the Developer changes task status to `implementation-in-progress` and sets `remediates-review` to the exact latest `changes-requested` filename. The field is invalid for an approved, blocked, missing, non-latest, or different-task review. On completion, the Developer records all finding dispositions and sets `implementation-complete`. A later review must use the next sequence number; once it exists, the stale remediation reference is ignored for detection and should be removed during the next owned task-plan update.

### Blocked-review resolution evidence

A blocked review remains authoritative until the responsible handoff produces owned evidence and the Task Planner or Developer records this mapping in task-plan front matter:

```yaml
blocked-review-resolution:
  review: review-NN.md
  route: code | review
  recorded-by: task-planner | developer
  evidence:
    - repository-relative artifact reference
```

The mapping is valid only when `review` is the exact latest review, that review has outcome `blocked` for the active task, `evidence` is a non-empty YAML sequence of project-root-relative file paths, every path names an existing readable file (not a directory) recording the decision or completed work, and `recorded-by` owns both the task-plan update and the selected route's readiness judgment. Absolute paths, path traversal, globs, and URI or fragment syntax are invalid. The notes must summarize the blocker, its disposition, the originating Product Owner, Task Planner, Developer, or human decision, and the cited evidence; notes alone never supersede the review.

The supported routes are:

- `code`: task status must be `implementation-in-progress`. The Task Planner may record this after repairing a defective plan, or the Developer may record it when the blocked review assigns missing in-scope implementation, tests, documentation, or evidence. Product Owner or human decisions that require implementation first flow through the Task Planner when they change task intent, or through the Developer when they merely unblock already-scoped work.
- `review`: task status must be `implementation-complete`, the cited owned artifacts must contain the completed resolution, and notes must contain a fresh reviewer handoff. The Task Planner may record this after a plan-only correction that leaves implementation complete; the Developer may record it after completing already-scoped work. A Product Owner or human decision must first be incorporated into the task by the Task Planner or Developer, according to artifact ownership.

The mapping is invalid for an approved, changes-requested, missing, non-latest, or different-task review; for an unsupported recorder; for an unresolved evidence reference; or when its route and task status disagree. A later review supersedes the mapping for detection, but the mapping remains task history until an authorized task-plan update removes it. This resolution does not change the blocked review's outcome, treat it as approval, or permit archival.

## Transition model

A role command has two moments:

1. The command validates the starting state and deterministically renders an inspectable handoff for its selected persona. Rendering is state-preserving.
2. The selected human or agent performs the authorized role work and persists the outgoing handoff. That completed work establishes the resulting state.

Directly launching or supervising an agent is outside this initial contract. A generated prompt is not proof that its work completed.

| Starting state | Command | Selected persona or operation | State after successful role work |
| --- | --- | --- | --- |
| `uninitialized` | `init <project>` | Bootstrap operation; no workflow persona | `ready` |
| `ready` | `roadmap` | Product Owner | `ready` |
| `ready` | `plan <roadmap-id>` | Task Planner | `planned` |
| `planned` | `code` | Developer | `implementation-in-progress`, then `implementation-complete` |
| `implementation-in-progress` | `code` | Developer | `implementation-in-progress`, then `implementation-complete` |
| `implementation-complete` | `review` | Reviewer | One review outcome state |
| `review-changes-requested` | `code` | Developer in remediation mode | `implementation-in-progress`, then `implementation-complete` |
| Latest blocked review, resolved with route `code` | responsible-role handoff, then `code` | Task Planner or Developer records resolution; Developer implements | `implementation-in-progress`, then `implementation-complete` |
| Latest blocked review, resolved with route `review` | responsible-role handoff, then `review` | Task Planner or Developer records completed resolution; Reviewer reviews | One review outcome state |
| `implementation-complete` after remediation | `review` | Reviewer using the next sequence | One review outcome state |
| `review-approved` | `archive`, then `archive --complete` | Archivist + CLI validation | Git: `archived`; non-Git: `ready` |
| `archived` | `integrate` | CLI transaction | `ready` or `integrating` |
| `integrating` | `integrate --continue` / `--abort` | Human + CLI transaction | `ready` / `archived` |
| Any initialized valid state | `status` | Read-only reporting; no persona | Unchanged |

`roadmap` may make an item eligible for planning, but because no active task exists, the detected state remains `ready`. `plan` may be invoked directly from `ready` when the named item is already eligible; a separate `roadmap` invocation is not required.

Planning eligibility treats outstanding roadmap dependencies and accepted
capability prerequisites as distinct evidence. Every declared capability
prerequisite must resolve uniquely to an `active` capability record before the
planning prompt is rendered. Structural validation does not decide whether a
documented capability limitation is compatible with the desired outcome; that
readiness judgment remains with the Task Planner using the limitation and
archive context named by the prompt.

### Remediation loop

The repeatable loop is:

```text
implementation-complete
  → review-NN: changes-requested
  → review-changes-requested
  → code
  → implementation-in-progress
  → implementation-complete
  → review-(NN+1)
  → changes-requested | approved | blocked
```

All completed reviews remain append-only. Remediation records every prior finding's disposition in notes and never edits an earlier review.

### Blocked review routing

`review-blocked` is neither rejection nor approval. The latest review must identify its blocker, evidence, required decision or work, and responsible destination:

- Product Owner for product behavior, scope, priority, or capability intent;
- Task Planner for a contradictory, oversized, incomplete, or unreviewable plan;
- Developer for missing implementation, tests, documentation, or evidence within scope;
- a human decision-maker for authorization or external knowledge.

Resolving a blocker is an explicit handoff under the responsible role. Product Owner and human decisions are first persisted in their owned or designated durable evidence, then incorporated by the Task Planner or Developer under the resolution contract above. Until that complete evidence exists, detection remains `review-blocked`. A valid `code` resolution selects `implementation-in-progress`; a valid `review` resolution selects `implementation-complete`. There is no universal automatic next command and no initial `handoff` command is required.

## State-preserving operations

- `status` never changes workflow artifacts.
- Successful prompt rendering by `next`, `roadmap`, `plan`, `code`, `review`, or `archive` never changes workflow state by itself.
- `code --complete` validates Developer-owned output as a coherent whole before
  committing it, including a Git-backed freshness comparison for the final
  reviewer handoff section. Non-Git completion requires the complete current
  handoff artifact because no committed baseline exists. `review --reserve`
  validates the clean recorded Git entry boundary before creating only the next
  incomplete review path; `review --complete` validates and commits its single
  outcome.
- `archive --complete` validates byte-identical task/review copies, summary
  schema and sections, declared capability reconciliation, roadmap references,
  and the Git or non-Git lifecycle boundary before committing or clearing state.
- Product Owner work that changes roadmap content but creates no active task leaves state `ready`.
- A failed validation, prompt rendering, or role precondition check leaves state unchanged.
- Inspection, blocker routing, and recovery diagnostics are non-mutating unless the responsible role deliberately repairs an owned artifact.

## Invalid and ambiguous evidence

The CLI must refuse a transition and report `invalid` when it cannot determine one safe state. Representative cases include:

- only one of a populated task plan and required notes exists;
- multiple active roadmap items exist where the project supports only one;
- task and roadmap identifiers disagree, or the named item is missing or in an incompatible status;
- task status is unknown, missing, or incompatible with current reviews;
- `remediates-review` is stale, names a non-latest or non-`changes-requested` review, or lacks complete finding dispositions when task status is `implementation-complete`;
- `blocked-review-resolution` names a non-latest or non-blocked review, has an unauthorized recorder, lacks resolvable evidence, or disagrees with the task status and selected route;
- review numbering has gaps, collisions, duplicate internal numbers, or non-zero-padded names;
- a review has no outcome, multiple outcomes, an unknown outcome, or a different task identifier;
- current reviews exist without an active task;
- an approved review lacks resolved capability-impact metadata or another required archive input;
- current artifacts remain after a completed archive while the roadmap and capability records say the task is delivered;
- archive, roadmap, and capability cross-references disagree after an interrupted archival attempt;
- required YAML front matter or canonical artifacts are malformed or unreadable.

Where evidence could describe more than one valid state, the result is still `invalid`; detection must not choose the most convenient interpretation.

## Failure atomicity and recovery

Validation and prompt-rendering failures perform no workflow mutation. Errors identify:

- the detected state or `invalid` condition;
- the specific files and fields that conflict;
- the command's allowed starting states;
- a non-destructive corrective action and the responsible role;
- the command to retry after repair, when one is unambiguous.

Recovery must preserve evidence. Do not delete an active task, renumber or rewrite completed reviews, downgrade an approval, or clear a partial archive automatically. Repair malformed metadata only under the artifact owner's role. For interrupted archival work, compare the current artifacts, archive copy, roadmap, and capabilities; complete or deliberately roll forward the transaction before clearing `.concoct/current/`.

`archive` is transactional in this order:

1. validate the active task, approval, capability impact, repository consistency, and unused destination;
2. create the dated archive directory and copy all accepted task and review artifacts;
3. create and validate `summary.md`;
4. reconcile `.concoct/capabilities.md` with delivered behavior;
5. validate complete archive and capability evidence;
6. Git-backed: commit archive/pending-delivery evidence and stop at `archived`;
7. non-Git: mark delivery and clear current to reach `ready`.

Any partial durable write requires reconciliation and is never treated as
`ready` merely because archive files exist.

## Complete scenario traces

### Non-Git happy path

```text
uninitialized
  → init → ready
  → roadmap/Product Owner work → ready with an eligible item
  → plan/Task Planner work → planned
  → code/Developer work → implementation-in-progress → implementation-complete
  → review/Reviewer work → review-approved
  → archive/Archivist work → ready
```

### Repeated remediation

```text
implementation-complete
  → review-01 changes-requested
  → code → implementation-complete
  → review-02 changes-requested
  → code → implementation-complete
  → review-03 approved
  → archive → ready
```

The repeated-remediation trace above is non-Git. A Git-backed approval continues
`archive → archived → integrate`, with conflict recovery as defined above.

### Blocked review

```text
implementation-complete
  → review-01 blocked
  → review-blocked
  → responsible-role or human handoff and owned evidence
  → Task Planner records blocked-review-resolution route: review
  → implementation-complete
  → review-02
```

An implementation-required route is also explicit:

```text
review-02 blocked
  → review-blocked
  → Developer records blocked-review-resolution route: code
     and task status: implementation-in-progress
  → code → implementation-complete
  → review-03
```

No path above requires `concoct handoff`, `concoct abandon`, or `concoct doctor`.
