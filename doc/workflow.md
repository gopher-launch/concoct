# Concoct Workflow

## Optional Git task lifecycle

A Git-backed task records `git.enabled`, the exact `trunk`, deterministic
`task-branch`, immutable `base`, and later `archive-commit` and `status` in
task-plan front matter. The branch slug lowercases roadmap ID and title,
replaces non-alphanumeric runs with hyphens, trims and truncates it to 56
characters, then prefixes `concoct/`. Planning refuses dirty, detached,
operation-in-progress, and branch-collision inputs.

The Archivist ends at `archived`, without delivery or current-state cleanup.
After the Archivist authors the archive, summary, capability, roadmap, and
current metadata, `concoct archive --complete` validates them as one boundary.
The non-recursive `archive-commit: self` value is valid only in a committed
archived task and resolves to that exact checked-out HEAD.
`concoct integrate` squash-integrates the recorded archive commit into the
recorded trunk. Recovery evidence under `.git/concoct/integrations/` supports
human-resolved `--continue` and exact `--abort`; it is removed only after final
bookkeeping and branch cleanup. Non-Git projects retain the unbranched flow.

Recovery advances through prepared, squashed, integrated, and delivered
checkpoints, so `--continue` can resume without duplicate commits. A matching
trunk upstream prompts before push unless `.concoct/config.yaml` contains
`git.auto-push: true`; no or non-matching upstream remains local-only success.

`Concoct` turns ideas into implementation-ready plans that any capable coding agent can execute.

## Policy-selected lifecycle

The executable composes `.concoct/policy.md` into a closed typed activity model
before detecting state or rendering a role. Product ownership, task planning,
development, archival, and managed integration remain required. Independent
review may remain required, be explicitly non-required with a policy-owned
reason, or be externally satisfied by authorized task evidence whose readable
repository-relative path contains no symbolic-link component. Status and
non-default role prompts show each activity's requirement, disposition, reason,
and policy source.

Missing artifacts never imply a skip. Invalid or contradictory policy evidence
refuses new commands without being rendered as satisfied. An already-recorded
Git integration recovery remains available through `--continue` or `--abort`
even if current guidance becomes invalid, because recovery preserves or unwinds
an existing transaction rather than authorizing a new lifecycle transition.

## The loop

```text
Idea
  ↓
Product Owner adds the idea to the product roadmap
  ↓
Planner turns the product roadmap into task artifacts
  ↓
Developer implements tasks
  ↓
Reviewer checks the result when required
  ↓
Archivist records the outcome
  ↓
Writers document the project
  ↓
Eatin' big time
```

In short: `idea → concoct → eatin’ big time`.

Concoct's source assets live in this repository's root-level directories.
Projects initialized by Concoct keep project-owned task state under
`.concoct/`, with `AGENTS.md` and tool-required adapters at the project root.
The executable supplies built-in protocol, persona, and handoff content when it
renders a role prompt; those resources are not installed as project files.

The roles may be handled by different tools or by the same tool in different modes.

Each role has reusable executable-owned guidance. A task prompt selects and
embeds the appropriate persona after composing the attributed protocol, policy,
repository guidance, and active task context described in
`instruction-layers.md`.

## Durable files

The workflow depends on durable files in the repository.

### Canonical project instruction

```text
AGENTS.md
```

This is the source of truth for project-level agent instructions.

It should describe:

- project intent
- design principles
- package boundaries
- coding style
- naming conventions
- verification commands
- planning workflow expectations

### Active task files

```text
.concoct/current/task-plan.md
.concoct/current/notes.md
```

These describe the current task and durable task context.

### Product direction and current truth

```text
.concoct/roadmap.md
.concoct/capabilities.md
```

The roadmap contains outstanding future outcomes. Its `Depends on` fields
express only unresolved ordering between outstanding deliveries; `Capability
prerequisites` reference accepted behavior already recorded in the capability
ledger. Completed delivery provenance belongs in capabilities and archives, so
delivered items leave the roadmap after Product Owner reconciliation.

### Archive files

```text
.concoct/archive/YYYY-MM-DD-roadmap-id-short-task-name/
  task-plan.md
  notes.md
  summary.md
```

These preserve completed work for future reference.

## Agent-specific adapters

Different tools may read different instruction files.

Keep those files thin.

They should point back to `AGENTS.md` instead of duplicating project rules.

Examples:

```text
CLAUDE.md
CONVENTIONS.md
.github/copilot-instructions.md
.codex/skills/concoct/SKILL.md
```

## Personas

Use one executable-rendered role persona at a time for the primary work being
performed: Task Planner for planning, Developer for implementation, Reviewer
for independent review, and the audience-selected writer persona for
documentation. Read the persona included in the rendered task prompt before
starting the role. If a task spans implementation and documentation, use the
Developer persona for implementation and then explicitly switch to the
appropriate writer persona for the documentation pass.

## When to use planning files

Use planning files for:

- architecture changes
- refactors touching multiple files
- public API changes
- new features
- repository setup
- multi-session work
- tasks where losing context would be costly

Skip planning files for:

- typo fixes
- tiny one-file edits
- quick explanations
- throwaway experiments

## Good operating principle

Planning files are working memory, not bureaucracy.

Write down what helps future work.

Do not log noise.
