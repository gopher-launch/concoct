---
version: 1
project: concoct
updated: 2026-08-04
---

# Capabilities

## Purpose

This file is the canonical human-readable record of what Concoct can do now. It describes observable behavior evidenced by the repository, not planned behavior from the roadmap.

This initial inventory predates Concoct's normal reviewed-task archive history. Its entries cite repository evidence directly. A historical override archive preserves the stale pre-workflow task and the inventory findings, but does not constitute an approving review or roadmap delivery.

`CAP-NNN` identifiers are the stable canonical references used by roadmap and
workflow artifacts. Their semantic titles describe the enduring behavior;
`Added by`, `Updated by`, and archive fields record delivery provenance rather
than an ongoing dependency on a roadmap item.

## CAP-001 — Durable file-based workflow contract

- Status: `active`
- Audience: `developers and coding agents`
- Added by: `baseline inventory`
- Archive: `.concoct/archive/2026-07-29-legacy-hitl-restructuring/` — baseline evidence; no approving review
- Updated by: `.concoct/archive/2026-07-29-CON-003-command-contract-state-machine/`
- Updated by: `.concoct/archive/2026-07-31-CON-017-separate-protocol-policy-and-project-guidance/summary.md`
- Documentation: `.codex/skills/concoct/SKILL.md`, `doc/workflow.md`

### Capability

Concoct provides a Markdown-based workflow contract for moving substantial software work through product ownership, task planning, implementation, independent review, and archival. The contract separates non-overridable executable-owned protocol, project-selected workflow policy, repository-owned project guidance, and active task context with explicit source attribution and artifact ownership. It defines canonical roadmap, capability, active-task, review, and archive artifacts; supplies version-matched personas and handoffs while rendering role guidance; and provides an artifact-backed state machine covering valid transitions, remediation, blocked-review recovery, invalid evidence, and transactional archival.

### User value

Humans and agents can preserve product direction, task state, decisions, review evidence, and accepted outcomes in version-controlled files instead of relying on one tool's conversation history.

### Inputs

A repository whose participants follow `AGENTS.md`, the relevant Concoct artifacts,
and the executable-rendered role guidance.

### Outputs and effects

The workflow produces and maintains human-readable roadmap, task-plan, notes, sequential review, capability, and archive records. The files remain inspectable and usable without a Concoct service or database.

### Limitations

- Role outcomes are still produced by humans or agents following rendered
  guidance; the CLI validates durable evidence and selected transition
  boundaries but does not perform Product Owner, Planner, Developer, Reviewer,
  or Archivist judgment.
- Validation targets the checked-in artifact schemas and cannot establish the
  semantic correctness of human-authored plans, reviews, or conflict choices.

### Verification evidence

- `.codex/skills/concoct/SKILL.md` defines the canonical artifacts, role workflows, state discipline, review outcomes, and archive process.
- `templates/.concoct/personas/product-owner.md`, `task-planner.md`,
  `developer.md`, `reviewer.md`, and `archivist.md` are the readable Markdown
  sources for executable-owned role guidance.
- `.concoct/roadmap.md` and `.concoct/current/` demonstrate the living artifact layout in this repository.
- `doc/command-reference.md` defines the complete normative contract for the current command surface.
- `doc/state-machine.md` defines workflow state from observable artifacts and specifies transitions, review recovery, invalid states, and archive atomicity.
- `.concoct/archive/2026-07-29-CON-003-command-contract-state-machine/review-02.md` records approval of the command and state-machine contract.
- `.concoct/archive/2026-07-31-CON-017-separate-protocol-policy-and-project-guidance/review-02.md` records approval of layered ownership, deterministic composition, and structural conflict validation.

### Related capabilities

- `CAP-002` supplies reusable prompts for the workflow transitions.
- `CAP-003` packages the contract for use in another repository.
- `CAP-004` connects several coding-agent tools to the shared contract.

## CAP-002 — Reusable role-transition guidance

