# Notes

## Planning summary

CON-010 is ready for implementation on the recorded Git task branch. It adds
one-shot execution of the current workflow recommendation through a configured
agent adapter while preserving the manual workflow, role ownership, structured
action/outcome authority, policy gates, and Git safety boundaries.

## Confirmed findings

- Git identity is trunk `main`, task branch
  `concoct/con-010-execute-one-recommended-action-through-an-agent`, and base
  `c30e7718462b3dba1affe6aa6ce924786b5312db`. Planning began with a clean
  checkout exactly at that base.
- Before planning, `.concoct/current/` contained only `.gitkeep`; no active
  task or review conflicted with CON-010.
- CON-010 is `planned`, has no roadmap dependency, and all eight declared
  prerequisites (CAP-005 through CAP-012 as selected by the roadmap) resolve to
  active accepted capability records.
- `workflow.Detect` owns state and its current `Next` recommendation;
  `InspectNextActionEvidence` owns deterministic ready-state evidence while
  leaving semantic choice to the Product Owner.
- `prompt.Render` already provides deterministic version-matched role guidance
  and exact authorized read/write context. It is the prompt source for adapter
  execution, not a template to duplicate.
- `internal/orchestration` already supplies the `v1` action/outcome model,
  registry, correlation, bounded fields, observable-state completion
  validators, and atomic no-replace result publication. It does not yet launch
  or supervise an adapter.
- The current orchestration snapshot covers roadmap, capabilities, task plan,
  notes, state, task, and latest-review identity. Execution will need to account
  for all policy, review, configuration, Git, and archive evidence material to
  authorization and reconciliation.
- The current generic completion check rejects a completed result with no
  observed artifact change. That is correct for state-changing actions but
  does not represent a successful ready-state Product Owner recommendation;
  CON-010 needs an action-specific bounded decision-result rule rather than a
  global weakening.
- CLI configuration currently exists only as a narrow optional
  `.concoct/config.yaml` Git auto-push reader. Execution needs a strict shared
  project/user configuration model with source attribution and compatibility
  with that accepted setting.
- Generated projects currently have no installed `.gitignore`. The required
  local `.concoct/runtime/invocations/` path therefore needs an explicit
  ignored-state installation strategy as well as this repository's ignore
  rule.
- The current Codex CLI/manual supports non-interactive `codex exec`, prompt
  input on standard input, explicit `--cd`, `--sandbox`, `--model`,
  `--output-schema`, `--output-last-message`, `--ephemeral`, progress output,
  and configuration overrides such as `model_reasoning_effort`. The Codex
  adapter can use these supported surfaces without treating stdout or exit
  status as completion evidence.

## Capability limitation compatibility

- CAP-005 is compatible: `exec` composes existing read/mutation compatibility
  checks and state detection rather than changing `status` from read-only or
  auto-migrating unsupported projects.
- CAP-006 is compatible: direct execution consumes the same executable-owned
  prompt renderer, while manual prompt rendering and create-only export remain
  supported.
- CAP-007 is compatible: task branch identity, clean transition boundaries,
  integration recovery, and push policy remain authoritative. `exec` adds no
  provider integration, worktree concurrency, or automatic permission bypass.
- CAP-008 is compatible: Task Planner judgment and artifact validation remain
  role-owned. `exec` can carry out the rendered planning action but cannot
  infer semantic compatibility from capability structure.
- CAP-009 is compatible: ready-state ordering remains presentation, not
  autonomous selection. The Product Owner action may return one bounded
  recommendation, and the one-shot executor stops before acting on it.
- CAP-010 is compatible: Developer and Reviewer completion still passes through
  explicit durable evidence and existing completion validators; the adapter's
  result cannot manufacture code quality or review judgment.
- CAP-011 is compatible: Archivist semantic authorship, archive validation,
  pending delivery, and integration ownership remain unchanged.
