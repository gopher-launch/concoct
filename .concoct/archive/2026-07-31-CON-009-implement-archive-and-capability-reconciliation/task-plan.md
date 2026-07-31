---
id: CON-009
title: Implement archive and capability reconciliation
roadmap-id: CON-009
status: implementation-complete
created: 2026-07-31
updated: 2026-07-31
remediates-review: review-02.md
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-009-implement-archive-and-capability-reconciliation
  base: c89529b90eb53d9628a88bb93b6e1ef670583beb
  status: archived
  archive-commit: self
capability-impact:
  type: add
  ids:
    - CAP-011
  rationale: Adds validated transactional archival and capability reconciliation for accepted Git-backed and non-Git tasks.
---

# Task Plan

## Goal

Implement the durable archival completion boundary so an approved task can be validated, summarized, reconciled with current capability truth, and preserved without premature delivery claims or loss of recoverable evidence.

## Context

Concoct already detects approved, archived, integrating, integrated, and ready states; renders Archivist guidance; validates Developer and Reviewer completion; isolates Git-backed tasks; and performs final Git integration bookkeeping. The remaining workflow gap is the mutation between an approved review and those existing post-archive states. Today plain `concoct archive` only renders a prompt, leaving the Archivist to assemble archive, capability, roadmap, and Git evidence manually.

CON-009 implements that missing acceptance boundary for both supported lifecycles. Git-backed archival must commit accepted archive and capability evidence on the task branch while retaining current artifacts and pending delivery. Non-Git archival must complete delivery bookkeeping, clear current artifacts only after durable validation, and return directly to `ready`.

## Why this matters

Capabilities are Concoct's accepted product truth and archives are its durable delivery history. Without a validated transaction connecting approval to those records, a partial or mistaken archival pass can claim undelivered behavior, lose the active recovery context, overwrite history, or leave cross-references contradictory. This task makes acceptance inspectable and recoverable while leaving semantic capability and summary authorship with the Archivist.

## Current state

- `internal/cli` routes `archive` through the read-only prompt path; no archival completion action exists.
- `internal/prompt` selects the Archivist persona from `review-approved` and names the authorized archive, capability, roadmap, task-plan, and notes outputs.
- `internal/workflow.Detect` validates task/review/capability/roadmap metadata and recognizes Git `archived` evidence, but it does not execute or fully validate an archival transaction or a non-Git archive result after current cleanup.
- `internal/workflow` already parses capability records and archive provenance for planning/status, but no mutation API reconciles declared `add`, `update`, `remove`, or `none` impact with authored capability changes.
- `internal/gitrepo` provides branch, base, cleanliness, changed-path, staging, commit, and retry primitives used by planning and role completion.
- `internal/integration` already treats an archived Git task as input and owns squash integration, final roadmap delivery, current-state cleanup, and task-branch deletion. Its `reconcile` path is the existing Git delivery boundary that archive must preserve.
- The normative docs, Archivist persona, handoff prompt, and skill define ordered archival, required summary content, explicit override evidence, archive naming, cross-references, and different Git/non-Git completion states.
- Existing archive directories demonstrate the expected task-plan, notes, sequential review, and summary structure, including pending-integration summaries.

## Target state

- Plain `concoct archive` remains deterministic, state-preserving Archivist guidance, with an explicit completion action that validates and commits authored archival work.
- Archival requires the latest sequential review to be `approved` unless a deliberate, explicit override records its authority and reason in durable archive evidence.
- The completion boundary validates the active task, notes, full review history, implementation diff, unused deterministic archive destination, summary schema/content, declared capability impact, capability ledger edits, roadmap evidence, and cross-references as one coherent transaction.
- Capability reconciliation accepts only the declared `add`, `update`, `remove`, or `none` outcome, preserves unrelated records and history, and verifies archive provenance without trying to judge prose quality.
- Git-backed archival validates the recorded branch/base and clean completion boundary, records pending delivery plus exact `git.archive-commit`/`git.status: archived`, commits once with safe retry reuse, retains `.concoct/current/`, and recommends `concoct integrate`.
- Non-Git archival marks the roadmap item delivered, validates all durable writes, clears populated current artifacts last, and returns to `ready` without fabricating Git evidence.
- Interrupted or invalid archival preserves authored evidence, reports which invariants remain unsatisfied, and never silently deletes, overwrites, or rolls back archive, capability, roadmap, review, or current-task content.
- Successful Git integration continues to perform final roadmap delivery and current cleanup using the existing CAP-007 lifecycle, now against archive evidence produced by the validated boundary.

