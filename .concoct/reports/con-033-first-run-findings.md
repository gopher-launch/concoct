# CON-033 First Real-Run Findings

- Run date: 2026-08-13
- Test task: CON-036 — Make capability-ledger change detection record-aware
- Outcome: completed through local integration and returned to `ready`
- Purpose: preserve the complete findings from the first real end-to-end use of
  `concoct run` before consolidating them into prioritized roadmap work

## Executive summary

The first real `concoct run` completed the supported lifecycle:

```text
Product Owner
→ next approval
→ planning
→ plan approval
→ development
→ independent review
→ development remediation
→ independent re-review
→ archival
→ integration approval
→ local integration
→ ready
```

The test validated the overall architecture. Concoct selected the correct roles,
enforced meaningful human gates, preserved workflow integrity, reconciled repaired
durable evidence, supported a productive code/review cycle, integrated locally, and
returned to `ready`.

It did not yet deliver the intended hands-off experience. Manual intervention was
required at most supervised completion boundaries. The dominant problems were not
the lifecycle state model itself, but the preparation, completion, recovery, and
reporting mechanics surrounding agent-authored candidates. Token consumption was
also high enough to threaten Concoct's viability as a daily execution engine.

## Successful behaviour

### End-to-end orchestration

`concoct run` successfully coordinated Product Owner, Task Planner, Developer,
Reviewer, Archivist, and Integrator actions across one complete task lifecycle.

### Approval gates

The following gates stopped before their protected actions and continued only after
explicit approval:

- `next`
- `plan`
- `integration`

The final integration boundary behaved as intended:

```text
concoct run
→ approval required before integration

concoct run --approve integration
→ local integration completed
```

### Evidence-driven reconciliation and resumption

After manual recovery at several boundaries, `concoct run` re-evaluated current
repository and workflow evidence and selected the correct next action. It did not
blindly replay an old invocation.

Examples:

- After manual Developer recovery and `concoct code --complete`, `run` invoked the
  Reviewer.
- After manually completing a review transition, `run` invoked remediation
  development.
- After archive completion, `run` detected the integration gate.
- After integration, the workflow returned to `ready`.

### Independent review

Review 01 found a genuine major defect: required capability metadata was validated
in the candidate ledger but not in the baseline ledger. The remediation Developer
fixed the issue and added regression coverage. Review 02 approved the implementation.

### Protective validation

Executable validation correctly rejected incomplete or incorrectly attributed
evidence, including:

- incomplete Developer transition metadata;
- unreserved review artifacts;
- an empty required archive-summary section;
- capability provenance assigned to the wrong record;
- incorrect roadmap archive-reference syntax.

### Work preservation

Implementation, review, and archive candidates survived rejected transitions and
could be inspected and manually repaired. No implementation work was lost.

## Findings

### Candidate selection cannot start from `candidate`

The intended lifecycle is:

```text
candidate
→ Product Owner proposal
→ approve next
→ planned
→ planning
```

The current Product Owner contract considers only `planned` items structurally
eligible. A candidate therefore requires a human to mark it `planned` before the
Product Owner can recommend it. This makes the new `next` approval gate unable to
perform the selection transition it was designed to protect.

Required behaviour:

- The Product Owner may propose an eligible `candidate` item.
- The proposal is retained as pending selection evidence.
- `concoct run --approve next` changes that exact item to `planned`.
- The same invocation proceeds directly into planning.

### Delivered roadmap items remain active

After delivery, the completed task remained `active` in the roadmap. This created
contradictory ready-state evidence and caused the next Product Owner invocation to
request roadmap reconciliation.

The owner of final roadmap removal needs to be explicit. A likely lifecycle is:

- Archivist records the item as pending integration.
- Integration completes delivery.
- Integration or a deterministic post-integration reconciliation removes the
  delivered item from the outstanding roadmap.

Returning to `ready` must not leave completed work active in the roadmap.

### Developer finalization failed after successful agent work

Both initial development and review remediation produced substantive implementation
candidates, but `run` failed while finalizing the supervised Developer transition.

Initial development did not leave valid `implementation-complete` task evidence.
Review remediation did not add the required relationship:

```yaml
remediates-review: review-01.md
```

Required behaviour and guidance:

