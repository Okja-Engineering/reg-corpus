// Tests for reading a govinfo CFR granule (R1; plan step 6, ADR 0014):
// the layout rule, the heading and paragraphs it gives, the hash they give
// against an independent oracle's, and the granules it refuses.
package govinfo_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/govinfo"
	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
)

// The committed granule: govinfo's CFR 2026 edition of 12 CFR 1024.41, as
// govinfo served it on 2026-11-25 through
// /link/cfr/12/1024?sectionnum=41&year=2026&link-type=xml.
const granule41 = "testdata/CFR-2026-title12-vol8-sec1024-41.xml"

// The oracle's values: internal/anchors/testdata/oracle.py (Python's
// standard library only, the granule read with regular expressions, not
// this package), run as
//
//	python3 -I internal/anchors/testdata/oracle.py \
//	    sources/ecfr/title-12/1024.41/raw/2025-07-15.xml \
//	    internal/govinfo/testdata/CFR-2026-title12-vol8-sec1024-41.xml
//
// gives "govinfo sha256_text ecda50e7…", 75 paragraphs, all 75 matching
// eCFR's, which is 1024.41's committed sha256_text. The granule's own
// sha256 pins its bytes as served.
const (
	oracleGovinfoText = "ecda50e7ce958e2fd19a8f1843b3c60bde0dcf533cc2c313cb9c1e1960760fe1"
	granule41SHA256   = "147ace15c532251bddb55bd78360909bf4efe14923060c80e8edb356d117c391"
)

// nbsp is U+00A0, the no-break space, written so no invisible character is
// in this file.
var nbsp = string(rune(0xA0))

