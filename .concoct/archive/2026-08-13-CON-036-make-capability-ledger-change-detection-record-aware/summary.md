---
task-id: CON-036
roadmap-id: CON-036
status: archived
archived: 2026-08-13
review: review-02.md
delivery: pending-integration
capability-impact:
  type: update
  ids:
    - CAP-011
---

# Summary

## Task

Make capability-ledger change detection record-aware.

## Delivered outcome

Git-backed archive completion now compares ordered parsed capability records and
protected ledger structure. Record-boundary blank lines, LF/CRLF differences,
and final-newline differences no longer falsely attribute changes to adjacent
capabilities, while meaningful record, ordering, malformed-structure, and
undeclared-content changes remain protected.

## Key decisions

- Use one shared record-schema parser for ordinary capability validation and
  archive reconciliation.
- Normalize only narrowly defined ledger formatting; preserve meaningful body
  whitespace and ordering.
- Validate both committed baseline and authored candidate before attribution.
- Keep unrelated date-sensitive runloop fixture debt as follow-up work.

## Files and areas changed

Capability-ledger parsing and Git archive reconciliation in
`internal/workflow`, focused and composed regression tests, and the normative
archive command documentation.

## Verification

Focused workflow and archive CLI tests, full Git-backed append regression, vet,
build, shell syntax, executable-mode, and diff checks passed. The full suite
retains only the two documented pre-existing date-sensitive `internal/runloop`
fixture failures.

## Review outcome

`review-02.md` approved the implementation after resolving Review 01's baseline
metadata-validation finding.

## Capability changes

Updated CAP-011 to describe record-aware reconciliation and validation of both
baseline and candidate ledgers before change attribution.

## Skipped work

None.

## Follow-up work

Repair the unrelated date-sensitive `internal/runloop` fixtures in separate
work. Git delivery remains pending integration.

## References

- `.concoct/roadmap.md` — CON-036
- `.concoct/capabilities.md` — CAP-011
- `.concoct/current/review-02.md`
