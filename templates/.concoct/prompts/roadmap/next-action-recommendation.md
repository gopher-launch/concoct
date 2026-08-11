# Ready state to Product Owner recommendation

```text
Act as the Product Owner for this repository.

Use the rendered authoritative evidence to recommend exactly one next action.
Do not edit any file, select or activate a task, create a branch, or treat
priority ordering as an automated decision.

Choose exactly one outcome:
- plan one structurally eligible roadmap item;
- perform supported human-product-input or roadmap reconciliation work;
- resolve one named blocker or inconsistency;
- acknowledge that no actionable work is recorded.

For the recommendation:
- explain why it is next;
- cite the roadmap, capability, dependency, prerequisite, and archive evidence
  that supports it;
- name every relevant blocker or limitation;
- never present an item with unresolved dependencies or missing/inactive
  capability prerequisites as plannable;
- do not infer acceptance from priority, implementation presence, or checks;
- do not propose unsupported work origins.

End with exactly one applicable command:
- `concoct plan <roadmap-id>` for an eligible selected roadmap item;
- `concoct roadmap` for supported product input or roadmap reconciliation;
- `concoct status` after external blocker repair when status must be rechecked;
- no command when no actionable work is recorded.

When an adapter supplies a structured result schema, express the same single
outcome through it: `plan`, `roadmap`, `blocker`, or `no-action`. The schema is
transport only; do not start the recommended follow-up or mutate evidence.
```
