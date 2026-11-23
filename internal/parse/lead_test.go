// A paragraph's lead (ADR 0012): where a label counts, what a label looks
// like, and every way out of the lead. A label-shaped token that sits where
// the lead almost continues fails; it is never folded into running text.
package parse_test

import (
	"fmt"
	"reflect"
	"testing"
)

// TestLeadContinues: the lead goes on past a heading's one "—" or one
// space, and from a label straight to the next. Where the run after a label
// or heading holds no "(" and no misplaced heading, the first letter or
// digit ends the lead, and a label after that is running text.
func TestLeadContinues(t *testing.T) {
	for _, c := range []struct {
		para string
		want []string
	}{
		{"(a) <I>Head</I>—(1) text.", []string{"(a)", "(a)(1)"}},
		{"(a) <I>Head.</I> (1) text.", []string{"(a)", "(a)(1)"}},
		{"(a)(1) text.", []string{"(a)", "(a)(1)"}},
		{"(a) <I>Head</I>—(1)(i) <I>Head.</I> (A)(<I>1</I>) text.", []string{"(a)", "(a)(1)", "(a)(1)(i)", "(a)(1)(i)(A)", "(a)(1)(i)(A)(1)"}},
		{"(a) <I>Head.</I> Text (1) text.", []string{"(a)"}},
		{"(a) Text—(1) text.", []string{"(a)"}},
		{"(a) <I>Head</I>—Text (1) text.", []string{"(a)"}},
		{"(a) <I>Head</I>—", []string{"(a)"}},
		{"(a) 2(1) text.", []string{"(a)"}},
		{"(a) “Term” (1) text.", []string{"(a)"}},
		{"(a) [Reserved]", []string{"(a)"}},
		{"(a) <I>Loss mitigation (general).</I> Text (1) text.", []string{"(a)"}},
		{"(a) <I>Head (see (1)) more.</I> Text.", []string{"(a)"}},
		{"(a) <I>Self-help (see (1)).</I> Text.", []string{"(a)"}},
		{"(a) <I>Head—Sub (1) x</I> y", []string{"(a)"}},
		{"(a) <I>Head.</I> <I> Id.</I> text.", []string{"(a)"}},
		{"(a) <I>Pre‐ and post−closing review (general).</I> Text.", []string{"(a)"}},
	} {
		t.Run(c.para, func(t *testing.T) {
			if got := paths(t, section(c.para)); !reflect.DeepEqual(got, c.want) {
				t.Errorf("paths = %v, want %v", got, c.want)
			}
		})
	}
}

