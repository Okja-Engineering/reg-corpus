// Tests for subcommand dispatch (decision 6: CLI only).
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// files returns a stdin/stdout pair that is not a terminal and a stderr
// file whose contents the test can read back.
func files(t *testing.T) (in, out, errf *os.File) {
	t.Helper()
	dir := t.TempDir()
	open := func(name string) *os.File {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		return f
	}
	return open("in"), open("out"), open("err")
}

func stderrOf(t *testing.T, f *os.File) string {
	t.Helper()
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestUsageErrorsExitTwo(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}, {"fetch"}} {
		in, out, errf := files(t)
		if code := run(args, in, out, errf); code != 2 {
			t.Errorf("args %q: exit %d, want 2", args, code)
		}
		if got := stderrOf(t, errf); !strings.Contains(got, "usage: regcorpus") {
			t.Errorf("args %q: stderr %q, want usage", args, got)
		}
	}
}

func TestDispatchesCheck(t *testing.T) {
	in, out, errf := files(t)
	if code := run([]string{"check", "--corpus", "--root", t.TempDir()}, in, out, errf); code != 2 {
		t.Fatalf("check on an empty root: exit %d, want 2 (not a corpus root)", code)
	}
	if got := stderrOf(t, errf); !strings.Contains(got, "is not a corpus root") {
		t.Fatalf("stderr %q, want the not-a-corpus message", got)
	}
}

func TestDispatchesVerify(t *testing.T) {
	in, out, errf := files(t)
	if code := run([]string{"verify"}, in, out, errf); code != 2 {
		t.Fatalf("verify without a terminal: exit %d, want 2", code)
	}
	if got := stderrOf(t, errf); !strings.Contains(got, "requires an interactive terminal") {
		t.Fatalf("stderr %q, want the terminal refusal", got)
	}
}
