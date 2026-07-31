# GitHub Copilot Instructions

Read `.concoct/protocol.md`, `.concoct/policy.md`, and the repository-owned
`AGENTS.md` entry point, in that precedence order.

Task prompts select a role from `.concoct/personas/`. Read and adopt the selected
persona after the layered sources. Do not weaken protected protocol controls.

For substantial implementation tasks, use the active planning files:

```text
.concoct/current/task-plan.md
.concoct/current/notes.md
```

Before making changes:

1. Read `AGENTS.md`.
2. Read the active planning files.
3. Inspect relevant code.

Before finishing:

1. Update the active planning files.
2. Run the documented project checks.
3. Summarize what changed, what passed, and what remains.

Keep changes focused on the active task.
