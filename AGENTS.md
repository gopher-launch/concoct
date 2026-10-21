# AGENTS.md

Concoct is being rebuilt from the accepted design in `doc/implementation-design.md` and the bounded task backlog in `roadmap.md`. The active source tree, tests and documentation are the authority for implemented behavior. `capabilities.md` records only delivered behavior.

Work on one task ID at a time. Check the task's scope, dependencies and acceptance criteria before editing. Keep changes focused, reconcile the roadmap and capabilities with candidate behavior, and make a reviewable commit on a task branch. Use a separate checkout when practical. Independent review and operator acceptance are required before integrating an exact candidate; do not push or publish as part of local integration.

The reagent owns task selection, implementation, checks, self-review and documentation judgment. The iragent independently judges the exact candidate and states actionable findings. Concoct coordinates their structured results and verifies task, source, execution and approval bindings; it does not grade engineering narrative or require verbatim roadmap prose in a claim.

Keep active task plans, decisions, checks and review records outside the product repository, under `$HOME/.concoct/manual/<task-id>/` during manual construction. Do not write operational `notes.md` or task evidence into this tree. Historical Git branches and commits are reference material to harvest sparingly for a bounded task; do not merge the old implementation or assume its behavior remains active.

For Go changes, run `gofmt` on changed Go files, `go test ./...`, `go vet ./...`, `go build ./cmd/concoct`, and `git diff --check`. Run focused smoke checks for CLI behavior. Update these checks only when the task actually introduces a new build or test contract.

The old Concoct skill, persona handoffs, `.concoct/current/` notes, self-hosting workflow and generated init templates are outside this rebuild. Use ordinary manually directed implementation and independent review sessions until the new controller is delivered.
