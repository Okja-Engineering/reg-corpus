// Tests for the eCFR versioner client (R1, plan step 2), offline: an httptest
// server stands in for eCFR, so `make test` never touches the network.
package ecfr

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// apiPath is the path prefix of the versioner API, so the fake serves the
// same paths as eCFR under DefaultBaseURL.
const apiPath = "/api/versioner/v1"

// reply is one canned response. The body is gzipped unless encoding says
// otherwise: "identity" sends it as is with no Content-Encoding, any other
// non-empty value is sent as the Content-Encoding header over the plain body.
type reply struct {
	status   int // 0 means 200
	body     string
	encoding string
}

// fakeECFR serves replies by request URI (path and query) under apiPath, and
// eCFR's 404 for anything else. It records every request.
type fakeECFR struct {
	t       *testing.T
	mu      sync.Mutex
	replies map[string]reply
	got     []*http.Request
}

const (
	versions41 = `{"content_versions":[` +
		`{"date":"2017-01-01","amendment_date":"2017-01-01","issue_date":"2017-01-01","identifier":"1024.41","substantive":true,"removed":false,"type":"section"},` +
		`{"date":"2025-07-15","amendment_date":"2025-07-15","issue_date":"2025-07-15","identifier":"1024.41","substantive":true,"removed":false,"type":"section"},` +
		`{"date":"2021-08-31","amendment_date":"2021-08-31","issue_date":"2023-03-29","identifier":"1024.41","substantive":false,"removed":false,"type":"section"},` +
		`{"date":"2025-05-16","amendment_date":"2025-05-16","issue_date":"2025-05-16","identifier":"1024.41","substantive":true,"removed":false,"type":"section"}` +
		`],"meta":{"title":"12","part":"1024","section":"1024.41","result_count":"4"}}`
	versions30 = `{"content_versions":[` +
		`{"date":"2017-01-01","amendment_date":"2017-01-01","issue_date":"2017-01-01","identifier":"1024.30","substantive":true,"removed":false,"type":"section"},` +
		`{"date":"2018-04-19","amendment_date":"2018-04-19","issue_date":"2018-04-19","identifier":"1024.30","substantive":true,"removed":false,"type":"section"}` +
		`],"meta":{"title":"12","part":"1024","section":"1024.30","result_count":"2"}}`
	versions31 = `{"content_versions":[` +
		`{"date":"2025-07-15","amendment_date":"2025-07-15","issue_date":"2025-07-15","identifier":"1024.31","substantive":true,"removed":false,"type":"section"}` +
		`],"meta":{"title":"12","part":"1024","section":"1024.31","result_count":"1"}}`
	titlesJSON = `{"titles":[` +
		`{"number":11,"name":"Federal Elections","up_to_date_as_of":"2026-11-02","reserved":false},` +
		`{"number":12,"name":"Banks and Banking","latest_amended_on":"2026-11-18","up_to_date_as_of":"2026-11-19","reserved":false}` +
		`],"meta":{"date":"2026-11-19","import_in_progress":false}}`
)

// xmlBody is a stand-in section body: not eCFR's text, but shaped like it,
// with the trailing space eCFR leaves after the declaration and non-ASCII
// characters, so a byte-exact store is visible.
func xmlBody(id, date string) string {
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?> \n<DIV8 N=\"" + id + "\" TYPE=\"SECTION\">\n" +
		"<HEAD>§ " + id + " Stand-in.</HEAD>\n<P>(a) Stand-in text as of " + date + " — “quoted” text.</P>\n</DIV8>\n"
}

func versionsURI(id string) string {
	return apiPath + "/versions/title-12.json?part=1024&section=" + id
}

func xmlURI(date, id string) string {
	return apiPath + "/full/" + date + "/title-12.xml?part=1024&section=" + id
}

// newFake returns a fake serving the three sections' version lists, their
// latest XML (1024.41 and 1024.31 at 2025-07-15, 1024.30 at 2018-04-19) and
// titles.json, and a client pointed at it.
func newFake(t *testing.T) (*fakeECFR, *Client) {
	t.Helper()
	f := &fakeECFR{t: t, replies: map[string]reply{
		versionsURI("1024.41"):          {body: versions41},
		versionsURI("1024.30"):          {body: versions30},
		versionsURI("1024.31"):          {body: versions31},
		xmlURI("2025-07-15", "1024.41"): {body: xmlBody("1024.41", "2025-07-15")},
		xmlURI("2018-04-19", "1024.30"): {body: xmlBody("1024.30", "2018-04-19")},
		xmlURI("2025-07-15", "1024.31"): {body: xmlBody("1024.31", "2025-07-15")},
		apiPath + "/titles.json":        {body: titlesJSON},
		xmlURI("2025-05-15", "1024.41"): {body: xmlBody("1024.41", "2025-05-15")},
		xmlURI("2025-05-15", "1024.30"): {body: xmlBody("1024.30", "2025-05-15")},
		xmlURI("2025-05-15", "1024.31"): {body: xmlBody("1024.31", "2025-05-15")},
	}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, &Client{BaseURL: srv.URL + apiPath, HTTP: srv.Client()}
}

func (f *fakeECFR) set(uri string, r reply) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies[uri] = r
}

