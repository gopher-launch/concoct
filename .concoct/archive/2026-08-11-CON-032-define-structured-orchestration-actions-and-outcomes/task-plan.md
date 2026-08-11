---
id: CON-032
title: Define structured orchestration actions and outcomes
roadmap-id: CON-032
status: implementation-complete
created: 2026-08-11
updated: 2026-08-11
remediates-review: review-01.md
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-032-define-structured-orchestration-actions-and-outc
  base: 95cc94cca997aa5c0c9f1710a1e7c1e3f63190ed
  archive-commit: self
  status: archived
capability-impact:
  type: none
  ids:
  rationale: Adds an executable-owned, agent-neutral contract for authorized orchestration actions and correlated outcomes without executing agents or replacing the manual workflow.
---

# Task Plan

## Goal

Define and implement the versioned, agent-neutral action and outcome contract
that lets Concoct authorize a single workflow operation, accept a correlated
claim about its result, and mechanically determine whether the claim is
consistent with the resulting repository and workflow state.

## Context

Concoct currently renders deterministic, role-specific guidance and validates
separate Developer, Reviewer, Archivist, and Git completion boundaries. It has
no common structured representation of an authorized operation or an agent's
result. Existing prompt output, process exit status, and conversational claims
are deliberately insufficient to establish a transition.

CON-010 will later execute one authorized action through an adapter, and
CON-033 will later repeat those executions subject to gates. This task supplies
their shared action, outcome, validation, and diagnostic semantics without
launching an agent.

## Why this matters

An adapter cannot safely automate a workflow handoff unless Concoct can state
what is authorized, bind it to current evidence, and reject a claimed result
that is stale, malformed, unauthorized, or contradicted by repository state.
The same contract must remain inspectable by humans and independent of any one
agent runtime.

## Current state

- `internal/prompt` renders manual role guidance from validated workflow state;
  `concoct next` presents Product Owner input but does not select work.
- `internal/workflow` is the authority for durable state, role completion,
  review outcomes, archival, and policy dispositions.
- `internal/cli` dispatches project-aware commands but has no action envelope,
  outcome schema, action registry, adapter protocol, or invocation correlation.
- The thin agent adapters provide instructions only. No command launches an
  agent, parses standard output as a workflow result, or retains execution
  logs.

## Target state

- An executable-owned, versioned JSON envelope represents authorized actions
  and outcomes independently of transport or agent runtime.
- An action registry defines each orchestratable existing workflow operation's
  role, authority, gate, preconditions, permitted effects, postconditions,
  supported outcomes, intervention behavior, and completion validator.
- Action authorization is bound to an evidence snapshot and to unpredictable
  invocation, action, task, attempt, and role identities. Ready-state Product
  Owner selection remains a correlated, validated human/agent result rather
  than an autonomous CLI choice.
- Validation gives precedence to executable authority and observed artifact,
  workflow, and repository state. Process exit status and human-readable output
  remain invocation diagnostics and cannot independently advance workflow.
- Process-based adapters can use an invocation-specific temporary JSON result
  path and atomic single-result delivery; adapter-native transports normalize to
  the same contract. Raw envelopes and logs remain ephemeral, while only
  bounded sanitized outcome facts are suitable for durable history.

## Design constraints

- Preserve CAP-001's role ownership and evidence-backed state machine; no
  structured result substitutes for semantic Product Owner, Planner,
  Developer, Reviewer, or Archivist judgment.
- Preserve CAP-004's agent neutrality and CAP-006's deterministic manual
  prompts. Prompts describe semantics but do not duplicate an authoritative
  wire schema.
- Preserve CAP-005/CAP-007 compatibility and Git safety boundaries. No result
  may authorize a branch mutation, archival, integration, or push outside the
  existing workflow and policy controls.
- Preserve CAP-009's distinction between deterministic ready-state evidence and
  Product Owner work selection.
- Use versioned JSON with transport-independent semantics; standard output and
  standard error are human-readable diagnostics, never outcome transport.
- Keep raw envelopes, prompts, logs, secrets, environment contents, arbitrary
  file contents, and unbounded tool output out of durable task history.

## Non-goals

- No `concoct exec`, direct adapter launch, timeout, cancellation, or process
  supervision; those belong to CON-010.
- No repeated lifecycle runner, configurable execution gates, retry loop, or
  approval UX; those belong to CON-033 and CON-034.
- No Codex-specific output parsing, adapter discovery/configuration, or changes
  to existing manual prompt/role-completion behavior.
- No automatic roadmap prioritization or ready-state task selection.
- No redesign of the durable task, review, archive, Git, policy, or project
  contract schemas beyond bounded references needed by the new contract.

## Working assumptions

- CAP-001, CAP-004, CAP-005, CAP-006, CAP-007, and CAP-009 limitations are
  compatible because this task adds a validator and contract boundary, not
  autonomous role work, execution, generic artifact parsing, or concurrent Git
  lifecycle support.
