// Tests for `regcorpus fetch --verify` (R1; plan step 6, ADR 0014),
// offline against the fake eCFR and govinfo in client_test.go and
// granule_test.go: the version date, tree and sha256_text compared with a
// fresh fetch (sha256_raw information only), then the normalized text with
// the govinfo 2026 edition, and verified_on written through l0.Update; the
// exit codes 0 matched, 1 a finding or a refusal (a govinfo mismatch writes
// null), 2 usage, network, eCFR or govinfo errors.
package ecfr

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
)

const dir41 = "sources/ecfr/title-12/1024.41/"

// stored41 puts 1024.41's committed raw XML, tree.json and text.md into
// root, and its committed meta.yaml with verified_on set to v (nil:
// absent), and returns that meta.yaml.
func stored41(t *testing.T, root string, v *meta.NullDate) string {
	t.Helper()
	m, err := meta.Parse(repoFile(t, metaPath("1024.41")))
	if err != nil {
		t.Fatal(err)
	}
	m.VerifiedOn = v
	b, err := meta.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{rawPath("1024.41"), dir41 + "tree.json", dir41 + "text.md"} {
		writeFile(t, root, p, string(repoFile(t, p)))
	}
	writeFile(t, root, metaPath("1024.41"), string(b))
	return string(b)
}

// withVerifiedOn is meta.yaml b with line added after ecfr_up_to_date_as_of
// (spec order: verified_on comes before sha256_raw).
func withVerifiedOn(t *testing.T, b, line string) string {
	t.Helper()
	i := strings.Index(b, "sha256_raw: ")
	if i < 0 {
		t.Fatalf("no sha256_raw in %q", b)
	}
	return b[:i] + line + "\n" + b[i:]
}

// verifyFake is newFake serving 1024.41's committed raw XML at its stored
// date and govinfo's committed 2026 granule of 1024.41 behind the link
// service, as govinfo served them.
func verifyFake(t *testing.T) (*fakeECFR, *Client) {
	t.Helper()
	f, c := newFake(t)
	f.set(xmlURI("2025-07-15", "1024.41"), reply{body: string(repoFile(t, rawPath("1024.41")))})
	f.set(govLink41, govRedirect(govContent41))
	f.set(govContent41, reply{body: string(repoFile(t, granule41))})
	return f, c
}

func verifyAt(t *testing.T, root string, c *Client, day string, args ...string) (int, string, string) {
	t.Helper()
	return fetchAt(t, root, c, day, append([]string{"--verify"}, args...)...)
}

var (
	text41 = "ecda50e7ce958e2fd19a8f1843b3c60bde0dcf533cc2c313cb9c1e1960760fe1"
	raw41  = "3b69726d99fb64837c2f0767f257cff830a1d3602f16a9749615c31f9ace840b"
)

// cache41 is where --verify keeps the granule with sha256 sum.
func cache41(sum string) string { return ".cache/govinfo/2026/title-12/1024.41/" + sum + ".xml" }

// verifyLines is what --verify 1024.41 prints when everything matches:
// cacheVerb for the granule's cache file, metaVerb and verified for
// meta.yaml.
func verifyLines(t *testing.T, c *Client, cacheVerb, metaVerb, verified string) string {
	g := l0.SHA256Hex(repoFile(t, granule41))
	return "1024.41: version: eCFR's latest 2025-07-15, stored source_version_date 2025-07-15: equal\n" +
		"1024.41: tree: equal (91 nodes: the same paths in the same order, each with the same sha256)\n" +
		"1024.41: sha256_text: equal, " + text41 + "\n" +
		"1024.41: sha256_raw: fresh " + raw41 + ", stored " + raw41 + " (information only)\n" +
		"1024.41: overrides: none\n" +
		"1024.41: govinfo: " + c.Govinfo + govContent41 + " (DATE 2026-01-01, sha256 " + g + ")\n" +
		"1024.41: govinfo: " + cacheVerb + " " + cache41(g) + "\n" +
		"1024.41: govinfo 2026 edition: normalized text matches sha256_text\n" +
		"1024.41: " + metaVerb + " " + metaPath("1024.41") + ": verified_on: " + verified + "\n"
}

