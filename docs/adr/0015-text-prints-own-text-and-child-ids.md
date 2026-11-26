# 0015. `regcorpus text` prints one anchor's own text, its children's IDs and its source's `meta.yaml`

- **Status:** Accepted
- **Date:** 2026-11-26
- **Decided by:** the owner, 2026-11-26 about 10:23 CST, in chat (answers to the chief of staff's questions), to be quoted on this ADR's PR: (F1) "Own text + child IDs (Recommended)"; (F2) "All of meta.yaml + sha256 (Recommended)"; (F3) "Exact IDs only (Recommended)"; (F8) "ADR 0015 + acceptance (Recommended)". Each call is only what its option text says (Options considered, below). The rest are agents' choices, not the owner's: the chief of staff's rulings (F4, F5, F6, F7) and the implementer's choices, each marked below.

## Context
Plan step 7 builds R8's `text` command:

> 7. `regcorpus text <anchor>`.

R8 said:

> - `regcorpus text <anchor>` prints one anchor's text and its source metadata.
>
> Output is plain text by default, with `--json` for tools. No command prints more than the records asked for.

and its acceptance item named `show` and `topics`, not `text`:

> - [ ] **R8:** `regcorpus show RX-41-ACK-01` prints the record with its verbatim anchor text and its anchor rules. `regcorpus topics acknowledgment` prints the ID. An unknown term exits non-zero.

So AGENTS.md's "a tool step ends when … its acceptance command behaves as
the spec says" had no command to run for step 7.

- **"One anchor's text".** The spec defines own text ("an anchor's text
  excluding its children, with its label and heading"), and each node's
  `sha256` in `tree.json` covers its own text alone (decision 13). It
  didn't say whether a lookup also says where an anchor's children are.
  For a lead-in, the own text is often only its heading: `1024-41-b-2`'s
  is "(2) Review of loss mitigation application submission—", and
  `1024-41-c-4-ii-A`'s is "(A)". `text.md` holds no anchor IDs (ADR 0013,
  T1, to keep eval arm (a) text-only), so without them a reader of a
  lookup can only guess a child's ID.
- **Anchor syntax.** The spec's one form is "Definitions", Anchor:
  `1024-41-b-2-i-B`, `1024-41-b-2-i-interp-1`, `1024-31-day`.
  `anchors.ID` makes the first from a node's label path (ADR 0013), case
  sensitive, since a capital is its own level. Nothing parsed an anchor ID
  back to a source and a node: the resolver this step builds is the one
  C6's quote matching (step 13), `inventory --init` (step 14), `show` (step
  15) and `verify` (step 19) need too.
- **What is stored.** 1024.41 has `tree.json` (91 nodes) and a `meta.yaml`
  with all 13 fields. 1024.30 and 1024.31 have no `tree.json` (step 9);
  Supplement I is not in the source table (step 8).
- **Where the source table is.** Unexported, in `internal/ecfr`, beside the
  network client (ADRs 0010, 0013); a command that needs it has its handler
  there and passes an `l0.Source` to a network-free package.

## Options considered
Asked of the owner (the option texts as offered):
- F1, what prints besides the own text: "Own text + child IDs
  (Recommended)": "Prints the anchor's own text plus its children's anchor
  IDs, with no child text. A reader can walk the tree, and no extra rule
  text is printed." Or "Own text only" (the literal minimum; a reader finds
  children only by guessing their IDs), "Own text + whole subtree" (reads
  like the document, but prints more than one anchor, and the printed text
  no longer hashes to the printed sha256), or "Child IDs, plus --subtree".
  Chosen: own text plus child IDs.
- F2, which source metadata: "All of meta.yaml + sha256 (Recommended)":
  "Every field in the source's meta.yaml, as stored, plus the node's own
  sha256. Nothing is hidden, and no one has to decide what counts as
  provenance." Or "A chosen subset" (cfr, tier, role, the version dates and
  verified_on, plus the node's sha256). Chosen: all of `meta.yaml`.
