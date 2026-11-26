package govinfo

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// DefaultBaseURL is govinfo, whose link service and content need no API key
// (plan "Upstream calls").
const DefaultBaseURL = "https://www.govinfo.gov"

// The edition fetch --verify compares (spec "Concerns" 4; ADR 0014): the CFR
// 2026 annual edition, title 12 revised as of 2026-01-01, pinned by year,
// never "the most recent".
const (
	EditionYear           = "2026"
	EditionDate meta.Date = "2026-01-01"
)

// Indent is the most indentation govinfo's pretty-printer puts after a line
// break inside a section's text: 12 spaces, a <P>'s content being three
// levels of four in. A line break and up to Indent spaces after it are
// layout; a thirteenth space is text (ADR 0014, F3: C1).
const Indent = 12

// The granules Read refuses, each named in the error it returns.
var (
	ErrNotGranule   = errors.New("not a govinfo CFR section granule as this reads one")
	ErrOtherSection = errors.New("a granule for another section")
	ErrOtherEdition = errors.New("a granule of another edition")
)

// Section is a granule's section text, its layout deleted (Layout), each
// string raw otherwise: what fetch --verify hashes with anchors.JoinSection
// and compares with the stored sha256_text.
type Section struct {
	Date       meta.Date // the FDSYS DATE: the edition's "revised as of" date
	Heading    string    // the SECTNO, one space, the SUBJECT
	Paragraphs []string  // each <P>'s text: <E>'s text kept, PRTPAGE dropped
}

// sectionNumber is a section number as the source table names one:
// <part>.<section>, both digits.
var sectionNumber = regexp.MustCompile(`^([0-9]+)\.([0-9]+)$`)

// LinkPath is the path and query, below DefaultBaseURL, at which govinfo's
// link service answers with a redirect to section's granule XML in year's
// annual edition of title: /link/cfr/<title>/<part>?link-type=xml&
// sectionnum=<n>&year=<year>, the query's keys sorted. A section not
// written <part>.<section> is an error.
func LinkPath(title, section, year string) (string, error) {
	m := sectionNumber.FindStringSubmatch(section)
	if m == nil {
		return "", fmt.Errorf("govinfo: %q is not a section number written <part>.<section>", section)
	}
	q := url.Values{"link-type": {"xml"}, "sectionnum": {m[2]}, "year": {year}}
	return "/link/cfr/" + url.PathEscape(title) + "/" + m[1] + "?" + q.Encode(), nil
}

// Layout deletes govinfo's layout from s, a run of a granule's character
// data: each line break (LF) together with the spaces after it, up to
// Indent of them (ADR 0014, F3: C1). Every other character is text: a space
// before a break, a thirteenth space after one, a tab, a no-break space. A
// change in govinfo's layout can so give a false mismatch, and a false
// match only on whether a space stands at a line break (doc.go).
func Layout(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] != '\n' { // LF and space are ASCII: never inside a UTF-8 sequence
			b.WriteByte(s[i])
			i++
			continue
		}
		i++
		for n := 0; n < Indent && i < len(s) && s[i] == ' '; n++ {
			i++
		}
	}
	return b.String()
}

// Read reads granule, govinfo's XML for section in the edition dated date,
// and returns its section text. It checks it first: valid UTF-8, one
// well-formed document (parse.WellFormed, fetch's check), whose root is
// CFRGRANULE; else ErrNotGranule. Then:
//   - FDSYS holds one DATE, which must be date (else ErrOtherEdition);
//   - the root holds one SECTION, in which a SECTNO, a SUBJECT, then <P>s
//     and page breaks (PRTPAGE), and last a citation note (CITA), each
//     optional but the first two, and nothing else, are its children,
//     with only whitespace between them;
//   - the SECTNO, its layout deleted and normalized, is "§ " and section
//     (else ErrOtherSection);
//   - a SECTNO and a SUBJECT hold text only, a <P> text, <E>s of text and
//     empty PRTPAGEs; a CITA is not read.
//
// Anything else is ErrNotGranule: a structure this doesn't read is
// refused, not read past. Each run of character data has its layout
// deleted (Layout) on its own, so a run never crosses a tag; the heading
// is the SECTNO, one space and the SUBJECT, and a paragraph its text with
// each <E>'s text in place and each PRTPAGE dropped. The CITA is left out,
// as sha256_text leaves out eCFR's (ADR 0013, F3).
func Read(granule []byte, section string, date meta.Date) (*Section, error) {
	notGranule := func(format string, a ...any) error {
		return fmt.Errorf("%w: %s", ErrNotGranule, fmt.Sprintf(format, a...))
	}
	if !utf8.Valid(granule) {
		return nil, notGranule("not valid UTF-8")
	}
	err := parse.WellFormed(granule, func(t xml.StartElement) error {
		if got := parse.Qualified(t.Name); got != "CFRGRANULE" {
			return fmt.Errorf("root element <%s>, want <CFRGRANULE>", got)
		}
		return nil
	})
	if err != nil {
		return nil, notGranule("%v", err)
	}
	r := &reader{}
	d := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(granule, []byte("\xef\xbb\xbf"))))
	d.Strict = true
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil { // not reached: WellFormed has decoded the same bytes
			return nil, notGranule("%v", err)
		}
		if err := r.token(tok); err != nil {
			return nil, notGranule("%v", err)
		}
	}
	switch {
	case len(r.dates) != 1:
		return nil, notGranule("%d DATEs in FDSYS, want 1", len(r.dates))
	case r.sections != 1:
		return nil, notGranule("%d SECTIONs, want 1", r.sections)
	case r.phase < afterSubject:
		return nil, notGranule("a SECTION without its SECTNO and SUBJECT")
	}
	if got := strings.TrimSpace(r.dates[0]); got != string(date) {
		return nil, fmt.Errorf("%w: DATE %q, want %s", ErrOtherEdition, got, date)
	}
	sectno := Layout(r.sectno.String())
	if n, err := anchors.Normalize(sectno); err != nil || n != "§ "+section {
		return nil, fmt.Errorf("%w: SECTNO %q, want § %s", ErrOtherSection, sectno, section)
	}
	return &Section{
		Date:       date,
		Heading:    sectno + " " + Layout(r.subject.String()),
		Paragraphs: r.paras,
	}, nil
}

