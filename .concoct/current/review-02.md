---
task-id: CON-030
review: 2
status: changes-requested
created: 2026-08-04
persona: reviewer
---

# Review 02

<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->

## Outcome

`changes-requested`

## Summary

The Review 01 implementation defect is fixed: every runtime role now resolves
and embeds its executable-owned persona with provenance, local copies cannot
shadow it, and the generated-project path works without installed personas.
The full verification suite passes. However, several normative and user-facing
documents still define repository-local persona files as required role inputs.
Those files are now intentionally absent from generated projects, so the
documented operating contract contradicts the delivered behavior and requires
another focused remediation.

## Prior finding disposition

### Review 01 — Rendered prompts omit the executable-owned persona

- Status: `fixed`.
- Evidence: `internal/prompt/render.go` derives `persona-<role>`, reads it with
  `defaults.Read`, embeds its exact Markdown under an attributed persona
  section, and lists the persona as an effective built-in source. The focused
  role table covers Product Owner, Task Planner, Developer, Reviewer, and
  Archivist prompts and rejects a repository-local shadow marker. A fresh
  initialized project contained no local Product Owner persona yet rendered
  the complete `# Product Owner Persona` body from
  `built-in:persona-product-owner`.
- Assessment: the required runtime outcome and role-mapping coverage are
  satisfied. Embedded handoffs no longer direct agents to absent local persona
  files.

## Findings

### Major — Normative documentation still requires uninstalled persona files

- Evidence: `doc/command-reference.md:217-223`, `271-278`, `347-354`,
  `406-413`, and `481-488` list repository-local Product Owner, Task Planner,
  Developer, Reviewer, and Archivist persona paths under each command's
  “Files read.” `doc/workflow.md:129-140` tells users to select personas from
  `.concoct/personas/`, including three writer paths that do not match the
  registry's logical/source names. `doc/multi-agent-workflow.md:53-101` likewise
  says prompts select personas from that directory. `AGENTS.md:35` states that
  generated projects place personas under `.concoct/`. In contrast,
  `internal/project.projectOutput` deliberately omits the directory and a
  fresh initialization confirms it is absent.
- Impact: the normative command reference and workflow guidance tell users and
  agents to read files the product no longer installs, contradicting the core
  executable/project ownership boundary. This makes documented manual and
  multi-agent operation fail or encourages obsolete local copies, undermining
  the task's compatibility and no-shadowing goals. It also fails Phase 5's
  ownership-documentation and stale-path verification expectations.
- Required outcome: reconcile the command reference, workflow and multi-agent
  guides, and repository guidance with executable-rendered personas. Describe
  the selected built-in persona as part of rendered prompt content and remove
  claims that generated projects install or require `.concoct/personas/`.
  Search all current non-historical documentation and adapters for former
  built-in paths and either update each operational reference or clearly label
  it as source-tree/legacy context.

## Acceptance criteria assessment

- Executable protocol, handoff, and persona composition now works without
  repository fallback and resists local shadowing.
- Defaults inspection, logical registry ordering, selective initialization,
  and generated-project operation are implemented and passing.
- The current documentation is not yet compatible with the new installed
  contract because multiple active guides require deliberately omitted paths.
- No overlay, migration, configurable-policy, compatibility framework, or
  direct-agent-execution behavior was observed.

## Verification performed

- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed; executable mode confirmed.
- `git diff --check 93116c0..HEAD` — passed.
- Fresh initialization outside the repository — reached `ready`, staged the
  project-owned positive manifest, omitted local personas, and rendered the
  attributed Product Owner persona from the executable.
- `concoct defaults list` — listed the persona resources deterministically with
  development-build provenance.
- Stale-path search across active documentation, adapters, prompt sources, and
  tests — found the contradictory documentation cited above.
- Reviewed the full task diff and remediation diff, current task evidence,
  Review 01, source, tests, generated output, and capability truth.

## Scope and compatibility

The runtime remediation is focused and compatible with CON-030. The remaining
finding is documentation work already required by the active task, not a scope
expansion. Updating active docs should preserve historical archive evidence and
source registry paths where those paths are legitimately implementation detail.

## Capability impact assessment

The planned updates to CAP-003, CAP-004, and CAP-006 remain correct after the
documentation contract is reconciled. No new capability ID is needed. The
Archivist should not update capability truth while active documentation still
misstates the generated-project interface.

## Handoff

- Current state after completion: changes requested.
- Work completed: post-remediation independent review and full verification.
- Work remaining: reconcile active documentation and repository guidance with
  executable-rendered personas, then record a stale-path search.
- Decisions: Review 01 is fixed; the remaining documentation contradiction is
  a major acceptance issue within Developer scope.
- Known risks: broad replacement could corrupt legitimate source-tree paths or
  historical evidence, so each match needs contextual classification.
- Artifact updated: `.concoct/current/review-02.md` only.
- Expected next role: Developer.
- Recommended next command: `concoct code`.
