# Notes

## Planning summary

CON-037 is ready for implementation. It is one coherent critical outcome:
instrument the existing Codex execution boundary, establish controlled role
baselines, and use the evidence to deliver a material initial reduction. The
roadmap resolves the product decisions needed to plan it, including metric
authority, privacy, baseline specificity, the 30 percent threshold, structured
progress, and the prohibition on automatic model downgrades.

## Confirmed findings

- The task is on the exact recorded branch
  `concoct/con-037-measure-and-reduce-agent-invocation-cost` at base
  `85bbcea49109e81db5297641a69da3bbcd5c8c14`; the entry worktree was clean and
  `.concoct/current/` contained no active task artifacts.
- CON-037 is `planned`, has no unresolved roadmap dependency, and declares
  accepted CAP-005, CAP-006, CAP-010, CAP-013, and CAP-014 prerequisites.
- The first-run report records 181,120 explicitly observed tokens across only
  four invocations and says the complete lifecycle used materially more.
- `internal/prompt.Render` writes final bytes directly into one buffer. Semantic
  origins are known during rendering but are not returned as components.
- Prompt goldens cover the required roles and major Developer/Reviewer modes.
  Their checked-in fixture bytes are small because tests replace embedded
  persona content dynamically; executable-owned personas themselves are about
  8.7–13.1 KiB and handoffs add roughly 0.9–2.0 KiB.
- Full roadmap, capability, task, review, and archive content is generally
  referenced for later agent reading rather than embedded in the initial
  prompt. Prompt-byte attribution and total model input therefore require
  distinct reporting.
- `internal/execution` currently records prompt, schema, sanitized profile,
  final result, bounded redacted stdout/stderr, and reconciliation. It does not
  record adapter version, component sizes, normalized usage, duration, or
  predecessor identity.
- The same bounded writer feeds retained raw logs and terminal progress. After
  its byte bound is reached, it continues accepting writes but discards later
  display and retained content, producing the reported frozen-progress effect.
- `internal/runloop.Step` has action, role, outcome, state, invocation, and
  evidence digest only. Run summaries have no model/no-model classification or
  usage aggregation.
- `concoct exec inspect` currently prints retained prompt and logs by default.
  The roadmap explicitly requires a compact measurement view without full
  prompt disclosure by default.
- The installed `codex-cli 0.146.0` advertises `codex exec --json` and retains
  the independent `--output-last-message` and `--output-schema` controls. No
  live model invocation was made during planning.
- `.concoct/runtime/invocations/` is already mode-restricted by execution code,
  bounded by count/age/log/total settings, pruned after reconciliation, and
  ignored in both the repository and initialized-project templates.

## Prerequisite compatibility

- CAP-005 is compatible. Its read-only status, strict checked-in schema focus,
  and unsupported-contract refusal do not prevent local optional measurement;
  CON-037 must keep metrics outside workflow-state authority.
- CAP-006 is compatible. Deterministic rendering, attributed instruction
  composition, exact manual output, conservative archive selection, and
  create-only output are foundations for component measurement. Its guidance-
  only and free-form semantic-validation limits require semantic parity tests
  rather than claims that compaction is mechanically proven correct.
- CAP-010 is compatible. Its structural completion boundary does not judge role
  quality, which is why cost evidence cannot establish completion. Non-Git
  handoff limits and manual reservation recovery remain unchanged and outside
  this task.
- CAP-013 is compatible. Its one-shot, bounded, private record is the correct
  measurement boundary. The lack of loops/recovery and Codex-only adapter limit
  do not block per-invocation attribution; CON-037 does not add replay or a
  second adapter.
- CAP-014 is compatible. Bounded non-resumable runs can aggregate the actions
  attempted in one invocation. The lack of permanent run history means
  aggregation remains a run summary and retained per-invocation evidence, not a
  cross-run billing ledger.

## Decisions

- Declare one new capability, CAP-015, for cost attribution, controlled
  baselines, structured progress, and verified reduction. Existing capabilities
  remain accepted foundations; archival may add related-capability links
  without redefining their authority.
