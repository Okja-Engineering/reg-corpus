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
// from what is stored: a version other than the stored one, a fresh fetch
// whose tree or sha256_text differs, a govinfo edition whose text differs.
// It is reported and the command exits 1, as for an *l0.Conflict. It writes
// nothing, but for a govinfo mismatch, which writes verified_on: null
// (spec "L0 meta.yaml"; ADR 0014, F8: E-a).
type Finding struct{ msg string }

func (e *Finding) Error() string { return e.msg }

func findingf(format string, a ...any) error {
	return &Finding{fmt.Sprintf(format, a...)}
}

// govinfoCache keeps each granule --verify compares, by its sha256, under
// .cache/ (git-ignored, never committed): the same bytes are kept once,
// and other bytes beside them, never over them (storeCache).
const govinfoCache = ".cache/govinfo"

// granule is the govinfo granule --verify compared, kept in the cache at
// path only once verified_on is decided and written: the cache is not
// evidence (worklist round 1, §2).
type granule struct {
	path    string
	body    []byte
	matches bool
}

// verify is `regcorpus fetch --verify` for s (R1; plan step 6, ADR 0014),
// and returns the lines to print. Under s's lock (l0.Update), after the
// stored-state check, it compares, in order, and stops at the first that
// fails: what is stored, derived again from the raw XML that check read
// (storedVersion); eCFR's latest version date, then that version's fresh
// tree and sha256_text (freshVersion); then govinfo's 2026 edition
// (edition). It sets only verified_on: the run's UTC date on a match,
// unless it holds a date already (F5: V-a); null on a govinfo mismatch (F4
// a), which is also a *Finding. Then, the verdict written, it keeps the
// granule in the cache; a cache that can't keep it is a *cacheError (exit
// 2) beside that verdict. A source whose tree is a later plan step's
// (Goldens) is refused first, before the lock.
func (f fetcher) verify(ctx context.Context, s Source) ([]string, error) {
	if s.Goldens != "" {
		return nil, l0.Conflictf("tree and sha256_text are %s's: fetch --verify compares a source's tree and sha256_text, which it has only once its parser goldens are in (stage 01), so nothing is written", s.Goldens)
	}
	var lines []string
	say := func(format string, a ...any) { lines = append(lines, s.Name+": "+fmt.Sprintf(format, a...)) }
	today := meta.Date(f.now().UTC().Format(time.DateOnly))
	var g *granule
	b, w, err := l0.Update(f.root, s.stored(), func(m *meta.Meta, raw []byte, st l0.State) (*l0.Derived, error) {
		stored, err := storedVersion(f.root, s, m, raw, st, say)
		if err != nil {
			return nil, err
		}
		fresh, err := f.freshVersion(ctx, s, m, stored.Tree, say)
		if err != nil {
			return nil, err
		}
		if g, err = f.edition(ctx, s, m, fresh, say); err != nil {
			return nil, err
		}
		switch {
		case !g.matches:
			m.VerifiedOn = &meta.NullDate{}
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
	var mismatch error
	if !g.matches {
		mismatch = findingf("the govinfo %s edition's normalized text differs from the stored text, so verified_on is null, written (exit 1)", govinfo.EditionYear)
	}
	kept, err := storeCache(f.root, g.path, g.body)
	if err != nil {
		return lines, errors.Join(mismatch, err) // each reported, the larger code winning (Main)
	}
	verb = "unchanged"
	if kept {
		verb = "stored"
	}
	say("govinfo: %s %s", verb, g.path)
	return lines, mismatch
}

// storedVersion is what --verify compares a fresh fetch with: s's tree,
// text.md and sha256_text derived again from raw, the raw XML the
// stored-state check has just read, by anchors.Build, the one derivation
// regcorpus parse writes them by. verified_on vouches for the stored L0 as
// a whole, so it vouches only for what it derives again (worklist round 1,
// §1): unless the stored tree.json and text.md are Build's bytes
// (l0.State.Stale, the comparison parse's writes make) and meta.yaml's
// sha256_text is Build's, it is refused, an *l0.Conflict before any request,
// naming each that differs. Refused too: an overrides.yaml, whatever it
// is, since applying overrides isn't built (R1a) and the text built
// without one would not be the stored text; raw XML Build refuses.
func storedVersion(root string, s Source, m *meta.Meta, raw []byte, st l0.State, say func(string, ...any)) (*anchors.Derived, error) {
	src := s.stored()
	if st.Overrides() {
		return nil, l0.Conflictf("%s exists, and applying overrides isn't built yet: the first source that needs a reviewed override builds it, and fetch --verify then lists them (R1a); nothing is written", src.OverridesPath())
	}
	rawPath := src.RawPath(m.SourceVersionDate)
	d, err := anchors.Build(raw)
	if err != nil {
		return nil, l0.Conflictf("%s: %v; nothing is written", rawPath, err)
	}
	stale, err := st.Stale(root, src, l0.Derived{Tree: d.TreeJSON, Text: d.TextMD})
	if err != nil {
		return nil, err
	}
	if m.SHA256Text != d.SHA256Text {
		stale = append(stale, src.MetaPath()+"'s sha256_text")
	}
	if len(stale) > 0 {
		return nil, l0.Conflictf("%s: missing, or not what regcorpus parse derives from %s, and fetch --verify vouches only for what it derives again: run regcorpus parse %s to regenerate them; nothing is written (R1)",
			strings.Join(stale, ", "), rawPath, s.Name)
	}
	say("stored: tree.json, text.md and sha256_text are as regcorpus parse derives them from %s", rawPath)
	say("overrides: none")
	return d, nil
}

// freshVersion asks eCFR's version list where the stored version stands
// (standOf): eCFR's latest must be it, dated m's source_version_date. Then
// it fetches that version, builds it as regcorpus parse does
// (anchors.Build), and compares it with stored, the stored tree, and m's
// sha256_text (F6: i, T2), naming what differs. The normalized text is the
// version (F1: A): raw bytes that differ are information only. Any
// difference is a *Finding.
func (f fetcher) freshVersion(ctx context.Context, s Source, m *meta.Meta, stored *parse.Tree, say func(string, ...any)) (*anchors.Derived, error) {
	vs, err := f.client.Versions(ctx, s)
	if err != nil {
		return nil, err
	}
	sd, err := standOf(vs, m.SourceVersionDate)
	if err != nil {
		return nil, err
	}
	if !sd.current {
		say("version: eCFR's latest %s, stored source_version_date %s: differ", sd.latestText(), m.SourceVersionDate)
		return nil, findingf("stored source_version_date %s, eCFR's latest %s: %s: %s; nothing is written (R1)", m.SourceVersionDate, sd.latestText(), sd.verdict, sd.advice())
	}
	say("version: eCFR's latest %s, stored source_version_date %s: equal", sd.latest, m.SourceVersionDate)
	body, err := f.client.XML(ctx, s, sd.latest)
	if err != nil {
		return nil, err
	}
	d, err := anchors.Build(body)
	if err != nil {
		return nil, findingf("eCFR's fresh XML at %s doesn't build as regcorpus parse builds the stored version: %v; nothing is written", sd.latest, err)
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
		var where string
		if len(changed) == 0 {
			if where, err = outsideNodes(s.Identifier, stored, d.Tree); err != nil {
				return nil, err
			}
		}
		say("sha256_text: differs, fresh %s, stored %s%s", d.SHA256Text, m.SHA256Text, where)
		differs = append(differs, fmt.Sprintf("sha256_text: fresh %s, stored %s%s", d.SHA256Text, m.SHA256Text, where))
	}
	say("sha256_raw: fresh %s, stored %s (information only)", l0.SHA256Hex(body), m.SHA256Raw)
	if len(rawOnly) > 0 {
		say("raw text differs, normalized alike (information only): %s", strings.Join(rawOnly, ", "))
	}
	if len(differs) > 0 {
		return nil, findingf("eCFR's fresh fetch of %s differs from the stored version: %s; nothing is written (R1)", sd.latest, strings.Join(differs, "; "))
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

// outsideNodes says, for two trees of section whose nodes compareTrees
// finds the same, what else sha256_text covers that differs (F1: "names
// the changed nodes"; worklist round 1, §7): the heading, normalized, and
// which nodes start a <P> (Para), since sha256_text joins the own texts of
// one <P> with nothing between them and two <P>s with a space. It is ""
// when neither differs, and otherwise starts with "; ".
func outsideNodes(section string, stored, fresh *parse.Tree) (string, error) {
	var named []string
	sh, err := anchors.Normalize(stored.Heading)
	if err != nil {
		return "", err
	}
	fh, err := anchors.Normalize(fresh.Heading)
	if err != nil {
		return "", err
	}
	if sh != fh {
		named = append(named, fmt.Sprintf("the heading: stored %q, fresh %q", sh, fh))
	}
	sn, fn := anchors.NewDocument(stored).Nodes(), anchors.NewDocument(fresh).Nodes()
	starts := func(ns []*parse.Node, i int) bool { return i == 0 || ns[i].Para != ns[i-1].Para }
	for i := range sn {
		switch s, f := starts(sn, i), starts(fn, i); {
		case f && !s:
			named = append(named, anchors.ID(section, sn[i].Path)+" starts a paragraph in the fresh fetch, not in the stored version")
		case s && !f:
			named = append(named, anchors.ID(section, sn[i].Path)+" starts a paragraph in the stored version, not in the fresh fetch")
		}
	}
	if len(named) == 0 {
		return "", nil
	}
	return " (" + strings.Join(named, "; ") + ")", nil
}

// edition fetches s's granule in govinfo's 2026 edition (Client.Granule),
// checks it (govinfo.Read) and compares it with fresh, eCFR's text (F3:
// C1; worklist round 1, §4 d): it matches when its heading and its
// paragraphs, one for one and as many, equal eCFR's, each normalized, and
// its text, joined and normalized as sha256_text's is, hashes to m's
// sha256_text. On a mismatch it says what differs (F4 b): the hash, the
// paragraph counts, the heading, and each run of paragraphs between those
// that match (align), so a paragraph split, merged, added or removed shows
// alone. An HTTP error or a granule govinfo.Read refuses is an error (exit
// 2), with nothing written. It returns the granule and its cache path.
func (f fetcher) edition(ctx context.Context, s Source, m *meta.Meta, fresh *anchors.Derived, say func(string, ...any)) (*granule, error) {
	from, body, err := f.client.Granule(ctx, s.Identifier)
	if err != nil {
		return nil, fmt.Errorf("govinfo: %w", err)
	}
	sum := l0.SHA256Hex(body)
	g, err := govinfo.Read(body, s.Identifier, govinfo.EditionDate)
	if err != nil {
		return nil, fmt.Errorf("govinfo: %s (sha256 %s): %w", from, sum, err)
	}
	say("govinfo: %s (DATE %s, sha256 %s)", from, g.Date, sum)
	h, err := anchors.Hash(anchors.JoinSection(g.Heading, g.Paragraphs))
	if err != nil {
		return nil, fmt.Errorf("govinfo: %s: %w", from, err)
	}
	heads, err := normalized([]string{fresh.Tree.Heading, g.Heading})
	if err != nil {
		return nil, err
	}
	ours, err := normalized(anchors.NewDocument(fresh.Tree).Paragraphs())
	if err != nil {
		return nil, err
	}
	theirs, err := normalized(g.Paragraphs)
	if err != nil {
		return nil, err
	}
	hunks := align(ours, theirs)
	kept := &granule{
		path:    path.Join(govinfoCache, govinfo.EditionYear, "title-"+govinfo.EditionTitle, s.Dir, sum+".xml"),
		body:    body,
		matches: meta.SHA256(h) == m.SHA256Text && heads[0] == heads[1] && len(hunks) == 0,
	}
	year := govinfo.EditionYear
	if kept.matches {
		say("govinfo %s edition: the heading and each of the %d paragraphs equal eCFR's, normalized, and the text hashes to sha256_text", year, len(ours))
		return kept, nil
	}
	if meta.SHA256(h) == m.SHA256Text {
		say("govinfo %s edition: differs: its normalized text hashes to sha256_text, but its paragraphs are not eCFR's one for one", year)
	} else {
		say("govinfo %s edition: differs: its normalized text hashes to %s, not sha256_text %s", year, h, m.SHA256Text)
	}
	if len(theirs) != len(ours) {
		say("govinfo %s edition: %d paragraphs, eCFR's %d", year, len(theirs), len(ours))
	}
	if heads[0] != heads[1] {
		say("govinfo differs at the heading:")
		say("  eCFR:    %q", heads[0])
		say("  govinfo: %q", heads[1])
	}
	for _, k := range hunks {
		say("govinfo differs at eCFR's %s, govinfo's %s:", span(k.a0, k.a1, len(ours)), span(k.b0, k.b1, len(theirs)))
		for _, side := range []struct {
			who string
			ps  []string
		}{{"eCFR:   ", ours[k.a0:k.a1]}, {"govinfo:", theirs[k.b0:k.b1]}} {
			if len(side.ps) == 0 {
				say("  %s (none)", side.who)
			}
			for _, p := range side.ps {
				say("  %s %q", side.who, p)
			}
		}
	}
	return kept, nil
}

// normalized is each of texts normalized (anchors.Normalize, decision 13).
func normalized(texts []string) ([]string, error) {
	out := make([]string, len(texts))
	for i, t := range texts {
		n, err := anchors.Normalize(t)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

// hunk is a run of paragraphs that differ, between two that match: a[a0:a1]
// against b[b0:b1], one side possibly empty.
type hunk struct{ a0, a1, b0, b1 int }

// align matches a and b, two sides' paragraphs, by a longest common
// subsequence of equal paragraphs, and returns the runs between the
// matches: none when a and b are equal. So one paragraph split, merged,
// added or removed is one run, and never shifts the rest.
func align(a, b []string) []hunk {
	n, m := len(a), len(b)
	lcs := make([][]int, n+1) // lcs[i][j]: the length of one for a[i:] and b[j:]
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var hunks []hunk
	var open *hunk
	for i, j := 0, 0; i < n || j < m; {
		if i < n && j < m && a[i] == b[j] {
			if open != nil {
				open.a1, open.b1 = i, j
				hunks = append(hunks, *open)
				open = nil
			}
			i, j = i+1, j+1
			continue
		}
		if open == nil {
			open = &hunk{a0: i, b0: j}
		}
		if j == m || i < n && lcs[i+1][j] >= lcs[i][j+1] {
			i++
		} else {
			j++
		}
	}
	if open != nil {
		open.a1, open.b1 = n, m
		hunks = append(hunks, *open)
	}
	return hunks
}

// span names paragraphs [from, to) of total, counted from 1.
func span(from, to, total int) string {
	switch to - from {
	case 0:
		if from == 0 {
			return fmt.Sprintf("no paragraph, before paragraph 1 of %d", total)
		}
		return fmt.Sprintf("no paragraph, after paragraph %d of %d", from, total)
	case 1:
		return fmt.Sprintf("paragraph %d of %d", from+1, total)
	}
	return fmt.Sprintf("paragraphs %d-%d of %d", from+1, to, total)
}
