# Notes

## Planning summary

CON-018 is implementation-ready. It has no unresolved roadmap dependency;
CAP-001, CAP-005, CAP-006, and CAP-007 are active prerequisites whose
limitations are compatible with a structural, evidence-backed policy model.
Adjacent roadmap outcomes are explicitly excluded so this remains one coherent
task.

## Confirmed findings

- Git identity is trunk `main`, task branch
  `concoct/con-018-configure-workflow-policy`, and base
  `f2b8cee2ec4b411b10164c36a4e3dc576867b029`.
- Before planning, `.concoct/current/` contained only `.gitkeep`; no active task
  or review conflicted with CON-018.
- `internal/instruction.Compose` owns source ordering and structural conflict
  validation, but returns source bytes rather than typed policy.
- Its line-oriented parser checks presence and ownership of policy declarations,
  not supported values, combinations, or runtime meaning.
- `internal/workflow.Detect` is the central state authority, with hard-coded
  task/review/archive/integration states and next commands.
- `internal/prompt` reuses detection but hard-codes role modes and transitions;
  policy prose is attributed without governing behavior.
- Completion, archive, and integration validators independently encode gates
  and changed-path boundaries that must consume the same resolution.
- Root and template policy files express the current default lifecycle;
  initialization installs the template as project-owned configuration.
- Existing workflow, prompt golden, CLI, archive, integration, and project tests
  are the regression baseline for exact default compatibility.
- CON-017 established policy ownership and deferred selectable behavior here;
  CON-020 still owns alternate Git strategies.

## Planning decisions

- Use a finite registry derived from the accepted lifecycle, not user-defined
  phases or graph edges.
- Separate configured requirements from evaluated evidence dispositions.
- Produce one effective policy and one resolution for all consumers.
- Require explicit reasons for authorized skips and safe durable evidence for
  external satisfaction; absence alone is never a disposition.
- Keep policy project-owned and task-specific disposition evidence in canonical
  active-task evidence with enforced ownership.
- Update materially affected existing capabilities after acceptance rather than
  adding a capability solely for internal policy types.

## Capability compatibility

- CAP-001 remains compatible because protocol invariants and role ownership stay
  unconditional.
- CAP-005 remains compatible because status continues to derive from repository
  evidence and does not claim semantic judgment.
- CAP-006 remains compatible because prompts stay deterministic guidance rather
  than role execution or completion proof.
- CAP-007 remains compatible because task-branch squash stays the only managed
  Git strategy; policy can resolve applicability without implementing CON-020.

## Risks

- A resolver separate from workflow detection could create two state
  authorities.
- Optional activity support could normalize incomplete work without strict
  owned evidence and negative tests.
- New task evidence affects transition allowlists, archives, and integration
  recovery and needs end-to-end coverage.
- Repeated policy context could make prompts noisy or nondeterministic.
- Nested policy data may exceed the safe bounds of the existing custom parser;
  strict YAML known-field decoding is preferable when needed.

## Technical choices left to implementation

- Exact Go package/type names and the closed YAML representation.
- The smallest canonical task-evidence extension that preserves ownership.
- The compact deterministic status/prompt presentation of requirements and
  dispositions.

These choices do not alter accepted product scope.

## Likely follow-up work

- CON-016, CON-019, CON-023, and CON-025 can build adoption, origins, profiles,
  and explanation on this model.
- CON-020 can add alternate Git strategies without changing policy ownership.
- CON-010 and later orchestration can consume the same resolved blockers and
  gates rather than creating another policy engine.

## Handoff

The completed planning transition should report `planned`. The Developer should
next run `concoct code`, set the task to `implementation-in-progress`, and
implement the typed policy/resolution authority without broadening into the
excluded roadmap work.

## Development findings

- `internal/instruction.Policy` is now the single typed policy model produced
  by instruction composition. It has a closed activity and gate registry and
  rejects unknown, duplicate, inconsistent, or unsupported policy choices
  before returning a partial composition.
- `workflow.Detect` consumes the composed policy and renders a deterministic
  requirement/disposition record for every governed activity. It never treats
  an absent artifact as a skip; omitted activities are explicitly
  `not-required` by policy and non-Git integration is `not-applicable` only
  when integration was selected.
- The default policy has been preserved byte-for-byte. An explicitly
  non-required independent review now changes only the permitted archive path:
  status recommends archival after implementation-complete, the archive prompt
  is eligible, and archive validation requires no review evidence or override.
