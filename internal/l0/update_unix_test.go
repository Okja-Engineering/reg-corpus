//go:build unix

// A write that fails part-way (ADR 0013, §2's contract), forced with the
// file size limit on every unix, Linux CI included: Go ignores SIGXFSZ, so a
// write past RLIMIT_FSIZE returns EFBIG (internal/fsizetest).
package l0

import (
	"errors"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/fsizetest"
)

// updateOutcome is what Update returned in the child.
type updateOutcome struct {
	Wrote    Wrote
	Err      string
	Conflict bool
}

// TestUpdateReportsWhatItWroteWhenAWriteFails: with the file size limit
// between tree.json's size and text.md's, Update replaces tree.json, then
// fails writing text.md: tree.json is reported written, meta.yaml is not
// written, the error is not a *Conflict (which says nothing was written)
// and names text.md; a re-run, without the limit, writes the rest.
func TestUpdateReportsWhatItWroteWhenAWriteFails(t *testing.T) {
	const sum = "1111111111111111111111111111111111111111111111111111111111111111"
	text := strings.Repeat("X", 200)
	run := func(root string) any {
		_, w, err := Update(root, testSrc, derivedSum("T", text, sum))
		var c *Conflict
		o := updateOutcome{Wrote: w, Conflict: errors.As(err, &c)}
		if err != nil {
			o.Err = err.Error()
		}
		return o
	}
	root, _ := storedSource(t) // the child's own is never used
	before := snapshot(t, root)
	var o updateOutcome
	if !fsizetest.Run(t, root, 64, run, &o) {
		return
	}
	if o.Err == "" || o.Conflict || o.Wrote != (Wrote{Tree: true}) || !strings.Contains(o.Err, testSrc.TextPath()) {
		t.Errorf("%+v; want tree.json written, then an error naming %s that is not a conflict", o, testSrc.TextPath())
	}
	if got := changes(before, snapshot(t, root)); len(got) != 1 || got[0] != testSrc.TreePath() {
		t.Errorf("changed %q; want only tree.json", got)
	}
	if _, w, err := Update(root, testSrc, derivedSum("T", text, sum)); err != nil || w != (Wrote{Text: true, Meta: true}) {
		t.Errorf("re-run: %+v, %v; want text.md and meta.yaml written", w, err)
	}
}
