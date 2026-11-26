// Tests for `regcorpus fetch --latest` (R1b; plan step 6, ADR 0014),
// offline against the fake eCFR: it exits 0 when eCFR lists no version of a
// stored section dated after its source_version_date, 1 when it does or
// when the latest version removed it (F7: L1, report only), 2 on usage,
// network or eCFR errors; it reads the version lists only and writes
// nothing; a reissue of the stored date since it was fetched is printed as
// information (F1: A).
package ecfr

import (
	"net/http"
	"strings"
	"testing"
)

func latestAt(t *testing.T, root string, c *Client, args ...string) (int, string, string) {
	t.Helper()
	return fetchAt(t, root, c, "2026-11-25", append([]string{"--latest"}, args...)...)
}

// storedAll puts the three sections' committed raw XML and meta.yaml in a
// new corpus root (1024.41's tree.json and text.md too).
func storedAll(t *testing.T) string {
	t.Helper()
	root := corpusRoot(t)
	stored41(t, root, nil)
	storedSource(t, root, "1024.30")
	storedSource(t, root, "1024.31")
	return root
}

func TestLatestFindsNoNewerVersionAndWritesNothing(t *testing.T) {
	f, c := newFake(t)
	root := storedAll(t)
	before := tree(t, root)
	code, stdout, stderr := latestAt(t, root, c)
	want := "1024.41: stored source_version_date 2025-07-15, eCFR's latest 2025-07-15: no newer version\n" +
		"1024.30: stored source_version_date 2018-04-19, eCFR's latest 2018-04-19: no newer version\n" +
		"1024.31: stored source_version_date 2025-07-15, eCFR's latest 2025-07-15: no newer version\n"
	if code != 0 || stdout != want || stderr != "" {
		t.Fatalf("exit %d, stdout\n%s\nstderr %q; want 0 and\n%s", code, stdout, stderr, want)
	}
	if after := tree(t, root); !equalTrees(before, after) {
		t.Errorf("files changed: before %v, after %v", keysOf(before), keysOf(after))
	}
	for _, r := range f.requests() {
		if !strings.HasPrefix(r.URL.Path, apiPath+"/versions/") && r.URL.Path != apiPath+"/titles.json" {
			t.Errorf("asked %s; want only titles.json and the version lists", r.URL)
		}
	}
}

// TestLatestFailsOneVersionBack is R1b's acceptance fixture: a stored
// version one version back exits non-zero, naming the newer date.
func TestLatestFailsOneVersionBack(t *testing.T) {
	_, c := newFake(t)
	root := corpusRoot(t)
	body := xmlBody("1024.41", "2025-05-16")
	writeFile(t, root, "sources/ecfr/title-12/1024.41/raw/2025-05-16.xml", body)
	writeFile(t, root, metaPath("1024.41"), fetchMeta("2025-05-16", sha(body)))
	before := tree(t, root)
	code, stdout, stderr := latestAt(t, root, c, "1024.41")
	if want := "1024.41: stored source_version_date 2025-05-16, eCFR's latest 2025-07-15: newer versions 2025-07-15\n"; code != 1 || stdout != want {
		t.Errorf("exit %d, stdout %q; want 1 and %q", code, stdout, want)
	}
	if !strings.HasPrefix(stderr, "regcorpus fetch: 1024.41: ") || !strings.Contains(stderr, "2025-07-15") || !strings.Contains(stderr, "R1b") {
		t.Errorf("stderr %q; want an error naming the newer version and R1b", stderr)
	}
	if after := tree(t, root); !equalTrees(before, after) {
		t.Errorf("files changed")
	}
}

