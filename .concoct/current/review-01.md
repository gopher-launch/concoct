---
task-id: CON-032
review: 1
status: changes-requested
created: 2026-08-11
persona: reviewer
---

# Review 01

## Outcome

`changes-requested`

## Summary

The focused, agent-neutral JSON boundary preserves the manual workflow and
independent checks pass. Two material contract gaps prevent approval: the
claimed atomic single-result delivery has a concurrent overwrite race, and the
registry does not model several required action-contract dimensions.

## Acceptance criteria assessment

- Not met: every action must explicitly define authority, preconditions,
  permitted effects, postconditions, supported outcomes, intervention behavior,
  and a completion validator. `Spec` omits explicit authority, precondition,
  intervention, and completion-validator definitions.
- Not met: duplicate results must be rejected. Concurrent writers can replace
  a first result.
- Met: versioned JSON/correlation, observed-state completion checks,
  diagnostic-only process output, no ready-state work selection, no launcher,
  and preserved manual commands.

## Findings

### Finding 1 — Atomic result delivery can overwrite an existing result

- Severity: major
- Status: unresolved
- Evidence: `WriteAtomicResult` performs `os.Lstat(path)` before
  `os.Rename(name, path)`. Two writers can both pass the check; Unix rename
  replaces the destination, allowing the later result to overwrite the first.
- Impact: the required single-result/duplicate-delivery boundary is not
  mechanically enforced, so a future adapter could validate a different result
  than the one first delivered.
- Required action: use atomic no-replace publication or reserve a single-writer
  target before adapter launch, and add a concurrent-delivery test proving one
  writer succeeds and the stored result is not overwritten.

### Finding 2 — Registry omits required explicit contract dimensions

- Severity: major
- Status: unresolved
- Evidence: `Spec` in `internal/orchestration/orchestration.go` contains kind,
  role, gate, explanation, allowed/completed states, outcomes, and effects,
  but no explicit preconditions, intervention behavior, or completion
  validator. Its test only checks the implemented fields.
- Impact: callers cannot inspect or consistently apply the promised full
  action contract; future adapters would have to infer omitted semantics.
- Required action: add the missing definitions to each specification, apply
  them during authorization/outcome reconciliation, and test every registered
  action's precondition, intervention, and completion-validation behavior.

## Verification performed

- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed.
- `git diff --check 95cc94c..HEAD` — passed.
- Inspected the complete task diff, new package and tests, documentation,
  workflow behavior, and active artifacts.
- Attempted `concoct developer`, `reviewer`, and `archivist` as prompt checks;
  they are not CLI commands and correctly returned usage. Existing suite
  coverage passed, but this does not verify prompt rendering.

## Capability impact assessment

CAP-012 is an appropriate intended addition, but must not be reconciled into
the capability ledger until both findings are resolved.

## Scope and documentation assessment

Scope is disciplined: no agent execution, lifecycle loop, or manual-command
replacement was introduced. `doc/command-reference.md` describes the intended
boundary and should be kept aligned with the strengthened semantics.

## Handoff

Route to Developer for remediation. Record dispositions and exact checks in
`notes.md`, then run `concoct code --complete`. A subsequent review should
focus on race-safe single-result publication and complete, tested per-action
contract semantics.

Recommended next command: `concoct code`.

<!-- This review was created from the exclusive reservation: Replace status: reserved. -->
