---
task-id: CON-033
review: 1
status: changes-requested
created: 2026-08-12
persona: reviewer
---

# Review 01

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The implementation establishes the intended reusable execution, planning,
configuration, pending-state, coordinator, and local-integration boundaries,
and all existing repository checks pass. However, optional action gates are
remembered for the full run instead of consumed for one action occurrence. A
changes-requested cycle can therefore execute later Developer or Reviewer
actions without the configured fresh approval. The private pending-state path
also follows a symlinked runtime directory, and the new tests omit most of the
task's required lifecycle and failure matrix. These issues leave the approval,
safety, and verification acceptance contract incomplete.

## Findings

### Finding 1 — One action-gate approval authorizes later same-kind actions

- Severity: `major`
- Status: `open`
- Evidence: `internal/runloop/run.go:98` stores consumed approvals in maps keyed
  only by gate and action kind. After consumption, lines 212-225 set
  `consumed[gateName]` and retain the gate attempt in
  `actionAttempts[resolution.Kind]`; lines 212-215 then skip that gate for the
  rest of the invocation, and lines 271-274 reuse the original attempt ID.
  Development and independent-review can both recur after a
  `changes-requested` review. `doc/command-reference.md` says each approval
  names exactly one current gate and is consumed once, while acceptance
  criterion 8 scopes approval to the current action, attempt, and evidence and
  forbids pre-authorization of a later gate.
- Impact: with a configured `development` or `review` gate, approving the first
  occurrence silently authorizes all later occurrences in the same run. A
  remediation Developer or subsequent Reviewer therefore starts against new
  workflow evidence without a new approval, defeating the restrictive gate
  selected by the project or invocation and reusing stale approval
  correlation.
- Required action: scope a consumed approval and its attempt correlation to
  exactly one action occurrence. A later gated action of the same kind must
  create and require a fresh evidence-bound pending gate. Add a
  changes-requested/remediation test that proves repeated development and
  review gates each stop again and cannot reuse the earlier approval.

### Finding 2 — Pending-gate storage follows a symlinked runtime directory

- Severity: `major`
- Status: `open`
- Evidence: `internal/runstate/gate.go:207-211` implements
  `ensurePrivateDir` with `os.MkdirAll` followed by `os.Chmod`, both of which
  follow an existing symlink in `.concoct/runtime`. `Create` then creates and
  links files through that directory at lines 69-94, and `Consume` creates and
  renames claim files there at lines 148-176. `loadPath` rejects a symlink only
  at the final pending-gate file; it does not reject a symlinked parent. The
  existing invocation-state helper in `internal/execution/execution.go:838-850`
  does perform a non-symlink directory check, showing the established runtime
  boundary. No runstate test covers a symlinked parent.
- Impact: a repository containing a symlinked runtime directory can cause
  `concoct run` to chmod an external directory and create, consume, or replace
  pending evidence outside the project. This violates acceptance criterion
  18's private and safe pending-state boundary and can mutate an unintended
  filesystem target before the gate stops the run.
- Required action: reject symlinked or non-directory runtime path components
  before any chmod, temporary-file creation, link, or rename. Cover create,
  load, and consume behavior with parent-symlink tests and confirm no external
  path is changed.

### Finding 3 — Coordinator verification omits the required lifecycle matrix

- Severity: `major`
- Status: `open`
- Evidence: `internal/runloop/run_test.go` contains four tests: proposal drift,
  one accepted Product Owner intervention, approved planning, and a
  Developer-to-Reviewer path that ends in a recoverable Reviewer outcome.
  `internal/runstate/gate_test.go` contains two tests. There is no successful
  review/remediation/archive/integration lifecycle test, no repeated optional
  gate test, and no coordinator coverage for action or cycle exhaustion,
  no-progress detection, cancellation/process failures, integration recovery,
  non-required or externally satisfied review routing, or planning rollback.
  The task's Phase 6 and Verification sections explicitly require those cases,
  including real-Git happy paths and recovery boundaries.
