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
	"time"

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
	return storedBody(t, dir, raw, readRepo(t, raw))
}

// storedBody is stored with body as the raw XML, which meta.yaml's
// sha256_raw then describes.
func storedBody(t *testing.T, dir, raw string, body []byte) (root string, metaYAML []byte) {
	t.Helper()
	root = t.TempDir()
	m, err := meta.Parse(readRepo(t, dir+"/meta.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	m.SHA256Text, m.SHA256Raw = "", meta.SHA256(l0.SHA256Hex(body))
	if metaYAML, err = meta.Marshal(m); err != nil {
		t.Fatal(err)
	}
	put(t, root, raw, body)
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
	if want := (anchors.Written{Wrote: l0.Wrote{Tree: true, Text: true, Meta: true}, SHA256Text: d.SHA256Text}); w != want {
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

// TestWriteLeavesWhatItDoesNotChangeUntouched: a re-run that changes
// nothing writes nothing: each file is the same file (os.SameFile, so not
// replaced by another), with the modification time set on it before the
// run.
func TestWriteLeavesWhatItDoesNotChangeUntouched(t *testing.T) {
	root, _ := stored(t, dir41, raw41)
	if _, err := anchors.Write(root, src41); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	before := map[string]os.FileInfo{}
	for _, f := range []string{tree41, text41, meta41} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		before[f] = info
	}
	if _, err := anchors.Write(root, src41); err != nil {
		t.Fatal(err)
	}
	for f, b := range before {
		a, err := os.Stat(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(a, b) || !a.ModTime().Equal(old) {
			t.Errorf("%s was rewritten (same file %v, modified %v)", f, os.SameFile(a, b), a.ModTime())
		}
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
	if want := (anchors.Written{Wrote: l0.Wrote{Tree: true}, SHA256Text: d.SHA256Text}); err != nil || w != want {
		t.Errorf("stale tree.json: %+v, %v; want %+v", w, err, want)
	}
	put(t, root, text41, []byte("stale\n"))
	w, err = anchors.Write(root, src41)
	if want := (anchors.Written{Wrote: l0.Wrote{Text: true}, SHA256Text: d.SHA256Text}); err != nil || w != want {
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

// TestWriteRefusesWhatTextMDCannotHold: raw XML that parses but that
// text.md can't show as it reads (here a line break inside a <P>) is
// refused like raw XML that doesn't parse: a conflict naming the raw file
// and where the break is, with nothing written.
func TestWriteRefusesWhatTextMDCannotHold(t *testing.T) {
	body := strings.Replace(string(readRepo(t, raw41)), "Complete loss mitigation application.", "Complete loss&#10;mitigation application.", 1)
	if body == string(readRepo(t, raw41)) {
		t.Fatal("the raw XML has no (b)(1) text to change")
	}
	root, before := storedBody(t, dir41, raw41, []byte(body))
	_, err := anchors.Write(root, src41)
	if !isConflict(err) || !strings.Contains(err.Error(), raw41+": anchors: 1024-41-b-1 holds a line break") {
		t.Errorf("err = %v; want a conflict naming %s and 1024-41-b-1", err, raw41)
	}
	unchanged(t, root, dir41, before)
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

// TestStaleNamesWhatIsNotDerived: Derived.Stale is the one comparison of a
// source's stored L0 with what Build derives from its raw XML, which fetch
// --verify and regcorpus text both make (round 1 of PR #11, §1): none on
// the committed 1024.41; otherwise tree.json, text.md and meta.yaml's
// sha256_text, each that differs, by slash path, a missing file among them;
// a tree.json that isn't a regular file is a conflict.
func TestStaleNamesWhatIsNotDerived(t *testing.T) {
	d := build41(t)
	committed := func(t *testing.T) string {
		root := t.TempDir()
		for _, f := range []string{raw41, tree41, text41, meta41} {
			put(t, root, f, readRepo(t, f))
		}
		return root
	}
	stale := func(t *testing.T, root string) ([]string, error) {
		t.Helper()
		st, err := l0.Read(root, src41)
		if err != nil {
			t.Fatal(err)
		}
		return d.Stale(root, src41, st)
	}
	setText := func(t *testing.T, root string, sum meta.SHA256) {
		m, err := meta.Parse(readRepo(t, meta41))
		if err != nil {
			t.Fatal(err)
		}
		m.SHA256Text = sum
		b, err := meta.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		put(t, root, meta41, b)
	}
	for _, tc := range []struct {
		name  string
		spoil func(t *testing.T, root string)
		want  []string
	}{
		{"the committed files", func(*testing.T, string) {}, nil},
		{"tree.json edited", func(t *testing.T, root string) {
			put(t, root, tree41, append(readRepo(t, tree41), ' '))
		}, []string{tree41}},
		{"text.md missing", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(text41))); err != nil {
				t.Fatal(err)
			}
		}, []string{text41}},
		{"sha256_text another", func(t *testing.T, root string) {
			setText(t, root, meta.SHA256(strings.Repeat("0", 64)))
		}, []string{meta41 + "'s sha256_text"}},
		{"no sha256_text", func(t *testing.T, root string) { setText(t, root, "") }, []string{meta41 + "'s sha256_text"}},
		{"all three", func(t *testing.T, root string) {
			put(t, root, tree41, []byte("{}\n"))
			put(t, root, text41, []byte("x\n"))
			setText(t, root, "")
		}, []string{tree41, text41, meta41 + "'s sha256_text"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := committed(t)
			tc.spoil(t, root)
			got, err := stale(t, root)
			if err != nil || strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("Stale: %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	t.Run("a directory where tree.json goes", func(t *testing.T) {
		root := committed(t)
		p := filepath.Join(root, filepath.FromSlash(tree41))
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := stale(t, root); !isConflict(err) {
			t.Errorf("Stale: %v; want a conflict", err)
		}
	})
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