- The implementation may introduce focused internal packages/types and test
  fixtures where repository conventions support them; the externally observable
  schema, registry behavior, and diagnostic rules are the contract.
- CAP-012 is the next available capability identifier and will be reconciled by
  the Archivist only after accepted delivery.

## Risks and open questions

- The registry must be comprehensive enough for the existing lifecycle without
  treating a rendered prompt or a successful process as proof of completion.
- Evidence snapshots and correlation data must be stable and comparable without
  exposing sensitive or unbounded repository content.
- Future adapters may negotiate different transports; normalization must not
  weaken JSON version, atomic-delivery, or single-result guarantees.
- The Developer must distinguish contract validation from future execution:
  tests may exercise synthetic actions and outcomes, but must not add an agent
  launcher or claim end-to-end automation.

## Implementation phases

### Phase 1 — Establish the contract and registry

Status: `complete`

- Define versioned JSON action and outcome envelopes, supported outcome classes,
  bounded diagnostic fields, protocol-version compatibility, and validation
  errors.
- Introduce the executable-owned action registry and its reusable authority,
  precondition, permitted-effect, postcondition, intervention, and completion
  validation hooks for the existing workflow operations.

### Phase 2 — Bind authorization to repository evidence

Status: `complete`

- Derive authorized actions and human-facing explanations from the same
  executable-owned workflow evidence snapshot.
- Bind action and outcome correlation to invocation, task, attempt, role, and
  action identities; preserve the Product Owner selection boundary in ready
  state.
- Define and implement the precedence rules that reconcile a claimed outcome
  with observed artifact, workflow, repository, and invocation-health evidence.

### Phase 3 — Support transport normalization and safe result handling

Status: `complete`

- Provide the transport-independent result-validation boundary, including the
  process-adapter temporary result-path and atomic single-result contract.
- Reject missing, malformed, duplicate, stale, mismatched, unsupported, and
  mechanically contradicted outcomes without advancing workflow state.
- Separate bounded durable outcome facts from ephemeral raw envelopes and logs.

### Phase 4 — Verify and document the contract

Status: `complete`

- Add focused unit, workflow, CLI, and prompt tests for action authorization,
  JSON/schema compatibility, correlations, precedence, postconditions, and all
  outcome/intervention classes.
- Update normative and user/developer documentation to explain the manual-mode
  boundary, outcome semantics, diagnostics retention, and future adapter use.
- Run repository checks and prepare the required independent-review handoff.
- Review-01 remediation completed: atomic no-replace result publication and
  explicit, enforced per-action authority, precondition, intervention, and
  completion-validation definitions are covered by focused tests.

## Acceptance criteria

- Every registered orchestratable action has explicit role, authority,
  preconditions, permitted effects, success postconditions, supported outcomes,
  intervention behavior, and completion validator.
- Authorized actions and human-facing recommendations derive from the same
  evidence snapshot; ready-state evidence never autonomously selects work.
- The contract distinguishes `completed`, `blocked`, `decision-required`,
  `failed-recoverable`, and `failed-terminal` outcomes with a useful
  human-readable summary and bounded recovery/intervention information.
- Missing, malformed, duplicate, stale, mismatched, unsupported, or
  mechanically contradicted outcomes cannot advance workflow state.
- Validation treats executable authority and observed repository/workflow state
  as authoritative; process exit status and human-readable output are
  diagnostic-only.
- The JSON protocol and its validation do not depend on Codex-specific output
  conventions, and prompts do not become a second wire-schema authority.
- Raw result envelopes and execution logs remain ephemeral by default; any
  durable outcome representation is bounded and sanitized.
- Existing manual prompt rendering, explicit completion commands, policy gates,
  Git safety checks, and full workflow tests remain valid.

## Verification

- Run `gofmt` on changed Go files.
- Add focused tests for JSON decoding/encoding, protocol version handling,
  atomic/single-result delivery, correlation mismatch, stale evidence,
  unsupported action/outcome combinations, and bounded diagnostics.
- Test every registered action's authorization, precondition, postcondition,
  intervention, and completion-validation path against representative workflow
  and Git evidence.
- Test ready-state Product Owner selection and prove deterministic evidence does
  not select work without a correlated authorized result.
- Run `go test -count=1 ./...`, `go vet ./...`, `go build ./cmd/concoct`, and
  `bash -n cmd/concoct/concoct.sh`.
- Exercise manual prompt rendering and existing Developer, Reviewer, Archivist,
  and Git lifecycle fixtures to prove no regression or unintended mutation.
- Run `git diff --check` and search active documentation/templates for stale
  claims that prompts or process success alone establish completion.

## Handoff expectations

The Developer should record the selected schema/version rules, registry action
coverage, evidence-snapshot and precedence decisions, diagnostic-retention
boundary, changed files, and exact verification results in `notes.md`. Before
review, update phase status honestly and add a fresh `## Handoff to reviewer`
with focus on authorization boundaries, stale/mismatched-result rejection,
manual-workflow preservation, and accidental agent-runtime coupling.