- Initial development sets `status: implementation-complete` when work is complete.
- Remediation also identifies the exact latest review in `remediates-review`.
- Notes contain explicit completed dispositions for review findings.
- Developer instructions make these machine-readable requirements prominent.
- The outer executable validates and commits the candidate after the agent supplies
  all required evidence.

### Failed development strands a dirty candidate

After Developer finalization failed:

```text
concoct code --complete
→ rejected candidate

concoct code
→ rejected dirty worktree
```

Concoct created a state that no supported command could resume. Recovery required a
custom Developer prompt that explicitly authorized the existing dirty candidate.

Required behaviour:

- Recognize a dirty worktree produced by a retained supervised invocation.
- Permit `concoct code` or an explicit recovery command to resume that candidate.
- Distinguish authorized candidate changes from unrelated worktree changes.
- Preserve and safely resume work after failed, interrupted, or cancelled role
  completion.

### `run` does not reserve review artifacts before reviewer invocation

The supported manual sequence is:

```text
concoct review --reserve
→ Reviewer completes the reserved artifact
→ concoct review --complete
```

`run` instead invoked the Reviewer without first reserving the next review path. The
Reviewer created a substantively valid review, but completion rejected it because it
lacked executable-owned reservation provenance. This occurred for both review-01 and
review-02.

Required orchestration:

1. Establish the exclusive review reservation.
2. Render the Reviewer prompt against that reservation.
3. Invoke the Reviewer.
4. Validate that exactly the reserved artifact was completed.
5. Complete the review transition.

### Review recovery deadlocks on a dirty worktree

After an unreserved review artifact was created:

```text
concoct review --complete
→ rejected missing reservation provenance

concoct review --reserve
→ rejected dirty worktree
```

Manual recovery required temporarily moving the completed review, creating the
reservation, restoring the review content, and retaining this marker:

```html
<!-- Replace status: reserved with exactly one supported outcome and complete the review. -->
```

The reservation was therefore represented by document content as well as the path.
Recovery guidance and orchestration must preserve this provenance automatically.

### Developer-completion errors can be empty

Failures were reported as:

```text
developer output is not a valid implementation-complete transition:
```

The empty suffix occurs when workflow detection returns a valid non-complete state
without diagnostics. Completion errors must always include the observed state,
expected state, task status, latest review, current next action, and any missing
remediation relationship.

### Candidate state and durable state are blurred in run summaries

Summaries reported transitions such as:

```text
completed -> review-changes-requested
```

even when the protected completion boundary subsequently rejected the candidate.
Reporting should distinguish:

```text
Agent candidate outcome: changes-requested
Durable transition: rejected
Workflow state: unchanged
```

### Review-cycle accounting remained zero

The lifecycle completed:

```text
review-01 changes-requested
→ Developer remediation
→ review-02 approved
```

Run summaries nevertheless continued to report:

```text
review cycles 0/3
```

Cycle bounds must remain meaningful across paused and resumed run invocations. A
workflow must not evade the configured cycle limit merely because each action is
performed by a new `concoct run` process.

### Archivist omitted required summary content

The archive candidate contained an empty required section:

```markdown
## Skipped work
```

Archive completion succeeded past this check after it was changed to:

```markdown
## Skipped work

None.
```

Archivist guidance should require meaningful content in every required section and
explicitly permit `None.` where appropriate.

### Archivist attributed provenance to the wrong capability

CON-036 declared an update to CAP-011, but the Archivist inserted CON-036 archive
provenance into CAP-001. Archive validation correctly rejected the candidate because
CAP-011 did not cite the archive.

Archivist guidance and validation context should bind each declared capability impact
to its stable identifier and expected provenance update.

### Roadmap and capability archive-reference syntax is inconsistent

Capability provenance expects the summary file:

```markdown
- Updated by: `.concoct/archive/.../summary.md`
```

Roadmap archive metadata expects the archive directory:

```markdown
- Archive: `.concoct/archive/.../`
```

The roadmap parser appends `summary.md` internally. Supplying a summary path therefore
produced an effective `summary.md/summary.md` value. The syntax should be standardized,
both forms should be accepted safely, or errors should show the required source form.

### Archivist recommended integration before archive completion

The Archivist recommended `concoct integrate` while also acknowledging that the outer
executable still owned `concoct archive --complete`. Recommendations must describe the
actual next boundary and must not skip executable-owned completion.