func read(t *testing.T, slash string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(slash))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func sectionText(t *testing.T, s *govinfo.Section) string {
	t.Helper()
	h, err := anchors.Hash(anchors.JoinSection(s.Heading, s.Paragraphs))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestTheCommittedGranuleGivesTheOraclesSHA256Text(t *testing.T) {
	b := read(t, granule41)
	if h := l0.SHA256Hex(b); h != granule41SHA256 {
		t.Fatalf("%s has sha256 %s, want %s, govinfo's bytes", granule41, h, granule41SHA256)
	}
	s, err := govinfo.Read(b, "1024.41", govinfo.EditionDate)
	if err != nil {
		t.Fatal(err)
	}
	// govinfo's SECTNO has a thin space (U+2009) after the section sign,
	// where eCFR's HEAD has a space: White_Space both, so they normalize
	// alike.
	heading := "§" + string(rune(0x2009)) + "1024.41 Loss mitigation procedures."
	if s.Date != "2026-01-01" || s.Heading != heading || len(s.Paragraphs) != 75 {
		t.Fatalf("Date %q, Heading %q, %d paragraphs; want 2026-01-01, %q (the SECTNO and SUBJECT one space apart), 75", s.Date, s.Heading, len(s.Paragraphs), heading)
	}
	if got := sectionText(t, s); got != oracleGovinfoText {
		t.Errorf("sha256 of the granule's section text = %s, want the oracle's %s", got, oracleGovinfoText)
	}
	m, err := meta.Parse(read(t, "../../sources/ecfr/title-12/1024.41/meta.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(m.SHA256Text) != oracleGovinfoText {
		t.Errorf("1024.41's committed sha256_text %s, want %s", m.SHA256Text, oracleGovinfoText)
	}
	for i, want := range map[int]string{
		0:  "(a) Enforcement and limitations. A borrower may enforce",
		1:  "(b) Receipt of a loss mitigation application—(1) Complete loss mitigation application. A complete",
		3:  "(A) Promptly upon receipt of a loss mitigation application, review",
		74: "(5) Pending loss mitigation offers. A transfer does not affect",
	} {
		if !strings.HasPrefix(s.Paragraphs[i], want) {
			t.Errorf("paragraph %d = %.120q, want it to start %q", i+1, s.Paragraphs[i], want)
		}
	}
	for i, p := range s.Paragraphs {
		if strings.Contains(p, "\n") || strings.Contains(p, "  ") || strings.HasPrefix(p, " ") || strings.HasSuffix(p, " ") {
			t.Errorf("paragraph %d keeps layout: %.80q", i+1, p)
		}
	}
	// The page break inside the last paragraph but one is dropped, and the
	// words on either side of it are one space apart, as eCFR has them.
	if !strings.Contains(s.Paragraphs[73], "shall be considered complete as of the date the appeal") {
		t.Errorf("paragraph 74 = %q; want the words around its PRTPAGE one space apart", s.Paragraphs[73])
	}
}

func TestLayoutIsALineBreakAndUpToTwelveSpacesAfterIt(t *testing.T) {
	if govinfo.Indent != 12 {
		t.Fatalf("Indent = %d, want 12, the most govinfo's pretty-printer emits", govinfo.Indent)
	}
	twelve := strings.Repeat(" ", 12)
	for _, tc := range []struct{ in, want, why string }{
		{"a\n" + twelve + "b", "ab", "a break and twelve spaces"},
		{"a \n" + twelve + "b", "a b", "the space before the break is text"},
		{"a\n" + twelve + " b", "a b", "the thirteenth space is text"},
		{"a\n" + twelve + "  b", "a  b", "and so is a fourteenth"},
		{"a\nb", "ab", "a break with no indentation"},
		{"a\n    b", "ab", "a shallower one"},
		{"a\n\n" + twelve + "b", "ab", "each break with its own indentation"},
		{"a\n" + twelve + "\n" + twelve, "a", "a closing tag's line"},
		{"a\tb", "a\tb", "a tab is not layout"},
		{"a\n\tb", "a\tb", "nor is a tab after a break"},
		{"a  b", "a  b", "spaces without a break are text"},
		{"a" + nbsp + "\n" + twelve + "b", "a" + nbsp + "b", "a no-break space is text"},
		{"a\n" + nbsp + "b", "a" + nbsp + "b", "and is not indentation"},
		{"a\n \nb", "ab", "a space between two breaks is the first's indentation"},
		{"a \n \nb", "a b", "a space before the first break is text"},
		{"", "", "nothing"},
	} {
		if got := govinfo.Layout(tc.in); got != tc.want {
			t.Errorf("%s: Layout(%q) = %q, want %q", tc.why, tc.in, got, tc.want)
		}
	}
}

// granule builds a pretty-printed granule as govinfo lays one out: date in
// FDSYS, then the SECTION with its SECTNO, SUBJECT, body (its paragraphs,
// as given) and a CITA.
func granule(date, sectno, subject, body string) string {
	return `<?xml version="1.0"?>
<?xml-stylesheet type="text/xsl" href="cfr.xsl"?>
<CFRGRANULE xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="CFRMergedXML.xsd">
    <FDSYS>
        <CFRTITLE>12</CFRTITLE>
        <DATE>` + date + `</DATE>
        <ANCESTORS>
            <PARENT HEADING="PART 1024" SEQ="1">REAL ESTATE SETTLEMENT PROCEDURES ACT (REGULATION X)</PARENT>
        </ANCESTORS>
    </FDSYS>
    <SECTION>
        <SECTNO>` + sectno + `</SECTNO>
        <SUBJECT>` + subject + `</SUBJECT>
` + body + `
        <CITA>[78 FR 10876, Feb. 14, 2013]</CITA>
    </SECTION>
</CFRGRANULE>
`
}

// twoParas is two paragraphs laid out as govinfo lays them out: a label and
// a space, an <E> heading, a space and text, a page break, more text.
const twoParas = "        <P>\n" +
	"            (a) \n" +
	"            <E T=\"03\">Heading.</E>\n" +
	"             Text one \n" +
	"            <PRTPAGE P=\"721\"/>\n" +
	"            continues.\n" +
	"        </P>\n" +
	"        <P>(b) Text two.</P>"

func TestAGranuleGivesItsHeadingAndParagraphsWithoutLayout(t *testing.T) {
	s, err := govinfo.Read([]byte(granule("2026-01-01", "§ 1024.41", "Loss mitigation procedures.", twoParas)), "1024.41", govinfo.EditionDate)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"(a) Heading. Text one continues.", "(b) Text two."}
	if s.Heading != "§ 1024.41 Loss mitigation procedures." || strings.Join(s.Paragraphs, "|") != strings.Join(want, "|") {
		t.Errorf("Heading %q, Paragraphs %q; want %q, %q (E's text kept, PRTPAGE and CITA left out)", s.Heading, s.Paragraphs, "§ 1024.41 Loss mitigation procedures.", want)
	}
}

// TestLayoutOnlyDifferencesReadAlike: another indentation before a tag, or
// another place for the line breaks between elements, gives the same text.
func TestLayoutOnlyDifferencesReadAlike(t *testing.T) {
	base, err := govinfo.Read([]byte(granule("2026-01-01", "§ 1024.41", "Loss mitigation procedures.", twoParas)), "1024.41", govinfo.EditionDate)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"shallower before a tag": strings.ReplaceAll(twoParas, "\n            <", "\n    <"),
		"a space moved from before a break to the thirteenth after it": strings.Replace(twoParas,
			"Text one \n            <PRTPAGE P=\"721\"/>\n            continues", "Text one\n            <PRTPAGE P=\"721\"/>\n             continues", 1),
		"on one line": "<P>(a) <E T=\"03\">Heading.</E> Text one <PRTPAGE P=\"721\"/>continues.</P>\n" +
			"<P>(b) Text two.</P>",
		"broken around every element": "<P>\n            (a) \n            <E T=\"03\">\n            Heading.\n            </E>\n             Text one \n" +
			"            <PRTPAGE P=\"721\"/>\n            continues.\n        </P>\n        <P>\n            (b) Text two.\n        </P>",
	} {
		s, err := govinfo.Read([]byte(granule("2026-01-01", "§ 1024.41", "Loss mitigation \n            procedures.", body)), "1024.41", govinfo.EditionDate)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if s.Heading != base.Heading || strings.Join(s.Paragraphs, "|") != strings.Join(base.Paragraphs, "|") {
			t.Errorf("%s: Heading %q, Paragraphs %q; want %q, %q", name, s.Heading, s.Paragraphs, base.Heading, base.Paragraphs)
		}
	}
}

