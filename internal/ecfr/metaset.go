package ecfr

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/okja-engineering/reg-corpus/internal/meta"
)

const metaUsage = `usage: regcorpus meta set <section> version_effective_date YYYY-MM-DD
       regcorpus meta set <section> version_fr_citations "<vol> FR <page>" ...

Sets one of plan step 3's two fields in a stored source's meta.yaml,
sources/ecfr/title-12/<section>/meta.yaml, run from the corpus root (R1).
<section> is 1024.41, 1024.30 or 1024.31. meta.yaml is rewritten through
internal/meta, its one writer: the field named is set and every other field
is kept as it is. Setting the value a field already has is a no-op. Every
other field is written by the command that computes it (fetch, and the
commands of later plan steps), never by meta set.

  version_effective_date YYYY-MM-DD
        the effective date of the change that produced this version
  version_fr_citations "<vol> FR <page>" ...
        replaces the list with the citations given, in that order. Each must
        be cited in the citation note (<CITA>) of the raw XML meta.yaml
        describes: raw/<source_version_date>.xml, with sha256_raw's bytes.

Exit 0: set, or already that value. Exit 1: refused by what is stored: a
citation the note doesn't cite; no meta.yaml (run regcorpus fetch first), or
one not in the spec's schema; the raw file missing, or not sha256_raw's
bytes; a symlink or a file in place of meta.yaml, the raw file or a
directory on the way. Nothing is written. Exit 2: a usage error (no such
section, any other field, a value not of its field's type), no
sources/ecfr/title-12/ (not the corpus root), or a file that can't be read
or written.
`

// The fields meta set writes (plan step 3; issue #5).
const (
	keyEffective = "version_effective_date"
	keyCitations = "version_fr_citations"
)

// MetaMain runs `regcorpus meta` with args (excluding the command name) in the
// current directory and returns the exit code: 0 ok, 1 refused by what is
// stored, 2 usage or a file it can't read or write.
func MetaMain(args []string, stdout, stderr io.Writer) int {
	return metaMain(".", args, stdout, stderr)
}

func metaMain(root string, args []string, stdout, stderr io.Writer) int {
	usageError := func(err error) int {
		fmt.Fprintf(stderr, "regcorpus meta: %v\n%s", err, metaUsage)
		return 2
	}
	isHelp := func(a string) bool { return a == "-h" || a == "-help" || a == "--help" }
	switch {
	case len(args) > 0 && isHelp(args[0]), len(args) > 1 && args[0] == "set" && isHelp(args[1]):
		fmt.Fprint(stdout, metaUsage)
		return 0
	case len(args) == 0:
		return usageError(errors.New("no subcommand"))
	case args[0] != "set":
		return usageError(fmt.Errorf("unknown subcommand %q", args[0]))
	case len(args) < 3:
		return usageError(errors.New("set takes a section, a field and its value"))
	}
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") {
			return usageError(fmt.Errorf("%q: meta set takes no flags", a))
		}
	}
	s, ok := lookup(args[1])
	if !ok {
		fmt.Fprintf(stderr, "regcorpus meta: unknown section %q (known: %s)\n", args[1], strings.Join(names(), ", "))
		return 2
	}
	st, err := parseSetting(args[2], args[3:])
	if err != nil {
		return usageError(err)
	}
	if err := checkRoot(root, "meta set"); err != nil {
		fmt.Fprintf(stderr, "regcorpus meta: %v\n", err)
		return exitCode(err)
	}
	line, err := st.apply(root, s)
	if err != nil {
		fmt.Fprintf(stderr, "regcorpus meta: %s: %v\n", s.Name, err)
		return exitCode(err)
	}
	fmt.Fprintln(stdout, s.Name+": "+line)
	return 0
}

// setting is one meta set: the field, and set, which puts its value in m
// after checking it against what is stored for s under root.
type setting struct {
	key string
	set func(root string, s Source, m *meta.Meta) error
}

// parseSetting reads the field and the values given for it. Only step 3's
// two fields are set here; a value not of its field's type is an error.
func parseSetting(key string, vals []string) (setting, error) {
	switch key {
	case keyEffective:
		if len(vals) != 1 {
			return setting{}, fmt.Errorf("%s takes one date, YYYY-MM-DD; got %d values", key, len(vals))
		}
		d, err := meta.ParseDate(vals[0])
		if err != nil {
			return setting{}, fmt.Errorf("%s: %w", key, err)
		}
		return setting{key, func(_ string, _ Source, m *meta.Meta) error {
			m.VersionEffectiveDate = d
			return nil
		}}, nil
	case keyCitations:
		if len(vals) == 0 {
			return setting{}, fmt.Errorf(`%s takes one or more citations, "<vol> FR <page>"`, key)
		}
		cites := make([]meta.Citation, len(vals))
		for i, v := range vals {
			c, err := meta.ParseCitation(v)
			if err != nil {
				return setting{}, fmt.Errorf("%s: %w", key, err)
			}
			cites[i] = c
		}
		return setting{key, func(root string, s Source, m *meta.Meta) error {
			if err := cited(root, s, *m, cites); err != nil {
				return err
			}
			m.VersionFRCitations = cites
			return nil
		}}, nil
	}
	if slices.Contains(meta.Fields(), key) {
		return setting{}, fmt.Errorf("meta set sets only %s and %s, not %s: every other field is written by the command that computes it", keyEffective, keyCitations, key)
	}
	return setting{}, fmt.Errorf("unknown field %q; meta set sets %s or %s", key, keyEffective, keyCitations)
}

