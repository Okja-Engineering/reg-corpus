// Package release builds releases: VERSION, CHANGES.json (added, rev_bumped,
// superseded, withdrawn, verification_changed), the text-verified check behind
// `regcorpus release`, and the snapshot embedded by `make build
// CORPUS_REF=<ref>` and reported by `regcorpus version`.
//
// ID continuity (every ID in the previous release still exists) is not here:
// it is a C6 per-commit rule in internal/check (plan step 13).
//
// Requirements: R6 (releases), spec "Releases and distribution". Built in plan
// step 25 and after accept. No code yet (plan step 1 scaffold).
package release
