// Tests for `regcorpus text [--json] <anchor>` (R8; plan step 7, ADR 0015)
// on the committed corpus, read in place and never written: the handler
// passes the source table to internal/lookup, every one of 1024.41's
// anchors prints exactly its own text, hashing to its printed sha256, with
// nothing more, and the printed texts, joined as decision 13 joins them,
// give sha256_text. internal/lookup's tests cover the refusals and the sink.
package ecfr

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/lookup"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// repoRoot is the corpus root, from this package's directory.
const repoRoot = "../.."

func textAt(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errw strings.Builder
	code = textMain(repoRoot, args, &out, &errw)
	return code, out.String(), errw.String()
}

// tree41 is 1024.41's committed tree.json, decoded, and its nodes in
// document order.
func tree41(t *testing.T) (*parse.Tree, []*parse.Node) {
	t.Helper()
	var tr parse.Tree
	if err := json.Unmarshal(repoFile(t, "sources/ecfr/title-12/1024.41/tree.json"), &tr); err != nil {
		t.Fatal(err)
	}
	return &tr, anchors.NewDocument(&tr).Nodes()
}

// TestTextReadsTheSourceTable: text's corpus is the source table, each
// source by its name and stored form, its anchors pending until its parser
// goldens are in (the table's Goldens), under sources/ecfr/title-12.
func TestTextReadsTheSourceTable(t *testing.T) {
	c := corpus("r")
	if c.Root != "r" || c.Dir != sourcesDir || len(c.Sources) != len(sources) {
		t.Fatalf("corpus: %+v", c)
	}
	for i, s := range sources {
		want := lookup.Source{Name: s.Name, Stored: s.stored(), Pending: s.Goldens}
		if !reflect.DeepEqual(c.Sources[i], want) {
			t.Errorf("source %d: %+v; want %+v", i, c.Sources[i], want)
		}
	}
}

// TestTextPrintsEveryAnchor: for each of 1024.41's 91 nodes, text exits 0
// and prints its ID, exactly its own text, an empty line, a sha256 that is
// anchors.Hash of the printed text, its children's IDs in document order
// (or none), and meta.yaml's path and lines as stored: one text line, its
// own, and nothing else. --json gives the same values.
func TestTextPrintsEveryAnchor(t *testing.T) {
	tr, nodes := tree41(t)
	if len(nodes) != 91 {
		t.Fatalf("1024.41 has %d nodes; want 91", len(nodes))
	}
	metaYAML := string(repoFile(t, "sources/ecfr/title-12/1024.41/meta.yaml"))
	for _, n := range nodes {
		id := anchors.ID(tr.Section, n.Path)
		var children []string
		for _, c := range n.Children {
			children = append(children, anchors.ID(tr.Section, c.Path))
		}
		list := "none"
		if len(children) > 0 {
			list = strings.Join(children, " ")
		}
		code, stdout, stderr := textAt(t, id)
		if code != 0 || stderr != "" {
			t.Fatalf("%s: exit %d, stderr %q", id, code, stderr)
		}
		lines := strings.SplitN(stdout, "\n", 7)
		if len(lines) != 7 {
			t.Fatalf("%s: %q; want 6 lines and meta.yaml's", id, stdout)
		}
		sum, err := anchors.Hash(lines[1])
		switch {
		case err != nil:
			t.Errorf("%s: %v", id, err)
		case lines[0] != id || lines[1] != n.Text || lines[2] != "":
			t.Errorf("%s: printed %q, %q, %q; want the ID, its own text, an empty line", id, lines[0], lines[1], lines[2])
		case lines[3] != "sha256: "+sum || sum != n.SHA256:
			t.Errorf("%s: %q; the printed text hashes to %s", id, lines[3], sum)
		case lines[4] != "children: "+list:
			t.Errorf("%s: %q; want children: %s", id, lines[4], list)
		case lines[5] != "meta.yaml: sources/ecfr/title-12/1024.41/meta.yaml" || lines[6] != metaYAML:
			t.Errorf("%s: %q then\n%s\nwant meta.yaml's path and its lines as stored", id, lines[5], lines[6])
		}

		code, stdout, stderr = textAt(t, "--json", id)
		var got struct {
			Anchor, Text, SHA256 string
			Children             []string
			MetaPath             string `json:"meta_path"`
			Meta                 map[string]any
		}
		if err := json.Unmarshal([]byte(stdout), &got); code != 0 || stderr != "" || err != nil || strings.Count(stdout, "\n") != 1 {
			t.Fatalf("%s --json: exit %d, stderr %q, %v: %q", id, code, stderr, err, stdout)
		}
		if got.Anchor != id || got.Text != n.Text || got.SHA256 != n.SHA256 || strings.Join(got.Children, " ") != strings.Join(children, " ") ||
			got.Children == nil || got.MetaPath != "sources/ecfr/title-12/1024.41/meta.yaml" || len(got.Meta) != len(meta.Fields()) {
			t.Errorf("%s --json: %+v", id, got)
		}
	}
}

