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

	"gopkg.in/yaml.v3"
)

// conflictError is a fetch that would change a stored file. fetch never
// overwrites one; it reports the conflict and exits 1.
type conflictError struct{ msg string }

func (e *conflictError) Error() string { return e.msg }

// storeOnce writes body to the file at slash path p under root unless a file
// is already there. Identical bytes are a no-op (stored false); different
// bytes are a *conflictError and the file is left as it was. The body is
// written to a temporary file and hard-linked into place, which fails if p
// exists, so a file is never overwritten and never seen half-written.
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
	tmp, err := os.CreateTemp(dir, ".fetch-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(body)
	if err == nil {
		err = tmp.Chmod(0o644)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return false, err
	}
	if err := os.Link(tmp.Name(), full); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return false, err
		}
		if have, err = os.ReadFile(full); err != nil { // written since the read above
			return false, err
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

// meta is the part of an L0 meta.yaml that exists at fetch time, in the
// spec's field order ("L0 meta.yaml"). Later plan steps add the rest:
// version_effective_date and version_fr_citations after source_version_date
// (step 3), verified_on after ecfr_up_to_date_as_of (step 6) and sha256_text
// last (step 5).
type meta struct {
	CFR               string `yaml:"cfr"`
	Tier              Tier   `yaml:"tier"`
	Role              Role   `yaml:"role"`
	Source            string `yaml:"source"`
	Official          bool   `yaml:"official"`
	SourceVersionDate Date   `yaml:"source_version_date"`
	RetrievedAt       Date   `yaml:"retrieved_at"`
	ECFRUpToDateAsOf  Date   `yaml:"ecfr_up_to_date_as_of"`
	SHA256Raw         string `yaml:"sha256_raw"`
}

// newMeta returns s's meta.yaml for raw XML body at version date, retrieved
// on retrieved when eCFR was up to date as of asOf.
func newMeta(s Source, date, retrieved, asOf Date, body []byte) meta {
	return meta{
		CFR: s.CFR, Tier: s.Tier, Role: s.Role, Source: "ecfr", Official: false,
		SourceVersionDate: date, RetrievedAt: retrieved, ECFRUpToDateAsOf: asOf,
		SHA256Raw: sha256Hex(body),
	}
}

func (m meta) yaml() ([]byte, error) { return yaml.Marshal(m) }
