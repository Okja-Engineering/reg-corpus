// Anchor IDs, the operative-anchor test (ADR 0013, F1) and the multi-anchor
// join (ADR 0013, F2), on 1024.41's stored raw XML and on small sections.
package anchors_test

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/parse"
)

func TestID(t *testing.T) {
	for _, tc := range []struct {
		section string
		path    []string
		want    string
	}{
		{"1024.41", []string{"b", "2", "i", "B"}, "1024-41-b-2-i-B"},
		{"1024.41", []string{"c", "2", "v", "A", "1"}, "1024-41-c-2-v-A-1"},
		{"1024.41", []string{"a"}, "1024-41-a"},
		{"1024.30", []string{"c", "2"}, "1024-30-c-2"},
	} {
		if got := anchors.ID(tc.section, tc.path); got != tc.want {
			t.Errorf("ID(%q, %q) = %q, want %q", tc.section, tc.path, got, tc.want)
		}
	}
}

// operative41 is every operative anchor of 1024.41, in document order: 75 of
// its 91 nodes (ADR 0013, F1).
var operative41 = []string{
	"1024-41-a",
	"1024-41-b-1", "1024-41-b-2-i", "1024-41-b-2-i-A", "1024-41-b-2-i-B", "1024-41-b-2-ii", "1024-41-b-3",
	"1024-41-c-1", "1024-41-c-1-i", "1024-41-c-1-ii",
	"1024-41-c-2-i", "1024-41-c-2-ii", "1024-41-c-2-iii", "1024-41-c-2-iv",
	"1024-41-c-2-v-A", "1024-41-c-2-v-A-1", "1024-41-c-2-v-A-2", "1024-41-c-2-v-A-3", "1024-41-c-2-v-B",
	"1024-41-c-3-i", "1024-41-c-3-i-A", "1024-41-c-3-i-B", "1024-41-c-3-i-C", "1024-41-c-3-i-D",
	"1024-41-c-3-i-D-1", "1024-41-c-3-i-D-2", "1024-41-c-3-i-E", "1024-41-c-3-i-F",
	"1024-41-c-3-ii", "1024-41-c-3-ii-A", "1024-41-c-3-ii-B", "1024-41-c-3-ii-C",
	"1024-41-c-4-i", "1024-41-c-4-ii-A-1", "1024-41-c-4-ii-A-2",
	"1024-41-c-4-ii-B", "1024-41-c-4-ii-B-1", "1024-41-c-4-ii-B-2", "1024-41-c-4-ii-B-3", "1024-41-c-4-ii-B-4",
	"1024-41-c-4-ii-C",
	"1024-41-d",
	"1024-41-e-1", "1024-41-e-2-i", "1024-41-e-2-ii", "1024-41-e-2-iii",
	"1024-41-f-1", "1024-41-f-1-i", "1024-41-f-1-ii", "1024-41-f-1-iii",
	"1024-41-f-2", "1024-41-f-2-i", "1024-41-f-2-ii", "1024-41-f-2-iii",
	"1024-41-g", "1024-41-g-1", "1024-41-g-2", "1024-41-g-3",
	"1024-41-h-1", "1024-41-h-2", "1024-41-h-3", "1024-41-h-4",
	"1024-41-i", "1024-41-j",
	"1024-41-k-1-i", "1024-41-k-1-ii", "1024-41-k-2-i", "1024-41-k-2-ii", "1024-41-k-2-ii-A", "1024-41-k-2-ii-B",
	"1024-41-k-3", "1024-41-k-4", "1024-41-k-4-i", "1024-41-k-4-ii", "1024-41-k-5",
}

// notOperative41 is the rest: 15 lead-ins whose own text past the label and
// heading is only the "—" or space that ends the heading, and (c)(4)(ii)(A),
// whose own text is its label alone.
var notOperative41 = []string{
	"1024-41-b", "1024-41-b-2", "1024-41-c", "1024-41-c-2", "1024-41-c-2-v", "1024-41-c-3",
	"1024-41-c-4", "1024-41-c-4-ii", "1024-41-c-4-ii-A", "1024-41-e", "1024-41-e-2", "1024-41-f",
	"1024-41-h", "1024-41-k", "1024-41-k-1", "1024-41-k-2",
}