- Status: `active`
- Audience: `developers and coding agents`
- Added by: `baseline inventory`
- Archive: `.concoct/archive/2026-07-29-legacy-hitl-restructuring/` — baseline evidence; no approving review
- Updated by: `.concoct/archive/2026-08-04-CON-030-make-built-in-workflow-content-executable-owned/summary.md`
- Documentation: `README.md`, `doc/workflow.md`, `doc/command-reference.md`

### Capability

Concoct provides reusable, version-matched guidance for roadmap intake and for
handoffs from Product Owner to Task Planner, Task Planner to Developer,
Developer to Reviewer, Reviewer to Developer or Archivist, blocked Reviewer to
the responsible role, and Archivist back to Product Owner. The executable owns
these built-in resources and exposes their logical identifiers and exact bytes
through `concoct defaults list` and `concoct defaults show`.

### User value

Users can run the workflow with a human or agent while keeping role boundaries,
required inputs, allowed mutations, completion evidence, and next actions
explicit and matched to the installed executable.

### Inputs

The repository's current workflow artifacts and the executable resource matching
the desired transition.

### Outputs and effects

Each handoff tells the acting human or agent which inputs apply, which artifacts
the selected role may update, what outcome to produce, and which transition
should follow. CAP-006 selects and renders these resources with validated
repository context.

### Limitations

- Built-in resources are inspectable guidance, not autonomous role execution or
  proof that the requested work was completed.
- Former project-local protocol, persona, and handoff copies are ignored; Concoct
  does not migrate or delete them automatically.

### Verification evidence

- `internal/defaults/defaults.go` defines the stable built-in resource registry.
- `internal/defaults/defaults_test.go` verifies stable listing, reading, and
  unknown-resource diagnostics.
- `.concoct/archive/2026-08-04-CON-030-make-built-in-workflow-content-executable-owned/review-04.md`
  records approval of executable ownership and inspection behavior.

### Related capabilities

- `CAP-001` defines the artifact and role contract coordinated by these prompts.
- `CAP-006` renders selected manual prompt assets with validated workflow context.

## CAP-003 — Reusable project workflow template

- Status: `active`
- Audience: `project maintainers`
- Added by: `baseline inventory`
- Archive: `.concoct/archive/2026-07-29-legacy-hitl-restructuring/` — baseline evidence; no approving review
- Archive: `.concoct/archive/2026-08-04-CON-030-make-built-in-workflow-content-executable-owned/summary.md`
- Updated by: `.concoct/archive/2026-07-29-CON-005-go-cli-foundation/`
- Updated by: `.concoct/archive/2026-07-31-CON-017-separate-protocol-policy-and-project-guidance/summary.md`
- Documentation: `README.md`, `templates/AGENTS.md`

### Capability

Concoct supplies a reusable filesystem template for equipping another repository with project-owned guidance, workflow state, truth, configuration, and coding-agent adapters. Immutable protocol, personas, and transition guidance are supplied by the version-matched executable rather than installed as mutable runtime copies.

### User value

Project maintainers can reuse a consistent, agent-neutral workflow contract rather than assembling planning, review, and archival guidance from scratch.

### Inputs

The contents of `templates/` and project-specific edits to the installed placeholders and guidance.

### Outputs and effects

The initialized project contains conventional root files and tool adapters
alongside project-owned Concoct material under `.concoct/`, including policy,
configuration, `current/`, `archive/`, a roadmap, and a capability ledger.
Built-in protocol, personas, handoffs, and prompt documentation remain embedded
in the executable and are not installed as mutable project files.

### Limitations

- The API, code, and user writer persona files are empty.
- Repository conventions and product-specific guidance remain intentionally project-owned and may require project-specific customization.

### Verification evidence

- `templates/` contains the distributed source material for project-owned outputs
  and executable resources.
- `internal/project/project_test.go` verifies selective project-owned output,
  exclusion of built-in protocol/persona/handoff files, and real-Git
  initialization behavior.
- `.concoct/archive/2026-07-29-CON-005-go-cli-foundation/review-03.md` records
  approval of the original caller-directory-independent initialization,
  staging, and no-generated-commit foundation later narrowed by CON-030's
  accepted ownership boundary.

### Related capabilities

