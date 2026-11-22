// Tests for the interactive-terminal requirement of `regcorpus verify`
// (R2a: requires an interactive terminal; R11: a person runs it).
package verify

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const refusal = "regcorpus verify requires an interactive terminal (R2a); a person runs it"

func pipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	return r, w
}

func regular(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "f"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// device opens a character device that is not a terminal.
func device(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		f, err = os.Open(name)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestRequireTerminalRefusesNonTerminals(t *testing.T) {
	pr, pw := pipe(t)
	tty := openPTY(t)
	cases := []struct {
		name          string
		stdin, stdout *os.File
	}{
		{"pipe in, pipe out", pr, pw},
		{"regular file in, regular file out", regular(t), regular(t)},
		{"/dev/null both", device(t, os.DevNull), device(t, os.DevNull)},
		{"/dev/zero both", device(t, "/dev/zero"), device(t, "/dev/zero")},
		{"/dev/random both", device(t, "/dev/random"), device(t, "/dev/random")},
		{"terminal in, /dev/null out", tty, device(t, os.DevNull)},
		{"/dev/null in, terminal out", device(t, os.DevNull), tty},
		{"terminal in, pipe out", tty, pw},
		{"nil handles", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireTerminal(tc.stdin, tc.stdout)
			if err == nil || err.Error() != refusal {
				t.Fatalf("RequireTerminal = %v, want %q", err, refusal)
			}
		})
	}
}

func TestRequireTerminalAcceptsATerminal(t *testing.T) {
	tty := openPTY(t)
	if err := RequireTerminal(tty, tty); err != nil {
		t.Fatalf("RequireTerminal(pty, pty) = %v, want nil", err)
	}
}

func TestMainRefusesWithoutTerminal(t *testing.T) {
	var errb bytes.Buffer
	code := Main([]string{"RX-41-TEST-01"}, device(t, os.DevNull), device(t, os.DevNull), &errb)
	if code != 2 || errb.String() != refusal+"\n" {
		t.Fatalf("exit %d, stderr %q; want 2 and %q", code, errb.String(), refusal)
	}
}

func TestMainInATerminalIsNotImplementedYet(t *testing.T) {
	tty := openPTY(t)
	var errb bytes.Buffer
	code := Main([]string{"RX-41-TEST-01"}, tty, tty, &errb)
	want := "regcorpus verify: not implemented yet (plan step 19)\n"
	if code != 1 || errb.String() != want {
		t.Fatalf("exit %d, stderr %q; want 1 and %q", code, errb.String(), want)
	}
	if strings.Contains(errb.String(), refusal) {
		t.Fatal("refused a terminal")
	}
}
