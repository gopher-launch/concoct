---
instruction-layer: policy
required-phases:
  - product-ownership
  - task-planning
  - development
  - independent-review
  - archival
  - integration
approval-gates:
  - reviewer-approval-before-archive
  - archive-before-integration
git-strategy: task-branch-with-squash-integration
---

# Default Concoct Policy

This project-selected policy reproduces Concoct's accepted default lifecycle.
Substantial tasks pass through Product Owner, Task Planner, Developer,
independent Reviewer, and Archivist roles. Review approval is required before
archival. Git-backed tasks use a clean recorded task branch, commit each role
transition, archive before integration, and squash onto the recorded trunk;
integration is a required lifecycle activity and resolves not-applicable only
for non-Git tasks.

The active persona, task plan, notes, and sequential reviews provide the
role-specific and task-specific detail. General policy selection is reserved
for future work; this file is currently the supported default.
