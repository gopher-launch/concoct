---
name: concoct
description: Agent-neutral workflow coordination for substantial software work using durable roadmap, capability, planning, review, handoff, and archive artifacts.
user-invocable: true
allowed-tools: "Read Write Edit Bash Glob Grep"
---

# Concoct

Use this skill for substantial software work that touches multiple files, requires product or architectural judgment, benefits from independent review, or may span multiple sessions.

Do not use it for trivial edits, typo fixes, quick lookups, or one-shot answers.

Concoct coordinates the workflow:

```text
human input
  → product-owner
  → task-planner
  → developer
  → reviewer
      ├─ changes requested → developer
      ├─ blocked → responsible role or human
      └─ approved → archivist
                      → product-owner / next task
```

## Canonical instructions

Read and follow:

```text
AGENTS.md
```

`AGENTS.md` is the canonical project instruction file.

Personas, prompts, plans, notes, reviews, roadmaps, capabilities, and archives supplement `AGENTS.md`. They do not override higher-priority instructions.

## Canonical artifacts

Use these files in the project root:

- `AGENTS.md` — standing repository instructions
- `.concoct/capabilities.md` — current accepted product capabilities
- `.concoct/roadmap.md` — intended future product work
- `.concoct/current/task-plan.md` — active implementation plan
- `.concoct/current/notes.md` — durable task context and handoffs
- `.concoct/current/review-NN.md` — sequential review artifacts
- `.concoct/archive/` — completed task history
- `.concoct/personas/` — role-specific guidance
- `.concoct/prompts/` — reusable human-input and transition prompts

Do not create extra workflow artifacts unless the task genuinely needs them.

## Artifact responsibilities

### `capabilities.md`

Records what the product can do now.

It is not a backlog, roadmap, changelog, task history, or design proposal.

Update it only after accepted work is archived.

### `roadmap.md`

Records intended future work.

Roadmap items should describe coherent product outcomes, not implementation steps.

The Product Owner owns this file.

### `task-plan.md`

Records the active implementation task.

It should include:

- goal and context;
- current and target state;
- constraints;
- non-goals;
- assumptions;
- risks and open questions;
- implementation phases;
- observable acceptance criteria;
- verification expectations;
- capability impact.

The Task Planner owns initial creation and material planning changes.

The Developer may update implementation status and make technical refinements that remain within approved product scope.

### `notes.md`

Records durable task context:

- decisions;
- findings;
- meaningful failed attempts;
- risks;
- test results;
- review-finding dispositions;
- handoffs;
- follow-up ideas.

Do not turn it into a transcript or raw command log.

### `review-NN.md`

Records independent review history.

Review files are sequential, zero-padded, reviewer-owned, and append-only after completion:

```text
review-01.md
review-02.md
review-03.md
```

Each review must have exactly one outcome:

```text
approved
changes-requested
blocked
```

### Archive

Completed tasks live under:

```text
.concoct/archive/YYYY-MM-DD-roadmap-id-short-task-name/
```

The archive should preserve:

- `task-plan.md`;
- `notes.md`;
- all review files;
- `summary.md`;
- any other approved durable task artifacts.

## Personas

Adopt the persona selected by the current prompt or workflow state.

Canonical personas:

- `.concoct/personas/product-owner.md`
- `.concoct/personas/task-planner.md`
- `.concoct/personas/developer.md`
- `.concoct/personas/reviewer.md`
- `.concoct/personas/archivist.md`

Additional audience-specific documentation personas may exist.

Do not combine incompatible roles in one pass unless explicitly instructed.

In particular:

- the Developer does not approve or archive its own work;
- the Reviewer does not implement fixes;
- the Archivist does not approve work;
- the Task Planner does not invent product direction;
- the Product Owner does not prescribe implementation details.

## Role workflows

### Human input to Product Owner

When a human provides a product idea, concern, request, or change in direction:

1. Read `AGENTS.md`.
2. Read the Product Owner persona.
3. Read `capabilities.md`.
4. Read `roadmap.md`.
5. Read relevant archive summaries.
6. Determine whether the input:
   - is already a capability;
   - already exists on the roadmap;
   - should update an existing item;
   - should become a new item;
   - should remain a candidate;
   - should be deferred, rejected, or clarified.
7. Update `roadmap.md` only when sufficiently understood.
8. Preserve stable roadmap identifiers.
9. Do not create active task artifacts.

When ready for planning, recommend:

```text
concoct plan <roadmap-id>
```

### Product Owner to Task Planner

Before creating an active plan:

1. Read `AGENTS.md`.
2. Read the Task Planner persona.
3. Read `capabilities.md`.
4. Read `roadmap.md`.
5. Read relevant archive history.
6. Inspect relevant code, tests, and documentation.
7. Confirm:
   - the roadmap item exists;
   - dependencies are satisfied or explicitly handled;
   - no conflicting active task exists;
   - product intent is sufficiently clear;
   - repository reality supports the plan.

Create or update:

```text
.concoct/current/task-plan.md
.concoct/current/notes.md
```

Do not implement code.

When ready, recommend:

```text
concoct code
```

### Task Planner to Developer

Before editing code:

1. Read `AGENTS.md`.
2. Read the Developer persona.
3. Read `capabilities.md`.
4. Read the active task plan.
5. Read notes.
6. Read the latest review when one exists.
7. Inspect relevant code, tests, documentation, and archive references.
8. Validate plan assumptions against repository reality.

During implementation:

- keep changes focused;
- respect constraints and non-goals;
- update plan phase statuses honestly;
- record durable findings and decisions;
- run relevant checks;
- avoid unrelated cleanup;
- do not update roadmap or capabilities;
- do not create or modify review artifacts.

