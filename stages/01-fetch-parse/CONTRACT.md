# Stage 01: fetch and parse L0 (R1, R1a, R1b)

## Purpose
Store the rule text verbatim (R1) and turn it into a deterministic tree of
anchors (R1a), so every later stage cites stable anchors with known hashes.
This stage records what the source says. It never decides what the source means.

## Inputs
- The eCFR versioner API: version lists and point-in-time XML for 1024.41,
  Supplement I to Part 1024 (sliced to its 41() comments), 1024.30 and 1024.31
  (plan, upstream calls). These hosts are blocked from the cloud workspace, so
  fetches run on the owner's machine.
- The govinfo CFR 2026 annual edition, to set `verified_on` (R1): each section's granule
  XML, through govinfo's link service, kept in the git-ignored `.cache/govinfo/` (spec
  decision 14).
- The Federal Register, for `version_effective_date` and the amending document (R1).
  For 1024.41, 1024.30 and 1024.31, `version_fr_citations` records the amendment page
  as the stored raw XML's citation note (`<CITA>`) cites it (spec decision 11), and
  `regcorpus meta set` sets both fields for these three. Supplement I's two fields are
  open until plan step 8.
- `overrides.yaml` per source, if a source defect needs one (R1a). Each entry
  has a location, a replacement and a reason, and is reviewed.

## Outputs
Per source directory: `sources/ecfr/title-12/1024.41/`,
`sources/ecfr/title-12/supplement-i/41/`, `sources/ecfr/title-12/1024.30/`,
`sources/ecfr/title-12/1024.31/`:
- `raw/<date>.xml`: the fetched XML, untouched (Supplement I: the whole appendix).
- `tree.json`: the parsed tree; each node carries the `sha256` of its normalized own text
  (spec decision 13).
- `text.md`: generated from `tree.json`: the section as it reads, verbatim, with no
  markup but the "# " that starts its heading line.
- `meta.yaml`: the fields listed in R1 and the spec's L0 `meta.yaml` (tier, role,
  version dates, retrieval date, eCFR "up to date as of", `verified_on`,
  `sha256_raw`, `sha256_text`, `official: false`).
- `overrides.yaml`: only if needed.

`regcorpus parse <section>` writes `tree.json` and `text.md` and sets `sha256_text`,
offline, from the stored raw XML (plan step 5). It writes a source's files only once
its parser goldens are in: today 1024.41's. It refuses 1024.30 and 1024.31 (exit 1,
nothing written) until plan step 9.

Local exploratory downloads go in `.cache/` (git-ignored); the committed raw XML
of L0 lives only under `sources/**/raw/`. A test fixture under `internal/*/testdata/`
(1024.41's govinfo granule, say) is not L0 (spec decision 14).

## Check that ends it
- `regcorpus fetch --verify` exits 0 (R1; spec decision 14). Exit 0 means eCFR's
  latest version is dated `source_version_date`, its tree (node paths in order, each
  node's `sha256`) and `sha256_text` match a fresh fetch (`sha256_raw` is
  informational), the govinfo 2026 edition's text, its layout deleted, hashes to
  `sha256_text`, `verified_on` is set, and there is no `overrides.yaml` (it prints
  "overrides: none"; one is refused until applying overrides is built, R1a). It covers
  1024.41 today: 1024.30 and 1024.31 are refused, exit 1, until plan step 9, and
  Supplement I is plan step 8's. Save the output.
- `make test` exits 0 with the parser goldens in it (R1a). Exit 0 then means
  every golden passes and the corrupted-sequence fixture is rejected. Each
  source has its own goldens: 1024.41 in plan step 4, Supplement I in plan
  step 8, and 1024.30/1024.31 (cite-only; the `1024-31-day` golden) in plan
  step 9. Until a source's goldens exist, `make test` passing says nothing
  about R1a for that source: do not report R1a met for it; name its plan step.
  From plan step 5, exit 0 also means 1024.41's committed `tree.json`, `text.md`
  and `sha256_text` are what `regcorpus parse` makes from its raw XML.
- `regcorpus fetch --latest` exits 0 (R1b; spec decision 14). Exit 0 means eCFR's
  latest version of each stored section is its stored one; a newer, an older or a
  removed latest version exits 1. A reissue of a stored date is printed as
  information only. From plan step 8 it also means the fresh Supplement I 41()
  slice hash equals the stored one.

## Who may run it
- A person or an agent, on the owner's machine (the cloud workspace cannot reach
  the upstream hosts). Later stages work offline from the committed XML.
- An agent may write parser code and goldens, run the checks above, and run
  `regcorpus parse`, which is offline and derives every file from the committed
  raw XML; a person reviews its output in the PR diff.
- A source defect is fixed only by a reviewed `overrides.yaml` entry, never in
  parser code and never by editing stored text.

## Requirements
- R1: verbatim storage and the metadata each source records.
- R1a: deterministic, rule-based parse. Labels count only in a paragraph's
  lead: at the start of a paragraph element, right after a heading past the
  dash or space that ends it, or right after another lead label; the lead
  ends at the first other text (spec decision 12). (i)/(v)/(x) resolved by
  sequence, and by the next label when it could be either; any skipped,
  repeated, out-of-order or unplaceable label fails; goldens are built from
  eCFR or official-edition XML only.
- R1b: the newer-version check, run before every release (stage 04).
- Spec "Granularity": one file set per CFR section; per-anchor hashes.
