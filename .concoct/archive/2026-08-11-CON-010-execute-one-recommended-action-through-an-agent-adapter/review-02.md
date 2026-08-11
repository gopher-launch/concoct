---
task-id: CON-010
review: 2
status: approved
created: 2026-08-11
persona: reviewer
---

# Review 02

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`approved`

## Summary

CON-010 now satisfies the approved task plan and all 16 acceptance criteria.
The remediation closes each major finding from review 01 with policy-aware
authorization, a complete evidence-freshness check immediately before launch,
and cancellable/time-bounded direct integration that preserves recoverable Git
transaction phases. No material correctness, compatibility, safety,
documentation, scope, or verification issue remains.

## Acceptance criteria assessment

- The typed resolver and orchestration registry now agree for required,
  `not-required`, and `externally-satisfied` independent-review dispositions;
  direct authorization independently rejects any action not selected by the
  current workflow and policy authority.
- Real execution revalidates both resolved configuration and the complete
  repository evidence digest after record creation and immediately before
  either an adapter or direct action starts. Covered drift closes the attempt
  without launch and remains inspectable.
- Direct integration observes the resolved cancellation/timeout context through
  Git operations, push confirmation, and cleanup. Mutations that complete near
  interruption are captured in the recovery record before control returns, and
  no completed result is fabricated for interrupted work.
- The remaining one-shot, prompt-parity, adapter safety, result correlation,
  observed-postcondition, private-runtime, inspection, retention, redaction,
  manual-fallback, compatibility, and documentation criteria remain satisfied
  by the implementation and regression suite reviewed in both passes.

## Prior finding disposition

### Review 01 finding 1 — Policy-satisfied review cannot reach archival execution

- Status: `fixed`.
- Evidence: The archival spec now permits both `implementation-complete` and
  `review-approved`, while `Authorize` requires the requested kind to equal the
  typed policy-selected action. Orchestration and CLI tests cover required,
  `not-required`, and `externally-satisfied` routes and explicitly reject
  archival authorization when review is still required.

### Review 01 finding 2 — Repository evidence is not revalidated before launch

- Status: `fixed`.
- Evidence: `authorizationStable` re-resolves configuration and compares a new
  complete snapshot with the authorized state and digest before launch.
  Adapter and direct-integration tests mutate covered evidence after prepare
  and prove neither action starts; reconciliation records
  `authorization-changed`.

### Review 01 finding 3 — Direct integration swallows cancellation and timeout

- Status: `fixed`.
- Evidence: Direct execution applies the resolved timeout to
  `integration.RunContext`; context-aware Git operations and push confirmation
  return cancellation while durable recovery phases capture completed local
  mutations. Integration- and execution-level cancellation/timeout tests prove
  prompt interruption, retained recovery, rejected result acceptance, and
  inspectable `cancelled` or `timed-out` dispositions.

## Verification performed

- `go test -count=1 ./...` — passed.
- `go test -count=1 -race ./internal/orchestration ./internal/execution ./internal/integration` — passed.
- `go test -count=5 ./internal/execution ./internal/integration ./internal/orchestration` — passed.
- `go vet ./...` — passed.
- Native and `GOOS=windows GOARCH=amd64` builds of `./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` and executable-mode check — passed.
- `git diff --check c30e7718462b3dba1affe6aa6ce924786b5312db...HEAD` — passed.
- Fresh initialization outside the repository — passed: project-owned
  dotfiles/adapters, current/archive directories, staged bootstrap guidance,
  unborn Git history, runtime ignore behavior, and exclusion of built-in
  protocol/persona material were confirmed.
- Fresh-project `concoct exec --dry-run --reasoning high --timeout 5m` — passed
  with the Product Owner action, expected profile provenance and safety posture,
  exact prompt-on-stdin behavior, and no runtime creation.
- Active documentation/template searches found no stale direct-execution or
  legacy built-in-path claims.
- No live external model invocation was run; deterministic fake-process and
  real-repository tests exercise the relevant execution and recovery boundary.

## Capability impact assessment

The declared `add` impact and proposed identifier `CAP-013` are accurate. The
Archivist should record an active capability describing one-shot execution of
the policy-authorized recommendation through the built-in Codex adapter or
direct integration authority, with exact manual prompt parity, private retained
inspection evidence, launch-time freshness, bounded cancellation/timeout, and
observed repository state taking precedence over adapter claims.

## Scope assessment

The implementation and remediation remain focused on CON-010. Context-aware
Git support is limited to the direct integration behavior required by the task;
no unrelated roadmap work or product expansion was introduced.

## Documentation assessment

README and the normative command, state-machine, and workflow documentation
accurately describe one-shot authorization, configuration precedence, safety
posture, dry-run, inspection, retention, failure behavior, manual fallback,
evidence precedence, and direct integration interruption/recovery.

## Risks and follow-up

- A cancelled confirmation read can leave its reader goroutine blocked until
  the underlying input exits. It cannot mutate repository state, publish a
  result, or keep the CLI process alive, so this is non-blocking and accurately
  documented in the Developer handoff.
- Repeated execution, durable multi-invocation recovery, shared locking, and
  additional adapters remain correctly deferred to later roadmap work.

## Handoff

Current state: approved. The Archivist should run `concoct archive`, preserve
both reviews and the remediation history, add CAP-013 with the capability
impact above, validate the complete archive transaction, and then use
`concoct archive --complete`. For this Git-backed task, archival should stop in
`archived` with pending delivery and recommend `concoct integrate`.
