---
version: 1
project: concoct
updated: 2026-10-22
---

# Concoct implementation roadmap

This roadmap contains intended future work only. `capabilities.md` describes delivered behavior; Git history and external task/review evidence retain completed task specifications and delivery records. Task IDs are permanent and must not be reused.

Concoct coordinates one task through reagent implementation, independent iragent review and correction, and operator-authorized local integration. Agents own engineering judgment and verification. Concoct routes their results, retains execution context and enforces configured limits. The nominal workflow has been demonstrated on real Curu work, including a requested-changes cycle and exact-commit local integration.

## Delivery rules

- Execute and review one task at a time. Select the highest-priority suitable ready task; a planned task is not eligible until its prerequisites are satisfied and it is promoted to ready.
- Continue using manually directed reagent/iragent sessions to develop Concoct. Do not use the runner to build itself. CON-073 introduces a skill for the established operator workflow after the relevant usability changes settle.
- Every candidate includes implementation, proportionate verification, self-review and relevant documentation reconciliation. Independent review and explicit operator acceptance precede local integration. Publication remains operator-controlled.
- Reconcile documentation in the candidate for its intended post-integration state: summarize delivered behavior in `capabilities.md`, remove the completed task from this roadmap, remove its satisfied dependency references from remaining tasks, and promote newly eligible tasks when appropriate. Preserve the IDs and acceptance scope of all unfinished work.
- Before removing the active task specification, retain it with the ordinary external task evidence so review can assess the original scope. Completed specifications remain available through Git history and external evidence; do not create a second completion ledger in the roadmap or capabilities.
- `Depends on` names unfinished tasks still present in this roadmap. Remove a prerequisite reference only when delivery has satisfied it. Missing or misspelled references remain errors, never implicit evidence of completion. Retire epic/feature entries when they no longer organize unfinished work.
- `capabilities.md` describes current behavior and practical limits, organized by capability. It does not accumulate completed-task acceptance criteria, obsolete mechanisms or a chronological delivery log. Source remains the authority for implemented behavior.
- Keep trial output, investigations, diagnostics and review records outside the project. Use focused deterministic checks for these follow-ups; another billable live trial is not required solely for presentation or bookkeeping changes.

## CON-074 — Keep the roadmap prospective and capabilities current

- Kind: task
- Status: ready
- Priority: high
- Parent: none
- Depends on: none

**Objective:** Make future-only roadmap maintenance the normal delivery practice, without accumulating completed tasks or obsolete implementation descriptions.

**Acceptance criteria:**

- Align active contributor guidance, agent prompts, documentation and relevant fixtures with the delivery rules above. Replace blanket instructions to preserve every roadmap entry or mark completed entries delivered with instructions to preserve unfinished IDs and remove completed entries after capability reconciliation.
- Summarize delivered behavior in `capabilities.md`; retain the completed task specification in existing external evidence and Git history rather than copying its acceptance criteria into capabilities.
- Reconcile remaining dependencies in the same candidate: remove satisfied prerequisites, retain unfinished prerequisites, and update readiness when appropriate. Remove completed epic/feature scaffolding and stale milestone instructions.
- Keep the existing bounded roadmap metadata contract usable. A task with no remaining prerequisites uses `Depends on: none`; unresolved references and cycles still fail validation. A capabilities prose parser or separate completed-ID registry is unnecessary.
- Ensure independent review receives the active task's original acceptance scope even when its entry has been removed from the candidate roadmap. Reuse the ordinary task evidence supplied to agents.

**Verification:** Use a small roadmap fixture with two dependent tasks to demonstrate delivery, capability reconciliation, completed-entry removal and readiness promotion without dangling references. Cover an unresolved reference and preservation of unfinished IDs. Check active instructions for contradictory maintenance rules; run applicable repository checks.

## CON-068 — Preserve execution context in suggested continuation commands

- Kind: task
- Status: ready
- Priority: medium
- Parent: none
- Depends on: none

**Objective:** Make a copied continuation command use the same executable, repository and evidence root as the invocation that produced it.

**Context:** A trial invoked `/tmp/concoct` with an explicit `--root`, but the suggested `concoct run --approve COMMIT` selected a different installed binary and evidence directory.

**Acceptance criteria:**

