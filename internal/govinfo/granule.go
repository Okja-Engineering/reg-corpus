package govinfo

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// DefaultBaseURL is govinfo, whose link service and content need no API key
// (plan "Upstream calls").
const DefaultBaseURL = "https://www.govinfo.gov"

// The edition fetch --verify compares (spec "Concerns" 4; ADR 0014): the CFR
// 2026 annual edition of title 12, revised as of 2026-01-01, pinned by year,
// never "the most recent".
const (
	EditionTitle           = "12"
	EditionYear            = "2026"
	EditionDate  meta.Date = "2026-01-01"
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
// string raw otherwise: what fetch --verify compares with eCFR's heading
// and paragraphs, and hashes with anchors.JoinSection to compare with the
// stored sha256_text.
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
// before a break, a thirteenth space after one, a tab, a no-break space.
// Read refuses a granule with a CR or a character reference to a line
// break, so each LF here was written as one. A change in govinfo's layout
// can so give a false mismatch, and a false match in one way only: where
// govinfo wrote a literal LF, with no space before it and up to 12 after
// it, to mean the space between two words, and eCFR's text has no space
// there, the two read alike (doc.go; ADR 0014, Decision 2).
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

// breakReference is a character reference to a CR or an LF, decimal or
// hexadecimal, with any leading zeros.
var breakReference = regexp.MustCompile(`&#(0*1[03]|[xX]0*[aAdD]);`)

// Read reads granule, govinfo's XML for section in title 12's edition dated
// date, and returns its section text. It reads the granule's whole form, a
// closed grammar (grammar), and refuses anything else, never reading past
// it (ErrNotGranule):
//   - valid UTF-8, with no CR and no character reference to a CR or an LF:
//     a granule's lines end in a literal LF, the only line break Layout
//     deletes;
//   - one well-formed document (parse.WellFormed, fetch's check) whose root
//     is CFRGRANULE, holding its FDSYS and then one SECTION;
//   - the FDSYS holding CFRTITLE, CFRTITLETEXT, VOL, DATE, COVERONLY,
//     ORIGINALDATE and ANCESTORS, in that order, each at most once, the
//     ANCESTORS PARENTs: its CFRTITLE must be EditionTitle, one PARENT
//     must name the section's part ("PART 1024"), and its DATE must be date
//     (else ErrOtherEdition);
//   - the SECTION holding a SECTNO, a SUBJECT, then <P>s and page breaks
//     (PRTPAGE), and last a citation note (CITA); its SECTNO, normalized,
//     must be "§ " and section (else ErrOtherSection);
//   - a <P> holding text, <E>s of text and empty PRTPAGEs, and some text;
//   - an attribute only where govinfo's granules have one: the root's
//     schema location, a PARENT's HEADING and SEQ, a PRTPAGE's page
//     number, and an <E>'s T="03" (italics);
//   - the CFRTITLE, DATE and SECTNO each one run of character data as
//     written, with no reference, CDATA section or anything else in it; no
//     comment or processing instruction inside the root.
//
// Each run of character data has its layout deleted (Layout) on its own, so
// a run never crosses a tag, and each text then has the White_Space at its
// edges trimmed, the last paragraph's as any other's. The heading is the
// SECTNO, one space and the SUBJECT, and a paragraph its text with each
// <E>'s text in place and each PRTPAGE dropped. The CITA is left out, as
// sha256_text leaves out eCFR's (ADR 0013, F3).
func Read(granule []byte, section string, date meta.Date) (*Section, error) {
	notGranule := func(format string, a ...any) error {
		return fmt.Errorf("%w: %s", ErrNotGranule, fmt.Sprintf(format, a...))
	}
	sn := sectionNumber.FindStringSubmatch(section)
	switch {
	case sn == nil:
		return nil, fmt.Errorf("govinfo: %q is not a section number written <part>.<section>", section)
	case !utf8.Valid(granule):
		return nil, notGranule("not valid UTF-8")
	}
	if i := bytes.IndexByte(granule, '\r'); i >= 0 {
		return nil, notGranule("a CR at byte %d: a granule's lines end in LF alone, the one line break deleted as layout", i)
	}
	if loc := breakReference.FindIndex(granule); loc != nil {
		return nil, notGranule("%s at byte %d, a reference to a line break: a granule writes its line breaks as they are, and only those are layout", granule[loc[0]:loc[1]], loc[0])
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
	doc := bytes.TrimPrefix(granule, []byte("\xef\xbb\xbf"))
	r := &reader{}
	d := xml.NewDecoder(bytes.NewReader(doc))
	d.Strict = true
	for {
		from := d.InputOffset()
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil { // not reached: WellFormed has decoded the same bytes
			return nil, notGranule("%v", err)
		}
		if err := r.token(tok, doc[from:d.InputOffset()]); err != nil {
			return nil, notGranule("%v", err)
		}
	}
	if got := r.text["CFRTITLE"]; got != EditionTitle {
		return nil, notGranule("CFRTITLE %q, want %s", got, EditionTitle)
	}
	if want := "PART " + sn[1]; !slices.Equal(r.parts, []string{want}) {
		return nil, notGranule("the ANCESTORS name the parts %q, want %q alone", r.parts, want)
	}
	if got := r.text["DATE"]; got != string(date) {
		return nil, fmt.Errorf("%w: DATE %q, want %s", ErrOtherEdition, got, date)
	}
	sectno := r.text["SECTNO"]
	if n, err := anchors.Normalize(sectno); err != nil || n != "§ "+section {
		return nil, fmt.Errorf("%w: SECTNO %q, want § %s", ErrOtherSection, sectno, section)
	}
	return &Section{Date: date, Heading: sectno + " " + r.text["SUBJECT"], Paragraphs: r.paras}, nil
}

// What an element holds.
type holds int

const (
	elements  holds = iota // child elements, and XML whitespace between them
	text                   // character data only
	plainText              // one run of character data as written
	paragraph              // character data, <E>s and PRTPAGEs, and some text
	nothing                // nothing: an empty element
)

// element is the grammar's rule for an element of a given name.
type element struct {
	parents []string          // the elements it may be a child of ("": it is the root)
	holds   holds             // what it holds
	order   int               // its place among its parent's children, none after a later one
	once    bool              // at most one in its parent
	needs   []string          // the children it must hold
	attrs   map[string]string // the attributes it may carry: one with a value must carry it, "" any value
}

// xsi is the XML Schema instance namespace, as encoding/xml names an
// attribute in it.
const xsi = "http://www.w3.org/2001/XMLSchema-instance"

// grammar is every element a granule holds, as govinfo's granules of
// 1024.30, 1024.31 and 1024.41 in the 2026 edition do: what Read reads.
var grammar = map[string]element{
	"CFRGRANULE":   {parents: []string{""}, holds: elements, needs: []string{"FDSYS", "SECTION"}, attrs: map[string]string{"xmlns:xsi": "", xsi + ":noNamespaceSchemaLocation": ""}},
	"FDSYS":        {parents: []string{"CFRGRANULE"}, holds: elements, order: 0, once: true, needs: []string{"CFRTITLE", "DATE"}},
	"CFRTITLE":     {parents: []string{"FDSYS"}, holds: plainText, order: 0, once: true},
	"CFRTITLETEXT": {parents: []string{"FDSYS"}, holds: text, order: 1, once: true},
	"VOL":          {parents: []string{"FDSYS"}, holds: text, order: 2, once: true},
	"DATE":         {parents: []string{"FDSYS"}, holds: plainText, order: 3, once: true},
	"COVERONLY":    {parents: []string{"FDSYS"}, holds: text, order: 4, once: true},
	"ORIGINALDATE": {parents: []string{"FDSYS"}, holds: text, order: 5, once: true},
	"ANCESTORS":    {parents: []string{"FDSYS"}, holds: elements, order: 6, once: true},
	"PARENT":       {parents: []string{"ANCESTORS"}, holds: text, attrs: map[string]string{"HEADING": "", "SEQ": ""}},
	"SECTION":      {parents: []string{"CFRGRANULE"}, holds: elements, order: 1, once: true, needs: []string{"SECTNO", "SUBJECT"}},
	"SECTNO":       {parents: []string{"SECTION"}, holds: plainText, order: 0, once: true},
	"SUBJECT":      {parents: []string{"SECTION"}, holds: text, order: 1, once: true},
	"P":            {parents: []string{"SECTION"}, holds: paragraph, order: 2},
	"PRTPAGE":      {parents: []string{"SECTION", "P"}, holds: nothing, order: 2, attrs: map[string]string{"P": ""}},
	"E":            {parents: []string{"P"}, holds: text, order: 2, attrs: map[string]string{"T": "03"}},
	"CITA":         {parents: []string{"SECTION"}, holds: text, order: 3, once: true},
}

// frame is an open element as the reader reads it.
type frame struct {
	name  string
	rule  element
	last  int             // the latest order among its children so far
	seen  map[string]bool // its children's names so far
	runs  int             // its runs of character data
	plain bool            // each run as written
	text  strings.Builder // its text, each run's layout deleted; an <E>'s goes to its <P>
}

// reader reads a granule's tokens in order (Read).
type reader struct {
	stack []*frame
	text  map[string]string // each CFRTITLE, DATE, SECTNO and SUBJECT text
	parts []string          // each PARENT HEADING that names a part
	paras []string
}

func (r *reader) top() *frame {
	if len(r.stack) == 0 {
		return nil
	}
	return r.stack[len(r.stack)-1]
}

// token reads one token, raw its bytes as written, or says why the granule
// isn't one Read reads.
func (r *reader) token(tok xml.Token, raw []byte) error {
	f := r.top()
	switch t := tok.(type) {
	case xml.StartElement:
		return r.start(t)
	case xml.EndElement:
		return r.end()
	case xml.CharData:
		if f == nil { // outside the root: WellFormed allows only XML whitespace
			return nil
		}
		return r.chars(f, string(t), string(raw))
	case xml.Comment, xml.ProcInst, xml.Directive:
		if f != nil {
			return fmt.Errorf("%s inside <%s>, which this doesn't read", strings.TrimSpace(string(raw)), f.name)
		}
	}
	return nil // a comment or processing instruction outside the root
}

// start checks an element opening where the stack is, by the grammar.
func (r *reader) start(t xml.StartElement) error {
	name, parent := parse.Qualified(t.Name), r.top()
	pname := ""
	if parent != nil {
		pname = parent.name
	}
	rule, ok := grammar[name]
	switch {
	case !ok || !slices.Contains(rule.parents, pname):
		return fmt.Errorf("<%s> inside <%s>, where this reads none", name, pname)
	case parent != nil && rule.order < parent.last:
		return fmt.Errorf("<%s> after another child of <%s> that comes after it", name, pname)
	case parent != nil && rule.once && parent.seen[name]:
		return fmt.Errorf("a second <%s> in <%s>", name, pname)
	}
	carried := map[string]string{}
	for _, a := range t.Attr {
		an := parse.Qualified(a.Name)
		if want, ok := rule.attrs[an]; !ok || (want != "" && a.Value != want) {
			return fmt.Errorf("<%s %s=%q>: an attribute this doesn't read", name, an, a.Value)
		}
		carried[an] = a.Value
	}
	for an, want := range rule.attrs {
		if _, ok := carried[an]; want != "" && !ok {
			return fmt.Errorf("<%s> without its %s=%q", name, an, want)
		}
	}
	if h := carried["HEADING"]; name == "PARENT" && strings.HasPrefix(h, "PART ") {
		r.parts = append(r.parts, h)
	}
	if parent != nil {
		parent.last = rule.order
		if parent.seen == nil {
			parent.seen = map[string]bool{}
		}
		parent.seen[name] = true
	}
	r.stack = append(r.stack, &frame{name: name, rule: rule, plain: true})
	return nil
}

// chars reads a run of character data, s as decoded and raw as written,
// in the open element f.
func (r *reader) chars(f *frame, s, raw string) error {
	switch f.rule.holds {
	case elements:
		if strings.Trim(s, " \t\n") != "" {
			return fmt.Errorf("text %q in <%s>, which holds elements only", s, f.name)
		}
		return nil
	case nothing:
		return fmt.Errorf("text %q in <%s>, which holds none", s, f.name)
	}
	f.runs++
	f.plain = f.plain && s == raw
	into := f
	if f.name == "E" {
		into = r.stack[len(r.stack)-2] // its <P>: an <E> is read only there
	}
	into.text.WriteString(Layout(s))
	return nil
}

// end closes the open element: its children and text checked, its text
// kept where Read uses it.
func (r *reader) end() error {
	f := r.top()
	r.stack = r.stack[:len(r.stack)-1]
	for _, n := range f.rule.needs {
		if !f.seen[n] {
			return fmt.Errorf("<%s> without its <%s>", f.name, n)
		}
	}
	t := strings.TrimFunc(f.text.String(), func(c rune) bool { return unicode.Is(unicode.White_Space, c) })
	switch {
	case f.rule.holds == plainText && (f.runs != 1 || !f.plain):
		return fmt.Errorf("<%s> is not one run of text as written: no reference, CDATA section, comment or element in it", f.name)
	case f.rule.holds == paragraph && t == "":
		return fmt.Errorf("an empty <P>: a paragraph with no text is not one this reads")
	case f.rule.holds == paragraph:
		r.paras = append(r.paras, t)
	case f.name == "CFRTITLE" || f.name == "DATE" || f.name == "SECTNO" || f.name == "SUBJECT":
		if r.text == nil {
			r.text = map[string]string{}
		}
		r.text[f.name] = t
	}
	return nil
}
