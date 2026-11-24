// Write: a source's derived files from its stored raw XML, under l0.Update
// (ADR 0010): tree.json and text.md, and sha256_text in meta.yaml, every
// other field kept. Offline, in a temporary corpus root.
package anchors_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
)

var (
	src41 = l0.Source{Dir: dir41, CFR: "12 CFR 1024.41", Tier: meta.TierLaw, Role: meta.RoleInventoried}
	src31 = l0.Source{Dir: dir31, CFR: "12 CFR 1024.31", Tier: meta.TierLaw, Role: meta.RoleCiteOnly}
)

// stored makes a corpus root holding the committed raw XML and meta.yaml of
// the source at dir, its meta.yaml without sha256_text (as main had it
// before plan step 5), and returns the root and that meta.yaml.
func stored(t *testing.T, dir, raw string) (root string, metaYAML []byte) {
	t.Helper()
	root = t.TempDir()
	m, err := meta.Parse(readRepo(t, dir+"/meta.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	m.SHA256Text = ""
	if metaYAML, err = meta.Marshal(m); err != nil {
		t.Fatal(err)
	}
	put(t, root, raw, readRepo(t, raw))
	put(t, root, dir+"/meta.yaml", metaYAML)
	return root, metaYAML
}

func put(t *testing.T, root, slash string, body []byte) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(slash))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, root, slash string) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(slash)))
	if errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b), true
}

// TestWriteDerivesThenChangesNothing: the first run writes tree.json and
// text.md as Build makes them and adds exactly one line to meta.yaml,
// sha256_text, after sha256_raw; a second run changes nothing.
func TestWriteDerivesThenChangesNothing(t *testing.T) {
	root, before := stored(t, dir41, raw41)
	d := build41(t)
	w, err := anchors.Write(root, src41)
	if err != nil {
		t.Fatal(err)
	}
	if want := (anchors.Written{Tree: true, Text: true, Meta: true, SHA256Text: d.SHA256Text}); w != want {
		t.Errorf("first run: %+v, want %+v", w, want)
	}
	if got, _ := get(t, root, tree41); got != string(d.TreeJSON) {
		t.Errorf("tree.json is not Build's")
	}
	if got, _ := get(t, root, text41); got != string(d.TextMD) {
		t.Errorf("text.md is not Build's")
	}
	wantMeta := string(before) + "sha256_text: \"" + string(d.SHA256Text) + "\"\n"
	if got, _ := get(t, root, meta41); got != wantMeta {
		t.Errorf("meta.yaml =\n%s\nwant\n%s", got, wantMeta)
	}

	w, err = anchors.Write(root, src41)
	if err != nil {
		t.Fatal(err)
	}
	if want := (anchors.Written{SHA256Text: d.SHA256Text}); w != want {
		t.Errorf("second run: %+v, want %+v (nothing changed)", w, want)
	}
	if got, _ := get(t, root, meta41); got != wantMeta {
		t.Errorf("second run changed meta.yaml")
	}
}

// TestWriteReplacesOnlyWhatDiffers: a stale tree.json is replaced and an
// up-to-date text.md and meta.yaml are left as they are, and the other way.
func TestWriteReplacesOnlyWhatDiffers(t *testing.T) {
	root, _ := stored(t, dir41, raw41)
	if _, err := anchors.Write(root, src41); err != nil {
		t.Fatal(err)
	}
	d := build41(t)
	put(t, root, tree41, []byte("{}\n"))
	w, err := anchors.Write(root, src41)
	if want := (anchors.Written{Tree: true, SHA256Text: d.SHA256Text}); err != nil || w != want {
		t.Errorf("stale tree.json: %+v, %v; want %+v", w, err, want)
	}
	put(t, root, text41, []byte("stale\n"))
	w, err = anchors.Write(root, src41)
	if want := (anchors.Written{Text: true, SHA256Text: d.SHA256Text}); err != nil || w != want {
		t.Errorf("stale text.md: %+v, %v; want %+v", w, err, want)
	}
	if got, _ := get(t, root, text41); got != string(d.TextMD) {
		t.Errorf("text.md was not replaced")
	}
}

