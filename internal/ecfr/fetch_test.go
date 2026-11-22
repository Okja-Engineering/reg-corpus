// Tests for `regcorpus fetch` (R1, plan step 2), offline against the fake
// eCFR in client_test.go: raw XML stored untouched and never overwritten,
// meta.yaml's fetch-owned fields, --date into .cache/ only, and the exit
// codes 0 ok, 1 conflict, 2 usage, network or unreadable input.
package ecfr

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

// tree maps every file under root, by slash path, to its bytes and
// modification time, and every symlink to its target.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			to, err := os.Readlink(p)
			rel, _ := filepath.Rel(root, p)
			out[filepath.ToSlash(rel)] = "symlink to " + to
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
		if !strings.Contains(r.URL.Path, "/full/2025-05-15/") && r.URL.Path != apiPath+"/titles.json" {
			t.Errorf("requested %s; --date needs only titles.json and the point-in-time XML", r.URL)
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

// TestUsageAndRootErrors: a usage error or a directory that isn't a corpus
// root (no sources/ecfr/title-12) exits 2 before any request, and writes
// nothing. A symlink or a file in place of a directory there is what it is
// anywhere under sources/: not a real directory, exit 1, also before any
// request.
func TestUsageAndRootErrors(t *testing.T) {
	notRoot := t.TempDir()
	titleFile := t.TempDir()
	writeFile(t, titleFile, "sources/ecfr/title-12", "not a directory")
	titleLink := t.TempDir()
	writeFile(t, titleLink, "sources/ecfr/.gitkeep", "")
	if err := os.Symlink(corpusRoot(t), filepath.Join(titleLink, "sources", "ecfr", "title-12")); err != nil {
		t.Fatal(err)
	}
	sourcesFile := t.TempDir()
	writeFile(t, sourcesFile, "sources", "not a directory")
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
			`regcorpus fetch: invalid value "2025-5-15" for flag -date: "2025-5-15" is not YYYY-MM-DD` + "\nusage: regcorpus fetch"},
		{"empty date", corpusRoot(t), []string{"--date="}, 2, "",
			`regcorpus fetch: invalid value "" for flag -date: "" is not YYYY-MM-DD` + "\nusage: regcorpus fetch"},
		{"date given twice", corpusRoot(t), []string{"--date", "2025-05-15", "--date", "2025-05-16"}, 2, "",
			`regcorpus fetch: invalid value "2025-05-16" for flag -date: --date given more than once` + "\nusage: regcorpus fetch"},
		{"a flag after a section", corpusRoot(t), []string{"1024.41", "--date", "2025-05-15"}, 2, "",
			`regcorpus fetch: "--date" comes after a section; flags go before the sections` + "\nusage: regcorpus fetch"},
		{"unknown flag", corpusRoot(t), []string{"--latest"}, 2, "",
			"regcorpus fetch: flag provided but not defined: -latest\nusage: regcorpus fetch"},
		{"not a corpus root", notRoot, nil, 2, "",
			"regcorpus fetch: " + notRoot + " is not a corpus root (no sources/ecfr/title-12/ directory); run fetch from the corpus root\n"},
		{"sources/ecfr/title-12 is a file", titleFile, nil, 1, "",
			"regcorpus fetch: sources/ecfr/title-12 is not a real directory"},
		{"sources/ecfr/title-12 is a symlink", titleLink, nil, 1, "",
			"regcorpus fetch: sources/ecfr/title-12 is not a real directory"},
		{"sources is a file", sourcesFile, nil, 1, "",
			"regcorpus fetch: sources is not a real directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := newFake(t)
			before := tree(t, tc.root)
			code, stdout, stderr := fetchAt(t, tc.root, c, "2026-11-22", tc.args...)
			if code != tc.code || !prefixOrEmpty(stdout, tc.stdout) || !prefixOrEmpty(stderr, tc.stderr) {
				t.Fatalf("exit %d, stdout %q, stderr %q; want %d, %q, %q", code, stdout, stderr, tc.code, tc.stdout, tc.stderr)
			}
			if n := len(f.requests()); n != 0 {
				t.Errorf("%d requests; want none", n)
			}
			if after := tree(t, tc.root); !equalTrees(before, after) {
				t.Errorf("files changed: before %v, after %v", keysOf(before), keysOf(after))
			}
		})
	}
}

