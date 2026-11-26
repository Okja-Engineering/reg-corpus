# 0014. Normalized text is the version; govinfo is compared exactly, its layout deleted

- **Status:** Accepted
- **Date:** 2026-11-25
- **Decided by:** the owner, 2026-11-25 about 19:29 CST, in chat (answers to the chief of staff's questions), quoted on PR #10 (https://github.com/Okja-Engineering/reg-corpus/pull/10#issuecomment-7180640566): (F1) "Normalized text (Recommended)"; (F3) "Exact, with layout rule (Recommended)"; (F5) "First matching run (Recommended)"; (F7) "Report only (Recommended)". Each call is only what its option text says, read with the correction that comment records (Decision, below). The rest are agents' choices, not the owner's: the chief of staff's rulings (F2, F4, F6, F8, F9, F10, the step's mandate's assumptions 1–11, the bound of 12, and the round-1, round-2, round-3 and round-4 review rulings) and the implementer's choices, each marked below.

## Context
Plan step 6 builds `regcorpus fetch --verify` (R1) and `regcorpus fetch
--latest` (R1b):

> 6. `regcorpus fetch --verify`: compare the version date, tree and `sha256_text`, recording `sha256_raw` as information only. Then compare the normalized text to the govinfo CFR 2026 edition and set `verified_on`. Add `regcorpus fetch --latest` (R1b).

The spec's acceptance line and `meta.yaml` comment:

> - [ ] **R1:** `regcorpus fetch --verify` exits 0: the version date, tree and `sha256_text` match a fresh fetch (`sha256_raw` is informational). `verified_on` is set by comparing normalized text to the govinfo 2026 edition. The output is saved.

> verified_on: <YYYY-MM-DD or null>  # normalized text matched the govinfo annual edition; null on mismatch, diff recorded

None of the spec, the plan or an ADR said what makes a re-served copy "the
same version", how govinfo's text is extracted, what date `verified_on`
holds, or what `--latest` does past exiting non-zero.

- **Reissues.** eCFR re-serves a published date with other bytes at times:
  its version list gives each entry an `issue_date`, and Supplement I's
  2023-03-20 entry was reissued with one dash changed. Plain `fetch` exits 1
  on stored bytes that differ (plan step 2), and `sha256_raw` is
  informational (spec R1).
- **govinfo's layout.** The CFR 2026 annual edition's granule for 1024.41
  (`CFR-2026-title12-vol8-sec1024-41.xml`, 34,935 bytes, sha256
  `147ace15c532251bddb55bd78360909bf4efe14923060c80e8edb356d117c391`) is
  pretty-printed: each element on its own line, indented four spaces a
  level, a `<P>`'s text broken across lines and indented twelve spaces, so
  a line that starts with a real space is indented thirteen. Under
  decision 13's normalization alone, 25 of 1024.41's 75 paragraphs equal
  eCFR's; the rest differ by 146 spaces the layout adds: 100 at a
  paragraph's edge, and 46 before a heading's dash or inside "( 1 )". No
  line break falls inside a word. ADR 0013's F3 option text told the owner "step 6
  can compute the same hash from govinfo's text for verified_on"; that
  holds only with a rule for this layout, as the owner was told in F3's
  question.
- **The granule's form.** CFRGRANULE holds FDSYS (its DATE, 2026-01-01)
  and one SECTION: SECTNO ("§", U+2009, "1024.41"), SUBJECT, the `<P>`s
  with `<E T="03">` for italics and empty `<PRTPAGE P="…"/>` page breaks,
  then the citation note, CITA.

## Options considered
The chief of staff asked the owner four questions. Every option offered,
quoted as offered:
- F1, "Reissue": "Step 6, reissue identity: eCFR sometimes re-serves an
  already-published date with slightly different bytes (Supplement I's
  2023-03-20 reissue changed one dash). What makes a re-served copy 'the
  same version'?"
  - **"Normalized text (Recommended)"**: "A version is its date plus its
    normalized text (tree paths, per-node hashes, sha256_text). Byte-only
    reissues pass --verify, with raw hashes printed as info; a text change
    fails --verify and names the nodes. --latest stays date-only but notes
    any reissue since fetch. Stage 04's release check gains fetch --verify.
    No schema change." Chosen.
  - **"Also refetch in --latest"**: "As the first option, plus --latest
    also refetches the stored date and fails if its normalized text
    changed. Catches text reissues at release with no extra step; amends
    R1b with a third case and costs a fetch+parse per source (1024.41 only
    until step 9)."
  - **"Date + issue_date"**: "--latest fails on any reissue of the stored
    date. Covers all sources with no parsing, but byte-only reissues fail
    too and keep failing unless issue_date becomes a 14th meta.yaml field
    (schema change)."
- F3, "govinfo": "Step 6, govinfo comparison: govinfo's 2026 XML is
  pretty-printed, so decision-13 normalization alone matches only 25 of
  1024.41's 75 paragraphs. ADR 0013's option text told you step 6 'can
  compute the same hash from govinfo's text' — that holds only with a
  layout rule. How should --verify compare to govinfo?"
  - **"Exact, with layout rule (Recommended)"**: "Delete each line break
    plus its indentation (layout), build the heading as SECTNO + space +
    SUBJECT, drop PRTPAGE and the CITA, then normalize: the hash must equal
    stored sha256_text. Matches exactly today for all three sections. A
    govinfo format change can only cause a false mismatch (null), never a
    false match; mismatches print each differing paragraph." Chosen.
  - **"Whitespace-blind"**: "Compare heading and paragraphs with all
    whitespace removed. Matches today and ignores layout, but can't see a
    space added or removed between words, so it's weaker than 'normalized
    text'."
- F5, "Date": "Step 6, verified_on: what date does it hold, and when does a
  re-run change it?"
  - **"First matching run (Recommended)"**: "The UTC date of the run that
    first matched; a later run that still matches changes nothing (no
    meta.yaml churn, like parse and meta set). A later mismatch overwrites
    it with null, as the spec says." Chosen.
  - **"Every matching run"**: "The UTC date of the most recent matching
    run; every re-run rewrites meta.yaml."
  - **"Edition date"**: "2026-01-01, the govinfo edition's date. Records
    which edition was used, but it's then not a 'verified on' date."
- F7, "--latest": "Step 6, --latest finds a newer eCFR version (the spec
  requires a non-zero exit). What else should it do?"
  - **"Report only (Recommended)"**: "Exit 1, print each section's stored
    date and eCFR's newer dates, write nothing. How a new version lands
    stays undecided (upgrade comes after accept)." Chosen.
  - **"Report and cache it"**: "Also fetch the newer XML into .cache/ (as
    fetch --date does) for a manual diff; --latest then writes files."