func TestOperativeAnchors1024_41(t *testing.T) {
	if len(operative41) != 75 || len(notOperative41) != 16 {
		t.Fatalf("the lists hold %d and %d anchors, want 75 and 16", len(operative41), len(notOperative41))
	}
	tree := parseRepo(t, raw41)
	var op, not []string
	for _, n := range walk(tree.Children) {
		ok, err := anchors.Operative(n)
		if err != nil {
			t.Fatal(err)
		}
		id := anchors.ID(tree.Section, n.Path)
		if ok {
			op = append(op, id)
		} else {
			not = append(not, id)
		}
	}
	if !reflect.DeepEqual(op, operative41) {
		t.Errorf("operative (%d) =\n%s\nwant (%d)\n%s", len(op), strings.Join(op, "\n"), len(operative41), strings.Join(operative41, "\n"))
	}
	if !reflect.DeepEqual(not, notOperative41) {
		t.Errorf("not operative (%d) =\n%s\nwant (%d)\n%s", len(not), strings.Join(not, "\n"), len(notOperative41), strings.Join(notOperative41, "\n"))
	}
}

// TestOperativeRule: operative means own text minus the label, the one
// space before a heading, the heading, and the one dash or space that ends
// the heading is non-empty, the rest read as normalized text: one "-" or
// one space there is that separator. Nothing else is taken away.
func TestOperativeRule(t *testing.T) {
	for _, tc := range []struct {
		name, para string
		path       string
		want       bool
	}{
		{"a heading ending in a dash, then a child", "(a) <I>Head</I>—(1) Text.", "a", false},
		{"the child", "(a) <I>Head</I>—(1) Text.", "a-1", true},
		{"a heading ending in a space, then a child", "(a) <I>Head.</I> (1) Text.", "a", false},
		{"a label, then a child at once", "(a)(1) Text.", "a", false},
		{"a heading alone", "(a) <I>Head.</I>", "a", false},
		{"a heading, then its one separating space alone", "(a) <I>Head.</I> ", "a", false},
		{"a heading, then its dash alone", "(a) <I>Head</I>—", "a", false},
		{"a heading, then an en dash alone: normalized, one -", "(a) <I>Head</I>–", "a", false},
		{"a heading, then a no-break space alone: normalized, one space", "(a) <I>Head.</I>\u00A0", "a", false},
		{"a heading, then two spaces: normalized, one space", "(a) <I>Head.</I>  ", "a", false},
		{"a heading, then text", "(a) <I>Head.</I> Text.", "a", true},
		{"a heading, then a dash and text", "(a) <I>Head</I>— text.", "a", true},
		{"a heading, then two dashes: only one is the separator", "(a) <I>Head</I>——", "a", true},
		{"a heading, then a dash and a space: only the dash is the separator", "(a) <I>Head</I>— ", "a", true},
		{"a heading, then a combining mark: text, and no error", "(a) <I>Note</I>\u0301 text.", "a", true},
		{"a heading, then a combining mark alone: text, not a separator", "(a) <I>Note</I>\u0301", "a", true},
		// A heading that ends in White_Space: normalized as one string with
		// what follows, that run is one space, the heading's own, and one
		// dash after it is still the one separator.
		{"a heading ending in a space, then its dash", "(a) <I>Head </I>\u2014", "a", false},
		{"a heading ending in a space, then a space and its dash", "(a) <I>Head </I> \u2014", "a", false},
		{"a heading ending in a space, then two spaces and its dash", "(a) <I>Head </I>  \u2014", "a", false},
		{"a heading ending in a space, then a no-break space and its dash", "(a) <I>Head </I>\u00a0\u2014", "a", false},
		{"a heading ending in a space, then a space and a hyphen", "(a) <I>Head </I> -", "a", false},
		{"a heading ending in a no-break space, then a space and its dash", "(a) <I>Head\u00a0</I> \u2014", "a", false},
		{"a heading ending in a space, then a space alone", "(a) <I>Head </I> ", "a", false},
		{"a heading ending in a space, then text", "(a) <I>Head </I> Text.", "a", true},
		{"a heading ending in a space, then two dashes", "(a) <I>Head </I> \u2014\u2014", "a", true},
		{"no heading, text", "(a) Text.", "a", true},
		{"no heading, a space alone: only a heading's separator is taken away", "(a) ", "a", true},
		{"an italic label alone", "(a)(1)(i)(A)(<I>1</I>) Text.", "a-1-i-A", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := byPath(t, parseXML(t, section("", tc.para)))[tc.path]
			if n == nil {
				t.Fatalf("no node %s", tc.path)
			}
			got, err := anchors.Operative(n)
			if err != nil || got != tc.want {
				t.Errorf("Operative(%q) = %v, %v; want %v", n.Text, got, err, tc.want)
			}
		})
	}
}