// fetchMeta is the meta.yaml fetch writes for 1024.41's raw file at
// date with sha256 sum.
func fetchMeta(date, sum string) string {
	return "cfr: \"12 CFR 1024.41\"\ntier: law\nrole: inventoried\nsource: ecfr\nofficial: false\n" +
		"source_version_date: \"" + date + "\"\nretrieved_at: \"2026-11-01\"\n" +
		"ecfr_up_to_date_as_of: \"2026-10-30\"\nsha256_raw: \"" + sum + "\"\n"
}

// TestStoredStateKeepsOneRawPerSource: fetch exits 0 only when, after it,
// raw/ holds exactly one entry, a regular file named for eCFR's latest date
// with eCFR's bytes, and meta.yaml (a regular file in the spec's schema)
// names that file's date and sha256. fetch reads what is stored once; when
// what is there contradicts that, it stores nothing and exits 1 naming it.
func TestStoredStateKeepsOneRawPerSource(t *testing.T) {
	const older = "sources/ecfr/title-12/1024.41/raw/2025-05-16.xml"
	latestBody := xmlBody("1024.41", "2025-07-15")
	goodMeta := fetchMeta("2025-07-15", sha(latestBody))
	file := func(p, body string) func(*testing.T, string) {
		return func(t *testing.T, root string) { writeFile(t, root, p, body) }
	}
	symlink := func(p, to string) func(*testing.T, string) {
		return func(t *testing.T, root string) {
			full := filepath.Join(root, filepath.FromSlash(p))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(to, full); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, tc := range []struct {
		name  string
		setup []func(*testing.T, string)
		want  []string // in stderr
	}{
		// raw/ holds exactly one entry, the latest, as a regular file.
		{"meta.yaml whose raw file is missing", []func(*testing.T, string){file(metaPath("1024.41"), goodMeta)},
			[]string{metaPath("1024.41"), "sources/ecfr/title-12/1024.41/raw/ holds no raw XML", "git restore"}},
		{"an older raw file, no meta.yaml", []func(*testing.T, string){file(older, "<DIV8/>")},
			[]string{older, "one raw file per source"}},
		{"an older raw file and its meta.yaml", []func(*testing.T, string){file(older, "<DIV8/>"),
			file(metaPath("1024.41"), fetchMeta("2025-05-16", sha("<DIV8/>")))},
			[]string{older, "one raw file per source"}},
		{"the latest raw file and an older one", []func(*testing.T, string){file(older, "<DIV8/>"), file(rawPath("1024.41"), latestBody)},
			[]string{older, "one raw file per source"}},
		{"the latest raw file, an older one and meta.yaml", []func(*testing.T, string){file(older, "<DIV8/>"),
			file(rawPath("1024.41"), latestBody), file(metaPath("1024.41"), goodMeta)},
			[]string{older, "one raw file per source"}},
		{"an upper-case .XML beside the latest", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file("sources/ecfr/title-12/1024.41/raw/2025-05-16.XML", "<DIV8/>")},
			[]string{"raw/2025-05-16.XML", "one raw file per source"}},
		{"a backup beside the latest", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(rawPath("1024.41")+".bak", latestBody)},
			[]string{rawPath("1024.41") + ".bak", "one raw file per source"}},
		{"a leftover temporary file", []func(*testing.T, string){file("sources/ecfr/title-12/1024.41/raw/.fetch-123", "partial")},
			[]string{"raw/.fetch-123", "one raw file per source"}},
		{"a directory in raw/", []func(*testing.T, string){file("sources/ecfr/title-12/1024.41/raw/old/x", "x")},
			[]string{"raw/old", "one raw file per source"}},
		{"the latest raw file as a symlink", []func(*testing.T, string){file("elsewhere.xml", latestBody),
			symlink(rawPath("1024.41"), "../../../../../elsewhere.xml")},
			[]string{rawPath("1024.41"), "not a regular file"}},
		// meta.yaml is fetch's own, and names the one raw file.
		{"meta.yaml naming another date", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), fetchMeta("2025-05-16", sha(latestBody)))},
			[]string{metaPath("1024.41"), `source_version_date "2025-05-16"`, "2025-07-15"}},
		{"meta.yaml naming another hash", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), fetchMeta("2025-07-15", strings.Repeat("d", 64)))},
			[]string{metaPath("1024.41"), "sha256_raw", strings.Repeat("d", 64), sha(latestBody)}},
		{"an empty meta.yaml", []func(*testing.T, string){file(rawPath("1024.41"), latestBody), file(metaPath("1024.41"), "")},
			[]string{metaPath("1024.41"), "no source_version_date"}},
		{"meta.yaml without sha256_raw", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), "source_version_date: \"2025-07-15\"\n")},
			[]string{metaPath("1024.41"), "no sha256_raw"}},
		{"meta.yaml with an unquoted date", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), strings.Replace(goodMeta, `"2025-07-15"`, "2025-07-15", 1))},
			[]string{metaPath("1024.41"), "source_version_date"}},
		{"meta.yaml with a back-quoted date", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), strings.Replace(goodMeta, `"2025-07-15"`, "`2025-07-15`", 1))},
			[]string{metaPath("1024.41"), "source_version_date"}},
		{"meta.yaml with a short hash", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), fetchMeta("2025-07-15", "deadbeef"))},
			[]string{metaPath("1024.41"), "sha256_raw", "deadbeef"}},
		{"meta.yaml with an upper-case hash", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), fetchMeta("2025-07-15", strings.ToUpper(sha(latestBody))))},
			[]string{metaPath("1024.41"), "sha256_raw", strings.ToUpper(sha(latestBody))}},
		{"meta.yaml naming its date twice", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), goodMeta+"source_version_date: \"2025-05-16\"\n")},
			[]string{metaPath("1024.41"), "source_version_date", "twice"}},
		{"meta.yaml as a symlink to nothing", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			symlink(metaPath("1024.41"), "nowhere.yaml")},
			[]string{metaPath("1024.41"), "not a regular file"}},
		// meta.yaml is in the spec's schema (internal/meta), and every field
		// fetch writes says what fetch's source table says (issue #5).
		{"meta.yaml naming another section", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), strings.Replace(goodMeta, "12 CFR 1024.41", "12 CFR 1024.30", 1))},
			[]string{metaPath("1024.41"), "cfr", `"12 CFR 1024.30"`, `"12 CFR 1024.41"`}},
		{"meta.yaml naming another tier", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), strings.Replace(goodMeta, "tier: law", "tier: regulator_interpretation", 1))},
			[]string{metaPath("1024.41"), "tier", "regulator_interpretation", "law"}},
		{"meta.yaml naming another role", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), strings.Replace(goodMeta, "role: inventoried", "role: cite_only", 1))},
			[]string{metaPath("1024.41"), "role", "cite_only", "inventoried"}},
		{"meta.yaml saying official: true", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), strings.Replace(goodMeta, "official: false", "official: true", 1))},
			[]string{metaPath("1024.41"), "official", "true", "false"}},
		{"meta.yaml naming a source the spec doesn't", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), strings.Replace(goodMeta, "source: ecfr", "source: govinfo", 1))},
			[]string{metaPath("1024.41"), "source", "govinfo"}},
		{"meta.yaml with an unknown field", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), goodMeta+"note: \"x\"\n")},
			[]string{metaPath("1024.41"), `"note"`}},
		{"meta.yaml out of the spec's order", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), strings.Replace(goodMeta, "tier: law\nrole: inventoried\n", "role: inventoried\ntier: law\n", 1))},
			[]string{metaPath("1024.41"), "tier", "role"}},
		{"meta.yaml without retrieved_at", []func(*testing.T, string){file(rawPath("1024.41"), latestBody),
			file(metaPath("1024.41"), strings.Replace(goodMeta, "retrieved_at: \"2026-11-01\"\n", "", 1))},
			[]string{metaPath("1024.41"), "no retrieved_at"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, c := newFake(t)
			root := corpusRoot(t)
			for _, s := range tc.setup {
				s(t, root)
			}
			before := tree(t, root)
			code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", "1024.41")
			if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "regcorpus fetch: 1024.41: ") {
				t.Errorf("exit %d, stdout %q, stderr %q; want 1 for 1024.41", code, stdout, stderr)
			}
			for _, w := range tc.want {
				if !strings.Contains(stderr, w) {
					t.Errorf("stderr %q lacks %q", stderr, w)
				}
			}
			if after := tree(t, root); !equalTrees(before, after) {
				t.Errorf("files changed: before %v, after %v", keysOf(before), keysOf(after))
			}
		})
	}
	t.Run("raw XML and the meta.yaml naming it", func(t *testing.T) {
		_, c := newFake(t)
		root := corpusRoot(t)
		writeFile(t, root, rawPath("1024.41"), latestBody)
		writeFile(t, root, metaPath("1024.41"), goodMeta)
		before := tree(t, root)
		code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", "1024.41")
		if want := "1024.41 2025-07-15: unchanged " + rawPath("1024.41") + "\n"; code != 0 || stdout != want || stderr != "" {
			t.Errorf("exit %d, stdout %q, stderr %q; want 0, %q", code, stdout, stderr, want)
		}
		if !equalTrees(before, tree(t, root)) {
			t.Errorf("files changed")
		}
	})
}

