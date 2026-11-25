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

// The names of the files a source's directory holds beside raw/ (spec
// "Granularity"). Read finds them by exact name (exactly, which RealDirs
// uses for directories): a name that differs from one only in case is
// refused, since a case-insensitive filesystem (APFS, the default on macOS)
// would read and write it as that file while git records another name.
const (
	metaName      = "meta.yaml"
	treeName      = "tree.json"
	textName      = "text.md"
	overridesName = "overrides.yaml"
)

// MetaPath is s's meta.yaml, by slash path from the corpus root.
func (s Source) MetaPath() string { return path.Join(s.Dir, metaName) }

// RawDir is s's raw/ directory, by slash path from the corpus root.
func (s Source) RawDir() string { return path.Join(s.Dir, "raw") }

// RawPath is s's raw XML for the version dated d, raw/<d>.xml, by slash path
// from the corpus root.
func (s Source) RawPath(d meta.Date) string { return path.Join(s.RawDir(), string(d)+".xml") }

// TreePath, TextPath and OverridesPath are s's tree.json, text.md and
// overrides.yaml, by slash path from the corpus root (spec "Granularity").
func (s Source) TreePath() string      { return path.Join(s.Dir, treeName) }
func (s Source) TextPath() string      { return path.Join(s.Dir, textName) }
func (s Source) OverridesPath() string { return path.Join(s.Dir, overridesName) }

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
// whatever its name or type, its meta.yaml (Meta nil when there is none), and
// which of meta.yaml, tree.json, text.md and overrides.yaml it holds, by
// exact name.
type State struct {
	Raw       []fs.DirEntry
	Meta      *meta.Meta
	metaBytes []byte
	files     map[string]fs.DirEntry
}

// Read reads s's directory under root: its entries, the entries of raw/ and
// meta.yaml, through real directories only, each by its exact name
// (RealDirs). It finds meta.yaml, tree.json, text.md and overrides.yaml by
// exact name among the directory's entries, as Stored finds the raw file,
// and an entry whose name differs from one of them only in case
// ("TREE.JSON"), whatever it is, is a *Conflict: a case-insensitive
// filesystem would read and write it as that file while git records
// another name. A meta.yaml that isn't a regular file in the spec's
// schema (meta.Parse) is a *Conflict: no command replaces one it can't read.
// A directory or file it can't read is an error as is (exit 2).
func Read(root string, s Source) (State, error) {
	if err := RealDirs(root, s.RawDir()); err != nil {
		return State{}, err
	}
	ents, err := readDir(root, s.Dir)
	if err != nil {
		return State{}, err
	}
	st := State{files: map[string]fs.DirEntry{}}
	for _, name := range []string{metaName, treeName, textName, overridesName} {
		e, err := exactly(ents, s.Dir, name)
		if err != nil {
			return State{}, err
		}
		if e != nil {
			st.files[name] = e
		}
	}
	if st.Raw, err = readDir(root, s.RawDir()); err != nil {
		return State{}, err
	}
	e, ok := st.files[metaName]
	switch {
	case !ok:
		return st, nil
	case !e.Type().IsRegular():
		return State{}, Conflictf("%s is not a regular file; meta.yaml is read and written only as one (R1)", s.MetaPath())
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(s.MetaPath())))
	if err != nil {
		return State{}, err
	}
	m, err := meta.Parse(b)
	if err != nil {
		return State{}, Conflictf("%s: %v. It is not a meta.yaml in the spec's schema as internal/meta writes it, and no command replaces one (R1)", s.MetaPath(), err)
	}
	st.Meta, st.metaBytes = &m, b
	return st, nil
}

// Overrides reports whether the source's directory holds an entry named
// exactly overrides.yaml, whatever it is (a file, an empty one, a
// directory, a symlink), as Read found it: the one record of that.
func (st State) Overrides() bool {
	_, ok := st.files[overridesName]
	return ok
}

