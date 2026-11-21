// Package verify implements `regcorpus verify <ID>`, the only path for a record
// from staging/ into registry/ (R2a). A person walks each cited field beside its
// anchor text; the command writes `verification`, moves the record and appends
// "ID, rev, record sha256, by, on, method" to registry/verify.log.
//
// Requirements: R2a (requires an interactive terminal), R11 (a person runs it;
// agents are blocked by the guard). The interactive walk arrives in plan step
// 19; today only the terminal requirement is built.
package verify
