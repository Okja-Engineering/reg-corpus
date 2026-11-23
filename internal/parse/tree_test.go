// The tree's JSON form. internal/parse owns it; plan step 5 adds each
// node's sha256 and writes it to tree.json.
package parse_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/parse"
)

func TestTreeJSON(t *testing.T) {
	data := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DIV8 N=\"0.1\" TYPE=\"SECTION\">\n<HEAD>§ 0.1 Head.</HEAD>\n" +
		"<P>(a) <I>Head</I>—(1) <I>Head.</I> (i) Text (<I>2</I>).</P>\n<P>(A)(<I>1</I>) Text.</P>\n<CITA TYPE=\"N\">[Cite]\n</CITA>\n</DIV8>\n")
	tree, err := parse.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"section":"0.1","heading":"§ 0.1 Head.","citation":"[Cite]\n","children":[` +
		`{"path":["a"],"kind":"letter","para":1,"heading":"Head","text":"(a) Head—","children":[` +
		`{"path":["a","1"],"kind":"number","para":1,"heading":"Head.","text":"(1) Head. ","children":[` +
		`{"path":["a","1","i"],"kind":"roman","para":1,"text":"(i) Text (2).","children":[` +
		`{"path":["a","1","i","A"],"kind":"capital","para":2,"text":"(A)","children":[` +
		`{"path":["a","1","i","A","1"],"kind":"italic","para":2,"text":"(1) Text."}]}]}]}]}]}`
	if string(got) != want {
		t.Errorf("JSON =\n%s\nwant\n%s", got, want)
	}

	var back parse.Tree
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&back, tree) {
		t.Errorf("JSON round trip differs")
	}
}

func TestKindText(t *testing.T) {
	for k, name := range map[parse.Kind]string{
		parse.Letter: "letter", parse.Number: "number", parse.Roman: "roman", parse.Capital: "capital", parse.Italic: "italic",
	} {
		if got := k.String(); got != name {
			t.Errorf("Kind(%d).String() = %q, want %q", int(k), got, name)
		}
	}
	var k parse.Kind
	if err := json.Unmarshal([]byte(`"bold"`), &k); err == nil {
		t.Errorf("unmarshal of an unknown kind succeeded")
	}
	for _, k := range []parse.Kind{-1, 5} {
		if _, err := json.Marshal(k); err == nil {
			t.Errorf("marshal of Kind(%d) succeeded", int(k))
		}
		if got, want := k.String(), fmt.Sprintf("Kind(%d)", int(k)); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

// TestTreeJSONOmits: a section with no CITA has no "citation", and a node
// with no children has no "children".
func TestTreeJSONOmits(t *testing.T) {
	tree, err := parse.Parse(section("(a) text."))
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"section":"0.1","heading":"Head.","children":[{"path":["a"],"kind":"letter","para":1,"text":"(a) text."}]}`
	if string(got) != want {
		t.Errorf("JSON =\n%s\nwant\n%s", got, want)
	}
}