// TestAWordOrASpaceIsText: what layout can't explain changes the text, and
// so the hash. (Two spaces for one don't: normalization collapses a run of
// White_Space, on both sides alike.)
func TestAWordOrASpaceIsText(t *testing.T) {
	base, err := govinfo.Read([]byte(granule("2026-01-01", "§ 1024.41", "Loss mitigation procedures.", twoParas)), "1024.41", govinfo.EditionDate)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"a word":                         strings.Replace(twoParas, "Text two", "Text 2", 1),
		"a space between words removed":  strings.Replace(twoParas, "Text two", "Texttwo", 1),
		"the space before a tag removed": strings.Replace(twoParas, "Text one \n", "Text one\n", 1),
		"the thirteenth space removed":   strings.Replace(twoParas, "\n             Text one", "\n            Text one", 1),
		"the E's text":                   strings.Replace(twoParas, "Heading.", "Headings.", 1),
	} {
		s, err := govinfo.Read([]byte(granule("2026-01-01", "§ 1024.41", "Loss mitigation procedures.", body)), "1024.41", govinfo.EditionDate)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if sectionText(t, s) == sectionText(t, base) {
			t.Errorf("%s: the same hash as the base granule; want another", name)
		}
	}
	s, err := govinfo.Read([]byte(granule("2026-01-01", "§ 1024.41", "Loss mitigation procedures!", twoParas)), "1024.41", govinfo.EditionDate)
	if err != nil || sectionText(t, s) == sectionText(t, base) {
		t.Errorf("the SUBJECT changed: %v, or the same hash; want another", err)
	}
}

