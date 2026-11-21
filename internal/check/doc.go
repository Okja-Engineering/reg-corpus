// Package check implements `regcorpus check`: the corpus check C6 (per-commit,
// and --complete for the decision gate and releases) and, later, the consumer
// checks C1–C5.
//
// Requirements: C6, R2 (registry holds only verified records), R5 (corpus
// gate). C1–C5 and R4 arrive in plan step 22; the rest of C6 in plan step 13.
//
// Built (plan step 1): C6 registry state, verify.log part. Every entry under
// registry/ is a regular file or directory (never a symlink or gitlink); every
// file in registry/records/ is named RX-*.yaml and has a registry/verify.log
// line (fields "ID, rev, record sha256, by, on, method", comma-separated,
// trimmed) whose ID equals the file stem and whose sha256 equals the file's
// bytes. The corpus is read from a directory (LoadDir: the working tree) or a
// git tree (LoadTree: the commit, raw blobs); a root without a go.mod
// declaring this module and a stages/ directory is not a corpus (exit 2).
//
// TODO(plan step 13), the remaining C6 per-commit rules from the spec:
//   - quotes: a cited field missing cite or text
//   - quotes: a quote under 4 words, or not found at its anchors under the join rule
//   - values: deadline.amount not present in deadline.text as digits or a number word
//   - values: excluding_weekends_holidays without "Saturdays" in the quote
//   - values: calendar without day_type_cite: 1024-31-day
//   - values: qualitative with an amount
//   - lead-ins: citing a child of a conditional lead-in without citing the lead-in
//   - hashes: a mismatch on any per-anchor source_hashes, or on a section hash for L0
//   - registry state: a registry record below text-verified, or with incomplete verification
//   - registry state: a record with no prompt_version
//   - references: an applies_to or superseded_by ID that doesn't exist
//   - references: superseded with an empty superseded_by
//   - references: an ID from the previous release missing
//   - references: a topic mapping an ID that doesn't exist
//
// TODO(plan step 23): skill: any 8-word run of anchor text in the skill file.
//
// TODO(plan step 13), C6 --complete: an operative anchor neither cited nor na;
// an inventory entry without reviewed; an active record in no topic.
//
// TODO(plan step 16): check --staging runs C6 on staged records and fails when
// two staged records share the same action cite and text (R2a).
package check
