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
