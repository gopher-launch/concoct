---
id: CON-038
title: Reduce agent context amplification and bound invocation cost
roadmap-id: CON-038
status: implementation-complete
created: 2026-08-14
updated: 2026-08-14
remediates-review: review-02.md
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-038-reduce-agent-context-amplification-and-bound-inv
  base: 7a7fdaae6bf31b8c237cb07d13a68edf8e499ba6
  status: active
capability-impact:
  type: update
  ids:
    - CAP-013
    - CAP-014
    - CAP-015
  rationale: Extends one-shot and repeated execution with semantic activity evidence, concise continuous progress, enforceable role budgets, reusable-candidate diagnostics, and verified reductions in cumulative agent context processing.
---

# Task Plan

## Goal

Reduce the cumulative input processed by supported Codex-backed Concoct roles,
not merely their initial prompt size, while making the remaining amplification
observable and safely bounded.

Deliver semantic activity attribution, controlled Developer/Reviewer/Archivist
benchmarks, interaction-efficient role guidance and evidence selection,
per-role warning and enforceable hard budgets, concise continuous progress,
and useful accounting and recovery guidance for rejected or budget-stopped
candidates. Preserve workflow correctness, role authority, review independence,
structured-result validation, Git safety, and explicit human gates.

## Context

CON-037 added exact prompt composition and bounded adapter-reported usage, but
the first measured lifecycle actions show that fixed prompt size is not the
dominant cost:

| Role | Prompt bytes | Events | Reported items | Input tokens | Cached input | Output tokens | Result |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Developer | 6,546 | 77 | 34 | 1,742,734 | 1,637,888 | 14,566 | accepted |
| Reviewer | 13,090 | 41 | 17 | 526,532 | 469,760 | 4,463 | accepted |
| Archivist | 16,234 | 24 | 8 | 306,372 | 261,120 | 4,742 | finalization failed |

These three invocations processed 2,575,638 input tokens, approximately 92
percent of them reported as cached input, from only 35,870 rendered prompt
bytes. The smallest prompt produced the largest invocation. The Archivist's
otherwise reusable candidate was rejected after all model cost had been
incurred because it added an unsupported roadmap field.

The retained event streams reveal many command and file-change items, large
command outputs, and repeated broad reads, but the normalized measurement
currently collapses this evidence to generic `turn.started`/`item.started`
labels plus terminal aggregate usage. CON-038 must close that attribution gap
and use the evidence to produce a measured reduction; measurement alone is not
a complete outcome.

## Why this matters

Concoct cannot be a routine workflow coordinator if a small role prompt expands
into hundreds of thousands or millions of repeatedly processed tokens, or if a
mechanically repairable finalization error forces another expensive invocation.
Users need evidence that distinguishes productive work from repeated discovery,
oversized command output, retries, and rejected candidates, plus containment
that stops enforceable runaway activity without inventing unavailable usage or
advancing workflow state.

## Current state

- `internal/adapter` incrementally decodes Codex JSONL but records only one
  optional usage object, a total event count, generic lifecycle progress, and
  bounded diagnostics. A second usage object is diagnosed as duplicate or
  contradictory rather than represented as a usage timeline.
- Retained Codex 0.146.0 streams contain `thread.started`, `turn.started`,
  `turn.completed`, and `item.started`/`item.completed` events. Observed item
  types are `agent_message`, `command_execution`, and `file_change`.
  `command_execution` includes the command, aggregate output, status, and exit
  code, but observed events contain no native timestamp or duration.
- Reads, searches, edits, and tests commonly arrive as command executions.
  Adapter-owned conservative classification is therefore required; generic
  item counts cannot safely be treated as model turns.
- `internal/execution` streams every event through the decoder after bounded
  raw retention fills, retains prompt/measurement/reconciliation records, and
  independently validates and finalizes candidates. It has timeout,
  cancellation, process-group termination, and storage limits, but no
  activity-, token-, or command-output budget contract.
