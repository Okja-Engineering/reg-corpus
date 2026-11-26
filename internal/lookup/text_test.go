// Tests for `regcorpus text [--json] <anchor>` (R8; plan step 7, ADR 0015)
// on corpus roots made in t.TempDir(): from 1024.41's committed raw XML,
// meta.yaml, tree.json and text.md (read, never written), or from raw XML
// built here as regcorpus parse builds it (anchors.Build). What it refuses
// and with which exit code; that an anchor ID resolves only to the own
// text regcorpus parse derives for it from the stored raw XML, so every
// forged tree.json or text.md is refused, whatever it agrees with (round 1
// of PR #11, §1); that the resolver's errors are typed (§2, §5); that every
// line goes through the sink in one Write; and that this package has no
// network code. internal/ecfr's tests run it on the committed corpus.
package lookup_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/lookup"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
	"github.com/okja-engineering/reg-corpus/internal/sink"
)

const (
	sourcesDir = "sources/ecfr/title-12"
	dir41      = sourcesDir + "/1024.41"
	metaPath   = dir41 + "/meta.yaml"
	treePath   = dir41 + "/tree.json"
	textPath   = dir41 + "/text.md"
)

var src41 = l0.Source{Dir: dir41, CFR: "12 CFR 1024.41", Tier: meta.TierLaw, Role: meta.RoleInventoried}

// corpus is a source table as internal/ecfr passes one: 1024.41 built,
// Supplement I's comments on it plan step 8's, and 1024.30 and 1024.31 a
// later plan step's.
func corpus(root string) lookup.Corpus {
	return lookup.Corpus{Root: root, Dir: sourcesDir, Sources: []lookup.Source{
		{Name: "1024.41", Stored: src41, Comments: "plan step 8"},
		{Name: "1024.30", Stored: l0.Source{Dir: sourcesDir + "/1024.30", CFR: "12 CFR 1024.30", Tier: meta.TierLaw, Role: meta.RoleCiteOnly},
			Pending: "plan step 9"},
		{Name: "1024.31", Stored: l0.Source{Dir: sourcesDir + "/1024.31", CFR: "12 CFR 1024.31", Tier: meta.TierLaw, Role: meta.RoleCiteOnly},
			Pending: "plan step 9"},
	}}
}

// committed reads the committed file at slash path p.
func committed(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// committedMeta is 1024.41's committed meta.yaml, parsed.
func committedMeta(t *testing.T) meta.Meta {
	t.Helper()
	m, err := meta.Parse(committed(t, metaPath))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// rawPath is 1024.41's committed raw XML, by slash path.
func rawPath(t *testing.T) string {
	t.Helper()
	return src41.RawPath(committedMeta(t).SourceVersionDate)
}

// committedTree is 1024.41's committed tree.json, decoded.
func committedTree(t *testing.T) *parse.Tree {
	t.Helper()
	var tr parse.Tree
	if err := json.Unmarshal(committed(t, treePath), &tr); err != nil {
		t.Fatal(err)
	}
	return &tr
}

func put(t *testing.T, root, slash string, b []byte) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(slash))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func remove(t *testing.T, root, slash string) {
	t.Helper()
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(slash))); err != nil {
		t.Fatal(err)
	}
}

func putMeta(t *testing.T, root string, m meta.Meta) {
	t.Helper()
	b, err := meta.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	put(t, root, metaPath, b)
}

// stored makes a corpus root holding 1024.41's committed raw XML,
// meta.yaml, tree.json and text.md: what regcorpus parse derives.
func stored(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, p := range []string{rawPath(t), metaPath, treePath, textPath} {
		put(t, root, p, committed(t, p))
	}
	return root
}

// built makes a corpus root holding raw as 1024.41's raw XML, beside what
// regcorpus parse derives from it (anchors.Build): its tree.json and
// text.md, and the committed meta.yaml with raw's sha256_raw and Build's
// sha256_text.
func built(t *testing.T, raw []byte) string {
	t.Helper()
	d, err := anchors.Build(raw)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	m := committedMeta(t)
	m.SHA256Raw, m.SHA256Text = meta.SHA256(l0.SHA256Hex(raw)), d.SHA256Text
	put(t, root, rawPath(t), raw)
	put(t, root, treePath, d.TreeJSON)
	put(t, root, textPath, d.TextMD)
	putMeta(t, root, m)
	return root
}

// section is 1024.41's raw XML as eCFR serves a section, its HEAD and one
// <P> per paragraph.
func section(paras ...string) []byte {
	s := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DIV8 N=\"1024.41\" TYPE=\"SECTION\">\n<HEAD>§ 1024.41 Head.</HEAD>\n"
	for _, p := range paras {
		s += "<P>" + p + "</P>\n"
	}
	return []byte(s + "</DIV8>\n")
}

// find is tr's node with anchor ID id, or nil.
func find(tr *parse.Tree, id string) *parse.Node {
	for _, n := range anchors.NewDocument(tr).Nodes() {
		if anchors.ID(tr.Section, n.Path) == id {
			return n
		}
	}
	return nil
}

