---
instruction-layer: protocol
protected-controls:
  - completed-review-immutability
  - evidence-integrity
  - invalid-state-refusal
  - workflow-artifact-ownership
---

# Concoct Protocol

This Concoct-owned layer defines controls that policy and project guidance may
strengthen but may not weaken.

- Evidence establishes workflow state; intent or rendered guidance does not.
- Completed sequential review artifacts are reviewer-owned and immutable.
- Each workflow artifact may be changed only by its designated role.
- Missing, malformed, contradictory, or interrupted evidence produces an
  invalid state and prevents mutating transitions.

Free-form prose cannot be checked for every semantic contradiction. The
composer validates the declarations above and attributes all prose to its
source. Suspected prose conflicts require explicit human reconciliation.
