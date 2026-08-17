# Notes

## Planning summary

CON-038 is `ready` for implementation. It is a critical but coherent
execution-cost task: semantic activity evidence is required to choose and prove
the interaction changes, budget behavior, context compaction, and
failed-finalization accounting in one acceptance cycle. General durable run
recovery remains outside this task.

The exact Git context supplied by `concoct plan` is:

- trunk: `main`
- task branch: `concoct/con-038-reduce-agent-context-amplification-and-bound-inv`
- base: `7a7fdaae6bf31b8c237cb07d13a68edf8e499ba6`

The branch was checked out clean at that base before planning artifacts were
authored.

## Confirmed findings

- CON-038 exists at status `planned`, priority `critical`, with no unresolved
  roadmap dependency. No populated active task or current review conflicts
  with it.
- CAP-005, CAP-013, CAP-014, and CAP-015 are uniquely present and `active`.
  Their limitations are compatible with the selected outcome: the task does
  not require legacy migration, a second adapter, automatic replay, crash
  recovery, permanent run history, or fabricated usage.
- Retained production records confirm the roadmap table for invocation IDs
  `2a973d09d420622dbaf99aba747c8742c184` (Developer),
  `376a948d9edbcbffa471d0a4175627f054dc` (Reviewer), and
  `266ac422ae8389c81f58f649038cad4f8a2a` (Archivist). All identify Codex CLI
  `0.146.0`; newer ready-state records identify `0.147.0`, so current live
  compatibility still needs explicit validation.
- The retained streams expose `agent_message`, `command_execution`, and
  `file_change` items. Completed command events carry command text, aggregated
  output, exit code, and status, but the observed events have no native
  timestamp/duration field.
- The measured Developer performed 25 completed commands and four file changes;
  the Reviewer performed 16 commands and one file change; the Archivist
  performed seven commands and one file change. Broad planning, notes,
  capability, and diff reads produced individual outputs up to tens of
  kilobytes, and some inputs were read more than once in one invocation.
- The Developer's retained raw log filled its 256 KiB bound before a terminal
  event, while the streaming decoder still captured the later usage snapshot.
  This proves the value of keeping decoding independent from retention and also
  shows why normalized payload-safe evidence must not depend on raw logs.
- `internal/adapter/metrics.go` currently permits only one usage object and
  stores generic progress labels. Per-exchange snapshots, item semantics,
  repetition, command output, compaction, and availability provenance require
  a richer adapter-neutral model.
- `internal/execution` already has process-group timeout/cancellation and
  independent evidence bounds that can be reused for hard stops. The stop
  decision must remain separate from the existing timeout disposition.
- `internal/config` already supplies strict user/project role profiles and
  invocation overrides with source provenance. Per-role budgets should extend
  this model rather than create an unrelated configuration path.
- Default raw `stdout.log` retention can contain command output and repository
  content. It is private, redacted, bounded, pruned, and Git-ignored, but it is
  not optional or summarized by default today.
- Current progress records at most 64 allowlisted top-level lifecycle labels.
  After that bound is reached, later activity is not shown; the labels do not
  distinguish reading, searching, editing, testing, or completion.
- Embedded persona sizes are currently 3,217 bytes for Developer, 9,144 for
  Reviewer, 11,531 for Archivist, 11,919 for Product Owner, and 13,063 for Task
  Planner. Handoffs and generated prompt sections repeat some of the same role
  inputs and completion rules.
- The existing live harness covers an isolated Product Owner action only. The
  existing checked-in role report is deterministic and prompt-byte focused; it
  does not establish a processed-input reduction for Developer, Reviewer, or
  Archivist.
- CAP-015 cites `.concoct/reports/con-037-agent-cost-baseline.md`, which does
  not exist. The accepted report is
  `.concoct/reports/con-037-baseline-and-reduction.md`. Because capability truth
  is Archivist-owned, planning records the discrepancy for reconciliation after
  approval rather than editing it now.

## Decisions

- Declare capability impact as updates to CAP-013, CAP-014, and CAP-015. The
  work extends existing execution, run coordination, and measurement rather
  than creating a separate durable capability.
