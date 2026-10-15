---
version: 1
project: concoct
updated: 2026-10-13
---

# Concoct implementation roadmap

Implementation scope 0.5

Deliver the accepted operating model in implementation design revision 0.5: one reagent selects and completes one bounded task, iragent independently reviews and revisits corrections in the same session, and the operator approves the exact candidate for local integration. Active work survives interruption; source remains the truth of implemented behavior.

The source tree includes internal selection, engineering, candidate freezing, bounded independent review/correction, stop/reconciliation/resume and operator interventions. The CLI remains a buildable skeleton; the complete workflow remains planned. CON-053 is the next ready task. Task IDs CON-001–CON-042 remain reserved in Git history; CON-042 is an undelivered frozen reference, not an active or delivered capability. CON-043–CON-057 retain their identities here.

## Delivery rules

- Execute and review **one task at a time**. A milestone groups outcomes; it does not authorize one agent to implement its whole task range. If a task proves too large or exposes a material product ambiguity, retain its checkpoint and bound/revise that task before expanding work.
- `CON-053` is ready after CON-052 delivery. Later tasks remain planned until their dependencies are delivered. Promote the next eligible task through candidate roadmap reconciliation, reviewed with that delivery; no separate planning persona or routine operator gate is needed for satisfied dependencies.
- Each implementation candidate includes relevant documentation, task-specific verification and concise roadmap/capability reconciliation. Independent review and operator acceptance precede integration. During bootstrap, use ordinary Codex sessions; do not require the unfinished Concoct runner to orchestrate its own construction.
- Keep the repository/module identity and Git history while using the clean CON-043 rebuild skeleton. Preserve CON-042 as frozen reference material outside the active tree. Harvest small pieces only when a later bounded task needs them; do not merge/cherry-pick the old implementation wholesale.
- Build Concoct through manually directed reagent/iragent sessions for this entire roadmap. Do not invoke or maintain the existing Concoct skill, and do not use the unfinished runner to build itself. The reset CLI truthfully exposes only implemented commands; there is no legacy runtime compatibility/cutover obligation. CON-055 completes the new operator interface.
- Every task uses focused non-model tests. Run broader module/build checks when code changes require them. Do not spend provider turns to validate every internal step; the final live trial proves the integrated behavior.

## Delivery checkpoints

| Checkpoint | Tasks | Reviewable result |
| --- | --- | --- |
| M0: clean rebuild baseline | CON-043 | Integrated minimal buildable CLI and current guidance, with no legacy engine or skill active |
| M1: trusted execution foundation | CON-057, CON-044–CON-046 | Small roadmap parser, external active store, app-server client and enforced dispatch policy, verified with fixtures/fakes |
| M2: automatic candidate/review flow | CON-047–CON-050 | Continuous selection/engineering and independent review/correction through the new controller; candidate ready for operator acceptance |
| M3: durable interventions | CON-051–CON-052 | Verified stop/resume, reconstruction from active evidence, operator answers and task disposition |
| M4: accepted local delivery | CON-053–CON-054 | Exact-candidate integration and mechanical completion/archive lifecycle |
| M5: nominal CLI and real proof | CON-055–CON-056 | Complete `concoct run` operator experience and one fresh real-task demonstration |

The order is deliberately sequential for the first delivery. The dependencies below identify concrete prerequisites rather than suggesting parallel agent work. Intermediate milestones are development checkpoints, not claims that a usable redesigned release has shipped.

## Backlog organization

The epic/feature IDs below organize work and are not selectable execution units. Only task entries can be claimed.

## CON-E001 — Deliver the autonomous, governed engineering workflow

- Kind: epic
- Status: planned
- Priority: high
- Parent: none
- Depends on: none

Complete the nominal selection → engineering → independent review/correction → operator acceptance → local integration flow with durable active work and readable progress.

## CON-F001 — Establish trusted execution and active-work foundations

- Kind: feature
- Status: planned
- Priority: high
- Parent: CON-E001
- Depends on: none

CON-043 establishes the clean baseline; CON-057 and CON-044–CON-046 establish backlog parsing, external durability, provider transport and dispatch governance.

## CON-F002 — Deliver a reviewed candidate without operator relaying

- Kind: feature
- Status: planned
- Priority: high
- Parent: CON-E001
- Depends on: none

Tasks CON-047–CON-050 let reagent select and engineer continuously, then let iragent assess and reassess exact candidates.

## CON-F003 — Preserve work through stops and genuine decisions

- Kind: feature
- Status: planned
- Priority: high
- Parent: CON-E001
- Depends on: none

