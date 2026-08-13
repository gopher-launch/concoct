# Notes

## Planning summary

CON-036 is ready for implementation. It is a coherent correction to Git-backed
capability reconciliation, has no unresolved roadmap dependency or product
decision, and has observable acceptance criteria. The active CAP-005
prerequisite is compatible with the outcome: its checked-in-schema parsing and
read-only status limitations do not prevent a stricter record-aware archive
comparison.

The task should update CAP-011 after approval because it changes the observable
behavior of validated archive and capability reconciliation. It should not add a
new capability or change capability-impact semantics.

## Confirmed findings

- The checked-out Git identity before planning exactly matched the executable
  context: trunk `main`, task branch
  `concoct/con-036-make-capability-ledger-change-detection-record-a`, and
  base/HEAD `160f5b1ce86629a779247dc095e4c88ea1dfe900`.
- The worktree was clean and `.concoct/current/` contained only `.gitkeep`; no
  active task or review conflicted with planning.
- CON-036 was `planned`, has no unresolved `Depends on` item, and declares
  CAP-005 as its sole accepted prerequisite.
- CAP-005 is active. Its limitations reserve semantic correctness for role
  judgment, keep `status` read-only, and target checked-in schemas. CON-036 stays
  within those boundaries by refining structural comparison at the existing
  archive completion boundary.
- `capabilityHeading` in `internal/workflow/workflow.go` recognizes canonical
  headings with `^## (CAP-[0-9]+)\s+—`.
- `parseCapabilities` slices each record from its heading through the next
  heading and extracts status, limitations, and archive provenance. It reports
  duplicates and missing status, but does not expose ordered record boundaries
  or ledger-level separators to archive comparison.
- `validateGitCapabilityDiff` in `internal/workflow/archive.go` reads the
  committed baseline and current ledger, compares `capabilitySections`, and then
  compares `capabilityUndeclaredBytes` projections before validating the
  resulting records and provenance.
