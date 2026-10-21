# Delivered capabilities

This ledger describes implemented behavior in the source tree. Task IDs identify the delivery that introduced each capability.

## CON-043 — Buildable reset skeleton

- A Go 1.27.2 module retains the `github.com/gopher-launch/concoct` identity and existing license.
- `concoct` builds and displays help or development version information.
- Workflow and unknown commands exit unsuccessfully with an explicit unavailable message.
- The active repository contains the accepted design, bounded rebuild roadmap and source-first construction guidance.

## CON-057 — Read-only roadmap metadata parser

- Parses task, feature and epic headings with bounded kind, status, priority, parent and dependency metadata while retaining objective and acceptance prose.
- Validates IDs, references, parent kinds and dependency cycles; ignores fenced examples and diagnoses malformed metadata.
- Reports ready task entries only when their dependencies are delivered, including IDs recorded in this ledger. It does not select tasks or launch agents.

## CON-044 — External active-work store

- Canonical Git common-directory and storage-root bindings share one run across worktrees, including repositories with a separate Git directory; one explicit root override is supported.
- A repository execution lock protects mutation while status reads atomic snapshots without that lock.
- Run/task metadata, checkpoints, operator decisions, pending/final reviews and bounded receipts use atomic file writes; interrupted cross-file writes reconcile on reopen.
- Dedicated host methods protect agreed task scope and finalized reviews from ordinary checkpoint or review updates.

## CON-045 — Codex app-server transport

- A Linux stdio client checks for the locally verified Codex 0.147.0 runner, negotiates experimental API support and reads model catalog and rate-limit snapshots.
- Thread start/resume applies explicit role settings and validates effective model, effort, full-access sandbox and approval policy. Zero-turn thread IDs remain provisional.
- Turn execution streams progress and terminal results, retains observed token receipts, and handles bound host callbacks only after validating the response and committing its receipt.
- Turn interruption and background-terminal list, terminate and clean methods are exposed. CON-051 binds these controls to exact owned-process cessation checks.

## CON-046 — Dispatch governance

- Internal policy resolves reagent and iragent model/effort pairs from explicit Concoct invocation, project and user choices over the provider-recommended model; it validates supported efforts and retains full-access settings with active work. Default efforts are medium and high respectively.
- The active-work owner exposes a pre-dispatch reservation method that persists an attempt or a task/role/activity/candidate-bound authorization wait before returning dispatch permission. Default policy grants ordinary selection, engineering, review and correction without a first-review gate; configured gates require explicit matching authorization.
- Twelve total dispatch attempts, three completed independent assessments and two recovery attempts per failure episode are starting bounds. Extensions raise ceilings without resetting consumption. Three reagent self-review passes per candidate cycle are guidance inside a turn, not a runtime dispatch counter.
- Provider quota windows, reset times and observation timestamps are separate from per-attempt token receipts. Configured warnings and next-dispatch holds use observed percentages; unavailable or stale telemetry remains unknown. Watchdogs default off.

## CON-047 — Internal selection and engineering controller

- When no active work exists, the controller requires a clean named parent branch, captures its baseline and prepares an external detached checkout. Existing active work is reported before new selection.
- A fresh reagent thread receives current repository instructions, roadmap and capabilities, eligible task context and an explicit parent binding. Dispatch is durably reserved before provider start.
- The reagent can create one task branch and call `claim_task` with its roadmap task ID before source edits. The host takes scope prose from that entry, validates eligibility, unchanged parent and clean baseline checkout, then records the task binding and callback receipt before acknowledgment. The same turn can continue engineering and save bounded checkpoints without changing agreed scope.
- Normal no-work removes the clean unclaimed checkout and releases the active pointer; a material question retains active work. Focused fake-agent and disposable Git tests cover one-turn continuation, competing claims and dirty prerequisites.

## CON-048 — Internal candidate freeze

- A completed reagent turn can request review readiness with a schema-defined result and checkpoint. Reagent owns criterion coverage, check sufficiency, self-review and documentation judgment; Concoct preserves this narrative without matching acceptance prose or command strings. Progress events still stream before result validation.
- Readiness requires resolved claim/checkpoint callbacks, an idle provider background-terminal list and an unchanged parent and task branch. Invalid or uncertain results retain active work and do not create a candidate.
- Candidate capture includes baseline and candidate revisions and trees, a binary diff digest, changed paths and the checkpoint digest. Existing reagent commits are preserved; dirty source can be staged and committed mechanically without another agent turn. Untracked deliverables are included even when ignored by Git; an omitted generated output must be individually declared and match an explicit exclusion.

## CON-049 — Internal initial independent review

