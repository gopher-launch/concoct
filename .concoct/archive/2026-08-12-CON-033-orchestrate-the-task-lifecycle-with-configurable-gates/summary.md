---
task-id: CON-033
roadmap-id: CON-033
status: archived
archived: 2026-08-12
review: review-03.md
delivery: pending-integration
capability-impact:
  type: add
  ids:
    - CAP-014
---

# Summary

## Task

Orchestrate the task lifecycle with configurable gates.

## Delivered outcome

Concoct now provides `concoct run`, a bounded lifecycle coordinator that
re-resolves and executes authorized workflow actions through planning,
development, independent review, remediation, archival, and local integration.
It stops at evidence-bound approval gates and reports blockers, decisions,
unsafe states, failures, cancellation, recovery choices, and exhausted bounds.

## Key decisions

- Approvals are one-use, evidence-bound, and scoped to the protected action.
- Supervised agents author role-owned candidates; the outer executable retains
  canonical completion and Git-transition authority.
- Reviewer invocations remain independent, and run-driven integration never
  implies remote push authority.
- Crash-resumable coordination remains deferred to CON-034.

## Files and areas changed

The run coordinator, finite run policy, pending-gate state, planning setup,
supervised execution boundary, progress and intervention reporting, integration
recovery, tests, and command/workflow documentation were added or updated.

## Verification

The approved review records passing full and focused tests, race tests, vet,
build, shell validation, diff checks, source/template parity, and fresh
initialization checks.

## Review outcome

`review-03.md` approved the implementation after resolving all prior findings
and confirming all 24 acceptance criteria.

## Capability changes

Added CAP-014 for bounded repeated lifecycle execution with configurable
restrictive gates, evidence-bound approvals, independent review, supervised
canonical transitions, progress and intervention reporting, recovery stops, and
local-only integration. CAP-013 remains the one-shot primitive.

## Skipped work

Crash-resumable runs, abandoned-run reconciliation, concurrent tasks, automatic
retries, arbitrary gates, and remote push remain outside this task.

## Follow-up work

Plan CON-034 for durable interrupted-run recovery when product ownership is
ready. Integration of this archived task remains pending under the Git workflow.

## References

- `.concoct/roadmap.md` — CON-033
- `.concoct/capabilities.md` — CAP-014
- `.concoct/current/review-03.md`