// unchanged checks that nothing was written: no tree.json or text.md, and
// meta.yaml as it was.
func unchanged(t *testing.T, root, dir string, metaYAML []byte) {
	t.Helper()
	for _, name := range []string{"tree.json", "text.md"} {
		if _, ok := get(t, root, dir+"/"+name); ok {
			t.Errorf("%s/%s was written", dir, name)
		}
	}
	if got, _ := get(t, root, dir+"/meta.yaml"); got != string(metaYAML) {
		t.Errorf("%s/meta.yaml changed", dir)
	}
}

func isConflict(err error) bool {
	var c *l0.Conflict
	return errors.As(err, &c)
}

// TestWriteRefusesUnparsableRawXML: raw XML the parser fails is a conflict
// naming the raw file and the parser's location, and nothing is written.
func TestWriteRefusesUnparsableRawXML(t *testing.T) {
	root, before := stored(t, dir31, raw31)
	_, err := anchors.Write(root, src31)
	if !isConflict(err) || !strings.Contains(err.Error(), raw31+": parse: line 4, column 4: unlabeled paragraph") {
		t.Errorf("err = %v; want a conflict naming %s and line 4, column 4: unlabeled paragraph", err, raw31)
	}
	unchanged(t, root, dir31, before)
}

// TestWriteRefusesOverrides: applying overrides.yaml isn't built, so a
// source that holds one is refused, never parsed as if it had none.
func TestWriteRefusesOverrides(t *testing.T) {
	for _, tc := range []struct {
		name string
		mk   func(p string) error
	}{
		{"a file", func(p string) error { return os.WriteFile(p, []byte("[]\n"), 0o644) }},
		{"an empty file", func(p string) error { return os.WriteFile(p, nil, 0o644) }},
		{"a directory", func(p string) error { return os.Mkdir(p, 0o755) }},
		{"a symlink to nothing", func(p string) error { return os.Symlink("nowhere", p) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, before := stored(t, dir41, raw41)
			if err := tc.mk(filepath.Join(root, filepath.FromSlash(dir41+"/overrides.yaml"))); err != nil {
				t.Fatal(err)
			}
			_, err := anchors.Write(root, src41)
			if !isConflict(err) || !strings.Contains(err.Error(), dir41+"/overrides.yaml") {
				t.Errorf("err = %v; want a conflict naming overrides.yaml", err)
			}
			unchanged(t, root, dir41, before)
		})
	}
}

// TestWriteRefusesWhatIsNotARegularFile: a symlink or a directory where
// tree.json or text.md goes is a conflict, and neither file is written.
func TestWriteRefusesWhatIsNotARegularFile(t *testing.T) {
	for _, name := range []string{"tree.json", "text.md"} {
		for kind, mk := range map[string]func(p string) error{
			"a symlink":   func(p string) error { return os.Symlink("elsewhere.json", p) },
			"a directory": func(p string) error { return os.Mkdir(p, 0o755) },
		} {
			t.Run(kind+" at "+name, func(t *testing.T) {
				root, before := stored(t, dir41, raw41)
				p := filepath.Join(root, filepath.FromSlash(dir41+"/"+name))
				if err := mk(p); err != nil {
					t.Fatal(err)
				}
				_, err := anchors.Write(root, src41)
				if !isConflict(err) || !strings.Contains(err.Error(), dir41+"/"+name) {
					t.Errorf("err = %v; want a conflict naming %s", err, name)
				}
				other := map[string]string{"tree.json": "text.md", "text.md": "tree.json"}[name]
				if _, ok := get(t, root, dir41+"/"+other); ok {
					t.Errorf("%s was written", other)
				}
				if got, _ := get(t, root, meta41); got != string(before) {
					t.Errorf("meta.yaml changed")
				}
			})
		}
	}
}

// TestWriteMakesTheStoredStateCheck: Write goes through l0.Update, so a
// meta.yaml whose sha256_raw is not the raw file's is refused first.
func TestWriteMakesTheStoredStateCheck(t *testing.T) {
	root, before := stored(t, dir41, raw41)
	bad := strings.Replace(string(before), "sha256_raw: \"3", "sha256_raw: \"4", 1)
	put(t, root, meta41, []byte(bad))
	_, err := anchors.Write(root, src41)
	if !isConflict(err) || !strings.Contains(err.Error(), "sha256_raw") {
		t.Errorf("err = %v; want the stored-state conflict", err)
	}
	unchanged(t, root, dir41, []byte(bad))
}