func (f *fakeECFR) requests() []*http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*http.Request(nil), f.got...)
}

func (f *fakeECFR) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.got = append(f.got, r)
	rep, ok := f.replies[r.URL.RequestURI()]
	f.mu.Unlock()
	if !ok {
		rep = reply{status: http.StatusNotFound, body: `{"error":"No matching content found."}`}
	}
	body := []byte(rep.body)
	switch rep.encoding {
	case "":
		var b bytes.Buffer
		zw := gzip.NewWriter(&b)
		if _, err := zw.Write(body); err != nil {
			f.t.Error(err)
		}
		if err := zw.Close(); err != nil {
			f.t.Error(err)
		}
		body = b.Bytes()
		w.Header().Set("Content-Encoding", "gzip")
	case "identity":
	default:
		w.Header().Set("Content-Encoding", rep.encoding)
	}
	if rep.status != 0 {
		w.WriteHeader(rep.status)
	}
	w.Write(body)
}

func TestDefaultBaseURLIsTheVersionerAPI(t *testing.T) {
	if got, want := NewClient().BaseURL, "https://www.ecfr.gov/api/versioner/v1"; got != want {
		t.Fatalf("BaseURL %q, want %q", got, want)
	}
}

func TestVersionsAndLatestPickTheLatestDate(t *testing.T) {
	_, c := newFake(t)
	for _, tc := range []struct{ id, want string }{
		{"1024.41", "2025-07-15"}, // listed out of order, with a later issue date on an earlier version
		{"1024.30", "2018-04-19"},
		{"1024.31", "2025-07-15"}, // a single version
	} {
		s, _ := lookup(tc.id)
		vs, err := c.Versions(context.Background(), s)
		if err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		v, err := Latest(vs)
		if err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		if string(v.Date) != tc.want {
			t.Errorf("%s: latest %s, want %s", tc.id, v.Date, tc.want)
		}
	}
}

func TestLatestRejectsAnEmptyListAndARemovedLatest(t *testing.T) {
	if _, err := Latest(nil); err == nil || !strings.Contains(err.Error(), "no versions") {
		t.Errorf("empty list: err %v, want one naming no versions", err)
	}
	vs := []Version{
		{Date: "2017-01-01", Identifier: "1024.41"},
		{Date: "2025-07-15", Identifier: "1024.41", Removed: true},
	}
	if _, err := Latest(vs); err == nil || !strings.Contains(err.Error(), "removed") {
		t.Errorf("removed latest: err %v, want one naming removed", err)
	}
}

