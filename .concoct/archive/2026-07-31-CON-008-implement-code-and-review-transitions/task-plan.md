---
id: CON-008
title: Implement code and review transitions
roadmap-id: CON-008
status: implementation-complete
created: 2026-07-31
updated: 2026-07-31
remediates-review: review-01.md
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-008-implement-code-and-review-transitions
  base: 2654973537dd89c942a2503bd3922b757aabc073
  status: active
capability-impact:
  type: add
  ids:
    - CAP-010
  rationale: Adds validated durable Developer and Reviewer role-completion transitions around the existing prompt-rendering workflow.
---

# Task Plan

## Goal

Complete `concoct code` and `concoct review` with validated, durable role-completion bookkeeping so Developer and Reviewer work can safely advance the artifact-backed workflow through implementation, review, remediation, and repeated review without treating prompt rendering as completed role work.

## Context

CON-006 delivered deterministic, state-preserving Developer and Reviewer prompt rendering. CON-007 delivered validated task planning, and CON-015 added exact Git task identity and clean role boundaries. The workflow detector already recognizes implementation progress, implementation completion, all three review outcomes, remediation dispositions, blocked-review recovery, and sequential review artifacts.

The remaining gap is the completion boundary after an acting Developer or Reviewer performs the rendered role work. Today `concoct code` and `concoct review` only render guidance; the CLI does not validate that owned changes match the selected mode, protect review reservation/finalization against collisions, or persist a complete Git-backed role transition.

## Why this matters

The core implementation/review loop cannot be relied on until each role's durable output is checked at its ownership boundary. Safe completion must preserve human or agent judgment while preventing false state advancement, overwritten reviews, incomplete remediation evidence, checkout drift, and duplicated transition commits.

## Current state

- `internal/cli.runPrompt` routes `code` and `review` directly through the shared prompt renderer and supports deterministic stdout or create-only `--output` behavior.
- `internal/prompt` selects initial implementation, continuation, review remediation, blocked-recovery, initial review, and post-remediation review modes from validated `workflow.PromptContext`.
- `internal/workflow.Detect` validates the canonical task status vocabulary, matching roadmap/task metadata, contiguous zero-padded reviews, exactly one supported review outcome, remediation references and dispositions, and blocked-review resolution evidence.
- `PromptContext.NextReview` derives the next name from validated reviews, but no mutation boundary reserves that path or prevents a race between selection and Reviewer output.
- `internal/gitrepo` can inspect branch, HEAD, cleanliness, operations, changed paths, staging, and commits; the personas require complete Developer and Reviewer transitions to be committed on the recorded task branch and retries to reuse valid completed transitions.
- Existing tests cover state derivation and prompt modes, but they do not exercise state-changing role completion, collision-safe review creation, retry idempotence, or complete `code → review → code → review` transitions.

## Target state

- `concoct code` retains state-preserving prompt rendering, then provides a bounded completion protocol that validates the acting Developer's allowed task-plan, notes, implementation, test, and documentation changes against the exact initial, continuation, remediation, or blocked-recovery mode.
- Developer entry and completion are durably distinguishable through valid task status and mode-specific metadata; completion requires an updated plan, meaningful notes and reviewer handoff, required remediation dispositions when applicable, and no modification to completed reviews.
- `concoct review` retains state-preserving prompt rendering, safely claims the exact next sequential review path without overwriting existing evidence, and finalizes only a schema-valid Reviewer-owned artifact with exactly one documented outcome.
- Completed reviews remain append-only, later reviews use deterministic consecutive numbers, and abandoned or malformed reservations cannot be mistaken for completed review evidence.
- Git-backed transitions validate the recorded task branch/base and a clean repository at role boundaries, commit each complete role transition once, and make valid retries idempotent. Non-Git projects retain the same artifact transition semantics without fabricated commits.
- `concoct status` derives the correct next command after each completed transition, including repeated changes-requested remediation and blocked-review recovery routes.

## Design constraints

- Preserve CAP-006 semantics: prompt rendering selects and guides the acting role but never manufactures role judgment or proves role completion.
- Preserve role ownership. The Developer may update implementation files, tests, required documentation, task plan, and notes, but never completed reviews. The Reviewer creates only the reserved next review and does not alter Developer-owned history or implementation.
- Use `internal/workflow` as the canonical artifact/state validator; do not introduce a second state machine or permissive Markdown interpretation.
- Build on `internal/gitrepo` for exact branch, base, cleanliness, changed-path, operation, and commit checks; do not weaken CON-015 safety boundaries.
- A completed transition must be validated as a coherent whole before its resulting state is accepted. Failures preserve authored evidence and report non-destructive recovery guidance.
- Review allocation must be exclusive and create-only. Never overwrite, renumber, truncate, or silently repair a completed or ambiguous `review-NN.md`.
- Keep source and embedded template personas, prompts, skills, and documentation synchronized wherever the role-completion contract changes.
- Preserve deterministic prompt bytes except for intentional, tested additions to the completion protocol, and preserve executable mode on `cmd/concoct/concoct.sh`.

## Non-goals