The chief of staff ruled on the rest, each the recommended default of the
step's mandate (agents' choices):
- F2, which govinfo document: G1, the per-section granule XML through
  govinfo's link service, year pinned to 2026; not the whole volume's bulk
  XML sliced by SECTNO (G2), nor `api.govinfo.gov`, which needs a key
  (G3). At run time it is kept only in the git-ignored `.cache/govinfo/`,
  a cache never committed, its URL and sha256 printed; it is not L0 and
  not stored under `sources/`. Apart from that, 1024.41's granule is
  committed as a test fixture under F9 (N1), and a later section's granule
  may be committed as testdata the same way.
- F4, a govinfo mismatch: (a) it overwrites an earlier date with `null`;
  (b) the diff is in the saved output only, not a new file beside the
  source; (c) the eCFR comparison runs first, and if it fails nothing is
  written and govinfo is not asked.
- F6, the eCFR side: (i) the fresh fetch is eCFR's latest version, as
  plain `fetch` would fetch it, not `full/{source_version_date}`; (T2) the
  tree matches when it has the same node paths in the same order, each with
  the same `sha256`, not when `tree.json`'s bytes are equal (T1, which a
  byte-only reissue fails).
- F8, exit codes: 0, 1 and 2 as the CLI uses them, and a govinfo mismatch
  exits 1 although it writes `null` (E-a), which `-h` says; not a new code
  3 (E-b). A latest eCFR version that removed the section is exit 1 for
  `--verify` and `--latest`, a finding; plain `fetch` keeps exit 2 for it.
- F9: tests use only fakes and committed testdata (N1), not an opt-in or a
  CI live test; the live output is quoted in the PR body.
- F10: `--verify` covers 1024.41 only and refuses 1024.30 and 1024.31 until
  plan step 9 gives them a tree and `sha256_text`; `--latest` covers all
  three stored sections.

## Decision
The owner decided, each call only what its option text says:
1. (F1) A version is its date plus its normalized text: its tree's node
   paths, their per-node `sha256` and `sha256_text`. `fetch --verify` passes
   a re-serve whose bytes differ but normalize alike, and prints both
   `sha256_raw` values as information; a changed normalized text fails and
   names the changed nodes. `fetch --latest` stays date-only, as R1b reads,
   but notes any reissue since the fetch. Stage 04's release check gains
   `fetch --verify`. `meta.yaml`'s schema doesn't change.
2. (F3) govinfo's text is compared exactly, after a layout rule: each line
   break, together with its indentation, is layout and is deleted. The
   heading is SECTNO, one space, SUBJECT; PRTPAGE and the CITA are dropped;
   the text is normalized and its hash must equal the stored
   `sha256_text`. On a mismatch, each differing paragraph is printed.
3. (F5) `verified_on` is the UTC date of the first matching run; a later
   run that still matches changes nothing, and a later mismatch overwrites
   it with `null`.
4. (F7) When eCFR lists a newer version, `fetch --latest` exits 1, prints
   each section's stored date and eCFR's newer dates, and writes nothing.