- Generate continuation commands using the executable used for the current invocation. Preserve an explicit executable path such as `/tmp/concoct`; do not replace it with an unqualified `concoct`.
- Preserve relevant explicit context, including `--root` and `--repo`, and any invocation-only settings required for the proposed continuation. Reuse durably retained settings where that is already the continuation contract; avoid silently reverting invocation-only choices to defaults.
- Quote executable paths and argument values correctly for the documented shell. Handle spaces and shell metacharacters as literal argument content.
- Apply the same command construction to run summaries, `status` suggestions and other continuation hints. A separate `status` invocation uses its current executable and selected repository/root context.
- Preserve the specific next action and its exact identifier or approved commit. Do not carry an obsolete action flag into the suggested command.
- Update operator examples and help where needed.

**Verification:** Capture generated commands for an absolute executable path, explicit root/repository, paths with spaces/metacharacters, and a continuation from `status`. Verify parsed argument values with a harmless executable fixture rather than starting a provider. Check applicable tests, build and whitespace validation.

## CON-069 — Timestamp terminal messages and standardize run summaries

- Kind: task
- Status: ready
- Priority: medium
- Parent: none
- Depends on: none

**Objective:** Complete terminal presentation with message timestamps, consistent task/status summaries and elapsed invocation time.

**Context:** Readable prose, reagent/iragent result labels, source attribution, blank lines and external provider diagnostics are already delivered. The recent successful output still lacks streamed message timestamps.

**Acceptance criteria:**

- Prefix each operator-facing terminal message with date and time in local time by default, identifying its timezone/offset. Honor the existing timezone selection mechanism, including `TZ`. Apply this to agent progress, completed results and Concoct coordination messages.
- Timestamp a streamed message when it begins; preserve immediate chunk delivery and internal formatting without inserting a prefix into each fragment. Keep blank lines between distinct messages and retain clear source labels.
- Present task summaries with `Task: ID` followed immediately by `Status: VALUE`. Replace the summary label `Concoct: integrated` with `Status: integrated`. Use the same ordering and labels in run summaries and `status` output.
- Report elapsed wall time for each `concoct run ...` invocation, covering all agent turns and coordination within that command. A later continuation starts its own invocation timer; do not label cumulative task time or a per-turn timeout as this duration.
- Measure elapsed duration with a monotonic clock and report it on normal completion and handled stop/error paths, including a stop awaiting operator input or approval. Use a readable duration format.
- Preserve readable prose, hyphenated public statuses, quiet-mode summaries and external provider diagnostics. Keep explicit JSON output machine-readable; timestamp prefixes must not corrupt its records. Expose invocation duration as a numeric value with a documented unit in the relevant JSON summary.

**Verification:** Use injected clocks and output fixtures for fragmented messages, source switches, result messages, local/UTC display, an offset transition, summary ordering and elapsed time across multiple turns. Cover completion, approval wait, handled error and interruption, plus quiet/JSON output. No live provider run is required.

## CON-070 — Retain completed assessment counts when continuing earlier records

- Kind: task
- Status: ready
- Priority: medium
- Parent: none
- Depends on: none

**Objective:** Correct the observed undercount when continuing a coordinator record created before the current assessment bookkeeping.

**Context:** CURU-001 reported `Independent assessments: 0/3` despite completed independent approval after crossing the CON-064 to CON-065 upgrade. A fresh CURU-014 run correctly reported `2/3`.

**Acceptance criteria:**

- Establish how the affected coordinator record represents its completed approval and why the count is omitted. Use retained structured coordinator/result metadata; record the diagnosis externally.
- Preserve a recorded completed independent assessment when loading or continuing an affected record. Count completed approved and changes-requested assessments once each; an incomplete assessment does not count as completed.
- Use the corrected count consistently in summaries, status, persisted continuation state and assessment-limit enforcement. Reopening the record must neither reset nor increment completed consumption.
- Keep current-record counting correct, including the fresh two-assessment correction cycle. Preserve the configured limit and all other consumed execution allowances.
- Keep any compatibility handling bounded to the affected coordinator record shape. Conflicting or insufficient structured metadata must not be converted into invented review history or guessed consumption; explain the limitation clearly.

**Verification:** Add a representative pre-count approval record showing `1/3`, a fresh requested-changes/approval record showing `2/3`, and repeated-load/continuation checks with no duplicate counting. Verify limit enforcement and incomplete/conflicting metadata. Use fixtures; do not restart or modify the original live trial.

## CON-071 — Include the source revision in development versions

- Kind: task
- Status: ready
- Priority: medium
- Parent: none
- Depends on: none

**Objective:** Make development binaries distinguishable by source revision through `concoct version`.

