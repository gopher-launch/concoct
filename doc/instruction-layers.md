# Instruction layers

Concoct composes four attributed sources for role prompts:

1. The executable's `built-in:protocol` is Concoct-owned protocol.
2. `.concoct/policy.md` is project-selected workflow policy.
3. `AGENTS.md` is repository-owned project guidance and the conventional entry point.
4. The selected persona and active artifacts are task context.

Protocol has highest workflow precedence. Policy selects lifecycle phases,
approval gates, and Git behavior within protocol bounds. Project guidance owns
naming, architecture, coding standards, and verification, and may add a stricter
compatible constraint. Task context narrows work without overriding other layers.

## Validation and conflicts

Each source begins with an `instruction-layer` declaration. Protocol protects
evidence integrity, completed-review immutability, workflow-artifact ownership,
and invalid-state refusal. Policy declares required phases, approval gates, and
Git strategy. Policy or project guidance may list a protected control under
`strengthen-controls`; listing one under `weaken-controls` is rejected.

Declaration keys are owned by their layer. Protocol accepts its layer identity
and protected controls; policy accepts its layer identity, phases, gates, Git
strategy, and explicit strengthening or weakening declarations; project
guidance accepts only its layer identity and explicit strengthening or weakening
declarations. A policy-owned key in `AGENTS.md`, or any other unsupported key,
is rejected with the owning layers and source paths identified. Repository
conventions belong in the Markdown body rather than declaration front matter.

Composition uses the fixed order above, reports every source, and returns no
partial guidance when a source is missing, malformed, or weakens an invariant.
Diagnostics identify the invariant, ownership layers, source paths, and corrective
action. Structured conflicts fail explicitly; there is no last-writer-wins rule.

The validator does not claim to prove arbitrary semantic contradictions in
free-form Markdown. Such prose remains source-attributed and requires explicit
human reconciliation.

## Typed policy and activity evidence

`required-phases` is a closed list: `product-ownership`, `task-planning`,
`development`, `independent-review`, `archival`, and `integration`.
The supported lifecycle requires product ownership, task planning, development,
archival, and integration. Independent review is the one selectable activity:
it may remain required or be explicitly omitted with a durable reason. The only
managed `git-strategy` is `task-branch-with-squash-integration`. The known
approval gates are `reviewer-approval-before-archive` and
`archive-before-integration`; the former is present exactly when review is
required, and the latter is mandatory. Unknown values, duplicates, unsupported
omissions, and incompatible combinations are rejected before command output or
state is returned.

Status resolves every activity deterministically as `completed`,
`not-required`, `not-applicable`, `externally-satisfied`, or `blocked`.
An omitted independent review is explicitly `not-required` by the policy; an
absent task artifact is never a skip. Integration is `not-applicable` only for
a non-Git task. Every rendered activity names `.concoct/policy.md` as its
requirement source; task evidence and canonical artifact reasons remain visible
alongside that attribution.

Every omission must have exactly one `not-required-reasons` list entry in the
form `activity: reason`. Reasons are policy-owned durable evidence: an unknown,
duplicate, blank, or required activity reason is rejected during composition.

An active task may record an externally satisfied independent review in its
front matter under `policy-activity-evidence`. The entry must name the required
`independent-review` activity, provide a non-empty reason, be recorded by the
Task Planner or Developer, and cite readable safe repository-relative evidence.
No path component may be a symbolic link, so a regular file reached through a
symlinked parent cannot escape the repository. Invalid evidence is diagnosed
but never rendered as satisfied. The entry cannot contradict a `not-required`
selection. Other activities require their canonical lifecycle evidence and
cannot be externally satisfied. A valid external review permits archival
without inventing a mutable review file.

## Ownership and compatibility

Initialization installs project-owned policy and guidance. The executable supplies
protocol, personas, and handoffs from embedded resources; obsolete copies at their
former paths are ignored rather than migrated or removed. Use `concoct defaults
list` to inspect resource IDs and `concoct defaults show <logical-id>` to print
their exact bytes.
`AGENTS.md` stays human-editable and project-owned. Composition reads it
byte-for-byte and never rewrites it, allowing future upgrades to replace
Concoct-owned sources without taking ownership of repository guidance.

Repositories initialized before this boundary may mix project and workflow
content in `AGENTS.md`. They require explicit reconciliation; Concoct does not
infer ownership or silently split the file. Adoption, upgrade, and selectable
policy remain separate roadmap work.
