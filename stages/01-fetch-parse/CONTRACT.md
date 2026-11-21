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
- The govinfo CFR 2026 annual edition, to set `verified_on` (R1).
- The Federal Register, for `version_fr_citations` and `version_effective_date` (R1).
- `overrides.yaml` per source, if a source defect needs one (R1a). Each entry
  has a location, a replacement and a reason, and is reviewed.

## Outputs
Per source directory: `sources/ecfr/title-12/1024.41/`,
`sources/ecfr/title-12/supplement-i/41/`, `sources/ecfr/title-12/1024.30/`,
`sources/ecfr/title-12/1024.31/`:
- `raw/<date>.xml`: the fetched XML, untouched (Supplement I: the whole appendix).
- `tree.json`: the parsed tree; each node carries the `sha256` of its normalized own text.
- `text.md`: generated from `tree.json`.
- `meta.yaml`: the fields listed in R1 and the spec's L0 `meta.yaml` (tier, role,
  version dates, retrieval date, eCFR "up to date as of", `verified_on`,
  `sha256_raw`, `sha256_text`, `official: false`).
- `overrides.yaml`: only if needed.

Local exploratory downloads go in `.cache/` (git-ignored); committed raw XML
lives only under `sources/**/raw/`.

## Check that ends it
- `regcorpus fetch --verify` exits 0 (R1; built in plan step 6). Exit 0 means the
  version date, tree and `sha256_text` match a fresh fetch (`sha256_raw` is
  informational), `verified_on` is set, and every override is listed. Save the output.
- `make test` exits 0 with the parser goldens in it (R1a; goldens built in plan
  steps 4, 8 and 9). Exit 0 then means every golden passes and the
  corrupted-sequence fixture is rejected. Until the goldens exist (plan step 4),
  `make test` passing says nothing about R1a: do not report R1a met; name plan
  step 4 instead.
- `regcorpus fetch --latest` exits 0 (R1b; built in plan step 6). Exit 0 means eCFR
  lists no newer version of any stored section, and the fresh Supplement I 41()
  slice hash equals the stored one.

## Who may run it
- A person or an agent, on the owner's machine (the cloud workspace cannot reach
  the upstream hosts). Later stages work offline from the committed XML.
- An agent may write parser code and goldens, and run the checks above.
- A source defect is fixed only by a reviewed `overrides.yaml` entry, never in
  parser code and never by editing stored text.

## Requirements
- R1: verbatim storage and the metadata each source records.
- R1a: deterministic, rule-based parse. Labels count only at the start of a
  paragraph element or right after a heading dash; (i)/(v)/(x) resolved by
  sequence; any skipped, repeated, out-of-order or unplaceable label fails;
  goldens are built from eCFR or official-edition XML only.
- R1b: the newer-version check, run before every release (stage 04).
- Spec "Granularity": one file set per CFR section; per-anchor hashes.