// Where a SECTION's children have reached: its SECTNO comes first, its
// SUBJECT second, then its <P>s and page breaks, and its CITA last.
const (
	beforeSectno = iota
	afterSectno
	afterSubject
	afterCita
)

// childAt is where each child a SECTION may hold may come: a SECTNO
// first, a SUBJECT second, then any number of <P>s and PRTPAGEs, then one
// CITA.
var childAt = map[string]int{
	"SECTNO": beforeSectno, "SUBJECT": afterSectno,
	"P": afterSubject, "PRTPAGE": afterSubject, "CITA": afterSubject,
}

// reader reads a granule's tokens in order (Read).
type reader struct {
	stack    []string // the open elements' names, the root first
	dates    []string // each FDSYS DATE's text
	sections int
	phase    int
	sectno   strings.Builder
	subject  strings.Builder
	paras    []string
	para     strings.Builder // the open <P>'s text, its layout deleted
}

// in is the name of the open element depth levels in (1: the root), or "".
func (r *reader) in(depth int) string {
	if len(r.stack) < depth {
		return ""
	}
	return r.stack[depth-1]
}

// token reads one token, or says why the granule isn't one Read reads.
func (r *reader) token(tok xml.Token) error {
	switch t := tok.(type) {
	case xml.StartElement:
		name := parse.Qualified(t.Name)
		if err := r.start(name); err != nil {
			return err
		}
		r.stack = append(r.stack, name)
	case xml.EndElement:
		name := r.stack[len(r.stack)-1]
		r.stack = r.stack[:len(r.stack)-1]
		if name == "P" && len(r.stack) == 2 {
			r.paras = append(r.paras, r.para.String())
			r.para.Reset()
		}
	case xml.CharData:
		return r.text(string(t))
	}
	return nil // comments and processing instructions are not text
}

// start checks an element named name opening where the stack is.
func (r *reader) start(name string) error {
	parent := ""
	if len(r.stack) > 0 {
		parent = r.stack[len(r.stack)-1]
	}
	if name == "SECTION" {
		if len(r.stack) != 1 {
			return fmt.Errorf("a SECTION inside <%s>; a granule's SECTION is the root's child", parent)
		}
		r.sections++
		return nil
	}
	if r.in(2) != "SECTION" { // outside the section text
		if name == "DATE" && parent == "FDSYS" && len(r.stack) == 2 {
			r.dates = append(r.dates, "")
		}
		return nil
	}
	if len(r.stack) == 2 { // a child of the SECTION
		if at, ok := childAt[name]; !ok || r.phase != at {
			return fmt.Errorf("<%s> where the SECTION's next child is %s", name, r.expected())
		}
		switch name {
		case "SECTNO", "SUBJECT":
			r.phase++
		case "CITA":
			r.phase = afterCita
		}
		return nil
	}
	switch {
	case r.in(3) == "CITA": // a citation note is not read
		return nil
	case parent == "P" && (name == "E" || name == "PRTPAGE"):
		return nil
	}
	return fmt.Errorf("<%s> inside <%s>, which holds no element this reads", name, parent)
}

// expected names what a SECTION's next child may be.
func (r *reader) expected() string {
	switch r.phase {
	case beforeSectno:
		return "its SECTNO"
	case afterSectno:
		return "its SUBJECT"
	case afterSubject:
		return "a P, a PRTPAGE or its CITA"
	}
	return "none, after its CITA"
}

// text reads a run of character data where the stack is.
func (r *reader) text(s string) error {
	if len(r.stack) == 3 && r.in(2) == "FDSYS" && r.in(3) == "DATE" {
		r.dates[len(r.dates)-1] += s
		return nil
	}
	if r.in(2) != "SECTION" {
		return nil
	}
	switch top := r.stack[len(r.stack)-1]; {
	case len(r.stack) == 2:
		if strings.Trim(s, " \t\r\n") != "" {
			return fmt.Errorf("text %q in the SECTION, outside its children", s)
		}
	case top == "SECTNO" && len(r.stack) == 3:
		r.sectno.WriteString(s)
	case top == "SUBJECT" && len(r.stack) == 3:
		r.subject.WriteString(s)
	case r.in(3) == "P" && (len(r.stack) == 3 || top == "E"):
		r.para.WriteString(Layout(s))
	case top == "PRTPAGE":
		return fmt.Errorf("text %q in a PRTPAGE, a page break that holds none", s)
	}
	return nil // a CITA's text
}
