// Package parse is the deterministic, rule-based label state machine that turns
// eCFR XML into a tree (R1a). It recognizes labels only by position, resolves
// (i)/(v)/(x) by sequence, and fails on any label it cannot place. It never
// guesses, and it never reads what the text means.
//
// Built (plan step 4): Parse for a section of 1024.41's form: a <DIV8
// TYPE="SECTION"> with an N attribute, holding one HEAD, then one or more P
// elements, then at most one CITA, with <I> the only element inside a P, not
// nested and not empty. HEAD, P and I carry no attribute and CITA none but
// TYPE; the DIV8's other attributes are not read. Anything else fails as
// Malformed: another element, a second HEAD or CITA, a P before the HEAD or
// after the CITA, no HEAD, no P, an empty <I> (<I></I> or <I/>), another
// attribute, a comment, a directive, CDATA, a namespaced element, a
// processing instruction other than one XML declaration at the very start
// of the file, a character reference to a surrogate (U+D800 to U+DFFF), and
// character data outside HEAD, P and CITA that is not XML whitespace (space,
// tab, CR, LF), such as a no-break space or a byte-order mark. Inside the
// section that is judged as decoded, so a character reference to whitespace
// passes; outside it, as written (WellFormed), so one fails.
//
// WellFormed (wellformed.go) is the one check of an XML document's form, and
// Parse runs it first. fetch (internal/ecfr) runs it on every body before
// storing it, and meta set on the raw XML it reads citations from, so the
// three refuse the same malformed XML and can't drift apart. What it refuses
// fails as Malformed with its message: XML that is not well-formed, an XML
// declaration that breaks its grammar, a duplicate attribute on any element
// (the DIV8's too), text or a second element outside the root, and a
// declaration such as <!DOCTYPE>. Parse's own rules above then apply. It lives
// here because fetch may import parse but parse must not import internal/ecfr,
// which holds the network client, and a package of its own isn't needed for
// one function. The root element is left to the caller: fetch checks it is the
// section asked for, Parse that it is a <DIV8 TYPE="SECTION"> with an N.
//
// Parse reads in three passes and reports one failure, from the first pass
// that fails: the XML's form, WellFormed's and then the rules above, each the
// first fault the reader meets as it reads; then each paragraph's lead, the
// paragraphs in order, and within a lead the failure that comes first in the
// XML; then each label's place in document order.
//
//   - Label position (ADR 0012, spec decision 12): a label counts only in a
//     paragraph's lead: at the start of the <P>, right after a heading past
//     the one "—" or one space that ends it, or right after another lead
//     label ("(A)(<I>1</I>)"). The lead ends at the first other text; a
//     label after that is running text (a cross-reference) and is never
//     structural. A label after another in one lead must open the level
//     below it.
//   - Where the lead almost goes on (ADR 0012), decided in one place,
//     follow in label.go. A run is the text up to the first letter or
//     digit, italic or not: after a label (so it holds the start of any
//     heading), after a heading, and after each dash inside a heading. A
//     dash there is any Unicode dash punctuation, Pd (in Go's Unicode 17.0.0
//     tables: U+002D, U+058A, U+05BE, U+1400, U+1806, U+2010 to U+2015,
//     U+2E17, U+2E1A, U+2E3A, U+2E3B, U+2E40, U+2E5D, U+301C, U+3030,
//     U+30A0, U+FE31, U+FE32, U+FE58, U+FE63, U+FF0D, U+10D6E and
//     U+10EAD), or the minus sign U+2212. Each case:
//     1. A chained label: right after a label with nothing between, or
//     right after a heading past exactly one "—" or one space.
//     2. One rule, paren in label.go, decides every other "(" in every
//     run: the first fails as Unplaceable at that "(". If no label starts
//     there, it is a "(" that starts no label ("(1 )", "(1.)", "()", "(é)",
//     "(see below)"), as at the start of a <P>. If one does, it is a label
//     past the wrong separator ("(a) (1)", or "–", ". " or two spaces after
//     a heading), in the italic text where a heading goes, before its
//     first letter or digit ("(a) <I>“(1) Head”</I>"), or after a dash
//     inside a heading ("(a) <I>Head—(1) Sub.</I>"). So a heading that
//     begins with a parenthesis fails ("(a) <I>(see below) Head.</I>"); a
//     parenthesis later in its words, after its first letter or digit and
//     not after a dash, is its text ("(see (1))").
//     3. A heading is one <I> element after a label and exactly one space
//     outside the <I>. Italic text in the run, or right after it, past
//     anything else (nothing, two spaces, a no-break space, a quotation
//     mark), or with a space first inside the <I>, fails as Malformed at
//     its first italic character. A second <I> right after the heading
//     fails as Malformed at its start tag.
//     4. Otherwise the lead ends at the run's end, and what follows is
//     running text: in "(a) 2(1) text." the digit ends the run, so "(1)"
//     is running text.
//   - Label shape: "(", ASCII letters or digits, ")". A label's token is one
//     run of text: all in one <I> element or none of it in any. Only an
//     italic-level label has italics: its whole token, a number, is in one
//     <I>, as in "(<I>1</I>)". Italic parentheses, a partly italic token, a
//     token in more than one <I> element ("(<I>1</I><I>2</I>)"), an italic
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
//     A line ends at LF, at CRLF or at a lone CR, as XML reads line ends.
//     For a label, they are its "("; for a "(" that starts no label, that
//     "("; for Unlabeled, the paragraph's first character, or its <P> tag if
//     it is empty; for a misplaced heading, its first italic character, and
//     for a second <I> right after a heading, its start tag; for Malformed,
//     the start of what is wrong: the element or token (an element's start
//     tag for an attribute it may not carry, an empty <I>'s start tag), the
//     character reference to a surrogate, the first byte of stray text that
//     is not whitespace (outside the section, as written: the "&" of a
//     character reference), the <DIV8> tag for a missing N, HEAD or P, the
//     end of the file for no root element, and for XML that is not
//     well-formed, the start of the token the decoder was reading (e.g. the
//     text that holds a bad entity), or the end of a truncated file.
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
// ("(A)"). internal/anchors (plan step 5, ADR 0013) decides what that
// separator means: the operative-anchor test leaves it out, the per-node
// hash keeps it, and a multi-anchor join puts nothing between own texts of
// one <P>. Inline <I> is carried as its character data, its markup dropped:
// in a heading, in an italic label ("(<I>1</I>)" is "(1)", its kind Italic)
// and in running text. Locations are not in the tree, so the tree depends on
// the text, not the XML's layout.
//
// The tree's JSON form is the json tags on Tree and Node, kinds by name
// ("letter", "number", "roman", "capital", "italic"). This package owns it.
// Each node's sha256, of its normalized own text, is empty in what Parse
// returns; internal/anchors sets it and writes the form to tree.json.
//
// Not built here:
//   - Writing tree.json, text.md and per-node hashes into sources/,
//     normalizing text and anchor IDs from label paths (1024-41-b-2-i-B):
//     internal/anchors (plan step 5) and `regcorpus parse`.
//   - Overrides: 1024.41 parses with no override, so none are applied yet.
//     The first source that needs a reviewed overrides.yaml entry builds its
//     application, before parsing (R1a).
//   - Supplement I comment headings and numbering: plan step 8.
//   - 1024.30 and 1024.31, and definition anchors by term: plan step 9.
package parse