- Focused and full test suites, `go vet`, `go build`, shell syntax validation,
  and `git diff --check` passed after this slice.

## Remaining implementation work

- None within CON-018 scope. The remaining adjacent policy profiles, alternate
  Git strategies, adoption, upgrades, workflow explanation, and orchestration
  outcomes retain their existing roadmap ownership.

## Development continuation

- Added `policy-activity-evidence` to active task metadata for the one
  supported non-default runtime disposition: `externally-satisfied`. Each
  record names one known required activity, uses that exact disposition, gives
  a durable reason, is recorded by the Developer or Task Planner, and cites at
  least one readable repository-relative regular file. Duplicate, unknown,
  unsafe, missing, unauthorized, and contradictory records invalidate state.
- The resolution rendering now includes accepted external evidence and its
  reason. `go test ./internal/workflow ./internal/prompt ./internal/instruction`
  passed after the change.
- A validated externally satisfied independent review is now a real completion
  condition: implementation-complete recommends archival, the Archivist prompt
  is eligible, and archive validation accepts an empty summary review field.
  An override remains forbidden because the task already has durable
  satisfaction evidence.

## Checkpoint verification

- `go test -count=1 ./...`, `go vet ./...`, `go build ./cmd/concoct`,
  `bash -n cmd/concoct/concoct.sh`, executable-bit validation, and
  `git diff --check` passed.
- Initialized `/tmp/concoct-con018-init.O1b8ka/project` with the local command.
  It was a Git repository with staged generated files and no commit; it
  contained project-owned policy, bootstrap, dotfiles, nested Codex skill, and
  planning artifacts, while executable-owned protocol and persona paths were
  absent.
- The task remains deliberately in implementation progress. The committed
  behavior is a checkpoint, not a reviewable completion: the remaining plan
  sections identify the resolver and transition coverage still required for a
  complete policy-aware lifecycle.

## Developer continuation: resolved review eligibility

- Role rendering and `concoct review --reserve` now use the resolved next
  action. A policy-valid externally satisfied independent review produces an
  archival handoff and refuses a redundant reviewer prompt or reservation.
- Added prompt regression coverage for the external-review path. Focused
  prompt, workflow, and CLI tests passed. The command reference now describes
  the policy-valid archival entry condition.
- Full verification passed: `go test -count=1 ./...`, `go vet ./...`,
  `go build ./cmd/concoct`, `bash -n cmd/concoct/concoct.sh`, executable-bit
  validation, and `git diff --check`. A fresh initialization at
  `/tmp/concoct-con018-code3.CkY0uC/project` produced staged, uncommitted
  project-owned artifacts and omitted executable-owned protocol/persona paths.

## Developer continuation: reservation integrity

- Added workflow coverage proving `concoct review --reserve` refuses a
  policy-valid externally satisfied independent review and leaves no
  `review-01.md` artifact behind. This preserves completed-review ownership and
  avoids creating a contradictory review path after the resolver selects
  archival.
- Re-ran `go test -count=1 ./...`, `go vet ./...`, `go build ./cmd/concoct`,
  `bash -n cmd/concoct/concoct.sh`, executable-bit validation, and
  `git diff --check`; all passed.

## Developer continuation: policy omission reasons

- Added the closed policy-owned `not-required-reasons` declaration. Every
  omitted activity must carry exactly one `activity: reason` entry; unknown,
  duplicate, blank, and required-activity entries fail composition. Resolved
  status now displays the configured durable reason rather than a generic
  omission message.
- Added parser coverage for a valid non-default policy and its missing-reason
  failure. Full Go tests, vet, build, shell syntax, executable-bit, and diff
  validation passed.

## Developer continuation: duplicate declarations

- The instruction front-matter parser now rejects duplicate declaration keys
  instead of silently overwriting a prior value. This closes an all-or-nothing
  policy-validation gap; focused regression coverage and the full required
  verification suite passed.

## Developer continuation: Git applicability

- Git-backed active tasks now reject policies that omit `integration`; managed
  task-branch archival cannot truthfully promise a no-integration path. A
  focused regression test covers the diagnostic, and the full verification
  suite passed.

## Developer continuation: integration entry gate

- `concoct integrate` now composes the effective policy before starting a new
  managed integration and refuses when integration is not required. Recovery
  modes intentionally retain their prior artifact-preservation path, including
  when unrelated work makes current guidance invalid. Added integration
  regression coverage; the full verification suite passed.

## Developer continuation: archive entry gate

