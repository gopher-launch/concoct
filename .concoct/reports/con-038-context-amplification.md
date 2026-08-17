# CON-038 context-amplification evidence

## Originating production evidence

The private CON-037 records remain the originating evidence; this tracked
report contains aggregates only and no prompt, command, path, output, repository
content, secret, or credential.

| Role | Prompt bytes | Events | Input | Cached input | Output | Disposition |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| Developer | 6,546 | 77 | 1,742,734 | 1,637,888 | 14,566 | accepted |
| Reviewer | 13,090 | 41 | 526,532 | 469,760 | 4,463 | accepted |
| Archivist | 16,234 | 24 | 306,372 | 261,120 | 4,742 | finalization failed |

Total processed input was 2,575,638 tokens. Cached input is reported separately
and is neither assumed free nor converted into subscription or monetary cost.

## Deterministic controlled comparison

The offline role fixtures use the same checked-in repository evidence,
structured outcome schema, Codex adapter profile, workspace-write safety mode,
and no-live-cache condition. They make no model call, so token and exchange
fields are unavailable rather than estimated.

```text
GOCACHE=/tmp/concoct-go-cache go test -v -count=1 \
  -run TestRenderRolesAndModesDeterministically ./internal/prompt
```

| Role/mode | CON-037 bytes | CON-038 bytes | Change |
| --- | ---: | ---: | ---: |
| Developer / implementation | 6,339 | 6,339 | 0.0% |
| Reviewer / independent review | 12,374 | 12,659 | +2.3% |
| Archivist / archival | 15,577 | 9,339 | -40.0% |

The corrected Archivist persona is 5,293 bytes versus its 11,531-byte baseline,
a 54.1 percent reduction that still exceeds the plan's persona threshold. The
Reviewer gained 285 bytes to make its machine-required outcome syntax explicit
after the first live failure exposed the omission. The remaining prompts
includes generated authority, exact inputs, completion schema, policy/project
guidance, and handoff context. Shared interaction discipline moved to the
single embedded protocol contribution rather than being repeated by each
persona and handoff.

## Semantic and containment evidence

Deterministic Codex JSONL fixtures now cover native command/file-change/message
mapping, ambiguous commands, bounded fingerprints, repeated activity,
command-output bytes, usage snapshots, unsupported availability, malformed and
oversized events, terminal ordering, hard elapsed stops, late-result rejection,
and accepted/wasted aggregation. Metrics retain only categories, counts,
bounded digests, sizes, status, and provenance by default. Raw JSONL is opt-in,
private, redacted, bounded, and Git-ignored.

Warning-only role defaults are based on the production aggregates above and do
not terminate work. Elapsed, activity, and command-output hard budgets are
explicit configuration because those dimensions are observable live. Input and
output token warnings are terminal-only for the supported stream and therefore
cannot be configured as hard bounds.

## Live compatibility and processed-input benchmark

The isolated Developer/Reviewer/Archivist harness is:

```text
CONCOCT_LIVE_CODEX_ROLES=1 \
CONCOCT_LIVE_CODEX_MODEL=<model> \
CONCOCT_LIVE_CODEX_REASONING=<reasoning> \
CONCOCT_LIVE_CODEX_EVIDENCE_DIR=<absolute-empty-evidence-directory> \
GOCACHE=/tmp/concoct-go-cache \
go test -v -run TestLiveCodexRoleBenchmarks ./internal/execution
```

It creates three disposable Git repositories and records role, fixture,
adapter version, model, reasoning, sandbox, schema, cache condition, prompt
bytes, activity, command-output bytes, native usage, disposition, and
acceptance. It cannot mutate the active task.

The operator approved one run on 2026-08-14 with Codex CLI 0.147.0,
`gpt-5.6-sol`, and `medium` reasoning. The suite ran for 432.72 seconds and all
three isolated fixtures failed their required completion/finalization boundary:

| Role | Duration | Result |
| --- | ---: | --- |
| Developer | 71.64s | Returned an accepted structured `blocked` non-completion outcome. |
| Reviewer | 82.75s | Finalization rejected `review-01.md` because its body did not document exactly one `Outcome` matching front matter. |
| Archivist | 278.33s | Finalization rejected the candidate because roadmap item `APP-001` did not reference `.concoct/archive/2026-08-14-APP-001-demo/summary.md`. |

The original harness failed before its success-only log statement emitted
normalized activity, command-output, and native usage summaries. It has since
been corrected to require an absolute durable evidence directory before launch
and to export each role's create-only metrics-first JSON before evaluating
lifecycle success. Regression coverage proves that an accepted structured
non-completion retains usage, semantic activity, command-output, reconciliation,
and disposition evidence, and that an existing role evidence file cannot be
overwritten.

