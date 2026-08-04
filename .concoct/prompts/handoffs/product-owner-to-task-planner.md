# Product Owner to Task Planner

```text
Act as the Task Planner for this repository.

Read:
- `AGENTS.md`
- the selected executable-owned Task Planner persona rendered in this prompt
- `.concoct/capabilities.md`
- `.concoct/roadmap.md`
- relevant archive summaries
- relevant source code, tests, and documentation

Roadmap item:
`<ROADMAP-ID>`

Confirm:
1. The item exists and is ready for planning.
2. Dependencies are satisfied or explicitly handled.
3. Every declared capability prerequisite is accepted, and its documented
   limitations are compatible with the selected outcome.
4. No conflicting active task exists.
5. Repository reality supports the roadmap assumptions.
6. No unresolved product decision must return to the Product Owner.

When ready, create:
- `.concoct/current/task-plan.md`
- `.concoct/current/notes.md`

The task plan must define:
- identifiers and metadata;
- goal, context, current state, and target state;
- constraints and non-goals;
- assumptions, risks, and open questions;
- implementation phases;
- observable acceptance criteria;
- verification;
- capability impact;
- developer handoff expectations.

Do not implement code or invent product direction.

For a safe Git repository, record the exact current branch and HEAD, derive the
documented deterministic task branch, refuse dirty/detached/operation or branch
collision inputs, and create/check out the branch only after active artifacts
validate. For a non-Git project, omit Git metadata and continue unbranched.

Summarize planning readiness, scope, capability impact, risks, and unresolved questions.

When ready, recommend:
`concoct code`
```