// sourcesOf is tree(t, root) restricted to sources/: what --verify may
// change.
func sourcesOf(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for p, v := range tree(t, root) {
		if strings.HasPrefix(p, "sources/") {
			out[p] = v
		}
	}
	return out
}

// asked reports whether any request's path starts with prefix.
func asked(f *fakeECFR, prefix string) bool {
	for _, r := range f.requests() {
		if strings.HasPrefix(r.URL.Path, prefix) {
			return true
		}
	}
	return false
}

// TestVerifyWritesOneLineThenChangesNothing: a match writes verified_on,
// the UTC date of the run, as one added line of meta.yaml; a later run that
// still matches prints "unchanged" and changes no byte (F5: V-a).
func TestVerifyWritesOneLineThenChangesNothing(t *testing.T) {
	_, c := verifyFake(t)
	root := corpusRoot(t)
	before := stored41(t, root, nil)
	code, stdout, stderr := verifyAt(t, root, c, "2026-11-25", "1024.41")
	if want := verifyLines(t, c, "stored", "wrote", `"2026-11-25"`); code != 0 || stdout != want || stderr != "" {
		t.Fatalf("exit %d, stdout\n%s\nstderr %q; want 0 and\n%s", code, stdout, stderr, want)
	}
	if got, want := readFile(t, root, metaPath("1024.41")), withVerifiedOn(t, before, `verified_on: "2026-11-25"`); got != want {
		t.Fatalf("meta.yaml\n%s\nwant one added line\n%s", got, want)
	}
	srcs := sourcesOf(t, root)
	code, stdout, stderr = verifyAt(t, root, c, "2026-12-03", "1024.41")
	if want := verifyLines(t, c, "unchanged", "unchanged", `"2026-11-25"`); code != 0 || stdout != want || stderr != "" {
		t.Fatalf("second run: exit %d, stdout\n%s\nstderr %q; want 0 and\n%s", code, stdout, stderr, want)
	}
	if after := sourcesOf(t, root); !equalTrees(srcs, after) {
		t.Errorf("the second run changed files under sources/")
	}
}