// TestTextPrintsNothingMoreThanAsked: a lead-in's output holds its own text
// and its children's IDs, never a child's text; 1024-41-b-2's lacks (i)'s.
func TestTextPrintsNothingMoreThanAsked(t *testing.T) {
	code, stdout, _ := textAt(t, "1024-41-b-2")
	if code != 0 || strings.Contains(stdout, "(i) Requirements.") || !strings.Contains(stdout, "\nchildren: 1024-41-b-2-i 1024-41-b-2-ii\n") {
		t.Errorf("exit %d, stdout:\n%s", code, stdout)
	}
	tr, nodes := tree41(t)
	for _, n := range nodes {
		_, stdout, _ := textAt(t, anchors.ID(tr.Section, n.Path))
		for _, c := range n.Children {
			if strings.Contains(stdout, c.Text) && !strings.Contains(n.Text, c.Text) {
				t.Errorf("%s's output holds its child's text %q", anchors.ID(tr.Section, n.Path), c.Text)
			}
		}
	}
}

// TestTextRoundTripsSHA256Text: the 91 printed own texts, joined in
// document order under the section heading as decision 13 joins them
// (anchors.SHA256Text), hash to meta.yaml's sha256_text: nothing is missing
// or doubled.
func TestTextRoundTripsSHA256Text(t *testing.T) {
	tr, nodes := tree41(t)
	for _, n := range nodes {
		id := anchors.ID(tr.Section, n.Path)
		_, stdout, _ := textAt(t, id)
		lines := strings.SplitN(stdout, "\n", 3)
		if len(lines) < 3 {
			t.Fatalf("%s: %q; want the ID and its text line", id, stdout)
		}
		n.Text = lines[1]
	}
	m, err := meta.Parse(repoFile(t, "sources/ecfr/title-12/1024.41/meta.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if sum, err := anchors.SHA256Text(tr); err != nil || sum != m.SHA256Text {
		t.Errorf("the printed texts give sha256_text %s, %v; meta.yaml says %s", sum, err, m.SHA256Text)
	}
}

// TestTextRefusesWhatIsNotBuilt: through the source table, 1024.30's and
// 1024.31's anchors are plan step 9's and a comment anchor plan step 8's
// (exit 1); a string that names no source exits 2.
func TestTextRefusesWhatIsNotBuilt(t *testing.T) {
	for _, tc := range []struct {
		anchor string
		code   int
		say    string
	}{
		{"1024-31-day", 1, "anchors for 1024.31 are plan step 9's"},
		{"1024-30-a", 1, "anchors for 1024.30 are plan step 9's"},
		{"1024-41-b-2-i-interp-1", 1, "Supplement I comment anchors are plan step 8's"},
		{"1024.41(b)(2)(i)(B)", 2, "names no source"},
	} {
		code, stdout, stderr := textAt(t, tc.anchor)
		if code != tc.code || stdout != "" || !strings.Contains(stderr, tc.say) {
			t.Errorf("%s: exit %d, stdout %q, stderr %q; want %d, %q", tc.anchor, code, stdout, stderr, tc.code, tc.say)
		}
	}
}