func TestReadRefusesWhatIsNotTheSectionsGranule(t *testing.T) {
	ok := granule("2026-01-01", "§ 1024.41", "Loss mitigation procedures.", twoParas)
	for _, tc := range []struct {
		name    string
		granule string
		want    error
	}{
		{"not UTF-8", strings.Replace(ok, "Text two", "Text \xff", 1), govinfo.ErrNotGranule},
		{"not well-formed", strings.Replace(ok, "</SECTION>", "", 1), govinfo.ErrNotGranule},
		{"a DOCTYPE", strings.Replace(ok, "<CFRGRANULE ", "<!DOCTYPE CFRGRANULE []>\n<CFRGRANULE ", 1), govinfo.ErrNotGranule},
		{"eCFR's XML", "<DIV8 N=\"1024.41\" TYPE=\"SECTION\"><HEAD>§ 1024.41 x.</HEAD></DIV8>", govinfo.ErrNotGranule},
		{"no SECTION", strings.Replace(strings.Replace(ok, "<SECTION>", "<PART>", 1), "</SECTION>", "</PART>", 1), govinfo.ErrNotGranule},
		{"two SECTIONs", strings.Replace(ok, "</CFRGRANULE>", "<SECTION><SECTNO>§ 1024.41</SECTNO><SUBJECT>x</SUBJECT></SECTION></CFRGRANULE>", 1), govinfo.ErrNotGranule},
		{"a SECTION in the FDSYS", strings.Replace(ok, "</FDSYS>", "<SECTION><SECTNO>§ 1024.41</SECTNO><SUBJECT>x</SUBJECT></SECTION></FDSYS>", 1), govinfo.ErrNotGranule},
		{"no SUBJECT", strings.Replace(ok, "<SUBJECT>Loss mitigation procedures.</SUBJECT>", "", 1), govinfo.ErrNotGranule},
		{"two SECTNOs", strings.Replace(ok, "<SUBJECT>", "<SECTNO>§ 1024.41</SECTNO><SUBJECT>", 1), govinfo.ErrNotGranule},
		{"a P before the SUBJECT", strings.Replace(ok, "<SUBJECT>", "<P>(z) x.</P><SUBJECT>", 1), govinfo.ErrNotGranule},
		{"a P after the CITA", strings.Replace(ok, "</SECTION>", "<P>(z) x.</P></SECTION>", 1), govinfo.ErrNotGranule},
		{"an unknown element in a P", strings.Replace(ok, "Text two.", "Text <SU>1</SU> two.", 1), govinfo.ErrNotGranule},
		{"an unknown element in the SECTION", strings.Replace(ok, "<CITA>", "<EXTRACT><P>x</P></EXTRACT><CITA>", 1), govinfo.ErrNotGranule},
		{"an element in an E", strings.Replace(ok, "Heading.</E>", "Head<E T=\"03\">ing</E>.</E>", 1), govinfo.ErrNotGranule},
		{"an element in the SUBJECT", strings.Replace(ok, "Loss mitigation procedures.</SUBJECT>", "Loss <E T=\"03\">x</E>.</SUBJECT>", 1), govinfo.ErrNotGranule},
		{"a PRTPAGE with text", strings.Replace(ok, `<PRTPAGE P="721"/>`, `<PRTPAGE P="721">x</PRTPAGE>`, 1), govinfo.ErrNotGranule},
		{"text in the SECTION", strings.Replace(ok, "<CITA>", "stray<CITA>", 1), govinfo.ErrNotGranule},
		{"no DATE", strings.Replace(ok, "<DATE>2026-01-01</DATE>", "", 1), govinfo.ErrNotGranule},
		{"two DATEs", strings.Replace(ok, "<CFRTITLE>", "<DATE>2026-01-01</DATE><CFRTITLE>", 1), govinfo.ErrNotGranule},
		{"another section", granule("2026-01-01", "§ 1024.40", "Scope.", twoParas), govinfo.ErrOtherSection},
		{"a SECTNO naming the section twice", granule("2026-01-01", "§§ 1024.41", "x.", twoParas), govinfo.ErrOtherSection},
		{"another edition", granule("2025-01-01", "§ 1024.41", "Loss mitigation procedures.", twoParas), govinfo.ErrOtherEdition},
		{"a DATE not YYYY-MM-DD", granule("2026", "§ 1024.41", "Loss mitigation procedures.", twoParas), govinfo.ErrOtherEdition},
	} {
		s, err := govinfo.Read([]byte(tc.granule), "1024.41", govinfo.EditionDate)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: Read = %v, %v; want %v", tc.name, s, err, tc.want)
		}
	}
	// The SECTNO is read normalized: a no-break space after the section
	// sign, or the SECTNO laid out over two lines, still names it.
	for _, sectno := range []string{"§" + nbsp + "1024.41", "\n            § 1024.41\n        "} {
		if _, err := govinfo.Read([]byte(granule("2026-01-01", sectno, "Loss mitigation procedures.", twoParas)), "1024.41", govinfo.EditionDate); err != nil {
			t.Errorf("SECTNO %q: %v; want it read as § 1024.41", sectno, err)
		}
	}
}

