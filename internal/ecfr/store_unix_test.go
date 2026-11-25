//go:build unix

// Writes that fail mid-way (R1, plan steps 2 and 5), forced with the file
// size limit on every unix, Linux CI included: Go ignores SIGXFSZ, so a
// write past RLIMIT_FSIZE returns EFBIG (internal/fsizetest).
package ecfr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/fsizetest"
)

// fsizeResult is a command's exit code and output in the child.
type fsizeResult struct {
	Code           int
	Stdout, Stderr string
}

// underFileLimit runs a command, run, in root with RLIMIT_FSIZE at limit
// bytes (fsizetest.Run, in a child test binary) and returns its outcome. It
// returns false in the child, where the test returns at once.
func underFileLimit(t *testing.T, root string, limit uint64, run func(root string) (code int, stdout, stderr string)) (fsizeResult, bool) {
	t.Helper()
	var r fsizeResult
	ok := fsizetest.Run(t, root, limit, func(root string) any {
		var c fsizeResult
		c.Code, c.Stdout, c.Stderr = run(root)
		return c
	}, &r)
	return r, ok
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
