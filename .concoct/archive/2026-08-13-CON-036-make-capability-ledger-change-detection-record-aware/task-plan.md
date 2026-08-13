---
id: CON-036
title: Make capability-ledger change detection record-aware
roadmap-id: CON-036
status: implementation-complete
remediates-review: review-01.md
created: 2026-08-13
updated: 2026-08-13
git:
  enabled: true
  trunk: main
  task-branch: concoct/con-036-make-capability-ledger-change-detection-record-a
  base: 160f5b1ce86629a779247dc095e4c88ea1dfe900
  status: active
capability-impact:
  type: update
  ids:
    - CAP-011
  rationale: Makes validated archive capability reconciliation compare semantic capability records without attributing record-separator whitespace to adjacent records, while preserving strict undeclared-change detection.
---

# Task Plan

## Goal

Make Git-backed archive completion detect capability-ledger changes from parsed
capability records and ledger-level structure rather than raw heading-to-heading
slices, so declared additions, insertions, removals, and boundary-formatting
changes do not falsely implicate adjacent capabilities.

Preserve the safety purpose of the current validation: meaningful edits to an
undeclared capability or to protected non-record ledger content must still fail
with an actionable diagnostic.

## Context

`archive --complete` compares the authored `.concoct/capabilities.md` with the
recorded Git baseline before accepting archival evidence. The current
implementation in `internal/workflow/archive.go` uses `capabilitySections` and
`capabilityUndeclaredBytes`, both of which slice each record from its `## CAP-NNN
—` heading to the next heading. Blank separator lines therefore become trailing
content of the preceding record.

When an Archivist appends a declared capability, the newly introduced separator
can change the preceding record's raw slice even when that record is semantically
unchanged. `validateCapabilitySectionChanges` then reports that existing record
as an undeclared change. The CAP-012/CAP-013 archive scenario in the roadmap is
the concrete regression to reproduce.

## Why this matters

Capability reconciliation is the accepted archive boundary for product truth.
False undeclared-record failures block ordinary archival and require manual
intervention, while weakening the check broadly would allow unrelated capability
edits to enter accepted truth. Record-aware comparison must remove the false
positive without reducing the protection.

## Current state

- `internal/workflow/workflow.go` recognizes capability headings with
  `capabilityHeading` and parses status, limitations, and archive provenance for
  workflow validation, but it does not expose an ordered lossless record model
  for change attribution.
- `internal/workflow/archive.go` obtains the baseline with
  `gitrepo.Repository.FileAt`, compares maps returned by `capabilitySections`,
  and separately projects undeclared raw bytes with
  `capabilityUndeclaredBytes`.
- Record slices include blank lines immediately before the next capability
  heading. The undeclared projection uses the same incidental boundary.
- `validateCapabilitySectionChanges` correctly enforces declared `add`,
  `update`, and `remove` shapes once given accurate before/after record content.
- `validateCapabilityResult` separately verifies that resulting declared
  records are active and cite archive provenance.
- Existing focused tests cover all impact types, selected-record projection,
  protected preamble/content, and ordering, but not append/insert/remove
  separator changes, LF/CRLF equivalence, or final-newline equivalence.
- CAP-005 is active accepted truth. Its schema-focused parsing and read-only
  status limitations are compatible: this task refines the existing checked-in
  capability schema and archive validator rather than adding migration,
  arbitrary Markdown support, or semantic prose judgment.

## Target state

- One record-aware capability-ledger parser identifies ordered `CAP-NNN`
  records, their semantic content, and protected ledger-level content without
  assigning blank inter-record separators to either adjacent record.
- Archive comparison uses that parsed representation for both the changed-record
  set and preservation of undeclared/non-record content; it is independent of
  Git diff hunks and context selection.
- Line endings are canonicalized for attribution, and a final newline difference
  is non-semantic. Blank-line changes at record boundaries are ledger formatting.
- Content inside a record remains significant after only the explicitly allowed
  line-ending/boundary normalization. Heading text, status, metadata, prose,
  internal whitespace, and record order remain protected.
- Declared append, insertion, and removal operations affect only the intended
  record set. Unauthorized record changes and incompatible impact shapes still
  fail and identify the responsible capability when possible.
- Duplicate IDs, malformed capability headings or structure, ambiguous
  boundaries, and meaningful protected content outside records fail safely.

## Design constraints

