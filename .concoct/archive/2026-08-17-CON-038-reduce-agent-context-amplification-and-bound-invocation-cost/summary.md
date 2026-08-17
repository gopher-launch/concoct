---
task-id: CON-038
roadmap-id: CON-038
status: archived
archived: 2026-08-17
review: review-03.md
delivery: pending-integration
capability-impact:
  type: update
  ids:
    - CAP-013
    - CAP-014
    - CAP-015
---

# Summary

## Task

Reduce agent context amplification and bound invocation cost.

## Delivered outcome

Concoct now normalizes supported Codex activity into conservative, payload-safe
evidence; tracks bounded repetition and command-output measurements; evaluates
live-observable warnings; and enforces configured elapsed, activity, and
command-output hard budgets. Budget stops preserve partial evidence, reject
late candidates, and leave workflow state unchanged. Role guidance is more
compact, and execution/run summaries distinguish accepted from stopped or
rejected cost.

## Key decisions

- Classification remains conservative and does not infer tests, reads, or model exchanges without affirmative evidence.
- Token warnings remain terminal-only because usage is not live-enforceable.
- No paid live benchmark was run; the accepted deterministic Archivist reduction and unlike-live-workload observations remain explicitly non-causal.

## Files and areas changed

Adapter normalization, execution budgets and warning tracking, role-budget
configuration, run aggregation, compact built-in guidance, documentation,
controlled reports, and adversarial execution tests.

## Verification

Review 03 records passing the full deterministic suite, focused race suite,
vet, native and Windows builds, shell validation, source/template parity,
fresh isolated initialization, and diff checks. The approved review also
records repeated warning and event-driven hard-budget coverage.

## Review outcome

Review 03 approved CON-038 after confirming that the Review 01 and Review 02
findings were resolved and that the amended task contract was satisfied.

## Capability changes

CAP-013, CAP-014, and CAP-015 gain conservative semantic activity evidence,
warning and enforceable budget behavior, stopped/rejected-cost accounting, and
bounded role execution. CAP-015's stale CON-037 report reference is corrected.

## Skipped work

No further billable live benchmark was run under the operator-approved cost
stop. Unlike live workloads remain directional observations and no causal
processed-input reduction is claimed.

## Follow-up work

Codex event shapes remain version-sensitive; command classification is
intentionally conservative; and command-output enforcement observes only
decoded bounded events. A deliberate live compatibility run may be considered
after adapter or Codex CLI upgrades. Git delivery remains pending integration.

## References

- `.concoct/roadmap.md` — CON-038
- `.concoct/capabilities.md` — CAP-013, CAP-014, CAP-015
- `.concoct/reports/con-038-context-amplification.md`
- `review-03.md`
