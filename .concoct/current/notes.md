# Notes

## Planning summary

CON-017 is implementation-ready. It has no roadmap dependencies, and CAP-001, CAP-004, and CAP-006 are active accepted prerequisites whose limitations are compatible with a structural ownership/composition change. The task updates the existing workflow, template, adapter, and rendering capabilities; it does not automate semantic product or role judgment.

## Confirmed findings

- The planning prompt's Git identity is exact: branch `concoct/con-017-separate-protocol-policy-and-project-guidance`, trunk `main`, and base/initial HEAD `2b99900cd1e98804644a13d7e5b1a417655824c9`.
- Before planning, `.concoct/current/` contained only `.gitkeep`, so no active task conflicted with CON-017.
- CON-017 was `planned`, has no dependencies, and declares CAP-001, CAP-004, and CAP-006 as prerequisites.
- CAP-001 permits structural validation but explicitly does not claim semantic correctness of authored guidance. CAP-004 adapters are guidance-only. CAP-006 rendering is state-preserving and conservatively source-driven. Those limitations remain intentional boundaries.
- `templates/AGENTS.md` currently mixes repository-owned project intent, architecture, coding and naming conventions with planning phases, personas, reviews, archives, and handoff policy.
- The installed skill, personas, and prompts contain Concoct protocol and policy rules; adapters point at `AGENTS.md`, but the system has no durable ownership declaration or composition validator.
- Several shipped GitHub prompts and `doc/multi-agent-workflow.md` retain obsolete persona paths such as `planner.md` and `code-developer.md`, plus the obsolete single `.concoct/current/review.md` convention. This is direct evidence of the template/adapter limitation recorded in CAP-003 and CAP-004.
- `internal/project.Initialize` copies the embedded template and substitutes content without provenance or layer validation. `internal/project.Discover` requires `AGENTS.md`, roadmap, and capabilities.
- `internal/prompt.Render` deterministically lists input paths and appends a canonical prompt source, but it neither labels instruction ownership nor checks cross-layer conflicts.

## Planning decisions

- Treat protocol, default policy, project guidance, and active task context as explicit separate sources. Avoid editable generated regions inside `AGENTS.md`, because future upgrades could not preserve their ownership safely.
- Keep `AGENTS.md` project-owned and human-facing. It remains the conventional entry point and points agents to Concoct-owned protocol/policy plus the selected persona and task context.
- Represent the existing accepted lifecycle as the default policy. CON-017 establishes the boundary and validates that default; general policy configuration belongs to CON-018.
- Update existing capabilities rather than add a new capability ID. The user-visible behavior is a refinement of CAP-001, CAP-003, CAP-004, and CAP-006.
- Limit deterministic conflict detection to declared controls and protected invariants. Preserve and attribute free-form project prose, but do not claim complete semantic contradiction detection.
- Treat legacy mixed `AGENTS.md` content as requiring explicit reconciliation. Do not infer ownership and rewrite existing clients during this task.

## Risks

- An overly broad protocol layer would make intended policy choices unconfigurable; an overly narrow layer would allow evidence-integrity weakening.
- Agents that only load `AGENTS.md` may miss referenced layers unless the entry point and thin adapters state the required read order clearly.
- Source attribution will intentionally change prompt golden fixtures and must remain byte-deterministic.
- Full semantic conflict detection is impossible for arbitrary prose. Documentation and diagnostics must describe the enforceable boundary honestly.
- Changes span live source guidance and embedded templates; drift between counterparts would create different behavior in this repository and initialized projects.

## Initial verification

- Read `AGENTS.md`, the Task Planner persona, roadmap, capabilities, all prompt-listed archive summaries, relevant workflow/adapter documentation, template guidance, project initialization, discovery, and prompt rendering code.
- Confirmed the worktree began clean on the exact recorded task branch and base.
- Confirmed no unresolved roadmap dependency or product decision blocks an implementation-ready plan.

