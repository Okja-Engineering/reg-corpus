// Tests for the eCFR versioner client (R1, plan step 2), offline: an httptest
// server stands in for eCFR, so `make test` never touches the network.
package ecfr

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// apiPath is the path prefix of the versioner API, so the fake serves the
// same paths as eCFR under DefaultBaseURL.
const apiPath = "/api/versioner/v1"

// reply is one canned response.
type reply struct {
	status int // 0 means 200
	body   string
	// raw sends body as is; otherwise it is gzipped, as eCFR sends it.
	raw bool
	// encoding is the Content-Encoding header: "" means gzip for a gzipped
	// body and none for a raw one.
	encoding string
	header   map[string]string // more response headers, e.g. Location
	// cut promises more bytes than body, sends body, then drops the
	// connection, so reading the body fails.
	cut bool
}

// fakeECFR serves replies by request URI (path and query) under apiPath, and
// eCFR's 404 for anything else. Like eCFR it refuses (406) a full XML request
// that doesn't accept gzip. It records every request.
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
// with the root element eCFR serves for a section, the trailing space eCFR
// leaves after the declaration and non-ASCII characters, so a byte-exact
// store is visible.
func xmlBody(id, date string) string {
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?> \n<DIV8 N=\"" + id + "\" TYPE=\"SECTION\">\n" +
		"<HEAD>§ " + id + " Stand-in.</HEAD>\n<P>(a) Stand-in text as of " + date + " — “quoted” text.</P>\n</DIV8>\n"
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
	f := fake(t)
	return f, serve(t, httptest.NewServer(f))
}

func fake(t *testing.T) *fakeECFR {
	return &fakeECFR{t: t, replies: map[string]reply{
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
}

// serve returns a client for the started server srv, which stands in for
// both eCFR (under apiPath) and govinfo (at its root). Its transport adds no
// Accept-Encoding of its own, so the fake sees only what the client asks for.
func serve(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	t.Cleanup(srv.Close)
	hc := srv.Client()
	hc.Transport.(*http.Transport).DisableCompression = true
	return &Client{BaseURL: srv.URL + apiPath, HTTP: hc, Govinfo: srv.URL}
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
	if strings.Contains(r.URL.Path, "/full/") && !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		rep = reply{status: http.StatusNotAcceptable, body: "Not Acceptable", raw: true}
	}
	body := []byte(rep.body)
	enc := rep.encoding
	if !rep.raw {
		body = gzipped(f.t, body)
		if enc == "" {
			enc = "gzip"
		}
	}
	for k, v := range rep.header {
		w.Header().Set(k, v)
	}
	if enc != "" {
		w.Header().Set("Content-Encoding", enc)
	}
	if rep.cut {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)+100))
	}
	if rep.status != 0 {
		w.WriteHeader(rep.status)
	}
	w.Write(body)
	if rep.cut {
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler) // drops the connection mid-body
	}
}

func gzipped(t *testing.T, b []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		t.Error(err)
	}
	if err := zw.Close(); err != nil {
		t.Error(err)
	}
	return buf.Bytes()
}

func TestDefaultBaseURLIsTheVersionerAPI(t *testing.T) {
	if got, want := NewClient().BaseURL, "https://www.ecfr.gov/api/versioner/v1"; got != want {
		t.Fatalf("BaseURL %q, want %q", got, want)
	}
}

func TestParseDateTakesOnlyRealCalendarDates(t *testing.T) {
	for _, s := range []string{"2025-07-15", "2024-02-29", "2000-01-01"} {
		if d, err := ParseDate(s); err != nil || string(d) != s {
			t.Errorf("ParseDate(%q) = %q, %v; want it back, no error", s, d, err)
		}
	}
	for _, s := range []string{"2025-02-30", "2023-02-29", "2025-13-01", "2025-7-15", "2025-07-5",
		"20250715", "2025/07/15", "2025-07-15T00:00:00Z", " 2025-07-15", "2025-07-15 ", ""} {
		if d, err := ParseDate(s); err == nil || !strings.Contains(err.Error(), strconv.Quote(s)) {
			t.Errorf("ParseDate(%q) = %q, %v; want an error quoting it", s, d, err)
		}
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
		d, removed, err := Latest(vs)
		if err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		if string(d) != tc.want || removed {
			t.Errorf("%s: latest %s, removed %t; want %s, not removed", tc.id, d, removed, tc.want)
		}
	}
}

