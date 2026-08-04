---
instruction-layer: project-guidance
---

# AGENTS.md

This project uses Concoct for lightweight, file-based planning. This file is
repository-owned project guidance and remains the conventional entry point.

## Effective instruction sources

Read these sources in precedence order before substantial work:

1. The Concoct executable's built-in protocol — non-overridable controls.
2. `.concoct/policy.md` — project-selected workflow policy.
3. `AGENTS.md` — repository-owned project guidance (this file).
4. The persona selected by the prompt and `.concoct/current/` task context.

Project guidance may strengthen protocol controls but may not weaken them.
Agent adapters point here and to the same layered sources without redefining
durable rules.

## Project intent

Describe what <project-name> is, who it is for, and what it should be good at.
Keep this section durable; temporary scope belongs in the active task plan.

## Design principles

- Keep the core model small and understandable.
- Prefer explicit, maintainable behavior and clear failure modes.
- Preserve public compatibility where practical.
- Keep changes focused on the active task.
- Tests should demonstrate behavior rather than implementation details.

## Architecture and package boundaries

Document project-owned architecture, dependency direction, and package
responsibilities here.

## Coding and naming conventions

- Use the project's standard formatter and verification commands.
- Prefer small interfaces near their consumers and useful contextual errors.
- Avoid unnecessary global state and hidden initialization behavior.
- Use repository-appropriate naming conventions.

## Project verification

Record the exact formatting, test, static-analysis, build, and integration
commands required by this repository.

## Concoct working context

Substantial work uses:

- `.concoct/current/task-plan.md`
- `.concoct/current/notes.md`
- `.concoct/current/review-NN.md` for sequential independent reviews
- role-specific operating guidance rendered by the Concoct executable

Read current files before updating them. Keep durable decisions and verification
results in notes, and do not create workflow ceremony for trivial work.
