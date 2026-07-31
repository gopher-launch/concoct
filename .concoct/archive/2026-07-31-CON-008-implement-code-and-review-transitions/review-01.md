---
task-id: CON-008
review: 1
status: changes-requested
created: 2026-07-31
persona: reviewer
---

# Review 01

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The implementation establishes the intended explicit Developer completion and
Reviewer reservation/finalization commands, keeps ordinary prompt rendering
read-only, preserves source/template parity, and passes the full documented Go
verification suite. Two major transition-boundary gaps remain: review
reservation does not validate the recorded Git boundary, and Developer
completion does not establish that the required reviewer handoff is fresh.

## Findings

### Finding 1 — Review reservation bypasses the Git entry boundary

- Severity: `major`
- Evidence: `internal/workflow/transition.go:69-89` calls
  `InspectPromptContext`, checks only `implementation-complete`, and creates the
  reservation directly. It does not call `InspectGitContext` or the shared
  `transitionRepository` checks that enforce the recorded task branch, base,
  attached checkout, clean worktree, and absence of an unrelated Git
  operation. The new tests exercise reservation only from the expected clean
  branch and contain no dirty, detached, wrong-branch, or operation-in-progress
  reservation cases.
- Impact: A Git-backed Reviewer can reserve `review-NN.md` from a wrong branch,
  detached checkout, dirty worktree, or active Git operation. This violates
  acceptance criterion 9's entry-boundary guarantee and can leave an untracked
  reservation in an unsafe checkout before finalization rejects it.
- Required outcome: Validate the recorded Git identity and clean entry boundary
  before creating a reservation, without weakening create-only behavior; add
  focused coverage for wrong branch or detached HEAD, dirty state, unrelated
  Git operations, recorded base validation, and the valid clean path.

### Finding 2 — Existing handoff headings satisfy the claimed fresh-handoff check

- Severity: `major`
- Evidence: `internal/workflow/transition.go:50-61` requires notes to appear in
  Git status, then searches the final file for headings with
  `strings.Contains`; it never establishes that the handoff section or its
  required content changed in this transition. The primary loop test in
  `internal/cli/transition_test.go:18-21` starts with a pre-existing complete
  `reviewerHandoff` and merely appends `Initial implementation complete.`, yet
  completion succeeds.
- Impact: Stale implementation details, verification, risks, capability impact,
  and review focus can be presented as the current transition's handoff. This
  fails acceptance criterion 3 and the documented promise that completion
  requires a complete fresh reviewer handoff.
- Required outcome: Validate freshness against the Git diff or adopt explicit
  transition metadata that unambiguously identifies the current handoff, and
  add a negative test proving that an unrelated notes edit cannot reuse an old
  handoff. Preserve non-Git behavior with a documented artifact-level rule.

## Acceptance assessment

- Prompt rendering remains deterministic and state-preserving, and explicit
  completion commands are documented.
- Review numbering, reserved-state recognition, completed-outcome validation,
  append-only finalization, Git commit reuse, and non-Git completion have
  focused coverage.
- Acceptance criteria 3 and 9 are not met for the reasons above.
- No unrelated implementation scope was found. The roadmap activation and
  planned CAP-010 impact are consistent with the task.
- No prior review findings exist.

## Verification

- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed; executable mode confirmed.
- Changed source workflow assets match their `templates/` counterparts.
- `git diff --check 2654973..HEAD` — passed.

## Capability impact

CAP-010 remains the appropriate expected addition after remediation and
approval. Do not reconcile the capability ledger while these transition safety
claims remain unaccepted.

## Handoff

- Current state: `changes-requested` after review completion.
- Work completed: independent diff, source, test, documentation, template, and
  verification review.
- Work remaining: enforce the Git boundary at reservation and make reviewer
  handoff freshness structurally verifiable, with focused regression tests.
- Known risks: unsafe reservation creation and stale reviewer context.
- Artifact updated: `.concoct/current/review-01.md` only.
- Expected next role: Developer in remediation mode.
- Recommended next command: `concoct code`.
