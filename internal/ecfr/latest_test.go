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
	const line = "stored source_version_date 2025-07-15, eCFR's latest 2026-02-01 (removed): newer versions 2026-02-01 (removed)"
	if !strings.Contains(stdout, "1024.31: "+line+"\n") {
		t.Errorf("stdout %q lacks 1024.31's removal", stdout)
	}
	if want := "regcorpus fetch: 1024.31: " + line + " (R1b); nothing is written\n"; code != 1 || stderr != want {
		t.Errorf("exit %d, stderr %q; want 1 and %q, what stdout says", code, stderr, want)
	}
}

// TestLatestReadsEcfrsLatestFromTheList: eCFR's latest is the list's own
// newest date, never the stored one, so each fact printed is true, and
// stdout and stderr say one thing (worklist round 1, §5: B2, B6b). A list
// whose newest date is before the stored one, or whose entry at the
// stored date removed the section, exits 1 and writes nothing.
func TestLatestReadsEcfrsLatestFromTheList(t *testing.T) {
	const entry41 = `{"date":"2025-07-15","amendment_date":"2025-07-15","issue_date":"2025-07-15","identifier":"1024.41","substantive":true,"removed":false,"type":"section"},`
	for _, tc := range []struct{ name, list, line string }{
		{"the newest before the stored date", strings.Replace(versions41, entry41, "", 1),
			"stored source_version_date 2025-07-15, eCFR's latest 2025-05-16: eCFR no longer lists the stored version as latest"},
		{"the stored date's entry removed it", strings.Replace(versions41, `],"meta"`,
			`,{"date":"2025-07-15","issue_date":"2025-08-01","identifier":"1024.41","removed":true}],"meta"`, 1),
			"stored source_version_date 2025-07-15, eCFR's latest 2025-07-15 (removed): the latest entry, at the stored date, removed the section"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.list == versions41 {
				t.Fatal("the case doesn't change the list")
			}
			f, c := newFake(t)
			f.set(versionsURI("1024.41"), reply{body: tc.list})
			root := storedAll(t)
			before := tree(t, root)
			code, stdout, stderr := latestAt(t, root, c, "1024.41")
			if code != 1 || stdout != "1024.41: "+tc.line+"\n" || stderr != "regcorpus fetch: 1024.41: "+tc.line+" (R1b); nothing is written\n" {
				t.Errorf("exit %d, stdout %q, stderr %q; want 1 and %q on both", code, stdout, stderr, tc.line)
			}
			if !equalTrees(before, tree(t, root)) {
				t.Errorf("files changed")
			}
		})
	}
}

// TestLatestChecksTheStoredStateFirst: --latest reads a meta.yaml only
// after the check every writer of one makes (l0's Stored), so one that
// doesn't describe its raw file exits 1 before its version list is asked.
func TestLatestChecksTheStoredStateFirst(t *testing.T) {
	f, c := newFake(t)
	root := storedAll(t)
	writeFile(t, root, rawPath("1024.41"), "<DIV8>other bytes</DIV8>")
	code, stdout, stderr := latestAt(t, root, c, "1024.41")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "is not the raw file") {
		t.Errorf("exit %d, stdout %q, stderr %q; want 1 and the stored-state refusal", code, stdout, stderr)
	}
	if asked(f, apiPath+"/versions/") {
		t.Errorf("the version list was asked")
	}
}

// TestLatestNotesAReissueOfTheStoredDate: an entry for the stored date
// issued on or after the day retrieved_at names, a UTC date, is printed as
// information and doesn't change the exit code (F1: A); one issued before
// it isn't. An issue date on retrieved_at's day may be before or after the
// fetch, which no date says, so it is noted (worklist round 1, §8).
func TestLatestNotesAReissueOfTheStoredDate(t *testing.T) {
	for _, tc := range []struct {
		issued string
		note   bool
	}{{"2026-11-24", true}, {"2026-11-22", true}, {"2026-11-21", false}, {"2026-11-01", false}} {
		f, c := newFake(t)
		root := storedAll(t) // 1024.41's meta.yaml: retrieved_at 2026-11-22
		f.set(versionsURI("1024.41"), reply{body: strings.Replace(versions41, `],"meta"`,
			`,{"date":"2025-07-15","issue_date":"`+tc.issued+`","identifier":"1024.41","substantive":false,"removed":false}],"meta"`, 1)})
		code, stdout, stderr := latestAt(t, root, c, "1024.41")
		note := "1024.41: information: eCFR lists 2025-07-15 with issue_date " + tc.issued + ", issued on or after the day the stored file was fetched (retrieved_at 2026-11-22); run regcorpus fetch --verify 1024.41\n"
		if code != 0 || stderr != "" || strings.Contains(stdout, note) != tc.note {
			t.Errorf("issued %s: exit %d, stdout %q, stderr %q; want 0, the note %t", tc.issued, code, stdout, stderr, tc.note)
		}
	}
}

