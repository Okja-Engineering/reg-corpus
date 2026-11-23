// The XML's form (R1a; plan step 4): anything a section of 1024.41's form
// does not hold fails as Malformed, located at the start of what is wrong.
// And locations count the XML's bytes, under each way text can be written.
package parse_test

import (
	"reflect"
	"strings"
	"testing"
)

const decl = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" // line 1

// doc is a document whose body starts on line 2.
func doc(body string) []byte { return []byte(decl + body) }

const (
	div8 = `<DIV8 N="0.1" TYPE="SECTION">` // 29 bytes
	head = "<HEAD>Head.</HEAD>"
	para = "<P>(a) text.</P>"
)

func TestMalformed(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		want string
	}{
		// Before the section element.
		{"no section element", doc(""),
			"parse: line 2, column 1: malformed: no <DIV8> section element"},
		{"text before the section", doc("x" + div8 + head + para + "</DIV8>"),
			"parse: line 2, column 1: malformed: text outside the section element"},
		{"no-break space before the section", doc("\u00a0" + div8 + head + para + "</DIV8>"),
			"parse: line 2, column 1: malformed: text outside the section element"},
		{"byte-order mark", []byte("\xef\xbb\xbf" + decl + div8 + head + para + "</DIV8>"),
			"parse: line 1, column 1: malformed: text outside the section element"},
		{"comment before the section", doc("<!-- c -->" + div8 + head + para + "</DIV8>"),
			"parse: line 2, column 1: malformed: unexpected comment before the section element"},
		{"directive before the section", doc("<!DOCTYPE DIV8>" + div8 + head + para + "</DIV8>"),
			"parse: line 2, column 1: malformed: unexpected directive before the section element"},
		{"second XML declaration", doc("<?xml version=\"1.0\"?>" + div8 + head + para + "</DIV8>"),
			"parse: line 2, column 1: malformed: unexpected processing instruction before the section element"},
		{"processing instruction before the section", doc("<?pi x?>" + div8 + head + para + "</DIV8>"),
			"parse: line 2, column 1: malformed: unexpected processing instruction before the section element"},
		{"processing instruction at the start", []byte("<?pi x?>" + div8 + head + para + "</DIV8>"),
			"parse: line 1, column 1: malformed: unexpected processing instruction before the section element"},
		{"XML declaration not at the start", []byte("\n" + decl + div8 + head + para + "</DIV8>"),
			"parse: line 2, column 1: malformed: unexpected processing instruction before the section element"},
		{"root not DIV8", doc(`<DIV9 N="0.1" TYPE="SECTION">` + head + para + "</DIV9>"),
			`parse: line 2, column 1: malformed: the root element is not <DIV8 TYPE="SECTION">`},
		{"DIV8 not a section", doc(`<DIV8 N="0.1" TYPE="PART">` + head + para + "</DIV8>"),
			`parse: line 2, column 1: malformed: the root element is not <DIV8 TYPE="SECTION">`},
		{"TYPE in a namespace", doc(`<DIV8 N="0.1" z:TYPE="SECTION">` + head + para + "</DIV8>"),
			`parse: line 2, column 1: malformed: the root element is not <DIV8 TYPE="SECTION">`},
		{"DIV8 in a namespace", doc(`<z:DIV8 N="0.1" TYPE="SECTION">` + head + para + "</z:DIV8>"),
			`parse: line 2, column 1: malformed: element <DIV8> has namespace "z"`},
		{"default namespace", doc(`<DIV8 xmlns="u" N="0.1" TYPE="SECTION">` + head + para + "</DIV8>"),
			`parse: line 2, column 1: malformed: element <DIV8> has namespace "u"`},
		{"no N", doc(`<DIV8 TYPE="SECTION">` + head + para + "</DIV8>"),
			"parse: line 2, column 1: malformed: the section element has no N attribute"},
		{"empty N", doc(`<DIV8 N="" TYPE="SECTION">` + head + para + "</DIV8>"),
			"parse: line 2, column 1: malformed: the section element has no N attribute"},

		// In the section element.
		{"text in the section", doc(div8 + "x\n" + head + para + "</DIV8>"),
			"parse: line 2, column 30: malformed: text directly in the section element"},
		{"no-break space in the section", doc(div8 + head + para + "\u00a0" + para + "</DIV8>"),
			"parse: line 2, column 64: malformed: text directly in the section element"},
		{"empty section", doc(div8 + "\n</DIV8>"),
			"parse: line 2, column 1: malformed: the section element has no <HEAD>"},
		{"self-closing section", doc(`<DIV8 N="0.1" TYPE="SECTION"/>`),
			"parse: line 2, column 1: malformed: the section element has no <HEAD>"},
		{"no P", doc(div8 + "\n" + head + "\n</DIV8>"),
			"parse: line 2, column 1: malformed: the section element has no <P>"},
		{"no P, a CITA", doc(div8 + "\n" + head + "\n<CITA>[Cite]</CITA>\n</DIV8>"),
			"parse: line 2, column 1: malformed: the section element has no <P>"},
		{"two HEADs", doc(div8 + "\n" + head + "\n" + head + "\n" + para + "\n</DIV8>"),
			"parse: line 4, column 1: malformed: unexpected <HEAD> in the section element"},
		{"P before HEAD", doc(div8 + "\n" + para + "\n" + head + "\n</DIV8>"),
			"parse: line 3, column 1: malformed: unexpected <P> in the section element"},
		{"P after CITA", doc(div8 + "\n" + head + "\n" + para + "\n<CITA>[Cite]</CITA>\n<P>(b) text.</P>\n</DIV8>"),
			"parse: line 6, column 1: malformed: unexpected <P> in the section element"},
		{"two CITAs", doc(div8 + "\n" + head + "\n" + para + "\n<CITA>[Cite]</CITA>\n<CITA>[Cite]</CITA>\n</DIV8>"),
			"parse: line 6, column 1: malformed: unexpected <CITA> in the section element"},
		{"another element", doc(div8 + "\n" + head + "\n<FP>(a) text.</FP>\n</DIV8>"),
			"parse: line 4, column 1: malformed: unexpected <FP> in the section element"},
		{"comment in the section", doc(div8 + "\n" + head + "\n<!-- c -->\n" + para + "\n</DIV8>"),
			"parse: line 4, column 1: malformed: unexpected comment in the section element"},
		{"processing instruction in the section", doc(div8 + "\n" + head + "\n<?pi x?>\n" + para + "\n</DIV8>"),
			"parse: line 4, column 1: malformed: unexpected processing instruction in the section element"},
		{"stray text after a character reference", doc(div8 + "&#32;x\n" + head + para + "</DIV8>"),
			"parse: line 2, column 35: malformed: text directly in the section element"},
		{"stray text after a CRLF", doc(div8 + "\r\n x\n" + head + para + "</DIV8>"),
			"parse: line 3, column 2: malformed: text directly in the section element"},

		// After the section element.
		{"text after the section", doc(div8 + head + para + "</DIV8>x"),
			"parse: line 2, column 71: malformed: content after the section element"},
		{"element after the section", doc(div8 + head + para + "</DIV8><P>(b) text.</P>"),
			"parse: line 2, column 71: malformed: content after the section element"},
		{"processing instruction after the section", doc(div8 + head + para + "</DIV8><?pi x?>"),
			"parse: line 2, column 71: malformed: content after the section element"},
		{"not well-formed after the section", doc(div8 + head + para + "</DIV8><"),
			"parse: line 2, column 71: malformed: unexpected EOF"},

		// In HEAD, CITA and P.
		{"element in HEAD", doc(div8 + "\n<HEAD>Head <E>x</E></HEAD>\n" + para + "\n</DIV8>"),
			"parse: line 3, column 12: malformed: unexpected <E> in <HEAD>"},
		{"comment in HEAD", doc(div8 + "\n<HEAD>Head<!-- c --></HEAD>\n" + para + "\n</DIV8>"),
			"parse: line 3, column 11: malformed: unexpected comment in <HEAD>"},
		{"element in CITA", doc(div8 + "\n" + head + "\n" + para + "\n<CITA>[<I>Cite</I>]</CITA>\n</DIV8>"),
			"parse: line 5, column 8: malformed: unexpected <I> in <CITA>"},
		{"element in P", section("(a) text <E T=\"03\">x</E>."),
			"parse: line 4, column 13: malformed: unexpected <E> in <P>"},
		{"I in I", section("(a) <I>Head <I>x</I></I> text."),
			"parse: line 4, column 16: malformed: unexpected <I> in <I>"},
		{"comment in P", section("(a) text<!-- c -->."),
			"parse: line 4, column 12: malformed: unexpected comment in <P>"},
		{"processing instruction in P", section("(a) text<?pi x?>."),
			"parse: line 4, column 12: malformed: unexpected processing instruction in <P>"},
		{"element in I", section("(a) <I>Head <E>x</E></I> text."),
			"parse: line 4, column 16: malformed: unexpected <E> in <I>"},
		{"I in a namespace", section("(a) <z:I>Head</z:I> text."),
			`parse: line 4, column 8: malformed: element <I> has namespace "z"`},
		{"empty I between chained labels", section("(a)<I></I>(1) text."),
			"parse: line 4, column 7: malformed: unexpected empty <I> in <P>"},
		{"empty I in running text", section("(a) Text <I></I> more."),
			"parse: line 4, column 13: malformed: unexpected empty <I> in <P>"},
		{"self-closing I", section("(a) Text <I/> more."),
			"parse: line 4, column 13: malformed: unexpected empty <I> in <P>"},
		{"empty I at the end of P", section("(a) Text.<I></I>"),
			"parse: line 4, column 13: malformed: unexpected empty <I> in <P>"},

		// Attributes: none on HEAD, P or I; none on CITA but TYPE.
		{"attribute on P", doc(div8 + "\n" + head + "\n" + `<P class="x">(a) text.</P>` + "\n</DIV8>"),
			`parse: line 4, column 1: malformed: unexpected attribute "class" on <P>`},
		{"attribute on I", section(`(a) <I k="v">Head</I> text.`),
			`parse: line 4, column 8: malformed: unexpected attribute "k" on <I>`},
		{"attribute on HEAD", doc(div8 + "\n<HEAD id=\"h\">Head.</HEAD>\n" + para + "\n</DIV8>"),
			`parse: line 3, column 1: malformed: unexpected attribute "id" on <HEAD>`},
		{"attribute on CITA other than TYPE", doc(div8 + "\n" + head + "\n" + para + "\n<CITA TYPE=\"N\" x=\"y\">[Cite]</CITA>\n</DIV8>"),
			`parse: line 5, column 1: malformed: unexpected attribute "x" on <CITA>`},
		{"TYPE in a namespace on CITA", doc(div8 + "\n" + head + "\n" + para + "\n<CITA q:TYPE=\"N\">[Cite]</CITA>\n</DIV8>"),
			`parse: line 5, column 1: malformed: unexpected attribute "q:TYPE" on <CITA>`},
		{"namespace declaration on P", doc(div8 + "\n" + head + "\n" + `<P xmlns:q="u">(a) text.</P>` + "\n</DIV8>"),
			`parse: line 4, column 1: malformed: unexpected attribute "xmlns:q" on <P>`},
		{"attribute holding a surrogate on P", doc(div8 + "\n" + head + "\n" + `<P a="&#xD800;">(a) text.</P>` + "\n</DIV8>"),
			`parse: line 4, column 1: malformed: unexpected attribute "a" on <P>`},
		{"surrogate, hex", section("(a) text &#xD800; text."),
			"parse: line 4, column 13: malformed: character reference &#xD800; is to a surrogate"},
		{"surrogate, decimal", section("(a) text &#57343; text."),
			"parse: line 4, column 13: malformed: character reference &#57343; is to a surrogate"},
		{"surrogate in an attribute", doc(`<DIV8 N="&#xDC00;" TYPE="SECTION">` + head + para + "</DIV8>"),
			"parse: line 2, column 10: malformed: character reference &#xDC00; is to a surrogate"},
		{"CDATA in P", section("(a) <![CDATA[text]]>."),
			"parse: line 4, column 8: malformed: unexpected CDATA section"},
		{"CDATA in HEAD", doc(div8 + "\n<HEAD><![CDATA[Head.]]></HEAD>\n" + para + "\n</DIV8>"),
			"parse: line 3, column 7: malformed: unexpected CDATA section"},
		{"CDATA between paragraphs", doc(div8 + "\n" + head + "\n" + para + "<![CDATA[ ]]>\n</DIV8>"),
			"parse: line 4, column 17: malformed: unexpected CDATA section"},

		// Not well-formed: located at the start of the token the decoder
		// was reading (for a truncated file, its end).
		{"end tag mismatch", section("(a) text.</Q>"),
			"parse: line 4, column 13: malformed: element <P> closed by </Q>"},
		{"unknown entity", section("(a) text &bogus; text."),
			"parse: line 4, column 4: malformed: invalid character entity &bogus;"},
		{"truncated", doc(div8 + "\n" + head + "\n<P>(a) text."),
			"parse: line 4, column 13: malformed: unexpected EOF"},
	} {
		t.Run(c.name, func(t *testing.T) { wantFail(t, c.data, c.want) })
	}
}