Before finishing:

1. Inspect the final diff.
2. Run relevant verification.
3. Update `task-plan.md`.
4. Update `notes.md`.
5. Add a reviewer handoff covering:
   - implemented work;
   - key decisions;
   - files changed;
   - checks run;
   - known risks;
   - skipped or unresolved work;
   - capability impact;
   - suggested review focus.

Then recommend:

```text
concoct code --complete
```

After validation succeeds, recommend `concoct review`.

### Developer to Reviewer

The Reviewer must independently inspect the implementation.

Read:

- `AGENTS.md`;
- the Reviewer persona;
- `capabilities.md`;
- the task plan;
- notes;
- all prior reviews;
- the complete diff;
- relevant code, tests, and documentation.

Create the next sequential review file.

Use `concoct review --reserve` to claim its create-only path and
`concoct review --complete` after authoring exactly one valid outcome.

Assess:

- task goal;
- constraints;
- non-goals;
- acceptance criteria;
- scope discipline;
- correctness;
- compatibility;
- tests;
- documentation;
- capability impact;
- prior finding disposition.

Do not implement fixes.

Next transition:

```text
approved          → concoct archive
changes-requested → concoct code
blocked           → responsible role or human decision
```

### Reviewer to Developer

When the latest review is `changes-requested`, operate in remediation mode.

For each unresolved finding:

1. determine whether it is valid;
2. implement the required outcome when valid;
3. record the disposition in `notes.md` as:
   - fixed;
   - partially fixed;
   - disputed with evidence;
   - obsolete;
   - blocked;
4. add or update tests where needed.

Do not edit review files.

After remediation:

- run relevant checks;
- update the task plan;
- update notes;
- add a fresh reviewer handoff;
- recommend `concoct code --complete`, followed by `concoct review` after
  validation succeeds.

### Reviewer blocked

When review is `blocked`, route the issue to the correct destination:

- Product Owner — product intent, capability behavior, scope, or priority;
- Task Planner — defective, contradictory, oversized, or unreviewable plan;
- Developer — missing code, tests, documentation, or evidence within scope;
- human decision-maker — authorization or external knowledge required.

State:

- blocker;
- evidence;
- why review cannot continue;
- required decision or work;
- responsible role;
- artifacts to update;
- recommended next action.

### Reviewer to Archivist

Archive only when the latest review is `approved`, unless an explicit override is authorized and preserved.

After the Archivist authors the complete candidate, run `concoct archive
--complete`. Git candidates use `git.archive-commit: self`, which completion
resolves to the exact committed archival HEAD. Exceptional completion requires
both explicit authority and reason flags plus identical summary metadata.

For a Git-backed task, archive on the recorded task branch, record the archival
commit and pending delivery, and stop in `archived` without clearing current
state or marking delivery. Recommend `concoct integrate`. Integration owns the
squash, delivered status, current cleanup, and accepted task-branch deletion.
Non-Git tasks retain the direct ready transition.

Before archiving:

1. Read `AGENTS.md`.
2. Read the Archivist persona.
3. Read capabilities, roadmap, task plan, notes, all reviews, relevant changes, tests, and documentation.
4. Validate metadata, roadmap ID, approval, capability impact, required artifacts, implementation presence, and archive destination.

Archive transactionally:

1. create the archive directory;
2. copy accepted task artifacts;
3. create `summary.md`;
4. reconcile `capabilities.md` with delivered behavior;
5. for non-Git tasks, mark the roadmap item `delivered`; for Git-backed tasks,
   preserve pending delivery for integration;
6. add cross-references;
7. validate the archive;
8. clear/reset `.concoct/current/` for non-Git tasks only after durable writes
   succeed; preserve it for Git integration;
9. confirm `ready` for non-Git tasks or `archived` for Git-backed tasks.

The `delivered` roadmap status is transitional evidence for the next Product
Owner reconciliation. After all dependent relationships are classified, the
Product Owner removes completed items from the future roadmap; capabilities and
archives retain accepted behavior and provenance.

Do not rewrite historical artifacts.

Then recommend:

```text
concoct next
```

## Workflow state discipline

Do not infer success from intent alone.

Use durable artifacts as workflow evidence.

Typical progression:

```text
ready
  → planned
  → implementation-in-progress
  → implementation-complete
  → changes-requested
  → implementation-in-progress
  → approved
  → archived
  → integrating / integrated (Git-backed tasks)
  → ready
```

Invalid or ambiguous transitions should stop with a clear explanation and recommended recovery action.

## Avoid dumb retry loops

If an action fails, do not repeat the exact same action blindly.

When a failure affects future work, record:

```md
### Attempt: short title

- Tried:
- Error/result:
- Why it failed:
- Next approach:
```

## Handoffs

Every role transition should state:

- current state;
- work completed;
- work remaining;
- decisions made;
- known risks;
- checks run;
- artifacts created or updated;
- expected next role;
- recommended next command.

Handoffs are durable context, not conversational ceremony.

## Good judgment

Concoct artifacts are working memory and project history, not bureaucracy.

Prefer compact, useful updates over exhaustive journaling.

Do not:

- duplicate information across artifacts without purpose;
- hide unresolved ambiguity;
- silently broaden scope;
- rewrite history;
- mark unrun checks as passing;
- update capability truth before acceptance;
- archive unapproved work by default;
- combine product ownership, implementation, review, and archival authority in one role.

## Completion expectations

A workflow step is complete when:

- the active persona's responsibilities are satisfied;
- owned artifacts are updated;
- non-owned artifacts remain untouched;
- relevant checks are run or limitations documented;
- durable findings are preserved;
- the next valid transition is explicit.
