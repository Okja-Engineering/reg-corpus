//go:build darwin

package verify

import (
	"bytes"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

// openPTY returns the terminal side of a new pseudo-terminal.
func openPTY(t *testing.T) *os.File {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	var name [128]byte
	for _, req := range []struct {
		op  uintptr
		arg uintptr
	}{
		{syscall.TIOCPTYGRANT, 0},
		{syscall.TIOCPTYUNLK, 0},
		{syscall.TIOCPTYGNAME, uintptr(unsafe.Pointer(&name[0]))},
	} {
		if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, m.Fd(), req.op, req.arg); e != 0 {
			t.Fatalf("ioctl %#x: %v", req.op, e)
		}
	}
	s, err := os.OpenFile(string(name[:bytes.IndexByte(name[:], 0)]), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