- `CAP-001` is the workflow contract represented by the template.
- `CAP-004` describes the tool adapters included in the template.
- `CAP-005` initializes projects from the embedded template and reports their workflow state.

## CAP-004 — Agent-neutral tool adapters

- Status: `active`
- Audience: `developers using coding agents`
- Added by: `baseline inventory`
- Archive: `.concoct/archive/2026-07-29-legacy-hitl-restructuring/` — baseline evidence; no approving review
- Updated by: `.concoct/archive/2026-07-31-CON-017-separate-protocol-policy-and-project-guidance/summary.md`
- Updated by: `.concoct/archive/2026-08-04-CON-030-make-built-in-workflow-content-executable-owned/summary.md`
- Documentation: `doc/multi-agent-workflow.md`

### Capability

Concoct provides thin adapters that direct Codex, Claude Code, GitHub Copilot, Aider, and tools that read a generic conventions file through the same executable-rendered protocol and personas, project policy and guidance, and active task context.

### User value

Teams can use the file-based workflow with multiple coding-agent tools without maintaining a separate durable rule set for each tool.

### Inputs

The adapter appropriate to the user's tool, the project-owned files installed in
the repository, and the version-matched executable guidance rendered for the
selected role.

### Outputs and effects

The adapters point tools to repository-owned `AGENTS.md`, project policy,
`.concoct/current/task-plan.md`, `.concoct/current/notes.md`, and the
executable-rendered protocol and role guidance where supported.

### Limitations

- Adapters provide instructions only; they do not launch agents or enforce workflow transitions.

### Verification evidence

- `templates/.codex/skills/concoct/SKILL.md`
- `templates/CLAUDE.md`
- `templates/.github/copilot-instructions.md` and `templates/.github/prompts/`
- `templates/.aider.conf.yml`
- `templates/CONVENTIONS.md`

### Related capabilities

- `CAP-001` provides the canonical workflow and artifact rules referenced by the adapters.
- `CAP-003` distributes the adapters with the project template.

## CAP-005 — Executable CLI initialization and workflow status

- Status: `active`
- Audience: `project maintainers, developers, and coding agents`
- Added by: `.concoct/archive/2026-07-29-CON-005-go-cli-foundation/`
- Updated by: `.concoct/archive/2026-08-04-CON-030-make-built-in-workflow-content-executable-owned/summary.md`
- Documentation: `README.md`, `doc/command-reference.md`, `doc/state-machine.md`

### Capability

Concoct provides a Go CLI with `init` and read-only `status` commands. It can
create a Concoct-enabled Git repository from the selectively installed,
project-owned portion of its embedded distribution and derive deterministic
workflow state from canonical repository artifacts.

### User value

Project maintainers can bootstrap the workflow reliably from an installed binary, and humans or agents can inspect the current task, review outcome, capability impact, diagnostics, and recommended next action without manually interpreting artifact combinations.

### Inputs

- `concoct init <project>` accepts one new project target whose parent already exists.
- `concoct status` runs from a Concoct project root or any nested directory.

### Outputs and effects

- `init` copies project-owned root files, dotfiles, adapters, policy, truth, and
  state; excludes built-in protocol, personas, handoffs, and prompt
  documentation; writes bootstrap guidance; initializes Git; stages generated
  files; and creates no commit.
- `status` discovers the project, validates roadmap, capability, task, notes, review, remediation, and blocked-review evidence, then reports the applicable state and next action without modifying the repository.
- Malformed, incomplete, contradictory, or representative interrupted-archive evidence is reported as `invalid` with actionable diagnostics.

### Limitations

- This capability's `status` command remains read-only. CAP-006 adds
  state-preserving prompt rendering, and CAP-007 adds the explicitly bounded
  Git planning and integration mutations.
- Remediation disposition validation is textual because the notes schema does not define structured review-finding identifiers.
- Metadata parsing intentionally targets the checked-in Concoct schemas rather than arbitrary Markdown documents.

### Verification evidence

- `internal/project/project_test.go` covers discovery, template copying, Git behavior, initialization safety, and read-only inspection.
- `internal/workflow/workflow_test.go` covers normative states, metadata validation, sequential reviews, remediation, blocked-review recovery, composed recovery precedence, and invalid evidence.
- `.concoct/archive/2026-07-29-CON-005-go-cli-foundation/review-03.md` records the approving independent review and end-to-end verification.

