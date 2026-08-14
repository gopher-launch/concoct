---
task-id: CON-037
roadmap-id: CON-037
status: archived
archived: 2026-08-14
review: review-03.md
delivery: pending-integration
capability-impact:
  type: add
  ids:
    - CAP-015
---

# Summary

## Task

Measure and reduce agent invocation cost.

## Delivered outcome

Concoct now attributes exact rendered prompt bytes to semantic components,
captures optional adapter-reported usage and execution identity, retains
bounded private structured evidence, and reports per-action and aggregate run
measurements without double-counting. Controlled baselines cover all six
prompt-backed roles and document a 44.2% reduction in fixed Developer prompt
bytes, exceeding the required threshold while preserving workflow authority.

## Key decisions

- Adapter-reported usage remains optional observed evidence; Concoct does not
  manufacture totals or treat metrics as transition authority.
- Progress, diagnostics, raw events, display, and partial JSONL input are
  independently bounded, with explicit degraded/truncation signals and later
  usage/result resynchronization.
- Metrics-first inspection/export omits prompts and payload-derived diagnostics;
  full/raw evidence remains an explicit local diagnostic path.

## Files and areas changed

Prompt composition, Codex adapter event and usage translation, execution
retention/reconciliation, invocation inspection, run aggregation, controlled
baseline fixtures/reporting, documentation, and adversarial tests.

## Verification

Review 03 records passing the full deterministic test matrix, focused race
tests, vet, native and Windows builds, shell validation, fresh initialization,
diff checks, and stale-branding checks. The opt-in live Codex harness was not
run because it consumes model allocation.

## Review outcome

`review-03.md` approved the implementation after resolving all prior findings.

## Capability changes

Added CAP-015 for exact prompt attribution, optional native usage, bounded
private measurement, metrics-first inspection, run aggregation, reproducible
baselines, and the verified Developer prompt-byte reduction.

## Skipped work

Live compatibility/baseline execution against the installed Codex CLI remains
deliberately unrun. Version-sensitive event semantics and oversized-event loss
remain documented bounded limitations.

## Follow-up work

Consider a deliberate live compatibility run after adapter or Codex CLI
upgrades. Git delivery remains pending integration on the recorded task branch.

## References

- `.concoct/roadmap.md` — CON-037
- `.concoct/capabilities.md` — CAP-015
- `.concoct/current/review-03.md`