- A frozen candidate can flow directly into a candidate-bound iragent dispatch under the recorded policy. The reviewer gets a separate thread with explicit resolved settings and a detached checkout pinned to the candidate. The host instructions require source-read-only review and supply the agreed task, verification evidence, candidate identity and reviewer responsibility.
- The active record binds review 01 to the candidate and agreed scope digest. Pending assessment text and the same review number survive interrupted or uncertain turns. Source or branch drift, unresolved tool activity, an invalid result or mismatched candidate cannot produce approval. A structured reviewer result supplies normal assessment prose without a mandatory callback.
- A completed approved or changes-requested result finalizes one external review-01.md with candidate, scope and verdict bindings and consumes one assessment. A genuine blocker retains the pending review. Configured authorization can pause the initial dispatch. CON-051 reconciles interrupted and observed completed turns before continuation.

## CON-050 — Internal review correction loop

- Completed findings and the active task record are sent to the bound reagent thread. A correction turn updates the checkpoint and freezes a distinct candidate against the original baseline after the same mechanical source and execution checks.
- Once the prior reviewer is idle, its clean detached checkout advances from the assessed candidate to the corrected commit. The same iragent thread receives the revised identity, prior findings and current validation evidence; completed reassessments receive successive review numbers.
- Recorded dispatch and completed-assessment limits govern each cycle. An exhausted allowance or uncertain turn retains the current task, candidate and findings with a continuation request. Routine correction and reassessment use the recorded role settings and policy without automatic escalation.

## CON-051 — Stop, reconcile and resume active work

- Internal reagent and reviewer turns persist dispatch intent, thread and turn identities, callback receipts and observed outcomes. Candidate freeze has a durable pre-commit intent. Restart inspects the exact pending turn and applies verified completion once; a missing start acknowledgment or rollout holds work for reconciliation.
- Cancellation and configured stop triggers request provider interruption, list/terminate/clean the bound thread's opaque background-terminal handles, then require exact owned Linux process identities to cease through a stable grace interval. Unrelated processes are excluded; failed or uncertain cleanup retains an abnormal stop record. Automatic host signaling is disabled.
- The active controller continues the bound task and role with recorded model, effort and full-access settings. Verified interrupted turns consume a bounded recovery dispatch without resetting prior consumption. Known zero-turn threads may be recreated; lost sessions reconstruct from current active task, review and source evidence. The controller does not select new work while an active run awaits reconciliation.

## CON-052 — Internal operator questions and task disposition

- Structured reagent and reviewer blockers persist a question ID, requesting role and attempt, objective, evidence, missing action, options with tradeoffs, safe work and next step. Unanswered questions hold the active run without polling or new task selection.
- Choice, text and answer-file contents are recorded in external task evidence before requester continuation. A source-changing reviewer question requires a receipted, candidate-bound assessment before it can be answered; its answer then becomes findings for reagent correction and renewed independent review within the agreed scope. A confirmed failed pre-turn reagent launch retains the answer for a bounded recovery retry after the requester is idle. Scope-changing questions require separate explicit authority; an answer does not grant dispatch authorization or verify an external action.
- An idle active task can be deferred or abandoned with retained evidence and released ownership. Deferred work requires explicit reactivation with its recorded settings and consumption; abandoned work is terminal and excluded from automatic selection.

## CON-053 — Internal exact-candidate local integration

- A durable acceptance ID binds the task and scope digest, candidate commit/tree, approving review number/attempt/turn and assessment digest, and expected parent branch/commit. A task ID or stale token cannot approve substitute work.
- The internal integration path requires settled execution, idle bound roles, a clean unchanged parent checkout and the exact frozen task/review source. It records intent before preparing the parent checkout, fast-forwards the parent with expected-old-ref protection, and verifies the clean checkout at the reviewed commit and tree. It does not merge, reset or publish.
- Retries classify the actual Git ref and checkout state. A clean already integrated target is recorded without repeating Git mutations; conflicting or partial states remain for reconciliation. Completion, archive and active ownership retirement remain separate.

## CON-054 — Mechanical completion and external archive

- The internal completion path verifies the accepted commit, tree, target branch and clean checkout, including when Git integration succeeded before its record was finalized. It records completion facts before maintenance, archives the bound task and final reviews outside the product repository, then retires active ownership after safe workspace cleanup. It invokes no agent and does not repeat integration.
- Interrupted archive or cleanup work resumes from the durable completion marker. Conflicting archive contents or changed and ignored workspace files remain in place with a maintenance-pending result. Completed archives are retained indefinitely by default.
- Explicit cutoff-based pruning removes completed archives in deterministic task/run order while retaining the active run and supplied task or run IDs. Archived narratives are not added to automatic selection or resume context; full-access agents are not physically barred from the external archive.

