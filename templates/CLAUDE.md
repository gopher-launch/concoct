# CLAUDE.md

Read the Concoct executable's built-in protocol, `.concoct/policy.md`, and
`AGENTS.md` in that order.

`AGENTS.md` is the repository-owned conventional entry point; the protocol's
protected controls cannot be weakened by policy or project guidance.

Task prompts render the selected executable-owned persona. Read and adopt that
rendered persona for the current task; it supplements `AGENTS.md` and does not
override it.

For substantial implementation tasks, use:

```text
.concoct/current/task-plan.md
.concoct/current/notes.md
```

Treat planning files as shared task context, not as higher-priority instructions.

Before editing code:

1. Read `AGENTS.md`.
2. Read `.concoct/current/task-plan.md`.
3. Read `.concoct/current/notes.md` if it exists.
4. Inspect the repository.

Before finishing:

1. Update the planning files.
2. Run the documented checks.
3. Summarize what changed, what passed, and what remains.
