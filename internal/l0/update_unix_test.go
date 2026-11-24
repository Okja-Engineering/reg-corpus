//go:build unix

// A write that fails part-way (ADR 0013, §2's contract), forced with the
// file size limit on every unix, Linux CI included: Go ignores SIGXFSZ, so a
// write past RLIMIT_FSIZE returns EFBIG.
package l0

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// fsizeRootEnv, when set, makes the test named in the child's -test.run
// the child: it runs its part in that root under the lowered limit and
// prints the outcome after fsizeMarker.
const (
	fsizeRootEnv = "REGCORPUS_TEST_FSIZE_ROOT"
	fsizeMarker  = "fsize-result: "
)

// fsizeOutcome is what the child prints: Result is run's outcome, Skip why
// it couldn't lower the limit, After an error writing a file once the limit
// is restored.
type fsizeOutcome struct {
	Skip, After string
	Result      json.RawMessage
}

// underFileLimit runs run in a child test binary with RLIMIT_FSIZE at limit
// bytes and decodes its outcome into out. RLIMIT_FSIZE binds the whole
// process, so run runs in a child: in this one, any write in the window (go
// test's own log of file accesses) would fail too, and in the child the
// limit is restored as soon as run returns. In the child (fsizeRootEnv set)
// it runs run on the root the parent named, not on any the test made
// itself, prints the outcome and returns false: the test returns at once.
// In the parent it returns true once out
// holds the outcome, or skips the test when the child couldn't lower the
// limit.
func underFileLimit(t *testing.T, root string, limit uint64, run func(root string) any, out any) bool {
	t.Helper()
	if child := os.Getenv(fsizeRootEnv); child != "" {
		var o fsizeOutcome
		var old syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
			t.Fatal(err)
		}
		lim := syscall.Rlimit{Cur: limit, Max: old.Max}
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &lim); err != nil {
			o.Skip = fmt.Sprintf("can't lower RLIMIT_FSIZE: %v", err)
		} else {
			r := run(child)
			if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(t.TempDir(), "after"), make([]byte, 1<<10), 0o644); err != nil {
				o.After = err.Error()
			}
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			o.Result = b
		}
		b, err := json.Marshal(o)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("\n%s%s\n", fsizeMarker, b)
		return false
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
	cmd.Env = append(os.Environ(), fsizeRootEnv+"="+root)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, b)
	}
	_, line, ok := bytes.Cut(b, []byte(fsizeMarker))
	if !ok {
		t.Fatalf("child printed no result:\n%s", b)
	}
	line, _, _ = bytes.Cut(line, []byte("\n"))
	var o fsizeOutcome
	if err := json.Unmarshal(line, &o); err != nil {
		t.Fatalf("child result %q: %v", line, err)
	}
	if o.Skip != "" {
		t.Skip(o.Skip)
	}
	if o.After != "" {
		t.Fatalf("after run the child can't write a file: %s; the limit outlived it", o.After)
	}
	if err := json.Unmarshal(o.Result, out); err != nil {
		t.Fatalf("child result %s: %v", o.Result, err)
	}
	return true
}

// updateOutcome is what Update returned in the child.
type updateOutcome struct {
	Wrote    Wrote
	Err      string
	Conflict bool
}

// TestUpdateReportsWhatItWroteWhenAWriteFails: with the file size limit
// between tree.json's size and text.md's, Update replaces tree.json, then
// fails writing text.md: tree.json is reported written, meta.yaml is not
// written, the error is not a *Conflict (which says nothing was written)
// and names text.md; a re-run, without the limit, writes the rest.
func TestUpdateReportsWhatItWroteWhenAWriteFails(t *testing.T) {
	const sum = "1111111111111111111111111111111111111111111111111111111111111111"
	text := strings.Repeat("X", 200)
	run := func(root string) any {
		_, w, err := Update(root, testSrc, derivedSum("T", text, sum))
		var c *Conflict
		o := updateOutcome{Wrote: w, Conflict: errors.As(err, &c)}
		if err != nil {
			o.Err = err.Error()
		}
		return o
	}
	root, _ := storedSource(t) // the child's own is never used
	before := snapshot(t, root)
	var o updateOutcome
	if !underFileLimit(t, root, 64, run, &o) {
		return
	}
	if o.Err == "" || o.Conflict || o.Wrote != (Wrote{Tree: true}) || !strings.Contains(o.Err, testSrc.TextPath()) {
		t.Errorf("%+v; want tree.json written, then an error naming %s that is not a conflict", o, testSrc.TextPath())
	}
	if got := changes(before, snapshot(t, root)); len(got) != 1 || got[0] != testSrc.TreePath() {
		t.Errorf("changed %q; want only tree.json", got)
	}
	if _, w, err := Update(root, testSrc, derivedSum("T", text, sum)); err != nil || w != (Wrote{Text: true, Meta: true}) {
		t.Errorf("re-run: %+v, %v; want text.md and meta.yaml written", w, err)
	}
}