// TestVerifyKeepsADateAndReplacesNull: a match keeps a date already there
// and replaces null (F5: V-a).
func TestVerifyKeepsADateAndReplacesNull(t *testing.T) {
	for _, tc := range []struct {
		name     string
		v        *meta.NullDate
		verb     string
		verified string
	}{
		{"a date", &meta.NullDate{Date: "2026-11-20", Valid: true}, "unchanged", `"2026-11-20"`},
		{"null", &meta.NullDate{}, "wrote", `"2026-11-25"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, c := verifyFake(t)
			root := corpusRoot(t)
			stored41(t, root, tc.v)
			code, stdout, stderr := verifyAt(t, root, c, "2026-11-25", "1024.41")
			if want := verifyLines(t, c, "stored", tc.verb, tc.verified); code != 0 || stdout != want || stderr != "" {
				t.Fatalf("exit %d, stdout\n%s\nstderr %q; want 0 and\n%s", code, stdout, stderr, want)
			}
			if got := readFile(t, root, metaPath("1024.41")); !strings.Contains(got, "verified_on: "+tc.verified+"\n") {
				t.Errorf("meta.yaml\n%s\nwant verified_on: %s", got, tc.verified)
			}
		})
	}
}

// TestVerifyPassesAByteOnlyReissue: a fresh fetch whose bytes differ but
// whose normalized text is the same is the same version (F1: A): both
// sha256_raw values and the nodes whose raw text differs are printed as
// information, and verified_on is written.
func TestVerifyPassesAByteOnlyReissue(t *testing.T) {
	f, c := verifyFake(t)
	fresh := strings.Replace(string(repoFile(t, rawPath("1024.41"))), "shall exercise reasonable diligence", "shall  exercise reasonable"+string(rune(0xA0))+"diligence", 1)
	f.set(xmlURI("2025-07-15", "1024.41"), reply{body: fresh})
	root := corpusRoot(t)
	stored41(t, root, nil)
	code, stdout, stderr := verifyAt(t, root, c, "2026-11-25", "1024.41")
	for _, want := range []string{
		"1024.41: tree: equal (91 nodes: the same paths in the same order, each with the same sha256)\n",
		"1024.41: sha256_text: equal, " + text41 + "\n",
		"1024.41: sha256_raw: fresh " + l0.SHA256Hex([]byte(fresh)) + ", stored " + raw41 + " (information only)\n",
		"1024.41: raw text differs, normalized alike (information only): 1024-41-b-1\n",
		"1024.41: wrote " + metaPath("1024.41") + ": verified_on: \"2026-11-25\"\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q", want)
		}
	}
	if code != 0 || stderr != "" {
		t.Errorf("exit %d, stderr %q; want 0 and empty\n%s", code, stderr, stdout)
	}
	if got := readFile(t, root, rawPath("1024.41")); got != string(repoFile(t, rawPath("1024.41"))) {
		t.Errorf("the stored raw XML changed")
	}
}

// TestVerifyEcfrMismatchesWriteNothing: a newer version, a removal, or a
// fresh fetch whose normalized text changed exits 1, names what differs
// (the changed nodes), writes nothing, and never asks govinfo (F4 c, F6).
func TestVerifyEcfrMismatchesWriteNothing(t *testing.T) {
	newer := strings.Replace(versions41, `],"meta"`, `,{"date":"2026-01-10","amendment_date":"2026-01-10","issue_date":"2026-01-10","identifier":"1024.41","substantive":true,"removed":false,"type":"section"}],"meta"`, 1)
	removed := strings.Replace(newer, `"date":"2026-01-10","amendment_date":"2026-01-10","issue_date":"2026-01-10","identifier":"1024.41","substantive":true,"removed":false`,
		`"date":"2026-01-10","amendment_date":"2026-01-10","issue_date":"2026-01-10","identifier":"1024.41","substantive":true,"removed":true`, 1)
	raw := string(repoFile(t, rawPath("1024.41")))
	for _, tc := range []struct {
		name   string
		uri    string
		reply  reply
		stdout []string
		stderr []string
		xml    bool // whether the eCFR XML is fetched
	}{
		{"a newer version", versionsURI("1024.41"), reply{body: newer},
			[]string{"1024.41: version: eCFR's latest 2026-01-10, stored source_version_date 2025-07-15: differ\n"},
			[]string{"2026-01-10", "2025-07-15", "fetch --latest"}, false},
		{"the latest removed it", versionsURI("1024.41"), reply{body: removed}, nil,
			[]string{"removed it"}, false},
		{"a word changed", xmlURI("2025-07-15", "1024.41"), reply{body: strings.Replace(raw, "application 45 days or more", "application 46 days or more", 1)},
			[]string{"1024.41: tree: differs: 1024-41-b-2-i changed\n", "1024.41: sha256_text: differs, fresh "},
			[]string{"1024-41-b-2-i changed", "nothing is written"}, true},
		{"a paragraph added", xmlURI("2025-07-15", "1024.41"), reply{body: strings.Replace(raw, "<CITA", "<P>(l) Added.</P>\n<CITA", 1)},
			[]string{"1024.41: tree: differs: 1024-41-l added\n"},
			[]string{"1024-41-l added"}, true},
		{"a fresh fetch that doesn't parse", xmlURI("2025-07-15", "1024.41"), reply{body: strings.Replace(raw, "(b) <I>Receipt", "(c) <I>Receipt", 1)},
			nil, []string{"parse: line"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := verifyFake(t)
			f.set(tc.uri, tc.reply)
			root := corpusRoot(t)
			stored41(t, root, &meta.NullDate{Date: "2026-11-20", Valid: true})
			srcs := tree(t, root)
			code, stdout, stderr := verifyAt(t, root, c, "2026-11-25", "1024.41")
			if code != 1 || !strings.HasPrefix(stderr, "regcorpus fetch: 1024.41: ") {
				t.Errorf("exit %d, stderr %q; want 1 and an error for 1024.41", code, stderr)
			}
			for _, w := range tc.stdout {
				if !strings.Contains(stdout, w) {
					t.Errorf("stdout %q lacks %q", stdout, w)
				}
			}
			for _, w := range tc.stderr {
				if !strings.Contains(stderr, w) {
					t.Errorf("stderr %q lacks %q", stderr, w)
				}
			}
			if strings.Contains(stdout, "wrote") || strings.Contains(stdout, "govinfo") {
				t.Errorf("stdout %q; want no govinfo line and nothing written", stdout)
			}
			if after := tree(t, root); !equalTrees(srcs, after) {
				t.Errorf("files changed: before %v, after %v", keysOf(srcs), keysOf(after))
			}
			if asked(f, "/link/") || asked(f, "/content/") {
				t.Errorf("govinfo was asked; want it never asked after an eCFR mismatch (F4 c)")
			}
			if got := asked(f, apiPath+"/full/"); got != tc.xml {
				t.Errorf("eCFR XML fetched: %t, want %t", got, tc.xml)
			}
		})
	}
}

// TestVerifyGovinfoMismatchWritesNull: a granule whose normalized text
// differs by one word writes verified_on: null over an earlier date, prints
// each differing paragraph, and exits 1 (F4 a, b; F8: E-a); a re-run that
// still differs leaves null as it is.
func TestVerifyGovinfoMismatchWritesNull(t *testing.T) {
	f, c := verifyFake(t)
	g := strings.Replace(string(repoFile(t, granule41)), "application 45 days or more", "application 46 days or more", 1)
	f.set(govContent41, reply{body: g})
	root := corpusRoot(t)
	before := stored41(t, root, &meta.NullDate{Date: "2026-11-20", Valid: true})
	for i, verb := range []string{"wrote", "unchanged"} {
		code, stdout, stderr := verifyAt(t, root, c, "2026-11-25", "1024.41")
		for _, want := range []string{
			"1024.41: govinfo 2026 edition: normalized text differs from sha256_text (govinfo's ",
			"1024.41: govinfo differs at paragraph 3 of 75:\n",
			`1024.41:   eCFR:    "(2) Review of loss mitigation application submission-(i) Requirements. If a servicer receives a loss mitigation application 45 days or more`,
			`1024.41:   govinfo: "(2) Review of loss mitigation application submission-(i) Requirements. If a servicer receives a loss mitigation application 46 days or more`,
			"1024.41: " + verb + " " + metaPath("1024.41") + ": verified_on: null\n",
		} {
			if !strings.Contains(stdout, want) {
				t.Errorf("run %d: stdout lacks %q\n%s", i+1, want, stdout)
			}
		}
		if strings.Count(stdout, "govinfo differs at") != 1 {
			t.Errorf("run %d: want exactly one differing paragraph\n%s", i+1, stdout)
		}
		if code != 1 || !strings.Contains(stderr, "verified_on") || !strings.Contains(stderr, "null") {
			t.Errorf("run %d: exit %d, stderr %q; want 1 and an error saying verified_on is null", i+1, code, stderr)
		}
		if got, want := readFile(t, root, metaPath("1024.41")), strings.Replace(before, `verified_on: "2026-11-20"`, "verified_on: null", 1); got != want {
			t.Errorf("run %d: meta.yaml\n%s\nwant\n%s", i+1, got, want)
		}
	}
}

