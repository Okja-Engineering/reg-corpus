// Tests for `regcorpus parse` (R1, R1a; plan step 5, ADR 0013): offline, it
// writes each named section's tree.json and text.md and sets sha256_text
// in its meta.yaml through internal/anchors and internal/l0, for a source
// whose parser goldens are in; the exit codes are 0 ok, 1 refused (nothing
// written), 2 usage or a file it can't read or write, the larger across
// sections.
package ecfr

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
)

func repoFile(t *testing.T, slash string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(slash)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// storedSource puts the committed raw XML and meta.yaml of section id into
// root, its meta.yaml without sha256_text, and returns that meta.yaml.
func storedSource(t *testing.T, root, id string) string {
	t.Helper()
	return storedSourceBody(t, root, id, repoFile(t, rawPath(id)))
}

// storedSourceBody is storedSource with body as the raw XML, which
// meta.yaml's sha256_raw then describes.
func storedSourceBody(t *testing.T, root, id string, body []byte) string {
	t.Helper()
	m, err := meta.Parse(repoFile(t, metaPath(id)))
	if err != nil {
		t.Fatal(err)
	}
	m.SHA256Text, m.SHA256Raw = "", meta.SHA256(l0.SHA256Hex(body))
	b, err := meta.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, rawPath(id), string(body))
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

// lines41 is what parse prints for 1024.41 when each file is verb ("wrote"
// or "unchanged"), its sha256_text sum.
func lines41(verb string, sum meta.SHA256) string {
	dir := "sources/ecfr/title-12/1024.41/"
	return "1024.41: " + verb + " " + dir + "tree.json\n" +
		"1024.41: " + verb + " " + dir + "text.md\n" +
		"1024.41: " + verb + " " + dir + "meta.yaml: sha256_text: \"" + string(sum) + "\"\n"
}

func TestParseWritesThenChangesNothing(t *testing.T) {
	root := corpusRoot(t)
	storedSource(t, root, "1024.41")
	d, err := anchors.Build(repoFile(t, rawPath("1024.41")))
	if err != nil {
		t.Fatal(err)
	}
	for _, verb := range []string{"wrote", "unchanged"} {
		code, stdout, stderr := parseAt(t, root, "1024.41")
		if want := lines41(verb, d.SHA256Text); code != 0 || stdout != want || stderr != "" {
			t.Fatalf("exit %d, stdout %q, stderr %q; want 0 and\n%s", code, stdout, stderr, want)
		}
	}
	dir := "sources/ecfr/title-12/1024.41/"
	if readFile(t, root, dir+"tree.json") != string(d.TreeJSON) || readFile(t, root, dir+"text.md") != string(d.TextMD) {
		t.Errorf("tree.json or text.md is not Build's")
	}
}

// TestParseFailureExitsOneAndWritesNothing: raw XML that doesn't parse, in
// a source whose goldens are in, exits 1 with the parser's location, and
// nothing is written. (1024.31's raw XML, which fails at line 4, column 4,
// stands in for 1024.41's: 1024.31 itself is refused before it is parsed.)
func TestParseFailureExitsOneAndWritesNothing(t *testing.T) {
	root := corpusRoot(t)
	before := storedSourceBody(t, root, "1024.41", repoFile(t, rawPath("1024.31")))
	code, stdout, stderr := parseAt(t, root, "1024.41")
	want := "regcorpus parse: 1024.41: " + rawPath("1024.41") + ": parse: line 4, column 4: unlabeled paragraph"
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, want) {
		t.Errorf("exit %d, stdout %q, stderr %q; want 1, nothing printed, %q", code, stdout, stderr, want)
	}
	dir := "sources/ecfr/title-12/1024.41/"
	if exists(root, dir+"tree.json") || exists(root, dir+"text.md") || readFile(t, root, metaPath("1024.41")) != before {
		t.Errorf("1024.41's files changed")
	}
}