A correction to F3's option text, recorded as ADR 0013's was, and in the
PR comment that quotes the calls (its "Corrections to the option texts"):
the option said a govinfo format change "can only cause a false mismatch
(null), never a false match". That is not strictly true. The rule deletes,
in each run of character data on its own, each literal line feed and up to
12 literal spaces after it, and nothing else: a granule with a CR is
refused, and so is one with, inside its root, a CDATA section or a
character reference to any White_Space character, so every LF and space
deleted was written as itself; nothing is trimmed, and every other
character, White_Space at a paragraph's or the heading's edge too, is
compared, each side normalized by decision 13 alone, with the stored
version's text. So a change in govinfo's layout can give a false mismatch
(`verified_on: null`), and a false match in one way only: each maximal run
of one or more literal LFs, each followed by up to 12 literal spaces,
standing where govinfo meant a word space (between two words, or at a
text's edge) that the stored text lacks reads as no space, as the stored
text does. govinfo's layout writes such a space as a space, outside the
run: before its first break, or as a thirteenth space after a break. In
the live granules of 1024.30, 1024.31 and 1024.41, the heading and every
paragraph equal the stored version's with nothing trimmed.

The bound, 12, is the chief of staff's reading of the owner's "its
indentation": what govinfo's pretty-printer emits before a `<P>`'s text
(three levels of four), the deepest place text is broken; a thirteenth
space after a break, or a space before the first break of a run, is text.
On 1024.41's granule the rule, with nothing trimmed, gives 75 of 75
paragraphs equal to the stored version's and `sha256_text`
`ecda50e7ce958e2fd19a8f1843b3c60bde0dcf533cc2c313cb9c1e1960760fe1`, the
stored one; a bound of 11 gives 25 of 75 and of 13 gives 38 of 75. It gives
8 of 8 for 1024.30 and 24 of 24 for 1024.31 against their stored raw XML's
paragraphs (live granules, not committed): those two figures are Go's,
which the review hunters' own oracles confirmed; the committed
`oracle.py` checks 1024.41 only. The rule depends on govinfo's formatter.

So ADR 0013's F3 option text, "step 6 can compute the same hash from
govinfo's text for verified_on", holds only with this layout rule.

The chief of staff ruled (agents' choices): F2 G1; F4 (a), (b) and (c);
F6 (i) and T2; F8 with E-a, a removal a finding, the larger code winning
across sections; F9 N1; F10; the mandate's assumptions 1–11, among them
that `--verify` refuses an `overrides.yaml` (assumption 7: applying
overrides isn't built, R1a; with none it prints "overrides: none"); the
bound of 12; and, on review of PR #10 (round 1):
- (§1) `--verify` vouches only for what it derives again: under the lock,
  before any request but `titles.json`, it builds the stored raw XML as
  `regcorpus parse` does and refuses, exit 1, a `tree.json`, `text.md` or
  `sha256_text` that differs, naming each and `regcorpus parse`.
- (§2) The cache is not evidence: `verified_on` is decided and written
  first, then the granule is kept; a file the cache can't keep (`--date`'s
  too) is the cache's failure, exit 2, named as a disposable, git-ignored
  cache that may be deleted; a symlink there is refused.
- (§3) The granule is read in a closed grammar, refusing what it doesn't
  read: a root of FDSYS then one SECTION; FDSYS's elements, its title 12
  and the section's part; a DATE only in FDSYS; attributes only where
  govinfo's granules have them (`<E T="03">`); an empty `<P>`; a DATE or
  SECTNO not one plain run; comments and processing instructions inside
  the root.
- (§4) The false-match class: a CR or a reference to a line break is
  refused; layout is deleted per run; every paragraph's edges are trimmed
  alike (round 2's §1 removed the trim); the heading and each paragraph,
  normalized, must equal eCFR's one for one (round 2's §1: the stored
  version's), as well as the joined hash, and a split or merged paragraph
  is printed alone, without a cascade; the remaining false match is stated
  above.
- (§5) `--latest` takes eCFR's latest from the version list: older than
  the stored version ("eCFR no longer lists the stored version as
  latest") exits 1 too, and `--verify`'s advice follows the direction.
- (§6) `issue_date` is read only by `--latest`'s note; one that isn't a
  date is noted there and fails nothing, and plain `fetch` ignores it.
- (§7) A heading or paragraph-grouping change the trees don't show is
  named.
- (§8) The reissue note covers an `issue_date` on or after `retrieved_at`'s
  day: "issued on or after the day the stored file was fetched; run
  regcorpus fetch --verify".

And on review round 2:
- (§1) govinfo is compared with the stored version that `--verify` derives
  again, never with the fresh fetch, so a byte-only reissue never touches
  the govinfo verdict and a stored grouping govinfo lacks is a mismatch;
  each side is normalized by decision 13 alone, with no trim; a differing
  heading is named.
- (§2) Inside the root, a CDATA section and a character reference to any
  White_Space character are refused, and the oracle refuses the same; the
  one remaining false match is stated exactly (above); a SECTION holds one
  or more `<P>`s, with the CITA once and last or not named in the prose
  (the implementer's choice, below); comments and processing instructions
  are refused inside the root.
- (§3) Every eCFR or govinfo value printed is validated to its form first
  or quoted, so none can forge a line of the saved output.
- (§4) A latest entry at the stored date that removed the section is named
  as that, exit 1, with no advice to run `fetch --latest`.
- (§5) One reading of eCFR's latest (`Latest`: the newest date, and
  whether an entry at it removed the section) serves plain `fetch` and
  `--latest`; plain `fetch` still exits 2 on a removal.
- (§6) The refusals are pinned by tests that kill their mutants (a
  PRTPAGE in an E, two CITAs, and the round's own), and so is a byte order
  mark, which is not refused: a pin that a granule with one reads as one
  without.

And on review round 3:
- (§1) No upstream byte reaches the output unescaped: every line
  regcorpus writes to stdout or stderr, a printed line or an error, passes
  through one sink, which writes each C0 control but the LF ending the
  line, DEL, each C1 control (U+0085 among them), U+2028 and U+2029
  escaped, visibly. The quoting of round 2's §3 stays where it is; the
  sink is the guarantee that holds whatever a call site prints, and the
  claim of §3 holds through it. `parse.WellFormed` names a token it
  refuses quoted, and an HTTP status is quoted. One table test drives a
  payload that would forge a line through every upstream channel.
- (§2) A latest entry older than the stored date and removed gives the
  older verdict, and a no-break space between two `<P>`s is refused: both
  pinned.
- (§3) `--verify`'s line for a latest entry at the stored date that
  removed the section says that, not that the two dates differ.

And on review round 4:
- (§1) One table test pins, for every command, that its output reaches
  both streams only through the sink, its help and usage a line per Write.

The implementer's choices (agents' choices, within those rulings):
- The granule is read by a new package, `internal/govinfo`, which does no
  I/O; the request goes through `internal/ecfr`'s client, with the same
  `get`: a generic User-Agent, gzip, redirects followed only within the
  origin asked (govinfo's link service redirects on its own host), the
  status decided before reading, the size cap; an error names the URL that
  failed, after a redirect too.
- A granule of another section or edition, or one the grammar refuses, is
  exit 2, with nothing written: it is not the document to compare, so it
  says nothing about the text.
- The granule is kept at
  `.cache/govinfo/2026/title-12/<section>/<sha256>.xml`, stored once.
- In round 0, a strict decoding of `tree.json` (it had to encode back to
  its own bytes); round 1's derivation (§1) replaced it, and
  `l0.State.Stale` is now the one comparison of a source's derived files,
  for `parse`'s writes and `--verify`'s refusal.
- Each side's heading and paragraphs are compared normalized as they are
  (round 1 trimmed govinfo's; round 2's §1 removed that), so equal
  paragraphs and heading give the joined hash too; an alignment by longest
  common subsequence prints each differing run of paragraphs; an
  `issue_date` that is empty is read as absent.
- The oracle `internal/anchors/testdata/oracle.py` is extended in place,
  not as a sibling script, since `python3 -I` doesn't put the script's
  directory on the import path: given the granule too, it decodes it with
  Python's own XML parser, refuses a CR, and inside the root a CDATA
  section or a reference to a White_Space character (found with expat's
  byte offsets), deletes the layout in each run, trims nothing and prints
  the govinfo `sha256_text` (75 of 75 paragraphs), which a Go test pins;
  it is run and pinned on 1024.41 only.
- `.gitattributes` keeps the committed granule's bytes.
- In round 2: the CITA is kept in the grammar, required once and last, as
  each of the three live granules has it, rather than dropped from the
  prose; a `<P>` of nothing but White_Space is refused, as round 1's empty
  `<P>` was (a check, not a trim); the White_Space-reference refusal covers
  attribute values inside the root too, and the CR refusal the whole
  granule; the round-1 trim of a byte order mark goes, since Go's decoder
  reads one outside the root as no text (a test pins that a granule with
  one reads as one without); `parse.Qualified` quotes a namespace unless
  it is made of ASCII XML-name bytes, a root's TYPE and N are quoted, a
  URL a redirect led to must be printable ASCII or is refused, quoted, and
  `issue_date` is printed as a date once read as one and quoted otherwise;
  encoding/xml's own messages for XML that isn't well-formed are printed
  as it writes them (a name there can't hold an ASCII control character,
  but can hold U+0085 or U+2028, which round 3's sink escapes);
  `Latest` returns the date and the removal, and plain `fetch` words its
  own exit-2 error from them; the output says "the stored version's" where
  it said "eCFR's".
- In round 3: the sink is a new package, `internal/sink`, which
  `cmd/regcorpus` wraps stdout and stderr in for every command, `check`,
  `meta` and `parse` too, and run's own errors and help; commands get the
  sink, never the files (`verify` gets the stdout file only to test it
  for a terminal, and writes to its stderr sink). A Write is one line, as
  `fmt.Fprintf` and `Fprintln` make it: an LF inside it is escaped, the
  one ending it kept; a help text is a `sink.Text` that `sink.Print`
  writes a line per Write, so each command prints its usage apart from a
  usage error's line. A C0 control or DEL is written `\xNN`, a C1 control,
  U+2028 or U+2029 `\uNNNN`, and a byte that isn't UTF-8 `\xNN` too, since
  a raw 0x9b, a control sequence introducer to a terminal reading bytes as
  C1, reached stderr from an HTTP reason phrase before this round; a tab
  is a C0 control, so it is escaped too, and a backslash is not; a line
  without them is written byte for byte. A token `parse.WellFormed`
  refuses is named as written, quoted and capped at 300 bytes
  (`Snippet`), and the whole HTTP status is quoted.
  `--verify`'s version line says the verdict `--latest` prints in each
  case, newer, older or removed.
- In round 4: no payload can reach stdout from `check`, `meta` or `parse`,
  nor offline from `fetch`, so `run` calls those four from one table,
  `commands`, with the sinks, and the test puts a fake in each place that
  writes a payload to both; no behaviour changes. Each row is held to what
  the same command writes to plain buffers, byte for byte, its payload
  escaped. Step 2's line of `fetch -h`, 84 columns, is reflowed with step
  3's.

## Spec changes
- `docs/03-spec.md`:
  - The header's "**Amended:** 2026-11-22 by ADRs 0007–0012; 2026-11-24 by
    ADR 0013" → "**Amended:** 2026-11-22 by ADRs 0007–0012; 2026-11-24 by
    ADR 0013; 2026-11-25 by ADR 0014".
  - Decisions-table row 14 is added, this line exactly (its link is
    relative to `docs/`):

    ```
    | 14 | What makes a re-served copy the same version, how is govinfo's text compared, what date does `verified_on` hold, and what does `fetch --latest` do past exiting non-zero? | A version is its date plus its normalized text: the tree's node paths in order, each node's `sha256`, and `sha256_text`. `fetch --verify` first derives the stored `tree.json`, `text.md` and `sha256_text` again from the raw XML, and refuses, writing nothing, when one differs; then it compares eCFR's latest version, read from its version list, which must be `source_version_date`, with them, printing both `sha256_raw` values and the nodes whose raw text differs as information only; then, only if that matches, the section's granule in the govinfo CFR 2026 annual edition (title 12, revised as of 2026-01-01), through govinfo's link service, read only in the form a granule has (no CR; inside its root no CDATA section and no character reference to a White_Space character; one or more `<P>`s, then the citation note once and last): each line break and up to 12 spaces after it are deleted as layout and nothing else is deleted or trimmed, the heading is SECTNO, one space, SUBJECT, a paragraph keeps `<E>`'s text and drops PRTPAGE, the citation note is left out, and the heading and each paragraph, each side normalized by decision 13 alone, must equal the stored version's, derived again, one for one, and joined and normalized as decision 13 joins them, hash to `sha256_text`; so a change in govinfo's layout can give a false match in one way only: a maximal run of one or more literal LFs, each followed by up to 12 literal spaces, standing where govinfo meant a word space that the stored text lacks. `verified_on` is the UTC date of the first matching run, kept by a later match and overwritten with `null` by a mismatch, which prints each differing paragraph and exits 1; only then is the granule kept in the git-ignored `.cache/govinfo/`, a cache, never committed. `fetch --latest` reads `titles.json`, each `meta.yaml` (checked) and the version lists, and writes nothing: eCFR's latest version newer or older than the stored one, or one that removed the section, exits 1 with the stored and latest dates; an entry for the stored date issued on or after `retrieved_at`'s day is printed as information. Stage 04's release check runs `fetch --verify` too. No schema change (the owner's calls: the normalized text as the version, govinfo compared exactly with its layout deleted, the first matching run's date, and `--latest` reporting only; the bound of 12, the granule, its form and cache, the stored files derived again, the paragraph-by-paragraph comparison with the stored version, the order, the exit codes, the issue-date note and the sources each flag covers: the chief of staff's rulings; the citation note once and last in the granule's form: the implementer's choice). [ADR 0014](adr/0014-normalized-text-is-the-version.md) |
    ```
  - Acceptance, "R1": "- [ ] **R1:** `regcorpus fetch --verify` exits 0:
    the version date, tree and `sha256_text` match a fresh fetch
    (`sha256_raw` is informational). `verified_on` is set by comparing
    normalized text to the govinfo 2026 edition. The output is saved." →
    "- [ ] **R1:** `regcorpus fetch --verify` exits 0: eCFR's latest
    version is dated `source_version_date`, and its tree (node paths in
    order, each node's `sha256`) and `sha256_text` match a fresh fetch
    (`sha256_raw` is informational). `verified_on` is set by comparing
    normalized text to the govinfo 2026 edition, its layout deleted first
    (decision 14). The output is saved."
  - L0 `meta.yaml`, the `verified_on` line: "verified_on: <YYYY-MM-DD or
    null>  # normalized text matched the govinfo annual edition; null on
    mismatch, diff recorded" → "verified_on: <YYYY-MM-DD or null>  # UTC
    date of the first run whose normalized text matched the govinfo 2026
    edition (decision 14); null on mismatch, the diff in the saved output".
  - Upstream, "Rule text": "- **Rule text:** fetched point-in-time from the
    eCFR versioner API. `fetch --latest` (R1b) detects newer versions." →
    "- **Rule text:** fetched point-in-time from the eCFR versioner API.
    `fetch --latest` (R1b) detects newer versions by date; a reissue of a
    stored date is compared by `fetch --verify` (decision 14)."
  - Upstream, "Official-edition check": "- **Official-edition check:**
    govinfo's annual edition, for `verified_on`." → "- **Official-edition
    check:** govinfo's CFR 2026 annual edition, for `verified_on`: each
    section's granule XML, through govinfo's link service, kept at run time
    only in the git-ignored `.cache/govinfo/`, a cache never committed; a
    granule may be committed apart from that as a test fixture, as
    1024.41's is (decision 14)."
  - R1a, "Overrides": "applied before parsing, and listed by `fetch
    --verify`; never in parser code" → "applied before parsing, and listed
    by `fetch --verify` once applying overrides is built (until then `fetch
    --verify` refuses a source with an `overrides.yaml`; decision 14); never
    in parser code", as stage 01's end check says.
  - Carried forward, a row added: "| Whether `fetch --verify` against a
    stored version dated after the govinfo edition's 2026-01-01 records
    `verified_on: null` or refuses, since that edition can't hold it (moot
    while every stored version predates 2026-01-01; decision 14) | the
    first stored version after 2026-01-01 |".
- `docs/04-plan.md`:
  - The header's "**Amended:** 2026-11-22 by ADRs 0007, 0010, 0011 and
    0012; 2026-11-24 by ADR 0013" → "**Amended:** 2026-11-22 by ADRs 0007,
    0010, 0011 and 0012; 2026-11-24 by ADR 0013; 2026-11-25 by ADR 0014".
  - Files: "internal/ecfr/                    # versioner client;
    full/{date} XML; fetch, meta set and parse" → "internal/ecfr/
    # versioner and govinfo client; full/{date} XML; fetch (--verify,
    --latest), meta set and parse", and after it a line added:
    "internal/govinfo/                 # the govinfo 2026 granule: its
    checks, the layout rule, heading and paragraphs (ADR 0014)"; and, in
    round 3, after the `cmd/regcorpus/main.go` line a line added:
    "internal/sink/                    # stdout and stderr: every line
    regcorpus writes, control characters and line separators escaped (ADR
    0014)" (the comments aligned at column 35, as the list's are).
  - Upstream calls, "govinfo": "- **govinfo:** the CFR 2026 annual
    edition, for `verified_on`." → "- **govinfo:** the CFR 2026 annual
    edition, for `verified_on`: `link/cfr/12/<part>?link-type=xml&sectionnum=<n>&year=2026`,
    which redirects on govinfo's host to the section's granule XML (ADR
    0014)."
  - Step 6: "6. `regcorpus fetch --verify`: compare the version date, tree
    and `sha256_text`, recording `sha256_raw` as information only. Then
    compare the normalized text to the govinfo CFR 2026 edition and set
    `verified_on`. Add `regcorpus fetch --latest` (R1b)." → that line, then
    three sub-items: "   - A version is its date plus its normalized text:
    the tree's node paths in order, each node's `sha256`, and `sha256_text`
    (spec decision 14).", "   - govinfo's granule, its layout deleted
    (each line break and up to 12 spaces after it), must hash to
    `sha256_text`; `verified_on` is the first matching run's UTC date,
    `null` on a mismatch." and "   - `--verify` covers 1024.41; 1024.30
    and 1024.31 are step 9's. `--latest` covers all three, writes nothing,
    and exits 1 when eCFR's latest version is newer or older than the
    stored one, or removed the section."
- `stages/01-fetch-parse/CONTRACT.md`:
  - "Local exploratory downloads go in `.cache/` (git-ignored); committed
    raw XML lives only under `sources/**/raw/`." → "Local exploratory
    downloads go in `.cache/` (git-ignored); the committed raw XML of L0
    lives only under `sources/**/raw/`. A test fixture under
    `internal/*/testdata/` (1024.41's govinfo granule, say) is not L0 (spec
    decision 14)."
  - Inputs: "- The govinfo CFR 2026 annual edition, to set `verified_on`
    (R1)." → "- The govinfo CFR 2026 annual edition, to set `verified_on`
    (R1): each section's granule XML, through govinfo's link service, kept
    in the git-ignored `.cache/govinfo/` (spec decision 14)."
  - "Check that ends it", the first item: "- `regcorpus fetch --verify`
    exits 0 (R1; built in plan step 6). Exit 0 means the version date, tree
    and `sha256_text` match a fresh fetch (`sha256_raw` is informational),
    `verified_on` is set, and every override is listed. Save the output." →
    "- `regcorpus fetch --verify` exits 0 (R1; spec decision 14). Exit 0
    means eCFR's latest version is dated `source_version_date`, its tree
    (node paths in order, each node's `sha256`) and `sha256_text` match a
    fresh fetch (`sha256_raw` is informational), the govinfo 2026 edition's
    text, its layout deleted, hashes to `sha256_text`, `verified_on` is
    set, and there is no `overrides.yaml` (it prints "overrides: none";
    one is refused until applying overrides is built, R1a). It covers
    1024.41 today: 1024.30 and 1024.31 are refused, exit 1, until plan step
    9, and Supplement I is plan step 8's. Save the output."
  - "Check that ends it", the last item: "- `regcorpus fetch --latest`
    exits 0 (R1b; built in plan step 6). Exit 0 means eCFR lists no newer
    version of any stored section, and the fresh Supplement I 41() slice
    hash equals the stored one." → "- `regcorpus fetch --latest` exits 0
    (R1b; spec decision 14). Exit 0 means eCFR's latest version of each
    stored section is its stored one; a newer, an older or a removed latest
    version exits 1. A reissue of a stored date is printed as information
    only. From plan step 8 it also means the fresh Supplement I 41() slice
    hash equals the stored one."
- `stages/04-release/CONTRACT.md`:
  - The title's "(R6, R1b, C6 --complete)" → "(R6, R1, R1b, C6
    --complete)".
  - "Check that ends it": "(step 1 needs eCFR, which the cloud workspace
    can't reach)" → "(steps 1 and 2 need eCFR and govinfo, which the cloud
    workspace can't reach)"; step 1, "1. `regcorpus fetch --latest` (R1b;
    plan step 6). Exit 0 means no stored section has a newer eCFR version
    and the Supplement I 41() slice is unchanged." → "1. `regcorpus fetch
    --latest` (R1b; spec decision 14). Exit 0 means eCFR's latest version
    of each stored section is its stored one (a newer, an older or a
    removed latest version exits 1) and the Supplement I 41() slice is
    unchanged."; a step 2 is added after step 1: "2.
    `regcorpus fetch --verify` (R1; spec decision 14). Exit 0 means each
    stored source's normalized text still matches a fresh fetch and the
    govinfo 2026 edition, and its `verified_on` is set."; the old steps 2
    and 3 become 3 and 4.
  - "Who may run it": "the three checks above" → "the four checks above".
  - Requirements, a line added before R1b's: "- R1: `fetch --verify`
    before every release, so a reissue whose text changed is caught (spec
    decision 14)."
- `docs/adr/README.md`: the index line for 0014.
- `docs/adr/0013-cites-join-as-the-document-reads.md`: an "Amended" header
  line is added after "Decided by", naming F3's option text (below,
  Consequences) and the Decision's sentences on a name in another case
  under `.cache/`, which exit 2 since this ADR's round-1 ruling §2, this
  line exactly:

  ```
  - **Amended:** 2026-11-25 by ADR 0014. F3's option text "step 6 can compute the same hash from govinfo's text for verified_on" describes what the owner was told before plan step 6: govinfo's 2026 XML is pretty-printed, so it holds only with ADR 0014's layout rule (each line break and up to 12 spaces after it deleted), under which 1024.41's granule gives the stored `sha256_text`. The Decision's "Exact names (PR #9's reviews)" sentences "A source's `meta.yaml`, `tree.json`, `text.md` and `overrides.yaml` (the first review), and every directory on the way to them, from `sources/` down and under `.cache/` (the second), are found by exact name among their parent's entries; an entry whose name differs from one only in case (Unicode simple case folding, so `ſ`, U+017F, which APFS folds to `s`, too), whatever it is, is refused (exit 1)." and "The file `StoreOnce` stores (`raw/<date>.xml`, a first `meta.yaml`, `fetch --date`'s file under `.cache/`) is found by exact name too, and a name that differs from it only in case (`2025-07-15.XML`) is refused (exit 1) with nothing stored" describe `.cache/` before plan step 6: since ADR 0014 (the chief of staff's round-1 ruling §2), every refusal under `.cache/`, such a name among them, is the cache's failure, exit 2, naming the file as a disposable, git-ignored cache; under `sources/` they are still exit 1.
  ```
- `regcorpus help` (`cmd/regcorpus/main.go`) and `regcorpus fetch -h`
  list `--verify` and `--latest`; the package docs of `internal/ecfr`,
  `internal/l0` and `internal/anchors` say what step 6 built where they
  said it was coming, `internal/parse`'s that a refused token is named
  quoted, and `internal/govinfo/doc.go` and `internal/sink/doc.go` are
  new. These are code, not spec lines, and are not quoted here.
- No other lines. The intent says nothing of how `verified_on` is
  compared; AGENTS.md names no flag of `fetch` (it stays at 99 lines); R1b
  and R6 in the spec are unchanged, since stage 04 holds the release
  checks.

## Consequences
- 1024.41's `meta.yaml` gains one line, `verified_on: "2026-11-26"`, the
  UTC date of the first live run that matched (2026-11-26T02:03Z, the
  evening of 2026-11-25 in CST; its output is quoted in this ADR's PR
  body). A second run printed "unchanged" and changed no byte. No other L0
  file changes.
- A byte-only reissue of a stored date passes `fetch --verify`; plain
  `fetch` still exits 1 on it, since its raw file is stored once (plan step
  2). How a new version or a reissue lands stays undecided (`upgrade`,
  after accept).
- A reissue that changes the normalized text is caught only by `fetch
  --verify`, which stage 04 now runs before every release; `fetch --latest`
  notes the reissue and still exits 0.
- `fetch --verify` with no section exits 1 until plan steps 8 and 9, since
  it refuses 1024.30 and 1024.31 (and Supplement I isn't stored); stage
  01's end check says so. Stage 04 runs after both.
- A govinfo mismatch is the one exit 1 that writes (`verified_on: null`);
  `regcorpus fetch -h` says so.
- The run-time cache isn't committed, so `verified_on` can be re-checked
  only by a fetch, on the owner's machine; the URL and sha256 in the saved
  output name what was compared. 1024.41's granule is committed apart from
  that, as a test fixture (F9, N1), which the offline tests and the oracle
  read; nothing reads the cache back.
- After review round 1, a re-run at `6d8c8b8` printed the same verdict,
  "unchanged", and changed no byte. `--verify` now refuses a stored `tree.json`,
  `text.md` or `sha256_text` that `regcorpus parse` wouldn't write, and a
  granule outside the form above; `--latest` exits 1 too when eCFR's list
  ends before the stored version.
- After review round 2, a re-run at `aa2a37f` printed the same verdict,
  "unchanged", the heading and 75 paragraphs equal to the stored
  version's with nothing trimmed, and changed no byte.
- After review round 3, a re-run at `e294bc9` printed the same verdict,
  "unchanged", and changed no byte, and `fetch --latest` exited 0. Every
  line regcorpus writes passes the sink, so in saved output a control
  character, a line separator or a byte that isn't UTF-8 shows as `\xNN`
  or `\uNNNN` on its line, whichever command printed it.
- Plan step 19's interactive text in `regcorpus verify` must reach its
  stderr sink one line per Write (`sink.Print` for a text of many lines),
  or the sink writes each LF inside a Write as `\x0a`.
- The test for the layout rule pins its bound: a granule whose text
  starts in the thirteenth column after a break, or with a space removed
  between words, gives another hash; one changed only in indentation of
  12 or less gives the same.
- ADR 0013 gains an "Amended" line: its F3 option text "step 6 can compute
  the same hash from govinfo's text for verified_on" describes what the
  owner was told before this step; it holds only with this ADR's layout
  rule. The same line names its Decision's sentences that give a name in
  another case under `.cache/` exit 1: since round 1's §2 it is exit 2.
- Carried forward (spec "Carried forward"): a stored version dated after
  2026-01-01, which the 2026 edition can't hold.

## Requirement IDs
R1 (`verified_on`, the version date, tree and `sha256_text` against a fresh
fetch), R1a (overrides listed), R1b (the newer-version check); spec "L0
`meta.yaml`", "Upstream" and "Carried forward"; stage 01's and stage 04's
checks; decisions 6 (CLI only), 10 (one writer of `meta.yaml`), 13
(normalization, the join, what `sha256_text` covers) and 14.

## Links
- This ADR's PR, PR #10 (https://github.com/Okja-Engineering/reg-corpus/pull/10),
  with the comment quoting the owner's calls, every option offered and the
  corrections to the option texts
  (https://github.com/Okja-Engineering/reg-corpus/pull/10#issuecomment-7180640566),
  and the live output of `fetch --verify 1024.41` (twice) and `fetch
  --latest` in its body.
- Commits: `17c0655` (the tests against stubs, red); `7987daa`
  (`internal/govinfo`, `fetch --verify` and `--latest`, the package docs;
  green); `9e74436` (1024.41's `verified_on` line, from the first live
  run); `490c85b` (this ADR and its amendments); review round 1: `b73675e`
  (the tests for the reconciled worklist, red), `6d8c8b8` (its fixes,
  green), `4c9323e` (their record here and in the spec), `90bd5dd` (a test
  pinning "a DATE only in FDSYS" on its own), `f0c52a6` (adding those two
  to this list); review round 2: `7264a39` (the tests for the reconciled
  worklist, red), `d013da1` (its fixes, green), `aa2a37f` (tests pinning
  the round's surviving mutants), `e2ba509` (its record here and in the
  spec); review round 3: `bf32240` (the tests for the reconciled
  worklist, red), `d45989a` (its fixes, green), `e294bc9` (tests pinning
  the round's surviving mutants), `952a89c` (its record here and in the
  spec); review round 4: `9347e80` (the table test, and `run`'s table of
  commands it needs), `c14ad10` (`fetch -h` reflowed), and the commit
  that records round 4 here.
- Files:
  - Code: `internal/govinfo/granule.go` and `doc.go` (new),
    `internal/ecfr/verify.go` and `newer.go` (new),
    `internal/ecfr/client.go` (`Version.IssueDate`, `Granule`, `Latest`,
    a redirect's URL checked), `internal/ecfr/fetch.go` (the flags, `-h`,
    the cache's errors), `internal/ecfr/sources.go` (a root's TYPE and N
    quoted), `internal/parse/wellformed.go` (`Qualified` quotes a
    namespace that isn't made of XML name bytes; a refused token is named
    quoted), `internal/sink/sink.go` and `doc.go` (new, round 3),
    `cmd/regcorpus/main.go` (every command given the sink; in round 4,
    `check`, `fetch`, `meta` and `parse` from one table),
    `internal/check/check.go`, `internal/ecfr/metaset.go` and
    `internal/ecfr/parsecmd.go` (each usage written by `sink.Print`),
    `internal/anchors/build.go` (`JoinSection`),
    `internal/anchors/anchors.go` (`Document.Paragraphs`),
    `internal/l0/source.go` (`State.Stale`), `internal/l0/store.go`
    (`StoreOnce` compares only a regular file),
    `.gitignore` (`__pycache__/`), `.gitattributes` (the granule's bytes).
  - Package docs: `internal/ecfr/doc.go`, `internal/l0/doc.go`,
    `internal/anchors/doc.go`, `internal/parse/doc.go`.
  - Tests: `internal/govinfo/granule_test.go` and
    `internal/govinfo/testdata/CFR-2026-title12-vol8-sec1024-41.xml` (the
    live granule, kept byte for byte by `.gitattributes`),
    `internal/ecfr/verify_test.go`, `internal/ecfr/latest_test.go`,
    `internal/ecfr/granule_test.go`, `internal/ecfr/client_test.go`,
    `internal/ecfr/fetch_test.go`, `internal/ecfr/names_test.go`,
    `internal/ecfr/output_test.go` (new, round 3: the channel table),
    `internal/ecfr/metaset_test.go`, `internal/ecfr/parsecmd_test.go`,
    `internal/sink/sink_test.go` (new), `internal/parse/wellformed_test.go`,
    `internal/parse/malformed_test.go`,
    `internal/anchors/section_test.go`,
    `internal/anchors/testdata/oracle.py`, `internal/l0/store_test.go`,
    `cmd/regcorpus/main_test.go` (round 4: the table of every command).
  - L0: `sources/ecfr/title-12/1024.41/meta.yaml` (`verified_on`).
  - Spec docs: `docs/03-spec.md`, `docs/04-plan.md`, `docs/adr/README.md`,
    `docs/adr/0013-cites-join-as-the-document-reads.md`,
    `stages/01-fetch-parse/CONTRACT.md`, `stages/04-release/CONTRACT.md`
    and this ADR.
- ADR 0013, F3 ("Rule text only"), which fixed what `sha256_text` covers;
  ADR 0010, the one writer through which `verified_on` is set.
