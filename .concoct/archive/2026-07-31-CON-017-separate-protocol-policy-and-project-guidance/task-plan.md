---
id: CON-017
title: Separate protocol, policy, and project guidance
roadmap-id: CON-017
status: implementation-complete
remediates-review: review-01.md
created: 2026-07-31
updated: 2026-07-31
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-017-separate-protocol-policy-and-project-guidance
  base: 2b99900cd1e98804644a13d7e5b1a417655824c9
  archive-commit: self
  status: archived
capability-impact:
  type: update
  ids:
    - CAP-001
    - CAP-003
    - CAP-004
    - CAP-006
  rationale: Clarifies ownership and composition across the durable workflow contract, installed template, agent adapters, and rendered role guidance without adding a separate user capability.
---

# Task Plan

## Goal

Introduce an explicit layered instruction model that separates Concoct-owned protocol invariants, project-selected workflow policy, and repository-owned project guidance while preserving `AGENTS.md` as the practical entry point for humans and agents.

## Context

Concoct currently installs workflow rules, project conventions, personas, prompts, and tool adapters through one template surface. `AGENTS.md` is described as the canonical instruction file, but the shipped version mixes project placeholders and coding conventions with Concoct lifecycle policy. Protocol rules also appear across the Concoct skill, personas, prompts, and documentation without a machine-readable ownership or composition boundary.

The accepted workflow, adapter, and prompt-rendering capabilities establish the durable artifacts and entry points this task must preserve. Their documented limitations are compatible with CON-017: semantic judgment remains human/agent work, adapters remain guidance-only, and deterministic rendering can be extended to expose layered sources without claiming to prove semantic correctness.

## Why this matters

Concoct cannot safely upgrade its own workflow material or later offer configurable policy while repository truth and Concoct invariants are indistinguishable. A layered model must make ownership visible, preserve project-authored content byte-for-byte unless explicitly reconciled, and reject attempts to weaken evidence-integrity rules.

## Current state

- `templates/AGENTS.md` combines project intent, architecture and coding conventions, naming rules, and lifecycle instructions; initialization copies it as a Concoct-owned template and substitutes only the project name elsewhere.
- The source and embedded `.codex/skills/concoct/SKILL.md`, personas, and handoff prompts carry the operative protocol and policy rules, but no artifact declares which rules are invariant versus selectable.
- `CLAUDE.md`, `CONVENTIONS.md`, `.aider.conf.yml`, Copilot instructions, and GitHub prompt files all route through `AGENTS.md`; several GitHub prompts and documentation still name obsolete personas and a single `review.md` contract.
- `internal/project.Discover` uses `AGENTS.md`, roadmap, and capabilities as project markers. `internal/project.Initialize` copies the embedded tree without an ownership manifest or composition validation.
- `internal/prompt.Render` lists exact source files but does not identify an instruction's ownership layer or detect cross-layer conflicts.
- No supported upgrade command exists yet. CON-017 must define upgrade-safe ownership and composition boundaries for CON-013 and CON-030 without implementing those later lifecycle operations.

## Target state

- A documented and represented three-layer model identifies Concoct protocol, project-selected policy, and repository-owned guidance, including precedence and permitted strengthening behavior.
- Evidence integrity, immutable completed reviews, ownership of workflow artifacts, and invalid-state refusal are non-overridable protocol invariants.
- Required lifecycle phases, approval/control gates, and Git strategy are policy inputs. The default policy reproduces the existing accepted workflow behavior.
- Naming, architecture, coding standards, project verification commands, and other repository conventions remain project-owned and survive initialization and supported composition unchanged unless explicitly reconciled.
- `AGENTS.md` remains a concise usable entry point that points to the effective layered instruction set while keeping repository-owned guidance clearly bounded.
- Deterministic composition reports each effective source and rejects conflicting or invariant-weakening instructions with actionable diagnostics before emitting partial guidance.
- Installed templates, adapters, personas, prompts, documentation, and tests consistently use the new boundaries and current persona/review names.

## Design constraints

- Preserve the accepted artifact-backed workflow and role ownership. Do not weaken evidence integrity, review immutability, invalid-state handling, or prompt rendering's separation from completed role work.
- Keep the model agent-neutral and filesystem-based; no service, database, or tool-specific source of truth.
- Keep `AGENTS.md` human-editable and useful as the conventional entry point. Do not require tools to understand a proprietary opaque format before they can find project guidance.
- Project guidance may add stricter requirements but may not disable or contradict protocol invariants. Policy and guidance conflicts must have deterministic precedence or fail explicitly; silent last-writer-wins behavior is forbidden.
- Preserve project-authored bytes across composition and initialization after ownership is established. Any migration or reconciliation of a pre-existing mixed `AGENTS.md` must be explicit rather than inferred destructively.
- Use hyphenated paths and keep source/template counterparts synchronized. Preserve executable mode on `cmd/concoct/concoct.sh`.
- Keep the scope compatible with future configurable policy, adoption, overlays, and upgrades, but do not implement those roadmap outcomes here.

