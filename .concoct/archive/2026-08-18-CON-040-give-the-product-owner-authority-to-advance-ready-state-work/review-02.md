---
task-id: CON-040
review: 2
status: changes-requested
created: 2026-08-18
persona: reviewer
---

# Review 02

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The remediation fixes the concrete applied-`select` failure from Review 01 and
adds a passing regression test for that path. The underlying state-authority
defect remains for the other Product Owner decision kinds: status can still let
retained ready-state evidence override the canonical continuation of a planned
task. The task's shared-next-action acceptance contract is therefore not yet
satisfied.

## Acceptance criteria assessment

- The applied `select` path now defers to a planned task's `concoct code`
  continuation, and focused coverage verifies the corrected output.
- The broader requirement that retained decisions account for workflow state
  consistently remains unmet for `reconcile`, `reconcile-and-select`,
  `human-decision-required`, and `no-action`.
- The complete Go suite, vet, shell syntax, executable-mode, and diff checks
  pass.

## Findings

### Finding 1 — Non-selection decisions can still mask an active task

- Severity: `major`
- Status: `unresolved`
- Evidence: `internal/cli/cli.go:116-119` now restricts the `select` override to
  `workflow.Ready`, but `internal/cli/cli.go:120-131` applies the other decision
  overrides without any workflow-state guard. A retained proposed `reconcile`
  replaces a planned report's next action with `concoct run --approve next`; an
  approved `reconcile-and-select` replaces it with `concoct plan <selection>`;
  and retained `human-decision-required` or `no-action` clears it entirely. The
  added regression at `internal/cli/cli_test.go:206-242` covers only an applied
  `select`. This conflicts with the target contract in
  `.concoct/current/task-plan.md:134-136`, which requires one typed authority
  accounting for all decision lifecycle states.
- Impact: if a task becomes active while any of these private ready-state
  records remains present, `concoct status` can suppress or replace the active
  task's canonical `concoct code` continuation. Status again disagrees with
  workflow truth and can misroute an operator.
- Required action: make every retained Product Owner decision override the next
  action only in the workflow states where that decision is authoritative,
  preserving the canonical active-task continuation otherwise. Add regression
  coverage for the non-selection decision kinds against a planned task.

## Prior finding disposition

- Review 01 Finding 1 — `partially fixed`. The exact applied-`select` example is
  corrected and tested. The same state-precedence issue remains in adjacent
  branches of the same status decision switch, so the required shared authority
  outcome is incomplete.

## Verification performed

- `GOCACHE=/tmp/concoct-review-02-go-cache go test -count=1 ./...` — passed.
- `GOCACHE=/tmp/concoct-review-02-go-cache go vet ./...` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed.
- Executable-mode check for `cmd/concoct/concoct.sh` — passed.
- `git diff --check 2bbdd28cec958b613836ddb4161f12f92642a728...HEAD` — passed.
- Independently inspected the remediation commit, status decision rendering,
  focused regression, prior review, task contract, and developer disposition.

## Capability impact assessment

The declared `update` impact for CAP-001, CAP-005, CAP-006, CAP-009, CAP-011,
CAP-012, CAP-013, CAP-014, and CAP-015 remains appropriate if the behavior is
accepted after remediation. The capability ledger must remain unchanged for
now.

## Scope and documentation assessment

The remediation is appropriately narrow and documentation remains aligned with
the intended design. Resolving the remaining finding requires completing the
same status-state precedence rule, not expanding product scope.

## Handoff

Return to the Developer with `concoct code`. Apply workflow-state precedence to
all retained Product Owner decision kinds and add planned-task regression
coverage for their status behavior. The next review should verify that private
ready-state evidence remains visible for inspection but never overrides an
active task's canonical next action.
