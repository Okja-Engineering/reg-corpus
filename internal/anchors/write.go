package anchors

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// Written says what Write changed, and the sha256_text it set.
type Written struct {
	Tree, Text, Meta bool // whether tree.json, text.md and meta.yaml changed
	SHA256Text       meta.SHA256
}

// Write derives s's files under root from its stored raw XML, offline,
// through l0.Update (ADR 0010): under s's lock, after the stored-state
// check (l0's Stored), it builds from the raw XML that check has just read
// (Build), writes tree.json and text.md (l0.Derive) and sets sha256_text,
// keeping every other field of meta.yaml. A file that already holds what
// Build makes is left as it is, so a second run changes nothing. Refused
// (an *l0.Conflict, with nothing written): what Stored refuses; an
// overrides.yaml in s's directory, whatever it is, since applying overrides
// isn't built (R1a), and parsing without one would drop a reviewed fix; raw
// XML that doesn't parse, named with the parser's location; and anything
// but a regular file where tree.json or text.md goes.
func Write(root string, s l0.Source) (Written, error) {
	var w Written
	_, changed, err := l0.Update(root, s, func(m *meta.Meta, raw []byte) error {
		_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(s.OverridesPath())))
		switch {
		case err == nil:
			return l0.Conflictf("%s exists, and applying overrides isn't built yet: the first source that needs a reviewed override builds it (R1a). Parsing as if it weren't there would drop its fixes, so nothing is written", s.OverridesPath())
		case !errors.Is(err, fs.ErrNotExist):
			return err
		}
		d, err := Build(raw)
		var pe *parse.Error
		if errors.As(err, &pe) {
			return l0.Conflictf("%s: %v", s.RawPath(m.SourceVersionDate), err)
		} else if err != nil {
			return err
		}
		ch, err := l0.Derive(root, []l0.File{{Path: s.TreePath(), Body: d.TreeJSON}, {Path: s.TextPath(), Body: d.TextMD}})
		if err != nil {
			return err
		}
		w.Tree, w.Text = ch[0], ch[1]
		m.SHA256Text, w.SHA256Text = d.SHA256Text, d.SHA256Text
		return nil
	})
	if err != nil {
		return Written{}, err
	}
	w.Meta = changed
	return w, nil
}
