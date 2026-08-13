---
task-id: CON-036
review: 1
status: changes-requested
created: 2026-08-13
persona: reviewer
---

# Review 01

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The record-boundary correction is well scoped and its
primary append, insertion, removal, separator, and newline behavior is covered,
but the implementation does not satisfy the explicit requirement to reject a
malformed baseline ledger before diff attribution.

## Acceptance criteria assessment

Criteria 1-7 and 9 are supported by the implementation and focused regression
tests. Criterion 8 is only partially met: duplicate and malformed headings are
rejected in both ledgers, and current declared results retain metadata checks,
but required record metadata is not validated in the baseline ledger. Criterion
10 is met for the affected workflow and CLI surfaces; the full suite still has
the two unrelated, pre-existing date-sensitive runloop failures documented by
the Developer and reproduced during review.

## Findings

### Finding 1 — Baseline records bypass required metadata validation

- Severity: major
- Status: unresolved
- Evidence: `validateGitCapabilityDiff` parses the committed baseline only with
  `parseCapabilityLedger` (`internal/workflow/archive.go:525-529`). That parser
  reports malformed capability-like headings and duplicate IDs, but it does not
  validate required record metadata (`internal/workflow/workflow.go:1113-1148`).
  The missing-`Status` diagnostic exists only in `parseCapabilities`
  (`internal/workflow/workflow.go:1159-1167`), which is not applied to the
  baseline here. `validateCapabilityResult` at the end validates only the
  authored result and only the task's declared IDs. The new negative tests cover
  malformed headings and duplicates but do not cover a baseline record missing
  required metadata.
- Impact: a committed baseline with a structurally recognized record but missing
  required metadata can reach record comparison instead of failing safely as an
  invalid baseline. This violates acceptance criterion 8 and the design
  constraint that the archive boundary must not accept a structure ordinary
  workflow validation considers malformed.
- Required action: validate required capability-record structure and metadata
  for both baseline and candidate ledgers before change attribution, using the
  shared schema authority, and add a regression proving an invalid baseline is
  rejected with an actionable baseline diagnostic. Preserve the existing narrow
  boundary normalization and impact semantics.

## Verification performed

- `GOCACHE=/tmp/concoct-review-go-cache go test -count=1 ./internal/workflow` — passed.
- `GOCACHE=/tmp/concoct-review-go-cache go test -count=1 ./internal/cli -run 'GitArchive|ArchiveCompletion'` — passed.
- `GOCACHE=/tmp/concoct-review-go-cache go vet ./...` — passed.
- `GOCACHE=/tmp/concoct-review-go-cache go build ./cmd/concoct` — passed, with a
  non-fatal read-only module stat-cache warning.
- `bash -n cmd/concoct/concoct.sh`, executable-mode check, and
  `git diff --check` — passed.
- `GOCACHE=/tmp/concoct-review-go-cache go test -count=1 ./...` — failed only in
  `internal/runloop` at `TestRealGitLifecycleReachesLocalIntegrationWithoutPush`
  and `TestIntegrationConflictStopsInRecoveryWithExactContinuation`; both fail
  at the documented roadmap `updated` date mismatch before integration. This is
  unrelated to the capability-ledger diff implementation and does not add a
  finding for this task.
- Inspected the complete base-to-HEAD diff, parser and archive-validation paths,
  focused tests, task plan, notes, documentation, and capability ledger.

## Capability impact assessment

The planned `update` to CAP-011 remains accurate after remediation. The task
changes existing validated archive/capability reconciliation behavior and does
not add or remove a capability. The capability ledger itself should remain
unchanged until archival.

## Scope and documentation assessment

The implementation stays within capability-ledger comparison and its normative
documentation. No unrelated source, template, initialization, roadmap, or
capability changes were introduced. The documentation accurately describes the
intended normalization behavior; no documentation correction is required for
the finding.

## Handoff

Run `concoct code`. The Developer should close Finding 1 by applying the shared
required-metadata validation to both baseline and candidate evidence and adding
focused invalid-baseline coverage. The next review should recheck that failure
path and rerun the existing boundary-regression matrix.
