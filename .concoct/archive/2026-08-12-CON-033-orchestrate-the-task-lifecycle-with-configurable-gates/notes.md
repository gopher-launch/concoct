# Notes

## Planning summary

CON-033 is ready for implementation. It has no unresolved roadmap dependency,
all five declared capability prerequisites are active and compatible, no active
task artifacts conflicted with planning, and repository inspection confirms the
accepted one-shot executor and structured action protocol can be composed into
the requested bounded lifecycle loop.

The plan adds `CAP-014` for repeated authorized workflow execution. It keeps
workflow state, action authorization, role completion, immutable review,
archival, and Git integration under their existing authorities. Minimal private
pending-gate evidence supports the documented approval commands; general
durable run recovery remains CON-034.

## Confirmed findings

- Git identity is trunk `main`, task branch
  `concoct/con-033-orchestrate-the-task-lifecycle-with-configurable`, and base
  `b822466e9c969576cc5365460de6c84d2e8b4733`. The task branch was clean and at
  that exact base before planning artifacts were authored.
- `.concoct/current/task-plan.md` and `.concoct/current/notes.md` were absent;
  no active reviews or conflicting task existed.
- The roadmap item is `planned`, has no unresolved dependency, and provides
  explicit decisions for ready-state proposals, approval scope, default gates,
  limits, reviewer independence, progress detection, stop reporting, and push
  authority.
- `workflow.ResolveAction` is the typed recommendation authority shared by
  status and one-shot execution. Ready state resolves only
  `product-owner-next`; the registered `task-planning` action is not selected
  without an additional approved planning context.
- `orchestration.ValidateOutcome` validates Product Owner plan recommendations
  against current eligibility, but `DurableFacts` and `execution.Result` do not
  expose that recommendation to a caller.
- `execution.Run` already rechecks configuration and action evidence before
  every launch, creates a new ephemeral Codex process for every prompt-backed
  action, records a bounded private invocation, and reconciles actual state.
  Its public one-shot contract treats accepted non-completion outcomes as a
  returned error after recording them.
- Manual `plan` owns deterministic branch creation and exact Planner prompt Git
  context inside CLI dispatch. That logic must become reusable for the
  approved-selection path rather than being called recursively.
- Direct integration currently owns optional remote push and observes
  `git.auto-push`. The run path needs an explicit local-only integration option
  while preserving manual behavior and recovery.
- `.gitignore` and `templates/.gitignore` ignore only
  `.concoct/runtime/invocations/`, so a new private pending-gate path requires
  matched source/template ignore updates and initialization verification.
- The repository has focused orchestration, execution, configuration, CLI,
  workflow-transition, integration, prompt-golden, and real-Git test seams that
  can cover the new coordinator without replacing accepted tests.

## Prerequisite compatibility

- CAP-001 requires humans or agents to supply semantic role judgment and makes
  durable repository evidence authoritative. The coordinator executes those
  roles through existing prompts and validates their artifacts; it does not
  manufacture judgment.
- CAP-005 limits parsing to checked-in schemas and keeps status read-only. A
  finite run-policy and private gate schema are compatible, and every run
  mutation remains behind the project-contract and workflow validators.
- CAP-007 leaves semantic conflict resolution to humans and does not support
  concurrent or stacked tasks. CON-033 stops at integration recovery and stays
  within the single task branch model.
- CAP-009 reserves semantic prioritization for Product Owner judgment. The
  invariant proposal gate validates and retains the Product Owner's structured
  recommendation rather than ranking or selecting an item itself.
- CAP-013 is intentionally one-shot and lacks looping or durable multi-run
  recovery. CON-033 composes fresh one-shot actions with finite bounds; it does
  not weaken CAP-013 or claim CON-034 recovery.

CAP-012 is also relevant accepted history even though it is not a declared
roadmap prerequisite: its structured action/outcome registry and observed-state
validation are the direct transport boundary used by CAP-013 and this task.

## Planning decisions

- Add `CAP-014` rather than redefining CAP-013; one-shot execution remains
  independently useful and behaviorally compatible.
- Put the finite orchestration policy in a dedicated `run` section of the
  strict configuration model. Keep it separate from lifecycle-state
  definitions and adapter role profiles. Defaults require plan and integration
  approval; `next` and all safety/intervention stops are invariant.
