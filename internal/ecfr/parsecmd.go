package ecfr

import (
	"fmt"
	"io"
	"strings"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/sink"
)

const parseUsage = `usage: regcorpus parse <section> ...

Writes each named section's derived L0 files from its stored raw XML,
offline, run from the corpus root (R1, R1a; plan step 5, ADR 0013).
<section> is 1024.41, 1024.30 or 1024.31; name at least one. parse writes
a section's files only once its parser goldens are in: today 1024.41's.
1024.30 and 1024.31 are refused (their goldens are plan step 9's). In
sources/ecfr/title-12/<section>/ it writes:
  tree.json   the parsed tree (internal/parse), each node with the sha256
              of its normalized own text
  text.md     the section as it reads, generated from the tree: its HEAD,
              each paragraph and its citation note, verbatim, with no
              markup but the "# " that starts its heading line
and sets sha256_text in meta.yaml (the sha256 of the HEAD and paragraphs,
normalized as one string, the citation note left out) through internal/l0,
under the source's lock, keeping every other field. A file that already
holds what would be written is left as it is, so a re-run changes nothing.
Each file it writes is printed ("wrote"), and each it leaves ("unchanged").

Before writing, parse checks what is stored, as fetch and meta set do:
meta.yaml is in the spec's schema and names the section as the source table
does (cfr, tier, role, source, official), and raw/ holds exactly
raw/<source_version_date>.xml, a regular file with sha256_raw's bytes. It
reads nothing from the network.

Exit 0: written, or already so. Exit 1: refused, with nothing written for
that section: its goldens aren't in (1024.30, 1024.31); the check above
fails (no meta.yaml: run regcorpus fetch first); the raw XML doesn't parse
(the message gives the parser's line and column), or holds a section
heading, a paragraph or a citation note that text.md can't show as it
reads, on one line of its own: one with a line break in it (in the
citation note, anywhere but one LF ending it), or one that shows as an
empty line (nothing but whitespace); the source directory holds an
overrides.yaml (applying overrides isn't built yet, R1a); a symlink, a
directory or another non-regular file is where tree.json, text.md,
meta.yaml or the raw file goes, or where a directory on the way should be;
or a name differs only in case from meta.yaml, tree.json, text.md or
overrides.yaml, or from a directory on the way.
Exit 2: a usage error (no section, an unknown one, a flag), no
sources/ecfr/title-12/ (not the corpus root), a file that can't be read or
written, or a platform without the lock (darwin and linux have it). A
failure part-way through writing a section may leave the files before it
written; they are printed ("wrote"), and a re-run writes the rest. Each
section is parsed whatever another's outcome; when they fail differently
the larger code wins (2 over 1).
`

// ParseMain runs `regcorpus parse` with args (excluding the command name) in
// the current directory and returns the exit code: 0 ok, 1 a section
// refused, with nothing written for that section, 2 usage or a file it
// can't read or write; the larger code when sections fail differently.
func ParseMain(args []string, stdout, stderr io.Writer) int {
	return parseMain(".", args, stdout, stderr)
}

// parseMain is ParseMain in root. The command is the chief of staff's
// choice (ADR 0013, F7): plan step 5 derives tree.json, text.md and
// sha256_text offline from committed raw XML, and fetch needs the network.
// Its handler is here, as meta set's is, because the source table is; the
// work is internal/anchors', which takes an l0.Source and never imports this
// package, so plan step 6's fetch --verify, here, can call it.
func parseMain(root string, args []string, stdout, stderr io.Writer) int {
	usageError := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "regcorpus parse: "+format+"\n", a...)
		sink.Print(stderr, parseUsage)
		return 2
	}
	if len(args) > 0 && (args[0] == "-h" || args[0] == "-help" || args[0] == "--help") {
		sink.Print(stdout, parseUsage)
		return 0
	}
	if len(args) == 0 {
		return usageError("no section; name one or more of %s", strings.Join(names(), ", "))
	}
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return usageError("%q: parse takes no flags", a)
		}
	}
	srcs, err := selectSources(args)
	if err != nil {
		fmt.Fprintf(stderr, "regcorpus parse: %v\n", err)
		return 2
	}
	if err := checkRoot(root, "parse"); err != nil {
		fmt.Fprintf(stderr, "regcorpus parse: %v\n", err)
		return exitCode(err)
	}
	code := 0
	for _, s := range srcs {
		w, err := parseSource(root, s)
		st := s.stored()
		for _, f := range []struct {
			changed bool
			path    string
		}{
			{w.Tree, st.TreePath()},
			{w.Text, st.TextPath()},
			{w.Meta, st.MetaPath() + `: sha256_text: "` + string(w.SHA256Text) + `"`},
		} {
			switch {
			case f.changed:
				fmt.Fprintf(stdout, "%s: wrote %s\n", s.Name, f.path)
			case err == nil:
				fmt.Fprintf(stdout, "%s: unchanged %s\n", s.Name, f.path)
			}
		}
		if err != nil {
			fmt.Fprintf(stderr, "regcorpus parse: %s: %v\n", s.Name, err)
			code = max(code, exitCode(err))
		}
	}
	return code
}

// parseSource writes s's derived files under root (anchors.Write), unless
// its parser goldens aren't in yet: then it is refused, with nothing
// written. On an error, the files written before it are in what it returns.
func parseSource(root string, s Source) (anchors.Written, error) {
	if s.Goldens != "" {
		return anchors.Written{}, l0.Conflictf("goldens are %s's: parse writes a source's files only once its parser goldens are in (stage 01), so nothing is written", s.Goldens)
	}
	return anchors.Write(root, s.stored())
}
