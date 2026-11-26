package ecfr

import (
	"net/url"
	"path"
	"strconv"

	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
)

// Kind is how eCFR selects a source within part 1024: the query key of the
// versioner API, which also names the source in its version list.
type Kind string

// KindSection selects a CFR section. eCFR versions whole sections and whole
// appendices (spec "Granularity"); Supplement I (plan step 8) adds "appendix".
const KindSection Kind = "section"

// rootElement is the root element of eCFR's full XML for one source: the
// element name and its TYPE and N attributes, e.g. <DIV8 TYPE="SECTION"
// N="1024.41">.
type rootElement struct{ Name, Type, N string }

// String writes the root element as a message names it: its name as
// parse.Qualified gives it and each value quoted (strconv.Quote), since a
// served root's are eCFR's (worklist round 2, §3).
func (r rootElement) String() string {
	s := "<" + r.Name
	if r.Type != "" {
		s += " TYPE=" + strconv.Quote(r.Type)
	}
	if r.N != "" {
		s += " N=" + strconv.Quote(r.N)
	}
	return s + ">"
}

// kindRoots is the root element eCFR serves for each kind, its N being the
// source's Identifier (as in the committed raw XML). Supplement I (plan step
// 8) adds KindAppendix: {"DIV9", "APPENDIX"}, as eCFR served it on 2026-11-22.
var kindRoots = map[Kind]rootElement{
	KindSection: {Name: "DIV8", Type: "SECTION"},
}

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
	Tier       meta.Tier
	Role       meta.Role
	Kind       Kind
	Identifier string // eCFR's identifier for Kind, e.g. "1024.41"
	// Goldens is the plan step that adds the source's parser goldens, empty
	// once they are in. Until then `regcorpus parse` refuses the source and
	// writes none of its derived files (stage 01: until a source's goldens
	// exist, make test says nothing about R1a for it; ADR 0013).
	Goldens string
}

// sources is the fixed source table, in fetch order (R1; spec "Upstream" and
// "Granularity").
// Supplement I to Part 1024 (plan step 8) joins it as an "appendix" source
// with Identifier "Supplement I to Part 1024", Dir "supplement-i/41" and tier
// regulator_interpretation; eCFR versions it whole.
var sources = []Source{
	section("1024.41", meta.RoleInventoried, ""),
	section("1024.30", meta.RoleCiteOnly, "plan step 9"),
	section("1024.31", meta.RoleCiteOnly, "plan step 9"),
}

func section(id string, role meta.Role, goldens string) Source {
	return Source{
		Name: id, Dir: id, CFR: "12 CFR " + id,
		Tier: meta.TierLaw, Role: role, Kind: KindSection, Identifier: id,
		Goldens: goldens,
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

// stored is s as internal/l0 reads and writes it: its directory, and the
// cfr, tier and role its meta.yaml must name.
func (s Source) stored() l0.Source {
	return l0.Source{Dir: s.dir(), CFR: s.CFR, Tier: s.Tier, Role: s.Role}
}

// root is the root element of s's full XML. A kind with no entry in kindRoots
// has an empty name, which no element matches, so its XML is refused.
func (s Source) root() rootElement {
	r := kindRoots[s.Kind]
	r.N = s.Identifier
	return r
}
