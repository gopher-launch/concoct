# Notes

## Planning summary

- Readiness: `ready` before this planning transition.
- `CON-035` exists with status `planned`, has no outstanding roadmap
  dependencies, and defines one coherent canonical-identity migration.
- The prompt-recorded task branch
  `concoct/con-035-adopt-the-gopher-launch-repository-identity` is checked out
  at the exact recorded base `138bb60c790e8121c7ae4c122acce172e856a087`.
- No active task artifacts conflicted with planning, and repository reality
  supports the selected outcome.

## Confirmed findings

- `origin` already uses `git@github.com:gopher-launch/concoct.git` for fetch and
  push, and the public HTTPS repository resolves.
- `go.mod` and `go list -m` still identify `github.com/cthain/concoct`.
- Fifteen imports in active command, internal implementation, and test files
  use the former module prefix; `go list ./...` consequently reports every
  package beneath it.
- README source-build guidance is local-only and its manual-repository-rename
  section is stale after the completed hosting move.
- Searches found no former repository URL in `.concoct/archive/`. The archive
  remains an explicit preservation boundary if implementation discovers a
  historical reference through broader matching.
- No badges, release workflow, or other hosted-repository configuration is
  currently present.

## Capability prerequisite compatibility

- `CAP-005` is active and compatible. Changing the module and installation
  namespace does not alter initialization or read-only status behavior.
- CAP-005's limitations concern prompt/mutation boundaries, textual remediation
  matching, and schema-scoped validation; none blocks this migration.
- CAP-003 remains `limited` and is not a prerequisite. Its known template
  limitations do not prevent checking that embedded initialization remains
  unchanged.

## Decisions

- Scope the implementation to module/import identity, truthful README
  installation and repository guidance, and any additional active stale
  references found by classified inventory.
- Use `@main` for remote installation before the first release. Reserve
  `@latest`, version metadata, tagging, and release policy for CON-031.
- Preserve archives and append-only reviews rather than applying a repository-
  wide mechanical replacement.
- Record capability impact as `none`; canonical provenance changes without a
  workflow behavior change.
- Treat remote `@main` installation as a terminal delivery check. Before the
  new module declaration reaches remote `main`, its expected failure is not
  evidence against the local implementation.

## Risks

- Mechanical replacement could erase historically meaningful context.
- Module proxies or caches could hide remote resolution state, so the final
  remote installation check needs fresh temporary caches.
- Review necessarily occurs before integration; the handoff must state clearly
  that remote `@main` verification remains pending until the accepted commit is
  pushed to the remote trunk.

## Relevant history

- CON-005 introduced the Go module, embedded templates, CLI entry point, and
  current source-build model represented by CAP-005.
- CON-029 intentionally avoided inventing a package or release channel; this
  task now establishes a truthful pre-release `@main` installation path.
- CON-031 depends on CON-035 so release and compatibility versioning will be
  defined only after the canonical repository identity is consistent.

## Handoff

- Current state: `planned` after the selected roadmap status and paired active
  artifacts validate.
- Work completed: prerequisite, archive, module/import, documentation, remote,
  history, and verification-surface inspection; implementation-ready plan and
  acceptance criteria.
- Work remaining: implement and verify the identity migration, preserve
  historical boundaries, document results, and prepare independent review.
- Unresolved questions: none requiring Product Owner action; remote `@main`
  verification is deliberately staged after accepted integration reaches the
  remote trunk.
- Expected next role: Developer.
- Recommended next command: `concoct code`.

## Implementation findings and decisions

- The classified inventory found 15 active Go imports plus the `go.mod`
  declaration using the former module path. No source, fixture, golden,
  configuration, template, archive, or completed review contained another
  former canonical repository reference.
- Former names remain only in the roadmap, task plan, and planning notes where
  they accurately describe the migration and its acceptance boundary.
- `go mod tidy` changed no dependency requirements or checksums.
- Repository guidance named `cmd/concoct/concoct` as the source-checkout
  wrapper, but that path is ignored build output. The tracked executable
  wrapper is `cmd/concoct/concoct.sh`; README source usage and verification
  now name the actual wrapper. The standing `AGENTS.md` command remains stale
  and was not changed because this task does not authorize unrelated canonical
  instruction maintenance.
