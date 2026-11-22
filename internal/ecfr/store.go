package ecfr

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
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
// filesystem with hard links (not FAT or exFAT); there is no fallback. Every
// directory on the way to p must be a real one (realDirs).
func storeOnce(root, p string, body []byte) (stored bool, err error) {
	if err := realDirs(root, path.Dir(p)); err != nil {
		return false, err
	}
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
	return link(tmp.Name(), full, p, body)
}

// link places tmp, holding body, at full (slash path p) by a hard link, which
// fails if anything is there. Then (something written since storeOnce read
// full, or a symlink to nothing) what is there is compared, never replaced:
// the same bytes are a no-op (stored false), others a *conflictError.
func link(tmp, full, p string, body []byte) (stored bool, err error) {
	err = os.Link(tmp, full)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return false, fmt.Errorf("storing %s: fetch places a file by hard link so it never overwrites one, and the link failed (a filesystem without hard links, such as FAT or exFAT, can't hold the corpus): %w", p, err)
	}
	have, err := os.ReadFile(full)
	if err != nil {
		return false, fmt.Errorf("%s exists but can't be read: %w", p, err)
	}
	return false, compare(p, have, body)
}

// realDirs returns a *conflictError when an entry on slash path dir under
// root, dir itself included, is anything but a real directory: a symlink
// (even to a directory) or a file. fetch reads and writes only through real
// directories, so nothing it stores lands outside the root or behind a link
// git can't see through. A missing entry ends the walk: what fetch creates
// there is real.
func realDirs(root, dir string) error {
	p := ""
	for _, name := range strings.Split(dir, "/") {
		p = path.Join(p, name)
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(p)))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil
		case err != nil:
			return err
		case !info.IsDir():
			return &conflictError{fmt.Sprintf("%s is not a real directory (it is a symlink or a file); fetch reads and writes only through real directories under the corpus root, and changes nothing (R1)", p)}
		}
	}
	return nil
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
