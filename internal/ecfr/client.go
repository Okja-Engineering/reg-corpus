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
// as served, after checking it is valid UTF-8 and well-formed XML. Nothing in
// it is changed (R1).
func (c *Client) XML(ctx context.Context, s Source, date Date) ([]byte, error) {
	u := c.url("/full/"+string(date)+"/title-"+title+".xml", s.query())
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(body) {
		return nil, fmt.Errorf("GET %s: body is not valid UTF-8 (first bad byte offset %d)", u, firstInvalid(body))
	}
	if err := wellFormed(body); err != nil {
		return nil, fmt.Errorf("GET %s: body is not well-formed XML: %w", u, err)
	}
	return body, nil
}

// UpToDateAsOf returns title 12's "up to date as of" date from titles.json.
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
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", fmt.Errorf("GET %s: titles JSON: %w", u, err)
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

// get fetches u and returns the decoded body. It asks for gzip (eCFR's full
// XML endpoint refuses uncompressed requests) and decodes it itself, so the
// bytes returned are exactly what was served. Any status but 200 is an error
// quoting the start of the body, which is where eCFR puts its reason.
func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err // already names the method and URL
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GET %s: reading body: %w", u, err)
	}
	body, err := decode(resp.Header.Get("Content-Encoding"), raw)
	if resp.StatusCode != http.StatusOK {
		if err != nil { // the status is the cause; show the body as received
			body = raw
		}
		return nil, fmt.Errorf("GET %s: %s: %s", u, resp.Status, snippet(body))
	}
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	return body, nil
}

// decode undoes a gzip Content-Encoding; an absent one is the body as is.
func decode(encoding string, raw []byte) ([]byte, error) {
	switch encoding {
	case "", "identity":
		return raw, nil
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("gzip body: %w", err)
		}
		body, err := io.ReadAll(zr) // checks the gzip checksum and length
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

// wellFormed reads every token of b as XML.
func wellFormed(b []byte) error {
	d := xml.NewDecoder(bytes.NewReader(b))
	d.Strict = true
	sawElement := false
	for {
		tok, err := d.Token()
		if err == io.EOF {
			if !sawElement {
				return errors.New("no root element")
			}
			return nil
		}
		if err != nil {
			return err
		}
		if _, ok := tok.(xml.StartElement); ok {
			sawElement = true
		}
	}
}