- Keep `internal/workflow` as the single capability-schema and archive-validation
  authority. Do not add a second permissive parser solely for tests or CLI code.
- Represent record boundaries explicitly from valid capability headings. Trim or
  canonicalize only ledger-level separators, line endings, and final-newline
  state authorized by the roadmap; do not apply general whitespace folding to
  record bodies.
- Preserve deterministic record ordering and stable identifiers. Maps may support
  identity lookup, but ordered ledger structure must remain available for
  insertion, removal, duplicate, and protected-content validation.
- Reconcile the parsed-record comparison with `parseCapabilities` so the archive
  boundary cannot accept a structure that normal workflow validation considers
  malformed, or silently ignore unrecognized capability-like content.
- Retain existing `add`, `update`, `remove`, and `none` semantics and archive
  provenance validation. This task changes attribution, not the authorized
  capability-impact contract.
- Return specific safe failures for malformed or unauthorized evidence; do not
  mutate or normalize the authored ledger as part of validation.
- Keep the change scoped to capability-ledger comparison and its normative
  documentation. Preserve archive transaction, roadmap reconciliation, Git
  commit/retry, and non-Git lifecycle behavior.

## Non-goals

- No broad Markdown parser or capability-ledger schema redesign.
- No semantic assessment or rewriting of capability prose.
- No automatic formatting of `.concoct/capabilities.md`.
- No changes to capability-impact categories, archive provenance rules, roadmap
  reconciliation, review approval, integration, or task-branch strategy.
- No general tolerance for whitespace edits inside capability records or changes
  to front matter and introductory ledger content.
- No project upgrade, migration, legacy-ledger repair, or arbitrary Markdown
  compatibility work.

## Working assumptions

- CAP-011 is the accepted capability whose behavior changes. The outcome fixes an
  observable false rejection in validated archive/capability reconciliation; it
  does not create a new user capability.
- The existing capability heading form, `## CAP-NNN — ...`, remains the canonical
  record start. Malformed heading-like content must be diagnosed rather than
  absorbed into a neighboring valid record.
- A shared internal parsed representation can serve workflow metadata extraction
  and archival attribution without changing the public capability ledger format.
- CRLF/LF normalization applies to comparison only. Other whitespace within a
  record remains byte-significant after line-ending canonicalization.
- Git-backed archive validation is the primary affected path because it has a
  committed baseline. Non-Git structural result validation should continue to
  use the same accepted parser but has no before/after semantic diff to add.

## Risks and open questions

- A parser that merely trims every record can hide meaningful trailing or
  internal whitespace edits. The implementation must distinguish blank
  inter-record separators from record-owned content deliberately.
- A parser that recognizes only valid headings can accidentally classify a
  malformed capability-like heading as harmless prose in a prior record. Add
  negative fixtures that prove malformed and ambiguous boundaries fail safely.
- Separate code paths for workflow parsing, changed-record comparison, and
  undeclared-content preservation could drift. Prefer one boundary model and
  test the composed archive behavior, not only helpers.
- Normalizing all line endings must not also normalize spaces, tabs, Markdown
  structure, or record order.
- Map-only comparison cannot distinguish ordering changes or protected
  ledger-level edits. Preserve and compare the ordered structural projection in
  addition to per-ID content.
- No product question remains open. The roadmap explicitly decides separator,
  line-ending, final-newline, internal-whitespace, and malformed-ledger behavior.

## Implementation phases

### Phase 1 — Define and test the record boundary model

Status: `complete`

- Trace every use of `capabilityHeading`, `parseCapabilities`,
  `capabilitySections`, and `capabilityUndeclaredBytes` and define one ordered
  parsed representation for valid records, protected ledger content, and
  inter-record formatting.
- Add focused failing fixtures for the reported CAP-012/CAP-013 append, declared
  insertion and removal, separator-only changes, LF/CRLF normalization, final
  newline changes, meaningful body edits, ordering, malformed headings,
  duplicates, and content outside the recognized structure.
- Confirm diagnostics and impact-shape expectations before replacing the raw
  slicing logic.

### Phase 2 — Implement record-aware comparison

Status: `complete`

- Parse baseline and candidate ledgers with explicit semantic record boundaries
  and reject parser and required-record metadata diagnostics from either side
  before comparing changes.
- Compare normalized record content by stable ID, preserve ordered/protected
  ledger structure, and exclude only allowed boundary formatting from record
  attribution.
