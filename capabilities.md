# Delivered capabilities

This ledger describes behavior in the current candidate's intended post-integration state. Task IDs identify provenance, not execution status.

## CON-043 — Go baseline

- The Go 1.27.2 module retains `github.com/gopher-launch/concoct` identity and license. The CLI builds and reports help and version.

## CON-057 — Roadmap metadata

- A read-only parser validates bounded Markdown task, feature and epic metadata, references and dependency cycles. It reports ready tasks whose dependencies are delivered, without selecting work.

## CON-045, CON-060, CON-063 — Codex transport and results

- On Linux, the stdio app-server client checks the locally verified Codex 0.147.0 runner, initializes the experimental API, discovers models, starts explicit full-access role sessions and streams turn events and usage.
- Final-answer messages supply structured role results; malformed results do not become verdicts. The outbound schema is provider compatible, with required closed object properties and nullable optional values. A rejected invalid schema is identified as a definite startup failure.
- Interruption, background-terminal list/termination/cleanup and exact owned Linux process inspection support bounded stops.

## CON-046 — Role profiles

- Concoct resolves supported reagent and iragent model/effort settings from explicit invocation choices over the provider-recommended model. Default efforts are medium and high respectively. Role settings are retained with active work.

## CON-064 — Sequential agent handoff

- A fresh plain `concoct run` requires a clean named parent checkout. In one initial turn, reagent selects a ready roadmap task, creates its task branch in that checkout, maintains external task evidence, engineers and verifies it, and reports a committed candidate.
- Concoct uses no host claim/checkpoint/assessment callbacks. It does not stage, commit, classify or assess the candidate's engineering contents. It checks the reported commit against the clean task-branch HEAD and recorded base before launching a separate iragent session in the same checkout.
- Iragent receives the exact candidate and a readable external task evidence file, assesses independently and records external review evidence. A completed verdict must explicitly identify the task and candidate and have a readable external review file. Concoct checks file presence without grading narrative.
- Prose streams with role labels; structured result summaries stay separate from Concoct's coordination status. Atomic external state retains minimal bindings, role settings, phase, latest result, invocation count and an in-flight turn marker saved before dispatch. An unrecorded turn outcome stops later runs as uncertain rather than replaying work. An execution lock prevents concurrent runs. Invocation and optional turn/inactivity limits stop further work; timeout stopping invokes provider cleanup and owned-process cessation checks.

## CON-065 — Correction, approval and integration handoff

- Requested changes route back to reagent with the prior assessment and findings. A distinct corrected commit returns to iragent for another numbered assessment. Concoct forwards agent narrative without deciding its merit.
- Reagent and iragent sessions are reused when available; a replacement session receives current source and external evidence context. Replacement turns, reassessments and integration turns retain the same invocation and assessment consumption. Elapsed and inactivity limits apply to each turn.
- An explicit blocker stops the current step with a simple question. An operator answer resumes the requesting role with the answer and retained context. Completed handoff boundaries can continue in a later `run`; uncertain in-flight execution remains stopped for inspection.
- An approved assessment exposes the exact reviewed commit, role summaries and review evidence. If an invocation remains, it waits for `run --approve COMMIT`; otherwise it stops at the exhausted limit without offering unusable approval. Concoct binds authorization to the approved commit and asks reagent to perform local guarded fast-forward integration. Reagent owns Git checks and reports integrated, failed or blocked; Concoct does not perform a Git integration transaction or publish.

## CON-066 — Readable progress and external provider diagnostics

- Agent prose streams under reagent or iragent labels with blank lines between messages. Structured commentary across supported phases displays its human summary without showing control fields or treating intermediate dispositions as workflow decisions. A completed result is announced only after handoff accepts its routing and evidence bindings, including phase-less final results. Agent results and Concoct coordination identify their sources; explicit JSON output and quiet progress suppression remain available.
- Each provider execution retains startup and execution stderr in an external diagnostic file whose path appears in status. Recoverable stderr messages do not turn a successful agent result into failure. Execution failure is reported from the provider outcome with concise terminal text and a retained diagnostic location.

## CON-056 — Prove the agent handoff workflow on real work
- Completed through operator-run Linux/Codex trials.
- First live trial demonstrated task selection, implementation, independent approval and exact-commit local integration.
- Second live trial demonstrated the complete correction loop: independent review requested changes, reagent corrected the candidate, iragent approved the replacement, and explicit operator authorization led to local integration. The workflow used five agent invocations and two independent assessments without manual prompt relaying.
- Both integrations ended on clean `main` at the approved commit. Durable evidence was retained.

CON-042 remains a frozen, undelivered historical reference.
