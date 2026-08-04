---
id: CON-030
title: Make built-in workflow content executable-owned
roadmap-id: CON-030
status: implementation-complete
remediates-review: review-03.md
created: 2026-08-04
updated: 2026-08-04
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-030-make-built-in-workflow-content-executable-owned
  base: 93116c0f7d00533418e4e5b26071ba432b03489d
  status: archived
  archive-commit: self
capability-impact:
  type: update
  ids:
    - CAP-003
    - CAP-004
    - CAP-006
  rationale: Changes the installed project contract, adapter relationship to built-ins, and role-prompt rendering so executable-owned workflow resources are version-matched, immutable, inspectable, and no longer read from mutable repository copies.
---

# Task Plan

## Goal

Make prompts, personas, protocol-level workflow guidance, and other built-in
resources immutable contents of the Concoct executable, addressed through
stable logical identifiers and inspectable with `concoct defaults`, while
initialization materializes only project-owned guidance, configuration,
adapters, truth, and lifecycle state.

## Context

The executable currently embeds `all:templates`, and initialization copies
that entire tree into every project. Prompt rendering then reads handoff assets
from `.concoct/prompts/` in the target repository and instructs agents to read
repository-local persona files. Instruction composition also reads protocol,
policy, and project guidance from repository paths. This makes built-in behavior
editable per project and dependent on initialization time even though the
binary already carries the source distribution.

The clarified CON-030 product contract establishes the ownership boundary,
defines personas as built-ins, requires logical resource identifiers and a
`defaults list/show` inspection family, and explicitly excludes overlays,
legacy detection, migration automation, and compatibility machinery.

## Why this matters

Agent execution and future overlays need one authoritative, version-matched
definition of built-in behavior. Removing accidental repository-local forks
makes rendered guidance reproducible for a given executable and project state,
while retaining project truth and conventions as durable repository evidence.

## Current state

- `templates.go` embeds the complete `templates/` tree as one `embed.FS`.
- `internal/project.copyTemplates` walks and copies every embedded template,
  substituting the project name without an ownership filter.
- `internal/prompt.Render` selects handoff assets by repository path and reads
  them with `os.ReadFile`; personas are listed as repository inputs rather than
  composed into the prompt from executable resources.
- `internal/instruction.Compose` reads `.concoct/protocol.md`,
  `.concoct/policy.md`, and `AGENTS.md` from the project. Protocol is a built-in;
  policy selection and project guidance remain project-owned inputs.
- CLI dispatch has no `defaults` command family or executable provenance
  surface.
- Initialization tests require every source template path to be copied, and
  prompt tests use repository-local built-in fixtures plus full-output goldens.
- The planning inventory in `notes.md` classifies every currently installed
  template path and identifies mixed ownership boundaries that implementation
  must separate.

## Target state

- A focused built-in-resource boundary embeds readable Markdown and exposes a
  deterministic registry keyed by stable logical IDs rather than template paths.
- Prompt rendering obtains built-in handoffs and personas from that registry,
  composes them with validated project-owned evidence, and never falls back to
  matching repository files.
- Protocol-level built-in guidance is executable-owned; project-selected policy,
  `AGENTS.md`, roadmap, capabilities, configuration, adapters, and lifecycle
  state remain repository-owned.
- `concoct defaults list` deterministically reports logical ID, kind, and
  executable provenance; `concoct defaults show <logical-id>` emits exact source
  bytes with clear errors for unknown or missing resources.
- Initialization selectively materializes only project-owned outputs and
  produces a valid ready repository without installed prompt/persona/default
  copies.
- Existing projects continue to operate while repository-local old defaults are
  ignored. Documentation explains manual identification/removal during active
  development without introducing migration behavior.

## Design constraints

- Preserve protocol controls, workflow artifact ownership, role boundaries,
  deterministic rendering, and the distinction between rendered guidance and
  completed role work.
- Keep built-in source content readable Markdown in the source repository.
- Use logical IDs as the supported API; source-tree and Go package paths may be
  diagnostics only.
- Keep project policy, project guidance, adapters, roadmap/capability truth,
  configuration, current task state, and archive history repository-resident.
- Split mixed assets or generation concerns instead of treating the whole
  existing template tree as one ownership class.