## Design constraints

- Preserve CAP-001 role boundaries: the CLI validates durable structure and transaction invariants but does not author summary judgments, decide semantic capability wording, or manufacture override authority.
- Preserve ordinary role-command prompt semantics established by CAP-006 and CAP-010. Completion must be explicit and scriptable rather than inferred from rendered guidance.
- Use `internal/workflow` as the canonical schema and state authority and `internal/gitrepo` for Git safety; do not introduce a parallel state model or permissive Markdown parser.
- Treat completed reviews and existing archive directories as append-only. Archive creation and any reservation of its destination must be create-only and collision-safe.
- Validate the complete candidate result before destructive cleanup. For non-Git tasks, `.concoct/current/` is cleared only after archive, summary, capability, roadmap, and cross-reference validation succeeds.
- For Git tasks, do not mark the roadmap item delivered or clear current state during archival; integration remains the accepted delivery boundary.
- Preserve unrelated roadmap items, capability records, archives, source, tests, and accepted task history byte-for-byte except where required metadata dates or cross-references are intentionally reconciled.
- Keep root workflow assets and their embedded `templates/` counterparts synchronized wherever the installed contract changes.
- Preserve executable mode on `cmd/concoct/concoct.sh` and the existing non-Git fallback.

## Non-goals

- Writing the Archivist's summary, capability prose, override rationale, or acceptance judgment automatically.
- Changing implementation or tests being archived, rewriting completed reviews, or reopening approval during archival.
- Reworking squash integration, conflict resolution, push policy, branch deletion, or other CAP-007 behavior beyond compatibility and final-bookkeeping validation needed by CON-009.
- Adding hosted pull requests, provider integration, worktrees, concurrent or stacked tasks, general locking, or multi-repository archival.
- Implementing product-owner post-delivery roadmap reconciliation, history/diagnostic commands, upgrades, or direct agent execution.
- Broad capability or roadmap schema redesign unrelated to a safe archival transaction.

## Working assumptions

- The established explicit completion pattern should be extended to archival while retaining plain `concoct archive` as prompt rendering. The exact flag spelling is a local CLI decision, but it must include an unmistakable explicit path for unapproved override rather than treating ordinary completion as consent.
- The Archivist authors the candidate archive directory, summary, capability reconciliation, and required pending/delivery evidence before invoking completion; the CLI validates and commits that authored evidence rather than synthesizing semantic content.
- CAP-011 is the next unused stable capability identifier and will describe validated archive/capability reconciliation if the work is accepted.
- Existing task `capability-impact` metadata is authoritative expected scope. The completed archive summary and capability ledger must agree with it; changing impact requires an authorized artifact correction, not silent CLI inference.
- A dated archive directory derived from task identity and title is deterministic for the archival date. Existing destinations are evidence to reconcile or a collision to diagnose, never targets to overwrite.
- All declared prerequisites are compatible. Their limitations intentionally reserve semantic judgment for humans or agents while allowing structural validation, bounded Git mutation, recovery, and accepted state detection.

## Risks and open questions

- Archival spans multiple filesystem writes and, for Git tasks, a commit whose hash must then be recorded. The implementation must define an ordering and retry protocol that cannot require rewriting the commit it identifies or mistake a partial commit for completion.
- The current archive summary format is human-readable Markdown with front matter. Validation must be strong enough to establish identifiers, outcome, review, impact, delivery state, and required sections without pretending to assess narrative quality.
- Capability `remove` semantics need careful evidence rules: historical provenance must remain inspectable even if current active truth changes, and unrelated records must not be removed or reordered accidentally.
- An explicit unapproved override is exceptional acceptance evidence. Its command shape, required durable authority/reason fields, and allowed starting outcomes must be narrow enough that a typo or generic force flag cannot bypass review accidentally.
- Non-Git cleanup is the only destructive portion. Failure after durable writes but before cleanup must be recoverable by forward validation and retry, while a retry after cleanup must recognize the already-complete transaction from archive/roadmap/capability evidence.
- Existing integration reconciliation uses text-oriented roadmap mutation and current-file removal. Changes needed for shared validated delivery bookkeeping must preserve its guarded recovery phases and exact abort behavior.
- No unresolved product decision blocks implementation. The roadmap fixes approval default, explicit override, capability truth, transactionality, and lifecycle boundaries; local schema and CLI details must honor those outcomes.