- F3, which anchor forms: "Exact IDs only (Recommended)": "For example
  `1024-41-b-2-i-B`, matched exactly and case-sensitively. It is the spec's
  one form, and B and b are different levels." Or "Also the CFR form"
  (`1024.41(b)(2)(i)(B)`, a second syntax to specify and test) or
  "Case-insensitive IDs" (hides a wrong level). Chosen: exact IDs only.
- F8, paperwork: "ADR 0015 + acceptance (Recommended)": "Records your calls
  and adds this acceptance item: `regcorpus text 1024-41-b-2-i-B` prints
  that anchor's own text and its source metadata; an unknown anchor exits
  non-zero." Or "No ADR" (the acceptance gap stays open). Chosen: this ADR
  and the item.

Ruled by the chief of staff, as the step's mandate drafted them, each
following what `parse` and `fetch` already do:
- F4, exit codes: within the CLI's 0/1/2 (chosen), or every miss 1, or a
  miss listing the anchors that exist (91 IDs, more than asked for).
- F5, sources not built yet: refuse with the plan step that builds them
  (chosen), or print a section's `meta.yaml` alone (not an anchor).
- F6, checks before printing: I2, check what it reads (chosen); I1, trust
  `tree.json`; I3, I2 plus the raw XML's hash and a rebuild (heavier, and a
  read would depend on the parser).
- F7, where the code lives: A, the handler in `internal/ecfr` and the
  resolver and printers in `internal/lookup`, which never imports
  `internal/ecfr` (chosen); B, move the source table out (refactors merged
  code and amends ADRs 0010 and 0013 for no step 7 gain); C, everything in
  `internal/ecfr` (the resolver then pulls in the network client).

## Decision
The owner decided, each call only what its option text says:
1. (F1) `regcorpus text` prints an anchor's own text and its children's
   anchor IDs, never a child's text.
2. (F2) It prints every field of the source's `meta.yaml`, as stored, and
   the node's own `sha256`.
3. (F3) It reads anchor IDs only, matched exactly and case-sensitively:
   `1024-41-b-2-i-B` resolves; `1024.41(b)(2)(i)(B)` and `1024-41-B` don't.
4. (F8) This ADR records the calls, and R8's acceptance gains the item
   quoted above.

