// Tests for meta.yaml's one writer (spec "L0 meta.yaml"; R1; issue #5): the
// thirteen fields in the spec's order, each value well-typed and written one
// way, a file parsed only when it is exactly what Marshal writes, and the
// fields a later plan step owns absent until it sets them.
package meta

import (
	"reflect"
	"strings"
	"testing"
)

const (
	rawSum  = "3b69726d99fb64837c2f0767f257cff830a1d3602f16a9749615c31f9ace840b"
	textSum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

// created is what fetch writes when it creates a meta.yaml (plan step 2).
func created() Meta {
	return Meta{
		CFR: "12 CFR 1024.41", Tier: TierLaw, Role: RoleInventoried, Source: SourceECFR, Official: false,
		SourceVersionDate: "2025-07-15", RetrievedAt: "2026-11-22", ECFRUpToDateAsOf: "2026-11-19",
		SHA256Raw: rawSum,
	}
}

const createdYAML = `cfr: "12 CFR 1024.41"
tier: law
role: inventoried
source: ecfr
official: false
source_version_date: "2025-07-15"
retrieved_at: "2026-11-22"
ecfr_up_to_date_as_of: "2026-11-19"
sha256_raw: "` + rawSum + `"
`

// full has every field set, the later steps' too.
func full() Meta {
	m := created()
	m.VersionEffectiveDate = "2025-07-15"
	m.VersionFRCitations = []Citation{"90 FR 20792", "86 FR 34899"}
	m.VerifiedOn = &NullDate{Date: "2026-11-23", Valid: true}
	m.SHA256Text = textSum
	return m
}

const fullYAML = `cfr: "12 CFR 1024.41"
tier: law
role: inventoried
source: ecfr
official: false
source_version_date: "2025-07-15"
version_effective_date: "2025-07-15"
version_fr_citations: ["90 FR 20792", "86 FR 34899"]
retrieved_at: "2026-11-22"
ecfr_up_to_date_as_of: "2026-11-19"
verified_on: "2026-11-23"
sha256_raw: "` + rawSum + `"
sha256_text: "` + textSum + `"
`

// TestMarshalAndParseAreInverses: Marshal writes the spec's fields in its
// order, absent fields left out; Parse reads that file back to the same
// value. So an update that parses, sets a field and marshals keeps every
// other field byte for byte.
func TestMarshalAndParseAreInverses(t *testing.T) {
	step3 := created()
	step3.VersionEffectiveDate = "2025-07-15"
	step3.VersionFRCitations = []Citation{"90 FR 20792"}
	nullVerified := full()
	nullVerified.VerifiedOn = &NullDate{}
	for _, tc := range []struct {
		name string
		m    Meta
		yaml string
	}{
		{"as fetch creates it", created(), createdYAML},
		{"with step 3's fields", step3, strings.Replace(createdYAML, "retrieved_at:",
			"version_effective_date: \"2025-07-15\"\nversion_fr_citations: [\"90 FR 20792\"]\nretrieved_at:", 1)},
		{"every field", full(), fullYAML},
		{"verified_on null", nullVerified, strings.Replace(fullYAML, `verified_on: "2026-11-23"`, "verified_on: null", 1)},
		{"another tier and role", func() Meta {
			m := created()
			m.Tier, m.Role = TierRegulatorInterpretation, RoleCiteOnly
			return m
		}(), strings.Replace(strings.Replace(createdYAML, "tier: law", "tier: regulator_interpretation", 1), "role: inventoried", "role: cite_only", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := Marshal(tc.m)
			if err != nil || string(b) != tc.yaml {
				t.Fatalf("Marshal: %v\n%s\nwant:\n%s", err, b, tc.yaml)
			}
			m, err := Parse([]byte(tc.yaml))
			if err != nil || !reflect.DeepEqual(m, tc.m) {
				t.Fatalf("Parse: %+v, %v; want %+v", m, err, tc.m)
			}
		})
	}
}

// TestParseRefusesAnythingMarshalWouldNotWrite: known keys only, in the
// spec's order, each once, each value of its type and written one way, and
// every field fetch writes present. Each error names the field or line.
func TestParseRefusesAnythingMarshalWouldNotWrite(t *testing.T) {
	line := func(old, new string) string {
		if !strings.Contains(fullYAML, old) {
			t.Fatalf("fixture lacks %q", old)
		}
		return strings.Replace(fullYAML, old, new, 1)
	}
	for _, tc := range []struct {
		name, yaml string
		want       []string // in the error
	}{
		// Layout.
		{"no final newline", strings.TrimSuffix(fullYAML, "\n"), []string{"newline"}},
		{"a blank line", line("tier: law\n", "tier: law\n\n"), []string{"line 3"}},
		{"a comment", line("tier: law\n", "tier: law\n# note\n"), []string{"line 3", "# note"}},
		{"CRLF", strings.ReplaceAll(fullYAML, "\n", "\r\n"), []string{"line 1", "cfr"}},
		{"two spaces after the colon", line(`cfr: "12`, `cfr:  "12`), []string{"line 1", "cfr"}},
		{"no space after the colon", line(`cfr: "12`, `cfr:"12`), []string{"line 1"}},
		{"an unknown key", line("tier: law\n", "tier: law\nnote: \"x\"\n"), []string{"line 3", `"note"`}},
		{"a key in another case", line("tier: law", "Tier: law"), []string{"line 2", `"Tier"`}},
		{"a key twice", fullYAML + "sha256_text: \"" + textSum + "\"\n", []string{"line 14", "sha256_text", "twice"}},
		{"keys out of the spec's order", line("tier: law\nrole: inventoried\n", "role: inventoried\ntier: law\n"),
			[]string{"line 3", "tier", "role"}},
		{"a later field before an earlier one", line("retrieved_at: \"2026-11-22\"\n", "") + "retrieved_at: \"2026-11-22\"\n",
			[]string{"line 13", "retrieved_at"}},
		// Presence: every field fetch writes.
		{"empty", "", []string{"no cfr", "no source_version_date", "no sha256_raw"}},
		{"no sha256_raw", line("sha256_raw: \""+rawSum+"\"\n", ""), []string{"no sha256_raw"}},
		{"no official", line("official: false\n", ""), []string{"no official"}},
		// Values.
		{"an empty cfr", line(`cfr: "12 CFR 1024.41"`, `cfr: ""`), []string{"cfr"}},
		{"an unquoted cfr", line(`cfr: "12 CFR 1024.41"`, `cfr: 12 CFR 1024.41`), []string{"cfr"}},
		{"a tier the spec doesn't name", line("tier: law", "tier: statute"), []string{"tier", "statute"}},
		{"a quoted tier", line("tier: law", `tier: "law"`), []string{"tier"}},
		{"a role the spec doesn't name", line("role: inventoried", "role: cited"), []string{"role", "cited"}},
		{"a source the spec doesn't name", line("source: ecfr", "source: govinfo"), []string{"source", "govinfo"}},
		{"official not a bool", line("official: false", "official: no"), []string{"official", "no"}},
		{"official quoted", line("official: false", `official: "false"`), []string{"official"}},
		{"an unquoted date", line(`source_version_date: "2025-07-15"`, `source_version_date: 2025-07-15`), []string{"source_version_date"}},
		{"a single-quoted date", line(`source_version_date: "2025-07-15"`, `source_version_date: '2025-07-15'`), []string{"source_version_date"}},
		{"a date that isn't one", line(`source_version_date: "2025-07-15"`, `source_version_date: "2025-02-30"`), []string{"source_version_date", "2025-02-30"}},
		{"a date with an escape", line(`retrieved_at: "2026-11-22"`, `retrieved_at: "2026`+`\`+`u002d11-22"`), []string{"retrieved_at"}},
		{"verified_on quoted null", line(`verified_on: "2026-11-23"`, `verified_on: "null"`), []string{"verified_on"}},
		{"verified_on empty", line(`verified_on: "2026-11-23"`, `verified_on: `), []string{"verified_on"}},
		{"a short hash", line(`sha256_raw: "`+rawSum+`"`, `sha256_raw: "deadbeef"`), []string{"sha256_raw", "deadbeef"}},
		{"an upper-case hash", line(`sha256_text: "`+textSum+`"`, `sha256_text: "`+strings.ToUpper(textSum)+`"`), []string{"sha256_text"}},
		// version_fr_citations: a list of "<vol> FR <page>", written one way.
		{"citations not a list", line(`version_fr_citations: ["90 FR 20792", "86 FR 34899"]`, `version_fr_citations: "90 FR 20792"`),
			[]string{"version_fr_citations"}},
		{"an empty citation list", line(`["90 FR 20792", "86 FR 34899"]`, `[]`), []string{"version_fr_citations", "empty"}},
		{"citations without the space", line(`["90 FR 20792", "86 FR 34899"]`, `["90 FR 20792","86 FR 34899"]`), []string{"version_fr_citations"}},
		{"spaces inside the brackets", line(`["90 FR 20792", "86 FR 34899"]`, `[ "90 FR 20792", "86 FR 34899" ]`), []string{"version_fr_citations"}},
		{"a block list", line(`version_fr_citations: ["90 FR 20792", "86 FR 34899"]`, "version_fr_citations:\n  - \"90 FR 20792\""),
			[]string{"version_fr_citations"}},
		{"an unquoted citation", line(`["90 FR 20792", "86 FR 34899"]`, `[90 FR 20792]`), []string{"version_fr_citations"}},
		{"a citation without its page", line(`["90 FR 20792", "86 FR 34899"]`, `["90 FR"]`), []string{"version_fr_citations", "90 FR"}},
		{"a citation as F.R.", line(`["90 FR 20792", "86 FR 34899"]`, `["90 F.R. 20792"]`), []string{"version_fr_citations"}},
		{"a citation with a leading zero", line(`["90 FR 20792", "86 FR 34899"]`, `["90 FR 020792"]`), []string{"version_fr_citations"}},
		{"a citation with two spaces", line(`["90 FR 20792", "86 FR 34899"]`, `["90  FR 20792"]`), []string{"version_fr_citations"}},
		{"a citation with a date", line(`["90 FR 20792", "86 FR 34899"]`, `["90 FR 20792, May 16, 2025"]`), []string{"version_fr_citations"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Parse([]byte(tc.yaml))
			if err == nil {
				t.Fatalf("Parse succeeded: %+v; want an error", m)
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q lacks %q", err, w)
				}
			}
		})
	}
}

// TestMarshalRefusesWhatParseWouldRefuse: Marshal never writes a file Parse
// would refuse, so no writer can leave a meta.yaml the next one can't read.
func TestMarshalRefusesWhatParseWouldRefuse(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(*Meta)
		want string
	}{
		{"no cfr", func(m *Meta) { m.CFR = "" }, "no cfr"},
		{"no tier", func(m *Meta) { m.Tier = "" }, "no tier"},
		{"no source_version_date", func(m *Meta) { m.SourceVersionDate = "" }, "no source_version_date"},
		{"no sha256_raw", func(m *Meta) { m.SHA256Raw = "" }, "no sha256_raw"},
		{"a cfr with a newline", func(m *Meta) { m.CFR = "12 CFR\n1024.41" }, "cfr"},
		{"an unknown tier", func(m *Meta) { m.Tier = "statute" }, "tier"},
		{"an unknown role", func(m *Meta) { m.Role = "cited" }, "role"},
		{"an unknown source", func(m *Meta) { m.Source = "govinfo" }, "source"},
		{"a bad date", func(m *Meta) { m.RetrievedAt = "2026-13-01" }, "retrieved_at"},
		{"a bad effective date", func(m *Meta) { m.VersionEffectiveDate = "July 15, 2025" }, "version_effective_date"},
		{"an empty citation list", func(m *Meta) { m.VersionFRCitations = []Citation{} }, "version_fr_citations"},
		{"a bad citation", func(m *Meta) { m.VersionFRCitations = []Citation{"90 FR 20792", "90 FR"} }, "version_fr_citations"},
		{"a bad verified_on", func(m *Meta) { m.VerifiedOn = &NullDate{Date: "soon", Valid: true} }, "verified_on"},
		{"a null verified_on with a date", func(m *Meta) { m.VerifiedOn = &NullDate{Date: "2026-11-23"} }, "verified_on"},
		{"a bad hash", func(m *Meta) { m.SHA256Text = "deadbeef" }, "sha256_text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := full()
			tc.set(&m)
			b, err := Marshal(m)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Marshal: %q, %v; want an error naming %q", b, err, tc.want)
			}
		})
	}
}

