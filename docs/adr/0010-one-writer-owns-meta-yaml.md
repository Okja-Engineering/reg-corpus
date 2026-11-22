# 0010. One writer owns L0 `meta.yaml`

- **Status:** Accepted
- **Date:** 2026-11-22
- **Decided by:** the owner, 2026-11-22 15:48 CST, in chat (answer to "Who should own meta.yaml?"), recorded on issue #5: "One shared writer (Recommended)"

## Context
The spec's "L0 `meta.yaml`" lists thirteen fields, and the plan fills them in
different steps: the fetch-time facts in step 2, `version_effective_date` and
`version_fr_citations` in step 3, `sha256_text` in step 5 and `verified_on`
in step 6.

The plan gave the file to one step. Plan step 5 read:

> 5. `internal/anchors`:
>    - Generate `text.md` from the tree.
>    - Normalize the text, and write a per-node `sha256` of normalized own text into `tree.json`.
>    - Write `meta.yaml`.

PR #2 (plan step 2) made `regcorpus fetch` write its own fields when
`meta.yaml` is missing and never change an existing one, and checked only two
of them (`source_version_date`, `sha256_raw`) on a re-run. So step 3 had no
way to add its two fields, and the plan's command list had no command that
sets them:

> `cmd/regcorpus/main.go             # fetch, check, inventory, verify, upgrade, release, show, text, topics, version`

## Options considered
As on issue #5:
1. One shared writer: one package owns the whole `meta.yaml` schema; it
   reads, checks and rewrites the file in spec order, keeping fields it
   didn't set, and each step sets only its own fields through it. Stdlib
   only. Chosen: the owner's call above.
2. Follow the plan: `internal/anchors` writes the file at step 5, and earlier
   values are carried until then. Step 3's values would have no home until
   step 5.
3. Hand edits for step 3, with `fetch` taught to accept and keep them. No
   check on what a person types.

## Decision
The owner decided option 1: one shared writer owns L0 `meta.yaml`, and each
plan step sets only its own fields through it, keeping the rest.

How it is carried out was the chief of staff's (an agent's) choice, not the
owner's call. Each choice and its reason:
- **Two packages.** `internal/meta` owns the schema: the thirteen fields,
  their names, order, types and how each value is written, with a strict
  parser and a renderer that are inverses. It does no file I/O, because the
  checker binary links it. `internal/l0` owns the file: it stores raw XML and
  a new `meta.yaml` once, checks the stored state, and replaces an existing
  `meta.yaml` (`Update`). `internal/l0` imports only `internal/meta`, so plan
  steps 5 and 6 use it without `internal/ecfr`.
- **One stored-state check before any write.** Every writer of a
  `meta.yaml` (`fetch` on a re-run, `meta set`, and later steps 5 and 6)
  first checks the same things. The file is in the spec's schema. Its `cfr`,
  `tier`, `role`, `source` and `official` are the source table's. And `raw/`
  holds exactly `raw/<source_version_date>.xml`, a regular file reached
  through real directories, with `sha256_raw`'s bytes. Why: without this, a
  command that sets one field would rewrite a `meta.yaml` that describes
  another source or another raw file (PR #7 review).
- **A lock from read to replace.** `Update` holds `flock(2)` on the source
  directory from before it reads `meta.yaml` until the new file has been
  renamed over it. Why: two concurrent updates of one source each kept the
  copy they had read, and one field was lost (PR #7 review; reproduced). The
  kernel releases the lock when the process exits, so no lock file is left
  behind. It is built for darwin and linux only, like `verify`'s terminal
  check; elsewhere `Update` refuses (exit 2).
- **`regcorpus meta set <section> <field> <value ...>`** sets step 3's two
  fields and refuses every other field. Why: it is the smallest command that
  sets them through the writer. Every other field has a command that
  computes it.

## Spec changes
- `docs/03-spec.md`: decisions-table row 10 is added: "Who writes an L0
  source's `meta.yaml`? One shared writer: one package owns the whole
  schema, and each plan step sets only its own fields through it, keeping
  the rest." Header "Amended: 2026-11-22 by ADRs 0007–0009" → "… by ADRs
  0007–0011".
- `docs/04-plan.md`:
  - Header "Amended: 2026-11-22 by ADR 0007" → "… by ADRs 0007, 0010 and
    0011".
  - Files: `cmd/regcorpus/main.go` "fetch, check, inventory, …" → "fetch,
    meta, check, inventory, …"; `internal/ecfr/` "versioner client;
    full/{date} XML" → "…; fetch and meta set"; added `internal/meta/`
    ("meta.yaml schema: strict parse and render") and `internal/l0/`
    ("stored L0 files: store once, the stored-state check, meta.yaml
    updates").
  - Step 5: "Write `meta.yaml`." → "Set `sha256_text` in `meta.yaml` through
    `internal/l0`, keeping the other fields (ADR 0010)."
  - Step 3 names `regcorpus meta set` (recorded with ADR 0011's change to
    that line).
- No other lines. The stage contracts and AGENTS.md say nothing about who
  writes `meta.yaml`.

## Consequences
- `fetch` re-runs refuse (exit 1) a `meta.yaml` that names another source,
  or that is not in the spec's schema (an unknown key, keys out of order, a
  key twice, a value not of its type, a repeated citation, an effective
  date after `source_version_date`). This closes PR #2's deferred finding
  that `fetch` checked only two fields.
- `meta set` refuses (exit 1) under the same check. It also refuses a
  `version_effective_date` after `source_version_date`.
- `meta.yaml` is no longer write-once: `Update` replaces it whole (a
  temporary file renamed over it), under the source's lock. Raw XML is
  still stored once and never overwritten.
- Plan steps 5 and 6 set `sha256_text` and `verified_on` through
  `l0.Update`, which already checks the stored state and holds the lock.
  The source table (`cfr`, `tier`, `role` per source) stays in
  `internal/ecfr`'s `sources.go`, and a caller passes each source to
  `internal/l0`. Step 5 reads the table from there.
- The writer uses the standard library only. `gopkg.in/yaml.v3` is still
  blocked by the missing `go.sum` in the gate test harnesses (PR #2,
  Findings).
- `regcorpus help` lists `meta`, and `regcorpus meta -h` gives its exit
  codes.

## Requirement IDs
R1 (the metadata each source records); spec "L0 `meta.yaml`"; decision 6
(CLI only); decision 10.

## Links
- Issue #5 (the decision): https://github.com/Okja-Engineering/reg-corpus/issues/5
- PR #7 (https://github.com/Okja-Engineering/reg-corpus/pull/7), which
  records this ADR. Its commits add `internal/meta/` and `internal/l0/`,
  move `fetch` onto them and add `internal/ecfr/metaset.go`. Its review
  threads on `internal/ecfr/store.go` and `internal/ecfr/metaset.go` found
  the lost update and the unchecked identity.
- PR #2 (https://github.com/Okja-Engineering/reg-corpus/pull/2): Findings,
  "`meta.yaml` has two writers" and "fetch checks only the two meta.yaml
  fields it can compare".