// TestFetchKeepsTheFieldsLaterStepsSet: a meta.yaml that also holds the
// fields other plan steps set (version_effective_date and
// version_fr_citations: step 3; sha256_text: step 5; verified_on: step 6) is
// still fetch's: a re-run checks fetch's own fields, leaves the rest as they
// are, and is a no-op.
func TestFetchKeepsTheFieldsLaterStepsSet(t *testing.T) {
	latestBody := xmlBody("1024.41", "2025-07-15")
	for _, tc := range []struct{ name, meta string }{
		{"step 3's fields", strings.Replace(fetchMeta("2025-07-15", sha(latestBody)), "retrieved_at:",
			"version_effective_date: \"2025-07-15\"\nversion_fr_citations: [\"90 FR 20792\"]\nretrieved_at:", 1)},
		{"every field", strings.Replace(fetchMeta("2025-07-15", sha(latestBody)), "retrieved_at:",
			"version_effective_date: \"2025-07-15\"\nversion_fr_citations: [\"90 FR 20792\", \"86 FR 34899\"]\nretrieved_at:", 1) +
			"sha256_text: \"" + strings.Repeat("a", 64) + "\"\n"},
		{"verified_on null", strings.Replace(fetchMeta("2025-07-15", sha(latestBody)), "sha256_raw:",
			"verified_on: null\nsha256_raw:", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, c := newFake(t)
			root := corpusRoot(t)
			writeFile(t, root, rawPath("1024.41"), latestBody)
			writeFile(t, root, metaPath("1024.41"), tc.meta)
			before := tree(t, root)
			code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", "1024.41")
			if want := "1024.41 2025-07-15: unchanged " + rawPath("1024.41") + "\n"; code != 0 || stdout != want || stderr != "" {
				t.Errorf("exit %d, stdout %q, stderr %q; want 0, %q", code, stdout, stderr, want)
			}
			if !equalTrees(before, tree(t, root)) {
				t.Errorf("files changed")
			}
		})
	}
}