func TestParseCitation(t *testing.T) {
	for _, s := range []string{"90 FR 20792", "1 FR 1", "81 FR 72370"} {
		if c, err := ParseCitation(s); err != nil || string(c) != s {
			t.Errorf("ParseCitation(%q) = %q, %v; want it back", s, c, err)
		}
	}
	for _, s := range []string{"", "90 FR", "FR 20792", "90 FR 20792 ", " 90 FR 20792", "90 fr 20792", "90 F.R. 20792",
		"0 FR 1", "90 FR 0", "90 FR 020792", "90\tFR 20792", "90 FR 20792-20795", "90 FR 20,792"} {
		if c, err := ParseCitation(s); err == nil {
			t.Errorf("ParseCitation(%q) = %q; want an error", s, c)
		}
	}
}

func TestParseDateTakesOnlyRealCalendarDates(t *testing.T) {
	for _, s := range []string{"2025-07-15", "2024-02-29"} {
		if d, err := ParseDate(s); err != nil || string(d) != s {
			t.Errorf("ParseDate(%q) = %q, %v; want it back", s, d, err)
		}
	}
	for _, s := range []string{"", "2025-7-15", "2025-02-29", "2025-07-15T00:00:00Z", "20250715"} {
		if d, err := ParseDate(s); err == nil || !strings.Contains(err.Error(), `"`+s+`"`) {
			t.Errorf("ParseDate(%q) = %q, %v; want an error quoting it", s, d, err)
		}
	}
}

// TestFieldsAreTheSpecsInItsOrder: the thirteen keys of spec "L0 meta.yaml",
// in its order.
func TestFieldsAreTheSpecsInItsOrder(t *testing.T) {
	want := []string{"cfr", "tier", "role", "source", "official", "source_version_date", "version_effective_date",
		"version_fr_citations", "retrieved_at", "ecfr_up_to_date_as_of", "verified_on", "sha256_raw", "sha256_text"}
	if got := Fields(); !reflect.DeepEqual(got, want) {
		t.Errorf("Fields() = %q, want %q", got, want)
	}
}
