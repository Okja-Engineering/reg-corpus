// What a reader of a stored source checks (plan step 7, ADR 0015): that its
// meta.yaml names the source (Names, the check Stored makes first), its
// tree.json as Read found it (TreeJSON), and the corpus root (CorpusRoot).
// Offline.
package l0

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/meta"
)

// TestNamesRefusesAMissingOrForeignMetaYAML: Names is a *Conflict when the
// source has no meta.yaml (run regcorpus fetch first) or one that names
// another source, in any identity field; a meta.yaml that names the source
// passes.
func TestNamesRefusesAMissingOrForeignMetaYAML(t *testing.T) {
	var c *Conflict
	if err := (State{}).Names(testSrc); !errors.As(err, &c) ||
		!strings.Contains(err.Error(), testSrc.MetaPath()+" does not exist") || !strings.Contains(err.Error(), "run regcorpus fetch first") {
		t.Errorf("Names with no meta.yaml: %v; want a conflict saying it does not exist and to run regcorpus fetch first", err)
	}
	m := testSrc.Identity()
	if err := (State{Meta: &m}).Names(testSrc); err != nil {
		t.Errorf("Names of its own meta.yaml: %v", err)
	}
	for _, tc := range []struct {
		name string
		set  func(*meta.Meta)
	}{
		{"cfr", func(m *meta.Meta) { m.CFR = "12 CFR 1024.30" }},
		{"tier", func(m *meta.Meta) { m.Tier = meta.TierStandard }},
		{"role", func(m *meta.Meta) { m.Role = meta.RoleInventoried }},
		{"source", func(m *meta.Meta) { m.Source = "govinfo" }},
		{"official", func(m *meta.Meta) { m.Official = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := testSrc.Identity()
			tc.set(&m)
			if err := (State{Meta: &m}).Names(testSrc); !errors.As(err, &c) || !strings.Contains(err.Error(), "another source") {
				t.Errorf("Names: %v; want a conflict naming another source", err)
			}
		})
	}
}

// TestTreeJSONIsTheFileReadFound: TreeJSON gives tree.json's bytes when
// Read found a regular file by that exact name; none (ok false, no error)
// when there is none; a *Conflict when something else is there (a
// directory, a symlink, even to a regular file); and an error that isn't a
// *Conflict when the file can't be read.
func TestTreeJSONIsTheFileReadFound(t *testing.T) {
	treePath := func(root string) string { return filepath.Join(root, filepath.FromSlash(testSrc.TreePath())) }
	read := func(t *testing.T, root string) ([]byte, bool, error) {
		t.Helper()
		st, err := Read(root, testSrc)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		return st.TreeJSON(root, testSrc)
	}
	var c *Conflict

	root, _ := storedSource(t)
	if b, ok, err := read(t, root); b != nil || ok || err != nil {
		t.Errorf("no tree.json: %q, %t, %v; want none, false, no error", b, ok, err)
	}

	putFile(t, root, testSrc.TreePath(), "{}\n")
	if b, ok, err := read(t, root); string(b) != "{}\n" || !ok || err != nil {
		t.Errorf("a regular tree.json: %q, %t, %v; want its bytes", b, ok, err)
	}

	for _, tc := range []struct {
		name string
		make func(t *testing.T, root string)
	}{
		{"a directory", func(t *testing.T, root string) {
			if err := os.Mkdir(treePath(root), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"a symlink to a regular file", func(t *testing.T, root string) {
			putFile(t, root, "elsewhere.json", "{}\n")
			if err := os.Symlink(filepath.Join(root, "elsewhere.json"), treePath(root)); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := storedSource(t)
			tc.make(t, root)
			if b, _, err := read(t, root); !errors.As(err, &c) || !strings.Contains(err.Error(), testSrc.TreePath()) {
				t.Errorf("TreeJSON: %q, %v; want a conflict naming %s", b, err, testSrc.TreePath())
			}
		})
	}

	t.Run("unreadable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a file with no permissions")
		}
		root, _ := storedSource(t)
		putFile(t, root, testSrc.TreePath(), "{}\n")
		if err := os.Chmod(treePath(root), 0); err != nil {
			t.Fatal(err)
		}
		if _, _, err := read(t, root); err == nil || errors.As(err, &c) {
			t.Errorf("TreeJSON: %v; want an error that isn't a conflict", err)
		}
	})
}

// TestCorpusRootNeedsTheSourcesDirectory: a root is the corpus root when the
// directory every source is under is a real directory there; without it
// the error (not a *Conflict) says so and names the command; one reached
// through a symlink is a *Conflict (RealDirs).
func TestCorpusRootNeedsTheSourcesDirectory(t *testing.T) {
	root, _ := storedSource(t)
	if err := CorpusRoot(root, "sources", "text"); err != nil {
		t.Errorf("CorpusRoot: %v", err)
	}
	var c *Conflict
	empty := t.TempDir()
	err := CorpusRoot(empty, "sources/ecfr/title-12", "text")
	if want := empty + " is not a corpus root (no sources/ecfr/title-12/ directory); run text from the corpus root"; err == nil || err.Error() != want || errors.As(err, &c) {
		t.Errorf("CorpusRoot: %v; want %q, not a conflict", err, want)
	}
	linked := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "sources"), filepath.Join(linked, "sources")); err != nil {
		t.Fatal(err)
	}
	if err := CorpusRoot(linked, "sources", "text"); !errors.As(err, &c) {
		t.Errorf("CorpusRoot through a symlink: %v; want a conflict", err)
	}
}
