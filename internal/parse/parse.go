package parse

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// Parse parses one eCFR section's XML into a Tree. It fails with an *Error
// on any label it cannot place, and on any element or text a section of
// 1024.41's form does not hold.
func Parse(data []byte) (*Tree, error) {
	p := &parser{data: data, dec: xml.NewDecoder(bytes.NewReader(data))}
	if err := p.section(); err != nil {
		return nil, err
	}
	return p.tree, nil
}

type parser struct {
	data  []byte
	dec   *xml.Decoder
	tree  *Tree
	stack []level // the open levels; stack[k] holds the last label of Kind k
}

type level struct {
	node  *Node
	value int
}

// next returns the next token and the byte offset where it starts. Tokens
// are contiguous, so a token starts where the one before it ended.
func (p *parser) next() (xml.Token, int, error) {
	off := int(p.dec.InputOffset())
	tok, err := p.dec.Token()
	if err == io.EOF {
		return nil, off, err
	}
	if err != nil {
		var se *xml.SyntaxError
		if errors.As(err, &se) {
			return nil, off, p.fail(int(p.dec.InputOffset()), Malformed, "", se.Msg)
		}
		return nil, off, p.fail(off, Malformed, "", err.Error())
	}
	return xml.CopyToken(tok), off, nil
}

func (p *parser) fail(off int, reason Reason, label, detail string) *Error {
	off = min(off, len(p.data))
	line := 1 + bytes.Count(p.data[:off], []byte("\n"))
	col := off - bytes.LastIndexByte(p.data[:off], '\n')
	return &Error{Line: line, Column: col, Reason: reason, Label: label, Detail: detail}
}

func blank(cd xml.CharData) bool { return len(bytes.TrimSpace(cd)) == 0 }

// section reads the one DIV8: a HEAD, the P elements, then at most one CITA.
func (p *parser) section() error {
	for p.tree == nil {
		tok, off, err := p.next()
		if err == io.EOF {
			return p.fail(off, Malformed, "", "no <DIV8> section element")
		} else if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.ProcInst:
		case xml.CharData:
			if !blank(t) {
				return p.fail(off, Malformed, "", "text outside the section element")
			}
		case xml.StartElement:
			if t.Name.Local != "DIV8" || attr(t, "TYPE") != "SECTION" {
				return p.fail(off, Malformed, "", "the root element is not <DIV8 TYPE=\"SECTION\">")
			}
			p.tree = &Tree{Section: attr(t, "N")}
		default:
			return p.fail(off, Malformed, "", fmt.Sprintf("unexpected %T before the section element", tok))
		}
	}

	var head, cita bool
	for {
		tok, off, err := p.next()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.CharData:
			if !blank(t) {
				return p.fail(off, Malformed, "", "text directly in the section element")
			}
		case xml.EndElement: // </DIV8>: the decoder has checked it matches
			return p.end()
		case xml.StartElement:
			switch {
			case t.Name.Local == "HEAD" && !head:
				head = true
				if p.tree.Heading, err = p.text("HEAD"); err != nil {
					return err
				}
			case t.Name.Local == "P" && head && !cita:
				if err := p.paragraph(); err != nil {
					return err
				}
			case t.Name.Local == "CITA" && head && !cita:
				cita = true
				if p.tree.Citation, err = p.text("CITA"); err != nil {
					return err
				}
			default:
				return p.fail(off, Malformed, "", fmt.Sprintf("unexpected <%s> in the section element", t.Name.Local))
			}
		default:
			return p.fail(off, Malformed, "", fmt.Sprintf("unexpected %T in the section element", tok))
		}
	}
}

// end checks that only blank text follows the section element.
func (p *parser) end() error {
	for {
		tok, off, err := p.next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if cd, ok := tok.(xml.CharData); !ok || !blank(cd) {
			return p.fail(off, Malformed, "", "content after the section element")
		}
	}
}

func attr(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// text reads the character data of an element that holds no elements.
func (p *parser) text(name string) (string, error) {
	var b strings.Builder
	for {
		tok, off, err := p.next()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.CharData:
			b.Write(t)
		case xml.EndElement:
			return b.String(), nil
		default:
			return "", p.fail(off, Malformed, "", fmt.Sprintf("unexpected %T in <%s>", tok, name))
		}
	}
}

// para is one <P>'s character data, with each byte's italic flag and the
// byte offset in the XML it was decoded from.
type para struct {
	text   string
	italic []bool
	raw    []int
	end    int // the offset of </P>
}

// readPara reads a <P>'s content: character data and <I> elements, not nested.
func (p *parser) readPara() (*para, error) {
	var b strings.Builder
	pa := &para{}
	italic := false
	for {
		tok, off, err := p.next()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.CharData:
			b.Write(t)
			end := int(p.dec.InputOffset())
			for _, o := range rawOffsets(p.data[off:end], len(t)) {
				pa.raw = append(pa.raw, off+o)
				pa.italic = append(pa.italic, italic)
			}
		case xml.StartElement:
			if t.Name.Local != "I" || italic {
				return nil, p.fail(off, Malformed, "", fmt.Sprintf("unexpected <%s> in <P>", t.Name.Local))
			}
			italic = true
		case xml.EndElement:
			if t.Name.Local == "P" {
				pa.text, pa.end = b.String(), off
				return pa, nil
			}
			italic = false
		default:
			return nil, p.fail(off, Malformed, "", fmt.Sprintf("unexpected %T in <P>", tok))
		}
	}
}