## Developer handoff

### Current state

Planning complete; task status is `planned` on the recorded Git task branch.

### Completed

- Validated roadmap eligibility, prerequisite limitations, branch/base identity, and absence of active-task conflicts.
- Grounded the plan in the installed template, adapters, prompt rendering, discovery/initialization behavior, documentation, and accepted archive history.
- Defined scope, phases, observable acceptance criteria, verification, capability impact, and explicit non-goals.

### Remaining

- Classify existing instructions and settle the concrete file/schema layout.
- Implement the layered installed contract, deterministic composition/diagnostics, attribution, adapters, documentation, and tests.
- Update task status, notes, and reviewer handoff after implementation.

### Known risks

Protocol/policy misclassification, compatibility for agents reading only `AGENTS.md`, byte-preserving project ownership, and honest limits on free-form semantic conflict detection.

### Checks run

Repository and artifact inspection; exact Git branch/base/cleanliness checks. Planning validation and whitespace checks are to be run after both active artifacts and the selected roadmap status are updated.

### Expected next role

Developer.

### Recommended next command

`concoct code`

## Implementation findings and decisions

- Added `.concoct/protocol.md`, `.concoct/policy.md`, and project-owned
  `AGENTS.md` declarations as the narrow ownership boundary. Protocol protects
  evidence integrity, completed-review immutability, workflow-artifact
  ownership, and invalid-state refusal; policy owns phases, gates, and Git
  strategy.
- Added `internal/instruction` composition in fixed protocol, policy, and
  project-guidance order. Composition preserves source bytes, attributes each
  layer, accepts declared compatible strengthening, and returns no partial
  result for missing, malformed, unknown, or invariant-weakening declarations.
- Integrated composition before prompt state inspection/output so incompatible
  layers cannot emit partial role guidance. Rendered prompts now attribute the
  three standing layers plus task context, and exact input lists include both
  Concoct-owned sources.
- Initialization now validates the installed composition. Tests prove a
  customized `AGENTS.md` remains byte-identical through composition.
- Replaced stale template persona and single-review paths in GitHub prompts and
  workflow documentation. Thin Codex, Claude, Copilot, Aider, and generic
  adapters now route through the same attributed sources.
- Documented deterministic precedence, strengthening, structural conflict
  failure, the limit of free-form semantic analysis, and explicit legacy
  reconciliation in `doc/instruction-layers.md`. No adoption, upgrade, or
  selectable policy behavior was added.

### Attempt: status command with unsupported project flag

- Tried: checking the generated fixture with `concoct status --project <path>`.
- Error/result: `status accepts no positional arguments`.
- Why it failed: status discovers the project from its working directory and
  has no project flag.
- Next approach: ran the same script from inside the generated project; it
  reported `ready` and recommended `concoct next`.

### Attempt: manual implementation commit before completion validation

- Tried: committed the complete implementation as `0071266`, then ran
  `concoct code --complete`.
- Error/result: completion refused because no authored plan/notes changes
  remained relative to `HEAD`.
- Why it failed: the completion command owns validation and creation of the
  formal Git transition; committing first consumed its authored-change baseline.
- Next approach: preserved the implementation commit, added this durable retry
  evidence and completion evidence to the plan, and delegated the formal
  transition commit to `concoct code --complete` without rewriting history.

### Attempt: reviewer handoff headings were semantically complete but noncanonical

- Tried: completion validation with headings `Implemented work`, `Checks run`,
  and `Known risks and skipped work`.
- Error/result: validation required the exact `### Verification` heading.
- Why it failed: transition validation uses canonical handoff heading names so
  evidence can be checked deterministically.
- Next approach: renamed the handoff headings to `Implemented`, `Verification`,
  and `Known risks` while preserving their content.

## Verification results

