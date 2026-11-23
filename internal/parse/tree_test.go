// The tree's JSON form: the shape plan step 5 writes to tree.json.
package parse_test

import (
	"encoding/json"
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
		`{"path":["a"],"kind":"letter","heading":"Head","text":"(a) Head—","children":[` +
		`{"path":["a","1"],"kind":"number","heading":"Head.","text":"(1) Head. ","children":[` +
		`{"path":["a","1","i"],"kind":"roman","text":"(i) Text (2).","children":[` +
		`{"path":["a","1","i","A"],"kind":"capital","text":"(A)","children":[` +
		`{"path":["a","1","i","A","1"],"kind":"italic","text":"(1) Text."}]}]}]}]}]}`
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
	if _, err := json.Marshal(parse.Kind(9)); err == nil {
		t.Errorf("marshal of an unknown kind succeeded")
	}
}