// TestWhitespaceBetweenElements: space, tab, CR and LF between elements, and
// before and after the section, are not text.
func TestWhitespaceBetweenElements(t *testing.T) {
	data := []byte(decl + " \t\r\n" + div8 + "\t" + head + " \r\n\t" + para + "\t</DIV8>\t \r\n")
	if got := paths(t, data); !reflect.DeepEqual(got, []string{"(a)"}) {
		t.Errorf("paths = %v", got)
	}
}

// TestLocations: a location counts the XML's bytes, so an entity counts at
// its written length and a CRLF at two bytes, while the decoded text holds
// the character; a lone CR ends a line, as a LF does; an empty <P> is
// located at its start tag.
func TestLocations(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		want string
	}{
		{"decimal entity", section("(a) <I>Head</I>&#8212;(1)(iii) text."),
			"parse: line 4, column 29: skipped label (iii): the first roman label under (a)(1) is (i)"},
		{"hex entity", section("(a) <I>Head</I>&#x2014;(1)(iii) text."),
			"parse: line 4, column 30: skipped label (iii): the first roman label under (a)(1) is (i)"},
		{"named entity", section("(a) <I>A &amp; B</I>—(1)(iii) text."),
			"parse: line 4, column 30: skipped label (iii): the first roman label under (a)(1) is (i)"},
		{"CRLF in a paragraph", section("(a) <I>Head\r\nHead</I>—(1)(iii) text."),
			"parse: line 5, column 15: skipped label (iii): the first roman label under (a)(1) is (i)"},
		{"CRLF before a label", section("(a) <I>Head</I>\r\n(1) text."),
			"parse: line 5, column 1: unplaceable label (1): a label follows a heading only past one \"—\" or one space; here \"\\n\" separates them"},
		{"CRLF line ends", []byte(strings.ReplaceAll(string(section("(a) text.", "(2) text.")), "\n", "\r\n")),
			"parse: line 5, column 4: skipped label (2): the first number label under (a) is (1)"},
		{"empty paragraph", section("(a) text.", ""),
			"parse: line 5, column 1: unlabeled paragraph: the paragraph does not start with a label"},
		{"self-closing paragraph", doc(div8 + "\n" + head + "\n" + para + "\n<P/>\n</DIV8>"),
			"parse: line 5, column 1: unlabeled paragraph: the paragraph does not start with a label"},
		{"CR line ends", []byte(strings.ReplaceAll(string(section("(a) text.", "(2) text.")), "\n", "\r")),
			"parse: line 5, column 4: skipped label (2): the first number label under (a) is (1)"},
		{"truncated after a lone CR", doc(div8 + "\r" + head + "\r<P>(a) text.\r"),
			"parse: line 5, column 1: malformed: unexpected EOF"},
		{"CR before a label", section("(a) <I>Head</I>\r(1) text."),
			"parse: line 5, column 1: unplaceable label (1): a label follows a heading only past one \"—\" or one space; here \"\\n\" separates them"},
		{"named entity before a label", section("(a) <I>A</I>—&amp;(1) text."),
			"parse: line 4, column 24: unplaceable label (1): a label follows a heading only past one \"—\" or one space; here \"—&\" separates them"},
	} {
		t.Run(c.name, func(t *testing.T) { wantFail(t, c.data, c.want) })
	}
}
