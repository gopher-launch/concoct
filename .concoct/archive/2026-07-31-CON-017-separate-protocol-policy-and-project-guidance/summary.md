---
task-id: CON-017
roadmap-id: CON-017
status: archived
archived: 2026-07-31
review: review-02.md
delivery: pending-integration
capability-impact:
  type: update
  ids:
    - CAP-001
    - CAP-003
    - CAP-004
    - CAP-006
---

# Summary

## Task

Separate Concoct-owned protocol invariants, project-selected workflow policy,
repository-owned guidance, and active task context while preserving `AGENTS.md`
as the conventional project entry point.

## Delivered outcome

Concoct now represents protocol, policy, and project guidance as distinct,
attributed instruction layers. The protocol protects evidence integrity,
completed-review immutability, workflow-artifact ownership, and invalid-state
refusal. The shipped policy records the accepted phases, approval gates, and
Git strategy, while `AGENTS.md` remains project-owned and byte-preserved.

Deterministic composition loads the layers in fixed order, retains compatible
strengthening, and rejects missing, malformed, unsupported, conflicting, or
invariant-weakening structural declarations before returning partial guidance.
Prompt rendering and initialization validate the same composition, installed
adapters route through it, and documentation states the explicit limit that
free-form prose conflicts still require human reconciliation.

## Key decisions

- Used separate human-readable Markdown artifacts with a deliberately narrow
  declaration schema rather than attempting semantic analysis of arbitrary prose.
- Kept `AGENTS.md` repository-owned and made protocol and default policy
  Concoct-owned inputs that can later evolve without silently overwriting it.
- Rejected policy-owned declarations in project guidance with diagnostics that
  identify both owners and sources; unknown declarations also fail explicitly.
- Preserved the existing lifecycle as the only shipped policy and left policy
  selection, migration, adoption, overlays, and upgrades to future work.

## Files and areas changed

- Added protocol and policy artifacts to the source and reusable template.
- Added deterministic instruction composition and validation under
  `internal/instruction`, with project initialization and prompt integration.
- Updated prompt fixtures, tool adapters, personas, workflow prompts, and their
  embedded template counterparts to use current layered sources and review paths.
- Added `doc/instruction-layers.md` and reconciled README and workflow,
  state-machine, and multi-agent documentation.
- Added focused composition, preservation, conflict, initialization, and prompt
  rendering coverage.

## Verification

- `go test -count=1 ./...`, `go vet ./...`, and `go build ./cmd/concoct` passed.
- `bash -n cmd/concoct/concoct.sh` passed and executable mode was preserved.
- Fresh initialization copied root files, dotfiles, nested templates,
  protocol/policy, personas, prompts, planning files, and bootstrap guidance;
  staged all generated files; created no commit; and reported `ready`.
- Customized `AGENTS.md` bytes, source/template consistency, conflict failure,
  no-partial-output behavior, stale-path searches, and `git diff --check` passed.
- The Archivist confirmed the recorded task branch, base ancestry, clean
  pre-archive state, sequential review history, and deterministic destination.

## Review outcome

`review-02.md` approved the implementation after remediation. Review 01 found
that project guidance could silently declare a policy-owned value; remediation
added closed per-layer declaration schemas, actionable owner/source diagnostics,
deterministic unsupported-key validation, and focused all-or-nothing tests.
Review 02 confirmed the finding fixed and reported no material remaining issue.

## Capability changes

- Updated CAP-001 to describe the explicit protocol, policy, project-guidance,
  and task-context ownership model.
- Updated CAP-003 to describe the reusable layered template and corrected its
  stale-reference limitation.
- Updated CAP-004 to describe adapters consuming the same attributed effective
  instruction set and removed the resolved stale GitHub prompt limitation.
- Updated CAP-006 to describe validated deterministic instruction composition,
  byte preservation, all-or-nothing structural conflict handling, and the
  honest limit on free-form semantic analysis.

## Skipped work

Selectable policies, adoption, upgrades, overlays, migration of existing mixed
guidance, general semantic conflict proof, agent execution, concurrent tasks,
and schema redesign remain outside CON-017.

## Follow-up work

Delivery remains pending `concoct integrate`. CON-017 stays active and current
task evidence remains intact until integration succeeds. CON-018 can build
configurable policy on the accepted ownership boundary; CON-030 and CON-013 can
use it for executable-owned content and safe upgrades.

## References

- Roadmap item: `CON-017` in `.concoct/roadmap.md`
- Approving review: `review-02.md`
- Prior review: `review-01.md`
- Updated capabilities: `CAP-001`, `CAP-003`, `CAP-004`, and `CAP-006`
- Layer contract: `doc/instruction-layers.md`
- Workflow documentation: `doc/workflow.md`, `doc/state-machine.md`