- Ensure initialized and existing projects cannot shadow built-ins by retaining
  files at former installation paths.
- Preserve source wrapper executable mode and agent-neutral behavior.
- Keep source/template duplication only where the generated output is
  intentionally project-owned; update tests to enforce ownership rather than
  blanket source-tree parity.

## Non-goals

- No CON-014 project overlays or arbitrary built-in overrides.
- No legacy-default detection, automatic deletion, migration, upgrade command,
  compatibility framework, or stable-release promise.
- No CON-018 configurable workflow-policy implementation.
- No direct agent launching or orchestration.
- No redesign of roadmap, capability, task, review, archive, or integration
  schemas beyond what selective initialization requires.
- No removal of project-specific adapters whose operational purpose requires a
  repository-local entry point.

## Working assumptions

- The executable version/provenance provider may initially report a truthful
  development identity; CON-031 remains responsible for the full SemVer and
  release metadata contract.
- Project-selected `.concoct/policy.md` remains repository-owned, while the
  default policy skeleton/generator and protocol source are executable-owned.
- Empty API, code, and user writer personas remain built-in resources unless
  implementation proves they are examples with no runtime consumer; they must
  not be silently installed.
- Existing golden prompt behavior should change only where built-in source
  attribution/input paths necessarily change; all semantic role guidance and
  deterministic bytes remain covered.
- CAP-003, CAP-004, and CAP-006 limitations are compatible: optional empty
  personas can remain honestly limited, adapters remain non-executing, and
  semantic role judgment remains outside deterministic rendering.

## Risks and open questions

- Selective initialization can accidentally omit a project-owned file required
  by discovery, state validation, adapters, or bootstrap. End-to-end generated
  project tests must assert the positive and negative manifests explicitly.
- Personas and handoffs may contain repository-specific statements despite the
  intended generic boundary. Any discovered project truth must move to an
  appropriate project artifact without weakening the role contract.
- Prompt composition could duplicate persona or handoff guidance when switching
  from file references to embedded content. Golden review must check structure
  and attribution, not merely update bytes.
- Protocol is currently a required repository input to composition. Moving it
  into the executable must preserve structural validation of project policy and
  guidance and produce actionable errors without fallback.
- CON-031 has not yet defined stable release metadata. Defaults provenance must
  remain truthful for development binaries and avoid inventing a release API.
- No product decision remains unresolved. Package layout, registry type, exact
  development-version label, and internal grouping are Developer choices within
  the recorded contract.

## Implementation phases

### Phase 1 — Establish the resource and ownership model

Status: `complete`

- Confirm the planning inventory against every embedded template and runtime
  consumer; split mixed ownership concerns where required.
- Introduce a deterministic built-in registry with stable kind/name logical IDs,
  exact source bytes, and provenance metadata.
- Define required-resource lookup and integrity errors that identify the logical
  ID, operation, executable provenance, and corrective action.

### Phase 2 — Render workflow guidance from executable resources

Status: `complete`

- Replace repository-path reads for built-in handoffs, personas, and protocol
  guidance with required registry lookups.
- Compose built-ins with project-owned policy, guidance, capabilities, roadmap,
  and active-task evidence while preserving source attribution and all-or-nothing
  validation.
- Remove built-in files from exact project-input lists and ensure former local
  copies cannot shadow or alter rendering.

Remediation: complete. The selected runtime role now maps to and renders its
`persona-*` registry resource with executable attribution; embedded handoffs
and installed adapters no longer direct agents to absent repository persona
paths.

### Phase 3 — Add defaults inspection and provenance

Status: `complete`

- Add `concoct defaults list` with deterministic ID, kind, and provenance output.
- Add `concoct defaults show <logical-id>` with exact embedded source output and
  strict argument/unknown-resource diagnostics.
- Keep role-specific prompt commands as the project-aware inspection path and
  document the distinction from raw built-in inspection.

### Phase 4 — Make initialization ownership-aware

Status: `complete`

- Replace blanket template copying with an explicit project-output manifest or
  equivalent ownership-aware generation boundary.
- Continue generating project policy/configuration, guidance, adapters, roadmap,
  capabilities, current-state placeholders, archive structure, and bootstrap
  state while omitting built-in prompt/persona/protocol/default copies.
