// Package verify implements `regcorpus verify <ID>`, the only path from
// staging/ into registry/ (R2a). A person walks each cited field beside its
// anchor text; the command writes `verification`, moves the record and appends
// "ID, rev, record sha256, by, on, method" to registry/verify.log.
//
// Requirements: R2a (requires an interactive terminal), R11 (a person runs it;
// agents are blocked by the guard). The interactive walk arrives in plan step
// 19; today only the terminal requirement is built.
package verify

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrNoTerminal is returned when stdin or stdout is not a terminal.
var ErrNoTerminal = errors.New("regcorpus verify requires an interactive terminal (R2a); a person runs it")

// RequireTerminal returns ErrNoTerminal unless both stdin and stdout are
// terminals (a terminal ioctl succeeds on each; see isTerminal).
func RequireTerminal(stdin, stdout *os.File) error {
	if !isTerminal(stdin) || !isTerminal(stdout) {
		return ErrNoTerminal
	}
	return nil
}

// Main runs `regcorpus verify` with args (excluding the subcommand name) and
// returns the exit code: 2 without a terminal, 1 while unimplemented.
func Main(args []string, stdin, stdout *os.File, stderr io.Writer) int {
	if err := RequireTerminal(stdin, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintln(stderr, "regcorpus verify: not implemented yet (plan step 19)")
	return 1
}
