---
id: CON-037
title: Measure and reduce agent invocation cost
roadmap-id: CON-037
status: implementation-complete
created: 2026-08-13
updated: 2026-08-14
remediates-review: review-02.md
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-037-measure-and-reduce-agent-invocation-cost
  base: 85bbcea49109e81db5297641a69da3bbcd5c8c14
  status: active
capability-impact:
  type: add
  ids:
    - CAP-015
  rationale: Adds observable, bounded agent-usage and prompt-composition evidence, reproducible role baselines, structured progress, and verified cost reduction across one-shot and repeated execution.
---

# Task Plan

## Goal

Make the cost of every supported Codex-backed Concoct action attributable and
inspectable, establish controlled baselines for all six prompt-backed role
modes, and use those measurements to ship at least one material initial cost
reduction without weakening workflow authority or correctness.

The delivered reduction must either decrease exact rendered prompt bytes by at
least 30 percent for one or more dominant fixed-role baselines or eliminate one
provably redundant complete agent invocation. Instrumentation alone is not a
complete outcome.

## Context

The first real `concoct run` completed a lifecycle but explicitly reported at
least 181,120 tokens across only four observed invocations; several other role
and recovery invocations were not included in that subtotal. The originating
evidence and retry causes are preserved in
`.concoct/reports/con-033-first-run-findings.md`.

The accepted execution stack already provides the required authority
boundaries:

- `internal/prompt` renders deterministic manual role prompts.
- `internal/adapter` owns Codex-specific command construction.
- `internal/execution` prepares, supervises, retains, and reconciles one
  invocation.
- `internal/runloop` composes fresh one-shot actions and prints a bounded run
  summary.
- `internal/orchestration`, workflow completion commands, and Git operations
  remain the authorities for structured results and durable transitions.

CON-037 extends those boundaries with measurement and evidence-backed prompt or
invocation reduction. It does not create a competing workflow state or infer
cost from subscription allocation.

## Why this matters

Agent execution cannot serve as Concoct's daily workflow path while one
lifecycle consumes an unpredictable and unsustainable share of the user's
available allocation. Exact local attribution is needed to distinguish fixed
prompt cost from variable model usage, cache behavior, reasoning output, tool
activity, and retries; controlled evidence is then needed to prove that an
optimization preserves the workflow contract.

## Current state

- `prompt.Render` returns one concatenated byte slice. The renderer knows where
  headings, selected persona, handoff, policy-specific additions, provenance,
  and input references originate, but discards those semantic boundaries.
- Role prompts embed large executable-owned personas (currently roughly
  8.7–13.1 KiB each) plus handoff material. Existing prompt goldens cover
  Product Owner, Task Planner, initial and remediation Developer, Reviewer, and
  Archivist states, but assert only final bytes.
- Task inputs such as the roadmap, capability ledger, task plan, notes,
  reviews, and archive summaries are named for agent inspection rather than
  represented as measured prompt components. Selection is conservative and
  can direct an agent to broad ledgers or histories.
- Automated prompts preserve the manual prompt bytes and add one fixed
  supervision appendix for Planner, Developer, Reviewer, and Archivist work.
- The Codex adapter uses `codex exec` with schema-bound final-output capture,
  but it does not request the installed CLI's machine-readable `--json` event
  stream or record the adapter version.
- `internal/execution` combines bounded stdout and stderr into the live
  progress display. Once the shared byte limit is reached, later progress is
  discarded and the invocation can appear frozen.
- Invocation metadata records action, role, profile, start time, and
  disposition, but not duration, predecessor/retry identity, prompt-component
  measurements, adapter version, normalized usage, or partial structured
  events.
- `concoct exec inspect` reproduces retained files including the full prompt by
  default. `concoct exec` and `concoct run` summaries contain no per-action or
  aggregate usage.
- Runtime records are already private, bounded, pruned, and ignored by Git.
  Tests use fake adapters and temporary repositories; no default automated
  check requires a live or billable model invocation.
