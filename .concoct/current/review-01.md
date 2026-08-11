---
task-id: CON-010
review: 1
status: changes-requested
created: 2026-08-11
persona: reviewer
---

# Review 01

<!-- Reservation provenance: Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The implementation establishes a substantial portion of the planned one-shot
execution surface, and all independently run repository checks pass. Three
major execution-boundary defects remain: one accepted policy route cannot be
authorized, state-changing actions can launch after their repository evidence
has drifted, and direct integration ignores the cancellation/timeout context
that the CLI installs. These defects affect acceptance criteria 1, 3, 9, and
10 and require Developer remediation before approval.

## Acceptance criteria assessment

- Criteria 2, 4-8, 11-14, and 16 are substantially evidenced by the inspected
  implementation, tests, documentation, and fresh-project dry-run.
- Criteria 1 and 3 are not met for `implementation-complete` tasks whose
  independent review is policy-selected as `not-required` or
  `externally-satisfied`; Finding 1 records the contract mismatch.
- Criterion 9 is not met because only resolved configuration, not the
  authorized repository evidence digest, is checked immediately before launch;
  Finding 2 records the stale-authorization path.
- Criterion 10 is not met for the direct integration action because its
  execution path does not observe cancellation or the resolved timeout;
  Finding 3 records the issue.
- Criterion 15's regression checks pass, but the task-required execution
  matrix does not cover the policy route, pre-launch evidence drift, or direct
  integration cancellation/timeout cases that expose the findings below.

## Findings

### Finding 1 — Policy-satisfied review cannot reach archival execution

- Severity: `major`
- Status: `unresolved`
- Evidence: `workflow.ResolveAction` selects `archival` from
  `implementation-complete` when independent review is `not-required` or
  `externally-satisfied` (`internal/workflow/workflow.go:60-64`). The archival
  orchestration spec permits only `review-approved`
  (`internal/orchestration/orchestration.go:159`), and `orchestration.Resolve`
  rejects the selected action when the current state is absent from that list
  (`internal/orchestration/orchestration.go:186-189`). The added workflow test
  asserts the first half of this route but never passes it through the
  orchestration resolver (`internal/workflow/workflow_test.go:39-63`).
- Impact: A valid policy disposition that the manual `archive` prompt supports
  cannot be dry-run or executed through `concoct exec`; it fails as an internal
  contract disagreement instead of displaying and authorizing the one valid
  action. This contradicts the documented state/policy selection and acceptance
  criteria 1 and 3.
- Required action: Make archival authorization accept the policy-valid
  implementation-complete route without permitting review bypass when review
  remains required. Add orchestration and CLI coverage for both
  `not-required` and `externally-satisfied` dispositions, including refusal of
  ordinary unapproved implementation-complete evidence.

### Finding 2 — Repository evidence is not revalidated before launch

- Severity: `major`
- Status: `unresolved`
- Evidence: Authorization snapshots repository/workflow evidence during
  `Prepare`, but immediately before execution `Run` calls only
  `configurationStable` and then starts the direct operation or adapter
  (`internal/execution/execution.go:176-204`). `configurationStable` compares
  only resolved execution settings (`internal/execution/execution.go:735-751`).
  At reconciliation, non-completed outcomes require the original digest, while
  a completed state-changing action merely requires the digest to differ and
  the final workflow state to match (`internal/orchestration/orchestration.go:307-330`).
  Therefore a repository change made after authorization but before process
  start is indistinguishable from authorized action changes and can be accepted
  when the final state is valid. The drift test covers only user configuration
  (`internal/execution/execution_test.go:131-162`).
- Impact: Concurrent or intervening Git, policy, task, review, or source changes
  can invalidate the exact prompt/action snapshot without stopping launch, yet
  a later completed claim may still be accepted. That breaks the task's stated
  recommendation-drift boundary and acceptance criterion 9's evidence-
  freshness requirement.
- Required action: Compare the complete current action evidence with the
  authorized digest immediately before any adapter or direct action starts and
  close/reconcile the attempt without launch on mismatch. Preserve the
  separate post-action validation needed for legitimate mutations. Add a
  controllable test that changes covered repository evidence between prepare
  and launch and proves no process or direct operation starts.

### Finding 3 — Direct integration swallows cancellation and timeout

- Severity: `major`
- Status: `unresolved`
- Evidence: The CLI converts `SIGINT` into a cancelled context before calling
  execution (`internal/cli/cli.go:237-239`). Adapter execution observes that
  context and the resolved timeout (`internal/execution/execution.go:262-285`),
  but the archived-state direct branch calls synchronous `runDirect` without
  passing the context (`internal/execution/execution.go:193-204`). `runDirect`
  calls `integration.Run` directly and has no cancellation or timeout path
  (`internal/execution/execution.go:221-235`).
- Impact: During `concoct exec` from `archived`, Ctrl-C is intercepted but not
  acted on, and `--timeout` is not enforced. A push-confirmation wait or long
  integration operation can therefore outlive both user cancellation and the
  resolved invocation deadline, contrary to acceptance criterion 10 and the
  inspectable closure contract.
- Required action: Give direct integration a cancellation/timeout behavior
  consistent with the one-shot boundary without weakening the existing Git,
  recovery, and push-confirmation authority. Ensure cancellation cannot leave
  a fabricated completed result, reconcile any actual state change, and add
  direct-action tests for cancellation and timeout.

## Prior finding disposition

No prior review artifacts exist for CON-010.

## Verification performed

- `go test -count=1 ./...` — passed.
- `go test -count=1 -race ./internal/orchestration ./internal/execution` — passed.
- `go vet ./...` — passed.
- Native and `GOOS=windows GOARCH=amd64` builds of `./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` and executable-mode check — passed.
- `git diff --check c30e7718462b3dba1affe6aa6ce924786b5312db...HEAD` — passed.
- Fresh initialization outside the repository — passed: expected project files,
  Git repository, bootstrap prompt, planning directories, runtime ignore rule,
  and exclusion of executable-owned protocol/persona material were confirmed.
- Fresh-project `concoct exec --dry-run --reasoning high --timeout 5m` — passed
  and created no runtime directory.
- Installed `codex-cli 0.146.0` help was checked; all constructed Codex flags
  used by the adapter are present.
- No live external model invocation was run; this was not needed to establish
  the three code-path findings.

## Capability impact assessment

The declared `add` impact for proposed CAP-013 remains the correct category and
identifier. Capability truth must not be added until the one-shot authorization,
freshness, and cancellation boundaries above are remediated and approved.

## Scope assessment

The implementation is focused on CON-010. No unrelated product behavior or
roadmap work was identified in the diff.

## Documentation assessment

The README and normative command, state-machine, and workflow documentation
cover the intended one-shot behavior, configuration, retention, manual
fallback, and observed-state precedence. The current implementation does not
yet fulfill the documented evidence-drift and cancellation claims described in
Findings 2 and 3.

## Risks and follow-up

- Add the task-plan-requested table coverage across every workflow state and
  policy disposition at the orchestration/CLI boundary; the current unit split
  allowed Finding 1 to pass all tests.
- Expand the failure matrix to exercise pre-launch repository drift and direct
  integration interruption, not only adapter cancellation and user-config
  drift.

## Handoff

Current state: review changes requested. The Developer should run
`concoct code`, remediate all three findings, record each disposition in
`.concoct/current/notes.md`, rerun the full and focused race checks plus the new
policy/freshness/direct-interruption tests, and complete a fresh Reviewer
handoff. The next independent review should focus on policy-gated archival
authorization, the precise pre-launch evidence boundary, direct-integration
interrupt recovery, and preservation of existing manual/Git authority.