Tasks CON-051–CON-052 provide stop/recovery and operator intervention without abandoning the current task or resetting its allowances.

## CON-F004 — Integrate and finalize exactly accepted work

- Kind: feature
- Status: planned
- Priority: high
- Parent: CON-E001
- Depends on: none

Tasks CON-053–CON-054 implement guarded local integration and mechanical completion/history retention.

## CON-F005 — Make the complete workflow nominal and prove it

- Kind: feature
- Status: planned
- Priority: high
- Parent: CON-E001
- Depends on: none

Tasks CON-055–CON-056 expose the complete operator interface and demonstrate it on fresh real work.

## CON-043 — Reset the repository to a clean rebuild baseline

- Kind: task
- Status: delivered
- Priority: high
- Parent: CON-F001
- Depends on: none

Delivered baseline: minimal Go CLI, current guidance, accepted design and rebuild backlog; legacy runtime, generated templates, local skill and release machinery retired.

## CON-057 — Parse the bounded roadmap metadata contract

- Kind: task
- Status: delivered
- Priority: high
- Parent: CON-F001
- Depends on: CON-043

Delivered parser: read-only validation of bounded roadmap metadata, references and dependency cycles, with task prose retained and eligible ready tasks reported. No selection or runner behavior was delivered.

## CON-044 — Persist active work and expose a read-only snapshot

- Kind: task
- Status: delivered
- Priority: high
- Parent: CON-F001
- Depends on: CON-057

Delivered store: an external active-work store binds one run to its repository and parent, keeps task/review/receipt evidence with atomic writes and recoverable pending transitions, enforces one execution owner, and serves read-only status snapshots. Host methods separate checkpoint, decision and review writes.

## CON-045 — Implement the verified app-server transport

- Kind: task
- Status: delivered
- Priority: high
- Parent: CON-F001
- Depends on: CON-044

Delivered transport: Linux Codex app-server 0.147.0 stdio handshake and version check, model/quota reads, explicit thread/turn settings, streaming lifecycle events, continuing-turn host callbacks, and interruption/background-terminal controls. Fake-server tests cover interleaved progress, malformed callbacks, settings mismatch, provider disconnect and zero-turn loss. This package does not implement controller policy or prove process cessation.

## CON-046 — Govern every agent dispatch and retain consumption

- Kind: task
- Status: delivered
- Priority: high
- Parent: CON-F001
- Depends on: CON-045

Delivered policy: provider-catalog role resolution with explicit Concoct invocation/project/user precedence, supported model/effort validation, and retained full-access settings. The active-work owner exposes a durable pre-dispatch reservation and exact authorization wait method. Default grants cover selection, engineering, review and correction; configured gates bind task, role, activity and candidate. The state retains dispatch, assessment and recovery consumption across resume, supports upward limit extensions, and keeps provider quota windows and token receipts separate. Quota warnings, next-dispatch holds and unknown-telemetry fallback are available; watchdogs default off. Controller routing and threshold-triggered interruption remain later work.

## CON-047 — Select, create the task branch and engineer in one turn

- Kind: task
- Status: delivered
- Priority: high
- Parent: CON-F002
- Depends on: CON-046

**Objective:** let reagent choose one suitable task and continue engineering without an operator proxy or planning handoff.

**Acceptance criteria:**

- Check external active work first. Otherwise require a clean named current branch, capture it as parent/baseline and prepare an external task checkout. Dirty/unbound new work returns a prerequisite error before agent dispatch.
- Supply automatic reagent/shared instructions and current roadmap/source/documentation. Reagent chooses the highest-priority suitable ready task, creates its task branch from baseline, calls `claim_task`, then continues ordinary engineering in that same turn.
- Claim validates one task, readiness/references/dependencies, unchanged parent baseline and clean pre-implementation task contents. A second different claim is rejected; retries cannot create another binding even with a new callback ID.
- `save_checkpoint` updates current plan/progress/evidence without unauthorized scope changes. No-work or a material selection question is explicit; no fabricated work, archive lookup or sibling-task expansion.
- Launch without another task's thread/context; source changes before an acknowledged valid claim cannot be accepted as a candidate.

**Verification:** controller/fake-agent plus disposable Git tests proving claim → branch binding → engineering/checkpoint within one turn, prerequisite rejection without dispatch, two competing claims and normal no-work. No live turn required.

