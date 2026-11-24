// Exact names (R1; ADR 0013): fetch, meta set and parse read and write a
// source only by the names git records. A directory on the way, or a
// source's meta.yaml, tree.json, text.md or overrides.yaml, whose name
// differs only in case from the one it stands for is refused (exit 1), on
// a case-insensitive filesystem, where it would be read and written by the
// other name, and on a case-sensitive one alike; and each command's -h
// says so.
package ecfr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommandsRefuseANameInAnotherCase: from a root where 1024.41 is
// fetched (so fetch, meta set and parse would each exit 0), a name made to
// differ from its own only in case exits 1, naming it as the directory
// lists it, and nothing changes.
func TestCommandsRefuseANameInAnotherCase(t *testing.T) {
	const dir = "sources/ecfr/title-12/1024.41"
	rename := func(from, to string) func(t *testing.T, root string) {
		return func(t *testing.T, root string) {
			if err := os.Rename(filepath.Join(root, filepath.FromSlash(from)), filepath.Join(root, filepath.FromSlash(to))); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, root string)
		named string
	}{
		{"SOURCES", rename("sources", "SOURCES"), "SOURCES"},
		{"sources with a long s", rename("sources", "\U0000017Fources"), "\U0000017Fources"},
		{"TITLE-12", rename("sources/ecfr/title-12", "sources/ecfr/TITLE-12"), "sources/ecfr/TITLE-12"},
		{"RAW", rename(dir+"/raw", dir+"/RAW"), dir + "/RAW"},
		{"an empty RAW, nothing fetched", func(t *testing.T, root string) {
			for _, p := range []string{dir + "/raw", dir + "/meta.yaml"} {
				if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(p))); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(filepath.Join(root, filepath.FromSlash(dir+"/RAW")), 0o755); err != nil {
				t.Fatal(err)
			}
		}, dir + "/RAW"},
		{"META.YAML", rename(dir+"/meta.yaml", dir+"/META.YAML"), dir + "/META.YAML"},
		{"Tree.Json", func(t *testing.T, root string) { writeFile(t, root, dir+"/Tree.Json", "{}\n") }, dir + "/Tree.Json"},
	} {
		for _, cmd := range []struct {
			name string
			run  func(t *testing.T, root string) (int, string, string)
		}{
			{"fetch", func(t *testing.T, root string) (int, string, string) {
				_, c := newFake(t)
				return fetchAt(t, root, c, "2026-11-22", "1024.41")
			}},
			{"meta set", func(t *testing.T, root string) (int, string, string) {
				return metaAt(t, root, "set", "1024.41", "version_effective_date", "2025-07-15")
			}},
			{"parse", func(t *testing.T, root string) (int, string, string) { return parseAt(t, root, "1024.41") }},
		} {
			t.Run(cmd.name+", "+tc.name, func(t *testing.T) {
				root := corpusRoot(t)
				_, c := newFake(t)
				if code, _, stderr := fetchAt(t, root, c, "2026-11-22", "1024.41"); code != 0 {
					t.Fatalf("fetch: exit %d, %s", code, stderr)
				}
				tc.setup(t, root)
				before := tree(t, root)
				code, stdout, stderr := cmd.run(t, root)
				if code != 1 || stdout != "" || !strings.Contains(stderr, tc.named+" ") || !strings.Contains(stderr, "in another case") {
					t.Errorf("exit %d, stdout %q, stderr %q; want 1 naming %s in another case", code, stdout, stderr, tc.named)
				}
				if !equalTrees(before, tree(t, root)) {
					t.Errorf("files changed: before %v, after %v", keysOf(before), keysOf(tree(t, root)))
				}
			})
		}
	}
}

// TestFetchMatchesTheFileItStoresExactly: what fetch reports storing, or
// finding unchanged, is the name git records. A file beside the one fetch
// stores, named for it in another case and holding eCFR's bytes, exits 1
// naming it, with nothing printed or changed, under .cache/ (--date) and in
// raw/ alike; never "unchanged" against a file git records as another. The
// -h check is in TestUsageNamesTheCaseRefusal.
func TestFetchMatchesTheFileItStoresExactly(t *testing.T) {
	const cache, raw = ".cache/ecfr/title-12/1024.41/", "sources/ecfr/title-12/1024.41/raw/"
	for _, tc := range []struct {
		name, exact, there string // the file fetch stores, and its variant
		args               []string
	}{
		{"--date, .XML", cache + "2025-05-15.xml", cache + "2025-05-15.XML", []string{"--date", "2025-05-15", "1024.41"}},
		{"--date, .Xml", cache + "2025-05-15.xml", cache + "2025-05-15.Xml", []string{"--date", "2025-05-15", "1024.41"}},
		{"latest, .XML", raw + "2025-07-15.xml", raw + "2025-07-15.XML", []string{"1024.41"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := corpusRoot(t)
			_, c := newFake(t)
			// Fetch the file under its own name, then rename it.
			if code, _, stderr := fetchAt(t, root, c, "2026-11-22", tc.args...); code != 0 {
				t.Fatalf("fetch: exit %d, %s", code, stderr)
			}
			if err := os.Rename(filepath.Join(root, filepath.FromSlash(tc.exact)), filepath.Join(root, filepath.FromSlash(tc.there))); err != nil {
				t.Fatal(err)
			}
			before := tree(t, root)
			code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", tc.args...)
			// In raw/, the variant is also an entry besides the latest
			// version's file, which fetch refuses as such (admits).
			if code != 1 || stdout != "" || !strings.Contains(stderr, tc.there) {
				t.Errorf("exit %d, stdout %q, stderr %q; want 1 naming %s", code, stdout, stderr, tc.there)
			}
			if !equalTrees(before, tree(t, root)) {
				t.Errorf("files changed: before %v, after %v", keysOf(before), keysOf(tree(t, root)))
			}
		})
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