// apply sets st's field in s's meta.yaml under root, keeping every other
// field, and returns the line to print: "wrote" or, when meta.yaml already
// says it, "unchanged", with the field's line. Only a meta.yaml fetch wrote,
// in the spec's schema, is changed (readMeta); anything else is a
// *conflictError and nothing is written.
func (st setting) apply(root string, s Source) (string, error) {
	metaPath := path.Join(s.dir(), "meta.yaml")
	have, m, err := readMeta(root, s)
	if err != nil {
		return "", err
	}
	if m == nil {
		return "", &conflictError{fmt.Sprintf("%s does not exist: fetch the source first (regcorpus fetch %s); meta set changes only a meta.yaml fetch wrote (R1)", metaPath, s.Name)}
	}
	if err := st.set(root, s, m); err != nil {
		return "", err
	}
	b, err := meta.Marshal(*m)
	if err != nil {
		return "", err
	}
	line := fieldLine(b, st.key)
	if bytes.Equal(b, have) {
		return "unchanged " + metaPath + ": " + line, nil
	}
	if err := replace(root, metaPath, b); err != nil {
		return "", err
	}
	return "wrote " + metaPath + ": " + line, nil
}

// fieldLine is key's line in b, a meta.yaml Marshal wrote.
func fieldLine(b []byte, key string) string {
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, key+": ") {
			return l
		}
	}
	return key + " absent"
}

// cited returns a *conflictError unless each of cites is cited in the
// citation note (<CITA>) of the raw XML m describes: raw/<source_version_date>.xml
// under s's directory, a regular file through real directories, with
// sha256_raw's bytes. So version_fr_citations is checkable against L0
// itself (issue #6). A file it can't read is an error as is (exit 2).
func cited(root string, s Source, m meta.Meta, cites []meta.Citation) error {
	raw := path.Join(s.dir(), "raw", string(m.SourceVersionDate)+".xml")
	if err := realDirs(root, path.Dir(raw)); err != nil {
		return err
	}
	full := filepath.Join(root, filepath.FromSlash(raw))
	info, err := os.Lstat(full)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return &conflictError{fmt.Sprintf("%s, the raw file meta.yaml describes, does not exist: restore it from git (git restore %s) (R1)", raw, raw)}
	case err != nil:
		return err
	case !info.Mode().IsRegular():
		return &conflictError{fmt.Sprintf("%s is not a regular file; raw XML is read only as one (R1)", raw)}
	}
	body, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	if sum := meta.SHA256(sha256Hex(body)); sum != m.SHA256Raw {
		return &conflictError{fmt.Sprintf("%s has sha256 %s, but meta.yaml says sha256_raw %s: it is not the raw file meta.yaml describes (R1)", raw, sum, m.SHA256Raw)}
	}
	note, err := citationNote(body)
	if err != nil {
		return &conflictError{fmt.Sprintf("%s: %v", raw, err)}
	}
	var missing []string
	for _, c := range cites {
		if !slices.Contains(note, c) {
			missing = append(missing, strconv.Quote(string(c)))
		}
	}
	if len(missing) == 0 {
		return nil
	}
	verb := "is"
	if len(missing) > 1 {
		verb = "are"
	}
	which := "no Federal Register citation"
	if len(note) > 0 {
		which = joinCitations(note)
	}
	return &conflictError{fmt.Sprintf("%s %s not cited in the citation note (<CITA>) of %s, the raw file meta.yaml describes, which cites %s; %s records only citations that note cites (R1, issue #6)",
		strings.Join(missing, ", "), verb, raw, which, keyCitations)}
}

// frCitation is a Federal Register citation in running text, "<vol> FR
// <page>", whatever whitespace separates its parts.
var frCitation = regexp.MustCompile(`\b([0-9]+)\s+FR\s+([0-9]+)\b`)

// citationNote returns the "<vol> FR <page>" citations in the text of the
// citation notes (<CITA> elements) of body, eCFR XML, in order, each once.
// Text elsewhere (an XREF's amendment link, the rule text) is not read.
func citationNote(body []byte) ([]meta.Citation, error) {
	d := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(body, bom)))
	d.Strict = true
	var note strings.Builder
	depth := 0 // > 0 inside a CITA element
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("not well-formed XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth > 0 || t.Name.Local == "CITA" {
				depth++
			}
		case xml.EndElement:
			if depth > 0 {
				if depth--; depth == 0 {
					note.WriteString("\n")
				}
			}
		case xml.CharData:
			if depth > 0 {
				note.Write(t)
			}
		}
	}
	var out []meta.Citation
	for _, m := range frCitation.FindAllStringSubmatch(note.String(), -1) {
		if c := meta.Citation(m[1] + " FR " + m[2]); !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out, nil
}

func joinCitations(cs []meta.Citation) string {
	s := make([]string, len(cs))
	for i, c := range cs {
		s[i] = string(c)
	}
	return strings.Join(s, ", ")
}
