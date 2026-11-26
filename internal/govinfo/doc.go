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
//     no CR; inside the root, no CDATA section and no character reference
//     to a White_Space character (a space, a line break, a no-break space
//     or any other); one well-formed document (parse.WellFormed, fetch's
//     check of eCFR's XML) whose root CFRGRANULE holds an FDSYS, then one
//     SECTION, and nothing else; the FDSYS's elements in govinfo's order,
//     its CFRTITLE 12, a PARENT naming the section's part, and its DATE the
//     edition's (else ErrOtherEdition); the SECTION a SECTNO, normalized
//     "§ " and the section (else ErrOtherSection), a SUBJECT, then one or
//     more <P>s, none of nothing but White_Space, and page breaks
//     (PRTPAGE), then one citation note (CITA), last, as the granules of
//     1024.30, 1024.31 and 1024.41 have; a <P> text, <E>s of text and
//     empty PRTPAGEs; an attribute only where govinfo's granules have one
//     (an <E>'s T="03", say); a DATE only in the FDSYS; the CFRTITLE, DATE
//     and SECTNO one plain run of text each; no comment or processing
//     instruction inside the root (worklist round 1, §3; round 2, §2). A
//     value of the granule's that an error names is quoted.
//   - The layout rule (F3: C1; Layout): govinfo pretty-prints its XML, so
//     each line break, together with up to Indent (12) spaces after it, is
//     layout and is deleted, in each run of character data on its own, the
//     SECTNO's and SUBJECT's too. Nothing else is deleted or trimmed: a
//     space that is not among the first 12 after a break (one before the
//     first break of a run of them, or a thirteenth after the last) is
//     text, and so is White_Space at a text's edge, every paragraph's
//     alike, as decision 13 trims nothing. Indent is the most govinfo's
//     pretty-printer emits there (a <P>'s content, three levels of four
//     in). The heading is the SECTNO, one space and the SUBJECT; a
//     paragraph is its text with each <E>'s text in place and each PRTPAGE
//     dropped; the CITA is left out, as sha256_text leaves eCFR's out (ADR
//     0013, F3).
//   - What fetch --verify compares: the heading and each paragraph, each
//     side normalized by decision 13 alone, with the stored version's, one
//     for one, and the hash anchors.JoinSection and anchors.Hash give,
//     sha256_text's. Every character is compared but the layout. So a
//     change in govinfo's layout can give a false mismatch (verified_on:
//     null), and a false match in one way only: where govinfo meant a
//     space, between two words or at a text's edge, and wrote it as a
//     maximal run of one or more literal LFs, each followed by up to 12
//     literal spaces, while the stored text has no space there, the two
//     read alike. The live granules of 1024.30, 1024.31 and 1024.41 equal
//     the paragraphs of the stored raw XML one for one, 8, 24 and 75 of
//     them, with nothing trimmed (ADR 0014, "A correction to F3's option
//     text", which corrects "never a false match").
//   - An independent oracle, internal/anchors/testdata/oracle.py (Python's
//     standard library), decodes the committed granule of 1024.41 with its
//     own XML parser, refuses a CR, and inside the root a CDATA section or a
//     reference to a White_Space character, applies the layout rule to each
//     run on its own, trims nothing, and computes the same hash;
//     granule_test.go pins it: 1024.41's sha256_text.
//
// It holds no rule content and reads nothing into the text: the comparison
// is equality of normalized text.
package govinfo
