---
id: CON-031
title: Establish release and compatibility versioning
roadmap-id: CON-031
status: implementation-complete
created: 2026-08-11
updated: 2026-08-11
remediates-review: review-01.md
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-031-establish-release-and-compatibility-versioning
  base: 5afd771f24b890cd0bac69675fd07366bfc3ee81
  status: archived
  archive-commit: self
capability-impact:
  type: update
  ids:
    - CAP-003
    - CAP-005
  rationale: Adds observable product and project-contract identities, release provenance, and compatibility enforcement to initialized projects and the CLI that creates and operates on them.
---

# Task Plan

## Goal

Give every Concoct executable and initialized project an unambiguous identity:
official binaries report their Semantic Version and source revision, development
builds are visibly distinct, projects record the durable contract they use, and
the CLI refuses unsafe project mutations before encountering an unsupported
schema.

## Context

Concoct is currently source-build only. The Go executable has no `version`
command or injected build metadata, `internal/defaults` labels every embedded
resource as a development build, and initialized projects contain version `1`
front matter on several Markdown artifacts but no project-level contract
record. Those per-file versions are parser discriminators, not a product release
or a sufficient compatibility declaration.

CON-030 established that built-in protocol, personas, and handoffs are owned by
the executable while policy, guidance, adapters, truth, and lifecycle state are
project-owned. CON-031 must put release and compatibility identities around
that boundary without implementing project upgrades.

## Why this matters

Users need to know exactly what binary they are running and whether it can
safely operate on a repository. Maintainers need a repeatable official release
path and rules for deciding when a change is breaking. CON-013 needs stable
source and target identities before it can plan migrations without guessing
from repository contents.

## Current state

- `cmd/concoct/main.go` delegates directly to `internal/cli.Run`; the command
  surface has no version reporting.
- `internal/defaults.Provenance` is the fixed string `development build
  (embedded Concoct defaults)` and is not tied to executable build metadata.
- `templates.go` embeds the project template, and `internal/project.Initialize`
  copies project-owned outputs without recording their originating release or
  project-contract schema.
- Discovery checks only for `AGENTS.md`, roadmap, and capabilities. Workflow
  parsers validate artifact shapes but do not perform one project-level
  compatibility check before project-aware commands or mutations.
- Markdown front matter uses local `version: 1` fields. These fields do not
  identify the product release, the complete installed contract, or a supported
  executable range.
- README documents source installation from `main` and explicitly disclaims a
  packaged release channel. There are no release workflows, release-tool
  configuration, or release tags.

## Target state

- Concoct releases use one `vMAJOR.MINOR.PATCH` tag and SemVer product identity,
  starting with `v0` prereleases while compatibility remains fluid.
- A small build-information boundary supplies version, source revision, dirty
  state where knowable, and official/development classification. Release builds
  inject exact immutable values; ordinary source builds report a reserved,
  unmistakable development identity and never claim to be official.
- `concoct version` works without a project and has stable human-readable output;
  a script-friendly representation may be added if it is covered by the same
  fields and documented contract.
- Embedded-content identity is the product version plus source revision of the
  executable that contains it. It has no independent release sequence.
- Each initialized project contains the executable-owned installation record
  `.concoct/project.yaml`. It records a monotonically increasing
  `contract-version`, immutable `created-with` provenance, and
  `last-upgraded-with` provenance changed only by an explicit operation that
  changes the installed contract. It contains no user-authored configuration,
  and ordinary commands neither pin nor rewrite it for the running executable.
- One compatibility package reads and validates the record. Each executable
  declares separate project-contract ranges it can read and safely mutate; the
  initial ranges may both contain only contract version `1`, but they remain
  structurally distinct for future read-only compatibility.
- An absent installation record means a legacy or unversioned project, never an
  implicit current contract. Concoct may inspect only enough evidence to explain
  that condition and must not mutate the project until initialization or a
  future explicit upgrade establishes a supported record.
- Project-independent commands remain usable for every project condition.
  `status`, `why`, and future upgrade diagnostics retain a reduced mode that
  reports installation and compatibility evidence when full interpretation is
  unsafe. Mutation-capable commands validate mutate compatibility before their
  first side effect; ordinary workflow commands never upgrade the contract.
- Official artifacts are built by GoReleaser through GitHub Actions from valid
  Semantic Version tags. The tagged commit must pass required verification and
  every binary embeds the exact release and source revision. Published evidence
  includes archives, checksums, an SBOM, and artifact provenance.

## Design constraints

- Preserve the exact recorded Git trunk, task branch, and base in metadata and
  keep the planning transition separate from implementation.
- Use Semantic Versioning 2.0.0. Do not imply `v1` stability or compatibility
  guarantees during `v0` development.
