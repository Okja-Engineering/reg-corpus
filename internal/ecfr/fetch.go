package ecfr

import (
	"io"
	"time"
)

// fetcher runs `regcorpus fetch` in a corpus root.
type fetcher struct {
	root   string
	client *Client
	now    func() time.Time
}

// Main runs `regcorpus fetch` with args (excluding the subcommand name) in the
// current directory and returns the exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	return fetcher{root: ".", client: NewClient(), now: time.Now}.main(args, stdout, stderr)
}

func (f fetcher) main(args []string, stdout, stderr io.Writer) int { return 2 }
