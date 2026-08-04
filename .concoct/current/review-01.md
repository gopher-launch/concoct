---
task-id: CON-018
review: 1
status: changes-requested
created: 2026-08-04
persona: reviewer
---

# Review 01

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The typed policy, shared resolution model, prompt selection, documentation, and
default-policy compatibility are broadly aligned with the task. One major
evidence-integrity defect remains at the archival mutation boundary: archive
completion trusts the presence of externally satisfied review metadata without
running the validation that makes that metadata authoritative.

## Acceptance criteria assessment

- Criteria 1, 2, 4, 5, 6, 8, and 10 are supported by the implementation and
  passing parser, workflow, prompt, transition, archive, integration, and
  initialization coverage.
- Criterion 3 is not met at archive completion because an external-review entry
  can bypass its required reason, recorder, and safe evidence checks.
- Criterion 7 is not met at every mutation boundary because contradictory or
  malformed policy evidence can still authorize archival.
- Criterion 9 is met by status and prompt detection, but not by direct archive
  completion, which does not reject the same invalid evidence before mutation.

## Findings

### Finding 1 — Archival bypasses external-review evidence validation

- Severity: major
- Status: unresolved
- Evidence: `CompleteArchive` parses the task and immediately derives
  `externalReview` through `activityExternallySatisfied` at
  `internal/workflow/archive.go:29-66`. That helper checks only for an activity
  name and the literal `externally-satisfied` disposition
  (`internal/workflow/workflow.go:760-775`). The required reason, authorized
  recorder, non-empty evidence list, safe non-symlink paths, and policy
  contradiction checks exist only in `validatePolicyEvidence`
  (`internal/workflow/workflow.go:777-812`), which `CompleteArchive` never calls.
  The archive candidate validator repeats the same unvalidated boolean at
  `internal/workflow/archive.go:150-156`.
- Impact: A caller can invoke `concoct archive --complete` directly with an
  implementation-complete task containing, for example, an external-review
  entry recorded by an unauthorized role or pointing through an unsafe or
  missing path. Status and prompt rendering correctly classify that repository
  as invalid, but archive completion treats review as satisfied, accepts an
  empty summary review, and can commit or deliver the archive. This violates
  the plan's durable-evidence, invalid-state-refusal, and every-mutation-boundary
  requirements.
- Required action: Make archive completion consume validated workflow/policy
  resolution, or explicitly run the same policy-evidence and contradiction
  validation before accepting external satisfaction or mutating repository
  state. Add archive-level regression tests showing malformed, unsafe,
  unauthorized, and contradictory policy evidence cannot satisfy review.

## Prior finding disposition

No prior reviews exist.

## Verification performed

- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed.
- Executable-mode check for `cmd/concoct/concoct.sh` — passed.
- Root/template Codex skill byte comparison — passed.
- `git diff --check` against the recorded task base — passed.
- Independently inspected the complete task diff and relevant policy,
  workflow, prompt, transition, archive, integration, test, and documentation
  changes.

## Capability impact assessment

The declared updates to CAP-001, CAP-005, CAP-006, CAP-007, CAP-008, CAP-009,
CAP-010, and CAP-011 are appropriate if the archival validation gap is fixed.
Capability truth must not be reconciled until a later review approves the
remediated behavior.

## Scope assessment

The implementation remains within CON-018's finite policy boundary. Alternate
Git strategies, profiles, origins, adoption, upgrades, arbitrary graphs, and
agent orchestration were not pulled into scope.

## Documentation assessment

The updated documentation accurately describes the intended typed policy,
selectable review behavior, evidence containment, policy-selected handoffs, and
default lifecycle. The archival implementation must be brought into agreement
with those claims.

## Handoff

Current state: changes requested. The Developer should run `concoct code`, fix
Finding 1 at the archival boundary, record its disposition and verification in
notes, and return the task for a second independent review. The next review
should focus on direct archive completion with every invalid external-evidence
variant and confirm no mutation occurs before validation succeeds.
