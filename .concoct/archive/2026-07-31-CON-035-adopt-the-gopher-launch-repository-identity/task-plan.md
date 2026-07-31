---
id: CON-035
title: Adopt the Gopher Launch repository identity
roadmap-id: CON-035
status: implementation-complete
created: 2026-07-31
updated: 2026-07-31
capability-impact:
  type: none
  ids: []
  rationale: Changes canonical repository, module, and installation identity without adding, removing, or changing Concoct workflow behavior.
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-035-adopt-the-gopher-launch-repository-identity
  base: 138bb60c790e8121c7ae4c122acce172e856a087
  status: active
---

# Task Plan

## Goal

Complete the repository move by making `github.com/gopher-launch/concoct` the
single canonical identity used by the active Go module, source imports,
installation guidance, and current product documentation, while preserving Git
history and historically accurate durable artifacts.

## Context

The Git repository and its `origin` remote already use the Gopher Launch
organization, but `go.mod` still declares `github.com/cthain/concoct` and every
internal Go import is rooted at that former module path. The README provides
only local source-build guidance and still describes the repository rename as
future manual work. This split identity must be resolved before CON-031 defines
release and compatibility identities.

## Why this matters

Go module identity participates in package resolution, source installation,
build provenance, and user-facing installation commands. A canonical remote
with a different declared module path makes those surfaces contradictory and
would establish the wrong namespace if released unchanged.

## Current state

- `origin` fetch and push URLs are `git@github.com:gopher-launch/concoct.git`.
- The task branch is checked out at the prompt-recorded base
  `138bb60c790e8121c7ae4c122acce172e856a087`, which is also the current local
  `main` commit.
- `go.mod` declares `module github.com/cthain/concoct`.
- Fifteen imports across command, internal implementation, and test packages
  use `github.com/cthain/concoct`.
- `go list ./...` reports all packages under the former module namespace.
- README source instructions do not yet offer the canonical remote `go
  install` path and its manual-rename section is stale now that the move has
  occurred.
- A repository-wide search found no former repository URL in archived Concoct
  artifacts. Archives remain excluded from mechanical replacement if such a
  reference is discovered during implementation.
- The public HTTPS remote resolves, while its current remote `main` still
  precedes this task and therefore cannot validate the new module identity yet.

## Target state

- The Go module and every internal import use
  `github.com/gopher-launch/concoct`.
- Active source, tests, fixtures, generated content, configuration,
  documentation, and workflow guidance contain no stale claim that a former
  repository is canonical.
- The README documents truthful pre-release installation using `@main` and
  local source checkout, without presenting `@latest` as available.
- Git remote and ancestry evidence confirm the move retained the repository's
  existing history.
- Local build, test, vet, module, installation, template-initialization, and
  stale-reference checks pass; remote `@main` installation is confirmed after
  accepted integration is available on the remote trunk.

## Design constraints

- Treat each identity match contextually; do not use an unreviewed global
  replacement across the repository.
- Do not edit `.concoct/archive/` or completed append-only review evidence to
  modernize historical names.
- Preserve the repository's established Go package layout and executable
  behavior; this is an identity migration, not a CLI or architecture change.
- Keep installation claims aligned with current maturity: `@main` is the
  pre-release remote selector, while `@latest` belongs to CON-031 after a
  SemVer release exists.
- Preserve the executable bit and behavior of the source-checkout wrapper.
- Keep generated-project template content unchanged unless an identity search
  finds an active canonical reference requiring correction.
- Do not rewrite Git history, replace the remote, push branches, tag a release,
  or publish packages as part of implementation.
- Preserve the exact Git trunk, task branch, and base recorded in front matter.

## Non-goals

- No SemVer policy, release automation, `concoct version` command, tag, or
  `@latest` installation claim; those belong to CON-031.
- No GitHub repository administration, organization settings, description,
  topics, redirect configuration, or hosting-provider workflow changes.
- No workflow behavior, CLI command, template ownership, schema, or capability
  change.
- No broad documentation rewrite beyond correcting canonical identity and
  installation/rename guidance.
- No modification of archived task history solely to replace former names.

## Working assumptions

- `github.com/gopher-launch/concoct` is the durable product namespace selected
  by the Product Owner and represented by the configured remote.
- Existing Go package names remain valid when only the module/import prefix
  changes.
- `go mod tidy` should not materially change dependencies because the module
  has no dependency on its former path, but its result must still be reviewed.
- Capability impact is `none`: installation identity and provenance change,
  but the accepted observable workflow described by CAP-005 is unchanged.
- Remote `@main` installation is a delivery-stage check because the new module
  declaration cannot be resolved from the remote trunk before accepted work is
  integrated and pushed.

## Risks and open questions

- A blind replacement could alter historical context or append-only evidence;
  inventory and classification must precede edits and be repeated afterward.
- Go may report a module-path mismatch remotely until the updated `go.mod`
  reaches `main`; do not misclassify that expected pre-integration condition as
  an implementation defect.
- Public remote installation can be affected by network, proxy, or remote-push
  state. Record the exact command, environment-relevant failure, and whether
  the new trunk commit was available before drawing a conclusion.
- Cached module downloads could mask remote resolution problems. The terminal
  remote check must use a fresh temporary `GOMODCACHE` and `GOBIN`.
- No product decision remains unresolved. Exact README placement and wording
  are Developer choices within the truthful pre-release boundary.

## Implementation phases

### Phase 1 — Inventory and classify identity surfaces

Status: `complete`