// node is tr's node with anchor ID id.
func node(t *testing.T, tr *parse.Tree, id string) *parse.Node {
	t.Helper()
	n := find(tr, id)
	if n == nil {
		t.Fatalf("no node %s", id)
	}
	return n
}

// forged makes a stored corpus root whose tree.json is the committed tree
// as edit leaves it, encoded as regcorpus parse encodes a tree, and whose
// text.md and meta.yaml's sha256_text agree with it, each node's sha256
// being its own text's: a forgery every check but deriving the tree again
// from the raw XML passes. raw XML and sha256_raw are the committed ones.
func forged(t *testing.T, edit func(t *testing.T, tr *parse.Tree)) string {
	t.Helper()
	tr := committedTree(t)
	edit(t, tr)
	for _, n := range anchors.NewDocument(tr).Nodes() {
		var err error
		if n.SHA256, err = anchors.Hash(n.Text); err != nil {
			t.Fatal(err)
		}
	}
	tb, err := anchors.EncodeTree(tr)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(tb, committed(t, treePath)) {
		t.Fatal("the forgery doesn't change tree.json")
	}
	root := stored(t)
	put(t, root, treePath, tb)
	if md, err := anchors.TextMD(tr); err == nil {
		put(t, root, textPath, md)
	}
	m := committedMeta(t)
	if m.SHA256Text, err = anchors.SHA256Text(tr); err != nil {
		t.Fatal(err)
	}
	putMeta(t, root, m)
	return root
}

// forgedBytes makes a stored corpus root whose tree.json is the committed
// one's bytes as edit leaves them; nothing else changes.
func forgedBytes(t *testing.T, edit func(b []byte) []byte) string {
	t.Helper()
	b := committed(t, treePath)
	e := edit(bytes.Clone(b))
	if bytes.Equal(e, b) {
		t.Fatal("the forgery doesn't change tree.json")
	}
	root := stored(t)
	put(t, root, treePath, e)
	return root
}

// writes records each Write as it is made.
type writes struct{ calls []string }

func (w *writes) Write(p []byte) (int, error) {
	w.calls = append(w.calls, string(p))
	return len(p), nil
}

func (w *writes) String() string { return strings.Join(w.calls, "") }

// run runs text in c with args, each stream a recorder behind the sink, as
// cmd/regcorpus gives them, and returns the exit code, what each stream
// holds and stdout's Writes.
func run(c lookup.Corpus, args ...string) (code int, stdout, stderr string, outWrites []string) {
	var out, errw writes
	code = lookup.TextMain(c, args, sink.New(&out), sink.New(&errw))
	return code, out.String(), errw.String(), out.calls
}

// refusal is one row of TestTextRefuses.
type refusal struct {
	name  string
	args  []string
	root  func(t *testing.T) string // a corpus root; stored(t) if nil
	code  int
	say   []string // what stderr says
	not   []string // what stderr doesn't say
	parse bool     // stderr says to run regcorpus parse 1024.41
}