// TestLatestReadsRemovedThroughTheVersionList: a version that removed the
// section is read from eCFR's JSON; only a removal at the latest date is
// one (plain fetch stores nothing then; TestFetchAndLatestReadEcfrsLatestAlike).
func TestLatestReadsRemovedThroughTheVersionList(t *testing.T) {
	entry := func(date string, removed bool) string {
		return `{"date":"` + date + `","identifier":"1024.41","removed":` + strconv.FormatBool(removed) + `}`
	}
	list := func(es ...string) string { return `{"content_versions":[` + strings.Join(es, ",") + `]}` }
	for _, tc := range []struct {
		name, body string
		removed    bool // whether the latest date, 2025-07-15, removed it
	}{
		{"none removed", list(entry("2017-01-01", false), entry("2025-07-15", false)), false},
		{"an earlier version removed", list(entry("2017-01-01", true), entry("2025-07-15", false)), false},
		{"the latest removed", list(entry("2017-01-01", false), entry("2025-07-15", true)), true},
		{"the latest removed, listed first", list(entry("2025-07-15", true), entry("2017-01-01", false)), true},
		{"two at the latest date, the second removed", list(entry("2025-07-15", false), entry("2025-07-15", true)), true},
		{"two at the latest date, the first removed", list(entry("2025-07-15", true), entry("2025-07-15", false)), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := newFake(t)
			f.set(versionsURI("1024.41"), reply{body: tc.body})
			s, _ := lookup("1024.41")
			vs, err := c.Versions(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			d, removed, err := Latest(vs)
			if err != nil || d != "2025-07-15" || removed != tc.removed {
				t.Errorf("latest %s, removed %t, %v; want 2025-07-15, removed %t", d, removed, err, tc.removed)
			}
		})
	}
	if _, _, err := Latest(nil); err == nil || !strings.Contains(err.Error(), "no versions") {
		t.Errorf("empty list: err %v, want one naming no versions", err)
	}
}

// TestRequestsSendGzipAndAGenericUserAgent: the client asks for gzip itself
// (the fake, like eCFR, refuses full XML otherwise) and names no one.
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