## Non-goals

- Implementing `concoct upgrade`, adoption/apply, overlays, or general migration of existing client repositories.
- Exposing user-selectable policy values beyond representing and validating the existing default workflow policy.
- Making Git lifecycle strategy selectable, adding concurrent tasks, or changing current lifecycle outcomes.
- Replacing Markdown guidance with an opaque compiled format or attempting general natural-language theorem proving.
- Launching agents or treating composed/rendered instructions as completed role work.
- Broadly redesigning roadmap, capability, task, review, archive, or integration schemas except where source attribution or layer identity is essential.

## Working assumptions

- The ownership boundary should be represented by distinct Concoct-owned and project-owned files rather than by editable regions inside one generated file; this is the safest basis for byte-preserving upgrades and deterministic attribution.
- `AGENTS.md` can remain project-owned after initialization and act as the entry point by referencing Concoct-owned protocol/policy material. Concoct should not need to overwrite it during future upgrades.
- The default policy belongs in a durable, inspectable project-local artifact so existing behavior can be reproduced and later configured by CON-018.
- Conflict detection should initially cover structural declarations and explicit protected invariants. Arbitrary semantic contradiction in free-form prose remains outside what validation can prove and must be documented as a limitation.
- CAP-001, CAP-004, and CAP-006 limitations are compatible because this task strengthens inspectability and structural validation while retaining human/agent semantic judgment. CAP-003 is an affected limited capability because the installed template is the ownership boundary being corrected.

## Risks and open questions

- Free-form Markdown cannot support complete semantic conflict detection. Phase 1 must define a narrow enforceable instruction schema or declarations for protected controls and clearly distinguish validated conflicts from source-attributed prose.
- Moving workflow material out of `AGENTS.md` can reduce compatibility with agents that read only that file. The entry point must contain enough direction to load the other layers, and adapter-specific mechanics must remain thin and tested.
- Existing initialized projects have mixed ownership with no provenance marker. This task should define future ownership and an explicit reconciliation boundary without pretending their content can be split automatically.
- Source attribution can destabilize golden prompt output. Changes must be intentional and covered by deterministic fixtures.
- Protocol and default-policy allocation must avoid freezing configurable choices as invariants or accidentally making evidence integrity configurable.
- No unresolved product decision blocks planning: the roadmap fixes the three ownership categories, precedence constraint, preservation rule, diagnostics, and default behavior. The Developer retains local design choice over filenames and data representation within those boundaries.

## Implementation phases

### Phase 1 — Define the layered contract

Status: `complete`

- Inventory every workflow instruction currently carried by `AGENTS.md`, the skill, personas, prompts, adapters, and documentation, and classify it as protocol, policy, project guidance, or task context.
- Define layer ownership, precedence, permitted strengthening, source identity, composition order, and the explicit conflict/invariant declarations that can be validated deterministically.
- Specify the default policy so it reproduces the accepted lifecycle, including required roles, review controls, Git behavior, and completion boundaries.
- Record limits on detecting semantic conflicts in unrestricted prose and the reconciliation boundary for legacy mixed guidance.

### Phase 2 — Restructure installed guidance and adapters

Status: `complete`

- Introduce the minimum source/template artifacts needed to keep protocol and default policy Concoct-owned while leaving project guidance project-owned.
- Refocus `templates/AGENTS.md` as the project-owned entry point and repository convention surface, retaining clear references to effective protocol, policy, personas, and active task context.
- Update the skill, personas, handoff prompts, adapters, and GitHub prompt templates to consume the layered sources and current canonical persona/review conventions without duplicating durable rules.
- Ensure fresh initialization produces a complete standard workflow and clearly marks ownership without requiring project customization for internal consistency.

### Phase 3 — Implement deterministic composition and validation

Status: `complete`

- Add a narrow workflow/instruction model that loads all three layers, preserves deterministic ordering, attaches source attribution, and identifies protected protocol controls.
- Reject missing, malformed, contradictory, or invariant-weakening declarations before returning a partial effective instruction set; diagnostics must name the conflicting layers, sources, and corrective action.
- Integrate the effective instruction context into role-prompt rendering while preserving command eligibility, state preservation, deterministic output, and create-only output semantics.
- Keep repository discovery compatible with the conventional `AGENTS.md` entry point and make ownership data accessible to future initialization/upgrade lifecycle work.

### Phase 4 — Verify preservation and default compatibility

Status: `complete`

- Add focused tests for attribution, deterministic ordering, allowed strengthening, protocol weakening, policy/project conflicts, missing sources, malformed declarations, and no partial output on failure.
- Prove that fresh initialization preserves a deliberately customized project-guidance fixture unchanged through composition and that the default layered model renders the currently accepted workflow behavior.
- Update golden prompts and initialization tests for intentional output/layout changes and confirm every tool adapter reaches the same effective instruction set.
- Exercise a representative legacy mixed-guidance case and document the explicit reconciliation requirement rather than mutating it silently.