// forgeries are the forgery classes PR #11's round-1 hunters built
// (hunt-integrity/roots and hunt-behavior), each a tree.json or text.md
// that regcorpus parse doesn't derive from the stored raw XML, with the ID
// whose output it would have forged. Each is refused, exit 1, nothing on
// stdout, naming what differs and regcorpus parse (§1).
func forgeries() []refusal {
	swap := func(a, b string) func(t *testing.T, tr *parse.Tree) {
		return func(t *testing.T, tr *parse.Tree) {
			x, y := node(t, tr, a), node(t, tr, b)
			x.Path, y.Path = y.Path, x.Path
		}
	}
	setPath := func(id string, path ...string) func(t *testing.T, tr *parse.Tree) {
		return func(t *testing.T, tr *parse.Tree) { node(t, tr, id).Path = path }
	}
	replace := func(id, old, new string) func(t *testing.T, tr *parse.Tree) {
		return func(t *testing.T, tr *parse.Tree) {
			n := node(t, tr, id)
			if !strings.Contains(n.Text, old) {
				t.Fatalf("%s's own text holds no %q", id, old)
			}
			n.Text = strings.Replace(n.Text, old, new, 1)
		}
	}
	tree := func(edit func(t *testing.T, tr *parse.Tree)) func(t *testing.T) string {
		return func(t *testing.T) string { return forged(t, edit) }
	}
	raw := func(edit func(b []byte) []byte) func(t *testing.T) string {
		return func(t *testing.T) string { return forgedBytes(t, edit) }
	}
	before := func(at, insert string) func(b []byte) []byte {
		return func(b []byte) []byte { return bytes.Replace(b, []byte(at), []byte(insert+at), 1) }
	}
	rows := []refusal{
		{name: "paths swapped", args: []string{"1024-41-b-2-i-B"}, root: tree(swap("1024-41-b-2-i-A", "1024-41-b-2-i-B"))},
		{name: "same-label paths swapped", args: []string{"1024-41-c-3-i-B"}, root: tree(swap("1024-41-c-3-i-B", "1024-41-c-3-ii-B"))},
		{name: "a node reparented", args: []string{"1024-41-b-2-i-A"}, root: tree(func(t *testing.T, tr *parse.Tree) {
			i := node(t, tr, "1024-41-b-2-i")
			a, b := i.Children[0], i.Children[1]
			i.Children = append([]*parse.Node{a}, i.Children[2:]...)
			a.Children = append(a.Children, b)
		})},
		{name: "a new ID", args: []string{"1024-41-b-2-i-Z"}, root: tree(setPath("1024-41-b-2-i-B", "b", "2", "i", "Z"))},
		{name: "an empty path segment", args: []string{"1024-41-b-2-i-"}, root: tree(setPath("1024-41-b-2-i-B", "b", "2", "i", ""))},
		{name: "a path segment holding -", args: []string{"1024-41-b-2-i-B"}, root: tree(setPath("1024-41-b-2-i-B", "b-2-i-B"))},
		{name: "a boundary moved, the hashes recomputed", args: []string{"1024-41-b-1"}, root: tree(func(t *testing.T, tr *parse.Tree) {
			b, b1 := node(t, tr, "1024-41-b"), node(t, tr, "1024-41-b-1")
			cut := strings.Index(b1.Text, ". ") + 2
			b.Text, b1.Text = b.Text+b1.Text[:cut], b1.Text[cut:]
		})},
		{name: "an em dash made -", args: []string{"1024-41-b"}, root: tree(replace("1024-41-b", emDash, "-"))},
		{name: "a space made NBSP", args: []string{"1024-41-b-2-i-B"}, root: tree(replace("1024-41-b-2-i-B", " ", nbsp))},
		{name: "a run of spaces", args: []string{"1024-41-b-2-i-B"}, root: tree(replace("1024-41-b-2-i-B", " ", "    "))},
		{name: "another section's tree", args: []string{"1024-41-a"}, root: tree(func(_ *testing.T, tr *parse.Tree) { tr.Section = "1024.31" })},
		{name: "a node with no label path", args: []string{"1024-41-a"}, root: tree(setPath("1024-41-k"))},
		{name: "two nodes with one anchor ID", args: []string{"1024-41-a"}, root: tree(setPath("1024-41-k", "a"))},
		{name: "a duplicate text key", args: []string{"1024-41-b-2-i-B"}, root: raw(before(`"text": "(B) Notify`, `"text": "(B) A decoy a reader sees first.",`+"\n      "))},
		{name: "a case-variant path key", args: []string{"1024-41-b-2-i-Z"}, root: raw(func(b []byte) []byte {
			i := bytes.Index(b, []byte(`"(A) Promptly`))
			j := bytes.LastIndex(b[:i], []byte(`"path"`))
			k := j + bytes.IndexByte(b[j:], ']') + 1
			return append(b[:k:k], append([]byte(`,`+"\n      "+`"PATH": ["b","2","i","Z"]`), b[k:]...)...)
		})},
		{name: "a case-variant section key", args: []string{"1024-41-a"}, root: raw(func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"section": "1024.41"`), []byte(`"section": "1024.30", "SECTION": "1024.41"`), 1)
		})},
		{name: "escaped Unicode", args: []string{"1024-41-b-2-i-B"}, root: raw(func(b []byte) []byte {
			return bytes.Replace(b, []byte(`(B) Notify`), []byte(`\`+"u0028B) Notify"), 1)
		})},
		{name: "trailing whitespace", args: []string{"1024-41-a"}, root: raw(func(b []byte) []byte { return append(b, "\n\n   \t\n"...) })},
		{name: "invalid UTF-8", args: []string{"1024-41-b-2-i-B"}, root: raw(func(b []byte) []byte {
			return bytes.Replace(b, []byte(`(B) Notify`), []byte("(B) Notif\xff"), 1)
		})},
		{name: "another node's bad sha256", args: []string{"1024-41-b-2-i-A"}, root: func(t *testing.T) string {
			sum := node(t, committedTree(t), "1024-41-b-2-i-B").SHA256
			return forgedBytes(t, func(b []byte) []byte { return bytes.Replace(b, []byte(sum), []byte(strings.Repeat("0", 64)), 1) })
		}},
		{name: "an unknown field", args: []string{"1024-41-a"}, root: raw(before(`"section"`, `"extra": 1, `))},
		{name: "a second value", args: []string{"1024-41-a"}, root: raw(func(b []byte) []byte { return append(b, "{}\n"...) })},
		{name: "not JSON", args: []string{"1024-41-a"}, root: raw(func([]byte) []byte { return []byte("not json\n") })},
		{name: "a null node", args: []string{"1024-41-a"}, root: raw(func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"children": [`), []byte(`"children": [null, `), 1)
		})},
	}
	for i := range rows {
		rows[i].code, rows[i].say, rows[i].parse = 1, []string{treePath}, true
	}
	return append(rows,
		refusal{name: "a stale text.md", args: []string{"1024-41-a"}, code: 1, say: []string{textPath}, not: []string{treePath}, parse: true,
			root: func(t *testing.T) string {
				root := stored(t)
				put(t, root, textPath, bytes.Replace(committed(t, textPath), []byte("A borrower may enforce"), []byte("A borrower may not enforce"), 1))
				return root
			}},
	)
}

