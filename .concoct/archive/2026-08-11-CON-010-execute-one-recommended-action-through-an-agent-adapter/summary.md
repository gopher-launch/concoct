---
task-id: CON-010
roadmap-id: CON-010
status: archived
archived: 2026-08-11
review: review-02.md
delivery: pending-integration
capability-impact:
  type: add
  ids:
    - CAP-013
---

# Summary

## Delivered outcome

CON-010 added `concoct exec` as a guarded one-shot execution boundary. It
resolves the current typed workflow recommendation, renders byte-identical
manual role guidance, runs the configured Codex adapter or authorized direct
integration action, validates the correlated structured outcome against fresh
repository state, and reports the resulting state and next recommendation.
It also adds dry-run resolution and retained invocation inspection.

## Key decisions

- Execution authorizes exactly one action and never loops, integrates, pushes,
  or bypasses human and policy gates.
- Launch-time evidence is revalidated; observed repository and workflow state
  outranks process status or agent claims.
- Runtime records are private, bounded, sanitized, and safely retryable from
  fresh authorization. Cancellation and timeout close the attempt without
  fabricating completion.

## Files and areas changed

The CLI, orchestration, execution, adapter, configuration, runtime-record,
integration, project-template, and documentation layers were extended with
the one-shot execution contract, Codex adapter, inspection surface, safety
posture, and recovery behavior. Focused and regression tests cover normal,
failure, drift, cancellation, timeout, and direct-integration paths.

## Verification

`go test -count=1 ./...`, focused race tests, `go vet ./...`, native and
Windows builds, shell syntax and executable-mode checks, fresh initialization,
stale-claim/path searches, and `git diff --check` passed. No live external
model invocation was run; deterministic adapter and repository tests exercised
the execution boundary.

## Review outcome

`review-02.md` approved the implementation after remediation of review-01
findings covering archival authorization, launch-time evidence freshness, and
cancellable/time-bounded direct integration.

## Capability changes

Added CAP-013 for one-shot execution of an authorized workflow action through
the configured adapter or direct integration authority, with exact prompt
parity, private retained evidence, freshness checks, bounded interruption, and
observed-state precedence.

## Skipped work

Repeated execution, durable multi-invocation recovery, shared locking,
interactive or resumable sessions, automatic adapter discovery, and additional
adapters remain outside CON-010 scope.

## Follow-up work

Run `concoct integrate` to deliver this archived task from its task branch to
`main`. Future execution repetition and recovery remain represented by the
roadmap items that depend on this capability.
