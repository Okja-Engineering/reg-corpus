// Package meta owns an L0 source's meta.yaml schema (spec "L0 meta.yaml";
// R1): its thirteen fields, their names, order, types and how each value is
// written (ADR 0010). Every command that writes a meta.yaml builds
// a Meta and writes what Marshal returns, and reads one only through Parse.
// It does no file I/O: internal/l0 stores, checks and replaces the file.
//
//   - Marshal writes one "key: value" line per field present, in the spec's
//     order. Enum words (tier, role, source), the bool and verified_on's
//     null are plain, as in the spec; every other value is double-quoted
//     (strconv.Quote's form, without a \x escape, which YAML reads as
//     another character than Go does), so no YAML reader types a date or a
//     hash as anything but a string; version_fr_citations is a flow list of
//     quoted "<vol> FR <page>", each once (ParseCitations).
//     Rendered with the standard library only, as the owner decided on
//     issue #5 (ADR 0010).
//   - Parse reads a file only when it is exactly what Marshal writes: known
//     keys, in the spec's order, each once, each value of its type and
//     written the one way. So parsing, setting a field and marshalling
//     changes that field's line and no other byte.
//   - Every meta.yaml has the fields `regcorpus fetch` writes when it creates
//     one (plan step 2): cfr, tier, role, source, official,
//     source_version_date, retrieved_at, ecfr_up_to_date_as_of, sha256_raw.
//     The others are absent until the plan step that owns them sets them:
//     version_effective_date and version_fr_citations (plan step 3,
//     `regcorpus meta set`), sha256_text (plan step 5, internal/anchors) and
//     verified_on (plan step 6, `fetch --verify`).
//
// Which command writes which field, and the checks each makes, live with the
// command (internal/ecfr: fetch and meta set) and with internal/l0 (the
// stored state every writer checks). This package checks only that each
// value is well-typed; it holds no rule content.
package meta
