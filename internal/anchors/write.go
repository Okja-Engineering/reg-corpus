package anchors

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
)

// Written says which of a source's files Write replaced (l0.Wrote), and the
// sha256_text it set.
type Written struct {
	l0.Wrote
	SHA256Text meta.SHA256
}

// Write derives s's files under root from its stored raw XML, offline,
// through l0.Update (ADRs 0010, 0013): under s's lock, after the
// stored-state check (l0's Stored), it builds from the raw XML that check
// has just read (Build), sets sha256_text, keeping every other field of
// meta.yaml, and hands Update tree.json's and text.md's bytes, which Update
// alone writes. A file that already holds what Build makes is left as it
// is, so a second run changes nothing.
//
// Refused (an *l0.Conflict, with nothing written): what Update refuses
// (Stored's check, a name like meta.yaml, tree.json, text.md or
// overrides.yaml, or like a directory on the way, in another case, anything
// but a regular file where tree.json or text.md goes); an overrides.yaml in
// s's directory, whatever it is, since applying overrides isn't built
// (R1a), and parsing without one would drop a reviewed fix; and raw XML
// that Build refuses, named with the raw file and Build's error (the
// parser's location, or where text.md would break a line or show an empty
// one). An error while writing returns, in Written, the files replaced
// before it, and names the file it was writing; a re-run writes the rest.
func Write(root string, s l0.Source) (Written, error) {
	var sum meta.SHA256
	_, w, err := l0.Update(root, s, func(m *meta.Meta, raw []byte) (*l0.Derived, error) {
		_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(s.OverridesPath())))
		switch {
		case err == nil:
			return nil, l0.Conflictf("%s exists, and applying overrides isn't built yet: the first source that needs a reviewed override builds it (R1a). Parsing as if it weren't there would drop its fixes, so nothing is written", s.OverridesPath())
		case !errors.Is(err, fs.ErrNotExist):
			return nil, err
		}
		d, err := Build(raw)
		if err != nil {
			return nil, l0.Conflictf("%s: %v", s.RawPath(m.SourceVersionDate), err)
		}
		m.SHA256Text, sum = d.SHA256Text, d.SHA256Text
		return &l0.Derived{Tree: d.TreeJSON, Text: d.TextMD}, nil
	})
	return Written{Wrote: w, SHA256Text: sum}, err
}
