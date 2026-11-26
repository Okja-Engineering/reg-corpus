package meta

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Date is a date written YYYY-MM-DD. Values come from ParseDate.
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

// Tier is a source's tier (spec "L0 meta.yaml").
type Tier string

// The tiers the spec names.
const (
	TierLaw                     Tier = "law"
	TierRegulatorInterpretation Tier = "regulator_interpretation"
	TierInvestorGuide           Tier = "investor_guide"
	TierStandard                Tier = "standard"
)

// Role is a source's role: inventoried, or cite-only (spec "Definitions":
// records may cite it, but it is neither inventoried nor extracted).
type Role string

// The roles the spec names.
const (
	RoleInventoried Role = "inventoried"
	RoleCiteOnly    Role = "cite_only"
)

// Source is where a source's text comes from. The spec names one.
type Source string

// SourceECFR is the eCFR versioner API (spec "Upstream").
const SourceECFR Source = "ecfr"

// Citation is a Federal Register citation, "<vol> FR <page>" (spec "L0
// meta.yaml"). Values come from ParseCitation.
type Citation string

var citation = regexp.MustCompile(`^[1-9][0-9]* FR [1-9][0-9]*$`)

// ParseCitation returns s as a Citation, or an error unless s is
// "<vol> FR <page>": two numbers without leading zeros, single spaces.
func ParseCitation(s string) (Citation, error) {
	if !citation.MatchString(s) {
		return "", fmt.Errorf(`%q is not "<vol> FR <page>"`, s)
	}
	return Citation(s), nil
}

// ParseCitations returns vals as a version_fr_citations list, or an error
// unless it names at least one citation, each a Citation, each once.
func ParseCitations(vals []string) ([]Citation, error) {
	if len(vals) == 0 {
		return nil, errors.New("an empty list; it names at least one citation when set")
	}
	cs := make([]Citation, len(vals))
	for i, v := range vals {
		c, err := ParseCitation(v)
		if err != nil {
			return nil, err
		}
		if slices.Contains(cs[:i], c) {
			return nil, fmt.Errorf("%q twice; the list names each citation once", v)
		}
		cs[i] = c
	}
	return cs, nil
}

// SHA256 is a sha256 written as 64 lowercase hex digits.
type SHA256 string

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

func parseSHA256(s string) (SHA256, error) {
	if !sha256Hex.MatchString(s) {
		return "", fmt.Errorf("%q is not a sha256 (64 lowercase hex digits)", s)
	}
	return SHA256(s), nil
}

// NullDate is verified_on once it is set: a date (Valid), or null (the
// normalized text didn't match the official edition; spec "L0 meta.yaml").
type NullDate struct {
	Date  Date
	Valid bool
}

// Meta is an L0 source's meta.yaml (spec "L0 meta.yaml"; R1). Every
// meta.yaml has the fields fetch writes when it creates one (plan step 2).
// The others are absent until the plan step that owns them sets them; absent
// is the zero value: "" for a Date or SHA256, nil for VersionFRCitations and
// VerifiedOn.
type Meta struct {
	CFR                  string // e.g. "12 CFR 1024.41"
	Tier                 Tier
	Role                 Role
	Source               Source
	Official             bool
	SourceVersionDate    Date       // eCFR version date
	VersionEffectiveDate Date       // plan step 3
	VersionFRCitations   []Citation // plan step 3; never empty when present
	RetrievedAt          Date
	ECFRUpToDateAsOf     Date
	VerifiedOn           *NullDate // plan step 6
	SHA256Raw            SHA256
	SHA256Text           SHA256 // plan step 5
}

// field is one meta.yaml key: whether every meta.yaml has it, and how its
// value is written and read. render returns the value as written, ok false
// when absent, or an error when the value can't be written; value is the
// value as JSON holds it, read only once render has written it; parse reads
// a value as written into m.
type field struct {
	key      string
	required bool
	render   func(m *Meta) (v string, ok bool, err error)
	value    func(m *Meta) any
	parse    func(m *Meta, v string) error
}

// schema is spec "L0 meta.yaml", in its order. required marks the fields
// fetch writes when it creates the file.
var schema = []field{
	text("cfr", true, func(m *Meta) *string { return &m.CFR }, parseCFR),
	word("tier", func(m *Meta) *Tier { return &m.Tier },
		TierLaw, TierRegulatorInterpretation, TierInvestorGuide, TierStandard),
	word("role", func(m *Meta) *Role { return &m.Role }, RoleInventoried, RoleCiteOnly),
	word("source", func(m *Meta) *Source { return &m.Source }, SourceECFR),
	{key: "official", required: true,
		render: func(m *Meta) (string, bool, error) { return strconv.FormatBool(m.Official), true, nil },
		value:  func(m *Meta) any { return m.Official },
		parse: func(m *Meta, v string) error {
			switch v {
			case "true", "false":
				m.Official = v == "true"
				return nil
			}
			return fmt.Errorf("%q is not true or false", v)
		}},
	text("source_version_date", true, func(m *Meta) *Date { return &m.SourceVersionDate }, ParseDate),
	text("version_effective_date", false, func(m *Meta) *Date { return &m.VersionEffectiveDate }, ParseDate),
	{key: "version_fr_citations", render: renderCitations, parse: parseCitations,
		value: func(m *Meta) any { return m.VersionFRCitations }},
	text("retrieved_at", true, func(m *Meta) *Date { return &m.RetrievedAt }, ParseDate),
	text("ecfr_up_to_date_as_of", true, func(m *Meta) *Date { return &m.ECFRUpToDateAsOf }, ParseDate),
	{key: "verified_on", render: renderVerifiedOn, parse: parseVerifiedOn,
		value: func(m *Meta) any {
			if !m.VerifiedOn.Valid {
				return nil
			}
			return m.VerifiedOn.Date
		}},
	text("sha256_raw", true, func(m *Meta) *SHA256 { return &m.SHA256Raw }, parseSHA256),
	text("sha256_text", false, func(m *Meta) *SHA256 { return &m.SHA256Text }, parseSHA256),
}

