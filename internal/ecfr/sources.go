package ecfr

import (
	"net/url"
	"path"
)

// Tier is a source's tier in meta.yaml (spec "L0 meta.yaml": law,
// regulator_interpretation, investor_guide or standard). Only law is in the
// table today; Supplement I (plan step 8) adds regulator_interpretation.
type Tier string

// TierLaw is the tier of the regulation's own sections.
const TierLaw Tier = "law"

// Role is a source's role in meta.yaml: inventoried, or cite-only (spec
// "Definitions": records may cite it, but it is neither inventoried nor
// extracted).
type Role string

// The roles the spec names.
const (
	RoleInventoried Role = "inventoried"
	RoleCiteOnly    Role = "cite_only"
)

// Kind is how eCFR selects a source within part 1024: the query key of the
// versioner API, which also names the source in its version list.
type Kind string

// KindSection selects a CFR section. eCFR versions whole sections and whole
// appendices (spec "Granularity"); Supplement I (plan step 8) adds "appendix".
const KindSection Kind = "section"

const (
	title = "12"
	part  = "1024"
	// sourcesDir holds every eCFR source directory, by slash path from the
	// corpus root (stage 01 Outputs).
	sourcesDir = "sources/ecfr/title-12"
	// cacheDir holds exploratory point-in-time fetches (`fetch --date`);
	// git ignores .cache/ (stage 01 Outputs).
	cacheDir = ".cache/ecfr/title-12"
)

// Source is one L0 source fetched from eCFR: the name `regcorpus fetch` takes,
// where it is stored, how eCFR selects it, and the tier and role its meta.yaml
// records.
type Source struct {
	Name       string // the command-line name, e.g. "1024.41"
	Dir        string // slash path under sources/ecfr/title-12/
	CFR        string // meta.yaml cfr, e.g. "12 CFR 1024.41"
	Tier       Tier
	Role       Role
	Kind       Kind
	Identifier string // eCFR's identifier for Kind, e.g. "1024.41"
}

// sources is the fixed source table, in fetch order (spec "Sources", R1).
// Supplement I to Part 1024 (plan step 8) joins it as an "appendix" source
// with Identifier "Supplement I to Part 1024", Dir "supplement-i/41" and tier
// regulator_interpretation; eCFR versions it whole.
var sources = []Source{
	section("1024.41", RoleInventoried),
	section("1024.30", RoleCiteOnly),
	section("1024.31", RoleCiteOnly),
}

func section(id string, role Role) Source {
	return Source{
		Name: id, Dir: id, CFR: "12 CFR " + id,
		Tier: TierLaw, Role: role, Kind: KindSection, Identifier: id,
	}
}

// lookup returns the source named name.
func lookup(name string) (Source, bool) {
	for _, s := range sources {
		if s.Name == name {
			return s, true
		}
	}
	return Source{}, false
}

// names lists the source names in table order.
func names() []string {
	out := make([]string, len(sources))
	for i, s := range sources {
		out[i] = s.Name
	}
	return out
}

// query is the versioner API selector for s, e.g. part=1024&section=1024.41.
func (s Source) query() url.Values {
	return url.Values{"part": {part}, string(s.Kind): {s.Identifier}}
}

// dir is s's source directory, by slash path from the corpus root.
func (s Source) dir() string { return path.Join(sourcesDir, s.Dir) }