- `gofmt` completed on all changed Go files.
- `go test -count=1 ./...` passed.
- `go vet ./...` passed.
- `go build ./cmd/concoct` passed.
- `bash -n cmd/concoct/concoct.sh` passed, and the script remains executable.
- Fresh initialization in a temporary parent copied root files, dotfiles,
  nested templates, protocol/policy, personas, prompts, planning files, and the
  bootstrap prompt; staged all generated files; created no commit; and reported
  `ready` from the generated repository.
- `git diff --check` passed.
- Stale-path search found no obsolete planner, developer, reviewer, or singular
  review artifact references in current source/template guidance.

## Handoff to reviewer

### Implemented

Introduced and documented explicit protocol, policy, project-guidance, and task
context layers; added deterministic all-or-nothing validation/composition;
integrated attributed sources into prompt rendering and initialization; updated
templates and adapters; and added focused composition, preservation, conflict,
initialization, and golden-output coverage.

### Key decisions

- Use human-readable Markdown front matter with a deliberately narrow declared
  control schema rather than attempting semantic analysis of arbitrary prose.
- Keep `AGENTS.md` project-owned and untouched by composition.
- Treat unknown strengthening targets and weakening of protected controls as
  explicit errors; retain the existing lifecycle as the sole shipped policy.

### Files changed

Protocol/policy and project guidance; `internal/instruction`, prompt rendering,
project initialization and their tests; prompt goldens; installed adapters,
skill/persona counterparts; README and workflow/state/multi-agent/layer docs;
and active task artifacts.

### Verification

All commands and generated-project checks listed under Verification results
passed. The one incorrect status invocation and corrected check are recorded
above.

### Known risks

Free-form prose conflicts remain human reconciliation work by design. Existing
mixed-ownership client repositories are documented as requiring explicit
reconciliation; migration, upgrade, adoption, and policy selection remain
out-of-scope roadmap work. The declaration parser intentionally supports only
the simple front-matter shape shipped by Concoct.

### Capability impact

Still an expected update to CAP-001, CAP-003, CAP-004, and CAP-006. Capability
truth remains unchanged pending independent approval and archival.

### Suggested review focus

Review invariant allocation and diagnostic completeness, all-or-nothing prompt
behavior, project-guidance byte preservation, default-policy equivalence,
adapter/source-template consistency, and whether the documented semantic limit
is sufficiently clear.

### Recommended next command

`concoct code --complete`, followed by `concoct review` after validation.

## Archival handoff

- Current state: approved CON-017 evidence archived on its recorded Git task branch, pending the validated archive completion commit.
- Work completed: preserved the accepted plan, notes, and both reviews; reconciled CAP-001, CAP-003, CAP-004, and CAP-006 with the approved layered instruction behavior; recorded pending-delivery roadmap evidence; and authored the archive summary.
- Work remaining: run `concoct archive --complete`, then integrate the resulting archival commit.
- Decisions: capability impact remains `update`; CON-017 stays active until Git integration, and unrestricted prose conflicts remain explicit human reconciliation work.
- Known risks: delivery is not complete until integration; current task evidence must remain intact at the archival boundary.
- Checks run: approving Review 02 records full tests, vet, build, shell syntax, fresh initialization, stale-path scanning, and diff validation; Archivist validation confirmed the clean recorded branch, base ancestry, approved review sequence, and deterministic archive destination.
- Artifacts updated: capability ledger, selected roadmap archive reference, current task Git metadata and notes, and the dated archive.
- Expected next role: integration coordinator.
- Recommended next command: `concoct integrate`.

### Attempt: archival completion with stale CAP-003 status

- Tried: `concoct archive --complete` after reconciling the four approved capability records.
- Error/result: completion refused because capability impact `update` requires CAP-003 to be an active resulting record.
- Why it failed: CAP-003 retained its pre-task `limited` status even though CON-017 resolved the stale-reference and instruction-ownership limitations addressed by this update.
- Next approach: mark CAP-003 active, preserve this retry evidence in the accepted archive copy, revalidate, and rerun the completion boundary.

