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
//     of a text's normalized UTF-8 bytes. Both refuse text that isn't valid
//     UTF-8 with an error, rather than read it as U+FFFD.
//   - ID (anchors.go): the section with "." made "-", then the label path
//     joined by "-": 1024-41-b-2-i-B, and for the italic level
//     1024-41-c-2-v-A-1.
//   - Operative: a node's own text minus its label, the one space before
//     its heading, the heading, and the one dash or space that ends the
//     heading is not empty (the owner's F1 call). It is decided on the whole
//     own text normalized as one string, past the normalized label, space
//     and heading, so an em or en dash, a no-break space or a run of
//     White_Space is that one separator, and a heading's own trailing
//     White_Space is the heading's one space. Same path, same heading and
//     same normalized own text give the same verdict. 75 of 1024.41's 91
//     nodes are operative.
//   - Document: a tree's nodes in document order, each before its children,
//     the one notion of document order and of anchors consecutive in it.
//     Runs splits a cite into runs of consecutive anchors; Join gives one
//     string per run, own texts with nothing between two in one <P> and one
//     space between <P>s (the owner's F2 call), raw, so a quote is matched
//     within one run's text normalized as one string (N6), and never crosses
//     a join between anchors that aren't consecutive. Both refuse an empty
//     cite, a nil anchor, one not in the document and one given twice, each
//     with its named error.
//   - Build (build.go): parse.Parse, then each node's sha256 is Hash of its
//     whole own text, its heading's separator included (the owner's F1 call);
//     tree.json is the tree's JSON form (EncodeTree: two-space indent, no
//     HTML escaping, a trailing LF); text.md is the section as it reads
//     (TextMD: "# " and the HEAD, each <P>'s text, the CITA, each after a
//     blank line, verbatim, with no markup but the "# " that starts the
//     heading line; the chief of staff's F5 choice, T1, ending with exactly
//     one LF), and a HEAD, a node's own text or a CITA that holds a line
//     break (but one LF ending the CITA), or nothing but White_Space (an
//     empty line), is refused, not rewritten; sha256_text is Hash of the
//     HEAD, one space and the whole document joined as one run (SectionText;
//     the owner's F3 call, the CITA left out), normalized as one string. The
//     same raw bytes give the same bytes out.
//   - Write (write.go): `regcorpus parse` (internal/ecfr, parsecmd.go) calls
//     it with a source from the source table. Through l0.Update, under the
//     source's lock and after the stored-state check (ADR 0010), it builds
//     from the raw XML that check read, sets sha256_text, keeping every
//     other field, and hands Update tree.json's and text.md's bytes, which
//     Update alone writes (only what differs). It refuses (*l0.Conflict,
//     nothing written) what Update refuses; a source with an overrides.yaml,
//     since applying overrides isn't built (R1a); and raw XML that Build
//     refuses, with the parser's location or where text.md would break a
//     line or show an empty one.
//   - Derived.Stale (write.go) is the one comparison of a source's stored
//     L0 with what Build derived from its raw XML: tree.json and text.md
//     (l0.State.Stale) and meta.yaml's sha256_text, naming each that
//     differs. fetch --verify (plan step 6, ADR 0014) refuses to vouch, and
//     `regcorpus text` (internal/lookup, plan step 7, ADR 0015) to print,
//     while it names any.
//
// Operative and ID understand regulation paragraphs only, whose own text
// starts with a parenthesized label. Supplement I's comment numbering
// ("1.", "i."; plan step 8) and definition anchors (1024-31-day; plan step
// 9) extend them.
//
// This package imports internal/parse, internal/l0, internal/meta and
// x/text, never internal/ecfr, so `fetch --verify` (internal/ecfr, plan
// step 6, ADR 0014) builds the stored raw XML again with Build, refusing
// stored files that aren't Build's (as `regcorpus text` does, from
// internal/lookup, which has no network code), rebuilds a fresh fetch with
// Build and compares its nodes with the stored tree's, compares govinfo's
// paragraphs with Document.Paragraphs one by one, and hashes govinfo's
// heading and paragraphs as sha256_text is (JoinSection, the one join
// SectionText uses too).
//
// Not built here: applying overrides (the first source that needs one);
// Supplement I's comment anchors (plan step 8); 1024.30's and 1024.31's
// derived files and 1024.31's definition anchors (plan step 9); C6's quote
// matching, within one run of a cite's Join, and its hash checks (plan step
// 13); the conditional-lead-in test (plan step 13); inventory --init (plan
// step 14).
package anchors