### Related capabilities

- `CAP-001` defines the workflow and state contract implemented by status detection.
- `CAP-003` supplies the embedded project template used by initialization.
- `CAP-004` supplies the agent adapters installed with that template.
- `CAP-006` adds deterministic role-prompt rendering to the CLI.

## CAP-006 — Deterministic role-aware prompt rendering

- Status: `active`
- Audience: `developers and coding agents`
- Added by: `.concoct/archive/2026-07-30-CON-006-deterministic-prompt-rendering/`
- Updated by: `.concoct/archive/2026-07-30-CON-015-isolate-integrate-git-tasks/`
- Updated by: `.concoct/archive/2026-07-31-CON-008-implement-code-and-review-transitions/`
- Updated by: `.concoct/archive/2026-07-31-CON-017-separate-protocol-policy-and-project-guidance/summary.md`
- Updated by: `.concoct/archive/2026-08-04-CON-030-make-built-in-workflow-content-executable-owned/summary.md`
- Documentation: `README.md`, `doc/command-reference.md`, `doc/state-machine.md`

### Capability

Concoct can render complete, deterministic prompts for Product Owner roadmap
intake, task planning, development, independent review, and accepted archival
from validated repository state. Rendering first composes attributed protocol,
policy, and project-guidance sources in deterministic order, rejects malformed,
unsupported, conflicting, or invariant-weakening structural declarations without
partial output, and preserves project-guidance bytes. It then selects the applicable persona and
workflow mode, including implementation continuation, changes-requested
remediation, blocked-review recovery routes, and Git-aware archival guidance.
Built-in protocol, personas, and handoffs come from an inspectable executable
resource registry with truthful provenance; `defaults list` and `defaults show`
expose those resources, and local former copies cannot shadow them.

### User value

Humans and coding agents can obtain the correct inspectable role handoff
without manually interpreting workflow state or duplicating durable role
rules.

### Inputs

- `concoct roadmap` renders Product Owner roadmap guidance from the ready state.
- `concoct plan <roadmap-id>` validates an eligible item and satisfied
  dependencies before rendering Task Planner guidance.
- `concoct code` renders the applicable initial, continuing, remediation, or
  blocked-recovery Developer guidance.
- `concoct review` renders Reviewer guidance with prior reviews and the next
  sequential review path.
- `concoct archive` renders Archivist guidance from approved state, preserving
  the distinct Git-backed and non-Git completion paths.
- Each command accepts optional create-only `--output <path>` output.

### Outputs and effects

- Commands write deterministic prompt bytes to stdout by default or create a
  new output file containing identical bytes.
- Rendered prompts identify exact inputs, authorized updates, detected state
  and mode, expected outcome, validation requirements, and next transition.
- Rendering validates command eligibility and workflow evidence but does not
  mutate workflow state, persist role outcomes, or launch an agent.

### Limitations

- Prompt commands provide guidance only; CAP-010 provides separate validated
  Developer and Reviewer completion boundaries, and CAP-011 provides the
  separate validated archival completion boundary. Direct agent execution
  remains future work.
- Output files are create-only and existing destinations are never overwritten.
- Archive-summary relevance is selected conservatively from identifiers in
  validated task and command context because no archive index exists.
- Free-form prose is source-attributed but cannot be checked exhaustively for
  semantic contradictions; suspected prose conflicts require human reconciliation.

### Verification evidence

- `internal/prompt/render_test.go` and its ten golden fixtures cover all five
  role commands and the materially distinct development, review, and archival
  modes.
- `internal/cli/cli_test.go` covers stdout/file byte equality, nested project
  discovery, workflow non-mutation, argument errors, and collision refusal.
- `.concoct/archive/2026-07-30-CON-006-deterministic-prompt-rendering/review-02.md`
  records approval after full-output golden coverage was added.

### Related capabilities