### Attempt: roadmap archive field used the resolved summary path

- Tried: archival completion after activating CAP-003.
- Error/result: completion refused because the parsed roadmap archive reference did not equal the required summary path.
- Why it failed: the roadmap schema stores the archive directory and resolves it to `summary.md`; the authored field incorrectly stored the already-resolved summary path, causing the parser to append `summary.md` twice.
- Next approach: record the canonical archive directory in CON-017, preserve this retry evidence in the archive copy, and rerun validation.

## Review 01 remediation

### Finding disposition — Policy/project declarations have silent conflict behavior

Status: `fixed`.

Composition now enforces an explicit declaration-key allowlist for protocol,
policy, and project-guidance layers. If project guidance declares a
policy-owned key such as `git-strategy`, composition fails with an actionable
diagnostic naming `AGENTS.md`, the project-guidance layer,
`.concoct/policy.md`, the policy layer, and the corrective action. Other
unsupported declaration keys also fail rather than being silently discarded.
Declaration validation uses sorted keys so diagnostics remain deterministic.

Focused tests cover policy/project rejection at both composition and rendered
prompt boundaries, assert no partial output, and cover generic unsupported
declarations. Documentation now states the closed declaration schema and keeps
repository conventions in the Markdown body.

### Attempt: completion before remediation linkage

- Tried: `concoct code --complete` after implementing and documenting the
  review 01 fix.
- Error/result: transition validation reported that the developer output was
  not a valid implementation-complete state.
- Why it failed: the task plan did not yet declare
  `remediates-review: review-01.md`, so the latest changes-requested review
  remained authoritative instead of being explicitly linked to this completed
  remediation.
- Next approach: added the required task-plan linkage and retained the completed
  task status before retrying validation.

## Handoff to reviewer

### Implemented

Closed the declaration ownership gap identified in review 01. Added explicit
per-layer declaration schemas, actionable policy/project conflict diagnostics,
deterministic unsupported-key validation, focused composition tests, focused
prompt all-or-nothing coverage, and matching instruction-layer documentation.

### Key decisions

- Fail explicitly when project guidance uses a policy-owned declaration instead
  of silently applying precedence; this keeps policy selection in
  `.concoct/policy.md`.
- Reject all other unsupported front-matter declarations so parsed structured
  values cannot be silently ignored.
- Preserve unrestricted repository guidance in the Markdown body and continue
  preserving `AGENTS.md` bytes unchanged.

### Files changed

- `internal/instruction/compose.go`
- `internal/instruction/compose_test.go`
- `internal/prompt/render_test.go`
- `doc/instruction-layers.md`
- `.concoct/current/task-plan.md`
- `.concoct/current/notes.md`

### Verification

- `gofmt` on changed Go files passed.
- `go test -count=1 ./...` passed.
- `go vet ./...` passed.
- `go build ./cmd/concoct` passed.
- `bash -n cmd/concoct/concoct.sh` passed; executable mode is preserved.
- Fresh initialization copied protocol, policy, nested and dotfile content,
  personas, planning files, and bootstrap prompt; staged all files; created no
  commit; and reported `ready` from the generated repository.
- `git diff --check` passed.
- The stale canonical persona and singular-review path scan returned no matches
  outside prompt golden fixtures.

### Known risks

Free-form prose conflicts remain outside structural validation by design. The
closed front-matter schema is intentionally narrow; future configurable policy
work must extend the owning policy schema explicitly rather than relying on
unknown declarations.

### Capability impact

Still an expected update to CAP-001, CAP-003, CAP-004, and CAP-006. Capability
truth remains unchanged pending independent approval and archival.

### Suggested review focus

Confirm the allowlists match the shipped layer contract, policy-owned project
keys identify both owners and sources, unsupported keys cannot disappear
silently, and both composition and prompt rendering return no partial output.

### Recommended next command

`concoct code --complete`, followed by `concoct review` after validation.
