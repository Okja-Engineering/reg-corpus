# 0015. `regcorpus text` prints one anchor's own text, its children's IDs and its source's `meta.yaml`

- **Status:** Accepted
- **Date:** 2026-11-26
- **Decided by:** the owner, 2026-11-26 about 10:23 CST, in chat (answers to the chief of staff's questions), quoted on PR #11 (https://github.com/Okja-Engineering/reg-corpus/pull/11#issuecomment-7198137757): (F1) "Own text + child IDs (Recommended)"; (F2) "All of meta.yaml + sha256 (Recommended)"; (F3) "Exact IDs only (Recommended)"; (F8) "ADR 0015 + acceptance (Recommended)". Each call is only what its option text says (Options considered, below). The rest are agents' choices, not the owner's: the chief of staff's rulings (F4, F5, F6, F7; the step's mandate's assumptions 4, 5 and 9 and its definition of done, which fix the output's form: verbatim, in document order, offline, the plain and JSON shapes; and the round-1 review rulings, §1 of which replaced F6's I2 with I3) and the implementer's choices, each marked below.

## Context
Plan step 7 builds R8's `text` command:

> 7. `regcorpus text <anchor>`.

R8 said:

> - `regcorpus text <anchor>` prints one anchor's text and its source metadata.
> - …
>
> Output is plain text by default, with `--json` for tools. No command prints more than the records asked for.

(the "…" marks the `topics` line, cut here)

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
The chief of staff asked the owner four questions. Every question and
every option offered, quoted as offered (PR #11's comment):
- F1, "What prints": "Step 7, `regcorpus text <anchor>`: besides the
  anchor's own verbatim text, what should it print? Example: 1024-41-b-2's
  own text is only its heading \"(2) Review of loss mitigation application
  submission—\", so without more a reader must guess the child IDs."
  - **"Own text + child IDs (Recommended)"**: "Prints the anchor's own text
    plus its children's anchor IDs, with no child text. A reader can walk
    the tree, and no extra rule text is printed." Chosen.
  - **"Own text only"**: "The literal minimum. A reader can find children
    only by guessing their IDs."
  - **"Own text + whole subtree"**: "Reads like the document, but prints
    more than one anchor, and the printed text no longer hashes to the
    printed sha256."
  - **"Child IDs, plus --subtree"**: "Like the first option, plus a
    --subtree flag that prints each descendant's own text under its ID."
- F2, "Metadata": "Step 7, `regcorpus text`: which source metadata should
  print with the anchor text?"
  - **"All of meta.yaml + sha256 (Recommended)"**: "Every field in the
    source's meta.yaml, as stored, plus the node's own sha256. Nothing is
    hidden, and no one has to decide what counts as provenance." Chosen.
  - **"A chosen subset"**: "cfr, tier, role, the version dates and
    verified_on, plus the node's sha256. Shorter."
- F3, "Anchor form": "Step 7, `regcorpus text`: which anchor forms should
  it accept?"
  - **"Exact IDs only (Recommended)"**: "For example `1024-41-b-2-i-B`,
    matched exactly and case-sensitively. It is the spec's one form, and B
    and b are different levels." Chosen.
  - **"Also the CFR form"**: "Also accepts `1024.41(b)(2)(i)(B)` and prints
    the canonical ID. Friendlier, but it is a second syntax to specify and
    test."
  - **"Case-insensitive IDs"**: "Easier to type, but it hides a wrong level
    (B vs b)."
- F8, "Paperwork": "Step 7: R8's acceptance list tests `show` and `topics`
  but not `text`. Should step 7 record your calls in a short ADR 0015 that
  adds a `text` acceptance item to the spec?"
  - **"ADR 0015 + acceptance (Recommended)"**: "Records your calls and adds
    this acceptance item: `regcorpus text 1024-41-b-2-i-B` prints that
    anchor's own text and its source metadata; an unknown anchor exits
    non-zero." Chosen.
  - **"No ADR"**: "Treat the calls as already within R8. The acceptance gap
    stays open."

Ruled by the chief of staff, as the step's mandate drafted them, each
following what `parse` and `fetch` already do:
- F4, exit codes: within the CLI's 0/1/2 (chosen), or every miss 1, or a
  miss listing the anchors that exist (91 IDs, more than asked for).
- F5, sources not built yet: refuse with the plan step that builds them
  (chosen), or print a section's `meta.yaml` alone (not an anchor).
- F6, checks before printing: I2, check what it reads (chosen, then
  replaced on review, round 1, §1); I1, trust `tree.json`; I3, I2 plus the
  raw XML's hash and a rebuild compared through `State.Stale` (heavier, and
  a read would depend on the parser; chosen in round 1).
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
  it (an unknown label, another case, a section's own ID such as
  `1024-41`); its source's anchors are a later step's (F5); or a stored
  file refuses (F6). Exit 2: usage (no anchor, more than one, an unknown
  flag); a string that names no source (`1024.41(b)`, `1024-40-a`), as
  `parse` treats an unknown section; not the corpus root; a file that
  can't be read. Nothing goes to stdout on an error. What a miss names,
  as the messages say it (reworded in round 1, §5; F4 as drafted said a
  miss "names the anchor and its source and lists nothing else", which
  over-claimed): no such anchor names the ID and its source ("no anchor
  "1024-41-z" in 1024.41"), with the reason for a section's own ID and
  an `interp` segment where no comments are; one a later step builds
  names the ID and the plan step; a string that names no source names
  it and lists the section IDs it could start with (`1024-41, 1024-30,
  1024-31`) and an example, as `parse` lists the known sections.
- **F5.** 1024.30's and 1024.31's anchors exit 1, "anchors for 1024.31 are
  plan step 9's", from the source table's `Goldens`; an anchor with an
  `interp` segment under 1024.41 exits 1, "Supplement I comment anchors
  are plan step 8's" (under 1024.30 and 1024.31 it is no anchor: round 1,
  §5).
- **F6 (I2, replaced in round 1 by I3, below).** As first ruled, before
  printing, it checked only what it read: `l0.Read` (exact names, case
  variants refused, a strict `meta.yaml`); the `meta.yaml` names the
  source as the source table does; `tree.json` decodes strictly and its
  `section` is the source's; the anchor's `sha256` is `anchors.Hash` of
  its own text; and `anchors.SHA256Text` of the tree is `meta.yaml`'s
  `sha256_text`. It read no raw XML or `text.md`. It never reads
  `.cache/`, `registry/` or `staging/`, or anything from the network.
- **F7 (A).** `ecfr.TextMain` is a thin handler beside `parse`'s; it
  passes the source table, each source's name, `l0.Source` and `Goldens`,
  to `internal/lookup`, which holds the resolver and the printers and
  imports `internal/l0`, `internal/meta`, `internal/parse`,
  `internal/anchors` and `internal/sink`, never `internal/ecfr`; a test
  proves `go list -deps` of it holds no `net/http`.
- **The output** (the mandate's assumptions 4, verbatim, 5, offline, and
  9, through the sink, and its definition of done, the plain and JSON
  shapes; that assumption 5 reads no raw XML and no `text.md` is replaced
  by round 1's §1, below). Plain, a line each:
  the anchor ID; its own text, verbatim, as `parse` derives it and
  `tree.json` holds it (never normalized or wrapped); an empty line; `sha256: ` and the node's
  `sha256`; `children: ` and its children's IDs in document order,
  space-separated, or `none`; `meta.yaml: ` and the source's `meta.yaml`
  path; then each line of that `meta.yaml` as `meta.Marshal` renders it,
  which is the stored file's bytes. `--json`: one line, a JSON object with
  the keys `anchor`, `text`, `sha256`, `children` (an array), `meta_path`
  and `meta` (an object keyed by the `meta.yaml` field names, in schema
  order, absent fields left out, `verified_on` null when null). Every line
  goes through the sink, one Write per line; `--json` is one Write.

On review of PR #11 (round 1, head 420c5bc), the chief of staff ruled, from
its reconciled worklist of four hunters' reports:
- **§1, I3 replaces F6's I2.** An anchor ID resolves to exactly the own text
  `parse` derives for that label path from the stored raw XML, byte for
  byte, and the `meta.yaml` printed describes that text. Why: I2's checks
  bound only normalized hashes of the decoded tree, not the label paths
  (the IDs), the tree's shape (the children line), the verbatim bytes
  past normalization or the JSON encoding; so a hand-edited `tree.json`
  put another anchor's text, or a text not verbatim, under a real ID with
  exit 0 and verified provenance beside it. C6 (`check --corpus`) doesn't
  catch that, and steps 13, 14, 15 and 19 reuse the resolver. So the
  resolver derives the source again as `fetch --verify`'s first check does
  (ADR 0014): `l0.Read`; `l0.State.Stored` (`meta.yaml` names the source;
  `raw/` holds exactly the raw file, its sha256 `sha256_raw`); no
  `overrides.yaml`; `anchors.Build` of the raw XML; and `tree.json`,
  `text.md` and `sha256_text` must be what it builds (`l0.State.Stale`,
  through `anchors.Derived.Stale`). Any failure exits 1, nothing printed.
  The ad-hoc tree checks this makes redundant go; one stays (below).
- **§2, the resolver owns the root check.** `Corpus.Anchor` checks the
  corpus root (`l0.CorpusRoot`); "not the corpus root" is the typed
  `lookup.ErrNotRoot` (exit 2), so the resolver's errors mean the same to
  every caller; `TextMain` keeps no copy.
- **§3, exit 0 means the output was written.** The sink keeps its first
  write error, and `run` turns a stdout write failure into exit 2 with a
  stderr line, for every command and help.
- **§4, one JSON line through the sink.** `internal/sink` writes a value as
  one JSON line, through the sink, that decodes to the value; `text
  --json` uses it, so `lookup` no longer knows the sink's escape table.
- **§5, what misses name.** An `interp` segment names plan step 8 only
  under 1024.41 (step 8 slices out only the 41() comments); under 1024.30
  or 1024.31 it is no anchor. A section's own ID, such as `1024-31`, is no
  anchor under any source. The claims about messages are reworded to what
  the messages say (F4, above, and `-h`), not the messages to the claims.
- The record's gaps (§6 to §8: tests that kill three surviving mutants,
  `-h` and the usage's exit codes, and this ADR's credits, quotes and
  links) are fixed with no change of ruling.

The implementer's choices (an agent's), each with its reason:
- **The resolver is `lookup.Corpus.Anchor`.** A `lookup.Corpus` is the
  corpus root, the directory every source is under, and the source table
  as `lookup.Source`s (`Name`, `Stored`, `Pending`, the table's `Goldens`,
  and `Comments`, the table's new field naming the step that builds a
  source's Supplement I comment anchors, set for 1024.41 alone, which
  carries §5); `ecfr`'s `corpus` builds it. Why: steps 13, 14, 15 and 19
  resolve anchors too, and the table stays where ADR 0013 put it.
  `lookup.ErrNotRoot` (round 1: it is `l0.ErrNotRoot`, which
  `l0.CorpusRoot`'s error wraps, so `ecfr`'s commands share it),
  `ErrNoSource`, `ErrNotBuilt` and `ErrNoAnchor` name the misses for later
  callers (`errors.Is`); a stored file's refusal stays an `*l0.Conflict`.
