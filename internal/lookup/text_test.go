// Tests for `regcorpus text [--json] <anchor>` (R8; plan step 7, ADR 0015)
// on corpus roots made in t.TempDir() from 1024.41's committed meta.yaml and
// tree.json (read, never written): what it refuses and with which exit
// code, that it checks what it reads before printing anything, that every
// line goes through the sink in one Write, and that this package has no
// network code. internal/ecfr's tests run it on the committed corpus.
package lookup_test

import (
	"bytes"
	"encoding/json"
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
)

// corpus is a source table as internal/ecfr passes one: 1024.41 built, and
// 1024.31 a later plan step's.
func corpus(root string) lookup.Corpus {
	return lookup.Corpus{Root: root, Dir: sourcesDir, Sources: []lookup.Source{
		{Name: "1024.41", Stored: l0.Source{Dir: dir41, CFR: "12 CFR 1024.41", Tier: meta.TierLaw, Role: meta.RoleInventoried}},
		{Name: "1024.31", Stored: l0.Source{Dir: sourcesDir + "/1024.31", CFR: "12 CFR 1024.31", Tier: meta.TierLaw, Role: meta.RoleCiteOnly},
			Pending: "plan step 9"},
	}}
}

// committed reads 1024.41's committed file name.
func committed(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(dir41), name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// committedTree is 1024.41's committed tree.json, decoded.
func committedTree(t *testing.T) *parse.Tree {
	t.Helper()
	var tr parse.Tree
	if err := json.Unmarshal(committed(t, "tree.json"), &tr); err != nil {
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

// fixture makes a corpus root holding 1024.41's committed meta.yaml and
// tree.json, each as edit leaves it, if edit is given.
func fixture(t *testing.T, edit func(tr *parse.Tree, m *meta.Meta)) string {
	t.Helper()
	tr := committedTree(t)
	m, err := meta.Parse(committed(t, "meta.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(tr, &m)
	}
	tb, err := anchors.EncodeTree(tr)
	if err != nil {
		t.Fatal(err)
	}
	mb, err := meta.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	put(t, root, treePath, tb)
	put(t, root, metaPath, mb)
	return root
}

// node is tr's node with anchor ID id.
func node(t *testing.T, tr *parse.Tree, id string) *parse.Node {
	t.Helper()
	for _, n := range anchors.NewDocument(tr).Nodes() {
		if anchors.ID(tr.Section, n.Path) == id {
			return n
		}
	}
	t.Fatalf("no node %s", id)
	return nil
}

// setText sets the own text of tr's node id and makes the hashes agree with
// it: the node's sha256 and meta.yaml's sha256_text.
func setText(t *testing.T, tr *parse.Tree, m *meta.Meta, id, text string) {
	t.Helper()
	n := node(t, tr, id)
	n.Text = text
	var err error
	if n.SHA256, err = anchors.Hash(text); err != nil {
		t.Fatal(err)
	}
	if m.SHA256Text, err = anchors.SHA256Text(tr); err != nil {
		t.Fatal(err)
	}
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

// TestTextRefuses: each refusal exits with its code (F4, F5, F6), prints
// nothing to stdout, and says why on stderr.
func TestTextRefuses(t *testing.T) {
	sum := func(s string) string { h, _ := anchors.Hash(s); return h }
	for _, tc := range []struct {
		name  string
		args  []string
		root  func(t *testing.T) string // a corpus root; fixture(t, nil) if nil
		code  int
		say   []string // what stderr says
		check bool     // the refusal is a stored file's: stderr names regcorpus parse or fetch
	}{
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
		// Not the corpus root (exit 2).
		{name: "not the corpus root", args: []string{"1024-41-a"}, root: func(t *testing.T) string { return t.TempDir() },
			code: 2, say: []string{"is not a corpus root (no sources/ecfr/title-12/ directory); run text from the corpus root"}},
		// No such anchor in a known source (exit 1), naming it and its source.
		{name: "no such anchor", args: []string{"1024-41-z"}, code: 1, say: []string{`no anchor "1024-41-z" in 1024.41`}},
		{name: "another case", args: []string{"1024-41-B"}, code: 1, say: []string{`no anchor "1024-41-B" in 1024.41`}},
		{name: "the section itself", args: []string{"1024-41"}, code: 1, say: []string{`no anchor "1024-41" in 1024.41`}},
		{name: "a trailing dash", args: []string{"1024-41-a-"}, code: 1, say: []string{`no anchor "1024-41-a-" in 1024.41`}},
		// Not built yet (exit 1), naming the plan step.
		{name: "a later step's source", args: []string{"1024-31-day"}, code: 1, say: []string{"anchors for 1024.31 are plan step 9's"}},
		{name: "a comment anchor", args: []string{"1024-41-b-2-i-interp-1"}, code: 1, say: []string{"Supplement I comment anchors are plan step 8's"}},
		// A stored file refuses (exit 1).
		{name: "no tree.json", args: []string{"1024-41-a"}, code: 1, check: true,
			root: func(t *testing.T) string {
				root := fixture(t, nil)
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(treePath))); err != nil {
					t.Fatal(err)
				}
				return root
			}, say: []string{treePath + " does not exist", "run regcorpus parse 1024.41"}},
		{name: "no meta.yaml", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string {
				root := fixture(t, nil)
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(metaPath))); err != nil {
					t.Fatal(err)
				}
				return root
			}, say: []string{metaPath + " does not exist", "run regcorpus fetch first"}},
		{name: "a node whose sha256 isn't its text's", args: []string{"1024-41-b-2-i-B"}, code: 1, check: true,
			root: func(t *testing.T) string {
				return fixture(t, func(tr *parse.Tree, _ *meta.Meta) { node(t, tr, "1024-41-b-2-i-B").SHA256 = sum("other") })
			}, say: []string{"1024-41-b-2-i-B", "sha256"}},
		{name: "a sha256_text the tree doesn't give", args: []string{"1024-41-a"}, code: 1, check: true,
			root: func(t *testing.T) string {
				return fixture(t, func(_ *parse.Tree, m *meta.Meta) { m.SHA256Text = meta.SHA256(sum("other")) })
			}, say: []string{"sha256_text"}},
		{name: "a text the hashes agree with but meta.yaml's sha256_text doesn't", args: []string{"1024-41-a"}, code: 1, check: true,
			root: func(t *testing.T) string {
				return fixture(t, func(tr *parse.Tree, m *meta.Meta) {
					keep := m.SHA256Text
					setText(t, tr, m, "1024-41-b-2-i-B", "(B) Another text.")
					m.SHA256Text = keep
				})
			}, say: []string{"sha256_text"}},
		{name: "no sha256_text", args: []string{"1024-41-a"}, code: 1, check: true,
			root: func(t *testing.T) string {
				return fixture(t, func(_ *parse.Tree, m *meta.Meta) { m.SHA256Text = "" })
			}, say: []string{"sha256_text", "run regcorpus parse 1024.41"}},
		{name: "a meta.yaml naming another source", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string {
				return fixture(t, func(_ *parse.Tree, m *meta.Meta) { m.CFR = "12 CFR 1024.31" })
			}, say: []string{"another source"}},
		{name: "a tree.json of another section", args: []string{"1024-41-a"}, code: 1, check: true,
			root: func(t *testing.T) string {
				return fixture(t, func(tr *parse.Tree, _ *meta.Meta) { tr.Section = "1024.31" })
			}, say: []string{`"1024.31"`, "1024.41"}},
		{name: "a case-variant TREE.JSON", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string {
				root := fixture(t, nil)
				d := filepath.Join(root, filepath.FromSlash(dir41))
				if err := os.Rename(filepath.Join(d, "tree.json"), filepath.Join(d, "TREE.JSON")); err != nil {
					t.Fatal(err)
				}
				return root
			}, say: []string{dir41 + "/TREE.JSON", "another case"}},
		{name: "a tree.json that is a directory", args: []string{"1024-41-a"}, code: 1,
			root: func(t *testing.T) string {
				root := fixture(t, nil)
				p := filepath.Join(root, filepath.FromSlash(treePath))
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(p, 0o755); err != nil {
					t.Fatal(err)
				}
				return root
			}, say: []string{treePath, "not a regular file"}},
		{name: "a tree.json with an unknown field", args: []string{"1024-41-a"}, code: 1, check: true,
			root: func(t *testing.T) string {
				root := fixture(t, nil)
				b := bytes.Replace(committed(t, "tree.json"), []byte(`"section"`), []byte(`"extra": 1, "section"`), 1)
				put(t, root, treePath, b)
				return root
			}, say: []string{treePath, `unknown field "extra"`}},
		{name: "a tree.json with a second value", args: []string{"1024-41-a"}, code: 1, check: true,
			root: func(t *testing.T) string {
				root := fixture(t, nil)
				put(t, root, treePath, append(committed(t, "tree.json"), "{}\n"...))
				return root
			}, say: []string{treePath}},
		{name: "a tree.json that isn't JSON", args: []string{"1024-41-a"}, code: 1, check: true,
			root: func(t *testing.T) string {
				root := fixture(t, nil)
				put(t, root, treePath, []byte("not json\n"))
				return root
			}, say: []string{treePath}},
		{name: "a null node", args: []string{"1024-41-a"}, code: 1, check: true,
			root: func(t *testing.T) string {
				root := fixture(t, nil)
				put(t, root, treePath, bytes.Replace(committed(t, "tree.json"), []byte(`"children": [`), []byte(`"children": [null, `), 1))
				return root
			}, say: []string{treePath}},
		{name: "a node with no label path", args: []string{"1024-41-a"}, code: 1, check: true,
			root: func(t *testing.T) string {
				return fixture(t, func(tr *parse.Tree, _ *meta.Meta) { node(t, tr, "1024-41-k").Path = nil })
			}, say: []string{treePath, "no label path"}},
		{name: "two nodes with one anchor ID", args: []string{"1024-41-a"}, code: 1, check: true,
			root: func(t *testing.T) string {
				return fixture(t, func(tr *parse.Tree, _ *meta.Meta) { node(t, tr, "1024-41-k").Path = []string{"a"} })
			}, say: []string{treePath, "1024-41-a", "twice"}},
		// A file that can't be read (exit 2).
		{name: "an unreadable tree.json", args: []string{"1024-41-a"}, code: 2,
			root: func(t *testing.T) string {
				if os.Geteuid() == 0 {
					t.Skip("root reads a file with no permissions")
				}
				root := fixture(t, nil)
				if err := os.Chmod(filepath.Join(root, filepath.FromSlash(treePath)), 0); err != nil {
					t.Fatal(err)
				}
				return root
			}, say: []string{"tree.json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var root string
			if tc.root != nil {
				root = tc.root(t)
			} else {
				root = fixture(t, nil)
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
			if tc.check && !strings.Contains(stderr, "regcorpus parse 1024.41") {
				t.Errorf("stderr %q doesn't say to run regcorpus parse 1024.41", stderr)
			}
		})
	}
}

// TestTextPrintsTheAnchor: on a fixture of the committed files, text prints
// the anchor ID, its own text, an empty line, its sha256, its children's
// IDs and its source's meta.yaml path and lines, as stored; each line in
// one Write. --json prints one line in one Write, a JSON object with the
// keys anchor, text, sha256, children, meta_path and meta, in that order.
func TestTextPrintsTheAnchor(t *testing.T) {
	root := fixture(t, nil)
	tr := committedTree(t)
	m := committed(t, "meta.yaml")
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
	mj, err := meta.JSON(mustParse(t, m))
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

func mustParse(t *testing.T, b []byte) meta.Meta {
	t.Helper()
	m, err := meta.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestTextEscapesWhatItPrints: an own text holding an LF, ESC, U+2028, DEL
// and a C1 control, with hashes that agree with it, prints on its one line
// with each written escaped by the sink; with --json, as one line of JSON
// that decodes to the stored text exactly.
func TestTextEscapesWhatItPrints(t *testing.T) {
	text := "(B) A\nline\x1b[2K and\x7fthe\u0085rest."
	root := fixture(t, func(tr *parse.Tree, m *meta.Meta) { setText(t, tr, m, "1024-41-b-2-i-B", text) })

	code, stdout, stderr, _ := run(corpus(root), "1024-41-b-2-i-B")
	lines := strings.Split(stdout, "\n")
	if want := `(B) A\x0aline\x1b[2K and\x7fthe\u0085rest.`; code != 0 || stderr != "" || len(lines) < 2 || lines[1] != want {
		t.Fatalf("exit %d, stderr %q, stdout:\n%s\nwant line 2 %q", code, stderr, stdout, want)
	}

	code, stdout, stderr, calls := run(corpus(root), "--json", "1024-41-b-2-i-B")
	if code != 0 || stderr != "" || len(calls) != 1 || strings.Count(stdout, "\n") != 1 {
		t.Fatalf("--json: exit %d, stderr %q, Writes %q; want one line", code, stderr, calls)
	}
	var got struct{ Text string }
	if err := json.Unmarshal([]byte(stdout), &got); err != nil || got.Text != text {
		t.Errorf("--json: %s decodes to %q, %v; want the stored text %q", stdout, got.Text, err, text)
	}
	for _, raw := range []string{"\x1b", " ", "\x7f", "\u0085"} {
		if strings.Contains(stdout, raw) {
			t.Errorf("--json holds %q unescaped: %s", raw, stdout)
		}
	}
}

// TestTextHelp: -h prints the usage on stdout and exits 0; it states the
// argument, the output and the exit codes.
func TestTextHelp(t *testing.T) {
	for _, h := range []string{"-h", "-help", "--help"} {
		code, stdout, stderr, _ := run(corpus(t.TempDir()), h)
		if code != 0 || stderr != "" || !strings.HasPrefix(stdout, "usage: regcorpus text [--json] <anchor>\n") {
			t.Fatalf("%s: exit %d, stderr %q, stdout %q", h, code, stderr, stdout)
		}
		flat := strings.Join(strings.Fields(stdout), " ")
		for _, want := range []string{"1024-41-b-2-i-B", "Exit 0", "Exit 1", "Exit 2", "--json", "children", "sha256", "meta.yaml"} {
			if !strings.Contains(flat, want) {
				t.Errorf("%s: usage lacks %q", h, want)
			}
		}
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