// TestXMLReturnsTheDecodedBodyByteForByte: whatever the transfer encoding,
// the bytes returned are the document as served: CRLF, a BOM, comments and
// processing instructions included.
func TestXMLReturnsTheDecodedBodyByteForByte(t *testing.T) {
	plain := xmlBody("1024.41", "2025-07-15")
	crlfBOM := "\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"UTF-8\"?>\r\n<DIV8 N=\"1024.41\" TYPE=\"SECTION\">\r\n<P>§ (a) “x”</P>\r\n</DIV8>\r\n"
	around := "<?xml version=\"1.0\"?>\n<!-- before --><?pi x?>\n<DIV8 TYPE=\"SECTION\" hierarchy_metadata=\"{}\" N=\"1024.41\"><P/></DIV8>\n<!-- after -->\n"
	for _, tc := range []struct {
		name string
		r    reply
	}{
		{"gzip", reply{body: plain}},
		{"GZIP", reply{body: plain, encoding: "GZIP"}},
		{"x-gzip", reply{body: plain, encoding: "x-gzip"}},
		{"padded gzip", reply{body: plain, encoding: " gzip "}},
		{"no Content-Encoding", reply{body: plain, raw: true}},
		{"identity", reply{body: plain, raw: true, encoding: "identity"}},
		{"CRLF and a BOM", reply{body: crlfBOM}},
		{"comments and a PI around the root", reply{body: around}},
		{"no declaration", reply{body: `<DIV8 N="1024.41" TYPE="SECTION"/>`}},
		{"every XML whitespace outside the root", reply{body: "\t\r\n <DIV8 N=\"1024.41\" TYPE=\"SECTION\"/>\t\r\n "}},
		{"a PI whose target starts with xml", reply{body: `<DIV8 N="1024.41" TYPE="SECTION"/><?xml-stylesheet href="a"?>`}},
		{"CDATA and character references inside the root", reply{body: `<DIV8 N="1024.41" TYPE="SECTION"><![CDATA[<x>]]>&#167;</DIV8>`}},
		{"a declaration with spaces around = and single quotes", reply{body: "<?xml version = '1.0' ?>\n<DIV8 N=\"1024.41\" TYPE=\"SECTION\"/>"}},
		{"a declaration with every part", reply{body: "<?xml version=\"1.0\" encoding=\"utf-8\" standalone=\"yes\"?>\n<DIV8 N=\"1024.41\" TYPE=\"SECTION\"/>"}},
		{"a declaration with standalone only", reply{body: "<?xml version=\"1.0\"\tstandalone='no'\r\n?><DIV8 N=\"1024.41\" TYPE=\"SECTION\"/>"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := newFake(t)
			f.set(xmlURI("2025-07-15", "1024.41"), tc.r)
			s, _ := lookup("1024.41")
			got, err := c.XML(context.Background(), s, "2025-07-15")
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.r.body {
				t.Errorf("body %q, want %q", got, tc.r.body)
			}
		})
	}
}