// Fields returns the keys of spec "L0 meta.yaml", in its order.
func Fields() []string {
	out := make([]string, len(schema))
	for i, f := range schema {
		out[i] = f.key
	}
	return out
}

// text is a field whose value is a double-quoted string (quote), so no
// YAML reader types a date or a hash as anything but a string. parse checks
// the unquoted value; "" is absent.
func text[T ~string](key string, required bool, p func(*Meta) *T, parse func(string) (T, error)) field {
	return field{key: key, required: required,
		render: func(m *Meta) (string, bool, error) {
			v := *p(m)
			if v == "" {
				return "", false, nil
			}
			if _, err := parse(string(v)); err != nil {
				return "", true, err
			}
			q, err := quote(string(v))
			return q, true, err
		},
		value: func(m *Meta) any { return string(*p(m)) },
		parse: func(m *Meta, v string) error {
			u, err := unquote(v)
			if err != nil {
				return err
			}
			t, err := parse(u)
			if err != nil {
				return err
			}
			*p(m) = t
			return nil
		}}
}

// word is a required field whose value is one of the words the spec names,
// written plain as in the spec.
func word[T ~string](key string, p func(*Meta) *T, words ...T) field {
	check := func(v string) error {
		for _, w := range words {
			if v == string(w) {
				return nil
			}
		}
		return fmt.Errorf("%q is not one of %v", v, words)
	}
	return field{key: key, required: true,
		render: func(m *Meta) (string, bool, error) {
			v := string(*p(m))
			if v == "" {
				return "", false, nil
			}
			return v, true, check(v)
		},
		value: func(m *Meta) any { return string(*p(m)) },
		parse: func(m *Meta, v string) error {
			if err := check(v); err != nil {
				return err
			}
			*p(m) = T(v)
			return nil
		}}
}

// parseCFR accepts a non-empty single-line string.
func parseCFR(s string) (string, error) {
	if s == "" || strings.ContainsAny(s, "\r\n") {
		return "", fmt.Errorf("%q is not a citation on one line", s)
	}
	return s, nil
}

// quote writes s as meta.yaml's double-quoted string: strconv.Quote's form,
// whose escapes YAML reads as Go does, but for \x: Go reads \xe9 as the byte
// 0xe9, YAML as U+00E9. So a string Quote writes with a \x escape (one that
// isn't UTF-8, or holds a control character Quote writes that way) is
// refused.
func quote(s string) (string, error) {
	q := strconv.Quote(s)
	for i := 0; i < len(q); i++ {
		if q[i] != '\\' {
			continue
		}
		if i++; q[i] == 'x' { // Quote ends every escape before the closing quote
			return "", fmt.Errorf(`%s has a \x escape, which YAML reads as another character than Go does`, q)
		}
	}
	return q, nil
}

// unquote returns the string v writes, when v is written as quote writes
// it; anything else is an error.
func unquote(v string) (string, error) {
	u, err := strconv.Unquote(v)
	if err != nil || strconv.Quote(u) != v {
		return "", fmt.Errorf("%s is not a double-quoted string as meta.yaml writes one", v)
	}
	if _, err := quote(u); err != nil {
		return "", err
	}
	return u, nil
}

// renderCitations writes version_fr_citations as a flow list of quoted
// citations, as in the spec: ["90 FR 20792", "86 FR 34899"].
func renderCitations(m *Meta) (string, bool, error) {
	if m.VersionFRCitations == nil {
		return "", false, nil
	}
	vals := make([]string, len(m.VersionFRCitations))
	for i, c := range m.VersionFRCitations {
		vals[i] = string(c)
	}
	if _, err := ParseCitations(vals); err != nil {
		return "", true, err
	}
	for i, v := range vals {
		vals[i] = strconv.Quote(v) // a citation has no escape
	}
	return "[" + strings.Join(vals, ", ") + "]", true, nil
}

func parseCitations(m *Meta, v string) error {
	inner, ok := strings.CutPrefix(v, "[")
	if ok {
		inner, ok = strings.CutSuffix(inner, "]")
	}
	if !ok {
		return fmt.Errorf(`%s is not a list written ["<vol> FR <page>", ...]`, v)
	}
	var vals []string
	if inner != "" {
		for _, q := range strings.Split(inner, ", ") {
			u, err := unquote(q)
			if err != nil {
				return err
			}
			vals = append(vals, u)
		}
	}
	cs, err := ParseCitations(vals)
	if err != nil {
		return err
	}
	m.VersionFRCitations = cs
	return nil
}

