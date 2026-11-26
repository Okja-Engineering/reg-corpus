package ecfr

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/okja-engineering/reg-corpus/internal/govinfo"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// DefaultBaseURL is the eCFR versioner API (plan "Upstream calls"). It needs
// no API key.
const DefaultBaseURL = "https://www.ecfr.gov/api/versioner/v1"

// userAgent is generic on purpose: no request carries personal details.
const userAgent = "regcorpus"

// Date is an eCFR date, YYYY-MM-DD: the one date type meta.yaml records
// (internal/meta owns it). Values come from ParseDate.
type Date = meta.Date

// ParseDate returns s as a Date, or an error unless s is a real calendar date
// written YYYY-MM-DD.
func ParseDate(s string) (Date, error) { return meta.ParseDate(s) }

// Version is one entry of a source's eCFR version list: the date eCFR serves
// it at (full/{date}), whether that version removed the source, and the
// date eCFR issued that entry, later than Date when eCFR reissued the
// version ("" when the list doesn't say).
type Version struct {
	Date       Date
	Identifier string
	Removed    bool
	IssueDate  Date
}

// Client calls the eCFR versioner API at BaseURL, and govinfo, for the
// official edition fetch --verify compares with, at Govinfo. A nil HTTP is
// an http.Client with defaultTimeout. Whichever is used, the redirect
// policy is get's own (sameOrigin), for both hosts.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Govinfo string // govinfo's base URL; "" asks govinfo nothing
}

// defaultTimeout bounds each request of a Client with no HTTP client.
const defaultTimeout = 2 * time.Minute

// NewClient returns a client for DefaultBaseURL and govinfo.DefaultBaseURL.
func NewClient() *Client {
	return &Client{BaseURL: DefaultBaseURL, Govinfo: govinfo.DefaultBaseURL}
}

// RemovedError is Latest's error when the latest entry of a version list
// removed the source: there is no text at that date. fetch stops on it
// (exit 2); fetch --verify and --latest report it as a finding (exit 1).
type RemovedError struct {
	Identifier string
	Date       Date
}

func (e *RemovedError) Error() string {
	return fmt.Sprintf("eCFR's latest version of %s, %s, removed it", e.Identifier, e.Date)
}

// Granule returns section's granule XML in govinfo's annual edition of
// title 12 for govinfo.EditionYear, and the URL it came from: it asks
// govinfo's link service (govinfo.LinkPath), whose redirect get follows on
// govinfo's own origin, and returns the body as served, after get's checks
// (status, size cap, gzip). govinfo.Read checks what it is. A client with
// no Govinfo asks nothing and errs.
func (c *Client) Granule(ctx context.Context, section string) (from string, body []byte, err error) {
	if c.Govinfo == "" {
		return "", nil, errors.New("no govinfo base URL: the client asks govinfo nothing")
	}
	p, err := govinfo.LinkPath(title, section, govinfo.EditionYear)
	if err != nil {
		return "", nil, err
	}
	body, at, err := c.getFrom(ctx, c.Govinfo+p)
	if err != nil {
		return "", nil, err
	}
	return at, body, nil
}