- Raw structured stdout is bounded and local but retained by default even
  though command events can contain repository content. Metrics-first output
  removes progress and diagnostics, but it has no normalized activity or
  repeated-operation evidence.
- Progress displays only allowlisted top-level event names and permanently
  stops adding new entries after its history bound. It cannot tell an operator
  whether work is reading, searching, editing, testing, retrying, or finishing.
- `internal/config` resolves strict adapter, role profile, timeout, retention,
  and run bounds through built-in, user, project, and invocation layers. It has
  no per-role execution budget schema or budget provenance.
- `internal/runloop` aggregates terminal native usage by role and action but
  does not separate accepted cost from rejected/finalization-failed cost or
  expose normalized activity and budget stops.
- Prompt composition is byte-accounted, but most role prompts still direct the
  agent to rediscover broad files. The embedded Reviewer and Archivist personas
  are approximately 9.1 KiB and 11.5 KiB, compared with the compact 3.2 KiB
  Developer persona, and personas, handoffs, generated input lists, and the
  Concoct skill repeat several responsibilities.
- The existing offline role fixtures measure prompt bytes only. The opt-in live
  harness exercises a Product Owner action, not fixed development, review, and
  archival before/after comparisons.
- The retained production records and `.concoct/reports/con-033-first-run-findings.md`
  preserve the originating amplification and failed-finalization evidence.
  `.concoct/reports/con-037-baseline-and-reduction.md` is the existing offline
  prompt baseline; CAP-015 currently cites a stale alternate filename that the
  Archivist can reconcile when updating that capability.

## Target state

- Adapter-owned normalization reports stable activity categories for model
  messages/reasoning when exposed, commands, file reads, searches, edits,
  tests/checks, compaction, terminal results, and an explicit unknown/other
  category. It retains native type/version provenance and never calls a generic
  item a model exchange without evidence.
- Bounded, payload-safe fingerprints and counters identify repeated reads,
  searches, commands, and checks. Command evidence includes aggregate output
  bytes, truncation, exit status, and authoritative or clearly labelled
  host-observed duration where available, without copying command text or
  output into default metrics.
- Usage evidence distinguishes terminal-only aggregate usage from observable
  cumulative or incremental exchange snapshots. Missing exchange, compaction,
  duration, or tool detail is reported as unavailable, not inferred.
- Detailed raw event retention is explicitly configurable, always private and
  bounded, and defaults to disabled or payload-safe summary retention when the
  stream can contain repository content. Result, usage, budget, and
  reconciliation evidence survive raw-retention limits.
- The terminal shows concise semantic current progress even after old display
  history is discarded. A deliberate verbose/diagnostic mode exposes deeper
  bounded local detail without making rendering failures authoritative.
- Strict per-role warning budgets cover elapsed time, normalized activity,
  processed input, uncached input, output, and command output whenever those
  values are observable. Hard budgets are offered only for dimensions that can
  be enforced during the invocation; budget exhaustion is distinct from
  timeout, cancellation, adapter failure, refusal, and finalization failure.
- A hard stop terminates safely, preserves partial measurement and candidate
  evidence, rejects stale or incomplete results, leaves canonical workflow
  state unadvanced, and reports whether fresh retry, manual repair/completion,
  or human investigation is safe.
- Finalization and run summaries separately report accepted and wasted usage.
  A rejected candidate names the exact invariant, candidate-artifact
  reusability, and the supported no-new-agent repair path when deterministic
  evidence makes one safe.
- Every built-in role persona and handoff uses one deliberate initial discovery
  pass, batched bounded reads, focused iterative checks, one broad completion
  validation, reasoned rereads/reruns, and an early stop at the role completion
  contract. Deterministically selected task evidence replaces broad ledger or
  history rediscovery where safe.
- Reviewer and Archivist role context is compacted without removing authority,
  prohibited operations, required evidence, structured completion, or
  supervision. Archivist persona bytes fall by at least 50 percent unless a
  controlled benchmark proves a different correctness-preserving change has a
  larger impact on Archivist processed input.