Delivered internal controller: fresh reagent selection uses current instructions and roadmap context, a clean parent baseline and external task checkout. Host callbacks bind one eligible task after branch creation and before source changes, then persist engineering checkpoints in the same turn. Durable dispatch reservation precedes provider start. Normal no-work retires the unclaimed checkout and active pointer; decision requests retain active work. CLI workflow and correction routing remain later work.

## CON-048 — Freeze a verified and reconciled candidate

- Kind: task
- Status: delivered
- Priority: high
- Parent: CON-F002
- Depends on: CON-047

**Objective:** turn reagent readiness into an exact candidate that includes implementation, verification and roadmap/capability reconciliation.

**Acceptance criteria:**

- Reagent performs plan/implementation/validation/self-review continuously. Task criteria and configured mandatory project checks have candidate-bound evidence, with exact command/source paths from applicable instructions, contributor/testing docs, build/task scripts or CI. Record relevant additional checks and any genuinely unavailable check with its explicit acceptance decision.
- Ready result requires a completed provider turn, valid disposition/checkpoint and idle tool work. Display progress immediately; result validation only controls state transitions.
- Reconciliation is in candidate source and preserves task IDs; no parent roadmap/capability update occurs before accepted integration.
- Capture baseline/candidate revision and tree, diff digest and changed paths. Respect explicit generated-output exclusions; untracked deliverable source cannot disappear. Validate ordinary reagent commits or mechanically finalize identical contents without another agent turn.
- Invalid/missing results or unresolved mandatory callbacks retain work and bounded recovery; they never imply readiness or approval.

**Verification:** Git/candidate and role-result fixtures for changed/untracked files, identical-content commit finalization, incomplete verification, callback uncertainty and reconciliation in the reviewed diff.

Delivered internal freeze path: a completed reagent turn with resolved callbacks and idle background tools can present a structured final checkpoint. The host checks acceptance and mandatory-check evidence, post-integration roadmap/capability reconciliation and the complete source path set. It accepts ordinary reagent commits or makes a mechanical commit of the same staged contents, then records baseline/candidate revisions and trees, diff digest, changed paths and checkpoint digest. Unresolved or invalid results retain active work without creating a candidate. Initial independent review is delivered by CON-049; correction routing remains later work.

## CON-049 — Independently assess an exact candidate

- Kind: task
- Status: delivered
- Priority: high
- Parent: CON-F002
- Depends on: CON-048

**Objective:** invoke a separate iragent session automatically against the frozen candidate and active task evidence.

**Acceptance criteria:**

- Automatic iragent/shared instructions identify task acceptance, exact candidate, validation evidence and reviewer responsibility; the operator does not paste personas.
- Use a distinct role thread and a checkout pinned to the candidate, with explicit resolved high effort/full access. Reviewer source-read-only behavior is an instruction; detect changed candidate contents before accepting a verdict.
- Preserve a pending assessment across interruption and finalize one `review-NN.md` for each completed approved/changes-requested verdict. Bind it to candidate and agreed scope revision.
- Approved, changes-requested and genuine blocker results remain distinct; provider completion or older approval is insufficient. Initial dispatch follows CON-046's automatic policy.

**Verification:** fake role/provider tests for thread separation, candidate/scope binding, source drift, interrupted assessment reuse and valid verdict routing. Reviewer/high real execution is covered in CON-056, not another standalone probe.

Delivered internal initial review: after candidate freeze, the controller can automatically reserve the candidate-bound iragent dispatch and start a separate explicitly configured reviewer thread in a detached checkout. The active record binds the pending assessment to candidate and agreed scope, retaining its number and checkpoint through interruption. The host checks candidate source integrity and completed, idle reviewer execution before finalizing review-01.md as approved or changes requested; a genuine blocker remains pending. Configured dispatch authorization still applies. Correction routing is delivered in CON-050 and interrupted-turn reconciliation in CON-051; broader live reviewer execution remains later work.

## CON-050 — Close the bounded review/correction loop

- Kind: task
- Status: delivered
- Priority: high
- Parent: CON-F002
- Depends on: CON-049

**Objective:** route review findings to reagent and resume the existing iragent against each revised candidate until approval or a genuine hold.

**Acceptance criteria:**

- Findings and relevant decisions reach the existing reagent without operator relaying; correction, validation and self-review produce a new frozen candidate.
- Update the reviewer checkout only while idle; resume the same iragent thread with exact revised identity and correction evidence. Session identity is separate from candidate identity.
- No new reviewer by default, no routine first-review/correction gate and no automatic model/effort escalation for disagreement.
- Completed assessments, candidate cycles and dispatch/recovery bounds are consumed consistently. Exhaustion retains the current task/candidate/findings and supplies an exact continuation/extension action.

