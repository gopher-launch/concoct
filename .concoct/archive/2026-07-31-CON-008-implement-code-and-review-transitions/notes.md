# Notes

## Planning summary

CON-008 is implementation-ready. It is a coherent role-transition outcome with no roadmap dependencies, and its five declared capability prerequisites are active accepted truth whose limitations are compatible with the work. The task adds structural completion bookkeeping around existing agent-authored Developer and Reviewer work; it does not automate their judgment.

## Confirmed findings

- The checked-out Git state exactly matches the planning prompt: branch `concoct/con-008-implement-code-and-review-transitions`, trunk `main`, and base/HEAD `2654973537dd89c942a2503bd3922b757aabc073` before planning artifacts.
- `.concoct/current/` contained only `.gitkeep`, so no active task or review conflicts with CON-008.
- CON-008 is `planned`, has no roadmap dependencies, and declares CAP-001, CAP-005, CAP-006, CAP-007, and CAP-008 as prerequisites.
- `internal/cli.runPrompt` treats `code` and `review` only as prompt commands. It has no post-role completion or reservation flow.
- `internal/prompt` already distinguishes initial implementation, continuation, remediation, post-remediation review, and both blocked-review recovery modes, and calculates the next review path from validated workflow context.
- `internal/workflow.Detect` already validates most target end states: task status, sequential review naming and metadata, exactly one supported outcome, review precedence, remediation references/dispositions, and blocked-review resolution evidence.
- `internal/gitrepo` already exposes the branch, HEAD, cleanliness, operation, changed-path, staging, and commit primitives needed to enforce the accepted Git lifecycle instead of creating a parallel Git implementation.
- Developer and Reviewer personas require complete Git-backed transitions to be committed and retries to reuse existing valid transitions. Developer ownership excludes completed reviews; Reviewer ownership is limited to the next review artifact.

## Prerequisite compatibility

- CAP-001 permits structural validation while explicitly leaving semantic correctness to the acting roles. CON-008 preserves that boundary.
- CAP-005 already detects durable implementation/review evidence and reports invalid contradictions. Its narrow Markdown parsing and textual remediation-disposition limitations are acceptable; tests must protect those deliberate rules.
- CAP-006 deliberately leaves role-completion mutations to later work. CON-008 adds that missing boundary while preserving deterministic, state-preserving rendering and agent-authored outcomes.
- CAP-007 supplies exact Git identity and bounded role-transition safety but leaves role execution to agents. CON-008 can reuse those primitives without adding provider, worktree, or concurrency features.
- CAP-008 supplies the validated active task and exact task Git metadata required before Developer entry. Its semantic planning judgment remains complete before this task starts.

## Planning decisions

- Reuse `internal/workflow` as the sole artifact and state authority and `internal/gitrepo` as the Git boundary. Do not duplicate parsing or state derivation in CLI orchestration.
- Preserve existing no-flag `code` and `review` behavior as prompt rendering. Add an explicit post-role boundary rather than making prompt generation itself mutate state.
- Treat review reservation and finalization as separate concepts. Reservation must be exclusive and recognizable as incomplete; only a fully validated artifact may advance workflow state.
- Validate role-owned path changes and exact resulting evidence, but do not attempt to decide whether implementation semantics or review judgment are correct.
- Plan an `add` capability impact using CAP-009, the next identifier visible in current capability truth.

## Risks for implementation

- The exact completion/resume CLI shape is not yet implemented or prescribed by current source. The Developer must select and document a clear interface consistent with existing command conventions while satisfying the acceptance criteria.
- Interrupted review reservation is the sharpest artifact-design risk. It must never collide with or masquerade as a completed review, and recovery must preserve evidence.
- Git dirty-state validation must allow the acting role's intended changes at completion while still detecting forbidden workflow mutations and unrelated operations; the entry and completion snapshots need a precise contract.
- Existing workflow fixtures use complete review artifacts only. Introducing a reserved form may require carefully scoped detection behavior and backward-compatible diagnostics.
- Prompt golden files are byte-level public evidence; any completion guidance change must be intentional and tested.

