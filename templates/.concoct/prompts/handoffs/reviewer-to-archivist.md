# Reviewer to Archivist

```text
Act as the Archivist for this repository.

Read:
- `AGENTS.md`
- the selected executable-owned Archivist persona rendered in this prompt
- `.concoct/capabilities.md`
- `.concoct/roadmap.md`
- `.concoct/current/task-plan.md`
- `.concoct/current/notes.md`
- all current review files
- the latest approved review
- relevant code changes, tests, and documentation

Validate:
1. An active task exists.
2. Metadata and roadmap ID are valid.
3. Latest review is `approved`.
4. Capability impact is resolved.
5. Required artifacts exist.
6. Accepted implementation is present.
7. Archive destination is safe.

Archive transactionally:
1. Create the dated archive directory.
2. Copy accepted task artifacts.
3. Create `summary.md`.
4. Reconcile `capabilities.md` with delivered behavior.
5. Add cross-references and validate the archive.
6. For a Git-backed task, record pending roadmap reconciliation, preserve the
   accepted task plan byte-for-byte in the archive candidate, and leave current
   task metadata unchanged while authoring. The completion boundary applies
   `git.status: archived` and non-recursive `git.archive-commit: self` to the
   current task, commits all archival evidence on the recorded task branch,
   and resolves the sentinel to exact HEAD. Do not mark delivery or clear
   current state.
7. For a non-Git task only, mark the roadmap item `delivered`, clear current
   state after validation, and confirm `ready`.

Do not approve work, alter source code, rewrite history, or copy planned capability claims without evidence.

Report archive path, delivered roadmap item, approving review, capability changes, reset state, follow-ups, and manual actions.

After authoring all evidence, run `concoct archive --complete`. Use the explicit
authority/reason flags only for an authorized exceptional override and mirror
their values exactly in summary front matter.

Recommend `concoct integrate` for a Git-backed task. For a non-Git task,
recommend `concoct next`.
```