// TestReadRefusesWhatItDoesNotRead: a structure the reader doesn't read is
// refused, never read past (ADR 0014; worklist round 1, §3 and §4): in the
// root, only the FDSYS and then one SECTION; in the FDSYS, only the
// elements a granule's FDSYS holds (CFRTITLE, CFRTITLETEXT, VOL, DATE,
// COVERONLY, ORIGINALDATE, ANCESTORS of PARENTs), naming title 12 and the
// section's part; a DATE only there; an attribute only where govinfo's
// granules have one; the DATE and the SECTNO each one run of plain
// character data; no comment or processing instruction inside the root; no
// CR, no CDATA section and no character reference to a White_Space
// character, so every line break and space deleted as layout is written as
// itself; no <P> of nothing but White_Space; at least one <P>; and the CITA
// once and last.
func TestReadRefusesWhatItDoesNotRead(t *testing.T) {
	ok := granule("2026-01-01", "§ 1024.41", "Loss mitigation procedures.", twoParas)
	if _, err := govinfo.Read([]byte(ok), "1024.41", govinfo.EditionDate); err != nil {
		t.Fatalf("the base granule: %v", err)
	}
	for _, tc := range []struct{ name, granule string }{
		{"a P outside the SECTION", strings.Replace(ok, "    <SECTION>", "    <P>(z) x.</P>\n    <SECTION>", 1)},
		{"an APPENDIX beside the SECTION", strings.Replace(ok, "</SECTION>", "</SECTION>\n    <APPENDIX>x</APPENDIX>", 1)},
		{"an EDNOTE before the FDSYS", strings.Replace(ok, "    <FDSYS>", "    <EDNOTE>x</EDNOTE>\n    <FDSYS>", 1)},
		{"text in the root", strings.Replace(ok, "</FDSYS>", "</FDSYS>\n    stray", 1)},
		{"the SECTION before the FDSYS", strings.Replace(strings.Replace(ok, "<FDSYS>", "<SECTION><SECTNO>§ 1024.41</SECTNO><SUBJECT>x</SUBJECT><P>(a) x.</P><CITA>x</CITA></SECTION><FDSYS>", 1),
			"    <SECTION>\n", "    <X>\n", 1)},
		{"an extra empty SECTION", strings.Replace(ok, "</SECTION>", "</SECTION>\n    <SECTION/>", 1)},
		{"an unknown element in the FDSYS", strings.Replace(ok, "<CFRTITLE>", "<EXTRA>x</EXTRA><CFRTITLE>", 1)},
		{"text in the FDSYS", strings.Replace(ok, "<CFRTITLE>", "stray<CFRTITLE>", 1)},
		{"an element in a PARENT", strings.Replace(ok, "SEQ=\"1\">REAL", "SEQ=\"1\"><E T=\"03\">x</E>REAL", 1)},
		{"title 24", strings.Replace(ok, "<CFRTITLE>12</CFRTITLE>", "<CFRTITLE>24</CFRTITLE>", 1)},
		{"no CFRTITLE", strings.Replace(ok, "<CFRTITLE>12</CFRTITLE>", "", 1)},
		{"another part", strings.Replace(ok, `HEADING="PART 1024"`, `HEADING="PART 1025"`, 1)},
		{"no part", strings.Replace(ok, `HEADING="PART 1024"`, `HEADING="SUBPART C"`, 1)},
		{"a DATE in the root", strings.Replace(ok, "</FDSYS>", "</FDSYS>\n    <DATE>2026-01-01</DATE>", 1)},
		{"a DATE in the ANCESTORS", strings.Replace(ok, "<ANCESTORS>", "<ANCESTORS><DATE>2026-01-01</DATE>", 1)},
		{"a DATE in the SECTION", strings.Replace(ok, "<SUBJECT>", "<DATE>2026-01-01</DATE><SUBJECT>", 1)},
		// A DATE outside FDSYS where the order of the elements alone would
		// admit it, last in its parent: only "a DATE only in FDSYS" refuses
		// these (worklist round 1, §3 and §10).
		{"a DATE after the SECTION", strings.Replace(ok, "</SECTION>", "</SECTION>\n    <DATE>2026-01-01</DATE>", 1)},
		{"a DATE after the CITA", strings.Replace(ok, "</CITA>", "</CITA>\n        <DATE>2026-01-01</DATE>", 1)},
		{"a DATE after the PARENT", strings.Replace(ok, "</PARENT>", "</PARENT><DATE>2026-01-01</DATE>", 1)},
		{"a DATE in a P", strings.Replace(ok, "(b) Text two.</P>", "(b) Text two.<DATE>2026-01-01</DATE></P>", 1)},
		{"an attribute on a P", strings.Replace(ok, "<P>(b) Text two.</P>", `<P ID="x">(b) Text two.</P>`, 1)},
		{"an attribute on the SECTION", strings.Replace(ok, "<SECTION>", `<SECTION TYPE="x">`, 1)},
		{"an attribute on the SECTNO", strings.Replace(ok, "<SECTNO>", `<SECTNO X="1">`, 1)},
		{"an E of another type", strings.Replace(ok, `<E T="03">`, `<E T="04">`, 1)},
		{"another attribute on an E", strings.Replace(ok, `<E T="03">`, `<E T="03" X="1">`, 1)},
		{"an E with no type", strings.Replace(ok, `<E T="03">`, `<E>`, 1)},
		{"a DATE in a CDATA section", strings.Replace(ok, "<DATE>2026-01-01</DATE>", "<DATE><![CDATA[2026-01-01]]></DATE>", 1)},
		{"a DATE with a comment", strings.Replace(ok, "<DATE>2026-01-01</DATE>", "<DATE>2026-01-01<!-- x --></DATE>", 1)},
		{"a DATE with an element", strings.Replace(ok, "<DATE>2026-01-01</DATE>", "<DATE>2026-01-01<E T=\"03\"></E></DATE>", 1)},
		{"a DATE with a character reference", strings.Replace(ok, "<DATE>2026-01-01</DATE>", "<DATE>2026&#45;01-01</DATE>", 1)},
		{"a SECTNO in a CDATA section", strings.Replace(ok, "<SECTNO>§ 1024.41</SECTNO>", "<SECTNO><![CDATA[§ 1024.41]]></SECTNO>", 1)},
		{"a SECTNO with a comment", strings.Replace(ok, "<SECTNO>§ 1024.41</SECTNO>", "<SECTNO>§ 1024.41<!-- x --></SECTNO>", 1)},
		{"a SECTNO with an element", strings.Replace(ok, "<SECTNO>§ 1024.41</SECTNO>", "<SECTNO>§ <E T=\"03\">1024.41</E></SECTNO>", 1)},
		{"a SECTNO with a character reference", strings.Replace(ok, "<SECTNO>§ 1024.41</SECTNO>", "<SECTNO>&#167; 1024.41</SECTNO>", 1)},
		{"a comment in a P", strings.Replace(ok, "Text two.", "Text <!-- x -->two.", 1)},
		{"a processing instruction in a P", strings.Replace(ok, "Text two.", "Text <?x y?>two.", 1)},
		{"an element in the CITA", strings.Replace(ok, "Feb. 14, 2013]", "Feb. 14, <E T=\"03\">2013</E>]", 1)},
		{"CR LF line ends", strings.ReplaceAll(ok, "\n", "\r\n")},
		{"a CR in a P", strings.Replace(ok, "Text two.", "Text\rtwo.", 1)},
		{"a reference to LF", strings.Replace(ok, "Text two.", "Text&#10;two.", 1)},
		{"a hex reference to LF", strings.Replace(ok, "Text two.", "Text&#xA;two.", 1)},
		{"a zero-padded reference to LF", strings.Replace(ok, "Text two.", "Text&#x000a;two.", 1)},
		{"a reference to CR", strings.Replace(ok, "Text two.", "Text&#13;two.", 1)},
		{"an empty P last", strings.Replace(ok, "<P>(b) Text two.</P>", "<P>(b) Text two.</P>\n        <P/>", 1)},
		{"an empty P first", strings.Replace(ok, "        <P>\n            (a) ", "        <P></P>\n        <P>\n            (a) ", 1)},
		{"a P of layout only", strings.Replace(ok, "<P>(b) Text two.</P>", "<P>(b) Text two.</P>\n        <P>\n            \n        </P>", 1)},
		{"a P of a no-break space only", strings.Replace(ok, "<P>(b) Text two.</P>", "<P>(b) Text two.</P>\n        <P>"+nbsp+"</P>", 1)},
		// A SECTION holds elements and XML whitespace between them: a
		// no-break space between two <P>s is text there (worklist round
		// 3, §2: G2).
		{"a no-break space between two Ps", strings.Replace(ok, "<P>(b) Text two.</P>", nbsp+"<P>(b) Text two.</P>", 1)},
		// Worklist round 2, §2 (C-N3, C-N7, R-N4) and §6 (M44, M48): a
		// CDATA section, and a reference to any White_Space character, are
		// refused, so every character Layout deletes was written as itself;
		// a SECTION holds at least one <P>, and its CITA once and last.
		{"a CDATA section in a P", strings.Replace(ok, "Text two.", "Text <![CDATA[two]]>.", 1)},
		{"a CDATA section of a break and spaces in a P", strings.Replace(ok, "Text two.", "Text<![CDATA[\n   ]]>two.", 1)},
		{"a CDATA section in the SUBJECT", strings.Replace(ok, "Loss mitigation procedures.", "Loss <![CDATA[mitigation]]> procedures.", 1)},
		{"a CDATA section in an E", strings.Replace(ok, "Heading.</E>", "<![CDATA[Heading.]]></E>", 1)},
		{"a reference to a space after a break, in the SUBJECT", strings.Replace(ok, "Loss mitigation procedures.", "Loss\n&#32;mitigation procedures.", 1)},
		{"a reference to a space", strings.Replace(ok, "Text two.", "Text&#32;two.", 1)},
		{"a hex reference to a space", strings.Replace(ok, "Text two.", "Text&#x20;two.", 1)},
		{"a reference to a tab", strings.Replace(ok, "Text two.", "Text&#9;two.", 1)},
		{"a reference to a no-break space", strings.Replace(ok, "Text two.", "Text&#160;two.", 1)},
		{"a hex reference to a no-break space", strings.Replace(ok, "Text two.", "Text&#xa0;two.", 1)},
		{"a reference to a next line", strings.Replace(ok, "Text two.", "Text&#x85;two.", 1)},
		{"a reference to a thin space", strings.Replace(ok, "Text two.", "Text&#x2009;two.", 1)},
		{"a reference to a line separator", strings.Replace(ok, "Text two.", "Text&#8232;two.", 1)},
		{"a reference to an ideographic space", strings.Replace(ok, "Text two.", "Text&#x3000;two.", 1)},
		{"a zero-padded reference to a space", strings.Replace(ok, "Text two.", "Text&#0032;two.", 1)},
		{"a reference to a space in an E", strings.Replace(ok, "Heading.</E>", "Heading.&#32;</E>", 1)},
		{"a reference to a space in the CITA", strings.Replace(ok, "Feb. 14, 2013]", "Feb.&#32;14, 2013]", 1)},
		{"a reference to a space in an attribute", strings.Replace(ok, `HEADING="PART 1024"`, `HEADING="PART&#32;1024"`, 1)},
		{"a reference to a line break in an attribute", strings.Replace(ok, `SEQ="1"`, `SEQ="&#10;1"`, 1)},
		{"a SECTION with no P", strings.Replace(ok, twoParas, "", 1)},
		{"a SECTION with no CITA", strings.Replace(ok, "        <CITA>[78 FR 10876, Feb. 14, 2013]</CITA>\n", "", 1)},
		{"two CITAs", strings.Replace(ok, "</CITA>", "</CITA>\n        <CITA>x</CITA>", 1)},
		{"a page break in an E", strings.Replace(ok, "Heading.</E>", `Head<PRTPAGE P="1"/>ing.</E>`, 1)},
		{"a page break after the CITA", strings.Replace(ok, "</CITA>", "</CITA>\n        <PRTPAGE P=\"1\"/>", 1)},
	} {
		if tc.granule == ok {
			t.Errorf("%s: the case doesn't change the base granule", tc.name)
			continue
		}
		s, err := govinfo.Read([]byte(tc.granule), "1024.41", govinfo.EditionDate)
		if !errors.Is(err, govinfo.ErrNotGranule) {
			t.Errorf("%s: Read = %v, %v; want %v", tc.name, s, err, govinfo.ErrNotGranule)
		}
	}
}

