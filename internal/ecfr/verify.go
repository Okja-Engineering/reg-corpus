package ecfr

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/govinfo"
	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// Finding is what fetch --verify or --latest finds upstream that differs
// from what is stored: a newer version, a removal, a fresh fetch whose tree
// or sha256_text differs, a govinfo edition whose text differs. It is
// reported and the command exits 1, as for an *l0.Conflict. It writes
// nothing, but for a govinfo mismatch, which writes verified_on: null
// (spec "L0 meta.yaml"; ADR 0014, F8: E-a).
type Finding struct{ msg string }

func (e *Finding) Error() string { return e.msg }

func findingf(format string, a ...any) error {
	return &Finding{fmt.Sprintf(format, a...)}
}

// govinfoCache keeps each granule --verify fetches, by its sha256, under
// .cache/ (git-ignored, never committed): the same bytes are kept once,
// and other bytes beside them, never over them (l0.StoreOnce).
const govinfoCache = ".cache/govinfo"

// verify is `regcorpus fetch --verify` for s (R1; plan step 6, ADR 0014),
// and returns the lines to print. Under s's lock (l0.Update), after the
// stored-state check, it compares, in order, and stops at the first that
// fails: what is stored (sha256_text, tree.json, no overrides.yaml); eCFR's
// latest version date, then that version's fresh tree and sha256_text
// (storedVersion, freshVersion); then govinfo's 2026 edition (edition). It
// sets only verified_on: the run's UTC date on a match, unless it holds a
// date already (F5: V-a); null on a govinfo mismatch (F4 a), which is also
// a *Finding. A source whose tree is a later plan step's (Goldens) is
// refused first.
func (f fetcher) verify(ctx context.Context, s Source) ([]string, error) {
	if s.Goldens != "" {
		return nil, l0.Conflictf("tree and sha256_text are %s's: fetch --verify compares a source's tree and sha256_text, which it has only once its parser goldens are in (stage 01), so nothing is written", s.Goldens)
	}
	var lines []string
	say := func(format string, a ...any) { lines = append(lines, s.Name+": "+fmt.Sprintf(format, a...)) }
	today := meta.Date(f.now().UTC().Format(time.DateOnly))
	var mismatch error
	b, w, err := l0.Update(f.root, s.stored(), func(m *meta.Meta, raw []byte, st l0.State) (*l0.Derived, error) {
		stored, err := storedVersion(f.root, s, m, st)
		if err != nil {
			return nil, err
		}
		fresh, err := f.freshVersion(ctx, s, m, stored, say)
		if err != nil {
			return nil, err
		}
		say("overrides: none")
		matches, err := f.edition(ctx, s, m, fresh, say)
		switch {
		case err != nil:
			return nil, err
		case !matches:
			m.VerifiedOn = &meta.NullDate{}
			mismatch = findingf("the govinfo %s edition's normalized text differs from sha256_text, so verified_on is null, written (exit 1)", govinfo.EditionYear)
		case m.VerifiedOn == nil || !m.VerifiedOn.Valid:
			m.VerifiedOn = &meta.NullDate{Date: today, Valid: true}
		}
		return nil, nil
	})
	if err != nil {
		return lines, err
	}
	verb := "unchanged"
	if w.Meta {
		verb = "wrote"
	}
	say("%s %s: %s", verb, s.stored().MetaPath(), fieldLine(b, "verified_on"))
	return lines, mismatch
}

// storedVersion is what --verify compares a fresh fetch with: s's tree.json
// as regcorpus parse writes it, and its meta.yaml's sha256_text. Refused
// (an *l0.Conflict, before any request): an overrides.yaml, whatever it is,
// since applying overrides isn't built (R1a) and a fresh fetch built
// without one would not be the stored text; no sha256_text; no tree.json,
// or one not as parse writes it.
func storedVersion(root string, s Source, m *meta.Meta, st l0.State) (*parse.Tree, error) {
	src := s.stored()
	if st.Overrides() {
		return nil, l0.Conflictf("%s exists, and applying overrides isn't built yet: the first source that needs a reviewed override builds it, and fetch --verify then lists them (R1a); nothing is written", src.OverridesPath())
	}
	if m.SHA256Text == "" {
		return nil, l0.Conflictf("%s has no sha256_text: run regcorpus parse %s first; nothing is written (R1)", src.MetaPath(), s.Name)
	}
	b, err := st.TreeJSON(root, src)
	if err != nil {
		return nil, err
	}
	t, err := anchors.DecodeTree(b)
	if err != nil {
		return nil, l0.Conflictf("%s: %v; nothing is written", src.TreePath(), err)
	}
	return t, nil
}

