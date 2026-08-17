---
task-id: CON-038
review: 2
status: changes-requested
created: 2026-08-14
persona: reviewer
---

# Review 02

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

Both findings from Review 01 are resolved in the implementation. Warning
events are now deduplicated, retained across the common terminal paths, and
separated from terminal-only token warnings. Command classification now
requires affirmative test/check arguments and has representative positive,
negative, and ambiguous coverage.

Approval is still withheld because the enforceable-budget acceptance contract
is not fully verified. Only hard elapsed termination is exercised. No test
drives either hard activity or hard command-output exhaustion, and no test
proves that a result racing with one of those hard-stop decisions remains
rejected. These are public, independently configured enforcement paths and
were an explicit acceptance and prior-review focus.

## Acceptance criteria assessment

- The two Review 01 defects are corrected as described under prior finding
  disposition.
- The broad deterministic suite, focused remediation tests, race checks, vet,
  native and Windows builds, shell validation, diff validation, and external
  initialization all pass.
- The operator-approved amendment is represented honestly. No further paid
  benchmark was run, and unlike live workloads are not presented as causal
  reduction evidence.
- Payload-safe measurement, bounded evidence, accepted/wasted accounting,
  raw-retention defaults, role-guidance compaction, and the documented 54.1%
  Archivist persona reduction remain supported by the inspected code, tests,
  and report.
- The criterion requiring offline coverage of safe hard stops and late-result
  races is only partially met: hard elapsed has a focused stop test, while hard
  activity and hard command-output do not.

## Findings

### Finding 1 — Enforceable activity and command-output stops lack execution coverage

- Severity: major
- Status: unresolved
- Evidence: `internal/execution/execution.go` independently enforces
  `HardElapsed`, `HardActivity`, and `HardCommandOutput` in the live polling
  loop and terminal check. In `internal/execution/execution_test.go`,
  `TestHardElapsedBudgetStopsWithDistinctDisposition` is the only focused hard
  budget execution test. Repository-wide searches for `hard-activity`,
  `hard-command-output`, `HardActivity`, and `HardCommandOutput` find only
  configuration parsing assertions outside production code; they do not drive
  either execution stop. No hard-budget test writes or races a candidate result
  against an activity or command-output stop. The task acceptance criteria
  explicitly require offline tests for safe hard stops and late-result races,
  and Review 01 identified these paths as next-review focus.
- Impact: regressions in two of the three advertised enforceable dimensions
  could terminate incorrectly, accept a stale candidate, lose partial
  measurement, or advance workflow state without detection. Configuration
  tests and the elapsed-only path do not establish those dimension-specific
  decoder and byte-count triggers.
- Required action: add deterministic execution tests that cross the hard
  activity and hard command-output thresholds and verify the distinct
  `budget-exhausted` disposition, correct dimension/source/observation,
  preserved partial measurement, rejected result, and unchanged workflow
  state. Include a controlled late-candidate race for at least these event-
  driven stop paths, and rerun the focused race and broad suites.

## Prior finding disposition

- Review 01 Finding 1 — fixed. `warningTracker` samples live-observable
  elapsed, activity, and command-output values, records each dimension once,
  and attaches those events through the shared close path used by success,
  nonzero exit, cancellation, timeout, and budget termination. Terminal token
  warnings remain separately labelled `terminal-only`. The focused completed,
  nonzero-exit, and hard-stop cases pass repeatedly without duplicates.
- Review 01 Finding 2 — fixed. `isCheckCommand` now recognizes explicit
  `go`, `npm`, `cargo`, and `make` test/check forms rather than executable names
  alone. Positive checks, general-purpose negative cases, and an ambiguous
  pipeline are covered and pass repeatedly.

## Verification performed

- `GOCACHE=/tmp/concoct-review02-go-test go test -count=1 ./...` — passed.
- `GOCACHE=/tmp/concoct-review02-go-race go test -race -count=1 ./internal/adapter ./internal/prompt ./internal/execution ./internal/runloop` — passed; run-loop was also rerun independently with the race detector.
- `GOCACHE=/tmp/concoct-review02-go-vet go vet ./...` — passed.
- Native and `GOOS=windows GOARCH=amd64` builds of `./cmd/concoct` — passed; the Go tool emitted non-fatal read-only module stat-cache warnings.
- The warning remediation test and conservative-classification test each
  passed for 20 repetitions.
- `bash -n cmd/concoct/concoct.sh`, executable-mode validation, and
  `git diff --check` against the recorded task base — passed.
- Fresh initialization under `/tmp/concoct-review02-init-DMmhkb/generated` —
  passed: project-owned dotfiles and nested templates were staged, current and
  archive directories and the bootstrap prompt were present, built-in
  protocol/persona resources were excluded, Git was initialized, and no
  commit was created.
- The billable live benchmark was not rerun, consistent with the explicit
  operator cost stop and amended acceptance contract.

## Capability impact assessment

The declared `update` impact to CAP-013, CAP-014, and CAP-015 remains accurate
in shape. Archival wording should describe all three hard-budget dimensions as
supported only after the missing execution safety coverage passes, retain the
non-causal live-benchmark limitation, and reconcile CAP-015's stale CON-037
report filename.

## Scope and documentation assessment

The implementation remains within CON-038 scope, and the checked-in
documentation accurately distinguishes conservative normalization, live and
terminal-only warnings, hard-budget stops, privacy boundaries, and directional
benchmark observations. The remaining issue is verification of advertised
behavior rather than documentation or scope drift.

## Handoff

Return to the Developer with `concoct code`. Add the missing hard activity,
hard command-output, and late-result execution coverage without another paid
model benchmark, then rerun the focused race and broad deterministic checks.
The next review should concentrate on stop-decision precedence, partial
measurement preservation, and unchanged workflow state for both event-driven
hard-budget dimensions.
