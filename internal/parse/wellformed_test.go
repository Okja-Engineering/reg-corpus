// WellFormed, the one check of an XML document's form (R1a; plan step 4):
// fetch reports its errors as they are and Parse reports Msg at Offset, so
// both are pinned here; Parse's own reports are in malformed_test.go.
package parse_test

import (
	"encoding/xml"
	"errors"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/parse"
)

func TestWellFormedErrors(t *testing.T) {
	for _, c := range []struct {
		name   string
		data   string
		want   string // the whole of Error(), fetch's wording
		offset int
	}{
		// No token to place it at, so no line: "line 0" would be wrong.
		{"no root element", "", "not well-formed XML: no root element", 0},
		{"no root element after a comment", "<!-- c -->\n", "not well-formed XML: no root element", 11},
		{"a duplicate attribute", "\n<a b=\"1\" b=\"2\"/>", "not well-formed XML: line 2: duplicate attribute b on <a>", 1},
		// The decoder's own error, whole, at the start of the token it was reading.
		{"a decoder error", "<a>", "not well-formed XML: XML syntax error on line 1: unexpected EOF", 3},
		// A declaration or PI it refuses is named as written, quoted, so a
		// line break in one stays on the message's line (worklist round 3,
		// §1): a document's, eCFR's or govinfo's, is upstream bytes.
		{"a DOCTYPE", "<!DOCTYPE a\n1024.41: wrote x>\n<a/>", `not well-formed XML: line 2: "<!DOCTYPE a\n1024.41: wrote x>"`, 0},
		{"a second XML declaration", "<a/><?xml version=\"1.0\"\n1024.41: wrote x?>",
			`not well-formed XML: line 2: "<?xml version=\"1.0\"\n1024.41: wrote x?>" is not an XML declaration, <?xml version=...?> at the very start`, 4},
		{"a PI named XML in another case", "<a><?XML x\n1024.41: wrote x?></a>",
			`not well-formed XML: line 2: "<?XML x\n1024.41: wrote x?>" is not an XML declaration, <?xml version=...?> at the very start`, 3},
		{"an XML declaration that breaks its grammar", "<?xml version=\"1.0\"\n1024.41: wrote x?><a/>",
			`not well-formed XML: line 2: XML declaration "<?xml version=\"1.0\"\n1024.41: wrote x?>": unexpected "1024.41: wrote x"`, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := parse.WellFormed([]byte(c.data), nil)
			var fe *parse.FormError
			if !errors.As(err, &fe) {
				t.Fatalf("WellFormed = %v; want a *parse.FormError", err)
			}
			if err.Error() != c.want || fe.Offset != c.offset {
				t.Errorf("WellFormed = %q at %d; want %q at %d", err.Error(), fe.Offset, c.want, c.offset)
			}
		})
	}
}

// A caller can still reach the decoder's error, as it could when fetch
// wrapped it with %w.
func TestWellFormedUnwrapsTheDecodersError(t *testing.T) {
	var se *xml.SyntaxError
	if err := parse.WellFormed([]byte("<a>"), nil); !errors.As(err, &se) || se.Msg != "unexpected EOF" {
		t.Errorf("WellFormed = %v; want it to wrap the decoder's *xml.SyntaxError", err)
	}
}

// Attributes are the same only when their namespace and local name both are
// (Namespaces in XML 1.0, 6.3).
func TestWellFormedAttributesInTwoNamespaces(t *testing.T) {
	if err := parse.WellFormed([]byte(`<a xmlns:u="U" xmlns:v="V" u:x="1" v:x="2"/>`), nil); err != nil {
		t.Errorf("WellFormed = %v; want nil", err)
	}
}

// root is called once, with the root element, and what it returns is
// WellFormed's error as it is: fetch's wrong-section error is not about form.
func TestWellFormedRoot(t *testing.T) {
	stop := errors.New("not the section asked for")
	var got []string
	err := parse.WellFormed([]byte(`<a><b/></a>`), func(s xml.StartElement) error {
		got = append(got, s.Name.Local)
		return stop
	})
	if err != stop || len(got) != 1 || got[0] != "a" {
		t.Errorf("WellFormed = %v, root called with %q; want %v, called with [a]", err, got, stop)
	}
}
