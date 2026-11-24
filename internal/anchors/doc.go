// Package anchors turns a parsed tree into anchors and a source's derived L0
// files: it normalizes text, names anchors, decides which are operative,
// joins own texts for a multi-anchor cite, and builds tree.json (each node
// with the sha256 of its normalized own text), text.md and sha256_text from
// a source's raw XML. It never reads what the text means.
//
// Requirements: R1 (hashes and metadata), R1a (text.md generated from
// tree.json), spec "Definitions" (anchor, own text, operative anchor,
// normalized text, multi-anchor cite) and "Granularity" (per-node hashes).
// Built in plan step 5; the owner's calls and the chief of staff's choices
// are ADR 0013.
//
//   - Normalize (normalize.go): Unicode NFC (golang.org/x/text, the module's
//     one dependency), then U+2018 and U+2019 to ', U+201C and U+201D to ",
//     U+2013 and U+2014 to -, then every run of Unicode White_Space to one
//     space, nothing trimmed (pins N1–N5). Hash is the lowercase hex sha256
//     of a text's normalized UTF-8 bytes.
//   - ID (anchors.go): the section with "." made "-", then the label path
//     joined by "-": 1024-41-b-2-i-B, and for the italic level
//     1024-41-c-2-v-A-1.
//   - Operative: a node's own text minus its label, the one space before
//     its heading, the heading, and the one "—" or space that ends the
//     heading is not empty (the owner's F1 call, O2). 75 of 1024.41's 91
//     nodes are operative.
//   - Join: own texts in document order, nothing between two in one <P>,
//     one space between <P>s (the owner's F2 call, J2), raw; a cite's text is
//     the join normalized as one string (N6).
//   - Build (build.go): parse.Parse, then each node's sha256 is Hash of its
//     whole own text, its heading's separator included (the owner's F1 call,
//     H1); tree.json is the tree's JSON form (EncodeTree: two-space indent,
//     no HTML escaping, a trailing LF); text.md is the section as it reads
//     (TextMD: "# " and the HEAD, each <P>'s text, the CITA, each after a
//     blank line, verbatim; the chief of staff's F5 choice, T1); sha256_text
//     is Hash of the HEAD, one space and every node joined (SectionText; the
//     owner's F3 call, S1, the CITA left out). The same raw bytes give the
//     same bytes out.
//   - Write (write.go): `regcorpus parse` (internal/ecfr, parsecmd.go) calls
//     it with a source from the source table. Through l0.Update, under the
//     source's lock and after the stored-state check (ADR 0010), it builds
//     from the raw XML that check read, writes tree.json and text.md
//     (l0.Derive, only what differs) and sets sha256_text, keeping every
//     other field. It refuses (*l0.Conflict, nothing written) a source with
//     an overrides.yaml, since applying overrides isn't built (R1a); raw XML
//     that doesn't parse, with the parser's location; and anything but a
//     regular file where tree.json or text.md goes.
//
// This package imports internal/parse, internal/l0, internal/meta and
// x/text, never internal/ecfr, so plan step 6's `fetch --verify`
// (internal/ecfr) can rebuild a fetched source with Build and compare.
//
// Not built here: applying overrides (the first source that needs one);
// Supplement I's comment anchors (plan step 8); 1024.30's and 1024.31's
// derived files and 1024.31's definition anchors (plan step 9); C6's quote
// matching, which crosses a join only between anchors consecutive in the
// document, and its hash checks (plan step 13); the conditional-lead-in
// test (plan step 13); inventory --init (plan step 14).
package anchors