// rawOffsets maps each of the n decoded bytes of a character-data token to
// its offset in raw, the token's bytes in the XML: an entity's decoded bytes
// all map to its "&", and a decoded "\n" from "\r\n" to the "\r".
func rawOffsets(raw []byte, n int) []int {
	out := make([]int, 0, n)
	for j := 0; j < len(raw) && len(out) < n; {
		switch {
		case raw[j] == '&' && bytes.IndexByte(raw[j:], ';') > 0:
			semi := j + bytes.IndexByte(raw[j:], ';')
			for range entityLen(string(raw[j+1 : semi])) {
				out = append(out, j)
			}
			j = semi + 1
		case raw[j] == '\r' && j+1 < len(raw) && raw[j+1] == '\n':
			out = append(out, j)
			j += 2
		default:
			out = append(out, j)
			j++
		}
	}
	for len(out) < n { // not reached for well-formed XML; keeps the map total
		out = append(out, len(raw))
	}
	return out
}

// entityLen is the UTF-8 length of what an entity reference decodes to.
func entityLen(name string) int {
	if strings.HasPrefix(name, "#") {
		var r int64
		var err error
		if strings.HasPrefix(name, "#x") {
			r, err = strconv.ParseInt(name[2:], 16, 32)
		} else {
			r, err = strconv.ParseInt(name[1:], 10, 32)
		}
		if err != nil {
			return 1
		}
		return len(string(rune(r)))
	}
	return 1 // amp, lt, gt, quot, apos
}

// lead is one label found in a paragraph's lead, with its heading if any.
type lead struct {
	start   int // index in the paragraph's text where the label starts
	label   label
	heading string
}

// paragraph reads one <P> and places each label of its lead.
func (p *parser) paragraph() error {
	pa, err := p.readPara()
	if err != nil {
		return err
	}
	leads, err := p.leads(pa)
	if err != nil {
		return err
	}
	for i, l := range leads {
		end := len(pa.text)
		if i+1 < len(leads) {
			end = leads[i+1].start
		}
		n, err := p.place(l.label)
		if err != nil {
			return err
		}
		n.Heading, n.Text = l.heading, pa.text[l.start:end]
	}
	return nil
}

// leads finds the labels of a paragraph's lead. A label counts only where
// the lead puts one: at the start of the paragraph, right after a heading
// (past the dash or space that ends it), or right after another label. A
// heading is the italic text right after a label and one space. The lead
// ends at the first text that is none of these; any label after that is
// running text.
func (p *parser) leads(pa *para) ([]lead, error) {
	var out []lead
	for pos := 0; ; {
		l, end, ok := labelAt(pa, pos)
		if !ok && len(out) == 0 {
			return nil, p.fail(pa.offset(0), Unlabeled, "", "the paragraph does not start with a label")
		}
		if !ok {
			return out, nil
		}
		ld := lead{start: pos, label: l}
		pos = end
		if pos+1 < len(pa.text) && pa.text[pos] == ' ' && pa.italic[pos+1] {
			h := pos + 1
			for pos = h; pos < len(pa.text) && pa.italic[pos]; pos++ {
			}
			ld.heading = pa.text[h:pos]
			pos += sepLen(pa.text[pos:])
		}
		out = append(out, ld)
	}
}

// sepLen is the length of the dash or space that ends a heading at the
// start of s, or 0.
func sepLen(s string) int {
	for _, sep := range []string{"—", " "} {
		if strings.HasPrefix(s, sep) {
			return len(sep)
		}
	}
	return 0
}

// offset is the XML byte offset of the paragraph's text at index i.
func (pa *para) offset(i int) int {
	if i < len(pa.raw) {
		return pa.raw[i]
	}
	return pa.end
}

// label is a label as the text shows it, with its readings. A lowercase
// letter that is also a roman numeral ((i), (v), (x), ...) has both.
type label struct {
	text   string // e.g. "(C)", markup dropped
	offset int    // XML byte offset of its "("
	kind   Kind   // the one reading, or Roman when there are two
	value  int
	letter int    // the letter reading's value when there are two; else 0
	err    string // why the label has no reading, if it has none
}