// TestParseRefusesASourceWithoutGoldens: 1024.30 and 1024.31, whose parser
// goldens are plan step 9's, are refused (exit 1) with nothing written for
// them, even where their raw XML parses (1024.30's does); a section named
// beside them is still written.
func TestParseRefusesASourceWithoutGoldens(t *testing.T) {
	root := corpusRoot(t)
	storedSource(t, root, "1024.41")
	before := map[string]string{"1024.30": storedSource(t, root, "1024.30"), "1024.31": storedSource(t, root, "1024.31")}
	code, stdout, stderr := parseAt(t, root, "1024.30", "1024.41", "1024.31")
	d, err := anchors.Build(repoFile(t, rawPath("1024.41")))
	if err != nil {
		t.Fatal(err)
	}
	wantErr := "regcorpus parse: 1024.30: goldens are plan step 9's: parse writes a source's files only once its parser goldens are in (stage 01), so nothing is written\n" +
		"regcorpus parse: 1024.31: goldens are plan step 9's: parse writes a source's files only once its parser goldens are in (stage 01), so nothing is written\n"
	if code != 1 || stdout != lines41("wrote", d.SHA256Text) || stderr != wantErr {
		t.Errorf("exit %d, stdout %q, stderr %q; want 1, 1024.41 written, and\n%s", code, stdout, stderr, wantErr)
	}
	for id, m := range before {
		dir := "sources/ecfr/title-12/" + id + "/"
		if exists(root, dir+"tree.json") || exists(root, dir+"text.md") || readFile(t, root, metaPath(id)) != m {
			t.Errorf("%s's files changed", id)
		}
	}
}

// TestParseExitCodesCombine: each section is parsed whatever another's
// outcome, and the larger code wins, in either order: a refused section
// (1) beside one that can't be read (2) exits 2, and beside one written
// (0), 1.
func TestParseExitCodesCombine(t *testing.T) {
	for _, tc := range []struct {
		name     string
		readable bool
		args     []string
		code     int
	}{
		{"refused, then unreadable", false, []string{"1024.30", "1024.41"}, 2},
		{"unreadable, then refused", false, []string{"1024.41", "1024.30"}, 2},
		{"refused, then written", true, []string{"1024.30", "1024.41"}, 1},
		{"written, then refused", true, []string{"1024.41", "1024.30"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := corpusRoot(t)
			storedSource(t, root, "1024.41")
			if !tc.readable {
				raw := filepath.Join(root, filepath.FromSlash(rawPath("1024.41")))
				if err := os.Chmod(raw, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.Chmod(raw, 0o644) })
			}
			code, _, stderr := parseAt(t, root, tc.args...)
			if code != tc.code || !strings.Contains(stderr, "regcorpus parse: 1024.30: goldens are plan step 9's") {
				t.Errorf("exit %d, stderr %q; want %d, 1024.30 refused", code, stderr, tc.code)
			}
			if !tc.readable && !strings.Contains(stderr, "regcorpus parse: 1024.41: ") {
				t.Errorf("stderr %q lacks 1024.41's error", stderr)
			}
		})
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
		{"no meta.yaml", root, []string{"1024.41"}, 1, "", "regcorpus parse: 1024.41: " + metaPath("1024.41") + " does not exist"},
		{"no goldens", root, []string{"1024.30"}, 1, "", "regcorpus parse: 1024.30: goldens are plan step 9's"},
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
// command's -h gives them), which sources parse writes, what a failure
// part-way leaves, that exit 1 writes nothing for that section, and what
// text.md can't hold: a line break but one LF ending the citation note, and
// a part that would show as an empty line.
func TestParseUsageGivesExitCodes(t *testing.T) {
	_, stdout, _ := parseAt(t, corpusRoot(t), "-h")
	usage := strings.Join(strings.Fields(stdout), " ")
	for _, want := range []string{
		"Exit 0:", "Exit 1:", "Exit 2:", "offline", "overrides.yaml", "1024.41, 1024.30 or 1024.31",
		"today 1024.41's", "plan step 9's", "a re-run writes the rest", "nothing written for that section",
		`no markup but the "# " that starts its heading line`,
		"anywhere but one LF ending it", "shows as an empty line",
	} {
		if !strings.Contains(usage, want) {
			t.Errorf("usage lacks %q:\n%s", want, stdout)
		}
	}
}