// TestLeadEnds: each way out of the lead with a label right after it fails
// at that label's "(".
func TestLeadEnds(t *testing.T) {
	const afterHeading = `a label follows a heading only past one "—" or one space; here `
	const inItalic = "it is in the italic text after (a), before any letter or digit, where a heading goes"
	const afterDash = "it follows a dash inside the heading of (a)"
	for _, c := range []struct {
		name, para, mark, want string
	}{
		{"heading, no separator", "(a) <I>Head</I>(1) text.", "(1)",
			"unplaceable label (1): " + afterHeading + "nothing separates them"},
		{"heading, dash inside the heading", "(a) <I>Head—</I>(1) text.", "(1)",
			"unplaceable label (1): " + afterHeading + "nothing separates them"},
		{"heading, en dash", "(a) <I>Head</I>–(1) text.", "(1)",
			"unplaceable label (1): " + afterHeading + `"–" separates them`},
		{"heading, hyphen", "(a) <I>Head</I>-(1) text.", "(1)",
			"unplaceable label (1): " + afterHeading + `"-" separates them`},
		{"heading, two spaces", "(a) <I>Head.</I>  (1) text.", "(1)",
			"unplaceable label (1): " + afterHeading + `"  " separates them`},
		{"heading, no-break space", "(a) <I>Head.</I>\u00a0(1) text.", "(1)",
			"unplaceable label (1): " + afterHeading + "\"\\u00a0\" separates them"},
		{"heading, period and space", "(a) <I>Head</I>. (1) text.", "(1)",
			"unplaceable label (1): " + afterHeading + `". " separates them`},
		{"heading, dash and space", "(a) <I>Head</I>— (1) text.", "(1)",
			"unplaceable label (1): " + afterHeading + `"— " separates them`},
		{"label, space, label", "(a) (1) text.", "(1)",
			`unplaceable label (1): a label follows another only with nothing between them, or past its heading; here " " separates them`},
		{"label, space, italic label", "(a) (<I>1</I>) text.", "(<I>",
			`unplaceable label (1): a label follows another only with nothing between them, or past its heading; here " " separates them`},
		{"label, dash, label", "(a)—(1) text.", "(1)",
			`unplaceable label (1): a label follows another only with nothing between them, or past its heading; here "—" separates them`},
		{"heading that is a label", "(a) <I>(1)</I> text.", "(1)",
			"unplaceable label (1): " + inItalic},
		{"heading that starts with a label", "(a) <I>(b) Head.</I> text.", "(b)",
			"unplaceable label (b): " + inItalic},
		{"heading that starts with a quoted label", "(a) <I>“(1) Head”</I> text.", "(1)",
			"unplaceable label (1): " + inItalic},
		{"heading that starts with a period and a label", "(a) <I>. (1) Head</I> text.", "(1)",
			"unplaceable label (1): " + inItalic},
		{"heading that starts with a word in parentheses", "(a) <I>(see) Head.</I> text.", "(see)",
			"unplaceable label (see): " + inItalic},
		{"label in the heading after a dash", "(a) <I>Head—(1) Sub.</I> Text.", "(1)",
			"unplaceable label (1): " + afterDash},
		{"label in the heading after a spaced en dash", "(a) <I>Head – (1) Sub.</I> Text.", "(1)",
			"unplaceable label (1): " + afterDash},
		{"label in the heading after a hyphen (U+2010)", "(a) <I>Head‐(1) Sub.</I> Text.", "(1)",
			"unplaceable label (1): " + afterDash},
		{"label in the heading after a figure dash (U+2012)", "(a) <I>Head‒(1) Sub.</I> Text.", "(1)",
			"unplaceable label (1): " + afterDash},
		{"label in the heading after a horizontal bar (U+2015)", "(a) <I>Head―(1) Sub.</I> Text.", "(1)",
			"unplaceable label (1): " + afterDash},
		{"label in the heading after a minus sign (U+2212)", "(a) <I>Head−(1) Sub.</I> Text.", "(1)",
			"unplaceable label (1): " + afterDash},
		{"label in the heading after a dash, its parenthesis italic", "(a) <I>Head—(</I>1) text.", "(</I>",
			"unplaceable label (1): " + afterDash},
		{"italic parenthesis chained", "(a)<I>(</I>1) text.", "(</I>",
			"unplaceable label (1): its parentheses are italic"},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantFail(t, section(c.para), fmt.Sprintf("parse: line 4, column %d: %s", col(c.para, c.mark), c.want))
		})
	}
}

// TestHeadingPosition: a heading is one <I> element after a label and
// exactly one space outside the <I>. Italic text after a label past anything
// else, or an <I> that begins with a space, fails at the heading's first
// character. The heading's start tag is the only <I> start tag from the
// label's end to the heading's end: any other, empty or not, before the
// heading or right after it, fails at its start tag.
func TestHeadingPosition(t *testing.T) {
	const want = "malformed: a heading follows its label past one space; here "
	const inside = "malformed: a heading follows its label past one space outside the <I>, not a space inside it"
	const second = "malformed: a heading is one <I> element; here a second <I> follows it"
	const before = "malformed: a heading is one <I> element; here another <I> comes before it"
	for _, c := range []struct {
		name, para, mark, want string
	}{
		{"no space", "(a)<I>Head</I>—(1) text.", "Head", want + "nothing separates them"},
		{"two spaces", "(a)  <I>Head</I>—(1) text.", "Head", want + `"  " separates them`},
		{"no-break space", "(a)\u00a0<I>Head</I>—(1) text.", "Head", want + "\"\\u00a0\" separates them"},
		{"a quotation mark", "(a) “<I>Term</I>” means.", "Term", want + `" “" separates them`},
		{"a dash", "(a)—<I>Head</I> text.", "Head", want + `"—" separates them`},
		{"the space inside the <I>", "(a)<I> Head</I>—(1) text.", " Head", inside},
		{"a second space inside the <I>", "(a) <I> Head</I>—(1) text.", " Head", inside},
		{"a second <I> holding a label", "(a) <I>Head</I><I>(1)</I> text.", "<I>(1)", second},
		{"a second <I> holding words", "(a) <I>Head.</I><I>Sub.</I> text.", "<I>Sub", second},
		{"an empty second <I>", "(a) <I>Head</I><I></I>—(1) text.", "<I></I>", second},
		{"an empty second <I>, then a third", "(a) <I>Head</I><I></I><I>Sub</I> text.", "<I></I>", second},
		{"an empty <I> right before the heading", "(a) <I></I><I>Head</I> text.", "<I></I>", before},
		{"an empty <I> before the space", "(a)<I></I> <I>Head</I> text.", "<I></I>", before},
		{"two empty <I> before the heading", "(a) <I></I><I></I><I>Head</I>—(1) text.", "<I></I><I></I>", before},
		{"a space and a label inside the <I>", "(a) <I> (1) Head.</I> text.", " (1)", inside},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantFail(t, section(c.para), fmt.Sprintf("parse: line 4, column %d: %s", col(c.para, c.mark), c.want))
		})
	}
}

