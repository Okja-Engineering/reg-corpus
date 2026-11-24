# 0013. A cite joins as the document reads; a heading's separator is not operative text

- **Status:** Accepted
- **Date:** 2026-11-24
- **Decided by:** the owner, 2026-11-24 about 15:18 CST, in chat (answers to the chief of staff's questions): (F1) "Separator doesn't count (Recommended)"; (F2) "As the document reads (Recommended)"; (F3) "Rule text only (Recommended)"; (F4) "I'll land the 3-line edit (Recommended)"

## Context
Plan step 5 builds `internal/anchors`: `text.md`, normalized text, each
node's `sha256` in `tree.json`, and `sha256_text`. C6's quote matching (plan
step 13), `inventory --init` (step 14), `units.yaml` (step 16) and `fetch
--verify` (step 6) all read what it fixes, and once records store
`source_hashes` (step 16 on), changing it makes cited records stale. No
record exists yet.

ADR 0012 left the separator after a heading at the end of the parent's own
text, so the own texts of one `<P>` joined are exactly its character data
(`(b) Receipt of a loss mitigation application—`), and the spec carried the
rest forward:

> | Whether a heading's "—" or space left at the end of a parent's own text counts in the operative-anchor test and in the per-node hash, and how a multi-anchor cite joins own texts across a heading separator or a paragraph boundary (decision 12) | plan, step 5 |

The spec's definitions left three things open:

> - **Operative anchor:** an anchor whose own text, minus label and heading, is non-empty.

Read word for word, this also leaves the space between a label and its
heading, so every headed node would be operative. On 1024.41's stored raw
XML, 16 of its 91 nodes hold no words of their own: 15 lead-ins whose own
text past the heading is only "—" or " " (`b`, `b-2`, `c`, `c-2`, `c-2-v`,
`c-3`, `c-4`, `c-4-ii`, `e`, `e-2`, `f`, `h`, `k`, `k-1`, `k-2`), and
`c-4-ii-A`, whose own text is "(A)".

> - **Multi-anchor cite:** `cite` may be a list. The anchors' own texts are joined with one space in document order, and a quote may cross a join only between anchors that are consecutive in the document.

One space always gives "…application— (1) Complete…" and "…delay. (A) (1)
Except…", which the document doesn't read; a quote copied from the
regulation across them would fail C6. Fourteen of 1024.41's 75 `<P>` hold
more than one node.

> - **Normalized text:**
>   - Unicode NFC
>   - every whitespace run (including U+00A0) collapsed to one space
>   - curly quotes mapped to ASCII; en and em dashes mapped to `-`
>
>   C6 matches `text` against normalized own text. Each `tree.json` node carries the `sha256` of its normalized own text, and `sha256_text` hashes the whole normalized source.

Which characters are whitespace, whether runs are trimmed, which quotes and
dashes, in what order, and what "the whole normalized source" is (the
citation note too?) were not said, and every hash depends on them.

Go's standard library has no exported NFC; the plan's stack names
`golang.org/x/text/unicode/norm`. With a dependency, the repo's shell gate
tests (`.githooks/pre-commit_test.sh`, `.github/c6_test.sh`) fail, because
they copy `go.mod` without `go.sum`.

No command wrote `tree.json` or `text.md`, and the plan's command list
("fetch, meta, check, inventory, verify, upgrade, release, show, text,
topics, version") has none that could, offline.

## Options considered
The chief of staff asked the owner four questions. Each chosen option's
text as offered (calls.md, the step 5 record):
- F1, the operative test and the per-node hash: **"Separator doesn't count
  (Recommended)"**, offered as "75 of 91 nodes operative. Only the one dash
  or space that ends a heading is ignored. The per-node hash still covers
  it, so an upstream change to that separator still marks the node stale."
  Chosen.
- F2, the multi-anchor join: **"As the document reads (Recommended)"**,
  offered as "Nothing between own texts inside one <P>, one space between
  <P>s. Rebuilds the text exactly, so a quote copied from the regulation
  matches. Amends the spec's "one space" line." Chosen.
- F3, `sha256_text`: **"Rule text only (Recommended)"**, offered as
  "Heading plus all paragraphs joined. Changes only when the rule's words
  change, and step 6 can compute the same hash from govinfo's text for
  verified_on." The question stated the normalization: "NFC, whitespace
  collapsed (no trim), curly quotes and en/em dashes mapped." Chosen.
- F4, the x/text dependency: **"I'll land the 3-line edit
  (Recommended)"**, offered as "You make (or approve me drafting for you to
  apply) a small PR first; step 5 then uses x/text as the plan says.
  yaml.v3 (step 12) needs the same edit anyway." Chosen.

The other options, as the chief of staff drafted them for these questions
(their wording as offered is not recorded):
- F1: O1, the literal rule, the separator counts (90 of 91 operative; 15
  lead-ins with no words of their own each need an inventory sign-off); O3,
  a general trim of whitespace and dashes (75 today, but it would also drop
  a remainder such as " – "). For the hash, H2: hash with the separator
  removed, which drops a source character from change detection.
- F2: J1, one space always, the spec's words; J3, one space except nothing
  after a heading's "—", which still gives "(A) (1)".
- F3: S2, S1 plus the citation note, which changes with each amendment
  even when the rule's words don't; S3, the sha256 of `text.md`, which ties
  the hash to a display format.
- F4: B, standard library only, with a guard that allows only the
  characters seen so far and fails on any other; C, port the NFC tables by
  hand.

## Decision
The owner decided:
1. (F1) An anchor is operative when its own text, minus its label, the one
   space before its heading, the heading, and the one "—" or space that
   ends the heading, is non-empty: 75 of 1024.41's 91 nodes. A node's
   `sha256` covers its whole normalized own text, that separator included.
2. (F2) A multi-anchor cite joins own texts with nothing between two in one
   `<P>` and one space between `<P>`s. This amends the spec's "one space".
3. (F3) `sha256_text` is the sha256 of the normalized section heading, one
   space, then every anchor's own text joined by the F2 rule; the citation
   note is left out. Normalization: "NFC, whitespace collapsed (no trim),
   curly quotes and en/em dashes mapped."
4. (F4) `golang.org/x/text` is a dependency, as the plan's stack says; the
   gate tests' change lands first, in the owner's own PR.

How it is carried out was the chief of staff's (an agent's) choice, not the
owner's call. Each choice and its reason:
- **Normalization, pinned (N1–N6).** Within the owner's F3 statement:
  - N1, whitespace: Unicode White_Space (Go's `unicode.White_Space`, which
    holds U+00A0, U+2000 to U+200A, U+202F, U+3000, U+0085 and the ASCII
    ones). Zero-width characters (U+200B, U+200D, U+FEFF) are not
    White_Space and stay.
  - N2: runs collapse to one space and are not trimmed, so a lead-in whose
    own text ends in a heading's space keeps it in its hashed text.
  - N3: exactly U+2018 and U+2019 to `'`, U+201C and U+201D to `"`. Left as
    they are: U+201A, U+201B, U+201E, U+201F, primes and guillemets.
  - N4: exactly U+2013 and U+2014 to `-`. Left as they are: U+2010 to
    U+2012, U+2015 and U+2212. The parser's wider dash set is for finding
    labels, another job.
  - N5, order: NFC, then the mappings, then the collapse. No input tells
    the orders apart under Unicode 17.0.0 (x/text v0.42.0's tables and Go
    1.27's): no character's NFC form is a mapped character or a new
    whitespace run, and no mapped character, ASCII replacement or space is
    the first of a composition. The order is pinned all the same.
  - N6: a cite's text is the raw own texts joined, then normalized as one
    string. Normalizing each piece first would leave two spaces where a
    `<P>` ends in one, and NFC is not closed under concatenation. Each
    node's hash normalizes its own text alone.

  Why: every hash depends on these, and each is the narrowest reading of
  the owner's words.
- **`text.md`'s form (F5: T1).** `# ` and the section heading, a blank
  line, then each `<P>`'s text (its own texts joined by the F2 rule) with a
  blank line between, and last the citation note; verbatim, with no anchor
  IDs, markup or normalization; UTF-8, LF, one LF at the end. Why: eval arm
  (a) reads it (spec "Evaluation", arms), so it stays a text-only baseline; anchor IDs
  in it would give arm (a) the index meant for arms (b) and (c).
- **Files for 1024.41 only (F6).** 1024.30 builds (a test shows it), but
  its goldens are plan step 9's, and stage 01 says `make test` passing says
  nothing about R1a for a source until its goldens exist; 1024.31 doesn't
  parse yet (step 9); Supplement I has no raw XML yet (step 8).
- **`regcorpus parse <section> ...`, a command the plan's list lacks
  (F7).** Offline, from the corpus root: it reads the stored raw XML and,
  through `l0.Update` under the source's lock and after the stored-state
  check (ADR 0010), writes `tree.json` and `text.md` and sets
  `sha256_text`. A re-run that would write the same bytes changes nothing.
  A parse failure exits 1 with the parser's line and column, an
  `overrides.yaml` exits 1 (applying overrides isn't built yet, R1a), an
  unknown section exits 2; each writes nothing. Why: the files are derived
  from committed raw XML, so deriving them needs no network, unlike
  `fetch`; a test that writes under `sources/` would hide the write. It
  also gives a parse failure a command that exits non-zero, which step 4
  lacked. Plan step 6's `fetch --verify` rebuilds a fresh fetch with the
  same `Build` and compares.
- **Where it lives (F8).** The command's handler is in `internal/ecfr`,
  as `meta set`'s is, because the source table is (ADR 0010 left exporting
  or moving it to step 5); it passes an `l0.Source` to `internal/anchors`,
  which imports `internal/parse`, `internal/l0`, `internal/meta` and x/text
  and never `internal/ecfr`, so step 6, in `internal/ecfr`, can import it.
  `internal/l0` gains `Derive`, the one writer of derived files, called
  inside `Update`'s callback.
- **`tree.json`'s form (F9).** `parse.Node` gains `sha256`
  (`json:"sha256"`), empty in what `Parse` returns; no anchor ID or
  operative flag is stored, since both follow from the tree. Written with
  a two-space indent, no HTML escaping, and a trailing LF.
- **`Operative` and `Join` are exported and tested now (F10),** before
  their first callers (C6, `inventory --init`, `units.yaml`), so the
  owner's F1 and F2 calls are fixed in tests, not in prose.

Made while carrying it out, by the implementing agent, within the choices
above (for the chief of staff to confirm):
- **The separator is exactly one "—" or one U+0020 space,** as ADR 0012
  defines a heading's end. So a no-break space, a second space or an en
  dash after a heading is text, and a node holding only that is operative;
  and a node with no heading whose own text is its label and a space,
  "(a) ", is operative too, since only a heading's separator is taken
  away. None of these is in 1024.41.
- **`Join` puts the nodes in document order** (by `<P>`, then depth: the
  nodes of one `<P>` are a chain) whatever order they are given in, and
  refuses a node given twice. Anchors of one `<P>` that are not consecutive
  are still joined with nothing between; that a quote may not cross that
  join is C6's rule (step 13).
- **`Operative` returns an error** for a node whose own text doesn't start
  with its label, and its heading if it has one, rather than guess.
- **Sections are named, never defaulted:** `regcorpus parse` with no
  section exits 2, so a bare run can't write 1024.30's files before step 9.
- **`Derive` checks every derived file before writing any:** each must be
  missing or a regular file, reached through real directories, or nothing
  is written; then only files whose bytes differ are replaced. An
  `overrides.yaml` of any kind (a file, an empty file, a directory, a
  symlink) refuses.
- **`text.md`'s last LF** is the citation note's own (its character data
  ends in one in 1024.41), or one added when it has none.

## Spec changes
- `docs/03-spec.md`:
  - Decisions-table row 13 is added: "| 13 | How is text normalized, which
    anchors are operative, how does a multi-anchor cite join, and what do
    the hashes cover? | Normalized text is NFC, then the curly quotes
    U+2018, U+2019, U+201C and U+201D mapped to ASCII and the en and em
    dashes to `-`, then every run of Unicode White_Space collapsed to one
    space, never trimmed. An anchor is operative when its own text, minus
    its label, the one space before its heading, the heading, and the one
    "—" or space that ends the heading, is non-empty. A multi-anchor cite
    joins own texts in document order with nothing between two in one `<P>`
    and one space between `<P>`s, and is normalized after joining. A node's
    `sha256` covers its whole normalized own text, a heading's separator
    included; `sha256_text` covers the normalized section heading, one space
    and every anchor joined, not the citation note (the owner's calls; the
    normalization detail, `text.md`'s form, and the offline `regcorpus
    parse` that writes `tree.json`, `text.md` and `sha256_text`: the chief of
    staff's choices). [ADR 0013](adr/0013-cites-join-as-the-document-reads.md) |"
  - Definitions, "Operative anchor": "an anchor whose own text, minus label
    and heading, is non-empty." → "an anchor whose own text, minus its
    label, the one space before its heading, the heading, and the one "—"
    or space that ends the heading, is non-empty (decision 13)."
  - Definitions, "Normalized text": "- **Normalized text:**" → "-
    **Normalized text** (decision 13), in this order:"; its items "Unicode
    NFC", "every whitespace run (including U+00A0) collapsed to one space"
    and "curly quotes mapped to ASCII; en and em dashes mapped to `-`" →
    "Unicode NFC (not NFKC)", "curly quotes mapped to ASCII (U+2018 and
    U+2019 to `'`, U+201C and U+201D to `"`); en and em dashes (U+2013,
    U+2014) mapped to `-`; no other character is mapped" and "every run of
    Unicode White_Space (including U+00A0) collapsed to one space, never
    trimmed", in that order; and "Each `tree.json` node carries the
    `sha256` of its normalized own text, and `sha256_text` hashes the whole
    normalized source." → "Each `tree.json` node carries the `sha256` of
    its whole normalized own text, a heading's separator included, and
    `sha256_text` hashes the normalized source: the section heading, one
    space, then every anchor's own text joined as a multi-anchor cite joins
    them, without the citation note."
  - Definitions, "Multi-anchor cite": "The anchors' own texts are joined
    with one space in document order, and a quote may cross a join only
    between anchors that are consecutive in the document." → "The anchors'
    own texts are joined in document order, with nothing between two in one
    paragraph (`<P>`) and one space between paragraphs, so the join reads as
    the document does; it is normalized after joining (decision 13). A quote
    may cross a join only between anchors that are consecutive in the
    document."
  - "Carried forward": the row "Whether a heading's "—" or space left at
    the end of a parent's own text counts in the operative-anchor test and
    in the per-node hash, and how a multi-anchor cite joins own texts across
    a heading separator or a paragraph boundary (decision 12) | plan, step
    5" is removed.
  - Header "Amended: 2026-11-22 by ADRs 0007–0012" → "Amended: 2026-11-22
    by ADRs 0007–0012; 2026-11-24 by ADR 0013".
- `docs/04-plan.md`:
  - Header "Amended: 2026-11-22 by ADRs 0007, 0010, 0011 and 0012" → "…
    and 0012; 2026-11-24 by ADR 0013".
  - Files: `cmd/regcorpus/main.go` "fetch, meta, check, inventory, …" →
    "fetch, meta, parse, check, inventory, …"; `internal/ecfr/` "versioner
    client; full/{date} XML; fetch and meta set" → "…; fetch, meta set and
    parse"; `internal/l0/` "…, meta.yaml updates (ADR 0010)" → "…, meta.yaml
    updates (ADR 0010), derived files (ADR 0013)"; `internal/anchors/` "tree
    → anchors, text.md, normalization, per-node hashes" → "…, per-node
    hashes, the operative test, joins (ADR 0013)".
  - Step 5: "5. `internal/anchors`:" → "5. `internal/anchors` and
    `regcorpus parse`, offline from the stored raw XML (ADR 0013):";
    "Generate `text.md` from the tree." → "Generate `text.md` from the tree:
    the section as it reads, verbatim."; "Decide whether a heading's "—" or
    space left at the end of a parent's own text counts in the
    operative-anchor test and the per-node hash, and how a multi-anchor cite
    joins own texts across a heading separator or a paragraph boundary (spec
    "Carried forward"; ADR 0012)." → "A heading's "—" or space left at the
    end of a parent's own text doesn't count in the operative-anchor test and
    does count in the per-node hash; a multi-anchor cite joins own texts with
    nothing inside a `<P>` and one space between `<P>`s (spec decision 13).";
    and a bullet is added: "Write 1024.41's files; 1024.30's and 1024.31's
    are step 9's."
- `stages/01-fetch-parse/CONTRACT.md`:
  - Outputs: "`tree.json`: the parsed tree; each node carries the `sha256`
    of its normalized own text." → the same, then "(spec decision 13)."
    (rewrapped); "`text.md`: generated from `tree.json`." → "`text.md`:
    generated from `tree.json`: the section as it reads, verbatim."; and
    after the list, a paragraph is added: "`regcorpus parse <section>`
    writes `tree.json` and `text.md` and sets `sha256_text`, offline, from
    the stored raw XML (plan step 5)."
  - Check that ends it, the `make test` item, gains: "From plan step 5,
    exit 0 also means 1024.41's committed `tree.json`, `text.md` and
    `sha256_text` are what `regcorpus parse` makes from its raw XML."
  - Who may run it: "An agent may write parser code and goldens, and run
    the checks above." → "An agent may write parser code and goldens, run
    the checks above, and run `regcorpus parse`, which is offline and
    derives every file from the committed raw XML; a person reviews its
    output in the PR diff."
- `docs/adr/README.md`, the index, gains the line "| [0013](0013-cites-join-as-the-document-reads.md) | A cite joins as the document reads; a heading's separator is not operative text | Accepted |".
- Package docs: `internal/anchors/doc.go` states what is built; the
  sentences of `internal/parse/doc.go`, `internal/l0/doc.go`,
  `internal/meta/doc.go` and `internal/ecfr/doc.go` that left the
  separator, the hashes, `tree.json` and the source table to plan step 5
  now say what step 5 decided.
- No other lines. The intent and AGENTS.md say nothing about normalization,
  operative anchors or joins; AGENTS.md's `./regcorpus help` lists the new
  command.

## Consequences
- 1024.41 has `tree.json` (91 nodes, each with a `sha256`), `text.md` and
  `sha256_text: "ecda50e7ce958e2fd19a8f1843b3c60bde0dcf533cc2c313cb9c1e1960760fe1"`,
  which an independent computation (Python's `unicodedata` NFC, the raw
  XML read without the Go parser) also gives, as it gives the hashes of
  `b`, `c-2-v` and `c-2-v-A-1`. `make test` checks that the committed files
  are what `regcorpus parse` makes from the raw XML.
- 75 of 1024.41's 91 nodes are operative; the 15 heading-only lead-ins and
  (c)(4)(ii)(A) are not, so the inventory (step 14) lists 75 anchors.
- A change upstream to a heading's separator alone ("—" to ". ") changes
  that node's hash but not whether it is operative.
- A quote copied from the regulation across "—(1)" or "(A)(1)" matches its
  cite's join; across two `<P>`s it matches with one space.
- `sha256_text` doesn't change when only the citation note does.
- `go.mod` requires `golang.org/x/text`. Until the owner's gate-test PR
  merges to main, `.githooks/pre-commit_test.sh` and `.github/c6_test.sh`
  fail on this branch, since they copy `go.mod` without `go.sum`; the
  pre-commit hook and CI build main's checker, so commits are unaffected.
- `regcorpus help` lists `parse`, and `regcorpus parse -h` gives its exit
  codes.
- Changing any of the above after records store `source_hashes` (step 16
  on) makes the records citing a changed anchor stale.

## Requirement IDs
R1 (raw and normalized hashes), R1a (`text.md` generated from `tree.json`),
R3 (coverage at operative anchors); spec "Definitions" (operative anchor,
normalized text, multi-anchor cite) and "Granularity"; C6 (the join rule
quotes are matched under); decisions 6 (CLI only) and 13.

## Links
- calls.md (the owner's calls, quoted on the PR that records this ADR) and
  the step 5 mandate, in the chief of staff's working notes.
- The PR that records this ADR (the chief of staff opens it).
- Commits `e7359a6` (the tests against stubs, red), `0837bda`
  (`internal/anchors`, `l0.Derive`, `regcorpus parse` and 1024.41's files,
  green) and `2e8806b` (a row from the mutant pass):
  `internal/anchors/normalize.go` (`Normalize`, `Hash`),
  `internal/anchors/anchors.go` (`ID`, `Operative`, `Join`),
  `internal/anchors/build.go` (`Build`, `EncodeTree`, `TextMD`,
  `SectionText`, `SHA256Text`), `internal/anchors/write.go` (`Write`),
  `internal/l0/derive.go` (`Derive`), `internal/l0/source.go`,
  `internal/ecfr/parsecmd.go`, `internal/parse/tree.go`, `cmd/regcorpus/main.go`,
  `sources/ecfr/title-12/1024.41/{tree.json,text.md,meta.yaml}`, and the
  tests `internal/anchors/*_test.go`, `internal/l0/derive_test.go`,
  `internal/ecfr/parsecmd_test.go`.
- ADR 0012, "Where the dash or space after a heading goes"; ADR 0010, "Where
  the source table lives".