- Store only the current pending gate below the private ignored runtime
  boundary. It must survive the separate approval invocation, but it is not
  canonical workflow truth, permanent history, or a resumable in-flight run.
- Reuse the one-shot executor as a typed library boundary and preserve the
  existing `exec` CLI contract. The coordinator must not inspect retained JSON
  or parse command output to discover an outcome.
- Share planning setup between manual and orchestrated paths so eligibility,
  branch identity, prompt bytes, rollback, and compatibility behavior cannot
  diverge.
- Count a code/review cycle when a Reviewer action completes with an observed
  review outcome. The third review may approve or otherwise stop; a fourth
  Reviewer action is outside the per-run bound.
- Make run-driven integration local-only. Remote delivery remains a separate
  authority and no existing auto-push configuration is consumed by `run`.

## Risks

- A stale gate checked before final authorization could apply a proposal, plan,
  or integration approval to changed evidence.
- Planning branch creation can leave a collision after a failed invocation;
  rollback is safe only before agent-authored changes may exist.
- Refactoring one-shot execution may regress its established error, retention,
  cancellation, or inspection behavior if the reusable typed result and CLI
  presentation are not kept separate.
- Fingerprints must distinguish productive remediation from no progress without
  treating irrelevant private runtime bytes as workflow advancement.
- Reviewer independence can be undermined if the loop reuses adapter session
  context or stale role configuration instead of preparing a fresh invocation.
- Local integration and optional push are currently coupled closely enough that
  a run-specific no-push test is mandatory, especially because this repository
  has `git.auto-push: true`.

## Initial verification

- Read the complete executable-rendered Task Planner prompt and selected
  persona, `.concoct/policy.md`, `AGENTS.md`, capabilities, roadmap, every
  required archive summary, and the relevant CON-032 structured-protocol
  archive summary.
- Inspected CLI dispatch, workflow action resolution and state detection,
  orchestration action/outcome validation, one-shot execution and records,
  strict configuration, adapter invocation, integration, prompt planning,
  ignore templates, tests, README, command reference, state machine, workflow,
  and instruction-layer documentation.
- Confirmed the exact branch, base, clean starting state, absence of active
  artifacts, and current accepted test/package boundaries.
- No source code, tests, product capabilities, archives, or review artifacts
  were changed during planning.

## Developer handoff

### Current state

Planning is complete; CON-033 is `planned` on its recorded task branch.

### Completed

- Validated roadmap readiness, prerequisite compatibility, Git identity,
  repository reality, task scope, and capability impact.
- Defined the reusable one-shot/planning boundaries, finite run policy, pending
  approval model, loop termination and cycle semantics, local-only integration
  constraint, acceptance criteria, and verification matrix.

### Remaining

- Implement the task plan without expanding into CON-034 recovery, concurrent
  tasks, arbitrary workflow configuration, automatic retries, or remote push.
- Record exact technical choices and evidence here, update phase statuses, and
  prepare a fresh Reviewer handoff.

### Known risks

Approval freshness, planning-branch rollback, typed-result compatibility,
progress fingerprint accuracy, reviewer isolation, and accidental remote push.

### Suggested next step

`concoct code`

## Implementation findings and decisions

### Reusable execution and planning boundaries

- `execution.RunAccepted` now returns the accepted structured outcome,
  recommendation, durable facts, reconciliation, and observed state without
  changing the public one-shot `execution.Run` refusal for non-completion.
- The shared `internal/planning` operation owns eligibility, deterministic Git
  branch/base setup, exact planner prompt context, and rollback before an agent
  can start. Manual `plan` and approved run-driven planning use that same
  operation.
- `orchestration.AuthorizePlanning` is the only new selection authority. It
  requires a validated eligible Product Owner selection and ready state;
  ordinary ready-state resolution still authorizes only `product-owner-next`.

### Effective run policy

- Strict `run` configuration has additive optional gates `development`,
  `review`, and `archive`. Built-in `plan` and `integration` gates remain
  mandatory, and `next` remains invariant.