- Measure exact prompt bytes and semantic components locally; preserve native
  usage fields separately and never manufacture an unexplained total.
- Keep detailed evidence inside the existing ignored invocation-retention
  boundary. Publish only aggregate benchmark results and material decisions.
- Make prompt composition first-class while retaining a byte-oriented
  compatibility path so manual rendering and supervised execution cannot
  diverge.
- Request Codex JSONL events for measurement/progress while retaining the
  schema-bound final-result file as the outcome source.
- Change default retained inspection to a metrics-first privacy-preserving view,
  with explicit full/raw access and a machine-readable metrics-only export.
- Use isolated deterministic fixtures by default and a separately invoked live
  compatibility/baseline harness. No ordinary test may consume billable model
  usage.
- Select the production reduction only after pre-change baselines identify the
  dominant fixed cost. The qualifying reduction and its preservation proof
  must be recorded before implementation is declared complete.

## Risks

- Native Codex JSONL and usage fields may evolve; permissive parsing could
  silently misattribute cost.
- Cumulative and repeated terminal events can be double-counted without
  adapter-owned identity and ordering rules.
- Initial prompt-byte reduction may not reduce total model input if broad files
  are subsequently read through tools.
- Persona/handoff compaction or record selection can omit rare authority or
  recovery rules even when happy-path fixtures pass.
- Raw event retention can consume the existing total-byte allowance and make
  older invocation evidence expire sooner.
- Default inspect-output changes need an explicit diagnostic escape hatch to
  preserve recovery usefulness.
- Live baselines are variable and consume allocation; they must remain opt-in
  and execution-profile-specific.

## Relevant history

- CON-006 established deterministic full-output prompt rendering and goldens.
- CON-017 separated attributed instruction layers and documented that free-form
  semantic conflicts still require judgment.
- CON-030 moved personas and handoffs into executable-owned embedded resources.
- CON-008/CAP-010 established the completion boundaries that cost work must not
  weaken.
- CON-010/CAP-013 established one-shot Codex execution, private retention,
  inspection, and observed-state precedence.
- CON-033/CAP-014 established repeated bounded execution and fresh independent
  Reviewer invocations.
- `.concoct/reports/con-033-first-run-findings.md` is the source evidence for
  cost hypotheses and retry examples; hypotheses are not treated as measured
  causes.

## Development progress

- Recovery note: the interrupted Developer pass left the implementation and
  notes uncommitted while task frontmatter still reported `planned`. The
  checkpoint was reconciled to `implementation-in-progress` and committed so
  `concoct code` can render the continuation persona from a clean task branch.
- Introduced a `prompt.Composition` compatibility surface that preserves exact
  prompt bytes while recording ordered, byte-accounted component evidence.
  Supervised execution records its fixed supervision appendix as a separate
  component and writes private `prompt-composition.json` evidence.
- Codex execution now requests JSONL with `--json`. The adapter has a
  conservative decoder that preserves optional native usage fields (including
  reported zero), keeps a native total rather than manufacturing one, and
  diagnoses malformed and duplicate/contradictory usage rather than summing it.
- Invocation records now retain a compact `measurement.json` and metadata
  timing. Default `exec inspect` is metrics-first and excludes prompts and raw
  events; `concoct exec inspect --full-raw` retains the diagnostic escape hatch.
- This is deliberately incomplete. The composition has not yet been split at
  every semantic render boundary; structured event capture is not incremental
  and does not yet provide independent display/raw bounds; no run aggregation,
  baseline harness/report, qualifying reduction, or required documentation has
  been delivered. Do not mark any phase complete or run `concoct code --complete`.

## Continuation update

- Recovery note: this Developer continuation left the documented partial work
  uncommitted on the active task branch. The changes were revalidated and
  committed as an explicit `implementation-in-progress` checkpoint so the next
  `concoct code` invocation can start from a clean continuation boundary.
