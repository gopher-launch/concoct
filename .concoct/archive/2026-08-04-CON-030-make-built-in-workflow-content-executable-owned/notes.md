# Notes

## Planning summary

CON-030 is implementation-ready. It is active with no roadmap dependencies;
CAP-003, CAP-004, and CAP-006 are active prerequisites whose documented
limitations remain compatible. The Product Owner resolved the prior ownership,
persona, inspection, provenance, and legacy-repository decisions in the roadmap.

## Confirmed findings

- The exact Git identity is trunk `main`, task branch
  `concoct/con-030-make-built-in-workflow-content-executable-owned`, and base
  `93116c0f7d00533418e4e5b26071ba432b03489d`.
- Before active artifacts were created, `.concoct/current/` contained only
  `.gitkeep`; no active task conflicted with CON-030.
- `templates.go` embeds `all:templates`; `internal/project.copyTemplates` walks
  and materializes every file beneath it.
- Prompt role selection stores repository paths for handoffs and personas.
  `internal/prompt.Render` reads the selected handoff with `os.ReadFile` and
  exposes personas as exact project inputs.
- `internal/instruction.Compose` reads protocol, policy, and `AGENTS.md` from the
  project. The clarified ownership model makes protocol executable-owned while
  policy selection and project guidance remain project-owned.
- CLI command parsing is centralized in `internal/cli/cli.go`; no defaults or
  general embedded-resource inspection command currently exists.
- Current initialization and prompt tests encode blanket-copy and
  repository-local-built-in assumptions, so they require deliberate contract
  updates rather than mechanical fixture regeneration.

## Initialization file inventory

Every file currently materialized from `templates/` is classified below. The
source template or generator remains executable-owned even where its generated
output is project-owned.

### Executable built-ins — embed and stop installing

- Protocol/general workflow: `.concoct/protocol.md`.
- Built-in personas: `.concoct/personas/api-writer.md`, `archivist.md`,
  `code-writer.md`, `developer.md`, `product-owner.md`, `reviewer.md`,
  `task-planner.md`, and `user-writer.md`.
- Built-in prompt documentation and handoffs: `.concoct/prompts/README.md`, all
  files under `.concoct/prompts/handoffs/`, and all files under
  `.concoct/prompts/roadmap/`.

### Project-owned generated truth, guidance, configuration, and state — continue installing

- Project guidance/conventions: `AGENTS.md` and `CONVENTIONS.md`.
- Selected workflow configuration: `.concoct/policy.md`.
- Product truth and direction: `.concoct/capabilities.md` and
  `.concoct/roadmap.md`.
- Lifecycle state skeletons: `.concoct/current/task-plan.md` and
  `.concoct/current/notes.md`; initialization also creates archive/current
  directories and bootstrap state.

### Repository-local agent adapters — continue installing

- Codex adapter: `.codex/skills/concoct/SKILL.md`.
- Claude adapter: `CLAUDE.md`.
- Aider adapter: `.aider.conf.yml`.
- Copilot adapters: `.github/copilot-instructions.md` and every file under
  `.github/prompts/`.

These adapters must become thin references to executable-built guidance plus
project-owned context; they must not duplicate built-in personas or handoffs as
durable authority.

### Examples and samples

- No separately classified example/sample file is currently installed. Prompt
  README content is built-in documentation and should be inspectable but not
  materialized.

## Planning decisions

- Use stable logical IDs by resource purpose and kind, not source path. The
  registry must cover every runtime built-in and permit deterministic listing;
  exact final names may be refined locally while preserving roadmap examples
  and future CON-014 targetability.
- Treat project policy as project-owned selected configuration, but keep its
  default source/generation logic executable-owned. Treat protocol as an
  immutable built-in input to composition.
- Render generic persona and handoff content into complete prompts from the
  executable. Exact-input lists should name only repository evidence the acting
  agent must still read.
- Update initialization tests from blanket template parity to explicit positive
  project-output and negative built-in manifests.
- Update CAP-003, CAP-004, and CAP-006 after acceptance; do not create a new
  capability merely for the internal registry or inspection subcommands.

## Risks

- Adapter files may currently repeat workflow rules and need thinning without
  losing the entry point required by each external tool.
- Empty writer personas are accepted CAP-003 limitations. Their disposition
  must remain explicit rather than silently dropping or presenting them as
  complete behavior.
- Removing repository protocol copies changes project discovery/composition
  assumptions and could make initialized repositories invalid unless all
  validators use the same ownership model.
- A development executable lacks CON-031 release metadata. Provenance must be
  honest and deterministic without claiming a stable version contract.
- Existing project files at old built-in paths must be ignored, not deleted,
  migrated, or interpreted as overlays.

