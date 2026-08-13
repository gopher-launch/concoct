---
task-id: CON-033
review: 2
status: changes-requested
created: 2026-08-12
persona: reviewer
---

# Review 02

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The remediation fixes the one-occurrence approval and symlinked-runtime
defects, and it adds a sound outer-finalization boundary for supervised role
candidates. The full existing suite, focused race tests, build, vet, shell,
diff, and generated-project checks pass. However, Review 01 Finding 3 is only
partially resolved: the coordinator and supervision tests still omit several
boundaries explicitly required by the task plan and the prior review. The
remaining verification gap is material for a new multi-action coordinator, so
approval is not yet warranted.

## Finding

### Finding 1 — Required coordinator and prompt-boundary matrix remains incomplete

- Severity: `major`
- Status: `open` (continues Review 01 Finding 3)
- Evidence: `internal/runloop/run_test.go` now covers Product Owner proposal
  drift and interventions, planning context, a fresh Reviewer invocation,
  repeated development/review gates, action/cycle bounds, structured
  non-completion classes, cancellation, process failure, and a successful
  real-Git archive/integration path. It still has no coordinator-level case
  for non-required or externally satisfied review routing, repeated
  action/evidence no-progress detection, planning startup rollback and
  preservation after possible agent mutation, or an integration conflict that
  must stop in recovery with `--continue`/`--abort`. Those cases are expressly
  required by Task Plan Phase 6 and the Verification section, and Review 01
  required them at the coordinator boundary. Package-level workflow,
  execution, planning, or integration tests do not exercise the run loop's
  summary, bound accounting, and continuation behavior at those joins.
  Separately, `internal/execution/execution_test.go:215-243` checks that one
  Developer prompt contains both a Developer heading and supervision wording,
  but does not prove that each supervised role receives the byte-identical
  manual prompt as a prefix followed only by the fixed appendix, as required
  by acceptance criterion 22 and the Verification section.
- Impact: the added tests catch the approval-reuse defect from Review 01 and
  establish the main happy path, but important stop/recovery and policy routes
  remain protected mainly by inspection and lower-layer tests. A regression in
  coordinator-specific state reporting or continuation selection across those
  boundaries could pass the current suite. Prompt-core drift for Planner,
  Reviewer, or Archivist could likewise pass while violating manual/supervised
  parity.
- Required action: complete the agreed boundary matrix with focused
  coordinator tests for the omitted policy, no-progress, planning rollback or
  preservation, and integration-recovery outcomes, asserting the resulting
  summary and safe continuation. Add exact byte-level prompt-core/appendix
  coverage for every supervised role. The implementation need not be changed
  if those tests confirm the current behavior.

## Acceptance criteria assessment

- Criteria 1-21 and 23-24 have credible implementation evidence and passing
  coverage for the primary behavior, including the complete real-Git local
  lifecycle, canonical outer commits, repeated fresh gates, partial-work
  preservation, forbidden-effect rejection, and remote-push suppression.
- Criterion 22 is only partially evidenced because the test asserts prompt
  substrings for one supervised role rather than byte-identical manual cores
  plus only the fixed appendix for all supervised roles.
- The task's explicit verification contract is not complete for criteria 9,
  13, 14, and 19 at the coordinator boundary, despite the corresponding source
  paths and lower-level tests appearing plausible.

## Prior finding disposition

- Review 01 Finding 1 — `fixed`. Approval state and attempt correlation are
  cleared before each executed action occurrence. Chained plan/development
  gates carry only the consumed immediate prerequisite, and focused tests prove
  later Developer and Reviewer occurrences receive fresh gate attempt IDs.
- Review 01 Finding 2 — `fixed`. Pending-gate create, load, and consume validate
  `.concoct` and `runtime` with `Lstat`, reject symlinks and non-directories,
  and the new test confirms a symlink target is not chmodded or populated.
- Review 01 Finding 3 — `partially fixed`. The suite is substantially expanded
  and now covers the main lifecycle and failure classes, but the specific
  coordinator and prompt-parity omissions in Finding 1 remain.

## Verification performed

- Inspected the complete diff from recorded base
  `b822466e9c969576cc5365460de6c84d2e8b4733` through remediation commit
  `9ffe44c58642`, the remediation-only diff, relevant source, tests,
  documentation, task evidence, prior review, and capability ledger.
- `go test -count=1 ./...` with an isolated build cache — passed.
- `go test -race -count=1 ./internal/runstate ./internal/runloop
  ./internal/execution ./internal/integration` with an isolated build cache —
  passed.
- `go vet ./...` — passed.
- `go build -o /tmp/concoct-review-bin ./cmd/concoct` — passed; Go emitted the
  documented non-fatal read-only module stat-cache warning.
- `bash -n cmd/concoct/concoct.sh` — passed; executable mode remains present.
- `git diff --check b822466e9c969576cc5365460de6c84d2e8b4733..HEAD`
  — passed.
- Source/template Concoct skill comparison — identical.
- Fresh initialization under `/tmp` — passed: Git repository, staged dotfiles
  and nested adapters, planning directories, project contract and bootstrap
  evidence, pending-gate ignore coverage, no generated commit, and no installed
  built-in protocol/persona/handoff directories.

## Capability impact assessment

The declared `add` impact remains accurate once the remaining verification
finding is resolved. Archival should add CAP-014 for bounded repeated lifecycle
execution with restrictive gates, supervised canonical transitions,
independent review, progress detection, intervention reporting, and local-only
integration. CAP-013 remains the compatible one-shot primitive. Capability
truth must remain unchanged before approval.

## Scope and documentation assessment

The implementation remains focused on CON-033 and does not pull in CON-034
crash recovery, concurrent tasks, automatic retries, arbitrary gates, sandbox
bypass, or remote push. Documentation accurately describes one-occurrence
gates, supervised outer finalization, partial-work behavior, and local-only
integration. No material documentation defect was found.

## Handoff

- Current outcome: changes requested.
- Work remaining: complete the coordinator and exact prompt-parity verification
  described in Finding 1; change production code only if those tests expose a
  defect.
- Expected next role: Developer.
- Recommended next command: `concoct code`.
- Suggested next review focus: confirm every omitted boundary is exercised at
  the run-loop surface, all supervised prompt cores are byte-identical to their
  manual renderings, and the full verification set remains green.
