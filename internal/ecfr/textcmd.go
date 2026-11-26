package ecfr

import (
	"io"

	"github.com/okja-engineering/reg-corpus/internal/lookup"
)

// TextMain is not built yet (plan step 7, red).
func TextMain(args []string, stdout, stderr io.Writer) int {
	return textMain(".", args, stdout, stderr)
}

func textMain(root string, args []string, stdout, stderr io.Writer) int {
	return lookup.TextMain(corpus(root), args, stdout, stderr)
}

// corpus is not built yet (plan step 7, red).
func corpus(root string) lookup.Corpus { return lookup.Corpus{} }
