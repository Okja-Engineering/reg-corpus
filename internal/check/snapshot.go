// The corpus as the per-commit C6 rules see it (C6, R5). Two readers build
// it: LoadDir reads a directory (the working tree: `make check`, manual runs),
// LoadTree reads a git tree object (the commit: the pre-commit hook). Neither
// follows a symlink, so no rule ever reads outside the snapshot.

package check

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// modulePath is the module a corpus root's go.mod declares.
const modulePath = "github.com/okja-engineering/reg-corpus"

type kind int

const (
	regular kind = iota
	directory
	symlink
	gitlink
	irregular // a device, socket or pipe
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

// Snapshot holds go.mod, whether stages/ is a directory, and every entry at or
// under registry/, keyed by slash path relative to the corpus root.
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

// LoadDir reads the corpus in directory root.
func LoadDir(root string) (*Snapshot, error) {
	s := &Snapshot{registry: map[string]entry{}}
	s.goMod, _ = os.ReadFile(filepath.Join(root, "go.mod"))
	if fi, err := os.Stat(filepath.Join(root, "stages")); err == nil && fi.IsDir() {
		s.stages = true
	}
	reg := filepath.Join(root, "registry")
	fi, err := os.Lstat(reg)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return s, nil
	case err != nil:
		s.registry["registry"] = entry{err: err}
		return s, nil
	case !fi.IsDir(): // WalkDir would follow a symlinked root
		s.registry["registry"] = entry{kind: kindOf(fi.Mode())}
		return s, nil
	}
	err = filepath.WalkDir(reg, func(p string, d fs.DirEntry, err error) error {
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
	return s, err
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
		switch {
		case e.path == "go.mod" && e.kind == regular:
			blobs = append(blobs, e)
		case e.path == "stages" && e.kind == directory:
			s.stages = true
		case e.path == "registry":
			s.registry["registry"] = entry{kind: e.kind}
			if e.kind != directory {
				break
			}
			under, err := lsTree(dir, e.oid, true)
			if err != nil {
				return nil, err
			}
			for _, u := range under {
				u.path = path.Join("registry", u.path)
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
