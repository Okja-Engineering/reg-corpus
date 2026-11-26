// TreeJSON: what fetch --verify compares a fresh fetch's tree with (plan
// step 6, ADR 0014) is the source's own tree.json, read as Read found it,
// and only as a regular file. Offline.
package l0

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTreeJSONReadsTheSourcesOwnTreeJSON(t *testing.T) {
	root, _ := storedSource(t)
	dir := filepath.Join(root, filepath.FromSlash(testSrc.Dir))
	read := func() ([]byte, error) {
		st, err := Read(root, testSrc)
		if err != nil {
			t.Fatal(err)
		}
		return st.TreeJSON(root, testSrc)
	}
	var c *Conflict
	if b, err := read(); !errors.As(err, &c) {
		t.Errorf("no tree.json: %q, %v; want a *Conflict", b, err)
	}
	putFile(t, dir, "tree.json", "{}\n")
	if b, err := read(); err != nil || string(b) != "{}\n" {
		t.Errorf("tree.json: %q, %v; want its bytes", b, err)
	}
	os.Remove(filepath.Join(dir, "tree.json"))
	putFile(t, root, "elsewhere.json", "{}\n")
	if err := os.Symlink(filepath.Join(root, "elsewhere.json"), filepath.Join(dir, "tree.json")); err != nil {
		t.Fatal(err)
	}
	if b, err := read(); !errors.As(err, &c) {
		t.Errorf("tree.json a symlink: %q, %v; want a *Conflict", b, err)
	}
	os.Remove(filepath.Join(dir, "tree.json"))
	if err := os.Mkdir(filepath.Join(dir, "tree.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if b, err := read(); !errors.As(err, &c) {
		t.Errorf("tree.json a directory: %q, %v; want a *Conflict", b, err)
	}
}
