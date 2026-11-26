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
//     (l0.RealDirs, the one check of that; under sources/ exit 1 otherwise,
//     before any request for sources/ecfr/title-12/, and under .cache/ exit
//     2, the cache's failure, below, since ADR 0014). A name that differs
//     only in case from a directory's on the way ("RAW", "TITLE-12"), or
//     from meta.yaml, tree.json, text.md or overrides.yaml in a source's
//     directory (l0.Read),
//     or from the file fetch stores ("2025-07-15.XML"; l0.StoreOnce, which
//     compares only a regular file), exits 1 too, with nothing stored: a
//     case-insensitive filesystem would store under, or report as unchanged,
//     a name git doesn't record. Under .cache/ (--date's file, --verify's
//     granule) the same rules hold, but a file the cache can't keep is the
//     cache's failure (cacheError, exit 2, naming the file as a disposable,
//     git-ignored cache that may be deleted), not a conflict with L0. A
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
//   - --verify refuses a source whose tree is a later plan step's (Goldens:
//     1024.30 and 1024.31, plan step 9) before it takes the lock. Then,
//     under the source's lock (l0.Update), before any other request, it
//     refuses an overrides.yaml, a meta.yaml without sha256_text, and
//     stored files that aren't what parse derives: it builds the stored raw
//     XML again (anchors.Build, parse's one derivation) and compares
//     tree.json and text.md (l0.State.Stale) and sha256_text, naming each
//     that differs and regcorpus parse (worklist round 1, §1). Then eCFR:
//     its latest version, read from the version list (standOf, by Latest,
//     the one reading plain fetch shares), must be source_version_date
//     (newer, older or a removal is a *Finding, exit 1, its advice by
//     direction, none for a removal at the stored date); that version's
//     XML, built as parse builds it, must have tree.json's node paths in
//     order with their sha256s, and the stored sha256_text (F6: i, T2), else
//     the nodes, or the heading or paragraph that differs outside them, are
//     named. The normalized text is the version (F1: A): sha256_raw, fresh
//     and stored, and the nodes whose raw text differs are printed as
//     information only. Then, only if that passes (F4 c), govinfo:
//     Client.Granule asks govinfo's link service for the section's
//     2026-edition granule (F2: G1), on govinfo's own origin, through the
//     same get as eCFR, an error naming the URL that failed; internal/govinfo
//     reads it (its closed grammar and the layout rule, F3: C1). Its heading
//     and each paragraph, each side normalized by decision 13 alone, nothing
//     trimmed, must equal the stored version's, the one derived again, one
//     for one, never the fresh fetch's (worklist round 2, §1), and joined
//     and hashed as sha256_text is (anchors.JoinSection), give the stored
//     sha256_text; what differs is printed by paragraph, aligned (align, a
//     longest common subsequence) so a split, merged, added or removed
//     paragraph shows alone. --verify sets only verified_on: the run's UTC
//     date on a match, unless a date is there already (F5: V-a); null on a
//     mismatch, over any date (F4 a), and a *Finding (exit 1 that writes;
//     F8: E-a). The lock is held from the stored-state check through that
//     write. Only after it is released is the granule kept under
//     .cache/govinfo/ by its sha256 (l0.StoreOnce): a cache, not evidence,
//     which nothing reads back (§2); if it can't be kept, exit 2 after the
//     verdict is written. Every other refusal or finding writes nothing; a
//     network, eCFR or govinfo error is exit 2.
//   - --latest reads each source's meta.yaml, after the stored-state check,
//     and its version list, and writes nothing. eCFR's latest is the list's
//     own (standOf): one newer than source_version_date is a *Finding, exit
//     1 (F7: L1, report only); one older (the list ends before it), or one
//     that removed the source, is one too, by the chief of staff's rulings
//     (F8; worklist round 1, §5). An entry for the stored date whose
//     issue_date (Version.IssueDate, read only here) is on or after
//     retrieved_at's day, a possible reissue, or isn't a YYYY-MM-DD date, is
//     printed as information only, a date as a date and anything else
//     quoted. R1b's Supplement I clause is plan step 8's.
//   - An upstream value printed, in a line or an error, is validated to its
//     form first or quoted (worklist round 2, §3): eCFR's dates and
//     identifiers are checked (Versions, ParseDate); issue_date as above; a
//     root element's name and values (rootElement, parse.Qualified); a URL
//     a redirect led to must be printable ASCII (getFrom); a heading or a
//     paragraph in a diff, and a granule's values in internal/govinfo's
//     errors, are quoted. encoding/xml's own messages for XML that is not
//     well-formed are printed as it writes them: a name or an entity there
//     can't hold an ASCII control character, a line break or an escape.
//
// TODO(plan step 8): Supplement I, an appendix source fetched whole, its
// --verify and --latest's slice-hash clause.
package ecfr
