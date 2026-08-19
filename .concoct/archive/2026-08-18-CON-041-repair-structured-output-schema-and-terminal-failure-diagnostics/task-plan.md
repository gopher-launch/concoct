---
id: CON-041
title: Repair structured-output schema and terminal failure diagnostics
roadmap-id: CON-041
status: implementation-complete
remediates-review: review-01.md
created: 2026-08-18
updated: 2026-08-18
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-041-repair-structured-output-schema-and-terminal-fai
  base: d8351e295fa90fd22e4157d838eedb6414f8488a
  status: active
capability-impact:
  type: update
  ids:
    - CAP-012
    - CAP-013
    - CAP-014
    - CAP-015
  rationale: Corrects structured contracts, bounded adapter failure evidence, operator reporting, and retry guidance.
---

# Task Plan

## Goal

Repair the exact post-CON-040 Product Owner schema rejection and make all
generated role/action schemas verifiably compatible with Codex strict structured
outputs, while preserving bounded terminal failure diagnostics and actionable
retry guidance throughout one-shot and run execution.

## Context and current state

Invocation `de7e75925787e7607d5b9b68ae53e467d42d` retained a Product Owner
schema whose `product_decision.properties` declared `mutations` but whose
`required` array omitted it. Codex emitted `turn.failed`, exited nonzero, wrote no
adapter result or usage, and left stdout/stderr empty. Current measurement retains
only lifecycle labels, reconciliation reduces the failure to exit status, and
`RetrySafe` is true solely because repository evidence did not change.

The schema is generated centrally by `internal/adapter`, JSONL measurement is
decoded there, and `internal/execution`, `internal/runloop`, and `internal/cli`
retain and display reconciliation. Accepted capability limitations allow bounded,
version-sensitive event handling; this repair narrows that limitation without
changing workflow authority.

## Target state

- Every generated object schema recursively has closed properties and a complete,
  duplicate-free `required` array.
- Product Owner decisions always carry `mutations`, using `[]` when none apply,
  and every supported decision kind validates against the exact regression schema.
- Invalid schemas and invalid Product Owner mutation shapes fail offline before
  the Codex process starts.
- A bounded allowlisted type/message from `turn.failed` survives measurement and
  reconciliation and appears in `exec` and `run` output without raw-event retention.
- Retry reporting distinguishes evidence freshness from correction readiness and
  names the schema/configuration change required after a pre-inference rejection.

## Design constraints

- Preserve existing structured orchestration authority and result correlation.
- Recursively validate generated schemas without network access or live inference.
- Diagnostics must use explicit byte/count bounds and must not retain arbitrary
  event fields or depend on stdout/stderr logs.
- Complete schema paths must identify missing or duplicate required properties.
- No live or billable model invocation is permitted.
- Preserve source/template parity and initialization behavior.

## Non-goals

- No general JSON Schema implementation or remote schema resolution.
- No change to Product Owner semantics, roadmap ranking, or reconciliation scope.
- No automatic retry, crash-resumable run work, or unrelated adapter expansion.
- No retention of raw Codex JSONL events.

## Risks and assumptions

- Codex `turn.failed` payload shape is version-sensitive; parsing must accept only
  the observed bounded diagnostic fields and degrade safely when absent/malformed.
- Existing `RetrySafe` may be consumed by tests and UI; semantics and wording need
  precise changes without conflating repository freshness with retry readiness.
- Schemas for non-Product-Owner actions use an empty closed Product Decision object
  and must remain valid under the recursive contract.

## Implementation phases

### Phase 1 — Establish exact regressions

Status: `complete`

Add a recursive offline schema-contract validator and table-driven coverage for
every registered role/action schema, including complete-path missing/duplicate
failures and the retained post-CON-040 Product Owner contract.

### Phase 2 — Correct schema and local validation

Status: `complete`

Require `mutations`, update no-mutation fixtures and prompts/contracts as needed,
validate generated schemas before adapter resolution/launch, and cover all five
Product Owner decision kinds plus malformed/omitted mutations.

### Phase 3 — Preserve terminal diagnostics and retry truth

Status: `complete`

Parse bounded `turn.failed` type/message evidence, thread it through measurement
and reconciliation, display it from `exec` and `run`, and replace unchanged-evidence
blind retry advice with the exact schema/configuration blocker and correction.

### Phase 4 — Verify and document

Status: `complete`

Run formatting, focused and full tests, race tests, vet, native and Windows builds,
shell validation, source/template parity, fresh initialization checks, stale-path
searches, and `git diff --check`; update durable notes and relevant documentation.

## Acceptance criteria

- `product_decision.mutations` is required in the exact regression schema.
- All supported Product Owner decisions with an explicit mutation array validate;
  omitted or malformed mutations fail locally before Codex launch.
- Every role/action output schema passes recursive closed-object and exact-required
  validation; failures report the complete schema path and named property.
- `turn.started` → `turn.failed` → nonzero exit with no result/usage retains a
  bounded diagnostic and displays it in both execution surfaces.
- Pre-inference rejection does not recommend unchanged blind retry and instead
  names the required schema/configuration correction.
- The complete user-requested deterministic verification matrix passes.

## Handoff expectations

The Developer should implement and verify without invoking a live model, record
material decisions and test results in notes, and leave a clean reviewable task
branch. The Reviewer must independently inspect schema recursion, launch ordering,
diagnostic bounds/provenance, retry semantics, and the full failure sequence.
