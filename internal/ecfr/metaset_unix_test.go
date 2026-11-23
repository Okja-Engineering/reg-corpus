//go:build darwin || linux

// meta set takes its source's lock only on a real directory (R1, plan step 3).
package ecfr

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestMetaSetLocksOnlyARealSourceDirectory: when the source directory is
// not a real directory (a named pipe, a symlink to one, a symlink to
// nothing, a symlink to a directory outside the root), meta set exits 1
// naming it as not a real directory, at once, and changes nothing inside
// the root or outside it.
func TestMetaSetLocksOnlyARealSourceDirectory(t *testing.T) {
	const dir = "sources/ecfr/title-12/1024.41"
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, root, outside, full string)
	}{
		{"a named pipe", func(t *testing.T, _, _, full string) {
			if err := syscall.Mkfifo(full, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"a symlink to a named pipe", func(t *testing.T, _, outside, full string) {
			fifo := filepath.Join(outside, "pipe")
			if err := syscall.Mkfifo(fifo, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(fifo, full); err != nil {
				t.Fatal(err)
			}
		}},
		{"a symlink to nothing", func(t *testing.T, _, outside, full string) {
			if err := os.Symlink(filepath.Join(outside, "nowhere"), full); err != nil {
				t.Fatal(err)
			}
		}},
		{"a symlink to a source outside the root", func(t *testing.T, _, outside, full string) {
			writeFile(t, outside, "1024.41/raw/2025-07-15.xml", citedBody)
			writeFile(t, outside, "1024.41/meta.yaml", fetchMeta("2025-07-15", sha(citedBody)))
			if err := os.Symlink(filepath.Join(outside, "1024.41"), full); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, outside := corpusRoot(t), t.TempDir()
			full := filepath.Join(root, filepath.FromSlash(dir))
			if err := os.RemoveAll(full); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, root, outside, full)
			before, beforeOutside := tree(t, root), tree(t, outside)
			type result struct {
				code           int
				stdout, stderr string
			}
			done := make(chan result, 1)
			go func() {
				code, stdout, stderr := metaAt(t, root, "set", "1024.41", "version_effective_date", "2025-07-15")
				done <- result{code, stdout, stderr}
			}()
			var r result
			select {
			case r = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("meta set did not return within 5s: it is blocked opening what is at " + dir)
			}
			if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, dir+" is not a real directory") {
				t.Errorf("exit %d, stdout %q, stderr %q; want 1 naming %s as not a real directory", r.code, r.stdout, r.stderr, dir)
			}
			if !equalTrees(before, tree(t, root)) || !equalTrees(beforeOutside, tree(t, outside)) {
				t.Errorf("files changed under the root or outside it")
			}
		})
	}
}
