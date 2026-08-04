---
task-id: CON-030
review: 3
status: changes-requested
created: 2026-08-04
persona: reviewer
---

# Review 03

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The runtime persona-composition defect from Review 01 remains fixed, and the
Review 02 documentation remediation corrects role-command inputs, workflow
guides, repository guidance, and generated adapters. All repository checks and
fresh-project verification pass. One normative initialization contract still
states that generated projects contain personas and prompts as part of the
complete copied template. That directly contradicts both the implementation
and CON-030's selective-ownership acceptance criteria, so Review 02 is only
partially resolved.

## Prior finding disposition

### Review 01 — Rendered prompts omit the executable-owned persona

- Status: `fixed`.
- Evidence: each runtime role resolves and embeds its `persona-*` resource with
  executable attribution; focused tests cover every mapping and reject a local
  shadow. Fresh `concoct next` output includes the complete Product Owner
  persona while no local persona directory exists.

### Review 02 — Normative documentation requires uninstalled persona files

- Status: `partially fixed`.
- Evidence: the command-specific role inputs, workflow guide, multi-agent
  guide, state-machine description, repository guidance, executable-owned
  persona/policy sources, and generated adapters now describe rendered
  executable personas. However, the same normative command reference retains
  the contradictory initialization statements described below.

## Findings

### Major — `init` contract still promises personas and prompts in generated projects

- Evidence: `doc/command-reference.md:79-84` says initialization creates “the
  complete distributed template” and explicitly lists “personas, prompts”
  among `.concoct/` outputs. `internal/project.projectOutput` intentionally
  excludes protocol, persona, and prompt paths. Independent fresh-project
  verification confirmed that `.concoct/personas/` and `.concoct/prompts/` do
  not exist, while the project reaches `ready` and renders its persona from the
  executable.
- Impact: the normative command contract still misstates CON-030's primary
  installed-product boundary. Users and future implementers are told to expect
  the exact mutable built-ins this task removes, and the recorded stale-path
  search incorrectly claims all remaining documentation matches were
  intentional. This leaves Review 02's required command-reference
  reconciliation incomplete.
- Required outcome: update the `concoct init` “Files created or updated” section
  to enumerate only project-owned state, guidance, configuration, and adapters,
  and explicitly distinguish executable-owned built-ins where useful. Re-run
  the active-documentation stale-contract search and record the corrected
  result.

## Acceptance criteria assessment

- Runtime registry, integrity lookup, persona/handoff/protocol composition,
  defaults inspection, shadow resistance, and selective initialization pass.
- Generated role prompts and adapters now use executable-owned personas.
- The normative initialization documentation remains materially inconsistent
  with the selective output manifest, so documentation acceptance is not yet
  complete.
- No excluded overlay, migration, configurable-policy, compatibility, or
  direct-agent-execution behavior was observed.

## Verification performed

- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed; executable mode confirmed.
- `git diff --check 93116c0..HEAD` — passed.
- Fresh initialization outside the repository — reached `ready`, created the
  archive/current state and staged project-owned manifest, omitted protocol,
  personas, and prompts, and rendered the attributed Product Owner persona.
- Targeted active-documentation search — found the stale initialization
  promises at `doc/command-reference.md:81` and `:83`.
- Reviewed the complete task diff, Review 02 remediation diff, task evidence,
  all prior reviews, relevant runtime and tests, documentation, generated
  output, and capability truth.

## Scope and capability impact

The remaining correction is narrowly within CON-030's documentation phase and
does not require runtime changes. After correction, the planned CAP-003,
CAP-004, and CAP-006 updates remain appropriate; no new capability ID is
needed. Capability truth must remain unchanged until approval and archival.

## Handoff

- Current state after completion: changes requested.
- Work completed: third independent review and full verification.
- Work remaining: correct the two stale initialization-output statements and
  repeat the documented stale-contract search.
- Decision: Review 01 is fixed; Review 02 is partially fixed because its
  normative command-reference requirement remains unresolved.
- Known risks: none beyond overlooking another generated-output promise; keep
  the remediation contextual rather than replacing source-tree evidence.
- Artifact updated: `.concoct/current/review-03.md` only.
- Expected next role: Developer.
- Recommended next command: `concoct code`.