// TestOperativeFollowsTheHash: the verdict is a function of what the hash
// covers. Separators that normalize alike at the end of a <P> (an em or en
// dash; a space, a no-break space or two spaces) give the same hash, and
// every one of them the same verdict: a heading-only lead-in, not
// operative.
func TestOperativeFollowsTheHash(t *testing.T) {
	for _, group := range [][]string{{"—", "–"}, {" ", "\u00A0", "  ", "\u2003"}} {
		hash := ""
		for _, sep := range group {
			n := byPath(t, parseXML(t, section("", "(a) <I>Head</I>"+sep)))["a"]
			h, err := anchors.Hash(n.Text)
			if err != nil {
				t.Fatal(err)
			}
			if hash == "" {
				hash = h
			} else if h != hash {
				t.Errorf("separator %+q: hash %s, want %s, as %+q gives", sep, h, hash, group[0])
			}
			if op, err := anchors.Operative(n); err != nil || op {
				t.Errorf("separator %+q: Operative = %v, %v; want false, as for %+q", sep, op, err, group[0])
			}
		}
	}
}

// TestOperativeIsAFunctionOfTheNormalizedText (ADR 0013): two nodes with the
// same path, the same heading and the same normalized own text get the same
// verdict, over every own text of up to two characters past the heading,
// and a sample of longer ones, drawn from all 25 White_Space characters,
// the dashes, a letter and characters NFC composes with a heading's last
// letter; and under headings that end in each White_Space character. A
// node's text is in the form the parser makes: its label, then one space
// and its heading if it has one.
func TestOperativeIsAFunctionOfTheNormalizedText(t *testing.T) {
	var white []string
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.Is(unicode.White_Space, r) {
			white = append(white, string(r))
		}
	}
	if len(white) != 25 {
		t.Fatalf("%d White_Space characters; want 25", len(white))
	}
	// The dashes; a letter; U+0301 and U+0327, which NFC composes with an e
	// before them; U+00E9, the composed e with an acute.
	alphabet := append([]string{"-", "\U00002013", "\U00002014", "x", "e", "\U00000301", "\U000000E9", "\U00000327"}, white...)
	headings := []string{"", "Head", "Note", "Not\U000000E9", "Note\U00000301"}
	for _, w := range white {
		headings = append(headings, "Head"+w, w+"Head")
	}
	rests := []string{""}
	for _, a := range alphabet {
		rests = append(rests, a)
		for _, b := range alphabet {
			rests = append(rests, a+b)
		}
	}
	rng := rand.New(rand.NewPCG(13, 41))
	for range 20000 {
		var b strings.Builder
		for range 3 + rng.IntN(4) {
			b.WriteString(alphabet[rng.IntN(len(alphabet))])
		}
		rests = append(rests, b.String())
	}
	type key struct{ heading, norm string }
	verdicts := map[key]bool{}
	example := map[key]string{}
	for _, h := range headings {
		for _, rest := range rests {
			n := &parse.Node{Path: []string{"a"}, Heading: h, Text: "(a)" + rest}
			if h != "" {
				n.Text = "(a) " + h + rest
			}
			op, err := anchors.Operative(n)
			if err != nil {
				t.Fatalf("Operative(%+q, heading %+q): %v", n.Text, h, err)
			}
			k := key{h, normalize(t, n.Text)}
			if v, ok := verdicts[k]; !ok {
				verdicts[k], example[k] = op, n.Text
			} else if v != op {
				t.Fatalf("heading %+q: Operative(%+q) = %v, but Operative(%+q) = %v; both normalize to %+q", h, n.Text, op, example[k], v, k.norm)
			}
		}
	}
}

// TestOperativeRefusesANodeItsTextDoesNotFit: a node whose own text does not
// start with its label, and its heading if it has one, is not a node the
// parser makes; Operative says so rather than guess.
func TestOperativeRefusesANodeItsTextDoesNotFit(t *testing.T) {
	for _, n := range []*parse.Node{
		{Path: []string{"a"}, Text: "(b) Text."},
		{Path: []string{"a"}, Text: "a) Text."},
		{Path: []string{"a"}, Heading: "Head.", Text: "(a) Other. Text."},
		{Path: []string{"a"}, Heading: "Head.", Text: "(a)Head. Text."},
		{Path: nil, Text: "Text."},
		{Path: []string{"a"}, Text: "(a) \xff"},
	} {
		if got, err := anchors.Operative(n); err == nil {
			t.Errorf("Operative(%q, path %q, heading %q) = %v, nil; want an error", n.Text, n.Path, n.Heading, got)
		}
	}
}

