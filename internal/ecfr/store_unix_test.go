//go:build unix

// A store that fails mid-write (R1, plan step 2), forced with the file size
// limit: Go ignores SIGXFSZ, so a write past RLIMIT_FSIZE returns EFBIG.
package ecfr

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestAFailedWriteStoresNothing: when writing the body fails, fetch exits 2
// and leaves neither the file nor a temporary behind.
func TestAFailedWriteStoresNothing(t *testing.T) {
	_, c := newFake(t)
	root := corpusRoot(t)
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
		t.Fatal(err)
	}
	lim := old
	lim.Cur = 64 // bytes: less than any section body
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &lim); err != nil {
		t.Skipf("can't lower RLIMIT_FSIZE: %v", err)
	}
	code, stdout, stderr := fetchAt(t, root, c, "2026-11-22", "1024.41")
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
		t.Fatal(err)
	}
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "regcorpus fetch: 1024.41: ") {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2 for 1024.41", code, stdout, stderr)
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.41/raw")))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("left raw/%s behind", e.Name())
	}
}