// TestLayoutIsDeletedInEachRunOnItsOwn: a line break ending one run of
// character data and the spaces starting the next, inside an <E>, are not
// one break and its indentation: the spaces are text.
func TestLayoutIsDeletedInEachRunOnItsOwn(t *testing.T) {
	body := strings.Replace(twoParas, "<P>(b) Text two.</P>", "<P>(b) x\n<E T=\"03\">   y</E></P>", 1)
	s, err := govinfo.Read([]byte(granule("2026-01-01", "§ 1024.41", "Loss mitigation procedures.", body)), "1024.41", govinfo.EditionDate)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.Paragraphs[1], "(b) x   y"; got != want {
		t.Errorf("paragraph 2 = %q, want %q: the break ends its run, the spaces start the <E>'s", got, want)
	}
}

// TestEveryEdgeIsText: the layout rule is the only thing deleted, so
// White_Space at a text's edge that the rule doesn't delete is text, as it
// is in decision 13, which trims nothing: in every paragraph alike, the
// last's as much as an interior one's, and in the SECTNO and the SUBJECT
// (worklist round 2, §1: C-N2, R-N2).
func TestEveryEdgeIsText(t *testing.T) {
	thirteen := "\n" + strings.Repeat(" ", 13)
	for _, tc := range []struct {
		name, subject, body string
		heading, para       string // the heading, and the paragraph changed, as Read gives them
		at                  int    // that paragraph's index
	}{
		{"the last ending in a space", "Loss mitigation procedures.", strings.Replace(twoParas, "<P>(b) Text two.</P>", "<P>(b) Text two. </P>", 1),
			"§ 1024.41 Loss mitigation procedures.", "(b) Text two. ", 1},
		{"the last ending in a no-break space", "Loss mitigation procedures.", strings.Replace(twoParas, "<P>(b) Text two.</P>", "<P>(b) Text two."+nbsp+"\n        </P>", 1),
			"§ 1024.41 Loss mitigation procedures.", "(b) Text two." + nbsp, 1},
		{"the last ending in a break and 13 spaces", "Loss mitigation procedures.", strings.Replace(twoParas, "<P>(b) Text two.</P>", "<P>(b) Text two."+thirteen+"</P>", 1),
			"§ 1024.41 Loss mitigation procedures.", "(b) Text two. ", 1},
		{"the last starting with a space", "Loss mitigation procedures.", strings.Replace(twoParas, "<P>(b) Text two.</P>", "<P> (b) Text two.</P>", 1),
			"§ 1024.41 Loss mitigation procedures.", " (b) Text two.", 1},
		{"the first ending in a space", "Loss mitigation procedures.", strings.Replace(twoParas, "            continues.\n", "            continues. \n", 1),
			"§ 1024.41 Loss mitigation procedures.", "(a) Heading. Text one continues. ", 0},
		{"the SUBJECT ending in a no-break space", "Loss mitigation procedures." + nbsp, twoParas,
			"§ 1024.41 Loss mitigation procedures." + nbsp, "(b) Text two.", 1},
		{"the SUBJECT ending in a break and 13 spaces", "Loss mitigation procedures." + thirteen, twoParas,
			"§ 1024.41 Loss mitigation procedures. ", "(b) Text two.", 1},
		{"the SUBJECT starting with a space", " Loss mitigation procedures.", twoParas,
			"§ 1024.41  Loss mitigation procedures.", "(b) Text two.", 1},
	} {
		s, err := govinfo.Read([]byte(granule("2026-01-01", "§ 1024.41", tc.subject, tc.body)), "1024.41", govinfo.EditionDate)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if s.Heading != tc.heading || s.Paragraphs[tc.at] != tc.para {
			t.Errorf("%s: Heading %q, paragraph %d %q; want %q, %q: an edge's White_Space is text", tc.name, s.Heading, tc.at+1, s.Paragraphs[tc.at], tc.heading, tc.para)
		}
	}
}