// TestXMLIsOneDocumentWhoseRootIsTheSectionAsked: a body is stored only when
// it is exactly one well-formed XML document (strict: no unknown entities, no
// unquoted attributes, no duplicate attributes) whose root element is the one
// eCFR serves for the section asked for, <DIV8 TYPE="SECTION" N="<section>">.
// Outside the root only XML whitespace (space, tab, CR, LF) as written,
// comments and PIs may appear; an XML declaration, <?xml version=...?> in
// that case, only at the very start. Anything else, from an empty body to
// another section's XML, names the URL and what is wrong.
func TestXMLIsOneDocumentWhoseRootIsTheSectionAsked(t *testing.T) {
	const root = `<DIV8 N="1024.41" TYPE="SECTION">`
	const decl = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
	for _, tc := range []struct{ name, body, want string }{
		{"empty", "", "no root element"},
		{"whitespace only", " \n", "no root element"},
		{"declaration only", decl, "no root element"},
		{"comment only", decl + "<!-- nothing -->\n", "no root element"},
		{"two roots", decl + root + "</DIV8>" + root + "</DIV8>\n", "second root element"},
		{"text after the root", decl + root + "</DIV8>junk\n", "text outside the root element"},
		{"text before the root", "junk" + root + "</DIV8>", "text outside the root element"},
		{"a maintenance page", "Service Unavailable<br/>", "text outside the root element"},
		{"a no-break space outside the root", decl + root + "</DIV8> ", "text outside the root element"},
		{"a vertical tab outside the root", decl + root + "</DIV8>\v", "not well-formed XML"},
		{"a form feed outside the root", decl + root + "</DIV8>\f", "not well-formed XML"},
		{"a second BOM", "\xef\xbb\xbf\xef\xbb\xbf" + root + "</DIV8>", "text outside the root element"},
		{"CDATA after the root", decl + root + "</DIV8><![CDATA[ ]]>", "text outside the root element"},
		{"a character reference after the root", decl + root + "</DIV8>&#32;", "text outside the root element"},
		{"a character reference before the root", "&#10;" + root + "</DIV8>", "text outside the root element"},
		{"a declaration after the root", root + "</DIV8>" + decl, "XML declaration"},
		{"a second declaration", decl + decl + root + "</DIV8>", "XML declaration"},
		{"whitespace before the declaration", " " + decl + root + "</DIV8>", "XML declaration"},
		{"an upper-case declaration", `<?XML version="1.0"?>` + root + "</DIV8>", "XML declaration"},
		{"a mixed-case declaration", `<?Xml version="1.0"?>` + root + "</DIV8>", "XML declaration"},
		{"a declaration without a version", `<?xml encoding="UTF-8"?>` + root + "</DIV8>", "XML declaration"},
		// The declaration is parsed by its grammar (XML 1.0, 2.8): version 1.0,
		// then optionally encoding UTF-8, then optionally standalone, in that
		// order, each with Eq (= with optional spaces) and a quoted value.
		{"an empty declaration", `<?xml?>` + root + "</DIV8>", "XML declaration"},
		{"a version without a value", `<?xml version?>` + root + "</DIV8>", "XML declaration"},
		{"a misspelt version", `<?xml versionx="2.0"?>` + root + "</DIV8>", "XML declaration"},
		{"version 2.0 with spaces around =", `<?xml version = "2.0"?>` + root + "</DIV8>", "XML declaration"},
		{"version 1.1", `<?xml version="1.1"?>` + root + "</DIV8>", "not well-formed XML"},
		{"mismatched quotes", `<?xml version="1.0'?>` + root + "</DIV8>", "XML declaration"},
		{"a value not quoted", "<?xml version=`1.0`?>" + root + "</DIV8>", "XML declaration"},
		{"no space before encoding", `<?xml version="1.0"encoding="UTF-8"?>` + root + "</DIV8>", "XML declaration"},
		{"Latin-1 with spaces around =", `<?xml version="1.0" encoding = "ISO-8859-1"?>` + root + "</DIV8>", "XML declaration"},
		{"encoding before version", `<?xml encoding="UTF-8" version="1.0"?>` + root + "</DIV8>", "XML declaration"},
		{"standalone maybe", `<?xml version="1.0" standalone="maybe"?>` + root + "</DIV8>", "XML declaration"},
		{"standalone before encoding", `<?xml version="1.0" standalone="yes" encoding="UTF-8"?>` + root + "</DIV8>", "XML declaration"},
		{"junk after the declaration's attributes", `<?xml version="1.0" junk?>` + root + "</DIV8>", "XML declaration"},
		{"a namespaced N on the root", decl + `<DIV8 xmlns:x="urn:x" x:N="1024.41" TYPE="SECTION"></DIV8>`, `root element <DIV8 TYPE="SECTION">`},
		{"a namespaced TYPE on the root", decl + `<DIV8 xmlns:x="urn:x" N="1024.41" x:TYPE="SECTION"></DIV8>`, `root element <DIV8 N="1024.41">`},
		{"a default namespace on the root", decl + `<DIV8 N="1024.41" TYPE="SECTION" xmlns="urn:x"></DIV8>`, "root element <urn:x:DIV8"},
		{"a duplicate attribute through two prefixes", decl + root + `<P xmlns:a="u" xmlns:b="u" a:x="1" b:x="2"/></DIV8>`, "duplicate attribute u:x"},
		{"a doctype", decl + "<!DOCTYPE DIV8>" + root + "</DIV8>", "<!DOCTYPE DIV8>"},
		{"a duplicate N on the root", decl + `<DIV8 N="1024.40" N="1024.41" TYPE="SECTION"></DIV8>`, `duplicate attribute N`},
		{"a duplicate TYPE on the root", decl + `<DIV8 TYPE="APPENDIX" N="1024.41" TYPE="SECTION"></DIV8>`, `duplicate attribute TYPE`},
		{"a duplicate attribute inside", decl + root + `<P a="1" a="2"/></DIV8>`, `duplicate attribute a`},
		{"an unknown entity", decl + root + "&nbsp;</DIV8>", "not well-formed XML"},
		{"an unquoted attribute", decl + `<DIV8 N=1024.41 TYPE="SECTION"></DIV8>`, "not well-formed XML"},
		{"an unclosed element", decl + root + "<P></DIV8>", "not well-formed XML"},
		{"unterminated", decl + root + "<P>text</P>", "not well-formed XML"},
		{"html", "<html><body>maintenance</body></html>", "root element <html>"},
		{"another section", decl + `<DIV8 N="1024.40" TYPE="SECTION"></DIV8>`, `root element <DIV8 TYPE="SECTION" N="1024.40">`},
		{"a section whose number starts the same", decl + `<DIV8 N="1024.4" TYPE="SECTION"></DIV8>`, `N="1024.4">`},
		{"another type", decl + `<DIV8 N="1024.41" TYPE="APPENDIX"></DIV8>`, `TYPE="APPENDIX"`},
		{"another element", decl + `<DIV9 N="1024.41" TYPE="SECTION"></DIV9>`, "root element <DIV9"},
		{"no N", decl + `<DIV8 TYPE="SECTION"></DIV8>`, `root element <DIV8 TYPE="SECTION">`},
		{"a namespaced root", decl + `<x:DIV8 xmlns:x="urn:x" N="1024.41" TYPE="SECTION"></x:DIV8>`, "root element"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := newFake(t)
			f.set(xmlURI("2025-07-15", "1024.41"), reply{body: tc.body})
			s, _ := lookup("1024.41")
			_, err := c.XML(context.Background(), s, "2025-07-15")
			if err == nil {
				t.Fatal("no error")
			}
			for _, w := range []string{xmlURI("2025-07-15", "1024.41"), tc.want} {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q lacks %q", err, w)
				}
			}
			if strings.Contains(tc.want, "root element <") && !strings.Contains(err.Error(), `want <DIV8 TYPE="SECTION" N="1024.41">`) {
				t.Errorf("error %q doesn't name the root wanted", err)
			}
		})
	}
}

