# Concoct Workflow

## Optional Git task lifecycle

A Git-backed task records `git.enabled`, the exact `trunk`, deterministic
`task-branch`, immutable `base`, and later `archive-commit` and `status` in
task-plan front matter. The branch slug lowercases roadmap ID and title,
replaces non-alphanumeric runs with hyphens, trims and truncates it to 56
characters, then prefixes `concoct/`. Planning refuses dirty, detached,
operation-in-progress, and branch-collision inputs.

The Archivist ends at `archived`, without delivery or current-state cleanup.
The authored archive keeps `task-plan.md` byte-identical to the accepted active
task and leaves current task metadata unchanged. After the Archivist authors
the archive, summary, capability, and roadmap evidence, `concoct archive
--complete` validates the candidate, applies `git.status: archived` and
`git.archive-commit: self` to current task metadata, and commits the transition
as one boundary. The non-recursive `self` value is valid only in that committed
archived task and resolves to the exact checked-out HEAD.
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

## One-shot adapter execution

Manual role prompts remain the portable workflow surface. When Codex is
available, `concoct exec --dry-run` shows the one action currently authorized,
its exact prompt and resolved profile, and the launch safety posture.
`concoct exec` then runs at most that one action and returns control. It never
loops into the next role.

From `ready`, supervised execution captures one Product Owner semantic decision
instead of a disposable command recommendation. The private decision is bound
to the observed evidence and can be inspected without mutation. When the
selected policy requires it, `concoct run --approve next` applies that exact
decision once; a selection then enters the normal separately-gated planning
step. Manual `concoct next` remains a deterministic read-only inspection path.
Reconciliation replacements are record-scoped and digest-bound. The executable
allows only a `candidate` to `planned` roadmap promotion or a delivered
reconciliation backed by an existing accepted archive summary; it never treats
the Product Owner outcome as a general Markdown edit.

The executor binds the action to workflow, policy, Git, configuration, current
task/review, and referenced archive evidence. The adapter receives the exact
manual role prompt as its semantic core, followed by a fixed supervision
appendix, plus a schema-bound correlation. For Planner, Developer, Reviewer,
and Archivist completion, the adapter authors and verifies the candidate while
the outer executable validates role-owned effects, invokes the existing
completion authority, and creates any Git transition commit. Existing manual
completion commands remain available, and actual post-run state outranks the
adapter's claim. Private bounded records under
`.concoct/runtime/invocations/` support `concoct exec inspect`; they are ignored
by Git and are not workflow artifacts.

For Codex-backed actions the record also contains exact rendered prompt bytes,
byte-accounted prompt composition, adapter-native optional usage, duration, and
the self-reported adapter version when available. Codex JSONL is decoded as it
arrives; bounded progress display and raw retention never stop later event
decoding. Progress stores only bounded allowlisted lifecycle labels, never
adapter event payload text. Diagnostic evidence and an unterminated JSONL event
have independent count/byte ceilings; oversized input is discarded through the
next line boundary and decoding resumes with explicit truncation/degradation
evidence. The usage line reports only fields supplied by Codex—input, cached
input, output, reasoning output, and native total—and never converts missing
fields to zero or reconstructs a total. `concoct exec inspect` is
privacy-preserving by default; use `--full-raw` only for local recovery, or
`--json` for the metrics-only comparison export.
Where a consumed gate identifies a prior invocation, metadata records it as a
non-authoritative predecessor link. A terminal `turn.*` event followed by later
events is retained as partial evidence and marked `degraded`, rather than being
silently counted as a complete ordered transcript.
Generated structured-output schemas are recursively checked offline before
Codex starts: every object is closed and requires every property exactly once.
Product Owner decisions explicitly emit `mutations: []` when no mutation is
proposed. A terminal `turn.failed` retains only bounded type/message evidence,
which remains visible in one-shot and run summaries even without usage or an
adapter result; schema/configuration rejection requires correction before retry.

Normalized measurement adds payload-free semantic activity, repeated-operation
fingerprints, command-output byte totals, bounded native usage snapshots, and
explicit availability/provenance. Raw JSONL retention is disabled by default
and remains an explicit private diagnostic. Per-role warnings may be live or
terminal-only; only elapsed, activity, and command-output dimensions support
hard live enforcement. A `budget-exhausted` stop terminates the process group,
preserves partial evidence, rejects late results, and leaves workflow state
unadvanced. Run summaries separate accepted from wasted reported usage.

Composition is recorded as bytes are rendered, never reconstructed by parsing
the final prompt. It attributes generated context, policy/roadmap/capability
evidence, the embedded persona, instruction provenance, input references,
authorized updates, completion contract, and handoff. Exact and
whitespace-normalized duplicate links use digests rather than retaining content
in the comparison export.

Each role has reusable executable-owned guidance. A task prompt selects and
embeds the appropriate persona after composing the attributed protocol, policy,
repository guidance, and active task context described in
`instruction-layers.md`.

## Bounded lifecycle runs

`concoct run` composes the same one-shot action boundary into a bounded loop.
It re-detects workflow, Git, policy, configuration, role, prompt, and adapter
before every action. Ready state can execute only the Product Owner decision;
a valid semantic decision is stored privately and stops at the invariant `next`
gate when approval is required. A retained approved selection resumes Task
Planner work directly; it never invokes Product Owner again. Plan acceptance
and local integration are gated by default, while project
configuration and invocation flags may add finite gates or lower the hard
20-action and three-review-cycle bounds.

Only the current gate is retained under
`.concoct/runtime/pending-gate.json`. It is mode `0600`, bounded, atomic,
Git-ignored, one-use, and tied to the exact evidence fingerprint. It is not
workflow truth, action history, or crash-recovery evidence. When multiple gates
protect the same forthcoming action, the record may carry the already-consumed
prerequisite gate so a later approval cannot erase or repeat it. Every
recurring action occurrence still requires a fresh gate and attempt. Every Reviewer uses
a fresh adapter invocation. A changes-requested review is productive progress;
failures are not retried automatically. Integration started by `run` never
pushes a remote. Manual prompts, `exec`, completion commands, and manual
integration remain available unchanged.

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
