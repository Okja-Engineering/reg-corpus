package meta

import "errors"

// Date is a date written YYYY-MM-DD.
type Date string

// ParseDate is a stub (red).
func ParseDate(s string) (Date, error) { return "", errors.New("not built") }

// Tier is a source's tier.
type Tier string

// The tiers the spec names.
const (
	TierLaw                     Tier = "law"
	TierRegulatorInterpretation Tier = "regulator_interpretation"
	TierInvestorGuide           Tier = "investor_guide"
	TierStandard                Tier = "standard"
)

// Role is a source's role.
type Role string

// The roles the spec names.
const (
	RoleInventoried Role = "inventoried"
	RoleCiteOnly    Role = "cite_only"
)

// Source is where a source's text comes from.
type Source string

// SourceECFR is the eCFR.
const SourceECFR Source = "ecfr"

// Citation is a Federal Register citation.
type Citation string

// ParseCitation is a stub (red).
func ParseCitation(s string) (Citation, error) { return "", errors.New("not built") }

// SHA256 is a sha256.
type SHA256 string

// NullDate is a date or null.
type NullDate struct {
	Date  Date
	Valid bool
}

// Meta is an L0 source's meta.yaml.
type Meta struct {
	CFR                  string
	Tier                 Tier
	Role                 Role
	Source               Source
	Official             bool
	SourceVersionDate    Date
	VersionEffectiveDate Date
	VersionFRCitations   []Citation
	RetrievedAt          Date
	ECFRUpToDateAsOf     Date
	VerifiedOn           *NullDate
	SHA256Raw            SHA256
	SHA256Text           SHA256
}

// Fields is a stub (red).
func Fields() []string { return nil }

// Parse is a stub (red).
func Parse(b []byte) (Meta, error) { return Meta{}, nil }

// Marshal is a stub (red).
func Marshal(m Meta) ([]byte, error) { return nil, nil }
