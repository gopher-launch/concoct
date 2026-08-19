# Notes

## Planning summary

CON-041 is a critical conformance repair prompted by retained invocation
`de7e75925787e7607d5b9b68ae53e467d42d`. It combines the inseparable schema,
failure-evidence, operator-reporting, and retry-truth corrections required to make
that supervised boundary safe and actionable.

## Confirmed findings

- The retained schema declares `product_decision.mutations` but omits it from
  `product_decision.required`.
- The retained event measurement records `turn.started` and `turn.failed`, no
  usage, and no failure type/message.
- The invocation has no adapter result; stdout and stderr logs are empty.
- Reconciliation reports only nonzero exit status, sets `retry_safe: true`, and
  recommends `concoct next`, despite the unchanged generated schema defect.
- Schema generation is centralized in `internal/adapter`; execution and run-loop
  layers own retention and display.

## Decisions

- Validate the supported strict-schema subset recursively and offline at the
  generation/launch boundary rather than adding a general JSON Schema engine.
- Treat diagnostic type/message as allowlisted bounded evidence, never as workflow
  authority and never as permission to retain the rest of the event.
- Distinguish unchanged repository evidence from readiness to retry a rejected
  schema/configuration.

## Risks

- Event-shape compatibility and byte truncation need adversarial tests.
- Local malformed-result rejection and pre-launch schema rejection are separate
  boundaries and both require explicit coverage.

## Handoff

Implement against the exact retained schema and event sequence. Do not run the
live Codex harness or any billable model invocation.

## Implementation findings and decisions

- Schema generation now invokes a recursive offline validator before adapter
  resolution. It checks every object with `properties`, including array item
  objects, for `additionalProperties: false`, a `required` array, and an exact
  one-to-one match between declared and required property names.
- `product_decision.mutations` is required by the generated schema and no longer
  uses `omitempty`, so Go-produced no-mutation decisions serialize as `[]`.
  Result reading also rejects an omitted key with an actionable local diagnostic.
- `turn.failed.error.type` and `.message` are the only promoted event payload
  fields. They are individually bounded and retained as structured measurement
  and reconciliation evidence; arbitrary sibling/error content is discarded.
- Repository evidence remaining unchanged is not enough to retry a terminal
  Codex schema/configuration rejection. Such failures set `retry_safe: false`
  and report the required correction in `exec` and `run`; the run loop does not
  recommend unchanged `concoct run`.

## Verification results

- Focused adapter, orchestration, execution, run-loop, and CLI tests passed.
- `go test -count=1 ./...` passed.
- `go test -count=1 -race ./...` passed.
- `go vet ./...` passed.
- Native and Windows amd64 builds passed.
- `bash -n cmd/concoct/concoct.sh` and executable-mode check passed.
- Fresh initialization in an external temporary parent passed: dotfiles,
  nested templates, planning/bootstrap evidence, and Git initialization were
  present; executable-owned protocol/persona/handoff files were absent.
- Source/template diff check passed; no template source changed.
- `git diff --check` passed.
- No live or billable model invocation was run.

## Handoff to reviewer

### Implemented

Corrected the Product Owner schema, added recursive strict-schema validation,
enforced explicit mutation arrays, retained bounded terminal Codex diagnostics,
displayed them through one-shot and run output, and corrected retry guidance.

### Key decisions

The validator intentionally covers the strict generated schema subset rather
than becoming a general JSON Schema engine. Terminal failure evidence is a
dedicated bounded type/message pair and remains non-authoritative.

### Files changed

Adapter schema/metrics, orchestration result reading, execution reconciliation,
CLI/run-loop display, focused regressions, and command/workflow documentation.

### Verification

The complete requested deterministic matrix passed as recorded above.

### Known risks

Codex event shapes remain version-sensitive; missing or malformed diagnostic
objects degrade to bounded parser diagnostics without inventing error content.

### Skipped or unresolved work

None. The live harness was deliberately not run per task scope.

### Capability impact

Updates CAP-012 through CAP-015 as planned; the Developer did not edit the
capability ledger.

### Suggested review focus

Independently inspect recursive schema paths, exact-required semantics, mutation
presence enforcement, terminal diagnostic bounds/provenance, no-result/no-usage
regression, and the absence of blind retry advice.

## Review 01 disposition

- Finding 1 — fixed. The schema validator now walks every nested map and array,
  independent of schema keyword, while retaining complete map-key and array-index
  paths. A `$defs` nested inside `oneOf[0]` regression proves the formerly missed
  object is rejected.

## Handoff to reviewer — Review 01 remediation

### Implemented

Replaced keyword-specific recursion with a generic nested map/array schema walk.

### Key decisions

Object contract validation remains specialized, while traversal is deliberately
schema-keyword-neutral so future generated composition keywords cannot bypass it.

### Files changed

`internal/adapter/adapter.go`, its focused tests, and these durable notes.

### Verification

Focused adapter tests and the full deterministic suite pass after remediation.

### Known risks

None beyond the previously documented Codex event-shape sensitivity.

### Skipped or unresolved work

None; Review 01 has no unresolved findings.

### Capability impact

Unchanged from the plan.

### Suggested review focus

Confirm arbitrary-keyword recursion, complete array-index paths, and preservation
of the exact-required checks for all generated action schemas.

## Archival handoff

- Archive path: `.concoct/archive/2026-08-18-CON-041-repair-structured-output-schema-and-terminal-failure-diagnostics/`.
- Summary records Review 02 approval, Review 01 remediation, delivered behavior,
  the full deterministic verification matrix, and the deliberate absence of a
  live model invocation.
- CAP-012 through CAP-015 now describe schema validation, explicit mutation
  arrays, bounded terminal diagnostics, operator display, and correction-gated
  retry behavior.
- The active roadmap record retains status `active` and gains only the required
  pending archive directory reference until Git integration completes.
- Archive candidate copies are byte-identical to current accepted artifacts;
  focused validation and `git diff --check` pass.
- Pending delivery action after archive completion: `concoct integrate`.
