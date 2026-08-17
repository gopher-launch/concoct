---
task-id: CON-038
review: 1
status: changes-requested
created: 2026-08-14
persona: reviewer
---

# Review 01

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The implementation establishes useful payload-safe activity evidence, bounded
raw retention, hard elapsed-budget termination, accepted/wasted accounting,
role-fixture evidence, and a materially smaller Archivist persona. The amended
benchmark contract is represented honestly: the live observations are labelled
directional and no causal reduction is claimed.

Two material execution-evidence defects prevent approval. Observable warning
budgets are evaluated only after a successful adapter exit, rather than live or
on failure paths, and command classification labels every invocation of several
general-purpose executables as a test/check. Both conflict with the task's
warning and conservative-normalization acceptance criteria, and the required
offline coverage for these paths is absent.

## Acceptance criteria assessment

- Payload-safe default metrics, bounded repetition fingerprints, command-output
  counts, usage snapshots, accepted/wasted disposition, and finalization
  diagnostics are implemented and documented.
- Raw structured events default to disabled and are independently bounded,
  private, redacted, and ignored by Git.
- Hard elapsed budgets use bounded process-group termination and produce the
  distinct `budget-exhausted` disposition with partial measurement. The full
  hard-budget contract is not sufficiently covered for activity,
  command-output, or late-result races.
- The deterministic report records a 54.1 percent Archivist-persona reduction,
  satisfying the amended deterministic threshold. Live results and their
  failed-finalization limitations are stated without a causal claim.
- Offline fixture completion and the broad test suite pass, but warning
  behavior and conservative command mapping do not meet the contract below.

## Findings

### Finding 1 — Observable warning budgets are not evaluated live or preserved on failure

- Severity: major
- Status: unresolved
- Evidence: `internal/execution/execution.go:448-509` polls only hard budgets
  while the adapter runs. `appendTerminalWarnings` is called only on the
  successful-exit path at lines 510-515. Nonzero exit, timeout, cancellation,
  and hard-budget termination return before it. Lines 557-559 also label
  elapsed, activity, and command-output warnings as `terminal`, although all
  three measurements are observable during execution. The only budget behavior
  test, `TestHardElapsedBudgetStopsWithDistinctDisposition`, covers a hard
  elapsed stop and no warning path.
- Impact: users receive no warning evidence during a long-running invocation,
  and an invocation that later times out, fails, or reaches a hard limit can
  lose evidence that its warning threshold was crossed. This violates the
  acceptance requirement that warnings be evaluated as values become
  observable and visibly distinguish live from terminal-only evaluation.
- Required action: evaluate and record each live-observable warning once when
  its threshold is crossed, preserve that evidence on every terminal
  disposition, retain token warnings as terminal-only, and add tests covering
  successful, failing, and hard-stopped paths without duplicate warning events.

### Finding 2 — General-purpose commands are reported as tests/checks without evidence

- Severity: major
- Status: unresolved
- Evidence: `internal/adapter/metrics.go:321-336` classifies every simple command
  whose first word is `go`, `bash`, `sh`, `make`, `npm`, or `cargo` as
  `test-check`. Commands such as `go env`, `bash migration.sh`, `sh read-data.sh`,
  or `npm publish` provide no evidence that a test or check occurred. The sole
  normalization test exercises file reads, an ambiguous pipeline, and a file
  change; it does not test positive and negative check classification.
- Impact: normalized activity and any analysis based on it can overstate
  verification work and understate generic command activity. This contradicts
  the plan's conservative-classification constraint and its explicit rule not
  to claim a test/check from an unsupported heuristic.
- Required action: narrow check classification to command forms that provide
  affirmative check/test evidence, otherwise use `command-other`, and add
  representative positive, negative, and ambiguous mapping tests.

## Verification performed

- `GOCACHE=/tmp/concoct-review-go-cache go test -count=1 ./...` — passed.
- `GOCACHE=/tmp/concoct-review-go-cache go test -race -count=1 ./internal/adapter ./internal/prompt ./internal/execution ./internal/runloop` — passed.
- `GOCACHE=/tmp/concoct-review-go-cache go vet ./...` — passed.
- Native and `GOOS=windows GOARCH=amd64` builds of `./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` and executable-mode check — passed.
- External temporary-project initialization — passed; initialization reported
  ready state, staged generated files, and no generated commit, with required
  project-owned files and exclusions checked.
- `git diff --check main...HEAD` — passed.
- The opt-in live benchmark was not rerun, consistent with the explicit
  operator cost stop and amended acceptance contract.

## Capability impact assessment

The declared `update` impact to CAP-013, CAP-014, and CAP-015 is appropriate in
shape, but archival wording must not claim conforming live warning evidence or
fully conservative activity attribution until the findings are resolved. The
CAP-015 stale CON-037 report reference should still be reconciled during
archival after approval.

## Scope and documentation assessment

The implementation remains within CON-038's execution-cost, evidence,
budgeting, role-guidance, and benchmark scope. The operator-approved acceptance
amendment and paid-run limitations are preserved clearly. Documentation is
generally comprehensive, but the claim in `doc/command-reference.md:351-352`
that warnings record live versus terminal-only evaluation overstates current
runtime behavior described in Finding 1.

## Handoff

Return to the Developer with `concoct code`. Resolve both findings, add focused
warning/failure and conservative-classification coverage, then rerun the broad
and focused race checks. The next review should concentrate on warning-event
deduplication and preservation across every disposition, classification false
positives, hard activity/command-output stops, and late-result rejection.
