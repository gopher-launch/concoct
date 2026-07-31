# Concoct

Turn raw ideas into durable, reviewable work for coding agents.

Concoct is a lightweight workflow for developers who want to use coding agents
on substantial work without leaving product intent, decisions, or review
history trapped in a chat. It keeps that context in ordinary repository files,
assigns clear Product Owner, Planner, Developer, Reviewer, and Archivist roles,
and makes every transition inspectable.

The result is a repeatable loop: decide what matters, plan it, implement it,
review it independently, and retain the accepted outcome for the next task.

```text
idea → plan → implement → review → archive → integrate
```

## Is Concoct a fit?

Use Concoct when losing context would be costly: features, architecture
changes, multi-file refactors, public API changes, repository setup, or work
that may span several sessions. For a typo or a one-shot answer, it is probably
more workflow than you need.

Concoct is agent-neutral: its shared repository contract can be followed by
Codex, Claude Code, GitHub Copilot, Aider, and other capable tools. That does not
mean every tool has identical native integration. Concoct renders validated
guidance; it does not launch agents, make product decisions, perform role work,
or establish that a human-authored result is semantically correct. Humans and
agents still supply that judgment.

## Quick start

The normal journey starts with the `concoct` binary available on your `PATH`:

```bash
concoct init hello-world
cd hello-world
```

`init` creates a Git repository, installs and stages the complete workflow, and
writes `.concoct/current/bootstrap-prompt.md`. It does not create the initial
commit. Review the staged files, customize the project guidance, then commit
the bootstrap before continuing.

```bash
git status
git commit -m "Initialize Concoct project"
concoct status
```

You can run `status` from the project root or a nested directory. It validates
the durable evidence, reports the current workflow state, and recommends the
next command without changing anything.

### 1. Decide what comes next

```bash
concoct next
```

In a ready project, `next` renders a Product Owner prompt using the recorded
roadmap, capabilities, dependencies, archive history, and supported human
input. Give that prompt to a human or coding agent to recommend one next
action. The command itself neither chooses work nor edits the roadmap.

For a new idea, render the roadmap-intake prompt:

```bash
concoct roadmap
```

The Product Owner follows that guidance to clarify the idea and update
`.concoct/roadmap.md` when it is ready. The project remains in `ready` state
until a task is planned.

### 2. Plan one eligible roadmap item

```bash
concoct plan APP-001
```

In a Git-backed project, `plan` first validates eligibility and establishes a
deterministic task branch from the current clean branch. It then renders the
Task Planner prompt. The Planner—not the CLI—creates the active task plan and
notes, validates repository assumptions, and leaves the task ready for
implementation.

### 3. Implement the plan

```bash
concoct code
```

`code` renders the Developer prompt for the active task. A developer or coding
agent follows it to make the scoped changes, run checks, record decisions and
results, and leave a durable handoff for review. Finish the authored transition
with `concoct code --complete`; Concoct validates role ownership and resulting
workflow evidence, including a handoff changed from the committed version, then
commits the complete Git-backed transition once.

### 4. Review independently

```bash
concoct review
```

`review` renders guidance for an independent Reviewer. Run
`concoct review --reserve` to create the next append-only review path
exclusively from the validated clean task checkout, complete that reservation
with exactly one outcome (`approved`,
`changes-requested`, or `blocked`), then run `concoct review --complete`.

When changes are requested, run `concoct code` again. The Developer addresses
each finding without rewriting prior reviews, records every disposition, and
hands the task back for the next `concoct review`. Repeat until approved.

### 5. Archive the accepted result

```bash
concoct archive
```

`archive` renders the Archivist prompt; it does not archive the task by itself.
The Archivist preserves the approved plan, notes, reviews, and summary, then
reconciles accepted capability evidence. After authoring those files, run
`concoct archive --complete`; Concoct validates the complete transaction. For a
Git-backed task it commits archival once on the task branch and stops before
delivery to the recorded trunk. An exceptional unapproved transition requires
both `--override-authority` and `--override-reason`, with exactly matching
durable summary evidence.

### 6. Integrate the task

