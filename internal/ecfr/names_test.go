// Exact names (R1; ADR 0013): fetch, fetch --date, meta set and parse read
// and write only by the names git records. A directory on the way, a
// source's meta.yaml, tree.json, text.md or overrides.yaml, its raw file,
// or the file fetch --date stores under .cache/, whose name differs only in
// case from the one it stands for is refused (exit 1), alone or beside the
// exact name, on a case-insensitive filesystem, where it would be read and
// written by the other name, and on a case-sensitive one alike; a directory
// on the way that can't be listed is an error (exit 2); and each command's
// -h says so.
package ecfr

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryCommandFindsEachNameExactly crosses every name each command
// reaches (each directory on the way and each file, the raw file and
// --date's cached file included) with three conditions: a variant of the
// name alone (the entry renamed, or made if the command finds none there),
// a variant beside the exact name, and the directory that holds the name
// unlistable (mode 0311: it can be passed through and written, not read).
// From a root where the command exits 0 (checked first), a variant exits 1
// naming it as its directory lists it, an unlistable directory exits 2, and
// either way nothing is printed and no file changes.
//
// A variant is the name in capitals, and, when it holds an s, with a long s
// (U+017F, which simple case folding and APFS both fold to s) for its last
// s. A name with no letter (1024.41) has none, and a case-insensitive
// filesystem can't hold a variant beside the exact name: those cells are
// skipped, saying why, and TestExactlyFindsOnlyTheExactName (internal/l0)
// checks the beside case on synthetic entries on every platform.
func TestEveryCommandFindsEachNameExactly(t *testing.T) {
	const (
		src   = "sources/ecfr/title-12/1024.41"
		cache = ".cache/ecfr/title-12/1024.41"
	)
	way := []string{"sources", "sources/ecfr", "sources/ecfr/title-12"}
	source := append(append([]string{}, way...), src, src+"/raw", src+"/meta.yaml", src+"/tree.json",
		src+"/text.md", src+"/overrides.yaml", src+"/raw/2025-07-15.xml")
	cached := append(append([]string{}, way...), ".cache", ".cache/ecfr", ".cache/ecfr/title-12", cache,
		cache+"/2025-05-15.xml")
	files := map[string]bool{}
	for _, p := range []string{src + "/meta.yaml", src + "/tree.json", src + "/text.md", src + "/overrides.yaml",
		src + "/raw/2025-07-15.xml", cache + "/2025-05-15.xml"} {
		files[p] = true
	}
	type command struct {
		name    string
		run     func(t *testing.T, root string) (int, string, string)
		reaches []string
	}
	fetch := func(args ...string) func(t *testing.T, root string) (int, string, string) {
		return func(t *testing.T, root string) (int, string, string) {
			_, c := newFake(t)
			return fetchAt(t, root, c, "2026-11-22", args...)
		}
	}
	commands := []command{
		{"fetch", fetch("1024.41"), source},
		{"fetch --date", fetch("--date", "2025-05-15", "1024.41"), cached},
		{"meta set", func(t *testing.T, root string) (int, string, string) {
			return metaAt(t, root, "set", "1024.41", "version_effective_date", "2025-07-15")
		}, source},
		{"parse", func(t *testing.T, root string) (int, string, string) { return parseAt(t, root, "1024.41") }, source},
	}
	// fetched is a root where 1024.41 is fetched, and its point in time
	// under .cache/, so every name a command reaches but tree.json, text.md
	// and overrides.yaml is there.
	fetched := func(t *testing.T) string {
		t.Helper()
		root := corpusRoot(t)
		for _, args := range [][]string{{"1024.41"}, {"--date", "2025-05-15", "1024.41"}} {
			if code, _, stderr := fetch(args...)(t, root); code != 0 {
				t.Fatalf("fetch %q: exit %d, %s", args, code, stderr)
			}
		}
		return root
	}
	full := func(root, p string) string { return filepath.Join(root, filepath.FromSlash(p)) }
	// check runs cmd and wants code, nothing printed, naming named if not
	// "", and no file changed: before is the tree once set up; restore
	// undoes a mode change first.
	check := func(t *testing.T, cmd command, root string, code int, named string, restore func()) {
		t.Helper()
		before := tree(t, root)
		got, stdout, stderr := cmd.run(t, root)
		restore()
		if got != code || stdout != "" || !strings.Contains(stderr, named) {
			t.Errorf("exit %d, stdout %q, stderr %q; want %d naming %q", got, stdout, stderr, code, named)
		}
		if !equalTrees(before, tree(t, root)) {
			t.Errorf("files changed: before %v, after %v", keysOf(before), keysOf(tree(t, root)))
		}
	}
	for _, cmd := range commands {
		t.Run(cmd.name+", nothing changed", func(t *testing.T) {
			root := fetched(t)
			if code, _, stderr := cmd.run(t, root); code != 0 {
				t.Fatalf("exit %d, %s; every cell below starts from this root and wants 1 or 2", code, stderr)
			}
		})
		unlistable := map[string]bool{}
		for _, p := range cmd.reaches {
			dir, base := path.Dir(p), path.Base(p)
			var variants []string
			if v := strings.ToUpper(base); v != base {
				variants = append(variants, v)
			}
			if i := strings.LastIndex(base, "s"); i >= 0 {
				variants = append(variants, base[:i]+"\U0000017F"+base[i+1:])
			}
			if len(variants) == 0 {
				t.Run(cmd.name+", "+p+" in another case", func(t *testing.T) {
					t.Skipf("%s holds no letter, so no name differs from it only in case", base)
				})
			}
			for _, v := range variants {
				vp := path.Join(dir, v)
				t.Run(cmd.name+", "+vp+" alone", func(t *testing.T) {
					root := fetched(t)
					if _, err := os.Lstat(full(root, p)); err == nil {
						if err := os.Rename(full(root, p), full(root, vp)); err != nil {
							t.Fatal(err)
						}
					} else {
						writeFile(t, root, vp, "x")
					}
					check(t, cmd, root, 1, vp, func() {})
				})
				t.Run(cmd.name+", "+vp+" beside "+base, func(t *testing.T) {
					root := fetched(t)
					if _, err := os.Lstat(full(root, p)); err != nil {
						writeFile(t, root, p, "x")
					}
					var err error
					if files[p] {
						var f *os.File
						if f, err = os.OpenFile(full(root, vp), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644); err == nil {
							err = f.Close()
						}
					} else {
						err = os.Mkdir(full(root, vp), 0o755)
					}
					switch {
					case errors.Is(err, fs.ErrExist):
						t.Skipf("this filesystem is case-insensitive: %s is %s, so it can't be beside it; TestExactlyFindsOnlyTheExactName checks this case on synthetic entries", vp, p)
					case err != nil:
						t.Fatal(err)
					}
					check(t, cmd, root, 1, vp, func() {})
				})
			}
			if unlistable[dir] {
				continue
			}
			unlistable[dir] = true
			t.Run(cmd.name+", "+dir+"/ unlistable", func(t *testing.T) {
				if os.Geteuid() == 0 {
					t.Skip("running with privileges that list a mode-0311 directory")
				}
				root := fetched(t)
				d := full(root, dir)
				info, err := os.Stat(d)
				if err != nil {
					t.Fatal(err)
				}
				before := tree(t, root)
				if err := os.Chmod(d, 0o311); err != nil {
					t.Fatal(err)
				}
				restore := func() { os.Chmod(d, info.Mode().Perm()) }
				t.Cleanup(restore)
				got, stdout, stderr := cmd.run(t, root)
				restore()
				if got != 2 || stdout != "" {
					t.Errorf("exit %d, stdout %q, stderr %q; want 2, nothing printed", got, stdout, stderr)
				}
				if !equalTrees(before, tree(t, root)) {
					t.Errorf("files changed: before %v, after %v", keysOf(before), keysOf(tree(t, root)))
				}
			})
		}
	}
}

