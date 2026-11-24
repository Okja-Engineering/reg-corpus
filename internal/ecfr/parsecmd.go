package ecfr

import (
	"fmt"
	"io"
	"strings"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
)

const parseUsage = `usage: regcorpus parse <section> ...

Writes each named section's derived L0 files from its stored raw XML,
offline, run from the corpus root (R1, R1a; plan step 5, ADR 0013).
<section> is 1024.41, 1024.30 or 1024.31; name at least one. In
sources/ecfr/title-12/<section>/ it writes:
  tree.json   the parsed tree (internal/parse), each node with the sha256
              of its normalized own text
  text.md     the section as it reads, generated from the tree: its HEAD,
              each paragraph and its citation note, verbatim
and sets sha256_text in meta.yaml (the sha256 of the normalized HEAD and
paragraphs, the citation note left out) through internal/l0, under the
source's lock, keeping every other field. A file that already holds what
would be written is left as it is, so a re-run changes nothing.

Before writing, parse checks what is stored, as fetch and meta set do:
meta.yaml is in the spec's schema and names the section as the source table
does (cfr, tier, role, source, official), and raw/ holds exactly
raw/<source_version_date>.xml, a regular file with sha256_raw's bytes. It
reads nothing from the network.

Exit 0: written, or already so. Exit 1: refused by what is stored, with
nothing written for that section: the check above fails (no meta.yaml: run
regcorpus fetch first); the raw XML doesn't parse (the message gives the
parser's line and column); the source directory holds an overrides.yaml
(applying overrides isn't built yet, R1a); or a symlink, a directory or
another non-regular file is where tree.json, text.md, meta.yaml or the raw
file goes, or where a directory on the way should be.
Exit 2: a usage error (no section, an unknown one, a flag), no
sources/ecfr/title-12/ (not the corpus root), a file that can't be read or
written, or a platform without the lock (darwin and linux have it). Each
section is parsed whatever another's outcome; when they fail differently
the larger code wins (2 over 1).
`

// ParseMain runs `regcorpus parse` with args (excluding the command name) in
// the current directory and returns the exit code: 0 ok, 1 refused by what
// is stored, 2 usage or a file it can't read or write; the larger code when
// sections fail differently.
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
		fmt.Fprintf(stderr, "regcorpus parse: "+format+"\n%s", append(a, parseUsage)...)
		return 2
	}
	if len(args) > 0 && (args[0] == "-h" || args[0] == "-help" || args[0] == "--help") {
		fmt.Fprint(stdout, parseUsage)
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
		w, err := anchors.Write(root, s.stored())
		if err != nil {
			fmt.Fprintf(stderr, "regcorpus parse: %s: %v\n", s.Name, err)
			code = max(code, exitCode(err))
			continue
		}
		st := s.stored()
		for _, f := range []struct {
			changed bool
			path    string
		}{
			{w.Tree, st.TreePath()},
			{w.Text, st.TextPath()},
			{w.Meta, st.MetaPath() + `: sha256_text: "` + string(w.SHA256Text) + `"`},
		} {
			verb := "unchanged"
			if f.changed {
				verb = "wrote"
			}
			fmt.Fprintf(stdout, "%s: %s %s\n", s.Name, verb, f.path)
		}
	}
	return code
}
