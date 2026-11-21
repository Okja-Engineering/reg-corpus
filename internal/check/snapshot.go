// The corpus as the per-commit C6 rules see it (C6, R5). Two readers build
// it: LoadDir reads a directory (the working tree: `make check`, manual runs),
// LoadTree reads a git tree object (the commit: the pre-commit hook and CI).
// Both judge each top-level entry the same way (addTop), and neither follows
// a symlink, so no rule ever reads outside the snapshot.

package check

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// modulePath is the module a corpus root's go.mod declares.
const modulePath = "github.com/okja-engineering/reg-corpus"

type kind int

const (
	regular kind = iota
	directory
	symlink
	gitlink
	irregular // a device, socket or pipe; in a git tree, any other mode
)

func (k kind) String() string {
	switch k {
	case regular:
		return "regular file"
	case directory:
		return "directory"
	case symlink:
		return "symlink"
	case gitlink:
		return "gitlink (submodule)"
	}
	return "special file"
}

type entry struct {
	kind kind
	body []byte // regular files only
	err  error  // set when the entry could not be read
}

// Snapshot holds go.mod (when a regular file), whether stages/ is a real
// directory, and, keyed by slash path relative to the corpus root, every entry
// at or under each registry (see isRegistry) and each top-level entry whose
// name isn't ASCII.
type Snapshot struct {
	goMod    []byte
	stages   bool
	registry map[string]entry
}

// notCorpus says why the snapshot is not a corpus root, or returns "".
func (s *Snapshot) notCorpus() string {
	if !declaresModule(s.goMod) {
		return "no go.mod declaring module " + modulePath
	}
	if !s.stages {
		return "no stages/ directory"
	}
	return ""
}

func declaresModule(goMod []byte) bool {
	for _, line := range strings.Split(string(goMod), "\n") {
		line, _, _ = strings.Cut(line, "//")
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == "module" {
			p := f[1]
			if u, err := strconv.Unquote(p); err == nil {
				p = u
			}
			return p == modulePath
		}
	}
	return false
}

// isRegistry reports whether a top-level name is the registry: an ASCII name
// equal to "registry" ignoring case (Registry/ too: one directory on a
// case-insensitive checkout). A name that isn't ASCII is never the registry;
// addTop keeps it to be reported, since filesystems fold such names in ways no
// rule here can mirror (APFS folds "regiﬆry" into registry/).
func isRegistry(name string) bool { return isASCII(name) && strings.EqualFold(name, "registry") }

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// addTop judges top-level entry name, of kind k, for both readers: go.mod and
// stages/ are corpus markers only as a regular file and a real directory; a
// registry or a name that isn't ASCII is kept for the rules. It reports
// whether the caller must read this entry's bytes as go.mod, and whether it
// must add every entry under it (a registry directory).
func (s *Snapshot) addTop(name string, k kind) (goMod, walk bool) {
	switch {
	case name == "go.mod":
		return k == regular, false
	case name == "stages":
		s.stages = k == directory
	case isRegistry(name):
		s.registry[name] = entry{kind: k}
		return false, k == directory
	case !isASCII(name):
		s.registry[name] = entry{kind: k}
	}
	return false, false
}

// LoadDir reads the corpus in directory root, as a commit of it would hold
// it: an entry git ignores and doesn't track is left out, since no commit
// holds it (.DS_Store, editor swap files). A root that doesn't exist is an
// empty snapshot (not a corpus); one that can't be listed is an error.
func LoadDir(root string) (*Snapshot, error) {
	s := &Snapshot{registry: map[string]entry{}}
	top, err := os.ReadDir(root) // Lstat types: a symlink is never followed
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return s, nil
	case err != nil:
		return nil, fmt.Errorf("cannot list %s: %v", root, err)
	}
	for _, d := range top {
		goMod, walk := s.addTop(d.Name(), kindOf(d.Type()))
		if goMod {
			s.goMod, _ = os.ReadFile(filepath.Join(root, "go.mod"))
		}
		if walk {
			if err := s.walk(root, d.Name()); err != nil {
				return nil, err
			}
		}
	}
	for _, p := range ignored(root, slices.Collect(maps.Keys(s.registry))) {
		delete(s.registry, p)
	}
	return s, nil
}

// ignored returns the paths (slash, relative to root) that git ignores and
// doesn't track in root's repository. Outside a repository, or if git fails,
// it returns none, so nothing is skipped.
func ignored(root string, paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out, err := git(root, strings.Join(paths, "\x00")+"\x00", "check-ignore", "-z", "--stdin")
	if err != nil { // exit 1: none ignored; 128: not a repository
		return nil
	}
	return strings.FieldsFunc(out, func(r rune) bool { return r == 0 })
}

