// Package ecfr is the client for the eCFR versioner API: version lists and
// point-in-time full/{date} XML for 12 CFR 1024.41, Supplement I to Part 1024,
// 1024.30 and 1024.31. It stores raw XML untouched and backs
// `regcorpus fetch --verify` and `regcorpus fetch --latest`.
//
// Requirements: R1 (verbatim storage and source metadata), R1b (newer-version
// check). Built in plan steps 2, 3, 6 and 8 (see the TODOs below).
//
// Who writes meta.yaml (ADR 0010): internal/meta owns its schema and is its
// one writer; every command here reads it through meta.Parse and writes what
// meta.Marshal returns. fetch creates it with the fetch-time fields; meta set
// sets version_effective_date and version_fr_citations (plan step 3); plan
// step 5 (internal/anchors) adds sha256_text and plan step 6 (fetch --verify)
// verified_on, each through internal/meta, each keeping the fields it
// doesn't set.
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
//     (<DIV8 TYPE="SECTION" N="<section>">); document() says what that
//     checks. An HTTP error, malformed JSON or a bad body is an error naming
//     the URL; nothing falls back.
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
//     never overwritten (store.go, by hard link), and every directory on
//     the way to one, from sources/ down and under .cache/, must be a real
//     directory, not a symlink or a file (realDirs, the one check of that;
//     exit 1 otherwise, before any request for sources/ecfr/title-12/). A
//     root without sources/ecfr/title-12/ is not the corpus root (exit 2).
//   - Beside it, when missing, fetch writes meta.yaml's fetch-owned fields in
//     the spec's order: cfr, tier, role, source, official, source_version_date,
//     retrieved_at (UTC), ecfr_up_to_date_as_of, sha256_raw. When meta.yaml
//     exists it must be in the spec's schema, and fetch checks every field it
//     writes but the two facts of the run that wrote it (retrieved_at,
//     ecfr_up_to_date_as_of): cfr, tier, role, source and official against
//     the source table, source_version_date and sha256_raw against the raw
//     file (exit 1 otherwise). It never rewrites meta.yaml, and keeps the
//     fields other steps set.
//   - --date fetches that point in time into .cache/ecfr/title-12/ (git
//     ignores it), never into sources/, and writes no meta.yaml.
//
// Built (plan step 3; issues #5 and #6, ADRs 0010 and 0011): `regcorpus meta
// set <section> <field> <value ...>` (metaset.go) sets version_effective_date
// or version_fr_citations in a stored source's meta.yaml and nothing else;
// every other field is refused (exit 2). Each citation must be cited in the
// citation note (<CITA>) of the raw XML meta.yaml describes,
// raw/<source_version_date>.xml with sha256_raw's bytes, read through real
// directories as a regular file (exit 1 otherwise). It replaces meta.yaml
// whole (store.go: replace, a temporary file renamed over it); setting the
// value a field has is a no-op. The values are read from the Federal Register
// and eCFR outside the tool (plan step 3's diff record, in its PR); meta set
// checks only what L0 itself can check.
//
// TODO(plan step 6): `fetch --verify` (verified_on) and
// `fetch --latest` (R1b). TODO(plan step 8): Supplement I, an appendix source
// fetched whole.
package ecfr