- Use total processed input as the primary amplification metric while retaining
  native cached and uncached distinctions. Do not convert these measurements
  into subscription or monetary claims.
- Keep native event parsing and version mapping in the adapter/execution
  boundary. Downstream reporting consumes normalized evidence and explicit
  availability/provenance.
- Treat command classification as conservative adapter-level evidence. Complex
  or unknown shell activity must not be forced into a misleading category.
- Use bounded fingerprints and aggregate sizes/counts in default metrics.
  Commands, paths, output, prompts, and repository content remain available
  only through explicit bounded local diagnostics when enabled.
- Warning budgets can report any observable dimension. A hard bound is exposed
  only when it can terminate before accepting completion; terminal-only token
  usage is not a live enforceable budget.
- Distinguish budget exhaustion from timeout, cancellation, adapter failure,
  refusal, and finalization failure in both retained evidence and user output.
- Restrict no-new-agent recovery to candidate preservation, precise invariant
  diagnosis, reusability classification, and mechanical/manual completion that
  does not require role judgment. CON-034 retains general resumability and
  abandoned-attempt ownership.
- Require evidence-led prompt and interaction changes across all roles, with
  priority on Reviewer and Archivist. Prompt compaction alone cannot satisfy
  the processed-input threshold.
- Require an explicit operator-approved live compatibility and controlled
  benchmark run before implementation can claim the 50 percent processed-input
  criterion. Ordinary tests remain offline.

## Assumptions

- The observed Codex event shapes are representative enough to design fixtures,
  but `0.147.0` semantics will not be claimed until the explicit compatibility
  capture runs.
- Host-observed stream timing and byte lengths can be useful if clearly
  distinguished from native adapter fields.
- Existing workflow parsers can supply selected roadmap, capability, archive,
  task, and review records without moving semantic workflow authority into the
  prompt layer.
- Current process-group termination is an appropriate mechanical primitive for
  enforceable budget exhaustion once late-result rejection is added and tested.
- The three retained production records remain local originating evidence; the
  tracked report should preserve only bounded aggregates and conclusions.

## Risks

- Token baselines vary with model service behavior and cache state even under a
  fixed local fixture. Reports need exact controls plus deterministic activity
  evidence and must avoid universal cost claims.
- A live token warning may be impossible before the terminal event for the
  supported Codex version. Reporting must distinguish live from terminal-only
  evaluation.
- Shell command classification and normalization can leak sensitive values or
  create false repetition groups if designed carelessly.
- A hard-stop/result race could incorrectly accept a stale structured result.
  The stop decision must have explicit precedence in reconciliation.
- Selected context can omit rare authority. Safe structural fallback to the
  full input is preferable to an unsafe reduction.
- The 50 percent processed-input threshold is materially harder than fixed
  persona compaction and may require repeated-read, output-volume, and
  check-sequencing changes together.
- Live compatibility and benchmark runs consume model allocation and therefore
  require explicit operator approval during development.

## Relevant history

- CON-010 established one-shot supervised execution and bounded private
  invocation records.
- CON-033 added finite lifecycle coordination, independent Reviewer
  invocations, gates, and run bounds but deliberately deferred crash recovery.
- CON-037 added prompt composition, optional native usage, bounded structured
  decoding, privacy-preserving inspection, run aggregation, and a 44.2 percent
  fixed Developer prompt reduction. Its two review cycles are useful evidence
  for adversarial privacy and memory-bound testing.
- `.concoct/reports/con-033-first-run-findings.md` records the broader real-run
  completion and recovery issues. CON-038 takes only the cost, progress, and
  no-new-agent finalization-accounting slice; other lifecycle recovery remains
  separate.

## Handoff to developer

### Current state

Planning is complete and the intended workflow state is `planned` after the
planning transition validates and commits.

### Work completed

The roadmap outcome, prerequisite compatibility, retained production evidence,
current adapter/execution/configuration/prompt behavior, historical findings,
scope boundary, phases, acceptance criteria, verification, and capability
impact have been validated and recorded.

### Work remaining