- The locally installed Codex CLI is `0.146.0`; its `exec` command advertises
  `--json`, `--output-schema`, and `--output-last-message`. The exact native
  event and usage semantics still need to be captured in adapter fixtures and
  documented rather than assumed from human-readable output.

## Target state

- Prompt construction produces both byte-identical final prompt content and an
  ordered semantic component manifest. Every component records category,
  source/provenance, inclusion mode (`full`, `selected`, `summarized`, or
  `absent` where relevant), exact byte contribution, and duplicate grouping.
- Manual rendering and supervised execution consume the same composed result.
  The supervision appendix is an explicit component, and compatibility wrappers
  retain existing prompt-command output unless the measured optimization
  deliberately changes the shared semantic prompt.
- The Codex adapter requests structured JSONL execution events, captures its
  version, keeps the schema-bound final result independent, and translates
  native optional usage and progress into adapter-neutral evidence. Input,
  cached input, output, reasoning-output, and native total fields retain their
  original meanings and presence.
- Partial valid event and usage evidence survives nonzero exit, timeout, and
  cancellation. Duplicate, contradictory, malformed, or out-of-order terminal
  usage is diagnosed and never silently summed. Usage-attribution failure does
  not overwrite an otherwise accepted structured role outcome or observed
  workflow state.
- Retained records separately store bounded raw structured events, stderr
  diagnostics, final structured result, prompt composition, normalized usage,
  and reconciliation. All remain private, Git-ignored, and governed by the
  existing count, age, and total-byte retention policy.
- Live progress is rendered from selected semantic events and continues after
  earlier display history is discarded. Display bounds do not stop event
  parsing or bounded diagnostic retention.
- Default retained-invocation inspection is a compact privacy-preserving view
  of profile, timing, disposition, usage, prompt composition, duplicates, and
  comparison. Full prompt/raw evidence requires an explicit existing-or-new
  diagnostic option. A documented machine-readable metrics-only export permits
  controlled comparisons without exporting repository content.
- `concoct exec` prints a concise usage line for a model invocation.
  `concoct run` records the same line per action and aggregates by role and
  action without double-counting native usage events. Direct mechanical actions
  are explicitly identified as having no agent usage.
- Controlled fixtures cover Product Owner, Task Planner, initial Developer,
  remediation Developer, Reviewer, and Archivist. Deterministic prompt and
  parser baselines run offline; an explicit opt-in live compatibility/baseline
  harness runs only against isolated temporary fixture repositories and names
  adapter version, model, reasoning, fixture, and cache conditions.
- A checked-in report records before/after prompt composition, captured
  adapter-reported usage, largest components, repeated context, avoidable
  invocations, the selected reduction, preservation evidence, and remaining
  dominant drivers.

## Design constraints

- Keep workflow state, action authorization, role completion, review
  independence, archival validation, Git safety, approval gates, and structured
  outcomes in their existing authorities. Metrics are evidence, never
  transition authority.
- Keep Codex event syntax, version detection, compatibility checks, usage
  semantics, and degraded-mode decisions inside the adapter/execution boundary.
  Workflow and run coordination consume only normalized optional measurements.
- Use adapter-reported fields as observed values. Do not manufacture a total,
  subtract cached input, or label a token estimate as reported usage. Preserve
  unknown fields needed to explain supported-version changes without retaining
  unrestricted event content in the normalized export.
- Define prompt components while composing bytes. Do not recover boundaries by
  splitting or heuristically parsing the final prompt. The sum of component
  byte counts must equal the exact prompt length, including separators and the
  supervision appendix.
- Attribute input references and selected/summarized source context honestly.
  A path telling an agent to read a whole file is not equivalent to embedding
  or selecting its contents. Reports must distinguish initial prompt bytes from
  later model/tool context that only adapter usage can observe.
- Define and document exact-duplicate and normalized-duplicate comparison.
  Duplicate reporting may use bounded identifiers and sizes; it must not expose
  secret component content in default inspection or export.
- Preserve manual/automated semantic parity for the same role and state. Any
  compacted persona, handoff, or selected context is shared by prompt-only and
  supervised paths, remains source-attributed, and retains sufficient role
  authority, inputs, constraints, outputs, and completion instructions.
