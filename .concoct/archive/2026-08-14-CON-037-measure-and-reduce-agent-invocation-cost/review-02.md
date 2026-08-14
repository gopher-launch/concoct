---
task-id: CON-037
review: 2
status: changes-requested
created: 2026-08-14
persona: reviewer
---

# Review 02

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The Review 01 payload-exposure path is closed: progress now consists only of
allowlisted lifecycle labels, those labels are bounded for retention and live
display, and metrics-first inspection/export removes progress and diagnostics,
including from legacy measurement files. Late terminal usage also survives
progress and raw-log truncation in the focused tests.

One major boundedness defect remains. The same incremental decoder retains an
unlimited number of diagnostic strings and an unlimited unterminated JSONL
line, and the diagnostics are persisted in `measurement.json`. Therefore an
adapter stream can still grow invocation memory and detailed measurement
evidence without the configured log bound, contrary to the task contract.

## Acceptance criteria assessment

- The six-role baseline and 44.2% Developer prompt-byte reduction remain
  sufficient for the required material cost reduction.
- Prompt composition, native usage, timing, predecessor linkage, aggregation,
  private inspection, and workflow-authority behavior remain covered by the
  previously reviewed implementation and the independently passing full suite.
- Review 01's unrestricted progress payload leak is resolved. Progress labels
  have a count and byte ceiling, display is independently bounded, and default
  inspection/export excludes progress and diagnostic text.
- The requirement to keep detailed measurements local and bounded is not yet
  met because diagnostic accumulation and the partial-line decoder buffer have
  no count or byte ceiling.
- The opt-in live Codex harness was appropriately not run; live compatibility
  remains a disclosed residual risk rather than claimed evidence.

## Findings

### Finding 1 — Decoder diagnostics and partial-line input remain unbounded

- Severity: major
- Status: open
- Evidence: `internal/adapter/metrics.go` bounds only `Progress`. Every
  malformed JSON line, missing-type line, post-terminal event, malformed usage,
  and duplicate usage appends another string to `EventEvidence.Diagnostics`
  with no count or byte limit (`consume` and `degrade`). `StreamDecoder.Write`
  also appends all bytes after the most recent newline to `buffer` without a
  maximum. `internal/execution/execution.go` continues feeding the decoder after
  raw retention reaches `MaxLogBytes`, and reconciliation serializes all
  diagnostics into `measurement.json`. `compactMeasurement` removes them only
  when later presenting default inspection/export; it does not bound live
  memory or the retained measurement. The new adversarial test sends many
  valid allowlisted events, so it does not exercise either unbounded path.
- Impact: A noisy, malformed, out-of-order, or single oversized unterminated
  adapter stream can consume memory independently of configured retention and
  can produce an arbitrarily large `measurement.json`. This violates the task
  constraints to bound events and diagnostics independently and to keep
  detailed measurements bounded; it also weakens the intended protection
  against adapter-version drift.
- Required action: Add explicit count/byte ceilings for diagnostic evidence and
  for an in-progress JSONL event. Preserve a compact truncation/degradation
  signal, continue resynchronizing at later line boundaries, and ensure late
  valid usage/result reconciliation remains available. Add adversarial tests
  for many malformed or post-terminal events and a chunked oversized line,
  asserting bounded memory-facing evidence and retained measurement size as
  well as recovery of a later valid usage event.

## Prior finding disposition

Review 01 Finding 1 is partially fixed. Its privacy defect and unrestricted
payload-derived progress are fixed, as are progress count/display bounds and
legacy metrics-export sanitization. Its broader bounded-measurement outcome is
not complete because diagnostics and partial-line buffering remain unlimited.

## Verification performed

- `GOCACHE=/tmp/concoct-review02-gocache GOMODCACHE=/home/cthain/go/pkg/mod go test -count=1 ./...` — passed.
- Race-enabled tests for `internal/prompt`, `internal/adapter`,
  `internal/execution`, and `internal/runloop` — passed.
- `go vet ./...`, native build, and Windows amd64 build with the same caches —
  passed; Go emitted only read-only module-cache stat-cache warnings.
- `bash -n cmd/concoct/concoct.sh` and executable-mode check — passed.
- External temporary-project initialization — passed. The generated project is
  a Git repository, stages dotfiles and nested adapters/current artifacts,
  contains `.concoct/current/bootstrap-prompt.md`, and excludes installed
  built-in protocol, persona, and prompt directories.
- `git diff --check 85bbcea49109e81db5297641a69da3bbcd5c8c14..HEAD` — passed.
- Manually inspected the complete task diff, remediation diff, relevant source,
  tests, documentation, task artifacts, baseline report, and prior review.

The first initialization assertion used the wrong bootstrap filename
(`.concoct/bootstrap.md`) after initialization itself succeeded. Inspection
confirmed the documented `.concoct/current/bootstrap-prompt.md`, and the
corrected assertions passed. The live Codex test was not run because it consumes
model allocation.

## Capability impact assessment

The planned `add` impact for CAP-015 remains accurate after Finding 1 is fully
resolved. The eventual capability should describe exact prompt attribution,
native usage when reported, bounded private event/measurement evidence,
metrics-first inspection, run aggregation, reproducible baselines, and the
verified Developer fixed-prompt reduction. Codex event-version sensitivity and
the absence of a live token-reduction claim should remain limitations.

## Scope and documentation assessment

The remediation remains within CON-037's scope, and no unrelated product change
was found. Documentation accurately describes payload-free bounded progress and
privacy-preserving inspection, but its broader bounded-record claims remain
incomplete until decoder diagnostics and partial-line buffering are bounded.

## Handoff

Return to the Developer with `concoct code`. Remediation should bound diagnostic
evidence and oversized partial JSONL events without allowing truncation to hide
later valid usage or structured-result reconciliation. The next review should
focus on adversarial malformed streams, resynchronization, retained
`measurement.json` size, and preservation of the already-fixed privacy surface.