Implement the five task phases in order, preserve a pre-change controlled
baseline before altering role context, and do not claim completion without the
operator-approved live compatibility and processed-input threshold evidence.

### Key decisions

Keep event semantics adapter-owned, make unavailable evidence explicit, expose
only enforceable hard bounds, preserve workflow authority, use payload-safe
repetition evidence, and limit recovery to deterministic/manual completion of
safe retained candidates.

### Known risks

Version-sensitive JSONL, terminal-only token usage, command-classification
ambiguity, hard-stop/result races, context-selection omissions, benchmark
variability, and the live allocation requirement need explicit treatment.

### Checks run

- Confirmed branch `concoct/con-038-reduce-agent-context-amplification-and-bound-inv`
  at base `7a7fdaae6bf31b8c237cb07d13a68edf8e499ba6` with a clean worktree.
- Inspected all executable-selected prerequisite capability records and archive
  summaries.
- Inspected the selected roadmap item, retained CON-037 measurements and event
  shapes, prior CON-037 plan/reviews, first-run findings, relevant source,
  tests, configuration, prompts, and documentation.
- No live or billable model invocation was run during planning.

### Artifacts created or updated

- `.concoct/current/task-plan.md`
- `.concoct/current/notes.md`
- `.concoct/roadmap.md` selected CON-038 status only

### Capability impact

Expected `update` impact to CAP-013, CAP-014, and CAP-015. Do not edit the
capability ledger during development.

### Expected next role

Developer.

### Recommended next command

`concoct code`

## Implementation findings and decisions

- Normalized Codex evidence now uses finite payload-free activity categories,
  bounded 64-bit SHA-256 fingerprints for repeated commands, aggregate command
  output bytes, a 32-entry usage-snapshot bound, and explicit
  native/host-observed/conservatively-classified/unavailable provenance. Generic
  items are not called model exchanges. Ambiguous pipelines remain
  `command-other`.
- Supported observed Codex item mappings are `agent_message`/`reasoning` to
  `model-message`, `file_change` to `edit`, `command_execution` to conservative
  read/search/check/other categories, and explicit `compaction` only when the
  native type is present. Exchange usage, compaction, and item duration remain
  unavailable unless explicitly evidenced; usage snapshots are never summed.
- Raw JSONL retention now defaults off. Explicit `exec.retention.raw-events`
  retains the existing private, redacted, independently bounded, Git-ignored
  diagnostic stream. Result, measurement, and reconciliation remain available
  without raw payload retention.
- Warning-only Developer/Reviewer/Archivist defaults were derived above the
  retained CON-037 observations. Live hard configuration is limited to elapsed
  time, normalized activity, and command-output bytes. Token warnings are
  terminal-only because the supported stream does not expose enforceable live
  token totals.
- A hard stop has distinct `budget-exhausted` disposition, uses the existing
  process-group termination path, rejects terminal candidates even when the
  process exit races the final observation, preserves partial measurement, and
  records evidence-class-specific reuse guidance. Finalization failures now
  name the invariant and mark their cost as wasted.
- Run summaries label attempts and aggregates as accepted or wasted and expose
  disposition, activity, and command-output totals. Semantic progress reports
  the current category and, after its independent display bound fills, emits a
  final bounded latest-activity marker.
- Shared one-pass discovery, bounded output, focused iteration, reasoned
  reread/rerun, broad completion validation, and early-stop guidance lives once
  in the executable-owned protocol. The Archivist persona was compacted from
  11,531 to 5,293 bytes (54.1%) after restoring the exact roadmap-reference
  syntax exposed by the first live failure, while retaining authority, prohibitions,
  transaction order, Git/non-Git completion, overrides, recovery, and handoff.
  Reviewer and Developer personas were left unchanged because deterministic
  evidence did not justify removing additional role-specific authority.
- Existing generated exact-input references and complete-authority fallback
  were preserved. No narrower task-context selector was added because the
  current structural inputs do not prove that omitting ledger/history records
  is safe across every role/state; the interaction guidance instead directs one
  bounded batched read of those authoritative inputs.

## Verification and attempts

### Attempt: external initialization with default Go cache