// TestUnreadableStoredStateExitsTwo: a source directory fetch can't read is
// a local file error (exit 2), not a conflict, and nothing is stored.
func TestUnreadableStoredStateExitsTwo(t *testing.T) {
	unreadable := func(p string) func(*testing.T, string) {
		return func(t *testing.T, root string) {
			if os.Geteuid() == 0 {
				t.Skip("running with privileges that read a mode-0 file")
			}
			full := filepath.Join(root, filepath.FromSlash(p))
			if err := os.Chmod(full, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(full, 0o755) })
		}
	}
	for _, tc := range []struct {
		name  string
		files map[string]string
		then  func(*testing.T, string)
	}{
		{"raw/ can't be listed", map[string]string{rawPath("1024.41"): xmlBody("1024.41", "2025-07-15"),
			metaPath("1024.41"): fetchMeta("2025-07-15", sha(xmlBody("1024.41", "2025-07-15")))},
			unreadable("sources/ecfr/title-12/1024.41/raw")},
		{"meta.yaml can't be read", map[string]string{rawPath("1024.41"): xmlBody("1024.41", "2025-07-15"),
			metaPath("1024.41"): fetchMeta("2025-07-15", sha(xmlBody("1024.41", "2025-07-15")))},
			unreadable(metaPath("1024.41"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, c := newFake(t)
			root := corpusRoot(t)
			for p, b := range tc.files {
				writeFile(t, root, p, b)
			}
			if tc.then != nil {
				tc.then(t, root)
			}
			code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", "1024.41")
			if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "regcorpus fetch: 1024.41: ") {
				t.Errorf("exit %d, stdout %q, stderr %q; want 2 for 1024.41", code, stdout, stderr)
			}
		})
	}
}

