# Delivered capabilities

This document describes current delivered behavior, organized by capability. It is not a task history. Reconcile it in each reviewed implementation candidate so it remains accurate after integration; keep intended future work in `roadmap.md` and detailed delivery/review records in Git history and external evidence. Source remains authoritative.

## Build and operator interface

- The Go 1.27.2 module retains the `github.com/gopher-launch/concoct` identity and license. The CLI builds and reports help and version.
- A read-only Markdown parser validates bounded task, feature and epic metadata, references and dependency cycles. It reports ready tasks whose dependencies are delivered, without selecting work itself.
- Concoct resolves supported reagent and iragent model/effort settings from explicit invocation choices over the provider-recommended model. Default efforts are medium and high respectively. Role settings are retained with active work.

## Codex execution

- On Linux, the stdio app-server client checks the locally verified Codex 0.147.0 runner, initializes the experimental API, discovers models, starts explicit full-access role sessions and streams turn events and usage.
- Final-answer messages supply structured role results; malformed results do not become verdicts. The outbound schema uses provider-compatible closed objects, required properties and nullable optional values. A rejected invalid schema is identified as a definite startup failure.
- Interruption, background-terminal list/termination/cleanup and exact owned Linux process inspection support bounded stops.

## Task selection and implementation

- A fresh plain `concoct run` requires a clean named parent checkout. In one initial turn, reagent selects a ready roadmap task, creates its task branch in that checkout, maintains external task evidence, engineers and verifies it, and reports a committed candidate.
- Concoct uses no host claim/checkpoint/assessment callbacks. It does not stage, commit, classify or assess the candidate's engineering contents. It checks the reported commit against the clean task-branch HEAD and recorded base before launching a separate iragent session in the same checkout.
- Iragent receives the exact candidate and a readable external task evidence file, assesses independently and records external review evidence. A completed verdict must identify the task and candidate and have a readable external review file. Concoct checks file presence without grading narrative.

## Review, correction and authorized integration

- Requested changes route back to reagent with the prior assessment and findings. A distinct corrected commit returns to iragent for another numbered assessment. Concoct forwards agent narrative without deciding its merit.
- Reagent and iragent sessions are reused when available; a replacement session receives current source and external evidence context. Replacement turns, reassessments and integration turns retain invocation and assessment consumption. A known earlier-record assessment-counter defect remains tracked in the roadmap.
- An explicit blocker stops the current step with a simple question. An operator answer resumes the requesting role with the answer and retained context.
- An approved assessment exposes the exact reviewed commit, role summaries and review evidence. If an invocation remains, it waits for `run --approve COMMIT`; otherwise it stops at the exhausted limit without offering unusable approval. Concoct binds authorization to the approved commit and asks reagent to perform local guarded fast-forward integration.
- Reagent owns Git checks and reports integrated, failed or blocked. Concoct does not perform a Git integration transaction or publish changes. The operator initiates subsequent work.

## External state, continuation and execution limits

- Atomic external state retains repository/task context, role settings, current phase, latest result, invocation count and an in-flight turn marker saved before dispatch. An execution lock prevents concurrent runs.
- Completed handoff boundaries can continue in a later `run`. An unrecorded or uncertain in-flight outcome stops later runs for inspection rather than automatically replaying work.
- Invocation and assessment limits stop further launches without resetting consumption. Limit-exhausted output identifies each exhausted allowance and suggests an explicitly authorized higher total ceiling. An operator can increase either or both ceilings for active work; Concoct atomically records the old and new totals, retains all consumption and workflow context, and resumes the pending step only when every applicable allowance permits it. Startup limit and role-setting flags remain immutable during active work.
- Optional elapsed and inactivity limits apply to each turn; timeout stopping invokes provider cleanup and owned-process cessation checks.

## Terminal output and diagnostics

- Agent prose streams under reagent or iragent labels with blank lines between messages. Completed agent results have distinct result labels, and Concoct coordination messages identify their source.
- Structured commentary across supported phases displays its human summary without showing control fields or treating intermediate dispositions as workflow decisions. A completed result is announced only after handoff accepts its routing and evidence bindings, including phase-less final results.
- Explicit JSON output and quiet progress suppression remain available.
- Each provider execution retains startup and execution stderr in an external diagnostic file whose path appears in status. Recoverable stderr messages do not turn a successful agent result into failure. Execution failures produce concise terminal text and a retained diagnostic location.

## Demonstrated workflow and limits

Operator-run Linux/Codex trials in Curu demonstrated task selection, implementation, automatic independent review, exact-commit operator-authorized local integration, and a complete requested-changes/correction/reassessment cycle without manual prompt relaying. Both integrations ended on clean `main` at the approved commit, with durable evidence retained outside the project.

These trials establish the nominal workflow and a naturally occurring correction cycle. They do not establish exhaustive live coverage of interruption, recovery, provider failure or limit exhaustion.