- Tried: `./cmd/concoct/concoct.sh init /tmp/concoct-init-9azlUV/generated`.
- Error/result: the wrapper's Go build could not write the host default cache.
- Why it failed: `/home/cthain/.cache/go-build` is read-only in this supervised
  environment, not an initialization defect.
- Next approach: reran with `GOCACHE=/tmp/concoct-go-cache`; initialization and
  all external assertions passed.

Completed deterministic checks:

- `gofmt` on every changed Go file.
- Focused adapter, config, execution, run-loop, and prompt tests.
- `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./...`.
- Race tests for adapter, prompt, execution, and run-loop packages.
- `go vet ./...`, native build, and Windows amd64 build with the isolated cache;
  the Go command emitted non-fatal stat-cache warnings for the read-only module
  cache while the combined check completed successfully.
- `bash -n cmd/concoct/concoct.sh` and executable-mode validation.
- Fresh external initialization under `/tmp/concoct-init-9azlUV/generated`:
  verified nested/dotfile installation, current/archive directories, exclusion
  of built-in protocol/persona/prompt files, Git initialization, bootstrap
  prompt, staged files, and no generated commit.
- Source/template Concoct skill byte parity, deterministic role prompt byte
  comparison, Archivist persona byte target, `git diff --check`, and full diff
  inspection.

The opt-in live role harness was added and initially left unrun because it
consumes model allocation and requires explicit operator approval. The later
approved attempt is recorded below. Installed Codex event compatibility and
the required 50 percent processed-input reduction remain unproven.
`.concoct/reports/con-038-context-amplification.md` records the exact command,
controls, deterministic results, and live outcome.

### Attempt: operator-approved live role benchmark

- Tried: `CONCOCT_LIVE_CODEX_ROLES=1 CONCOCT_LIVE_CODEX_MODEL=gpt-5.6-sol CONCOCT_LIVE_CODEX_REASONING=medium GOCACHE=/tmp/concoct-go-cache go test -v -count=1 -run TestLiveCodexRoleBenchmarks ./internal/execution` with Codex CLI 0.147.0.
- Error/result: all three disposable fixtures failed. Developer returned an accepted `blocked` non-completion outcome after 71.64s; Reviewer finalization rejected a mismatched/missing body `Outcome` after 82.75s; Archivist finalization rejected a missing exact roadmap archive-summary reference after 278.33s. Total suite duration was 432.72s.
- Why it failed: the three role candidates did not satisfy their fixture completion/finalization contracts. The harness also logs measurement only after success, so the failed run did not emit its normalized activity, command-output, or native usage summary.
- Next approach: improve the harness to emit bounded measurement and reconciliation evidence for failed candidates, inspect the fixture prompts/contracts for the three failure modes, run all resulting checks offline, and request explicit operator approval before any billable retry.

### Live evidence retention correction

- The execution boundary had already persisted measurement and reconciliation before returning lifecycle/finalization errors. The live test discarded that valid evidence because its success-only log followed `t.Fatalf`, and the fixture's `t.TempDir` cleanup then removed the private invocation record.
- The role suite now requires `CONCOCT_LIVE_CODEX_EVIDENCE_DIR` to be an absolute path before launching any adapter. Each role exports the existing privacy-preserving `Metrics` surface to a create-only `<role>-metrics.json` file and logs the same JSON before checking the run error or acceptance state.
- Create-only output prevents a retry from overwriting prior paid evidence. The external directory survives disposable fixture cleanup; prompts, command text/output, raw events, diagnostics, and repository content remain excluded by the existing metrics-first boundary.
- `TestLiveBenchmarkMetricsSurviveRejectedOutcome` uses a non-billable fake Codex stream to prove that a rejected completion retains native token usage, normalized activity, command-output bytes, outcome/reconciliation state, and adapter metadata, and refuses overwrite.
- This corrects evidence loss but does not reinterpret lifecycle failure as benchmark success. The three fixture contract failures still require diagnosis before a fresh operator-approved run.

### Live fixture contract correction