## Implementation phases

### Phase 1 — Define archival evidence and completion protocol

Status: `complete`

- Trace approval detection, Archivist prompt inputs, summary conventions, capability parsing, archive naming, Git archive metadata, integration reconciliation, and non-Git ready detection end to end.
- Define the explicit completion and exceptional override interfaces, including required durable override authority/reason evidence and safe retry behavior.
- Specify candidate and completed invariants for summary metadata, required sections, artifact copies, cross-references, capability impact, roadmap status, current-state retention/cleanup, and Git commit identity.
- Reuse existing schemas where they can express the contract; document any narrow schema additions in source and template assets before relying on them.

### Phase 2 — Implement archive and reconciliation validation

Status: `complete`

- Add narrow workflow APIs that inspect the approved task, notes, complete review sequence, repository changes, candidate archive, summary, capability ledger, and roadmap as one transition.
- Validate declared `add`, `update`, `remove`, and `none` impacts, stable capability identifiers, unrelated-record preservation, archive provenance, task/roadmap/review identity, and required summary evidence.
- Add deterministic create-only archive destination handling and distinguish a valid retry/partial forward recovery from a conflicting pre-existing archive.
- Return actionable diagnostics without rewriting authored semantic evidence or deleting ambiguous partial work.

### Phase 3 — Complete Git-backed archival

Status: `complete`

- Extend CLI orchestration with the explicit archival completion/override path while retaining read-only prompt rendering.
- Validate exact recorded trunk `main`, task branch `concoct/con-009-implement-archive-and-capability-reconciliation`, base `c89529b90eb53d9628a88bb93b6e1ef670583beb`, attached checkout, ancestry, allowed paths, cleanliness, and absence of unrelated Git operations.
- Commit coherent archive, capability, pending-roadmap, summary, and retained-current evidence once; safely establish and validate the recorded archive commit without recursive commit identity.
- End at `archived`, preserve current artifacts and non-delivered roadmap status, and remain compatible with `concoct integrate`, `--continue`, and `--abort` final bookkeeping.

### Phase 4 — Complete non-Git archival and recovery

Status: `complete`

- Apply equivalent artifact, approval/override, summary, capability, and cross-reference validation without Git metadata or commits.
- Mark the selected roadmap item delivered only at the non-Git delivery boundary and clear populated current task/review files only after all durable outputs validate.
- Make retries recognize complete or partial transactions, preserve forward-recovery evidence, and refuse contradictory archive/capability/roadmap combinations.
- Confirm successful non-Git archival returns `ready` and does not disturb `.gitkeep` conventions or unrelated project content.

### Phase 5 — Verify, document, and prepare review

Status: `complete`

- Add focused workflow, CLI, real-Git, non-Git, summary-schema, capability-impact, collision, override, interruption, and retry tests.
- Exercise Git archive through successful integration and recovery paths, plus non-Git archive through ready state.
- Update README, command/state/workflow documentation, Archivist persona, handoff prompts, skill guidance, usage text, and source/template counterparts for the accepted completion protocol.
- Inspect the complete diff, record decisions and verification in notes, and provide an independent Reviewer handoff focused on transaction atomicity, capability truth, override safety, append-only evidence, and lifecycle parity.

## Acceptance criteria

