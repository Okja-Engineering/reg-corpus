// The parser's rules (R1a), each on a small synthetic section, for the cases
// 1024.41 itself does not exercise. The text is filler; only labels matter.
package parse_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// section builds a section whose i-th paragraph is on line 4+i, starting at
// column 4 (after "<P>").
func section(paras ...string) []byte {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DIV8 N=\"0.1\" TYPE=\"SECTION\">\n<HEAD>Head.</HEAD>\n")
	for _, p := range paras {
		b.WriteString("<P>" + p + "</P>\n")
	}
	b.WriteString("</DIV8>\n")
	return []byte(b.String())
}

func paths(t *testing.T, data []byte) []string {
	t.Helper()
	tree, err := parse.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, n := range walk(tree.Children) {
		out = append(out, pathString(n.Path))
	}
	return out
}

func wantError(t *testing.T, data []byte, reason parse.Reason, label string, line, column int) {
	t.Helper()
	_, err := parse.Parse(data)
	var pe *parse.Error
	if !errors.As(err, &pe) {
		t.Fatalf("Parse error = %v, want a *parse.Error", err)
	}
	if pe.Reason != reason || pe.Label != label || pe.Line != line || pe.Column != column {
		t.Errorf("error = %s %q at %d:%d (%v), want %s %q at %d:%d",
			pe.Reason, pe.Label, pe.Line, pe.Column, err, reason, label, line, column)
	}
}

// wantFail asserts that Parse fails with a *parse.Error whose message is
// want: its location, reason, label and detail.
func wantFail(t *testing.T, data []byte, want string) {
	t.Helper()
	tree, err := parse.Parse(data)
	var pe *parse.Error
	if !errors.As(err, &pe) {
		n := 0
		if tree != nil {
			n = len(walk(tree.Children))
		}
		t.Fatalf("Parse = %d nodes, error %v; want a *parse.Error\n%s", n, err, want)
	}
	if err.Error() != want {
		t.Errorf("error =\n%s\nwant\n%s", err, want)
	}
}

// col is the column of the first byte of mark in a paragraph p that
// section puts on its own line: p's byte index plus 4, after "<P>".
func col(p, mark string) int {
	i := strings.Index(p, mark)
	if i < 0 || strings.Count(p, mark) != 1 {
		panic("mark " + mark + " is not in " + p + " exactly once")
	}
	return 4 + i
}

// TestLabelPosition: a label counts only at the start of a <P>, or right
// after a heading (past the dash or space that ends it), or right after
// another such label. A label anywhere in running text is text.
func TestLabelPosition(t *testing.T) {
	data := section(
		"(a) <I>Head.</I> Text (b) text—(1) text. (2) text (A)(<I>1</I>) text.",
		"(b) Text—(1) text <I>Not a heading</I>—(2) text.",
		"(c) <I>Head</I>—(1)(i) <I>Head.</I> (A) text.",
	)
	want := []string{"(a)", "(b)", "(c)", "(c)(1)", "(c)(1)(i)", "(c)(1)(i)(A)"}
	if got := paths(t, data); !reflect.DeepEqual(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
}

// TestNewLevelStartsAtFirst: a level's first label is (a), (1), (i), (A) or
// italic (1); any other is a skip, also mid-paragraph.
func TestNewLevelStartsAtFirst(t *testing.T) {
	wantError(t, section("(a) text.", "(2) text."), parse.Skipped, "(2)", 5, 4)
	wantError(t, section("(a) <I>Head</I>—(1)(iii) text."), parse.Skipped, "(iii)", 4, 25)
	wantError(t, section("(b) text."), parse.Skipped, "(b)", 4, 4)
}

// TestLocationCountsRawBytes: a location is in the XML's bytes, so an
// entity before a label (here the heading dash written as &#8212;) counts
// at its written length, and the decoded text holds the character.
func TestLocationCountsRawBytes(t *testing.T) {
	wantError(t, section("(a) <I>Head</I>&#8212;(1)(iii) text."), parse.Skipped, "(iii)", 4, 29)
	tree, err := parse.Parse(section("(a) <I>H &amp; H</I>&#x2014;(1) text."))
	if err != nil {
		t.Fatal(err)
	}
	if a := tree.Children[0]; a.Heading != "H & H" || a.Text != "(a) H & H—" || len(a.Children) != 1 {
		t.Errorf("(a) = heading %q, text %q, %d children", a.Heading, a.Text, len(a.Children))
	}
}

// TestSiblingSequence: each label follows its last sibling by one.
func TestSiblingSequence(t *testing.T) {
	wantError(t, section("(a) text.", "(1) text.", "(3) text."), parse.Skipped, "(3)", 6, 4)
	wantError(t, section("(a) text.", "(1) text.", "(1) text."), parse.Repeated, "(1)", 6, 4)
	wantError(t, section("(a) text.", "(b) text.", "(a) text."), parse.OutOfOrder, "(a)", 6, 4)
}

// TestUnplaceable: a label whose kind is more than one level below the
// open levels, or that is no letter, number, roman or capital letter.
func TestUnplaceable(t *testing.T) {
	wantError(t, section("(1) text."), parse.Unplaceable, "(1)", 4, 4)
	wantError(t, section("(a) text.", "(A) text."), parse.Unplaceable, "(A)", 5, 4)
	wantError(t, section("(a) text.", "(1) text.", "(<I>1</I>) text."), parse.Unplaceable, "(1)", 6, 4)
	wantError(t, section("(a) text.", "(ab) text."), parse.Unplaceable, "(ab)", 5, 4)
}

// TestUnlabeledParagraph: every <P> starts with a label.
func TestUnlabeledParagraph(t *testing.T) {
	wantError(t, section("(a) text.", "Text."), parse.Unlabeled, "", 5, 4)
}

// TestLetterOrRoman: (i), (v) and (x) read as letters only right after (h),
// (u) and (w) at the letter level; otherwise roman. Here (h), (u) and (w)
// each have a child (1), so the roman reading fits too, and the next label,
// a letter, decides (TestLookahead has the rest).
func TestLetterOrRoman(t *testing.T) {
	var paras, want []string
	for c := 'a'; c <= 'z'; c++ {
		l := string(c)
		paras = append(paras, "("+l+") text.")
		want = append(want, "("+l+")")
		if l == "h" || l == "u" || l == "w" {
			paras = append(paras, "(1) text.")
			want = append(want, "("+l+")(1)")
		}
	}
	if got := paths(t, section(paras...)); !reflect.DeepEqual(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
	tree, err := parse.Parse(section(paras...))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range tree.Children {
		if n.Kind != parse.Letter {
			t.Errorf("%s kind = %v, want letter", pathString(n.Path), n.Kind)
		}
	}

	roman := section("(a) text.", "(1) text.", "(i) text.", "(ii) text.", "(iii) text.", "(iv) text.", "(v) text.",
		"(vi) text.", "(vii) text.", "(viii) text.", "(ix) text.", "(x) text.")
	got := paths(t, roman)
	if want := "(a)(1)(x)"; got[len(got)-1] != want {
		t.Errorf("last path = %s, want %s", got[len(got)-1], want)
	}
	// (v) after (h): not right after (u), so it reads only as a roman, and
	// a roman sits under a number, of which none is open.
	wantFail(t, section("(a) text.", "(b) text.", "(c) text.", "(d) text.", "(e) text.", "(f) text.", "(g) text.", "(h) text.", "(v) text."),
		"parse: line 12, column 4: unplaceable label (v): roman labels sit under number labels; the open levels are (h)")
}
