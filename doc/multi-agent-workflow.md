# Concoct Multi-Agent Workflow

This document explains how to use Concoct with multiple agents or agent tools.

## Keep task context neutral

The active task files should not be specific to one tool.

Use:

```text
.concoct/current/task-plan.md
.concoct/current/notes.md
```

Avoid phrases like:

```text
Codex must...
Claude must...
Copilot must...
```

Prefer:

```text
The implementer should...
The reviewer should...
Before finishing, run...
```

## Use thin adapters

Different agents may use different instruction mechanisms.

Recommended adapters:

```text
AGENTS.md                         # canonical shared instructions
CLAUDE.md                         # Claude Code adapter
CONVENTIONS.md                    # generic adapter for tools that read convention files
.aider.conf.yml                   # Aider adapter
.github/copilot-instructions.md   # GitHub Copilot adapter
.codex/skills/project-planning/   # Codex skill adapter
```

The adapters should point back to `AGENTS.md`.

Do not maintain separate copies of the same rules in every adapter.

## Roles

Multiple agents should be coordinated by role.

The prompt for each role should explicitly select its persona from `.concoct/personas/`. Personas define how to perform a role; `AGENTS.md` and the planning artifacts continue to define project rules and task scope.

### Planner

Persona: `.concoct/personas/planner.md`

Owns:

- task definition
- `.concoct/current/task-plan.md`
- initial `.concoct/current/notes.md`

Responsibilities:

- clarify the goal
- define constraints and non-goals
- break the task into phases
- record initial assumptions and risks

### Implementer

Persona: `.concoct/personas/code-developer.md`

Owns:

- code changes
- test changes
- documentation changes needed for the task

Updates:

- `.concoct/current/task-plan.md`
- `.concoct/current/notes.md`

Responsibilities:

- inspect before editing
- update the plan if assumptions are wrong
- implement in focused steps
- run checks
- record meaningful failures

### Reviewer

Persona: `.concoct/personas/code-reviewer.md`

Owns:

- review findings
- design concerns
- testing gaps
- documentation gaps

May create:

```text
.concoct/current/review.md
```

Only create `review.md` when there is a meaningful separate review phase.

### Archivist

Owns:

```text
.concoct/archive/YYYY-MM-DD-short-task-name/summary.md
```

Responsibilities:

- move completed task artifacts to archive
- summarize outcome
- record key decisions
- record checks run
- record follow-up work

Archiving uses the dedicated Archivist persona selected by the archive prompt;
it does not retain an implementation or review persona. After authoring the
candidate evidence, the Archivist runs `concoct archive --complete`.

### Technical writer

Select the persona by audience:

- `.concoct/personas/doc-technical-writer-user.md` for end users
- `.concoct/personas/doc-technical-writer-api.md` for API consumers
- `.concoct/personas/doc-technical-writer-code.md` for developers working with the codebase

If documentation is a distinct phase, switch explicitly from the prior role's persona to the selected writer persona.

## Handoff protocol

When handing off between agents, update `notes.md` with:

- what changed
- what remains
- known risks
- commands already run
- commands still needed
- files likely needing attention

Suggested handoff format:

```md
## Handoff

### Current state

### Completed

### Remaining

### Known risks

### Commands run

### Suggested next step
```

## Avoid agent soup

Do not let multiple agents edit the same files blindly.

Before starting work, each agent should read:

```text
AGENTS.md
.concoct/current/task-plan.md
.concoct/current/notes.md
```

If present, also read:

```text
.concoct/current/review.md
```

## Practical recommendation

Most of the time, use one strong agent in distinct modes:

```text
planner mode → implementer mode → reviewer mode → archivist mode
```

Use truly separate agents only when there is a clear reason.