### Structured recommendation kinds were empty

Developer, Reviewer, and Archivist outcomes frequently supplied a concrete command
with an empty recommendation `kind`. The structured contract should require a
recognized kind whenever a command is present, or omit the recommendation when the
outer executable owns the next transition.

### Date-sensitive test fixtures

The full Go test suite reproduced two unrelated `internal/runloop` failures because
fixtures were pinned to 2026-08-12 while archive validation used 2026-08-13.

Tests should use an injected clock, derive fixture dates from a shared test clock, or
otherwise avoid wall-clock rollover failures.

### Progress output is too noisy and eventually appears frozen

Codex non-interactive execution emits a large live progress stream. Concoct forwards
too much raw output and truncates captured logs at a fixed size. Once the limit is
reached, useful visible progress stops even though the agent continues working.

Display and diagnostic retention are separate concerns:

- Terminal display should contain selected, compact, continuously updated progress.
- Diagnostic retention should preserve the structured event stream and stderr up to
  independently configured limits.

The Codex adapter should consume JSONL execution events, retain them separately from
the final structured result, and render bounded semantic progress rather than a raw
byte stream.

## Token consumption

### Observed usage

Explicitly observed invocations included:

| Invocation                         |  Tokens |
| ---------------------------------- | ------: |
| Initial blocked Product Owner run  |  29,513 |
| Successful Product Owner selection |  29,627 |
| Review-remediation Developer       |  76,142 |
| Archivist                          |  45,838 |
| Known subtotal                     | 181,120 |

This subtotal excludes planning, initial development, both reviews, recovery
development, other lifecycle invocations, and human/assistant diagnosis. The complete
test therefore consumed materially more than 181,120 tokens.

The user is currently consuming more than half of the available Plus-plan usage in
less than two days. At that rate, Concoct is not viable as a daily execution engine.
This is a critical product blocker, not merely an optimization opportunity.

### Likely contributors requiring measurement

- Repeated inclusion of large executable-owned protocol, persona, handoff, and
  supervision instructions.
- Repeated inclusion of the full roadmap and capability ledger.
- Repeated reading of task plan, notes, reviews, archives, and repository guidance.
- Repository rediscovery by every role.
- Broad verification and large tool-output capture.
- Duplicate or overlapping instructions across layers.
- Failed mechanical transitions causing additional full agent invocations.
- High-cost model and reasoning profiles used uniformly across roles.
- Insufficient selection of task-relevant context.

These are hypotheses. The next critical investigation must measure prompt composition,
reported input and cached-input tokens, output and reasoning tokens, tool activity,
duration, and retry relationships per role before choosing optimizations.

## Integration reporting clarification

After successful local integration, the run summary reported the pre-integration task
branch snapshot:

```text
Repository state: branch concoct/con-036-... at 1ad15e1aa7b2 (clean)
```

Immediate shell inspection confirmed that the actual checkout was correctly on
`main`, clean, and ahead of `origin/main` by the expected commits. The subsequent push
succeeded. This is therefore classified as a reporting snapshot inconsistency, not an
integration failure or repository-state defect.

Post-action reporting should nevertheless refresh repository state so the summary
describes the actual final checkout and revision.

## Manual interventions required

The first real run required these workarounds:

1. Manually mark CON-036 `planned` because candidate proposal/approval was not
   supported end to end.
2. Manually reconcile the previously delivered roadmap item out of active roadmap
   state.
3. Recover and complete the initial Developer transition with a custom prompt.
4. Manually reserve and reconstruct review-01 while preserving reservation provenance.
5. Add `remediates-review: review-01.md` to complete remediation development.
6. Manually reserve and reconstruct review-02 while preserving reservation provenance.
7. Populate the archive summary's empty `Skipped work` section.
8. Move CON-036 provenance from CAP-001 to the declared CAP-011 record.
9. Correct the roadmap Archive value from a summary-file reference to a directory
   reference.
10. Complete archival and approve local integration.

## Overall assessment

The first real run successfully proved the state-machine and evidence-driven
orchestration architecture. The system completed a full lifecycle, protected human
authority, found and remediated a real defect, preserved work through failures, and
integrated locally.

