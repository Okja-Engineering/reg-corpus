//go:build darwin

package ecfr

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
)

// TestParsePrintsWhatItWroteBeforeAFailure: when text.md can't be replaced
// (chflags uchg), tree.json, replaced before it, is printed as written, the
// command exits 2, not 1 (which says nothing was written), and a re-run
// writes the rest.
func TestParsePrintsWhatItWroteBeforeAFailure(t *testing.T) {
	root := corpusRoot(t)
	storedSource(t, root, "1024.41")
	dir := "sources/ecfr/title-12/1024.41/"
	writeFile(t, root, dir+"tree.json", "stale\n")
	writeFile(t, root, dir+"text.md", "stale\n")
	text := filepath.Join(root, filepath.FromSlash(dir+"text.md"))
	if err := syscall.Chflags(text, 0x2); err != nil { // UF_IMMUTABLE
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Chflags(text, 0) })
	code, stdout, stderr := parseAt(t, root, "1024.41")
	if want := "1024.41: wrote " + dir + "tree.json\n"; code != 2 || stdout != want || !strings.HasPrefix(stderr, "regcorpus parse: 1024.41: ") {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2, %q and the error", code, stdout, stderr, want)
	}
	if err := syscall.Chflags(text, 0); err != nil {
		t.Fatal(err)
	}
	d, err := anchors.Build(repoFile(t, rawPath("1024.41")))
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = parseAt(t, root, "1024.41")
	want := "1024.41: unchanged " + dir + "tree.json\n1024.41: wrote " + dir + "text.md\n" +
		"1024.41: wrote " + dir + `meta.yaml: sha256_text: "` + string(d.SHA256Text) + "\"\n"
	if code != 0 || stdout != want || stderr != "" {
		t.Errorf("re-run: exit %d, stdout %q, stderr %q; want 0 and\n%s", code, stdout, stderr, want)
	}
}
