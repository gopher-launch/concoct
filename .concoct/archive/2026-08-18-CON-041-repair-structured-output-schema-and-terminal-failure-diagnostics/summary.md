---
task-id: CON-041
roadmap-id: CON-041
status: archived
archived: 2026-08-18
review: review-02.md
delivery: pending-integration
capability-impact:
  type: update
  ids:
    - CAP-012
    - CAP-013
    - CAP-014
    - CAP-015
---

# Summary

## Delivered outcome

CON-041 repairs the post-CON-040 Product Owner invocation failure. Every
generated role/action structured-output schema is now recursively checked
offline for closed objects and an exact one-to-one required/property contract.
Product Owner outcomes require `mutations` and emit `[]` when none apply.

Codex `turn.failed` events now retain only bounded diagnostic type/message
evidence through measurement and reconciliation, display it in `exec` and
`run`, and prevent unchanged blind retry after schema/configuration rejection.
The behavior does not depend on stdout/stderr logs or retain arbitrary event
payload content.

## Key decisions

- Schema traversal is keyword-neutral across arbitrary nested maps and arrays,
  while validation intentionally covers the strict object subset required by
  generated Codex schemas rather than implementing general JSON Schema.
- Terminal failure diagnostics remain bounded observational evidence and never
  become workflow authority.
- Unchanged repository evidence does not make a rejected schema/configuration
  safe to retry; the reported defect must change first.

## Files and areas changed

Adapter schema generation and JSONL measurement, orchestration result reading,
execution reconciliation, CLI and run-loop reporting, focused regressions, and
the command/workflow documentation.

## Verification

Focused tests, `go test -count=1 ./...`, `go test -count=1 -race ./...`,
`go vet ./...`, native and Windows amd64 builds, shell syntax and executable-mode
checks, source/template parity, fresh external initialization, and
`git diff --check` passed. No live or billable model invocation was run.

## Review outcome

Review 01 requested keyword-neutral recursive traversal. The Developer added a
generic map/array walk and nested `$defs`/`oneOf` regression. Review 02 verified
the remediation and approved the complete task.

## Capability changes

CAP-012 through CAP-015 now record recursive strict-schema validation, explicit
Product Owner mutation arrays, bounded terminal failure evidence, one-shot and
run display, and correction-gated retry reporting.

## Skipped work

The live Codex harness was deliberately not run because the repair requires no
live or billable model invocation. No scoped implementation work was skipped.

## Follow-up work

Codex JSONL event shapes remain version-sensitive. Missing or malformed failure
diagnostics degrade safely without invented content. No additional follow-up is
required for CON-041.

## References

- `.concoct/roadmap.md` — CON-041 pending delivery evidence
- `.concoct/capabilities.md` — CAP-012, CAP-013, CAP-014, CAP-015
- `review-01.md`
- `review-02.md`
