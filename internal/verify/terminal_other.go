//go:build !darwin && !linux

package verify

import "os"

// isTerminal has no terminal check on this platform, so verify always refuses.
func isTerminal(*os.File) bool { return false }