// labelAt reads a label's shape, "(" token ")", at index i: the token is
// ASCII letters or digits, all italic or all not. ok reports the shape; a
// shape with no reading has err set.
func labelAt(pa *para, i int) (label, int, bool) {
	s := pa.text
	if i >= len(s) || s[i] != '(' {
		return label{}, 0, false
	}
	j := i + 1
	for j < len(s) && isAlnum(s[j]) {
		j++
	}
	if j == i+1 || j >= len(s) || s[j] != ')' {
		return label{}, 0, false
	}
	tok := s[i+1 : j]
	l := label{text: s[i : j+1], offset: pa.offset(i)}
	italic := pa.italic[i+1]
	for k := i + 1; k < j; k++ {
		if pa.italic[k] != italic {
			l.err = "part of the label is italic"
			return l, j + 1, true
		}
	}
	l.kind, l.value, l.letter, l.err = read(tok, italic)
	return l, j + 1, true
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// read gives a label token's kind and value.
func read(tok string, italic bool) (kind Kind, value, letter int, err string) {
	if n, ok := number(tok); ok {
		if italic {
			return Italic, n, 0, ""
		}
		return Number, n, 0, ""
	}
	if italic {
		return 0, 0, 0, "an italic label that is not a number"
	}
	if len(tok) == 1 && tok[0] >= 'A' && tok[0] <= 'Z' {
		return Capital, int(tok[0]-'A') + 1, 0, ""
	}
	if len(tok) == 1 && tok[0] >= 'a' && tok[0] <= 'z' {
		letter = int(tok[0]-'a') + 1
	}
	if r, ok := roman(tok); ok {
		return Roman, r, letter, ""
	}
	if letter > 0 {
		return Letter, letter, 0, ""
	}
	return 0, 0, 0, "not a letter, number, roman numeral or capital letter"
}

func number(tok string) (int, bool) {
	if tok == "" || tok[0] == '0' {
		return 0, false
	}
	n := 0
	for _, c := range []byte(tok) {
		if c < '0' || c > '9' || n > 1e6 {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

var romanDigits = []struct {
	value int
	text  string
}{{1000, "m"}, {900, "cm"}, {500, "d"}, {400, "cd"}, {100, "c"}, {90, "xc"},
	{50, "l"}, {40, "xl"}, {10, "x"}, {9, "ix"}, {5, "v"}, {4, "iv"}, {1, "i"}}

func toRoman(n int) string {
	var b strings.Builder
	for _, d := range romanDigits {
		for ; n >= d.value; n -= d.value {
			b.WriteString(d.text)
		}
	}
	return b.String()
}

// roman reads a lowercase roman numeral written the standard way.
func roman(tok string) (int, bool) {
	n, rest := 0, tok
	for _, d := range romanDigits {
		for strings.HasPrefix(rest, d.text) {
			n, rest = n+d.value, rest[len(d.text):]
		}
	}
	return n, n > 0 && rest == "" && toRoman(n) == tok
}

// labelText writes a value as a label of a kind, e.g. (Capital, 2) is "(B)".
func labelText(k Kind, v int) string {
	switch k {
	case Letter:
		return "(" + string(rune('a'+v-1)) + ")"
	case Roman:
		return "(" + toRoman(v) + ")"
	case Capital:
		return "(" + string(rune('A'+v-1)) + ")"
	default:
		return "(" + strconv.Itoa(v) + ")"
	}
}

func (p *parser) path() string {
	var b strings.Builder
	for _, l := range p.stack {
		b.WriteString(labelText(l.node.Kind, l.value))
	}
	return b.String()
}

// place puts a label on the stack of open levels and returns its new node.
// Depth comes from the kind: a label of kind k is a child of the open level
// k-1 and a sibling of the open level k, which closes the levels below it.
func (p *parser) place(l label) (*Node, error) {
	fail := func(r Reason, detail string) (*Node, error) {
		return nil, p.fail(l.offset, r, l.text, detail)
	}
	if l.err != "" {
		return fail(Unplaceable, l.err)
	}
	kind, value := l.kind, l.value
	// A lowercase label that reads both ways is a letter only when the last
	// letter-level sibling directly precedes it.
	if l.letter > 0 && len(p.stack) > 0 && p.stack[0].value == l.letter-1 {
		kind, value = Letter, l.letter
	}
	k := int(kind)
	switch {
	case k > len(p.stack):
		open := "no level is open"
		if len(p.stack) > 0 {
			open = "the open levels are " + p.path()
		}
		return fail(Unplaceable, fmt.Sprintf("%s labels sit under %s labels; %s", kind, Kind(k-1), open))
	case k == len(p.stack):
		if value != 1 {
			under := "at the top level"
			if k > 0 {
				under = "under " + p.path()
			}
			return fail(Skipped, fmt.Sprintf("the first %s label %s is %s", kind, under, labelText(kind, 1)))
		}
	default:
		last := p.stack[k].value
		p.stack = p.stack[:k+1]
		switch {
		case value == last+1:
		case value == last:
			return fail(Repeated, "it repeats "+p.path())
		case value < last:
			return fail(OutOfOrder, "it comes after "+p.path())
		default:
			return fail(Skipped, fmt.Sprintf("after %s the next %s label is %s", p.path(), kind, labelText(kind, last+1)))
		}
		p.stack = p.stack[:k]
	}

	n := &Node{Path: []string{strings.Trim(labelText(kind, value), "()")}, Kind: kind}
	if k == 0 {
		p.tree.Children = append(p.tree.Children, n)
	} else {
		parent := p.stack[k-1].node
		n.Path = append(slices.Clone(parent.Path), n.Path...)
		parent.Children = append(parent.Children, n)
	}
	p.stack = append(p.stack, level{node: n, value: value})
	return n, nil
}