- CAP-012 is compatible and foundational: execution transports and validates
  its action/outcome contract. Action-specific recommendation and expanded
  evidence coverage may refine the `v1` schema without making process output or
  runtime records authoritative.

## Planning decisions

- Add proposed CAP-013 for the observable one-shot agent-execution capability.
- Use one typed recommendation resolver shared by status-facing output,
  authorization, dry-run, and execution; avoid interpreting formatted command
  strings as the action contract.
- Execute ready-state `product-owner-next` as one agent action and stop with its
  bounded recommendation. Do not chain directly into `plan`.
- Keep adapter discovery executable-owned and finite. Ship only `codex` now;
  future external discovery remains deferred.
- Resolve settings in the roadmap-specified order: invocation overrides,
  project role configuration, user role configuration, adapter role defaults,
  adapter general default. Read user settings from the platform user-config
  directory and reuse `.concoct/config.yaml` for project settings.
- Build the Codex invocation from an inspectable allowlist. Supply the exact
  prompt on stdin, use the repository root explicitly, request a schema-bound
  final result, and map reasoning through Codex-owned configuration rather than
  the workflow contract.
- Dry-run and real launch consume the same resolved invocation object. Dry-run
  performs no runtime write so inspection continues to mean an actual attempt.
- Store real attempts under private ignored runtime state, preserve retained
  bytes for inspection, and prune only closed records after reconciliation.
- Reconcile observed state after every exit path. A missing or failed adapter
  claim does not erase real durable changes, and a successful claim does not
  establish changes that are absent.

## Risks

- Evidence can change between authorization, process launch, result delivery,
  and reconciliation.
- Agent-authored artifacts may be valid even when the process or result
  transport fails; the final report must distinguish invocation disposition
  from observed workflow state.
- Timeout and cancellation may race with atomic result publication or leave a
  subprocess tree alive.
- Exact prompts and process diagnostics require careful private storage,
  bounds, and redaction to avoid retaining secrets or unrestricted output.
- A new configuration parser must preserve the accepted `git.auto-push`
  behavior and reject unsafe partial/unknown execution settings consistently.
- Integration and push can cross a stronger authority boundary than ordinary
  role work. Existing direct command checks and intervention behavior must not
  be replaced by agent intent.

## Relevant history

- CON-005, CON-006, CON-007, CON-008, CON-009, CON-015, CON-018, and CON-028
  established the CLI, prompt, planning, completion, archive, Git, policy, and
  next-action boundaries this task composes.
- CON-030 made built-in workflow content executable-owned; the adapter must use
  that version-matched source.
- CON-031 established executable/project compatibility checks that must run
  before execution or runtime mutation.
- CON-032 deliberately stopped at the structured action/outcome validation
  boundary and names CON-010 as the adapter-execution consumer.
- Current Codex non-interactive behavior was verified against the official
  Codex manual and the installed `codex-cli 0.146.0` help on 2026-08-11.

## Handoff to developer

### Current state

Planning is complete; CON-010 is ready for implementation on the recorded task
branch.

### Completed

- Validated roadmap scope, dependencies, prerequisite limitations, archive
  provenance, repository implementation, documentation, Codex adapter surface,
  current workflow state, and exact Git identity.
- Defined the one-shot recommendation boundary, adapter/configuration split,
  ready-state decision behavior, runtime/retention contract, failure
  reconciliation, acceptance criteria, and verification matrix.

### Remaining

- Implement the six phases in `task-plan.md`, keep phase status and durable
  findings current, and add the required fresh Reviewer handoff before
  completion.

### Known risks

- Preserve observed-state precedence, ready-state Product Owner authority,
  prompt-byte parity, process termination safety, secret exclusion, and the
  manual workflow.

### Commands and evidence inspected

- `concoct status`, Git branch/base/cleanliness checks, capabilities, roadmap,
  all prompt-selected archive summaries, relevant prior task artifacts, CLI,
  workflow, prompt, orchestration, configuration, integration, initialization,
  tests, and active documentation.
