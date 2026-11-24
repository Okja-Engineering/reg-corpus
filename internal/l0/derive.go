package l0

import "errors"

// File is a file derived from a source's raw XML: its slash path from the
// corpus root and its bytes. Stub for the red commit (plan step 5).
type File struct {
	Path string
	Body []byte
}

// Derive is a stub for the red commit (plan step 5).
func Derive(root string, files []File) ([]bool, error) {
	return nil, errors.New("l0: Derive not built yet (plan step 5)")
}