// TestRedirectsStayOnTheOriginAsked: a redirect is followed only to the same
// scheme and host; one to another host, or from https to http, is refused
// before it is followed.
func TestRedirectsStayOnTheOriginAsked(t *testing.T) {
	s, _ := lookup("1024.41")
	ctx := context.Background()

	t.Run("a same-origin loop stops after 10", func(t *testing.T) {
		f, c := newFake(t)
		f.set(xmlURI("2025-07-15", "1024.41"), reply{status: http.StatusFound, raw: true,
			header: map[string]string{"Location": xmlURI("2025-07-15", "1024.41")}})
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, err := c.XML(ctx, s, "2025-07-15")
		if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
			t.Errorf("err %v; want one naming the redirect limit", err)
		}
		if n := len(f.requests()); n != 10 {
			t.Errorf("%d requests, want 10", n)
		}
	})

	t.Run("same origin", func(t *testing.T) {
		f, c := newFake(t)
		f.set(xmlURI("2025-07-15", "1024.41"), reply{status: http.StatusFound, raw: true,
			header: map[string]string{"Location": xmlURI("2025-05-15", "1024.41")}})
		got, err := c.XML(ctx, s, "2025-07-15")
		if err != nil || string(got) != xmlBody("1024.41", "2025-05-15") {
			t.Errorf("body %q, err %v; want the redirect followed", got, err)
		}
	})

	t.Run("another host", func(t *testing.T) {
		other := fake(t)
		otherSrv := httptest.NewServer(other)
		defer otherSrv.Close()
		f, c := newFake(t)
		to := otherSrv.URL + xmlURI("2025-07-15", "1024.41")
		f.set(xmlURI("2025-07-15", "1024.41"), reply{status: http.StatusFound, raw: true,
			header: map[string]string{"Location": to}})
		_, err := c.XML(ctx, s, "2025-07-15")
		if err == nil || !strings.Contains(err.Error(), "redirect") || !strings.Contains(err.Error(), to) {
			t.Errorf("err %v; want a refused redirect naming %s", err, to)
		}
		if n := len(other.requests()); n != 0 {
			t.Errorf("the other host got %d requests, want 0", n)
		}
	})

	t.Run("https to http", func(t *testing.T) {
		f := fake(t)
		srv := httptest.NewTLSServer(f)
		c := serve(t, srv)
		to := "http://" + srv.Listener.Addr().String() + xmlURI("2025-05-15", "1024.41")
		f.set(xmlURI("2025-07-15", "1024.41"), reply{status: http.StatusMovedPermanently, raw: true,
			header: map[string]string{"Location": to}})
		_, err := c.XML(ctx, s, "2025-07-15")
		if err == nil || !strings.Contains(err.Error(), "redirect") || !strings.Contains(err.Error(), to) {
			t.Errorf("err %v; want a refused redirect naming %s", err, to)
		}
	})
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