// TestBrokenLabel: a "(" that starts no label, in a run (the text up to the
// first letter or digit after a label, after a heading, or after a dash
// inside a heading, italic or not), fails at the "(", as it does at the
// start of a <P>.
func TestBrokenLabel(t *testing.T) {
	const want = `unplaceable: a "(" here starts no label`
	for _, c := range []struct {
		name, para string
		at         int // the "(" that fails, by its index in para
	}{
		{"a second parenthesis", "(a)((1) text.", 3},
		{"two parentheses past a heading", "(a) <I>Head</I>— ((1) text.", 19},
		{"empty parentheses, then a label", "(a) <I>Head</I>—()(1) text.", 18},
		{"a space in the label", "(a)(1 ) text.", 3},
		{"a period in the label", "(a) <I>Head</I>—(1.) text.", 18},
		{"empty parentheses", "(a)() text.", 3},
		{"a letter outside ASCII", "(a)(é) text.", 3},
		{"a parenthesis in words past a heading", "(a) <I>Head</I>—(see (1)) text.", 18},
		{"a parenthesis in words past a label", "(a) (see below) text.", 4},
		{"a second parenthesis at a heading's start", "(a) <I>((1) Head</I> text.", 7},
		{"a period in a label at a heading's start", "(a) <I>(1.) Head</I> text.", 7},
		{"an italic parenthesis, then a broken label", "(a) <I>(</I>1.) text.", 7},
		{"an italic parenthesis, then a space and a label", "(a) <I>(</I> (1) text.", 7},
		{"a heading that begins with words in parentheses", "(a) <I>(see below) Head.</I> text.", 7},
		{"a period in a label after a dash in the heading", "(a) <I>Head—(1.) Sub.</I> text.", 14},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.para[c.at] != '(' {
				t.Fatalf("index %d of %q is not a \"(\"", c.at, c.para)
			}
			wantFail(t, section(c.para), fmt.Sprintf("parse: line 4, column %d: %s", 4+c.at, want))
		})
	}
}

// TestFirstFailure: where a lead holds more than one failure, Parse reports
// the one that comes first in the XML.
func TestFirstFailure(t *testing.T) {
	for _, c := range []struct {
		name, para, mark, want string
	}{
		{"a label after a dash, then a second <I>", "(a) <I>Head—(1) Sub</I><I>x</I> text.", "(1)",
			"unplaceable label (1): it follows a dash inside the heading of (a)"},
		{"an empty <I>, then a label at the heading's start", "(a) <I></I><I>(1) Head</I> text.", "<I></I>",
			"malformed: a heading is one <I> element; here another <I> comes before it"},
		{"no space before the heading, then a label in its run", "(a)<I>. (1) Head</I> text.", ". (1)",
			"malformed: a heading follows its label past one space; here nothing separates them"},
		{"a label in the heading's run, then no space after a dash", "(a) <I>“(1) Head—(2)</I> text.", "(1)",
			"unplaceable label (1): it is in the italic text after (a), before any letter or digit, where a heading goes"},
		{"no space before the heading, then a label in its run on the next line", "(a)<I>.\n(1) Head</I> text.", ".\n",
			"malformed: a heading follows its label past one space; here nothing separates them"},
		{"a period before a heading that starts with a label: both at one place, the label", "(a).<I>(1) Head</I> text.", "(1)",
			"unplaceable label (1): it is in the italic text after (a), before any letter or digit, where a heading goes"},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantFail(t, section(c.para), fmt.Sprintf("parse: line 4, column %d: %s", col(c.para, c.mark), c.want))
		})
	}
}

