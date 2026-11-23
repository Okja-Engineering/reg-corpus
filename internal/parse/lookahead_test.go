// (i), (v) and (x) (R1a; ADR 0012): a letter only when the last
// letter-level sibling directly precedes it, otherwise roman; and when both
// readings can be placed, the next label decides.
package parse_test

import (
	"reflect"
	"slices"
	"testing"
)

// letters is one paragraph per letter from (a) to last.
func letters(last byte) []string {
	var out []string
	for c := byte('a'); c <= last; c++ {
		out = append(out, "("+string(c)+") text.")
	}
	return out
}

// letterPaths is the paths of letters(last).
func letterPaths(last byte) []string {
	var out []string
	for c := byte('a'); c <= last; c++ {
		out = append(out, "("+string(c)+")")
	}
	return out
}

func romans(n int) []string {
	all := []string{"i", "ii", "iii", "iv", "v", "vi", "vii", "viii", "ix", "x"}
	var out []string
	for _, r := range all[:n] {
		out = append(out, "("+r+") text.")
	}
	return out
}

func join(parts ...[]string) []string { return slices.Concat(parts...) }

// TestLookahead: an (i), (v) or (x) that both readings can place is read
// the way that places the next label, in the same lead or the next <P>.
func TestLookahead(t *testing.T) {
	h, hp := letters('h'), letterPaths('h')
	for _, c := range []struct {
		name  string
		paras []string
		want  []string
	}{
		{"roman (i), then (ii)", join(h, []string{"(1) text.", "(i) text.", "(ii) text."}),
			join(hp, []string{"(h)(1)", "(h)(1)(i)", "(h)(1)(ii)"})},
		{"roman (i), then (A)", join(h, []string{"(1) text.", "(i) text.", "(A) text."}),
			join(hp, []string{"(h)(1)", "(h)(1)(i)", "(h)(1)(i)(A)"})},
		{"roman (i), then (2)", join(h, []string{"(1) text.", "(i) text.", "(2) text."}),
			join(hp, []string{"(h)(1)", "(h)(1)(i)", "(h)(2)"})},
		{"roman (i), then letter (i)", join(h, []string{"(1) text.", "(i) text.", "(i) text."}),
			join(hp, []string{"(h)(1)", "(h)(1)(i)", "(i)"})},
		{"roman (i), then (A) in its lead", join(h, []string{"(1) text.", "(i) <I>Head.</I> (A) text."}),
			join(hp, []string{"(h)(1)", "(h)(1)(i)", "(h)(1)(i)(A)"})},
		{"letter (i), then (j)", join(h, []string{"(1) text.", "(i) text.", "(j) text."}),
			join(hp, []string{"(h)(1)", "(i)", "(j)"})},
		{"letter (i), then (1)", join(h, []string{"(1) text.", "(i) text.", "(1) text."}),
			join(hp, []string{"(h)(1)", "(i)", "(i)(1)"})},
		{"letter (i), then (1) in its lead", join(h, []string{"(1) text.", "(i) <I>Head</I>—(1) text."}),
			join(hp, []string{"(h)(1)", "(i)", "(i)(1)"})},
		{"roman (v), then (vi)", join(letters('u'), []string{"(1) text."}, romans(6)),
			join(letterPaths('u'), []string{"(u)(1)", "(u)(1)(i)", "(u)(1)(ii)", "(u)(1)(iii)", "(u)(1)(iv)", "(u)(1)(v)", "(u)(1)(vi)"})},
		{"letter (v), then (w)", join(letters('u'), []string{"(1) text.", "(i) text.", "(v) text.", "(w) text."}),
			join(letterPaths('u'), []string{"(u)(1)", "(u)(1)(i)", "(v)", "(w)"})},
		{"roman (x), then (xi)", join(letters('w'), []string{"(1) text."}, romans(10), []string{"(xi) text."}),
			join(letterPaths('w'), []string{"(w)(1)", "(w)(1)(i)", "(w)(1)(ii)", "(w)(1)(iii)", "(w)(1)(iv)", "(w)(1)(v)", "(w)(1)(vi)", "(w)(1)(vii)", "(w)(1)(viii)", "(w)(1)(ix)", "(w)(1)(x)", "(w)(1)(xi)"})},
		{"letter (x), then (y)", join(letters('w'), []string{"(1) text."}, romans(9), []string{"(x) text.", "(y) text."}),
			join(letterPaths('w'), []string{"(w)(1)", "(w)(1)(i)", "(w)(1)(ii)", "(w)(1)(iii)", "(w)(1)(iv)", "(w)(1)(v)", "(w)(1)(vi)", "(w)(1)(vii)", "(w)(1)(viii)", "(w)(1)(ix)", "(x)", "(y)"})},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := paths(t, section(c.paras...)); !reflect.DeepEqual(got, c.want) {
				t.Errorf("paths =\n%v\nwant\n%v", got, c.want)
			}
		})
	}
}

// TestLookaheadFails: when both readings can be placed and the next label
// doesn't decide, the parse fails at the label, naming both readings.
func TestLookaheadFails(t *testing.T) {
	h := letters('h')
	for _, c := range []struct {
		name  string
		paras []string
		want  string
	}{
		{"(i) is the last label", join(h, []string{"(1) text.", "(i) text."}),
			"parse: line 13, column 4: unplaceable label (i): it could be (i) or (h)(1)(i), and no label follows to decide"},
		{"(v) is the last label", join(letters('u'), []string{"(1) text."}, romans(5)),
			"parse: line 30, column 4: unplaceable label (v): it could be (v) or (u)(1)(v), and no label follows to decide"},
		{"(x) is the last label", join(letters('w'), []string{"(1) text."}, romans(10)),
			"parse: line 37, column 4: unplaceable label (x): it could be (x) or (w)(1)(x), and no label follows to decide"},
		{"the next label fits neither", join(h, []string{"(1) text.", "(i) text.", "(k) text."}),
			"parse: line 13, column 4: unplaceable label (i): it could be (i) or (h)(1)(i), and the next label, (k), fits after neither"},
		{"the next label is (B)", join(h, []string{"(1) text.", "(i) text.", "(B) text."}),
			"parse: line 13, column 4: unplaceable label (i): it could be (i) or (h)(1)(i), and the next label, (B), fits after neither"},
		// A chained next label must open the level below: (j) does that after neither reading.
		{"the next label is chained", join(h, []string{"(1) text.", "(i)(j) text."}),
			"parse: line 13, column 4: unplaceable label (i): it could be (i) or (h)(1)(i), and the next label, (j), fits after neither"},
	} {
		t.Run(c.name, func(t *testing.T) { wantFail(t, section(c.paras...), c.want) })
	}
}

// TestOnlyIVXReadBothWays: (c), (d), (l) and (m) are roman numerals too, but
// only (i), (v) and (x) read both ways (R1a). The others are letters, and
// fail as letters.
func TestOnlyIVXReadBothWays(t *testing.T) {
	for _, l := range []string{"c", "d", "l", "m"} {
		t.Run(l, func(t *testing.T) {
			wantFail(t, section("(a) text.", "("+l+") text."),
				"parse: line 5, column 4: skipped label ("+l+"): after (a) the next letter label is (b)")
			wantFail(t, section("(a) text.", "(1) text.", "(i) text.", "("+l+") text."),
				"parse: line 7, column 4: skipped label ("+l+"): after (a) the next letter label is (b)")
		})
	}
}