## Relevant history

- CON-003 defined the normative command and state-machine contract.
- CON-005 delivered CLI/project discovery and artifact-backed status validation.
- CON-006 delivered deterministic role-aware prompt rendering.
- CON-015 delivered Git task isolation, boundary validation, and integration safety.
- CON-007 delivered validated task planning and the active task evidence on which this transition begins.

## Task Planner handoff

- Current state: `planned` after both active artifacts validate and CON-008 alone becomes `active`.
- Work completed: repository inspection, prerequisite limitation review, scope decomposition, acceptance criteria, verification expectations, Git metadata preservation, and risk identification.
- Work remaining: implement and independently review the Developer/Reviewer completion transitions described in `task-plan.md`.
- Decisions made: preserve prompt-only semantics; add an explicit completion boundary; reuse workflow/Git authorities; separate review reservation from finalization; expect capability addition CAP-009.
- Known risks: completion interface clarity, interrupted reservation recovery, owned-path validation across broad code changes, and idempotent Git retry detection.
- Checks run: exact branch/HEAD and clean starting state; roadmap/prerequisite/archive inspection; relevant CLI, workflow, prompt, Git, persona, documentation, and test inspection.
- Artifacts created: `.concoct/current/task-plan.md` and `.concoct/current/notes.md`.
- Expected next role: Developer.
- Recommended next command: `concoct code`.

## Implementation decisions

- Added explicit `concoct code --complete`, `concoct review --reserve`, and
  `concoct review --complete` boundaries while preserving ordinary prompt
  rendering as deterministic and read-only.
- A review reservation is the exact next `review-NN.md` with `status: reserved`
  and a generated marker. Workflow detection ignores a valid reservation as
  outcome evidence, so state remains `implementation-complete`; malformed
  reservations are invalid and preserved for recovery.
- Git-backed completion validates the recorded task branch/base and absence of
  unrelated Git operations. Developer completion rejects reviews and protected
  product/archive paths, while Reviewer completion accepts only the single
  reserved review. Each coherent transition is staged and committed once;
  clean retries reuse only a matching commit whose resulting workflow state is
  still valid.
- Non-Git projects use the same artifact validation without fabricating commits.
- Corrected the planned capability identifier from CAP-009 to CAP-010. CAP-009
  was already accepted by CON-028 before this task was planned; this is an
  identifier correction with no change to the roadmap outcome.

## Verification results

- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` — passed; wrapper executable mode retained.
- Temporary initialization — passed for Git creation, staged root and dotfiles,
  nested templates, personas, current planning directory, bootstrap prompt,
  and absence of a generated commit.
- Source/template comparisons for every changed shared workflow asset — passed.
- `git diff --check` — passed before final task-artifact updates.

## Handoff to reviewer

### Implemented

Explicit Developer completion and Reviewer reservation/finalization boundaries,
canonical workflow recognition of incomplete reservations, Git-backed commit
and retry behavior, non-Git parity, CLI and real-repository coverage, and
synchronized user/workflow guidance.

### Key decisions

Prompt rendering remains non-mutating. Completion validates role-authored
evidence rather than generating it. Review reservation uses the canonical next
review path and remains non-authoritative until finalized.

### Files changed

CLI orchestration, workflow transition validation, Git commit inspection,
transition tests, README and normative docs, Developer/Reviewer personas,
handoff prompts, the Concoct skill and matching embedded template assets, plus
the active task plan and notes.

### Verification

Full Go tests, vet, build, shell syntax/executable checks, temporary project
initialization, source/template parity, and whitespace validation passed.

### Known risks

Non-Git projects cannot use Git status to prove which paths changed, so they
receive the same resulting-artifact validation and ownership contract but no
diff-based path enforcement. Reservation recovery is deliberately manual and
preserves authored evidence rather than deleting or repairing it.

### Skipped or unresolved work

No task-scoped work is unresolved. Direct agent execution, general locking,
worktrees, and archival automation remain explicit non-goals.

### Capability impact

Expected addition is CAP-010 (not the stale planned CAP-009): validated durable
Developer/Reviewer coordination without replacing role judgment.

### Suggested review focus

Role-owned path enforcement, reserved review recognition and recovery,
resulting-state validation, exact clean retry behavior, repeated numbering,
non-Git parity, and source/template contract consistency.

## Review 01 remediation

### Finding dispositions

- Finding 1 — `disputed with evidence`, with its required verification outcome
  implemented. `ReserveReview` calls `InspectPromptContext`, and that shared
  validator already calls `InspectGitContext` and checks the recorded trunk and
  base, attached task branch, clean worktree, and unrelated Git operations
  before reservation creation. Added focused real-repository coverage for the
  valid clean path and refusal from dirty, wrong-branch, detached,
  operation-in-progress, and invalid-recorded-base states; every refusal also
  proves no reservation was created.
- Finding 2 — `fixed`. Git-backed Developer completion now extracts the final
  `## Handoff to reviewer` section and compares it byte-for-byte with the same
  section committed at `HEAD`. An unrelated notes edit with an unchanged
  handoff is rejected. Required headings are validated within the final handoff
  rather than anywhere in the notes. Non-Git completion retains a documented
  artifact-level rule requiring a complete final handoff because no committed
  comparison baseline exists.

