---
task-id: CON-037
review: 1
status: changes-requested
created: 2026-08-14
persona: reviewer
---

# Review 01

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The implementation delivers most of the planned CAP-015 surface: semantic
prompt composition, optional native usage preservation, adapter/version and
timing evidence, metrics-first inspection, per-action/run summaries, six-role
offline baselines, an opt-in isolated live harness, and a documented 44.2%
Developer prompt-byte reduction. The full deterministic verification matrix
passed independently.

One major issue prevents acceptance. Structured progress strings bypass the
raw-event redaction and size limit, remain unbounded in memory and in
`measurement.json`, and are included verbatim in the supposedly metrics-only
JSON export. This violates the task's bounded-retention and export-privacy
acceptance criteria.

## Acceptance criteria assessment

- Exact prompt bytes, semantic components, byte conservation, duplicate
  digests, manual/supervised parity, adapter identity, optional native usage,
  timing, predecessor linkage, and run aggregation are implemented and covered
  by deterministic tests.
- The controlled report records all six required role baselines and demonstrates
  a 44.2% supervised Developer prompt reduction, exceeding the 30% threshold.
- Raw JSONL and stderr retention are separately bounded, and late usage remains
  decodable after raw-event truncation.
- Default inspection omits prompts and raw logs, but measurement retention and
  JSON export are not privacy-preserving or bounded because they contain the
  decoder's unrestricted progress strings.
- All requested offline checks passed. The opt-in live Codex harness was
  appropriately not run, so current-version live event compatibility remains
  the documented residual risk rather than a claimed passing check.

## Findings

### Finding 1 — Progress evidence bypasses privacy and retention bounds

- Severity: major
- Status: open
- Evidence: `internal/adapter/metrics.go` stores every top-level `message`,
  `text`, or `status` value verbatim in `EventEvidence.Progress` (lines 22–30,
  96–98, and 178–185), with no event-type allowlist, redaction, per-value
  limit, count limit, or total-byte limit. `internal/execution/execution.go`
  applies redaction and `MaxLogBytes` only while writing raw JSONL (lines
  518–534); the decoder still accumulates all progress strings and writes all
  of them to the live progress sink (lines 510–514 and 548–557). Reconciliation
  serializes that complete evidence to `measurement.json`, and `Metrics`
  embeds the file unchanged in `exec inspect --json` (lines 646–679). The
  existing export test uses a handcrafted measurement containing only usage,
  so it does not exercise this path.
- Impact: An adapter event can place repository content, credentials, or
  unrestricted model/tool text into default inspection and the metrics-only
  export without the raw-log redactor ever seeing that copy. A long stream can
  also grow decoder memory, terminal output, and the current invocation record
  without the configured log or total-retention bound. This directly violates
  the acceptance requirements that detailed measurements remain bounded and
  that default inspection/export exclude credentials, unrestricted events,
  and repository content.
- Required action: Make structured progress a deliberately selected,
  adapter-owned, privacy-safe bounded representation. Apply explicit event
  selection, redaction, per-entry/count/total bounds, and truncation diagnostics
  before display or persistence; ensure `measurement.json` and `--json` cannot
  expose unrestricted progress content. Add adversarial tests covering secrets,
  repository-like content, oversized values, many events, display truncation,
  retained measurement size, and metrics export while proving that later usage
  and result reconciliation still survive.

## Verification performed

- `GOCACHE=/tmp/concoct-review-gocache GOMODCACHE=/home/cthain/go/pkg/mod go test -count=1 ./...`
- `GOCACHE=/tmp/concoct-review-gocache GOMODCACHE=/home/cthain/go/pkg/mod go test -race -count=1 ./internal/prompt ./internal/adapter ./internal/execution ./internal/runloop`
- `GOCACHE=/tmp/concoct-review-gocache GOMODCACHE=/home/cthain/go/pkg/mod go vet ./...`
- Native and Windows `go build ./cmd/concoct` using the same caches.
- `bash -n cmd/concoct/concoct.sh` and executable-mode check.
- External temporary-project initialization, including Git/bootstrap, nested
  adapter/current-state presence, and built-in protocol/persona/prompt exclusion.
- `git diff --check 85bbcea49109e81db5297641a69da3bbcd5c8c14..HEAD`.
- Manual inspection of the complete task diff, relevant source, tests,
  documentation, and baseline report.

The first temporary initialization attempt omitted the writable `GOCACHE` and
failed on the sandbox's read-only default cache. Repeating the same documented
initialization with the review cache succeeded. The opt-in live Codex test was
not run because it consumes model allocation.

## Capability impact assessment

The planned `add` impact for CAP-015 is accurate once Finding 1 is resolved.
The Archivist should describe cost attribution, prompt composition, bounded
private measurement, run aggregation, controlled baselines, structured
progress, and the verified Developer fixed-prompt reduction as one new
capability, while retaining Codex version sensitivity and absence of live token
reduction evidence as limitations.

## Scope and documentation assessment

The implementation remains within CON-037's approved scope. The Developer
persona compaction is the selected evidence-backed optimization rather than a
broad persona rewrite, and no unrelated product behavior was found. The command
and workflow documentation is generally aligned with the intended surface, but
its privacy and bounded-progress claims are inaccurate until Finding 1 is
closed.

## Handoff

Return to the Developer with `concoct code`. Remediation should focus on the
single open major finding and on adversarial privacy/bounding tests across live
display, retained measurement, default inspection, and JSON export. The next
Reviewer should recheck that progress truncation cannot prevent terminal usage,
structured result, or reconciliation evidence.