- Search active and historical paths separately for former and canonical
  repository identities, including source, tests, fixtures, golden files,
  generated content, configuration, badges, documentation, templates, and
  Concoct artifacts.
- Record any match not already identified and classify it as active canonical
  identity, historical context, or unrelated text before editing.
- Confirm the prompt-recorded Git branch/base, configured remote, and clean
  implementation boundary.

### Phase 2 — Migrate the Go module identity

Status: `complete`

- Change the module declaration and all active internal imports to
  `github.com/gopher-launch/concoct`.
- Run Go formatting where changed Go files require it and run `go mod tidy`.
- Inspect `go.mod` and `go.sum` diffs to ensure module reconciliation introduced
  no unrelated dependency changes.

### Phase 3 — Reconcile active documentation and repository guidance

Status: `complete`

- Update README installation guidance to show the canonical pre-release `go
  install github.com/gopher-launch/concoct/cmd/concoct@main` path alongside
  local source-build usage.
- Replace the stale manual-rename guidance with current canonical repository
  information, without implying a release or `@latest` channel.
- Correct any other active canonical references discovered in Phase 1 while
  leaving historical and append-only artifacts accurate.

### Phase 4 — Verify local behavior and identity consistency

Status: `complete`

- Repeat scoped stale-reference searches across active and historical paths and
  explain any intentionally retained former identity.
- Run the full Go test, vet, build, module-list, tidy-diff, and local
  installation checks.
- Exercise the repository-standard initialization check in a temporary parent
  directory and verify dotfiles, nested templates, personas, planning
  directories, Git initialization, and bootstrap prompt creation.
- Verify wrapper syntax/executable mode, Git remote identity, and ancestry from
  the recorded task base.

### Phase 5 — Prepare review and delivery verification

Status: `complete`

- Record changed identity surfaces, verification results, intentional
  historical exceptions, and capability-impact confirmation in `notes.md`.
- Distinguish checks completed before review from remote `@main` installation,
  which becomes conclusive only after integration and push make the accepted
  commit available on the remote trunk.
- Set the plan to `implementation-complete`, commit the complete Developer
  transition on the task branch, and hand off for independent review.

## Acceptance criteria

- `go.mod` declares `module github.com/gopher-launch/concoct` and `go list -m`
  reports that exact path.
- Every internal Go import and `go list ./...` package path uses the canonical
  Gopher Launch module prefix.
- No active source, test, fixture, golden file, generated content,
  configuration, documentation, badge, template, or current Concoct artifact
  incorrectly presents `github.com/cthain/concoct` or
  `github.com/cthain/agent-workflow` as canonical.
- Any former identity retained in historical or append-only material is
  contextually correct and documented; no archive or completed review is
  rewritten solely for renaming.
- README installation guidance uses
  `go install github.com/gopher-launch/concoct/cmd/concoct@main` until a release
  exists and does not claim `@latest` availability.
- `go test ./...`, `go vet ./...`, `go build ./cmd/concoct`, `go mod tidy`, and
  `go install ./cmd/concoct` complete successfully.
- A temporary generated project contains the complete expected template,
  dotfiles, nested content, personas, planning directories, initialized Git
  repository, and bootstrap prompt.
- `origin` identifies `github.com/gopher-launch/concoct`, and the recorded base
  commit remains an ancestor of the accepted task work.
- Once the accepted commit is present on remote `main`, a fresh-cache `go
  install github.com/gopher-launch/concoct/cmd/concoct@main` resolves and
  installs a working executable.

## Verification

- Run `gofmt` on changed Go files if the import edits disturb formatting.
- Run `go mod tidy`, then confirm `git diff -- go.mod go.sum` contains only the
  intended module reconciliation.
- Run `go test ./...`, `go vet ./...`, and `go build ./cmd/concoct`.
- Install locally with a temporary destination, for example `GOBIN=<temporary
  directory> go install ./cmd/concoct`, then run the installed executable's
  `help` command.
- Run `bash -n cmd/concoct/concoct` and verify the wrapper remains executable.
- Run the repository's initializer against a temporary parent outside this
  repository; confirm root files, dotfiles, nested templates, personas,
  `.concoct/current/`, `.concoct/archive/`, Git initialization, staged files,
  bootstrap guidance, and absence of an initial generated commit.
- Run `rg` searches for both former repository namespaces across active paths;
  separately inspect archive matches rather than treating them as failures.
- Run `git remote -v`, verify the canonical fetch/push URL, and use `git
  merge-base --is-ancestor 138bb60c790e8121c7ae4c122acce172e856a087
  HEAD` before review and against the delivered commit.
- After accepted integration is available on remote `main`, use fresh temporary
  `GOMODCACHE` and `GOBIN` directories to run `go install
  github.com/gopher-launch/concoct/cmd/concoct@main`, then execute the installed
  binary's `help` command.
- Run `git diff --check` and inspect the complete diff for scope, historical
  accuracy, and unintended dependency or generated-content changes.

## Capability impact

Expected impact is `none`. CON-035 changes where the existing product is
canonically sourced and installed, but it does not change the accepted
initialization or workflow-status behavior in CAP-005. If implementation
discovers an observable behavior change or a capability record that actively
asserts the former namespace, stop and route capability-impact reconciliation
through the workflow rather than silently expanding this plan.

## Handoff expectations

The Developer should first set the task to `implementation-in-progress`, then
perform the classified identity migration without modifying archived evidence.
Before review, record every changed surface, retained historical exception,
commands and results, remote-verification state, and capability-impact
confirmation in `notes.md`; set the task to `implementation-complete`; commit
the complete transition on the recorded task branch; and recommend `concoct
review`.
