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

Substantial tasks pass through Product Owner, Task Planner, Developer,
independent Reviewer, and Archivist roles. Approval precedes archival.
Git-backed tasks use a clean recorded task branch, committed role transitions,
archival before integration, and squash integration onto the recorded trunk.
Integration is a required lifecycle activity and is not applicable only to
non-Git tasks.

Role details are executable-owned and rendered into role prompts; task context
lives in `.concoct/current/`. This is the supported default policy.
