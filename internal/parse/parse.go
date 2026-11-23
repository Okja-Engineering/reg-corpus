package parse

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Parse parses one eCFR section's XML into a Tree. It reads in three passes
// and fails with an *Error at the first failure of the first pass that
// fails: the XML's form (Malformed), each paragraph's lead (Unlabeled, and
// Unplaceable for a label's shape or position), then each label's place in
// document order (Skipped, Repeated, OutOfOrder, Unplaceable).
func Parse(data []byte) (*Tree, error) {
	p := &parser{data: data, dec: xml.NewDecoder(bytes.NewReader(data))}
	paras, err := p.section()
	if err != nil {
		return nil, err
	}
	var leads []lead
	for i, pa := range paras {
		ls, err := p.leads(pa, i+1)
		if err != nil {
			return nil, err
		}
		leads = append(leads, ls...)
	}
	if err := p.place(leads); err != nil {
		return nil, err
	}
	return p.tree, nil
}

type parser struct {
	data []byte
	dec  *xml.Decoder
	tree *Tree
	open levels // the open levels while labels are placed
}

// next returns the next token and the byte offset where it starts. Tokens
// are contiguous, so a token starts where the one before it ended. A token
// that is not well-formed fails at the start of the token being read. CDATA
// and namespaced elements fail: 1024.41's form holds neither.
func (p *parser) next() (xml.Token, int, error) {
	off := int(p.dec.InputOffset())
	tok, err := p.dec.Token()
	if err == io.EOF {
		return nil, off, err
	}
	if err != nil {
		msg := err.Error()
		var se *xml.SyntaxError
		if errors.As(err, &se) {
			msg = se.Msg
		}
		return nil, off, p.fail(off, Malformed, "", msg)
	}
	switch t := tok.(type) {
	case xml.CharData:
		if bytes.HasPrefix(p.data[off:], []byte("<![CDATA[")) {
			return nil, off, p.fail(off, Malformed, "", "unexpected CDATA section")
		}
	case xml.StartElement:
		if t.Name.Space != "" {
			return nil, off, p.fail(off, Malformed, "", fmt.Sprintf("element <%s> has namespace %q", t.Name.Local, t.Name.Space))
		}
	}
	return xml.CopyToken(tok), off, nil
}

func (p *parser) fail(off int, reason Reason, label, detail string) *Error {
	off = min(off, len(p.data))
	line := 1 + bytes.Count(p.data[:off], []byte("\n"))
	col := off - bytes.LastIndexByte(p.data[:off], '\n')
	return &Error{Line: line, Column: col, Reason: reason, Label: label, Detail: detail}
}

// space fails unless character data that starts at off is only XML
// whitespace (space, tab, CR, LF), located at its first other byte. A
// no-break space or a byte-order mark is text.
func (p *parser) space(cd xml.CharData, off int, detail string) error {
	i := 0
	for i < len(cd) && strings.IndexByte(" \t\r\n", cd[i]) >= 0 {
		i++
	}
	if i == len(cd) {
		return nil
	}
	raw := p.data[off:p.dec.InputOffset()]
	return p.fail(off+rawOffsets(raw, len(cd))[i], Malformed, "", detail)
}

// what names a token other than character data or an end tag in a message.
func what(tok xml.Token) string {
	switch t := tok.(type) {
	case xml.StartElement:
		return "<" + t.Name.Local + ">"
	case xml.Comment:
		return "comment"
	case xml.ProcInst:
		return "processing instruction"
	}
	return "directive"
}