// TestTextRefuses: each refusal exits with its code (F4, F5; round 1, §1,
// §2, §5), prints nothing to stdout, and says why on stderr.
func TestTextRefuses(t *testing.T) {
	setMeta := func(edit func(m *meta.Meta)) func(t *testing.T) string {
		return func(t *testing.T) string {
			root := stored(t)
			m := committedMeta(t)
			edit(&m)
			putMeta(t, root, m)
			return root
		}
	}
	editRaw := func(fixSum bool) func(t *testing.T) string {
		return func(t *testing.T) string {
			root := stored(t)
			b := bytes.Replace(committed(t, rawPath(t)), []byte("application 45 days or more"), []byte("application 99 days or more"), 1)
			put(t, root, rawPath(t), b)
			if fixSum {
				m := committedMeta(t)
				m.SHA256Raw = meta.SHA256(l0.SHA256Hex(b))
				putMeta(t, root, m)
			}
			return root
		}
	}
	rows := []refusal{
		// Usage (exit 2).
		{name: "no anchor", code: 2, say: []string{"regcorpus text: no anchor", "usage: regcorpus text"}},
		{name: "two anchors", args: []string{"1024-41-a", "1024-41-b"}, code: 2, say: []string{"regcorpus text: ", "usage: regcorpus text"}},
		{name: "a flag after the anchor", args: []string{"1024-41-a", "--json"}, code: 2, say: []string{"usage: regcorpus text"}},
		{name: "an unknown flag", args: []string{"--subtree", "1024-41-a"}, code: 2, say: []string{"usage: regcorpus text"}},
		// A string that names no source (exit 2).
		{name: "the CFR form", args: []string{"1024.41(b)(2)(i)(B)"}, code: 2, say: []string{`"1024.41(b)(2)(i)(B)" names no source`}},
		{name: "a citation", args: []string{"12 CFR 1024.41(b)"}, code: 2, say: []string{"names no source"}},
		{name: "a section not in the table", args: []string{"1024-40-a"}, code: 2, say: []string{"names no source"}},
		{name: "a section's prefix", args: []string{"1024-410-a"}, code: 2, say: []string{"names no source"}},
		{name: "an empty anchor", args: []string{""}, code: 2, say: []string{"names no source"}},
		// Not the corpus root (exit 2), before anything else but usage.
		{name: "not the corpus root", args: []string{"1024-41-a"}, root: func(t *testing.T) string { return t.TempDir() },
			code: 2, say: []string{"is not a corpus root (no sources/ecfr/title-12/ directory); run regcorpus from the corpus root"}},
		{name: "not the corpus root, an ID naming no source", args: []string{"x"}, root: func(t *testing.T) string { return t.TempDir() },
			code: 2, say: []string{"is not a corpus root"}, not: []string{"names no source"}},
		// No such anchor in a known source (exit 1), naming it and its source.
		{name: "no such anchor", args: []string{"1024-41-z"}, code: 1, say: []string{`no anchor "1024-41-z" in 1024.41`}},
		{name: "another case", args: []string{"1024-41-B"}, code: 1, say: []string{`no anchor "1024-41-B" in 1024.41`}},
		{name: "a trailing dash", args: []string{"1024-41-a-"}, code: 1, say: []string{`no anchor "1024-41-a-" in 1024.41`}},
		{name: "the section itself", args: []string{"1024-41"}, code: 1, say: []string{`no anchor "1024-41" in 1024.41`}},
		{name: "a later step's section itself", args: []string{"1024-31"}, code: 1, say: []string{`no anchor "1024-31" in 1024.31`}, not: []string{"plan step"}},
		{name: "an interp segment under 1024-30", args: []string{"1024-30-a-interp-1"}, code: 1,
			say: []string{`no anchor "1024-30-a-interp-1" in 1024.30`}, not: []string{"plan step"}},
		{name: "an interp segment under 1024-31", args: []string{"1024-31-interp-1"}, code: 1,
			say: []string{`no anchor "1024-31-interp-1" in 1024.31`}, not: []string{"plan step"}},
		// Not built yet (exit 1), naming the plan step.
		{name: "a later step's source", args: []string{"1024-31-day"}, code: 1, say: []string{"anchors for 1024.31 are plan step 9's"}},
		{name: "a comment anchor", args: []string{"1024-41-b-2-i-interp-1"}, code: 1, say: []string{"Supplement I comment anchors are plan step 8's"}},
		// meta.yaml refuses (exit 1).
		{name: "no meta.yaml", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string { root := stored(t); remove(t, root, metaPath); return root },
			say:  []string{metaPath + " does not exist", "run regcorpus fetch first"}},
		{name: "a meta.yaml naming another source", args: []string{"1024-41-a"}, code: 1,
			root: setMeta(func(m *meta.Meta) { m.CFR = "12 CFR 1024.31" }), say: []string{"another source"}},
		// The raw XML refuses (exit 1).
		{name: "no raw XML", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string { root := stored(t); remove(t, root, rawPath(t)); return root },
			say:  []string{"holds no raw XML"}},
		{name: "a second raw file", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string {
				root := stored(t)
				put(t, root, dir41+"/raw/2025-05-15.xml", committed(t, rawPath(t)))
				return root
			}, say: []string{dir41 + "/raw/2025-05-15.xml"}},
		{name: "the raw file a symlink", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string {
				root := stored(t)
				p := filepath.Join(root, filepath.FromSlash(rawPath(t)))
				put(t, root, "elsewhere.xml", committed(t, rawPath(t)))
				remove(t, root, rawPath(t))
				if err := os.Symlink(filepath.Join(root, "elsewhere.xml"), p); err != nil {
					t.Fatal(err)
				}
				return root
			}, say: []string{"not a regular file"}},
		{name: "raw/ a file", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string {
				root := stored(t)
				if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(dir41+"/raw"))); err != nil {
					t.Fatal(err)
				}
				put(t, root, dir41+"/raw", []byte("a file\n"))
				return root
			}, say: []string{dir41 + "/raw", "not a real directory"}},
		{name: "the raw XML edited, sha256_raw kept", args: []string{"1024-41-a"}, code: 1, root: editRaw(false), say: []string{"sha256_raw"}},
		{name: "the raw XML edited, sha256_raw its sha256", args: []string{"1024-41-a"}, code: 1, root: editRaw(true), parse: true,
			say: []string{treePath, textPath, metaPath + "'s sha256_text", rawPath(t)}},
		{name: "raw XML parse refuses", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string {
				root := stored(t)
				b := bytes.Replace(committed(t, rawPath(t)), []byte("<P>(b)"), []byte("<P>(c)"), 1)
				put(t, root, rawPath(t), b)
				m := committedMeta(t)
				m.SHA256Raw = meta.SHA256(l0.SHA256Hex(b))
				putMeta(t, root, m)
				return root
			}, say: []string{rawPath(t)}},
		{name: "another section's raw XML, its files derived", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string {
				return built(t, bytes.Replace(section("(a) Text."), []byte(`N="1024.41"`), []byte(`N="1024.31"`), 1))
			}, say: []string{rawPath(t), `"1024.31"`}},
		{name: "an overrides.yaml", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string { root := stored(t); put(t, root, dir41+"/overrides.yaml", nil); return root },
			say:  []string{dir41 + "/overrides.yaml"}},
		// A derived file refuses (exit 1).
		{name: "no tree.json", args: []string{"1024-41-a"}, code: 1, parse: true,
			root: func(t *testing.T) string { root := stored(t); remove(t, root, treePath); return root },
			say:  []string{treePath}, not: []string{textPath}},
		{name: "no text.md", args: []string{"1024-41-a"}, code: 1, parse: true,
			root: func(t *testing.T) string { root := stored(t); remove(t, root, textPath); return root },
			say:  []string{textPath}, not: []string{treePath}},
		{name: "no sha256_text", args: []string{"1024-41-a"}, code: 1, parse: true,
			root: setMeta(func(m *meta.Meta) { m.SHA256Text = "" }), say: []string{metaPath + "'s sha256_text"}},
		{name: "a sha256_text parse doesn't derive", args: []string{"1024-41-a"}, code: 1, parse: true,
			root: setMeta(func(m *meta.Meta) { m.SHA256Text = meta.SHA256(strings.Repeat("0", 64)) }), say: []string{metaPath + "'s sha256_text"}},
		{name: "a case-variant TREE.JSON", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string {
				root := stored(t)
				d := filepath.Join(root, filepath.FromSlash(dir41))
				if err := os.Rename(filepath.Join(d, "tree.json"), filepath.Join(d, "TREE.JSON")); err != nil {
					t.Fatal(err)
				}
				return root
			}, say: []string{dir41 + "/TREE.JSON", "another case"}},
		{name: "a tree.json that is a directory", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string {
				root := stored(t)
				remove(t, root, treePath)
				if err := os.Mkdir(filepath.Join(root, filepath.FromSlash(treePath)), 0o755); err != nil {
					t.Fatal(err)
				}
				return root
			}, say: []string{treePath, "not a regular file"}},
		// A file that can't be read (exit 2).
		{name: "an unreadable tree.json", args: []string{"1024-41-a"}, code: 2, root: unreadable(treePath), say: []string{"tree.json"}},
		{name: "an unreadable raw XML", args: []string{"1024-41-a"}, code: 2, root: unreadable(""), say: []string{".xml"}},
	}
	for _, tc := range append(rows, forgeries()...) {
		t.Run(tc.name, func(t *testing.T) {
			var root string
			if tc.root != nil {
				root = tc.root(t)
			} else {
				root = stored(t)
			}
			code, stdout, stderr, _ := run(corpus(root), tc.args...)
			if code != tc.code || stdout != "" {
				t.Errorf("exit %d, stdout %q; want %d and nothing on stdout (stderr %q)", code, stdout, tc.code, stderr)
			}
			if !strings.HasPrefix(stderr, "regcorpus text: ") {
				t.Errorf("stderr %q doesn't start regcorpus text: ", stderr)
			}
			for _, s := range tc.say {
				if !strings.Contains(stderr, s) {
					t.Errorf("stderr %q lacks %q", stderr, s)
				}
			}
			for _, s := range tc.not {
				if strings.Contains(stderr, s) {
					t.Errorf("stderr %q says %q", stderr, s)
				}
			}
			if tc.parse != strings.Contains(stderr, "regcorpus parse 1024.41") {
				t.Errorf("stderr %q: says to run regcorpus parse 1024.41 %t, want %t", stderr, !tc.parse, tc.parse)
			}
		})
	}
}

