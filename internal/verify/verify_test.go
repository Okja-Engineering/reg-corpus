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

// charDevice returns a character device that is not a terminal. It stands in
// for "one side is a char device" so the BOTH requirement is exercised.
func charDevice(t *testing.T) *os.File {
	t.Helper()
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestRequireTerminalRefuses(t *testing.T) {
	pr, pw := pipe(t)
	cases := []struct {
		name          string
		stdin, stdout *os.File
	}{
		{"pipe in, pipe out", pr, pw},
		{"regular file in, pipe out", regular(t), pw},
		{"char device in, pipe out", charDevice(t), pw},
		{"pipe in, char device out", pr, charDevice(t)},
		{"char device in, regular file out", charDevice(t), regular(t)},
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

func TestMainRefusesWithoutTerminal(t *testing.T) {
	pr, pw := pipe(t)
	var errb bytes.Buffer
	code := Main([]string{"RX-41-TEST-01"}, pr, pw, &errb)
	if code != 2 || !strings.Contains(errb.String(), refusal) {
		t.Fatalf("exit %d, stderr %q; want 2 and %q", code, errb.String(), refusal)
	}
}
