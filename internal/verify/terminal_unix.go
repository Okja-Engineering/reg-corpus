//go:build darwin || linux

package verify

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// isTerminal reports whether f is a terminal: the terminal-attributes ioctl
// succeeds on it (what isatty(3) does). Character devices that are not
// terminals, such as /dev/null, fail it.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	var t syscall.Termios
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), ioctlReadTermios, uintptr(unsafe.Pointer(&t)))
	runtime.KeepAlive(f)
	return e == 0
}
