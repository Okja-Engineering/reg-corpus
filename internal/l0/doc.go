// Package l0 is a stored L0 source on disk (R1; ADR 0010): how a file under
// sources/ is stored, read and checked, and the one file-level writer of
// meta.yaml, whose schema internal/meta owns. Every command that writes a
// stored L0 file goes through it: fetch (internal/ecfr), meta set (plan
// step 3), parse (internal/anchors: tree.json, text.md and sha256_text,
// plan step 5) and fetch --verify (verified_on, plan step 6, ADR 0014),
// which compares tree.json and text.md with what is derived from the raw
// XML again through State.Stale, the comparison Update's writes make, and
// then keeps the govinfo granule it compared under .cache/govinfo/
// (StoreOnce), a git-ignored cache. It imports only internal/meta.
//
//   - Every directory on the way must be a real one below the root, not a
//     symlink or a file, reached with no "." or ".." and found by its exact
//     name among its parent's entries; one that differs from its name only
//     in case ("RAW", "TITLE-12") is refused (RealDirs, the one check of
//     the way). "In case" is Unicode simple case folding (strings.EqualFold,
//     so "ſ", U+017F, is "s"); a known limit: APFS folds fully too, so a
//     name APFS takes as another by a fold of one character into two ("ß"
//     and "ss", "ﬁ" and "fi") is not refused. No name the corpus uses holds
//     such a character. StoreOnce stores a file that is not there yet, by hard link,
//     and never overwrites one; one there is compared only as a regular
//     file, never through a symlink: fetch stores raw XML so, and a source's
//     first meta.yaml. It finds that file by exact name too, and refuses a
//     name that differs from it only in case ("2025-07-15.XML"), whatever it
//     is, so what it stores or finds unchanged is the name git records. An
//     existing meta.yaml changes only through Update.
//   - Read reads a source's directory once: its entries, the entries of raw/
//     and its meta.yaml, parsed by internal/meta. It finds meta.yaml,
//     tree.json, text.md and overrides.yaml by exact name, as RealDirs
//     finds directories, and refuses a name that differs from one only in
//     case, whatever it is. Stored is the check every writer of a meta.yaml
//     makes before writing anything: the meta.yaml names the source as the
//     source table does (cfr, tier, role, source, official;
//     Source.Identity), and raw/ holds exactly
//     raw/<source_version_date>.xml, a regular file with sha256_raw's bytes.
//   - Update is how an existing meta.yaml is changed, and the only writer of
//     a source's derived files (tree.json, text.md): under the source's lock
//     (flock(2) on its directory, so one Update per source at a time, across
//     processes), it reads, checks (Stored), lets the caller set its fields
//     and return the derived files' bytes, built from the raw XML Stored
//     checked, then checks that each derived file is missing or a regular
//     file before writing any, and replaces each file whose bytes differ
//     whole (a temporary file renamed over it), so no update is lost and no
//     reader sees half a file. No exported function replaces a file at a
//     path its caller names: StoreOnce, given a path, stores a file that is
//     not there and never replaces one, so raw XML is never overwritten,
//     meta.yaml is only what internal/meta renders, and a derived file is
//     only the source's own. A refusal (*Conflict) is found before the
//     first write; an error while writing is not one, names the file it was
//     writing, and Update reports the files it replaced before it.
//
// The source table (cfr, tier, role per source) is unexported, in
// internal/ecfr (sources.go); a caller passes each Source from it. fetch,
// meta set, parse, fetch --verify and fetch --latest have their handlers in
// internal/ecfr; parse's passes its Source to internal/anchors, which does
// the work, so the table stays where it is (the chief of staff's choice,
// ADR 0013). A refusal is a *Conflict (exit 1). It holds no rule content.
package l0
