// Package release builds releases: VERSION, CHANGES.json (added, rev_bumped,
// superseded, withdrawn, verification_changed), the ID-continuity and
// text-verified checks behind `regcorpus release`, and the snapshot embedded by
// `make build CORPUS_REF=<ref>` and reported by `regcorpus version`.
//
// Requirements: R6 (releases), spec "Releases and distribution". Built in plan
// step 25 and after accept. No code yet (plan step 1 scaffold).
package release
