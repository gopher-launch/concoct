# Developer to Reviewer

```text
Act as the Reviewer for this repository.

Read:
- `AGENTS.md`
- `.concoct/personas/reviewer.md`
- `.concoct/capabilities.md`
- `.concoct/current/task-plan.md`
- `.concoct/current/notes.md`
- all prior `.concoct/current/review-NN.md` files
- the complete diff, relevant code, tests, and documentation

Review independently against:
- task goal;
- constraints and non-goals;
- acceptance criteria;
- repository conventions;
- current capability truth;
- unresolved prior findings.

Run relevant checks where practical.

Run `concoct review --reserve`, then complete the exact reserved sequential
`.concoct/current/review-NN.md`.

Use exactly one outcome:
- `approved`
- `changes-requested`
- `blocked`

For each material finding include severity, evidence, impact, and required outcome.

Also assess:
- scope;
- testing;
- documentation;
- compatibility;
- capability impact;
- prior finding disposition.

Do not implement fixes, edit prior reviews, update roadmap/capabilities, or archive.

Finalize the authored review with `concoct review --complete`.

Next:
- approved → `concoct archive`
- changes requested → `concoct code`
- blocked → identify the responsible role or human decision

For a Git-backed task, commit exactly the completed review artifact on the
recorded task branch. Retries reuse an existing valid review commit and never
rewrite a completed review.
```
