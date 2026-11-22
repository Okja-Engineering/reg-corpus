package ecfr

import (
	"context"
	"errors"
	"net/http"
)

// DefaultBaseURL is the eCFR versioner API (plan "Upstream calls").
const DefaultBaseURL = ""

// Date is an eCFR date, YYYY-MM-DD.
type Date string

// Version is one entry of a source's eCFR version list.
type Version struct {
	Date        Date
	Identifier  string
	Substantive bool
	Removed     bool
}

// Client calls the eCFR versioner API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a client for DefaultBaseURL.
func NewClient() *Client { return &Client{} }

var errNotBuilt = errors.New("not built yet")

// Versions returns s's version list.
func (c *Client) Versions(ctx context.Context, s Source) ([]Version, error) {
	return nil, errNotBuilt
}

// Latest returns the latest version in vs.
func Latest(vs []Version) (Version, error) { return Version{}, errNotBuilt }

// XML returns s's point-in-time XML at date.
func (c *Client) XML(ctx context.Context, s Source, date Date) ([]byte, error) {
	return nil, errNotBuilt
}

// UpToDateAsOf returns title 12's "up to date as of" date.
func (c *Client) UpToDateAsOf(ctx context.Context) (Date, error) { return "", errNotBuilt }