- Measure the fixed baselines before choosing the production reduction. Select
  the reduction from observed dominant cost; do not pre-judge roadmap,
  capability, persona, handoff, archive, or retry hypotheses as proven.
- Prefer correctness-preserving context selection, removal of measured
  duplication, compact machine requirements, or deterministic elimination of a
  redundant full invocation. Do not automatically lower model or reasoning
  profiles in this item.
- A context-selection reduction must use validated record boundaries and stable
  provenance. It must include every role-required roadmap item, capability
  record, review, archive, task artifact, policy fact, or instruction and fail
  closed when safe selection cannot be established.
- Event processing must continue after terminal display truncation. Bound raw
  events and diagnostics independently enough that one stream cannot starve the
  final result, usage summary, or reconciliation record.
- Preserve available partial evidence on failed, cancelled, and timed-out
  attempts. Never accept stale results, duplicate native usage events, or
  malformed terminal events merely to produce a cost summary.
- Keep detailed measurements local and bounded. Do not retain environment
  contents, credentials, authorization headers, unrestricted tool output, or
  new repository content solely for attribution. Durable documentation contains
  aggregate benchmark results and material decisions only.
- Default unit, integration, and golden tests must be deterministic, offline,
  and non-billable. A live Codex compatibility or baseline check must be
  separately and explicitly invoked and must not mutate the active repository
  or consume a production workflow transition.
- Preserve the current supported manual commands and one-shot/run behavior.
  Inspection output may become privacy-preserving by default, but full retained
  diagnostic access must remain explicit and documented for recovery.
- If templates or initialization inputs change, keep source and embedded
  project outputs synchronized and perform the repository's full temporary-init
  verification.

## Non-goals

- No subscription-plan accounting, currency pricing, budget enforcement,
  quotas, billing forecasts, or claim that reported tokens map directly to a
  ChatGPT allocation.
- No automatic model downgrade, reasoning-effort downgrade, role merging,
  shared Developer/Reviewer session, or reduced Reviewer independence.
- No weakening or bypass of Product Owner selection, plan acceptance,
  capability-impact reconciliation, review, archive, Git, permission, or human
  approval boundaries.
- No general session replay, crash-resumable run history, abandoned-run
  recovery, or durable retry coordinator from CON-034.
- No repair of unrelated first-run orchestration defects. This task may measure
  their cost and remove only a redundant invocation proven safe within the
  existing execution authority.
- No arbitrary analytics service, remote telemetry, dashboard, uploaded prompt
  corpus, or permanent per-turn repository history.
- No second built-in adapter. The normalized contract should remain
  adapter-neutral, but only Codex event compatibility is implemented and
  verified here.
- No broad rewrite of the workflow personas, documentation, CLI framework, or
  configuration model beyond evidence-backed cost reduction and the required
  measurement/inspection surface.

## Working assumptions and decisions

- `CAP-015` is the next available capability identifier and will describe the
  enduring observable cost-attribution, baseline, progress, and reduction
  behavior. CAP-006, CAP-010, CAP-013, and CAP-014 remain compatible
  prerequisites and should be cross-related as appropriate during archival.
- Exact rendered prompt bytes are the deterministic primary local metric.
  Adapter usage, cache hits, duration, and reasoning output are variable
  observations that are comparable only under a named execution profile and
  fixture.
- The existing runtime invocation directory remains the storage and retention
  boundary. Measurement files are additional bounded evidence within an
  attempt, not a new workflow artifact or permanent run-history product.
- Prompt composition should become a first-class value returned by the prompt
  package, with the existing byte-oriented rendering surface retained as a
  compatibility path for CLI output and tests.
- Normalized measurement should distinguish an unavailable field from a
  reported zero. A native total, when emitted, is preserved alongside its
  constituent fields rather than recomputed or treated as universally
  equivalent.
- Retry/predecessor linkage is observational. It may use current attempt/action
  correlations and an explicit predecessor when known, but it does not grant
  replay authority or manufacture durable run continuity.
