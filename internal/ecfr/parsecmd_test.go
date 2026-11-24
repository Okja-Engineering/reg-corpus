// Tests for `regcorpus parse` (R1, R1a; plan step 5, ADR 0013): offline, it
// writes each named section's tree.json and text.md and sets sha256_text
// in its meta.yaml through internal/anchors and internal/l0; the exit codes
// are 0 ok, 1 refused by what is stored, 2 usage or a file it can't read or
// write.
package ecfr

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/meta"
)

// storedSource puts the committed raw XML and meta.yaml of section id into
// root, its meta.yaml without sha256_text, and returns that meta.yaml.
func storedSource(t *testing.T, root, id string) string {
	t.Helper()
	repo := func(slash string) []byte {
		b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(slash)))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	m, err := meta.Parse(repo(metaPath(id)))
	if err != nil {
		t.Fatal(err)
	}
	m.SHA256Text = ""
	b, err := meta.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, rawPath(id), string(repo(rawPath(id))))
	writeFile(t, root, metaPath(id), string(b))
	return string(b)
}

func parseAt(t *testing.T, root string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = parseMain(root, args, &out, &errb)
	return code, out.String(), errb.String()
}

func exists(root, slash string) bool {
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(slash)))
	return err == nil
}

func TestParseWritesThenChangesNothing(t *testing.T) {
	root := corpusRoot(t)
	storedSource(t, root, "1024.41")
	raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rawPath("1024.41"))))
	if err != nil {
		t.Fatal(err)
	}
	d, err := anchors.Build(raw)
	if err != nil {
		t.Fatal(err)
	}
	dir := "sources/ecfr/title-12/1024.41/"
	lines := func(verb string) string {
		return "1024.41: " + verb + " " + dir + "tree.json\n" +
			"1024.41: " + verb + " " + dir + "text.md\n" +
			"1024.41: " + verb + " " + dir + "meta.yaml: sha256_text: \"" + string(d.SHA256Text) + "\"\n"
	}
	for _, verb := range []string{"wrote", "unchanged"} {
		code, stdout, stderr := parseAt(t, root, "1024.41")
		if code != 0 || stdout != lines(verb) || stderr != "" {
			t.Fatalf("exit %d, stdout %q, stderr %q; want 0 and\n%s", code, stdout, stderr, lines(verb))
		}
	}
	if readFile(t, root, dir+"tree.json") != string(d.TreeJSON) || readFile(t, root, dir+"text.md") != string(d.TextMD) {
		t.Errorf("tree.json or text.md is not Build's")
	}
}

// TestParseFailureExitsOneAndWritesNothing: raw XML that doesn't parse
// exits 1 with the parser's location; nothing is written for it, and a
// section named beside it is still written.
func TestParseFailureExitsOneAndWritesNothing(t *testing.T) {
	root := corpusRoot(t)
	storedSource(t, root, "1024.41")
	before := storedSource(t, root, "1024.31")
	code, stdout, stderr := parseAt(t, root, "1024.31", "1024.41")
	want := "regcorpus parse: 1024.31: " + rawPath("1024.31") + ": parse: line 4, column 4: unlabeled paragraph"
	if code != 1 || !strings.HasPrefix(stderr, want) || !strings.Contains(stdout, "1024.41: wrote ") {
		t.Errorf("exit %d, stdout %q, stderr %q; want 1, %q, and 1024.41 written", code, stdout, stderr, want)
	}
	dir := "sources/ecfr/title-12/1024.31/"
	if exists(root, dir+"tree.json") || exists(root, dir+"text.md") || readFile(t, root, metaPath("1024.31")) != before {
		t.Errorf("1024.31's files changed")
	}
}

func TestParseOverridesExitOne(t *testing.T) {
	root := corpusRoot(t)
	before := storedSource(t, root, "1024.41")
	writeFile(t, root, "sources/ecfr/title-12/1024.41/overrides.yaml", "[]\n")
	code, stdout, stderr := parseAt(t, root, "1024.41")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "overrides.yaml") {
		t.Errorf("exit %d, stdout %q, stderr %q; want 1 naming overrides.yaml", code, stdout, stderr)
	}
	if exists(root, "sources/ecfr/title-12/1024.41/tree.json") || readFile(t, root, metaPath("1024.41")) != before {
		t.Errorf("files were written")
	}
}

func TestParseUsage(t *testing.T) {
	root := corpusRoot(t)
	for _, tc := range []struct {
		name   string
		root   string
		args   []string
		code   int
		stdout string // prefix
		stderr string // prefix
	}{
		{"help", root, []string{"-h"}, 0, "usage: regcorpus parse", ""},
		{"--help", root, []string{"--help"}, 0, "usage: regcorpus parse", ""},
		{"no section", root, nil, 2, "", "regcorpus parse: no section"},
		{"a flag", root, []string{"--all"}, 2, "", `regcorpus parse: "--all": parse takes no flags`},
		{"a flag after a section", root, []string{"1024.41", "-n"}, 2, "", `regcorpus parse: "-n": parse takes no flags`},
		{"unknown section", root, []string{"1024.99"}, 2, "", `regcorpus parse: unknown section "1024.99" (known: 1024.41, 1024.30, 1024.31)`},
		{"not the corpus root", t.TempDir(), []string{"1024.41"}, 2, "", "regcorpus parse: "},
		{"no meta.yaml", root, []string{"1024.30"}, 1, "", "regcorpus parse: 1024.30: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := parseAt(t, tc.root, tc.args...)
			if code != tc.code || !strings.HasPrefix(stdout, tc.stdout) || (tc.stdout == "" && stdout != "") ||
				!strings.HasPrefix(stderr, tc.stderr) || (tc.stderr == "" && stderr != "") {
				t.Errorf("exit %d, stdout %q, stderr %q; want %d, %q, %q", code, stdout, stderr, tc.code, tc.stdout, tc.stderr)
			}
		})
	}
}

// TestParseUsageGivesExitCodes: -h names every exit code (AGENTS.md: each
// command's -h gives them).
func TestParseUsageGivesExitCodes(t *testing.T) {
	_, stdout, _ := parseAt(t, corpusRoot(t), "-h")
	for _, want := range []string{"Exit 0:", "Exit 1:", "Exit 2:", "offline", "overrides.yaml", "1024.41, 1024.30 or 1024.31"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("usage lacks %q:\n%s", want, stdout)
		}
	}
}