- **Which source an ID names.** A source's section ID is `anchors.ID` of
  its name and no path (`1024-41`); an ID names the source whose section ID
  it is, or starts with followed by `-`. So `1024-410-a` names none (exit
  2), and `1024-41-a-` names 1024.41 and no anchor in it (exit 1). The
  checks run in this order: usage; the corpus root (`l0.CorpusRoot`, in
  the resolver since round 1, so outside it every anchor exits 2, as
  `parse` checks the root before a source's goldens); the source; then,
  by the ID alone, before any file is read, the section's own ID (no
  anchor), an `interp` segment (step 8 under 1024.41, else no anchor) and
  `Pending` (step 9); then the stored files, in the order `fetch --verify`
  checks them (`l0.Read`, `Stored`, no `overrides.yaml`, `anchors.Build`,
  `Derived.Stale`), then the raw XML's section; then the anchor in the
  derived tree. So `1024-99-a-interp-1` names no source (exit 2), and a
  source whose stored files refuse refuses an unknown ID too.
- **Which of I2's checks stay (round 1).** One: the derived tree's
  `section` is the source's. I3 doesn't bind the raw XML to the source: a
  section's raw XML with its derived files and hashes stored under another
  source's directory is consistent with itself, so `text` checks the
  section `anchors.Build` reads from the raw XML. The rest go, since every
  file they guarded is now compared byte for byte with what `Build`
  derives: strict decoding, the null-node, empty-path and duplicate-ID
  checks (`parse` refuses repeated labels, so a derived tree's IDs are
  unique), the per-node `sha256` check and the `sha256_text`-only check
  (`Derived.Stale` compares `sha256_text`).