// freshVersion fetches eCFR's latest version of s, which must be dated m's
// source_version_date, builds it as regcorpus parse does (anchors.Build),
// and compares it with stored, the tree.json, and m's sha256_text (F6: i,
// T2). The normalized text is the version (F1: A): raw bytes that differ
// are information only. Any difference is a *Finding.
func (f fetcher) freshVersion(ctx context.Context, s Source, m *meta.Meta, stored *parse.Tree, say func(string, ...any)) (*anchors.Derived, error) {
	vs, err := f.client.Versions(ctx, s)
	if err != nil {
		return nil, err
	}
	v, err := Latest(vs)
	var removed *RemovedError
	if errors.As(err, &removed) {
		return nil, findingf("%v, so the stored version, %s, is not eCFR's latest: run regcorpus fetch --latest; nothing is written", err, m.SourceVersionDate)
	}
	if err != nil {
		return nil, err
	}
	if v.Date != m.SourceVersionDate {
		say("version: eCFR's latest %s, stored source_version_date %s: differ", v.Date, m.SourceVersionDate)
		return nil, findingf("eCFR's latest version is dated %s, not the stored source_version_date %s: run regcorpus fetch --latest; nothing is written (R1)", v.Date, m.SourceVersionDate)
	}
	say("version: eCFR's latest %s, stored source_version_date %s: equal", v.Date, m.SourceVersionDate)
	body, err := f.client.XML(ctx, s, v.Date)
	if err != nil {
		return nil, err
	}
	d, err := anchors.Build(body)
	if err != nil {
		return nil, findingf("eCFR's fresh XML at %s doesn't build as regcorpus parse builds the stored version: %v; nothing is written", v.Date, err)
	}
	changed, rawOnly := compareTrees(s.Identifier, stored, d.Tree)
	var differs []string
	if len(changed) == 0 {
		say("tree: equal (%d nodes: the same paths in the same order, each with the same sha256)", len(anchors.NewDocument(stored).Nodes()))
	} else {
		say("tree: differs: %s", strings.Join(changed, ", "))
		differs = append(differs, "tree.json: "+strings.Join(changed, ", "))
	}
	if d.SHA256Text == m.SHA256Text {
		say("sha256_text: equal, %s", m.SHA256Text)
	} else {
		say("sha256_text: differs, fresh %s, stored %s", d.SHA256Text, m.SHA256Text)
		differs = append(differs, fmt.Sprintf("sha256_text: fresh %s, stored %s", d.SHA256Text, m.SHA256Text))
	}
	say("sha256_raw: fresh %s, stored %s (information only)", l0.SHA256Hex(body), m.SHA256Raw)
	if len(rawOnly) > 0 {
		say("raw text differs, normalized alike (information only): %s", strings.Join(rawOnly, ", "))
	}
	if len(differs) > 0 {
		return nil, findingf("eCFR's fresh fetch of %s differs from the stored version: %s; nothing is written (R1)", v.Date, strings.Join(differs, "; "))
	}
	return d, nil
}

// compareTrees compares stored and fresh, two trees of section, node by
// node in document order (F6: T2): changed names, by anchor ID, each node
// whose sha256 differs ("changed"), that only stored has ("removed") or
// that only fresh has ("added"), or says the nodes are the same in another
// order; it is empty when both have the same paths in the same order with
// the same sha256s. rawOnly names the nodes whose own text differs while
// their sha256 is the same: raw differences, information only (F1: A).
func compareTrees(section string, stored, fresh *parse.Tree) (changed, rawOnly []string) {
	sn, fn := anchors.NewDocument(stored).Nodes(), anchors.NewDocument(fresh).Nodes()
	id := func(n *parse.Node) string { return anchors.ID(section, n.Path) }
	same := len(sn) == len(fn)
	for i := 0; same && i < len(sn); i++ {
		same = id(sn[i]) == id(fn[i]) && sn[i].SHA256 == fn[i].SHA256
	}
	if same {
		for i, n := range sn {
			if n.Text != fn[i].Text {
				rawOnly = append(rawOnly, id(n))
			}
		}
		return nil, rawOnly
	}
	byID := func(ns []*parse.Node) map[string]*parse.Node {
		out := map[string]*parse.Node{}
		for _, n := range ns {
			out[id(n)] = n
		}
		return out
	}
	sm, fm := byID(sn), byID(fn)
	for _, n := range sn {
		switch f, ok := fm[id(n)]; {
		case !ok:
			changed = append(changed, id(n)+" removed")
		case f.SHA256 != n.SHA256:
			changed = append(changed, id(n)+" changed")
		}
	}
	for _, n := range fn {
		if _, ok := sm[id(n)]; !ok {
			changed = append(changed, id(n)+" added")
		}
	}
	if len(changed) == 0 {
		changed = []string{"the same nodes in another order"}
	}
	return changed, nil
}

