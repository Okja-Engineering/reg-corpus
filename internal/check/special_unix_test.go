//go:build unix

// A named pipe under registry/ (C6 registry state). Mkfifo exists only on
// unix, so this test builds only there; the rest of the suite builds anywhere.
package check

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSpecialFileFails(t *testing.T) {
	root := corpus(t, map[string]string{"registry/verify.log": ""})
	os.MkdirAll(filepath.Join(root, "registry", "records"), 0o755)
	if err := syscall.Mkfifo(filepath.Join(root, "registry", "records", "RX-41-TEST-01.yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, got := lines(t, "--corpus", "--root", root)
	want(t, code, got, 1, p1+"is a special file; registry/ holds only regular files")
}