- **Which refusals say to run `parse`.** A derived file that is missing or
  not what `Build` derives (`tree.json`, `text.md`, `sha256_text`) says
  "run regcorpus parse <section> to regenerate them"; a raw XML that
  `parse` builds nothing from says so, with `parse`'s error. A missing
  `meta.yaml` says to run `regcorpus fetch`, and a missing raw file to
  restore it from git; a raw file not `sha256_raw`'s or not alone, an
  `overrides.yaml`, a file that is not regular, a name in another case and
  another section's raw XML say what is wrong and name no command, as
  `fetch --verify` and `parse` say them.
- **`anchors.Derived.Stale` is the one comparison (round 1).** It is
  `l0.State.Stale` of `tree.json` and `text.md` plus `sha256_text`, and
  `ecfr`'s `fetch --verify` (`storedVersion`) now calls it too, with its
  messages unchanged. `anchors.Build` stays each caller's, so each words
  its own refusal. Why: §1 asks for no second copy, and the two readers
  can't drift.
- **No lock.** `text` writes nothing, and every writer replaces a whole
  file by rename, so a file read is never half written; one replaced
  between two reads differs from what `Build` derives, and is refused.
- **`meta`'s JSON is `internal/meta`'s.** `meta.JSON` writes the fields
  `meta.Marshal` writes, keyed and ordered as there, each value as its
  type is: a string, `official` a boolean, `version_fr_citations` an array,
  `verified_on` a string or null. Marshal and JSON share one walk of the
  schema, and JSON refuses what Marshal refuses. Why: `internal/meta` owns
  the schema (ADR 0010); a second list of the fields in `internal/lookup`
  would drift.
