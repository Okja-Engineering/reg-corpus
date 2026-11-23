// Package ecfr is the client for the eCFR versioner API: version lists and
// point-in-time full/{date} XML for 12 CFR 1024.41, Supplement I to Part 1024,
// 1024.30 and 1024.31. It stores raw XML untouched and backs
// `regcorpus fetch --verify` and `regcorpus fetch --latest`.
//
// Requirements: R1 (verbatim storage and source metadata), R1b (newer-version
// check). Built in plan steps 2, 3, 6 and 8 (see the TODOs below).
//
// Who writes meta.yaml (ADR 0010): internal/meta owns its schema and
// internal/l0 the file; every command here reads it through l0.Read
// (meta.Parse) and writes what meta.Marshal returns, through l0. fetch
// creates it with the fetch-time fields (l0.StoreOnce); meta set sets
// version_effective_date and version_fr_citations (plan step 3, l0.Update);
// plan step 5 (internal/anchors) adds sha256_text and plan step 6 (fetch
// --verify) verified_on, each through l0.Update, each keeping the fields it
// doesn't set. Before writing anything, each checks the same stored state
// (l0's Stored).
//
// Built (plan step 2): `regcorpus fetch [--date YYYY-MM-DD] [section ...]`
// for 1024.41, 1024.30 and 1024.31 (sources.go holds the fixed source table).
//   - The client (client.go) asks the versioner API (DefaultBaseURL) for a
//     section's version list, title 12's "up to date as of" date
//     (titles.json, refused unless it says eCFR is not mid-import) and the
//     full XML at a date. It sends Accept-Encoding: gzip and a generic
//     User-Agent, follows redirects only within the origin asked, decides on
//     the status before reading, caps what it reads, decodes gzip itself, and
//     returns the body exactly as served, after checking it is valid UTF-8 and
//     one XML document whose root element is the source's
//     (<DIV8 TYPE="SECTION" N="<section>">). document() checks the form with
//     parse.WellFormed, the one well-formedness check, which Parse also runs
//     first and meta set runs on the raw XML it reads citations from (its doc
//     says what it checks), and the root element itself.
//     WellFormed lives in internal/parse because fetch may import parse but
//     parse must not import this package, which holds the network client.
//     An HTTP error, malformed JSON or a bad body is an error naming the
//     URL; nothing falls back.
//   - Every fetch first reads titles.json, and fetches no date after the one
//     eCFR is up to date as of.
//   - fetch stores the latest version's XML at
//     sources/ecfr/title-12/<section>/raw/<date>.xml, <date> being the latest
//     date in the version list (meta.yaml's source_version_date). It exits 0
//     only when raw/ then holds exactly one entry, that file, as a regular
//     file with eCFR's bytes, and meta.yaml names its date and sha256. It
//     reads the source directory once; whatever contradicts that (another
//     entry in raw/, or a meta.yaml that lacks its raw file or that the next
//     item's checks refuse) exits 1 and stores nothing. A stored file is
//     never overwritten (l0.StoreOnce, by hard link), and every directory on
//     the way to one, from sources/ down and under .cache/, must be a real
//     directory, not a symlink or a file (l0.RealDirs, the one check of that;
//     exit 1 otherwise, before any request for sources/ecfr/title-12/). A
//     root without sources/ecfr/title-12/ is not the corpus root (exit 2).
//   - Beside it, when missing, fetch writes meta.yaml's fetch-owned fields in
//     the spec's order: cfr, tier, role, source, official, source_version_date,
//     retrieved_at (UTC), ecfr_up_to_date_as_of, sha256_raw. When meta.yaml
//     exists, fetch makes the check every writer of one makes (l0's Stored):
//     it is in the spec's schema, names the source as the source table does
//     (cfr, tier, role, source, official), and describes raw/'s one file,
//     raw/<source_version_date>.xml with sha256_raw's bytes; and that file
//     must be eCFR's latest, with eCFR's bytes (exit 1 otherwise). It never
//     rewrites meta.yaml, and keeps the fields other steps set.
//   - --date fetches that point in time into .cache/ecfr/title-12/ (git
//     ignores it), never into sources/, and writes no meta.yaml.
//
// Built (plan step 3; ADRs 0010 and 0011): `regcorpus meta
// set <section> <field> <value ...>` (metaset.go) sets version_effective_date
// or version_fr_citations in a stored source's meta.yaml and nothing else;
// every other field is refused (exit 2). Through l0.Update, under the
// source's lock, it first makes the stored-state check fetch makes (exit 1
// otherwise), then sets the field and replaces meta.yaml whole; setting the
// value a field has is a no-op. version_effective_date is set as given; only
// its form is checked (internal/meta). Each citation, each given once, must
// be cited in a citation note (<CITA>) of the raw XML meta.yaml describes
// (citationNote reads each run of a note's text on its own, shorthand pages
// too), and raw XML that is not well-formed, which parse.WellFormed (fetch's
// check) refuses, refuses the citations (exit 1); that checks membership
// only, not that the page is the amendment that produced the version. The
// values are read from the Federal Register and eCFR outside the tool (plan
// step 3's diff record, in its PR); meta set checks only what L0 itself can
// check.
//
// TODO(plan step 6): `fetch --verify` (verified_on) and
// `fetch --latest` (R1b). TODO(plan step 8): Supplement I, an appendix source
// fetched whole.
package ecfr