// edition fetches s's granule in govinfo's 2026 edition (Client.Granule),
// keeps it under .cache/, checks it (govinfo.Read) and reports whether its
// text, its layout deleted, hashes as m's sha256_text does (F3: C1). On a
// mismatch it says each paragraph, and the heading, that differs from
// fresh's, normalized (F4 b). An HTTP error or a granule govinfo.Read
// refuses is an error (exit 2), with nothing written.
func (f fetcher) edition(ctx context.Context, s Source, m *meta.Meta, fresh *anchors.Derived, say func(string, ...any)) (bool, error) {
	from, body, err := f.client.Granule(ctx, s.Identifier)
	if err != nil {
		return false, fmt.Errorf("govinfo: %w", err)
	}
	sum := l0.SHA256Hex(body)
	cache := path.Join(govinfoCache, govinfo.EditionYear, "title-"+title, s.Dir, sum+".xml")
	g, err := govinfo.Read(body, s.Identifier, govinfo.EditionDate)
	if err != nil {
		return false, fmt.Errorf("govinfo: %s (sha256 %s): %w", from, sum, err)
	}
	say("govinfo: %s (DATE %s, sha256 %s)", from, g.Date, sum)
	stored, err := l0.StoreOnce(f.root, cache, body)
	if err != nil {
		return false, err
	}
	verb := "unchanged"
	if stored {
		verb = "stored"
	}
	say("govinfo: %s %s", verb, cache)
	h, err := anchors.Hash(anchors.JoinSection(g.Heading, g.Paragraphs))
	if err != nil {
		return false, fmt.Errorf("govinfo: %s: %w", from, err)
	}
	if meta.SHA256(h) == m.SHA256Text {
		say("govinfo %s edition: normalized text matches sha256_text", govinfo.EditionYear)
		return true, nil
	}
	say("govinfo %s edition: normalized text differs from sha256_text (govinfo's %s, stored %s)", govinfo.EditionYear, h, m.SHA256Text)
	return false, sayDiffers(fresh.Tree, g, say)
}

// sayDiffers says where g, the edition's section text, differs from t,
// eCFR's: the heading, then each paragraph, in order, whose normalized
// text differs, both sides normalized (F4 b: the diff is recorded in the
// saved output). A paragraph one side lacks shows as "(none)". If no single
// one differs, the texts differ only where the paragraphs join, and it
// says so.
func sayDiffers(t *parse.Tree, g *govinfo.Section, say func(string, ...any)) error {
	ours := append([]string{t.Heading}, anchors.NewDocument(t).Paragraphs()...)
	theirs := append([]string{g.Heading}, g.Paragraphs...)
	shown := func(ps []string, i int) (string, error) {
		if i >= len(ps) {
			return "(none)", nil
		}
		n, err := anchors.Normalize(ps[i])
		return fmt.Sprintf("%q", n), err
	}
	found := false
	for i := range max(len(ours), len(theirs)) {
		a, err := shown(ours, i)
		if err != nil {
			return err
		}
		b, err := shown(theirs, i)
		if err != nil {
			return err
		}
		if a == b {
			continue
		}
		if i == 0 {
			say("govinfo differs at the heading:")
		} else {
			say("govinfo differs at paragraph %d of %d:", i, len(ours)-1)
		}
		say("  eCFR:    %s", a)
		say("  govinfo: %s", b)
		found = true
	}
	if !found {
		say("govinfo differs only where its paragraphs join, not in any one paragraph or the heading")
	}
	return nil
}
