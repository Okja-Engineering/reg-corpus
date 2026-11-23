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
	wantError(t, section("(a) <I>Head</I>—(ii) text."), parse.Skipped, "(ii)", 4, 22)
	wantError(t, section("(b) text."), parse.Skipped, "(b)", 4, 4)
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

// TestLetterOrRoman: (i), (v) and (x) are letters only right after (h), (u)
// and (w) at the letter level; otherwise roman.
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
	// (v) after (h): not right after (u), so roman, and a roman level starts at (i).
	wantError(t, section("(a) text.", "(b) text.", "(c) text.", "(d) text.", "(e) text.", "(f) text.", "(g) text.", "(h) text.", "(v) text."),
		parse.Unplaceable, "(v)", 12, 4)
}

// TestMalformed: elements other than those 1024.41 holds fail, with their location.
func TestMalformed(t *testing.T) {
	wantError(t, section("(a) text <E T=\"03\">x</E>."), parse.Malformed, "", 4, 13)
	data := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DIV8 N=\"0.1\" TYPE=\"SECTION\">\n<HEAD>Head.</HEAD>\n<FP>(a) text.</FP>\n</DIV8>\n")
	wantError(t, data, parse.Malformed, "", 4, 1)
}
