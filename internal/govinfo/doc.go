// Package govinfo reads the official edition that `regcorpus fetch
// --verify` compares L0 with: govinfo's CFR annual edition, as one
// section's granule XML (R1; spec "Upstream", "Concerns" 4). Built in plan
// step 6; the owner's calls and the chief of staff's rulings are ADR 0014.
// It does no I/O: internal/ecfr's client fetches the granule (through
// govinfo's link service, LinkPath) and its --verify compares.
//
//   - The edition is pinned (EditionTitle, EditionYear, EditionDate): the
//     CFR 2026 annual edition, title 12 revised as of 2026-01-01 (F2: G1).
//   - Read reads a granule's whole form, a closed grammar, and refuses
//     anything else (ErrNotGranule), never reading past it: valid UTF-8 with
//     no CR and no character reference to a CR or an LF; one well-formed
//     document (parse.WellFormed, fetch's check of eCFR's XML) whose root
//     CFRGRANULE holds an FDSYS, then one SECTION, and nothing else; the
//     FDSYS's elements in govinfo's order, its CFRTITLE 12, a PARENT naming
//     the section's part, and its DATE the edition's (else ErrOtherEdition);
//     the SECTION a SECTNO, normalized "§ " and the section (else
//     ErrOtherSection), a SUBJECT, then non-empty <P>s and page breaks
//     (PRTPAGE), then a citation note (CITA); a <P> text, <E>s of text and
//     empty PRTPAGEs; an attribute only where govinfo's granules have one
//     (an <E>'s T="03", say); a DATE only in the FDSYS; the CFRTITLE, DATE
//     and SECTNO one plain run of text each; no comment or processing
//     instruction (worklist round 1, §3).
//   - The layout rule (F3: C1; Layout): govinfo pretty-prints its XML, so
//     each line break, together with up to Indent (12) spaces after it, is
//     layout and is deleted, in each run of character data on its own, the
//     SECTNO's and SUBJECT's too; a space before a break, or a thirteenth
//     after one, is text. Indent is the most govinfo's pretty-printer emits
//     there (a <P>'s content, three levels of four in). Each text's edges
//     are then trimmed of White_Space, every paragraph's alike. The heading
//     is the SECTNO, one space and the SUBJECT; a paragraph is its text with
//     each <E>'s text in place and each PRTPAGE dropped; the CITA is left
//     out, as sha256_text leaves eCFR's out (ADR 0013, F3).
//   - What fetch --verify compares: the heading and each paragraph,
//     normalized (decision 13), with eCFR's, one for one, and the hash
//     anchors.JoinSection and anchors.Hash give, sha256_text's. Every
//     character is compared but these: a line break and up to 12 spaces
//     after it, and the White_Space at a text's edges. So a change in
//     govinfo's layout can give a false mismatch (verified_on: null), and a
//     false match in one way only: where govinfo wrote a literal LF, with no
//     space before it and up to 12 after it, to mean the space between two
//     words, and eCFR's text has no space there, the two read alike.
//     govinfo's layout writes such a space as a space, before the break or
//     a thirteenth after it: in the committed granule of 1024.41 no line
//     break stands between two characters of text in one run (ADR 0014,
//     Decision 2, which corrects "never a false match").
//   - An independent oracle, internal/anchors/testdata/oracle.py (Python's
//     standard library), decodes the committed granule of 1024.41 with its
//     own XML parser, refuses a CR or a reference to a line break, applies
//     the layout rule to each run on its own, and computes the same hash;
//     granule_test.go pins it: 1024.41's sha256_text.
//
// It holds no rule content and reads nothing into the text: the comparison
// is equality of normalized text.
package govinfo
