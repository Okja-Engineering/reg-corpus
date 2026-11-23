// Package anchors turns a parsed tree into anchors and text.md, normalizes
// text (NFC, collapsed whitespace, ASCII quotes and dashes), and computes the
// per-node sha256 of normalized own text and the source's sha256_text. It
// sets sha256_text in meta.yaml through l0.Update (internal/l0, the file's
// one writer; internal/meta, its schema; ADR 0010, plan step 5), which checks
// the stored state, sets sha256_text and keeps every other field. Update
// takes the source's Source, from the source table, which is unexported in
// internal/ecfr (sources.go): exporting or moving it is step 5's choice. `regcorpus fetch`
// (internal/ecfr, plan step 2) writes the fetch-time fields, `regcorpus meta
// set` (plan step 3) version_effective_date and version_fr_citations, and
// `fetch --verify` (plan step 6) verified_on.
//
// Requirements: R1 (hashes and metadata), R1a (text.md generated from
// tree.json), spec "Definitions" (anchor, own text, normalized text). Built in
// plan step 5. No code yet (plan step 1 scaffold).
package anchors
