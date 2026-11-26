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
//     case-sensitively (the owner's F3 call), to the own text regcorpus
//     parse derives for that label path from the source's stored raw XML,
//     byte for byte, and the meta.yaml that describes it (the chief of
//     staff's round-1 ruling, §1, which replaced F6's I2 with I3). It
//     checks the corpus root first (l0.CorpusRoot), then names the source
//     whose section ID (anchors.ID of its name, 1024-41) the ID is or
//     starts with and a "-"; a section's own ID is no anchor, and an
//     "interp" segment is plan step 8's under 1024.41 and no anchor under a
//     section Supplement I's slice has no comments on (Source.Comments);
//     then it derives the source again as fetch --verify's first check does
//     (ADR 0014): l0.Read and l0.State.Stored (meta.yaml names the source;
//     raw/ holds exactly the raw file, with sha256_raw's bytes), no
//     overrides.yaml, anchors.Build of the raw XML, and anchors.Derived.Stale
//     (tree.json, text.md and sha256_text are Build's), the raw XML being
//     the source's section; then it finds the node in the derived tree. Its
//     errors are typed (ErrNotRoot, ErrNoSource, ErrNotBuilt, ErrNoAnchor;
//     errors.Is), a stored file's refusal an *l0.Conflict, so C6's quote
//     matching (plan step 13), inventory --init (14), show (15) and verify
//     (19) can resolve anchors with the same meaning. It takes about 2 ms
//     for 1024.41, most of it anchors.Build.
//   - TextMain (text.go) prints the anchor's own text, verbatim, its
//     sha256, its children's anchor IDs (the owner's F1 call: never their
//     text) and every line of its source's meta.yaml as meta.Marshal
//     renders it, the stored bytes (F2); --json, one line through
//     sink.JSON, a JSON object whose meta is meta.JSON. It renders
//     everything before printing, so stdout holds nothing on an error, and
//     writes each line in one Write to the sink cmd/regcorpus gives it
//     (internal/sink), which keeps a write's error for cmd/regcorpus to exit
//     2 on. Exit codes: 0 printed; 1 no such anchor, one a later plan step
//     builds (1024.30's and 1024.31's anchors, plan step 9; Supplement I's
//     comment anchors on 1024.41, plan step 8), or a stored file that
//     refuses; 2 usage, not the corpus root, an ID that names no source,
//     or a file it can't read (the chief of staff's F4 and F5; round 1, §2
//     and §5).
//
// The source table is internal/ecfr's (sources.go), whose TextMain passes
// it in as a Corpus (the chief of staff's F7). This package never imports
// internal/ecfr, so nothing it imports is net/http: it reads the working
// tree, offline, and writes nothing. It reads a source's meta.yaml, raw
// XML, tree.json and text.md, and nothing under .cache/, registry/ or
// staging/. It holds no rule content.
//
// Not built here: show and its --staging (plan step 15), topics, with
// exact, case-insensitive term matching and no fuzzy search (plan step
// 20), Supplement I's comment anchors (plan step 8), 1024.30's and
// 1024.31's anchors (plan step 9), reading an embedded snapshot (plan step
// 25).
package lookup
