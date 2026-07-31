---
task-id: CON-009
review: 2
status: changes-requested
created: 2026-07-31
persona: reviewer
---

# Review 02

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The remediation correctly enforces non-Git summary lifecycle metadata, exact
title-derived archive paths, exact review-copy counts, and committed-baseline
preservation of unrelated roadmap and capability content. Those fixes resolve
Review 01 findings 1-3.

One correctness defect remains in Git retry handling: a clean checkout returns
success based only on a commit-subject prefix, before validating the roadmap
diff, capability diff, or resulting archived workflow state. The expanded test
suite proves a valid happy-path retry but does not exercise an invalid clean
candidate. Review 01 finding 4 is therefore only partially fixed.

## Acceptance criteria assessment

- Plain archive prompt and explicit mutation boundary: met.
- Approval and explicit override evidence: met for implemented validation.
- Exact deterministic archive path and review-copy identity: met.
- Summary schema and Git/non-Git lifecycle pairs: met.
- Authored Git capability and roadmap reconciliation: met before the initial
  commit; not re-established on a clean retry.
- Git transition identity and valid retry reuse: not met.
- Non-Git cleanup-last behavior: met for tested lifecycle failures and happy
  path.
- Failure/recovery verification: partially met; several plan-required archive
  boundary cases remain uncovered, including invalid clean retry evidence.
- Documentation, template parity, and existing behavior: met by reviewed
  changes and the passing suite.

## Findings

### Finding 1 — Clean Git retry trusts the commit subject instead of validating the transition

- Severity: major
- Status: unresolved
- Evidence: after common candidate validation, `completeGitArchive` reads the
  working-tree status at `internal/workflow/archive.go:192-196`. When it is
  clean, lines 197-200 return success if `LastCommitSubject` merely has prefix
  `concoct: archive <task-id>`. This branch occurs before
  `validateGitCapabilityDiff`, `validateGitRoadmapDiff`, and the final
  `Detect(...).State == Archived` check at lines 210-232. It also uses
  `strings.HasPrefix`, whereas `Detect` requires the exact subject. The added
  retry assertion in `internal/cli/transition_test.go:49-54` retries only the
  valid commit just created by the same test.
- Impact: a clean manually authored or partially recovered commit with a
  matching subject prefix can be reported as a successfully reused archive
  even when its roadmap transition was never validated or `Detect` reports an
  invalid/non-archived state. The CLI then prints that contradictory state but
  still returns success. This violates acceptance criteria 7, 8, and 12 and
  the requirement that retries reuse only an existing valid completed
  transition.
- Required action: make clean retry success require the exact archive commit
  subject and full validation of the committed archive transition, including
  resulting `Archived` state and exact resolved HEAD. Where diff validation
  needs the pre-transition baseline, compare the archival commit against its
  parent (or equivalent immutable evidence), not HEAD against itself. Add
  negative clean-retry tests for a subject prefix mismatch, invalid roadmap or
  capability transition, and non-archived detected state.

### Finding 2 — Archive transaction verification remains materially narrower than the plan

- Severity: major
- Status: unresolved
- Evidence: remediation adds useful unit coverage for lifecycle metadata,
  exact naming, extra reviews, projections, all impact types, and one valid Git
  retry. However, archive completion still has no focused tests for dirty,
  detached, wrong-branch, invalid-base, operation-in-progress, forbidden-path,
  malformed/missing summary sections, missing review copies, capability
  provenance failures, roadmap cross-reference failures, collision versus
  recovery, injected durable-write/cleanup failures, or integration
  continue/abort starting from an archive produced by this boundary. The
  similarly named boundary-refusal tests at
  `internal/cli/transition_test.go:166+` exercise Developer/Reviewer role
  completion, not `CompleteArchive`.
- Impact: Review 01's requested and plan-mandated verification for acceptance
  criteria 3-13 is not yet present. The surviving clean-retry defect
  demonstrates why passing generic workflow and integration suites is not
  equivalent to exercising the new destructive acceptance boundary.
- Required action: add focused archive-completion tests for the enumerated Git,
  schema, cross-reference, failure, retry, and recovery cases. Tests may reuse
  existing helpers, but must invoke the archive boundary whose guarantees they
  establish.

## Prior finding disposition

- Review 01 finding 1 — fixed. Non-Git completion requires `status: delivered`
  and `delivery: complete` before roadmap validation or cleanup, with negative
  preservation tests.
- Review 01 finding 2 — fixed for the initial authored Git transition.
  Committed-baseline projections now reject changes outside the selected
  roadmap fields and declared capability records, with focused unit tests.
- Review 01 finding 3 — fixed. Candidate lookup derives and requires the exact
  dated roadmap-ID/title slug, and extra archived review copies are rejected.
- Review 01 finding 4 — partially fixed. Coverage expanded substantially, but
  the plan-required archive transaction matrix remains incomplete and valid
  retry testing did not cover the early-return trust defect.

## Verification performed

- Inspected the full implementation diff from recorded base
  `c89529b90eb53d9628a88bb93b6e1ef670583beb` through remediation commit
  `a6c082a`, and the focused remediation diff from review commit `a7efd8f`.
- Reviewed the task plan, notes, capability ledger, Reviewer persona, prior
  review, archive workflow/CLI/Git/integration source, tests, documentation,
  and synchronized Archivist persona/template.
- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed.
- `git diff --check c89529b..HEAD` — passed before review reservation.
- Root/template Archivist persona byte comparison — passed.

## Capability impact assessment

The intended `add` of CAP-011 remains accurate, but the capability must not be
recorded as accepted while clean retries can succeed without proving the exact
archival transition and the new boundary lacks its required failure/recovery
evidence.

## Scope, documentation, and compatibility assessment

The remediation remains within CON-009 and documents the new non-Git lifecycle
pair consistently. Existing behavior and template parity remain passing. The
remaining work is narrowly within the promised archive completion and retry
contract; it does not require new product direction or scope expansion.

## Handoff

- Current state: CON-009 changes requested after post-remediation review.
- Work completed: independent disposition of all four prior findings, complete
  remediation diff inspection, and full project verification.
- Work remaining: validate clean Git retries as exact completed transitions
  and finish the archive-specific negative/failure/recovery test matrix.
- Decisions made: Review 01 findings 1-3 are fixed; finding 4 is partially
  fixed; CAP-011 remains pending.
- Known risk: a clean commit with a matching subject prefix can currently
  produce a successful command result without valid archived state.
- Artifacts created: `.concoct/current/review-02.md` only.
- Expected next role: Developer in remediation mode.
- Recommended next command: `concoct code`.