// unreadable makes a stored corpus root whose file at slash path p, or
// the raw XML when p is "", has no permissions.
func unreadable(p string) func(t *testing.T) string {
	return func(t *testing.T) string {
		if os.Geteuid() == 0 {
			t.Skip("root reads a file with no permissions")
		}
		root := stored(t)
		if p == "" {
			p = rawPath(t)
		}
		if err := os.Chmod(filepath.Join(root, filepath.FromSlash(p)), 0); err != nil {
			t.Fatal(err)
		}
		return root
	}
}

// TestAnchorErrorsAreTyped: the resolver's errors mean the same to every
// caller (§2): outside the corpus root, ErrNotRoot first, whatever the ID;
// an ID that names no source ErrNoSource; one a later plan step builds
// ErrNotBuilt; a source's own ID, an interp segment under a section
// Supplement I's comments aren't on, and an unknown label ErrNoAnchor; a
// stored file that refuses an *l0.Conflict; each only its own.
func TestAnchorErrorsAreTyped(t *testing.T) {
	kinds := map[string]error{"ErrNotRoot": lookup.ErrNotRoot, "ErrNoSource": lookup.ErrNoSource, "ErrNotBuilt": lookup.ErrNotBuilt, "ErrNoAnchor": lookup.ErrNoAnchor}
	elsewhere := t.TempDir()
	good, forgery := stored(t), forged(t, func(t *testing.T, tr *parse.Tree) {
		a, b := node(t, tr, "1024-41-b-2-i-A"), node(t, tr, "1024-41-b-2-i-B")
		a.Path, b.Path = b.Path, a.Path
	})
	linked := t.TempDir()
	if err := os.Symlink(filepath.Join(good, "sources"), filepath.Join(linked, "sources")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		root, id, kind string // kind "Conflict": an *l0.Conflict
	}{
		{elsewhere, "1024-41-a", "ErrNotRoot"},
		{elsewhere, "x", "ErrNotRoot"},
		{elsewhere, "1024-31-day", "ErrNotRoot"},
		{linked, "1024-41-a", "Conflict"},
		{good, "x", "ErrNoSource"},
		{good, "1024-31-day", "ErrNotBuilt"},
		{good, "1024-41-b-2-i-interp-1", "ErrNotBuilt"},
		{good, "1024-41", "ErrNoAnchor"},
		{good, "1024-31", "ErrNoAnchor"},
		{good, "1024-30-a-interp-1", "ErrNoAnchor"},
		{good, "1024-41-z", "ErrNoAnchor"},
		{forgery, "1024-41-b-2-i-B", "Conflict"},
	} {
		a, err := corpus(tc.root).Anchor(tc.id)
		var c *l0.Conflict
		if a != nil || err == nil || (tc.kind == "Conflict") != errors.As(err, &c) {
			t.Errorf("%s in %s: %+v, %v; want %s", tc.id, tc.root, a, err, tc.kind)
		}
		for name, kind := range kinds {
			if errors.Is(err, kind) != (name == tc.kind) {
				t.Errorf("%s in %s: %v: errors.Is %s %t", tc.id, tc.root, err, name, !(name == tc.kind))
			}
		}
	}
	a, err := corpus(good).Anchor("1024-41-b-2")
	n := node(t, committedTree(t), "1024-41-b-2")
	if err != nil || a.ID != "1024-41-b-2" || a.Text != n.Text || a.SHA256 != n.SHA256 || strings.Join(a.Children, " ") != "1024-41-b-2-i 1024-41-b-2-ii" ||
		a.MetaPath != metaPath || a.Meta.SHA256Text != committedMeta(t).SHA256Text {
		t.Errorf("Anchor(1024-41-b-2): %+v, %v", a, err)
	}
}

