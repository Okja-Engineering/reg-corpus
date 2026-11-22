package ecfr

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

// conflictError is a fetch that would change a stored file. fetch never
// overwrites one; it reports the conflict and exits 1.
type conflictError struct{ msg string }

func (e *conflictError) Error() string { return e.msg }

// tempPattern names the temporary file storeOnce writes beside its target.
// .gitignore ignores it under sources/, so an interrupted run leaves nothing
// that can be committed.
const tempPattern = ".fetch-*"

// storeOnce writes body to the file at slash path p under root unless a file
// is already there. Identical bytes are a no-op (stored false); different
// bytes are a *conflictError and the file is left as it was. The body is
// written to a temporary file, synced, and hard-linked into place, which
// fails if anything is at p, so a file is never overwritten, never seen
// half-written, and never left short by a crash. So the corpus needs a
// filesystem with hard links (not FAT or exFAT); there is no fallback.
func storeOnce(root, p string, body []byte) (stored bool, err error) {
	full := filepath.Join(root, filepath.FromSlash(p))
	have, err := os.ReadFile(full)
	if err == nil {
		return false, compare(p, have, body)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(body)
	if err == nil {
		err = tmp.Chmod(0o644)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return false, err
	}
	if err := os.Link(tmp.Name(), full); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return false, fmt.Errorf("storing %s: fetch places a file by hard link so it never overwrites one, and the link failed (a filesystem without hard links, such as FAT or exFAT, can't hold the corpus): %w", p, err)
		}
		// Something is at p after all: written since the read above, or a
		// symlink to nothing. It is compared, never replaced.
		if have, err = os.ReadFile(full); err != nil {
			return false, fmt.Errorf("%s exists but can't be read: %w", p, err)
		}
		return false, compare(p, have, body)
	}
	return true, nil
}

// compare returns nil when the stored bytes have (at slash path p) equal
// body, and a *conflictError naming p and both hashes when they don't.
func compare(p string, have, body []byte) error {
	if bytes.Equal(have, body) {
		return nil
	}
	return &conflictError{fmt.Sprintf("%s exists with different bytes (stored sha256 %s, fetched sha256 %s); fetch never overwrites a stored file (R1)",
		p, sha256Hex(have), sha256Hex(body))}
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// metaYAML renders the part of s's L0 meta.yaml that exists at fetch time
// (spec "L0 meta.yaml"), for raw XML body at version date, retrieved on
// retrieved when eCFR was up to date as of asOf. Later plan steps add the
// rest: version_effective_date and version_fr_citations after
// source_version_date (step 3), verified_on after ecfr_up_to_date_as_of (step
// 6) and sha256_text last (step 5).
//
// Enum words (tier, role, source) and the bool are plain, as in the spec;
// every other value is double-quoted, so no YAML reader types a date or a
// hash as anything but a string. strconv.Quote's escapes are all YAML
// double-quoted escapes. Rendered with the standard library: the checker
// binary links this package, and the gate's test harnesses build it without
// a go.sum (see PR #2).
func metaYAML(s Source, date, retrieved, asOf Date, body []byte) []byte {
	q := strconv.Quote
	return []byte("cfr: " + q(s.CFR) + "\n" +
		"tier: " + string(s.Tier) + "\n" +
		"role: " + string(s.Role) + "\n" +
		"source: ecfr\n" +
		"official: false\n" +
		"source_version_date: " + q(string(date)) + "\n" +
		"retrieved_at: " + q(string(retrieved)) + "\n" +
		"ecfr_up_to_date_as_of: " + q(string(asOf)) + "\n" +
		"sha256_raw: " + q(sha256Hex(body)) + "\n")
}