func renderVerifiedOn(m *Meta) (string, bool, error) {
	switch v := m.VerifiedOn; {
	case v == nil:
		return "", false, nil
	case !v.Valid && v.Date != "":
		return "", true, fmt.Errorf("null with a date, %q", v.Date)
	case !v.Valid:
		return "null", true, nil
	default:
		if _, err := ParseDate(string(v.Date)); err != nil {
			return "", true, err
		}
		return strconv.Quote(string(v.Date)), true, nil
	}
}

func parseVerifiedOn(m *Meta, v string) error {
	if v == "null" {
		m.VerifiedOn = &NullDate{}
		return nil
	}
	u, err := unquote(v)
	if err != nil {
		return fmt.Errorf("%w, nor null", err)
	}
	d, err := ParseDate(u)
	if err != nil {
		return err
	}
	m.VerifiedOn = &NullDate{Date: d, Valid: true}
	return nil
}

// Marshal writes m as meta.yaml: one "key: value" line per field present, in
// the spec's order. It refuses (an error naming the fields) a Meta lacking a
// field every meta.yaml has, or holding a value Parse would refuse, so what it
// writes always parses back to m.
func Marshal(m Meta) ([]byte, error) {
	var b strings.Builder
	err := written(&m, func(f field, v string) { b.WriteString(f.key + ": " + v + "\n") })
	if err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// JSON writes m as one compact JSON object (`regcorpus text --json`, plan
// step 7, ADR 0015): the fields Marshal writes, keyed and ordered as there,
// each value as its type is: a string for a text or word field, a boolean
// for official, an array of strings for version_fr_citations, and
// verified_on a string or null. Nothing is HTML-escaped. It refuses what
// Marshal refuses, so the fields are always those of a meta.yaml Marshal
// could write.
func JSON(m Meta) ([]byte, error) {
	var b bytes.Buffer
	var encErr error
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	b.WriteByte('{')
	err := written(&m, func(f field, _ string) {
		if b.Len() > 1 {
			b.WriteByte(',')
		}
		for i, v := range []any{f.key, f.value(&m)} {
			if i > 0 {
				b.WriteByte(':')
			}
			if err := enc.Encode(v); err != nil {
				encErr = cmp.Or(encErr, err)
				continue
			}
			b.Truncate(b.Len() - 1) // Encode ends each value with an LF
		}
	})
	if err = cmp.Or(err, encErr); err != nil {
		return nil, err
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// written calls emit with each field m holds, in the spec's order, and its
// value as Marshal writes it. It refuses (an error naming the fields) a Meta
// lacking a field every meta.yaml has, or holding a value Parse would
// refuse; it is the one walk of the schema Marshal and JSON both make.
func written(m *Meta, emit func(f field, v string)) error {
	var missing, bad []string
	for _, f := range schema {
		v, ok, err := f.render(m)
		switch {
		case err != nil:
			bad = append(bad, f.key+": "+err.Error())
		case ok:
			emit(f, v)
		case f.required:
			missing = append(missing, "no "+f.key)
		}
	}
	if problems := append(missing, bad...); len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// Parse reads b, a meta.yaml, only when it is exactly what Marshal writes:
// lines "key: value" ending in a newline, known keys in the spec's order,
// each once, every field fetch writes present, each value of its type and
// written the one way Marshal writes it. The error names the line and field.
func Parse(b []byte) (Meta, error) {
	var m Meta
	s := string(b)
	if s != "" && !strings.HasSuffix(s, "\n") {
		return m, errors.New("the last line does not end with a newline")
	}
	next := 0 // index in schema of the first key a line may still name
	seen := map[string]bool{}
	for i, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		if s == "" {
			break
		}
		n := i + 1
		key, v, ok := strings.Cut(line, ": ")
		at := index(key)
		switch {
		case !ok:
			return m, fmt.Errorf(`line %d: %q is not "key: value"`, n, line)
		case at < 0:
			return m, fmt.Errorf("line %d: unknown field %q; meta.yaml holds only %s", n, key, strings.Join(Fields(), ", "))
		case seen[key]:
			return m, fmt.Errorf("line %d: %s appears twice", n, key)
		case at < next:
			return m, fmt.Errorf("line %d: %s comes after %s; the spec's order puts it before", n, key, schema[next-1].key)
		}
		if err := schema[at].parse(&m, v); err != nil {
			return m, fmt.Errorf("line %d: %s: %w", n, key, err)
		}
		seen[key], next = true, at+1
	}
	var missing []string
	for _, f := range schema {
		if f.required && !seen[f.key] {
			missing = append(missing, "no "+f.key)
		}
	}
	if len(missing) > 0 {
		return m, errors.New(strings.Join(missing, ", "))
	}
	return m, nil
}

func index(key string) int {
	for i, f := range schema {
		if f.key == key {
			return i
		}
	}
	return -1
}