- Keep product version, source revision, embedded-content identity,
  project-contract schema, and local artifact format versions conceptually and
  structurally distinct.
- Keep `.concoct/project.yaml` executable-owned and version-controlled, while
  leaving user-authored configuration outside it. Preserve `created-with`
  forever; change `last-upgraded-with` and `contract-version` only through an
  explicit operation that changes the installed project contract.
- Treat read compatibility and mutation compatibility as separate executable
  declarations. Reduced diagnostics must not be mistaken for permission to
  render roles or mutate workflow evidence.
- Centralize compatibility decisions so individual commands cannot drift or
  partially parse an unsupported project before rejection.
- Compatibility failures must be read-only, identify executable and project
  values, and recommend a safe next action without claiming an upgrade exists.
- Preserve caller-directory-independent initialization, selective template
  ownership, deterministic built-in resources, Git staging/no-commit behavior,
  and current workflow safety controls.
- Official releases must be reproducible from the tagged source and must not
  depend on a maintainer's uncommitted workspace.
- Release credentials and hosting configuration must remain CI/platform
  concerns; no secrets or machine-specific paths belong in the repository.

## Non-goals

- No `concoct upgrade`, migrations between contract schemas, overlay
  compatibility, or adoption of arbitrary pre-existing repositories; those
  remain with CON-013 and CON-014.
- No promise of stable `v1` APIs or indefinite support for every `v0` project.
- No independent version sequence for embedded defaults unless later evidence
  justifies one.
- No redesign of roadmap, capability, task-plan, notes, review, or archive
  schemas beyond wiring their existing local format versions into compatibility
  documentation.
- No package-manager matrix, installer service, automatic self-update, release
  announcement process, or broad changelog automation.
- No release signing or distribution infrastructure beyond GoReleaser's
  checksums, SBOM, and GitHub artifact provenance required by this task.

## Working assumptions

- The current `contract-version` starts at integer `1`. The executable's read
  and mutate ranges are both explicit even when their initial bounds coincide.
- Exact nested provenance field names beneath `created-with` and
  `last-upgraded-with`, internal package layout, and the reduced diagnostic
  presentation are Developer choices within the product decisions and
  acceptance criteria.
- CAP-003's empty writer-persona limitation is unrelated to versioning.
  CAP-005's read-only status and checked-in schema-parser limitations are
  compatible: this task extends the CLI boundary without requiring autonomous
  role judgment or generic Markdown parsing.

## Risks and open questions

- Go's default module build metadata can help development reporting but is not
  sufficient by itself to classify official artifacts. Tests must cover forged,
  missing, and dirty metadata so an official label requires the complete
  release contract.
- Adding the required record makes repositories created before this task
  explicitly legacy/unversioned. This repository and the embedded template must
  receive the record in the same change; legacy inspection stays reduced and
  non-mutating until CON-013 supplies an explicit upgrade path.
- Compatibility checks placed too late could allow Git branches, prompt output
  files, archives, or other state to change first. Command-level tests must
  prove rejection occurs before every mutation boundary.
- Reproducibility claims can exceed what tests establish. Documentation should
  state the pinned inputs and verification procedure precisely and avoid
  claiming bit-for-bit identity unless CI proves it.
- The first actual release number is a release-management choice at release
  time; implementation should test representative `v0.x.y` values without
  hard-coding an announced version not present in a tag.

## Implementation phases

### Phase 1 — Define identities and compatibility model

Status: `complete`

- Introduce the build-information and project-contract model, including
  official/development classification, schema identifiers, supported range,
  and actionable compatibility errors.
- Add the executable-owned `.concoct/project.yaml` installation record to this
  repository and initialization output, and ensure selective template copying
  and ownership documentation treat it explicitly.
- Document the mapping among product release, revision, embedded content,
  project contract, and artifact-local format versions.

### Phase 2 — Expose version and enforce compatibility

Status: `complete`

- Add `concoct version` as a project-independent command and connect defaults
  provenance to the same build-information source.
- Gate full project interpretation through read compatibility and every
  project-aware mutation through mutate compatibility. Preserve reduced
  compatibility reporting for `status`, `why`, and upgrade diagnostics, and
  reject unsupported or absent records before mutation.
- Ensure initialization writes `contract-version`, `created-with`, and initial
  `last-upgraded-with` provenance from the running executable while retaining
  established Git and validation behavior; ordinary commands must not rewrite
  those values.

### Phase 3 — Establish the release path and policy

Status: `complete`

- Add pinned GoReleaser configuration and GitHub Actions release automation
  triggered only by valid `v0.x.y`/future SemVer tags, with tagged-source
  validation, required verification, and deterministic metadata injection.