// TestVerifyMatchesAGranuleDifferingOnlyInLayout: line breaks and their
// indentation moved, added or removed (the layout rule, F3: C1) change
// nothing; the cache keeps the granule as served.
func TestVerifyMatchesAGranuleDifferingOnlyInLayout(t *testing.T) {
	f, c := verifyFake(t)
	g := string(repoFile(t, granule41))
	g = strings.Replace(g, "A borrower may enforce", "A borrower \n            may enforce", 1)
	g = strings.Replace(g, "(a) \n            <E T=\"03\">Enforcement", "(a) <E T=\"03\">Enforcement", 1)
	g = strings.Replace(g, "loss mitigation", "loss miti\ngation", 1)
	f.set(govContent41, reply{body: g})
	root := corpusRoot(t)
	stored41(t, root, nil)
	code, stdout, stderr := verifyAt(t, root, c, "2026-11-25", "1024.41")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "normalized text matches sha256_text\n") {
		t.Errorf("exit %d, stdout\n%s\nstderr %q; want 0 and a match", code, stdout, stderr)
	}
	if got := readFile(t, root, cache41(l0.SHA256Hex([]byte(g)))); got != g {
		t.Errorf("the cached granule is not the bytes served")
	}
}

// TestVerifyGovinfoErrorsWriteNothing: a granule of another section or
// edition, one that isn't a granule, or a govinfo HTTP error exits 2 with
// nothing written under sources/.
func TestVerifyGovinfoErrorsWriteNothing(t *testing.T) {
	g := string(repoFile(t, granule41))
	for _, tc := range []struct {
		name  string
		uri   string
		reply reply
		want  string
	}{
		{"another section", govContent41, reply{body: strings.Replace(g, "<SECTNO>§ 1024.41</SECTNO>", "<SECTNO>§ 1024.40</SECTNO>", 1)}, "another section"},
		{"another edition", govContent41, reply{body: strings.Replace(g, "<DATE>2026-01-01</DATE>", "<DATE>2025-01-01</DATE>", 1)}, "another edition"},
		{"not UTF-8", govContent41, reply{body: strings.Replace(g, "Loss mitigation procedures.", "Loss \xff", 1)}, "not a govinfo"},
		{"eCFR's XML", govContent41, reply{body: string(repoFile(t, rawPath("1024.41")))}, "not a govinfo"},
		{"no granule", govContent41, reply{status: http.StatusNotFound, body: "not found", raw: true}, "404"},
		{"the link service down", govLink41, reply{status: http.StatusServiceUnavailable, body: "down", raw: true}, "503"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := verifyFake(t)
			f.set(tc.uri, tc.reply)
			root := corpusRoot(t)
			stored41(t, root, &meta.NullDate{Date: "2026-11-20", Valid: true})
			srcs := sourcesOf(t, root)
			code, stdout, stderr := verifyAt(t, root, c, "2026-11-25", "1024.41")
			if code != 2 || !strings.Contains(stderr, tc.want) || strings.Contains(stdout, "wrote") {
				t.Errorf("exit %d, stdout %q, stderr %q; want 2, an error naming %q, nothing written", code, stdout, stderr, tc.want)
			}
			if after := sourcesOf(t, root); !equalTrees(srcs, after) {
				t.Errorf("files under sources/ changed")
			}
		})
	}
}

