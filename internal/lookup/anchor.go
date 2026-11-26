package lookup

import (
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
	"github.com/okja-engineering/reg-corpus/internal/sink"
)

// Source is one L0 source as a lookup reads it, from the source table
// (internal/ecfr, sources.go), which passes it in: its name, how it is
// stored, and the plan steps that build its anchors and the Supplement I
// comment anchors on it, if they aren't built.
type Source struct {
	Name    string    // the section, e.g. "1024.41"; its anchor IDs start with anchors.ID(Name, nil), "1024-41"
	Stored  l0.Source // its directory, and who its meta.yaml must say it is
	Pending string    // the plan step that builds its anchors (the table's Goldens), "" once they are built
	// Comments is the plan step that builds Supplement I's comment anchors
	// on its paragraphs (the table's Comments), "" when Supplement I's
	// slice has none on it, so an "interp" segment under it is no anchor.
	Comments string
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
// is what regcorpus parse derives from the stored raw XML, which the stored
// files are, byte for byte (Corpus.Anchor).
type Anchor struct {
	ID       string    // the anchor ID asked for, e.g. 1024-41-b-2-i-B
	Text     string    // its own text, verbatim, as tree.json holds it
	SHA256   string    // its own text's sha256, as tree.json holds it
	Children []string  // its children's anchor IDs, in document order
	MetaPath string    // its source's meta.yaml, by slash path from the corpus root
	Meta     meta.Meta // that meta.yaml
}

// The errors a lookup can't resolve an anchor ID with, by kind (errors.Is),
// each meaning the same to every caller: the root isn't the corpus root
// (ErrNotRoot, l0.ErrNotRoot); the ID names no source in the table
// (ErrNoSource); its anchor is one a later plan step builds (ErrNotBuilt);
// it names a source but no anchor in it (ErrNoAnchor). Each error that
// wraps one says which ID, or which root. A stored file that refuses is an
// *l0.Conflict, and a file that can't be read an error as is.
var (
	ErrNotRoot  = l0.ErrNotRoot
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
// (the owner's F3 call), to the own text regcorpus parse derives for that
// label path from the source's stored raw XML, byte for byte, with the
// meta.yaml that describes it (the chief of staff's round-1 ruling, §1). It
// reads the source's meta.yaml, raw XML, tree.json and text.md, under
// c.Root, offline; nothing under registry/ or staging/.
//
// In order, the first that fails returns:
//   - c.Root is the corpus root (l0.CorpusRoot; ErrNotRoot, or a
//     *l0.Conflict for a symlink or a file on the way).
//   - By id alone: a source's section ID (anchors.ID of its name and no
//     path, 1024-41) is id, or starts it and a "-", else ErrNoSource; id
//     is not that section ID, which is no anchor (ErrNoAnchor); an "interp"
//     segment is a Supplement I comment's, the source's Comments step's
//     (ErrNotBuilt), or no anchor where it has none (ErrNoAnchor); the
//     source's anchors are built (Pending, else ErrNotBuilt).
//   - The stored files (derive), each refusal a *l0.Conflict: the source's
//     stored state as every writer checks it (l0.Read, l0.State.Stored:
//     meta.yaml names the source, raw/ holds exactly the raw file it
//     describes, with sha256_raw's bytes); no overrides.yaml; the raw XML
//     builds (anchors.Build, as regcorpus parse builds it); tree.json,
//     text.md and meta.yaml's sha256_text are what it builds
//     (anchors.Derived.Stale, as fetch --verify checks them); and the raw
//     XML is the source's section.
//   - id is a node's anchor ID (anchors.ID) in that tree, else
//     ErrNoAnchor.
func (c Corpus) Anchor(id string) (*Anchor, error) {
	if err := l0.CorpusRoot(c.Root, c.Dir, "regcorpus"); err != nil {
		return nil, err
	}
	s, err := c.source(id)
	if err != nil {
		return nil, err
	}
	switch {
	case id == anchors.ID(s.Name, nil):
		return nil, missf(ErrNoAnchor, "no anchor %q in %s: a section's own ID is no anchor", id, s.Name)
	case slices.Contains(strings.Split(id, "-"), commentSegment) && s.Comments == "":
		return nil, missf(ErrNoAnchor, "no anchor %q in %s: Supplement I's comments the corpus keeps are on no paragraph of it", id, s.Name)
	case slices.Contains(strings.Split(id, "-"), commentSegment):
		return nil, missf(ErrNotBuilt, "%q is a comment anchor: Supplement I comment anchors are %s's", id, s.Comments)
	case s.Pending != "":
		return nil, missf(ErrNotBuilt, "%q: anchors for %s are %s's", id, s.Name, s.Pending)
	}
	tr, m, err := c.derive(s)
	if err != nil {
		return nil, err
	}
	for _, n := range anchors.NewDocument(tr).Nodes() {
		if anchors.ID(tr.Section, n.Path) != id {
			continue
		}
		a := &Anchor{ID: id, Text: n.Text, SHA256: n.SHA256, Children: []string{}, MetaPath: s.Stored.MetaPath(), Meta: *m}
		for _, ch := range n.Children {
			a.Children = append(a.Children, anchors.ID(tr.Section, ch.Path))
		}
		return a, nil
	}
	return nil, missf(ErrNoAnchor, "no anchor %q in %s", id, s.Name)
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

// derive derives s again from its stored raw XML, as fetch --verify's first
// check does (ADR 0014, round 1, §1), and returns the tree and the
// meta.yaml, once the stored files are what it derives (Anchor says what
// it checks). It takes no lock: it writes nothing, and every writer
// replaces a whole file by rename, so a file it reads is never half
// written; one replaced between two reads differs from what it derives,
// and is refused.
func (c Corpus) derive(s Source) (*parse.Tree, *meta.Meta, error) {
	st, err := l0.Read(c.Root, s.Stored)
	if err != nil {
		return nil, nil, err
	}
	raw, err := st.Stored(c.Root, s.Stored)
	if err != nil {
		return nil, nil, err
	}
	if st.Overrides() {
		return nil, nil, l0.Conflictf("%s exists, and applying overrides isn't built yet (R1a): regcorpus parse writes nothing for %s while it is there, so no text is what parse derives",
			s.Stored.OverridesPath(), s.Name)
	}
	rawPath := s.Stored.RawPath(st.Meta.SourceVersionDate)
	d, err := anchors.Build(raw)
	if err != nil {
		return nil, nil, l0.Conflictf("%s: %v: regcorpus parse builds nothing from it", rawPath, err)
	}
	stale, err := d.Stale(c.Root, s.Stored, st)
	if err != nil {
		return nil, nil, err
	}
	if len(stale) > 0 {
		return nil, nil, l0.Conflictf("%s: missing, or not what regcorpus parse derives from %s, so what text would print is not the source's: run regcorpus parse %s to regenerate them (R1)",
			strings.Join(stale, ", "), rawPath, s.Name)
	}
	if d.Tree.Section != s.Name {
		return nil, nil, l0.Conflictf("%s is section %q's raw XML, not %s's (R1)", rawPath, d.Tree.Section, s.Name)
	}
	return d.Tree, st.Meta, nil
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

// writeJSON writes a's --json output to w as one line (sink.JSON): a JSON
// object with the keys anchor, text, sha256, children (an array, empty for
// a leaf), meta_path and meta (meta.JSON), in that order. Through the sink
// the line is JSON that decodes to the stored text. An error rendering
// meta.yaml or encoding the object is returned, and nothing is written.
func (a *Anchor) writeJSON(w io.Writer) error {
	mj, err := meta.JSON(a.Meta)
	if err != nil {
		return err
	}
	return sink.JSON(w, struct {
		Anchor   string          `json:"anchor"`
		Text     string          `json:"text"`
		SHA256   string          `json:"sha256"`
		Children []string        `json:"children"`
		MetaPath string          `json:"meta_path"`
		Meta     json.RawMessage `json:"meta"`
	}{a.ID, a.Text, a.SHA256, a.Children, a.MetaPath, mj})
}