// TestEveryDirectoryOnTheWayIsReal: every directory fetch reads or writes,
// from the corpus root down (sources/ to raw/, .cache/ to the source's cache
// directory), must be a real directory. A symlink (even to a directory) or a
// file in its place exits 1, and nothing is stored there or outside.
func TestEveryDirectoryOnTheWayIsReal(t *testing.T) {
	latestBody := xmlBody("1024.41", "2025-07-15")
	for _, tc := range []struct {
		name    string
		link    string            // slash path made a symlink to a directory outside the root
		outside map[string]string // what that outside directory holds
		file    string            // or: slash path made a regular file
		args    []string
	}{
		{name: "raw/ a symlink", link: "sources/ecfr/title-12/1024.41/raw"},
		{name: "raw/ a symlink to the latest file, with its meta.yaml", link: "sources/ecfr/title-12/1024.41/raw",
			outside: map[string]string{"2025-07-15.xml": latestBody}},
		{name: "raw/ a symlink to an older file", link: "sources/ecfr/title-12/1024.41/raw",
			outside: map[string]string{"2025-05-16.xml": "<DIV8/>"}},
		{name: "raw a file", file: "sources/ecfr/title-12/1024.41/raw"},
		{name: "the source directory a symlink", link: "sources/ecfr/title-12/1024.41"},
		{name: "sources/ a symlink", link: "sources",
			outside: map[string]string{"ecfr/title-12/1024.41/.gitkeep": ""}},
		{name: ".cache/ a symlink, with --date", link: ".cache", args: []string{"--date", "2025-05-15"}},
		{name: "the cache directory a symlink, with --date", link: ".cache/ecfr/title-12/1024.41", args: []string{"--date", "2025-05-15"}},
		{name: "the cache directory a file, with --date", file: ".cache/ecfr/title-12/1024.41", args: []string{"--date", "2025-05-15"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, c := newFake(t)
			root := t.TempDir()
			outside := t.TempDir()
			for p, b := range tc.outside {
				writeFile(t, outside, p, b)
			}
			for _, s := range sources {
				if !strings.HasPrefix(s.dir()+"/", tc.link+"/") || tc.link == "" {
					writeFile(t, root, s.dir()+"/.gitkeep", "")
				}
			}
			if tc.outside["2025-07-15.xml"] != "" {
				writeFile(t, root, metaPath("1024.41"), fetchMeta("2025-07-15", sha(latestBody)))
			}
			if tc.link != "" {
				full := filepath.Join(root, filepath.FromSlash(tc.link))
				if err := os.RemoveAll(full); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, full); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFile(t, root, tc.file, "not a directory")
			}
			before, beforeOutside := tree(t, root), tree(t, outside)
			code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", append(tc.args, "1024.41")...)
			if code != 1 || stdout != "" || !strings.Contains(stderr, "not a real directory") || !strings.Contains(stderr, tc.link+tc.file) {
				t.Errorf("exit %d, stdout %q, stderr %q; want 1 naming %s as not a real directory", code, stdout, stderr, tc.link+tc.file)
			}
			if !equalTrees(before, tree(t, root)) || !equalTrees(beforeOutside, tree(t, outside)) {
				t.Errorf("files changed under the root or outside it")
			}
		})
	}
}