- Git-backed archival now enforces the same required-integration policy before
  examining archive candidates, so no archival transition can begin under an
  incompatible Git policy. Added a focused archive regression test; full
  verification passed.

## Developer continuation: shared Git context gate

- `InspectGitContext`, used by Developer and Reviewer transition validation,
  now rejects a Git-backed task whose policy omits integration. This aligns the
  shared Git boundary with status, archive, and direct integration checks.
  Full verification passed.

## Developer continuation: contradictory review evidence

- State detection now rejects a task that claims externally satisfied
  independent review while immutable review evidence also exists. This prevents
  competing review paths from being silently resolved by precedence. Added
  regression coverage; full verification passed.

- The same contradiction rule now covers explicit `not-required` review
  policy, preventing an authored review from bypassing that selection.

## Developer continuation: supported policy boundary

- Tightened the finite policy contract to reject lifecycle choices that the
  current command/state machine cannot honor. Product ownership, task planning,
  development, archival, and integration remain required; independent review
  is the supported selectable activity. Review approval is required exactly
  when review is required, and archive-before-integration remains mandatory.
- Restricted task-scoped `externally-satisfied` evidence to independent review.
  Product ownership, planning, development, archival, and integration must use
  their canonical evidence, preventing status from claiming satisfaction while
  their transition gates still block.
- Added negative parser and resolver coverage for unsupported planning omission,
  review without its gate, and external development evidence. Updated the
  instruction-layer reference to document the actual supported surface.
- `go test -count=1 ./...`, `go vet ./...`, `go build ./cmd/concoct`,
  `bash -n cmd/concoct/concoct.sh`, executable-bit validation, and
  `git diff --check` passed. Fresh initialization at
  `/tmp/concoct-con018-code7.MAjAMM/project` produced a staged, uncommitted Git
  project with policy, bootstrap, dotfiles, nested Codex skill, and planning
  artifacts while omitting executable-owned protocol and persona paths.

## Developer continuation: policy-selected completion handoff

- `concoct code` now renders an explicit policy-specific instruction and
  archival next action when independent review is already resolved as
  non-required or externally satisfied. Default-policy prompt goldens remain
  byte-compatible.
- `concoct code --complete` resolves workflow state before validating the final
  outgoing handoff. It continues to require a fresh `## Handoff to reviewer`
  and suggested review focus while review remains unresolved, but requires a
  fresh `## Handoff to archivist` and suggested archive focus when archival is
  the resolved next transition. Git freshness and non-Git artifact-level
  validation apply to either target.
- Added prompt coverage for the non-required review selection and a non-Git CLI
  completion case proving the Archivist handoff is accepted, status recommends
  archival, and review reservation remains forbidden.
- Updated command and state documentation. `go test -count=1 ./...`, `go vet
  ./...`, `go build ./cmd/concoct`, `bash -n cmd/concoct/concoct.sh`,
  executable-bit validation, and `git diff --check` passed.

## Developer continuation: external-evidence containment and attribution

- Hardened repository-relative policy evidence resolution by walking every path
  component with `Lstat`, rejecting symbolic links at any level, requiring
  intermediate directories and a readable final regular file, and comparing a
  normalized slash path by components instead of rejecting harmless filenames
  containing two adjacent dots.
- Invalid workflow state no longer renders task-scoped external evidence as an
  accepted disposition. This applies both to invalid evidence itself and to
  otherwise valid evidence attached to a structurally invalid task.
- Every resolved activity now carries and renders its requirement source as
  `.concoct/policy.md`; non-default role prompts include the same attribution.
- Added regression coverage for a symlinked parent escaping the repository, a
  benign `audit..md` filename, invalid-evidence suppression, invalid-task
  suppression, and source rendering. Updated instruction-layer and command
  documentation.
- The full Go suite, vet, build, shell syntax, executable-bit, and diff checks
  passed; affected workflow, prompt, and CLI packages passed again after the
  final invalid-state suppression refinement.

## Final development verification

- Added a cross-state matrix proving all six governed activities always expose
  a known requirement, non-empty reason, `.concoct/policy.md` attribution, and
  one of the five supported dispositions; the fixtures collectively reach
  `completed`, `not-required`, `not-applicable`, `externally-satisfied`, and
  `blocked`.
- Added recovery coverage proving `integrate --abort` can remove an existing
  prepared recovery record without composing subsequently invalid policy or
  discarding that current evidence. Starting a new integration still fails
  policy composition before mutation.
- Reconciled README, workflow documentation, and both root/template Codex skill
  adapters with policy-selected Reviewer-or-Archivist handoffs. Embedded
  default-policy prompt resources and their goldens remain byte-compatible.