// TestAGranuleWithAByteOrderMarkReadsAsOneWithout: a UTF-8 byte order mark
// may open a document and is not part of it (XML 1.0, 4.3.3), as
// parse.WellFormed reads one (worklist round 2, §6: M49).
func TestAGranuleWithAByteOrderMarkReadsAsOneWithout(t *testing.T) {
	g := read(t, granule41)
	base, err := govinfo.Read(g, "1024.41", govinfo.EditionDate)
	if err != nil {
		t.Fatal(err)
	}
	s, err := govinfo.Read(append([]byte("\xef\xbb\xbf"), g...), "1024.41", govinfo.EditionDate)
	if err != nil {
		t.Fatalf("with a byte order mark: %v", err)
	}
	if s.Heading != base.Heading || strings.Join(s.Paragraphs, "|") != strings.Join(base.Paragraphs, "|") {
		t.Errorf("with a byte order mark: Heading %q and %d paragraphs; want the granule's own", s.Heading, len(s.Paragraphs))
	}
	if _, err := govinfo.Read(append([]byte("\xef\xbb\xbf\xef\xbb\xbf"), g...), "1024.41", govinfo.EditionDate); !errors.Is(err, govinfo.ErrNotGranule) {
		t.Errorf("two byte order marks: %v; want %v, the second is text outside the root", err, govinfo.ErrNotGranule)
	}
}