// TestUsageNamesTheCaseRefusal: each command's -h lists, under exit 1, a
// name that differs only in case from a source's file's or from a
// directory's on the way, and fetch's from the file it stores.
func TestUsageNamesTheCaseRefusal(t *testing.T) {
	_, c := newFake(t)
	for name, run := range map[string]func() (int, string, string){
		"fetch":    func() (int, string, string) { return fetchAt(t, corpusRoot(t), c, "2026-11-22", "-h") },
		"meta set": func() (int, string, string) { return metaAt(t, t.TempDir(), "set", "-h") },
		"parse":    func() (int, string, string) { return parseAt(t, t.TempDir(), "-h") },
	} {
		code, stdout, _ := run()
		_, exit1, _ := strings.Cut(stdout, "Exit 1")
		exit1, _, _ = strings.Cut(exit1, "Exit 2")
		exit1 = strings.Join(strings.Fields(exit1), " ")
		wants := []string{"differs only in case", "meta.yaml, tree.json, text.md or overrides.yaml", "a directory on the way"}
		if name == "fetch" {
			// fetch stores a file too, --date's under .cache/ among them.
			wants = append(wants, "from the file it stores")
		}
		for _, want := range wants {
			if code != 0 || !strings.Contains(exit1, want) {
				t.Errorf("%s -h: exit %d; its Exit 1 list lacks %q:\n%s", name, code, want, exit1)
			}
		}
	}
}
