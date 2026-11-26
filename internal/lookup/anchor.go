package lookup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// Source is one L0 source as a lookup reads it, from the source table
// (internal/ecfr, sources.go), which passes it in: its name, how it is
// stored, and the plan step that builds its anchors, if they aren't built.
type Source struct {
	Name    string    // the section, e.g. "1024.41"; its anchor IDs start with anchors.ID(Name, nil), "1024-41"
	Stored  l0.Source // its directory, and who its meta.yaml must say it is
	Pending string    // the plan step that builds its anchors (the table's Goldens), "" once they are built
}

// Corpus is the L0 a lookup reads: the corpus root, the slash path from it
// that every source is under (without which the root is not the corpus
// root), and the source table.
type Corpus struct {
	Root    string
	Dir     string // e.g. sources/ecfr/title-12
	Sources []Source
}

// Anchor is one resolved anchor: what `regcorpus text` prints. Every value
// is as stored, checked (Corpus.Anchor) but never rewritten.
type Anchor struct {
	ID       string    // the anchor ID asked for, e.g. 1024-41-b-2-i-B
	Text     string    // its own text, verbatim, as tree.json holds it
	SHA256   string    // its own text's sha256, as tree.json holds it
	Children []string  // its children's anchor IDs, in document order
	MetaPath string    // its source's meta.yaml, by slash path from the corpus root
	Meta     meta.Meta // that meta.yaml
}

// The anchor IDs a lookup can't resolve, by kind (errors.Is): one that names
// no source in the table (ErrNoSource), one whose source's anchors aren't
// built yet (ErrNotBuilt), and one that names a source but no anchor in it
// (ErrNoAnchor). Each error that wraps one says which ID.
var (
	ErrNoSource = errors.New("lookup: an anchor ID that names no source")
	ErrNotBuilt = errors.New("lookup: an anchor a later plan step builds")
	ErrNoAnchor = errors.New("lookup: no such anchor")
)

// miss is an error of one of the kinds above, with its own message.
type miss struct {
	kind error
	msg  string
}

func (m *miss) Error() string { return m.msg }
func (m *miss) Unwrap() error { return m.kind }

func missf(kind error, format string, a ...any) error {
	return &miss{kind, fmt.Sprintf(format, a...)}
}

// commentSegment is the segment that makes an anchor ID a Supplement I
// comment's (spec "Definitions": 1024-41-b-2-i-interp-1).
const commentSegment = "interp"

// Anchor resolves id, an anchor ID matched exactly and case-sensitively
// (the owner's F3 call), to the anchor it names, from its source's
// meta.yaml and tree.json alone: no raw XML, no text.md, nothing under
// registry/ or staging/, and no network.
//
// First, by id alone: an id that is no source's section ID (anchors.ID of
// its name and no path) and doesn't start with one and a "-" names no
// source (ErrNoSource); one with an "interp" segment is a Supplement I
// comment's, which plan step 8 builds, and one of a source whose anchors
// are Pending is that step's (ErrNotBuilt). Then it reads the source and
// checks what it reads, before resolving (the chief of staff's F6): l0.Read
// (names by exact name, meta.yaml in the spec's schema); meta.yaml names
// the source (l0.State.Names); tree.json is there, a regular file, and
// decodes strictly as the tree's JSON form (no unknown field, one value);
// its section is the source's; no node is null or has no label path, and
// no two have one anchor ID; and the tree's text gives meta.yaml's
// sha256_text (anchors.SHA256Text), so the metadata describes the text.
// Any failure is a *l0.Conflict. Then id must be a node's anchor ID
// (anchors.ID), else ErrNoAnchor (a section's own ID, 1024-41, is none),
// and that node's sha256 must be anchors.Hash of its own text (a
// *l0.Conflict otherwise). A file it can't read is an error as is.
func (c Corpus) Anchor(id string) (*Anchor, error) {
	s, err := c.source(id)
	if err != nil {
		return nil, err
	}
	if slices.Contains(strings.Split(id, "-"), commentSegment) {
		return nil, missf(ErrNotBuilt, "%q is a comment anchor: Supplement I comment anchors are plan step 8's", id)
	}
	if s.Pending != "" {
		return nil, missf(ErrNotBuilt, "%q: anchors for %s are %s's", id, s.Name, s.Pending)
	}
	tr, m, byID, err := c.read(s)
	if err != nil {
		return nil, err
	}
	n, ok := byID[id]
	if !ok {
		return nil, missf(ErrNoAnchor, "no anchor %q in %s", id, s.Name)
	}
	if sum, err := anchors.Hash(n.Text); err != nil || sum != n.SHA256 {
		return nil, l0.Conflictf("%s: anchor %s has sha256 %q, but its own text hashes to %q (%v): tree.json is not what parse writes; run regcorpus parse %s (R1)",
			s.Stored.TreePath(), id, n.SHA256, sum, err, s.Name)
	}
	a := &Anchor{ID: id, Text: n.Text, SHA256: n.SHA256, Children: []string{}, MetaPath: s.Stored.MetaPath(), Meta: *m}
	for _, ch := range n.Children {
		a.Children = append(a.Children, anchors.ID(tr.Section, ch.Path))
	}
	return a, nil
}

// source is the source id names: the one whose section ID is id, or
// starts it and a "-".
func (c Corpus) source(id string) (Source, error) {
	var ids []string
	for _, s := range c.Sources {
		p := anchors.ID(s.Name, nil)
		if id == p || strings.HasPrefix(id, p+"-") {
			return s, nil
		}
		ids = append(ids, p)
	}
	return Source{}, missf(ErrNoSource, "%q names no source: an anchor ID is its section's (%s) and a label path, e.g. 1024-41-b-2-i-B",
		id, strings.Join(ids, ", "))
}

