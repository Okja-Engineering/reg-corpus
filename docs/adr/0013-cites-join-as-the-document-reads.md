# 0013. A cite joins as the document reads; a heading's separator is not operative text

- **Status:** Accepted
- **Date:** 2026-11-24
- **Decided by:** the owner, 2026-11-24 about 15:18 CST, in chat (answers to the chief of staff's questions), quoted on PR #9 (https://github.com/Okja-Engineering/reg-corpus/pull/9#issuecomment-7145434918): (F1) "Separator doesn't count (Recommended)"; (F2) "As the document reads (Recommended)"; (F3) "Rule text only (Recommended)"; (F4) "I'll land the 3-line edit (Recommended)"

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
The chief of staff asked the owner four questions. These are every option
offered, each quoted as offered (PR #9's comment quoting the calls):
- F1, "Operative": does the separator left after a heading make a lead-in
  operative?
  - **"Separator doesn't count (Recommended)"**: "75 of 91 nodes
    operative. Only the one dash or space that ends a heading is ignored.
    The per-node hash still covers it, so an upstream change to that
    separator still marks the node stale." Chosen.
  - **"Separator counts"**: "90 of 91 operative: the 15 heading-only
    lead-ins each need a cite or an `na` with a reason, plus your sign-off
    in the inventory."
  - **"Trim all dashes/space"**: "75 operative on today's data, but it
    would also silently ignore any remainder made only of dashes or
    whitespace, not just the one separator."
- F2, "Join": which join quotes are checked against.
  - **"As the document reads (Recommended)"**: "Nothing between own texts
    inside one `<P>`, one space between `<P>`s. Rebuilds the text exactly,
    so a quote copied from the regulation matches. Amends the spec's "one
    space" line." Chosen.
  - **"One space always"**: "Keep the spec's words. Quotes across "—(1)"
    or "(A)(1)" fail the check unless the extractor and verifier insert a
    space."
- F3, "Text hash": whether `sha256_text` covers the citation note. The
  question itself stated the normalization, so the owner's answer was
  given on it: "Normalization itself follows the spec: NFC, whitespace
  collapsed (no trim), curly quotes and en/em dashes mapped."
  - **"Rule text only (Recommended)"**: "Heading plus all paragraphs
    joined. Changes only when the rule's words change, and step 6 can
    compute the same hash from govinfo's text for verified_on." Chosen.
  - **"Include the source note"**: "Also covers the CITA, so the hash
    changes with each amendment notice even when the rule text is the
    same."
- F4, "x/text": how to take the NFC dependency.
  - **"I'll land the 3-line edit (Recommended)"**: "You make (or approve me
    drafting for you to apply) a small PR first; step 5 then uses x/text as
    the plan says. yaml.v3 (step 12) needs the same edit anyway." Chosen.
  - **"Stdlib only, NFC guard"**: "No dependency: allow only characters
    already in NFC (ASCII, §, curly quotes, dashes, NBSP) and fail loudly
    on anything else. Each new character later needs an entry; departs
    from the plan's stack."

## Decision
The owner decided:
1. (F1) An anchor is operative when its own text, minus its label, the one
   space before its heading, the heading, and the one dash or space that
   ends the heading, is non-empty: 75 of 1024.41's 91 nodes. A node's
   `sha256` covers its whole normalized own text, that separator included.
2. (F2) A multi-anchor cite joins own texts with nothing between two in one
   `<P>` and one space between `<P>`s. This amends the spec's "one space".
3. (F3) `sha256_text` is the sha256 of the section heading, one space,
   then every anchor's own text joined by the F2 rule, normalized as one
   string; the citation note is left out. Normalization, as the question
   stated it: "NFC, whitespace collapsed (no trim), curly quotes and en/em
   dashes mapped."
4. (F4) `golang.org/x/text` is a dependency, as the plan's stack says; the
   gate tests' change lands first, in the owner's own PR.

How it is carried out was the chief of staff's (an agent's) choice, not the
owner's call. Each choice and its reason:
- **Normalization, pinned (N1–N6).** Within the owner's F3 answer, given
  on the question's statement of the normalization:
  - N1, whitespace: Unicode's White_Space property (Go's
    `unicode.White_Space`), all 25 of its characters in Unicode 17.0.0:
    U+0009 to U+000D, U+0020, U+0085, U+00A0, U+1680, U+2000 to U+200A,
    U+2028, U+2029, U+202F, U+205F and U+3000. Zero-width characters
    (U+200B, U+200D, U+FEFF) are not White_Space and stay.
  - N2: runs collapse to one space and are not trimmed, so a lead-in whose
    own text ends in a heading's space keeps it in its hashed text.
  - N3: exactly U+2018 and U+2019 to `'`, U+201C and U+201D to `"`. Left as
    they are: U+201A, U+201B, U+201E, U+201F, primes and guillemets.
  - N4: exactly U+2013 and U+2014 to `-`. Left as they are: U+2010 to
    U+2012, U+2015 and U+2212. The parser's wider dash set is for finding
    labels, another job.
  - N5, order: NFC, then the mappings, then the collapse. No input tells
    the orders apart under Unicode 17.0.0 (x/text v0.42.0's tables and Go
    1.27's), checked over every scalar value: no character's NFC form holds
    a mapped or White_Space character unless the character is one, and no
    mapped character, ASCII replacement or White_Space character is the
    first or the second of a canonical composition; each has combining
    class 0. The order is pinned all the same.
  - N6: a cite's text is the raw own texts joined, then normalized as one
    string, and so is `sha256_text`'s, the heading, the space after it and
    the body together. Normalizing each piece first would leave two spaces
    where a `<P>` or the heading ends in one, and NFC is not closed under
    concatenation. Each node's hash normalizes its own text alone.

  Why: every hash depends on these, and each is the narrowest reading of
  the owner's words.
- **`text.md`'s form (F5: T1).** `# ` and the section heading, a blank
  line, then each `<P>`'s text (its own texts joined by the F2 rule) with a
  blank line between, and last the citation note; verbatim, with no anchor
  IDs, no normalization and no markup but the "# " that starts its heading
  line; UTF-8, LF, one LF at the end. Why: eval arm (a) reads it (spec
  "Evaluation", arms), so it stays a text-only baseline; anchor IDs in it
  would give arm (a) the index meant for arms (b) and (c).
- **Files for 1024.41 only (F6).** 1024.30 builds (a test shows it), but
  its goldens are plan step 9's, and stage 01 says `make test` passing says
  nothing about R1a for a source until its goldens exist; 1024.31 doesn't
  parse yet (step 9); Supplement I has no raw XML yet (step 8).
- **`regcorpus parse <section> ...`, which this ADR adds to the plan's
  command list (F7).** Offline, from the corpus root: it reads the stored
  raw XML and, through `l0.Update` under the source's lock and after the
  stored-state check (ADR 0010), writes `tree.json` and `text.md` and sets
  `sha256_text`. A re-run that would write the same bytes changes nothing.
  A parse failure exits 1 with the parser's line and column, an
  `overrides.yaml` exits 1 (applying overrides isn't built yet, R1a), a
  source whose goldens aren't in (1024.30, 1024.31) exits 1, an unknown
  section exits 2; each writes nothing. Why: the files are derived from
  committed raw XML, so deriving them needs no network, unlike `fetch`; a
  test that writes under `sources/` would hide the write. It also gives a
  parse failure a command that exits non-zero, which step 4 lacked. Plan
  step 6's `fetch --verify` rebuilds a fresh fetch with the same `Build`
  and compares.
- **Where it lives (F8).** The command's handler is in `internal/ecfr`,
  as `meta set`'s is, because the source table is (ADR 0010 left exporting
  or moving it to step 5); it passes an `l0.Source` to `internal/anchors`,
  which imports `internal/parse`, `internal/l0`, `internal/meta` and x/text
  and never `internal/ecfr`, so step 6, in `internal/ecfr`, can import it.
- **`tree.json`'s form (F9).** `parse.Node` gains `sha256`
  (`json:"sha256"`), empty in what `Parse` returns; no anchor ID or
  operative flag is stored, since both follow from the tree. Written with
  a two-space indent, no HTML escaping, and a trailing LF.
- **`Operative` and `Join` are exported and tested now (F10),** before
  their first callers (C6, `inventory --init`, `units.yaml`), so the
  owner's F1 and F2 calls are fixed in tests, not in prose.
- **The separator is read on the normalized own text (PR #9's reviews).**
  Within the owner's F1 words, "the one dash or space that ends a
  heading": the verdict is decided on the node's whole own text normalized
  as one string, the text its hash covers. Past the normalized label, and
  the one space and the heading if it has one, one `-` or else one space is
  the separator, so an em or en dash, a space, a no-break space or a run of
  White_Space is it. So two nodes with the same path, the same heading and
  the same normalized own text get the same verdict. The first review read
  what follows the heading normalized on its own; the second made it the
  whole own text, since normalizing the heading and the rest apart gave a
  heading that ends in White_Space two verdicts for one hash.
- **A cite joins by runs (PR #9's first review).** `Join` takes the
  document and the cited anchors and gives one string per longest run of
  anchors consecutive in document order, in document order, and refuses an
  empty cite, a nil anchor, one not in the document and one given twice,
  each with a named error; `text.md` and `sha256_text` join the whole
  document as one run. So a quote can't cross a join between anchors that
  aren't consecutive (spec "Multi-anchor cite"): `[k-1, k-1-ii]` is two
  runs, `[k-1-i, k-1-ii]` one. Document order and adjacency are one
  exported notion, so plan step 13 needs no second copy.
- **`l0.Update` is the only writer of a source's files (PR #9's first
  review).** A derived file is written only under the source's lock, from
  the raw XML `Stored` checked, and only as the source's own `tree.json` or
  `text.md`. No exported function replaces a file at a path its caller
  names (`StoreOnce`, given a path, stores a file that isn't there and
  never replaces one), so raw XML is never overwritten and `meta.yaml` is
  only what `internal/meta` renders. Every refusal (exit 1) is found before
  the first write, so exit 1 means nothing was written for that section.
  An error while writing is not a refusal: `Update` returns the files it
  replaced before it, the error names the file it was writing (the second
  review), `regcorpus parse` prints them and exits 2, and a re-run writes
  the rest. An `overrides.yaml` of any kind (a file, an empty file, a
  directory, a symlink) refuses.
- **`regcorpus parse` writes only sources whose goldens are in (PR #9's
  first review).** Today it writes 1024.41's files and refuses 1024.30 and
  1024.31 (exit 1, "goldens are plan step 9's", nothing written).
- **Exact names (PR #9's reviews).** A source's `meta.yaml`, `tree.json`,
  `text.md` and `overrides.yaml` (the first review), and every directory on
  the way to them, from `sources/` down and under `.cache/` (the second),
  are found by exact name among their parent's entries; an entry whose name
  differs from one only in case (Unicode simple case folding, so `ſ`,
  U+017F, which APFS folds to `s`, too), whatever it is, is refused (exit
  1). `l0.RealDirs` is the one check of the way, for `fetch`, `StoreOnce`,
  `Read` and `Update`. Why: on a case-insensitive filesystem (APFS) a
  `TREE.JSON` would be read and written as `tree.json`, and a `RAW/` as
  `raw/`, while git records the other name. The file `StoreOnce` stores
  (`raw/<date>.xml`, a first `meta.yaml`, `fetch --date`'s file under
  `.cache/`) is found by exact name too, and a name that differs from it
  only in case (`2025-07-15.XML`) is refused (exit 1) with nothing stored
  (the chief of staff's ruling on the second review's decision 7): what a
  command reports writing, or finding unchanged, is the name git records,
  on every platform.
- **What `text.md` can't hold is refused (PR #9's reviews).** A section
  heading, a node's own text or a citation note (past one LF ending it)
  that holds a line break (CR, LF, U+0085, U+2028 or U+2029; the first
  review) or nothing but White_Space (the second), which would show as an
  empty line, refuses the source (exit 1), rather than being rewritten:
  `text.md` holds each text on one line of its own, verbatim, and ends with
  exactly one LF. None of the three stored raw XML files holds one (a test
  reads them).
- **Invalid UTF-8 is refused (PR #9's first review).** `Normalize` and
  `Hash` return an error for text that isn't valid UTF-8, rather than read
  a bad byte as U+FFFD, so two different byte strings never hash alike
  that way. The parser already refuses such XML; the functions are
  exported for steps 6 and 13.

Made while carrying it out, by the implementing agent, within the choices
and rulings above; each confirmed by the chief of staff (agent):
- **`Document`, `Runs` and `Join`.** `Document`, built from a tree
  (`NewDocument`, `Nodes`), holds its nodes in document order, each before
  its children: the one notion of that order and of anchors consecutive in
  it. `Runs` splits a cite into its runs and `Join` gives one string per
  run, both in document order whatever order the cite gives them in. A node
  is in the document by identity: a node of another tree at the same path
  is not. The refusals are `ErrEmptyCite`, `ErrNilAnchor`,
  `ErrForeignAnchor` and `ErrRepeatedAnchor`.
- **`Update` writes the bytes its set returns.** The command's set returns
  the derived files' bytes, and `Update`, under the source's lock, first
  checks that `tree.json` and `text.md` are each missing or a regular file
  (one it can't read is an error, never read as differing), then replaces
  `tree.json`, `text.md` and `meta.yaml` (what `meta.Marshal` renders) in
  turn, each only when its bytes differ. No writable handle is handed out.
- **`l0.RealDirs` refuses a path with a "." or ".." element, an empty one
  or a slash at either end,** for every caller (`fetch`, `StoreOnce`,
  `Read`, `Update`).
- **The source table's `Goldens`** names, for a source without its
  goldens, the plan step that adds them: plan step 9 for 1024.30 and
  1024.31. Sections are named, never defaulted: no section exits 2.
- **`Operative` returns an error** for a node whose own text doesn't start
  with its label, and one space and its heading if it has one, or isn't
  valid UTF-8, rather than guess.
- **`text.md`'s last LF** is the citation note's own (its character data
  ends in one in 1024.41), or one added when it has none.

Made in PR #9's second review, by the implementing agent, within its
rulings; each confirmed by the chief of staff (agent), as were the spec
wording below and `RealDirs` reading each directory on the way (one it
can't read is an error, exit 2):
- **The space that ends a heading that itself ends in White_Space.**
  Normalized as one string, the heading's trailing White_Space and any
  White_Space after it are one space, and that space is the heading's own:
  one `-` after it is still the separator. So with the heading "Head "
  (its space inside the `<I>`), the own text "(a) Head —" is
  heading-only, and so is "(a) Head  —", with a space or a no-break space
  after the heading's, which normalizes alike; with the heading "Head",
  "(a) Head —" leaves "-", and is operative, while "(a) Head—" is
  heading-only. Text that NFC joins with the heading's last character (a
  combining mark) is text. Nothing else is taken away: two dashes after a
  heading leave one, a dash and a space leave the space, and "(a) ", a
  node with no heading, is operative. None of these is in 1024.41.
- **The class of empty lines.** A section heading, a node's own text and a
  citation note are each refused when empty or nothing but White_Space,
  the citation note past its one LF; an empty `<CITA></CITA>` reads as no
  citation note, and shows no line.
- **A case variant beside the exact name is refused too,** on a
  case-sensitive filesystem, as `Read` refuses one for a source's files, so
  the check is tested on every filesystem. The refusal's message, and
  RealDirs' for a symlink or a file, no longer says nothing was written:
  `replace` wraps them as a write error after an earlier file may be
  written.

## Spec changes
- `docs/03-spec.md`:
  - Decisions-table row 13 is added, this line exactly (its link is
    relative to `docs/`):

    ```
    | 13 | How is text normalized, which anchors are operative, how does a multi-anchor cite join, and what do the hashes cover? | Normalized text is NFC, then the curly quotes U+2018, U+2019, U+201C and U+201D mapped to ASCII and the en and em dashes to `-`, then every run of Unicode White_Space collapsed to one space, never trimmed. An anchor is operative when its own text, minus its label, the one space before its heading, the heading, and the one dash or space that ends the heading (one `-` or one space, read on the whole own text normalized as one string), is non-empty. A multi-anchor cite joins own texts in document order with nothing between two in one `<P>` and one space between `<P>`s, and is normalized after joining. A node's `sha256` covers its whole normalized own text, a heading's separator included; `sha256_text` covers the section heading, one space and every anchor joined, normalized as one string, not the citation note; for the Supplement I 41() slice it is open until plan step 8 (the owner's calls; the normalization detail, the separator read normalized, `text.md`'s form, and the offline `regcorpus parse` that writes `tree.json`, `text.md` and `sha256_text`: the chief of staff's choices). [ADR 0013](adr/0013-cites-join-as-the-document-reads.md) |
    ```
  - Definitions, "Operative anchor": "an anchor whose own text, minus label
    and heading, is non-empty." → "an anchor whose own text, minus its
    label, the one space before its heading, the heading, and the one dash
    or space that ends the heading, is non-empty; it is read on the whole
    own text normalized as one string, where that separator is one `-` or
    one space after the normalized heading (decision 13)."
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
    them, normalized as one string, without the citation note. For the
    Supplement I 41() slice, what `sha256_text` covers is open until plan
    step 8."
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
    generated from `tree.json`: the section as it reads, verbatim, with no
    markup but the "# " that starts its heading line."; and after the list,
    a paragraph is added: "`regcorpus parse <section>` writes `tree.json`
    and `text.md` and sets `sha256_text`, offline, from the stored raw XML
    (plan step 5). It writes a source's files only once its parser goldens
    are in: today 1024.41's. It refuses 1024.30 and 1024.31 (exit 1, nothing
    written) until plan step 9."
  - Check that ends it, the `make test` item, gains: "From plan step 5,
    exit 0 also means 1024.41's committed `tree.json`, `text.md` and
    `sha256_text` are what `regcorpus parse` makes from its raw XML."
  - Who may run it: "An agent may write parser code and goldens, and run
    the checks above." → "An agent may write parser code and goldens, run
    the checks above, and run `regcorpus parse`, which is offline and
    derives every file from the committed raw XML; a person reviews its
    output in the PR diff."
- `docs/adr/README.md`, the index, gains the line "| [0013](0013-cites-join-as-the-document-reads.md) | A cite joins as the document reads; a heading's separator is not operative text | Accepted |".
- `docs/adr/0010-one-writer-owns-meta-yaml.md` gains, after its "Decided
  by" line, this header line exactly (docs/adr/README.md, Flow: an ADR that
  changes what an older one says amends its header):

  ```
  - **Amended:** 2026-11-24 by ADR 0013, which made plan step 5's choice this left open. The Decision's "Step 5's writer, `internal/anchors`, needs a source too: whether the table is exported or moved is step 5's choice." describes the plan before step 5: the table stays unexported in `internal/ecfr`, whose `regcorpus parse` passes each `l0.Source` to `internal/anchors`.
  ```
- `docs/adr/0012-labels-count-in-the-paragraph-lead.md` gains, after its
  "Decided by" line, this header line exactly:

  ```
  - **Amended:** 2026-11-24 by ADR 0013, which settled what this carried forward to plan step 5. The Decision's "Whether it counts in the operative-anchor test and the per-node hash, and how a multi-anchor cite joins across it or across a paragraph boundary, is plan step 5's (spec "Carried forward")." and the Consequences line "Plan step 5 inherits the dash or space at the end of a heading parent's own text (spec "Carried forward")." describe the spec before step 5: the separator doesn't count in the operative-anchor test and does count in the per-node hash, a multi-anchor cite joins own texts with nothing between two in one `<P>` and one space between `<P>`s (spec decision 13), and the "Carried forward" row is removed.
  ```
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
  which an independent computation,
  [`internal/anchors/testdata/oracle.py`](../../internal/anchors/testdata/oracle.py)
  (Python's `unicodedata` NFC, the raw XML read without the Go parser),
  also gives, as it gives the hashes of `b`, `c-2-v` and `c-2-v-A-1`;
  `make test` pins its output. `make test` also checks that the committed
  files are what `regcorpus parse` makes from the raw XML.
- 75 of 1024.41's 91 nodes are operative; the 15 heading-only lead-ins and
  (c)(4)(ii)(A) are not. So the inventory (step 14) lists 75 operative
  anchors of 1024.41's rule text; its 41() comments are added in plan step
  8.
- An upstream change to a heading's separator alone: where a child's label
  follows it, anything but one "—" or one space ("—" to ". ", say) fails
  the parse (ADR 0012). At the end of a `<P>`, "—" to "–", or " " to a
  no-break space or to two spaces, changes neither the node's hash nor
  whether it is operative, since normalization maps them alike; "—" to ".
  " changes the hash and makes the node operative, as ". " is text.
- A quote copied from the regulation across "—(1)" or "(A)(1)" matches its
  cite's join; across two `<P>`s it matches with one space. A quote across
  anchors that aren't consecutive matches nothing, since they are joined
  in separate runs.
- `sha256_text` doesn't change when only the citation note does.
- `go.mod` requires `golang.org/x/text`. Until the owner's gate-test PR
  merges to main, `.githooks/pre-commit_test.sh` and `.github/c6_test.sh`
  fail on this branch, since they copy `go.mod` without `go.sum`. The
  pre-commit hook builds main's checker, so commits are unaffected; but CI
  runs `make test` too, so the PR's "C6 gate" check stays red until the
  owner's gate-test PR lands and main is merged into this branch.
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
- PR #9 (https://github.com/Okja-Engineering/reg-corpus/pull/9), which
  records this ADR; its comment quoting the owner's calls and every option
  offered (https://github.com/Okja-Engineering/reg-corpus/pull/9#issuecomment-7145434918).
- Commits `e7359a6` (the tests against stubs, red), `0837bda`
  (`internal/anchors`, `l0.Derive`, `regcorpus parse` and 1024.41's files,
  green), `2e8806b` (a row from the mutant pass), `08ec51e` (this ADR and
  its amendments), `1d75130` (a comment); and from PR #9's first review,
  `4da7089` (the tests against stubs, red), `b7174d5` (`Document`, `Runs`
  and `Join` by run; `Update` the only writer, `Derive` and `Replace`
  gone; exact names; the separator read normalized; the refusals; the
  sources parse writes; green), `ae69a02` (escapes for invisible characters
  in Go source), `8e03279` (this ADR, the spec and the stage 01 contract
  revised); and from PR #9's second review, `88678e4` (the tests, red),
  `446aaf0` (the operative verdict on the normalized whole own text, exact
  names for directories, errors naming their file, no empty line in
  `text.md`; green), `4d4b454` (this ADR, the spec and ADRs 0010 and 0012
  revised), `6a493bb` (a comment), `5c5957d` (the file `StoreOnce` stores
  found by exact name, red), `82fdd55` (the same, green), and the commit
  that records it here.
- Files: `internal/anchors/normalize.go` (`Normalize`, `Hash`),
  `internal/anchors/anchors.go` (`ID`, `Operative`, `Document`, `Runs`,
  `Join`), `internal/anchors/build.go` (`Build`, `EncodeTree`, `TextMD`,
  `SectionText`, `SHA256Text`), `internal/anchors/write.go` (`Write`),
  `internal/anchors/testdata/oracle.py`, `internal/l0/source.go` (`Read`,
  `Update`), `internal/l0/store.go` (`RealDirs`, `StoreOnce`, `replace`),
  `internal/ecfr/parsecmd.go`, `internal/ecfr/sources.go`,
  `internal/ecfr/metaset.go`, `internal/ecfr/fetch.go`,
  `internal/parse/tree.go`, `cmd/regcorpus/main.go`, `.gitignore`,
  `sources/ecfr/title-12/1024.41/{tree.json,text.md,meta.yaml}`, the
  package docs named above, `docs/adr/0010-one-writer-owns-meta-yaml.md`
  and `docs/adr/0012-labels-count-in-the-paragraph-lead.md`, and the tests
  `internal/anchors/*_test.go`, `internal/l0/update_test.go`,
  `internal/l0/update_unix_test.go`, `internal/l0/update_darwin_test.go`,
  `internal/l0/store_test.go`, `internal/ecfr/parsecmd_test.go`,
  `internal/ecfr/parsecmd_darwin_test.go`, `internal/ecfr/store_unix_test.go`,
  `internal/ecfr/names_test.go`, `internal/parse/tree_test.go` and
  `cmd/regcorpus/main_test.go`.
- ADR 0012, "Where the dash or space after a heading goes"; ADR 0010, "Where
  the source table lives".