- Built-in maxima are 20 actions and three completed Reviewer actions per
  invocation. User and project configuration compose monotonically by gate
  union and minimum bound; invocation flags may add gates or lower the
  effective bounds but cannot raise them.
- Unknown fields or gates, duplicate gates, zero/out-of-range bounds, and
  permissive invocation bounds fail before runtime evidence.

### Pending approval evidence

- The sole private record is `.concoct/runtime/pending-gate.json`. Version 1
  binds gate, protected action, task, attempt, proposal/source correlation,
  workflow state, repository evidence digest, project/user configuration
  digest, optional selected roadmap ID, and creation time.
- Records are bounded to 16 KiB, mode `0600` below a mode-`0700` runtime
  directory, strict-decoded, atomic no-replace, Git-ignored, and claimed through
  a compare-validated atomic rename before one-use deletion. Concurrent consume
  attempts cannot consume a later replacement gate.
- Relevant artifact, Git, workflow, project-configuration, or user-
  configuration drift invalidates approval. Invalid private evidence is removed
  without changing canonical workflow state.

### Coordinator and stop behavior

- `internal/runloop` rechecks contract compatibility, run policy, workflow,
  repository evidence, action authority, role profile, prompt, and adapter for
  each action. It never invokes the CLI recursively or parses human output.
- Progress identity is the typed action plus the canonical artifact/Git
  evidence digest. Repeated pairs stop; changes-requested review can continue
  only after the material fingerprint advances. A Reviewer completion counts
  one cycle, permitting approval on cycle three but no fourth review.
- Every adapter action uses a new ephemeral invocation; Developer and Reviewer
  invocation IDs and processes are distinct. Accepted non-completion,
  cancellation, failure, blocked/invalid/recovery state, exhausted bounds, and
  no progress stop without automatic retry.
- Summaries include action/outcome/state steps, before/after progress digests,
  bounded repository identity, consumed and remaining action/cycle counts,
  pending gate or stop reason, and a safe continuation when available.

### Integration and compatibility

- `integration.Options.LocalOnly` suppresses the push path while preserving the
  existing transaction, recovery, bookkeeping, branch cleanup, and validation.
  `concoct run` always selects local-only integration. Manual `integrate` and
  one-shot `exec` preserve their prior confirmation and `git.auto-push`
  behavior.
- README, workflow, state-machine, command-reference, project configuration,
  generated skill guidance, and source/template runtime ignores document the
  bounded run contract without claiming CON-034 crash recovery.

## Verification results

- `gofmt` completed for all changed Go files.
- `go test -count=1 ./...` passed with a task-local Go build cache.
- `go test -race -count=1 ./internal/runstate ./internal/runloop ./internal/execution ./internal/integration` passed.
- `go vet ./...` passed; `go build ./cmd/concoct` passed. Go emitted a non-fatal
  stat-cache warning because the environment's module download cache is
  read-only; the build output itself completed and its generated root binary
  was removed.
- `bash -n cmd/concoct/concoct.sh` passed and the wrapper remains executable.
- A generated project under `/tmp` validated staged dotfiles, nested adapters,
  planning directories, project contract, bootstrap prompt, Git repository,
  no generated commit, pending-gate ignore coverage, generated run guidance,
  and absence of installed built-in protocol/persona/handoff files. The
  temporary project was removed after inspection.
- Focused tests cover monotonic policy and lower-only bounds; stale,
  malformed, oversized, public, replayed, and concurrently consumed gates;
  Product Owner proposal persistence and stale rejection before branch change;
  approved planning branch/base context; multi-action Developer-to-independent-
  Reviewer execution; accepted intervention stop; local-only integration with
  auto-push enabled; generated ignore coverage; and existing manual/one-shot
  compatibility suites.
- `git diff --check` passed. The one-shot wording found by the stale-claim
  search is scoped to `exec`; run documentation consistently states local-only
  integration.

## Human-authorized Review 01 remediation refinement

### Decision and authority

On 2026-08-12 the human accepted a technical refinement required for the
intended autonomous lifecycle: a supervised agent adapter performs the semantic
role work and verification, while the outer Concoct executable validates the
candidate through the canonical completion boundary and creates the Git
transition commit before handoff. The adapter must remain sandboxed and must not
receive Git-metadata write access or a bypass flag.