// TestReadNamesWhatItRefusesQuoted: the output of fetch --verify is saved
// as evidence, so a value from the granule in an error is quoted, never
// printed as written: no line break or escape in it can start a line of its
// own (worklist round 2, §3: C-N4).
func TestReadNamesWhatItRefusesQuoted(t *testing.T) {
	ok := granule("2026-01-01", "§ 1024.41", "Loss mitigation procedures.", twoParas)
	const forged = "\n1024.41: govinfo 2026 edition: forged"
	for _, tc := range []struct{ name, granule string }{
		{"a comment in a P", strings.Replace(ok, "Text two.", "Text <!--"+forged+" -->two.", 1)},
		{"a processing instruction in a P", strings.Replace(ok, "Text two.", "Text <?x"+forged+" ?>two.", 1)},
		{"an element in a namespace whose name has a line break", strings.Replace(ok, "<CITA>", `<x:Y xmlns:x="`+forged+`"/><CITA>`, 1)},
		{"an attribute in a namespace whose name has a line break", strings.Replace(ok, `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`, `xmlns:xsi="`+forged+`"`, 1)},
		{"a root in a namespace whose name has a line break", strings.Replace(strings.Replace(ok, "<CFRGRANULE ", `<x:CFRGRANULE xmlns:x="`+forged+`" `, 1), "</CFRGRANULE>", "</x:CFRGRANULE>", 1)},
	} {
		_, err := govinfo.Read([]byte(tc.granule), "1024.41", govinfo.EditionDate)
		if !errors.Is(err, govinfo.ErrNotGranule) {
			t.Errorf("%s: %v; want %v", tc.name, err, govinfo.ErrNotGranule)
			continue
		}
		if msg := err.Error(); strings.ContainsAny(msg, "\n\x1b") || !strings.Contains(msg, `\n1024.41: govinfo 2026 edition: forged`) {
			t.Errorf("%s: %q; want the value quoted, no line break or escape as written", tc.name, msg)
		}
	}
}

// TestAReferenceToAnythingElseIsText: only a reference to a White_Space
// character is refused; one to any other character is read as that
// character.
func TestAReferenceToAnythingElseIsText(t *testing.T) {
	s, err := govinfo.Read([]byte(granule("2026-01-01", "§ 1024.41", "Loss mitigation procedures.",
		strings.Replace(twoParas, "Text two.", "Text &#167; &#x2014; &amp; two.", 1))), "1024.41", govinfo.EditionDate)
	if err != nil {
		t.Fatal(err)
	}
	if want := "(b) Text § — & two."; s.Paragraphs[1] != want {
		t.Errorf("paragraph 2 = %q, want %q", s.Paragraphs[1], want)
	}
}

func TestLinkPathAsksTheLinkServiceForOneSectionsXML(t *testing.T) {
	got, err := govinfo.LinkPath("12", "1024.41", "2026")
	if want := "/link/cfr/12/1024?link-type=xml&sectionnum=41&year=2026"; err != nil || got != want {
		t.Errorf("LinkPath = %q, %v; want %q", got, err, want)
	}
	for _, s := range []string{"1024", "1024.", ".41", "1024.41.1", "1024.4a", "", "1024.41&year=2025"} {
		if got, err := govinfo.LinkPath("12", s, "2026"); err == nil {
			t.Errorf("LinkPath(%q) = %q; want an error", s, got)
		}
	}
}
