// Package parse is the deterministic, rule-based label state machine that turns
// eCFR XML into a tree (R1a). It recognizes labels only by position, resolves
// (i)/(v)/(x) by sequence, and fails on any label it cannot place. It never
// guesses, and it never reads what the text means.
//
// Built (plan step 4): Parse for a section of 1024.41's form: a <DIV8
// TYPE="SECTION"> with an N attribute, holding one HEAD, then one or more P
// elements, then at most one CITA, with <I> the only element inside a P, not
// nested. Anything else fails as Malformed: another element, a second HEAD or
// CITA, a P before the HEAD or after the CITA, no HEAD, no P, a comment, a
// directive, CDATA, a namespaced element, and character data outside HEAD, P
// and CITA that is not XML whitespace (space, tab, CR, LF), such as a
// no-break space or a byte-order mark.
//
// Parse reads in three passes and reports the first failure of the first
// pass that fails: the XML's form, then each paragraph's lead, then each
// label's place in document order.
//
//   - Label position (ADR 0012, spec decision 12): a label counts only in a
//     paragraph's lead: at the start of the <P>, right after a heading past
//     the one "—" or one space that ends it, or right after another lead
//     label ("(A)(<I>1</I>)"). A heading is the italic text right after a
//     label and one space. The lead ends at the first other text; a label
//     after that is running text (a cross-reference) and is never
//     structural. A label where the lead almost goes on, past any other run
//     of characters that are not letters or digits (nothing, an en dash, two
//     spaces, a no-break space or ". " after a heading; a space after a
//     label), fails as Unplaceable, and so does a heading that begins with a
//     label. A label after another in one lead must open the level below it.
//   - Label shape: "(", ASCII letters or digits, ")". Only an italic-level
//     label has italics: its whole token, a number, is in <I>, as in
//     "(<I>1</I>)". Italic parentheses, a partly italic token, an italic
//     letter, a number with a leading zero or too large for an int, and a
//     roman numeral not written the standard way fail as Unplaceable.
//   - Depth comes from the label sequence: letter, number, lowercase roman,
//     capital letter, italic number. A stack of open levels holds each
//     level's last value; a label of a kind is a child of the open level
//     above it and closes the levels below it.
//   - (i), (v) and (x) read both as a letter and as a roman numeral; (c),
//     (d), (l) and (m) are letters only. Each is a letter only when the last
//     letter-level sibling directly precedes it ((h), (u), (w)); otherwise
//     roman. When the roman reading can be placed too, the next label in
//     document order decides (ADR 0012): the reading after which it can be
//     placed is taken. If no label follows, or the next can be placed after
//     neither, the parse fails as Unplaceable, naming both readings.
//   - Each label must follow its last sibling by one, and a new level starts
//     at its first label. Otherwise the parse fails with an *Error naming the
//     reason: Skipped, Repeated, OutOfOrder or Unplaceable. A <P> that does
//     not start with a label fails as Unlabeled.
//   - Locations: an *Error holds a line and a column, in bytes, in the XML.
//     For a label, they are its "("; for Unlabeled, the paragraph's first
//     character, or its </P> if it is empty; for Malformed, the start of what
//     is wrong: the element or token, the first byte of stray text that is
//     not whitespace, the <DIV8> tag for a missing N, HEAD or P, and for XML
//     that is not well-formed, the start of the token the decoder was
//     reading (e.g. the text that holds a bad entity), or the end of a
//     truncated file.
//   - Deterministic: the same bytes give the same tree.
//
// The tree (tree.go): a Tree is the section, holding the DIV8's N attribute,
// the HEAD's and the CITA's character data (they are not paragraphs and have
// no label path) and the top-level nodes. A Node holds its label path (label
// values from the top level down, e.g. ["b", "2", "i", "B"]), its level kind,
// the <P> its own text is in (Para, counted from 1), its heading if any, its
// own text and its children in document order. Own text is the XML's
// character data verbatim, not normalized: from the node's label up to the
// next lead label in the same <P>, or the end of the <P>. So it holds the
// label and heading, and the own texts of one <P>, joined, are exactly its
// character data; with Para, the tree alone gives back each paragraph's text
// and which nodes share one. The "—" or space after a heading that a child's
// label follows stays at the end of the parent's own text, and a label
// followed at once by its child's label has its label alone as own text
// ("(A)"); what that trailing separator means for the operative-anchor test,
// the per-node hash and multi-anchor joins is plan step 5's (spec "Carried
// forward"). Inline <I> is carried as its character data, its markup
// dropped: in a heading, in an italic label ("(<I>1</I>)" is "(1)", its kind
// Italic) and in running text. Locations are not in the tree, so the tree
// depends on the text, not the XML's layout.
//
// The tree's JSON form is the json tags on Tree and Node, kinds by name
// ("letter", "number", "roman", "capital", "italic"). This package owns it;
// plan step 5 adds each node's sha256 of normalized own text and writes it to
// tree.json.
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
