package anchors

import (
	"cmp"
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
// space before its heading, the heading, and the one "—" or space that ends
// the heading is not empty. That separator is the one the parser leaves at
// the end of a lead-in's own text when a child's label follows it (ADR
// 0012); nothing else is taken away, so a second space, a no-break space or
// an en dash after a heading makes n operative, as does a space after a
// label with no heading. Own text is n.Text as the parser keeps it,
// verbatim. An error means n's own text does not start with its label, and
// its heading if it has one, as every node the parser makes does.
func Operative(n *parse.Node) (bool, error) {
	rest, err := pastHeading(n)
	return rest != "", err
}

// pastHeading is n's own text past its label and, if n has a heading, past
// the one space before it, the heading and the one "—" or space after it.
func pastHeading(n *parse.Node) (string, error) {
	if len(n.Path) == 0 {
		return "", fmt.Errorf("anchors: a node with no label path, own text %q", n.Text)
	}
	label := "(" + n.Path[len(n.Path)-1] + ")"
	rest, ok := strings.CutPrefix(n.Text, label)
	if !ok {
		return "", fmt.Errorf("anchors: node %s: own text %q does not start with its label %s", strings.Join(n.Path, "-"), n.Text, label)
	}
	if n.Heading == "" {
		return rest, nil
	}
	if rest, ok = strings.CutPrefix(rest, " "+n.Heading); !ok {
		return "", fmt.Errorf("anchors: node %s: own text %q does not hold its heading %q one space after its label", strings.Join(n.Path, "-"), n.Text, n.Heading)
	}
	if r, ok := strings.CutPrefix(rest, "—"); ok {
		return r, nil
	}
	return strings.TrimPrefix(rest, " "), nil
}

// Join is the text a multi-anchor cite quotes from (spec "Definitions";
// ADR 0013, the owner's F2 call): the nodes' own texts in document order,
// with nothing between two in one <P> and one space between two in
// different <P>s, so the own texts of one <P> join to exactly its character
// data, as the document reads. Nodes are put in document order whatever
// order they are given in: by <P> (Para), then by depth, since the nodes of
// one <P> are a chain, each a child of the one before. A node given twice
// is an error. The join is raw, not normalized: a cite's text is matched
// against the join normalized as one string (N6), never against normalized
// pieces joined. Where a quote may cross a join (only between anchors
// consecutive in the document) is C6's rule (plan step 13).
func Join(nodes []*parse.Node) (string, error) {
	ns := slices.Clone(nodes)
	slices.SortStableFunc(ns, docOrder)
	for i := 1; i < len(ns); i++ {
		if docOrder(ns[i-1], ns[i]) == 0 {
			return "", fmt.Errorf("anchors: two nodes at one place in the document, <P> %d at depth %d (%s and %s): an anchor given twice",
				ns[i].Para, len(ns[i].Path), strings.Join(ns[i-1].Path, "-"), strings.Join(ns[i].Path, "-"))
		}
	}
	return strings.Join(paragraphs(ns), " "), nil
}

// The red commit's stubs of the document-aware join: Runs puts every cited
// node in one run, as the old Join did, refusing only a node given twice.
type Document struct{ nodes []*parse.Node }

func NewDocument(t *parse.Tree) *Document { return &Document{walk(t.Children)} }

func (d *Document) Nodes() []*parse.Node { return slices.Clone(d.nodes) }

var (
	ErrEmptyCite      = errors.New("anchors: a cite with no anchor")
	ErrNilAnchor      = errors.New("anchors: a nil anchor in a cite")
	ErrForeignAnchor  = errors.New("anchors: an anchor that is not in the document")
	ErrRepeatedAnchor = errors.New("anchors: an anchor given twice in a cite")
)

func (d *Document) Runs(cite []*parse.Node) ([][]*parse.Node, error) {
	if slices.Contains(cite, nil) {
		return nil, errors.New("stub: a nil node")
	}
	if _, err := Join(cite); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRepeatedAnchor, err)
	}
	ns := slices.Clone(cite)
	slices.SortStableFunc(ns, docOrder)
	return [][]*parse.Node{ns}, nil
}

func (d *Document) Join(cite []*parse.Node) ([]string, error) {
	runs, err := d.Runs(cite)
	if err != nil {
		return nil, err
	}
	return []string{strings.Join(paragraphs(runs[0]), " ")}, nil
}

// docOrder orders nodes of one tree as the document does.
func docOrder(a, b *parse.Node) int {
	return cmp.Or(cmp.Compare(a.Para, b.Para), cmp.Compare(len(a.Path), len(b.Path)))
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
