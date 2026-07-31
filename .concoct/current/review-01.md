---
task-id: CON-009
review: 1
status: changes-requested
created: 2026-07-31
persona: reviewer
---

# Review 01

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The implementation establishes the explicit `archive --complete` surface and
passes the existing full verification suite, but it does not yet satisfy the
transactional validation contract. Non-Git completion can accept contradictory
summary state and then irreversibly clear current evidence, Git completion can
commit unrelated roadmap edits, and the candidate path check does not establish
the deterministic archive destination promised by the plan.

These are acceptance-boundary defects within Developer scope. Remediation also
needs focused negative and recovery coverage; the three new archive tests cover
only a non-Git happy path, override matching, and a Git happy-path integration.

## Acceptance criteria assessment

- Explicit completion while plain `concoct archive` remains prompt-only: met.
- Approval and explicit override gating: partially met; command and summary
  fields are checked, but negative-path coverage is incomplete.
- Exact deterministic archive destination and append-only history: not met.
- Complete, lifecycle-consistent summary validation: not met for non-Git.
- Capability reconciliation and unrelated-content preservation: partially met;
  declared Git capability records are checked, but content outside records and
  unrelated roadmap changes are not protected.
- Git pending-delivery archive and integration compatibility: happy path met;
  failure, retry, and recovery guarantees are insufficiently established.
- Non-Git durable validation before cleanup: not met because summary lifecycle
  state is not validated before cleanup.
- Source/template parity and regression verification: met for the documented
  checks run during this review.

## Findings

### Finding 1 — Non-Git completion accepts contradictory summary lifecycle state

- Severity: major
- Status: unresolved
- Evidence: `validateArchiveCandidate` parses `status` and `delivery` at
  `internal/workflow/archive.go:114-130`, but the non-Git path at lines 70-77 and
  `validateNonGitDelivery` at lines 224-228 validate only roadmap evidence.
  Unlike the Git path at lines 175-177, no non-Git check constrains either
  summary field. The accepted fixture happens to use `status: delivered`, but
  no negative test asserts the contract.
- Impact: an Archivist can supply `status: archived`, `delivery:
  pending-integration`, arbitrary values, or omitted values in a non-Git
  summary; completion will still delete `.concoct/current/` and report `ready`.
  The durable summary then contradicts the completed lifecycle after the only
  active recovery evidence has been removed, violating acceptance criteria 5,
  7, 11, and 12.
- Required action: define and enforce the exact non-Git summary status/delivery
  pair before cleanup, and add negative tests proving contradictory or missing
  values preserve current state and delivery evidence.

### Finding 2 — Git archival can commit unrelated roadmap and capability-ledger content

- Severity: major
- Status: unresolved
- Evidence: the allowed-path check permits the whole `.concoct/roadmap.md` and
  `.concoct/capabilities.md` files (`internal/workflow/archive.go:191-197`).
  `validateRoadmapEvidence` at lines 231-248 validates only CON-009's parsed
  status and archive reference, so arbitrary changes to other roadmap items or
  surrounding text pass. `validateGitCapabilityDiff` at lines 294-315 compares
  only `## CAP-NNN` sections; front matter, introductory contract text, and
  other non-record content can change without being classified. No new test
  exercises unrelated roadmap or non-record capability changes.
- Impact: `archive --complete` can commit out-of-scope product direction or
  capability-contract edits in the supposedly coherent archival transition.
  This violates the explicit byte-preservation constraint and acceptance
  criteria 6 and 12, and makes the CLI's successful result stronger than the
  evidence it actually validated.
- Required action: compare authored roadmap/capability content against the
  committed baseline with only the selected roadmap fields and declared
  capability records permitted to differ. Reject every other change and add
  tests for unrelated records, front matter, prose, ordering, and roadmap
  items.

### Finding 3 — Archive path validation accepts any same-day task prefix