- **DEL in `--json`, now `sink.JSON`'s (§4).** `encoding/json` escapes
  every C0 control, U+2028 and U+2029, but not DEL (U+007F), which the
  sink would write as a Go escape that is no JSON escape; so `sink.JSON`
  writes DEL as its JSON escape before the line reaches the sink. A C1
  control is left to the sink, whose escape of it is that character's
  JSON escape. So the line the stream holds is JSON that decodes to the
  value; `sink`'s tests pin it over the output hunter's characters (C0,
  DEL, C1, U+2028, U+2029, `<>&` and the rest). `sink.JSON` returns only
  an encoding error: the write's error is the sink's, which `run` reads
  (§3), so a caller checks nothing more. In plain output the sink escapes
  a control character, U+2028 or U+2029 in a text, on its line.
- **`internal/l0` gains what a reader needs, and nothing is copied.**
  `State.Names` is the identity check `Stored` makes first, split out (its
  messages now speak of reading too); `CorpusRoot` is `ecfr`'s `checkRoot`
  moved, which `checkRoot` now calls, and since round 1 its "not the
  corpus root" error wraps `l0.ErrNotRoot`. `State.TreeJSON`, which the
  first resolver read `tree.json` with, is gone in round 1: nothing calls
  it once `text` derives the tree. `ecfr`'s unexported `lookup` is renamed
  `sourceNamed`, since the `internal/lookup` import takes the name.
