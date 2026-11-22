// Tests for the store's placement of a file (R1): a stored file is never
// replaced, only compared. Offline.
package l0

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestALinkThatFindsAFileComparesIt: when the hard link finds a file already
// at the path (one written after storeOnce read it), that file is compared,
// never replaced: other bytes are a conflict and stay, the same bytes a
// no-op; with nothing there the file is placed.
func TestALinkThatFindsAFileComparesIt(t *testing.T) {
	for _, tc := range []struct {
		name, there string // "" is nothing at the path
		stored      bool
		conflict    bool
	}{
		{name: "other bytes", there: "theirs", conflict: true},
		{name: "the same bytes", there: "ours"},
		{name: "nothing", stored: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tmp, full := filepath.Join(dir, ".fetch-1"), filepath.Join(dir, "f.xml")
			if err := os.WriteFile(tmp, []byte("ours"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.there != "" {
				if err := os.WriteFile(full, []byte(tc.there), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			stored, err := link(tmp, full, "f.xml", []byte("ours"))
			var conflict *Conflict
			if stored != tc.stored || errors.As(err, &conflict) != tc.conflict || (!tc.conflict && err != nil) {
				t.Errorf("stored %v, err %v; want stored %v, conflict %v", stored, err, tc.stored, tc.conflict)
			}
			want := tc.there
			if want == "" {
				want = "ours"
			}
			if b, err := os.ReadFile(full); err != nil || string(b) != want {
				t.Errorf("f.xml holds %q, %v; want %q", b, err, want)
			}
		})
	}
	// A link that fails for another reason (here, no temporary file) is
	// reported as the link failing, not as a file found at the path.
	dir := t.TempDir()
	stored, err := link(filepath.Join(dir, ".fetch-1"), filepath.Join(dir, "f.xml"), "f.xml", []byte("ours"))
	if stored || err == nil || !strings.Contains(err.Error(), "hard link") {
		t.Errorf("stored %v, err %v; want the hard link's failure", stored, err)
	}
}