- `CAP-001` defines the workflow and state contract used for eligibility.
- `CAP-002` supplies the canonical manual prompt assets appended to rendering.
- `CAP-005` supplies CLI project discovery and validated state detection.
- `CAP-010` and `CAP-011` provide the explicit completion boundaries for the
  Developer, Reviewer, and Archivist work prepared by these prompts.

## CAP-007 — Git-backed task isolation and integration

- Status: `active`
- Audience: `developers and coding agents`
- Added by: `.concoct/archive/2026-07-30-CON-015-isolate-integrate-git-tasks/`
- Documentation: `README.md`, `doc/command-reference.md`, `doc/state-machine.md`, `doc/workflow.md`

### Capability

Concoct can isolate a substantial task on a deterministic Git branch derived
from its roadmap identity, preserve the exact integration trunk and task base,
and validate repository identity and cleanliness at role boundaries. After
approval, it supports task-branch archival followed by local squash integration
into the recorded trunk, final delivery bookkeeping, and accepted branch
cleanup. Non-Git projects retain the unbranched workflow.

### User value

Task implementation and review remain separate from the user's trunk until
accepted work is archived and explicitly integrated. Durable identity and
recovery evidence make interrupted or conflicted integration inspectable and
recoverable without requiring a hosting provider or remote.

### Inputs

- Git-backed planning starts from a clean, attached branch with unambiguous
  history and no conflicting task branch or repository operation.
- Active task metadata records the integration trunk, task branch, immutable
  base, and later archival and integration evidence.
- Approved archival supplies validated task artifacts, capability
  reconciliation, and pending roadmap evidence.
- Non-Git projects use the existing active task and review artifacts without
  fabricated Git metadata.

### Outputs and effects

- Planning creates and checks out a deterministic task branch and records exact
  trunk/base identity before implementation.
- Development, remediation, review, and Git archival reject checkout drift,
  contradictory metadata, dirty boundaries, and unrelated Git operations.
- `concoct archive` renders Archivist guidance, and `concoct archive --complete`
  validates and commits the authored Git-backed archival transition. It
  preserves current state, records pending delivery, and ends at `archived`.
- `concoct integrate`, `concoct integrate --continue`, and
  `concoct integrate --abort` perform or recover the recorded local squash
  transaction with exact commit, operation, worktree, and path guards.
- Successful integration records delivery on the exact trunk, clears active
  state, removes local recovery evidence, deletes the accepted task branch,
  and returns to `ready`.
- A matching upstream is pushed only after confirmation by default or explicit
  automatic-push configuration; no remote is required for local delivery.

### Limitations

- Semantic conflict choices remain human-attested; Concoct validates transaction
  structure and boundaries, not the meaning of conflict resolutions.
- Provider and pull-request integration, worktree concurrency, stacked or
  concurrent tasks, and automatic push without opt-in are not supported.
- The CLI renders role prompts but does not directly execute planning,
  development, review, or archival persona work.

### Verification evidence

- `internal/gitrepo/git_test.go` covers repository inspection, branch naming,
  status parsing, changed-path boundaries, and Git operations.
- `internal/integration/integration_test.go` uses real repositories to cover
  local success, conflict continuation, exact abort, interruption recovery,
  unsafe work refusal, branch cleanup, delivery, and upstream policy.
- CLI, prompt, and workflow tests cover Git planning, state precedence,
  archival guidance, identity validation, non-Git fallback, and recovery.
- `.concoct/archive/2026-07-30-CON-015-isolate-integrate-git-tasks/review-03.md`
  records final approval after destructive-recovery remediation.

### Related capabilities

- `CAP-001` defines the workflow and state contract extended by Git lifecycle
  states and evidence.
- `CAP-005` supplies project discovery and validated state reporting.
- `CAP-006` supplies deterministic role prompts, now including archival and
  Git-aware transition guidance.
- `CAP-011` supplies the validated archival completion boundary whose accepted
  Git evidence is consumed by integration.

## CAP-008 — Validated active task planning

- Status: `active`
- Audience: `developers and coding agents`
- Added by: `.concoct/archive/2026-07-30-CON-007-active-task-planning/`
- Documentation: `README.md`, `doc/command-reference.md`, `doc/state-machine.md`

