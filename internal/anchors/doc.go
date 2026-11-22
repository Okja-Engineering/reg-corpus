// Package anchors turns a parsed tree into anchors and text.md, normalizes
// text (NFC, collapsed whitespace, ASCII quotes and dashes), and computes the
// per-node sha256 of normalized own text and the source's sha256_text, which
// it adds to meta.yaml. `regcorpus fetch` (internal/ecfr, plan step 2) writes
// meta.yaml's fetch-time fields; plan steps 3 and 6 add theirs. Which code
// owns merging them is open until plan step 3.
//
// Requirements: R1 (hashes and metadata), R1a (text.md generated from
// tree.json), spec "Definitions" (anchor, own text, normalized text). Built in
// plan step 5. No code yet (plan step 1 scaffold).
package anchors
