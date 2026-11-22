//go:build unix

// A store that fails mid-write (R1, plan step 2), forced with the file size
// limit: Go ignores SIGXFSZ, so a write past RLIMIT_FSIZE returns EFBIG.
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
)

// fsizeRootEnv, when set, makes TestAFailedWriteStoresNothing the child: it
// runs the fetch in that root under the lowered limit and prints the result.
const fsizeRootEnv = "REGCORPUS_TEST_FSIZE_ROOT"

// fsizeResult is the child's fetch outcome, printed after fsizeMarker.
type fsizeResult struct {
	Skip           string
	Code           int
	Stdout, Stderr string
}

const fsizeMarker = "fsize-result: "

// TestAFailedWriteStoresNothing: when writing the body fails, fetch exits 2
// and leaves neither the file nor a temporary behind. RLIMIT_FSIZE binds the
// whole process, so the fetch runs in a child test binary: in this one, any
// write in the window (go test's own log of file accesses) would fail too.
func TestAFailedWriteStoresNothing(t *testing.T) {
	if root := os.Getenv(fsizeRootEnv); root != "" {
		fsizeChild(t, root)
		return
	}
	root := corpusRoot(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestAFailedWriteStoresNothing$", "-test.count=1")
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
	var r fsizeResult
	if err := json.Unmarshal(line, &r); err != nil {
		t.Fatalf("child result %q: %v", line, err)
	}
	if r.Skip != "" {
		t.Skip(r.Skip)
	}
	if r.Code != 2 || r.Stdout != "" || !strings.HasPrefix(r.Stderr, "regcorpus fetch: 1024.41: ") {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2 for 1024.41", r.Code, r.Stdout, r.Stderr)
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.41/raw")))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("left raw/%s behind", e.Name())
	}
}

// fsizeChild fetches 1024.41 into root with RLIMIT_FSIZE at 64 bytes, less
// than any section body, and prints the outcome on stdout (a pipe, which the
// limit doesn't bind).
func fsizeChild(t *testing.T, root string) {
	_, c := newFake(t)
	var r fsizeResult
	lim := syscall.Rlimit{Cur: 64}
	var old syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
		t.Fatal(err)
	}
	lim.Max = old.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &lim); err != nil {
		r.Skip = fmt.Sprintf("can't lower RLIMIT_FSIZE: %v", err)
	} else {
		r.Code, r.Stdout, r.Stderr = fetchAt(t, root, c, "2026-11-22", "1024.41")
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("\n%s%s\n", fsizeMarker, b)
}
