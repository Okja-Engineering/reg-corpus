// Package govinfo reads the official edition that `regcorpus fetch
// --verify` compares L0 with: govinfo's CFR annual edition, as one
// section's granule XML (R1; spec "Upstream", "Concerns" 4). Built in plan
// step 6; the owner's calls and the chief of staff's rulings are ADR 0014.
// It does no I/O: internal/ecfr's client fetches the granule (through
// govinfo's link service, LinkPath) and its --verify compares.
//
//   - The edition is pinned (EditionYear, EditionDate): the CFR 2026 annual
//     edition, title 12 revised as of 2026-01-01 (F2: G1).
//   - Read checks a granule before reading it: valid UTF-8, one well-formed
//     document (parse.WellFormed, fetch's check of eCFR's XML) rooted at
//     CFRGRANULE, one FDSYS DATE equal to the edition's (else
//     ErrOtherEdition), one SECTION whose SECTNO, normalized, is "§ " and
//     the section (else ErrOtherSection). The SECTION's children must be a
//     SECTNO, a SUBJECT, then <P>s and page breaks (PRTPAGE), then a
//     citation note (CITA); a SECTNO and a SUBJECT hold text, a <P> text,
//     <E>s of text and empty PRTPAGEs. Anything else is ErrNotGranule: a
//     structure it doesn't read is refused, never read past.
//   - The layout rule (F3: C1; Layout): govinfo pretty-prints its XML, so
//     each line break, together with up to Indent (12) spaces after it, is
//     layout and is deleted, in each run of character data on its own; a
//     space before a break, or a thirteenth after one, is text. Indent is
//     the most govinfo's pretty-printer emits there (a <P>'s content, three
//     levels of four in). Then the heading is the SECTNO, one space and the
//     SUBJECT; a paragraph is its text with each <E>'s text in place and
//     each PRTPAGE dropped; the CITA is left out, as sha256_text leaves
//     eCFR's out (ADR 0013, F3). anchors.JoinSection and anchors.Hash then
//     give the hash sha256_text is, decision 13's normalization applied to
//     the joined text. The rule deletes only line breaks and the spaces
//     after them, so every other character is compared: a change in
//     govinfo's layout gives a false mismatch (verified_on: null), and a
//     false match only on whether a space stands at a line break, if a
//     layout wrote a space between words as the break itself, which
//     govinfo's doesn't today.
//   - An independent oracle, internal/anchors/testdata/oracle.py (Python's
//     standard library, regular expressions on the raw granule), computes
//     the same hash from the committed granule of 1024.41; granule_test.go
//     pins it: 1024.41's sha256_text.
//
// It holds no rule content and reads nothing into the text: the comparison
// is equality of normalized text.
package govinfo