**Verification:** an integrated fake-agent cycle with a real disposable Git candidate change, followed by same-reviewer approval; tests for stale approval, interrupted assessment counting, source changes during review and exhausted parent allowance.

Delivered internal correction flow: completed findings return to the existing reagent thread, and a new validated checkpoint freezes a distinct candidate. Once the prior reviewer is idle, its detached checkout advances to that candidate and the same iragent thread reassesses it with the prior findings. Each completed verdict consumes one numbered assessment; recorded dispatch limits can retain the task and findings for an explicit extension. Stop/restart reconciliation is delivered in CON-051.

## CON-051 — Stop, reconcile and resume actual work safely

- Kind: task
- Status: delivered
- Priority: high
- Parent: CON-F003
- Depends on: CON-050

**Objective:** implement the demonstrated interrupt/cleanup/resume path and durable recovery without replaying uncertain work.

**Acceptance criteria:**

- Foreground Ctrl-C and configured stop triggers request interruption, list/terminate/clean the owned thread's background terminals and verify cessation before continuation or checkout replacement. Opaque provider handles are not treated as host PIDs.
- Identify owned process activity separately from unrelated sessions; uncertainty or failed cleanup returns abnormal stop with evidence retained. A host-signaling fallback remains disabled for automatic use until its exact-ownership/cessation test passes.
- Restart/resume the appropriate recorded thread with explicit settings and retained consumption. No new task selection while existing work awaits reconciliation.
- Persist intent/receipts around launch, turn, callback and candidate operations; reconcile observed completion once. A missing acknowledgment/rollout never proves uncertain work did not happen.
- Known undispatched zero-turn threads may be recreated; truly lost sessions reconstruct from current task/reviews/source, without transcripts or archives. Recovery obeys the parent limits.

**Verification:** fake crash-point tests plus a focused installed-runner/child fixture check through the implemented client, reusing the final probe mechanism rather than repeating exploration. Cap its extra model turns at two unless a concrete new failure justifies an explicit extension. Prove stopped heartbeat/processes, retained evidence, explicit resume settings and no command replay. Report fallback coverage honestly.

Delivered internal stop/recovery: canceled or configured-triggered turns record stop intent, request provider interruption, clean owned background terminals and require exact owned-process cessation before safe continuation. Pending attempts retain thread/turn identity and outcomes; restart observes the exact turn, applies verified completion once, or uses a bounded recovery dispatch after a verified interrupted stop. Missing turn acknowledgments remain held for reconciliation. Lost sessions reconstruct from current active evidence with recorded role settings. Automatic host signalling remains disabled.

## CON-052 — Route genuine answers and preserve deferred work

- Kind: task
- Status: delivered
- Priority: high
- Parent: CON-F003
- Depends on: CON-051

**Objective:** let operators answer real blockers and defer/abandon work without relaying agent messages or losing task state.

**Acceptance criteria:**

- Pending questions record requester, blocked objective, attempts/evidence, missing input/action, useful options/tradeoffs, safe remaining work and next step. Known unanswered blockers do not poll agents or permit new selection.
- Choices, free text and answer files are durable before dispatch; resume requester by default. Source-changing reviewer decisions route through reagent and renewed review, with scope authority explicit.
- An answer does not grant unrelated dispatch authorization or prove an external action succeeded. Agents verify material conditions and ask only focused remaining questions.
- Defer/abandon requires idle execution, preserves evidence and releases active ownership without integration. Explicit deferred resume requires no competing active task and retains its settings/consumption.

**Verification:** both-role blocker routing, durable answer before failed launch, stale/wrong question rejection, remaining authorization wait, deferred reactivation and terminal abandon tests.

Delivered internal intervention handling: structured reagent/reviewer questions bind the requesting attempt and retain context in active evidence. Choice, text and file contents persist before requester continuation; source-changing reviewer answers within the agreed scope become findings for reagent correction and renewed review. Questions that request scope revision remain held for separate explicit authority. Answers do not authorize pending dispatches or prove external actions. Idle-only defer and abandon release active ownership while retaining records; deferred tasks require explicit reactivation, and abandoned tasks remain unavailable to automatic selection.

## CON-053 — Integrate only the exact operator-accepted candidate

- Kind: task
- Status: ready
- Priority: high
- Parent: CON-F004
- Depends on: CON-052

**Objective:** bind acceptance to a reviewed candidate and unchanged parent, then integrate locally with recoverable Git mechanics.