### Capability

Concoct can validate one selected planned roadmap item against its unresolved
roadmap dependencies and declared accepted capability prerequisites before
starting a Task Planner session. It supplies deterministic prerequisite,
limitation, and archive-provenance context while preserving the Task Planner's
responsibility for semantic compatibility and implementation-ready artifacts.

### User value

Humans and coding agents can begin planning with explicit evidence that the
selected roadmap work is structurally eligible, without silently accepting
missing or inactive capabilities, overwriting active work, or mistaking prompt
rendering for a completed planning transition.

### Inputs

- `concoct plan <roadmap-id>` selects exactly one parseable item whose status is
  `planned` and whose outstanding roadmap dependencies are satisfied.
- Every declared capability prerequisite must resolve uniquely to an `active`
  record in `.concoct/capabilities.md`.
- Documented capability limitations and conventional archive-summary
  provenance are included for Task Planner readiness judgment.
- Existing active artifacts, ambiguous placeholders, reviews, and unsafe Git
  boundaries remain conflicts rather than overwrite targets.

### Outputs and effects

- Ineligible, malformed, missing, duplicate, or inactive prerequisite evidence
  fails with item- and capability-specific recovery guidance before Git branch
  creation or prompt output.
- Eligible planning renders deterministic Task Planner context and, in Git
  repositories, preserves the established deterministic task-branch boundary.
- The Task Planner may create absent artifacts or replace only recognized empty
  placeholders, validate the plan and notes as a pair, and activate only the
  selected roadmap item.
- Valid completed artifacts preserve roadmap identity, capability impact, and
  exact Git metadata; `concoct status` then reports `planned` and recommends
  `concoct code`.

### Limitations

- The CLI does not decide whether documented capability limitations are
  semantically compatible with product intent; that remains Task Planner
  judgment.
- Prompt rendering does not author a plan, mutate role-owned artifacts, or by
  itself establish the `planned` state.
- Capability parsing intentionally follows the checked-in Markdown heading and
  status conventions; schema changes must update the parser and tests.

### Verification evidence

- `internal/workflow/workflow_test.go` covers prerequisite parsing and accepted,
  missing, inactive, duplicate, malformed, and limitation-bearing records.
- CLI and Git tests verify prerequisite failures occur before branch creation
  and preserve existing collision, dirty-worktree, detached-HEAD, and rollback
  protections.
- Prompt tests and golden output cover deterministic prerequisite, limitation,
  and archive-provenance context.
- `.concoct/archive/2026-07-30-CON-007-active-task-planning/review-02.md`
  records approval after remediation of planning-diagnostic context.

### Related capabilities

- `CAP-001` defines the durable workflow and state contract.
- `CAP-005` supplies executable project discovery and workflow validation.
- `CAP-006` supplies deterministic Task Planner prompt rendering.
- `CAP-007` supplies the optional Git task-isolation and integration lifecycle.

## CAP-009 — Evidence-backed next-action recommendation

- Status: `active`
- Audience: `project maintainers, developers, and coding agents`
- Added by: `.concoct/archive/2026-07-30-CON-028-recommend-next-project-action/`
- Documentation: `README.md`, `doc/command-reference.md`, `doc/state-machine.md`

### Capability

Concoct provides `concoct next` as the single command recommended from valid
ready state. It renders a deterministic Product Owner prompt from validated
roadmap, capability, dependency, prerequisite, archive, and supported-origin
evidence so a human or agent can recommend one valid next action without the
CLI selecting work, creating a task, or changing lifecycle evidence.

### User value

Users returning to a ready project receive one unambiguous decision step that
distinguishes structurally plannable work, supported product or roadmap intake,
specific blockers, and the absence of actionable recorded work before routing
to an existing workflow command.

### Inputs

- `concoct next` runs from the project root or a nested directory only when the
  repository is in structurally valid `ready` state.
- It reads canonical roadmap and accepted capability records, shared planning
  eligibility, dependency and prerequisite evidence, relevant archive
  provenance, and the bounded set of currently supported work origins.
- Optional `--output <path>` uses the existing create-only prompt-output
  contract.

