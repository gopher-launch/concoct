# Instruction layers

Concoct composes four attributed sources for role prompts:

1. `.concoct/protocol.md` is Concoct-owned protocol.
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

## Ownership and compatibility

Initialization installs an internally consistent protocol and default policy.
`AGENTS.md` stays human-editable and project-owned. Composition reads it
byte-for-byte and never rewrites it, allowing future upgrades to replace
Concoct-owned sources without taking ownership of repository guidance.

Repositories initialized before this boundary may mix project and workflow
content in `AGENTS.md`. They require explicit reconciliation; Concoct does not
infer ownership or silently split the file. Adoption, upgrade, and selectable
policy remain separate roadmap work.