- Replace or refactor the raw section/projection helpers so
  `validateGitCapabilityDiff` uses the same model for changed declared records,
  undeclared records, and non-record content.
- Preserve `validateCapabilitySectionChanges`, capability result/provenance
  checks, and all four impact outcomes unless a narrow signature change is
  required to consume the richer representation.

### Phase 3 — Verify archive behavior and compatibility

Status: `complete`

- Exercise focused helper and workflow tests plus real Git-backed archive
  completion for declared append, insertion, removal, and unauthorized edits.
- Confirm existing archive retry, allowed-path, roadmap, provenance, impact, and
  non-Git tests remain passing.
- Update normative command/state documentation only where needed to state the
  record-boundary and normalization rules; avoid a broader documentation rewrite.

### Phase 4 — Complete verification and prepare review

Status: `complete`

- Run formatting and the full Go test, vet, and build checks.
- Run the repository-required shell syntax, initialization, generated-project,
  executable-mode, diff, and stale-path checks if templates or initialization
  surfaces change; otherwise record why the template-specific checks are not
  applicable after confirming no such files changed.
- Inspect the complete diff, update phase status and durable notes, and provide a
  fresh Reviewer handoff emphasizing false-positive removal, malformed-ledger
  refusal, and preservation of undeclared-change protection.

## Acceptance criteria

1. Appending a newly declared capability after an unchanged existing record
   succeeds when the only adjacent difference is record-separator whitespace;
   the changed-record set contains only the new ID.
2. Adding, removing, or normalizing blank lines between two unchanged capability
   records reports neither record as changed.
3. Inserting a declared capability between existing records does not report
   either adjacent existing record as changed, and removing a declared record
   under `remove` does not implicate its neighbors.
4. Equivalent LF and CRLF ledgers compare without an undeclared-record failure,
   and adding or removing the final newline is non-semantic.
5. A meaningful heading, status, metadata, prose, internal-whitespace, or other
   body edit to an undeclared existing capability fails and identifies that ID.
6. A meaningful undeclared addition or removal fails with an actionable
   capability attribution; declared `add`, `update`, and `remove` operations
   still require exactly their respective new, changed-existing, and deleted
   record shapes, and `none` still forbids meaningful ledger changes.
7. Protected front matter, introductory content, record ordering, and meaningful
   content outside declared records cannot change under cover of the new
   normalization rules.
8. Duplicate identifiers, malformed capability-like headings, missing required
   record metadata, ambiguous boundaries, and otherwise malformed baseline or
   candidate ledgers fail safely before archive mutation.
9. Regression coverage reproduces the CAP-012/CAP-013 append failure and covers
   append, insertion, removal, separator, line-ending, final-newline, malformed,
   and unauthorized-change cases without weakening existing archive tests.
10. Existing archive completion, retry, capability provenance, roadmap,
    integration, status, and non-Git behavior remains passing.

## Verification

- Run `gofmt` on changed Go files.
- Run focused tests for `internal/workflow`, including direct parser/comparison
  fixtures and full Git-backed `CompleteArchive` regression scenarios.
- Run relevant `internal/cli` transition tests if archive completion behavior or
  diagnostics cross the CLI boundary.
- Run `go test -count=1 ./...`.
- Run `go vet ./...`.
- Run `go build ./cmd/concoct`.
- Run `bash -n cmd/concoct/concoct.sh` and confirm the wrapper remains executable.
- If templates or initialization behavior change, run
  `./cmd/concoct/concoct.sh init <temporary-project-path>` outside the repository
  and verify the project-owned files, dotfiles, planning directories, Git
  repository, bootstrap prompt, and exclusion of built-in resources required by
  `AGENTS.md`.
- Run `git diff --check` and search changed documentation/tests for stale
  capability identifiers or raw-diff attribution claims.

## Handoff expectations

The Developer should begin with failing tests that demonstrate the current raw
slice defect, then introduce one explicit ledger boundary model shared by
workflow validation and archive comparison. Keep normalization as narrow as the
roadmap permits and retain negative evidence for undeclared content, malformed
structure, duplicates, and record ordering. Before review, update this plan and
notes with the exact representation chosen, checks run, any non-applicable
repository checks, capability impact, and a Reviewer handoff focused on both the
reported success cases and unchanged safety failures.
