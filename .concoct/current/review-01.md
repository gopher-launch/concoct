---
task-id: CON-031
review: 1
status: changes-requested
created: 2026-08-11
persona: reviewer
---

# Review 01

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The implementation establishes the intended build-identity boundary, stamped
installation record, project-command gates, version command, and release
configuration. Independent verification passed: `go test -count=1 ./...`,
`go vet ./...`, `go build ./cmd/concoct`, shell syntax, and diff checks. A
fully injected clean `v0.4.2` identity also reported as an official release.

Two major issues prevent acceptance: an incomplete project installation record
is accepted as compatible and permits full workflow interpretation, and the
release workflow does not pin the GoReleaser version despite the approved
reproducibility requirement.

## Acceptance criteria assessment

- Build identity, development fallback, `concoct version`, and defaults
  provenance are implemented and verified.
- Initialization stamps the expected record and existing supported-project
  regression checks pass.
- The compatibility gate is not sufficient for malformed records: required
  provenance is not validated, so reduced mode is bypassed.
- Release automation verifies tags and emits the configured checksums/SBOM/
  provenance, but its GoReleaser dependency floats within major version 2.

## Findings

### Finding 1 — Incomplete contract records bypass compatibility protection

- Severity: major
- Status: open
- Evidence: `internal/contract.Read` enables `KnownFields(true)` but validates
  only a positive `contract-version`; it never requires or validates
  `created-with` and `last-upgraded-with`, and it decodes only the first YAML
  document. In an independently initialized temporary project, replacing
  `.concoct/project.yaml` with only `contract-version: 1` made `status` report
  the full `ready` workflow state and let `next --output accepted-malformed.md`
  succeed. This is a malformed/incomplete installation record, yet it is
  treated as mutation-compatible.
- Impact: This violates the acceptance requirement that malformed records yield
  reduced actionable diagnostics and that full interpretation/mutation gates
  centrally reject unsupported evidence. It also gives CON-013 an unreliable
  source identity to compare.
- Requested change: Validate all required record fields and their values,
  reject trailing YAML documents, and add focused tests proving incomplete,
  empty, malformed, and multi-document records cannot proceed through status
  full interpretation or any project-aware command.

### Finding 2 — GoReleaser is not pinned

- Severity: major
- Status: open
- Evidence: `.github/workflows/release.yml` configures
  `goreleaser/goreleaser-action@v6` with `version: '~> v2'`. That range resolves
  to a future GoReleaser v2 release at CI execution time, rather than an exact
  pinned tool version.
- Impact: The task explicitly requires pinned GoReleaser configuration and a
  reproducible official release path. A floating release tool can change
  archive, SBOM, or build behavior for the same tagged source.
- Requested change: Pin GoReleaser to an exact reviewed v2 release (and update
  any documentation or validation evidence accordingly).

## Scope and capability impact

The declared CAP-003/CAP-005 update is accurate if the above issues are
resolved. No unrelated scope expansion was found.

## Verification performed

- `go test -count=1 ./...`
- `go vet ./...`
- `go build ./cmd/concoct`
- `bash -n cmd/concoct/concoct.sh`
- `git diff --check 5afd771f24b890cd0bac69675fd07366bfc3ee81..HEAD`
- Development and injected official `concoct version` command exercises.
- Fresh-project malformed-record exercise described in Finding 1.

## Recommendation

Return the task to the Developer to close both major findings, then request a
new independent review.