- Reproducible isolated Developer, Reviewer, and Archivist fixtures retain
  named profiles, semantic activity, native usage, disposition, and failure
  evidence without mutating the active task. Live observations are labelled
  directional unless workloads are equivalent; the operator cost stop does
  not require another paid causal comparison or successful finalization run.

## Design constraints

- Keep workflow detection, authorization, structured outcomes, role completion,
  review independence, archival validation, Git mutation, and gates in their
  existing authorities. Measurements and budgets observe or stop execution;
  they never establish completion.
- Keep Codex syntax, native event mapping, usage snapshot semantics,
  compatibility, and degraded-mode decisions in `internal/adapter` and the
  execution boundary. Workflow packages consume only normalized evidence.
- Treat total processed input as the primary amplification measure. Report
  cached input separately and derive uncached input only when supported native
  field semantics make subtraction valid; label that value as derived and
  otherwise report it unavailable. Do not assume cached input is free,
  reconstruct missing totals, or equate token usage with subscription
  accounting.
- Classify commands conservatively. Ambiguous shell pipelines remain
  `command-other` or expose multiple evidenced categories; never claim a file
  read, test, model exchange, or compaction from an unsupported heuristic.
- Fingerprints used for repetition analysis must be bounded and
  privacy-preserving. Default inspection and checked-in reports contain no raw
  command, path, output, prompt, repository content, secret, or credential.
- Separate authoritative adapter fields, host-observed timing/byte counts, and
  heuristic classifications in the data model and documentation.
- Warning evaluation may occur only when a value becomes observable. A
  terminal-only token total can be reported against a warning threshold but
  cannot be presented as a live enforceable hard bound.
- Hard-budget termination must use the existing bounded process-group shutdown,
  close result acceptance, and preserve partial evidence. No budget path may
  accept a stale terminal result that arrives after the stop decision.
- Derive built-in budget defaults from the retained production evidence and
  controlled fixtures. Publish each value, unit, role, enforcement status, and
  rationale; do not hide arbitrary constants.
- Context selection must use validated structural record boundaries with
  explicit provenance and fail closed to the complete authoritative input when
  safe selection is uncertain.
- Manual and supervised prompts for a role/state retain one shared semantic
  core. Compaction may change golden bytes deliberately, but byte conservation,
  source attribution, completion schemas, and supervision behavior remain
  covered.
- Bound command output at its source through agent guidance and focused command
  conventions where practical. Post-event measurement cannot undo output that
  the model already processed.
- Preserve valid candidate files on finalization failure. Automated repair is
  allowed only for deterministic executable-owned mutations that require no
  product, implementation, review, or archival judgment.
- Default automated tests remain deterministic, isolated, and non-billable.
  Live Codex compatibility and processed-token benchmarks require an explicit
  operator action and cannot mutate the real active task.
- Keep `.codex/skills/concoct/SKILL.md` and its template copy byte-identical.
  If initialization inputs change, preserve executable permissions and run the
  full external temporary-project checks required by `AGENTS.md`.

## Non-goals

- No subscription allocation prediction, currency pricing, remote telemetry,
  uploaded analytics, or guarantee that measured tokens map to billing.
- No automatic model or reasoning downgrade, role merging, shared
  Developer/Reviewer context, or weakening of independent review.
- No general conversation replay, crash-resumable run, abandoned-run recovery,
  durable multi-invocation history, or dirty-worktree lease from CON-034.
- No arbitrary workflow graph, new approval bypass, or transfer of semantic
  role judgment to deterministic code.
- No second built-in adapter. The normalized contract remains adapter-neutral,
  but only supported Codex streams require concrete mapping and live
  compatibility evidence.
- No promise to enforce a hard token budget when the adapter exposes usage only
  after completion.
- No unbounded raw event, command, output, or per-exchange history in repository
  artifacts or default inspection.
- No unrelated CLI framework, state-machine, roadmap, capability-ledger, or
  documentation rewrite.

## Working assumptions and decisions

