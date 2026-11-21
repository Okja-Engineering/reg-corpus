// Package check implements `regcorpus check`: the corpus check C6 (per-commit,
// and --complete for the decision gate and releases) and, later, the consumer
// checks C1–C5.
//
// Requirements: C6, R2 (registry holds only verified records), R5 (corpus
// gate). C1–C5 and R4 arrive in plan step 22. The rest of C6, per-commit and
// --complete, arrives in plan step 13, except the skill's 8-word rule (plan
// step 23).
//
// Built (plan step 1): C6 registry state, verify.log part. Every file in
// registry/records/ is named RX-*.yaml and has a registry/verify.log line
// (fields "ID, rev, record sha256, by, on, method", comma-separated, trimmed)
// whose ID equals the file stem and whose sha256 equals the file's bytes.
//
// Also built: the registry layout. The spec's C6 list has no such rule; it is
// a C6 addition, recorded as a finding in PR #1, so that a record kept
// elsewhere under registry/, or under a variant of its name, can't escape the
// rule above (R2: registry/ holds only verified records). The registry is every
// top-level name equal to "registry" in any ASCII case (Registry/ too: one
// directory on a case-insensitive checkout), and a top-level name that isn't
// ASCII is reported, since filesystems fold such names (APFS folds "regiﬆry"
// into registry/) in ways no rule here can mirror. Every entry in the registry
// is a regular file or directory (never a symlink or gitlink), and every entry
// other than a record is registry/, records/, inventory/, verify.log,
// topics.yaml or inventory/*.yaml, spelled exactly (the places spec "L1 and
// indexes" names).
//
// The corpus is read from a git tree (LoadTree: the commit, raw blobs; the
// pre-commit hook and CI) or a directory (LoadDir: the working tree, for `make
// check`). LoadDir leaves out what git ignores and doesn't track, since no
// commit holds it (.DS_Store, editor swap files). A root without a regular
// go.mod declaring this module and a real stages/ directory is not a corpus
// (exit 2).
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
