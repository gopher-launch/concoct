---
task-id: CON-017
review: 2
status: approved
created: 2026-07-31
persona: reviewer
---

# Review 02

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`approved`

## Summary

The remediation closes the policy/project structural-conflict gap identified
in Review 01. Each instruction layer now has a closed declaration-key schema;
project guidance that declares a policy-owned key fails with both layers and
source paths identified, and other unsupported declarations also fail instead
of disappearing silently. Focused composition and prompt tests demonstrate
all-or-nothing behavior, and the documentation accurately describes the chosen
explicit-failure rule.

No material correctness, compatibility, scope, testing, or documentation issue
remains. The implementation satisfies CON-017 and is ready for archival.

## Prior finding disposition

### Review 01 — Policy/project declarations have silent conflict behavior

- Status: `fixed`.
- Evidence: `validateDeclarationOwnership` checks deterministically sorted keys
  against per-layer allowlists. A policy-owned declaration in `AGENTS.md`
  reports the project-guidance layer and `AGENTS.md`, the policy layer and
  `.concoct/policy.md`, the conflicting key, and corrective action. Generic
  unsupported declarations fail explicitly. Focused tests cover both direct
  composition and rendered-prompt failure and assert no partial result.
- Assessment: resolved. Acceptance criterion 7 and the corresponding focused
  verification expectation are now satisfied.

## Acceptance criteria assessment

- Protocol, policy, project guidance, and task context have distinct ownership
  and deterministic source attribution.
- The four protected protocol controls are represented and cannot be weakened
  by policy or project guidance.
- The default policy retains the accepted lifecycle, approval gates, and Git
  strategy without introducing configurable-policy scope.
- `AGENTS.md` remains the project-owned conventional entry point and its bytes
  are preserved by composition.
- Structural declarations have an explicit closed schema; compatible
  strengthening is retained, invariant weakening and policy/project conflicts
  fail before effective guidance is returned, and unrestricted prose remains
  honestly outside semantic validation.
- Initialization, adapters, canonical persona/review paths, deterministic
  prompt rendering, existing workflow transitions, and create-only output
  behavior remain covered and passing.
- Legacy mixed ownership is documented for explicit reconciliation rather than
  silent mutation.

## Findings

No material findings.

## Verification performed

- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed; executable mode confirmed.
- Fresh initialization in a temporary parent installed protocol, policy,
  nested and dotfile content, personas, planning files, and bootstrap prompt;
  staged generated files; created no commit; and reported `ready`.
- `git diff --check 2b99900..HEAD` — passed.
- Stale canonical persona and singular-review path search returned no matches
  in current non-golden source and documentation.
- Reviewed the complete task diff, remediation diff, instruction composer and
  focused tests, prompt integration and tests, documentation, task evidence,
  prior review, and current capability truth.

## Scope and compatibility

The remediation is limited to the accepted declaration-conflict contract,
tests, documentation, and required workflow evidence. It does not add policy
selection, migration, adoption, upgrades, overlays, agent execution, or any
other stated non-goal. Existing Git and non-Git workflow behavior remains
passing.

## Capability impact

Recommend that the Archivist update CAP-001, CAP-003, CAP-004, and CAP-006 as
declared. The accepted behavior clarifies layered workflow ownership, corrects
the reusable template and adapter limitations, and adds attributed validated
composition to prompt rendering without creating a separate capability.

## Handoff

- Current state after completion: approved and ready for archival.
- Work completed: independent post-remediation review and full verification.
- Work remaining: archive the approved Git-backed task, then integrate it under
  the recorded lifecycle.
- Decisions: Review 01's major finding is fixed; no new material findings.
- Known risks: semantic conflicts in unrestricted prose remain human
  reconciliation work by explicit design; future policy configuration must
  extend the owning declaration schema deliberately.
- Artifact updated: `.concoct/current/review-02.md` only.
- Expected next role: Archivist.
- Recommended next command: `concoct archive`.