// read reads s's meta.yaml and tree.json, checks them (Anchor says how) and
// returns the tree, the meta.yaml and the tree's nodes by anchor ID.
func (c Corpus) read(s Source) (*parse.Tree, *meta.Meta, map[string]*parse.Node, error) {
	st, err := l0.Read(c.Root, s.Stored)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := st.Names(s.Stored); err != nil {
		return nil, nil, nil, err
	}
	treePath := s.Stored.TreePath()
	b, ok, err := st.TreeJSON(c.Root, s.Stored)
	switch {
	case err != nil:
		return nil, nil, nil, err
	case !ok:
		return nil, nil, nil, l0.Conflictf("%s does not exist: run regcorpus parse %s (R1)", treePath, s.Name)
	}
	refuse := func(format string, a ...any) error {
		return l0.Conflictf("%s: %s: it is not the tree.json regcorpus parse writes for %s; run regcorpus parse %s (R1)",
			treePath, fmt.Sprintf(format, a...), s.Name, s.Name)
	}
	tr, err := decodeTree(b)
	if err != nil {
		return nil, nil, nil, refuse("%v", err)
	}
	if tr.Section != s.Name {
		return nil, nil, nil, refuse("it is the tree of section %q, not %s", tr.Section, s.Name)
	}
	byID := map[string]*parse.Node{}
	if err := index(tr.Section, tr.Children, byID); err != nil {
		return nil, nil, nil, refuse("%v", err)
	}
	m := st.Meta
	if m.SHA256Text == "" {
		return nil, nil, nil, l0.Conflictf("%s has no sha256_text, so nothing says which text it describes: run regcorpus parse %s (R1)", s.Stored.MetaPath(), s.Name)
	}
	if sum, err := anchors.SHA256Text(tr); err != nil || sum != m.SHA256Text {
		return nil, nil, nil, l0.Conflictf("%s's text hashes to sha256_text %q (%v), but %s says %q: the metadata doesn't describe the text; run regcorpus parse %s (R1)",
			treePath, sum, err, s.Stored.MetaPath(), m.SHA256Text, s.Name)
	}
	return tr, m, byID, nil
}

// decodeTree decodes b strictly as the tree's JSON form (internal/parse's
// json tags): one JSON value, with no field the form doesn't have.
func decodeTree(b []byte) (*parse.Tree, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var tr parse.Tree
	if err := dec.Decode(&tr); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("more than one JSON value")
	}
	return &tr, nil
}

// index adds nodes and their descendants to byID by anchor ID. A null node,
// one with no label path (whose ID would be the section's) and an ID given
// twice are errors.
func index(section string, nodes []*parse.Node, byID map[string]*parse.Node) error {
	for _, n := range nodes {
		switch {
		case n == nil:
			return errors.New("a null node")
		case len(n.Path) == 0:
			return fmt.Errorf("a node with no label path, own text %q", n.Text)
		}
		id := anchors.ID(section, n.Path)
		if _, ok := byID[id]; ok {
			return fmt.Errorf("anchor ID %s twice", id)
		}
		byID[id] = n
		if err := index(section, n.Children, byID); err != nil {
			return err
		}
	}
	return nil
}

// lines is a's plain output, one string per line, each ending in an LF:
// the anchor ID; its own text; an empty line; "sha256: " and its sha256;
// "children: " and its children's IDs, space-separated, or "none";
// "meta.yaml: " and its source's meta.yaml path; then each line of that
// meta.yaml as meta.Marshal writes it, which is the stored file's bytes,
// since meta.Parse reads only what Marshal writes.
func (a *Anchor) lines() ([]string, error) {
	mb, err := meta.Marshal(a.Meta)
	if err != nil {
		return nil, err
	}
	children := "none"
	if len(a.Children) > 0 {
		children = strings.Join(a.Children, " ")
	}
	out := []string{a.ID + "\n", a.Text + "\n", "\n", "sha256: " + a.SHA256 + "\n", "children: " + children + "\n", "meta.yaml: " + a.MetaPath + "\n"}
	for l := range strings.Lines(string(mb)) {
		out = append(out, l)
	}
	return out, nil
}

// json is a's --json output: one line, a JSON object with the keys anchor,
// text, sha256, children (an array, empty for a leaf), meta_path and meta
// (meta.JSON), in that order, nothing HTML-escaped. encoding/json escapes
// every C0 control, U+2028 and U+2029; DEL, which it leaves and the sink
// would write as \x7f, not a JSON escape, is written \u007f here. The sink
// writes a C1 control as \u0080 to \u009f, a JSON escape of itself, so the
// line the stream holds is JSON that decodes to the stored text.
func (a *Anchor) json() ([]byte, error) {
	mj, err := meta.JSON(a.Meta)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	err = enc.Encode(struct {
		Anchor   string          `json:"anchor"`
		Text     string          `json:"text"`
		SHA256   string          `json:"sha256"`
		Children []string        `json:"children"`
		MetaPath string          `json:"meta_path"`
		Meta     json.RawMessage `json:"meta"`
	}{a.ID, a.Text, a.SHA256, a.Children, a.MetaPath, mj})
	if err != nil {
		return nil, err
	}
	return bytes.ReplaceAll(b.Bytes(), []byte{0x7f}, []byte(`\u007f`)), nil
}