// TestTextPrintsTheAnchor: on the committed files, text prints the anchor
// ID, its own text, an empty line, its sha256, its children's IDs and its
// source's meta.yaml path and lines, as stored; each line in one Write.
// --json prints one line in one Write, a JSON object with the keys anchor,
// text, sha256, children, meta_path and meta, in that order.
func TestTextPrintsTheAnchor(t *testing.T) {
	root := stored(t)
	tr := committedTree(t)
	m := committed(t, metaPath)
	n := node(t, tr, "1024-41-b-2")
	want := "1024-41-b-2\n" + n.Text + "\n\nsha256: " + n.SHA256 + "\nchildren: 1024-41-b-2-i 1024-41-b-2-ii\n" +
		"meta.yaml: " + metaPath + "\n" + string(m)
	code, stdout, stderr, calls := run(corpus(root), "1024-41-b-2")
	if code != 0 || stdout != want || stderr != "" {
		t.Fatalf("exit %d, stderr %q, stdout:\n%s\nwant:\n%s", code, stderr, stdout, want)
	}
	if len(calls) != strings.Count(want, "\n") {
		t.Errorf("%d Writes for %d lines: %q", len(calls), strings.Count(want, "\n"), calls)
	}
	for _, c := range calls {
		if strings.Count(c, "\n") != 1 || !strings.HasSuffix(c, "\n") {
			t.Errorf("Write %q is not one line", c)
		}
	}

	leaf := node(t, tr, "1024-41-b-2-i-B")
	code, stdout, _, _ = run(corpus(root), "1024-41-b-2-i-B")
	if code != 0 || !strings.Contains(stdout, "\nchildren: none\n") || !strings.HasPrefix(stdout, "1024-41-b-2-i-B\n"+leaf.Text+"\n\n") {
		t.Errorf("exit %d, stdout:\n%s\nwant a leaf with children: none", code, stdout)
	}

	code, stdout, stderr, calls = run(corpus(root), "--json", "1024-41-b-2")
	if code != 0 || stderr != "" || len(calls) != 1 || strings.Count(stdout, "\n") != 1 || !strings.HasSuffix(stdout, "\n") {
		t.Fatalf("--json: exit %d, stderr %q, Writes %q; want one line in one Write", code, stderr, calls)
	}
	mj, err := meta.JSON(committedMeta(t))
	if err != nil {
		t.Fatal(err)
	}
	enc := func(v any) string {
		var b bytes.Buffer
		e := json.NewEncoder(&b)
		e.SetEscapeHTML(false)
		if err := e.Encode(v); err != nil {
			t.Fatal(err)
		}
		return strings.TrimSuffix(b.String(), "\n")
	}
	wantJSON := `{"anchor":"1024-41-b-2","text":` + enc(n.Text) + `,"sha256":"` + n.SHA256 + `",` +
		`"children":["1024-41-b-2-i","1024-41-b-2-ii"],"meta_path":"` + metaPath + `","meta":` + string(mj) + "}\n"
	if stdout != wantJSON {
		t.Errorf("--json:\n%s\nwant:\n%s", stdout, wantJSON)
	}
	_, stdout, _, _ = run(corpus(root), "--json", "1024-41-b-2-i-B")
	if !strings.Contains(stdout, `"children":[],`) {
		t.Errorf("--json for a leaf: %s; want children []", stdout)
	}
}

