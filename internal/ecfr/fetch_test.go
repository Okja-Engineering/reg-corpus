// Tests for `regcorpus fetch` (R1, plan step 2), offline against the fake
// eCFR in client_test.go: raw XML stored untouched and never overwritten,
// meta.yaml's fetch-owned fields, --date into .cache/ only, and the exit
// codes 0 ok, 1 conflict, 2 usage, network or unreadable input.
package ecfr

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// corpusRoot returns a root holding the three section directories, each with
// only its .gitkeep, as the scaffold left them.
func corpusRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, s := range sources {
		writeFile(t, root, s.dir()+"/.gitkeep", "")
	}
	return root
}

func writeFile(t *testing.T, root, slash, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(slash))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, slash string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(slash)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// tree maps every regular file under root, by slash path, to its bytes and
// modification time.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out[filepath.ToSlash(rel)] = info.ModTime().String() + "\x00" + string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// fetchAt runs fetch in root against c with the clock at day (YYYY-MM-DD).
func fetchAt(t *testing.T, root string, c *Client, day string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	now, err := time.Parse(time.DateOnly, day)
	if err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	f := fetcher{root: root, client: c, now: func() time.Time { return now.Add(13 * time.Hour) }}
	code = f.main(args, &out, &errb)
	return code, out.String(), errb.String()
}

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// latest is each section's latest version date in the fake's lists.
var latest = map[string]string{"1024.41": "2025-07-15", "1024.30": "2018-04-19", "1024.31": "2025-07-15"}

func rawPath(id string) string  { return "sources/ecfr/title-12/" + id + "/raw/" + latest[id] + ".xml" }
func metaPath(id string) string { return "sources/ecfr/title-12/" + id + "/meta.yaml" }

func TestFetchStoresEachSectionsLatestRawXMLUntouched(t *testing.T) {
	_, c := newFake(t)
	root := corpusRoot(t)
	code, stdout, stderr := fetchAt(t, root, c, "2026-11-22")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q; want 0 and empty", code, stderr)
	}
	for _, id := range []string{"1024.41", "1024.30", "1024.31"} {
		want := xmlBody(id, latest[id])
		if got := readFile(t, root, rawPath(id)); got != want {
			t.Errorf("%s: stored %q, want the served bytes %q", rawPath(id), got, want)
		}
		for _, line := range []string{
			id + " " + latest[id] + ": stored " + rawPath(id) + " (sha256 " + sha(want) + ")\n",
			id + " " + latest[id] + ": wrote " + metaPath(id) + "\n",
		} {
			if !strings.Contains(stdout, line) {
				t.Errorf("stdout %q lacks %q", stdout, line)
			}
		}
	}
}