// TestVerifyRefusalsWriteNothingAndAskNothing: a stored state verify
// can't compare (an overrides.yaml, no sha256_text, no tree.json, no
// meta.yaml) and the sources whose tree is plan step 9's exit 1 with
// nothing written, before any request but titles.json.
func TestVerifyRefusalsWriteNothingAndAskNothing(t *testing.T) {
	noText := func(t *testing.T, root string) {
		m, err := meta.Parse([]byte(stored41(t, root, nil)))
		if err != nil {
			t.Fatal(err)
		}
		m.SHA256Text = ""
		b, _ := meta.Marshal(m)
		writeFile(t, root, metaPath("1024.41"), string(b))
	}
	for _, tc := range []struct {
		name    string
		setup   func(*testing.T, string)
		section string
		want    string
	}{
		{"an overrides.yaml", func(t *testing.T, root string) {
			stored41(t, root, nil)
			writeFile(t, root, dir41+"overrides.yaml", "")
		}, "1024.41", "overrides.yaml"},
		{"no sha256_text", noText, "1024.41", "regcorpus parse"},
		{"no tree.json", func(t *testing.T, root string) {
			stored41(t, root, nil)
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(dir41+"tree.json"))); err != nil {
				t.Fatal(err)
			}
		}, "1024.41", "tree.json"},
		{"no meta.yaml", func(t *testing.T, root string) {}, "1024.41", "regcorpus fetch first"},
		{"1024.30", func(t *testing.T, root string) { storedSource(t, root, "1024.30") }, "1024.30", "tree and sha256_text are plan step 9's"},
		{"1024.31", func(t *testing.T, root string) { storedSource(t, root, "1024.31") }, "1024.31", "tree and sha256_text are plan step 9's"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := verifyFake(t)
			root := corpusRoot(t)
			tc.setup(t, root)
			before := tree(t, root)
			code, stdout, stderr := verifyAt(t, root, c, "2026-11-25", tc.section)
			if code != 1 || !strings.Contains(stderr, tc.want) || strings.Contains(stdout, "wrote") {
				t.Errorf("exit %d, stdout %q, stderr %q; want 1, an error naming %q, nothing written", code, stdout, stderr, tc.want)
			}
			if after := tree(t, root); !equalTrees(before, after) {
				t.Errorf("files changed: before %v, after %v", keysOf(before), keysOf(after))
			}
			for _, r := range f.requests() {
				if r.URL.Path != apiPath+"/titles.json" {
					t.Errorf("asked %s; want only titles.json before a refusal", r.URL)
				}
			}
		})
	}
}