No role result from the first live attempt was accepted as a completed
benchmark, so that run did not establish the then-required 50 percent
processed-input reduction or successful current-version compatibility. The
later operator disposition below supersedes the former retry requirement. The
deterministic implementation and Archivist persona threshold remained
verified.

## Live fixture correction

Offline diagnosis found three independent contract defects in the first live
suite rather than evidence of an incapable profile:

- The Developer fixture supplied only task metadata and an empty `# Task Plan`,
  so a `blocked` outcome was reasonable. It now supplies a one-file exact-content
  task, constraints, acceptance criteria, and an executable check.
- The Reviewer persona required one semantic outcome but its suggested artifact
  left `## Outcome` empty. It now demonstrates matching front matter and one
  exact backticked body value and states the lexical matching rule.
- The compact Archivist persona referred generically to supported roadmap
  evidence without giving the directory-form `Archive` syntax enforced by the
  validator. It now gives the exact Git/non-Git form and explicitly forbids the
  commonly mistaken `summary.md` file form.

`TestLiveRoleBenchmarkFixturesReachCompletionOffline` drives the exact three
benchmark starting states through the production execution and deterministic
completion boundaries with a non-billable fake adapter. It proves Developer
reaches `implementation-complete`, Reviewer reaches `approved`, and Archivist
reaches `archived`. Prompt assertions also prove that each live role receives
its corrected benchmark/completion contract. These checks establish local
fixture validity but do not replace the separately approved live comparison.

## Second operator-approved live attempt

The corrected suite ran on 2026-08-14 with Codex CLI 0.147.0,
`gpt-5.6-sol`, and `medium` reasoning. Create-only metrics survive at
`/tmp/concoct-con038-live-evidence-OinrtY`.

| Role | Prompt bytes | Events | Activity | Commands | Command output | Input | Cached input | Output | Reasoning output | Duration |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Developer | 6,928 | 17 | 12 | 3 | 3,366 | 99,754 | 78,592 | 2,009 | 211 | 61.67s |
| Reviewer | 13,248 | 21 | 13 | 6 | 8,788 | 151,456 | 125,184 | 3,388 | 716 | 85.38s |
| Archivist | 9,928 | 16 | 11 | 3 | 7,638 | 93,882 | 50,432 | 5,101 | 1,149 | 107.67s |
| **Total** | **30,104** | **54** | **36** | **12** | **19,792** | **345,092** | **254,208** | **10,498** | **2,076** | **254.72s** |

Against the retained originating production observations, input was 94.3
percent lower for Developer (with the same Sol/medium profile), 71.2 percent
lower for Reviewer, 69.4 percent lower for Archivist, and 86.6 percent lower in
aggregate. Generic stream events were 62.0 percent lower, from 142 to 54.
However, the benchmark task was deliberately much smaller than the originating
production work. These are directional observed differences, not a controlled
or causal percentage-reduction claim. Current Codex 0.147.0 usage/activity
mapping is observed as supported.

Lifecycle acceptance is still unresolved because all three candidates reached
structured `completed` but failed deterministic finalization:

- Developer left durable task status `planned`; the completion error was also
  empty after the prefix, reproducing the known diagnostic defect.
- Reviewer produced review-approved evidence but lacked executable reservation
  provenance, reproducing the known supervised-run reservation defect.
- Archivist updated current notes after copying archive notes, so the two were
  no longer byte-identical.

These failures do not invalidate the retained measurements, but they do fail
the acceptance requirement that the optimized suite avoid increased
finalization/manual-repair cost. Developer guidance now makes the durable
`implementation-complete` transition explicit, the Reviewer benchmark starts
from executable-owned committed reservation evidence, and Archivist guidance
orders its final notes handoff before the byte-identical archive copy. Those
follow-up corrections are offline-verified; no third live attempt has been
authorized or run.

## Operator disposition

On 2026-08-14 the operator stopped further billable benchmarking after CON-037
and CON-038 consumed approximately 45 percent of weekly model allocation. The
operator accepted removal of the arbitrary 50 percent equivalent-workload gate
and successful-live-finalization requirement from CON-038 rather than spending
more allocation to tick a box.

Accepted evidence is therefore limited to:

- supported Codex CLI 0.147.0 semantic and native-usage capture;
- durable metrics for completed, blocked, and finalization-rejected outcomes;
- explicit accepted/wasted disposition and invariant diagnostics;
- deterministic privacy, budget, hard-stop, prompt, persona-size, and
  completion-boundary verification;
- the 54.1 percent Archivist persona reduction; and
- directional live observations with their workload-comparability and
  lifecycle-finalization limitations stated plainly.

No causal live cost-reduction percentage is claimed. No further billable model
call is authorized or required for CON-038. Supervised lifecycle reservation,
completion diagnostics, and recovery remain separate documented defects.