// TestMetaYAMLHasTheFetchOwnedFieldsInSpecOrder: the spec's L0 meta.yaml
// fields that exist at fetch time, in the spec's order and style; the fields
// later steps add (version_effective_date, version_fr_citations: step 3;
// verified_on: step 6; sha256_text: step 5) are absent. Enum words and the
// bool are plain, as in the spec; every other value is double-quoted, so no
// YAML reader types a date or a hash as anything but a string.
func TestMetaYAMLHasTheFetchOwnedFieldsInSpecOrder(t *testing.T) {
	_, c := newFake(t)
	root := corpusRoot(t)
	if code, _, stderr := fetchAt(t, root, c, "2026-11-22"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, tc := range []struct{ id, role string }{
		{"1024.41", "inventoried"}, {"1024.30", "cite_only"}, {"1024.31", "cite_only"},
	} {
		want := `cfr: "12 CFR ` + tc.id + `"
tier: law
role: ` + tc.role + `
source: ecfr
official: false
source_version_date: "` + latest[tc.id] + `"
retrieved_at: "2026-11-22"
ecfr_up_to_date_as_of: "2026-11-19"
sha256_raw: "` + sha(xmlBody(tc.id, latest[tc.id])) + `"
`
		if got := readFile(t, root, metaPath(tc.id)); got != want {
			t.Errorf("%s:\n%s\nwant:\n%s", metaPath(tc.id), got, want)
		}
	}
}

// TestSecondRunIsANoOp: a later re-run against unchanged eCFR versions
// touches nothing, not even meta.yaml's retrieval facts.
func TestSecondRunIsANoOp(t *testing.T) {
	f, c := newFake(t)
	root := corpusRoot(t)
	if code, _, stderr := fetchAt(t, root, c, "2026-11-22"); code != 0 {
		t.Fatalf("first run: exit %d, stderr %q", code, stderr)
	}
	before := tree(t, root)
	f.set(apiPath+"/titles.json", reply{body: strings.ReplaceAll(titlesJSON, "2026-11-19", "2026-12-01")})
	code, stdout, stderr := fetchAt(t, root, c, "2026-12-02")
	if code != 0 || stderr != "" {
		t.Fatalf("second run: exit %d, stderr %q; want 0 and empty", code, stderr)
	}
	want := ""
	for _, id := range []string{"1024.41", "1024.30", "1024.31"} {
		want += id + " " + latest[id] + ": unchanged " + rawPath(id) + "\n"
	}
	if stdout != want {
		t.Errorf("stdout %q, want %q", stdout, want)
	}
	after := tree(t, root)
	for p, v := range before {
		if after[p] != v {
			t.Errorf("%s changed on the second run", p)
		}
	}
	if len(after) != len(before) {
		t.Errorf("second run left %d files, want %d", len(after), len(before))
	}
}

// TestDifferingRawFailsAndLeavesItUntouched: stored raw XML is never
// overwritten (R1). The conflict exits 1, names the file, writes no
// meta.yaml beside it, and the other sections are still fetched.
func TestDifferingRawFailsAndLeavesItUntouched(t *testing.T) {
	_, c := newFake(t)
	root := corpusRoot(t)
	stored := "<?xml version=\"1.0\"?>\n<DIV8>earlier bytes</DIV8>\n"
	writeFile(t, root, rawPath("1024.41"), stored)
	code, _, stderr := fetchAt(t, root, c, "2026-11-22")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	for _, w := range []string{"regcorpus fetch: 1024.41: ", rawPath("1024.41"), sha(stored), sha(xmlBody("1024.41", "2025-07-15"))} {
		if !strings.Contains(stderr, w) {
			t.Errorf("stderr %q lacks %q", stderr, w)
		}
	}
	if got := readFile(t, root, rawPath("1024.41")); got != stored {
		t.Errorf("%s was changed to %q", rawPath("1024.41"), got)
	}
	if _, err := os.Stat(filepath.Join(root, metaPath("1024.41"))); !os.IsNotExist(err) {
		t.Errorf("%s: stat err %v, want not written", metaPath("1024.41"), err)
	}
	for _, id := range []string{"1024.30", "1024.31"} {
		if got := readFile(t, root, rawPath(id)); got != xmlBody(id, latest[id]) {
			t.Errorf("%s not fetched after the 1024.41 conflict", id)
		}
	}
}

// TestMetaForAnotherVersionFails: a meta.yaml without the latest raw file
// describes another stored version; fetch exits 1 and writes nothing there.
func TestMetaForAnotherVersionFails(t *testing.T) {
	_, c := newFake(t)
	root := corpusRoot(t)
	old := "cfr: 12 CFR 1024.41\nsource_version_date: 2025-05-16\n"
	writeFile(t, root, metaPath("1024.41"), old)
	before := tree(t, root)
	code, _, stderr := fetchAt(t, root, c, "2026-11-22", "1024.41")
	if code != 1 || !strings.Contains(stderr, metaPath("1024.41")) || !strings.Contains(stderr, rawPath("1024.41")) {
		t.Errorf("exit %d, stderr %q; want 1 naming %s and %s", code, stderr, metaPath("1024.41"), rawPath("1024.41"))
	}
	after := tree(t, root)
	if len(after) != len(before) || after[metaPath("1024.41")] != before[metaPath("1024.41")] {
		t.Errorf("files changed: before %v, after %v", keysOf(before), keysOf(after))
	}
}

// TestStoredRawWithoutMetaGetsItsMeta: a run cut short after storing raw XML
// converges on the next run.
func TestStoredRawWithoutMetaGetsItsMeta(t *testing.T) {
	_, c := newFake(t)
	root := corpusRoot(t)
	writeFile(t, root, rawPath("1024.30"), xmlBody("1024.30", "2018-04-19"))
	code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", "1024.30")
	want := "1024.30 2018-04-19: unchanged " + rawPath("1024.30") + "\n" +
		"1024.30 2018-04-19: wrote " + metaPath("1024.30") + "\n"
	if code != 0 || stdout != want || stderr != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q; want 0, %q", code, stdout, stderr, want)
	}
	if !strings.Contains(readFile(t, root, metaPath("1024.30")), sha(xmlBody("1024.30", "2018-04-19"))) {
		t.Errorf("meta.yaml lacks the stored file's sha256")
	}
}

