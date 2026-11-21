// Package lookup implements the deterministic read commands: `regcorpus show
// <ID>[@rev]` (with --staging), `regcorpus text <anchor>` and `regcorpus topics
// <term>`. Plain text by default, --json for tools; exact, case-insensitive term
// matching with no fuzzy search; never prints more than the records asked for.
//
// Requirements: R8 (lookup). Built in plan steps 7, 15 and 20. No code yet
// (plan step 1 scaffold).
package lookup