- Produce documented platform artifacts, checksums, an SBOM, exact source
  revision evidence, and GitHub artifact provenance.
- Document tag conventions and how major, minor, patch, and prerelease changes
  apply to CLI behavior, embedded content, project schemas, and compatibility.

### Phase 4 — Verify, document, and prepare review

Status: `complete`

- Add unit and CLI tests for official and development identities, exact
  revision reporting, contract parsing, compatibility range boundaries, missing
  metadata, and pre-mutation rejection.
- Exercise a fresh initialized project and representative older/newer contract
  fixtures without mutating unsupported repositories.
- Validate release configuration locally where tooling permits, update active
  user/developer documentation, run repository-standard checks, and record a
  complete reviewer handoff.

## Acceptance criteria

- `concoct version` runs inside or outside a project and reports a complete
  official SemVer plus source revision for a correctly built tagged release.
- A normal local/source build is labeled as a development build and cannot emit
  the official-release classification through absent, partial, malformed, or
  dirty build metadata. It reports source revision and modified status when
  those values are available.
- `defaults list/show` provenance uses the same executable identity and remains
  available when no project exists or a project contract is unsupported.
- Newly initialized projects and this repository contain executable-owned
  `.concoct/project.yaml` with `contract-version: 1`, immutable `created-with`,
  and initial `last-upgraded-with` provenance. User configuration is absent,
  and ordinary commands do not rewrite the record or pin operation to its
  generating executable.
- Product release, project-contract schema, embedded-content identity, and
  artifact-local format versions are separately named and documented; embedded
  content derives from the executable identity.
- The executable declares distinct read and mutate contract-version ranges.
  Fully supported records proceed normally; read-only-only, older, newer,
  malformed, and absent records yield actionable diagnostics with project and
  executable compatibility data. Absence is reported as legacy/unversioned.
- Unsupported project contracts are rejected before branch creation, artifact
  writes, output-file creation, Git staging/commits, archival, or integration.
  `status`, `why`, and upgrade diagnostics remain read-only in reduced mode and
  report compatibility without pretending to understand unsupported workflow
  state. Ordinary commands never perform an implicit upgrade.
- GoReleaser/GitHub Actions accepts valid SemVer tags, verifies the tagged
  commit, injects exact version and revision into every official binary, builds
  the documented artifact set, and publishes checksums, an SBOM, and artifact
  provenance.
- Documentation gives concrete major/minor/patch/prerelease rules, identifies
  `v0` instability, explains compatibility behavior, and does not claim a
  release channel until an official tagged release exists.
- Existing initialization, prompt rendering, workflow detection, role
  transitions, archival, and integration tests continue to pass for a supported
  project contract.
- The delivered identities are sufficient for CON-013 to compare an installed
  project with a candidate target release and plan an explicit migration.

## Verification

- Run `gofmt` on all changed Go files.
- Run focused tests for build metadata classification, version output,
  project-contract parsing, range comparison, missing/unsupported records,
  defaults provenance, and every mutating command's preflight ordering.
- Run `go test -count=1 ./...`, `go vet ./...`, and `go build ./cmd/concoct`;
  inspect the built binary's development `concoct version` output.
- Build a representative release binary with injected `v0.x.y` and exact Git
  revision, verify its version output, and verify malformed/partial/dirty inputs
  remain development identities.
- Validate release configuration and workflow syntax with the selected release
  tool, using snapshot/dry-run mode so planning and development do not publish.
- Run `bash -n cmd/concoct/concoct.sh` and verify executable mode is preserved.
- Initialize a project under a temporary parent outside this repository; confirm
  the contract record, substituted project identity, dotfiles, nested adapters,
  planning directories, Git repository, staged files, bootstrap prompt, no
  commit, supported status, and version-independent normal operation.
- Clone fully supported, read-compatible/mutate-incompatible, older, newer,
  malformed, and missing-record fixtures; run all relevant project-aware
  commands and compare Git status and filesystem manifests before/after to prove
  reduced-mode reporting works and every mutation-incompatible case is
  non-mutating.
- Run `git diff --check`, search active documentation for stale source-only and
  unqualified version claims, and inspect the full diff for leaked secrets,
  machine-specific paths, or accidental schema coupling.

## Handoff expectations

The Developer should record the exact identity fields, compatibility matrix,
release mechanism, affected command preflight points, verification results, and
any deviation from the assumed YAML/release-tool shape in `notes.md`. Before
review, all phases must reflect actual status and notes must contain a fresh
`## Handoff to reviewer` with suggested focus on official-build
misclassification, pre-mutation compatibility enforcement, initialized-project
metadata, and release provenance.