// readDir is the entries of the directory at slash path dir under root; none
// when it is missing.
func readDir(root, dir string) ([]fs.DirEntry, error) {
	ents, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return ents, err
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
	raw := s.RawPath(m.SourceVersionDate)
	name := path.Base(raw)
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

// Derived is a source's derived files (plan step 5, ADR 0013), by their
// bytes: what Update writes to its tree.json and text.md.
type Derived struct{ Tree, Text []byte }

// Wrote says which of a source's files Update replaced.
type Wrote struct{ Tree, Text, Meta bool }

// Update changes s's meta.yaml under root and, when set returns them, writes
// s's derived files: it is how every command that changes an existing
// meta.yaml writes it, and the only writer of tree.json and text.md. It
// first checks that s's directory and every directory on the way are real
// ones, by exact name (RealDirs), so the lock opens nothing else. Then,
// holding s's lock (one Update per source at a time, across processes) from
// before it reads until the last file is replaced, it reads s's directory
// (Read, which checks the way again), checks it (Stored), and lets set
// change the fields its command owns, given the raw XML that check read
// and what Read found (st: whether s holds an overrides.yaml, say), and
// return s's derived files or nil. Before writing anything it checks
// each derived file: missing, or a regular file (anything else is a
// *Conflict); one it can't read is an error, not a difference. Then it
// replaces, in turn, tree.json, text.md and meta.yaml (what meta.Marshal
// writes), each only when its bytes differ, each whole (a temporary file
// renamed over it). So raw XML is never written here, meta.yaml is only
// what internal/meta renders, and a derived file is only s's own tree.json
// or text.md, from the raw XML Stored checked, under s's lock.
//
// It returns that meta.yaml and which files it replaced. Every *Conflict is
// found before the first write, so a *Conflict means nothing was written. An
// error while writing is never a *Conflict, names the file it was writing,
// and the files replaced before it are in Wrote; a re-run writes the rest.
// A missing source directory has no meta.yaml: a *Conflict, as Stored gives
// for one.
func Update(root string, s Source, set func(m *meta.Meta, raw []byte, st State) (*Derived, error)) (b []byte, w Wrote, err error) {
	if err := RealDirs(root, s.Dir); err != nil {
		return nil, w, err
	}
	unlock, err := lock(filepath.Join(root, filepath.FromSlash(s.Dir)))
	if errors.Is(err, fs.ErrNotExist) {
		_, err = State{}.Stored(root, s)
	}
	if err != nil {
		return nil, w, err
	}
	defer unlock()
	st, err := Read(root, s)
	if err != nil {
		return nil, w, err
	}
	raw, err := st.Stored(root, s)
	if err != nil {
		return nil, w, err
	}
	m := *st.Meta
	d, err := set(&m, raw, st)
	if err != nil {
		return nil, w, err
	}
	b, err = meta.Marshal(m)
	if err != nil {
		return nil, w, err
	}
	type file struct {
		name  string
		body  []byte
		wrote *bool
	}
	var todo []file
	if d != nil {
		for _, f := range []file{{treeName, d.Tree, &w.Tree}, {textName, d.Text, &w.Text}} {
			differs, err := st.differs(root, s, f.name, f.body)
			if err != nil {
				return nil, w, err
			}
			if differs {
				todo = append(todo, f)
			}
		}
	}
	if !bytes.Equal(b, st.metaBytes) {
		todo = append(todo, file{metaName, b, &w.Meta})
	}
	for _, f := range todo {
		if err := replace(root, path.Join(s.Dir, f.name), f.body); err != nil {
			return b, w, err
		}
		*f.wrote = true
	}
	return b, w, nil
}

// differs reports whether s's file name, which Read found or didn't, must
// be replaced to hold body: it is missing, or a regular file with other
// bytes. Anything else there (a symlink, a directory) is a *Conflict.
func (st State) differs(root string, s Source, name string, body []byte) (bool, error) {
	e, ok := st.files[name]
	switch {
	case !ok:
		return true, nil
	case !e.Type().IsRegular():
		return false, Conflictf("%s is not a regular file (a symlink or a directory, say); a derived file is written only as one, so nothing is written (R1)", path.Join(s.Dir, name))
	}
	have, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path.Join(s.Dir, name))))
	if err != nil {
		return false, err
	}
	return !bytes.Equal(have, body), nil
}