- CON-038 is one coherent execution-cost outcome. Semantic attribution is the
  evidence layer used to select and validate interaction reductions and to
  evaluate budgets; it is not an independently delivered analytics project.
- CAP-005's read-only/status and compatibility limitations do not block this
  task. Existing strict configuration and compatibility preflight are reused.
- CAP-013's lack of looping or general retry remains intact. Reusable-candidate
  reporting and deterministic manual completion do not create replay authority.
- CAP-014's lack of crash-resumable coordination and permanent run history
  remains intact. Budget stops end the current bounded run and report a fresh
  safe continuation from observed evidence.
- CAP-015's optional usage and Codex-version limitations are inputs to this
  task, not blockers. New reports must preserve unavailable/degraded states and
  explicitly validate the supported installed event stream.
- The current Codex stream is known to expose command and file-change item
  payloads and terminal aggregate usage. Per-exchange usage, model-turn
  identity, compaction, and native item duration remain unconfirmed until the
  explicit compatibility capture proves them.
- Existing timeout remains a separate adapter-timeout contract. An elapsed
  hard budget, if delivered, needs its own lower deadline and
  `budget-exhausted` disposition so reports remain truthful.
- The minimum recovery outcome is precise invariant diagnostics, preservation
  of safe candidate artifacts, reusability classification, and an existing or
  narrowly added deterministic completion path that avoids another agent
  invocation. General automatic candidate continuation is out of scope.
- Capability impact updates CAP-013, CAP-014, and CAP-015 rather than adding a
  separate capability: the user-visible outcome extends the existing executor,
  lifecycle coordinator, and measurement surface.

## Risks and open questions

- Codex event shapes are version-sensitive. A supported-version mapping and
  representative fixtures must prevent silent reclassification when fields or
  semantics change.
- Shell commands can combine reads, searches, edits, and checks. Classification
  must prefer explicit unknown or multi-category evidence over false precision.
- Intermediate usage may be cumulative, incremental, duplicated, or absent.
  Incorrect normalization would corrupt budget warnings and benchmark totals.
- Token and cache measurements are externally variable. A single benchmark
  must name fixture, code revision, adapter version, model, reasoning, sandbox,
  schema, and cache condition, and deterministic activity evidence must support
  the token result.
- Processed-input reduction requires more than persona compaction. Semantic
  evidence should prioritize repeated reads, large command outputs, redundant
  checks, and selected context rather than assume the largest fixed component
  is the dominant total-cost driver. Unlike workloads must not be presented as
  a causal percentage comparison.
- More detailed decoding could retain sensitive payloads or exceed memory and
  record limits. Store counts, sizes, classifications, and fingerprints; keep
  raw payload retention explicit and independently bounded.
- Warning thresholds may first become observable at terminal completion. The
  UI and reports must say whether a warning was live or terminal-only.
- Hard termination can race with a late result file. The stop decision must
  outrank that result and reconciliation tests must cover the race.
- Candidate reusability is action-specific. A valid implementation diff may be
  reusable while a stale structured outcome is not; one boolean is unlikely to
  be sufficient for safe recovery guidance.
- The required live compatibility and benchmark runs consume model allocation.
  They need explicit operator approval and must be completed before claiming
  the processed-input acceptance threshold.

## Implementation phases

### Phase 1 — Freeze evidence contracts and controlled baselines

Status: `complete`

- Preserve the three retained CON-037 production records as aggregate
  originating evidence without copying private payloads into tracked files.
- Define stable normalized activity, repetition, command-output, usage-snapshot,
  compaction, provenance, availability, finalization, and wasted-cost schemas.
- Capture representative version-labelled Codex fixtures for the observed item
  types plus missing, malformed, oversized, repeated, out-of-order, partial,
  and late-terminal cases.
- Extend isolated Developer, Reviewer, and Archivist benchmark fixtures so the
  same repository/task evidence, result schema, adapter profile, safety mode,
  and expected outcome can be evaluated before and after optimization. Record
  the pre-change deterministic and approved live baseline conditions before
  changing role context.

