---
instruction-layer: project-guidance
---

# AGENTS.md

## Instruction entry point

Read these sources in precedence order before substantial work:

1. `.concoct/protocol.md` — non-overridable Concoct controls.
2. `.concoct/policy.md` — project-selected workflow policy.
3. `AGENTS.md` — repository-owned guidance (this file).
4. The selected persona and `.concoct/current/` task context.

Project guidance may add stricter compatible requirements but may not weaken
protocol controls. Concoct preserves this file as project-owned content.

## Project

Concoct is a lightweight, agent-neutral workflow that turns raw ideas into durable, implementation-ready plans for coding agents.

The primary product contract is the set of files installed into generated projects. Keep the workflow portable across Codex, Claude Code, GitHub Copilot, Aider, and future tools.

## Canonical structure

Long-lived conventional project files remain at the repository root, including `AGENTS.md`, `README.md`, and `LICENSE`.

Concoct's source content lives in root-level directories:

- Concoct is managed by Concoct and its workflow/task artifacts live in the `.concoct/` directory.
- `cmd/`, `doc/`, and `templates/` contain reusable workflow material.
- `templates/` contains the exact filesystem structure copied into generated projects.

Generated projects place Concoct-owned state and personas under `.concoct/`. Conventional files and tool adapters such as `AGENTS.md`, `.codex/`, and `.github/` remain at the generated project root.

## Design principles

- Keep the workflow lightweight and agent-neutral.
- Treat `AGENTS.md` as canonical project guidance.
- Keep tool-specific adapters thin and avoid duplicating durable rules.
- Preserve the distinction between repository-owned assets and installed templates.
- Prefer explicit, portable shell behavior and clear failure modes.
- Do not introduce planning ceremony for trivial work.

## Naming

- Use hyphens, not underscores, in file and directory names.
- Use uppercase Markdown filenames for long-lived top-level artifacts.
- Use lowercase hyphenated Markdown filenames for task and workflow artifacts.
- Use `Concoct` for the product name and `concoct` for paths, repository names, and identifiers.

## Working on Concoct

For substantial tasks, read `current/task-plan.md`, `current/notes.md`, and the relevant persona under `personas/`. Keep material decisions in the notes and archive completed work under `archive/`.

Before finishing changes to templates or initialization:

1. Run `bash -n cmd/concoct/concoct`.
2. Run `./cmd/concoct/concoct` against a temporary parent directory.
3. Confirm dotfiles, nested templates, personas, and planning directories were copied.
4. Confirm the generated project is a Git repository and contains its bootstrap prompt.
5. Run `git diff --check` and search for stale branding or paths.

Preserve executable permissions on `cmd/concoct/concoct`. Never initialize or commit generated test projects inside this repository.