// TestSectionsAreFetchedInTheOrderGiven: named sections are fetched and
// reported in the order named, not the table's.
func TestSectionsAreFetchedInTheOrderGiven(t *testing.T) {
	_, c := newFake(t)
	root := corpusRoot(t)
	code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", "1024.31", "1024.41")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	i31, i41 := strings.Index(stdout, "1024.31 "), strings.Index(stdout, "1024.41 ")
	if i31 < 0 || i41 < 0 || i31 > i41 {
		t.Errorf("stdout %q; want 1024.31 reported before 1024.41", stdout)
	}
}

func equalTrees(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for p, v := range a {
		if b[p] != v {
			return false
		}
	}
	return true
}

// TestALinkThatFindsAFileComparesIt: when the hard link finds a file already
// at the path (one written after storeOnce read it), that file is compared,
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
			var conflict *conflictError
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

// TestStoreNeverReplacesWhatIsAtThePath: a file is placed by a hard link,
// which fails when anything is at the path, even a symlink to nothing. That
// is an error, and the symlink is left as it was, its target not created.
func TestStoreNeverReplacesWhatIsAtThePath(t *testing.T) {
	_, c := newFake(t)
	root := corpusRoot(t)
	// The .cache/ path, where storeOnce alone guards the target: under
	// sources/, the stored-state check refuses a symlink before storeOnce.
	const cached = ".cache/ecfr/title-12/1024.41/2025-05-15.xml"
	at := filepath.Join(root, filepath.FromSlash(cached))
	target := filepath.Join(root, "elsewhere.xml")
	if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, at); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := fetchAt(t, root, c, "2026-11-22", "--date", "2025-05-15", "1024.41")
	if code != 2 || !strings.Contains(stderr, cached) {
		t.Errorf("exit %d, stderr %q; want 2 naming %s", code, stderr, cached)
	}
	if got, err := os.Readlink(at); err != nil || got != target {
		t.Errorf("%s: readlink %q, %v; want the symlink to %s left as it was", at, got, err, target)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Errorf("%s: lstat err %v; want not created", target, err)
	}
}

// TestStoredFilesAreWorldReadable: raw XML and meta.yaml are stored mode
// 0644, not the temporary file's 0600.
func TestStoredFilesAreWorldReadable(t *testing.T) {
	_, c := newFake(t)
	root := corpusRoot(t)
	if code, _, stderr := fetchAt(t, root, c, "2026-11-22", "1024.41"); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, p := range []string{rawPath("1024.41"), metaPath("1024.41")} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Errorf("%s: mode %v, want 0644", p, info.Mode().Perm())
		}
	}
}

// TestAMetaWriteFailureStillReportsTheStoredRaw: when meta.yaml can't be
// written, the raw file already stored is still reported, and fetch exits 2.
func TestAMetaWriteFailureStillReportsTheStoredRaw(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running with privileges that write into a mode-0555 directory")
	}
	_, c := newFake(t)
	root := corpusRoot(t)
	dir := filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.30"))
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", "1024.30")
	if want := "1024.30 2018-04-19: stored " + rawPath("1024.30"); code != 2 || !strings.Contains(stdout, want) || !strings.HasPrefix(stderr, "regcorpus fetch: 1024.30: ") {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2, stdout naming %q", code, stdout, stderr, want)
	}
}

// TestExitCodeIsTheWorstAcrossSections: each section is fetched whatever
// another's outcome, and the exit code is the largest: a network failure (2)
// outranks a conflict (1) in either order.
func TestExitCodeIsTheWorstAcrossSections(t *testing.T) {
	for _, args := range [][]string{{"1024.41", "1024.31"}, {"1024.31", "1024.41"}} {
		t.Run(strings.Join(args, " then "), func(t *testing.T) {
			f, c := newFake(t)
			root := corpusRoot(t)
			writeFile(t, root, rawPath("1024.41"), "<DIV8>earlier bytes</DIV8>")
			f.set(xmlURI("2025-07-15", "1024.31"), reply{status: 500, body: "boom"})
			code, _, stderr := fetchAt(t, root, c, "2026-11-22", args...)
			if code != 2 || !strings.Contains(stderr, "regcorpus fetch: 1024.41: ") || !strings.Contains(stderr, "regcorpus fetch: 1024.31: ") {
				t.Errorf("exit %d, stderr %q; want 2 and both sections named", code, stderr)
			}
		})
	}
}