1. Plain `concoct archive` remains deterministic, state-preserving Archivist prompt rendering; archival mutation occurs only through an explicit documented completion boundary.
2. Ordinary completion accepts only an active task whose latest contiguous review is valid, matches the task, has exactly one `approved` outcome, and whose implementation-complete evidence remains internally consistent.
3. Unapproved archival is rejected by default. The exceptional path requires an unmistakable explicit request plus durable authority and rationale in the resulting archive; missing, vague, stale, or mismatched override evidence fails without changing delivery state.
4. Completion validates an unused deterministic dated archive path containing the finalized task plan, notes, every sequential completed review, and `summary.md`; it never overwrites, renumbers, truncates, or silently repairs existing history.
5. `summary.md` has matching task/roadmap identifiers, archival and delivery state, approving review or override evidence, declared capability impact, and non-empty sections covering delivered outcome, key decisions, files changed, checks run, review result, capability changes, skipped work, and follow-up work.
6. Capability reconciliation supports `add`, `update`, `remove`, and `none`, agrees with task and summary metadata, preserves stable identifiers and unrelated records, records archive provenance for changed truth, and rejects undeclared or contradictory ledger edits.
7. Archive, capability, roadmap, summary, and current-task references agree. Partial or conflicting evidence reports actionable forward-recovery guidance and never causes premature current cleanup or delivery claims.
8. Git-backed completion validates the recorded trunk, task branch, immutable base, clean attached boundaries, ancestry, allowed workflow paths, and absence of unrelated operations; it commits one coherent transition, records an exact valid `git.archive-commit` and `git.status: archived`, and reuses a valid completed transition on retry.
9. Git-backed archival leaves CON-009 active/pending delivery, retains `.concoct/current/`, reports `archived`, and recommends `concoct integrate`. It does not claim delivery or delete the task branch.
10. Existing integration success performs final roadmap delivery bookkeeping, clears current artifacts, preserves accepted archive/capability evidence, removes recovery evidence and the accepted task branch, and returns `ready`; conflict continuation and abort remain recoverable under CAP-007.
11. Non-Git completion performs the same acceptance, archive, summary, capability, and cross-reference validation without Git evidence, marks only the selected roadmap item delivered, clears populated current artifacts last, and returns `ready`.
12. Failure at every tested transaction phase preserves authored and accepted evidence, does not overwrite prior archives or reviews, does not mutate unrelated roadmap/capability content, and emits a precise safe next action.
13. Existing planning, status, prompt, Developer/Reviewer completion, initialization, and integration behavior remains passing, and every changed installed workflow asset matches its `templates/` counterpart.

## Verification

- Run `gofmt` on changed Go files and focused tests for `internal/workflow`, `internal/cli`, `internal/gitrepo`, `internal/integration`, and `internal/prompt` during development.
- Add table-driven tests for summary metadata/sections, artifact-copy identity, sequential reviews, cross-references, all four capability-impact types, stable IDs, undeclared edits, unrelated-record preservation, deterministic archive naming, and collisions.
- Add real temporary Git repository tests for approval, explicit override, dirty/detached/wrong-branch/invalid-base/operation refusal, allowed-path enforcement, exact archive commit, retry idempotence, partial transaction recovery, and archive-to-integration success/continue/abort.
- Add non-Git tests for approval and override, durable-write failures before cleanup, retry after partial writes, final delivered roadmap evidence, current cleanup, and resulting `ready` state.
- Run `go test -count=1 ./...`, `go vet ./...`, and `go build ./cmd/concoct`.
- Run `bash -n cmd/concoct/concoct.sh` and confirm the wrapper remains executable.
- Run `./cmd/concoct/concoct.sh` against a temporary parent and confirm root files, dotfiles, nested templates, personas, planning directories, bootstrap prompt, Git initialization, staged files, no generated commit, and ready status.
- Compare every changed root workflow asset with its embedded `templates/` counterpart.
- Run `git diff --check` and search for stale branding, paths, persona names, archive semantics, capability identifiers, and obsolete prompt-only archive claims.

## Capability impact

Expected `add`: CAP-011, an observable archival coordination capability that validates approved or explicitly overridden acceptance evidence, reconciles declared capability truth, preserves dated history, commits recoverable Git-backed pending-delivery state, and completes non-Git delivery without replacing Archivist judgment.

CAP-001, CAP-005, CAP-007, and CAP-010 remain compatible prerequisites. Their limitations are intentional boundaries: semantic authorship and conflict choices remain human or agent responsibilities while the CLI validates structure, ownership, safe mutations, and resulting workflow state. CAP-007 may need relationship or evidence wording updated if accepted archive output strengthens its integration input, but its lifecycle scope remains unchanged.

## Handoff expectations

The Developer should begin by fixing the completion/override evidence and retry protocol against the existing normative archive transaction and integration phases. Implement through the current workflow, CLI, Git, and integration authorities rather than adding a parallel state machine. Before review, update phase statuses honestly, record durable decisions and verification in `notes.md`, inspect the complete diff, and add a fresh reviewer handoff covering implemented behavior, files changed, checks run, known risks, unresolved work, capability impact, and suggested focus on approval/override safety, commit identity, capability reconciliation, partial recovery, append-only evidence, and Git/non-Git parity.
