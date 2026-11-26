package anchors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

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
// from the tree. It does no I/O, so every error is about raw: a parse
// failure is the parser's *parse.Error, with its location, and text that
// text.md can't hold as it reads is TextMD's error, naming where it is. The
// same bytes give the same output.
func Build(raw []byte) (*Derived, error) {
	t, err := parse.Parse(raw)
	if err != nil {
		return nil, err
	}
	for _, n := range walk(t.Children) {
		if n.SHA256, err = Hash(n.Text); err != nil {
			return nil, err
		}
	}
	d := &Derived{Tree: t}
	if d.TreeJSON, err = EncodeTree(t); err != nil {
		return nil, err
	}
	if d.TextMD, err = TextMD(t); err != nil {
		return nil, err
	}
	if d.SHA256Text, err = SHA256Text(t); err != nil {
		return nil, err
	}
	return d, nil
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

// lineBreaks are the characters that end a line: CR, LF, NEL (U+0085), and
// the line and paragraph separators (U+2028, U+2029).
const lineBreaks = "\r\n\u0085\u2028\u2029"

// TextMD is text.md for t (ADR 0013, F5: T1), the section as it reads: "# "
// and the section's HEAD, then each <P>'s text (its nodes' own texts joined
// as Join joins them, with nothing between), then the CITA, if any, each
// after a blank line. Every text is verbatim: no anchor IDs, no markup but
// the "# " that starts the heading line, no normalization. The file is
// UTF-8 with LF line ends, and ends with exactly one LF: the CITA's own, or
// one added when its text has none. A tree with no CITA (an empty one
// reads alike) shows none.
//
// So that each text stays one line of its own, as it reads, TextMD refuses
// rather than rewrites (an error naming where) a HEAD, a node's own text,
// or a CITA past its one ending LF, that holds a line break (CR, LF,
// U+0085, U+2028 or U+2029) or holds nothing but White_Space, which would
// show as an empty line. Its tree may be one read back from tree.json, not
// only one the parser made.
func TextMD(t *parse.Tree) ([]byte, error) {
	doc := NewDocument(t)
	if err := shown(t.Section+" HEAD", t.Heading); err != nil {
		return nil, err
	}
	for _, n := range doc.nodes {
		if err := shown(ID(t.Section, n.Path), n.Text); err != nil {
			return nil, err
		}
	}
	if t.Citation != "" {
		if err := shown(t.Section+" CITA", strings.TrimSuffix(t.Citation, "\n")); err != nil {
			return nil, err
		}
	}
	var b strings.Builder
	b.WriteString("# " + t.Heading + "\n")
	for _, p := range paragraphs(doc.nodes) {
		b.WriteString("\n" + p + "\n")
	}
	if t.Citation != "" {
		b.WriteString("\n" + t.Citation)
		if !strings.HasSuffix(t.Citation, "\n") {
			b.WriteByte('\n')
		}
	}
	return []byte(b.String()), nil
}

// shown is an error when text, at where, can't be shown as one line of
// text.md as it reads: it holds a line break, or nothing but White_Space.
func shown(where, text string) error {
	if i := strings.IndexAny(text, lineBreaks); i >= 0 {
		r := []rune(text[i:])[0]
		return fmt.Errorf("anchors: %s holds a line break, %U, at byte %d; text.md holds each text on one line, verbatim, so it is refused, not rewritten", where, r, i)
	}
	if strings.TrimFunc(text, func(r rune) bool { return unicode.Is(unicode.White_Space, r) }) == "" {
		return fmt.Errorf("anchors: %s is %q, nothing but White_Space; text.md would show it as an empty line, so it is refused, not rewritten", where, text)
	}
	return nil
}

// SectionText is the text sha256_text hashes (ADR 0013, the owner's F3
// call: S1): the section's HEAD, one space, then every node's own text,
// the whole document joined as one run of a cite joins (Join: nothing
// inside a <P>, one space between <P>s). The CITA is left out, so the hash
// changes only when the rule's words do. It is raw; SHA256Text normalizes
// it as one string (N6), the HEAD and the space after it included. It is
// JoinSection of the HEAD and the document's Paragraphs.
func SectionText(t *parse.Tree) string {
	return JoinSection(t.Heading, NewDocument(t).Paragraphs())
}

// JoinSection is what sha256_text hashes, given a section's heading and
// its paragraphs' texts, each raw: the heading, one space, then the
// paragraphs with one space between them, normalized after, as one string
// (N6). It is the one definition of that text, for eCFR's tree
// (SectionText) and for the official edition fetch --verify compares with
// it (plan step 6, ADR 0014), whose heading and paragraphs come from
// another schema.
func JoinSection(heading string, paragraphs []string) string {
	return heading + " " + strings.Join(paragraphs, " ")
}

// SHA256Text is meta.yaml's sha256_text for t: Hash of SectionText.
func SHA256Text(t *parse.Tree) (meta.SHA256, error) {
	h, err := Hash(SectionText(t))
	return meta.SHA256(h), err
}
