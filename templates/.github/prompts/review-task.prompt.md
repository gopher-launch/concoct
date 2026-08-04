# Review active task

Use `concoct review` to render the built-in protocol and Reviewer persona with
the project context, then adopt that rendered persona for this task.

Review the completed changes against:

- `AGENTS.md`
- `.concoct/policy.md`
- `.concoct/current/task-plan.md`
- `.concoct/current/notes.md`

Produce a concise review covering:

1. Whether the implementation satisfies the task.
2. Any design concerns.
3. Any testing gaps.
4. Any documentation gaps.
5. Any unrelated changes that should be separated.
6. Whether the task is ready to archive.

Reserve and create exactly the next sequential review:

```text
.concoct/current/review-NN.md
```