// TestOnlyA200IsABody: every other status is an error naming it, decided
// before the body is read, so a body that can't be read or decoded never
// hides the status.
func TestOnlyA200IsABody(t *testing.T) {
	s, _ := lookup("1024.41")
	for _, st := range []int{201, 203, 204, 206, 304, 404, 406, 500, 503} {
		t.Run(strconv.Itoa(st), func(t *testing.T) {
			f, c := newFake(t)
			f.set(xmlURI("2025-07-15", "1024.41"), reply{status: st, body: xmlBody("1024.41", "2025-07-15")})
			_, err := c.XML(context.Background(), s, "2025-07-15")
			if err == nil || !strings.Contains(err.Error(), strconv.Itoa(st)) {
				t.Errorf("err %v; want one naming %d", err, st)
			}
		})
	}
	t.Run("an error whose body is cut short", func(t *testing.T) {
		f, c := newFake(t)
		f.set(xmlURI("2025-07-15", "1024.41"), reply{status: 503, body: "down for maintenance", raw: true, cut: true})
		_, err := c.XML(context.Background(), s, "2025-07-15")
		if err == nil || !strings.Contains(err.Error(), "503") {
			t.Errorf("err %v; want one naming 503", err)
		}
	})
	t.Run("a 200 whose body is cut short", func(t *testing.T) {
		f, c := newFake(t)
		f.set(xmlURI("2025-07-15", "1024.41"), reply{body: xmlBody("1024.41", "2025-07-15"), cut: true})
		_, err := c.XML(context.Background(), s, "2025-07-15")
		if err == nil || !strings.Contains(err.Error(), "reading body") {
			t.Errorf("err %v; want a failed read", err)
		}
	})
}

// TestBodiesAreCapped: neither the bytes received nor the bytes they decode
// to may exceed a generous cap (maxBody), so a runaway, endless or gzip-bomb
// response fails at the cap instead of exhausting memory.
func TestBodiesAreCapped(t *testing.T) {
	s, _ := lookup("1024.41")
	big := xmlBody("1024.41", "2025-07-15") + "<!--" + strings.Repeat(" ", maxBody) + "-->"
	for _, tc := range []struct {
		name string
		r    reply
	}{
		{"received", reply{body: big, raw: true}},
		{"decoded", reply{body: big}}, // about 64 KB gzipped
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := newFake(t)
			f.set(xmlURI("2025-07-15", "1024.41"), tc.r)
			_, err := c.XML(context.Background(), s, "2025-07-15")
			if err == nil || !strings.Contains(err.Error(), "exceeds") {
				t.Errorf("err %v; want one naming the cap", err)
			}
		})
	}
	t.Run("endless", func(t *testing.T) {
		chunk := []byte(strings.Repeat(" ", 1<<16))
		c := serve(t, httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for r.Context().Err() == nil {
				if _, err := w.Write(chunk); err != nil {
					return
				}
			}
		})))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := c.Versions(ctx, s)
		if err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Errorf("err %v; want one naming the cap", err)
		}
	})
}

// TestErrorBodyIsQuotedUpTo300Bytes: the start of an error body is where eCFR
// puts its reason; more than 300 bytes is cut.
func TestErrorBodyIsQuotedUpTo300Bytes(t *testing.T) {
	f, c := newFake(t)
	head := strings.Repeat("x", 300)
	f.set(xmlURI("2025-07-15", "1024.41"), reply{status: 500, body: head + "TAIL"})
	s, _ := lookup("1024.41")
	_, err := c.XML(context.Background(), s, "2025-07-15")
	if err == nil || !strings.Contains(err.Error(), strconv.Quote(head)+"...") || strings.Contains(err.Error(), "TAIL") {
		t.Errorf("err %v; want the first 300 bytes quoted, then ...", err)
	}
}