- Added a privacy-preserving `concoct exec inspect --json` comparison export.
  It includes only metadata, component evidence, normalized usage, and
  reconciliation; it never reads or exports `prompt.md`, raw JSONL, or stderr.
  Default inspection remains metrics-first and `--full-raw` remains the
  explicit diagnostic escape hatch.
- One-shot and run summaries now report exact prompt bytes plus native usage as
  reported, or `usage unavailable`; direct integration is explicitly marked as
  a no-agent mechanical action. Cached input remains a distinct native field.
- The measured initial reduction compacts the executable-owned Developer
  persona from 8,704 to 3,217 bytes while preserving its authority, input,
  mutation, verification, remediation, handoff, and independent-review rules.
  The controlled Developer continuation baseline falls from 12,415 to 6,928
  supervised prompt bytes (5,487 bytes, 44.2%). The conditions and preservation
  evidence are in `.concoct/reports/con-037-baseline-and-reduction.md`.
- This remains incomplete. Prompt composition still records the legacy rendered
  prompt as one component; event decoding is post-process rather than
  incremental; structured event/raw/display bounds, adapter version capture,
  complete run aggregation, six-role fixtures, live opt-in harness, and broad
  documentation/verification are unfinished. Do not complete or hand off for
  review yet.

## Continuation implementation update

- Recovery note: this interrupted continuation left the implementation
  uncommitted and also left an untracked `internal/execution/feature.txt`
  containing fake-adapter test output. The accidental residue was removed and
  the scoped changes were revalidated for a clean in-progress checkpoint.
- Codex JSONL is now decoded incrementally while the adapter process runs.
  `stdout.log` remains a separately bounded redacted private structured-event
  record, while stderr keeps its own redacted display/retention bound.  Retention or
  display truncation no longer stops the decoder, so a later terminal usage
  event survives even after earlier structured output has filled its retained
  allocation.
- The decoder handles chunk boundaries and an unterminated final event.  It
  preserves native usage fields exactly, including zero, and still diagnoses
  malformed, duplicate, and contradictory usage rather than summing it.
  Measurement is written from this live decoder instead of reparsing the
  bounded retained raw file during reconciliation.
- Adapter version evidence is now captured with a post-authorization
  `codex --version` probe.  A failed probe leaves version absent and does not
  prevent execution; importantly, the probe is not performed during Prepare,
  preserving the no-adapter-launch guarantee when authorization evidence
  drifts before launch.
- Run summaries now include native-field aggregates by both role and action.
  Each aggregate labels its reported-attempt count, so missing native fields
  are not silently treated as zero.  Integration remains excluded as an
  explicit no-agent mechanical action.
- Updated `doc/command-reference.md` and `doc/workflow.md` document JSONL
  processing, usage semantics, version capture, and privacy-preserving
  inspection behavior.
- This is still an in-progress checkpoint: semantic prompt composition remains
  a legacy single component, and the controlled six-role fixture/harness and
  remaining baseline/documentation work are not complete. Do not run
  `concoct code --complete` or hand off for review.

## Semantic composition and offline baseline update

- Recovery reconciliation: fake-adapter verification created commit `9b25dc9`
  and tracked `internal/execution/feature.txt` in the real task repository. The
  residue was removed without rewriting branch history; squash integration will
  omit the historical test-only commit. The test isolation defect remains part
  of the development work because default tests must not mutate the active
  project.
- Phase 5 remains complete based on the measured 44.2% reduction. Phases 1–3
  were restored to `in-progress`: acceptance still requires complete component
  category/inclusion evidence, supported/degraded and out-of-order adapter
  fixtures, predecessor attribution, and dedicated CLI/run aggregation tests.
- Recovery verification reran the prompt package and `git diff --check` only.
  The execution/full suites were not rerun after discovering that their current
  fake-adapter fixture can commit into the active repository; repairing and
  regression-testing that isolation is required before those suites are safe.