**Acceptance criteria:**

- Development builds report the base development version plus a Git revision identifier, for example `0.5.0-dev+819a7b1`.
- Support the ordinary local build workflow, including `go build -o /tmp/concoct`, without requiring the operator to remember custom linker flags. Prefer available Go build/VCS metadata over a runtime Git lookup.
- Use a documented, consistent short revision length sufficient for the project. If revision metadata is unavailable, report that honestly with a documented fallback rather than inventing a hash.
- Preserve release-version reporting and document how the development revision is obtained.

**Verification:** Test version formatting with development, release and missing-metadata fixtures. Build from a disposable Git checkout using the ordinary command and confirm the displayed revision corresponds to that checkout. Run applicable repository checks.

## CON-072 — Organize external artifacts by readable project and task identity

- Kind: task
- Status: ready
- Priority: medium
- Parent: none
- Depends on: none

**Objective:** Make external Concoct working directories understandable by browsing them, with the task as the unit of work.

**Acceptance criteria:**

- Define and document a compact external hierarchy under the selected Concoct root. Group each task's runs, invocations, task/review evidence, receipts where used, diagnostics and any associated workspaces beneath its readable task identity. Repository-wide indexes and locks may remain at the level their scope requires.
- Use meaningful names plus unique random identifiers for durable project/task/run containers where uniqueness is required. Use datetime names with nanosecond precision for invocation directories, with an explicit timezone convention and deterministic collision handling; timestamp precision alone does not guarantee uniqueness.
- Distinguish repositories with the same basename, identical task IDs in different repositories, repeated executions of one task, and simultaneous name allocation. Normalize names safely without allowing different labels or unsafe path characters to create accidental identity collisions or escape the configured root.
- Keep stable recorded identities separate from readable labels. Store enough metadata to identify the source repository, task and run without relying on the directory label as authoritative identity.
- Handle the initial selection invocation before a task has been chosen, then associate its artifacts with the selected task. Preserve diagnostics for a no-work or failed selection without inventing a task ID.
- Existing active coordinator records and retained evidence remain discoverable and usable. Choose the smallest compatibility approach needed for current records; existing history need not be bulk-renamed. Update generated paths, status locations and documentation consistently.
- Keep task artifacts outside the target project and retain the existing execution-lock behavior and custom `--root` support.

**Verification:** Use filesystem fixtures for same-name repositories, duplicate task IDs across projects, repeated runs, equal timestamp allocation, unsafe labels, pre-selection artifacts and continuation of an existing record. Verify that displayed locations resolve to the intended artifacts. No provider run is required.

## CON-073 — Provide a skill for the established Concoct operator workflow

- Kind: task
- Status: planned
- Priority: low
- Parent: none
- Depends on: CON-068, CON-069, CON-072

**Objective:** Package the viable Concoct workflow as a reusable skill after its command-context, output and artifact-location changes settle.

**Acceptance criteria:**

- Document the implemented first-use and continuation workflow: prerequisites, one-task execution, automatic review/correction, explicit blocker answers, exact-commit integration approval, status and evidence inspection, and configured execution bounds.
- Teach the actual coordinator/agent responsibility split and distinguish automatic handoffs from operator decisions. Keep publication operator-controlled.
- Derive commands, paths and continuation guidance from current supported CLI behavior and documentation. Preserve explicit executable/root/repository context and use the settled external hierarchy.
- Follow the supported skill packaging and discovery conventions for the intended Codex environment. Verify the skill with ordinary deterministic instruction/example checks; a skill must not become a prerequisite for using the CLI.
- Update maintenance guidance so the skill stays aligned with implemented behavior as the workflow changes.

**Verification:** Check packaging/discovery and every command example against the implemented CLI and current documentation. Walk the instructions against the demonstrated nominal and correction workflows. Do not add speculative recovery commands or revive the retired skill's obsolete controller requirements.

## CON-075 — Authorize explicit limit increases for active work

- Kind: task
- Status: ready
- Priority: critical
- Parent: none
- Depends on: none

**Objective:** Allow the operator to extend an active run’s execution allowance explicitly, preserving completed work and consumed counts.

**Context:** CURU-015 correctly stopped after three independent assessments, with seven of twelve agent invocations consumed. Continuation was blocked because startup configuration flags cannot change an active run. The operator needs a supported authorization path that requires no manual state editing.

**Acceptance criteria:**

