package l0

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

// Conflict is a command that what is stored refuses: a write that would
// change a stored file (raw XML is never overwritten), or a meta.yaml change
// that the stored meta.yaml or raw XML doesn't support. It is reported,
// nothing is written, and the command exits 1.
type Conflict struct{ msg string }

func (e *Conflict) Error() string { return e.msg }

// Conflictf returns a *Conflict with the formatted message.
func Conflictf(format string, a ...any) error {
	return &Conflict{fmt.Sprintf(format, a...)}
}

// TempPattern names the temporary file StoreOnce and Replace write beside
// their target.
// .gitignore ignores it under sources/, so an interrupted run leaves nothing
// that can be committed.
const TempPattern = ".fetch-*"

// StoreOnce writes body to the file at slash path p under root unless a file
// is already there. Identical bytes are a no-op (stored false); different
// bytes are a *Conflict and the file is left as it was. The body is
// written to a temporary file, synced, and hard-linked into place, which
// fails if anything is at p, so a file is never overwritten, never seen
// half-written, and never left short by a crash. So the corpus needs a
// filesystem with hard links (not FAT or exFAT); there is no fallback. Every
// directory on the way to p must be a real one (RealDirs).
func StoreOnce(root, p string, body []byte) (stored bool, err error) {
	if err := RealDirs(root, path.Dir(p)); err != nil {
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
	tmp, err := writeTemp(dir, body)
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp)
	return link(tmp, full, p, body)
}

// Replace writes body to the file at slash path p under root, which the
// caller has read as a regular file, in one step: a temporary file beside it,
// synced, then renamed over it, so the file is never seen half-written and
// never left short by a crash. Only meta.yaml is replaced, and only by
// Update, under its source's lock; raw XML is stored once (StoreOnce). Every
// directory on the way to p must be a real one (RealDirs).
func Replace(root, p string, body []byte) error {
	if err := RealDirs(root, path.Dir(p)); err != nil {
		return err
	}
	full := filepath.Join(root, filepath.FromSlash(p))
	tmp, err := writeTemp(filepath.Dir(full), body)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	return os.Rename(tmp, full)
}

// writeTemp writes body, mode 0644 and synced, to a new temporary file in
// dir (TempPattern) and returns its path. The caller removes it.
func writeTemp(dir string, body []byte) (string, error) {
	tmp, err := os.CreateTemp(dir, TempPattern)
	if err != nil {
		return "", err
	}
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
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

// link places tmp, holding body, at full (slash path p) by a hard link, which
// fails if anything is there. Then (something written since StoreOnce read
// full, or a symlink to nothing) what is there is compared, never replaced:
// the same bytes are a no-op (stored false), others a *Conflict.
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

// RealDirs returns a *Conflict when an entry on slash path dir under
// root, dir itself included, is anything but a real directory: a symlink
// (even to a directory) or a file. L0 is read and written only through real
// directories, so nothing stored lands outside the root or behind a link git
// can't see through. A missing entry ends the walk: what StoreOnce creates
// there is real.
func RealDirs(root, dir string) error {
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
			return Conflictf("%s is not a real directory (it is a symlink or a file); regcorpus reads and writes L0 only through real directories under the corpus root, and changes nothing (R1)", p)
		}
	}
	return nil
}

// compare returns nil when the stored bytes have (at slash path p) equal
// body, and a *Conflict naming p and both hashes when they don't.
func compare(p string, have, body []byte) error {
	if bytes.Equal(have, body) {
		return nil
	}
	return Conflictf("%s exists with different bytes (stored sha256 %s, fetched sha256 %s); fetch never overwrites a stored file (R1)",
		p, SHA256Hex(have), SHA256Hex(body))
}

// SHA256Hex is b's sha256 as 64 lowercase hex digits.
func SHA256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