**Acceptance criteria:**

- Acceptance ID binds task/scope revision, candidate identity, approving assessment and expected parent baseline. Task ID alone cannot approve substitute work.
- Verify matching candidate/review, idle execution, unchanged expected parent and clean parent checkout. Reject drift; do not silently rebase, merge, reset or push.
- Fast-forward the parent to the exact reviewed commit with expected-old-ref protection and checkout/index reconciliation. Persist integration intent before mutation.
- On interruption distinguish not-integrated, successfully-integrated and uncertain/conflicting outcomes from actual Git state. Do not replay successful integration or discard external changes.

**Verification:** disposable repository tests for stale acceptance, candidate/scope/target drift, concurrent ref changes and interrupted ref/checkout transitions. Prove exact resulting commit/tree and no publication.

## CON-054 — Finalize completion and retain operator history mechanically

- Kind: task
- Status: planned
- Priority: high
- Parent: CON-F004
- Depends on: CON-053

**Objective:** close integrated work, archive compact evidence outside the product and retire active ownership without another agent turn.

**Acceptance criteria:**

- Verify successful integration, finalize task/reviews/completion facts and retire active binding. Keep candidate reconciliation already integrated; do not regenerate a product ledger.
- Default retention is indefinite. Operator-requested/configured pruning is deterministic, excludes active/needed supplied records and requires no provider invocation.
- Cleanup/archive errors report maintenance pending after successful integration and resume maintenance only. Retain recoverable records/workspaces until cleanup is safe.
- Archive narratives never enter automatic selection/resume context. There is no agent archive browser or routine archive-permission question; full access is not described as physical exclusion.

**Verification:** archive/cleanup interruption and retry fixtures, maintenance failure after integration, pruning exclusions, no provider calls and fresh-task context without prior narratives.

## CON-055 — Complete the new operator CLI

- Kind: task
- Status: planned
- Priority: high
- Parent: CON-F005
- Depends on: CON-054

**Objective:** expose the complete new controller through the accepted operator commands. The legacy runtime is retired by CON-043; no migration or compatibility cutover is required.

**Acceptance criteria:**

- `run`/`status`, question answers, dispatch authorization, exact-candidate approval, extensions, defer/abandon and deferred resume invoke the new controller with documented stable IDs/exit dispositions. Unsupported/obsolete options fail clearly; no old choreography is available.
- Human output streams agent commentary/responses before completion and distinguishes reported progress from verified Concoct status. Show effective settings, quota observation age, consumed bounds, candidate and exact next commands.
- Quiet/JSON modes preserve actionable dispositions; status works during execution without an exclusive lock or model call. No-work is normal, not a failure loop.
- Update nominal README/command guidance and relevant project-entry examples. Do not automatically install state/prompts/notes into target repositories; removed legacy commands/resources do not reappear in this supported path.

**Verification:** CLI/controller integration tests for nominal flow, holds/answers/gates, quiet/JSON, status while running, invalid/stale IDs and unsupported-option rejection. Run full Go tests/vet/build and `git diff --check` for the complete interface.

## CON-056 — Prove the golden path on fresh real work

- Kind: task
- Status: planned
- Priority: high
- Parent: CON-F005
- Depends on: CON-055

**Objective:** demonstrate the complete supported Linux/Codex workflow on one fresh, genuinely useful bounded task in a small external project and report its practical limits. This does not require Concoct to build itself.

**Acceptance criteria:**

- A healthy roadmap and clean parent let one `concoct run` select one task, create its branch, engineer/validate/self-review, invoke iragent/high and present a reviewed candidate without persona copying, archive questions or operator relaying.
- The operator inspects/approves the exact candidate; Concoct integrates it locally, finalizes external evidence and leaves the agreed parent clean. No push/deployment is implied.
- Correction, intervention, limits, interruption/recovery and integration faults have focused automated evidence from preceding tasks. Do not manufacture all failures in this billable real trial or require a defect solely to force a correction cycle.
- Record supported runner/configuration, actual role settings, observed turns/token/quota evidence, task-criterion results and any limitations. Do not report account deltas as exact task spend or claim universal provider compatibility.

**Verification:** one real-task run and normal operator acceptance, within configured bounds. The task's product/acceptance must actually be ready; do not invent requirements. Review source/checkpoint/reviews and normal project checks. Further live dispatches require an explicit task/budget reason, not an open-ended convergence experiment.

## Next task boundary

CON-053 is the next ready task. Construction uses manually directed Codex sessions; the existing Concoct skill and self-hosting workflow are excluded.
