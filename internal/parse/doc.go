// Package parse is the deterministic, rule-based label state machine that turns
// eCFR XML into tree.json. It recognizes labels only by position, resolves
// (i)/(v)/(x) by sequence, applies reviewed overrides.yaml entries before
// parsing, and fails on any label it cannot place. It never guesses.
//
// Requirements: R1a (deterministic parse, Supplement I headings, 1024.31
// definition anchors, overrides, goldens from XML only). Built in plan steps
// 4, 8 and 9. No code yet (plan step 1 scaffold).
package parse
