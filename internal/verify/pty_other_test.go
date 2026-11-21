//go:build !darwin && !linux

package verify

import (
	"os"
	"testing"
)

func openPTY(t *testing.T) *os.File {
	t.Skip("no pseudo-terminal support in this test on this platform")
	return nil
}