- Developer root cause: `plannedGitFixture` exposed only metadata plus an empty `# Task Plan`; it contained no implementable goal or acceptance criteria. The model's accepted `blocked` outcome was reasonable. The fixture now requests one exact root file, bounds scope, states byte-level acceptance, and supplies a direct verification command. Later Reviewer/Archivist starting states include that accepted implementation.
- Reviewer root cause: the persona's suggested artifact showed an empty `## Outcome` even though deterministic parsing requires exactly one backticked `approved`, `changes-requested`, or `blocked` matching front matter. The example and explicit lexical rule now agree with validation.
- Archivist root cause: compacted guidance said to use supported roadmap evidence but omitted the validator-required directory-form `- Archive:` syntax and the distinction from a `summary.md` file path. The persona now states both exactly while preserving the existing Git pending-delivery boundary.
- `TestLiveRoleBenchmarkFixturesReachCompletionOffline` uses the same planned, implementation-complete, and approved fixture constructors with a fake adapter. It reaches the production completion boundary for all three roles and asserts durable states `implementation-complete`, `approved`, and `archived` respectively.
- `TestLiveRoleBenchmarkContractsAreConcrete` checks the actual authorities: the Developer fixture task contains the concrete file/acceptance contract, and the executable-owned Reviewer and Archivist resources contain their matching outcome and directory-to-summary parser rules. Existing exact-manual-prompt parity remains independently verified.
- These are offline corrections only. No live Codex call or other billable test was run; a fresh live suite remains gated on explicit operator authorization.
- Passed after correction with `GOCACHE=/tmp/concoct-go-cache`: focused benchmark contract/completion tests, `go test -count=1 ./...`, race tests for `internal/execution` and `internal/prompt`, and `go vet ./...`; also passed `bash -n`, executable-mode validation, and `git diff --check`.
- Fresh initialization at `/tmp/concoct-con038-fixture-Wo3l7h/generated` passed: nested project-owned dotfiles, current/archive planning directories, staged bootstrap prompt, Git initialization, and no generated commit were present; built-in protocol, personas, and prompts remained excluded from installed project files.

### Attempt: corrected operator-approved live role benchmark

- Tried: the documented role suite with Codex CLI 0.147.0, `gpt-5.6-sol`, `medium`, and durable evidence directory `/tmp/concoct-con038-live-evidence-OinrtY`.
- Error/result: all metrics were retained. Developer used 99,754 input / 78,592 cached / 2,009 output tokens with 12 activity events in 61.67s; Reviewer used 151,456 / 125,184 / 3,388 with 13 activities in 85.38s; Archivist used 93,882 / 50,432 / 5,101 with 11 activities in 107.67s. Aggregate input was 345,092, 86.6% below the 2,575,638 originating aggregate; the exact-profile Developer comparison fell 94.3%.
- Why it failed: all roles returned structured `completed`, but Developer left task status planned, Reviewer lacked executable reservation provenance, and Archivist copied notes before its final current-notes update. The first two reproduce lifecycle defects already documented in CON-033; the third exposed transaction-order ambiguity in the compacted persona.
- Next approach: make Developer durable completion status explicit, establish Reviewer reservation in the benchmark starting evidence instead of asking the model to create executable provenance, order Archivist's notes handoff before its final archive copy, prove all three offline, and require fresh approval before any live retry.

### Second live follow-up corrections

- Developer persona now states that structured `completed` requires exact durable `status: implementation-complete`; otherwise it must return a non-completion outcome.
- The implementation-complete Reviewer benchmark fixture now contains committed executable-style reservation evidence before invocation. This isolates review judgment from the separately known run-orchestration reservation defect while preserving completion validation.
- Archivist transaction order now adds the final current-notes handoff before copying archive artifacts and explicitly re-copies notes after any later change.
- The non-billable three-role completion test passes with these follow-up corrections. No third live attempt was run.
- Full offline tests, execution/prompt race tests, vet, shell/executable checks, and diff validation pass after the follow-up. External initialization at `/tmp/concoct-con038-followup-zwUtM7/generated` again verified Git/bootstrap/staged project files and exclusion of built-in protocol/persona/prompt resources.

