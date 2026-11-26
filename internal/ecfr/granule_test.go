// Tests for what plan step 6 adds to the client (R1, R1b; ADR 0014),
// offline against the fake in client_test.go: the version list's issue
// dates, a removal named by its own error, and govinfo's granule fetched
// through its link service on the origin asked.
package ecfr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The govinfo paths the fake serves for 1024.41: the link service, which
// redirects on the same host to the granule.
const (
	govLink41    = "/link/cfr/12/1024?link-type=xml&sectionnum=41&year=2026"
	govContent41 = "/content/pkg/CFR-2026-title12-vol8/xml/CFR-2026-title12-vol8-sec1024-41.xml"
	granule41    = "internal/govinfo/testdata/CFR-2026-title12-vol8-sec1024-41.xml"
)

// govRedirect is the link service's answer: a 302 to the granule.
func govRedirect(to string) reply {
	return reply{status: http.StatusFound, raw: true, header: map[string]string{"Location": to}}
}

func TestVersionsReadEachIssueDate(t *testing.T) {
	_, c := newFake(t)
	s, _ := lookup("1024.41")
	vs, err := c.Versions(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	got := map[Date]Date{}
	for _, v := range vs {
		got[v.Date] = v.IssueDate
	}
	if got["2021-08-31"] != "2023-03-29" || got["2025-07-15"] != "2025-07-15" {
		t.Errorf("issue dates %v; want 2021-08-31 issued 2023-03-29 (a reissue) and 2025-07-15 issued 2025-07-15", got)
	}
	// issue_date is read only by --latest's note, which checks it there: the
	// list keeps it as listed, and one that isn't a date fails nothing, as
	// on main (worklist round 1, §6).
	for _, tc := range []struct{ name, entry, want string }{
		{"absent", `{"date":"2025-07-15","identifier":"1024.41"}`, ""},
		{"null", `{"date":"2025-07-15","issue_date":null,"identifier":"1024.41"}`, ""},
		{"a date", `{"date":"2025-07-15","issue_date":"2025-08-01","identifier":"1024.41"}`, "2025-08-01"},
		{"not YYYY-MM-DD", `{"date":"2025-07-15","issue_date":"2025-8-1","identifier":"1024.41"}`, "2025-8-1"},
		{"empty", `{"date":"2025-07-15","issue_date":"","identifier":"1024.41"}`, ""},
		{"a number", `{"date":"2025-07-15","issue_date":20250801,"identifier":"1024.41"}`, "20250801"},
	} {
		f, c := newFake(t)
		f.set(versionsURI("1024.41"), reply{body: `{"content_versions":[` + tc.entry + `]}`})
		vs, err := c.Versions(context.Background(), s)
		if err != nil || len(vs) != 1 || string(vs[0].IssueDate) != tc.want {
			t.Errorf("%s: %v, %v; want issue date %q and no error", tc.name, vs, err, tc.want)
		}
	}
}

// TestGranuleErrorsNameTheURLThatFailed: after the link service's
// redirect, an error names the URL that failed, not only the link
// (worklist round 1, §9: IO-F3).
func TestGranuleErrorsNameTheURLThatFailed(t *testing.T) {
	f, c := newFake(t)
	f.set(govLink41, govRedirect(govContent41))
	f.set(govContent41, reply{status: http.StatusNotFound, body: "no such granule", raw: true})
	_, _, err := c.Granule(context.Background(), "1024.41")
	if err == nil || !strings.Contains(err.Error(), c.Govinfo+govContent41) || !strings.Contains(err.Error(), "404") {
		t.Errorf("err %v; want one naming 404 and %s", err, c.Govinfo+govContent41)
	}
}

func TestLatestNamesARemovalByItsError(t *testing.T) {
	_, err := Latest([]Version{{Date: "2017-01-01", Identifier: "1024.41"}, {Date: "2025-07-15", Identifier: "1024.41", Removed: true}})
	var r *RemovedError
	if !errors.As(err, &r) || r.Date != "2025-07-15" || r.Identifier != "1024.41" {
		t.Fatalf("Latest = %v; want a *RemovedError for 1024.41 at 2025-07-15", err)
	}
	if want := "eCFR's latest version of 1024.41, 2025-07-15, removed it"; err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
}

func TestGranuleFollowsTheLinkServiceOnItsOrigin(t *testing.T) {
	ctx := context.Background()
	t.Run("the granule", func(t *testing.T) {
		f, c := newFake(t)
		body := string(repoFile(t, granule41))
		f.set(govLink41, govRedirect(govContent41))
		f.set(govContent41, reply{body: body})
		from, got, err := c.Granule(ctx, "1024.41")
		if err != nil || string(got) != body || from != c.Govinfo+govContent41 {
			t.Fatalf("Granule = %q, %d bytes, %v; want %q and the granule's bytes", from, len(got), err, c.Govinfo+govContent41)
		}
		for _, r := range f.requests() {
			if r.Header.Get("User-Agent") != "regcorpus" || r.Header.Get("Accept-Encoding") != "gzip" {
				t.Errorf("%s: User-Agent %q, Accept-Encoding %q; want regcorpus and gzip", r.URL, r.Header.Get("User-Agent"), r.Header.Get("Accept-Encoding"))
			}
		}
	})
	t.Run("another host", func(t *testing.T) {
		other := fake(t)
		otherSrv := httptest.NewServer(other)
		defer otherSrv.Close()
		f, c := newFake(t)
		f.set(govLink41, govRedirect(otherSrv.URL+govContent41))
		if _, _, err := c.Granule(ctx, "1024.41"); err == nil || !strings.Contains(err.Error(), "redirect") {
			t.Errorf("err %v; want a refused redirect", err)
		}
		if n := len(other.requests()); n != 0 {
			t.Errorf("the other host got %d requests, want 0", n)
		}
	})
	t.Run("an HTTP error", func(t *testing.T) {
		f, c := newFake(t)
		f.set(govLink41, reply{status: http.StatusServiceUnavailable, body: "down", raw: true})
		if _, _, err := c.Granule(ctx, "1024.41"); err == nil || !strings.Contains(err.Error(), "503") || !strings.Contains(err.Error(), govLink41) {
			t.Errorf("err %v; want one naming 503 and the link", err)
		}
	})
	t.Run("no govinfo URL", func(t *testing.T) {
		f, c := newFake(t)
		c.Govinfo = ""
		if _, _, err := c.Granule(ctx, "1024.41"); err == nil {
			t.Errorf("no error; want one, and no request")
		}
		if n := len(f.requests()); n != 0 {
			t.Errorf("%d requests, want 0", n)
		}
	})
	if got := NewClient().Govinfo; got != "https://www.govinfo.gov" {
		t.Errorf("NewClient().Govinfo = %q, want https://www.govinfo.gov", got)
	}
}