// walk adds every entry at or under root/name, keyed by its path from root.
func (s *Snapshot) walk(root, name string) error {
	return filepath.WalkDir(filepath.Join(root, name), func(p string, d fs.DirEntry, err error) error {
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if err != nil { // a directory that can't be listed
			e := s.registry[rel]
			e.err = err
			s.registry[rel] = e
			return nil
		}
		e := entry{kind: kindOf(d.Type())}
		if e.kind == regular {
			e.body, e.err = os.ReadFile(p)
		}
		s.registry[rel] = e
		return nil
	})
}

func kindOf(m fs.FileMode) kind {
	switch {
	case m.IsRegular():
		return regular
	case m.IsDir():
		return directory
	case m&fs.ModeSymlink != 0:
		return symlink
	}
	return irregular
}

// LoadTree reads the corpus in git tree-ish treeish of the repository that
// holds dir. It reads raw blobs: no filters, no end-of-line conversion and no
// replace refs.
func LoadTree(dir, treeish string) (*Snapshot, error) {
	// Resolve first, then peel: "rev:path^{tree}" would name the path "path^{tree}".
	oid, err := git(dir, "", "rev-parse", "--verify", "--quiet", "--end-of-options", treeish)
	if err == nil {
		oid, err = git(dir, "", "rev-parse", "--verify", "--quiet", strings.TrimSpace(oid)+"^{tree}")
	}
	if err != nil {
		return nil, fmt.Errorf("cannot resolve tree %q in %s: %v", treeish, dir, err)
	}
	top, err := lsTree(dir, strings.TrimSpace(oid), false)
	if err != nil {
		return nil, err
	}
	s := &Snapshot{registry: map[string]entry{}}
	var blobs []treeEntry // regular files whose bytes the rules read
	for _, e := range top {
		goMod, walk := s.addTop(e.path, e.kind)
		if goMod {
			blobs = append(blobs, e)
		}
		if walk {
			under, err := lsTree(dir, e.oid, true)
			if err != nil {
				return nil, err
			}
			for _, u := range under {
				u.path = path.Join(e.path, u.path)
				s.registry[u.path] = entry{kind: u.kind}
				if u.kind == regular {
					blobs = append(blobs, u)
				}
			}
		}
	}
	bodies, err := catFile(dir, blobs)
	if err != nil {
		return nil, err
	}
	for i, b := range blobs {
		if b.path == "go.mod" {
			s.goMod = bodies[i]
		} else {
			s.registry[b.path] = entry{kind: regular, body: bodies[i]}
		}
	}
	return s, nil
}

type treeEntry struct {
	kind kind
	oid  string
	path string
}

// lsTree lists tree oid: its direct entries, or with recursive every entry
// below it, directories included.
func lsTree(dir, oid string, recursive bool) ([]treeEntry, error) {
	args := []string{"ls-tree", "-z", "--full-tree"}
	if recursive {
		args = append(args, "-r", "-t")
	}
	out, err := git(dir, "", append(args, oid)...)
	if err != nil {
		return nil, fmt.Errorf("git ls-tree %s: %v", oid, err)
	}
	var es []treeEntry
	for _, rec := range strings.Split(out, "\x00") {
		if rec == "" {
			continue
		}
		meta, p, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 {
			return nil, fmt.Errorf("git ls-tree %s: unexpected line %q", oid, rec)
		}
		k := irregular
		switch f[0] {
		case "100644", "100755":
			k = regular
		case "040000":
			k = directory
		case "120000":
			k = symlink
		case "160000":
			k = gitlink
		}
		es = append(es, treeEntry{kind: k, oid: f[2], path: p})
	}
	return es, nil
}

// catFile returns the raw bytes of each blob, in order.
func catFile(dir string, blobs []treeEntry) ([][]byte, error) {
	if len(blobs) == 0 {
		return nil, nil
	}
	var in strings.Builder
	for _, b := range blobs {
		in.WriteString(b.oid + "\n")
	}
	out, err := git(dir, in.String(), "cat-file", "--batch")
	if err != nil {
		return nil, fmt.Errorf("git cat-file: %v", err)
	}
	rest := []byte(out)
	bodies := make([][]byte, len(blobs))
	for i, b := range blobs {
		header, after, ok := bytes.Cut(rest, []byte("\n"))
		f := strings.Fields(string(header))
		if !ok || len(f) != 3 || f[0] != b.oid || f[1] != "blob" {
			return nil, fmt.Errorf("git cat-file: unexpected header %q for %s", header, b.path)
		}
		size, err := strconv.Atoi(f[2])
		if err != nil || size < 0 || len(after) < size+1 {
			return nil, fmt.Errorf("git cat-file: bad size in %q for %s", header, b.path)
		}
		bodies[i] = after[:size]
		rest = after[size+1:]
	}
	return bodies, nil
}

// git runs git in dir with stdin and returns stdout; the error carries
// git's stderr.
func git(dir, stdin string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"--no-replace-objects", "-C", dir}, args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errb.String()); msg != "" {
			return "", fmt.Errorf("%v: %s", err, msg)
		}
		return "", err
	}
	return out.String(), nil
}
