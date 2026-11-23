// Package l0 is a stored L0 source on disk (R1; ADR 0010): how a file under
// sources/ is stored, read and checked, and the one file-level writer of
// meta.yaml, whose schema internal/meta owns. Every command that writes a
// stored L0 file goes through it: fetch (internal/ecfr) and meta set (plan
// step 3) now; internal/anchors (sha256_text, plan step 5) and fetch
// --verify (verified_on, plan step 6) later. It imports only internal/meta.
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
//
// The source table (cfr, tier, role per source) is unexported, in
// internal/ecfr (sources.go); a caller passes each Source from it. fetch,
// meta set and fetch --verify (plan step 6) are in internal/ecfr; how plan
// step 5's writer (internal/anchors) gets its Source, the table exported or
// moved, is step 5's choice. A refusal is a *Conflict (exit 1). It holds no
// rule content.
package l0
