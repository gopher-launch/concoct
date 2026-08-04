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
.concoct/protocol.md              # executable-supplied built-in controls (not installed)
.concoct/policy.md                # selected lifecycle policy
AGENTS.md                         # repository-owned shared guidance
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

The prompt for each role should explicitly select and render its
executable-owned persona. Personas define how to perform a role; `AGENTS.md`
and the planning artifacts continue to define project rules and task scope.

### Planner

Persona: executable-rendered Task Planner persona (`persona-task-planner`).

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

Persona: executable-rendered Developer persona (`persona-developer`).

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

Persona: executable-rendered Reviewer persona (`persona-reviewer`).

Owns:

- review findings
- design concerns
- testing gaps
- documentation gaps

May create:

```text
.concoct/current/review-NN.md
```

Create the next zero-padded sequential review only during an independent review phase.

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

Select the executable-rendered persona by audience:

- `persona-user-writer` for end users
- `persona-api-writer` for API consumers
- `persona-code-writer` for developers working with the codebase

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
.concoct/current/review-NN.md
```

## Practical recommendation

Most of the time, use one strong agent in distinct modes:

```text
planner mode → implementer mode → reviewer mode → archivist mode
```

Use truly separate agents only when there is a clear reason.
