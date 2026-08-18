---
task-id: CON-040
review: 1
status: changes-requested
created: 2026-08-18
persona: reviewer
---

# Review 01

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The implementation establishes the central semantic decision, private retention,
approval, reconciliation, and direct planning continuation mechanisms, and the
full automated suite passes. One major operator-visible state defect remains:
after a successful `select` decision has created a planned task, `concoct status`
overrides the planned task's canonical next action with another planning command.
This violates the task's shared-next-action contract and can direct an operator
back into a transition that has already completed.

## Acceptance criteria assessment

- Semantic Product Owner decisions, bounded private persistence, one-use gates,
  constrained reconciliation, drift checks, and direct Task Planner continuation
  are implemented and covered by focused tests.
- Manual `next` remains read-only and uses the supervised decision vocabulary.
- Clean-ready task and Git metadata reporting is explicit.
- The criterion requiring status and lifecycle commands to derive the exact next
  action consistently is not met because an applied `select` remains able to
  override active-task workflow truth.
- Existing regression suites pass, but the successful selection test stops after
  asserting the retained decision is `applied`; it does not verify the subsequent
  `status` next action.

## Findings

### Finding 1 — Applied selection masks the planned task's real next action

- Severity: `major`
- Status: `unresolved`
- Evidence: `internal/runloop/run.go:523-532` retains the Product Owner decision
  and marks it `applied` after a successful Task Planner transition. The resulting
  workflow is `planned`, as asserted by `internal/runloop/run_test.go:217-226`.
  Nevertheless, `internal/cli/cli.go:108-120` loads the retained decision for
  every workflow state and, for an applied `select`, unconditionally replaces
  `report.Next` with `concoct plan <selection>`. It does not restrict this override
  to a ready-state continuation or defer to the now-active planned task.
- Impact: immediately after the accepted Product Owner-to-Planner flow succeeds,
  `concoct status` reports a stale/repeated planning command instead of the
  canonical planned-state continuation (normally `concoct code`). This makes
  status disagree with workflow/run resolution, defeats the promised common
  next-action authority, and can misroute manual or automated operators.
- Required action: make retained-decision rendering state- and lifecycle-aware so
  an applied selection cannot override an active task's canonical next action.
  Add a regression test that completes the selection-to-planned path, invokes the
  status surface, and verifies the exact next command is the planned task's real
  continuation rather than another `concoct plan`.

## Prior finding disposition

No prior review artifacts exist for CON-040.

## Verification performed

- `GOCACHE=/tmp/concoct-review-go-cache go test -count=1 ./...` — passed.
- `GOCACHE=/tmp/concoct-review-go-cache go vet ./...` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed.
- Executable-mode check for `cmd/concoct/concoct.sh` — passed.
- `git diff --check 2bbdd28cec958b613836ddb4161f12f92642a728...HEAD` — passed.
- Independently inspected the complete changed-file inventory and the decision
  validation, persistence, application, run continuation, CLI status, tests,
  prompts, and operator documentation relevant to the acceptance contract.
- Race, cross-platform build, and fresh initialization checks were not repeated
  in this review; the developer recorded passing results, and the independently
  rerun full suite plus focused inspection were sufficient to identify the
  blocking defect above.

## Capability impact assessment

The declared `update` impact for CAP-001, CAP-005, CAP-006, CAP-009, CAP-011,
CAP-012, CAP-013, CAP-014, and CAP-015 is appropriately broad for the intended
observable change. The Archivist should retain that recommendation if the next
review approves the corrected behavior; capability truth must remain unchanged
until then.

## Scope and documentation assessment

The implementation is substantial but remains aligned with CON-040's approved
scope. Documentation generally reflects the intended supervised Product Owner
boundary and the manual/supervised distinction. The status defect is behavioral,
not a reason to expand the task or rewrite product direction.

## Handoff

Return to the Developer with `concoct code`. Correct Finding 1 and add coverage
for status after a successful applied selection. The next review should focus on
the exact next action across ready, approved, applied/planned, and post-task
states while preserving the existing one-use and reconciliation behavior.