- Default `exec inspect` should no longer print the full prompt merely to show
  cost evidence. An explicit full/raw diagnostic option preserves current
  inspectability; a metrics-only JSON export is the reproducible comparison
  boundary.
- The baseline report should live under `.concoct/reports/` beside the
  originating real-run report and contain no full prompts, raw events,
  credentials, or repository-sensitive excerpts.
- No unresolved product decision blocks implementation. Exact internal type
  names, event-file names, compact progress-event selection, and inspection
  flag spelling may follow existing package and CLI conventions if they meet
  the observable contracts above.

## Risks and open questions

- Codex JSONL event fields may differ by installed version. The decoder needs
  representative versioned fixtures, explicit required/optional semantics,
  and a clear unsupported or degraded result rather than permissive guesses.
- A terminal usage event may be cumulative while intermediate events may be
  repeated or partial. Incorrect deduplication would create misleading run
  totals; adapter tests must establish identity, ordering, and aggregation
  rules before summaries use the data.
- Prompt component accounting can drift if separators are written outside the
  component builder. A byte-conservation invariant and full role/mode fixtures
  are required.
- Existing prompts identify files for later agent inspection. Optimizing only
  initial prompt bytes may not reduce total model input if agents reread broad
  files; the report must compare rendered bytes and native usage separately.
- Compaction or record selection can omit a rare but authoritative constraint.
  Before/after role fixtures and role-specific acceptance scenarios must prove
  required inputs, gates, ownership, and outcomes remain present.
- Structured event retention can consume the existing total-byte budget and
  trigger pruning sooner. Bounds and truncation must preserve terminal usage,
  result, and reconciliation evidence where possible and report when they
  cannot.
- Changing inspection defaults could surprise users who rely on full retained
  output. Documentation and an explicit full/raw option are required.
- Live token baselines remain inherently variable and may consume allocation.
  Collection must be opt-in, profile-labelled, and separable from the required
  offline suite; checked-in results must not be treated as universal pricing.

## Implementation phases

### Phase 1 — Establish measured composition and event contracts

Status: `complete`

- Introduce an ordered prompt-composition result and component taxonomy in the
  prompt layer while preserving exact current output bytes. Cover protocol,
  persona, handoff, supervision, guidance, policy, roadmap, capabilities, task
  plan, notes, reviews, archives, generated command context, and other selected
  inputs with honest inclusion modes.
- Add byte-conservation, provenance, inclusion-mode, exact/normalized duplicate,
  and manual/supervised parity tests across every supported role and materially
  distinct mode.
- Define an adapter-neutral optional measurement model for adapter identity and
  version, execution profile, timestamps/duration, predecessor relationship,
  prompt composition, native usage fields, progress, diagnostic status, and
  no-agent mechanical actions.
- Capture representative Codex JSONL fixtures for supported, partial, malformed,
  duplicate, contradictory, out-of-order, and missing-usage cases without
  invoking a live model in the default test suite. Document the supported CLI
  version evidence and degraded/unsupported boundary.

### Phase 2 — Consume Codex structured events safely

Status: `complete`

- Update the Codex invocation to request JSONL events while retaining the
  independent schema-bound final-result file and stderr diagnostics.
- Implement adapter-owned incremental decoding, version capture, native usage
  normalization, event identity/order validation, and partial-evidence closure
  for success, nonzero exit, timeout, and cancellation.
- Store bounded structured events, diagnostics, prompt composition, normalized
  measurements, and reconciliation as distinct private record material. Extend
  retention-size reservation and pruning tests for partial and complete records.
- Replace raw combined progress forwarding with a compact semantic progress
  renderer whose display history can truncate without stopping later event
  consumption or final usage/result capture.

### Phase 3 — Expose inspection and run-level attribution

Status: `complete`

- Extend execution results and `concoct exec` output with a concise usage and
  prompt-size line, including an explicit no-agent classification for direct
  mechanical actions.
- Make retained invocation inspection metrics-first and privacy-preserving;
  provide explicit full/raw diagnostic access plus a stable machine-readable
  metrics-only export containing no prompt or unrestricted event content.
