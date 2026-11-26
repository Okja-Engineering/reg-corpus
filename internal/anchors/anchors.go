package anchors

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// ID is the anchor ID of the node at path in section (spec "Definitions"):
// the section with each "." made "-", then each label value after a "-".
// ID("1024.41", ["b", "2", "i", "B"]) is "1024-41-b-2-i-B"; an italic
// label is its number, so (c)(2)(v)(A)(1) is "1024-41-c-2-v-A-1".
func ID(section string, path []string) string {
	return strings.Join(append([]string{strings.ReplaceAll(section, ".", "-")}, path...), "-")
}

// Operative reports whether n is an operative anchor (spec "Definitions";
// ADR 0013, the owner's F1 call): n's own text minus its label, the one
// space before its heading, the heading, and the one dash or space that
// ends the heading is not empty. It is decided on n's whole own text
// normalized as one string (Normalize, N6), the text n's hash covers: past
// the normalized label, and the one space and the heading if n has one,
// one "-" or else one space is that separator (an em or en dash, a space, a
// no-break space or any run of White_Space, as the parser leaves at the end
// of a lead-in's own text; ADR 0012). Where the heading itself ends in
// White_Space, that run and any White_Space after it are one space, the
// heading's own, so one "-" after it is still the separator: with the
// heading "Head ", "(a) Head —" and "(a) Head  —" are both heading-only,
// while with the heading "Head", "(a) Head —" leaves "-". So two
// nodes with the same path, the same heading and the same normalized own
// text get the same verdict. Nothing else is taken away: two dashes after a
// heading leave one, a space after a label with no heading is text, and
// text that NFC joins with the heading's last character (a combining mark)
// is text. An error means n's own text does not start with its label, and
// one space and its heading if it has one, as every node the parser makes
// does, or is not valid UTF-8.
func Operative(n *parse.Node) (bool, error) {
	l, err := lead(n)
	if err != nil {
		return false, err
	}
	var text string
	if text, err = Normalize(n.Text); err == nil {
		l, err = Normalize(l)
	}
	if err != nil {
		return false, fmt.Errorf("anchors: node %s: %w", strings.Join(n.Path, "-"), err)
	}
	rest, ok := strings.CutPrefix(text, l)
	if !ok {
		// NFC joined what follows the heading with its last character, which
		// only a character no separator is (a combining mark, say) can do.
		return true, nil
	}
	if n.Heading != "" {
		if r, ok := strings.CutPrefix(rest, "-"); ok {
			rest = r
		} else {
			rest = strings.TrimPrefix(rest, " ")
		}
	}
	return rest != "", nil
}

// lead is the start of n's own text, verbatim, that the operative test
// takes away before the separator: its label and, if n has a heading, one
// space and the heading. An error means n's own text doesn't start so.
func lead(n *parse.Node) (string, error) {
	if len(n.Path) == 0 {
		return "", fmt.Errorf("anchors: a node with no label path, own text %q", n.Text)
	}
	label := "(" + n.Path[len(n.Path)-1] + ")"
	if !strings.HasPrefix(n.Text, label) {
		return "", fmt.Errorf("anchors: node %s: own text %q does not start with its label %s", strings.Join(n.Path, "-"), n.Text, label)
	}
	if n.Heading == "" {
		return label, nil
	}
	if !strings.HasPrefix(n.Text, label+" "+n.Heading) {
		return "", fmt.Errorf("anchors: node %s: own text %q does not hold its heading %q one space after its label", strings.Join(n.Path, "-"), n.Text, n.Heading)
	}
	return label + " " + n.Heading, nil
}

// Document is one parsed section's anchors in document order: the order
// its nodes are read in, each before its children, which Join, text.md and
// sha256_text all follow. Two anchors are consecutive in the document when
// no anchor comes between them in that order (spec "Multi-anchor cite").
// Nodes are its tree's, by identity: a node of another tree, even one with
// the same path, is not in it.
type Document struct {
	nodes []*parse.Node
	at    map[*parse.Node]int
}

// NewDocument indexes t's nodes in document order.
func NewDocument(t *parse.Tree) *Document {
	d := &Document{nodes: walk(t.Children), at: map[*parse.Node]int{}}
	for i, n := range d.nodes {
		d.at[n] = i
	}
	return d
}

// Nodes is every node of the document, in document order.
func (d *Document) Nodes() []*parse.Node { return slices.Clone(d.nodes) }