// section reads the one DIV8: a HEAD, the P elements, then at most one CITA.
// It returns the P elements' content.
func (p *parser) section() ([]*para, error) {
	start, err := p.root()
	if err != nil {
		return nil, err
	}
	var paras []*para
	var head, cita bool
	for {
		tok, off, err := p.next()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.CharData:
			if err := p.space(t, off, "text directly in the section element"); err != nil {
				return nil, err
			}
		case xml.EndElement: // </DIV8>: the decoder has checked it matches
			switch {
			case !head:
				return nil, p.fail(start, Malformed, "", "the section element has no <HEAD>")
			case len(paras) == 0:
				return nil, p.fail(start, Malformed, "", "the section element has no <P>")
			}
			return paras, p.end()
		case xml.StartElement:
			switch {
			case t.Name.Local == "HEAD" && !head:
				head = true
				if p.tree.Heading, err = p.text("HEAD"); err != nil {
					return nil, err
				}
			case t.Name.Local == "P" && head && !cita:
				pa, err := p.readPara()
				if err != nil {
					return nil, err
				}
				paras = append(paras, pa)
			case t.Name.Local == "CITA" && head && !cita:
				cita = true
				if p.tree.Citation, err = p.text("CITA"); err != nil {
					return nil, err
				}
			default:
				return nil, p.fail(off, Malformed, "", "unexpected "+what(tok)+" in the section element")
			}
		default:
			return nil, p.fail(off, Malformed, "", "unexpected "+what(tok)+" in the section element")
		}
	}
}

// root reads up to the section element, a <DIV8 TYPE="SECTION"> with an N,
// and returns its offset.
func (p *parser) root() (int, error) {
	for {
		tok, off, err := p.next()
		if err == io.EOF {
			return 0, p.fail(off, Malformed, "", "no <DIV8> section element")
		} else if err != nil {
			return 0, err
		}
		switch t := tok.(type) {
		case xml.ProcInst:
		case xml.CharData:
			if err := p.space(t, off, "text outside the section element"); err != nil {
				return 0, err
			}
		case xml.StartElement:
			if t.Name.Local != "DIV8" || attr(t, "TYPE") != "SECTION" {
				return 0, p.fail(off, Malformed, "", "the root element is not <DIV8 TYPE=\"SECTION\">")
			}
			if p.tree = (&Tree{Section: attr(t, "N")}); p.tree.Section == "" {
				return 0, p.fail(off, Malformed, "", "the section element has no N attribute")
			}
			return off, nil
		default:
			return 0, p.fail(off, Malformed, "", "unexpected "+what(tok)+" before the section element")
		}
	}
}

// end checks that only XML whitespace follows the section element.
func (p *parser) end() error {
	for {
		tok, off, err := p.next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		cd, ok := tok.(xml.CharData)
		if !ok {
			return p.fail(off, Malformed, "", "content after the section element")
		}
		if err := p.space(cd, off, "content after the section element"); err != nil {
			return err
		}
	}
}

// attr is the value of an attribute with no namespace, or "".
func attr(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if a.Name.Space == "" && a.Name.Local == name {
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
			return "", p.fail(off, Malformed, "", fmt.Sprintf("unexpected %s in <%s>", what(tok), name))
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

// offset is the XML byte offset of the paragraph's text at index i.
func (pa *para) offset(i int) int {
	if i < len(pa.raw) {
		return pa.raw[i]
	}
	return pa.end
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
		in := "<P>"
		if italic {
			in = "<I>"
		}
		switch t := tok.(type) {
		case xml.CharData:
			b.Write(t)
			for _, o := range rawOffsets(p.data[off:p.dec.InputOffset()], len(t)) {
				pa.raw = append(pa.raw, off+o)
				pa.italic = append(pa.italic, italic)
			}
		case xml.StartElement:
			if t.Name.Local != "I" || italic {
				return nil, p.fail(off, Malformed, "", "unexpected "+what(tok)+" in "+in)
			}
			italic = true
		case xml.EndElement: // </I>, or </P>: the decoder has checked it matches
			if !italic {
				pa.text, pa.end = b.String(), off
				return pa, nil
			}
			italic = false
		default:
			return nil, p.fail(off, Malformed, "", "unexpected "+what(tok)+" in "+in)
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
	for len(out) < n { // not reached for well-formed XML without CDATA; keeps the map total
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
