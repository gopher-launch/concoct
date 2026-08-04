---
task-id: CON-018
roadmap-id: CON-018
status: archived
archived: 2026-08-04
review: review-02.md
delivery: pending-integration
capability-impact:
  type: update
  ids:
    - CAP-001
    - CAP-005
    - CAP-006
    - CAP-007
    - CAP-008
    - CAP-009
    - CAP-010
    - CAP-011
---

# Summary

## Task

Configure a finite, typed workflow policy while preserving Concoct's protocol
invariants and default lifecycle.

## Delivered outcome

Concoct now composes and validates a closed policy model, resolves every
governed activity from policy and durable evidence, and uses that shared result
for status, prompts, recommendations, completion, archival, and integration.
Explicit reasons and safe evidence are required for supported omissions or
external satisfaction; contradictory or unsafe evidence is rejected.

## Key decisions

- Keep lifecycle policy finite and declarative rather than allowing arbitrary graphs.
- Preserve protocol controls, review immutability, capability reconciliation,
  and task-branch squash integration as unconditional boundaries.
- Support selectable independent-review disposition while retaining the
  existing default policy byte-for-byte.

## Files and areas changed

Updated instruction composition, policy resolution, workflow state and
transitions, prompts, completion boundaries, archive and integration checks,
tests, documentation, and project policy templates.

## Verification

`go test -count=1 ./...`, `go vet ./...`, `go build ./cmd/concoct`,
`bash -n cmd/concoct/concoct.sh`, executable-mode validation, fresh project
initialization, template/resource checks, and `git diff --check` passed.

## Review outcome

`review-02.md` approved the implementation with no material findings after the
archival evidence-integrity finding from `review-01.md` was fixed.

## Capability changes

Updated CAP-001, CAP-005, CAP-006, CAP-007, CAP-008, CAP-009, CAP-010, and
CAP-011 to reflect policy-aware workflow truth, prompts, completion, Git
applicability, and archival validation.

## Skipped work

Alternate Git strategies, profiles, task origins, adoption, upgrades,
workflow explanation, orchestration, and direct agent execution remain owned
by their roadmap items.

## Follow-up work

Delivery remains pending `concoct integrate` on the recorded task branch.

## References

- Roadmap item: `CON-018` in `.concoct/roadmap.md`
- Approving review: `review-02.md`
- Active policy: `.concoct/policy.md`