- Launching, embedding, or supervising a Developer or Reviewer agent.
- Generating implementation, findings, finding dispositions, review outcomes, or approval automatically.
- Implementing archival/capability reconciliation from CON-009 or changing integration behavior beyond compatibility with completed review transitions.
- Changing product scope, roadmap prioritization, capability truth, or prior completed review artifacts.
- General-purpose locking, concurrent active tasks, worktrees, stacked tasks, hosted pull-request review, or multi-repository coordination.
- Replacing the accepted Markdown artifact formats with a database or broad schema redesign.

## Working assumptions

- The role-completion interface should be explicit and scriptable while keeping the existing no-flag invocation as prompt rendering; the Developer must confirm the least surprising CLI shape from the normative contract and existing command conventions before implementation.
- Task status plus mode-specific front matter and notes are the durable implementation entry/completion evidence; no new transcript or opaque session record is needed unless repository inspection proves an atomicity gap that existing artifacts cannot represent safely.
- An exclusive create-only review reservation needs an incomplete form that workflow detection never treats as a completed review; finalization must validate and commit that same path rather than replace or overwrite prior evidence.
- CAP-010 is the next capability identifier currently available. Planning's
  CAP-009 assumption was stale because accepted CON-028 already owns CAP-009;
  the semantic capability impact is unchanged.
- All declared prerequisites are compatible: their limitations deliberately leave role judgment to agents while permitting structural validation, bounded Git mutations, and accepted-artifact state detection, which is exactly the separation required here.

## Risks and open questions

- A single command invocation cannot both render a prompt and know when an external role has finished. Phase 1 must settle an explicit completion/resume interface and document it consistently without making rendering stateful.
- A reservation artifact can leave interrupted evidence. Its format and recovery must distinguish safely between reserved, completed, malformed, and colliding reviews while retaining the current rule that populated incomplete reviews are invalid.
- Developer-owned changes may legitimately span many repository paths. Validation must enforce forbidden workflow paths and task Git identity without pretending to judge whether arbitrary code changes are semantically correct.
- Idempotent Git retry detection must distinguish the exact already-completed transition from unrelated commits or dirty changes and must not duplicate commits.
- Changes to workflow schemas or prompt text can affect golden fixtures, templates, integration/archive assumptions, and backward compatibility with active projects; migrations should be avoided unless essential.
- No unresolved product decision blocks planning. The roadmap fixes role ownership, outcomes, preservation, and safe failure; the implementation may choose the narrow completion mechanism within those constraints.

## Implementation phases

### Phase 1 — Define completion protocol and invariants

Status: `complete`

- Trace the code/review command contracts, personas, prompt modes, workflow precedence, Git lifecycle, and archive consumers end to end.
- Define the explicit post-role completion/retry interface for both commands while preserving ordinary prompt rendering as state-preserving.
- Specify snapshots and invariants for each Developer mode and Reviewer reservation/finalization, including non-Git behavior and interrupted recovery.
- Confirm whether existing task/review schemas can express all required evidence; prefer no schema expansion unless safe completion cannot otherwise be established.

### Phase 2 — Implement shared transition validation

Status: `complete`

- Add narrow workflow transition APIs that validate expected starting evidence, candidate owned outputs, mode-specific task metadata, notes/handoff requirements, review immutability, and the exact resulting state.
- Require complete dispositions for the latest changes-requested review and valid route evidence for blocked recovery using the existing canonical rules.
- Reject unauthorized path mutations, stale starting context, incompatible status changes, missing handoffs, malformed outcomes, and partial evidence with actionable errors.
- Make retry detection explicit so an already-valid completed transition is reused rather than advanced or committed twice.

### Phase 3 — Complete Developer bookkeeping

Status: `complete`

- Extend CLI orchestration for the selected implementation, continuation, remediation, or blocked-recovery mode using the shared transition validator.
- Validate implementation entry/completion without editing Developer-authored content or completed review files.
- For Git-backed tasks, enforce exact recorded branch/base, clean entry and completion boundaries, allowed workflow ownership, and one complete transition commit; preserve the unbranched path for non-Git projects.
- Ensure failed validation leaves authored work intact, does not claim implementation completion, and recommends precise recovery.

### Phase 4 — Complete Reviewer reservation and finalization

Status: `complete`

- Reserve the exact `NextReview` path exclusively and collision-safely after revalidating current state and sequence.
- Supply the Reviewer the reserved path and accepted incomplete form without allowing it to count as a completed review.
- Finalize only when the same artifact has matching task/sequence/persona metadata, exactly one documented supported outcome, and required evidence structure; prior reviews and Developer-owned files must remain unchanged.
- Commit the complete Git-backed review once, support safe retries and interrupted reservations, and verify the resulting review state/next action.

### Phase 5 — Verify the complete loop and reconcile guidance

Status: `complete`

- Add focused workflow, CLI, prompt, and real-Git tests for each mode, invalid transition, collision, interrupted reservation, retry, forbidden mutation, and non-Git path.
- Exercise multiple review cycles including changes requested, remediation, approval, and both blocked-review recovery routes.
- Update command/state documentation, personas, handoffs, skill guidance, README, and source/template counterparts only where observable behavior changes.
- Inspect the complete diff, record verification and risks in notes, and prepare an independent Reviewer handoff.