- Carry normalized measurements into each run step and aggregate totals by role
  and action using validated native usage identities. Test repeated roles,
  partial attempts, missing fields, direct integration, and prevention of
  double-counting.
- Document field meanings, cache treatment, prompt-byte attribution,
  inspection/export, progress, retention, privacy, compatibility, and the
  distinction from subscription usage.

### Phase 4 — Build and record controlled baselines

Status: `complete`

- Create isolated, reproducible fixtures for Product Owner, Task Planner,
  initial Developer, remediation Developer, Reviewer, and Archivist using the
  same prompt, profile, schema, and state boundaries as production execution.
- Add an offline baseline path for exact prompt/component measurements and
  recorded Codex event fixtures. Provide a separately invoked live harness that
  copies a fixture to a temporary repository, fixes adapter version/model/
  reasoning/cache conditions, and never advances a real active task.
- Record pre-optimization component and adapter measurements in the CON-037
  report, identifying dominant fixed bytes, repeated normalized content, broad
  selected inputs, variable usage, and retry/invocation evidence.
- Choose the initial production reduction only after this report identifies a
  qualifying dominant target. Record the selection and correctness proof in
  notes before changing shared prompt or orchestration behavior.

### Phase 5 — Deliver and prove the initial reduction

Status: `complete`

- Implement the selected reduction using validated context selection,
  instruction deduplication/compaction, compact machine requirements, or safe
  elimination of a redundant full invocation. Keep shared manual and automated
  semantic instructions aligned.
- Add positive and negative role fixtures proving that required authority,
  provenance, constraints, inputs, completion evidence, review independence,
  structured outcomes, and gates remain unchanged. Fail closed for context that
  cannot be safely selected or summarized.
- Re-run the identical controlled baselines and publish before/after component
  and native-usage evidence. Name every component removed, selected,
  summarized, relocated, or every invocation eliminated, and explain why
  correctness is preserved.
- Confirm at least one dominant fixed-role baseline reaches the required 30
  percent exact-byte reduction, or prove that one complete redundant agent
  invocation is eliminated, before marking implementation complete.

### Phase 6 — Verify, document, and prepare review

Status: `complete`

- Run formatting, the complete deterministic test suite, focused race tests for
  concurrent process/event handling, static analysis, native and Windows
  builds, prompt golden/component checks, privacy/redaction checks, and CLI
  behavior tests.
- Run the opt-in current-Codex compatibility check only when deliberately
  authorized; record its version/profile/result separately from offline tests
  and do not make it a prerequisite for ordinary contributors.
- If executable-owned templates or initialization behavior changed, run the
  required shell validation and external temporary-project initialization,
  checking copied dotfiles/nested project assets, excluded built-ins, Git
  initialization, bootstrap prompt, status, and source/template parity.
- Complete the baseline/reduction report, user and command documentation, task
  phase statuses, durable notes, and Reviewer handoff with explicit checks,
  skipped live evidence, residual drivers, and capability impact.

## Acceptance criteria

- Every supported real Codex-backed action records role, action, invocation and
  predecessor identity when known, adapter/version, model, reasoning, timing,
  duration, disposition, exact prompt bytes, a byte-conserving semantic
  component breakdown, and every usage field emitted by the supported Codex
  version without reinterpreting cached input or fabricating missing totals.
- Prompt components cover all roadmap-required categories, preserve stable
  provenance and inclusion mode, and identify exact and normalized duplicate
  contribution. Component bytes sum exactly to the supplied prompt bytes.
- Codex JSONL events, stderr diagnostics, and the schema-bound final result are
  processed and retained as separate bounded concerns. Human-readable progress
  is not an authoritative usage source when structured usage exists.
- Long-running structured progress continues after earlier display history is
  discarded, while later usage, result, reconciliation, and bounded diagnostic
  evidence remain processable and inspectable.
- Successful, failed, cancelled, and timed-out model attempts preserve all
  valid usage and partial event evidence emitted before termination. Malformed,
  duplicate, contradictory, out-of-order, missing, or incompatible usage emits
  a clear measurement diagnostic or documented degraded result and never
  corrupts the accepted workflow outcome.