## Operator-approved acceptance amendment

- On 2026-08-14 the operator directed that no further billable CON-038 benchmark be run. CON-037 and CON-038 had consumed approximately 45 percent of weekly allocation, and additional equivalent-workload runs were not justified merely to satisfy an arbitrary numeric gate.
- The 50 percent controlled causal-reduction criterion and successful-live-finalization criterion are withdrawn for this task. The 94.3/71.2/69.4/86.6 percent live differences remain recorded only as directional observations because the originating and benchmark workloads differ materially.
- Accepted completion evidence is Codex 0.147.0 compatibility, durable measurement across failure dispositions, payload-safe semantic attribution, warning/enforceable budgets, hard-stop safety, accepted/wasted accounting, deterministic prompt/persona reduction, the 54.1 percent Archivist persona reduction, and offline completion-boundary coverage.
- Developer status, Reviewer reservation orchestration, empty completion diagnostics, and Archivist transaction ordering remain documented lifecycle defects. Corrections made within scope are retained, but CON-038 does not require another paid end-to-end proof of those separate lifecycle concerns.
- Phase 5 and the task are complete under this explicit human-approved amendment. Independent review should assess the delivered implementation and the honesty of its limitations, not reinstate the withdrawn billable threshold.

## Handoff to reviewer

### Implemented

Payload-safe semantic measurement, repeated-operation/output evidence, usage
snapshots and availability, semantic rollover progress, strict role budgets,
safe hard stops, raw-retention opt-in, accepted/wasted accounting, candidate
reuse diagnostics, shared interaction discipline, the compact Archivist
persona, disposable three-role live harness, and documentation/reporting.

### Key decisions

Do not infer model exchanges, compaction, duration, or token deltas; do not
offer hard token bounds; keep hard stops separate from timeout; preserve full
authoritative context when omission safety cannot be proved; keep billable live
evidence operator-approved.

### Files changed

Adapter/config/execution/run-loop source and tests; live benchmark harness;
protocol and Archivist persona; README and execution/workflow documentation;
CON-038 report; active plan and notes.

### Verification

All offline full, focused, race, vet, native/Windows build, shell, external
initialization, parity, deterministic benchmark, diff, and executable checks
listed above passed. The billable live compatibility/benchmark command was not
run.

### Known risks

Current installed Codex event semantics and the 50 percent total-processed-input
acceptance threshold remain unverified. Command classification is deliberately
conservative, and command-output hard enforcement can observe output only when
the completed event fits the independently bounded decoder.

### Skipped or unresolved work

Operator-approved Developer/Reviewer/Archivist live collection. Phase 5 is
blocked and the task remains `implementation-in-progress`; this handoff is
prepared for eventual independent review but is not a completion claim.

### Capability impact

Expected updates to CAP-013, CAP-014, and CAP-015 remain accurate. The capability
ledger was not edited.

### Suggested review focus

Event classification and privacy, cumulative snapshot semantics, hard-stop and
late-result races, configuration precedence, raw-retention defaults, accepted
versus wasted accounting, role authority after compaction, benchmark isolation,
and the unresolved live processed-input evidence.

## Handoff to reviewer

### Implemented

CON-038 now provides payload-safe semantic activity and repetition evidence,
optional bounded raw retention, usage availability/provenance, semantic
progress rollover, strict warning and enforceable budgets, safe hard stops,
accepted/wasted accounting, candidate-reuse diagnostics, compacted role
guidance, durable live benchmark metrics across failure outcomes, and corrected
benchmark completion contracts. The operator-approved acceptance amendment is
reflected in the plan and report without claiming a causal live percentage.

### Key decisions

Do not infer unavailable exchanges, compaction, duration, or token deltas; do
not expose hard token budgets when usage is terminal-only; preserve metrics
independently of lifecycle acceptance; and do not spend additional model
allocation to satisfy the withdrawn 50 percent equivalent-workload gate. The
two live attempts are directional compatibility/observability evidence only.

### Files changed

Adapter, configuration, execution, run-loop, prompt/protocol/persona source and
tests; README and command/workflow documentation; the CON-038 evidence report;
and active task plan/notes. The capability ledger remains unchanged for the
Archivist.