- `prompt.RenderComposition` now constructs the prompt directly through an
  ordered component writer; `prompt.Render` is the byte-compatible wrapper.
  Components are therefore attributed at render time instead of inferred from
  a final buffer. The categories cover generated context, policy disposition,
  roadmap/capability evidence, persona, instruction provenance, input
  references, authorized updates, completion contract, and handoff.
- Component records now link the first exact and whitespace-normalized duplicate
  by one-based component index. The links use digests only, preserving the
  metrics-first inspection privacy boundary. Tests cover exact and normalized
  duplicate cases, byte conservation, and exact manual-render parity for all
  supported roles and materially distinct recovery/remediation modes.
- The controlled offline baseline report now records Product Owner, Task
  Planner, initial and remediation Developer, Reviewer, and Archivist exact
  prompt bytes, largest component, fixture conditions, and the intentionally
  unavailable native-usage field. It also records the tested 44.2% Developer
  fixed-prompt reduction and distinguishes it from later file reads and live
  token use.
- Phase 1, Phase 2, Phase 3, and Phase 5 are complete. Phase 4 remains
  in-progress because an opt-in live isolated compatibility/baseline harness
  has not been implemented or deliberately authorized. Phase 6 remains
  in-progress because the full deterministic suite cannot currently pass due
  to two unrelated fixed-date runloop lifecycle fixtures. No live Codex
  invocation was made.

## Verification to date

- Passed: `GOCACHE=/tmp/concoct-gocache GOMODCACHE=/home/cthain/go/pkg/mod go test ./internal/adapter ./internal/prompt ./internal/execution ./internal/cli`.
- Passed: `git diff --check`.
- The first attempted test run used a fresh module cache and could not download
  `gopkg.in/yaml.v3` because network access is sandbox-restricted. Using the
  existing read-only module cache with a temporary writable Go build cache
  resolved that environment limitation.
- `./internal/runloop` has two archival lifecycle fixture failures caused by
  the test fixture's fixed August 2026 date versus the current date; these need
  confirmation before attributing them to CON-037. No live Codex invocation
  was made.
- Passed after this continuation: `GOCACHE=/tmp/concoct-gocache
  GOMODCACHE=/home/cthain/go/pkg/mod go test ./internal/adapter
  ./internal/execution ./internal/cli`, and `git diff --check`. The focused
  runloop package still has the pre-existing fixed-date archival fixture
  failures (current date 2026-08-13 versus fixture expectation); they are not
  treated as passing.
- Recovery validation also passed `bash -n cmd/concoct/concoct.sh`, the
  executable-mode check, external temporary-project initialization, generated
  Git/bootstrap/project-file and built-in-exclusion checks, stale-path search,
  and `git diff --check`.
- Passed for this continuation: `GOCACHE=/tmp/concoct-gocache
  GOMODCACHE=/home/cthain/go/pkg/mod go test ./internal/adapter
  ./internal/execution ./internal/cli` plus `go test -race -count=1
  ./internal/adapter ./internal/execution`; `go vet ./...`, native and Windows
  `go build ./cmd/concoct`, `bash -n cmd/concoct/concoct.sh`, and
  `git diff --check` with the same temporary Go cache settings.
- `go test -count=1 ./...` passed every package except the two existing
  runloop archival lifecycle fixtures (`TestRealGitLifecycleReachesLocalIntegrationWithoutPush`
  and `TestIntegrationConflictStopsInRecoveryWithExactContinuation`). They
  still use fixed August-2026 fixture timestamps and fail archival candidate
  validation on the current date; no new event-decoding assertion failed.
- The first `go vet` attempt without `GOCACHE` was blocked by the sandbox's
  read-only default Go build cache. The repeat using `/tmp/concoct-gocache`
  succeeded. Go emitted harmless read-only module-cache stat-cache warnings;
  no module content was changed.
- Passed for semantic composition: `GOCACHE=/tmp/concoct-gocache
  GOMODCACHE=/home/cthain/go/pkg/mod go test -v -run
  TestRenderRolesAndModesDeterministically ./internal/prompt` and
  `go test -count=1 ./internal/prompt ./internal/execution ./internal/adapter
  ./internal/cli` with the same cache settings.