- `concoct exec` reports concise per-invocation prompt and usage evidence.
  Default retained inspection shows measurement, profile, duration, outcome,
  composition, duplicates, and comparison without displaying the full prompt;
  explicit full/raw inspection and metrics-only machine export are documented
  and privacy-tested.
- `concoct run` reports per-action measurement and aggregate totals by role and
  action without double-counting a native event. Mechanical actions invoke no
  model and are explicitly excluded or reported as zero agent usage.
- Controlled Product Owner, Task Planner, initial Developer, remediation
  Developer, Reviewer, and Archivist baselines are reproducible with fixed
  repository state, prompt inputs, adapter version, model, reasoning, result
  contract, fixture identity, and named cache condition. Default automated
  tests make no billable live invocation and mutate no real active project.
- The checked-in CON-037 report distinguishes exact deterministic measurements
  from variable adapter observations and identifies each role's largest prompt
  components, repeated context, native usage, and avoidable invocation causes.
- At least one dominant fixed-role baseline demonstrates a 30 percent or
  greater exact rendered-prompt byte reduction, or one provably redundant full
  agent invocation is eliminated. Before/after evidence names every changed
  component or eliminated action and proves preservation of required inputs,
  authority, gates, role independence, structured outcomes, workflow
  validation, and fixture acceptance behavior.
- Manual and automated role prompts retain byte-identical semantic content for
  the same role/state, apart from the explicitly measured supervision component
  required only by supervised execution.
- Detailed measurements remain private, bounded by existing invocation
  retention, and Git-ignored. Default inspection/export does not expose full
  prompts, credentials, environment contents, unrestricted events, or new
  repository content.
- Documentation defines every measurement field and component category,
  baseline comparison conditions, degraded adapter behavior, retention/export
  privacy, progress behavior, measured reduction, and remaining cost drivers.
- All deterministic tests pass without a live model. The full Go test suite,
  focused race tests, `go vet`, native and Windows builds, shell validation when
  applicable, `git diff --check`, and required template/init checks pass.

## Verification

- `gofmt` on changed Go files and a clean formatting diff.
- `go test -count=1 ./...`.
- Focused `go test -race` for adapter event decoding, execution supervision,
  retention, and run aggregation packages affected by the implementation.
- `go vet ./...`.
- `go build ./cmd/concoct` and a Windows cross-build of the CLI.
- Focused prompt/component golden tests for all six required baseline roles and
  materially distinct Developer/Reviewer modes.
- Focused deterministic adapter-fixture tests for complete, partial,
  duplicated, malformed, contradictory, out-of-order, timed-out, cancelled,
  missing-usage, and unsupported-version event streams.
- CLI tests for metrics-first inspection, explicit full/raw access,
  machine-readable export, per-action lines, run aggregation, no-agent actions,
  redaction, permissions, retention, and partial records.
- Before/after baseline comparison command documented in the CON-037 report;
  any live Codex compatibility collection is explicit, isolated, versioned, and
  recorded separately.
- When templates change: `bash -n cmd/concoct/concoct.sh`, executable-mode
  check, source/template parity checks, and
  `./cmd/concoct/concoct.sh init <external-temporary-project>` with the complete
  AGENTS.md initialization assertions.
- `git diff --check` and a stale-path/branding/privacy search over changed
  source, templates, fixtures, reports, and documentation.

## Handoff expectations

The Developer should leave:

- completed phase statuses and durable implementation decisions;
- the exact Codex versions and fixture profiles verified;
- the normalized event and prompt-component contracts;
- retained-record, inspection, export, and aggregation behavior;
- the pre-optimization baseline and evidence used to choose the reduction;
- before/after results proving the materiality threshold and semantic parity;
- all commands run, including whether an opt-in live check was intentionally
  skipped;
- privacy, compatibility, partial-evidence, and remaining cost risks;
- the delivered CAP-015 impact and any recommended capability cross-links;
- a fresh Reviewer handoff emphasizing measurement correctness,
  double-counting, authority preservation, baseline reproducibility, and the
  claimed reduction.