func TestRequestsSendGzipAndAGenericUserAgent(t *testing.T) {
	f, c := newFake(t)
	s, _ := lookup("1024.41")
	if _, err := c.Versions(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if _, err := c.XML(context.Background(), s, "2025-07-15"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpToDateAsOf(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := f.requests()
	if len(got) != 3 {
		t.Fatalf("%d requests, want 3", len(got))
	}
	for _, r := range got {
		if r.Method != http.MethodGet || r.Header.Get("Accept-Encoding") != "gzip" || r.Header.Get("User-Agent") != "regcorpus" {
			t.Errorf("%s %s: Accept-Encoding %q, User-Agent %q; want GET, gzip, regcorpus",
				r.Method, r.URL, r.Header.Get("Accept-Encoding"), r.Header.Get("User-Agent"))
		}
	}
}

func TestXMLReturnsTheDecodedBodyByteForByte(t *testing.T) {
	f, c := newFake(t)
	s, _ := lookup("1024.41")
	want := xmlBody("1024.41", "2025-07-15")
	for _, enc := range []string{"", "identity"} { // gzipped, and served plain
		f.set(xmlURI("2025-07-15", "1024.41"), reply{body: want, encoding: enc})
		got, err := c.XML(context.Background(), s, "2025-07-15")
		if err != nil {
			t.Fatalf("encoding %q: %v", enc, err)
		}
		if string(got) != want {
			t.Errorf("encoding %q: body %q, want %q", enc, got, want)
		}
	}
}

func TestUpToDateAsOfReadsTitle12(t *testing.T) {
	_, c := newFake(t)
	got, err := c.UpToDateAsOf(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026-11-19" {
		t.Errorf("up to date as of %s, want 2026-11-19 (title 12's, not title 11's)", got)
	}
}

// TestFailuresNameTheURLAndTheCause: every failure is an error that names
// the request and why, never a partial result.
func TestFailuresNameTheURLAndTheCause(t *testing.T) {
	s, _ := lookup("1024.41")
	ctx := context.Background()
	xml := func(c *Client) error { _, err := c.XML(ctx, s, "2025-07-15"); return err }
	versions := func(c *Client) error { _, err := c.Versions(ctx, s); return err }
	asOf := func(c *Client) error { _, err := c.UpToDateAsOf(ctx); return err }
	for _, tc := range []struct {
		name  string
		uri   string
		reply reply
		call  func(*Client) error
		want  []string
	}{
		{"http error", xmlURI("2025-07-15", "1024.41"),
			reply{status: 404, body: `{"error":"No matching content found."}`}, xml,
			[]string{xmlURI("2025-07-15", "1024.41"), "404", "No matching content found."}},
		{"server error on versions", versionsURI("1024.41"),
			reply{status: 503, body: "<html>down</html>"}, versions,
			[]string{versionsURI("1024.41"), "503", "<html>down</html>"}},
		{"not UTF-8", xmlURI("2025-07-15", "1024.41"),
			reply{body: "<?xml version=\"1.0\"?>\n<P>\xa7 1024.41</P>\n"}, xml,
			[]string{xmlURI("2025-07-15", "1024.41"), "not valid UTF-8", "byte offset 25"}},
		{"not well-formed XML", xmlURI("2025-07-15", "1024.41"),
			reply{body: "<html><body>maintenance"}, xml,
			[]string{xmlURI("2025-07-15", "1024.41"), "not well-formed XML"}},
		{"unknown content encoding", xmlURI("2025-07-15", "1024.41"),
			reply{body: xmlBody("1024.41", "2025-07-15"), encoding: "br"}, xml,
			[]string{xmlURI("2025-07-15", "1024.41"), `Content-Encoding "br"`}},
		{"http error whose body can't be decoded still names the status", xmlURI("2025-07-15", "1024.41"),
			reply{status: 502, body: "bad gateway", encoding: "br"}, xml,
			[]string{xmlURI("2025-07-15", "1024.41"), "502", "bad gateway"}},
		{"corrupt gzip", xmlURI("2025-07-15", "1024.41"),
			reply{body: "\x1f\x8b\x08\x00garbage", encoding: "gzip"}, xml,
			[]string{xmlURI("2025-07-15", "1024.41"), "gzip"}},
		{"malformed versions JSON", versionsURI("1024.41"),
			reply{body: `{"content_versions":[{"date":"2025-07-15"`}, versions,
			[]string{versionsURI("1024.41"), "JSON"}},
		{"versions JSON without content_versions", versionsURI("1024.41"),
			reply{body: `{"meta":{}}`}, versions,
			[]string{versionsURI("1024.41"), "content_versions"}},
		{"a version date not YYYY-MM-DD", versionsURI("1024.41"),
			reply{body: `{"content_versions":[{"date":"2025-7-15","identifier":"1024.41"}]}`}, versions,
			[]string{versionsURI("1024.41"), `"2025-7-15"`}},
		{"another section's version", versionsURI("1024.41"),
			reply{body: `{"content_versions":[{"date":"2025-07-15","issue_date":"2025-07-15","identifier":"1024.4"}]}`}, versions,
			[]string{versionsURI("1024.41"), `"1024.4"`, `"1024.41"`}},
		{"malformed titles JSON", apiPath + "/titles.json",
			reply{body: `[]`}, asOf,
			[]string{apiPath + "/titles.json", "JSON"}},
		{"no title 12", apiPath + "/titles.json",
			reply{body: `{"titles":[{"number":11,"up_to_date_as_of":"2026-11-02"}]}`}, asOf,
			[]string{apiPath + "/titles.json", "title 12"}},
		{"title 12 without a date", apiPath + "/titles.json",
			reply{body: `{"titles":[{"number":12}]}`}, asOf,
			[]string{apiPath + "/titles.json", "up_to_date_as_of"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := newFake(t)
			f.set(tc.uri, tc.reply)
			err := tc.call(c)
			if err == nil {
				t.Fatal("no error")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q lacks %q", err, w)
				}
			}
		})
	}
}