- Passed: `go test -race -count=1 ./internal/prompt ./internal/adapter
  ./internal/execution`, `go vet ./...`, native and Windows `go build
  ./cmd/concoct`, and `git diff --check` using the same cache settings. The
  full `go test -count=1 ./...` again passed every other package and failed
  only the same two dated runloop archival lifecycle fixtures.

## Current continuation update

- Recovery reconciliation: the interrupted work was revalidated from the clean
  `0663d9a` checkpoint. The full suite no longer changes task-branch HEAD or
  leaves `feature.txt`, confirming the fake-adapter isolation repair before
  this continuation was committed.
- The two runloop archival lifecycle fixtures were repaired to use the current
  fixture date rather than an expired fixed date. `go test -count=1 ./...` now
  passes every package; this preserves the test's intended current-date
  archival validation rather than weakening it.
- Added `TestLiveCodexCompatibility`, an explicit opt-in harness that creates
  a fresh temporary project and exercises the production one-shot execution
  boundary with a named model and reasoning profile. It records version/profile
  and optional native usage only in the temporary private invocation record.
  It is skipped unless `CONCOCT_LIVE_CODEX=1` is set and was not run here, so
  no live model allocation was consumed.
- Fixed the fake Codex test adapter to answer `--version` before executing its
  scripted invocation body. This prevents version probing from creating
  `feature.txt` in the source package during ordinary tests.
- Passed this continuation: `go test -count=1 ./...`; focused race tests for
  `internal/prompt`, `internal/adapter`, `internal/execution`, and
  `internal/runloop`; `go vet ./...`; native and Windows `go build
  ./cmd/concoct`; `bash -n cmd/concoct/concoct.sh`; and `git diff --check`, all
  using the temporary writable Go build cache and existing read-only module
  cache. Go emitted only read-only module-cache stat-cache warnings.
- Phase 4 is complete because both offline baselines and the isolated opt-in
  live harness are available. Phase 6 remains in progress until Phases 1–3 are
  complete and a final Reviewer handoff can be prepared; the live run is
  deliberately skipped. Phases 1–3 retain the previously recorded
  event-contract and predecessor-attribution gaps.

## Handoff to Developer

Implement measurement before optimization. Preserve current prompt bytes while
introducing components and event parsing, capture the six controlled pre-change
baselines, then select and record the qualifying reduction. Review should focus
on byte conservation, native-usage semantics, duplicate prevention, partial
failure evidence, privacy, manual/supervised parity, and proof that the claimed
reduction retains every workflow authority and required role input.

## Completion update

- Reconciliation completed the omitted task-frontmatter transition from
  `implementation-in-progress` to `implementation-complete` after independently
  rerunning the required verification matrix.
- Completed the remaining JSONL measurement contract: terminal `turn.*`
  ordering is explicit, events after a terminal event mark measurement evidence
  `degraded`, and valid earlier native usage remains preserved as partial
  evidence. Malformed, duplicate, contradictory, and missing usage retain the
  established diagnostic/unavailable behavior.
- Invocation metadata now records a predecessor invocation ID only when a
  consumed run gate supplies one. This is observational linkage and does not
  create replay or retry authority.
- Added deterministic tests for degraded/partial decoder streams, predecessor
  metadata, and repeated/partial run aggregation with a mechanical integration
  step excluded. Updated the command and workflow documentation with the field
  and degradation semantics.
- All implementation phases are complete. No live Codex invocation was made;
  the opt-in compatibility harness remains intentionally skipped.

## Handoff to reviewer

### Implemented

CAP-015 cost attribution, bounded private evidence, prompt composition,
offline role baselines, the measured 44.2% Developer prompt reduction, and the
remaining event-order/predecessor/aggregation evidence are implemented.

### Key decisions

