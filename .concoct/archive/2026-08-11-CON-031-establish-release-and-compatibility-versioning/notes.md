# Notes

## Planning summary

CON-031 is ready for implementation. It has no unresolved roadmap dependency,
the selected base explicitly promotes it to `planned`, and its CAP-003/CAP-005
prerequisite limitations are compatible with the outcome. The plan defines one
product SemVer, derived embedded-content identity, a separate project-contract
schema, the executable-owned `.concoct/project.yaml` installation record,
separate read/mutate compatibility ranges, and the official
GoReleaser/GitHub Actions path without implementing upgrades.

The Product Owner supplied the previously unresolved decisions after the
initial planning commit. This revision replaces the provisional assumptions
with those authoritative decisions and narrows Developer discretion
accordingly.

## Confirmed findings

- Git identity is trunk `main`, task branch
  `concoct/con-031-establish-release-and-compatibility-versioning`, and base
  `5afd771f24b890cd0bac69675fd07366bfc3ee81`.
- Before planning, `.concoct/current/` contained only `.gitkeep`; no active task
  conflicted with CON-031.
- The base commit changes CON-031 from `candidate` to `planned` and makes no
  other product-contract edit.
- The CLI has no `version` command or shared build-information provider.
- `internal/defaults.Provenance` is a fixed development string, so embedded
  content currently has no release or revision identity.
- `templates.go` embeds `all:templates`; `internal/project.copyTemplates`
  selectively materializes project-owned files and substitutes project name.
- Initialized projects have no dedicated installation/project-contract record.
  Existing `version: 1` Markdown front matter is artifact-local parser data.
- `internal/project.Discover` and workflow parsing do not perform a centralized
  compatibility preflight before project-aware operations.
- README documents installation from source/`main`, disclaims a packaged release
  channel, and the repository has no release tags or release automation.

## Product decisions

- Store the executable-owned installation record at `.concoct/project.yaml`,
  separate from user configuration and artifact schemas. It contains monotonic
  `contract-version`, immutable `created-with`, and `last-upgraded-with`, which
  changes only for an explicit installed-contract upgrade. Ordinary execution
  neither pins nor updates the record.
- Treat a missing record as legacy/unversioned. Reduced inspection may explain
  the condition, but no mutation is allowed before initialization or explicit
  upgrade establishes a supported contract.
- Declare separate executable ranges for contracts it can read and mutate.
  Project-independent commands always work; `status`, `why`, and upgrade
  diagnostics retain reduced compatibility-reporting mode when full reading is
  unsafe. Every mutation checks compatibility before its first side effect.
- Keep upgrades explicit and reserved for CON-013; ordinary workflow commands
  never migrate the project contract.
- Derive embedded-content identity from the product release. Development builds
  identify development content; no independent content version is introduced.
- Use SemVer tags with a leading `v`. Official status requires complete,
  valid release metadata from a clean tagged source; all other binaries are
  development builds and report revision/modified status when available.
- Use GoReleaser through GitHub Actions from valid SemVer tags. Official builds
  come from the tagged commit after required verification, embed exact release
  and revision, and publish checksums, an SBOM, and artifact provenance.

## Prerequisite compatibility

- CAP-003's empty API/code/user writer personas do not affect executable or
  project-contract identity and can remain unchanged.
- CAP-005 deliberately parses only checked-in schemas. A project-level schema
  gate strengthens that constraint and preserves status as read-only; it does
  not require generic Markdown compatibility or role judgment.

## Risks

- A required new record creates an explicit boundary for older initialized
  projects. They must remain reduced-mode, legacy/unversioned, and non-mutating;
  migration/adoption belongs to CON-013.
- Official/development classification is security-adjacent provenance logic:
  partial `ldflags`, dirty trees, or local builds must never appear official.
- Compatibility checks must precede output-file creation and Git side effects,
  not merely workflow artifact writes.
- Release provenance documentation must match what CI actually emits and avoid
  overstating reproducibility or signing guarantees.

## Initial verification

- Read the effective executable protocol, project policy, AGENTS guidance,
  complete Task Planner persona and task prompt, roadmap item, CAP-003 and
  CAP-005 records, and every archive summary required by the rendered prompt.
- Inspected CLI dispatch, executable entry point, embedded defaults provenance,
  template ownership/copying, project discovery, workflow front-matter parsing,
  command documentation, README build guidance, Git tags/history, branch/base,
  and current-task state.
- No implementation or tests were run during the planning role.

## Developer handoff

### Current state

Planning is complete; task status is `planned` on the recorded task branch.

### Completed

- Validated roadmap eligibility, prerequisite compatibility, exact Git identity,
  repository reality, capability impact, and absence of a conflicting task.
- Reconciled the Product Owner's explicit manifest ownership/fields,
  legacy-project behavior, separate read/mutate ranges, reduced diagnostics,
  explicit-upgrade boundary, embedded-content identity, and GoReleaser evidence
  with the implementation phases, acceptance criteria, and verification.

### Remaining

- Implement the task plan without expanding into upgrades, overlays,
  self-update, or package-distribution work.
- Record exact technical choices and evidence here, update phase statuses, and
  prepare a fresh reviewer handoff.