// nodes41 is the nodes of 1024.41 at paths, and its document.
func nodes41(t *testing.T) (*anchors.Document, map[string]*parse.Node) {
	t.Helper()
	tree := parseRepo(t, raw41)
	return anchors.NewDocument(tree), byPath(t, tree)
}

func cite(n map[string]*parse.Node, paths ...string) []*parse.Node {
	var out []*parse.Node
	for _, p := range paths {
		out = append(out, n[p])
	}
	return out
}

func reversed(nodes []*parse.Node) []*parse.Node {
	out := make([]*parse.Node, len(nodes))
	for i, x := range nodes {
		out[len(nodes)-1-i] = x
	}
	return out
}

// TestJoinGoldens: a run of anchors consecutive in the document joins with
// nothing between two in one <P> and one space between <P>s (ADR 0013, F2),
// so a cite across a heading separator or a run of labels reads as the
// document does. The windows are the normalized text around each join.
func TestJoinGoldens(t *testing.T) {
	doc, n := nodes41(t)
	for _, tc := range []struct {
		name   string
		cite   []string
		want   string // the raw join, from own texts and the separators J2 puts between them
		window string // normalized, around the join
	}{
		{"(b) + (b)(1), one <P>, across a heading dash", []string{"b", "b-1"},
			n["b"].Text + n["b-1"].Text, "Receipt of a loss mitigation application-(1) Complete loss mitigation application."},
		{"(c)(4)(ii) + (A) + (A)(1), one <P>, a run of labels", []string{"c-4-ii", "c-4-ii-A", "c-4-ii-A-1"},
			n["c-4-ii"].Text + n["c-4-ii-A"].Text + n["c-4-ii-A-1"].Text, "(ii) Effect in case of delay. (A)(1) Except as provided"},
		{"(c)(2)(v) + (A), one <P>, across a heading's space", []string{"c-2-v", "c-2-v-A"},
			n["c-2-v"].Text + n["c-2-v-A"].Text, "loss mitigation options. (A) Notwithstanding paragraph"},
		{"(b)(2)(i) + (b)(2)(i)(A), two <P>s", []string{"b-2-i", "b-2-i-A"},
			n["b-2-i"].Text + " " + n["b-2-i-A"].Text, "a servicer shall: (A) Promptly upon receipt"},
		{"(k) + (k)(1) + (k)(1)(i) + (k)(1)(ii), one <P> then another", []string{"k", "k-1", "k-1-i", "k-1-ii"},
			n["k"].Text + n["k-1"].Text + n["k-1-i"].Text + " " + n["k-1-ii"].Text, "Servicing transfers-(1) In general-(i) Timing of compliance."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodes := cite(n, tc.cite...)
			for _, order := range [][]*parse.Node{nodes, reversed(nodes)} {
				got, err := doc.Join(order)
				if err != nil {
					t.Fatal(err)
				}
				if len(got) != 1 || got[0] != tc.want {
					t.Fatalf("Join = %q\nwant one run, %q", got, tc.want)
				}
				if !strings.Contains(normalize(t, got[0]), tc.window) {
					t.Errorf("normalized join lacks %q:\n%q", tc.window, normalize(t, got[0]))
				}
			}
		})
	}
}

// TestNoQuoteCrossesANonConsecutiveJoin (spec "Multi-anchor cite"): anchors
// that are not consecutive in the document are separate runs, each joined
// alone, so text that runs from one to the other, which the document never
// reads, is in no string Join returns, whichever order the cite gives.
func TestNoQuoteCrossesANonConsecutiveJoin(t *testing.T) {
	doc, n := nodes41(t)
	for _, tc := range []struct {
		name   string
		cite   []string
		window string // normalized, across the join the document never reads
	}{
		{"(k) + (k)(1)(i), one <P>, (k)(1) between", []string{"k", "k-1-i"}, "Servicing transfers-(i) Timing"},
		{"(c)(4)(ii) + (A)(1), one <P>, (A) between", []string{"c-4-ii", "c-4-ii-A-1"}, "delay. (1) Except"},
		{"(a) + (d), far apart", []string{"a", "d"}, "applicable law. (d) Denial"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodes := cite(n, tc.cite...)
			for _, order := range [][]*parse.Node{nodes, reversed(nodes)} {
				got, err := doc.Join(order)
				if err != nil {
					t.Fatal(err)
				}
				want := []string{nodes[0].Text, nodes[1].Text}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Join = %q, want each anchor alone, in document order: %q", got, want)
				}
				for _, s := range got {
					if strings.Contains(normalize(t, s), tc.window) {
						t.Errorf("%q, which the document never reads, is in %q", tc.window, s)
					}
				}
			}
		})
	}
}

