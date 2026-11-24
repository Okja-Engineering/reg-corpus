// Update as the one writer of a source's files (ADRs 0010, 0013): it writes
// only that source's tree.json, text.md and meta.yaml, under its lock, from
// what set returns; it refuses before writing anything, and reports what it
// replaced when a write fails part-way. Offline.
package l0

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/meta"
)

var testSrc = Source{Dir: "sources/s", CFR: "12 CFR 0.1", Tier: meta.TierLaw, Role: meta.RoleCiteOnly}

const testRaw = "<DIV8/>\n"

// storedSource makes a corpus root holding testSrc's raw XML and a meta.yaml
// that describes it, and returns the root and that meta.yaml.
func storedSource(t *testing.T) (string, []byte) {
	t.Helper()
	root := t.TempDir()
	return root, storeAt(t, filepath.Join(root, filepath.FromSlash(testSrc.Dir)))
}

// storeAt puts testSrc's raw XML and its meta.yaml in the directory full.
func storeAt(t *testing.T, full string) []byte {
	t.Helper()
	m := testSrc.Identity()
	m.SourceVersionDate, m.RetrievedAt, m.ECFRUpToDateAsOf = "2025-01-02", "2025-01-03", "2025-01-03"
	m.SHA256Raw = meta.SHA256(SHA256Hex([]byte(testRaw)))
	b, err := meta.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, full, "raw/"+string(m.SourceVersionDate)+".xml", testRaw)
	putFile(t, full, "meta.yaml", string(b))
	return b
}

func putFile(t *testing.T, root, slash, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(slash))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// snapshot is every entry under root, by slash path, with a file's bytes
// and a symlink's target.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		switch {
		case d.IsDir():
			out[filepath.ToSlash(rel)+"/"] = ""
			return nil
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			out[filepath.ToSlash(rel)] = "-> " + target
			return err
		}
		b, err := os.ReadFile(p)
		out[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// changes lists the paths whose entries differ between two snapshots.
func changes(before, after map[string]string) []string {
	var out []string
	for p, v := range after {
		if w, ok := before[p]; !ok || w != v {
			out = append(out, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			out = append(out, p)
		}
	}
	return out
}

// derived is a set that returns tree and text as the derived files.
func derived(tree, text string) func(*meta.Meta, []byte) (*Derived, error) {
	return derivedSum(tree, text, "0000000000000000000000000000000000000000000000000000000000000000")
}

// derivedSum is derived, setting sha256_text to sum.
func derivedSum(tree, text string, sum meta.SHA256) func(*meta.Meta, []byte) (*Derived, error) {
	return func(m *meta.Meta, _ []byte) (*Derived, error) {
		m.SHA256Text = sum
		return &Derived{Tree: []byte(tree), Text: []byte(text)}, nil
	}
}

// TestUpdateWritesOnlyTheSourcesOwnFiles: the derived files set returns
// land at the source's tree.json and text.md, meta.yaml is what
// meta.Marshal renders, and nothing else under the root changes; a second
// run with the same bytes replaces nothing, and one with other bytes only
// what differs.
func TestUpdateWritesOnlyTheSourcesOwnFiles(t *testing.T) {
	root, _ := storedSource(t)
	before := snapshot(t, root)
	b, w, err := Update(root, testSrc, derived("T1", "X1"))
	if err != nil || w != (Wrote{Tree: true, Text: true, Meta: true}) {
		t.Fatalf("first run: %+v, %v; want all three written", w, err)
	}
	got := changes(before, snapshot(t, root))
	want := map[string]bool{testSrc.TreePath(): true, testSrc.TextPath(): true, testSrc.MetaPath(): true}
	if len(got) != len(want) {
		t.Errorf("changed %q; want only %s's tree.json, text.md and meta.yaml", got, testSrc.Dir)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("%s changed", p)
		}
	}
	after := snapshot(t, root)
	if after[testSrc.TreePath()] != "T1" || after[testSrc.TextPath()] != "X1" || after[testSrc.MetaPath()] != string(b) {
		t.Errorf("files hold %q, %q, %q", after[testSrc.TreePath()], after[testSrc.TextPath()], after[testSrc.MetaPath()])
	}
	if _, err := meta.Parse(b); err != nil {
		t.Errorf("meta.yaml is not what meta.Marshal renders: %v", err)
	}
	for _, step := range []struct {
		tree, text string
		want       Wrote
	}{
		{"T1", "X1", Wrote{}},
		{"T1", "X2", Wrote{Text: true}},
		{"T2", "X2", Wrote{Tree: true}},
	} {
		if _, w, err := Update(root, testSrc, derived(step.tree, step.text)); err != nil || w != step.want {
			t.Errorf("%s, %s: %+v, %v; want %+v", step.tree, step.text, w, err, step.want)
		}
	}
	if m, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(testSrc.Dir), TempPattern)); len(m) > 0 {
		t.Errorf("temporaries left: %v", m)
	}
}

