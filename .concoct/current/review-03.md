---
task-id: CON-037
review: 3
status: approved
created: 2026-08-14
persona: reviewer
---

# Review 03

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`approved`

## Summary

The implementation satisfies CON-037. Review 02's remaining boundedness defect
is resolved: diagnostic evidence now has independent count and byte ceilings,
an oversized partial JSONL event is discarded through its line boundary, and
decoding resumes so later native usage and structured-result reconciliation
remain available. The previously accepted prompt composition, private
measurement, aggregation, six-role baselines, and measured 44.2% Developer
prompt-byte reduction remain intact.

The full deterministic verification matrix passed independently. No material
correctness, privacy, scope, documentation, or compatibility issue remains.

## Acceptance criteria assessment

- Detailed structured evidence is bounded across progress, diagnostics, raw
  retention, display, and an in-progress event. Truncation is explicit and
  degrades measurement compatibility without becoming workflow authority.
- Adversarial tests cover many malformed and post-terminal events, chunked
  oversized unterminated input, bounded serialized evidence, newline
  resynchronization, and recovery of later usage.
- Metrics-first inspection/export remains payload-free, while explicit local
  full/raw inspection remains available for diagnosis.
- The six required role baselines and the documented 44.2% supervised
  Developer prompt reduction exceed the required 30% fixed-prompt threshold.
- Prompt byte conservation, manual/supervised semantic parity, optional native
  usage, timing, predecessor linkage, run aggregation, and mechanical-action
  exclusion remain covered by the passing suite and inspected implementation.
- All required offline checks pass. The opt-in live Codex harness was not run,
  so live behavior for the locally installed version remains a disclosed
  compatibility risk rather than claimed verification.

## Findings

No material findings.

## Prior finding disposition

- Review 01 Finding 1: fixed. Progress is payload-free and bounded; default
  inspection and metrics export exclude progress and diagnostics, including
  from legacy measurements.
- Review 02 Finding 1: fixed. Diagnostics have 64-entry and configured-byte
  ceilings; partial event buffering has a configured-byte ceiling; truncation
  is explicit; and the decoder resynchronizes for later valid evidence.

## Verification performed

- `GOCACHE=/tmp/concoct-review03-full GOMODCACHE=/home/cthain/go/pkg/mod go test -count=1 ./...` — passed.
- Focused race-enabled tests for `internal/prompt`, `internal/adapter`,
  `internal/execution`, and `internal/runloop` — passed.
- `go vet ./...`, native build, and Windows amd64 build with isolated writable
  build caches — passed. Builds emitted only read-only module-cache stat-cache
  warnings.
- `bash -n cmd/concoct/concoct.sh`, executable-mode validation, and
  `git diff --check 85bbcea49109e81db5297641a69da3bbcd5c8c14..HEAD` — passed.
- External temporary-project initialization — passed. The generated project is
  a Git repository, stages dotfiles and nested adapters/current artifacts,
  contains `.concoct/current/bootstrap-prompt.md`, and excludes installed
  built-in protocol and persona content.
- Stale-branding search over relevant source, templates, documentation, and
  the CON-037 report found no matches.
- Manually inspected the complete task diff, Review 02 remediation diff,
  decoder/capture implementation, adversarial tests, documentation, baseline
  report, task artifacts, and both prior reviews.

The opt-in live Codex test was not run because it consumes model allocation.

## Capability impact assessment

The planned `add` impact for CAP-015 is accurate. The Archivist should record
exact prompt attribution and composition, adapter-reported native usage,
bounded private measurement and structured progress, metrics-first inspection,
run aggregation, reproducible role baselines, and the verified Developer
fixed-prompt reduction as one new capability. Codex event-version sensitivity
and the absence of live token-reduction evidence should remain limitations.

## Scope and documentation assessment

The implementation remains within the approved CON-037 scope. The Developer
persona compaction is the evidence-backed optimization required by the plan;
the measurement and supervision changes retain existing workflow authority and
role boundaries. Documentation now accurately describes diagnostic and partial
event bounds, truncation/degradation behavior, resynchronization, inspection
privacy, measurement fields, and the measured reduction.

## Risks and follow-up

Codex JSONL is version-sensitive, and an individual event larger than the
configured evidence ceiling is intentionally unavailable for attribution.
Both limitations are explicit, bounded, and non-blocking. A deliberate live
compatibility run may be useful after adapter or Codex CLI upgrades, but is not
required for acceptance.

## Handoff

Proceed with `concoct archive`. Approval is based on satisfaction of the task
contract, resolution of all prior findings, and the independently passing
offline verification matrix. Archive CAP-015 as an added capability with the
limitations described above.