// Paragraphs is the document's text one <P> at a time, in document order:
// each the own texts of its nodes with nothing between them, raw (the join
// rule inside a <P>). JoinSection puts one space between them for
// sha256_text, text.md a blank line, and fetch --verify normalizes each
// (Normalize, edges kept) and compares them one by one, in order, with the
// official edition's, normalized after govinfo.Read trims their edges
// (plan step 6, ADR 0014).
func (d *Document) Paragraphs() []string { return paragraphs(d.nodes) }

// The cites Runs and Join refuse, each named in the error they return.
var (
	ErrEmptyCite      = errors.New("anchors: a cite with no anchor")
	ErrNilAnchor      = errors.New("anchors: a nil anchor in a cite")
	ErrForeignAnchor  = errors.New("anchors: an anchor that is not in the document")
	ErrRepeatedAnchor = errors.New("anchors: an anchor given twice in a cite")
)

// Runs splits a cite's anchors into its runs: the longest lists of anchors
// consecutive in the document, each in document order, the runs in
// document order too, whatever order the cite gives them in. So [k-1,
// k-1-ii] is two runs, since k-1-i comes between them, and [k-1-i, k-1-ii]
// one. A quote may cross a join only within a run (spec "Multi-anchor
// cite"): this is the one place that says which anchors are consecutive.
// An empty cite, a nil anchor, one not in d, and one given twice are
// refused, with ErrEmptyCite, ErrNilAnchor, ErrForeignAnchor or
// ErrRepeatedAnchor.
func (d *Document) Runs(cite []*parse.Node) ([][]*parse.Node, error) {
	if len(cite) == 0 {
		return nil, ErrEmptyCite
	}
	pos := make([]int, len(cite))
	for i, n := range cite {
		if n == nil {
			return nil, fmt.Errorf("%w: item %d", ErrNilAnchor, i+1)
		}
		p, ok := d.at[n]
		if !ok {
			return nil, fmt.Errorf("%w: item %d, %s", ErrForeignAnchor, i+1, strings.Join(n.Path, "-"))
		}
		pos[i] = p
	}
	slices.Sort(pos)
	var runs [][]*parse.Node
	for i, p := range pos {
		switch {
		case i > 0 && p == pos[i-1]:
			return nil, fmt.Errorf("%w: %s", ErrRepeatedAnchor, strings.Join(d.nodes[p].Path, "-"))
		case i > 0 && p == pos[i-1]+1:
			runs[len(runs)-1] = append(runs[len(runs)-1], d.nodes[p])
		default:
			runs = append(runs, []*parse.Node{d.nodes[p]})
		}
	}
	return runs, nil
}

// Join is the text a multi-anchor cite quotes from (spec "Definitions";
// ADR 0013, the owner's F2 call): one string per run of the cite (Runs), in
// document order, each its anchors' own texts with nothing between two in
// one <P> and one space between two in different <P>s, so the own texts of
// one <P> join to exactly its character data, as the document reads. A
// quote may cross a join only within one run, so it is matched within one
// string, never across two. Each string is raw, not normalized: a quote is
// matched against it normalized as one string (N6), never against
// normalized pieces joined. It refuses what Runs refuses.
func (d *Document) Join(cite []*parse.Node) ([]string, error) {
	runs, err := d.Runs(cite)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(runs))
	for i, r := range runs {
		out[i] = joinRun(r)
	}
	return out, nil
}

// joinRun joins a run of consecutive nodes, in document order: their
// paragraphs with one space between.
func joinRun(nodes []*parse.Node) string {
	return strings.Join(paragraphs(nodes), " ")
}

// paragraphs joins nodes, in document order, within each <P>: the own texts
// of each run of nodes in one <P>, with nothing between them. It is the one
// place the join rule's "nothing inside a <P>" is applied: Join puts one
// space between the results, text.md a blank line.
func paragraphs(nodes []*parse.Node) []string {
	var out []string
	for i, n := range nodes {
		if i == 0 || n.Para != nodes[i-1].Para {
			out = append(out, "")
		}
		out[len(out)-1] += n.Text
	}
	return out
}

// walk lists nodes and their descendants in document order.
func walk(nodes []*parse.Node) []*parse.Node {
	var out []*parse.Node
	for _, n := range nodes {
		out = append(out, n)
		out = append(out, walk(n.Children)...)
	}
	return out
}