It did not yet prove the intended low-attention UX. The orchestration layer does not
consistently establish, supervise, finalize, or recover executable-owned role
boundaries, and its prompt/context strategy consumes an unsustainable amount of the
user's available usage allocation.

The principal conclusion is:

> The state machine and evidence model are substantially sound. Supervised role
> boundaries and recovery require critical repair, while invocation cost requires
> immediate measurement and reduction before Concoct can operate as a practical daily
> engine.

## Recommended follow-up grouping

This report intentionally preserves findings without assigning roadmap identifiers.
The findings should later be consolidated into a small set of coherent items:

1. Measure and reduce agent invocation cost.
2. Make supervised orchestration boundaries transactional and recoverable.
3. Complete ready-to-ready lifecycle reconciliation.
4. Harden orchestration contracts, diagnostics, accounting, fixtures, and reporting.
5. Improve structured progress rendering and retained execution diagnostics, either
   within the cost-instrumentation work or as a separately justified item.

# Post CON-037 Findings

## Quantified context amplification

Replace the earlier token hypotheses with the measured CON-037 evidence:

- 2,575,638 total input tokens across Developer, Reviewer, and Archivist.
- 2,368,768 cached input tokens—approximately 92%.
- Only 35,870 rendered prompt bytes.
- The 6.5 KB Developer prompt amplified into 1,742,734 processed input tokens.
- Activity count correlated much more strongly with consumption than initial prompt size.

This changes context amplification from a hypothesis into an observed critical defect.

## Invocation records lack semantic activity detail

CON-037 records aggregate usage and generic events, but not:

- semantic item types;
- tool commands;
- repeated file reads;
- command-output sizes;
- per-exchange usage;
- compaction events.

This is a specific observability gap now addressed by CON-038.

## Oversized narrow-role personas

The measured prompt composition showed:

- Reviewer persona: 9,279 bytes, 71% of its prompt.
- Archivist persona: 11,667 bytes, 72% of its prompt.
- Developer persona: only 3,353 bytes.

The narrowest, most deterministic role has the largest persona. That is a concrete prompt-design defect, not merely a general suspicion about instruction size.

## Archivist invented an unsupported roadmap field

For CON-037, the Archivist correctly created the archive path but added:

```markdown
- Delivery: `pending-integration` on the recorded task branch
```

This is distinct from the CON-036 archive-reference syntax problem. It demonstrates that the agent is being asked to perform schema-defined mutations that should be executable-owned.

## Finalization rejection wastes an otherwise usable invocation

The Archivist consumed 306,372 input tokens, generated substantially correct artifacts, and then failed finalization over one unsupported field. Concoct reported:

```text
Retry safe from original evidence: false
```

The candidate was actually repairable manually. This exposes a mismatch between:

- candidate reusability;
- retry classification;
- supported recovery;
- actual recoverability.

The document should explicitly record that the system can preserve work physically while still providing no supported way to recover it.

## Validation is protective but not sufficiently diagnostic

The error:

```text
roadmap changed content outside selected item status and Archive fields
```

correctly protected the ledger, but did not identify:

- the offending `Delivery` field;
- its location;
- the allowed fields;
- whether removing it was safe;
- whether another agent invocation was necessary.

This strengthens the existing conclusion that recovery requires expert artifact knowledge.

## Post-integration reporting remains misleading

After successful CON-037 integration, Concoct returned to ready, but reported completed-task policies as blocked and integration as not applicable:

```text
Policy task-planning: blocked
Policy development: blocked
Policy independent-review: blocked
Policy integration: not-applicable
Next: concoct next
```

This is related to—but different from—the existing stale branch snapshot. The report evaluates the new empty ready state without clearly reporting that CON-037 completed successfully. It should distinguish completed-transition results from current-state eligibility.

## Structured progress improved, but remains semantically weak

The new compact stream avoids flooding the terminal, which is progress. However:

```text
turn.started
item.started
item.started
...
```

does not tell the operator whether Codex is reading, editing, testing, retrying, or stuck. The findings should update the progress issue from “raw output flood” to “compact but non-semantic progress.”

## Codex model-cache errors are resolved environmental noise

Repeated `base_instructions` model-cache errors were caused by incompatible Codex CLI and VS Code extension versions sharing the cache. Updating the CLI resolved them. No evidence connects the errors to Concoct’s measured inference usage.