### Remediation verification

- `go test -count=1 ./internal/cli ./internal/workflow ./internal/gitrepo` — passed.
- `go test -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go build ./cmd/concoct` — passed.
- `bash -n cmd/concoct/concoct.sh` and executable-mode check — passed.
- Temporary initialization with the built CLI — passed: Git repository,
  staged root/dot/nested templates, personas, current planning directory,
  bootstrap prompt, and no generated commit were confirmed.
- Source/template Developer persona comparison — passed.
- `git diff --check` — passed before final artifact updates.
- The literal legacy initialization path in `AGENTS.md` was unavailable; the
  tracked command is `cmd/concoct/concoct.sh`, and the built root CLI was used
  for the required generated-project behavior check.

### Attempt: complete remediation without transition metadata

- Tried: `concoct code --complete` after code, tests, documentation, and handoff
  updates.
- Error/result: completion refused because the latest review remained
  `changes-requested` and the plan did not name it with `remediates-review`.
- Why it failed: the authored dispositions were present, but the required exact
  review linkage was missing from task front matter.
- Next approach: added `remediates-review: review-01.md`, then revalidated the
  resulting workflow state before retrying completion.

## Handoff to reviewer

### Implemented

Review-reservation entry-boundary regression coverage and structural
Git-backed reviewer-handoff freshness validation, including the negative stale
handoff case and the explicit non-Git artifact rule.

### Key decisions

Kept reservation validation centralized in `InspectPromptContext` rather than
duplicating its Git boundary. Used the final handoff section as the comparison
unit so historical notes remain append-only while the outgoing context must be
new.

### Files changed

Workflow transition and Git repository helpers; CLI transition tests; README,
command-reference and state-machine documentation; synchronized source/template
Developer personas; active task plan and notes.

### Verification

Full Go tests, vet, build, shell syntax/executable checks, generated-project
initialization, source/template parity, and whitespace validation passed.

### Known risks

Non-Git repositories cannot prove freshness against a committed baseline and
therefore retain the documented current-artifact completeness rule. Git-backed
freshness is textual and intentionally does not judge the semantic quality of
the role-authored handoff.

### Skipped or unresolved work

No task-scoped remediation remains. Review 01 is preserved unchanged.

### Capability impact

CAP-010 remains the expected addition after approval: durable, validated
Developer/Reviewer coordination without replacing role judgment.

### Suggested review focus

Confirm the indirect reservation boundary is complete and happens before file
creation; confirm stale handoffs fail while genuinely changed final handoffs,
clean retries, and non-Git completion continue to work; inspect the focused Git
boundary cases and source/template guidance parity.