// Characters a text holds that the tests write from their code points, so
// no invisible character is in this file: a combining acute accent
// (U+0301), whose NFC with a preceding e is U+00E9; the C1 control
// sequence introducer (U+009B), with how the sink writes it; the em dash
// (U+2014) and the no-break space (U+00A0), which normalize to "-" and a
// space.
var (
	acute, csi   = string(rune(0x301)), string(rune(0x9b))
	csiText      = `\` + "u009b"
	emDash, nbsp = string(rune(0x2014)), string(rune(0xa0))
)

// TestTextPrintsVerbatimInDocumentOrder: on raw XML built as regcorpus
// parse builds it (§6), a lead-in's children print in document order,
// (1) to (10), which is not their lexical order; and an own text holding
// "<", ">" and "&", a decomposed e and an acute accent (not NFC), a tab,
// DEL and a C1 control prints byte for byte as stored but for what the
// sink escapes on its line; with --json, as one line of JSON, nothing
// HTML-escaped, that decodes to the stored text.
func TestTextPrintsVerbatimInDocumentOrder(t *testing.T) {
	paras := []string{"(a) <I>Lead.</I> The rest:"}
	var kids []string
	for i := 1; i <= 10; i++ {
		paras = append(paras, fmt.Sprintf("(%d) Item %d.", i, i))
		kids = append(kids, fmt.Sprintf("1024-41-a-%d", i))
	}
	stored := "(b) a <b> & c e" + acute + " \t\x7f" + csi + "2K end."
	paras = append(paras, "(b) a &lt;b&gt; &amp; c e"+acute+" \t\x7f"+csi+"2K end.")
	root := built(t, section(paras...))

	code, stdout, stderr, _ := run(corpus(root), "1024-41-a")
	if want := "\nchildren: " + strings.Join(kids, " ") + "\n"; code != 0 || stderr != "" || !strings.Contains(stdout, want) {
		t.Errorf("exit %d, stderr %q, stdout:\n%s\nwant %q", code, stderr, stdout, want)
	}
	_, stdout, _, _ = run(corpus(root), "--json", "1024-41-a")
	if want := `"children":["` + strings.Join(kids, `","`) + `"]`; !strings.Contains(stdout, want) {
		t.Errorf("--json: %s\nwant %s", stdout, want)
	}

	code, stdout, stderr, _ = run(corpus(root), "1024-41-b")
	lines := strings.Split(stdout, "\n")
	if want := "(b) a <b> & c e" + acute + ` \x09\x7f` + csiText + "2K end."; code != 0 || stderr != "" || len(lines) < 2 || lines[1] != want {
		t.Errorf("exit %d, stderr %q, stdout:\n%s\nwant line 2 %q", code, stderr, stdout, want)
	}
	code, stdout, stderr, calls := run(corpus(root), "--json", "1024-41-b")
	if code != 0 || stderr != "" || len(calls) != 1 || strings.Count(stdout, "\n") != 1 {
		t.Fatalf("--json: exit %d, stderr %q, Writes %q; want one line", code, stderr, calls)
	}
	if want := `"text":"(b) a <b> & c e` + acute + ` \t\u007f` + csiText + `2K end."`; !strings.Contains(stdout, want) {
		t.Errorf("--json: %s\nwant it to hold %s", stdout, want)
	}
	var got struct{ Text string }
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || got.Text != stored {
		t.Errorf("--json: %s decodes to %q, %v; want the stored text %q", stdout, got.Text, err, stored)
	}
}

// TestTextHelp: -h prints the usage on stdout and exits 0; it states the
// argument, what it reads, the output, the flags and the exit codes.
func TestTextHelp(t *testing.T) {
	for _, h := range []string{"-h", "-help", "--help"} {
		code, stdout, stderr, _ := run(corpus(t.TempDir()), h)
		if code != 0 || stderr != "" || !strings.HasPrefix(stdout, "usage: regcorpus text [--json] <anchor>\n") {
			t.Fatalf("%s: exit %d, stderr %q, stdout %q", h, code, stderr, stdout)
		}
		flat := strings.Join(strings.Fields(stdout), " ")
		for _, want := range []string{"1024-41-b-2-i-B", "Exit 0", "Exit 1", "Exit 2", "--json", "children", "sha256", "meta.yaml",
			"raw XML", "tree.json", "text.md", "--json=false", "the last one given wins", "a symlink or a file"} {
			if !strings.Contains(flat, want) {
				t.Errorf("%s: usage lacks %q", h, want)
			}
		}
	}
}

// TestTextFlagsAreTheStdlibs: --json is a boolean flag as Go's flag
// package reads one, as fetch's are: --json=false turns it off, the last
// one given wins, and -h or --help prints the usage and exits 0, whatever
// else is given (§7).
func TestTextFlagsAreTheStdlibs(t *testing.T) {
	root := stored(t)
	_, plain, _, _ := run(corpus(root), "1024-41-a")
	_, asJSON, _, _ := run(corpus(root), "--json", "1024-41-a")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--json=false", "1024-41-a"}, plain},
		{[]string{"--json", "--json=false", "1024-41-a"}, plain},
		{[]string{"--json=false", "-json", "1024-41-a"}, asJSON},
		{[]string{"-json=t", "1024-41-a"}, asJSON},
	} {
		if code, stdout, stderr, _ := run(corpus(root), tc.args...); code != 0 || stderr != "" || stdout != tc.want {
			t.Errorf("%q: exit %d, stderr %q, stdout %q; want %q", tc.args, code, stderr, stdout, tc.want)
		}
	}
	if code, stdout, _, _ := run(corpus(root), "--json", "--help", "1024-41-a"); code != 0 || !strings.HasPrefix(stdout, "usage: regcorpus text") {
		t.Errorf("--json --help: exit %d, stdout %q; want the usage, exit 0", code, stdout)
	}
}

// TestLookupHasNoNetworkCode: internal/lookup is offline by construction
// (F7): neither it nor anything it imports is net/http or internal/ecfr.
func TestLookupHasNoNetworkCode(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go is required: %v", err)
	}
	out, err := exec.Command(gobin, "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	deps := strings.Fields(string(out))
	if len(deps) == 0 || !strings.Contains(string(out), "github.com/okja-engineering/reg-corpus/internal/lookup") {
		t.Fatalf("go list -deps listed %q; want internal/lookup's dependencies", deps)
	}
	for _, d := range deps {
		if d == "net/http" || d == "net" || d == "github.com/okja-engineering/reg-corpus/internal/ecfr" {
			t.Errorf("internal/lookup depends on %s", d)
		}
	}
}
