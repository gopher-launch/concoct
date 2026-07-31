---
task-id: CON-035
roadmap-id: CON-035
status: archived
archived: 2026-07-31
review: review-01.md
delivery: pending-integration
capability-impact:
  type: none
  ids: []
---

# Summary

## Task

Adopt `github.com/gopher-launch/concoct` as the canonical repository, Go
module, internal import, installation, and product namespace after the
repository move, while preserving Git history and historical artifacts.

## Delivered outcome

The Go module declaration and all internal imports now use the Gopher Launch
namespace. README guidance identifies the canonical repository, documents
truthful pre-release installation from `@main`, retains local source-checkout
usage, and does not claim an `@latest` release channel.

The migration preserves package layout, executable behavior, dependencies,
templates, archived history, completed reviews, and the recorded ancestry from
the pre-migration task base.

## Key decisions

- Classified identity references before editing instead of applying a global
  replacement, preserving intentional historical and current migration context.
- Used `@main` for pre-release remote installation and left release identity,
  tags, `@latest`, and version policy to CON-031.
- Treated the fresh-cache remote `@main` installation as a delivery-stage
  check because remote `main` does not yet contain the accepted module path.

## Files and areas changed

- Updated `go.mod` and internal Go imports across command, CLI, integration,
  project, prompt, workflow, and test packages.
- Updated `README.md` installation, local checkout, and canonical repository
  guidance.
- Updated active task artifacts with planning, implementation, verification,
  review, and archival evidence.

## Verification

- `go list -m` and `go list ./...` reported only the Gopher Launch module and
  package namespace.
- `go test ./...`, `go vet ./...`, `go build ./cmd/concoct`, `go mod tidy`, and
  local `go install ./cmd/concoct` with an installed `help` invocation passed.
- Wrapper syntax and executable mode, real temporary-project initialization,
  template copying, Git initialization, stale-identity searches, remote
  identity, base ancestry, and `git diff --check` passed.
- The Archivist reran module/package listing, tests, vet, build, wrapper,
  identity, ancestry, branch, remote, and diff validations before archival.

## Review outcome

`review-01.md` approved the implementation with no material findings. It
confirmed the canonical namespace, truthful pre-release documentation,
historical preservation, scope discipline, compatibility, verification, and
the deferred delivery-stage remote installation check.

## Capability changes

None. CON-035 changes canonical source and installation provenance without
changing the observable Concoct workflow, initialization, or status behavior.
The capability ledger therefore remains unchanged.

## Follow-up work

- Delivery remains pending `concoct integrate`; CON-035 stays active and the
  current task evidence remains intact until integration succeeds.
- After the accepted integration is pushed to remote `main`, run a fresh-cache
  `go install github.com/gopher-launch/concoct/cmd/concoct@main` and invoke the
  installed executable's `help` command.
- Correct the pre-existing stale wrapper path in `AGENTS.md` in a separately
  scoped repository-guidance change.

## References

- Roadmap item: `CON-035` in `.concoct/roadmap.md`
- Approving review: `review-01.md`
- Product documentation: `README.md`
- Canonical module declaration: `go.mod`