// TestRetrievedAtIsTheUTCDate: retrieved_at is the UTC date of the fetch,
// whatever the local zone, on either side of local midnight.
func TestRetrievedAtIsTheUTCDate(t *testing.T) {
	for _, tc := range []struct {
		now  time.Time
		want string
	}{
		{time.Date(2026, 11, 22, 23, 30, 0, 0, time.FixedZone("UTC-5", -5*3600)), "2026-11-23"},
		{time.Date(2026, 11, 23, 1, 0, 0, 0, time.FixedZone("UTC+9", 9*3600)), "2026-11-22"},
	} {
		_, c := newFake(t)
		root := corpusRoot(t)
		var out, errb bytes.Buffer
		f := fetcher{root: root, client: c, now: func() time.Time { return tc.now }}
		if code := f.main([]string{"1024.31"}, &out, &errb); code != 0 {
			t.Fatalf("exit %d, stderr %q", code, errb.String())
		}
		if got := readFile(t, root, metaPath("1024.31")); !strings.Contains(got, "retrieved_at: \""+tc.want+"\"\n") {
			t.Errorf("now %v: meta.yaml\n%s\nwant retrieved_at %s", tc.now, got, tc.want)
		}
	}
}

// TestFailuresBeforeTheXMLExitTwoAndStoreNothing: titles.json, the version
// list and the connection itself each fail the fetch (exit 2) before anything
// is stored.
func TestFailuresBeforeTheXMLExitTwoAndStoreNothing(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	importing := reply{body: strings.Replace(titlesJSON, `"import_in_progress":false`, `"import_in_progress":true`, 1)}
	future := strings.Replace(versions41, `{"date":"2017-01-01"`, `{"date":"2026-11-20"`, 1)
	for _, tc := range []struct {
		name  string
		uri   string
		reply reply
		args  []string
		want  string
	}{
		{"titles.json fails", apiPath + "/titles.json", reply{status: 503, body: "down"}, nil, "503"},
		{"eCFR is importing", apiPath + "/titles.json", importing, nil, "retry later"},
		{"eCFR is importing, with --date", apiPath + "/titles.json", importing, []string{"--date", "2025-05-15"}, "retry later"},
		{"--date after eCFR's up_to_date_as_of", "", reply{}, []string{"--date", "2026-11-20"}, "2026-11-19"},
		{"a latest version after eCFR's up_to_date_as_of", versionsURI("1024.41"), reply{body: future}, nil, "2026-11-19"},
		{"the version list fails", versionsURI("1024.41"), reply{status: 500, body: "boom"}, nil, "500"},
		{"the version list has no versions", versionsURI("1024.41"), reply{body: `{"content_versions":[]}`}, nil, "no versions"},
		{"the connection fails", "closed", reply{}, nil, closed.URL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := newFake(t)
			switch tc.uri {
			case "closed":
				c.BaseURL = closed.URL + apiPath
			case "":
			default:
				f.set(tc.uri, tc.reply)
			}
			f.set(xmlURI("2026-11-20", "1024.41"), reply{body: xmlBody("1024.41", "2026-11-20")})
			root := corpusRoot(t)
			before := tree(t, root)
			code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", append(tc.args, "1024.41")...)
			if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "regcorpus fetch: ") || !strings.Contains(stderr, tc.want) {
				t.Errorf("exit %d, stdout %q, stderr %q; want 2 and an error naming %q", code, stdout, stderr, tc.want)
			}
			if !equalTrees(before, tree(t, root)) {
				t.Errorf("files changed")
			}
			for _, r := range f.requests() {
				if strings.Contains(r.URL.Path, "/full/") {
					t.Errorf("requested %s after an earlier failure", r.URL)
				}
			}
		})
	}
}

