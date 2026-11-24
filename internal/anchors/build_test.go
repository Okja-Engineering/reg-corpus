// Build: raw XML to tree.json (each node with the sha256 of its normalized
// own text), text.md and sha256_text (R1, R1a; ADR 0013). The committed
// files of 1024.41 are what Build makes from its stored raw XML, and the
// hashes agree with an independent oracle's.
package anchors_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// The oracle's values: a Python one-off (stdlib unicodedata NFC, Unicode
// 17.0.0, the raw XML read with regular expressions, not the Go parser)
// computed them from 1024.41's stored raw XML; it is in the step 5 report,
// not in the repo.
const (
	oracleText   = "ecda50e7ce958e2fd19a8f1843b3c60bde0dcf533cc2c313cb9c1e1960760fe1"
	oracleB      = "3b97eaec814a015db9f4eca31e670e09ec059699c0ac04127d4b63fd849c7c47"
	oracleC2v    = "9e72894b51c5202d36ec00ee5d95c273cb1f74235f0d10b4c387328b9ea935a5"
	oracleC2vA1  = "bd9d12027d994a673ebc9d359feb7bde9f044c7024ed8d0040cf96a03d028e5d"
	nodes1024_41 = 91
)

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func build41(t *testing.T) *anchors.Derived {
	t.Helper()
	d, err := anchors.Build(readRepo(t, raw41))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestBuildHashesEveryNode: each of 1024.41's 91 nodes carries the sha256
// of its normalized own text, separator included (ADR 0013, F1: H1).
func TestBuildHashesEveryNode(t *testing.T) {
	d := build41(t)
	nodes := walk(d.Tree.Children)
	if len(nodes) != nodes1024_41 {
		t.Fatalf("%d nodes, want %d", len(nodes), nodes1024_41)
	}
	for _, n := range nodes {
		if !hex64.MatchString(n.SHA256) || n.SHA256 != sum(anchors.Normalize(n.Text)) {
			t.Errorf("node %v: sha256 %q, want sha256 of %q", n.Path, n.SHA256, anchors.Normalize(n.Text))
		}
	}
}

// TestBuildAgreesWithTheOracle: sha256_text and three node hashes equal an
// independent computation's (S1: the HEAD, one space, then every <P> joined
// by J2, normalized, the CITA left out).
func TestBuildAgreesWithTheOracle(t *testing.T) {
	d := build41(t)
	if d.SHA256Text != oracleText {
		t.Errorf("sha256_text = %s, want the oracle's %s", d.SHA256Text, oracleText)
	}
	n := byPath(t, d.Tree)
	for path, want := range map[string]string{"b": oracleB, "c-2-v": oracleC2v, "c-2-v-A-1": oracleC2vA1} {
		if got := n[path].SHA256; got != want {
			t.Errorf("node %s sha256 = %s, want the oracle's %s", path, got, want)
		}
	}
}

// TestCommittedFilesAreTheBuild: 1024.41's committed tree.json and text.md
// are byte for byte what Build makes from the stored raw XML, and its
// meta.yaml's sha256_text is Build's.
func TestCommittedFilesAreTheBuild(t *testing.T) {
	d := build41(t)
	if got := readRepo(t, tree41); !bytes.Equal(got, d.TreeJSON) {
		t.Errorf("%s is not what Build makes from %s; run regcorpus parse 1024.41", tree41, raw41)
	}
	if got := readRepo(t, text41); !bytes.Equal(got, d.TextMD) {
		t.Errorf("%s is not what Build makes from %s; run regcorpus parse 1024.41", text41, raw41)
	}
	m, err := meta.Parse(readRepo(t, meta41))
	if err != nil {
		t.Fatal(err)
	}
	if m.SHA256Text != d.SHA256Text {
		t.Errorf("%s sha256_text = %q, want %q", meta41, m.SHA256Text, d.SHA256Text)
	}
}

// TestTextMDRegeneratesFromCommittedTree: text.md is generated from
// tree.json (R1a): rendering the committed tree gives the committed text.md.
func TestTextMDRegeneratesFromCommittedTree(t *testing.T) {
	var tree parse.Tree
	if err := json.Unmarshal(readRepo(t, tree41), &tree); err != nil {
		t.Fatal(err)
	}
	if got, want := anchors.TextMD(&tree), readRepo(t, text41); !bytes.Equal(got, want) {
		t.Errorf("TextMD(committed tree.json) =\n%s\nwant the committed text.md\n%s", got, want)
	}
	if got := anchors.SHA256Text(&tree); got != oracleText {
		t.Errorf("SHA256Text(committed tree.json) = %s, want %s", got, oracleText)
	}
}

// TestTreeJSONRoundTrip: decoding the committed tree.json and encoding it
// again gives the same bytes.
func TestTreeJSONRoundTrip(t *testing.T) {
	b := readRepo(t, tree41)
	var tree parse.Tree
	if err := json.Unmarshal(b, &tree); err != nil {
		t.Fatal(err)
	}
	got, err := anchors.EncodeTree(&tree)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, b) {
		t.Errorf("tree.json round trip differs")
	}
}