### Phase 2 — Normalize semantic activity and retain bounded evidence

Status: `complete`

- Extend the adapter decoder and execution capture to classify semantic
  activity, measure output and timing, group repeated operations using
  payload-safe fingerprints, and retain usage snapshots and compaction evidence
  only when the stream supports them.
- Redesign detailed raw-event retention as an explicit private bounded mode;
  keep the default summary payload-safe while preserving terminal usage,
  structured result, reconciliation, and degraded/truncated diagnostics.
- Extend metrics-first inspection and run aggregation with normalized activity,
  availability/provenance, accepted versus wasted usage, finalization
  acceptance, and reusable-candidate evidence.
- Replace append-until-full lifecycle labels with a bounded semantic progress
  view that continues to show the latest activity; add an explicit bounded
  verbose/diagnostic mode.

### Phase 3 — Add warning and enforceable hard budgets

Status: `complete`

- Add strict per-role budget configuration and provenance using the established
  role configuration and override conventions. Document derived defaults and
  reject unknown units, negative values, contradictory bounds, and hard
  dimensions that cannot be enforced by the selected adapter/version.
- Evaluate warnings as elapsed time, activity, token, and command-output
  measurements become observable; keep warnings non-authoritative and record
  whether they were live or terminal-only.
- Enforce supported hard elapsed/activity/output bounds with the existing
  process-group shutdown, a distinct budget-stop disposition, partial evidence
  retention, stale-result rejection, and unchanged workflow state.
- Make finalization failures name the violated invariant, accepted and wasted
  cost, reusable artifacts, and safe next action. Support no-new-agent manual or
  deterministic repair/completion when evidence proves it safe, without adding
  general replay or resumable-run behavior.
- Review remediation: live-observable warnings are now deduplicated and
  preserved on completed, nonzero-exit, timeout/cancellation, and hard-budget
  paths; terminal token warnings remain explicitly terminal-only. Command
  classification requires affirmative check/test arguments rather than a
  general-purpose executable name.
- Review 02 remediation: deterministic execution coverage now crosses the
  activity and command-output hard limits with a structured candidate already
  present. Both paths preserve partial measurement, retain the candidate only
  as rejected late evidence, and leave workflow state unchanged.

### Phase 4 — Reduce role context and model/tool round trips

Status: `complete`

- Review every built-in persona, handoff, generated prompt section, and Concoct
  skill instruction for duplication and machine-enforced narrative. Record
  every removed, selected, summarized, or relocated component and its preserved
  correctness boundary.
- Add concise cross-role execution guidance for one batched initial discovery
  pass, no reasonless rereads or successful-check reruns, bounded source output,
  focused iteration, one required broad completion check, and immediate handoff
  once the role contract is satisfied.
- Use validated task-specific roadmap, capability, archive, and review
  selection where it reduces broad rediscovery. Preserve explicit provenance
  and fall back safely when selection is incomplete.
- Prioritize Reviewer and Archivist compaction, update prompt goldens and
  manual/supervised parity tests, and verify that completion requirements and
  role prohibitions remain explicit.

### Phase 5 — Evaluate the reduction, document limits, and prepare review

Status: `complete`

The operator-approved 2026-08-14 run used Codex CLI 0.147.0,
`gpt-5.6-sol`, and `medium` reasoning. Developer returned a blocked outcome,
Reviewer produced an invalid outcome declaration, and Archivist omitted the
required exact archive-summary reference. The success-only harness logging also
discarded normalized measurement from the failed candidates. The evidence-loss
defect is now corrected: the suite requires an external absolute evidence
directory and exports create-only metrics before lifecycle evaluation, with a
non-billable rejected-outcome regression. The Developer fixture now has a
concrete exact-content task, Reviewer and Archivist prompts expose their
machine-required output syntax, and an offline fake-adapter test proves all
three starting states can cross their deterministic completion boundaries.

