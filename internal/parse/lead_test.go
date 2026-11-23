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
// space, and from a label straight to the next. Anywhere else, text ends
// it, and a label after that text is running text.
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
		{"(a) <I>Head</I>—(see (1)) text.", []string{"(a)"}},
		{"(a) <I>Head</I>—Text (1) text.", []string{"(a)"}},
		{"(a) <I>Head</I>—", []string{"(a)"}},
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
		{"label, dash, label", "(a)—(1) text.", "(1)",
			`unplaceable label (1): a label follows another only with nothing between them, or past its heading; here "—" separates them`},
		{"heading that is a label", "(a) <I>(1)</I> text.", "(1)",
			"unplaceable label (1): it begins the italic text after (a), where a heading goes"},
		{"heading that starts with a label", "(a) <I>(b) Head.</I> text.", "(b)",
			"unplaceable label (b): it begins the italic text after (a), where a heading goes"},
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
			"parse: line 5, column 4: unlabeled paragraph: the paragraph does not start with a label"},
		{"no closing parenthesis", []string{"(a) text.", "(b text."},
			"parse: line 5, column 4: unlabeled paragraph: the paragraph does not start with a label"},
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