// TestUpdateRefusesADirectoryOutsideTheRoot: a source directory with a "."
// or ".." element, an empty one or a leading or trailing slash names no
// directory below the root, and Update refuses it, writing nothing
// anywhere, even where that path leads to a stored source it would accept.
func TestUpdateRefusesADirectoryOutsideTheRoot(t *testing.T) {
	for _, dir := range []string{"../escape", "sources/../../escape", "/abs", "sources//s", "sources/s/", "./sources/s", ".", ""} {
		t.Run(dir, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "root")
			storeAt(t, filepath.Join(root, filepath.FromSlash(dir)))
			before := snapshot(t, parent)
			s := testSrc
			s.Dir = dir
			if _, _, err := Update(root, s, derived("T", "X")); err == nil {
				t.Errorf("Update with Dir %q: nil error", dir)
			}
			if got := changes(before, snapshot(t, parent)); len(got) > 0 {
				t.Errorf("changed %q", got)
			}
		})
	}
}

// TestUpdateRefusesBeforeWritingAnything: something other than a regular
// file at tree.json or text.md is a *Conflict, and neither the other
// derived file nor meta.yaml is written.
func TestUpdateRefusesBeforeWritingAnything(t *testing.T) {
	for _, name := range []string{"tree.json", "text.md"} {
		for kind, mk := range map[string]func(p string) error{
			"a symlink":   func(p string) error { return os.Symlink("elsewhere", p) },
			"a directory": func(p string) error { return os.Mkdir(p, 0o755) },
		} {
			t.Run(kind+" at "+name, func(t *testing.T) {
				root, _ := storedSource(t)
				if err := mk(filepath.Join(root, filepath.FromSlash(testSrc.Dir), name)); err != nil {
					t.Fatal(err)
				}
				before := snapshot(t, root)
				_, w, err := Update(root, testSrc, derived("T", "X"))
				var c *Conflict
				if !errors.As(err, &c) || w != (Wrote{}) || !strings.Contains(err.Error(), testSrc.Dir+"/"+name) {
					t.Errorf("%+v, %v; want a conflict naming %s, nothing written", w, err, name)
				}
				if got := changes(before, snapshot(t, root)); len(got) > 0 {
					t.Errorf("changed %q", got)
				}
			})
		}
	}
}