// TestChainedLabelIsFirstChild: a label after another in one lead, straight
// after it or past its heading, opens the level below it at that level's
// first label.
func TestChainedLabelIsFirstChild(t *testing.T) {
	for _, c := range []struct {
		name, para, mark, want string
	}{
		{"a sibling", "(a)(b) text.", "(b)",
			"unplaceable label (b): in a paragraph's lead it follows (a), so it opens the level below it"},
		{"a sibling past a heading", "(a) <I>See</I> (b) of this section.", "(b)",
			"unplaceable label (b): in a paragraph's lead it follows (a), so it opens the level below it"},
		{"a parent's sibling", "(a)(1) <I>Head</I>—(2) text.", "(2)",
			"unplaceable label (2): in a paragraph's lead it follows (a)(1), so it opens the level below it"},
		{"two levels down", "(a)(i) text.", "(i)",
			"unplaceable label (i): roman labels sit under number labels; the open levels are (a)"},
		{"the right level, not its first label", "(a) <I>Head</I>—(2) text.", "(2)",
			"skipped label (2): the first number label under (a) is (1)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantFail(t, section(c.para), fmt.Sprintf("parse: line 4, column %d: %s", col(c.para, c.mark), c.want))
		})
	}
}

// TestLabelShape: a label is "(" token ")" with nothing italic but, for the
// italic level, the whole number. Any other label-shaped token at a label
// position fails at its "(".
func TestLabelShape(t *testing.T) {
	const not = "not a letter, number, roman numeral or capital letter"
	for _, c := range []struct {
		name  string
		paras []string
		want  string
	}{
		{"italic parentheses", []string{"(a) text.", "<I>(1)</I> text."},
			"parse: line 5, column 7: unplaceable label (1): its parentheses are italic"},
		{"italic closing parenthesis", []string{"(a) text.", "(<I>1)</I> text."},
			"parse: line 5, column 4: unplaceable label (1): its parentheses are italic"},
		{"part italic", []string{"(a) text.", "(<I>1</I>0) text."},
			"parse: line 5, column 4: unplaceable label (10): part of the label is italic"},
		{"part italic, second part", []string{"(a) text.", "(1<I>0</I>) text."},
			"parse: line 5, column 4: unplaceable label (10): part of the label is italic"},
		{"italic letter", []string{"(a) text.", "(1) text.", "(i) text.", "(<I>A</I>) text."},
			"parse: line 7, column 4: unplaceable label (A): an italic label that is not a number"},
		{"two letters", []string{"(a) text.", "(ab) text."},
			"parse: line 5, column 4: unplaceable label (ab): " + not},
		{"leading zero", []string{"(a) text.", "(01) text."},
			"parse: line 5, column 4: unplaceable label (01): " + not},
		{"too large a number", []string{"(a) text.", "(99999999999999999999) text."},
			"parse: line 5, column 4: unplaceable label (99999999999999999999): " + not},
		{"roman not written the standard way", []string{"(a) text.", "(1) text.", "(i) text.", "(ii) text.", "(iii) text.", "(iiii) text."},
			"parse: line 9, column 4: unplaceable label (iiii): " + not},
		{"italic letter chained", []string{"(a)(<I>b</I>) text."},
			"parse: line 4, column 7: unplaceable label (b): an italic label that is not a number"},
		{"empty parentheses", []string{"(a) text.", "() text."},
			"parse: line 5, column 4: unplaceable: a \"(\" here starts no label"},
		{"no closing parenthesis", []string{"(a) text.", "(b text."},
			"parse: line 5, column 4: unplaceable: a \"(\" here starts no label"},
		{"italic opening parenthesis", []string{"(a) text.", "<I>(</I>1) text."},
			"parse: line 5, column 7: unplaceable label (1): its parentheses are italic"},
		{"two capitals", []string{"(a) text.", "(1) text.", "(i) text.", "(AB) text."},
			"parse: line 7, column 4: unplaceable label (AB): " + not},
		{"capital roman", []string{"(a) text.", "(1) text.", "(II) text."},
			"parse: line 6, column 4: unplaceable label (II): " + not},
		{"zero", []string{"(a) text.", "(0) text."},
			"parse: line 5, column 4: unplaceable label (0): " + not},
		{"space before the label", []string{"(a) text.", " (b) text."},
			"parse: line 5, column 4: unlabeled paragraph: the paragraph does not start with a label"},
		{"first label (i)", []string{"(i) text."},
			"parse: line 4, column 4: unplaceable label (i): roman labels sit under number labels; no level is open"},
		{"first label (b)", []string{"(b) text."},
			"parse: line 4, column 4: skipped label (b): the first letter label at the top level is (a)"},
	} {
		t.Run(c.name, func(t *testing.T) { wantFail(t, section(c.paras...), c.want) })
	}
}