- Remote `@main` installation remains a delivery-stage check. The accepted
  module declaration is not yet present on remote `main`, so running it before
  integration and push would not verify this implementation.

## Verification results

- `gofmt` completed on every Go file whose imports changed.
- `go mod tidy` passed with no `go.sum` or dependency changes.
- `go list -m` reported `github.com/gopher-launch/concoct`.
- `go list ./...` reported every package beneath the Gopher Launch module.
- `go test ./...`, `go vet ./...`, and `go build ./cmd/concoct` passed.
- A temporary `GOBIN` local `go install ./cmd/concoct` succeeded, and the
  installed binary's `help` command succeeded.
- `bash -n cmd/concoct/concoct.sh` passed and the tracked wrapper remains
  executable.
- A real initialization through the source-checkout wrapper created a
  temporary project with root files, dotfiles, nested templates, personas,
  current and archive directories, bootstrap guidance, an initialized Git
  repository, 33 staged paths, and no initial commit. The temporary project
  was removed after verification.
- Scoped former-identity searches found no stale active product surface and no
  historical archive match. The remaining current-workflow and roadmap matches
  are intentional migration context.
- `origin` fetch and push both identify
  `git@github.com:gopher-launch/concoct.git`.
- `git merge-base --is-ancestor
  138bb60c790e8121c7ae4c122acce172e856a087 HEAD` passed before the Developer
  commit; the same check is required against the committed transition.
- `git diff --check` passed, and the complete diff was inspected for scope and
  dependency churn.

## Handoff to reviewer

### Implemented

- Migrated the Go module declaration and all internal imports to
  `github.com/gopher-launch/concoct`.
- Added truthful pre-release `@main` installation guidance, replaced the stale
  manual-rename section with canonical repository identity, and corrected the
  README source-wrapper filename while editing that guidance.

### Key decisions

- Preserved historical and workflow migration references rather than applying
  a global replacement.
- Kept `@latest`, releases, tags, and version policy out of scope.
- Deferred remote fresh-cache `@main` installation until the accepted commit
  is integrated and available on remote `main`.

### Files changed

- `go.mod`; `README.md`; command and internal Go source/test imports; active
  `task-plan.md` and `notes.md`.
- `go.sum`, templates, archives, completed reviews, roadmap, and capabilities
  were not changed.

### Verification

- Formatting, tidy, module/package listing, tests, vet, exact build, local
  install/help, wrapper syntax/mode, real initialization, stale-reference
  searches, remote identity, base ancestry, and diff checks passed as detailed
  above.

### Known risks

- Remote installation cannot become conclusive until integration and push make
  this module declaration available on the remote trunk.
- `AGENTS.md` retains a pre-existing stale wrapper path outside this task's
  identity-migration scope.

### Skipped or unresolved work

- Fresh-cache `go install
  github.com/gopher-launch/concoct/cmd/concoct@main` is intentionally pending
  delivery. No implementation work remains.

### Capability impact

- Confirmed `none`: the canonical source and installation namespace changed,
  but observable Concoct workflow behavior did not.

### Suggested review focus

- Confirm every changed import belongs to this module, README language does
  not imply a release channel, intentional former-name references are
  contextual, and remote verification is correctly deferred.

## Archival handoff

- Archive validation passed for active task `CON-035`, roadmap metadata, the
  recorded Git branch and base, accepted implementation presence, required
  artifacts, approving `review-01.md`, and the unused archive destination.
- Preserved accepted task, notes, and review evidence with a durable summary at
  `.concoct/archive/2026-07-31-CON-035-adopt-the-gopher-launch-repository-identity/`.
- Capability impact remains `none`; canonical source, module, and installation
  provenance changed without changing accepted workflow behavior, so
  `.concoct/capabilities.md` remains unchanged.
- Archivist validation reran module/package listing, tests, vet, build, wrapper
  syntax and mode, stale-identity searches, branch, remote, base ancestry, and
  diff checks successfully.
- Delivery remains pending integration; current task evidence stays intact and
  CON-035 remains active until `concoct integrate` succeeds.
- After integration is pushed to remote `main`, run the documented fresh-cache
  remote `@main` installation and installed-binary help check.
- Expected next transition: `concoct integrate`.
