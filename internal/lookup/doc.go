// Package lookup implements the deterministic read commands: `regcorpus show
// <ID>[@rev]` (with --staging), `regcorpus text <anchor>` and `regcorpus topics
// <term>`. Plain text by default, --json for tools; never prints more than
// the records asked for.
//
// Requirements: R8 (lookup). Built in plan steps 7, 15 and 20.
//
// Built (plan step 7; ADR 0015): `regcorpus text [--json] <anchor>` and the
// anchor resolver it is the first caller of.
//   - Corpus.Anchor (anchor.go) resolves an anchor ID, matched exactly and
//     case-sensitively (the owner's F3 call), to its source and node: the
//     source whose section ID (anchors.ID of its name, 1024-41) the ID is,
//     or starts with and a "-"; then, from that source's meta.yaml and
//     tree.json alone, the node whose anchors.ID it is. Before resolving it
//     checks what it reads (the chief of staff's F6): l0.Read, the
//     meta.yaml naming the source (l0.State.Names), tree.json (as Read
//     found it, l0.State.TreeJSON) decoding strictly for the source's
//     section, with no null node, none without a label path and no anchor
//     ID twice, and the tree giving meta.yaml's sha256_text
//     (anchors.SHA256Text); then the node's sha256 is anchors.Hash of its
//     own text. Its misses are typed (ErrNoSource, ErrNotBuilt,
//     ErrNoAnchor; errors.Is), a stored file's refusal an *l0.Conflict, so
//     C6's quote matching (plan step 13), inventory --init (14), show (15)
//     and verify (19) can resolve anchors with the same meaning.
//   - TextMain (text.go) prints the anchor's own text, verbatim, its
//     sha256, its children's anchor IDs (the owner's F1 call: never their
//     text) and every line of its source's meta.yaml as meta.Marshal
//     renders it, the stored bytes (F2); --json, one line, a JSON object
//     whose meta is meta.JSON. It renders everything before printing, so
//     stdout holds nothing on an error, and writes each line in one Write
//     to the sink cmd/regcorpus gives it (internal/sink). Exit codes: 0
//     printed; 1 no such anchor, one a later plan step builds (1024.30's
//     and 1024.31's anchors, plan step 9; Supplement I's comment anchors,
//     plan step 8), or a stored file that refuses; 2 usage, an ID that
//     names no source, not the corpus root (l0.CorpusRoot), or a file it
//     can't read (the chief of staff's F4 and F5).
//
// The source table is internal/ecfr's (sources.go), whose TextMain passes
// it in as a Corpus (the chief of staff's F7). This package never imports
// internal/ecfr, so nothing it imports is net/http: it reads the working
// tree, offline, and writes nothing. It reads no raw XML, text.md, .cache/,
// registry/ or staging/. It holds no rule content.
//
// Not built here: show and its --staging (plan step 15), topics, with
// exact, case-insensitive term matching and no fuzzy search (plan step
// 20), Supplement I's comment anchors (plan step 8), 1024.30's and
// 1024.31's anchors (plan step 9), reading an embedded snapshot (plan step
// 25).
package lookup
