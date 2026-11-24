// Package l0 is a stored L0 source on disk (R1; ADR 0010): how a file under
// sources/ is stored, read and checked, and the one file-level writer of
// meta.yaml, whose schema internal/meta owns. Every command that writes a
// stored L0 file goes through it: fetch (internal/ecfr), meta set (plan
// step 3) and parse (internal/anchors: tree.json, text.md and sha256_text,
// plan step 5) now; fetch --verify (verified_on, plan step 6) later. It
// imports only internal/meta.
//
//   - Every directory on the way must be a real one, not a symlink or a file
//     (RealDirs). StoreOnce stores a file that is not there yet, by hard
//     link, and never overwrites one: fetch stores raw XML so, and a
//     source's first meta.yaml. An existing meta.yaml changes only through
//     Update.
//   - Read reads a source's directory once: the entries of raw/ and its
//     meta.yaml, parsed by internal/meta. Stored is the check every writer of
//     a meta.yaml makes before writing anything: the meta.yaml names the
//     source as the source table does (cfr, tier, role, source, official;
//     Source.Identity), and raw/ holds exactly raw/<source_version_date>.xml,
//     a regular file with sha256_raw's bytes.
//   - Update is how an existing meta.yaml is changed: under the source's lock
//     (flock(2) on its directory, so one Update per source at a time, across
//     processes), it reads, checks (Stored), sets the caller's fields and
//     replaces the file whole (Replace: a temporary file renamed over it), so
//     no update is lost and no reader sees half a file.
//   - Derive writes a source's derived files (tree.json, text.md) inside
//     Update's set, so under the same lock and from the raw XML Stored has
//     just checked: it refuses anything but a missing or regular file at
//     each path before writing any, then replaces only those whose bytes
//     differ (Replace).
//
// The source table (cfr, tier, role per source) is unexported, in
// internal/ecfr (sources.go); a caller passes each Source from it. fetch,
// meta set, parse and fetch --verify (plan step 6) have their handlers in
// internal/ecfr; parse's passes its Source to internal/anchors, which does
// the work, so the table stays where it is (the chief of staff's choice,
// ADR 0013). A refusal is a *Conflict (exit 1). It holds no rule content.
package l0
