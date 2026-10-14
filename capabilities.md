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

No agent workflow, dispatch policy, review controller or integration operation is delivered. CON-042 remains a frozen, undelivered historical reference.
