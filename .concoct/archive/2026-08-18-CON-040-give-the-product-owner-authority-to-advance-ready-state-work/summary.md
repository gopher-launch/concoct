---
task-id: CON-040
roadmap-id: CON-040
status: archived
archived: 2026-08-18
review: review-03.md
delivery: pending-integration
capability-impact:
  type: update
  ids:
    - CAP-001
    - CAP-005
    - CAP-006
    - CAP-009
    - CAP-011
    - CAP-012
    - CAP-013
    - CAP-014
    - CAP-015
---

# Summary

## Task

Give the Product Owner authority to advance ready-state work.

## Delivered outcome

Concoct now retains bounded, evidence-bound semantic Product Owner decisions,
supports one-use approval and constrained roadmap/capability reconciliation,
and advances an approved selection directly into planning without reinvocation.
Status, execution, and bounded runs share the same decision authority while
manual `next` remains deterministic and read-only. Active task state retains
precedence over private ready-state decision evidence.

## Key decisions

- Product Owner judgment remains semantic authority; executable validation does not rank or select work.
- Private decision records remain inspectable evidence and do not replace canonical product truth.
- Reconciliation is bounded to exact record replacements bound by prior-record digests; crash-resumable coordination remains out of scope.

## Files and areas changed

Product Owner orchestration and decision persistence, workflow/state resolution,
execution and run-loop approval/application, status and inspection surfaces,
manual and executable-owned prompts, documentation, templates, and regression
coverage.

## Verification

Review 03 records passing the full Go suite, vet, shell syntax, executable-mode,
and diff checks. It also records independent inspection of the remediation and
planned-task precedence regressions. Race, cross-platform, and fresh-init checks
were not repeated in Review 03; earlier accepted implementation evidence records
those checks.

## Review outcome

Review 03 approved CON-040 after resolving the Review 01 and Review 02 findings
about retained Product Owner decisions masking active task continuations.

## Capability changes

Updated CAP-001, CAP-005, CAP-006, CAP-009, CAP-011, CAP-012, CAP-013, CAP-014,
and CAP-015 with the delivered decision, workflow, prompt, reconciliation,
orchestration, execution, run, status, and measurement behavior.

## Skipped work

None.

## Follow-up work

The documented per-file reconciliation crash window remains a known limitation.
General retained-candidate recovery and crash-resumable coordination remain
approved non-goals. Git delivery remains pending integration.

## References

- `.concoct/roadmap.md` — CON-040
- `.concoct/capabilities.md` — CAP-001, CAP-005, CAP-006, CAP-009, CAP-011, CAP-012, CAP-013, CAP-014, CAP-015
- `review-03.md`