The chief of staff ruled (agents' choices), as the step's mandate drafted
them:
- **F4.** Exit 0 printed. Exit 1: the ID names a source but no anchor in
  it (an unknown label, another case, the section's own ID `1024-41`);
  its source's anchors are a later step's (F5); or a stored file refuses
  (F6, or anything `l0.Read` refuses). Exit 2: usage (no anchor, more than
  one, an unknown flag); a string that names no source (`1024.41(b)`,
  `1024-40-a`), as `parse` treats an unknown section; not the corpus root;
  a file that can't be read. A miss names the anchor and its source and
  lists nothing else. Nothing goes to stdout on an error.
- **F5.** 1024.30's and 1024.31's anchors exit 1, "anchors for 1024.31 are
  plan step 9's", from the source table's `Goldens`; an anchor with an
  `interp` segment exits 1, "Supplement I comment anchors are plan step
  8's".
- **F6 (I2).** Before printing, it checks only what it reads: `l0.Read`
  (exact names, case variants refused, a strict `meta.yaml`); the
  `meta.yaml` names the source as the source table does; `tree.json`
  decodes strictly and its `section` is the source's; the anchor's
  `sha256` is `anchors.Hash` of its own text; and `anchors.SHA256Text` of
  the tree is `meta.yaml`'s `sha256_text`, so the metadata printed
  describes the text printed. Any failure exits 1. It reads no raw XML,
  `text.md`, `.cache/`, `registry/` or `staging/`, and nothing from the
  network.
- **F7 (A).** `ecfr.TextMain` is a thin handler beside `parse`'s; it
  passes the source table, each source's name, `l0.Source` and `Goldens`,
  to `internal/lookup`, which holds the resolver and the printers and
  imports `internal/l0`, `internal/meta`, `internal/parse`,
  `internal/anchors` and `internal/sink`, never `internal/ecfr`; a test
  proves `go list -deps` of it holds no `net/http`.
- **The output** (the mandate's definition of done). Plain, a line each:
  the anchor ID; its own text, verbatim, as `tree.json` holds it (never
  normalized or wrapped); an empty line; `sha256: ` and the node's
  `sha256`; `children: ` and its children's IDs in document order,
  space-separated, or `none`; `meta.yaml: ` and the source's `meta.yaml`
  path; then each line of that `meta.yaml` as `meta.Marshal` renders it,
  which is the stored file's bytes. `--json`: one line, a JSON object with
  the keys `anchor`, `text`, `sha256`, `children` (an array), `meta_path`
  and `meta` (an object keyed by the `meta.yaml` field names, in schema
  order, absent fields left out, `verified_on` null when null). Every line
  goes through the sink, one Write per line; `--json` is one Write.

The implementer's choices (an agent's), each with its reason:
- **The resolver is `lookup.Corpus.Anchor`.** A `lookup.Corpus` is the
  corpus root, the directory every source is under, and the source table
  as `lookup.Source`s (`Name`, `Stored`, and `Pending`, the table's
  `Goldens`); `ecfr`'s `corpus` builds it. Why: steps 13, 14, 15 and 19
  resolve anchors too, and the table stays where ADR 0013 put it.
  `lookup.ErrNoSource`, `ErrNotBuilt` and `ErrNoAnchor` name the misses
  for later callers (`errors.Is`); a stored file's refusal stays an
  `*l0.Conflict`.
- **Which source an ID names.** A source's section ID is `anchors.ID` of
  its name and no path (`1024-41`); an ID names the source whose section ID
  it is, or starts with followed by `-`. So `1024-410-a` names none (exit
  2), and `1024-41-a-` names 1024.41 and no anchor in it (exit 1). The
  checks run in this order: usage; the corpus root (`l0.CorpusRoot`, so
  outside it every anchor exits 2, as `parse` checks the root before a
  source's goldens); the source; an `interp` segment (step 8); `Pending`
  (step 9); then the stored files. So `1024-99-a-interp-1` names no source
  (exit 2), and the stored files are checked before the anchor is looked
  up: a tree that fails a check refuses an unknown ID too.
- **Three checks past F6's list**, each on the tree as it decodes: no null
  node (`"children": [null]` decodes without error, and would crash the
  walk); none with no label path (its ID would be the section's own, which
  F4 makes no anchor); and no anchor ID twice (an ID resolves to one node).
  Strict decoding is `DisallowUnknownFields` and nothing after the one
  value. Each tree refusal names `tree.json` and says to run `regcorpus
  parse <section>`.
- **`meta`'s JSON is `internal/meta`'s.** `meta.JSON` writes the fields
  `meta.Marshal` writes, keyed and ordered as there, each value as its
  type is: a string, `official` a boolean, `version_fr_citations` an array,
  `verified_on` a string or null. Marshal and JSON share one walk of the
  schema, and JSON refuses what Marshal refuses. Why: `internal/meta` owns
  the schema (ADR 0010); a second list of the fields in `internal/lookup`
  would drift.
- **DEL in `--json`.** `encoding/json` escapes every C0 control, U+2028 and
  U+2029, but not DEL (U+007F), which the sink would write as `\x7f`, not a
  JSON escape; so the JSON printer writes DEL as `\u007f` before the sink.
  A C1 control is left to the sink, whose `\u0080` to `\u009f` is a JSON
  escape of itself. So the line the stream holds is JSON that decodes to
  the stored text. In plain output the sink escapes a control character,
  U+2028 or U+2029 in a text as `\xNN` or `\uNNNN`, on its line.
- **`internal/l0` gains what a reader needs, and nothing is copied.**
  `State.Names` is the identity check `Stored` made first, split out so a
  reader makes it without reading raw XML (its messages now speak of
  reading too); `State.TreeJSON` reads `tree.json` as `Read` found it, by
  exact name, a non-regular file a `*Conflict`; `CorpusRoot` is `ecfr`'s
  `checkRoot` moved, which `checkRoot` now calls. `ecfr`'s unexported
  `lookup` is renamed `sourceNamed`, since the `internal/lookup` import
  takes the name.
- **Flags before the anchor**, as `fetch` takes them before sections:
  `text 1024-41-a --json` is a usage error (two arguments).

## Spec changes
- `docs/03-spec.md`:
  - Decisions-table row 15 is added, this line exactly (its link is
    relative to `docs/`):

    ```
    | 15 | What does `regcorpus text` print, and which anchor forms does it read? | One anchor's own text, verbatim, never normalized or wrapped; its `sha256` as `tree.json` holds it; its children's anchor IDs in document order, never their text; and every field of its source's `meta.yaml` as stored (`--json`: the same, as one JSON object). It reads anchor IDs only, matched exactly and case-sensitively, and only the source's `meta.yaml` and `tree.json`, offline, which it checks before printing: the `meta.yaml` names the source, `tree.json` is the source's tree, the anchor's `sha256` is its own text's, and the tree's text gives `sha256_text`. A miss, an anchor a later plan step builds (1024.30's and 1024.31's, step 9; Supplement I's comments, step 8) and a stored file that refuses exit 1; usage, a string that names no source and not the corpus root exit 2 (the owner's calls: the children's IDs, all of `meta.yaml`, exact IDs and the acceptance item; the exit codes, the refusals, the checks and where the code lives: the chief of staff's rulings). [ADR 0015](adr/0015-text-prints-own-text-and-child-ids.md) |
    ```
  - R8: "- `regcorpus text <anchor>` prints one anchor's text and its
    source metadata." → "- `regcorpus text [--json] <anchor>` prints one
    anchor's own text, verbatim, its `sha256`, its children's anchor IDs
    and its source's `meta.yaml`; `<anchor>` is an anchor ID, matched
    exactly and case-sensitively (decision 15)."
  - Acceptance, R8: "An unknown term exits non-zero." → "An unknown term
    exits non-zero. `regcorpus text 1024-41-b-2-i-B` prints that anchor's
    own text and its source metadata; an unknown anchor exits non-zero."
  - Header "**Amended:** 2026-11-22 by ADRs 0007–0012; 2026-11-24 by ADR
    0013; 2026-11-25 by ADR 0014" → "…; 2026-11-26 by ADR 0015".
- `docs/04-plan.md`:
  - Header "**Amended:** 2026-11-22 by ADRs 0007, 0010, 0011 and 0012;
    2026-11-24 by ADR 0013; 2026-11-25 by ADR 0014" → "…; 2026-11-26 by
    ADR 0015".
  - Files: `internal/ecfr/` "…; fetch (--verify, --latest), meta set and
    parse" → "…; fetch (--verify, --latest), meta set, parse and text's
    handler"; `internal/lookup/` "show, text, topics" → "show, text,
    topics; the anchor resolver (ADR 0015)".
  - Step 7: "7. `regcorpus text <anchor>`." → "7. `regcorpus text [--json]
    <anchor>` (ADR 0015): one anchor's own text, verbatim, its `sha256`,
    its children's anchor IDs and its source's `meta.yaml`, offline, from
    `meta.yaml` and `tree.json` alone; exact, case-sensitive anchor IDs.
    1024.41's anchors resolve; 1024.30's and 1024.31's are step 9's,
    Supplement I's step 8's."
  - Proof, R8: the command cell "`regcorpus show RX-41-ACK-01`; `regcorpus
    topics acknowledgment`; `regcorpus topics xyz`" gains "; `regcorpus
    text 1024-41-b-2-i-B`; `regcorpus text 1024-41-z`", and the evidence
    cell "…; the unknown term exits non-zero" gains "; the anchor's own
    text and its source metadata print; the unknown anchor exits
    non-zero".
- `docs/adr/README.md`, the index, gains the line "| [0015](0015-text-prints-own-text-and-child-ids.md) | `regcorpus text` prints one anchor's own text, its children's IDs and its source's `meta.yaml` | Accepted |".
- Package docs: `internal/lookup/doc.go` says what is built;
  `internal/ecfr/doc.go`, `internal/l0/doc.go` and `internal/meta/doc.go`
  name `text`'s handler, `Names`, `TreeJSON` and `CorpusRoot`, and `JSON`.
- No other lines. The intent and the stage contracts say nothing about
  `text`; AGENTS.md's step 7 row ("`regcorpus text <anchor>` | agent |
  R8") stays true, and `./regcorpus help` lists the command.

## Consequences
- A lookup is walkable: from `1024-41-b-2`, `children:` names
  `1024-41-b-2-i` and `1024-41-b-2-ii`, and no rule text but the one
  anchor's is printed, so "no command prints more than … asked for" holds
  and the printed text hashes to the printed `sha256`. Eval arms (b) and
  (c) get the index ADR 0013 kept out of arm (a), and arm (b)'s trace rate
  can count exact IDs in `text` calls.
- Every one of 1024.41's 91 anchors prints exactly its own text, and the
  91 printed texts, joined as decision 13 joins them, give `sha256_text`
  (`internal/ecfr`'s tests on the committed corpus).
- `text` reads the working tree. Plan step 25's `make build CORPUS_REF=`
  and `LOOKUP_ONLY=1` change where lookups read from; this step builds no
  seam for that.
- Plan step 8 adds Supplement I to the source table, and the `interp`
  refusal becomes resolution of its comment anchors; plan step 9 clears
  `Goldens` for 1024.30 and 1024.31, and their anchors resolve with no
  change here but definition anchors (`1024-31-day`), which
  `internal/anchors` doesn't name yet.
- The resolver's misses are typed for later callers: C6's quote matching
  (step 13), `inventory --init` (step 14), `show` (step 15) and `verify`
  (step 19).

## Requirement IDs
R8 (lookup; its acceptance item); spec "Definitions" (Anchor, Own text);
decisions 6 (CLI only), 10 (`internal/meta` owns `meta.yaml`'s schema),
13 (own text, the join, what the hashes cover) and 15.

## Links
- This ADR's PR, with the comment quoting the owner's calls and every
  option offered (the chief of staff adds both links when it opens).
- Commits: the tests against stubs (red); `internal/lookup`, `ecfr`'s
  `TextMain`, `meta.JSON` and `l0`'s `Names`, `TreeJSON` and `CorpusRoot`
  (green); this ADR and its amendments (record).
- Files:
  - Code: `internal/lookup/anchor.go` and `text.go` (new),
    `internal/ecfr/textcmd.go` (new), `internal/ecfr/fetch.go`
    (`checkRoot` calls `l0.CorpusRoot`), `internal/ecfr/sources.go`
    (`sourceNamed`), `internal/ecfr/metaset.go`, `internal/l0/source.go`,
    `internal/meta/meta.go`, `cmd/regcorpus/main.go`.
  - Tests: `internal/lookup/text_test.go`, `internal/ecfr/textcmd_test.go`,
    `internal/l0/read_test.go` (new), `internal/meta/meta_test.go`,
    `internal/ecfr/client_test.go` and `granule_test.go` (`sourceNamed`),
    `cmd/regcorpus/main_test.go`.
  - Package docs: `internal/lookup/doc.go`, `internal/ecfr/doc.go`,
    `internal/l0/doc.go`, `internal/meta/doc.go`.
  - Spec docs: `docs/03-spec.md`, `docs/04-plan.md`, `docs/adr/README.md`
    and this ADR.
- ADR 0013, which fixed own text, the per-node `sha256`, `sha256_text` and
  `anchors.ID`; ADR 0010, the `meta.yaml` schema `meta.JSON` renders.
