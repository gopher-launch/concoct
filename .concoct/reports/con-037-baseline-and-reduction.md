# CON-037 controlled prompt baseline and initial reduction

## Conditions and reproducibility

The deterministic baseline is the repository's Developer continuation fixture:
the same checked-in task state, `codex` profile defaults, result schema, manual
prompt composition, and fixed supervision appendix. It is offline and makes no
model invocation. Prompt bytes are exact rendered bytes; Codex token fields are
variable observations and are not estimated here. The offline command is:

```text
GOCACHE=/tmp/concoct-gocache GOMODCACHE=/home/cthain/go/pkg/mod \
  go test -v -run TestRenderRolesAndModesDeterministically ./internal/prompt
```

The separately invoked live compatibility harness uses a newly initialized
temporary project and the production execution boundary, so it cannot mutate
the active task. It requires an explicit model and reasoning profile:

```text
CONCOCT_LIVE_CODEX=1 CONCOCT_LIVE_CODEX_MODEL=<model> \
  CONCOCT_LIVE_CODEX_REASONING=<reasoning> \
  go test -v -run TestLiveCodexCompatibility ./internal/execution
```

It records the adapter-reported version, selected profile, exact prompt bytes,
native usage when available, and the named `no pre-seeded adapter cache`
condition in the private temporary invocation record. It is intentionally not
run as part of ordinary verification and has not been run for this task.

The table below is the current fixture baseline. Each fixture uses the
repository's fixed project inputs, embedded adapter result contract, default
profile, named cache condition (no live adapter cache), and the role/state
selection that production rendering uses. Native usage is unavailable in these
offline fake-adapter fixtures and is deliberately not estimated.

| Role/mode | Exact prompt bytes | Largest component | Native usage |
| --- | ---: | --- | --- |
| Product Owner / roadmap intake | 14,834 | embedded Product Owner persona | unavailable (offline) |
| Task Planner / task planning | 16,572 | embedded Task Planner persona | unavailable (offline) |
| Developer / implementation | 6,339 | embedded Developer persona | unavailable (offline) |
| Developer / review remediation | 5,923 | embedded Developer persona | unavailable (offline) |
| Reviewer / independent review | 12,374 | embedded Reviewer persona | unavailable (offline) |
| Archivist / archival | 15,577 | embedded Archivist persona | unavailable (offline) |

The test also covers continuation and blocked-recovery variants. It asserts
that every component manifest conserves prompt bytes and preserves the exact
manual prompt bytes for every mode.

## Before and after

| Role/mode | Before bytes | After bytes | Reduction |
| --- | ---: | ---: | ---: |
| Developer / implementation-continuation (supervised) | 12,415 | 6,928 | 44.2% |

The before value is the byte-identical current rendering with the pre-change
8,704-byte Developer persona. The after rendering uses the 3,217-byte compact
persona; every other component, including the supervision appendix, is
unchanged. The 5,487-byte reduction clears the task's 30% fixed-role threshold.

## Selection and preservation evidence

The pre-change persona was the dominant fixed component (8,704 bytes) of the
Developer prompt. It repeated authority, inputs, verification, handoff, and
completion material already supplied by the selected handoff and generated
prompt context. The compact persona retains role ownership, Git completion
boundary, canonical inputs, allowed and forbidden changes, scope discipline,
review remediation, phase/notes requirements, verification, the complete
Reviewer-handoff headings, and independent-review boundary.

`internal/prompt` golden rendering and `internal/execution` supervised-prompt
tests verify that manual and automated rendering use the same compact semantic
core, with only the separately measured supervision appendix added by
supervised execution. No model or reasoning profile was changed.

## Component and invocation findings

The new manifest records the fixed generated context, persona, instruction
provenance, input references, authorized updates, completion contract, and
handoff as ordered components. Dynamic Product Owner and Planner evidence is
separately attributed to roadmap/capability components. Exact and
whitespace-normalized digest links reveal repeated rendered contributions without
placing their content in the metrics export; the Developer fixtures identify
shared input/update references as repeated prompt material.

No complete agent invocation was removed. The only qualifying production change
is the compact Developer persona described above. The reported fixed-byte
reduction must not be read as a reduction in later file reads or native token
usage. The controlled fixtures show no native usage and therefore do not make a
claim about cache behavior, reasoning output, or variable model cost.

## Remaining drivers and limits

This report does not claim a reduction in later tool-read context or native
token usage. An opt-in live compatibility collection remains intentionally
separate from normal tests because it would consume allocation. The existing
fake-adapter fixtures emit no native usage, so their measurements are correctly
reported as unavailable.
