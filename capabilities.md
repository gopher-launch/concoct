# Delivered capabilities

This ledger describes implemented behavior in the source tree. Task IDs identify the delivery that introduced each capability.

## CON-043 — Buildable reset skeleton

- A Go 1.27.2 module retains the `github.com/gopher-launch/concoct` identity and existing license.
- `concoct` builds and displays help or development version information.
- Workflow and unknown commands exit unsuccessfully with an explicit unavailable message.
- The active repository contains the accepted design, bounded rebuild roadmap and source-first construction guidance.

## CON-057 — Read-only roadmap metadata parser

- Parses task, feature and epic headings with bounded kind, status, priority, parent and dependency metadata while retaining objective and acceptance prose.
- Validates IDs, references, parent kinds and dependency cycles; ignores fenced examples and diagnoses malformed metadata.
- Reports ready task entries only when their dependencies are delivered, including IDs recorded in this ledger. It does not select tasks or launch agents.

## CON-044 — External active-work store

- Canonical Git common-directory and storage-root bindings share one run across worktrees, including repositories with a separate Git directory; one explicit root override is supported.
- A repository execution lock protects mutation while status reads atomic snapshots without that lock.
- Run/task metadata, checkpoints, operator decisions, pending/final reviews and bounded receipts use atomic file writes; interrupted cross-file writes reconcile on reopen.
- Dedicated host methods protect agreed task scope and finalized reviews from ordinary checkpoint or review updates.

## CON-045 — Codex app-server transport

- A Linux stdio client checks for the locally verified Codex 0.147.0 runner, negotiates experimental API support and reads model catalog and rate-limit snapshots.
- Thread start/resume applies explicit role settings and validates effective model, effort, full-access sandbox and approval policy. Zero-turn thread IDs remain provisional.
- Turn execution streams progress and terminal results, retains observed token receipts, and handles bound host callbacks only after validating the response and committing its receipt.
- Turn interruption and background-terminal list, terminate and clean methods are exposed. Process cessation verification and stop policy belong to later controller work.

## CON-046 — Dispatch governance

- Internal policy resolves reagent and iragent model/effort pairs from explicit Concoct invocation, project and user choices over the provider-recommended model; it validates supported efforts and retains full-access settings with active work. Default efforts are medium and high respectively.
- The active-work owner exposes a pre-dispatch reservation method that persists an attempt or a task/role/activity/candidate-bound authorization wait before returning dispatch permission. Default policy grants ordinary selection, engineering, review and correction without a first-review gate; configured gates require explicit matching authorization.
- Twelve total dispatch attempts, three completed independent assessments and two recovery attempts per failure episode are starting bounds. Extensions raise ceilings without resetting consumption. Three reagent self-review passes per candidate cycle are guidance inside a turn, not a runtime dispatch counter.
- Provider quota windows, reset times and observation timestamps are separate from per-attempt token receipts. Configured warnings and next-dispatch holds use observed percentages; unavailable or stale telemetry remains unknown. Watchdogs default off.

## CON-047 — Internal selection and engineering controller

- When no active work exists, the controller requires a clean named parent branch, captures its baseline and prepares an external detached checkout. Existing active work is reported before new selection.
- A fresh reagent thread receives current repository instructions, roadmap and capabilities, eligible task context and an explicit parent binding. Dispatch is durably reserved before provider start.
- The reagent can create one task branch and call `claim_task` before source edits. The host validates eligibility, roadmap scope, unchanged parent and clean baseline checkout, then records the task binding and callback receipt before acknowledgment. The same turn can continue engineering and save bounded checkpoints without changing agreed scope.
- Normal no-work removes the clean unclaimed checkout and releases the active pointer; a material question retains active work. Focused fake-agent and disposable Git tests cover one-turn continuation, competing claims and dirty prerequisites.

## CON-048 — Internal candidate freeze

- A completed reagent turn can request review readiness with a structured final checkpoint. The host checks each task criterion, required project checks with command and source paths, any explicitly accepted unavailable check, self-review and next action. Progress events still stream before verdict validation.
- Readiness also requires resolved host callbacks, an idle provider background-terminal list and source documentation that marks the selected task delivered while preserving IDs and agreed scope. Invalid or uncertain results retain active work and do not create a candidate.
- Candidate capture includes baseline and candidate revisions and trees, a binary diff digest, changed paths and the checkpoint digest. Existing reagent commits are preserved; dirty source can be staged and committed mechanically without another agent turn. Untracked deliverables are included even when ignored by Git; an omitted generated output must be individually declared and match an explicit exclusion.

The CLI workflow, independent review controller, threshold-triggered interruption and integration operation remain planned. CON-042 remains a frozen, undelivered historical reference.