// TestVerifyEverySourceExitsOneUntilStepNine: with no section, --verify
// covers every source: 1024.41 is verified and written, 1024.30 and
// 1024.31 are refused, so it exits 1 (stage 01's end check, F10).
func TestVerifyEverySourceExitsOneUntilStepNine(t *testing.T) {
	_, c := verifyFake(t)
	root := corpusRoot(t)
	stored41(t, root, nil)
	storedSource(t, root, "1024.30")
	storedSource(t, root, "1024.31")
	code, stdout, stderr := verifyAt(t, root, c, "2026-11-25")
	if want := verifyLines(t, c, "stored", "wrote", `"2026-11-25"`); code != 1 || stdout != want {
		t.Errorf("exit %d, stdout\n%s\nwant 1 and\n%s", code, stdout, want)
	}
	for _, id := range []string{"1024.30", "1024.31"} {
		if !strings.Contains(stderr, "regcorpus fetch: "+id+": tree and sha256_text are plan step 9's") {
			t.Errorf("stderr %q lacks %s's refusal", stderr, id)
		}
	}
}

// TestVerifyFailuresBeforeTheGranuleExitTwo: eCFR mid-import, or an eCFR
// HTTP error, exits 2 with nothing written.
func TestVerifyFailuresBeforeTheGranuleExitTwo(t *testing.T) {
	for _, tc := range []struct {
		name, uri string
		reply     reply
	}{
		{"an import in progress", apiPath + "/titles.json", reply{body: strings.Replace(titlesJSON, `"import_in_progress":false`, `"import_in_progress":true`, 1)}},
		{"the version list down", versionsURI("1024.41"), reply{status: http.StatusInternalServerError, body: "down", raw: true}},
		{"no XML", xmlURI("2025-07-15", "1024.41"), reply{status: http.StatusNotFound, body: "{}", raw: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := verifyFake(t)
			f.set(tc.uri, tc.reply)
			root := corpusRoot(t)
			stored41(t, root, nil)
			before := tree(t, root)
			code, stdout, stderr := verifyAt(t, root, c, "2026-11-25", "1024.41")
			if code != 2 || stderr == "" || strings.Contains(stdout, "wrote") {
				t.Errorf("exit %d, stdout %q, stderr %q; want 2 and nothing written", code, stdout, stderr)
			}
			if after := tree(t, root); !equalTrees(before, after) {
				t.Errorf("files changed")
			}
			if asked(f, "/link/") {
				t.Errorf("govinfo was asked")
			}
		})
	}
}

func TestVerifyAndLatestAreUsedAlone(t *testing.T) {
	for _, args := range [][]string{
		{"--verify", "--latest"},
		{"--latest", "--verify"},
		{"--verify", "--date", "2025-05-15"},
		{"--date", "2025-05-15", "--latest"},
		{"1024.41", "--verify"},
	} {
		f, c := verifyFake(t)
		root := corpusRoot(t)
		code, stdout, stderr := fetchAt(t, root, c, "2026-11-25", args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "usage: regcorpus fetch") {
			t.Errorf("%q: exit %d, stdout %q, stderr %q; want 2 and usage", args, code, stdout, stderr)
		}
		if n := len(f.requests()); n != 0 {
			t.Errorf("%q: %d requests, want 0", args, n)
		}
	}
}

// TestFetchHelpGivesVerifyAndLatest: -h documents both flags and the exit
// codes of each (F8).
func TestFetchHelpGivesVerifyAndLatest(t *testing.T) {
	code, stdout, _ := fetchAt(t, corpusRoot(t), nil, "2026-11-25", "-h")
	for _, want := range []string{
		"usage: regcorpus fetch [--date YYYY-MM-DD | --verify | --latest] [section ...]",
		"--verify",
		"--latest",
		"verified_on",
		"govinfo",
		"normalized text",
		"layout",
		"information only",
		"writes verified_on: null and exits 1",
		"Supplement I",
		"plan step 8",
		"plan step 9",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("fetch -h lacks %q", want)
		}
	}
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
}