// Versions returns s's version list, every entry checked: a YYYY-MM-DD date
// and s's own identifier.
func (c *Client) Versions(ctx context.Context, s Source) ([]Version, error) {
	u := c.url("/versions/title-"+title+".json", s.query())
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var doc struct {
		ContentVersions *[]struct {
			Date       string  `json:"date"`
			Identifier string  `json:"identifier"`
			Removed    bool    `json:"removed"`
			IssueDate  *string `json:"issue_date"`
		} `json:"content_versions"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("GET %s: version list JSON: %w", u, err)
	}
	if doc.ContentVersions == nil {
		return nil, fmt.Errorf("GET %s: version list JSON has no content_versions", u)
	}
	out := make([]Version, len(*doc.ContentVersions))
	for i, v := range *doc.ContentVersions {
		d, err := ParseDate(v.Date)
		if err != nil {
			return nil, fmt.Errorf("GET %s: content_versions[%d]: date %w", u, i, err)
		}
		if v.Identifier != s.Identifier {
			return nil, fmt.Errorf("GET %s: content_versions[%d]: identifier %q, want %q", u, i, v.Identifier, s.Identifier)
		}
		var issued Date
		if v.IssueDate != nil {
			if issued, err = ParseDate(*v.IssueDate); err != nil {
				return nil, fmt.Errorf("GET %s: content_versions[%d]: issue_date %w", u, i, err)
			}
		}
		out[i] = Version{Date: d, Identifier: v.Identifier, Removed: v.Removed, IssueDate: issued}
	}
	return out, nil
}

// Latest returns the version with the latest date in vs. It fails on an empty
// list, and when an entry at that date removed the source (a *RemovedError):
// there is no text to store.
func Latest(vs []Version) (Version, error) {
	if len(vs) == 0 {
		return Version{}, errors.New("eCFR lists no versions")
	}
	latest := vs[0]
	for _, v := range vs[1:] {
		if v.Date > latest.Date { // YYYY-MM-DD orders as text
			latest = v
		}
	}
	for _, v := range vs {
		if v.Date == latest.Date && v.Removed {
			return Version{}, &RemovedError{Identifier: v.Identifier, Date: v.Date}
		}
	}
	return latest, nil
}

// XML returns s's point-in-time XML at date: the decoded response body exactly
// as served, after checking it is valid UTF-8 and one well-formed XML document
// whose root element is s's (see document). Nothing in it is changed (R1).
func (c *Client) XML(ctx context.Context, s Source, date Date) ([]byte, error) {
	u := c.url("/full/"+string(date)+"/title-"+title+".xml", s.query())
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(body) {
		return nil, fmt.Errorf("GET %s: body is not valid UTF-8 (first bad byte offset %d)", u, firstInvalid(body))
	}
	if err := document(body, s.root()); err != nil {
		return nil, fmt.Errorf("GET %s: body is %w", u, err)
	}
	return body, nil
}

// UpToDateAsOf returns title 12's "up to date as of" date from titles.json. It
// fails while eCFR reports an import in progress, when what it serves may be
// mid-update, and when titles.json doesn't say (no meta.import_in_progress
// boolean): that is read, never assumed.
func (c *Client) UpToDateAsOf(ctx context.Context) (Date, error) {
	u := c.url("/titles.json", nil)
	body, err := c.get(ctx, u)
	if err != nil {
		return "", err
	}
	var doc struct {
		Titles []struct {
			Number       int    `json:"number"`
			UpToDateAsOf string `json:"up_to_date_as_of"`
		} `json:"titles"`
		Meta *struct {
			ImportInProgress *bool `json:"import_in_progress"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", fmt.Errorf("GET %s: titles JSON: %w", u, err)
	}
	switch {
	case doc.Meta == nil || doc.Meta.ImportInProgress == nil:
		return "", fmt.Errorf("GET %s: titles JSON has no meta.import_in_progress, so whether eCFR is mid-import is unknown", u)
	case *doc.Meta.ImportInProgress:
		return "", fmt.Errorf("GET %s: eCFR reports an import in progress (meta.import_in_progress); retry later", u)
	}
	want, _ := strconv.Atoi(title)
	for _, t := range doc.Titles {
		if t.Number != want {
			continue
		}
		d, err := ParseDate(t.UpToDateAsOf)
		if err != nil {
			return "", fmt.Errorf("GET %s: title %s up_to_date_as_of %w", u, title, err)
		}
		return d, nil
	}
	return "", fmt.Errorf("GET %s: no title %s listed", u, title)
}

// url joins BaseURL, path and the encoded query (keys sorted).
func (c *Client) url(path string, q url.Values) string {
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

// maxBody caps both the bytes a response sends and the bytes they decode to:
// generous (Supplement I whole is about 160 KB), so only a runaway response or
// a gzip bomb reaches it.
const maxBody = 64 << 20

// get fetches u and returns the decoded body. It asks for gzip (eCFR's full
// XML endpoint refuses uncompressed requests) and decodes it itself, so the
// bytes returned are exactly what was served. It follows a redirect only to
// u's own scheme and host, so every body comes from the origin asked. Any
// status but 200 is an error, decided before the body is read, quoting the
// start of the body, which is where eCFR puts its reason.
func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	body, _, err := c.getFrom(ctx, u)
	return body, err
}

