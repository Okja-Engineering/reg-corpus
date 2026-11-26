package govinfo

import (
	"errors"

	"github.com/okja-engineering/reg-corpus/internal/meta"
)

// DefaultBaseURL is govinfo, whose link service and content need no API key
// (plan "Upstream calls").
const DefaultBaseURL = "https://www.govinfo.gov"

// The edition fetch --verify compares (spec "Concerns" 4; ADR 0014): the CFR
// 2026 annual edition, title 12 revised as of 2026-01-01, pinned by year.
const (
	EditionYear           = "2026"
	EditionDate meta.Date = "2026-01-01"
)

// Indent is the most indentation govinfo's pretty-printer puts after a line
// break inside a section's text (ADR 0014).
const Indent = 12

// The granules Read refuses, each named in the error it returns.
var (
	ErrNotGranule   = errors.New("not a govinfo CFR section granule as this reads one")
	ErrOtherSection = errors.New("a granule for another section")
	ErrOtherEdition = errors.New("a granule of another edition")
)

// Section is a granule's section text, its layout deleted.
type Section struct {
	Date       meta.Date
	Heading    string
	Paragraphs []string
}

// LinkPath is not built yet (plan step 6).
func LinkPath(title, section, year string) (string, error) {
	return "", errors.New("govinfo: LinkPath not built")
}

// Layout is not built yet (plan step 6).
func Layout(s string) string { return s }

// Read is not built yet (plan step 6).
func Read(granule []byte, section string, date meta.Date) (*Section, error) {
	return nil, errors.New("govinfo: Read not built")
}
