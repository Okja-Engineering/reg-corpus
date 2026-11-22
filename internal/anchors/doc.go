// Package anchors turns a parsed tree into anchors and text.md, normalizes
// text (NFC, collapsed whitespace, ASCII quotes and dashes), and computes the
// per-node sha256 of normalized own text and the source's sha256_text. It
// sets sha256_text in meta.yaml through internal/meta, the file's one writer
// (ADR 0010; this departs from plan step 5's "Write meta.yaml"): parse,
// set sha256_text, marshal, keeping every other field. `regcorpus fetch`
// (internal/ecfr, plan step 2) writes the fetch-time fields, `regcorpus meta
// set` (plan step 3) version_effective_date and version_fr_citations, and
// `fetch --verify` (plan step 6) verified_on.
//
// Requirements: R1 (hashes and metadata), R1a (text.md generated from
// tree.json), spec "Definitions" (anchor, own text, normalized text). Built in
// plan step 5. No code yet (plan step 1 scaffold).
package anchors
