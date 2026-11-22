// Package ecfr is the client for the eCFR versioner API: version lists and
// point-in-time full/{date} XML for 12 CFR 1024.41, Supplement I to Part 1024,
// 1024.30 and 1024.31. It stores raw XML untouched and backs
// `regcorpus fetch --verify` and `regcorpus fetch --latest`.
//
// Requirements: R1 (verbatim storage and source metadata), R1b (newer-version
// check). Built in plan steps 2, 3, 6 and 8 (see the TODOs below).
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
//     entry in raw/, a meta.yaml that isn't fetch's, lacks its raw file, or
//     names another date or sha256) exits 1 and stores nothing. A stored file
//     is never overwritten (store.go, by hard link).
//   - Beside it, when missing, fetch writes meta.yaml's fetch-owned fields in
//     the spec's order: cfr, tier, role, source, official, source_version_date,
//     retrieved_at (UTC), ecfr_up_to_date_as_of, sha256_raw. internal/anchors
//     adds sha256_text (plan step 5).
//   - --date fetches that point in time into .cache/ecfr/title-12/ (git
//     ignores it), never into sources/, and writes no meta.yaml.
//
// TODO(plan step 3): version_effective_date and version_fr_citations, from
// the Federal Register. TODO(plan step 6): `fetch --verify` (verified_on) and
// `fetch --latest` (R1b). TODO(plan step 8): Supplement I, an appendix source
// fetched whole.
package ecfr
