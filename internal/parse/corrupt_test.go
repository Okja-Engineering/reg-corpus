// The corrupted-sequence fixtures (R1a; plan step 4): each file in testdata/
// is the stored 1024.41 XML with one label changed. Each must fail with its
// reason and the label's XML location.
package parse_test

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/parse"
)

type corruption struct {
	file     string
	old, new string // the one change from the stored XML
	reason   parse.Reason
	label    string
	line     int
	column   int
}

var corruptions = []corruption{
	// (c)(3)(i)(B) relabeled (C): (C) follows (A).
	{"skipped.xml", "<P>(B) The date the servicer", "<P>(C) The date the servicer", parse.Skipped, "(C)", 25, 4},
	// (g)(3) relabeled (2): (2) follows (2).
	{"repeated.xml", "<P>(3) The borrower fails", "<P>(2) The borrower fails", parse.Repeated, "(2)", 61, 4},
	// (h)(4) relabeled (2): (2) follows (3).
	{"out-of-order.xml", "<P>(4) <I>Appeal determination.</I>", "<P>(2) <I>Appeal determination.</I>", parse.OutOfOrder, "(2)", 65, 4},
	// (c)(2)(v)(A), chained after its parent's heading, made an italic (1):
	// an italic number directly under a roman has no level to go to.
	{"unplaceable.xml", "options.</I> (A) Notwithstanding", "options.</I> (<I>1</I>) Notwithstanding", parse.Unplaceable, "(1)", 18, 65},
}

func TestCorruptedSequences(t *testing.T) {
	raw := readRaw(t)
	for _, c := range corruptions {
		t.Run(string(c.reason), func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + c.file)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Count(raw, []byte(c.old)) != 1 {
				t.Fatalf("%q is not in the stored XML exactly once", c.old)
			}
			if want := bytes.Replace(raw, []byte(c.old), []byte(c.new), 1); !bytes.Equal(data, want) {
				t.Fatalf("testdata/%s is not the stored XML with only %q → %q", c.file, c.old, c.new)
			}
			tree, err := parse.Parse(data)
			if err == nil {
				t.Fatalf("Parse succeeded with %d top-level nodes, want a %s error", len(tree.Children), c.reason)
			}
			var pe *parse.Error
			if !errors.As(err, &pe) {
				t.Fatalf("error %v is not a *parse.Error", err)
			}
			if pe.Reason != c.reason || pe.Label != c.label || pe.Line != c.line || pe.Column != c.column {
				t.Errorf("error = %s %s at %d:%d (%v), want %s %s at %d:%d",
					pe.Reason, pe.Label, pe.Line, pe.Column, err, c.reason, c.label, c.line, c.column)
			}
		})
	}
}