This belongs in CON-033 remediation because `concoct run` composes the same
execution boundary and cannot autonomously traverse a real Git-backed
Developer/Reviewer/Archivist lifecycle while the nested adapter is responsible
for commits it is structurally unable to create.

### Observed evidence

- Developer invocation `bc48b2655d6dd78633f376d0aa888ee4fcfa` authored and
  verified the implementation, then failed to create `.git/index.lock`. Its
  blocked result was rejected during post-run configuration resolution because
  the running installed binary did not understand the newly authored strict
  `run` configuration field.
- Reviewer invocation `34d4748e8d359e225a5d43b57fb4b9950bcc` authored the
  complete `changes-requested` `review-01.md`, then failed at the same Git
  boundary. Its recoverable result was rejected because non-completion
  validation required the post-review repository digest to equal the
  pre-review authorization digest.
- These are not malformed adapter results. They expose two conflations in the
  existing executor: sandboxed semantic work versus privileged transition
  finalization, and pre-launch freshness versus expected post-launch action
  effects.

### Required remediation in addition to Review 01

- Retain exact pre-launch configuration/evidence authorization and the
  `workspace-write` adapter posture.
- Give supervised prompts a fixed executable-owned appendix instructing the
  adapter to author and verify the candidate without invoking the final Git
  completion. Preserve the exact manual role prompt as the unchanged semantic
  core.
- After a correlated completed candidate returns, have the outer executable
  call reusable planning, Developer, Reviewer, or archival completion logic.
  Validate role-owned paths, required evidence, workflow/Git context, and
  postconditions before committing. Do not recursively invoke the CLI and do
  not infer completion from prose or exit status.
- Keep non-completion outcomes as stops. Preserve authorized partial work,
  reject forbidden effects, report unsafe replay truthfully, and never upgrade
  a blocker, decision, failure, cancellation, or malformed result into a
  completed transition.
- Preserve one-shot `exec` semantics and allow `run` to continue only after the
  outer executable has committed and re-detected the completed action. Fresh
  Reviewer process isolation, configured gates, action/cycle bounds, and
  local-only integration remain unchanged.
- Add focused and real-Git tests for adapter-without-Git-write finalization,
  legitimate configuration edits, partial outcomes, forbidden drift, commit
  failure and idempotence, plus the complete lifecycle matrix required by
  Review 01.

### Review finding disposition state

- Review 01 Finding 1: fixed. Consumed approval and attempt correlation now
  apply only to the immediate action occurrence. When plan acceptance and an
  optional development gate protect the same action, the later private record
  carries only that consumed prerequisite; remediation and later review
  occurrences create fresh evidence-bound gates and attempt IDs. Focused tests
  exercise two development and two review occurrences.
- Review 01 Finding 2: fixed. Pending-gate create, load, and consume validate
  `.concoct` and `runtime` with `Lstat`, reject symlinks and non-directories,
  and verify private permissions before mutation. Parent-symlink tests confirm
  that the external target's mode and contents remain unchanged.
- Review 01 Finding 3: fixed. Coordinator tests now cover real-Git local
  development/review/archive/integration delivery without push, productive
  changes-requested cycles, repeated optional gates, action/cycle bounds, all
  structured non-completion classes, cancellation, process failure, approval
  drift, fresh Reviewer invocation identity, planning context, and local-only
  integration. Existing focused execution and integration suites retain
  planning authorization/rollback, no-progress postconditions, policy routes,
  recovery, manual compatibility, and push-suppression coverage.
- Additional authorized execution-boundary finding: fixed. Supervised prompts
  retain the manual prompt as their unchanged semantic core and append a fixed
  executable boundary. The outer executor validates correlation, role-owned
  paths, unchanged Git HEAD, workflow postconditions, and then invokes the
  reusable Planner, Developer, Reviewer, or Archivist completion authority.
  Tests cover outer Git finalization, legitimate configuration edits, retained
  partial work, forbidden drift, refusal of adapter-created commits, outer
  commit failure, and clean canonical completion reuse.

### Manual handoff ownership

