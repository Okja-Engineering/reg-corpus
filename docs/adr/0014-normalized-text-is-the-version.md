# 0014. Normalized text is the version; govinfo is compared exactly, its layout deleted

- **Status:** Accepted
- **Date:** 2026-11-25
- **Decided by:** the owner, 2026-11-25 about 19:29 CST, in chat (answers to the chief of staff's questions), to be quoted on this ADR's PR: (F1) "Normalized text (Recommended)"; (F3) "Exact, with layout rule (Recommended)"; (F5) "First matching run (Recommended)"; (F7) "Report only (Recommended)". The rest (F2, F4, F6, F8, F9, F10) are the chief of staff's rulings, and the details marked below the implementer's choices: agents' choices, not the owner's.

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
  eCFR's; the rest differ by a space where a line was broken inside a word
  or before punctuation. ADR 0013's F3 option text told the owner "step 6
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
  (G3). Kept out of git, in `.cache/govinfo/`, its URL and sha256 printed;
  not committed under `sources/`.
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
  3 (E-b).
- F9: tests use only fakes and committed testdata (N1), not an opt-in or a
  CI live test; the live output is quoted in the PR body.
- F10: `--verify` covers 1024.41 only and refuses 1024.30 and 1024.31 until
  plan step 9 gives them a tree and `sha256_text`; `--latest` covers all
  three stored sections.

## Decision
The owner decided:
1. (F1) A version is its date plus its normalized text: its tree's node
   paths, their per-node `sha256` and `sha256_text`. `fetch --verify` passes
   a re-serve whose bytes differ but normalize alike, and prints both
   `sha256_raw` values, and the nodes whose raw text differs, as
   information; a changed normalized text exits 1 and names the changed
   nodes. `fetch --latest` stays date-only, as R1b reads, and prints as
   information, not changing its exit code, any version-list entry for the
   stored date whose `issue_date` is after `retrieved_at`. Stage 04's
   release check gains `fetch --verify`. `meta.yaml`'s schema doesn't
   change.
2. (F3) govinfo's text is compared exactly, after a layout rule: each line
   break, together with up to 12 spaces after it, is layout and is deleted,
   in each run of character data on its own. The heading is SECTNO, one
   space, SUBJECT; a paragraph is a `<P>`'s text with each `<E>`'s text in
   place and each PRTPAGE dropped; the CITA is left out. The heading and
   paragraphs are joined as `sha256_text` joins eCFR's (decision 13: one
   space after the heading and between `<P>`s), normalized as one string,
   and the sha256 must equal the stored `sha256_text`. On a mismatch, each
   differing paragraph is printed, normalized.
3. (F5) `verified_on` is the UTC date of the first matching run; a later
   run that still matches changes nothing, and a later mismatch overwrites
   it with `null`.
4. (F7) When eCFR lists a newer version, or its latest version removed the
   section, `fetch --latest` exits 1, prints the stored date and eCFR's
   newer dates, and writes nothing.

The bound, 12, is what govinfo's pretty-printer emits before a `<P>`'s
text (three levels of four), the deepest place text is broken; a thirteenth
space after a break, or a space before one, is text. On 1024.41's granule
the rule gives 75 of 75 paragraphs equal to eCFR's and `sha256_text`
`ecda50e7ce958e2fd19a8f1843b3c60bde0dcf533cc2c313cb9c1e1960760fe1`, the
stored one; a bound of 11 gives 25 of 75 and of 13 gives 38 of 75. It gives
8 of 8 for 1024.30 and 24 of 24 for 1024.31 against their eCFR text (live
granules, not committed). The rule depends on govinfo's formatter. It
deletes nothing but line breaks and the spaces after them, so every other
character of govinfo's text is compared: a change in the formatter gives a
false mismatch (`verified_on: null`), and a false match only on whether a
space stands where a line breaks, if a formatter wrote a space between
words as the break itself, which govinfo's doesn't today (it writes it
before the break, or as a thirteenth space).

So ADR 0013's F3 option text, "step 6 can compute the same hash from
govinfo's text for verified_on", holds only with this layout rule.

