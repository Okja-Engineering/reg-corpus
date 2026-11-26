package ecfr

import (
	"io"

	"github.com/okja-engineering/reg-corpus/internal/lookup"
)

// TextMain runs `regcorpus text [--json] <anchor>` (R8; plan step 7, ADR
// 0015) in the current directory and returns the exit code (lookup.TextMain
// gives them). Its handler is here, as parse's is, because the source table
// is (the chief of staff's F7); the resolver and the printers are
// internal/lookup's, which never imports this package, so it has no network
// code.
func TextMain(args []string, stdout, stderr io.Writer) int {
	return textMain(".", args, stdout, stderr)
}

func textMain(root string, args []string, stdout, stderr io.Writer) int {
	return lookup.TextMain(corpus(root), args, stdout, stderr)
}

// corpus is the source table as internal/lookup reads it, under root: each
// source by its name and stored form, its anchors pending until its parser
// goldens are in (Goldens names the plan step that adds them).
func corpus(root string) lookup.Corpus {
	c := lookup.Corpus{Root: root, Dir: sourcesDir}
	for _, s := range sources {
		c.Sources = append(c.Sources, lookup.Source{Name: s.Name, Stored: s.stored(), Pending: s.Goldens})
	}
	return c
}
