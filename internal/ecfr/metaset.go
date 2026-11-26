package ecfr

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/parse"
	"github.com/okja-engineering/reg-corpus/internal/sink"
)

const metaUsage = `usage: regcorpus meta set <section> version_effective_date YYYY-MM-DD
       regcorpus meta set <section> version_fr_citations "<vol> FR <page>" ...

Sets one of plan step 3's two fields in a stored source's meta.yaml,
sources/ecfr/title-12/<section>/meta.yaml, run from the corpus root (R1).
<section> is 1024.41, 1024.30 or 1024.31. meta.yaml is rewritten through
internal/meta and internal/l0, under the source's lock: the field named is
set and every other field is kept as it is. Setting the value a field
already has is a no-op. Every other field is written by the command that
computes it (fetch, and the commands of later plan steps), never by meta set.

Before setting either field, meta set checks what is stored, as fetch does:
meta.yaml is in the spec's schema and names the section as the source table
does (cfr, tier, role, source, official), and raw/ holds exactly
raw/<source_version_date>.xml, a regular file with sha256_raw's bytes.

  version_effective_date YYYY-MM-DD
        the effective date of the change that produced this version, as
        given; only its form is checked
  version_fr_citations "<vol> FR <page>" ...
        replaces the list with the citations given, in that order, each once.
        Each must be cited in a citation note (<CITA>) of that raw XML. This
        checks only that the note cites it, not which amendment it is.

Exit 0: set, or already that value. Exit 1: refused by what is stored: no
meta.yaml (run regcorpus fetch first), or one not in the spec's schema or
naming another source; raw/ not holding exactly the raw file meta.yaml
describes, with its bytes; a symlink or a file in place of meta.yaml, the
raw file or a directory on the way; a name that differs only in case from
meta.yaml, tree.json, text.md or overrides.yaml, or from a directory on the
way; when setting citations, raw XML that
is not well-formed (the form fetch checks before storing a body), or a
citation the note doesn't cite. Nothing is written.
Exit 2: a usage error (no such section, any other field, a flag, a value not
of its field's type, a citation given twice), no sources/ecfr/title-12/ (not
the corpus root), a file that can't be read or written, or a platform
without the lock (darwin and linux have it).
`