## Initial verification

- Read the effective protocol, policy, project guidance, Task Planner persona,
  capability prerequisites, selected roadmap item, and all archive summaries
  named by the rendered prompt.
- Inspected the embed boundary, initialization copier/tests, prompt renderer and
  goldens, instruction composition, CLI dispatch, workflow metadata validation,
  documentation surfaces, and every current template path.
- Confirmed the recorded task branch and base and confirmed no conflicting
  active artifacts.

## Developer handoff

### Current state

Planning complete; task status is `planned` on the recorded task branch.

### Completed

- Validated eligibility, prerequisite compatibility, exact Git identity, and
  the clarified Product Owner decisions.
- Classified every currently installed template path by runtime/output
  ownership.
- Grounded implementation phases, acceptance criteria, verification, capability
  impact, and exclusions in current code and tests.

### Remaining

- Implement the built-in registry and integrity boundary.
- Move prompt/persona/protocol consumption to executable resources.
- Add defaults inspection, selective initialization, documentation, and tests.
- Record implementation evidence and prepare independent review.

### Known risks

Selective initialization completeness, adapter duplication, composition
validity without repository protocol copies, development provenance, and
accidental fallback to obsolete local defaults.

### Commands run

Repository, roadmap, capability, archive, template, source, test, documentation,
Git branch/base, and workflow-schema inspection; no implementation checks were
run during planning.

### Suggested next step

`concoct code`

## Implementation record

- Added the executable-owned `internal/defaults` registry with stable logical
  IDs, deterministic listing, exact-byte reads, development provenance, and
  integrity diagnostics.
- Instruction composition now obtains protocol from `built-in:protocol`; prompt
  rendering obtains handoffs from the registry and does not treat local
  persona, handoff, prompt, or protocol copies as inputs.
- Added `concoct defaults list` and `concoct defaults show <logical-id>`.
- Initialization now uses an explicit ownership filter: policy, guidance,
  adapters, capabilities, roadmap, and current state are generated; protocol,
  personas, and prompts are not. Empty former directories are omitted too.
- Updated prompt goldens deliberately for executable attribution and complete
  embedded handoff text, plus project and defaults tests and ownership docs.

## Verification results

- `go test -count=1 ./...` passed.
- `go vet ./...` passed.
- `go build ./cmd/concoct` passed.
- `bash -n cmd/concoct/concoct.sh` passed.
- A temporary initialized project contained the positive project-owned manifest,
  no built-in protocol/persona/prompt paths, a Git repository with staged files,
  and rendered `next` successfully using executable-built guidance.
- `git diff --check` passed.

## Handoff to reviewer

Implementation is complete for CON-030. Review `internal/defaults/defaults.go`,
the instruction/prompt ownership boundary, the initialization filter, and the
new defaults CLI command. Confirm generated projects cannot shadow built-ins,
and that the intentionally changed prompt goldens accurately reflect source
attribution. No migration, overlay, configurable-policy, or direct-agent
execution behavior was added. Capability impact remains the planned update to
CAP-003, CAP-004, and CAP-006.

## Review-01 remediation

### Finding: rendered prompts omit the executable-owned persona

- Disposition: fixed.
- `internal/prompt.Render` now derives `persona-<role>` from the selected role,
  reads it through `defaults.Read`, and embeds the exact Markdown with explicit
  executable provenance. Repository-local persona files remain neither inputs
  nor fallbacks.
- The embedded handoffs and installed Claude/CONVENTIONS adapters now refer to
  the persona rendered in the prompt rather than an absent project path.
- `internal/prompt/render_test.go` preserves exact rendered-output comparisons
  while composing the deliberately added embedded-persona section into the
  expected fixture. It also verifies every runtime role and proves a malicious
  local persona copy cannot shadow the executable resource.

### Remediation verification

- `go test -count=1 ./internal/prompt` passed.
- `go test -count=1 ./...` passed.
- `go vet ./...` passed.
- `go build ./cmd/concoct` passed.
- `bash -n cmd/concoct/concoct.sh` passed.
- `git diff --check` passed.

## Handoff to reviewer

### Implemented

The Review-01 persona-composition finding is fixed. The renderer loads the
selected `persona-*` resource through the executable-owned registry and embeds
it with provenance. Embedded handoffs and generated adapters no longer direct
agents to absent local persona files.

### Verification

- `go test -count=1 ./internal/prompt` passed.
- `go test -count=1 ./...` passed.
- `go vet ./...` passed.
- `go build ./cmd/concoct` passed.
- `bash -n cmd/concoct/concoct.sh` passed.
- `git diff --check` passed.

### Known risks

Persona source Markdown makes rendered prompts longer by design. The registry
continues to be the sole runtime source, and focused tests cover every selected
runtime role plus a malicious local shadow copy.

