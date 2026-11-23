// Package parse is the deterministic, rule-based label state machine that turns
// eCFR XML into a tree (R1a). It recognizes labels only by position, resolves
// (i)/(v)/(x) by sequence, and fails on any label it cannot place. It never
// guesses, and it never reads what the text means.
//
// Built (plan step 4): Parse for a section of 1024.41's form, a <DIV8
// TYPE="SECTION"> holding one HEAD, the P elements and at most one CITA, with
// <I> the only element inside a P. Any other element, or text outside these,
// fails as Malformed.
//
//   - Label position: a label is "(" token ")" in a paragraph's lead: at the
//     start of the <P>, right after a heading (past the "—" or the space that
//     ends it), or right after another lead label ("(A)(<I>1</I>)"). A heading
//     is the italic text right after a label and one space. The lead ends at
//     the first text that is none of these; a label after that is running text
//     (a cross-reference) and is never structural (ADR 0012, spec decision 12).
//   - Depth comes from the label sequence: letter, number, lowercase roman,
//     capital letter, italic number (a number in <I>). A stack of open levels
//     holds each level's last sibling; a label of a kind is a child of the
//     open level above it and closes the levels below it.
//   - A lowercase label that is both a letter and a roman numeral ((i), (v),
//     (x)) is a letter only when the last letter-level sibling directly
//     precedes it ((h), (u), (w)); otherwise roman. There is no lookahead
//     (ADR 0012): this rule and the sequence checks below decide it. The one
//     gap: a lone roman (i) that is a section's last label, after (h), reads
//     as the letter (i).
//   - Each label must follow its last sibling by one, and a new level starts
//     at its first label. Otherwise the parse fails with an *Error naming the
//     reason (Skipped, Repeated, OutOfOrder, Unplaceable, Unlabeled for a <P>
//     with no label) and the location in the XML: line and column, in bytes,
//     of the label's "(".
//   - Deterministic: the same bytes give the same tree.
//
// The tree (tree.go): a Tree is the section, holding the DIV8's N attribute,
// the HEAD's and the CITA's character data (they are not paragraphs and have
// no label path) and the top-level nodes. A Node holds its label path (label
// values from the top level down, e.g. ["b", "2", "i", "B"]), its level kind,
// its heading if any, its own text and its children in document order. Own
// text is the XML's character data verbatim, not normalized: from the
// node's label up to the next lead label in the same <P>, or the end of the
// <P>. So it holds the label and heading, and the own texts of one <P>, joined,
// are exactly its character data: the "—" or space after a heading that a
// child's label follows stays at the end of the parent's own text, and a
// label followed at once by its child's label has its label alone as own
// text ("(A)"). Inline <I> is carried as its character data, its markup
// dropped: in a heading, in an italic label ("(<I>1</I>)" is "(1)", its kind
// Italic) and in running text. The tree's JSON form is the json tags on Tree
// and Node, kinds by name ("letter", "number", "roman", "capital", "italic").
// Locations are not in the tree, so the tree depends on the text, not the
// XML's layout.
//
// Not built here:
//   - Writing tree.json, text.md and per-node hashes into sources/, and
//     normalizing text: plan step 5 (internal/anchors), which also builds
//     anchor IDs from label paths (1024-41-b-2-i-B).
//   - Overrides: 1024.41 parses with no override, so none are applied yet.
//     The first source that needs a reviewed overrides.yaml entry builds its
//     application, before parsing (R1a).
//   - Supplement I comment headings and numbering: plan step 8.
//   - 1024.30 and 1024.31, and definition anchors by term: plan step 9.
package parse
