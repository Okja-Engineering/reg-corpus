package anchors

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// Derived is what Build makes from a source's raw XML: its derived L0
// files, and the value of meta.yaml's sha256_text.
type Derived struct {
	Tree       *parse.Tree // the parsed tree, each node's SHA256 set
	TreeJSON   []byte      // tree.json: EncodeTree(Tree)
	TextMD     []byte      // text.md: TextMD(Tree)
	SHA256Text meta.SHA256 // sha256_text: SHA256Text(Tree)
}

// Build parses raw (internal/parse), sets each node's sha256 to Hash of its
// own text, separator and all (ADR 0013, the owner's F1 call: the hash
// covers every character), and renders tree.json, text.md and sha256_text
// from the tree. A parse failure is the parser's *parse.Error, with its
// location. The same bytes give the same output.
func Build(raw []byte) (*Derived, error) {
	t, err := parse.Parse(raw)
	if err != nil {
		return nil, err
	}
	for _, n := range walk(t.Children) {
		n.SHA256 = Hash(n.Text)
	}
	b, err := EncodeTree(t)
	if err != nil {
		return nil, err
	}
	return &Derived{Tree: t, TreeJSON: b, TextMD: TextMD(t), SHA256Text: SHA256Text(t)}, nil
}

// EncodeTree is tree.json's bytes for t: the tree's JSON form (the json tags
// on parse.Tree and parse.Node, which internal/parse owns), indented two
// spaces, with no HTML escaping ("&", "<" and ">" as they are), and a
// trailing LF. Decoding them and encoding again gives the same bytes.
func EncodeTree(t *parse.Tree) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(t); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// TextMD is text.md for t (ADR 0013, F5: T1), the section as it reads: "# "
// and the section's HEAD, then each <P>'s text (its nodes' own texts joined
// as Join joins them, with nothing between), then the CITA, if any, each
// after a blank line. Every text is verbatim: no anchor IDs, no markup, no
// normalization. The file is UTF-8 with LF line ends, and ends with one LF:
// the CITA's own, or one added when its text has none.
func TextMD(t *parse.Tree) []byte {
	var b strings.Builder
	b.WriteString("# " + t.Heading + "\n")
	for _, p := range paragraphs(walk(t.Children)) {
		b.WriteString("\n" + p + "\n")
	}
	if t.Citation != "" {
		b.WriteString("\n" + t.Citation)
		if !strings.HasSuffix(t.Citation, "\n") {
			b.WriteByte('\n')
		}
	}
	return []byte(b.String())
}

// SectionText is the text sha256_text hashes (ADR 0013, the owner's F3
// call: S1): the section's HEAD, one space, then every node's own text
// joined as Join joins them (nothing inside a <P>, one space between <P>s).
// The CITA is left out, so the hash changes only when the rule's words do.
// It is raw; SHA256Text normalizes it as one string (N6).
func SectionText(t *parse.Tree) string {
	return t.Heading + " " + strings.Join(paragraphs(walk(t.Children)), " ")
}

// SHA256Text is meta.yaml's sha256_text for t: Hash of SectionText.
func SHA256Text(t *parse.Tree) meta.SHA256 {
	return meta.SHA256(Hash(SectionText(t)))
}