- Severity: major
- Status: unresolved
- Evidence: `validateArchiveCandidate` constructs only
  `<date>-<task-id>-` and accepts whichever single wildcard match exists
  (`internal/workflow/archive.go:82-89`). It never derives or compares the
  title-based slug, and the otherwise unused `taskData` parameter is discarded
  at line 155. The tests author hard-coded suffixes and do not test a wrong
  suffix or collision/retry distinction.
- Impact: a misspelled or arbitrary archive name is accepted as the
  "deterministic" destination, weakening stable cross-references and making a
  pre-existing conflicting directory indistinguishable from the intended
  candidate. This does not meet acceptance criterion 4 or the task's
  create-only/collision requirement.
- Required action: use one canonical archive-path derivation from archival
  date, task ID, and task title; require that exact path, and distinguish a
  valid authored/retry candidate from a collision with focused tests.

### Finding 4 — Verification does not cover the plan's transaction guarantees

- Severity: major
- Status: unresolved
- Evidence: `internal/workflow/archive_test.go` adds only two tests and
  `internal/cli/transition_test.go` adds one Git happy-path test. There are no
  new tests for `update`, `remove`, or `none` reconciliation at this boundary;
  dirty, detached, wrong-branch, invalid-base, operation-in-progress, forbidden
  path, exact retry, collision, partial failure, cleanup failure, roadmap
  preservation, summary schema failures, extra/missing review copies, or
  integration continue/abort from the newly produced archive. These cases are
  explicitly required by the task plan.
- Impact: the full suite passing does not establish acceptance criteria 3-13,
  and the defects above reached implementation-complete unnoticed. Archival is
  a destructive/acceptance-sensitive boundary, so happy-path coverage is not
  sufficient evidence for approval.
- Required action: add the focused table-driven, real-Git, non-Git, failure
  injection, retry, and integration recovery tests enumerated by the plan,
  including regression cases for every material finding in this review.

## Prior finding disposition

No prior reviews exist for CON-009.

## Verification performed

- Inspected the complete diff from recorded base
  `c89529b90eb53d9628a88bb93b6e1ef670583beb` through Developer commit
  `985e56ca49bf6bad4fbfb0e7447e2973982d7f8b`, including workflow, CLI, Git,
  integration-facing behavior, tests, documentation, personas, prompts, and
  root/template counterparts.
- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed.
- `git diff --check c89529b..HEAD` — passed before creation of this review.
- Confirmed the task branch is attached and clean at the reviewed Developer
  commit before reservation of this review.

## Capability impact assessment

The intended impact remains `add` CAP-011, but CAP-011 is not ready to enter the
accepted capability ledger. The implementation provides a substantial portion
of validated archival coordination, yet the observable guarantee currently
overstates lifecycle consistency, unrelated-content preservation, deterministic
destination enforcement, and recovery verification. The Archivist must not add
CAP-011 until these findings are resolved and independently approved.

## Scope, documentation, and compatibility assessment

The implementation is generally scoped to CON-009 and keeps plain archive
rendering separate from mutation. Root and embedded workflow assets changed by
the task are synchronized, and the `self` sentinel is documented consistently.
The material concerns are correctness and verification gaps at the acceptance
boundary, not unrelated feature expansion. Existing integration passes its
happy-path regression suite, but compatibility under the required retry and
recovery cases remains unproven.

## Handoff

- Current state: CON-009 changes requested after independent review.
- Work completed: complete diff and artifact inspection; full test, vet, build,
  wrapper syntax, and whitespace checks; four material findings recorded.
- Work remaining: enforce non-Git summary lifecycle state, preserve all
  unrelated roadmap/capability content, require the exact deterministic archive
  path, and add the plan-required negative/retry/recovery test matrix.
- Decisions made: CAP-011 must not be added yet; all requested changes remain
  within the approved task and Developer ownership.
- Known risks: non-Git cleanup can currently preserve contradictory durable
  truth, while Git archival can commit unvalidated product/capability edits.
- Artifacts created: `.concoct/current/review-01.md` only.
- Expected next role: Developer in remediation mode.
- Recommended next command: `concoct code`.