### Capability impact

No capability-ledger update is authorized before review and archive. The planned
updates to CAP-003, CAP-004, and CAP-006 remain accurate; no new capability,
migration, overlay, or policy-configurability behavior was introduced.

### Suggested review focus

Review `internal/prompt/render.go` for required-resource lookup and provenance,
`internal/prompt/render_test.go` for each role mapping and shadowing coverage,
and `templates/.concoct/prompts/` for self-contained handoff directions.

## Review-02 remediation

### Finding: normative documentation still requires uninstalled persona files

- Disposition: fixed.
- `doc/command-reference.md` now treats each command's selected persona as
  executable-rendered content rather than a repository file.
- `doc/workflow.md`, `doc/multi-agent-workflow.md`, `doc/state-machine.md`, and
  repository guidance distinguish installed project state from executable-owned
  protocol, personas, and handoffs. Writer references now use the registry's
  actual `persona-*-writer` logical IDs.
- Generated Codex and Copilot adapters, Copilot task prompts, and Claude and
  generic-conventions adapters direct users to role commands and their rendered
  built-ins instead of absent protocol or persona paths.
- The executable-owned default policy and persona resources no longer instruct
  a rendered role to reread its uninstalled source file.

### Remediation verification

- `go test -count=1 ./internal/prompt` passed.
- `go test -count=1 ./...` passed.
- `go vet ./...` passed.
- `go build ./cmd/concoct` passed.
- `bash -n cmd/concoct/concoct.sh` passed and the wrapper remains executable.
- A fresh generated project had the required project-owned manifest, staged Git
  files, and no installed protocol, persona, or prompt directories; `concoct
  next` rendered the Product Owner persona without a stale local-persona read.
- Active-documentation and adapter stale-path search found only intentional
  source-tree or explicitly not-installed references; registry, runtime, tests,
  and historical evidence retain paths as implementation detail.
- `git diff --check` passed.

## Handoff to reviewer

### Implemented

The Review-02 documentation finding is fixed. All active operational guidance
now describes the built-in protocol and selected persona as executable-owned
rendered prompt content. Generated adapters no longer require the deliberately
omitted `.concoct/personas/` or `.concoct/protocol.md` files.

### Verification

- `go test -count=1 ./...` passed.
- `go vet ./...` passed.
- `go build ./cmd/concoct` passed.
- `bash -n cmd/concoct/concoct.sh` passed; the wrapper remains executable.
- Fresh-project initialization, ownership-manifest, staged-Git, and rendered
  Product Owner persona checks passed.
- `git diff --check` passed.

### Known risks

The source repository, registry, runtime tests, and prompt golden fixtures
intentionally retain former paths where they identify executable resources or
exercise no-shadowing behavior. They are not generated-project instructions.

### Capability impact

No capability-ledger update is authorized before review and archive. The planned
updates to CAP-003, CAP-004, and CAP-006 remain accurate; no overlay,
migration, configurable-policy, or direct-agent-execution behavior was added.

### Suggested review focus

Confirm that every generated-project-facing guide and adapter uses rendered
executable built-ins, while source-path references remaining in implementation
and tests are clearly non-operational.

## Review-03 remediation

### Finding: `init` contract still promises installed personas and prompts

- Disposition: fixed.
- `doc/command-reference.md` now says `concoct init` creates only the
  project-owned outputs of the distributed template. Its output manifest names
  policy, product truth, lifecycle directories, root guidance, and adapters;
  it explicitly states that protocol, personas, and handoffs are immutable
  executable resources and are not installed.
- A contextual search of active documentation and generated adapters found no
  remaining promise that initialized projects contain built-in personas or
  prompts. Remaining former-path matches are either explicit “not installed”
  ownership descriptions, source-resource diagnostics, runtime no-shadowing
  tests, or historical artifacts.

### Remediation verification

- `go test -count=1 ./...` passed.
- `go vet ./...` passed.
- `go build ./cmd/concoct` passed.
- `bash -n cmd/concoct/concoct.sh` passed and the wrapper remains executable.
- A fresh project initialized outside this repository contained project-owned
  files only, omitted built-in protocol, personas, and prompts, reached
  `ready`, and rendered `concoct next` using executable-owned guidance.
- `git diff --check` passed.

## Handoff to reviewer

### Implemented

The Review-03 initialization-contract finding is fixed. The normative `init`
reference now matches the selective project-output manifest and distinguishes
runtime built-ins from installed project state.

### Suggested review focus

Confirm the `init` documentation manifest matches `internal/project`'s output
filter and that active generated-project-facing documentation makes no contrary
claim about installed protocol, personas, or prompts.
