---
task-id: CON-018
review: 2
status: approved
created: 2026-08-04
persona: reviewer
---

# Review 02

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`approved`

## Summary

CON-018 now satisfies the typed workflow-policy task and its acceptance
contract. Review 01's archival evidence-integrity finding is fixed: direct
archive completion validates external-review metadata and contradictory review
history before it can accept review satisfaction or mutate repository state.
No material correctness, compatibility, scope, testing, or documentation issue
remains.

## Acceptance criteria assessment

All ten acceptance criteria are met. The implementation provides a closed typed
policy, deterministic per-activity resolution with source attribution, durable
skip and external-evidence requirements, one shared policy authority across
workflow consumers, default-lifecycle compatibility, observable supported
review variations, mutation-boundary evidence enforcement, deterministic
status and prompt reporting, actionable invalid-state refusal, and the stated
non-goal boundaries.

## Findings

No material findings.

## Prior finding disposition

### Review 01 Finding 1 — Archival bypasses external-review evidence validation

- Status: fixed
- Evidence: `CompleteArchive` now calls `validatePolicyEvidence` before deriving
  or accepting external review and rejects immutable review evidence combined
  with a non-required or externally satisfied review before archive candidate
  validation (`internal/workflow/archive.go:45-58`).
- Verification: `TestCompleteArchiveRejectsInvalidExternalReviewEvidenceBeforeMutation`
  covers missing reason, unsafe path, unauthorized recorder, and immutable
  review contradiction. Each case snapshots the fixture and confirms the
  rejected completion performs no mutation. Existing approved-review and
  explicit-override archive tests continue to pass.

## Verification performed

- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed.
- Executable-mode check for `cmd/concoct/concoct.sh` — passed.
- Root/template Codex skill byte comparison — passed.
- `git diff --check` against the recorded task base — passed.
- Fresh initialization at `/tmp/concoct-con018-review2.Vus3z2/project` —
  passed: project-owned files and nested adapters were staged in a new Git
  repository with no commit; bootstrap guidance was present; executable-owned
  protocol, persona, and prompt resources were absent.
- Independently inspected the complete task diff, the remediation diff, prior
  review, task and notes dispositions, policy/workflow/archive boundaries,
  tests, documentation, and relevant capability records.

## Capability impact assessment

The declared `update` impact for CAP-001, CAP-005, CAP-006, CAP-007, CAP-008,
CAP-009, CAP-010, and CAP-011 is accurate. The Archivist should reconcile these
records to describe policy-aware workflow truth, status and prompts, Git
applicability, planning and recommendations, completion boundaries, and archive
validation while preserving their established limitations.

## Scope assessment

The implementation remains disciplined to CON-018. It does not introduce
alternate Git strategies, profiles, origins, adoption, upgrades, arbitrary
graphs, workflow orchestration, or direct agent execution.

## Documentation assessment

README, instruction-layer, workflow, command, state-machine, policy template,
and Codex adapter changes agree with the delivered finite policy surface and
its evidence rules. Default-policy prompt behavior remains compatible.

## Risks and follow-up

The documented limitation remains appropriate: structural validation cannot
establish the semantic quality of human-authored reasons or external review
evidence. Broader selectable lifecycle activities require future command and
state contracts and are not approval blockers for this task.

## Handoff

Current state: approved. The Archivist may run `concoct archive`, preserve both
reviews and the remediation evidence, reconcile the declared capability
updates, and prepare the Git-backed archive for subsequent integration.
