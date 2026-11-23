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

// FormError is a fault WellFormed finds in an XML document's form. Its
// Error is fetch's wording; Parse reports Msg as Malformed at Offset.
type FormError struct {
	Offset int    // the byte offset in the document of what is wrong
	Line   int    // the decoder's line once it has read that token; 0 if none
	Msg    string // what is wrong
	Err    error  // the decoder's error, when it refused the XML
}

func (e *FormError) Error() string {
	switch {
	case e.Err != nil:
		return "not well-formed XML: " + e.Err.Error()
	case e.Line == 0:
		return "not well-formed XML: " + e.Msg
	}
	return fmt.Sprintf("not well-formed XML: line %d: %s", e.Line, e.Msg)
}

func (e *FormError) Unwrap() error { return e.Err }

// bom is the UTF-8 byte order mark. It may open a document and is not part of
// it (XML 1.0, 4.3.3).
var bom = []byte("\xef\xbb\xbf")

// xmlSpace is XML's whitespace (XML 1.0, 2.3 S).
const xmlSpace = " \t\r\n"

// WellFormed returns nil when b is one well-formed XML document; otherwise a
// *FormError, or what root returned. It is the one check of an XML
// document's form: fetch (internal/ecfr) runs it on every body it stores, and
// Parse runs it before its own, stricter rules. It is not an XML 1.0
// validator: it checks what encoding/xml's strict parse checks (syntax,
// nesting, unknown entities, unquoted attributes, characters XML forbids)
// and, on top:
//   - exactly one root element; root, if not nil, is called with it and
//     its error returned as is;
//   - no element has the same attribute twice;
//   - outside the root, only comments, processing instructions and XML
//     whitespace (space, tab, CR, LF) as written: no other text, and no CDATA
//     or character reference even if it stands for whitespace;
//   - a PI target of "xml" in any case only as the XML declaration, at the
//     very start (after an optional UTF-8 BOM), and that by its grammar
//     (xmlDeclaration): version 1.0, encoding UTF-8 if any, standalone if any;
//   - no <!DOCTYPE> or other declaration anywhere.
func WellFormed(b []byte, root func(xml.StartElement) error) error {
	skip := 0
	if bytes.HasPrefix(b, bom) {
		skip = len(bom)
	}
	in := b[skip:]
	d := xml.NewDecoder(bytes.NewReader(in))
	d.Strict = true
	malformed := func(off int64, format string, a ...any) error {
		line, _ := d.InputPos()
		return &FormError{Offset: skip + int(off), Line: line, Msg: fmt.Sprintf(format, a...)}
	}
	depth, roots := 0, 0
	for first := true; ; first = false {
		start := d.InputOffset()
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			msg := err.Error()
			var se *xml.SyntaxError
			if errors.As(err, &se) {
				msg = se.Msg
			}
			return &FormError{Offset: skip + int(start), Msg: msg, Err: err}
		}
		switch t := tok.(type) {
		case xml.StartElement:
			for i, a := range t.Attr {
				for _, prev := range t.Attr[:i] {
					if a.Name == prev.Name {
						return malformed(start, "duplicate attribute %s on <%s>", qualified(a.Name), qualified(t.Name))
					}
				}
			}
			if depth == 0 {
				if roots++; roots > 1 {
					return malformed(start, "a second root element <%s>", t.Name.Local)
				}
				if root != nil {
					if err := root(t); err != nil {
						return err
					}
				}
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			// Judged as written, not as decoded: CDATA and character
			// references decode to text indistinguishable from whitespace.
			written := in[start:d.InputOffset()]
			if text := bytes.TrimLeft(written, xmlSpace); depth == 0 && len(text) > 0 {
				return malformed(start+int64(len(written)-len(text)), "text outside the root element: %s", snippet(written))
			}
		case xml.ProcInst:
			if !strings.EqualFold(t.Target, "xml") {
				break
			}
			if !first || t.Target != "xml" {
				return malformed(start, "<?%s %s?> is not an XML declaration, <?xml version=...?> at the very start", t.Target, t.Inst)
			}
			if err := xmlDeclaration(t.Inst); err != nil {
				return malformed(start, "XML declaration <?xml %s?>: %v", t.Inst, err)
			}
		case xml.Directive:
			return malformed(start, "<!%s>", t)
		}
	}
	if roots == 0 {
		return &FormError{Offset: len(b), Msg: "no root element"}
	}
	return nil
}

// xmlDeclaration returns nil when inst, what follows the target of an
// <?xml ...?> PI, is an XML declaration by its grammar (XML 1.0, 2.8): version
// 1.0, then optionally encoding UTF-8 (in any case), then optionally
// standalone yes or no, in that order, each name Eq (= with optional
// whitespace) and a single- or double-quoted value, separated by whitespace.
func xmlDeclaration(inst []byte) error {
	rest := string(inst)
	for i, part := range []struct {
		name string
		ok   func(string) bool
	}{
		{"version", func(v string) bool { return v == "1.0" }},
		{"encoding", func(v string) bool { return strings.EqualFold(v, "UTF-8") }},
		{"standalone", func(v string) bool { return v == "yes" || v == "no" }},
	} {
		// The parser has read the whitespace after the target, so version
		// needs none of its own; the others are each separated from the last.
		s := strings.TrimLeft(rest, xmlSpace)
		if (i > 0 && len(s) == len(rest)) || !strings.HasPrefix(s, part.name) {
			if i == 0 {
				return errors.New("no version")
			}
			continue // an optional part, absent
		}
		s = strings.TrimLeft(s[len(part.name):], xmlSpace)
		if !strings.HasPrefix(s, "=") {
			return fmt.Errorf("%s without =", part.name)
		}
		s = strings.TrimLeft(s[1:], xmlSpace)
		if s == "" || (s[0] != '"' && s[0] != '\'') {
			return fmt.Errorf("%s without a quoted value", part.name)
		}
		end := strings.IndexByte(s[1:], s[0])
		if end < 0 {
			return fmt.Errorf("%s's value is not closed", part.name)
		}
		if v := s[1 : 1+end]; !part.ok(v) {
			return fmt.Errorf("%s %q", part.name, v)
		}
		rest = s[2+end:]
	}
	if s := strings.Trim(rest, xmlSpace); s != "" {
		return fmt.Errorf("unexpected %q", s)
	}
	return nil
}

// qualified is n with its namespace, if any, before a colon.
func qualified(n xml.Name) string {
	if n.Space != "" {
		return n.Space + ":" + n.Local
	}
	return n.Local
}

// snippet quotes up to the first 300 bytes of b for a message.
func snippet(b []byte) string {
	const max = 300
	if len(b) > max {
		return strconv.Quote(string(b[:max])) + "..."
	}
	return strconv.Quote(string(b))
}