The human explicitly owns the manual commits for this context update. Preserve
role separation: commit the already-authored `review-01.md` as the Reviewer
transition by itself, then commit this task-plan/notes refinement separately.
Do not combine the immutable review artifact with Developer-owned context in one
commit. The next safe role after those commits is Developer remediation via the
version-matched source executable's `concoct code` prompt.

## Handoff to reviewer

### Implemented

Added `concoct run` with invariant Product Owner proposal approval, default
plan and integration gates, optional restrictive gates, finite action/review
bounds, fresh per-action authorization and adapter invocation, productive
remediation progress, one-use drift-bound pending evidence, complete stop
summaries, and local-only integration.

### Key decisions

- Compose the existing one-shot executor and canonical role completions rather
  than add a second state machine.
- Bind pending approvals to both canonical repository evidence and project/user
  configuration bytes while keeping invocation-only restrictions monotonic.
- Share planning setup between manual and run paths; selected planning remains
  a narrow authority unavailable to ordinary ready-state resolution.
- Count a cycle only when Reviewer completion is accepted; non-completion stops
  without consuming a cycle or retrying.

### Files changed

- CLI/configuration: `internal/cli`, `internal/config`, `.concoct/config.yaml`.
- Execution boundaries: `internal/execution`, `internal/orchestration`,
  `internal/planning`, `internal/integration`.
- Coordinator/private state: `internal/runloop`, `internal/runstate`.
- Tests: focused package tests plus project initialization coverage.
- Documentation/templates: README, command reference, workflow, state machine,
  repository and generated-project Codex skill counterparts, and
  source/template ignore files.

### Verification

Full tests, focused race tests, vet, build, shell syntax, generated-project
initialization, executable-bit, stale-claim, and diff checks passed as recorded
above. On 2026-08-12 the preserved implementation was revalidated with
`go test -count=1 ./...`, focused race tests for `internal/runstate`,
`internal/runloop`, `internal/execution`, and `internal/integration`,
`go vet ./...`, a clean `go build ./cmd/concoct`, shell syntax and executable
checks, `git diff --check`, and a fresh initialized project outside the
repository. All checks passed.

### Known risks

- The coordinator is intentionally same-process and non-resumable after a
  crash; only the current approval gate survives. CON-034 retains recovery.
- Pending approval evidence is deliberately local and non-authoritative; a
  process interruption after an action starts still requires the existing
  manual evidence inspection and recovery boundaries.

### Skipped or unresolved work

- No acceptance behavior was skipped. Crash replay, abandonment, concurrent
  tasks, automatic retries, arbitrary gates, and remote push remain explicit
  non-goals.

### Capability impact

Adds planned `CAP-014`: bounded repeated execution of authorized workflow
actions with configurable restrictive gates, progress detection, independent
review, and safe intervention reporting. `CAP-013` remains the compatible
one-shot primitive.

### Suggested review focus

Verify Product Owner selection authority, approval/configuration freshness,
atomic gate consumption, plan-branch rollback boundaries, action/cycle and
fingerprint termination, fresh Reviewer isolation, non-required review routing,
integration recovery summaries, and remote-push suppression.

### Attempt: complete Developer transition

- Tried: `concoct code --complete` after the complete verification pass.
- Error/result: Git could not create `.git/index.lock` because repository Git
  metadata is read-only in this execution environment. No commit was created.
- Why it failed: the transition requires `git add -A` and a Developer commit,
  while the available filesystem authority permits reading but not writing
  `.git`.
- Resolution: Git metadata is writable in the current session. The installed
  binary still predates the new strict `run` configuration field, so the
  version-matched source-checkout executable is the correct completion
  authority. Phase 6 and this handoff were refreshed after the full checks
  passed again, and the previously omitted repository-local Codex skill
  counterpart was synchronized with its generated-project template.
- Next approach: run `./cmd/concoct/concoct.sh code --complete` once to create
  the Developer transition commit, then hand off to independent review.

The first remediation completion attempt correctly refused because the task
front matter still lacked `remediates-review: review-01.md`. The Developer
added that required structured link to the immutable review before retrying.
The subsequent validated completion reached the Git transition boundary but
could not create `.git/index.lock` because Git metadata is read-only in this
environment. No transition commit was created and the complete worktree was
left intact for a writable Git context.

## Handoff to reviewer

### Implemented

