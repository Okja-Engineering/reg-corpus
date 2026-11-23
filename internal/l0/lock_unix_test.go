//go:build darwin || linux

package l0

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestLockOpensOnlyADirectoryItself: lock takes a directory as it is at the
// path, never what a symlink there points to, and returns at once, with an
// error, for anything but a directory: a named pipe never blocks it.
func TestLockOpensOnlyADirectoryItself(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, p string)
		ok    bool
	}{
		{"a directory", func(t *testing.T, p string) {
			if err := os.Mkdir(p, 0o755); err != nil {
				t.Fatal(err)
			}
		}, true},
		{"a named pipe", func(t *testing.T, p string) {
			if err := syscall.Mkfifo(p, 0o644); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"a regular file", func(t *testing.T, p string) {
			if err := os.WriteFile(p, nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"a symlink to a directory", func(t *testing.T, p string) {
			if err := os.Symlink(t.TempDir(), p); err != nil {
				t.Fatal(err)
			}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "src")
			tc.setup(t, p)
			type result struct {
				unlock func()
				err    error
			}
			done := make(chan result, 1)
			go func() {
				unlock, err := lock(p)
				done <- result{unlock, err}
			}()
			select {
			case r := <-done:
				if (r.err == nil) != tc.ok {
					t.Errorf("lock: %v; want ok %v", r.err, tc.ok)
				}
				if r.unlock != nil {
					r.unlock()
				}
			case <-time.After(5 * time.Second):
				t.Fatal("lock did not return within 5s")
			}
		})
	}
}
