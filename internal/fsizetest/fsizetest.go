//go:build unix

package fsizetest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// rootEnv, when set, makes the test named in the child's -test.run the
// child: it runs its part in that root under the lowered limit and prints
// the outcome after marker.
const (
	rootEnv = "REGCORPUS_TEST_FSIZE_ROOT"
	marker  = "fsize-result: "
)

// outcome is what the child prints: Result is run's outcome, Skip why it
// couldn't lower the limit, After an error writing a file once the limit is
// restored.
type outcome struct {
	Skip, After string
	Result      json.RawMessage
}

// Run runs run in a child test binary with RLIMIT_FSIZE at limit bytes and
// decodes what run returns (as JSON) into out. RLIMIT_FSIZE binds the whole
// process, so run runs in a child: in this one, any write in the window (go
// test's own log of file accesses) would fail too, and in the child the
// limit is restored as soon as run returns, so what writes after it (a
// coverage profile at exit, say) can write in full. In the child it runs
// run on the root the parent named, not on any the test made itself, prints
// the outcome and returns false: the test returns at once. In the parent it
// returns true once out holds the outcome, or skips the test when the child
// couldn't lower the limit.
func Run(t *testing.T, root string, limit uint64, run func(root string) any, out any) bool {
	t.Helper()
	if child := os.Getenv(rootEnv); child != "" {
		var o outcome
		var old syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &old); err != nil {
			t.Fatal(err)
		}
		lim := old
		setCur(&lim.Cur, limit)
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
		fmt.Printf("\n%s%s\n", marker, b)
		return false
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")
	cmd.Env = append(os.Environ(), rootEnv+"="+root)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, b)
	}
	_, line, ok := bytes.Cut(b, []byte(marker))
	if !ok {
		t.Fatalf("child printed no result:\n%s", b)
	}
	line, _, _ = bytes.Cut(line, []byte("\n"))
	var o outcome
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

// setCur sets a limit's Cur to n, converted to the field's type: uint64 on
// most unixes, int64 on FreeBSD and DragonFly.
func setCur[T int64 | uint64](cur *T, n uint64) { *cur = T(n) }