- Scoped optional approvals and their attempt IDs to one immediate action
  occurrence, including safe chained plan/development gate evidence.
- Hardened pending-gate parent validation against symlink and non-directory
  redirection before create, load, consume, chmod, link, or rename.
- Added executable-owned supervised finalization: adapters author and verify
  candidates without Git authority; the outer process validates effects and
  invokes canonical Planner, Developer, Reviewer, and Archivist completion.
- Expanded coordinator and execution coverage through a real-Git
  development/review/archive/local-integration lifecycle, repeated remediation
  gates, bounds, stop classes, partial work, forbidden effects, and no-push.
- Updated README, command reference, workflow, and state-machine documentation
  for prompt-core supervision, finalization ownership, and gate occurrence
  semantics.

### Key decisions

- A pending optional gate may carry only consumed `plan` approval when both
  gates protect the same forthcoming development action; no approval is cached
  by action kind or carried into a later occurrence.
- Pre-launch configuration and evidence remain immutable authorization checks.
  Post-launch reconciliation accepts only path-validated role effects, allowing
  legitimate Developer configuration changes and honest partial work.
- Completed supervised candidates are not authority by themselves. Correlation,
  bounded schema, unchanged HEAD, role-owned paths, workflow postconditions,
  and the existing completion primitive all must succeed before acceptance.

### Files changed

- Execution/finalization: `internal/execution`, `internal/orchestration`, and
  `internal/planning`.
- Coordinator/private approvals: `internal/runloop` and `internal/runstate`.
- Tests: focused execution, coordinator, repeated-gate, real-Git lifecycle,
  private-path, stop-class, bounds, and race coverage.
- Documentation: `README.md`, `doc/command-reference.md`, `doc/workflow.md`,
  and `doc/state-machine.md`.
- Durable task context: this plan's Phase 7 status and Review 01 dispositions.

### Verification

- `go test -count=1 ./...` passed.
- `go test -race -count=1 ./internal/runstate ./internal/runloop
  ./internal/execution ./internal/integration` passed.
- `go vet ./...` and `go build -o /tmp/concoct-con033-bin ./cmd/concoct`
  passed; Go emitted the known non-fatal read-only module stat-cache warning.
- `bash -n cmd/concoct/concoct.sh` passed and the wrapper remains executable.
- Fresh initialization under `/tmp` passed: Git repository, staged dotfiles,
  nested Codex skill, current/archive directories, project contract/bootstrap,
  and absence of installed built-in protocol/persona content were confirmed.
- Source/template Concoct skills are identical; stale prompt-ownership claims
  were checked; `git diff --check` passed.

### Known risks

- Same-process run crash recovery remains deliberately out of scope for
  CON-033; CON-034 retains interrupted-run reconciliation and abandonment.
- Valid partial non-completion work is intentionally left uncommitted for
  inspection. The summary reports it as unsafe to replay when evidence moved.
- The remediation is currently uncommitted solely because this environment
  cannot write `.git/index.lock`; independent review must not begin until the
  canonical Developer completion succeeds in a writable Git context.

### Skipped or unresolved work

No Review 01 finding remains open. The only unresolved transition work is the
required Developer commit blocked by read-only Git metadata. Crash-resumable
runs, concurrent tasks, automatic retries, arbitrary gates, sandbox bypass,
and remote push remain the task's explicit non-goals.

### Capability impact

The planned `CAP-014` addition remains accurate: bounded repeated lifecycle
execution with configurable restrictive gates, progress detection, independent
review, supervised canonical transitions, intervention reporting, and
local-only integration. Capability truth remains unchanged pending approval.

### Suggested review focus

Recheck one-occurrence approval handling across repeated Developer/Reviewer
cycles, symlink-safe pending-state operations, outer finalization refusal paths,
partial-work reconciliation, and the real-Git archive/integration no-push test.

## Review 02 remediation

### Finding disposition

- Review 02 Finding 1: fixed. Coordinator tests now cover non-required and
  externally satisfied independent-review routing without a fabricated review,
  repeated action/evidence no-progress termination, planning startup rollback,
  preservation after possible Planner mutation, and real-Git integration
  conflict recovery with the exact `--continue`/`--abort` continuation. Exact
  byte comparisons cover the Task Planner, Developer, Reviewer, and Archivist
  manual prompt cores followed only by the fixed supervision appendix.