- Official Codex manual non-interactive-mode guidance and local
  `codex exec --help` / `codex --version` output.

### Suggested next step

Run `concoct code`, then begin Phase 1 by extracting the typed
recommendation/action resolver and tightening execution evidence coverage
before introducing the process adapter.

## Implementation findings and decisions

- `workflow.ResolveAction` is now the typed authority shared by status-facing
  `Next` guidance and orchestration. It selects Product Owner decision,
  Developer, Reviewer, Archivist, or direct integration work and refuses
  blocked, invalid, informational, and recovery-choice states without parsing a
  formatted command.
- Protocol `v1` now carries a bounded Product Owner `Recommendation`. A
  successful ready-state decision must leave all repository evidence unchanged
  and return exactly one eligible `plan`, `roadmap`, `blocker`, or `no-action`
  outcome. State-changing actions retain their observed-postcondition rule.
- Material action evidence now covers instruction, policy, project/config,
  roadmap, capability, all current artifacts, archive summaries, exact Git
  branch/HEAD/status/diffs, and hashed untracked content. Resolved user/project
  execution configuration has a separate immutable digest checked before
  launch and reconciliation.
- `internal/config` strictly composes invocation overrides, project role
  settings, user role settings, Codex role defaults, and Codex defaults with
  source attribution. It preserves `git.auto-push` and defines retention
  defaults of 20 completed attempts, 14 days, 256 KiB per log, and 20 MiB total.
- `internal/adapter` is an executable-owned finite registry with one `codex`
  adapter. Codex receives the exact manual prompt on stdin, correlation through
  a strict output schema, an explicit project root, `workspace-write`, ignored
  user configuration, no bypass flags, an allowlisted environment, and optional
  validated model/reasoning settings.
- `internal/execution` creates mode-private ignored records, supervises one
  process group, applies graceful/forced bounded termination on Unix and Windows
  process trees, bounds and redacts progress logs, normalizes one no-replace
  result, reconciles observed state after every exit path, inspects retained
  bytes without regeneration, and prunes only closed attempts deterministically.
- Archived-state integration remains a direct executable action so the existing
  Git recovery, conflict, push-confirmation, and policy gates stay authoritative.
  The executor never fabricates an integration prompt or automatically invokes
  another recommendation.
- Initialized projects now install `.gitignore` with the runtime invocation
  boundary but do not create runtime state until a real attempt. Manual prompts,
  completion commands, and integration remain available unchanged.

## Verification results

- `go test -count=1 ./...` — passed.
- `go test -count=1 -race ./internal/orchestration ./internal/execution` — passed.
- Focused execution race and failure tests — passed for successful ready-state
  decisions, prompt parity, missing/malformed/nonzero results, cancellation,
  timeout, process-tree termination, configuration drift, private modes,
  redaction, partial inspection, and retention.
- `go vet ./...` and `go build ./cmd/concoct` — passed.
- `GOOS=windows GOARCH=amd64 go build ./cmd/concoct` — passed, including
  Windows process-tree termination support.
- `bash -n cmd/concoct/concoct.sh` and executable-mode validation — passed.
- Fresh initialization outside the repository — passed: project-owned
  dotfiles, nested adapters, current/archive directories, Git staging, bootstrap
  guidance, and runtime ignore behavior were present; built-in protocol,
  persona, and prompt files and runtime invocation contents were absent; no
  initial commit was created.
- Fresh-project `concoct exec --dry-run --reasoning high --timeout 5m` — passed
  with the expected Product Owner action, profile provenance, safe Codex command,
  prompt-on-stdin posture, and no runtime creation.
- `git diff --check` and stale direct-execution claim searches across active
  documentation, templates, defaults, and the Concoct skill — passed.

## Handoff to reviewer

### Implemented

Implemented the complete one-shot `concoct exec` surface: shared typed action
resolution, Product Owner decision results, strict role-profile resolution,
the Codex adapter, private runtime records, bounded process supervision,
outcome/state reconciliation, inspection, retention, CLI flags, ignored-state
installation, executable guidance, documentation, and focused tests.