### Review 01 remediation

Status: `complete`

- Confirm the review-reservation Git boundary through the existing validated
  prompt-context path and add focused clean, dirty, wrong-branch, detached,
  operation-in-progress, and invalid-base regression coverage.
- Require the final Git-backed reviewer handoff section to differ from its
  committed `HEAD` version, reject unrelated notes edits that reuse a stale
  handoff, and document the non-Git artifact-level rule.
- Re-run the complete verification suite and prepare a fresh reviewer handoff
  without modifying `review-01.md`.

## Acceptance criteria

1. Ordinary `concoct code` and `concoct review` prompt rendering remains deterministic and state-preserving; role completion requires an explicit, documented boundary and is never inferred from prompt output.
2. Developer completion is accepted only from an eligible planned, in-progress, changes-requested, or validated blocked-recovery state and only for the exact mode selected from the starting evidence.
3. Initial implementation, continuation, remediation, and blocked-recovery transitions validate the Developer-owned task-plan and notes changes, require a fresh reviewer handoff at completion, and reject modification of any completed review artifact.
4. Remediation cannot complete until every unresolved finding in the exact latest `changes-requested` review has a recognized durable disposition; stale, partial, or different-review references fail safely.
5. Reviewer work exclusively reserves the next zero-padded contiguous `review-NN.md`, never overwrites or renumbers an existing review, and has a documented recovery path for interrupted or abandoned reservations.
6. Review finalization accepts exactly one matching Reviewer-owned artifact with the expected task ID, sequence, persona, and exactly one outcome of `approved`, `changes-requested`, or `blocked`; malformed, stale, colliding, or multiple-outcome evidence is rejected without rewriting it.
7. A completed review advances state solely from its recorded outcome, preserves all earlier reviews append-only, leaves Developer-owned history unchanged, and causes `concoct status` to recommend `archive`, `code`, or blocker routing as appropriate.
8. The repeated loop `code → review-01 changes-requested → code remediation → review-02` completes with deterministic numbering and correct status recommendations; equivalent approval and blocked-review recovery paths are covered.
9. Git-backed entry, completion, and retry validate the recorded trunk `main`, task branch `concoct/con-008-implement-code-and-review-transitions`, base `2654973537dd89c942a2503bd3922b757aabc073`, attached checkout, clean boundaries, and absence of unrelated Git operations; each complete role transition is committed once. Non-Git projects work without Git metadata or commits.
10. Invalid or interrupted transitions preserve user-authored evidence, never manufacture role judgment, never modify completed reviews, and return actionable diagnostics that identify the responsible role and safe next action.
11. Existing planning, prompt rendering, workflow detection, archival guidance, integration, initialization, and status behavior remains passing, and every changed shared workflow asset matches its embedded template counterpart.

## Verification

- Run `gofmt` on changed Go files and focused tests for `internal/workflow`, `internal/cli`, `internal/prompt`, and `internal/gitrepo` during development.
- Add real temporary-repository tests for clean Git completion, dirty/detached/wrong-branch/operation refusal, forbidden-path mutation, exact commit reuse, retry idempotence, and non-Git parity.
- Test initial implementation, continuation, changes-requested remediation with complete and incomplete dispositions, Developer-routed blocked recovery, and review-routed blocked recovery.
- Test review-01 creation, repeated review numbering, pre-existing path collisions, incomplete/malformed reservations, all three outcomes, mismatched metadata, multiple outcomes, append-only enforcement, and recovery.
- Run `go test -count=1 ./...`, `go vet ./...`, and `go build ./cmd/concoct`.
- Run `bash -n cmd/concoct/concoct.sh` and confirm the wrapper remains executable.
- Run the AGENTS.md initialization check in a temporary parent and confirm root files, dotfiles, nested templates, personas, planning directories, bootstrap prompt, Git initialization, staged files, and no generated commit.
- Compare changed source workflow assets with their `templates/` counterparts.
- Run `git diff --check` and search for stale branding, paths, persona names, review numbering rules, and obsolete code/review command behavior.

## Capability impact

Expected `add`: CAP-010, an observable Developer/Reviewer coordination capability that validates durable role completion, protects artifact ownership and review sequencing, supports remediation and blocked recovery, commits safe Git-backed transitions, and exposes correct next actions without replacing role judgment.

CAP-001, CAP-005, CAP-006, CAP-007, and CAP-008 remain compatible prerequisites. Their limitations are intentional boundaries for this task; they may need relationship or limitation wording updates only if the accepted implementation changes observable behavior. Final capability reconciliation belongs to archival.

## Handoff expectations

The Developer should first resolve the explicit completion/resume interface and reservation recovery design against the accepted command contract, then implement through the existing workflow, CLI, prompt, and Git boundaries. Before review, update phase statuses honestly, record decisions and verification in `notes.md`, inspect the complete diff, and add a reviewer handoff covering files changed, checks run, risks, unresolved work, capability impact, and suggested focus on role ownership, transition atomicity, append-only reviews, idempotent Git retries, and non-Git parity.