### Defects exposed by the completed matrix

- No-progress detection previously ran after gate selection. A completed action
  that left the same action/evidence fingerprint could therefore recreate its
  gate before the coordinator recognized the repeat. Fingerprint detection now
  precedes gate creation, while only actually attempted actions enter the seen
  set.
- During a real integration conflict, recovery evidence existed on the trunk
  but workflow detection validated the archived `self` commit as though the
  repository were still attached to the task branch. Recovery precedence now
  resolves the state as `integrating` before task-branch-only `self` validation,
  allowing the coordinator to report the canonical recovery continuation.

## Handoff to reviewer

### Implemented

- Completed every coordinator and prompt-boundary case requested by Review 02.
- Fixed gate ordering so repeated unchanged action/evidence pairs stop for no
  progress before another approval record can be created.
- Fixed integration-recovery state precedence so a real conflict reports
  `integrating` with the exact safe continuation instead of invalid state.

### Key decisions

- The no-progress fake action runner is injected through a package-private
  coordinator seam; the public `runloop.Options` and production execution path
  remain unchanged.
- Existing recovery evidence retains its established precedence over checks
  that apply only to a pre-integration archived task branch. All task, review,
  policy, and Git metadata validation before that boundary remains intact.
- Prompt parity is asserted against independently rendered manual bytes for
  each supervised role, not by checking headings or substrings.

### Files changed

- `internal/runloop/run.go` and `internal/runloop/run_test.go` for fingerprint
  ordering and the omitted coordinator boundary matrix.
- `internal/workflow/workflow.go` for integration-recovery precedence.
- `internal/execution/execution_test.go` for exact four-role prompt parity.
- `.concoct/current/task-plan.md` and this notes file for Phase 8, Review 02
  disposition, verification, and handoff evidence.

### Verification

- `gofmt` completed for all changed Go files.
- `go test -count=1 ./...` passed.
- `go test -race -count=1 ./internal/runstate ./internal/runloop
  ./internal/execution ./internal/integration` passed.
- `go vet ./...` passed.
- `go build -o /tmp/concoct-con033-bin ./cmd/concoct` passed with only the
  known non-fatal read-only Go module stat-cache warning.
- `bash -n cmd/concoct/concoct.sh` passed; its mode remains `775` and executable.
- Fresh initialization under `/tmp` passed: Git initialization and staging,
  dotfiles, nested Codex skill, planning/archive directories, project contract,
  bootstrap prompt, pending-gate ignore coverage, and exclusion of built-in
  protocol/persona/handoff files were confirmed.
- Source/template Concoct skills are byte-identical; stale one-shot and
  auto-push claims were inspected; `git diff --check` passed.

### Known risks

- Same-process crash recovery remains intentionally outside CON-033 and belongs
  to CON-034.
- Integration conflict resolution remains an explicit human choice after the
  coordinator stops; this remediation verifies truthful state and continuation
  reporting but does not automate that choice.

### Skipped or unresolved work

No Review 02 finding remains unresolved. Crash-resumable runs, concurrent
tasks, automatic retries, arbitrary gates, sandbox bypass, and remote push
remain explicit non-goals.

### Capability impact

The planned `CAP-014` addition remains accurate: bounded repeated lifecycle
execution with configurable restrictive gates, no-progress detection,
independent review, supervised canonical transitions, recovery/intervention
reporting, and local-only integration. Capability truth remains unchanged
pending independent approval.

### Suggested review focus

Confirm the new tests exercise the coordinator rather than only lower layers,
the fingerprint check cannot be masked by gate recreation, recovery precedence
does not bypass pre-recovery validation, and all four supervised prompt values
equal manual bytes plus only the fixed appendix.

## Post-approval archival completion remediation

### Finding

The first real `concoct archive --complete` attempt exposed a contradictory Git
archive contract. Candidate validation required archive `task-plan.md` to be
byte-identical to current accepted evidence, while Git completion required both
copies to have already changed from `git.status: active` to `archived` with
`git.archive-commit: self`. The current active task and immutable approving
revision therefore could not satisfy both validators at once.