func TestLatestListsEveryNewerDateOnce(t *testing.T) {
	f, c := newFake(t)
	root := corpusRoot(t)
	body := xmlBody("1024.41", "2021-08-31")
	writeFile(t, root, "sources/ecfr/title-12/1024.41/raw/2021-08-31.xml", body)
	writeFile(t, root, metaPath("1024.41"), fetchMeta("2021-08-31", sha(body)))
	f.set(versionsURI("1024.41"), reply{body: strings.Replace(versions41, `],"meta"`,
		`,{"date":"2025-05-16","issue_date":"2025-06-01","identifier":"1024.41","removed":false}],"meta"`, 1)})
	code, stdout, _ := latestAt(t, root, c, "1024.41")
	if want := "1024.41: stored source_version_date 2021-08-31, eCFR's latest 2025-07-15: newer versions 2025-05-16, 2025-07-15\n"; code != 1 || stdout != want {
		t.Errorf("exit %d, stdout %q; want 1 and %q", code, stdout, want)
	}
}

func TestLatestFailsWhenTheLatestRemovedIt(t *testing.T) {
	f, c := newFake(t)
	root := storedAll(t)
	f.set(versionsURI("1024.31"), reply{body: strings.Replace(versions31, `],"meta"`,
		`,{"date":"2026-02-01","issue_date":"2026-02-01","identifier":"1024.31","removed":true}],"meta"`, 1)})
	code, stdout, stderr := latestAt(t, root, c)
	if code != 1 || !strings.Contains(stderr, "regcorpus fetch: 1024.31: ") || !strings.Contains(stderr, "removed it") {
		t.Errorf("exit %d, stderr %q; want 1 and an error saying the latest version removed 1024.31", code, stderr)
	}
	if !strings.Contains(stdout, "1024.31: stored source_version_date 2025-07-15, eCFR's latest 2026-02-01: newer versions 2026-02-01 (removed)\n") {
		t.Errorf("stdout %q lacks 1024.31's removal", stdout)
	}
}

// TestLatestNotesAReissueOfTheStoredDate: an entry for the stored date
// issued after retrieved_at is printed as information and doesn't change
// the exit code (F1: A); one issued before it isn't printed.
func TestLatestNotesAReissueOfTheStoredDate(t *testing.T) {
	for _, tc := range []struct {
		issued string
		note   bool
	}{{"2026-11-24", true}, {"2026-11-22", false}, {"2026-11-01", false}} {
		f, c := newFake(t)
		root := storedAll(t) // 1024.41's meta.yaml: retrieved_at 2026-11-22
		f.set(versionsURI("1024.41"), reply{body: strings.Replace(versions41, `],"meta"`,
			`,{"date":"2025-07-15","issue_date":"`+tc.issued+`","identifier":"1024.41","substantive":false,"removed":false}],"meta"`, 1)})
		code, stdout, stderr := latestAt(t, root, c, "1024.41")
		note := "1024.41: information: eCFR lists 2025-07-15 reissued on " + tc.issued + ", after retrieved_at 2026-11-22; regcorpus fetch --verify 1024.41 compares its text\n"
		if code != 0 || stderr != "" || strings.Contains(stdout, note) != tc.note {
			t.Errorf("issued %s: exit %d, stdout %q, stderr %q; want 0, the note %t", tc.issued, code, stdout, stderr, tc.note)
		}
	}
}

func TestLatestFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		uri    string
		reply  reply
		root   func(*testing.T) string
		code   int
		stderr string
	}{
		{"an import in progress", apiPath + "/titles.json", reply{body: strings.Replace(titlesJSON, `"import_in_progress":false`, `"import_in_progress":true`, 1)},
			storedAll, 2, "import in progress"},
		{"a version list down", versionsURI("1024.30"), reply{status: http.StatusInternalServerError, body: "down", raw: true},
			storedAll, 2, "500"},
		{"no meta.yaml", "", reply{}, corpusRoot, 1, "run regcorpus fetch first"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := newFake(t)
			if tc.uri != "" {
				f.set(tc.uri, tc.reply)
			}
			root := tc.root(t)
			before := tree(t, root)
			code, _, stderr := latestAt(t, root, c)
			if code != tc.code || !strings.Contains(stderr, tc.stderr) {
				t.Errorf("exit %d, stderr %q; want %d and %q", code, stderr, tc.code, tc.stderr)
			}
			if after := tree(t, root); !equalTrees(before, after) {
				t.Errorf("files changed")
			}
		})
	}
}