The second approved run retained all metrics and observed 345,092 aggregate
input tokens versus 2,575,638 across the originating production actions.
Because the workloads were not equivalent, the 86.6 percent difference and
the exact-profile Developer's 94.3 percent difference are directional, not a
causal CON-038 reduction claim. Current Codex 0.147.0 mapping was supported.
All three structured completed candidates nevertheless failed finalization.
Follow-up Developer status, Reviewer reservation-fixture, and Archivist
notes-order corrections pass offline.

On 2026-08-14 the operator accepted an explicit plan amendment after CON-037
and CON-038 consumed approximately 45 percent of weekly model allocation. No
further billable benchmark is authorized or required for this task. The
arbitrary 50 percent equivalent-workload threshold and successful-live-
finalization condition are replaced by honest preservation of the directional
observations, verified live compatibility/measurement retention, deterministic
prompt/persona evidence, offline safety/completion coverage, and explicit
limitations. Known lifecycle/finalization defects remain separate follow-up
work rather than a reason to spend more allocation within CON-038.

- Preserve the two explicitly approved live attempts and their limitations;
  make no further billable model call for CON-038.
- Publish prompt bytes, normalized activity, observable model exchanges,
  command-output bytes, duration, native token fields, disposition,
  finalization acceptance, wasted cost, changed prompt components, and remaining
  dominant drivers. Clearly separate deterministic from externally variable
  values.
- Report observed processed input and activity without treating unlike
  workloads as a controlled causal comparison. Meet the deterministic
  Archivist persona target and state unresolved lifecycle/finalization cost.
- Complete offline tests, race/static/build checks, documentation, source and
  template parity, external initialization verification, diff inspection, and
  the Developer handoff.

## Acceptance criteria

- Default retained inspection reports prompt composition, normalized activity
  counts, repeated-operation groups, bounded command-output measurements,
  duration, available usage snapshots/aggregate, disposition, finalization
  acceptance, budget events, artifact reusability, and accepted versus wasted
  cost without exposing payload content.
- Every normalized field identifies its adapter/version mapping and whether it
  is native, host-observed, conservatively classified, or unavailable. Generic
  Codex items are never reported as model turns without contract evidence.
- Missing per-exchange usage, compaction, duration, command detail, or another
  unsupported measurement is explicitly `unavailable`; no absent value becomes
  zero or an estimate.
- Repeated file reads, searches, commands, and checks can be identified by
  bounded privacy-safe group evidence, and raw event retention is optional,
  private, Git-ignored, independently bounded, and payload-safe by default.
- Developer, independent Reviewer, and Archivist benchmark fixtures are
  reproducible under named repository, task, adapter, model, reasoning,
  sandbox, completion-schema, and cache conditions without mutating the real
  active task. Paid repetition is not required after the operator cost stop.
- Live results are reported as directional observations unless repository,
  task, profile, and completion conditions are equivalent. CON-038 makes no
  causal percentage-reduction claim from the unlike originating workloads.
- The Archivist persona is at least 50 percent smaller than its recorded
  baseline, or retained benchmark evidence proves and documents a different
  correctness-preserving change with greater impact on Archivist processed
  input.
- Offline candidates satisfy the same structured outcome schema, role-owned
  mutation checks, workflow finalization, required verification, review
  independence, safety controls, and human gates. Live finalization failures
  are preserved as known limitations and routed to existing lifecycle work.
- All built-in roles receive concise evidence-discovery, batching, bounded
  output, focused-check, reasoned-reread/rerun, and completion-stop guidance;
  selected evidence preserves every required authority with explicit
  provenance and safe fallback.
- Per-role warning budgets are strictly configurable and visibly report their
  value, source, observed measurement, and live versus terminal-only status.
  Enforceable hard budgets stop safely with a distinct disposition, preserved
  partial evidence, no accepted stale result, and no workflow advance.
- A finalization-rejected candidate reports consumed usage as wasted cost, the
  exact failed invariant and location when deterministically known, artifact
  reusability by evidence class, and whether manual/deterministic repair can
  finish without another model invocation.