// TestRuns: the runs of a cite are its longest lists of anchors consecutive
// in document order, whatever order the cite gives them in.
func TestRuns(t *testing.T) {
	doc, n := nodes41(t)
	for _, tc := range []struct {
		name string
		cite []string
		want [][]string
	}{
		{"(k)(1) + (k)(1)(ii): (k)(1)(i) between, two runs", []string{"k-1", "k-1-ii"}, [][]string{{"k-1"}, {"k-1-ii"}}},
		{"(k)(1)(i) + (k)(1)(ii): consecutive, one run", []string{"k-1-i", "k-1-ii"}, [][]string{{"k-1-i", "k-1-ii"}}},
		{"one anchor", []string{"d"}, [][]string{{"d"}}},
		{"two runs and a lone anchor", []string{"k-2", "a", "b", "k-1", "b-1", "k"}, [][]string{{"a", "b", "b-1"}, {"k", "k-1"}, {"k-2"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodes := cite(n, tc.cite...)
			for _, order := range [][]*parse.Node{nodes, reversed(nodes)} {
				runs, err := doc.Runs(order)
				if err != nil {
					t.Fatal(err)
				}
				var got [][]string
				for _, r := range runs {
					var ps []string
					for _, x := range r {
						ps = append(ps, strings.Join(x.Path, "-"))
					}
					got = append(got, ps)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Errorf("Runs = %q, want %q", got, tc.want)
				}
			}
		})
	}
}

// TestJoinRefuses: an empty cite, a nil anchor, an anchor of another
// document (even one at the same path, from another parse of the same XML)
// and an anchor given twice are refused, each by its named error.
func TestJoinRefuses(t *testing.T) {
	doc, n := nodes41(t)
	_, other := nodes41(t)
	_, n30 := docOf(t, readRepo(t, raw30))
	for _, tc := range []struct {
		name string
		cite []*parse.Node
		want error
	}{
		{"no anchor", nil, anchors.ErrEmptyCite},
		{"an empty list", []*parse.Node{}, anchors.ErrEmptyCite},
		{"a nil anchor", []*parse.Node{n["b"], nil}, anchors.ErrNilAnchor},
		{"1024.30's (a)", []*parse.Node{n["b-1"], n30["a"]}, anchors.ErrForeignAnchor},
		{"1024.30's (a) beside 1024.41's", []*parse.Node{n["a"], n30["a"]}, anchors.ErrForeignAnchor},
		{"(b) from another parse", []*parse.Node{other["b"], n["b-1"]}, anchors.ErrForeignAnchor},
		{"(b) twice", []*parse.Node{n["b"], n["b-1"], n["b"]}, anchors.ErrRepeatedAnchor},
		{"(b) twice, after (a)", []*parse.Node{n["a"], n["b"], n["b"]}, anchors.ErrRepeatedAnchor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := doc.Join(tc.cite); !errors.Is(err, tc.want) {
				t.Errorf("Join = %q, %v; want %v", got, err, tc.want)
			}
			if got, err := doc.Runs(tc.cite); !errors.Is(err, tc.want) {
				t.Errorf("Runs = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

// TestJoinRebuildsEachParagraph: joining every node, one run, gives back
// each paragraph's character data, the paragraphs with one space between
// them.
func TestJoinRebuildsEachParagraph(t *testing.T) {
	doc, _ := docOf(t, section("", "(a) <I>Head</I>—(1) <I>Head.</I> (i) Text (<I>2</I>).", "(A)(<I>1</I>) Text.", "(2) Last."))
	got, err := doc.Join(doc.Nodes())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"(a) Head—(1) Head. (i) Text (2). (A)(1) Text. (2) Last."}; !reflect.DeepEqual(got, want) {
		t.Errorf("Join = %q, want %q", got, want)
	}
}

// TestJoinThenNormalize (N6): a cite's text is the raw own texts joined,
// then normalized as one string; normalizing each first and joining after
// can leave two spaces where a <P> ends in one.
func TestJoinThenNormalize(t *testing.T) {
	doc, n := docOf(t, section("", "(a) <I>Head.</I> Text. ", "(b) More."))
	got, err := doc.Join([]*parse.Node{n["a"], n["b"]})
	if err != nil {
		t.Fatal(err)
	}
	if want := "(a) Head. Text. (b) More."; len(got) != 1 || normalize(t, got[0]) != want {
		t.Errorf("Normalize(Join) = %q, want %q", got, want)
	}
}
