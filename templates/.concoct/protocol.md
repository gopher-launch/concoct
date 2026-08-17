---
instruction-layer: protocol
protected-controls:
  - completed-review-immutability
  - evidence-integrity
  - invalid-state-refusal
  - workflow-artifact-ownership
---

# Concoct Protocol

This Concoct-owned layer defines non-overridable controls:

- Durable artifact evidence establishes state; intent and rendered prompts do not.
- Completed sequential reviews are immutable and reviewer-owned.
- Workflow artifacts may be changed only by their designated role.
- Invalid or contradictory evidence prevents mutating transitions.

Policy and project guidance may strengthen these controls but may not weaken
them. Composition validates declared controls; semantic conflicts in unrestricted
prose require explicit reconciliation.

## Interaction discipline

For every role, perform one batched initial discovery pass over the exact
inputs, keep command output focused and bounded, and iterate with the narrowest
relevant checks. Reread evidence or rerun a successful check only for a named
uncertainty or intervening change. Run one required broad completion validation,
record durable results, and stop as soon as the role contract is satisfied.
