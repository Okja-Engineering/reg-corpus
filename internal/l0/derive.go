package l0

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// File is a file derived from a source's raw XML (tree.json and text.md,
// plan step 5): its slash path from the corpus root, and its bytes.
type File struct {
	Path string
	Body []byte
}

// Derive writes a source's derived files under root, and is how every
// command that writes one does it: inside Update's set, so under the
// source's lock and from the raw XML Stored has just checked. It first reads
// every file, through real directories only (RealDirs): each must be missing
// or a regular file, and anything else (a symlink, a directory) is a
// *Conflict, with nothing written. Then it replaces each file whose bytes
// differ (Replace: a temporary file renamed over it, so none is seen
// half-written) and leaves the others as they are. It returns, for each
// file, whether it changed. An error while writing can leave the files
// written before it; a re-run writes the rest.
func Derive(root string, files []File) ([]bool, error) {
	changed := make([]bool, len(files))
	for i, f := range files {
		if err := RealDirs(root, path.Dir(f.Path)); err != nil {
			return nil, err
		}
		full := filepath.Join(root, filepath.FromSlash(f.Path))
		info, err := os.Lstat(full)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			changed[i] = true
			continue
		case err != nil:
			return nil, err
		case !info.Mode().IsRegular():
			return nil, Conflictf("%s is not a regular file (a symlink or a directory, say); a derived file is written only as one, so nothing is written (R1)", f.Path)
		}
		have, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		changed[i] = !bytes.Equal(have, f.Body)
	}
	for i, f := range files {
		if changed[i] {
			if err := Replace(root, f.Path, f.Body); err != nil {
				return nil, err
			}
		}
	}
	return changed, nil
}
