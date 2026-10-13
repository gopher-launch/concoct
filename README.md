# Concoct

Concoct is at the CON-043 repository reset baseline. The active Go program is a development skeleton: it displays help and version information, and exits unsuccessfully for workflow commands. The agent workflow described in [the accepted design](doc/implementation-design.md) is planned work in [the roadmap](roadmap.md).

## Build and smoke check

Requires Go 1.27.2 or newer.

```sh
go test ./...
go vet ./...
go build -o /tmp/concoct ./cmd/concoct
/tmp/concoct help
/tmp/concoct version
/tmp/concoct run # exits nonzero: workflow unavailable
```

The CLI also accepts `--help` and `--version`. The read-only `internal/roadmap` package accepts `## ID — title` headings followed by contiguous `Kind`, `Status` and `Priority` bullets, plus optional `Parent` and `Depends on` bullets. Those top-level metadata bullet names are reserved throughout an entry; put examples in code fences. Its `Parse` result retains task prose and reports ready tasks with delivered dependencies; it does not choose among them. Supported kinds are `epic`, `feature`, `task`; statuses are `planned`, `ready`, `delivered`; priorities are `high`, `medium`, `low`. Delivered-task headings in `capabilities.md` can resolve dependencies after detailed completed work leaves the roadmap.

The `internal/activework` package provides an external active-run store for future controller work. It defaults to `$HOME/.concoct`, accepts one explicit root override, and rejects storage inside the target repository. An execution owner writes task checkpoints, decisions, pending/final reviews and bounded receipts; `Status` reads an atomic snapshot without taking the execution lock. A selecting run can be created before a task is claimed. Interrupted writes are reconciled when the next owner acquires the store. This package is not a CLI command or a filesystem confinement mechanism.

No initialization, task selection, agent execution, review, integration or release command is implemented in this baseline.

## Construction

Use ordinary manually directed Codex sessions for one bounded roadmap task at a time. Implement from current source and the accepted design, record active task evidence outside the repository under `$HOME/.concoct/manual/<task-id>/`, and commit a reviewable candidate on a task branch. An independent review and explicit operator acceptance precede local integration. Git history, including the CON-042 branch, is reference material for selective future harvesting. CON-045, Codex app-server transport, is next after the CON-044 store candidate is accepted and integrated.

The first planned runtime scope is Linux with Codex. The broader workflow and its operator commands remain future tasks; see `capabilities.md` for behavior delivered by this skeleton.