- Semantic progress remains visibly current after old display history reaches
  its bound. Default display, verbose display, raw retention, diagnostics, and
  normalized measurement have independent limits, and their failures cannot
  discard an otherwise valid result.
- Offline automated tests cover supported-version event mapping, unknown and
  unavailable fields, repeated activity, output bounds, usage-snapshot rules,
  privacy, live/terminal warnings, safe hard stops, late-result races, partial
  usage retention, progress rollover, finalization accounting, and benchmark
  isolation without a billable model call.
- An explicitly invoked compatibility test validates the currently supported
  Codex structured-event stream, records the adapter version and observed
  availability matrix, and remains outside the default test suite.
- Checked-in documentation explains normalized categories and version mapping,
  measurement authority, raw-event privacy/retention, budget configuration and
  recovery, benchmark reproduction, prompt-component changes, and why prompt
  bytes, cached/uncached/total input, output, and subscription usage are not
  interchangeable.

## Verification

- `gofmt` on changed Go files.
- Focused adapter, configuration, prompt, execution, CLI, run-loop, workflow,
  and benchmark-fixture tests while iterating.
- `go test -count=1 ./...`.
- `go test -race -count=1` for the adapter, prompt, execution, and run-loop
  packages, including concurrent decoder/progress/budget-stop paths.
- `go vet ./...`.
- Native `go build ./cmd/concoct` and `GOOS=windows GOARCH=amd64 go build
  ./cmd/concoct` with isolated writable caches.
- Deterministic all-role prompt/benchmark comparison and byte-conservation,
  source/template parity, authority-preservation, privacy, and no-real-task
  mutation checks.
- The two explicitly operator-approved live attempts against disposable
  repositories, recording adapter version, profiles, cache conditions,
  measurements, failures, and workload-comparability limits. No further paid
  attempt is required under the operator cost stop.
- `bash -n cmd/concoct/concoct.sh` and executable-mode validation.
- `./cmd/concoct/concoct.sh init <external-temporary-project>` from a temporary
  parent, verifying dotfiles, nested project-owned templates and planning
  directories, exclusion of built-in protocol/persona/prompt files, Git
  initialization, bootstrap prompt, staged files, and no generated commit.
- `git diff --check` and focused searches for stale report names, duplicated
  instructions, leaked payload examples, stale configuration/docs, branding,
  and paths.

## Capability impact

Expected impact is `update`:

- CAP-013 gains per-action semantic progress and measurement, warnings and
  enforceable stops, accepted/wasted cost, and reusable-candidate diagnostics.
- CAP-014 gains the same per-step evidence and bounded-run stop/reporting
  semantics without gaining crash recovery, replay, or permanent run history.
- CAP-015 expands from fixed prompt/aggregate usage attribution to semantic
  activity, repeated-operation and output evidence, controlled processed-input
  benchmarks, role budgets, and verified amplification reduction.

The Archivist should reconcile CAP-015's stale baseline-report filename while
updating the accepted capability record. No capability truth changes before
independent approval and archival.

## Handoff expectations

The Developer should leave:

- honest phase status and durable decisions, including the normalized schema,
  supported Codex version mapping, budget defaults/provenance, and raw-retention
  policy;
- before/after benchmark controls and results for Developer, Reviewer, and
  Archivist, including the required processed-input and Archivist threshold
  evidence;
- a component-by-component record of compacted, selected, summarized, removed,
  or relocated instructions and how each authority remains protected;
- verification of warning/hard-stop safety, late-result rejection, candidate
  reuse diagnostics, accepted/wasted aggregation, semantic progress rollover,
  privacy, and default offline isolation;
- all checks run and any operator-approved live command that was skipped or
  failed, with residual risk stated honestly;
- an accurate CAP-013/CAP-014/CAP-015 impact assessment without editing the
  capability ledger;
- a fresh Reviewer handoff focused on event-semantic correctness, budget races,
  privacy/bounds, workflow authority, benchmark comparability, finalization
  recovery, and the claimed cost reductions.