// TestReadMatchesNamesExactly: a name that differs from meta.yaml,
// tree.json, text.md or overrides.yaml only in case is refused, by its
// name as the directory lists it, whatever it is (a file, a directory, a
// symlink), on a case-insensitive filesystem (where it would be read and
// written as that file) and a case-sensitive one alike; and Update then
// writes nothing. Case is Unicode's: APFS folds "ſ" (U+017F) to "s", so
// "tree.jſon" is tree.json there.
func TestReadMatchesNamesExactly(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func(t *testing.T, root, p string, metaYAML []byte)
	}{
		{"TREE.JSON", file},
		{"Text.md", file},
		{"Overrides.YAML", file},
		{"META.YAML", nil},
		{"tree.j\U0000017Fon", file},
		{"Tree.json", func(t *testing.T, _, p string, _ []byte) {
			if err := os.Mkdir(p, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"TEXT.MD", func(t *testing.T, _, p string, _ []byte) {
			if err := os.Symlink("elsewhere", p); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		name := tc.name
		t.Run(name, func(t *testing.T) {
			root, metaYAML := storedSource(t)
			p := filepath.Join(root, filepath.FromSlash(testSrc.Dir), name)
			if tc.make == nil {
				if err := os.Rename(filepath.Join(root, filepath.FromSlash(testSrc.MetaPath())), p); err != nil {
					t.Fatal(err)
				}
			} else {
				tc.make(t, root, p, metaYAML)
			}
			before := snapshot(t, root)
			_, err := Read(root, testSrc)
			var c *Conflict
			if !errors.As(err, &c) || !strings.Contains(err.Error(), testSrc.Dir+"/"+name) {
				t.Errorf("Read: %v; want a conflict naming %s", err, name)
			}
			if _, _, err := Update(root, testSrc, derived("T", "X")); !errors.As(err, &c) {
				t.Errorf("Update: %v; want a conflict", err)
			}
			if got := changes(before, snapshot(t, root)); len(got) > 0 {
				t.Errorf("changed %q", got)
			}
		})
	}
}

// file writes metaYAML's bytes at p, a regular file.
func file(t *testing.T, _, p string, metaYAML []byte) {
	t.Helper()
	if err := os.WriteFile(p, metaYAML, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRealDirsMatchesNamesExactly: every directory on the way to a source's
// files is matched by exact name too, sources/ and the source's directory
// and raw/ alike. One that differs from its name only in case, renamed so
// on a case-insensitive filesystem and a case-sensitive one alike, or an
// empty one made where raw/ is missing, is refused (a *Conflict naming it
// as the directory lists it) by RealDirs, Read, Update and StoreOnce, and
// nothing is written: a case-insensitive filesystem would reach it by the
// name git doesn't record.
func TestRealDirsMatchesNamesExactly(t *testing.T) {
	for _, tc := range []struct {
		name, from, to string // rename from to to, slash paths under root
		mkdir          bool   // remove from, then make to empty
	}{
		{"SOURCES", "sources", "SOURCES", false},
		{"long s", "sources", "\U0000017Fources", false},
		{"the source's directory", "sources/s", "sources/S", false},
		{"RAW", "sources/s/raw", "sources/s/RAW", false},
		{"an empty RAW", "sources/s/raw", "sources/s/RAW", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := storedSource(t)
			from, to := filepath.Join(root, filepath.FromSlash(tc.from)), filepath.Join(root, filepath.FromSlash(tc.to))
			if tc.mkdir {
				if err := os.RemoveAll(from); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(to, 0o755); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Rename(from, to); err != nil {
				t.Fatal(err)
			}
			before := snapshot(t, root)
			conflict := func(what string, err error) {
				t.Helper()
				var c *Conflict
				if !errors.As(err, &c) || !strings.Contains(err.Error(), tc.to+" ") {
					t.Errorf("%s: %v; want a conflict naming %s", what, err, tc.to)
				}
			}
			conflict("RealDirs", RealDirs(root, testSrc.RawDir()))
			_, err := Read(root, testSrc)
			conflict("Read", err)
			_, w, err := Update(root, testSrc, derived("T", "X"))
			conflict("Update", err)
			if w != (Wrote{}) {
				t.Errorf("Update wrote %+v", w)
			}
			_, err = StoreOnce(root, testSrc.RawPath("2025-01-09"), []byte(testRaw))
			conflict("StoreOnce", err)
			if got := changes(before, snapshot(t, root)); len(got) > 0 {
				t.Errorf("changed %q", got)
			}
		})
	}
}

// TestUpdateRefusesADerivedFileItCannotRead: a derived file Update can't
// read can't be compared, so it is an error (exit 2), not a conflict and
// never read as differing, and nothing is written, not even the other
// derived file that does differ.
func TestUpdateRefusesADerivedFileItCannotRead(t *testing.T) {
	root, _ := storedSource(t)
	if _, _, err := Update(root, testSrc, derived("T1", "X1")); err != nil {
		t.Fatal(err)
	}
	text := filepath.Join(root, filepath.FromSlash(testSrc.TextPath()))
	if err := os.Chmod(text, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(text, 0o644) })
	if _, err := os.ReadFile(text); err == nil {
		t.Skip("text.md is readable at mode 0 (running as root?)")
	}
	_, w, err := Update(root, testSrc, derivedSum("T2", "X2", "1111111111111111111111111111111111111111111111111111111111111111"))
	var c *Conflict
	if err == nil || errors.As(err, &c) || w != (Wrote{}) {
		t.Errorf("%+v, %v; want an error that is not a conflict, nothing written", w, err)
	}
	if err := os.Chmod(text, 0o644); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{testSrc.TreePath(): "T1", testSrc.TextPath(): "X1"} {
		if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p))); err != nil || string(b) != want {
			t.Errorf("%s holds %q, %v; want %q, unchanged", p, b, err, want)
		}
	}
}

// TestWriteErrorsNameTheTarget: when writing a file fails, the error names
// the file it was writing, by its slash path from the root, not only the
// temporary file beside it: replace's (Update's) and StoreOnce's alike. A
// directory that can't be written (no temporary file can be made) and a
// directory where the file goes (the rename fails) both fail.
func TestWriteErrorsNameTheTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "meta.yaml", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replace(root, "src/meta.yaml", []byte("x\n")); err == nil || !strings.Contains(err.Error(), "src/meta.yaml") {
		t.Errorf("replace over a directory: %v; want an error naming src/meta.yaml", err)
	}
	ro := filepath.Join(root, "ro")
	if err := os.Mkdir(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ro, 0o755) })
	if f, err := os.CreateTemp(ro, "probe"); err == nil {
		f.Close()
		t.Skip("a mode 0555 directory is writable (running as root?)")
	}
	if err := replace(root, "ro/tree.json", []byte("T")); err == nil || !strings.Contains(err.Error(), "ro/tree.json") {
		t.Errorf("replace in a read-only directory: %v; want an error naming ro/tree.json", err)
	}
	if _, err := StoreOnce(root, "ro/raw.xml", []byte("R")); err == nil || !strings.Contains(err.Error(), "ro/raw.xml") {
		t.Errorf("StoreOnce in a read-only directory: %v; want an error naming ro/raw.xml", err)
	}
}

// TestReplaceIsNeverAConflict: replace is called only once Update has
// checked everything, so when the way to a file is no longer a real
// directory its error is not a *Conflict, which would say nothing was
// written.
func TestReplaceIsNeverAConflict(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(root, "s")); err != nil {
		t.Fatal(err)
	}
	err := replace(root, "s/tree.json", []byte("T"))
	var c *Conflict
	if err == nil || errors.As(err, &c) {
		t.Errorf("replace through a symlinked directory: %v; want an error that is not a conflict", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "real", "tree.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("tree.json was written through the symlink")
	}
}