### Verification

Final verification passed with `GOCACHE=/tmp/concoct-go-cache`: `go test
-count=1 ./...`; race tests for adapter, prompt, execution, and run-loop;
`go vet ./...`; native and Windows amd64 builds; `bash -n`; executable-mode
validation; and `git diff --check`. Earlier post-template external
initialization verified dotfiles, nested project-owned templates, planning
directories, Git/bootstrap/staging, and exclusion of built-in
protocol/persona/prompt files. No model call was made after the operator cost
stop.

### Known risks

Codex JSONL remains version-sensitive and token warnings remain terminal-only.
The live workloads were not equivalent to the originating production tasks, so
their numerical differences are not causal reduction evidence. Supervised
Developer diagnostics, Reviewer reservation orchestration, and other lifecycle
recovery defects remain documented separately; offline completion fixtures
pass but no further paid end-to-end proof is required by the amended plan.

### Skipped or unresolved work

No further billable benchmark will be run for CON-038. Equivalent-workload
causal benchmarking and paid confirmation of the follow-up lifecycle guidance
were explicitly withdrawn by the operator because CON-037/CON-038 consumed
approximately 45 percent of weekly allocation. Those omissions are accepted
limitations, not hidden passing claims.

### Capability impact

Expected updates to CAP-013, CAP-014, and CAP-015 remain accurate: execution
and run coordination gain bounded semantic measurement, budgets, stop and
cost-disposition evidence; measurement gains activity/repetition/output and
compatibility evidence. Capability truth should state the causal-benchmark
limitation and reconcile CAP-015's stale CON-037 report filename.

### Suggested review focus

Verify privacy and bounds, usage-snapshot semantics, hard-stop/result races,
configuration provenance, accepted versus wasted accounting, raw-retention
defaults, role authority after compaction, durable failed-outcome metrics, the
offline completion contracts, and faithful application of the explicit
operator acceptance amendment.

## Review 01 remediation

- Recovery reconciliation: the supervised Developer candidate implemented and
  verified both findings but finalization returned an empty diagnostic because
  task front matter omitted the deterministic relationship
  `remediates-review: review-01.md`. That relationship was restored without a
  new model invocation; candidate source, tests, dispositions, and handoff were
  preserved unchanged.
- Finding 1 — fixed. A per-invocation warning tracker records each
  live-observable elapsed, activity, and command-output threshold once, merges
  it into the final measurement for successful, nonzero-exit, cancellation,
  timeout, and hard-budget paths, and preserves terminal-only token warnings
  separately. Focused coverage verifies completed, nonzero-exit, and
  hard-stopped runs without duplicate events.
- Finding 2 — fixed. Check classification now requires affirmative subcommands
  or targets (`go test`/`vet`, selected `npm`, `cargo`, or `make` forms).
  General-purpose commands such as `go env`, shell scripts, `npm publish`, and
  ambiguous pipelines remain `command-other`. Focused positive, negative, and
  ambiguous mapping coverage was added.

## Handoff to reviewer

### Implemented

Resolved both Review 01 findings: live-observable warnings are once-only and
survive every terminal disposition, while test/check classification is now
evidence-based and conservative.

### Key decisions

Warning `evaluation: live` denotes a dimension observable during execution;
token usage remains `terminal-only`. The tracker stores threshold-crossing
evidence outside the decoder, then attaches it to the completed measurement so
process termination cannot discard it.

### Files changed

`internal/execution/execution.go`, `internal/execution/execution_test.go`,
`internal/adapter/metrics.go`, `internal/adapter/adapter_test.go`, and the
active plan/notes.

### Verification

- `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./...` — passed.
- `GOCACHE=/tmp/concoct-go-cache go test -race -count=1 ./internal/adapter ./internal/prompt ./internal/execution ./internal/runloop` — passed.
- `GOCACHE=/tmp/concoct-go-cache go vet ./...` — passed (the Go tool emitted non-fatal read-only module stat-cache warnings).
- Native and `GOOS=windows GOARCH=amd64` builds — passed.
- `bash -n cmd/concoct/concoct.sh`, executable-mode check, and `git diff --check` — passed.