- **A stdout that can't be written (round 1, §3).** `sink.Writer` keeps its
  first write error (`Err`), and `run` returns 2 with "regcorpus: writing
  stdout: ..." whenever stdout's sink has one, whatever the command's own
  code, since its output is not what it says. The tests use a stdout open
  read-only: Go's runtime opens `/dev/null` on a closed fd 1 at start, so
  `>&-` can't make a write fail. A broken pipe still ends the process by
  SIGPIPE, as before.
- **Test fixtures are built from raw XML (round 1).** Each `text` fixture
  is a raw XML file and what `anchors.Build` derives from it; a forgery is
  a stored file edited after, with its hashes recomputed where the class
  needs it. So the payload rows of `cmd/regcorpus`'s sink test hold a tab,
  DEL and a C1 control, characters raw XML can carry: `regcorpus parse`
  (`anchors.Build`) refuses a line break in a text (LF, CR, U+0085,
  U+2028, U+2029) and XML holds no other C0 control, so LF, ESC and U+2028
  can no longer reach `text`'s stdout, and `sink`'s own tests cover them.
- **Flags before the anchor**, as `fetch` takes them before sections:
  `text 1024-41-a --json` is a usage error (two arguments). `--json=false`
  is taken, the last of two `--json` flags wins, and `-h` and `--help`
  exit 0: the stdlib `flag` package's behaviour, which `fetch` shares and
  `-h` says since round 1 (§7).

