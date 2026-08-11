---
task-id: CON-032
roadmap-id: CON-032
status: archived
archived: 2026-08-11
review: review-02.md
delivery: pending-integration
capability-impact:
  type: none
  ids:
---

# Summary

## Delivered outcome

CON-032 established a versioned, agent-neutral JSON contract for one
authorized workflow action and its correlated outcome. The executable-owned
registry defines authority, preconditions, effects, supported outcomes,
intervention routes, and observed-state completion validators. Validation
rejects stale, malformed, duplicated, mismatched, unsupported, and
mechanically contradicted results while preserving the manual workflow.

## Key decisions

- Process status and human-readable output are diagnostics only; observed
  artifact, workflow, repository state, and executable authority decide
  completion.
- Ready-state evidence produces Product Owner input and never autonomously
  selects work.
- Process adapters use invocation-specific atomic no-replace result delivery;
  raw envelopes and logs remain ephemeral.

## Files and areas changed

The `internal/orchestration` contract, registry, evidence binding, validation,
bounded durable facts, atomic result helpers, focused tests, and normative
command documentation were added or updated.

## Verification

`go test -count=1 ./...`, `go test -count=1 -race ./internal/orchestration`,
`go vet ./...`, `go build ./cmd/concoct`, `bash -n cmd/concoct/concoct.sh`,
and `git diff --check` passed.

## Review outcome

`review-02.md` approved the implementation after resolving review-01 findings
on atomic duplicate delivery and complete per-action contract definitions.

## Capability changes

CAP-012 was added for the structured orchestration action/outcome validation
boundary. Direct agent execution and lifecycle orchestration remain future
roadmap work.

## Skipped work

Agent launching, repeated lifecycle execution, configurable gates, and
automatic ready-state task selection remain outside CON-032 scope.

## Follow-up work

CON-010 can wire this contract into a single adapter execution while preserving
its observed-state precedence and no-replace result boundary. Integration into
`main` remains pending under the recorded Git workflow.
