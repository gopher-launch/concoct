---
task-id: CON-009
roadmap-id: CON-009
status: archived
archived: 2026-07-31
review: review-03.md
delivery: pending-integration
capability-impact:
  type: add
  ids:
    - CAP-011
---

# Summary

## Task

Implement the validated archival completion boundary that turns approved task
evidence into a durable archive and reconciled capability truth across
Git-backed and non-Git Concoct lifecycles.

## Delivered outcome

Concoct now keeps ordinary `concoct archive` invocation read-only while
providing explicit `concoct archive --complete` validation for
Archivist-authored evidence. Completion validates the accepted task and full
sequential review history, deterministic append-only archive destination,
summary lifecycle metadata, declared capability impact, capability provenance,
roadmap cross-references, and permitted repository mutations.

Git-backed archival validates the recorded task branch and base, commits one
coherent pending-delivery transition, retains current evidence, and resolves
the non-recursive `git.archive-commit: self` sentinel to exact archival HEAD.
Clean retries must revalidate that immutable transition. Non-Git archival
validates complete delivery evidence before clearing current state and
returning the project to `ready`.

## Key decisions

- Preserve semantic summary and capability authorship as Archivist judgment
  while making the CLI responsible for structural and transactional validation.
- Require approved review by default; exceptional completion requires explicit
  command authority and reason that exactly match durable summary metadata.
- Use the deterministic date, roadmap ID, and normalized task title for the
  only accepted archive destination.
- Keep Git delivery, squash integration, current cleanup, and accepted branch
  deletion under the existing integration lifecycle.
- Use `archive-commit: self` to avoid recursive commit identity while allowing
  status and integration to resolve and verify exact archival HEAD.

## Files and areas changed

- Added archival completion validation and tests under `internal/workflow`.
- Extended CLI orchestration and Git repository support for archival commits,
  exact transition validation, and safe retries.
- Updated README and normative command, state-machine, workflow, persona,
  handoff, skill, and embedded-template guidance.
- Added the CON-009 archive, CAP-011 capability record, pending roadmap archive
  reference, and retained current pending-delivery metadata.

## Verification

- `go test -count=1 ./...` — passed after final approval and before archival.
- `go vet ./...` and `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` and executable-mode validation — passed.
- The approving review confirmed temporary generated-project initialization,
  complete nested/dotfile copying, Git staging without a generated commit,
  template parity, and reported `ready` state.
- `git diff --check c89529b90eb53d9628a88bb93b6e1ef670583beb..HEAD`
  passed before archival.

## Review outcome

`review-03.md` approved CON-009 after two remediation rounds. Review 01's
lifecycle, content-preservation, deterministic-path, and coverage findings were
fixed. Review 02's clean-retry validation and archive-boundary coverage
findings were then fixed. The approving review reports no material findings.

## Capability changes

Added CAP-011 for validated archive completion and capability reconciliation
across Git-backed pending-delivery and non-Git delivered lifecycles. CAP-001,
CAP-005, CAP-007, and CAP-010 remain compatible prerequisites and retain their
accepted meaning.

## Skipped work

Automatic Archivist prose or acceptance judgment, integration policy changes,
hosted-provider behavior, general locking, worktrees, concurrent tasks, schema
redesign, direct agent execution, and broader diagnostics remain outside
CON-009.

## Follow-up work

Delivery remains pending `concoct integrate`. Integration must squash the
accepted task into `main`, mark CON-009 delivered, clear current evidence, and
perform accepted task-branch cleanup. Non-Git repositories continue to have no
committed pre-transition baseline, and cleanup failures remain forward-retry
situations with durable evidence preserved.

## References

- Roadmap item: `CON-009` in `.concoct/roadmap.md`
- Capability: `CAP-011` in `.concoct/capabilities.md`
- Approving review: `review-03.md`
- Prior reviews: `review-01.md`, `review-02.md`
- User documentation: `README.md`
- Normative contracts: `doc/command-reference.md`, `doc/state-machine.md`,
  `doc/workflow.md`
