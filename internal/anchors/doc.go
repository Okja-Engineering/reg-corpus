// Package anchors turns a parsed tree into anchors and text.md, normalizes
// text (NFC, collapsed whitespace, ASCII quotes and dashes), and computes the
// per-node sha256 of normalized own text and the source's sha256_text. It also
// writes meta.yaml.
//
// Requirements: R1 (hashes and metadata), R1a (text.md generated from
// tree.json), spec "Definitions" (anchor, own text, normalized text). Built in
// plan step 5. No code yet (plan step 1 scaffold).
package anchors
