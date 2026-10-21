# AGENTS.md

Concoct is being rebuilt from `doc/implementation-design.md` and the bounded backlog in `roadmap.md`. Active source, tests and documentation are the authority for implemented behavior. `capabilities.md` records behavior delivered by the current candidate and intended post-integration state.

Work on one task ID at a time. Check scope, dependencies and acceptance before editing. Keep changes focused, reconcile roadmap and capabilities, and make a reviewable commit on a task branch. Reagent creates its branch in the current checkout; iragent reviews the exact commit sequentially in that same checkout without modifying it. Independent review and operator acceptance are required before integration. Do not push or publish as part of local integration.

Reagent owns task selection, implementation, checks, self-review and documentation judgment. Iragent independently judges the exact candidate and states actionable findings. Concoct coordinates their structured results and verifies task, source, execution and approval bindings; it does not grade engineering narrative or require verbatim roadmap prose in a claim. A task claim is agent-owned evidence, not an exclusive session or host callback.

Keep active task plans, decisions, checks and review records outside the product repository under `$HOME/.concoct/manual/<task-id>/` during manual construction. Agent-run evidence uses Concoct's external evidence path. Do not write operational `notes.md` or task evidence into this tree. Historical Git branches and commits are reference material to harvest sparingly for a bounded task; do not merge the old implementation or assume its behavior remains active.

For Go changes, run `gofmt` on changed Go files, `go test ./...`, `go vet ./...`, `go build ./cmd/concoct`, and `git diff --check`. Run focused smoke checks for CLI behavior. Update these checks only when the task introduces a new build or test contract.

The old Concoct skill, persona handoffs, `.concoct/current/` notes, self-hosting workflow and generated init templates are outside this rebuild. Use ordinary manually directed implementation and independent review sessions until the new controller is delivered.
