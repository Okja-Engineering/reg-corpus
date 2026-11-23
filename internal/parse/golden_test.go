// Goldens for 12 CFR 1024.41 (R1a; plan step 4), built from the stored raw
// XML only. Each golden asserts a node's label path, level kind, heading and
// own text exactly as the XML's character data holds it. The tree's full
// shape is asserted once, so a misplaced node anywhere goes red. The expected
// values were written by hand from the XML, not from the parser's output.
package parse_test

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/parse"
)

const rawPath = "../../sources/ecfr/title-12/1024.41/raw/2025-07-15.xml"

func readRaw(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func parseRaw(t *testing.T) *parse.Tree {
	t.Helper()
	tree, err := parse.Parse(readRaw(t))
	if err != nil {
		t.Fatalf("Parse(%s): %v", rawPath, err)
	}
	return tree
}

// pathString renders a label path as the CFR writes it, e.g. "(b)(2)(i)(B)".
func pathString(path []string) string {
	var b strings.Builder
	for _, p := range path {
		b.WriteString("(" + p + ")")
	}
	return b.String()
}

// walk lists every node in document order.
func walk(nodes []*parse.Node) []*parse.Node {
	var out []*parse.Node
	for _, n := range nodes {
		out = append(out, n)
		out = append(out, walk(n.Children)...)
	}
	return out
}

func byPath(t *testing.T, tree *parse.Tree) map[string]*parse.Node {
	t.Helper()
	m := map[string]*parse.Node{}
	for _, n := range walk(tree.Children) {
		m[pathString(n.Path)] = n
	}
	return m
}

type golden struct {
	path    string
	kind    parse.Kind
	heading string
	text    string
}

var goldens = []golden{
	// Letter (i) after (h): (h) has children (1)-(4), and (i) follows them.
	{"(i)", parse.Letter, "Duplicative requests.",
		"(i) Duplicative requests. A servicer must comply with the requirements of this section for a borrower's loss mitigation application, unless the servicer has previously complied with the requirements of this section for a complete loss mitigation application submitted by the borrower and the borrower has been delinquent at all times since submitting the prior complete application."},
	// Roman (i).
	{"(c)(1)(i)", parse.Roman, "",
		"(i) Evaluate the borrower for all loss mitigation options available to the borrower; and"},
	// The chain "(b) heading—(1) heading." in one <P>: the dash stays with the parent.
	{"(b)", parse.Letter, "Receipt of a loss mitigation application",
		"(b) Receipt of a loss mitigation application—"},
	{"(b)(1)", parse.Number, "Complete loss mitigation application.",
		"(1) Complete loss mitigation application. A complete loss mitigation application means an application in connection with which a servicer has received all the information that the servicer requires from a borrower in evaluating applications for the loss mitigation options available to the borrower. A servicer shall exercise reasonable diligence in obtaining documents and information to complete a loss mitigation application."},
	// The chain "(2) heading—(i) heading." in one <P>.
	{"(b)(2)", parse.Number, "Review of loss mitigation application submission",
		"(2) Review of loss mitigation application submission—"},
	{"(b)(2)(i)", parse.Roman, "Requirements.",
		"(i) Requirements. If a servicer receives a loss mitigation application 45 days or more before a foreclosure sale, a servicer shall:"},
	// The heading-dash chained label (b)(2)(i)(B).
	{"(b)(2)(i)(B)", parse.Capital, "",
		"(B) Notify the borrower in writing within 5 days (excluding legal public holidays, Saturdays, and Sundays) after receiving the loss mitigation application that the servicer acknowledges receipt of the loss mitigation application and that the servicer has determined that the loss mitigation application is either complete or incomplete. If a loss mitigation application is incomplete, the notice shall state the additional documents and information the borrower must submit to make the loss mitigation application complete and the applicable date pursuant to paragraph (b)(2)(ii) of this section. The notice to the borrower shall include a statement that the borrower should consider contacting servicers of any other mortgage loans secured by the same property to discuss available loss mitigation options."},
	// (c)(2)(iv), whose text holds the cross-reference "paragraph (b)(2)(i)(B) of this section" twice.
	{"(c)(2)(iv)", parse.Roman, "Facially complete application.",
		"(iv) Facially complete application. A loss mitigation application shall be considered facially complete when a borrower submits all the missing documents and information as stated in the notice required under paragraph (b)(2)(i)(B) of this section, when no additional information is requested in such notice, or once the servicer is required to provide the borrower a written notice pursuant to paragraph (c)(3)(i) of this section. If the servicer later discovers that additional information or corrections to a previously submitted document are required to complete the application, the servicer must promptly request the missing information or corrected documents and treat the application as complete for the purposes of paragraphs (f)(2) and (g) of this section until the borrower is given a reasonable opportunity to complete the application. If the borrower completes the application within this period, the application shall be considered complete as of the date it first became facially complete, for the purposes of paragraphs (d), (e), (f)(2), (g), and (h) of this section, and as of the date the application was actually complete for the purposes of this paragraph (c). A servicer that complies with this paragraph (c)(2)(iv) will be deemed to have fulfilled its obligation to provide an accurate notice under paragraph (b)(2)(i)(B) of this section."},
	// (f)(1)(i).
	{"(f)(1)(i)", parse.Roman, "",
		"(i) A borrower's mortgage loan obligation is more than 120 days delinquent;"},
	// A label right after a heading that ends in a period and a space,
	// "(v) heading. (A)": the space stays with the parent.
	{"(c)(2)(v)", parse.Roman, "Certain COVID-19-related loss mitigation options.",
		"(v) Certain COVID-19-related loss mitigation options. "},
	{"(c)(2)(v)(A)", parse.Capital, "",
		"(A) Notwithstanding paragraph (c)(2)(i) of this section, a servicer may offer a borrower a loss mitigation option based upon evaluation of an incomplete application, provided that all of the following criteria are met:"},
	// The italic level: (c)(2)(v)(A)(1), "<P>(<I>1</I>)". Inline <I> in
	// running text, "(c)(2)(v)(A)(<I>1</I>)", is carried as its character data.
	{"(c)(2)(v)(A)(1)", parse.Italic, "",
		"(1) The loss mitigation option permits the borrower to delay paying covered amounts until the mortgage loan is refinanced, the mortgaged property is sold, the term of the mortgage loan ends, or, for a mortgage loan insured by the Federal Housing Administration, the mortgage insurance terminates. For purposes of this paragraph (c)(2)(v)(A)(1), “covered amounts” includes, without limitation, all principal and interest payments forborne under a payment forbearance program made available to borrowers experiencing a COVID-19-related hardship, including a payment forbearance program made pursuant to the Coronavirus Economic Stabilization Act, section 4022 (15 U.S.C. 9056); it also includes, without limitation, all other principal and interest payments that are due and unpaid by a borrower experiencing COVID-19-related hardship. For purposes of this paragraph (c)(2)(v)(A)(1), “the term of the mortgage loan” means the term of the mortgage loan according to the obligation between the parties in effect when the borrower is offered the loss mitigation option."},
	// The italic level: (c)(4)(ii)(B)(2).
	{"(c)(4)(ii)(B)(2)", parse.Italic, "",
		"(2) Of the specific documents or information that the servicer lacks;"},
	// "(ii) heading. (A)(<I>1</I>) text": a heading, then two labels with
	// nothing between them. (A) holds its label alone.
	{"(c)(4)(ii)", parse.Roman, "Effect in case of delay.",
		"(ii) Effect in case of delay. "},
	{"(c)(4)(ii)(A)", parse.Capital, "", "(A)"},
	{"(c)(4)(ii)(A)(1)", parse.Italic, "",
		"(1) Except as provided in paragraph (c)(4)(ii)(A)(2) of this section, a servicer must not deny a complete loss mitigation application solely because the servicer lacks required documents or information not in the borrower's control."},
	// A three-label chain: "(k) heading—(1) heading—(i) heading."
	{"(k)", parse.Letter, "Servicing transfers", "(k) Servicing transfers—"},
	{"(k)(1)", parse.Number, "In general", "(1) In general—"},
}

func TestGoldens(t *testing.T) {
	nodes := byPath(t, parseRaw(t))
	for _, g := range goldens {
		t.Run(g.path, func(t *testing.T) {
			n, ok := nodes[g.path]
			if !ok {
				t.Fatalf("no node %s", g.path)
			}
			if n.Kind != g.kind {
				t.Errorf("kind = %v, want %v", n.Kind, g.kind)
			}
			if n.Heading != g.heading {
				t.Errorf("heading = %q, want %q", n.Heading, g.heading)
			}
			if n.Text != g.text {
				t.Errorf("text =\n%q\nwant\n%q", n.Text, g.text)
			}
		})
	}
}

// TestCrossReferenceDoesNotSplit: the labels in a cross-reference sentence
// are running text, so the sentence stays whole in one node's own text and
// no node is made from it.
func TestCrossReferenceDoesNotSplit(t *testing.T) {
	nodes := byPath(t, parseRaw(t))
	n := nodes["(c)(2)(iv)"]
	if n == nil {
		t.Fatal("no node (c)(2)(iv)")
	}
	const sentence = "A servicer that complies with this paragraph (c)(2)(iv) will be deemed to have fulfilled its obligation to provide an accurate notice under paragraph (b)(2)(i)(B) of this section."
	if !strings.Contains(n.Text, sentence) {
		t.Errorf("(c)(2)(iv) text does not hold the whole sentence %q", sentence)
	}
	if len(n.Children) != 0 {
		t.Errorf("(c)(2)(iv) has %d children, want 0", len(n.Children))
	}
}

// wantPaths is the full tree for 1024.41, in document order, one line per <P>.
var wantPaths = []string{
	"(a)",
	"(b)", "(b)(1)",
	"(b)(2)", "(b)(2)(i)",
	"(b)(2)(i)(A)",
	"(b)(2)(i)(B)",
	"(b)(2)(ii)",
	"(b)(3)",
	"(c)", "(c)(1)",
	"(c)(1)(i)",
	"(c)(1)(ii)",
	"(c)(2)", "(c)(2)(i)",
	"(c)(2)(ii)",
	"(c)(2)(iii)",
	"(c)(2)(iv)",
	"(c)(2)(v)", "(c)(2)(v)(A)",
	"(c)(2)(v)(A)(1)",
	"(c)(2)(v)(A)(2)",
	"(c)(2)(v)(A)(3)",
	"(c)(2)(v)(B)",
	"(c)(3)", "(c)(3)(i)",
	"(c)(3)(i)(A)",
	"(c)(3)(i)(B)",
	"(c)(3)(i)(C)",
	"(c)(3)(i)(D)",
	"(c)(3)(i)(D)(1)",
	"(c)(3)(i)(D)(2)",
	"(c)(3)(i)(E)",
	"(c)(3)(i)(F)",
	"(c)(3)(ii)",
	"(c)(3)(ii)(A)",
	"(c)(3)(ii)(B)",
	"(c)(3)(ii)(C)",
	"(c)(4)", "(c)(4)(i)",
	"(c)(4)(ii)", "(c)(4)(ii)(A)", "(c)(4)(ii)(A)(1)",
	"(c)(4)(ii)(A)(2)",
	"(c)(4)(ii)(B)",
	"(c)(4)(ii)(B)(1)",
	"(c)(4)(ii)(B)(2)",
	"(c)(4)(ii)(B)(3)",
	"(c)(4)(ii)(B)(4)",
	"(c)(4)(ii)(C)",
	"(d)",
	"(e)", "(e)(1)",
	"(e)(2)", "(e)(2)(i)",
	"(e)(2)(ii)",
	"(e)(2)(iii)",
	"(f)", "(f)(1)",
	"(f)(1)(i)",
	"(f)(1)(ii)",
	"(f)(1)(iii)",
	"(f)(2)",
	"(f)(2)(i)",
	"(f)(2)(ii)",
	"(f)(2)(iii)",
	"(g)",
	"(g)(1)",
	"(g)(2)",
	"(g)(3)",
	"(h)", "(h)(1)",
	"(h)(2)",
	"(h)(3)",
	"(h)(4)",
	"(i)",
	"(j)",
	"(k)", "(k)(1)", "(k)(1)(i)",
	"(k)(1)(ii)",
	"(k)(2)", "(k)(2)(i)",
	"(k)(2)(ii)",
	"(k)(2)(ii)(A)",
	"(k)(2)(ii)(B)",
	"(k)(3)",
	"(k)(4)",
	"(k)(4)(i)",
	"(k)(4)(ii)",
	"(k)(5)",
}

// TestTreeShape asserts every node's path, in document order, and that each
// node sits under the node its path names, at the kind its depth names.
func TestTreeShape(t *testing.T) {
	tree := parseRaw(t)
	var got []string
	for _, n := range walk(tree.Children) {
		got = append(got, pathString(n.Path))
	}
	if !reflect.DeepEqual(got, wantPaths) {
		t.Fatalf("paths (%d) =\n%s\nwant (%d)\n%s", len(got), strings.Join(got, "\n"), len(wantPaths), strings.Join(wantPaths, "\n"))
	}
	kinds := []parse.Kind{parse.Letter, parse.Number, parse.Roman, parse.Capital, parse.Italic}
	var check func(parent []string, nodes []*parse.Node)
	check = func(parent []string, nodes []*parse.Node) {
		for _, n := range nodes {
			if len(n.Path) != len(parent)+1 || !slices.Equal(n.Path[:len(parent)], parent) {
				t.Errorf("node %s sits under %s", pathString(n.Path), pathString(parent))
			}
			if want := kinds[len(n.Path)-1]; n.Kind != want {
				t.Errorf("node %s kind = %v, want %v", pathString(n.Path), n.Kind, want)
			}
			check(n.Path, n.Children)
		}
	}
	check(nil, tree.Children)
}

var (
	paraRE = regexp.MustCompile(`(?s)<P>(.*?)</P>`)
	italRE = regexp.MustCompile(`</?I>`)
)

// TestOwnTextsPartitionEachParagraph: the own texts, joined in document
// order, are exactly the character data of the <P> elements, joined: no
// character is dropped, added or moved. The expected side is read with a
// regexp, not the parser.
func TestOwnTextsPartitionEachParagraph(t *testing.T) {
	raw := readRaw(t)
	var want strings.Builder
	for _, m := range paraRE.FindAllSubmatch(raw, -1) {
		want.Write(italRE.ReplaceAll(m[1], nil))
	}
	var got strings.Builder
	for _, n := range walk(parseRaw(t).Children) {
		got.WriteString(n.Text)
	}
	if got.String() != want.String() {
		t.Errorf("joined own texts differ from the joined <P> character data")
	}
	if strings.ContainsAny(got.String(), "<>") {
		t.Errorf("own text holds markup")
	}
}

// TestSectionHeadAndCitation: HEAD and CITA are not paragraphs; the tree
// keeps their character data verbatim on the section.
func TestSectionHeadAndCitation(t *testing.T) {
	tree := parseRaw(t)
	if tree.Section != "1024.41" {
		t.Errorf("section = %q", tree.Section)
	}
	if want := "§ 1024.41 Loss mitigation procedures."; tree.Heading != want {
		t.Errorf("heading = %q, want %q", tree.Heading, want)
	}
	if want := "[78 FR 10876, Feb. 14, 2013, as amended at 78 FR 60437, Oct. 1, 2013; 81 FR 72373, Oct. 19, 2016; 85 FR 39065, June 30, 2020; 86 FR 34899, June 30, 2021; 90 FR 20792, May 16, 2025]\n"; tree.Citation != want {
		t.Errorf("citation = %q, want %q", tree.Citation, want)
	}
}

// TestDeterministic: the same bytes give the same tree.
func TestDeterministic(t *testing.T) {
	raw := readRaw(t)
	var first []byte
	for i := 0; i < 3; i++ {
		tree, err := parse.Parse(bytes.Clone(raw))
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(tree)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = b
		} else if !bytes.Equal(b, first) {
			t.Fatalf("parse %d differs from parse 0", i)
		}
	}
}
