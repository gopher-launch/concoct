---
task-id: CON-030
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

The implementation establishes an executable-owned registry, moves protocol
and handoff reads behind it, adds deterministic defaults inspection, and stops
installing the former built-in paths. The repository checks pass. However,
rendered role prompts do not include the selected built-in persona even though
personas are no longer installed. Several embedded handoffs still instruct the
agent to read those absent project paths. This breaks the core ownership and
prompt-completeness contract and requires developer remediation.

## Findings

### Major — Rendered prompts omit the executable-owned persona

- Evidence: `internal/prompt/render.go:72` reads only `spec.resource`, which is
  the selected handoff or roadmap prompt. The `roleSpec.persona` value is used
  only as a label at line 79; no `persona-*` resource is read or written to the
  rendered output. At the same time, `internal/project/project.go:129-133`
  excludes every `.concoct/personas/` path from initialization. The embedded
  planning and review handoffs still direct agents to read
  `.concoct/personas/task-planner.md` and `.concoct/personas/reviewer.md`.
  A fresh initialized project confirmed that the persona file is absent and
  its rendered `concoct next` prompt contains no Product Owner persona body.
- Impact: initialized projects cannot supply the selected persona through
  either repository evidence or rendered executable content. Agents receive a
  role label and handoff, but not the canonical role boundaries and operating
  guidance. For several transitions the prompt explicitly references a file
  guaranteed not to exist. This fails the target state and acceptance criterion
  requiring built-in personas to be composed from the executable, and it can
  weaken role ownership behavior in normal use.
- Required outcome: map every selected role to its `persona-*` resource, load
  it through the required-resource integrity boundary, and include its exact
  content with clear executable attribution in every applicable rendered
  prompt. Remove or revise stale repository-persona instructions in embedded
  handoffs so initialized-project prompts are self-contained. Add focused tests
  proving persona content is present, obsolete local persona files cannot
  shadow it, and missing required persona resources fail without fallback.

## Acceptance criteria assessment

- Executable protocol and handoff lookup, deterministic resource listing, raw
  exact-byte inspection, and selective initialization are implemented.
- Local protocol and handoff copies do not participate in prompt rendering.
- Built-in persona composition is not implemented, so standard role prompts
  are not complete when generated-project persona files are intentionally
  absent.
- No overlay, migration, configurable-policy, or direct-agent-execution scope
  was observed.

## Verification performed

- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed; executable mode confirmed.
- `git diff --check 93116c0..HEAD` — passed.
- Fresh initialization outside the repository — reached `ready`, omitted the
  built-in persona path as intended, and rendered `concoct next` from embedded
  guidance; inspection demonstrated that the output lacked persona content.
- Reviewed the complete task diff, resource registry, instruction composition,
  prompt selection/rendering, initialization filtering, tests, goldens,
  documentation, task evidence, and capability truth.

## Scope, compatibility, and documentation

The implementation otherwise remains within CON-030's stated scope. Existing
tests passing does not mitigate the missing persona because the goldens were
updated to remove persona inputs without adding persona bodies, and no focused
test asserts the required composition. Documentation describing built-ins as
executable-owned is directionally correct but overstates actual prompt behavior
until this finding is resolved.

## Capability impact assessment

The planned updates to CAP-003, CAP-004, and CAP-006 remain appropriate after
remediation. The Archivist should not update them yet because executable-owned
persona delivery is incomplete. No new capability ID is warranted.

## Prior finding disposition

No prior review exists for CON-030.

## Handoff

- Current state after completion: changes requested.
- Work completed: independent review and full documented verification.
- Work remaining: compose selected built-in persona content into rendered
  prompts, eliminate stale missing-file directions, and add focused coverage.
- Decision: the missing persona is a major acceptance defect suitable for
  Developer remediation within the active task.
- Known risks: without a test covering every role-to-persona mapping, one
  transition can remain incomplete even if the primary paths are corrected.
- Artifact updated: `.concoct/current/review-01.md` only.
- Expected next role: Developer.
- Recommended next command: `concoct code`.