### Known risks

Warnings are sampled on the existing 25 ms execution ticker; a process that
ends between ticks is still assessed while its final measurement is assembled.
Codex JSONL remains version-sensitive, and token warnings remain terminal-only.

### Skipped or unresolved work

No additional billable benchmark was run under the recorded operator cost
stop. No unresolved Review 01 findings remain.

### Capability impact

The declared CAP-013, CAP-014, and CAP-015 update remains accurate; the ledger
is intentionally unchanged until archival.

### Suggested review focus

Confirm warning-event ordering/deduplication around hard-stop races and ensure
the intentionally narrow command forms do not reintroduce false-positive
test/check attribution.

## Review 02 remediation

- Finding 1 — fixed. Added deterministic execution coverage for both
  `hard-activity` and `hard-command-output-bytes`. Each case emits a threshold
  event, writes a valid structured candidate before the stop is observed, and
  remains running. The tests verify the distinct live `budget-exhausted`
  evidence with project-role provenance, preserved activity/output measurement,
  preserved candidate, absent accepted result, rejected late-candidate reuse
  classification, and unchanged `ready` workflow state.

## Handoff to reviewer

### Implemented

Added missing event-driven hard-budget safety coverage for activity and command
output, including stop-decision precedence over a candidate result that exists
before termination.

### Key decisions

The tests use a real decoded command event and the configured project role
budget for each dimension. A candidate is intentionally written before the
polling stop so the assertions cover late-result rejection without a billable
adapter invocation.

### Files changed

`internal/execution/execution_test.go`, `.concoct/current/task-plan.md`, and
`.concoct/current/notes.md`.

### Verification

- `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./internal/execution` — passed.
- `GOCACHE=/tmp/concoct-go-cache go test -count=1 ./...` — passed.
- `GOCACHE=/tmp/concoct-go-race go test -race -count=1 ./internal/adapter ./internal/prompt ./internal/execution ./internal/runloop` — passed.
- `GOCACHE=/tmp/concoct-go-vet go vet ./...` — passed; Go emitted only non-fatal read-only module stat-cache warnings.
- Native and `GOOS=windows GOARCH=amd64` builds of `./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` and `git diff --check` — passed.
- `GOCACHE=/tmp/concoct-go-cache go run ./cmd/concoct status` confirms valid `implementation-complete` remediation evidence for `review-02.md` and recommends `concoct review`.

### Known risks

Budget polling remains deliberately periodic; terminal reconciliation retains
the same hard-budget precedence if process completion wins the poll race.

### Skipped or unresolved work

No additional paid benchmark was run under the recorded operator cost stop.
No Review 02 findings remain unresolved.

### Capability impact

The planned CAP-013, CAP-014, and CAP-015 updates remain accurate. The
capability ledger is intentionally unchanged until archival.

### Suggested review focus

Confirm both event-driven limits retain partial measurement and reject a
pre-existing candidate without advancing workflow state; then inspect the
focused race and broad deterministic verification.

## Archival handoff

- Archive candidate: `.concoct/archive/2026-08-17-CON-038-reduce-agent-context-amplification-and-bound-invocation-cost/`.
- Summary: approved CON-038 implementation with `review-03.md` as authority.
- Capability changes: update CAP-013, CAP-014, and CAP-015 with delivered semantic activity evidence, warning and enforceable budget behavior, stopped/rejected-cost accounting, bounded role execution, and documented limitations.
- Roadmap evidence: add the archive directory reference to CON-038; delivery remains pending integration on the recorded task branch.
- Checks: approved review records the full deterministic and race suites, vet, native and Windows builds, shell validation, source/template parity, fresh initialization, and diff checks; no paid live benchmark was run.
- Risks: Codex event semantics remain version-sensitive, classification is conservative, and command-output enforcement is limited to decoded bounded events.
- Pending delivery: `concoct archive --complete` must validate and commit the candidate; then run `concoct integrate`.
- Next action: complete archival validation and recommend `concoct integrate`.