### Known risks

Official-build misclassification, missing-record handling for older projects,
late compatibility checks that permit side effects, and release claims that
exceed emitted provenance.

### Suggested next step

`concoct code`

## Development findings

- Added `internal/buildinfo` as the sole executable identity boundary. An
  official identity requires injected release intent, valid leading-`v` SemVer,
  a hexadecimal revision, and `Dirty=false`; every other combination is
  rendered as `development`.
- Added `internal/contract` for the executable-owned installation record. The
  initial read and mutation ranges are separately declared, both `1..1`.
  Strict YAML decoding rejects malformed or unknown record fields.
- Initialization stamps `.concoct/project.yaml` after copying templates, so
  `created-with` and `last-upgraded-with` describe the invoking binary rather
  than static template bytes. Ordinary operations never rewrite that file.
- `status` and `why` offer reduced compatibility reporting for a missing or
  incompatible record. Every existing project-aware workflow command applies
  the centralized compatibility preflight before prompt output, branches,
  completion writes, archives, or integration.
- Added GoReleaser and tag-triggered GitHub Actions release configuration. The
  workflow validates both a clean tagged commit and leading-`v` SemVer before
  release metadata is injected.

## Verification

- `gofmt -w internal/buildinfo internal/contract internal/cli`
- `go test -count=1 ./...`
- `go vet ./...`
- `go build ./cmd/concoct`
- Built binary `concoct version` reported a development classification and the
  available source revision.
- `bash -n cmd/concoct/concoct.sh` and `git diff --check` passed.
- Fresh initialization under a temporary parent produced a stamped contract
  record, a Git repository with staged files and no commit, and `ready` status.
  After removing the record, `status` entered reduced compatibility reporting
  and `next` refused before output or Git side effects.
- GoReleaser was not installed locally, so its snapshot/dry-run validation was
  not run; the release workflow invokes pinned GoReleaser v2 in CI.

## Handoff to reviewer

### Implemented

Executable build identity, `concoct version`, defaults provenance, stamped
project contracts, centralized compatibility gates, release automation, and
versioning documentation for CON-031.

### Key decisions

Official classification is fail-closed; legacy and unsupported projects are
reduced-mode only. The contract is strict YAML and does not evolve implicitly.

### Files changed

`internal/buildinfo`, `internal/contract`, CLI/project/defaults integration,
the project template and this repository's installation record, tests, release
configuration, and user/command documentation.

### Verification

Full Go tests, vet, build, shell syntax, diff check, and fresh initialized and
legacy-project command exercises passed. GoReleaser local validation was
skipped because the executable is unavailable.

### Known risks

The first official release remains a maintainer tag decision. CI is the first
environment that will exercise GoReleaser's release publication path.

### Skipped or unresolved work

No upgrade/adoption command or contract migration was added; those remain
CON-013 scope.

### Capability impact

CAP-003 and CAP-005 gain executable/project identity and safe compatibility
boundaries after approval and archival.

### Suggested review focus

Attempt official-build misclassification with partial metadata; verify every
project-aware mutation preflights before side effects; inspect initialized
record provenance and release workflow artifact/provenance claims.

## Review 01 remediation

- Finding 1 — fixed. `internal/contract.Read` now rejects records whose
  required provenance fields are absent or blank, and rejects a second YAML
  document. The centralized read and mutation preflights therefore retain
  malformed records in reduced diagnostics and prevent workflow output.
- Finding 2 — fixed. The release workflow now pins GoReleaser to the reviewed
  exact version `v2.9.0`, rather than resolving a future v2 version at runtime.

## Verification after remediation

- `gofmt -w internal/contract/contract.go internal/contract/contract_test.go internal/cli/cli_test.go`
- `go test -count=1 ./internal/contract ./internal/cli`
- `go test -count=1 ./...`
- `go vet ./...`
- `go build ./cmd/concoct`
- `bash -n cmd/concoct/concoct.sh`
- `git diff --check`

## Handoff to reviewer

### Implemented

Closed both Review 01 findings: strict complete-record validation now protects
the centralized compatibility boundary, and release automation uses an exact
GoReleaser version.

### Key decisions

An installation record requires both provenance objects, each with a nonblank
version and revision; multiple YAML documents are invalid. `v2.9.0` is the
reviewed, exact GoReleaser release used by CI.

### Files changed

`internal/contract/contract.go`, contract and CLI compatibility tests, and
`.github/workflows/release.yml`; task context reflects the remediation.

### Verification

Focused contract/CLI tests, the full Go test suite, vet, build, shell syntax,
and diff checks passed.

### Known risks

GoReleaser is still unavailable locally, so its pinned release invocation will
first be exercised by CI; the exact version is confirmed by the upstream tag.

### Skipped or unresolved work

No upgrade/adoption command or contract migration was added; those remain
CON-013 scope.

### Capability impact

The approved CAP-003/CAP-005 identity and compatibility boundary remains
unchanged, with malformed installation evidence now safely rejected.

### Suggested review focus

Verify missing, blank, and multi-document provenance records stay in reduced
status mode and cannot produce workflow output; confirm CI resolves exactly
GoReleaser `v2.9.0`.