- Impact: the full suite and focused race suite pass, but they do not exercise
  most of the new 439-line coordinator's termination and authority behavior.
  Finding 1 is an example of a contract violation that the documented test
  matrix would have caught. Acceptance of the loop, safety stops, bounds,
  compatibility routes, and local delivery would rest largely on inspection
  rather than durable regression evidence.
- Required action: implement the task-plan verification matrix at the
  coordinator boundary, including successful local delivery, productive
  changes-requested cycles, repeated gated actions, bounds/no-progress, all
  stop classes, policy review routes, planning rollback/prompt parity, Git
  integration recovery, approval drift, and no-push behavior. Retain focused
  race coverage for gate consumption and coordinator state.

## Acceptance criteria assessment

- Criteria 1-7 and 9 have plausible implementation paths and partial focused
  evidence, including shared planning setup, fresh invocations, monotonic
  policy, and local-only integration.
- Criterion 8 is not met because one optional action-gate approval applies to
  later occurrences with different workflow evidence.
- Criterion 10 is only partially met: changes-requested state can progress,
  but configured gates are bypassed on the repeated actions.
- Criteria 11-17 are represented in source, but the required boundary,
  stop-class, summary, recovery, and end-to-end verification is materially
  incomplete.
- Criterion 18 is not met for a symlinked private runtime parent.
- Criterion 19's existing compatibility suites pass, but the missing run-level
  route matrix prevents complete compatibility acceptance.

## Prior finding disposition

No prior reviews exist for CON-033.

## Verification performed

- Inspected the complete diff from recorded base
  `b822466e9c969576cc5365460de6c84d2e8b4733` through Developer commit
  `9949ba12e7a5`, relevant source, tests, documentation, generated skill
  counterpart, task evidence, and capability ledger.
- `go test -count=1 ./...` with an isolated build cache — passed.
- `go test -race -count=1 ./internal/runstate ./internal/runloop
  ./internal/execution ./internal/integration` with an isolated build cache —
  passed.
- `go vet ./...` — passed.
- `go build -o /tmp/concoct-review-bin ./cmd/concoct` — passed; Go emitted the
  known non-fatal read-only module stat-cache warning.
- `bash -n cmd/concoct/concoct.sh` — passed; executable mode is `100755`.
- `git diff --check b822466..HEAD` — passed.
- Source/template Concoct skill comparison — identical.
- Fresh initialization outside the repository — passed: Git repository,
  staged dotfiles and nested adapter, current/archive directories, project
  contract and bootstrap prompt, pending-gate ignore coverage, no generated
  commit, and no installed built-in protocol or persona files.

## Capability impact assessment

The declared `add` impact is appropriate once the findings are resolved.
Archival should add CAP-014 for bounded repeated execution with restrictive
gates, independent review, progress detection, intervention reporting, and
local-only integration. CAP-013 should remain the compatible one-shot
primitive; capability truth must not change before approval.

## Scope and documentation assessment

The implementation is focused on CON-033 and does not pull in CON-034 crash
recovery, concurrent tasks, automatic retries, arbitrary gates, or remote
push. Documentation covers the intended policy, approval, bounds, isolation,
and local-only contract, but its one-current-gate/one-use claim is not true for
repeated action kinds until Finding 1 is fixed.

## Handoff

- Current state after completion: changes requested.
- Work completed: first independent review, source and documentation
  inspection, full existing verification, and fresh-project validation.
- Work remaining: make each action gate one-occurrence authority, harden the
  pending-state parent path, and complete the specified coordinator test
  matrix.
- Known risks: approval reuse can bypass explicit human/project restrictions;
  a symlinked runtime parent can redirect private-state mutations.
- Artifact updated: `.concoct/current/review-01.md` only.
- Expected next role: Developer.
- Recommended next command: `concoct code`.