// getFrom is get, also returning the URL the body came from: u, or where
// u's redirects, all on u's origin, led.
func (c *Client) getFrom(ctx context.Context, u string) (body []byte, from string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", fmt.Errorf("GET %s: %w", u, err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", userAgent)
	hc := c.httpClient()
	resp, err := hc.Do(req)
	if err != nil {
		return nil, "", err // already names the method and URL
	}
	defer resp.Body.Close()
	enc := resp.Header.Get("Content-Encoding")
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s: %s: %s", u, resp.Status, errorSnippet(enc, resp.Body))
	}
	raw, err := readCapped(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("GET %s: reading body: %w", u, err)
	}
	if body, err = decode(enc, raw); err != nil {
		return nil, "", fmt.Errorf("GET %s: %w", u, err)
	}
	return body, resp.Request.URL.String(), nil
}

// httpClient is the client get uses: a copy of c.HTTP, or one with
// defaultTimeout when c.HTTP is nil, with sameOrigin as its redirect policy.
func (c *Client) httpClient() http.Client {
	hc := http.Client{Timeout: defaultTimeout}
	if c.HTTP != nil {
		hc = *c.HTTP
	}
	hc.CheckRedirect = sameOrigin
	return hc
}

// sameOrigin is the redirect policy of get: a redirect is followed only to the
// scheme and host of the first request, so a body never comes from another
// host or over plain http after https.
func sameOrigin(req *http.Request, via []*http.Request) error {
	from := via[0].URL
	if req.URL.Scheme != from.Scheme || !strings.EqualFold(req.URL.Host, from.Host) {
		return fmt.Errorf("refusing a redirect to %s: only %s://%s is trusted", req.URL, from.Scheme, from.Host)
	}
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

// maxErrorBody is how much of an error response's body errorSnippet reads:
// eCFR's reasons are short JSON, and the status is the error.
const maxErrorBody = 8 << 10

// errorSnippet quotes the start of an error response's body, decoded when it
// can be and as received when it can't: the status is the error, and nothing
// about the body may hide it.
func errorSnippet(encoding string, r io.Reader) string {
	raw, _ := io.ReadAll(io.LimitReader(r, maxErrorBody))
	if body, err := decode(encoding, raw); err == nil {
		return parse.Snippet(body)
	}
	return parse.Snippet(raw)
}

// readCapped reads r to the end, failing past maxBody bytes.
func readCapped(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxBody {
		return nil, fmt.Errorf("body exceeds %d bytes", maxBody)
	}
	return b, nil
}

// decode undoes a gzip Content-Encoding (named in any case; x-gzip is gzip);
// an absent or identity one is the body as is.
func decode(encoding string, raw []byte) ([]byte, error) {
	switch strings.ToLower(encoding) {
	case "", "identity":
		return raw, nil
	case "gzip", "x-gzip":
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("gzip body: %w", err)
		}
		body, err := readCapped(zr) // checks the gzip checksum and length
		if err != nil {
			return nil, fmt.Errorf("gzip body: %w", err)
		}
		return body, nil
	default:
		return nil, fmt.Errorf("unsupported Content-Encoding %q", encoding)
	}
}

// firstInvalid returns the offset of the first byte of b that is not part of
// a valid UTF-8 sequence, or -1.
func firstInvalid(b []byte) int {
	for i := 0; i < len(b); {
		r, n := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && n <= 1 {
			return i
		}
		i += n
	}
	return -1
}

// document returns nil when b is one well-formed XML document whose root
// element is want; otherwise an error completing "body is ...". The
// well-formedness check is parse.WellFormed, the one Parse also runs first, so
// fetch and Parse refuse the same malformed XML; see its doc for what it
// checks beyond encoding/xml's strict parse. The root element is fetch's to
// check: it is the section asked for.
func document(b []byte, want rootElement) error {
	return parse.WellFormed(b, func(t xml.StartElement) error {
		if got := rootOf(t); got != want {
			return fmt.Errorf("not %s's eCFR XML: root element %s, want %s", want.N, got, want)
		}
		return nil
	})
}

// rootOf is start's name and TYPE and N attributes. A namespaced element or
// attribute keeps its namespace in the name, so it never equals an eCFR root.
// parse.WellFormed refuses duplicate attributes before it calls the root
// check, so each is read once.
func rootOf(start xml.StartElement) rootElement {
	r := rootElement{Name: parse.Qualified(start.Name)}
	for _, a := range start.Attr {
		switch parse.Qualified(a.Name) {
		case "TYPE":
			r.Type = a.Value
		case "N":
			r.N = a.Value
		}
	}
	return r
}
