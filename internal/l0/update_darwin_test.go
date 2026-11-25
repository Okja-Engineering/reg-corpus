//go:build darwin

package l0

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// immutable sets or clears the user immutable flag (chflags uchg) on p, so
// nothing can be renamed over it: a write that fails part-way.
func immutable(t *testing.T, p string, on bool) {
	t.Helper()
	flags := 0
	if on {
		flags = 0x2 // UF_IMMUTABLE
	}
	if err := syscall.Chflags(p, flags); err != nil {
		t.Fatal(err)
	}
}

// TestUpdateReportsWhatItWroteBeforeAFailure: when text.md can't be
// replaced, tree.json, replaced before it, is reported written, meta.yaml is
// not written, and the error is not a *Conflict (which says nothing was
// written); a re-run, once text.md can be replaced, writes the rest.
func TestUpdateReportsWhatItWroteBeforeAFailure(t *testing.T) {
	root, _ := storedSource(t)
	if _, _, err := Update(root, testSrc, derived("T1", "X1")); err != nil {
		t.Fatal(err)
	}
	text := filepath.Join(root, filepath.FromSlash(testSrc.TextPath()))
	immutable(t, text, true)
	t.Cleanup(func() { immutable(t, text, false) })
	before := snapshot(t, root)
	_, w, err := Update(root, testSrc, derivedSum("T2", "X2", "1111111111111111111111111111111111111111111111111111111111111111"))
	var c *Conflict
	if err == nil || errors.As(err, &c) || w != (Wrote{Tree: true}) {
		t.Errorf("%+v, %v; want tree.json written, then an error that is not a conflict", w, err)
	}
	got := changes(before, snapshot(t, root))
	if len(got) != 1 || got[0] != testSrc.TreePath() {
		t.Errorf("changed %q; want only tree.json", got)
	}
	immutable(t, text, false)
	if _, w, err := Update(root, testSrc, derivedSum("T2", "X2", "1111111111111111111111111111111111111111111111111111111111111111")); err != nil || w != (Wrote{Text: true, Meta: true}) {
		t.Errorf("re-run: %+v, %v; want text.md and meta.yaml written", w, err)
	}
	if b, err := os.ReadFile(text); err != nil || string(b) != "X2" {
		t.Errorf("text.md holds %q, %v", b, err)
	}
}
