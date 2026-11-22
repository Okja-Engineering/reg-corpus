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
// even by a crash: no lock file is left behind. It opens only a directory
// itself: not what a symlink at dir points to (O_NOFOLLOW), and never
// anything else, which fails at once (O_DIRECTORY; O_NONBLOCK, so a named
// pipe can't block the open).
func lock(dir string) (unlock func(), err error) {
	f, err := os.OpenFile(dir, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
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
