package lookup

import (
	"fmt"
	"io"

	"github.com/okja-engineering/reg-corpus/internal/l0"
)

// Source is not built yet (plan step 7, red).
type Source struct {
	Name    string
	Stored  l0.Source
	Pending string
}

// Corpus is not built yet (plan step 7, red).
type Corpus struct {
	Root    string
	Dir     string
	Sources []Source
}

// TextMain is not built yet (plan step 7, red).
func TextMain(c Corpus, args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "regcorpus text: not built yet")
	return 2
}
