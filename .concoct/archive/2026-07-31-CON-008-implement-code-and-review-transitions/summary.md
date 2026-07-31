---
task-id: CON-008
roadmap-id: CON-008
status: archived
archived: 2026-07-31
review: review-02.md
delivery: pending-integration
capability-impact:
  type: add
  ids:
    - CAP-010
---

# Summary

## Task

Implement validated, durable completion bookkeeping for the Developer and
Reviewer transitions around the existing deterministic `concoct code` and
`concoct review` prompt commands.

## Delivered outcome

Concoct now provides explicit `concoct code --complete`, `concoct review
--reserve`, and `concoct review --complete` boundaries while ordinary command
invocations remain deterministic, read-only prompt rendering. Developer
completion validates role-owned evidence, mode-specific metadata, remediation
dispositions, a complete fresh reviewer handoff, protected workflow paths, and
the resulting state. Reviewer work reserves the exact next review path
create-only and finalizes only a matching, schema-valid artifact with one
supported outcome.

Git-backed transitions validate the recorded branch, base, clean boundaries,
and absence of unrelated operations, then commit a coherent transition once
with safe retry reuse. Non-Git projects retain equivalent artifact validation
without fabricated commits. Completed reviews remain append-only, repeated
remediation and review numbering are deterministic, and invalid or interrupted
transitions preserve authored evidence with recovery guidance.

## Key decisions

- Kept prompt rendering state-preserving and exposed separate, scriptable
  completion and reservation boundaries.
- Reused the canonical workflow and Git inspection authorities instead of
  creating another transition model.
- Represented an incomplete review reservation at the canonical next path with
  `status: reserved`; valid reservations do not count as review outcomes.
- Required a Git-backed final reviewer handoff section to differ from its
  committed version while retaining a documented completeness rule for
  non-Git projects.
- Corrected the planned capability identifier from the already-used CAP-009 to
  CAP-010 without changing product scope.

## Files and areas changed

- Added shared transition validation and reservation handling under
  `internal/workflow` plus Git commit inspection support in `internal/gitrepo`.
- Extended CLI orchestration for explicit Developer completion and Reviewer
  reservation/finalization.
- Added focused workflow, CLI, and real-repository transition coverage.
- Updated README and normative command/state documentation.
- Synchronized Developer/Reviewer personas, handoff prompts, the Concoct skill,
  and their embedded template counterparts.

## Verification

- `go test -count=1 ./...` — passed during implementation, both reviews, and
  archival.
- `go vet ./...` and `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` and executable-mode validation — passed.
- Temporary generated-project initialization, source/template parity, focused
  Git boundary cases, and stale-handoff rejection passed before approval.
- Recorded task branch, immutable base ancestry, clean checkout, and
  `git diff --check 2654973..HEAD` were validated before archival.

## Review outcome

`review-02.md` approved the implementation after remediation. Review 01's Git
reservation-boundary finding was resolved as disputed with evidence and added
focused regression coverage; its stale-handoff finding was fixed with a
committed-section freshness comparison and a negative regression test. Both
reviews are preserved.

## Capability changes

Added CAP-010 for validated durable Developer and Reviewer completion
coordination. Existing CAP-001, CAP-005, CAP-006, CAP-007, CAP-008, and CAP-009
remain compatible and retain their accepted truth.

## Skipped and follow-up work

- Direct agent execution, general locking, worktrees, concurrent tasks, hosted
  review, and automated archival remain outside CON-008.
- Non-Git projects cannot compare a handoff with a committed baseline and use
  the accepted artifact-completeness rule.
- Delivery remains pending `concoct integrate`; CON-008 stays active and
  current task evidence remains intact until integration succeeds.

## References

- Roadmap item: `CON-008` in `.concoct/roadmap.md`
- Capability: `CAP-010` in `.concoct/capabilities.md`
- Approving review: `review-02.md`
- Prior review: `review-01.md`
- User documentation: `README.md`
- Normative contracts: `doc/command-reference.md`, `doc/state-machine.md`
