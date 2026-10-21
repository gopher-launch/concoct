# Concoct implementation design

Revision 0.6 · 2026-10-21 · Active handoff design

Concoct coordinates one roadmap task at a time. Reagent selects, engineers, verifies, documents and commits the work. Iragent independently assesses one exact candidate. Concoct invokes each agent, streams its prose, routes a small structured result and enforces execution bounds. Source and current project documents define implemented behavior; agent summaries are claims, not controller assessments.

## Repository and task selection

A plain `concoct run` starts in the operator's current checkout. With no active run, it requires a clean named parent branch and records its commit. The initial reagent turn examines `AGENTS.md`, `roadmap.md`, `capabilities.md`, relevant design and source, selects one suitable ready task with delivered dependencies, creates and switches to its task branch, then continues engineering in the same turn. A claim means the reagent has selected work and created its branch and external evidence. It establishes no exclusive task or session ownership. There is no host claim callback or separate selection turn.

The reagent maintains `task.md` outside the product repository under Concoct's evidence root. It records scope, decisions, progress, checks, self-review, candidate commit and next action. It keeps a task branch clean after committing. Reagent decides acceptance coverage, check sufficiency and documentation judgment. Concoct does not stage files, create a candidate commit, classify deliverables or parse task evidence to authorize a handoff. A no-work result ends selection without inventing a task. A genuine blocker is reported as an explicit question.

## Independent review and current slice boundary

A completed, schema-valid `ready_for_review` result identifies the selected task and candidate commit. Concoct records the reported identity, checks that the checkout is on a distinct clean task branch at that commit descended from the parent baseline, then starts a separate iragent session in the same checkout. No other Concoct agent runs concurrently in that checkout. The iragent receives the task, exact commit, base and evidence path, reviews source and criteria independently, and writes `review-01.md` outside the product repository. It does not modify the candidate while reviewing.

The iragent returns an approved or changes-requested result, with summary, findings and evidence path. Concoct preserves this narrative without grading it. This slice ends after displaying the assessment and evidence location. Correction routing, operator acceptance and exact local integration belong to CON-065. Concoct never pushes or publishes.

## Execution and state

One foreground Go binary talks to the locally verified Linux Codex 0.147.0 app-server over stdio. Each role has explicit model, reasoning effort, full-access sandbox and never-approval settings. The provider-recommended model is the default; default efforts are medium for reagent and high for iragent. Reagent and iragent use separate sessions. Sessions are convenient continuity, not task ownership; source and external evidence are the durable context.

Concoct keeps only repository and task identity, parent and candidate commits, role settings/session IDs, current step, latest result and invocation count in an external atomic state file. An execution lock prevents simultaneous coordinator runs. Agent-maintained Markdown holds engineering and review narrative. No `claim_task`, `save_checkpoint` or `save_assessment` callbacks are part of the workflow. Concoct does not reconstruct historical callback evidence or replay interrupted work automatically.

The shared provider response schema requires all declared object properties and closes additional properties. Optional values use nullable types. A result carries disposition, summary, task ID, candidate commit, findings, evidence path and an optional blocker question. Concoct validates routing fields and exact candidate identity; it treats narrative as opaque. Ordinary agent messages stream as prose with role labels and blank lines between messages. A structured commentary envelope is rendered by its human summary. Raw control JSON stays out of normal output. Controller messages describe coordination without claiming engineering verification.

A run defaults to twelve agent invocations, including both roles. A configured positive invocation limit can lower that bound for a fresh run. Optional elapsed-turn and inactivity timeouts stop an active turn; stopping invokes provider interruption, background-terminal cleanup and checks owned process cessation. A failed launch, malformed result, exhausted limit or explicit blocker is reported clearly while retaining source and external state. No general recovery subsystem or old-run migration is part of this slice.

## Active source and future work

The repository retains its module identity and license. The active source is authoritative. CON-042 remains a frozen historical reference and is not merged into the active tree. The old Concoct skill, self-hosting workflow, repository-local operational notes and generated templates are outside the rebuild. Construction uses ordinary manually directed Codex sessions and independent review.

CON-065 adds correction/reassessment routing and exact operator-approved integration. After that, CON-056 tests the complete path on useful fresh work. Before CON-065 begins, the operator confirms a small real documentation trial of this slice; record its observed selection, engineering and automatic independent review outcome externally.
