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
)

// DefaultBaseURL is the eCFR versioner API (plan "Upstream calls"). It needs
// no API key.
const DefaultBaseURL = "https://www.ecfr.gov/api/versioner/v1"

// userAgent is generic on purpose: no request carries personal details.
const userAgent = "regcorpus"

// Date is an eCFR date, YYYY-MM-DD. Values come from ParseDate.
type Date string

// ParseDate returns s as a Date, or an error unless s is a real calendar date
// written YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil || t.Format(time.DateOnly) != s {
		return "", fmt.Errorf("%q is not YYYY-MM-DD", s)
	}
	return Date(s), nil
}

// Version is one entry of a source's eCFR version list: the date eCFR serves
// it at (full/{date}), and whether that version removed the source.
type Version struct {
	Date       Date
	Identifier string
	Removed    bool
}

// Client calls the eCFR versioner API at BaseURL.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a client for DefaultBaseURL.
func NewClient() *Client {
	return &Client{BaseURL: DefaultBaseURL, HTTP: &http.Client{Timeout: 2 * time.Minute}}
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
			Date       string `json:"date"`
			Identifier string `json:"identifier"`
			Removed    bool   `json:"removed"`
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
		out[i] = Version{Date: d, Identifier: v.Identifier, Removed: v.Removed}
	}
	return out, nil
}

// Latest returns the version with the latest date in vs. It fails on an empty
// list, and when an entry at that date removed the source: there is no text
// to store.
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
			return Version{}, fmt.Errorf("eCFR's latest version of %s, %s, removed it", v.Identifier, v.Date)
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
// mid-update.
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
		Meta struct {
			ImportInProgress bool `json:"import_in_progress"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", fmt.Errorf("GET %s: titles JSON: %w", u, err)
	}
	if doc.Meta.ImportInProgress {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", userAgent)
	hc := *c.HTTP
	hc.CheckRedirect = sameOrigin
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err // already names the method and URL
	}
	defer resp.Body.Close()
	enc := resp.Header.Get("Content-Encoding")
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s: %s", u, resp.Status, errorSnippet(enc, resp.Body))
	}
	raw, err := readCapped(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GET %s: reading body: %w", u, err)
	}
	body, err := decode(enc, raw)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	return body, nil
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

// errorSnippet quotes the start of an error response's body, decoded when it
// can be and as received when it can't: the status is the error, and nothing
// about the body may hide it.
func errorSnippet(encoding string, r io.Reader) string {
	raw, _ := readCapped(r)
	if body, err := decode(encoding, raw); err == nil {
		return snippet(body)
	}
	return snippet(raw)
}

// readCapped reads r to the end, failing past maxBody bytes. On an error it
// also returns what it read.
func readCapped(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxBody+1))
	if err != nil {
		return b, err
	}
	if len(b) > maxBody {
		return b[:maxBody], fmt.Errorf("body exceeds %d bytes", maxBody)
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

// snippet quotes up to the first 300 bytes of an error body.
func snippet(body []byte) string {
	const max = 300
	if len(body) > max {
		return strconv.Quote(string(body[:max])) + "..."
	}
	return strconv.Quote(string(body))
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

// bom is the UTF-8 byte order mark. It may open a document and is not part of
// it (XML 1.0, 4.3.3).
var bom = []byte("\xef\xbb\xbf")

// document returns nil when b is exactly one well-formed XML document whose
// root element is want; otherwise an error completing "body is ...". Strict
// parsing refuses unknown entities and unquoted attributes. Outside the root
// only whitespace, comments and processing instructions may appear, an XML
// declaration only at the very start, and no <!DOCTYPE> or other declaration
// anywhere: eCFR serves none, and a document that has one is not eCFR's.
func document(b []byte, want rootElement) error {
	malformed := func(d *xml.Decoder, format string, a ...any) error {
		line, _ := d.InputPos()
		return fmt.Errorf("not well-formed XML: line %d: %s", line, fmt.Sprintf(format, a...))
	}
	d := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(b, bom)))
	d.Strict = true
	depth, roots := 0, 0
	for first := true; ; first = false {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("not well-formed XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				if roots++; roots > 1 {
					return malformed(d, "a second root element <%s>", t.Name.Local)
				}
				if got := rootOf(t); got != want {
					return fmt.Errorf("not %s's eCFR XML: root element %s, want %s", want.N, got, want)
				}
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && len(bytes.Trim(t, " \t\r\n")) > 0 {
				return malformed(d, "text outside the root element: %s", snippet(t))
			}
		case xml.ProcInst:
			if strings.EqualFold(t.Target, "xml") && !first {
				return malformed(d, "an XML declaration not at the start")
			}
		case xml.Directive:
			return malformed(d, "<!%s>", t)
		}
	}
	if roots == 0 {
		return errors.New("not well-formed XML: no root element")
	}
	return nil
}

// rootOf is start's name and TYPE and N attributes. A namespaced element or
// attribute keeps its prefix in the name, so it never equals an eCFR root.
func rootOf(start xml.StartElement) rootElement {
	qualified := func(n xml.Name) string {
		if n.Space != "" {
			return n.Space + ":" + n.Local
		}
		return n.Local
	}
	r := rootElement{Name: qualified(start.Name)}
	for _, a := range start.Attr {
		switch qualified(a.Name) {
		case "TYPE":
			r.Type = a.Value
		case "N":
			r.N = a.Value
		}
	}
	return r
}