func TestNamedSectionsOnly(t *testing.T) {
	f, c := newFake(t)
	root := corpusRoot(t)
	if code, _, stderr := fetchAt(t, root, c, "2026-11-22", "1024.30"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for p := range tree(t, root) {
		if strings.HasSuffix(p, ".xml") && p != rawPath("1024.30") {
			t.Errorf("stored %s; only 1024.30 was named", p)
		}
	}
	for _, r := range f.requests() {
		if strings.Contains(r.URL.RawQuery, "1024.41") || strings.Contains(r.URL.RawQuery, "1024.31") {
			t.Errorf("requested %s; only 1024.30 was named", r.URL)
		}
	}
}

// TestDateWritesOnlyUnderCache: --date fetches that point in time into
// .cache/, never into sources/, and writes no meta.yaml.
func TestDateWritesOnlyUnderCache(t *testing.T) {
	f, c := newFake(t)
	root := corpusRoot(t)
	before := tree(t, root)
	code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", "--date", "2025-05-15", "1024.41")
	cached := ".cache/ecfr/title-12/1024.41/2025-05-15.xml"
	wantOut := "1024.41 2025-05-15: stored " + cached + " (sha256 " + sha(xmlBody("1024.41", "2025-05-15")) + ")\n"
	if code != 0 || stdout != wantOut || stderr != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q; want 0, %q", code, stdout, stderr, wantOut)
	}
	after := tree(t, root)
	for p := range after {
		if _, ok := before[p]; !ok && p != cached {
			t.Errorf("wrote %s; --date writes only %s", p, cached)
		}
	}
	if got := readFile(t, root, cached); got != xmlBody("1024.41", "2025-05-15") {
		t.Errorf("cached %q", got)
	}
	for _, r := range f.requests() {
		if !strings.Contains(r.URL.Path, "/full/2025-05-15/") {
			t.Errorf("requested %s; --date needs only the point-in-time XML", r.URL)
		}
	}
	// Re-running is a no-op, the same as for sources/.
	code, stdout, _ = fetchAt(t, root, c, "2026-11-23", "--date", "2025-05-15", "1024.41")
	if want := "1024.41 2025-05-15: unchanged " + cached + "\n"; code != 0 || stdout != want {
		t.Errorf("re-run: exit %d, stdout %q; want 0, %q", code, stdout, want)
	}
}

func TestFetchFailureExitsTwoAndStoresNothingForThatSection(t *testing.T) {
	f, c := newFake(t)
	root := corpusRoot(t)
	f.set(xmlURI("2025-07-15", "1024.31"), reply{status: 500, body: "boom"})
	code, _, stderr := fetchAt(t, root, c, "2026-11-22")
	if code != 2 || !strings.HasPrefix(stderr, "regcorpus fetch: 1024.31: ") || !strings.Contains(stderr, "500") {
		t.Errorf("exit %d, stderr %q; want 2 naming 1024.31 and the status", code, stderr)
	}
	for p := range tree(t, root) {
		if strings.HasPrefix(p, "sources/ecfr/title-12/1024.31/") && !strings.HasSuffix(p, ".gitkeep") {
			t.Errorf("wrote %s for the failed section", p)
		}
	}
}

func TestUsageAndRootErrors(t *testing.T) {
	_, c := newFake(t)
	for _, tc := range []struct {
		name   string
		root   string
		args   []string
		code   int
		stdout string // prefix; "" means empty
		stderr string // prefix; "" means empty
	}{
		{"help", corpusRoot(t), []string{"-h"}, 0, "usage: regcorpus fetch", ""},
		{"unknown section", corpusRoot(t), []string{"1024.99"}, 2, "",
			`regcorpus fetch: unknown section "1024.99" (known: 1024.41, 1024.30, 1024.31)`},
		{"bad date", corpusRoot(t), []string{"--date", "2025-5-15"}, 2, "",
			`regcorpus fetch: --date "2025-5-15" is not YYYY-MM-DD`},
		{"unknown flag", corpusRoot(t), []string{"--latest"}, 2, "",
			"regcorpus fetch: flag provided but not defined: -latest\nusage: regcorpus fetch"},
		{"not a corpus root", t.TempDir(), nil, 2, "", "regcorpus fetch: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := fetchAt(t, tc.root, c, "2026-11-22", tc.args...)
			if code != tc.code || !prefixOrEmpty(stdout, tc.stdout) || !prefixOrEmpty(stderr, tc.stderr) {
				t.Fatalf("exit %d, stdout %q, stderr %q; want %d, %q, %q", code, stdout, stderr, tc.code, tc.stdout, tc.stderr)
			}
			if tc.code != 0 {
				for p := range tree(t, tc.root) {
					if !strings.HasSuffix(p, ".gitkeep") {
						t.Errorf("wrote %s on a usage error", p)
					}
				}
			}
		})
	}
}

func prefixOrEmpty(got, want string) bool {
	if want == "" {
		return got == ""
	}
	return strings.HasPrefix(got, want)
}

func keysOf(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