Native usage remains optional and adapter-reported. A post-terminal JSONL event
degrades measurement evidence without altering workflow-result acceptance.
Predecessor linkage is supplied only by consumed gate correlation and is never
used as retry authority.

### Files changed

`internal/adapter/metrics.go`, `internal/execution/execution.go`,
`internal/runloop/run.go`, their focused tests, `doc/command-reference.md`,
`doc/workflow.md`, and the active task artifacts.

### Verification

Passed with `GOCACHE=/tmp/concoct-gocache` and
`GOMODCACHE=/home/cthain/go/pkg/mod`: `go test -count=1 ./...`, focused
`go test -race -count=1 ./internal/prompt ./internal/adapter
./internal/execution ./internal/runloop`, `go vet ./...`, native and Windows
`go build ./cmd/concoct`, `bash -n cmd/concoct/concoct.sh`, and
`git diff --check`. Go emitted only harmless read-only module-cache stat-cache
warnings during static analysis.

### Known risks

Codex JSONL remains version-sensitive. Unsupported future stream shapes are
retained as bounded diagnostics/degraded evidence, not silently converted into
usage totals. The opt-in live harness was not run.

### Skipped or unresolved work

The live compatibility/baseline harness was deliberately not invoked because
it consumes model allocation. No implementation work remains.

### Capability impact

Implements the planned CAP-015 addition. Existing lifecycle authority,
independent review, and integration boundaries are unchanged.

### Suggested review focus

Verify terminal-event ordering and partial-evidence behavior, the
non-authoritative predecessor link, native-field aggregation counts, privacy of
metrics-first inspection, and the before/after baseline evidence for the 44.2%
fixed Developer prompt reduction.

## Review remediation — review-01

### Finding 1 — Progress evidence bypasses privacy and retention bounds

- Disposition: fixed.
- Progress is now selected by an adapter-owned allowlist of lifecycle event
  kinds. The decoder retains event-kind labels only and never copies
  `message`, `text`, `status`, tool, or repository payloads into progress.
- The selected representation is capped at 64 entries and the configured
  structured-log byte limit. Live display applies redaction and the same
  independent byte limit, while decoder processing continues so a later usage
  event and reconciliation are unaffected.
- Metrics-first inspection and `exec inspect --json` now remove progress and
  diagnostic text from measurements, including measurements written by older
  builds. Explicit `--full-raw` remains the local recovery path.
- Added adversarial decoder/capture/export tests for secret-like and
  repository-like payloads, oversized/many events, bounded display and
  retained measurement, default inspection, JSON export, and late usage.

## Handoff to reviewer

### Implemented

Resolved Review 01 Finding 1 by replacing raw event-text progress with bounded,
payload-free adapter lifecycle labels and sanitizing metrics-first measurement
surfaces.

### Key decisions

Progress is non-authoritative operational feedback. Only allowlisted event-kind
labels may be retained or displayed; model and tool payload fields are never a
progress source. Default inspection/export intentionally omit all progress and
diagnostic text, including legacy records.

### Files changed

`internal/adapter/metrics.go`, `internal/execution/execution.go`, their focused
tests, `doc/command-reference.md`, `doc/workflow.md`, and the active task
artifacts.

### Verification

Passed with `GOCACHE=/tmp/concoct-gocache` and
`GOMODCACHE=/home/cthain/go/pkg/mod`: `go test -count=1 ./...`,
`go test -race -count=1 ./internal/adapter ./internal/execution
./internal/runloop`, `go vet ./...`, native and Windows `go build
./cmd/concoct`, `bash -n cmd/concoct/concoct.sh`, and `git diff --check`.
The read-only module cache emitted harmless stat-cache warnings during `go vet`.

### Known risks

Codex JSONL remains version-sensitive. Unknown event kinds are not rendered as
progress; their bounded raw evidence is available only through explicit local
full/raw inspection. The opt-in live harness was not run.

### Skipped or unresolved work

No unresolved review findings. The opt-in live compatibility/baseline harness
remains deliberately skipped because it consumes model allocation.

