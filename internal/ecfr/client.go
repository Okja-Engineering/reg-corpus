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
// version. IssueDate is as listed, unchecked: a JSON string's value, any
// other JSON value's text, "" when the list doesn't say. Only fetch
// --latest's note reads it, and checks it there (ParseDate), so a list
// whose issue_date isn't a date fails nothing else, as before plan step 6.
type Version struct {
	Date       Date
	Identifier string
	Removed    bool
	IssueDate  string
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
// and s's own identifier. Its issue_date is kept as listed (Version).
func (c *Client) Versions(ctx context.Context, s Source) ([]Version, error) {
	u := c.url("/versions/title-"+title+".json", s.query())
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var doc struct {
		ContentVersions *[]struct {
			Date       string          `json:"date"`
			Identifier string          `json:"identifier"`
			Removed    bool            `json:"removed"`
			IssueDate  json.RawMessage `json:"issue_date"`
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
		out[i] = Version{Date: d, Identifier: v.Identifier, Removed: v.Removed, IssueDate: listed(v.IssueDate)}
	}
	return out, nil
}

// listed is a JSON value as a list gives it: a string's value, null as "",
// anything else as its JSON text.
func listed(v json.RawMessage) string {
	var s *string
	if json.Unmarshal(v, &s) == nil {
		if s == nil {
			return ""
		}
		return *s
	}
	return string(v)
}

// Latest is eCFR's latest version in vs, the one reading of it that plain
// fetch and fetch --verify and --latest share (standOf): the list's newest
// date, and whether an entry at that date removed the source, whatever the
// order the list gives them in. An empty list is an error. What a removal
// means is the caller's: plain fetch has no text to store (exit 2), while
// --verify and --latest report it as a finding.
func Latest(vs []Version) (latest Date, removed bool, err error) {
	if len(vs) == 0 {
		return "", false, errors.New("eCFR lists no versions")
	}
	for _, v := range vs {
		latest = max(latest, v.Date) // YYYY-MM-DD orders as text
	}
	for _, v := range vs {
		removed = removed || v.Date == latest && v.Removed
	}
	return latest, removed, nil
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
// u's redirects, all on u's origin, led. Each error names the URL that
// failed: that one, and u too when a redirect led there. A URL a redirect
// led to is printed in the saved output, so it must be one as RFC 3986
// writes it, printable ASCII; any other is refused, quoted (worklist round
// 2, §3). u is the client's own.
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
		return nil, "", err // already names the method and the URL that failed, quoted
	}
	defer resp.Body.Close()
	from = resp.Request.URL.String()
	if strings.IndexFunc(from, func(r rune) bool { return r <= ' ' || r > '~' }) >= 0 {
		return nil, "", fmt.Errorf("GET %s: redirected to %q, which is not a URL of printable ASCII, so it is not read", u, from)
	}
	got := from
	if from != u {
		got += " (redirected from " + u + ")"
	}
	enc := resp.Header.Get("Content-Encoding")
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s: %s: %s", got, resp.Status, errorSnippet(enc, resp.Body))
	}
	raw, err := readCapped(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("GET %s: reading body: %w", got, err)
	}
	if body, err = decode(enc, raw); err != nil {
		return nil, "", fmt.Errorf("GET %s: %w", got, err)
	}
	return body, from, nil
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
		return fmt.Errorf("refusing a redirect to %q: only %s://%s is trusted", req.URL, from.Scheme, from.Host)
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
