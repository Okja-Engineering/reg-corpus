# 0010. One writer owns L0 `meta.yaml`; `regcorpus meta set` sets step 3's fields

- **Status:** Accepted
- **Date:** 2026-11-22
- **Decided by:** the owner, 2026-11-22 15:48 CST, in chat (answer to "Who should own meta.yaml?"), recorded on issue #5: "One shared writer (Recommended)"

## Context
The spec's "L0 `meta.yaml`" lists thirteen fields, and the plan fills them in
different steps: the fetch-time facts in step 2, `version_effective_date` and
`version_fr_citations` in step 3, `sha256_text` in step 5 and `verified_on`
in step 6.

The plan gives the file to one step. Plan step 5 reads:

> 5. `internal/anchors`:
>    - Generate `text.md` from the tree.
>    - Normalize the text, and write a per-node `sha256` of normalized own text into `tree.json`.
>    - Write `meta.yaml`.

PR #2 (plan step 2) made `regcorpus fetch` write its own fields when
`meta.yaml` is missing and never change an existing one, and checked only two
of them (`source_version_date`, `sha256_raw`) on a re-run. Step 3 then had no
way to add its two fields. The plan's command list has no command that sets
them:

> `cmd/regcorpus/main.go             # fetch, check, inventory, verify, upgrade, release, show, text, topics, version`

## Decision
One package, `internal/meta`, owns the whole `meta.yaml` schema: the thirteen
fields, their names, order, types and how each value is written (dates and
hashes double-quoted, as PR #2 wrote them). It is the file's one writer: every
command reads `meta.yaml` only through its strict parser and writes only what
it renders, and sets only its own fields, keeping the rest. A field a later
step owns stays absent until that step sets it.

- `fetch` creates `meta.yaml` through it and, when one exists, checks every
  field it writes against its source table and the raw file, except the two
  facts of the run that wrote it (`retrieved_at`, `ecfr_up_to_date_as_of`).
- A new command, `regcorpus meta set <section> <field> <value ...>`, sets
  `version_effective_date` or `version_fr_citations` and refuses every other
  field.

This departs from plan step 5's "`internal/anchors` … Write `meta.yaml`":
`internal/anchors` will set `sha256_text` through `internal/meta`. It also
adds `meta` to the plan's command list.

## Consequences
- `fetch` re-runs now also refuse (exit 1) a `meta.yaml` whose `cfr`, `tier`,
  `role`, `source` or `official` differs from the source table, or that is not
  in the spec's schema (an unknown key, keys out of order, a key twice, a value
  not of its type). This closes PR #2's deferred finding that `fetch` checked
  only two fields.
- `meta.yaml` is no longer write-once: `meta set` replaces it whole (a
  temporary file renamed over it). Raw XML is still stored once and never
  overwritten.
- Plan steps 5 and 6 set `sha256_text` and `verified_on` through
  `internal/meta`; its parser already reads both fields.
- The writer uses the standard library only. `gopkg.in/yaml.v3` is still
  blocked by the missing `go.sum` in the gate test harnesses (PR #2, Findings).
- `regcorpus help` lists `meta`, and `regcorpus meta -h` gives its exit codes.

## Requirement IDs
R1 (the metadata each source records); spec "L0 `meta.yaml`"; R10 (`help`
and each command's `-h` name the exit codes); decision 6 (CLI only).

## Links
- Issue #5 (the decision).
- The plan step 3 PR, which records this ADR; its commits add
  `internal/meta/`, move `fetch` onto it and add `internal/ecfr/metaset.go`.
- PR #2 (https://github.com/Okja-Engineering/reg-corpus/pull/2): Findings,
  "`meta.yaml` has two writers" and "fetch checks only the two meta.yaml
  fields it can compare".
