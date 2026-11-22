package l0

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/okja-engineering/reg-corpus/internal/meta"
)

// Source is a stored L0 source as its writers see it: its directory, and
// who its meta.yaml must say it is. Its caller takes CFR, Tier and Role from
// the source table (internal/ecfr, sources.go).
type Source struct {
	Dir  string // slash path from the corpus root, e.g. sources/ecfr/title-12/1024.41
	CFR  string
	Tier meta.Tier
	Role meta.Role
}

// Identity is the part of s's meta.yaml that names the source: cfr, tier
// and role from the source table; source ecfr and official false, as for
// every L0 source (R1: eCFR is not an official edition).
func (s Source) Identity() meta.Meta {
	return meta.Meta{CFR: s.CFR, Tier: s.Tier, Role: s.Role, Source: meta.SourceECFR, Official: false}
}

// MetaPath is s's meta.yaml, by slash path from the corpus root.
func (s Source) MetaPath() string { return path.Join(s.Dir, "meta.yaml") }

// RawDir is s's raw/ directory, by slash path from the corpus root.
func (s Source) RawDir() string { return path.Join(s.Dir, "raw") }

// identity is the comparable form of the fields Identity sets.
type identity struct {
	cfr      string
	tier     meta.Tier
	role     meta.Role
	source   meta.Source
	official bool
}

func identityOf(m meta.Meta) identity {
	return identity{m.CFR, m.Tier, m.Role, m.Source, m.Official}
}

func (id identity) String() string {
	return fmt.Sprintf("cfr %q, tier %s, role %s, source %s, official %t", id.cfr, id.tier, id.role, id.source, id.official)
}

// State is what s's directory holds, read once: every entry in raw/,
// whatever its name or type, and its meta.yaml (Meta nil when there is none).
type State struct {
	Raw       []fs.DirEntry
	Meta      *meta.Meta
	metaBytes []byte
}

// Read reads s's directory under root: the entries of raw/ and meta.yaml,
// through real directories only (RealDirs). A meta.yaml that isn't a regular
// file in the spec's schema (meta.Parse) is a *Conflict: no command replaces
// one it can't read. A directory or file it can't read is an error as is
// (exit 2).
func Read(root string, s Source) (State, error) {
	var st State
	if err := RealDirs(root, s.RawDir()); err != nil {
		return st, err
	}
	ents, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(s.RawDir())))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return st, err
	}
	st.Raw = ents
	full := filepath.Join(root, filepath.FromSlash(s.MetaPath()))
	info, err := os.Lstat(full)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return st, nil
	case err != nil:
		return st, err
	case !info.Mode().IsRegular():
		return st, Conflictf("%s is not a regular file; meta.yaml is read and written only as one (R1)", s.MetaPath())
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return st, err
	}
	m, err := meta.Parse(b)
	if err != nil {
		return st, Conflictf("%s: %v. It is not a meta.yaml in the spec's schema as internal/meta writes it, and no command replaces one (R1)", s.MetaPath(), err)
	}
	st.Meta, st.metaBytes = &m, b
	return st, nil
}

// Stored checks what every writer of a meta.yaml checks before writing
// anything, and returns the raw XML that meta.yaml describes. The check:
// st has a meta.yaml (Read has parsed it); it names s (Identity); and raw/
// holds exactly raw/<source_version_date>.xml, a regular file with
// sha256_raw's bytes. Anything else is a *Conflict.
func (st State) Stored(root string, s Source) ([]byte, error) {
	m := st.Meta
	if m == nil {
		return nil, Conflictf("%s does not exist: run regcorpus fetch first; only a meta.yaml fetch wrote is changed (R1)", s.MetaPath())
	}
	if have, want := identityOf(*m), identityOf(s.Identity()); have != want {
		return nil, Conflictf("%s says %s, but the source table gives %s: it describes another source, and no command writes to it (R1)",
			s.MetaPath(), have, want)
	}
	name := string(m.SourceVersionDate) + ".xml"
	raw := path.Join(s.RawDir(), name)
	var others []string
	found := false
	for _, e := range st.Raw {
		switch {
		case e.Name() != name:
			others = append(others, path.Join(s.RawDir(), e.Name()))
		case !e.Type().IsRegular():
			return nil, Conflictf("%s is not a regular file; raw XML is read only as one (R1)", raw)
		default:
			found = true
		}
	}
	if !found {
		held := "no raw XML"
		if len(others) > 0 {
			held = strings.Join(others, ", ")
		}
		return nil, Conflictf("%s says source_version_date %q, but %s/ holds %s, not %s, the raw file it describes; if that was deleted, restore it from git (git restore %s) (R1)",
			s.MetaPath(), m.SourceVersionDate, s.RawDir(), held, raw, raw)
	}
	if len(others) > 0 {
		return nil, Conflictf("%s holds %s besides %s, the raw file %s describes: a source keeps one raw file, so nothing is written (R1)",
			s.RawDir()+"/", strings.Join(others, ", "), raw, s.MetaPath())
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(raw)))
	if err != nil {
		return nil, err
	}
	if sum := meta.SHA256(SHA256Hex(body)); sum != m.SHA256Raw {
		return nil, Conflictf("%s has sha256 %s, but %s says sha256_raw %s: it is not the raw file meta.yaml describes (R1)",
			raw, sum, s.MetaPath(), m.SHA256Raw)
	}
	return body, nil
}

// Update changes s's meta.yaml under root, and is how every command that
// changes an existing meta.yaml writes it. Holding s's lock (one Update per
// source at a time, across processes) from before it reads until meta.yaml
// is replaced, it reads s's directory (Read), checks it (Stored), lets set
// change the fields its command owns, given the raw XML, and replaces
// meta.yaml with what meta.Marshal writes (Replace), unless that is what it
// holds. Read, under the lock, refuses a symlink or a file on the way, as it
// does for every command. It returns that meta.yaml and whether it changed. A value
// meta.Marshal refuses after set (set gives only values of their type: one
// that disagrees with the fields stored) is a *Conflict; so is a missing
// source directory, which has no meta.yaml.
func Update(root string, s Source, set func(m *meta.Meta, raw []byte) error) (b []byte, changed bool, err error) {
	unlock, err := lock(filepath.Join(root, filepath.FromSlash(s.Dir)))
	if errors.Is(err, fs.ErrNotExist) {
		_, err = State{}.Stored(root, s)
	}
	if err != nil {
		return nil, false, err
	}
	defer unlock()
	st, err := Read(root, s)
	if err != nil {
		return nil, false, err
	}
	raw, err := st.Stored(root, s)
	if err != nil {
		return nil, false, err
	}
	m := *st.Meta
	if err := set(&m, raw); err != nil {
		return nil, false, err
	}
	b, err = meta.Marshal(m)
	if err != nil {
		return nil, false, Conflictf("%s: %v; nothing is written (R1)", s.MetaPath(), err)
	}
	if bytes.Equal(b, st.metaBytes) {
		return b, false, nil
	}
	return b, true, Replace(root, s.MetaPath(), b)
}