- Both current archive helpers use the next heading offset as the prior record's
  end. Blank lines before a newly appended or inserted heading therefore change
  the previous record's raw text and can trigger `capability ledger changed
  undeclared record <ID>`.
- `validateCapabilitySectionChanges` already enforces the intended identity
  shapes for `add`, `update`, `remove`, and `none` when supplied accurate record
  content. The defect is primarily the record/protected-content representation,
  not those impact rules.
- `internal/workflow/archive_test.go` covers all impact types, preservation of
  front matter/introduction/undeclared records/order, archive provenance, and
  failure preservation. It does not cover the reported append separator,
  insertion/removal adjacency, CRLF/LF, final-newline, or malformed-boundary
  matrix required by CON-036.
- CAP-011 describes the affected accepted behavior and cites
  `internal/workflow/archive.go` and its tests as evidence. Its limitation on
  semantic prose judgment remains unchanged.

## Planning decisions

- Declare `update CAP-011`. This is a correctness refinement to an existing
  observable archive safeguard, not a new independently useful capability.
- Use one ordered parsed ledger representation for record identity/content and
  protected ledger-level structure. Do not retain independent raw-slicing
  definitions whose boundaries can drift.
- Canonicalize CRLF/LF and final-newline state only for comparison. Treat blank
  inter-record separators as ledger formatting, while keeping other record-body
  whitespace meaningful.
- Preserve ordered structural comparison in addition to ID lookup so record
  reordering and changes outside authorized records remain detectable.
- Require both baseline and candidate ledgers to pass structural validation
  before attribution. Invalid content must not be made acceptable by excluding a
  declared record from an undeclared-byte projection.
- Keep the implementation in `internal/workflow`; no CLI or Git diff-hunk parser
  should gain a parallel capability model.

## Risks

- Broad trimming could conceal a meaningful record-body edit.
- Valid-heading-only scanning could absorb malformed capability-like headings
  into adjacent prose unless explicitly diagnosed.
- Per-ID maps alone lose ordering and ledger-level content evidence.
- Global newline normalization must not expand into general whitespace
  normalization.
- Helper-level tests alone may miss the composed `CompleteArchive` behavior that
  originally failed; at least one real Git-baseline regression is required.

## Relevant history

- CON-009 introduced CAP-011 and the current Git capability-diff validation. Its
  accepted design requires declared impact agreement, preservation of unrelated
  records, and safe failure; CON-036 refines that boundary without changing the
  archive transaction.
- CON-010 added CAP-013 after CAP-012, matching the concrete append pattern in
  the roadmap's failure report.
- CON-005 established the CLI/workflow parsing foundation, while CON-018 and
  CON-030 preserved policy-aware and executable-owned workflow boundaries.
- CON-031 established the project compatibility preflight. This task does not
  change project contracts or compatibility ranges.

## Initial verification

- Read the complete Concoct skill, executable-rendered Task Planner prompt,
  `.concoct/policy.md`, `AGENTS.md`, capabilities, roadmap item, and every exact
  archive summary required by the planning command.
- Inspected the CAP-005 prerequisite and CAP-011 affected capability, relevant
  CON-009 archive history, workflow capability parser, archive diff validator,
  focused archive tests, Git transition tests, and normative command/state
  documentation.
- Confirmed the exact branch/base, clean worktree, and absence of conflicting
  active artifacts before authoring this candidate.
- `go test ./internal/workflow` passed as a focused baseline check;
  `go run ./cmd/concoct status` validated the authored transition as `planned`
  with `concoct code` next, and `git diff --check` passed.
- No source code, tests, capabilities, archives, or review artifacts were
  changed during planning.

## Developer handoff

### Current state

Planning is complete; CON-036 is ready to enter development on its recorded task
branch after the outer completion boundary accepts this candidate.

### Completed

- Validated roadmap readiness, CAP-005 limitation compatibility, repository
  reality, Git identity, scope, acceptance criteria, verification, and expected
  CAP-011 impact.
- Located the raw heading-to-heading boundary defect and the existing unit and
  archive test seams.

### Remaining

- Add the regression matrix, implement the shared record-aware representation,
  preserve strict malformed/undeclared-content failures, and update only
  necessary normative documentation.
- Record exact implementation decisions and test results, update phase statuses,
  and prepare a fresh independent Reviewer handoff.

### Known risks

Over-normalization, malformed-heading absorption, loss of ordering evidence, and
insufficient end-to-end Git archive coverage.

### Suggested next step

`concoct code`

## Development findings

- Replaced the two raw heading-to-heading slices with one ordered
  `capabilityLedger` representation. It owns normalized record content, ordered
  IDs, and the protected preamble used by both ordinary capability validation
  and Git archive reconciliation.
- CRLF is canonicalized to LF and one final newline is ignored. Blank or
  whitespace-only separator lines are excluded from all record bodies, including
  when a removal makes the preceding record terminal. Other body whitespace,
  metadata, prose, and record ordering remain significant.
- Capability-like level-two headings that are not canonical `CAP-NNN —` record
  headings are diagnosed. Duplicate records are diagnosed before archive diff
  attribution; the existing missing-Status checks continue through the same
  parsed ledger.
- Added direct append, insertion, removal, separator, LF/CRLF, final-newline,
  malformed, duplicate, body-whitespace, and ordering coverage. Real Git
  regressions exercise both the capability-diff boundary and a complete
  `CompleteArchive` transaction for the reported declared-append case.
- Recovery inspection found that the rejected candidate still recorded top-level
  task status `planned` despite completing every phase. The status now records
  `implementation-complete`, matching the implemented result and the required
  Developer transition.

## Verification

- `gofmt -w internal/workflow/workflow.go internal/workflow/archive.go internal/workflow/archive_test.go` — passed.
- `GOCACHE=/tmp/concoct-go-build-cache go test -count=1 ./internal/workflow` — passed, including the full Git-backed `CompleteArchive` append regression.
- `GOCACHE=/tmp/concoct-go-build-cache go test -count=1 ./internal/cli -run 'GitArchive|ArchiveCompletion'` — passed.
- `GOCACHE=/tmp/concoct-go-build-cache go test -count=1 ./...` — all packages except `internal/runloop` passed. Its two lifecycle tests pin baseline roadmap evidence to `2026-08-12`, while the fake Archivist writes the current date (`2026-08-13`), so the existing roadmap validator rejects the changed `updated` field. The previously observed alternative of retaining the pinned date conflicts with the current deterministic archive date. This is pre-existing time-sensitive fixture debt, not a capability-ledger result.
- The same two `internal/runloop` tests were run from a temporary, untouched `git archive HEAD` of planning commit `aed0ed6` and failed identically: the fixtures pin roadmap evidence to `2026-08-12`, while their fake Archivist writes `updated: 2026-08-13`; the existing roadmap validator rejects that unrelated front-matter change before integration. This directly verifies the failure predates the CON-036 implementation.
- `GOCACHE=/tmp/concoct-go-build-cache go vet ./...` — passed.
- `GOCACHE=/tmp/concoct-go-build-cache go build ./cmd/concoct` — passed; Go emitted a non-fatal warning when it attempted to update its read-only shared module stat cache.
- `bash -n cmd/concoct/concoct.sh`, executable-mode check, and `git diff --check` — passed.
- Template/init checks were not applicable: no template or initialization files changed. Searches of changed documentation/tests found no stale CAP-012/CAP-013 or raw-diff-attribution claims.

## Recovery transition blocker

- Attempted the required exact Developer transition with commit subject
  `concoct: complete CON-036 implementation` after final diff inspection.
- Git failed before staging with `fatal: Unable to create '.git/index.lock':
  Read-only file system`. This execution environment exposes `.git` as read-only,
  so it cannot update the index, create the commit, advance the task-branch ref,
  or establish a clean committed retry boundary.
- The complete candidate remains preserved as six unstaged authorized file
  changes on the recorded task branch. `HEAD` remains planning commit `aed0ed6`;
  no partial or duplicate implementation commit was created.

## Review remediation

### Review 01, Finding 1 — fixed

- Confirmed that `validateGitCapabilityDiff` only applied
  `parseCapabilityLedger` diagnostics to the committed baseline, which omitted
  the required `Status` check used by ordinary capability parsing.
- Extracted `parseCapabilityLedgerRecords` as the shared record-schema helper.
  `parseCapabilities` and Git archive comparison both use it, and the archive
  path now validates baseline and candidate record metadata before calculating
  declared or undeclared changes.
- Added a Git-backed regression with a baseline `CAP-001` missing `Status` and
  a valid candidate. It refuses the operation with the actionable
  `invalid baseline capabilities: ... CAP-001 missing Status` diagnostic,
  proving that candidate changes cannot mask malformed committed evidence.
- `gofmt` and `GOCACHE=/tmp/concoct-go-build-cache go test -count=1
  ./internal/workflow` passed after the remediation.
- `GOCACHE=/tmp/concoct-go-build-cache go test -count=1 ./internal/cli -run
  'GitArchive|ArchiveCompletion'`, `go vet ./...`, build, shell syntax,
  executable-mode, and diff checks passed. The full suite still has only the
  two documented, pre-existing date-sensitive `internal/runloop` failures.

## Handoff to reviewer

### Implemented

Closed Review 01 Finding 1: Git-backed capability reconciliation now applies
the shared required-record metadata validation to both baseline and candidate
ledgers before diff attribution.

### Key decisions

The new `parseCapabilityLedgerRecords` helper retains the existing `Status`
schema and is used by ordinary capability parsing as well as archive comparison.
This avoids introducing an archive-only metadata validator or changing the
narrow record-boundary normalization.

### Files changed

- `internal/workflow/workflow.go`
- `internal/workflow/archive.go`
- `internal/workflow/archive_test.go`
- `doc/command-reference.md`
- `.concoct/current/task-plan.md`
- `.concoct/current/notes.md`

### Verification

`gofmt`, focused workflow and CLI archive tests, vet, build, shell syntax,
executable-mode, and diff checks pass after remediation. The full suite still
has only the pre-existing, date-sensitive `internal/runloop` fixtures,
reproduced from the untouched planning commit.

### Known risks

The required record metadata currently consists of the schema checks already
performed by ordinary capability parsing (notably `Status`). Future required
metadata belongs in that shared helper so baseline and candidate behavior stays
aligned.

### Skipped or unresolved work

No task-scope remediation is skipped. The date-sensitive `internal/runloop`
fixture debt remains unrelated. The outer Concoct executable owns the final
candidate transition and commit.

### Capability impact

CAP-011 remains the sole planned `update` after approval; no capability ledger
was modified during development.

### Suggested review focus

Verify that invalid baseline required metadata fails before attribution, while
the existing append, insertion, removal, and normalization regressions retain
their intended behavior.

## Previous reviewer handoff (superseded by review remediation)

### Implemented

Record-aware capability-ledger comparison for Git-backed archive validation, including narrow normalization of record separators, CRLF/LF, and a final newline.

### Key decisions

The shared parser leaves body content strict while treating only blank record-boundary lines as ledger formatting. It detects malformed capability-like headings and duplicate IDs before comparison.

### Files changed

- `internal/workflow/workflow.go`
- `internal/workflow/archive.go`
- `internal/workflow/archive_test.go`
- `doc/command-reference.md`
- `.concoct/current/task-plan.md`
- `.concoct/current/notes.md`

### Verification

Focused workflow tests, focused CLI archive tests, a full Git-backed archive append regression, vet, build, shell syntax, executable-mode, and diff checks passed. The full suite has only the two documented unrelated date-sensitive `internal/runloop` fixture failures, reproduced identically from untouched planning commit `aed0ed6`.

### Known risks

The parser deliberately treats terminal blank lines as formatting too, so a removal cannot reattribute a former separator to its preceding record. Non-blank content and whitespace on non-blank body lines remain protected. Review must not start until the recorded Git transition blocker is resolved and the exact Developer commit and clean-worktree postconditions are established.

### Skipped or unresolved work

No implementation work is skipped. The Developer transition commit and clean-worktree postcondition remain blocked by the read-only `.git` mount. The unrelated runloop date fixture also requires owner follow-up before a fully green suite on dates other than its pinned fixture day.

### Capability impact

CAP-011 remains the sole planned `update` after approval; its impact is unchanged from the accepted task plan and no capability ledger was modified during development.

### Suggested review focus

Verify the parser’s boundary semantics against append, insertion, and removal; confirm malformed-heading diagnostics are appropriately strict; and assess the documented runloop test failure as unrelated date-fixture debt.