### Outputs and effects

- The command emits byte-deterministic Product Owner guidance to stdout or a
  newly created output file.
- Guidance requires exactly one evidence-backed outcome: plan an eligible item,
  perform supported product or roadmap work, resolve a named blocker or
  inconsistency, or report that no actionable work is recorded.
- Ready-state status, initialization, bootstrap, and successful integration
  consistently recommend `concoct next`.
- Rendering is read-only and does not rank, select, activate, persist, or
  otherwise mutate workflow work.

### Limitations

- Semantic prioritization and recommendation remain Product Owner judgment;
  deterministic presentation order is not automated selection.
- Only roadmap planning and human product input or roadmap maintenance are
  supported work origins until other origin contracts are accepted.
- Invalid or contradictory canonical evidence is rejected rather than
  normalized into a recommendation.

### Verification evidence

- `internal/workflow/workflow_test.go` covers shared eligibility and blocker
  evidence.
- `internal/prompt/render_test.go` and the `next*.golden` fixtures cover every
  supported recommendation outcome, determinism, invalid evidence, and
  non-mutation.
- CLI, initialization, and integration tests cover state restriction, nested
  discovery, create-only output safety, and the sole ready-state recommendation.
- `.concoct/archive/2026-07-30-CON-028-recommend-next-project-action/review-02.md`
  records approval after the missing outcome and transition coverage was added.

### Related capabilities

- `CAP-001` defines the durable role and artifact contract.
- `CAP-005` supplies project discovery and validated workflow state.
- `CAP-006` supplies deterministic role-aware prompt rendering.
- `CAP-008` supplies the shared structural planning-eligibility authority.

## CAP-010 — Validated Developer and Reviewer completion

- Status: `active`
- Audience: `developers and coding agents`
- Added by: `.concoct/archive/2026-07-31-CON-008-implement-code-and-review-transitions/`
- Documentation: `README.md`, `doc/command-reference.md`, `doc/state-machine.md`

### Capability

Concoct provides explicit, validated completion boundaries for Developer and
Reviewer work while preserving ordinary `concoct code` and `concoct review`
invocations as deterministic, read-only prompt rendering. It validates durable
role-owned evidence through initial implementation, continuation, remediation,
blocked-review recovery, sequential review reservation, and review
finalization.

### User value

Humans and coding agents can advance the implementation and review loop through
inspectable artifacts without false completion, overwritten reviews, stale
handoffs, unsafe Git context, or manufactured role judgment.

### Inputs

- `concoct code --complete` accepts Developer-authored task-plan, notes,
  implementation, test, and documentation changes from an eligible mode.
- `concoct review --reserve` claims the exact next zero-padded review path from
  implementation-complete state, and `concoct review --complete` accepts the
  Reviewer-authored result at that reserved path.
- Git-backed tasks require their recorded branch and base, a clean entry
  boundary, and no unrelated Git operation. Non-Git tasks use the same artifact
  contract without commit evidence.

### Outputs and effects

- Developer completion validates task status, mode-specific metadata, notes,
  remediation dispositions where required, a complete reviewer handoff, review
  immutability, allowed paths, and the resulting workflow state.
- A valid review reservation is create-only and non-authoritative until the
  same artifact is finalized with matching task, sequence, and persona metadata
  plus exactly one supported outcome.
- Completed reviews remain append-only, later review numbers are contiguous,
  and the recorded outcome determines whether status recommends archive,
  remediation, or blocker routing.
- Complete Git-backed transitions are committed once and exact clean retries
  reuse valid transition commits; invalid or interrupted work is preserved with
  actionable recovery guidance.

### Limitations

- The CLI validates structural evidence and ownership boundaries but does not
  implement code, write findings or dispositions, choose a review outcome, or
  judge the semantic quality of role-authored content.
- Non-Git repositories cannot prove changed paths or handoff freshness against
  committed history, so they use resulting-artifact and handoff-completeness
  validation.
- Review reservation recovery is deliberately manual; malformed or abandoned
  evidence is preserved rather than silently repaired or deleted.

### Verification evidence