- Update discovery and initialization validation to match the new project
  contract and verify existing repositories ignore obsolete built-in copies.

### Phase 5 — Verify, document, and prepare review

Status: `complete`

- Add registry, lookup, missing-resource, defaults command, shadowing, prompt
  composition, and selective-initialization coverage.
- Update full-output prompt goldens deliberately and retain deterministic
  repeated-render checks.
- Document built-in/project ownership, inspection commands, development-stage
  handling of old installed defaults, and the exclusions for overlays and
  migration.
- Run repository-standard Go, wrapper, generated-project, parity/manifest,
  stale-path, and diff checks; record results and prepare a reviewer handoff.

Review-02 remediation: complete. Active documentation, generated adapters, and
executable-owned persona and policy resources now consistently describe
executable-rendered protocol and personas rather than uninstalled local paths.
Focused prompt tests, the full Go suite, vet, build, wrapper syntax, and a
fresh generated-project ownership check passed.

## Acceptance criteria

- Standard role prompts render without reading built-in prompt, persona,
  protocol, or general workflow files from the target repository.
- Repository files at former built-in paths cannot override, shadow, or serve as
  fallback content.
- Every formerly installed template path has an explicit ownership disposition,
  and mixed source/output ownership is represented without blanket copying.
- Stable logical IDs are unique, deterministic, source-path-independent, and
  suitable for later overlay targeting.
- `concoct defaults list` enumerates all built-ins with logical ID, kind, and
  truthful executable provenance in deterministic order.
- `concoct defaults show <logical-id>` prints exact embedded Markdown bytes;
  invalid arguments and unknown or missing resources fail clearly.
- Missing required resources report the logical ID, requiring operation,
  executable provenance, and integrity guidance without filesystem fallback.
- Newly initialized projects retain all required project-owned guidance,
  adapters, truth, configuration, and lifecycle directories but contain no
  installed built-in prompt, persona, protocol, or general-workflow copies.
- Existing repositories work with obsolete default copies present but unused;
  documentation explains optional manual cleanup without migration automation.
- Existing prompt commands remain deterministic, project-aware inspection
  surfaces and all golden tests pass after intentional attribution changes.
- No overlay, legacy detection, migration, compatibility, configurable-policy,
  or direct-agent-execution behavior is introduced.

## Verification

- Run `gofmt` on all changed Go files.
- Run `go test -count=1 ./...`, `go vet ./...`, and `go build ./cmd/concoct`.
- Run focused tests for registry uniqueness/order, exact-byte lookup, required
  resource failures, defaults CLI arguments/output/provenance, and no fallback.
- Run prompt rendering twice for every golden mode and compare exact output;
  include fixtures where conflicting files exist at former built-in paths.
- Run `bash -n cmd/concoct/concoct.sh` and verify the wrapper remains executable.
- Initialize a project under a temporary parent outside this repository; verify
  its explicit positive and negative ownership manifests, dotfiles, nested
  adapters, current/archive directories, Git repository, staged files,
  bootstrap prompt, no initial commit, and ready status.
- Run prompt commands from that generated project to prove the binary supplies
  built-ins absent from its filesystem.
- Search runtime source for direct reads of former built-in repository paths and
  search generated output for omitted defaults.
- Run `git diff --check` and inspect the complete task diff for scope and
  source/output ownership consistency.

## Capability impact

Expected impact is `update` for CAP-003, CAP-004, and CAP-006. CAP-003's
installed contract changes from a complete mutable workflow copy to selective
project-owned outputs backed by executable resources. CAP-004 adapters remain
repository-local but consume executable-built guidance. CAP-006 prompt rendering
becomes self-contained for built-ins and gains raw defaults inspection while
retaining human/agent semantic judgment. No new capability ID is expected.

## Handoff expectations

The Developer should begin by validating the inventory and setting the task to
`implementation-in-progress`. Keep resource, rendering, initialization, CLI,
documentation, and test changes focused on CON-030. Before review, record final
logical IDs, ownership dispositions, files changed, checks run, intentional
golden changes, known risks, skipped work, and capability-impact confirmation in
`notes.md`; set status to `implementation-complete`; and recommend
`concoct code --complete`.
