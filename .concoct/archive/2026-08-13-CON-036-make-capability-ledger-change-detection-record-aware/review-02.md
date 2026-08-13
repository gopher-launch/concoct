---
task-id: CON-036
review: 2
status: approved
created: 2026-08-13
persona: reviewer
---

# Review 02

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`approved`

## Summary

The remediation closes Review 01's sole major finding. Git-backed capability
reconciliation now validates required record metadata in both the committed
baseline and authored candidate before attributing changes, using the same
record-schema helper as ordinary capability parsing. The record-aware boundary
normalization, strict undeclared-content protection, and impact-shape behavior
remain supported by focused and composed regression coverage.

## Acceptance criteria assessment

Acceptance criteria 1-9 are met by the ordered ledger representation, shared
record-schema validation, and focused append, insertion, removal, separator,
line-ending, final-newline, malformed-ledger, ordering, and unauthorized-change
tests. The Git-backed `CompleteArchive` append regression exercises the reported
failure through the composed archive boundary. Criterion 10 is met for the
affected archive, workflow, and CLI behavior. The full suite retains only the
two documented date-sensitive `internal/runloop` fixture failures, which occur
before integration and are unrelated to this task's capability-ledger changes.

## Findings

No blocking findings.

## Prior finding disposition

### Review 01, Finding 1 — fixed

- Evidence: `validateGitCapabilityDiff` parses both baseline and candidate
  ledgers, applies `parseCapabilityLedgerRecords` to both, and returns a
  side-specific invalid-ledger diagnostic before section comparison or
  undeclared-content attribution. `parseCapabilities` uses the same helper, so
  ordinary workflow validation and archive reconciliation share the required
  record metadata schema.
- Regression: `TestValidateGitCapabilityDiffRejectsBaselineMissingRequiredMetadata`
  commits a baseline record without `Status`, authors an otherwise valid
  candidate, and requires the actionable `invalid baseline capabilities` and
  `CAP-001 missing Status` diagnostic.
- Assessment: the required remediation outcome is satisfied without broadening
  normalization or changing capability-impact semantics.

## Verification performed

- Inspected the complete base-to-HEAD diff, remediation commit, parser and
  archive-validation paths, focused tests, task plan, notes, prior review,
  command documentation, and capability ledger.
- `GOCACHE=/tmp/concoct-review02-go-cache go test -count=1 ./internal/workflow`
  — passed.
- `GOCACHE=/tmp/concoct-review02-go-cache go test -count=1 ./internal/cli -run
  'GitArchive|ArchiveCompletion'` — passed.
- `GOCACHE=/tmp/concoct-review02-go-cache go vet ./...` — passed.
- `GOCACHE=/tmp/concoct-review02-go-cache go build ./cmd/concoct` — passed,
  with a non-fatal warning when Go attempted to update its read-only shared
  module stat cache.
- `bash -n cmd/concoct/concoct.sh`, executable-mode verification, and
  `git diff --check 160f5b1ce86629a779247dc095e4c88ea1dfe900..HEAD` — passed.
- `GOCACHE=/tmp/concoct-review02-go-cache go test -count=1 ./...` — failed only
  at `TestRealGitLifecycleReachesLocalIntegrationWithoutPush` and
  `TestIntegrationConflictStopsInRecoveryWithExactContinuation` in
  `internal/runloop`. Both reject the fake Archivist's current-date roadmap
  update against fixtures pinned to the preceding date, matching the prior
  review and Developer evidence. This does not affect approval of CON-036.
- Template and initialization checks were not run because no template or
  initialization surface changed.

## Capability impact assessment

The planned `update` to CAP-011 is accurate. This task corrects existing
validated archive/capability reconciliation behavior; it does not add or remove
a distinct capability. The Archivist should update CAP-011 with the accepted
record-aware comparison behavior and CON-036 archive provenance.

## Scope and documentation assessment

The implementation remains scoped to capability-ledger parsing, Git archive
reconciliation, regression tests, and the corresponding normative command
documentation. It preserves archive transaction, roadmap, integration,
capability-impact, and non-Git contracts. The documentation accurately states
the narrow formatting equivalences and continued protection of record order,
record content, and ledger-level content.

## Risks and follow-up

The unrelated date-sensitive `internal/runloop` fixtures should be repaired in
separate work so the full suite remains deterministic across calendar dates.
This is non-blocking for CON-036 because the failure predates the implementation
and occurs outside the changed capability-ledger path.

## Handoff

Run `concoct archive`. Approval is based on the fixed baseline-validation gap,
passing affected checks, preserved strict comparison behavior, and accurate
CAP-011 update impact. The Archivist should retain the unrelated runloop fixture
debt as a follow-up rather than folding it into this archive.