- Add explicit active-run authorization flags:
  - `--approve-review-limit-increase TOTAL`
  - `--approve-invocation-limit-increase TOTAL`
- Each value specifies the new total ceiling. For example, increasing the review limit from three to five after three completed assessments leaves two assessments available.
- Keep ordinary startup limit flags and role-setting restrictions unchanged. Use the existing invocation accounting for implementation, correction, review, replacement-session and integration executions.
- Require an existing active run and a valid total strictly greater than its recorded limit. Reject invalid requests clearly before changing state or launching an agent. Both increase flags may be supplied together; validate the complete request before persisting either change.
- Persist approved increases under the existing execution lock before further dispatch. Preserve task, branch, candidate, findings, evidence, role settings, session context and all consumed counts. Record the previous and authorized ceilings in ordinary external execution metadata.
- Reevaluate exhausted limits and continue the pending workflow step when allowances permit. Increasing one ceiling does not bypass another exhausted limit or any other existing prerequisite. Completed work and assessments are not repeated.
- Preserve independent review and explicit operator authorization of the exact approved integration commit.
- Limit-exhausted output identifies each exhausted allowance, its consumed/total counts and a concrete continuation command using the explicit authorization flags. Preserve executable, repository and evidence-root context. Explain that the supplied value is a new total ceiling.
- Update CLI help and operator documentation with the stop → authorize increase → continue workflow.

**Verification:**

- Reproduce the observed state with a fixture: three completed assessments, seven of twelve invocations consumed, and changes requested on the current candidate.
- Authorize a review ceiling of five; verify persisted counts remain `3/5` and `7/12` before dispatch, and continuation routes the retained findings to reagent.
- Verify invocation-limit increases, combined increases, another limit remaining exhausted, invalid/non-increasing totals, missing active work and rejected startup-setting changes.
- Reopen the updated record and confirm the increased ceilings and consumed counts survive without duplicate execution. Verify stopping at the new ceilings.
- Exercise generated continuation commands and run applicable formatting, tests, vet, build and whitespace checks. Fixtures are sufficient; implementation must not modify or resume the live CURU-015 record.

## CON-076 — Suggest an operator-preferred task during selection

- Kind: task
- Status: ready
- Priority: medium
- Parent: none
- Depends on: none

**Objective:** Let the operator suggest a task for reagent selection while preserving agent judgment and the normal workflow.

**Acceptance criteria:**

- Add `concoct run --prefer-task TASK-ID` for fresh task selection. The preference applies only to that invocation and is passed explicitly to reagent as a suggestion.
- Instruct reagent to inspect the current roadmap and applicable repository guidance before deciding. The preference does not override dependencies, readiness, scope or other selection requirements. Eligibility assessment remains reagent’s responsibility.
- If reagent accepts the preference, it selects the proposed task and continues implementation in the same turn through the ordinary workflow. No additional operator confirmation is required.
- If reagent rejects the preference, it reports a concrete reason, such as an unsatisfied dependency, unavailable task or incompatible task status. It ends selection without implementing work or silently selecting another task.
- Represent rejection as a distinct normal structured outcome with a readable explanation. Concoct routes that outcome without interpreting the explanation or independently reassessing eligibility.
- On legitimate rejection, return a successful command outcome, retire the unclaimed active selection and leave Concoct ready for another fresh `concoct run`. Retain ordinary external execution evidence and usage; do not record the rejection as a failure or automatically retry.
- Reject use of `--prefer-task` when continuing active work, explaining that the existing task must be continued or resolved first.
- Genuine provider failures, malformed results and interrupted execution retain their existing handling; they do not become legitimate preference rejections.
- Update CLI help and operator documentation with accepted and declined preference examples.

**Verification:**

- Inspect the actual selection prompt to confirm it identifies the preference as advisory, requires roadmap inspection and defines acceptance and rejection behavior.
- Use scripted agent results to verify acceptance continues normal implementation/review routing for the proposed task.
- Verify a declined preference displays its reason, starts no implementation or review, returns successfully and leaves no active selection blocking a subsequent plain `concoct run`.
- Cover rejection while active work exists, malformed results and provider failure. Verify that rejection evidence and usage remain available.
- Run applicable formatting, tests, vet, build and whitespace checks. No live provider trial is required solely for this change.

**Scope boundary:** Keep task suitability and rejection reasoning in reagent instructions. Concoct handles the CLI argument, prompt context and deterministic result routing; it does not introduce a host-side eligibility engine.