// TestDateGuards: --date stores under .cache/ by the same rules as sources/:
// the body must be the section's XML, and a cached file is never overwritten.
func TestDateGuards(t *testing.T) {
	cached := ".cache/ecfr/title-12/1024.41/2025-05-15.xml"
	t.Run("a cached file with other bytes", func(t *testing.T) {
		_, c := newFake(t)
		root := corpusRoot(t)
		writeFile(t, root, cached, "<DIV8>other</DIV8>")
		code, _, stderr := fetchAt(t, root, c, "2026-11-22", "--date", "2025-05-15", "1024.41")
		if code != 1 || !strings.Contains(stderr, cached) {
			t.Errorf("exit %d, stderr %q; want 1 naming %s", code, stderr, cached)
		}
		if got := readFile(t, root, cached); got != "<DIV8>other</DIV8>" {
			t.Errorf("%s overwritten with %q", cached, got)
		}
	})
	t.Run("a body that isn't the section's XML", func(t *testing.T) {
		f, c := newFake(t)
		root := corpusRoot(t)
		f.set(xmlURI("2025-05-15", "1024.41"), reply{body: xmlBody("1024.30", "2025-05-15")})
		before := tree(t, root)
		code, _, stderr := fetchAt(t, root, c, "2026-11-22", "--date", "2025-05-15", "1024.41")
		if code != 2 || !strings.Contains(stderr, "root element") {
			t.Errorf("exit %d, stderr %q; want 2 naming the root element", code, stderr)
		}
		if !equalTrees(before, tree(t, root)) {
			t.Errorf("files changed")
		}
	})
}

// TestFetchTemporariesAndRawBytesAreSafeFromGit: git ignores the temporary
// files fetch writes beside a source (so an interrupted run can't be
// committed), and never rewrites raw XML (so a checkout or commit can't change
// L0 bytes). Needs the repository's git.
func TestFetchTemporariesAndRawBytesAreSafeFromGit(t *testing.T) {
	top := repoTop(t)
	temp := strings.Replace(tempPattern, "*", "123", 1)
	for _, p := range []string{
		"sources/ecfr/title-12/1024.41/raw/" + temp,
		"sources/ecfr/title-12/1024.41/" + temp,
		"sources/ecfr/title-12/supplement-i/41/raw/" + temp,
	} {
		if out, err := exec.Command("git", "-C", top, "check-ignore", "--no-index", "-q", p).CombinedOutput(); err != nil {
			t.Errorf("git check-ignore %s: %v %s; want it ignored", p, err, out)
		}
	}
	// Every file under sources/ is L0 whose bytes matter: the raw XML, the
	// meta.yaml fetch reads back (a CRLF checkout would make it unreadable),
	// and the tree.json, text.md and overrides.yaml later steps add.
	committed, err := exec.Command("git", "-C", top, "ls-files", "--", "sources/").Output()
	if err != nil {
		t.Fatal(err)
	}
	paths := strings.Fields(string(committed))
	var raw, meta int
	for _, p := range paths {
		switch {
		case strings.HasSuffix(p, ".xml"):
			raw++
		case strings.HasSuffix(p, "/meta.yaml"):
			meta++
		}
	}
	if raw == 0 || meta == 0 {
		t.Fatalf("committed under sources/: %v; want raw XML and meta.yaml", paths)
	}
	paths = append(paths, // a later source and later steps' files
		"sources/ecfr/title-12/supplement-i/41/raw/2025-01-01.xml",
		"sources/ecfr/title-12/supplement-i/41/meta.yaml",
		"sources/ecfr/title-12/1024.41/tree.json",
		"sources/ecfr/title-12/1024.41/text.md",
		"sources/ecfr/title-12/1024.41/overrides.yaml")
	// Every attribute by which git can rewrite bytes on checkout or commit is
	// unset: end-of-line conversion, clean/smudge filters, $Id$ expansion and
	// re-encoding.
	attrs := []string{"text", "filter", "ident", "working-tree-encoding"}
	for _, p := range paths {
		out, err := exec.Command("git", append([]string{"-C", top, "check-attr"}, append(attrs, "--", p)...)...).Output()
		if err != nil {
			t.Fatal(err)
		}
		want := ""
		for _, a := range attrs {
			want += p + ": " + a + ": unset\n"
		}
		if string(out) != want {
			t.Errorf("git check-attr: %q, want %q", out, want)
		}
	}
}

// repoTop is the repository's top directory, or skips the test when git or
// the repository isn't there (a source tarball).
func repoTop(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not in a git repository")
	}
	return strings.TrimSpace(string(out))
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