### Key decisions

- Manual prompts and existing completion commands remain lifecycle authority;
  adapter claims and process status never substitute for observed evidence.
- Ready state performs only Product Owner judgment and returns before its
  recommendation. Archived integration uses the existing direct transaction.
- Codex user configuration is ignored for reproducible launch posture while
  project rules and authentication remain available; environment inheritance is
  allowlisted and never retained.

### Files changed

- Added `internal/adapter`, `internal/config`, and `internal/execution`.
- Updated workflow/orchestration authority, CLI dispatch, integration config,
  prompt guidance/goldens, initialization tests, root/template ignore rules,
  README and normative workflow/command/state documentation, and both copies of
  the Concoct skill.
- Updated only Developer-owned task plan and notes under `.concoct/current/`;
  roadmap, capabilities, reviews, and archives remain untouched.

### Verification

Full Go tests, focused race tests, vet, build, shell syntax, fresh-project
initialization, real CLI dry-run, executable mode, stale-claim search, and diff
whitespace validation passed as recorded above.

### Known risks

- No live external Codex model run was launched during verification; a
  controllable real-process fake adapter exercised the same stdin, schema,
  output-file, logging, cancellation, timeout, and reconciliation boundaries,
  and the installed Codex `0.146.0` command surface was validated by real
  dry-run resolution.
- Redaction covers tested common credential, authorization, token, password,
  secret, and OpenAI-key forms. As with any finite redactor, novel secret shapes
  require extending the allowlist/redaction tests.

### Skipped or unresolved work

- Repeated lifecycle execution, resume/replay, stale-process recovery, shared
  locking, interactive sessions, remote adapters, additional built-in adapters,
  and broad profile management remain assigned to later roadmap work.
- No product-scope acceptance criterion was skipped.

### Capability impact

The planned `add` impact remains accurate for proposed `CAP-013`: observable
one-shot execution of one authorized recommendation through Codex or the direct
integration authority, with private retained inspection evidence and observed-
state precedence. Capability truth remains for the Archivist after approval.

### Suggested review focus

Review typed-state/action parity, unchanged ready-state decision validation,
Git and configuration freshness, exact prompt bytes, Codex safety arguments and
environment allowlist, process-tree termination races, no-replace result
normalization, non-completion/error reporting, private runtime modes and symlink
defenses, redaction/retention bounds, direct integration gate preservation, and
manual workflow regressions.

## Review-01 finding dispositions

### Finding 1 — Policy-satisfied review cannot reach archival execution

- Disposition: `fixed`.
- The archival orchestration contract now accepts both `review-approved` and
  `implementation-complete` starting states, while `Authorize` independently
  verifies that the typed workflow/policy authority selected the requested
  action. Ordinary implementation-complete evidence therefore continues to
  authorize only independent review; explicit `not-required` and validated
  `externally-satisfied` dispositions authorize archival.
- Added orchestration and CLI dry-run coverage for all three policy routes,
  including direct refusal of archival authorization when review is required.

### Finding 2 — Repository evidence is not revalidated before launch

- Disposition: `fixed`.
- Real execution now re-resolves configuration and compares a fresh complete
  orchestration snapshot with the authorized state and digest immediately
  before branching to either the adapter process or direct integration.
- Controllable after-prepare tests mutate covered repository evidence and prove
  that neither the fake adapter process nor direct integration starts; each
  attempt closes as `authorization-changed` with reconciliation evidence.

### Finding 3 — Direct integration swallows cancellation and timeout

- Disposition: `fixed`.
- Direct integration now receives the resolved invocation context and timeout.
  Git commands and push confirmation observe cancellation; completed local
  mutation steps are persisted to recovery before interruption is returned,
  and bookkeeping finishes to the durable delivered phase once it begins so a
  partially cleared current state is not exposed as a resume boundary.
