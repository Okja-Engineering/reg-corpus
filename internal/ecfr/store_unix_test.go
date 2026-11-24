//go:build unix

// Writes that fail mid-way (R1, plan steps 2 and 5), forced with the file
// size limit on every unix, Linux CI included: Go ignores SIGXFSZ, so a
// write past RLIMIT_FSIZE returns EFBIG.
package ecfr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
)

// fsizeRootEnv, when set, makes the test named in the child's -test.run
// the child: it runs its command in that root under the lowered limit and
// prints the outcome after fsizeMarker.
const (
	fsizeRootEnv = "REGCORPUS_TEST_FSIZE_ROOT"
	fsizeMarker  = "fsize-result: "
)

// fsizeResult is the child's outcome: the command's exit code and output,
// Skip why it couldn't lower the limit, After an error writing a file once
// the limit is restored.
type fsizeResult struct {
	Skip, After    string
	Code           int
	Stdout, Stderr string
}

// underFileLimit runs a command, run, in a child test binary with
// RLIMIT_FSIZE at limit bytes, in root, and returns its outcome.
// RLIMIT_FSIZE binds the whole process, so the command runs in a child: in
// this one, any write in the window (go test's own log of file accesses)
// would fail too, and in the child the limit is restored as soon as the
// command returns, so what writes after it (a coverage profile at exit,
// say) can write in full. In the child (fsizeRootEnv set) it runs run on
// the root the parent named, not on any the test made itself, prints the
// outcome and returns false: the test returns at once. In the parent it
// returns true with the outcome, or skips the test when the child couldn't
// lower the limit.
func underFileLimit(t *testing.T, root string, limit uint64, run func(root string) (code int, stdout, stderr string)) (fsizeResult, bool) {
	t.Helper()
	var r fsizeResult
	if child := os.Getenv(fsizeRootEnv); child != "" {
		var old syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
			t.Fatal(err)
		}
		lim := syscall.Rlimit{Cur: limit, Max: old.Max}
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &lim); err != nil {
			r.Skip = fmt.Sprintf("can't lower RLIMIT_FSIZE: %v", err)
		} else {
			r.Code, r.Stdout, r.Stderr = run(child)
			if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(t.TempDir(), "after"), make([]byte, 1<<10), 0o644); err != nil {
				r.After = err.Error()
			}
		}
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("\n%s%s\n", fsizeMarker, b)
		return r, false
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
	cmd.Env = append(os.Environ(), fsizeRootEnv+"="+root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	_, line, ok := bytes.Cut(out, []byte(fsizeMarker))
	if !ok {
		t.Fatalf("child printed no result:\n%s", out)
	}
	line, _, _ = bytes.Cut(line, []byte("\n"))
	if err := json.Unmarshal(line, &r); err != nil {
		t.Fatalf("child result %q: %v", line, err)
	}
	if r.Skip != "" {
		t.Skip(r.Skip)
	}
	if r.After != "" {
		t.Fatalf("after the command the child can't write a file: %s; the limit outlived it", r.After)
	}
	return r, true
}

// TestAFailedWriteStoresNothing: when writing the body fails, fetch exits 2,
// names the file it was storing, and leaves neither it nor a temporary
// behind.
func TestAFailedWriteStoresNothing(t *testing.T) {
	root := corpusRoot(t)
	_, c := newFake(t)
	r, ok := underFileLimit(t, root, 64, func(root string) (int, string, string) {
		return fetchAt(t, root, c, "2026-11-22", "1024.41")
	})
	if !ok {
		return
	}
	if want := "regcorpus fetch: 1024.41: storing " + rawPath("1024.41") + ": "; r.Code != 2 || r.Stdout != "" || !strings.HasPrefix(r.Stderr, want) {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2 for 1024.41, %q", r.Code, r.Stdout, r.Stderr, want)
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.41/raw")))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("left raw/%s behind", e.Name())
	}
}

// TestParsePrintsWhatItWroteWhenAWriteFails: with tree.json already right
// and the file size limit between text.md's size and meta.yaml's, parse
// writes text.md, then fails writing meta.yaml: it prints text.md as
// written, names meta.yaml in the error, and exits 2, not 1 (which says
// nothing was written); a re-run, without the limit, writes the rest.
func TestParsePrintsWhatItWroteWhenAWriteFails(t *testing.T) {
	root := corpusRoot(t)
	raw := []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<DIV8 N=\"1024.41\" TYPE=\"SECTION\">\n<HEAD>H</HEAD>\n<P>(a) x.</P>\n</DIV8>\n")
	storedSourceBody(t, root, "1024.41", raw)
	d, err := anchors.Build(raw)
	if err != nil {
		t.Fatal(err)
	}
	dir := "sources/ecfr/title-12/1024.41/"
	writeFile(t, root, dir+"tree.json", string(d.TreeJSON))
	const limit = 100
	if m := len(readFile(t, root, metaPath("1024.41"))); len(d.TextMD) >= limit || m < limit {
		t.Fatalf("text.md is %d bytes and meta.yaml %d; the limit, %d, must be between them", len(d.TextMD), m, limit)
	}
	r, ok := underFileLimit(t, root, limit, func(root string) (int, string, string) { return parseAt(t, root, "1024.41") })
	if !ok {
		return
	}
	if want := "1024.41: wrote " + dir + "text.md\n"; r.Code != 2 || r.Stdout != want || !strings.HasPrefix(r.Stderr, "regcorpus parse: 1024.41: writing "+metaPath("1024.41")+": ") {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2, %q and an error naming meta.yaml", r.Code, r.Stdout, r.Stderr, want)
	}
	code, stdout, stderr := parseAt(t, root, "1024.41")
	want := "1024.41: unchanged " + dir + "tree.json\n1024.41: unchanged " + dir + "text.md\n" +
		"1024.41: wrote " + dir + `meta.yaml: sha256_text: "` + string(d.SHA256Text) + "\"\n"
	if code != 0 || stdout != want || stderr != "" {
		t.Errorf("re-run: exit %d, stdout %q, stderr %q; want 0 and\n%s", code, stdout, stderr, want)
	}
}
