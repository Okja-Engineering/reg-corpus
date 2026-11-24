package anchors_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/parse"
)

// repo is the corpus root, from this package's directory.
const repo = "../.."

// The stored sources the tests read, by slash path from the corpus root.
const (
	dir41  = "sources/ecfr/title-12/1024.41"
	raw41  = dir41 + "/raw/2025-07-15.xml"
	dir30  = "sources/ecfr/title-12/1024.30"
	raw30  = dir30 + "/raw/2018-04-19.xml"
	dir31  = "sources/ecfr/title-12/1024.31"
	raw31  = dir31 + "/raw/2025-07-15.xml"
	tree41 = dir41 + "/tree.json"
	text41 = dir41 + "/text.md"
	meta41 = dir41 + "/meta.yaml"
)

func readRepo(t *testing.T, slash string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(slash)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func parseRepo(t *testing.T, slash string) *parse.Tree {
	t.Helper()
	tree, err := parse.Parse(readRepo(t, slash))
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// section is a section's XML holding one <P> per argument, with a CITA
// when cita is not empty.
func section(cita string, paras ...string) []byte {
	s := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DIV8 N=\"0.1\" TYPE=\"SECTION\">\n<HEAD>§ 0.1 Head.</HEAD>\n"
	for _, p := range paras {
		s += "<P>" + p + "</P>\n"
	}
	if cita != "" {
		s += "<CITA TYPE=\"N\">" + cita + "</CITA>\n"
	}
	return []byte(s + "</DIV8>\n")
}

func parseXML(t *testing.T, data []byte) *parse.Tree {
	t.Helper()
	tree, err := parse.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// walk lists the nodes in document order.
func walk(nodes []*parse.Node) []*parse.Node {
	var out []*parse.Node
	for _, n := range nodes {
		out = append(out, n)
		out = append(out, walk(n.Children)...)
	}
	return out
}

// byPath maps "b-2-i-B" to its node.
func byPath(t *testing.T, tree *parse.Tree) map[string]*parse.Node {
	t.Helper()
	m := map[string]*parse.Node{}
	for _, n := range walk(tree.Children) {
		m[strings.Join(n.Path, "-")] = n
	}
	return m
}

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