- `internal/workflow/transition.go` implements Developer completion and
  Reviewer reservation/finalization validation.
- `internal/cli/transition_test.go` covers the complete loop, all review
  outcomes, remediation, blocked recovery, ownership failures, reservation
  collisions, Git boundaries and retries, stale handoffs, and non-Git parity.
- `.concoct/archive/2026-07-31-CON-008-implement-code-and-review-transitions/review-02.md`
  records approval after focused boundary and handoff remediation.

### Related capabilities

- `CAP-001` defines the durable workflow and role-ownership contract.
- `CAP-005` supplies project discovery and validated workflow status.
- `CAP-006` supplies the state-preserving role prompts retained by these
  completion commands.
- `CAP-007` supplies the Git task identity and transition boundaries.
- `CAP-008` supplies the validated active task from which implementation begins.

## Known capability gaps

- Role commands render prompts and validate explicit Developer and Reviewer
  completion boundaries, but they do not directly execute persona work or
  treat rendered guidance as role completion.
- Direct agent execution, workflow diagnostics, recovery, history reporting,
  upgrades, and overlays remain roadmap intent rather than current capabilities.
## CAP-011 — Validated archive and capability reconciliation

- Status: `active`
- Audience: `project maintainers, developers, and coding agents`
- Added by: `.concoct/archive/2026-07-31-CON-009-implement-archive-and-capability-reconciliation/summary.md`
- Archive: `.concoct/archive/2026-07-31-CON-009-implement-archive-and-capability-reconciliation/summary.md`
- Documentation: `README.md`, `doc/command-reference.md`, `doc/state-machine.md`, `doc/workflow.md`

### Capability

Concoct provides an explicit `archive --complete` boundary that validates
Archivist-authored archive, summary, capability, roadmap, review, and task
evidence before accepting an archival transition. Ordinary `archive` remains
read-only role guidance, and exceptional completion requires matching durable
authority and reason evidence.

### User value

Maintainers can turn approved work into traceable product capability truth with
structural safeguards against incomplete, contradictory, unrelated, or
premature archival changes.

### Inputs

An implementation-complete task with accepted sequential reviews, a
deterministically named archive candidate, an authored summary, declared
capability reconciliation, and lifecycle-appropriate roadmap and Git evidence.

### Outputs and effects

- Git-backed archival validates branch, base, operation, path, capability, and
  roadmap boundaries; commits the accepted evidence once; records exact archival
  HEAD through the non-recursive `self` sentinel; retains current task evidence;
  and leaves delivery pending for integration.
- Non-Git archival validates complete delivered evidence before clearing current
  state and returning the project to `ready`.
- Valid clean Git retries reuse only an exact, fully revalidated archival
  transition, while invalid or interrupted attempts preserve durable evidence
  with actionable failures.

### Limitations

- The Archivist remains responsible for semantic summary and capability wording;
  the CLI validates structure, declared scope, provenance, and transaction
  invariants rather than prose quality.
- Non-Git repositories lack a committed pre-transition baseline, so preservation
  relies on complete structural validation and cleanup-last ordering.
- Git-backed delivery, squash integration, current cleanup, and accepted branch
  deletion remain owned by the integration lifecycle in CAP-007.

### Verification evidence

- `internal/workflow/archive.go` validates archive candidates, lifecycle evidence,
  capability impact, roadmap reconciliation, safe Git transitions, and retries.
- `internal/workflow/archive_test.go` covers archive schemas, exact destinations,
  all capability-impact types, unrelated-content preservation, and failure
  preservation.
- `internal/cli/transition_test.go` covers Git archival boundaries, corrupted
  retries, pending delivery, and successful integration from produced evidence.
- `.concoct/archive/2026-07-31-CON-009-implement-archive-and-capability-reconciliation/review-03.md`
  records independent approval after two remediation rounds.

### Related capabilities

- `CAP-001` defines the durable workflow and acceptance roles.
- `CAP-005` provides workflow state detection used to confirm archival outcomes.
- `CAP-007` owns Git-backed integration and final delivery after archival.
- `CAP-010` supplies validated implementation and review evidence consumed by
  archive completion.