```bash
concoct integrate
```

Unlike the role-prompt commands, `integrate` performs a guarded local Git
transaction. It squash-integrates the recorded archival commit into the exact
trunk, completes delivery bookkeeping, clears current task state, and deletes
the accepted task branch. No remote is required. A matching trunk upstream is
pushed only after confirmation unless automatic push was explicitly enabled.

If integration conflicts, resolve and stage the files yourself, then continue:

```bash
concoct integrate --continue
```

Concoct validates the transaction boundary, but the human remains responsible
for the meaning of the resolution. Use `concoct integrate --abort` to restore
the archived pre-integration state.

## Understand the workflow

The quick start is the representative Git-backed path. These references define
the complete contract and recovery behavior:

- [Workflow](doc/workflow.md) explains the roles, durable artifacts, and Git
  lifecycle.
- [Command reference](doc/command-reference.md) defines command inputs,
  effects, failures, and allowed transitions.
- [State machine](doc/state-machine.md) defines valid evidence, remediation,
  blocked review, archival, and integration recovery.
- [Multi-agent workflow](doc/multi-agent-workflow.md) explains coordination
  across agents and tools.

Role commands (`next`, `roadmap`, `plan`, `code`, `review`, and `archive`) write
deterministic prompts to standard output. Pass `--output <path>` to create a new
file containing the same bytes. Output is create-only: Concoct refuses to
overwrite an existing file. Prompt rendering validates its starting state but
does not persist the selected role's work. The explicit `code --complete`,
`review --reserve`, `review --complete`, and `archive --complete` boundaries validate and persist
role-authored evidence; they never generate implementation or review judgment.

## What Concoct installs

Generated projects use a stable, agent-neutral contract:

```text
AGENTS.md                          # repository-owned instruction entry point
.concoct/protocol.md               # Concoct-owned protected controls
.concoct/policy.md                 # project-selected workflow policy
.concoct/capabilities.md          # accepted product behavior
.concoct/roadmap.md               # intended future outcomes
.concoct/personas/                # role-specific guidance
.concoct/current/task-plan.md     # active implementation contract
.concoct/current/notes.md         # durable decisions and handoffs
.concoct/archive/                 # accepted task history
```

Thin, tool-specific adapters point back to those canonical files:

```text
CLAUDE.md
CONVENTIONS.md
.aider.conf.yml
.github/copilot-instructions.md
.github/prompts/
.codex/skills/concoct/SKILL.md
```

See [instruction layers](doc/instruction-layers.md) for ownership, precedence,
validation, source attribution, and legacy reconciliation.

Concoct-owned state lives under `.concoct/`; conventional instructions and
tool configuration remain at the generated project root.

## Build from source

This repository currently provides source-build and source-checkout usage; it
does not claim a packaged release channel.

Install the current pre-release command directly from the canonical repository:

```bash
go install github.com/gopher-launch/concoct/cmd/concoct@main
```

Or build from a local checkout:

```bash
go build -o ./bin/concoct ./cmd/concoct
./bin/concoct init ../my-new-project
```

From this checkout, `./cmd/concoct/concoct.sh` is a thin compatibility wrapper
around the same Go implementation:

```bash
./cmd/concoct/concoct.sh init ../my-new-project
```

## Repository layout

```text
.
├── AGENTS.md
├── README.md
├── LICENSE
├── cmd/concoct/   # Go command and source-checkout wrapper
├── .concoct/      # Concoct's own workflow artifacts
├── doc/           # workflow and command documentation
└── templates/     # files installed into generated projects
```

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/concoct
bash -n cmd/concoct/concoct.sh
```

Use hyphens rather than underscores in file and directory names. Keep
conventional, long-lived files such as `AGENTS.md`, `README.md`, `CHANGELOG.md`,
and `CONTRIBUTING.md` at the root; use lowercase hyphenated Markdown filenames
for task and workflow artifacts.

## Canonical repository

Concoct's canonical repository and Go module are
`github.com/gopher-launch/concoct`. This source tree retains its existing Git
history under the Gopher Launch organization.