// TestTreeJSONForm: two-space indent, kinds by name, each node's sha256
// after its text, no HTML escaping, a trailing LF.
func TestTreeJSONForm(t *testing.T) {
	d, err := anchors.Build(section("[Cite]\n", "(a) <I>Head</I>—(1) A &amp; B &lt;C&gt;."))
	if err != nil {
		t.Fatal(err)
	}
	a, a1 := sum("(a) Head-"), sum("(1) A & B <C>.")
	want := `{
  "section": "0.1",
  "heading": "§ 0.1 Head.",
  "citation": "[Cite]\n",
  "children": [
    {
      "path": [
        "a"
      ],
      "kind": "letter",
      "para": 1,
      "heading": "Head",
      "text": "(a) Head—",
      "sha256": "` + a + `",
      "children": [
        {
          "path": [
            "a",
            "1"
          ],
          "kind": "number",
          "para": 1,
          "text": "(1) A & B <C>.",
          "sha256": "` + a1 + `"
        }
      ]
    }
  ]
}
`
	if string(d.TreeJSON) != want {
		t.Errorf("tree.json =\n%s\nwant\n%s", d.TreeJSON, want)
	}
}

// TestTextMDForm (F5: T1): "# " and the section HEAD, a blank line, then
// each <P>'s text (its own texts joined by J2, verbatim), a blank line
// between paragraphs, and last the CITA; UTF-8, LF, one newline at the end.
// No anchor IDs, markup or normalization.
func TestTextMDForm(t *testing.T) {
	paras := []string{"(a) <I>Head</I>—(1) <I>Sub.</I> “Text”  (i) here.", "(2) <I>Two</I>—(i) Text.", "(b) Last."}
	body := "# § 0.1 Head.\n\n(a) Head—(1) Sub. “Text”  (i) here.\n\n(2) Two—(i) Text.\n\n(b) Last.\n"
	for _, tc := range []struct{ name, cita, want string }{
		{"a CITA ending in LF", "[Cite]\n", body + "\n[Cite]\n"},
		{"a CITA with no LF", "[Cite]", body + "\n[Cite]\n"},
		{"no CITA", "", body},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := anchors.Build(section(tc.cita, paras...))
			if err != nil {
				t.Fatal(err)
			}
			if string(d.TextMD) != tc.want {
				t.Errorf("text.md =\n%q\nwant\n%q", d.TextMD, tc.want)
			}
		})
	}
}

// TestSHA256Text (F3: S1): sha256 of the normalized HEAD, one space, then
// every anchor joined by J2; the CITA is not in it. Normalizing after the
// join leaves one space where a <P> ends in one (N6).
func TestSHA256Text(t *testing.T) {
	paras := []string{"(a) <I>Head</I>—(1) Text. ", "(b) “More”\u00A0text."}
	const want = "§ 0.1 Head. (a) Head-(1) Text. (b) \"More\" text."
	for _, cita := range []string{"", "[Cite]\n", "[Other]\n"} {
		d, err := anchors.Build(section(cita, paras...))
		if err != nil {
			t.Fatal(err)
		}
		if d.SHA256Text != meta.SHA256(sum(want)) {
			t.Errorf("CITA %q: sha256_text = %s, want sha256(%q) = %s", cita, d.SHA256Text, want, sum(want))
		}
	}
}

// TestBuildIsNotFor1024_41Only: 1024.30 builds too (9 nodes, 8 operative:
// all but (c), a heading then its children). Its files are plan step 9's.
func TestBuildIsNotFor1024_41Only(t *testing.T) {
	d, err := anchors.Build(readRepo(t, raw30))
	if err != nil {
		t.Fatal(err)
	}
	nodes := walk(d.Tree.Children)
	op := 0
	for _, n := range nodes {
		if ok, err := anchors.Operative(n); err != nil {
			t.Fatal(err)
		} else if ok {
			op++
		} else if got := anchors.ID(d.Tree.Section, n.Path); got != "1024-30-c" {
			t.Errorf("%s is not operative", got)
		}
		if !hex64.MatchString(n.SHA256) {
			t.Errorf("node %v has no sha256", n.Path)
		}
	}
	if len(nodes) != 9 || op != 8 {
		t.Errorf("%d nodes, %d operative; want 9 and 8", len(nodes), op)
	}
}

// TestBuildFailsWhereParseFails: Build returns the parser's error, with its
// location, and nothing else.
func TestBuildFailsWhereParseFails(t *testing.T) {
	d, err := anchors.Build(readRepo(t, raw31))
	var pe *parse.Error
	if !errors.As(err, &pe) || pe.Line != 4 || pe.Column != 4 || pe.Reason != parse.Unlabeled || d != nil {
		t.Errorf("Build(1024.31) = %v, %v; want nil and the parse error at line 4, column 4, unlabeled paragraph", d, err)
	}
}