## CON-055 — Operator CLI

- `run` starts or resumes one controller task using the verified Linux Codex app-server, and `status` reads an atomic active-work snapshot without a provider call or execution lock. Fresh role model/effort and dispatch-gate choices are explicit; effective settings, quota age/windows, consumption, candidate identity and bound next actions are displayed. Structured questions and exhausted-limit holds expose their action context; acceptance results expose candidate-bound task/review evidence, verification and exact diff commands.
- Exact stable IDs route question answers, dispatch authorization, candidate acceptance/local integration, limit extensions, defer/abandon and deferred reactivation. A successful accepted integration proceeds to mechanical completion; interrupted maintenance is retried without an agent turn. Invalid or unsupported options and stale IDs fail explicitly.
- Agent messages stream before the separate verified controller result. Human, quiet and JSON-lines output retain actionable dispositions and documented exit codes. The CLI does not install project templates, reuse legacy choreography, push or publish.

## CON-058 — Dynamic-tool completion and retained recovery

- The Codex 0.147.0 transport accepts a successful completed `dynamicToolCall` item for the matching host request, call, tool, thread and turn as provider acknowledgment. It records host receipt commit, response transport and provider completion separately. An unrelated `serverRequest/resolved` notification does not acknowledge a dynamic tool.
- Callback failure, missing completion and mismatched identities hold required claim/checkpoint callbacks. Returned transport outcomes retain bound callback evidence in active attempts; diagnostic recovery records retain the specific reason, stage, operation, evidence reference and agent-reported summary.
- Restart reconciliation reads the exact completed provider turn and checks available dynamic-tool items or retained live acknowledgments against durable host receipts. Earlier active records remain readable. Incomplete callback proof retains a pause; verified completion still passes through checkpoint, source, scope and candidate identity checks before independent review.

## CON-059 — Operator output and first use

- README guidance covers prerequisites, first run, role choices, exact candidate inspection and acceptance, holds, external state, exit codes and maintenance continuation.
- The CLI emits kebab-case Concoct public JSON fields, event names and dispositions while reading existing durable records and leaving provider schemas intact. Agent message items stream with separate boundaries and structured final summaries appear apart from verified controller results; malformed structured final items receive diagnostics.
- Human status identifies pre-claim selection, specific recovery evidence and review state, missing versus measured-zero token receipts, and local timestamps with zone and offset. `TZ` selects UTC or another zone for human output; JSON and durable timestamps remain UTC instants.

## CON-060 — Final results in live and resumed turns

- Codex 0.147.0 `final_answer` agent messages supply the role result in both live and historical turn reads. An explicit final answer overrides phase-less compatibility results, including when malformed; commentary cannot supply or replace a result. Phase-less compatibility accepts only a valid structured role result.
- Recovery distinguishes exact-turn identity, terminal status, absent final text, invalid structured results and missing required callback evidence. Existing callback receipts, source bindings and candidate gates still determine whether work can advance.

## CON-061 — Retained callback acknowledgments in recovery

- Live verified callback outcomes are durably bound to the dispatch role, thread, turn, request, call and tool, with host receipt, response transport and provider completion recorded separately. Recovery can use a complete retained acknowledgment and matching host receipt when an exact completed provider turn omits historical dynamic-tool items. A matching successful historical item and host receipt can verify a call when its live completion record is incomplete but nonconflicting.
- Claim, checkpoint and optional assessment callbacks use the same bounded check. Missing or conflicting proof for a callback retains a recovery pause; a receipt or agent result alone cannot establish callback acknowledgment. Candidate, review, correction and exact integration checks remain in force.

## CON-062 — Agent-owned coordination

- Reagent claims a ready roadmap task by ID and branch; Concoct supplies objective and acceptance from that entry. A ready result and checkpoint let Concoct freeze mechanically verified source without grading criterion text, checks or documentation prose.
- Schema-defined role results carry dispositions and summaries, with task and candidate identities, review findings or operator questions when relevant. Concoct checks supplied identities against active bindings and passes narrative through. Normal review results create exact candidate-bound assessment evidence without an extra callback; requested changes return to the same reagent and revised candidates go back for reassessment within retained limits.
- Fresh runs can set retained elapsed and inactivity limits. Exceeding one cancels the turn and invokes owned-process cessation, with the exceeded limit retained as the operator-facing reason. Human status gives plain pause reasons and next actions; exact reviewed-candidate approval still authorizes local integration only.

CON-042 remains a frozen, undelivered historical reference.
