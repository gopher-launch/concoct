# Delivered capabilities

This ledger describes delivered behavior, including the proposed post-merge CON-057 candidate. CON-057 still requires independent review and operator-authorized integration.

## CON-043 — Buildable reset skeleton

- A Go 1.27.2 module retains the `github.com/gopher-launch/concoct` identity and existing license.
- `concoct` builds and displays help or development version information.
- Workflow and unknown commands exit unsuccessfully with an explicit unavailable message.
- The active repository contains the accepted design, bounded rebuild roadmap and source-first construction guidance.

## CON-057 — Read-only roadmap metadata parser

- Parses task, feature and epic headings with bounded kind, status, priority, parent and dependency metadata while retaining objective and acceptance prose.
- Validates IDs, references, parent kinds and dependency cycles; ignores fenced examples and diagnoses malformed metadata.
- Reports ready task entries only when their dependencies are delivered, including IDs recorded in this ledger. It does not select tasks or launch agents.

No agent workflow, provider transport, active-work store, review controller or integration operation is delivered. CON-042 remains a frozen, undelivered historical reference.