// TestLatestNotesAMalformedIssueDate: issue_date is read only for
// --latest's note, so one that isn't a date there prints a note, the value
// quoted as listed (worklist round 2, §3), and doesn't fail (worklist round
// 1, §6).
func TestLatestNotesAMalformedIssueDate(t *testing.T) {
	for _, issued := range []string{`"2025-8-1"`, `"2025-02-30"`, `20250801`} {
		f, c := newFake(t)
		root := storedAll(t)
		f.set(versionsURI("1024.41"), reply{body: strings.Replace(versions41, `],"meta"`,
			`,{"date":"2025-07-15","issue_date":`+issued+`,"identifier":"1024.41","removed":false}],"meta"`, 1)})
		code, stdout, stderr := latestAt(t, root, c, "1024.41")
		note := "1024.41: information: eCFR lists 2025-07-15 with issue_date \"" + strings.Trim(issued, `"`) + "\", not a YYYY-MM-DD date, so whether it was reissued since it was fetched is unknown; run regcorpus fetch --verify 1024.41\n"
		if code != 0 || stderr != "" || !strings.Contains(stdout, note) {
			t.Errorf("issue_date %s: exit %d, stdout %q, stderr %q; want 0 and the note %q", issued, code, stdout, stderr, note)
		}
	}
}

// TestFetchAndLatestReadEcfrsLatestAlike: plain fetch and --latest read
// eCFR's latest from the version list by one rule: its newest date, and
// whether an entry at that date removed the section, in whatever order the
// list gives them. Plain fetch keeps its behaviour, a removal there exit 2
// with nothing stored; --latest reports it, exit 1 (worklist round 2, §5:
// C-N8).
func TestFetchAndLatestReadEcfrsLatestAlike(t *testing.T) {
	const removed = `{"date":"2025-07-15","issue_date":"2025-08-01","identifier":"1024.41","removed":true}`
	for _, tc := range []struct {
		name, list      string
		fetch, latest   int
		fetchOut, notes string
	}{
		{"the newest date's second entry removed it", strings.Replace(versions41, `],"meta"`, ","+removed+`],"meta"`, 1),
			2, 1, "eCFR's latest version of 1024.41, 2025-07-15, removed it", "eCFR's latest 2025-07-15 (removed)"},
		{"the newest date's first entry removed it", strings.Replace(versions41, `{"content_versions":[`, `{"content_versions":[`+removed+",", 1),
			2, 1, "eCFR's latest version of 1024.41, 2025-07-15, removed it", "eCFR's latest 2025-07-15 (removed)"},
		{"an older entry removed it", strings.Replace(versions41, `"date":"2017-01-01","amendment_date":"2017-01-01","issue_date":"2017-01-01","identifier":"1024.41","substantive":true,"removed":false`,
			`"date":"2017-01-01","amendment_date":"2017-01-01","issue_date":"2017-01-01","identifier":"1024.41","substantive":true,"removed":true`, 1),
			0, 0, "1024.41 2025-07-15: stored " + rawPath("1024.41"), "eCFR's latest 2025-07-15: no newer version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.list == versions41 {
				t.Fatal("the case doesn't change the list")
			}
			f, c := newFake(t)
			f.set(versionsURI("1024.41"), reply{body: tc.list})
			root := corpusRoot(t)
			code, stdout, stderr := fetchAt(t, root, c, "2026-11-25", "1024.41")
			if code != tc.fetch || !strings.Contains(stdout+stderr, tc.fetchOut) {
				t.Errorf("fetch: exit %d, stdout %q, stderr %q; want %d and %q", code, stdout, stderr, tc.fetch, tc.fetchOut)
			}
			if tc.fetch != 0 && len(tree(t, root)) != len(tree(t, corpusRoot(t))) {
				t.Errorf("fetch stored something")
			}
			code, stdout, stderr = latestAt(t, storedAll(t), c, "1024.41")
			if code != tc.latest || !strings.Contains(stdout, tc.notes) {
				t.Errorf("--latest: exit %d, stdout %q, stderr %q; want %d and %q", code, stdout, stderr, tc.latest, tc.notes)
			}
		})
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