The authored archive candidate demonstrates the contradiction directly: its
task plan differs from accepted current evidence only by the pre-applied
archival Git fields. Review 03 predates this remediation and does not approve
the resulting source, test, or workflow-contract changes.

### Decision

- Accepted evidence remains authoritative. A Git archive candidate must copy
  the accepted task plan byte-for-byte and must not pre-edit current task
  metadata.
- The outer `archive --complete` boundary reconstructs accepted task bytes from
  the immutable pre-archive `HEAD`, derives only the two completion-owned Git
  metadata values, writes the current task atomically, stages the validated
  transaction, and commits it.
- Clean retries reconstruct the expected current-only transition from the
  archive commit's immutable parent. A process-interrupted dirty retry is
  accepted only when current task bytes exactly equal that derived transition.
  Every other current or archive task edit remains rejected.
- A staging or commit failure with unchanged `HEAD` resets the index and
  atomically restores accepted current task bytes, leaving the authored
  candidate recoverable for a later retry.

### Implementation

- `internal/workflow/archive.go` now validates candidate task bytes against an
  explicit accepted snapshot, applies the current-only metadata transition at
  the completion boundary, validates clean retries against the immutable
  parent, and restores task/index state on pre-commit failure.
- `internal/gitrepo/git.go` exposes the mixed reset needed to restore the index
  without discarding the validated Archivist worktree candidate.
- CLI and workflow regressions cover byte-exact candidates, exact derived
  metadata, rejection of the formerly pre-mutated candidate, commit failure
  rollback and retry, interrupted-transition retry, clean committed retry,
  forbidden paths, and subsequent integration.
- The real run-loop Archivist fixture now authors an unchanged task copy, and
  Archivist persona, handoff, command/workflow documentation, embedded prompt
  expectation, and source/template Concoct skills describe completion-owned
  archival metadata consistently.

### Verification

- `go test -count=1 ./internal/workflow ./internal/cli ./internal/runloop
  ./internal/prompt` passed.
- `go test -count=20 ./internal/cli -run 'TestGitArchive'` passed.
- `go test -count=1 ./...` passed.
- `go test -race -count=1 ./internal/workflow ./internal/cli
  ./internal/runloop ./internal/execution ./internal/integration` passed.
- `go vet ./...` and `go build -o /tmp/concoct-con033-archive-fix
  ./cmd/concoct` passed.
- `bash -n cmd/concoct/concoct.sh` passed; the wrapper remains executable with
  mode `775`.
- Fresh initialization under `/tmp/concoct-con033-init.ydszcJ` passed: the
  generated project is a Git repository with staged dotfiles, nested Codex and
  GitHub adapter content, current/archive directories, and bootstrap guidance;
  it has no generated commit or installed built-in protocol/persona/handoff
  directories.
- Source/template Concoct skills are byte-identical, and `git diff --check`
  passed after the durable task updates.

## Handoff to reviewer

### Implemented

Moved archival Git metadata ownership into the executable completion boundary
while keeping the archive candidate byte-identical to accepted task evidence.
Added exact parent-based validation, rollback, interrupted and clean retry
handling, supervised lifecycle compatibility, and updated Archivist guidance.

### Verification

Focused archive tests, repeated Git archive tests, the full suite, relevant
race suites, vet, build, wrapper validation, source/template parity, diff
checking, and fresh external-project initialization all pass as recorded above.

### Known risks

- The already-authored Archivist candidate still contains the old pre-mutated
  task plan and a noncanonical summary Git block. It must be regenerated from
  the newly reviewed accepted evidence; this Developer remediation does not
  rewrite Archivist-owned evidence.
- Review 03 remains immutable historical approval of the pre-remediation
  revision. A fresh independent review is required before archival.

### Capability impact

The planned CAP-014 addition is unchanged. This remediation corrects the
transaction that archives its accepted evidence and does not add or broaden a
product capability.

### Suggested review focus

Verify that candidate task bytes remain exact accepted evidence, only current
metadata changes in the archive commit, all retry paths validate against
immutable Git parents, failed commits restore accepted task/index state,
forbidden paths remain rejected, and integration still consumes the exact
validated archive HEAD.