The chief of staff ruled (agents' choices): F2 G1, with the granule
checked before use (valid UTF-8 within the client's size cap, well-formed,
CFRGRANULE, its DATE 2026-01-01, its SECTNO the section) and kept in the
git-ignored `.cache/govinfo/`; F4 (a), (b) and (c); F6 (i) and T2; F8
with E-a, the larger code winning across sections; F9 N1; F10.

The implementer's choices (agents' choices, within those rulings):
- The granule is read by a new package, `internal/govinfo`, which does no
  I/O; the request goes through `internal/ecfr`'s client, with the same
  `get`: a generic User-Agent, gzip, redirects followed only within the
  origin asked (govinfo's link service redirects on its own host), the
  status decided before reading, the size cap.
- A granule of another section or edition, or one whose structure isn't
  the one above (an element where none is read, text where none is
  expected), is refused, exit 2, with nothing written: it is not the
  document to compare, so it says nothing about the text.
- A latest eCFR version that removed the section is exit 1 for `--verify`
  and `--latest`, a finding; plain `fetch` keeps exit 2 for it.
- The granule is kept at
  `.cache/govinfo/2026/title-12/<section>/<sha256>.xml`, stored once, and
  only after it passes its checks.
- `--verify` refuses, exit 1 and before any request but `titles.json`, a
  source with an `overrides.yaml` (applying overrides isn't built, R1a; with
  none it prints "overrides: none"), a `meta.yaml` without `sha256_text`,
  and a `tree.json` not in the form `regcorpus parse` writes (decoded
  strictly, it must encode back to its own bytes).
- The oracle `internal/anchors/testdata/oracle.py` is extended in place,
  not as a sibling script, since `python3 -I` doesn't put the script's
  directory on the import path: given the granule too, it applies the
  layout rule with a regular expression on the raw XML and prints the
  govinfo `sha256_text`, which a Go test pins.

## Spec changes
- `docs/03-spec.md`:
  - The header's "**Amended:** 2026-11-22 by ADRs 0007–0012; 2026-11-24 by
    ADR 0013" → "**Amended:** 2026-11-22 by ADRs 0007–0012; 2026-11-24 by
    ADR 0013; 2026-11-25 by ADR 0014".
  - Decisions-table row 14 is added, this line exactly (its link is
    relative to `docs/`):

    ```
    | 14 | What makes a re-served copy the same version, how is govinfo's text compared, what date does `verified_on` hold, and what does `fetch --latest` do past exiting non-zero? | A version is its date plus its normalized text: the tree's node paths in order, each node's `sha256`, and `sha256_text`. `fetch --verify` compares eCFR's latest version, which must be dated `source_version_date`, with them, printing both `sha256_raw` values and the nodes whose raw text differs as information only; then, only if that matches, the section's granule in the govinfo CFR 2026 annual edition (title 12, revised as of 2026-01-01), through govinfo's link service, kept in the git-ignored `.cache/govinfo/`: each line break and up to 12 spaces after it are deleted as layout, the heading is SECTNO, one space, SUBJECT, a paragraph keeps `<E>`'s text and drops PRTPAGE, the citation note is left out, and joined and normalized as decision 13 joins them, its sha256 must equal `sha256_text`. `verified_on` is the UTC date of the first matching run, kept by a later match and overwritten with `null` by a mismatch, which prints each differing paragraph and exits 1. `fetch --latest` reads version lists only and writes nothing: a newer version, or a latest one that removed the section, exits 1 with the stored and newer dates; a reissue of the stored date after `retrieved_at` is printed as information. Stage 04's release check runs `fetch --verify` too. No schema change (the owner's calls; the granule, its checks and cache, the order and exit codes, and the sources each flag covers: the chief of staff's choices). [ADR 0014](adr/0014-normalized-text-is-the-version.md) |
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
    section's granule XML, through govinfo's link service, kept in the
    git-ignored `.cache/govinfo/`, never committed (decision 14)."
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
    checks, the layout rule, heading and paragraphs (ADR 0014)" (the
    comments aligned at column 35, as the list's are).
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
    and exits 1 on a newer version."
- `stages/01-fetch-parse/CONTRACT.md`:
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
    (R1b; spec decision 14). Exit 0 means eCFR lists no newer version of
    any stored section; a reissue of a stored date is printed as
    information only. From plan step 8 it also means the fresh Supplement I
    41() slice hash equals the stored one."
- `stages/04-release/CONTRACT.md`:
  - The title's "(R6, R1b, C6 --complete)" → "(R6, R1, R1b, C6
    --complete)".
  - "Check that ends it": "(step 1 needs eCFR, which the cloud workspace
    can't reach)" → "(steps 1 and 2 need eCFR and govinfo, which the cloud
    workspace can't reach)"; a step 2 is added after step 1: "2.
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
  line naming F3's option text (below, Consequences).
- `regcorpus help` (`cmd/regcorpus/main.go`) and `regcorpus fetch -h`
  list `--verify` and `--latest`; the package docs of `internal/ecfr`,
  `internal/l0` and `internal/anchors` say what step 6 built where they
  said it was coming, and `internal/govinfo/doc.go` is new.
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
- The granule isn't committed, so `verified_on` can be re-checked only by
  a fetch, on the owner's machine; the URL and sha256 in the saved output
  name what was compared.
- The test for the layout rule pins its bound: a granule whose text
  starts in the thirteenth column after a break, or with a space removed
  between words, gives another hash; one changed only in indentation of
  12 or less gives the same.
- ADR 0013 gains an "Amended" line: its F3 option text "step 6 can compute
  the same hash from govinfo's text for verified_on" describes what the
  owner was told before this step; it holds only with this ADR's layout
  rule.
- Carried forward (spec "Carried forward"): a stored version dated after
  2026-01-01, which the 2026 edition can't hold.

## Requirement IDs
R1 (`verified_on`, the version date, tree and `sha256_text` against a fresh
fetch), R1a (overrides listed), R1b (the newer-version check); spec "L0
`meta.yaml`", "Upstream" and "Carried forward"; stage 01's and stage 04's
checks; decisions 6 (CLI only), 10 (one writer of `meta.yaml`), 13
(normalization, the join, what `sha256_text` covers) and 14.

## Links
- This ADR's PR, with the comment quoting the owner's calls and every
  option offered, and the live output of `fetch --verify 1024.41` (twice)
  and `fetch --latest` in its body.
- Commits: `17c0655` (the tests against stubs, red); `7987daa`
  (`internal/govinfo`, `fetch --verify` and `--latest`, the package docs;
  green); `9e74436` (1024.41's `verified_on` line, from the first live
  run); and the commit that adds this ADR and its amendments.
- Files:
  - Code: `internal/govinfo/granule.go` and `doc.go` (new),
    `internal/ecfr/verify.go` and `newer.go` (new),
    `internal/ecfr/client.go` (`Version.IssueDate`, `RemovedError`,
    `Granule`), `internal/ecfr/fetch.go` (the flags, `-h`),
    `internal/anchors/build.go` (`JoinSection`, `DecodeTree`),
    `internal/anchors/anchors.go` (`Document.Paragraphs`),
    `internal/l0/source.go` (`State.TreeJSON`), `cmd/regcorpus/main.go`.
  - Package docs: `internal/ecfr/doc.go`, `internal/l0/doc.go`,
    `internal/anchors/doc.go`.
  - Tests: `internal/govinfo/granule_test.go` and
    `internal/govinfo/testdata/CFR-2026-title12-vol8-sec1024-41.xml` (the
    live granule, kept byte for byte by `.gitattributes`),
    `internal/ecfr/verify_test.go`, `internal/ecfr/latest_test.go`,
    `internal/ecfr/granule_test.go`, `internal/ecfr/client_test.go`,
    `internal/ecfr/fetch_test.go`, `internal/anchors/section_test.go`,
    `internal/anchors/testdata/oracle.py`, `internal/l0/treejson_test.go`,
    `cmd/regcorpus/main_test.go`.
  - L0: `sources/ecfr/title-12/1024.41/meta.yaml` (`verified_on`).
  - Spec docs: `docs/03-spec.md`, `docs/04-plan.md`, `docs/adr/README.md`,
    `docs/adr/0013-cites-join-as-the-document-reads.md`,
    `stages/01-fetch-parse/CONTRACT.md`, `stages/04-release/CONTRACT.md`
    and this ADR.
- ADR 0013, F3 ("Rule text only"), which fixed what `sha256_text` covers;
  ADR 0010, the one writer through which `verified_on` is set.