// The fields meta set writes (plan step 3). The command is the chief of
// staff's choice for setting them through the one writer (ADR 0010).
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
		fmt.Fprintf(stderr, "regcorpus meta: %v\n", err)
		sink.Print(stderr, metaUsage)
		return 2
	}
	isHelp := func(a string) bool { return a == "-h" || a == "-help" || a == "--help" }
	switch {
	case len(args) > 0 && isHelp(args[0]), len(args) > 1 && args[0] == "set" && isHelp(args[1]):
		sink.Print(stdout, metaUsage)
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
	s, ok := sourceNamed(args[1])
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
// after checking it against raw, the raw XML m describes, at slash path
// rawPath.
type setting struct {
	key string
	set func(m *meta.Meta, raw []byte, rawPath string) error
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
		return setting{key, func(m *meta.Meta, _ []byte, _ string) error {
			m.VersionEffectiveDate = d
			return nil
		}}, nil
	case keyCitations:
		cites, err := meta.ParseCitations(vals)
		if err != nil {
			return setting{}, fmt.Errorf(`%s takes one or more citations, "<vol> FR <page>", each once: %w`, key, err)
		}
		return setting{key, func(m *meta.Meta, raw []byte, rawPath string) error {
			if err := cited(raw, rawPath, cites); err != nil {
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

// apply sets st's field in s's meta.yaml under root through l0.Update, which
// checks what is stored first and keeps every other field, and returns the
// line to print: "wrote" or, when meta.yaml already says it, "unchanged",
// with the field's line.
func (st setting) apply(root string, s Source) (string, error) {
	b, w, err := l0.Update(root, s.stored(), func(m *meta.Meta, raw []byte, _ l0.State) (*l0.Derived, error) {
		return nil, st.set(m, raw, s.stored().RawPath(m.SourceVersionDate))
	})
	if err != nil {
		return "", err
	}
	verb := "unchanged "
	if w.Meta {
		verb = "wrote "
	}
	return verb + s.stored().MetaPath() + ": " + fieldLine(b, st.key), nil
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

// cited returns a *l0.Conflict unless each of cites is cited in a citation
// note (<CITA>) of raw, the raw XML at rawPath that meta.yaml describes
// (l0 has checked it is). The chief of staff's choice (ADR 0011): a value
// that must be in the stored L0 can't be mistyped or taken from elsewhere,
// and anyone can re-check it there. It checks membership only: that the note
// cites the page, not that it is the amendment that produced the version.
func cited(raw []byte, rawPath string, cites []meta.Citation) error {
	note, err := citationNote(raw)
	if err != nil {
		return l0.Conflictf("%s: %v", rawPath, err)
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
	return l0.Conflictf("%s %s not cited in the citation note (<CITA>) of %s, the raw file meta.yaml describes, which cites %s; %s records only citations that note cites (R1, ADR 0011)",
		strings.Join(missing, ", "), verb, rawPath, which, keyCitations)
}

// space is the whitespace between a citation's parts: Go's \s, which holds
// the line breaks, and the no-break space.
const space = `[\s\x{A0}]`

// frCitation is a Federal Register citation in a note's text, "<vol> FR
// <page>", its parts separated by space.
var frCitation = regexp.MustCompile(`\b([0-9]+)` + space + `+FR` + space + `+([0-9]+)`)

// shorthand is one shorthand page eCFR writes after a citation (", <page>",
// as in "78 FR 63004, 63005"). It is looked for only between a citation and
// the next, so the volume of a citation that follows ("78 FR 6856, 81 FR
// 72370") is never read as a page.
var shorthand = regexp.MustCompile(`^,` + space + `*([0-9]+)`)

// citationNote returns the "<vol> FR <page>" citations of body's citation
// notes (<CITA> elements, eCFR XML), in order, each once. A citation lies
// within one run of a note's text: text in two notes, or on either side of
// an element inside a note, never joins into one. Text outside the notes
// (an XREF's amendment link, the rule text) is not read. If body fails
// parse.WellFormed, the check of form fetch makes before storing a body, that
// is the error, and no citation is read from it.
func citationNote(body []byte) ([]meta.Citation, error) {
	if err := parse.WellFormed(body, nil); err != nil {
		return nil, err
	}
	d := xml.NewDecoder(bytes.NewReader(body)) // a byte-order mark is text outside every note
	d.Strict = true
	var runs []string
	var run strings.Builder
	depth := 0 // > 0 inside a CITA element
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		// Not reached: WellFormed has decoded body after one leading BOM, if
		// any, and this decoder reads that BOM as text (U+FEFF, a legal XML
		// character), then the same bytes.
		if err != nil {
			return nil, fmt.Errorf("not well-formed XML: %w", err)
		}
		if t, ok := tok.(xml.CharData); ok {
			if depth > 0 {
				run.Write(t)
			}
			continue
		}
		if run.Len() > 0 { // any other token ends a run of text
			runs = append(runs, run.String())
			run.Reset()
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth > 0 || t.Name.Local == "CITA" {
				depth++
			}
		case xml.EndElement:
			if depth > 0 {
				depth--
			}
		}
	}
	var out []meta.Citation
	for _, r := range runs {
		ms := frCitation.FindAllStringSubmatchIndex(r, -1)
		for i, m := range ms {
			pages := []string{r[m[4]:m[5]]}
			next := len(r)
			if i+1 < len(ms) {
				next = ms[i+1][0]
			}
			for rest := r[m[1]:next]; ; {
				p := shorthand.FindStringSubmatchIndex(rest)
				if p == nil {
					break
				}
				pages, rest = append(pages, rest[p[2]:p[3]]), rest[p[1]:]
			}
			for _, page := range pages {
				if c := meta.Citation(r[m[2]:m[3]] + " FR " + page); !slices.Contains(out, c) {
					out = append(out, c)
				}
			}
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
