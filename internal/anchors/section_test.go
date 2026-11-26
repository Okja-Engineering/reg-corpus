// What fetch --verify reads from a source's derived files (plan step 6, ADR
// 0014): the section's paragraphs and the one join of a heading and
// paragraphs that sha256_text hashes, on eCFR's side and govinfo's.
package anchors_test

import (
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
)

func TestSectionTextIsTheHeadingAndTheParagraphsJoined(t *testing.T) {
	if got, want := anchors.JoinSection("§ 0.1 Head.", []string{"(a) One—", "(b) Two."}), "§ 0.1 Head. (a) One— (b) Two."; got != want {
		t.Errorf("JoinSection = %q, want %q", got, want)
	}
	if got := anchors.JoinSection("H", nil); got != "H " {
		t.Errorf("JoinSection with no paragraph = %q, want %q", got, "H ")
	}
	tree := parseRepo(t, raw41)
	paras := anchors.NewDocument(tree).Paragraphs()
	if len(paras) != 75 {
		t.Fatalf("%d paragraphs, want 75, one per <P>", len(paras))
	}
	if want := "(b) Receipt of a loss mitigation application—(1) Complete loss mitigation application. A complete"; !strings.HasPrefix(paras[1], want) {
		t.Errorf("paragraph 2 = %.100q, want it to start %q (nothing between own texts in one <P>)", paras[1], want)
	}
	if got, want := anchors.SectionText(tree), anchors.JoinSection(tree.Heading, paras); got != want {
		t.Errorf("SectionText is not JoinSection(Heading, Paragraphs)")
	}
	if got, err := anchors.Hash(anchors.JoinSection(tree.Heading, paras)); err != nil || got != oracleText {
		t.Errorf("Hash(JoinSection(...)) = %s, %v; want the oracle's %s", got, err, oracleText)
	}
}