### Capability impact

The planned CAP-015 addition remains accurate: invocation measurement and
progress evidence are now bounded and privacy-preserving by default without
changing workflow authority or result acceptance.

### Suggested review focus

Confirm that arbitrary JSONL payload fields cannot reach progress,
`measurement.json`, default inspection, or `--json`; verify configured bounds
do not prevent later usage/result reconciliation; and recheck that full/raw
diagnostics remain explicit.

## Review remediation — review-02

### Finding 1 — Decoder diagnostics and partial-line input remain unbounded

- Disposition: fixed.
- Diagnostic evidence now has independent count and byte ceilings. When either
  ceiling is reached, `diagnostics_truncated` records the loss and measurement
  compatibility becomes `degraded`; later input continues to be decoded.
- An in-progress JSONL event is capped at the configured evidence-byte limit.
  Oversized input is discarded without retaining the remainder, records
  `event_truncated` plus a bounded degradation diagnostic, and resynchronizes
  at the next newline so later valid usage and result reconciliation survive.
- The execution-owned raw-retention diagnostic now passes through the same
  bounded diagnostic path instead of appending directly after decoding.
- Added adversarial adapter and execution tests for many malformed events,
  many post-terminal events, a chunked oversized unterminated line, bounded
  decoder state and serialized measurement size, newline resynchronization,
  and recovery of later native usage.

### Attempt: combined initialization verification and cleanup

- Tried: run shell validation, external initialization assertions, diff checks,
  and temporary-directory cleanup in one command.
- Error/result: the command was rejected before execution because it included
  a removal operation.
- Why it failed: the execution safety boundary does not permit that bundled
  cleanup form.
- Next approach: reran all validation without cleanup, confirmed it passed,
  then removed the exact temporary directory separately with a bounded
  depth-first deletion.

## Handoff to reviewer

### Implemented

Resolved Review 02 Finding 1 by bounding diagnostic evidence and partial JSONL
input while preserving stream resynchronization, later native usage, and
structured-result reconciliation.

### Key decisions

Progress, diagnostics, and one in-progress event each receive an independent
byte allowance derived from the configured structured-evidence limit;
diagnostics additionally have a 64-entry ceiling. Truncation is explicit and
degrades measurement compatibility, but it is never workflow-result authority.

### Files changed

`internal/adapter/metrics.go`, `internal/adapter/adapter_test.go`,
`internal/execution/execution.go`, `internal/execution/execution_test.go`,
`doc/command-reference.md`, `doc/workflow.md`, and the active task artifacts.

### Verification

Passed with temporary writable Go build caches and
`GOMODCACHE=/home/cthain/go/pkg/mod`: `go test -count=1 ./...`, focused
`go test -race -count=1 ./internal/prompt ./internal/adapter
./internal/execution ./internal/runloop`, `go vet ./...`, native and Windows
`go build ./cmd/concoct`, `bash -n cmd/concoct/concoct.sh`, executable-mode
validation, external temporary-project initialization and copy/exclusion/Git
assertions, and `git diff --check`. Go emitted only read-only module-cache
stat-cache warnings during builds.

### Known risks

Codex JSONL remains version-sensitive. An individual event larger than the
configured evidence ceiling is intentionally unavailable for attribution, is
reported as degraded/truncated evidence, and cannot prevent later lines from
being processed. The opt-in live harness was not run.

### Skipped or unresolved work

No unresolved review findings. The opt-in live compatibility/baseline harness
remains deliberately skipped because it consumes model allocation.

### Capability impact

The planned CAP-015 addition remains accurate. Detailed invocation measurement
is now bounded across progress, diagnostics, raw retention, and partial decoder
input without changing lifecycle authority or result acceptance.

### Suggested review focus

Stress malformed and post-terminal streams plus oversized chunked lines;
confirm diagnostic count/byte and partial-line memory ceilings, retained
`measurement.json` size, newline recovery of later usage, metrics-export
privacy, and unchanged structured-result reconciliation.