### Phase 5 — Reconcile documentation and prepare review

Status: `complete`

- Update README and workflow, command, state-machine, and multi-agent documentation to explain ownership, composition, attribution, conflicts, defaults, and upgrade-safe boundaries.
- Search source and templates for stale persona names, single-review conventions, claims that all instruction ownership lives in `AGENTS.md`, and unlayered adapter guidance.
- Run the full repository checks, inspect the complete diff, update plan/notes, and provide an independent Reviewer handoff focused on invariant strength, project-content preservation, deterministic diagnostics, and default compatibility.

## Acceptance criteria

1. The repository defines protocol, policy, project guidance, and active task context as distinct ownership layers, and every effective instruction or rendered instruction section identifies its layer and source.
2. Evidence integrity, completed-review immutability, workflow-artifact ownership, and invalid-state refusal are represented as Concoct-owned non-overridable protocol controls.
3. Required phases, approval/control gates, and Git lifecycle behavior are represented as project-selected policy; the shipped default policy expresses the existing accepted workflow and yields equivalent state/role behavior.
4. Naming, architecture, coding standards, verification commands, and repository-specific conventions remain project-owned, with `AGENTS.md` retained as a usable conventional entry point.
5. Project guidance can add a stricter compatible constraint, and deterministic composition attributes and retains it without modifying its source bytes.
6. A project-guidance or policy declaration that weakens a protocol invariant is rejected before effective guidance is rendered; diagnostics name the invariant, ownership layers, source paths, and corrective action.
7. Other policy/project conflicts either resolve through the documented deterministic precedence rule or fail explicitly; composition never silently drops a source or uses undocumented last-writer-wins behavior.
8. Fresh initialization installs a complete internally consistent layered default, preserves root files, dotfiles, nested templates, personas, prompts, planning directories, bootstrap prompt, Git initialization, staging, and no generated commit.
9. Source/template skills, personas, prompts, and adapters use current canonical role and sequential-review paths and consistently direct agents through the same layered effective instructions.
10. Prompt rendering remains deterministic and state-preserving, includes source attribution, refuses incompatible layers without partial output, and retains create-only output behavior.
11. A representative project-owned customization remains byte-identical across supported initialization/composition operations; legacy mixed ownership is diagnosed for explicit reconciliation rather than silently rewritten.
12. Existing status detection, planning, development/review completion, archival, integration, and next-action behavior remains passing unless an intentional documentation/attribution change is expressly covered by updated tests.

## Verification

- Run `gofmt` on changed Go files and focused tests for `internal/project`, `internal/prompt`, `internal/workflow`, and any new instruction-composition package.
- Add golden or table-driven tests covering deterministic layer order and source attribution, compatible strengthening, invariant weakening, policy/project conflicts, malformed or missing layers, and all-or-nothing rendering.
- Add initialization tests that inspect ownership artifacts and compare project-owned guidance bytes before and after composition.
- Run `go test -count=1 ./...`, `go vet ./...`, and `go build ./cmd/concoct`.
- Run `bash -n cmd/concoct/concoct.sh` and confirm it remains executable.
- Run `./cmd/concoct/concoct.sh init` against a temporary parent and confirm dotfiles, nested templates, personas, planning directories, bootstrap prompt, staged Git files, no generated commit, and `ready` status.
- Compare all changed source workflow assets with their embedded `templates/` counterparts.
- Run `git diff --check` and search for stale branding, paths, persona names, single-review conventions, unqualified canonical-instruction claims, and duplicated cross-layer rules.

## Capability impact

Expected `update`: CAP-001, CAP-003, CAP-004, and CAP-006. The accepted workflow contract will distinguish invariant protocol from policy and guidance; the reusable template will gain internally consistent ownership boundaries; adapters will point to an attributed effective instruction set; and prompt rendering will compose and identify layered sources. Capability truth changes only after approval and archival.

## Completion evidence

Implementation is committed at `0071266`. Full Go tests, vet, build, shell
syntax, executable-mode, whitespace, stale-path, and generated-project checks
pass. The generated layered fixture reports `ready`, stages all installed
content, and contains no generated commit.

Review 01 remediation closes the remaining structural conflict gap: each layer
now has an explicit declaration-key allowlist, policy-owned declarations in
project guidance fail with both source paths identified, and composition and
prompt rendering return no partial output. Focused and full tests, vet, build,
shell syntax, executable-mode, whitespace, stale-path, and fresh initialization
checks pass after remediation.

## Handoff expectations

The Developer should begin by producing a classification inventory and an enforceable composition contract before moving files. Keep the data model narrow, preserve `AGENTS.md` compatibility, and test byte preservation and conflict failures early. Before review, update phase statuses and durable decisions, run all verification, inspect the full diff from the recorded base, and add a reviewer handoff covering files changed, checks run, known limitations, unresolved work, capability impact, and suggested focus on ownership, invariant protection, default-policy equivalence, project-content preservation, and source/template parity.
