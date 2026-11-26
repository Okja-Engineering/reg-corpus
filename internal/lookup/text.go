package lookup

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/sink"
)

const textUsage = `usage: regcorpus text [--json] <anchor>

Prints one L0 anchor's own text, verbatim, with its sha256, its children's
anchor IDs and its source's meta.yaml (R8; plan step 7, ADR 0015), offline,
run from the corpus root. <anchor> is one anchor ID, matched exactly and
case-sensitively: the section with "." made "-", then the label path
joined by "-", e.g. 1024-41-b-2-i-B, or 1024-41-c-2-v-A-1 for the italic
level. No other form is read: not 1024.41(b)(2)(i)(B), not 1024-41-B for
1024-41-b. Today 1024.41's anchors resolve; 1024.30's and 1024.31's are
plan step 9's, and Supplement I's comment anchors (an "interp" segment, as
in 1024-41-b-2-i-interp-1) plan step 8's.

It prints, a line each:
  the anchor ID
  its own text as tree.json holds it: label and heading included, children
  excluded, never normalized or wrapped
  an empty line
  sha256: the sha256 of its normalized own text, as tree.json holds it
  children: its children's anchor IDs in document order, or none (their
  text is not printed: text one of them to read it)
  meta.yaml: the source's meta.yaml path
  then each line of that meta.yaml, as stored
--json prints one line instead: a JSON object with the keys anchor, text,
sha256, children (an array), meta_path and meta (meta.yaml's fields in its
order, each a string but official, a boolean, and version_fr_citations, an
array; verified_on null when null). Every line goes through the sink: a
control character, U+2028 or U+2029 in a text is written escaped, as \xNN
or \uNNNN, or in --json as a JSON escape.

It reads only the source's meta.yaml and tree.json, and before printing
checks them: meta.yaml is in the spec's schema and names the source as the
source table does (cfr, tier, role, source, official); tree.json is a
regular file that decodes as the tree's JSON form, with no unknown field,
for the source's section, with no null node, none without a label path and
no anchor ID twice; its text, joined as decision 13 joins it, hashes to
meta.yaml's sha256_text; and the anchor's own text hashes to its sha256.
It reads no raw XML, nothing under registry/ or staging/, and nothing from
the network, and writes nothing.

Exit 0: printed. Exit 1, with nothing printed: no such anchor in the
source the ID names (an unknown label, another case, the section's own ID
such as 1024-41); the source's anchors are a later plan step's; or a stored
file refuses: the check above fails, meta.yaml or tree.json is missing (run
regcorpus fetch, or regcorpus parse), either is not a regular file, a name
differs only in case from meta.yaml, tree.json, text.md or overrides.yaml,
or a directory on the way is a symlink. Exit 2: a usage error (no anchor,
more than one, a flag but --json), an argument that names no source (an
anchor ID starts with its section's: 1024-41, 1024-30 or 1024-31), no
sources/ecfr/title-12/ (not the corpus root), or a file that can't be read.
`

// TextMain runs `regcorpus text [--json] <anchor>` in c with args
// (excluding the command name) and returns the exit code: 0 printed; 1 no
// such anchor, one a later plan step builds, or a stored file that refuses
// (*l0.Conflict); 2 usage, an argument that names no source, not the
// corpus root, or a file it can't read. It resolves and renders everything
// before it prints, so on an error stdout holds nothing; each line it
// prints is one Write, and --json's one line one Write.
func TextMain(c Corpus, args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("regcorpus text", flag.ContinueOnError)
	fl.SetOutput(io.Discard)
	asJSON := fl.Bool("json", false, "")
	usageError := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "regcorpus text: "+format+"\n", a...)
		sink.Print(stderr, textUsage)
		return 2
	}
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			sink.Print(stdout, textUsage)
			return 0
		}
		return usageError("%v", err)
	}
	switch fl.NArg() {
	case 0:
		return usageError("no anchor; name one, e.g. 1024-41-b-2-i-B")
	case 1:
	default:
		return usageError("%d arguments; text takes one anchor, after any flag", fl.NArg())
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "regcorpus text: %v\n", err)
		return exitCode(err)
	}
	if err := l0.CorpusRoot(c.Root, c.Dir, "text"); err != nil {
		return fail(err)
	}
	a, err := c.Anchor(fl.Arg(0))
	if err != nil {
		return fail(err)
	}
	var out []string
	if *asJSON {
		b, err := a.json()
		if err != nil {
			return fail(err)
		}
		out = []string{string(b)}
	} else if out, err = a.lines(); err != nil {
		return fail(err)
	}
	for _, l := range out {
		io.WriteString(stdout, l)
	}
	return 0
}

// exitCode is 1 for an anchor a lookup can't print (no such anchor, or one
// a later plan step builds) and for a stored file that refuses
// (*l0.Conflict); 2 for an ID that names no source and for anything else.
func exitCode(err error) int {
	var c *l0.Conflict
	switch {
	case errors.Is(err, ErrNoSource):
		return 2
	case errors.Is(err, ErrNoAnchor), errors.Is(err, ErrNotBuilt), errors.As(err, &c):
		return 1
	}
	return 2
}
