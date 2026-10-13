# Concoct

Concoct is at the CON-043 repository reset baseline. The active Go program is a development skeleton: it displays help and version information, and exits unsuccessfully for workflow commands. The agent workflow described in [the accepted design](doc/implementation-design.md) is planned work in [the roadmap](roadmap.md).

## Build and smoke check

Requires Go 1.23 or newer.

```sh
go test ./...
go vet ./...
go build -o /tmp/concoct ./cmd/concoct
/tmp/concoct help
/tmp/concoct version
/tmp/concoct run # exits nonzero: workflow unavailable
```

The CLI also accepts `--help` and `--version`. No initialization, task selection, agent execution, review, integration or release command is implemented in this baseline.

## Construction

Use ordinary manually directed Codex sessions for one bounded roadmap task at a time. Implement from current source and the accepted design, record active task evidence outside the repository under `$HOME/.concoct/manual/<task-id>/`, and commit a reviewable candidate on a task branch. An independent review and explicit operator acceptance precede local integration. Git history, including the CON-042 branch, is reference material for selective future harvesting. CON-057, roadmap metadata parsing, is next after the reset candidate is accepted and integrated.

The first planned runtime scope is Linux with Codex. The broader workflow and its operator commands remain future tasks; see `capabilities.md` for behavior delivered by this skeleton.
