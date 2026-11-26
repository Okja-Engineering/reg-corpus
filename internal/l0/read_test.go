// What a reader of a stored source checks too (plan step 7, ADR 0015): that
// its meta.yaml names the source (Names, the check Stored makes first) and
// the corpus root (CorpusRoot). Offline.
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

// TestCorpusRootNeedsTheSourcesDirectory: a root is the corpus root when the
// directory every source is under is a real directory there; without it
// the error, ErrNotRoot (errors.Is) and not a *Conflict, says so and names
// the command; one reached through a symlink is a *Conflict (RealDirs).
func TestCorpusRootNeedsTheSourcesDirectory(t *testing.T) {
	root, _ := storedSource(t)
	if err := CorpusRoot(root, "sources", "text"); err != nil {
		t.Errorf("CorpusRoot: %v", err)
	}
	var c *Conflict
	empty := t.TempDir()
	err := CorpusRoot(empty, "sources/ecfr/title-12", "text")
	if want := empty + " is not a corpus root (no sources/ecfr/title-12/ directory); run text from the corpus root"; err == nil || err.Error() != want || errors.As(err, &c) || !errors.Is(err, ErrNotRoot) {
		t.Errorf("CorpusRoot: %v; want %q, ErrNotRoot (errors.Is), not a conflict", err, want)
	}
	linked := t.TempDir()
	if err := os.Symlink(filepath.Join(root, "sources"), filepath.Join(linked, "sources")); err != nil {
		t.Fatal(err)
	}
	if err := CorpusRoot(linked, "sources", "text"); !errors.As(err, &c) || errors.Is(err, ErrNotRoot) {
		t.Errorf("CorpusRoot through a symlink: %v; want a conflict, not ErrNotRoot", err)
	}
}
