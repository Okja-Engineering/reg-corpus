// Tests for Derive (plan step 5): a source's derived files are replaced
// whole when their bytes differ, left alone when they don't, and refused,
// all of them before any is written, when something other than a regular
// file is where one goes. Offline.
package l0

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDerive(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "s"), 0o755); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(root, "s", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	files := func(a, b string) []File {
		return []File{{"s/tree.json", []byte(a)}, {"s/text.md", []byte(b)}}
	}
	for _, step := range []struct {
		name    string
		a, b    string
		changed []bool
	}{
		{"none there: both written", "A1", "B1", []bool{true, true}},
		{"the same bytes: neither", "A1", "B1", []bool{false, false}},
		{"one differs: that one", "A1", "B2", []bool{false, true}},
		{"both differ: both", "A2", "B3", []bool{true, true}},
	} {
		changed, err := Derive(root, files(step.a, step.b))
		if err != nil || !reflect.DeepEqual(changed, step.changed) {
			t.Fatalf("%s: changed %v, err %v; want %v", step.name, changed, err, step.changed)
		}
		if read("tree.json") != step.a || read("text.md") != step.b {
			t.Fatalf("%s: files hold %q and %q", step.name, read("tree.json"), read("text.md"))
		}
		if m, _ := filepath.Glob(filepath.Join(root, "s", TempPattern)); len(m) > 0 {
			t.Fatalf("%s: temporaries left: %v", step.name, m)
		}
	}
	if info, err := os.Stat(filepath.Join(root, "s", "tree.json")); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("tree.json mode %v, %v; want 0644", info.Mode(), err)
	}
}

func TestDeriveRefusesAllBeforeWritingAny(t *testing.T) {
	for _, tc := range []struct {
		name string
		mk   func(p string) error
	}{
		{"a symlink", func(p string) error { return os.Symlink("elsewhere", p) }},
		{"a directory", func(p string) error { return os.Mkdir(p, 0o755) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "s"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := tc.mk(filepath.Join(root, "s", "text.md")); err != nil {
				t.Fatal(err)
			}
			changed, err := Derive(root, []File{{"s/tree.json", []byte("A")}, {"s/text.md", []byte("B")}})
			var c *Conflict
			if !errors.As(err, &c) || changed != nil {
				t.Errorf("changed %v, err %v; want a conflict", changed, err)
			}
			if _, err := os.Lstat(filepath.Join(root, "s", "tree.json")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("tree.json was written before text.md was refused")
			}
		})
	}
	// A symlink on the way is refused as everywhere in L0 (RealDirs).
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(root, "s")); err != nil {
		t.Fatal(err)
	}
	_, err := Derive(root, []File{{"s/tree.json", []byte("A")}})
	var c *Conflict
	if !errors.As(err, &c) {
		t.Errorf("err %v; want a conflict for the symlinked directory", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "real", "tree.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("tree.json was written through the symlink")
	}
}