// countingTransport counts the response body bytes its caller reads.
type countingTransport struct {
	base http.RoundTripper
	n    *int64
}

func (t countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(r)
	if err == nil {
		resp.Body = countingBody{resp.Body, t.n}
	}
	return resp, err
}

type countingBody struct {
	io.ReadCloser
	n *int64
}

func (b countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	*b.n += int64(n)
	return n, err
}

// TestAnErrorBodyIsReadOnlyForItsSnippet: an error response's body is read
// only a few KB, enough to quote its start, however much is sent.
func TestAnErrorBodyIsReadOnlyForItsSnippet(t *testing.T) {
	f, c := newFake(t)
	var n int64
	c.HTTP.Transport = countingTransport{c.HTTP.Transport, &n}
	f.set(xmlURI("2025-07-15", "1024.41"), reply{status: 500, body: "boom" + strings.Repeat(" ", 4<<20), raw: true})
	s, _ := lookup("1024.41")
	_, err := c.XML(context.Background(), s, "2025-07-15")
	if err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), `"boom`) {
		t.Errorf("err %v; want the status and the body's start", err)
	}
	if n > 16<<10 {
		t.Errorf("read %d bytes of the error body, want at most 16 KiB", n)
	}
}

// deadlines is a RoundTripper that records each request's deadline and
// passes it on.
type deadlines struct {
	next http.RoundTripper
	mu   sync.Mutex
	seen []time.Time // the zero Time for a request without one
}

func (d *deadlines) RoundTrip(req *http.Request) (*http.Response, error) {
	dl, _ := req.Context().Deadline()
	d.mu.Lock()
	d.seen = append(d.seen, dl)
	d.mu.Unlock()
	return d.next.RoundTrip(req)
}

// TestAZeroClientHasADefaultHTTPClient: a Client with no HTTP client, the
// zero one or NewClient's, works, and each request it sends has
// defaultTimeout as its deadline, so it never waits forever. The deadline is
// read where the request leaves, in http.DefaultTransport, which such a
// Client sends through.
func TestAZeroClientHasADefaultHTTPClient(t *testing.T) {
	_, c := newFake(t)
	rec := &deadlines{next: http.DefaultTransport}
	http.DefaultTransport = rec
	t.Cleanup(func() { http.DefaultTransport = rec.next })
	s, _ := lookup("1024.41")
	nc := NewClient()
	nc.BaseURL = c.BaseURL
	for name, c := range map[string]*Client{"zero": {BaseURL: c.BaseURL}, "NewClient": nc} {
		rec.seen = nil
		start := time.Now()
		if _, err := c.XML(context.Background(), s, "2025-07-15"); err != nil {
			t.Fatalf("%s Client: %v", name, err)
		}
		end := time.Now()
		if defaultTimeout <= 0 || len(rec.seen) == 0 {
			t.Fatalf("%s Client: defaultTimeout %v, %d requests seen", name, defaultTimeout, len(rec.seen))
		}
		for _, dl := range rec.seen {
			if dl.Before(start.Add(defaultTimeout)) || dl.After(end.Add(defaultTimeout)) {
				t.Errorf("%s Client sent a request with deadline %v; want %v after it was sent", name, dl, defaultTimeout)
			}
		}
	}
}