## Spec changes
- `docs/03-spec.md`:
  - Decisions-table row 15 is added, this line exactly (its link is
    relative to `docs/`):

    ```
    | 15 | What does `regcorpus text` print, and which anchor forms does it read? | One anchor's own text, verbatim, never normalized or wrapped; its `sha256`; its children's anchor IDs in document order, never their text; and every field of its source's `meta.yaml` as stored (`--json`: the same, as one JSON object). It reads anchor IDs only, matched exactly and case-sensitively, and resolves one only to the own text `regcorpus parse` derives for it from the source's stored raw XML: offline, it reads the source's `meta.yaml`, raw XML, `tree.json` and `text.md` and derives them again as `fetch --verify` does, so it prints nothing unless the raw XML is `sha256_raw`'s and `tree.json`, `text.md` and `sha256_text` are what it derives. A miss (among them a section's own ID, and an `interp` segment under 1024.30 or 1024.31), an anchor a later plan step builds (1024.30's and 1024.31's, step 9; Supplement I's comments on 1024.41, step 8) and a stored file that refuses exit 1; usage, a string that names no source, not the corpus root and a stdout that can't be written exit 2 (the owner's calls: the children's IDs, all of `meta.yaml`, exact IDs and the acceptance item; the output's form (verbatim, document order, offline, the plain and JSON shapes), the exit codes, the refusals, the checks, deriving the source again (round 1) and where the code lives: the chief of staff's rulings). [ADR 0015](adr/0015-text-prints-own-text-and-child-ids.md) |
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
    the source's stored files, derived again from its raw XML as `fetch
    --verify` does; exact, case-sensitive anchor IDs. 1024.41's anchors
    resolve; 1024.30's and 1024.31's are step 9's, Supplement I's step
    8's."
  - Proof, R8: the command cell "`regcorpus show RX-41-ACK-01`; `regcorpus
    topics acknowledgment`; `regcorpus topics xyz`" gains "; `regcorpus
    text 1024-41-b-2-i-B`; `regcorpus text 1024-41-z`", and the evidence
    cell "…; the unknown term exits non-zero" gains "; the anchor's own
    text and its source metadata print; the unknown anchor exits
    non-zero".
- `docs/adr/README.md`, the index, gains the line "| [0015](0015-text-prints-own-text-and-child-ids.md) | `regcorpus text` prints one anchor's own text, its children's IDs and its source's `meta.yaml` | Accepted |".
- Package docs: `internal/lookup/doc.go` says what is built;
  `internal/ecfr/doc.go` (its header names `text`, R8, plan step 7 and
  `textcmd.go`), `internal/l0/doc.go`, `internal/meta/doc.go`,
  `internal/anchors/doc.go` and `internal/sink/doc.go` name `text`'s
  handler and the source table's `Comments`, `Names`, `CorpusRoot` and
  `ErrNotRoot`, `JSON`, `Derived.Stale`, and `Writer.Err` and `sink.JSON`.
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
  (step 19). Each gets an anchor's text only as `parse` derives it from
  the stored raw XML (round 1, §1).
- A call costs about 5 ms in-process for 1024.41 (about half of it
  `anchors.Build`), where reading `tree.json` alone (I2) cost about 2 ms;
  a process's wall time, about 10 to 15 ms, is mostly its start either way.
- A hand edit to `tree.json`, `text.md` or `sha256_text`, or a raw XML
  edited without them, makes every anchor of that source refuse until
  `regcorpus parse` regenerates them (or `fetch` stores a new version);
  `text` never prints around the stored files.
- An `overrides.yaml` makes `text` refuse, as `parse` and `fetch --verify`
  do, until applying overrides is built (R1a).
- Deferred by the chief of staff (round 1), no change: in plain output a
  text holding a backslash sequence and one holding the control character
  the sink escapes to that sequence print alike. That is the sink's design
  since ADR 0014; `-h` says so, and `--json` is the unambiguous form.
  1024.41's stored `text.md` holds no backslash and nothing the sink
  escapes. And no size cap on what `text` reads: no local reader in the
  repo is capped, and the hook and guard are seatbelts, not defences
  against attackers.

## Requirement IDs
R8 (lookup; its acceptance item); spec "Definitions" (Anchor, Own text);
decisions 6 (CLI only), 10 (`internal/meta` owns `meta.yaml`'s schema),
13 (own text, the join, what the hashes cover) and 15.

## Links
- This ADR's PR, PR #11 (https://github.com/Okja-Engineering/reg-corpus/pull/11),
  with the comment quoting the owner's calls and every option offered
  (https://github.com/Okja-Engineering/reg-corpus/pull/11#issuecomment-7198137757).
- Commits: `ba6da2d` (the tests against stubs, red); `043273c`
  (`internal/lookup`, `ecfr`'s `TextMain`, `meta.JSON` and `l0`'s
  `Names`, `TreeJSON` and `CorpusRoot`; green); `420c5bc` (this ADR and
  its amendments, record); review round 1: `36c60e4` (the tests for the
  reconciled worklist, red), `f04c5ec` (its fixes, green), and the commit
  that records round 1 here and in the spec.
- Files:
  - Code: `internal/lookup/anchor.go` and `text.go` (new),
    `internal/ecfr/textcmd.go` (new), `internal/ecfr/fetch.go`
    (`checkRoot` calls `l0.CorpusRoot`), `internal/ecfr/sources.go`
    (`sourceNamed`; in round 1, `Comments`), `internal/ecfr/metaset.go`,
    `internal/l0/source.go`, `internal/meta/meta.go`,
    `cmd/regcorpus/main.go`; in round 1, `internal/anchors/write.go`
    (`Derived.Stale`), `internal/ecfr/verify.go` (calls it) and
    `internal/sink/sink.go` (`Writer.Err`, `JSON`).
  - Tests: `internal/lookup/text_test.go`, `internal/ecfr/textcmd_test.go`,
    `internal/l0/read_test.go` (new), `internal/meta/meta_test.go`,
    `internal/ecfr/client_test.go` and `granule_test.go` (`sourceNamed`),
    `cmd/regcorpus/main_test.go`; in round 1,
    `internal/anchors/write_test.go` and `internal/sink/sink_test.go`.
  - Package docs: `internal/lookup/doc.go`, `internal/ecfr/doc.go`,
    `internal/l0/doc.go`, `internal/meta/doc.go`; in round 1,
    `internal/anchors/doc.go` and `internal/sink/doc.go`.
  - Spec docs: `docs/03-spec.md`, `docs/04-plan.md`, `docs/adr/README.md`
    and this ADR.
- ADR 0013, which fixed own text, the per-node `sha256`, `sha256_text` and
  `anchors.ID`; ADR 0010, the `meta.yaml` schema `meta.JSON` renders.
