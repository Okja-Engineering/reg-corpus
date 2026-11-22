//go:build darwin || linux

package l0

import (
	"fmt"
	"os"
	"syscall"
)

// lock takes an exclusive flock(2) on dir, a source directory, waiting while
// another Update holds it, and returns the function that releases it. The
// lock belongs to the open directory, so it is released when that is closed,
// even by a crash: no lock file is left behind.
func lock(dir string) (unlock func(), err error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if err != syscall.EINTR {
			break
		}
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("locking %s: %w", dir, err)
	}
	return func() { f.Close() }, nil
}
