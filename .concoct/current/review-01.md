---
task-id: CON-017
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

The implementation establishes the three owned files, protects the four
protocol controls, attributes prompt sources, preserves project-guidance bytes,
and keeps initialization and the existing workflow checks passing. It does not
yet implement the accepted contract for conflicts between policy and project
guidance: project guidance can declare a contradictory policy value and the
composer silently ignores it. This leaves acceptance criterion 7 unmet and
makes the documentation's structured-conflict claim inaccurate.

## Findings

### Major — Policy/project declarations have silent conflict behavior

- Evidence: `internal/instruction/compose.go` parses every front-matter key but,
  for the project-guidance layer, validates only `instruction-layer`,
  `weaken-controls`, and `strengthen-controls`. A project `AGENTS.md` may
  therefore declare a policy-owned key such as
  `git-strategy: direct-main`; composition succeeds and silently discards that
  declaration while `.concoct/policy.md` declares
  `task-branch-with-squash-integration`. The test suite covers protocol
  weakening, malformed and missing sources, ordering, and byte preservation,
  but contains no policy/project conflict case. `doc/instruction-layers.md`
  states that structured conflicts fail explicitly and that no
  last-writer-wins behavior exists.
- Impact: acceptance criterion 7 is not satisfied, the requested focused test
  for policy/project conflicts is absent, and users receive neither a
  deterministic precedence result nor an actionable error for contradictory
  structured declarations. The effective policy may differ from what the
  project-owned source appears to request.
- Required outcome: define and enforce the allowed declaration keys and the
  conflict/precedence behavior between policy and project guidance. Reject an
  unsupported or contradictory project declaration with diagnostics naming
  both layers and source paths, or implement and document an explicit
  deterministic precedence rule. Add focused tests for the chosen behavior,
  including no partial composition/render output.

## Acceptance assessment

- Scope remains focused on CON-017 and respects the stated non-goals.
- Protocol ownership, invariant declarations, source ordering and attribution,
  prompt all-or-nothing behavior, initialization, and project-guidance byte
  preservation are implemented and covered.
- Default artifacts represent the existing lifecycle, and existing workflow
  transition tests remain passing.
- Acceptance criterion 7 and its explicit policy/project conflict verification
  expectation remain unresolved as described above.
- No prior review findings exist.

## Verification

- `go test -count=1 ./...` passed.
- `go vet ./...` passed.
- `go build ./cmd/concoct` passed.
- `bash -n cmd/concoct/concoct.sh` passed; the script is executable.
- Fresh initialization in a temporary parent installed protocol, policy,
  nested and dotfile content, bootstrap prompt, Git repository, staged files,
  and no commit; generated-project `status` reported `ready`.
- `git diff --check 2b99900..HEAD` passed.
- Stale canonical persona and singular-review path search returned no matches
  in current non-golden source and documentation.

## Capability impact

The proposed update classification for CAP-001, CAP-003, CAP-004, and CAP-006
is accurate, but the capability ledger should not be updated until the finding
is remediated and a later independent review approves the task.

## Handoff

Developer remediation is required. Preserve this review, record the finding's
disposition in notes, implement and test explicit policy/project structural
conflict behavior, rerun the documented checks, and complete a new development
transition. The next command is `concoct code`.