// TestTheCommittedRawXMLPasses: each source's committed raw XML, served as
// eCFR served it, passes XML() for its section byte for byte, well within
// maxBody. So the checks fit real eCFR documents, not only stand-ins.
func TestTheCommittedRawXMLPasses(t *testing.T) {
	for _, src := range sources {
		t.Run(src.Name, func(t *testing.T) {
			files, err := filepath.Glob(filepath.Join("..", "..", filepath.FromSlash(src.dir()), "raw", "*.xml"))
			if err != nil || len(files) != 1 {
				t.Fatalf("raw XML for %s: %v, %v; want one file", src.Name, files, err)
			}
			b, err := os.ReadFile(files[0])
			if err != nil {
				t.Fatal(err)
			}
			if len(b) > maxBody/16 {
				t.Errorf("%s is %d bytes; maxBody %d leaves too little room", files[0], len(b), maxBody)
			}
			date := strings.TrimSuffix(filepath.Base(files[0]), ".xml")
			f, c := newFake(t)
			f.set(xmlURI(date, src.Identifier), reply{body: string(b)})
			got, err := c.XML(context.Background(), src, Date(date))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, b) {
				t.Errorf("XML() changed the bytes of %s", files[0])
			}
		})
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
			reply{body: "<DIV8 N=\"1024.41\" TYPE=\"SECTION\"><P>maintenance"}, xml,
			[]string{xmlURI("2025-07-15", "1024.41"), "not well-formed XML"}},
		{"unknown content encoding", xmlURI("2025-07-15", "1024.41"),
			reply{body: xmlBody("1024.41", "2025-07-15"), raw: true, encoding: "br"}, xml,
			[]string{xmlURI("2025-07-15", "1024.41"), `Content-Encoding "br"`}},
		{"http error whose body can't be decoded still names the status", xmlURI("2025-07-15", "1024.41"),
			reply{status: 502, body: "bad gateway", raw: true, encoding: "br"}, xml,
			[]string{xmlURI("2025-07-15", "1024.41"), "502", "bad gateway"}},
		{"corrupt gzip", xmlURI("2025-07-15", "1024.41"),
			reply{body: "\x1f\x8b\x08\x00garbage", raw: true, encoding: "gzip"}, xml,
			[]string{xmlURI("2025-07-15", "1024.41"), "gzip"}},
		{"not gzip at all", xmlURI("2025-07-15", "1024.41"),
			reply{body: xmlBody("1024.41", "2025-07-15"), raw: true, encoding: "gzip"}, xml,
			[]string{xmlURI("2025-07-15", "1024.41"), "gzip", "invalid header"}},
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
			reply{body: `{"titles":[{"number":11,"up_to_date_as_of":"2026-11-02"}],"meta":{"import_in_progress":false}}`}, asOf,
			[]string{apiPath + "/titles.json", "title 12"}},
		{"title 12 without a date", apiPath + "/titles.json",
			reply{body: `{"titles":[{"number":12}],"meta":{"import_in_progress":false}}`}, asOf,
			[]string{apiPath + "/titles.json", "up_to_date_as_of"}},
		{"an import in progress", apiPath + "/titles.json",
			reply{body: strings.Replace(titlesJSON, `"import_in_progress":false`, `"import_in_progress":true`, 1)}, asOf,
			[]string{apiPath + "/titles.json", "import in progress", "retry later"}},
		// Whether eCFR is importing is read, never assumed: fail closed.
		{"titles JSON without meta", apiPath + "/titles.json",
			reply{body: `{"titles":[{"number":12,"up_to_date_as_of":"2026-11-19"}]}`}, asOf,
			[]string{apiPath + "/titles.json", "import_in_progress"}},
		{"meta without import_in_progress", apiPath + "/titles.json",
			reply{body: `{"titles":[{"number":12,"up_to_date_as_of":"2026-11-19"}],"meta":{"importInProgress":false}}`}, asOf,
			[]string{apiPath + "/titles.json", "import_in_progress"}},
		{"import_in_progress null", apiPath + "/titles.json",
			reply{body: `{"titles":[{"number":12,"up_to_date_as_of":"2026-11-19"}],"meta":{"import_in_progress":null}}`}, asOf,
			[]string{apiPath + "/titles.json", "import_in_progress"}},
		{"meta null", apiPath + "/titles.json",
			reply{body: `{"titles":[{"number":12,"up_to_date_as_of":"2026-11-19"}],"meta":null}`}, asOf,
			[]string{apiPath + "/titles.json", "import_in_progress"}},
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