- Integration- and execution-level cancellation/timeout tests block at push
  confirmation, assert prompt interruption, retain recovery evidence, prevent
  fabricated result acceptance, and close a private reconciliation record with
  `cancelled` or `timed-out` disposition.

## Remediation verification results

- `go test -count=1 ./...` — passed.
- `go test -count=1 -race ./internal/orchestration ./internal/execution ./internal/integration` — passed.
- Focused orchestration, CLI, execution, integration, and Git repository tests
  passed, including all new policy, pre-launch drift, direct cancellation, and
  direct timeout cases.
- `go vet ./...`, native `go build ./cmd/concoct`, and
  `GOOS=windows GOARCH=amd64 go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` and executable-mode validation — passed.
- Fresh initialization outside the repository — passed: project-owned
  dotfiles and nested adapters, current/archive directories, staged bootstrap
  guidance, an unborn Git branch, and runtime ignore behavior were present;
  built-in protocol/persona/prompt directories and runtime contents were
  absent.
- `git diff --check` and stale direct-execution/legacy-path searches — passed.

### Attempt: complete remediation transition

- Tried: `./cmd/concoct/concoct.sh code --complete` after implementation and
  verification.
- Error/result: the transition validator reported that Developer output was
  not yet a valid implementation-complete transition.
- Why it failed: the task plan had not yet named `review-01.md` in the required
  `remediates-review` metadata, so the authoritative latest review continued to
  keep workflow state at `review-changes-requested` despite complete findings
  dispositions.
- Next approach: added the exact review reference to task metadata, preserved
  the completed review unchanged, and revalidated state before retrying.

## Handoff to reviewer

### Implemented

Remediated all three review-01 findings: policy-valid archival authorization,
complete pre-launch evidence freshness for adapter and direct actions, and
cancellable/time-bounded direct integration with recoverable interruption.

### Key decisions

- State membership alone is insufficient authorization: orchestration now also
  requires the requested kind to match the typed policy-selected action.
- Pre-launch freshness compares the original complete evidence digest; normal
  post-action mutation remains governed separately by observed postconditions.
- Direct integration propagates context through Git and confirmation waits, but
  never returns from partially applied local bookkeeping; it first records a
  safe recovery phase and then reports cancellation or timeout.

### Files changed

- Updated orchestration authorization, execution freshness/direct supervision,
  context-aware Git operations, and integration recovery handling.
- Added policy-route, adapter/direct drift, integration interruption, and full
  direct reconciliation tests.
- Clarified direct cancellation behavior in `doc/command-reference.md` and
  updated only Developer-owned task plan and notes artifacts.

### Verification

Full Go tests, focused race tests, vet, native and Windows builds, shell syntax
and mode checks, fresh initialization, stale-claim/path searches, and diff
whitespace validation passed as recorded above.

### Known risks

- Context cancellation can arrive after a Git command has committed locally.
  The implementation reopens the repository without the cancelled context,
  verifies the exact integration commit subject, and records that commit before
  returning, so subsequent recovery does not mistake it for unrelated work.
- A cancelled confirmation read may leave a blocked reader goroutine until its
  underlying process exits; it cannot mutate the repository, publish a result,
  or keep the CLI process alive.

### Skipped or unresolved work

- No review-01 finding or product-scope acceptance criterion remains skipped.
- Repeated execution, durable multi-invocation recovery, shared locking, and
  additional adapters remain outside CON-010 as previously recorded.

### Capability impact

The planned `add` impact for proposed `CAP-013` remains accurate. The remediated
behavior provides policy-correct one-shot authorization, launch-time evidence
freshness, and interruptible direct integration while capability truth remains
reserved for the Archivist after approval.

### Suggested review focus

Recheck the required/not-required/externally-satisfied archival matrix, the
after-prepare no-launch boundary for adapter and direct actions, cancellation
around Git commit recovery, timeout/cancellation at push confirmation, absence
of fabricated completed results, and preservation of manual integration paths.
