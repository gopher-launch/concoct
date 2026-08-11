---
task-id: CON-031
roadmap-id: CON-031
status: archived
archived: 2026-08-11
review: review-02.md
delivery: pending-integration
capability-impact:
  type: update
  ids:
    - CAP-003
    - CAP-005
---

# Summary

## Delivered outcome

CON-031 established release and compatibility identities for Concoct. The CLI
reports version and build provenance, initialized projects record an
executable-owned contract with provenance, and centralized compatibility
checks reject unsafe project mutations while preserving reduced diagnostics.
Release automation now has documented SemVer and reproducible GoReleaser
provenance.

## Key decisions

- Product releases use SemVer with distinguishable development builds.
- Embedded content inherits executable product provenance.
- Project contract version and executable read/mutation ranges remain distinct.
- Compatibility is checked before mutation; upgrades remain future scope.

## Files and areas changed

Build identity, version reporting, project contract initialization and parsing,
compatibility preflight, release workflow configuration, tests, and related
documentation were updated.

## Verification

The full Go test suite, `go vet`, Go build, shell syntax validation, diff
checks, fresh-project malformed-contract checks, and injected release-identity
checks passed. GoReleaser `v2.9.0` was confirmed as the pinned release.

## Review outcome

`review-02.md` approved the completed implementation with no material findings.

## Capability changes

CAP-003 and CAP-005 now include release/project identity and safe compatibility
boundaries for initialized projects and the CLI.

## Skipped work

Project upgrades, migrations, and adoption of existing unversioned projects
remain outside this task.

## Follow-up work

CON-013 can use these identities to plan explicit project upgrades safely.
