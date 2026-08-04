---
task-id: CON-030
roadmap-id: CON-030
status: archived
archived: 2026-08-04
review: review-04.md
delivery: pending-integration
capability-impact:
  type: update
  ids:
    - CAP-003
    - CAP-004
    - CAP-006
---

# Summary

## Task

Make built-in workflow content executable-owned and selectively initialize only
project-owned outputs.

## Delivered outcome

CON-030 moved built-in protocol, personas, handoffs, and prompt documentation
into a deterministic, inspectable executable resource registry. Prompt
rendering now includes the selected built-in persona and ignores former local
copies; `defaults list` and `defaults show` expose the resources and provenance.
Initialization now installs only project-owned guidance, configuration, truth,
state, and adapters.

The implementation was independently approved by `review-04.md` after all
earlier findings were remediated. Verification included the full Go test suite,
`go vet`, build checks, shell syntax validation, diff checks, fresh-project
initialization, executable-rendered persona output, and stale-contract search.

## Verification

- `go test -count=1 ./...`, `go vet ./...`, and `go build ./cmd/concoct` passed.
- Shell syntax, diff checks, fresh initialization, prompt rendering, and
  active documentation checks passed.

## Review outcome

`review-04.md` approved the completed implementation with no material findings.

## Key decisions

- Built-in resources use stable logical identifiers and executable provenance.
- Initialized projects retain project-owned files and adapters, but do not
  install mutable protocol, persona, or handoff copies.
- Integration into the recorded trunk remains a separate workflow transition.

## Files and areas changed

The executable resource registry, prompt and instruction composition, defaults
inspection commands, ownership-aware initialization, tests, adapters, and
active documentation were updated.

## Capability changes

Capability truth was updated for CAP-003, CAP-004, and CAP-006. The roadmap
item is delivered by this archive. Git archival is complete on the task branch;
integration into `main` remains pending under the recorded workflow.

## Skipped work

Overlays, migration or compatibility behavior, configurable policy, and direct
agent execution remain outside this task and are preserved as future scope.

## Follow-up work

CON-031 will define full release provenance and SemVer metadata; CON-014 may
later add explicit project overlays.
