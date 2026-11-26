// Package ecfr is the client for the eCFR versioner API: version lists and
// point-in-time full/{date} XML for 12 CFR 1024.41, Supplement I to Part 1024,
// 1024.30 and 1024.31, and for govinfo's annual-edition granules. It stores
// raw XML untouched and backs `regcorpus fetch --verify` and `regcorpus
// fetch --latest`.
//
// Requirements: R1 (verbatim storage and source metadata), R1b (newer-version
// check). Built in plan steps 2, 3, 5 and 6; Supplement I in plan step 8
// (see the TODO below).
//
// Who writes meta.yaml (ADR 0010): internal/meta owns its schema and
// internal/l0 the file; every command here reads it through l0.Read
// (meta.Parse) and writes what meta.Marshal returns, through l0. fetch
// creates it with the fetch-time fields (l0.StoreOnce); meta set sets
// version_effective_date and version_fr_citations (plan step 3, l0.Update);
// parse (plan step 5, internal/anchors) sets sha256_text and plan step 6
// (fetch --verify) verified_on, each through l0.Update, each keeping the
// fields it doesn't set. Before writing anything, each checks the same
// stored state (l0's Stored).
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
//     first and meta set runs on the raw XML it reads citations from
//     (WellFormed's doc says what it checks), and the root element itself.
//     WellFormed lives in internal/parse because fetch may import parse but
//     parse must not import this package, which holds the network client. An
//     HTTP error, malformed JSON or a bad body is an error naming the URL;
//     nothing falls back.
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
//     directory, not a symlink or a file, found by its exact name
//     (l0.RealDirs, the one check of that; exit 1 otherwise, before any
//     request for sources/ecfr/title-12/). A name that differs only in case
//     from a directory's on the way ("RAW", "TITLE-12"), or from meta.yaml,
//     tree.json, text.md or overrides.yaml in a source's directory (l0.Read),
//     or from the file fetch stores ("2025-07-15.XML", --date's under .cache/
//     too; l0.StoreOnce), exits 1 too, with nothing stored: a
//     case-insensitive filesystem would store under, or report as unchanged,
//     a name git doesn't record. A root without sources/ecfr/title-12/ is not the
//     corpus root (exit 2).
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
// source's lock, it first makes the stored-state check fetch makes, and
// refuses a name in another case as fetch does (exit 1 otherwise), then
// sets the field and replaces meta.yaml whole; setting the
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
// Built (plan step 5; ADR 0013): `regcorpus parse <section> ...`
// (parsecmd.go), offline, the chief of staff's choice of command. Its handler
// is here because the source table is; it passes each named source to
// internal/anchors' Write, which, through l0.Update, writes tree.json and
// text.md and sets sha256_text from the stored raw XML. Sections are named,
// never defaulted, and it writes only a source whose parser goldens are in
// (the table's Goldens is empty): today 1024.41. 1024.30 and 1024.31 are
// refused (exit 1, "goldens are plan step 9's"), with nothing written. A
// source that fails the stored-state check, whose raw XML doesn't parse (the
// parser's line and column) or holds a line break or an empty line text.md
// can't show, that holds an overrides.yaml, or a name like a source file's
// or a directory's on the way in another case, exits 1 with nothing written
// for it; no section, an unknown one or a flag exits 2, as does a write
// that fails part-way, after printing the files it wrote, its error naming
// the file it was writing.
//
// Built (plan step 6; ADR 0014): `regcorpus fetch --verify` (verify.go,
// R1) and `regcorpus fetch --latest` (newer.go, R1b), flags of fetch that
// exclude each other and --date. Both read titles.json first, as every
// fetch does (exit 2 mid-import).
//   - --verify, under the source's lock (l0.Update), refuses before any
//     other request a source whose tree is a later plan step's (Goldens:
//     1024.30 and 1024.31, plan step 9), an overrides.yaml, a meta.yaml
//     without sha256_text and a tree.json that isn't as parse writes it
//     (anchors.DecodeTree). Then eCFR: its latest version must be dated
//     source_version_date (a removal, a *RemovedError, or another date is a
//     *Finding, exit 1); that version's XML, built as parse builds it
//     (anchors.Build), must have tree.json's node paths in order with their
//     sha256s, and the stored sha256_text (F6: i, T2). The normalized text
//     is the version (F1: A): sha256_raw, fresh and stored, and the nodes
//     whose raw text differs are printed as information only. Then, only
//     if that passes (F4 c), govinfo: Client.Granule asks govinfo's link
//     service for the section's 2026-edition granule (F2: G1), on govinfo's
//     own origin, through the same get as eCFR; it is kept under
//     .cache/govinfo/ by its sha256 (l0.StoreOnce) and read by
//     internal/govinfo (its checks and the layout rule, F3: C1), and its
//     heading and paragraphs, joined and hashed as sha256_text is
//     (anchors.JoinSection), must give the stored sha256_text. --verify
//     sets only verified_on: the run's UTC date on a match, unless a date
//     is there already (F5: V-a); null on a mismatch, over any date (F4 a),
//     printing each differing paragraph normalized (F4 b), and a *Finding
//     (exit 1 that writes; F8: E-a). Every other refusal or finding writes
//     nothing; a network, eCFR or govinfo error is exit 2.
//   - --latest reads each source's meta.yaml, after the stored-state check,
//     and its version list, and writes nothing. A version dated after
//     source_version_date, or a latest version that removed the source, is
//     a *Finding (exit 1; F7: L1, report only); an entry for the stored
//     date issued after retrieved_at (Version.IssueDate), a reissue, is
//     printed as information only. R1b's Supplement I clause is plan step
//     8's.
//
// TODO(plan step 8): Supplement I, an appendix source fetched whole, its
// --verify and --latest's slice-hash clause.
package ecfr
