// Tests for the store's placement of a file (R1): a stored file is never
// replaced, only compared. Offline.
package l0

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/meta"
)

// TestALinkThatFindsAFileComparesIt: when the hard link finds a file already
// at the path (one written after StoreOnce read it), that file is compared,
// never replaced: other bytes are a conflict and stay, the same bytes a
// no-op; with nothing there the file is placed.
func TestALinkThatFindsAFileComparesIt(t *testing.T) {
	for _, tc := range []struct {
		name, there string // "" is nothing at the path
		stored      bool
		conflict    bool
	}{
		{name: "other bytes", there: "theirs", conflict: true},
		{name: "the same bytes", there: "ours"},
		{name: "nothing", stored: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tmp, full := filepath.Join(dir, ".fetch-1"), filepath.Join(dir, "f.xml")
			if err := os.WriteFile(tmp, []byte("ours"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.there != "" {
				if err := os.WriteFile(full, []byte(tc.there), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			stored, err := link(tmp, full, "f.xml", []byte("ours"))
			var conflict *Conflict
			if stored != tc.stored || errors.As(err, &conflict) != tc.conflict || (!tc.conflict && err != nil) {
				t.Errorf("stored %v, err %v; want stored %v, conflict %v", stored, err, tc.stored, tc.conflict)
			}
			want := tc.there
			if want == "" {
				want = "ours"
			}
			if b, err := os.ReadFile(full); err != nil || string(b) != want {
				t.Errorf("f.xml holds %q, %v; want %q", b, err, want)
			}
		})
	}
	// A link that fails for another reason (here, no temporary file) is
	// reported as the link failing, not as a file found at the path.
	dir := t.TempDir()
	stored, err := link(filepath.Join(dir, ".fetch-1"), filepath.Join(dir, "f.xml"), "f.xml", []byte("ours"))
	if stored || err == nil || !strings.Contains(err.Error(), "hard link") {
		t.Errorf("stored %v, err %v; want the hard link's failure", stored, err)
	}
}

// TestStoreOnceMatchesTheFileExactly: StoreOnce finds the file it stores by
// exact name among its directory's entries, as RealDirs finds a directory.
// An entry whose name differs from it only in case (a long s too), holding
// the same bytes, other bytes, or being a directory, is a conflict naming
// that entry, and nothing is stored: a case-insensitive filesystem would
// read it as the file (reporting it unchanged, or its bytes as the stored
// ones), and a case-sensitive one would store a second file beside it, while
// git records only the other name.
func TestStoreOnceMatchesTheFileExactly(t *testing.T) {
	for _, tc := range []struct {
		name, p, there, body string // there is "" for a directory
	}{
		{"the same bytes", "d/f.xml", "d/F.XML", "ours"},
		{"other bytes", "d/f.xml", "d/F.xml", "theirs"},
		{"a long s, the same bytes", "d/s.xml", "d/\U0000017F.xml", "ours"},
		{"a directory", "d/meta.yaml", "d/META.YAML", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			full := filepath.Join(root, filepath.FromSlash(tc.there))
			if tc.body == "" {
				if err := os.MkdirAll(full, 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				putFile(t, root, tc.there, tc.body)
			}
			before := snapshot(t, root)
			stored, err := StoreOnce(root, tc.p, []byte("ours"))
			var conflict *Conflict
			if stored || !errors.As(err, &conflict) || !strings.Contains(err.Error(), tc.there+" ") || !strings.Contains(err.Error(), "in another case") {
				t.Errorf("StoreOnce(%s): stored %v, %v; want a conflict naming %s in another case", tc.p, stored, err, tc.there)
			}
			if got := changes(before, snapshot(t, root)); len(got) > 0 {
				t.Errorf("changed %q", got)
			}
		})
	}
}

// TestStoredRefusesAMetaYAMLNamingAnotherSource: Stored refuses a meta.yaml
// whose identity differs from the source's in any field, source and official
// too, whatever read it (meta.Parse knows one source today, so this is
// checked on a State built here).
func TestStoredRefusesAMetaYAMLNamingAnotherSource(t *testing.T) {
	s := Source{Dir: "sources/ecfr/title-12/1024.41", CFR: "12 CFR 1024.41", Tier: meta.TierLaw, Role: meta.RoleInventoried}
	for _, tc := range []struct {
		name string
		set  func(*meta.Meta)
	}{
		{"cfr", func(m *meta.Meta) { m.CFR = "12 CFR 1024.30" }},
		{"tier", func(m *meta.Meta) { m.Tier = meta.TierStandard }},
		{"role", func(m *meta.Meta) { m.Role = meta.RoleCiteOnly }},
		{"source", func(m *meta.Meta) { m.Source = "govinfo" }},
		{"official", func(m *meta.Meta) { m.Official = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := s.Identity()
			tc.set(&m)
			_, err := State{Meta: &m}.Stored(t.TempDir(), s)
			var conflict *Conflict
			if !errors.As(err, &conflict) || !strings.Contains(err.Error(), "another source") {
				t.Errorf("Stored: %v; want a conflict naming another source", err)
			}
		})
	}
}

// TestAFailedReplaceLeavesNoTemporary: when the rename over the target fails
// (here the target is a directory), replace returns the error and leaves no
// temporary file beside it.
func TestAFailedReplaceLeavesNoTemporary(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "meta.yaml", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replace(root, "src/meta.yaml", []byte("cfr: \"x\"\n")); err == nil {
		t.Fatal("replace over a directory succeeded; want an error")
	}
	ents, err := os.ReadDir(filepath.Join(root, "src"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || ents[0].Name() != "meta.yaml" {
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Errorf("src/ holds %q, want meta.yaml only", names)
	}
}