- `go test -count=1 ./...`, `go vet ./...`, `go build ./cmd/concoct`,
  `bash -n cmd/concoct/concoct.sh`, executable-bit validation, root/template
  adapter equality, `git diff --check`, and the stale-claim search passed.
- Fresh initialization at `/tmp/concoct-con018-final.BRGard/project` created a
  staged, uncommitted Git repository containing project-owned policy,
  bootstrap, dotfiles, the updated nested Codex skill, and planning directories.
  Executable-owned protocol, persona, and prompt paths were absent.

## Review 01 remediation

- Finding 1 — fixed. `CompleteArchive` now runs the shared policy-evidence
  validator before treating external review as satisfied and before any
  archival mutation. It also applies the resolver's contradiction rule when
  immutable review evidence coexists with a non-required or externally
  satisfied review.
- Archive-level regression coverage exercises a missing reason, an unsafe
  repository-relative path, an unauthorized recorder, and contradictory
  immutable review evidence. Every case snapshots the fixture and proves the
  direct completion attempt leaves it unchanged.
- Remediation verification passed: `go test -count=1 ./...`, `go vet ./...`,
  `go build ./cmd/concoct`, `bash -n cmd/concoct/concoct.sh`, executable-mode
  validation, root/template adapter equality, and `git diff --check`.
- Fresh initialization at `/tmp/concoct-con018-remediation.VuOLGe/project`
  produced a Git repository with staged project-owned policy, bootstrap,
  dotfiles, nested Codex skill, and planning artifacts, no commit, and no
  executable-owned protocol or persona paths. The temporary fixture was
  removed after inspection.

## Handoff to reviewer

### Implemented

- Added a strict typed policy with a closed activity, gate, requirement, and Git
  strategy surface while preserving the accepted default lifecycle.
- Added one resolved activity record per governed activity with deterministic
  disposition, reason/evidence, and policy-source attribution.
- Made status, prompts, Developer completion, review reservation, archival, and
  managed integration agree on non-required or externally satisfied review.
- Added durable external-review evidence with role, reason, safe path, symlink,
  contradiction, and invalid-state enforcement.
- Closed Review 01's archive-completion bypass by validating that evidence and
  its review-history consistency before archival mutation.
- Updated documentation, default policy/template alignment, and agent guidance.

### Key decisions

- Kept product ownership, planning, development, archival, and integration
  mandatory because the current command/state contract has canonical evidence
  for those activities; independent review is the supported selectable phase.
- Allowed external satisfaction only for independent review. Other activities
  must complete their canonical transitions.
- Kept integration recovery available when current guidance is invalid only to
  preserve or unwind an existing transaction, never to authorize a new one.

### Files changed

- Policy composition and types under `internal/instruction`.
- Resolution, transition, archival, and evidence validation under
  `internal/workflow`; managed delivery checks under `internal/integration`.
- Policy-aware role rendering under `internal/prompt` and CLI transition tests.
- Root/template policy and Codex adapters, README, and normative documentation.
- Active CON-018 task plan and notes.
- Review remediation in `internal/workflow/archive.go` and
  `internal/workflow/archive_test.go`.

### Verification

- Focused archive/workflow tests and diff validation passed after remediation;
  full final verification is recorded in the latest remediation entry above.

### Known risks

- Policy validation remains structural; it cannot prove the semantic quality
  of human-authored reasons, external review evidence, or conflict resolutions.
- The supported selectable surface intentionally covers review variation only;
  expanding other activities requires a future command/state contract rather
  than silently accepting unsupported omissions.

### Skipped or unresolved work

- None within CON-018. Alternate Git strategies, profiles, task origins,
  adoption, upgrades, workflow explanation, and direct orchestration remain the
  explicit non-goals owned by adjacent roadmap items.

### Capability impact

- Updates CAP-001, CAP-005, CAP-006, CAP-007, CAP-008, CAP-009, CAP-010, and
  CAP-011 as declared in the task plan. Capability truth must be reconciled by
  the Archivist only after independent approval.

### Suggested review focus

- Confirm direct `archive --complete` validates external-review reason,
  recorder, safe evidence paths, and immutable-review contradictions before
  any mutation.
- Confirm the archival boundary now agrees with status and prompt resolution
  without changing approved-review or explicit-override behavior.
- Verify default-policy command behavior and prompt goldens remain unchanged,
  and integration recovery cannot become a new-transition bypass.
