package ecfr

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/okja-engineering/reg-corpus/internal/l0"
	"github.com/okja-engineering/reg-corpus/internal/meta"
)

const usage = `usage: regcorpus fetch [--date YYYY-MM-DD | --verify | --latest] [section ...]

Fetches each named section (default: all of 1024.41, 1024.30, 1024.31) from
the eCFR versioner API, run from the corpus root (R1). Flags go before the
sections; --date, --verify and --latest exclude each other.

Without a flag: stores eCFR's latest version, untouched, at
sources/ecfr/title-12/<section>/raw/<date>.xml, and writes meta.yaml's
fetch-owned fields beside it when it is missing. A source keeps exactly one
entry in raw/, the latest version's file, and meta.yaml names its date and
sha256; a stored file is never overwritten. Identical bytes are a no-op, and
the fields other steps set in meta.yaml are kept as they are. Different
bytes, any other entry in raw/, or a meta.yaml that isn't in the spec's
schema, lacks its raw file, names another date or sha256, or differs from
the source table in cfr, tier, role, source or official exit 1 and store
nothing.
So does a symlink or a file where a directory on the way should be, from
sources/ down and under .cache/, and a name that differs only in case from
meta.yaml, tree.json, text.md or overrides.yaml in a source's directory,
from a directory on the way (RAW for raw/, say), or from the file it stores
(2025-07-15.XML for 2025-07-15.xml, --date's under .cache/ too): fetch goes
only through real directories, by the names git records, so what it
reports storing, or finding unchanged, is the name git records.

Every fetch, with any flag, refuses (exit 2) while eCFR reports an import
in progress; without a flag or with --date, also any date after the one
eCFR is up to date as of.

  --date YYYY-MM-DD   fetch that point in time into .cache/ecfr/title-12/
                      (git-ignored, exploratory); never into sources/
  --verify            compare each stored section with a fresh fetch and
                      the govinfo 2026 edition, and set verified_on (R1)
  --latest            check that eCFR lists no newer version of a stored
                      section (R1b); writes nothing

--verify (plan step 6, ADR 0014) checks each section in three steps, and
stops at the first that fails:
  1. What is stored: the check above every writer of meta.yaml makes, a
     sha256_text and a tree.json as regcorpus parse writes them, and no
     overrides.yaml (applying overrides isn't built yet, R1a; with none,
     it prints "overrides: none"). 1024.30 and 1024.31 are refused: their
     tree and sha256_text are plan step 9's. No request but titles.json
     is made before this passes.
  2. eCFR: its latest version must be dated the stored source_version_date
     (else run --latest); that version, fetched fresh and built as
     regcorpus parse builds it, must have tree.json's nodes (the same
     paths, in the same order, each with the same sha256 of its normalized
     own text) and the stored sha256_text. The normalized text is the
     version: bytes that differ but normalize alike pass, and both
     sha256_raw values, with the nodes whose raw text differs, are
     printed as information only.
  3. govinfo, only once step 2 passes: the section's granule XML in the
     CFR 2026 annual edition (title 12, revised as of 2026-01-01), asked of
     govinfo's link service, printed with its URL, DATE and sha256, and kept,
     git-ignored, at .cache/govinfo/2026/title-12/<section>/<sha256>.xml. It
     must be valid UTF-8, well-formed, its SECTNO the section and its DATE
     2026-01-01. Its layout is deleted (each line break and up to 12 spaces
     after it); its heading (SECTNO, one space, SUBJECT) and paragraphs
     (<E>'s text kept, page breaks dropped, the citation note left out),
     joined and normalized as sha256_text's are, must hash to the stored
     sha256_text.
It holds the source's lock throughout and writes only verified_on in
meta.yaml (through internal/l0), keeping every other field. On a match it
sets the run's UTC date, unless verified_on is a date already, which stays,
so a re-run that still matches changes nothing ("unchanged"). On a govinfo
mismatch it prints each differing paragraph, normalized, and writes
verified_on: null over any date. Raw XML, tree.json and text.md are never
written.

--latest (plan step 6, ADR 0014) reads each section's meta.yaml, checked as
above, and eCFR's version list, and prints the stored source_version_date,
eCFR's latest date and every date after the stored one ("(removed)" for a
version that removed the section). An entry for the stored date issued
after retrieved_at, a reissue, is printed as information and doesn't
change the exit code: --verify compares its text. R1b's Supplement I
clause, a fresh 41() slice's hash, is plan step 8's, which stores
Supplement I.

Exit 0: ok; with --verify, matched, verified_on set or kept; with --latest,
no newer version. Exit 1: a conflict with a stored file; a symlink or a
file in place of a directory; or a name that differs only in case from
meta.yaml, tree.json, text.md or overrides.yaml, from a directory on the
way, or from the file it stores. With --verify, also a finding or a
refusal, with nothing written: eCFR's latest version is another date or
removed the section, the fresh tree or sha256_text differs, or step 1
refuses; and a govinfo mismatch, which writes verified_on: null and exits 1.
With --latest, also a newer version, or a latest version that removed the
section; --latest writes nothing. Exit 2: a usage error, no
sources/ecfr/title-12/ (not the corpus root), a network, eCFR or govinfo
error (an HTTP error, eCFR mid-import, a response that isn't the section's
XML, a granule of another section or edition, or one --verify doesn't
read), a local file that can't be read or written, or, for --verify, a
platform without the lock (darwin and linux have it); --verify then writes
nothing under sources/. Each section is fetched whatever another's
outcome; when they fail differently the larger code wins (2 over 1).
`

// fetcher runs `regcorpus fetch` in a corpus root.
type fetcher struct {
	root   string
	client *Client
	now    func() time.Time
}

// Main runs `regcorpus fetch` with args (excluding the subcommand name) in the
// current directory and returns the exit code: 0 ok, 1 a conflict with a
// stored file or, with --verify or --latest, a finding, 2 usage, network,
// eCFR, govinfo or local file errors; the larger code when sections fail
// differently.
func Main(args []string, stdout, stderr io.Writer) int {
	return fetcher{root: ".", client: NewClient(), now: time.Now}.main(args, stdout, stderr)
}

func (f fetcher) main(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("regcorpus fetch", flag.ContinueOnError)
	fl.SetOutput(io.Discard)
	var at Date // set by --date; "" fetches the latest version into sources/
	fl.Func("date", "", func(s string) error {
		if at != "" {
			return errors.New("--date given more than once")
		}
		var err error
		at, err = ParseDate(s) // an empty --date is an error, never "latest"
		return err
	})
	verify := fl.Bool("verify", false, "")
	newer := fl.Bool("latest", false, "")
	usageError := func(err error) int {
		fmt.Fprintf(stderr, "regcorpus fetch: %v\n%s", err, usage)
		return 2
	}
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		return usageError(err)
	}
	for _, a := range fl.Args() { // flag parsing stops at the first section
		if strings.HasPrefix(a, "-") {
			return usageError(fmt.Errorf("%q comes after a section; flags go before the sections", a))
		}
	}
	modes := 0
	for _, on := range []bool{at != "", *verify, *newer} {
		if on {
			modes++
		}
	}
	if modes > 1 {
		return usageError(errors.New("--date, --verify and --latest exclude each other"))
	}
	srcs, err := selectSources(fl.Args())
	if err != nil {
		fmt.Fprintf(stderr, "regcorpus fetch: %v\n", err)
		return 2
	}
	if err := checkRoot(f.root, "fetch"); err != nil {
		fmt.Fprintf(stderr, "regcorpus fetch: %v\n", err)
		return exitCode(err)
	}

	// Every fetch, --date's too, asks first whether eCFR is up to date and not
	// mid-import, and fetches no date after the date it is up to date as of.
	ctx := context.Background()
	asOf, err := f.client.UpToDateAsOf(ctx)
	if err == nil && at > asOf {
		err = fmt.Errorf("--date %s is after %s, the date eCFR's title %s is up to date as of", at, asOf, title)
	}
	if err != nil {
		fmt.Fprintf(stderr, "regcorpus fetch: %v\n", err)
		return 2
	}
	// One mode for every section: what fetch does with each.
	each := func(ctx context.Context, s Source) ([]string, error) { return f.latest(ctx, s, asOf) }
	switch {
	case at != "":
		each = func(ctx context.Context, s Source) ([]string, error) { return f.pointInTime(ctx, s, at) }
	case *verify:
		each = f.verify
	case *newer:
		each = f.newer
	}
	code := 0
	for _, s := range srcs {
		lines, err := each(ctx, s)
		for _, l := range lines {
			fmt.Fprintln(stdout, l)
		}
		if err != nil {
			fmt.Fprintf(stderr, "regcorpus fetch: %s: %v\n", s.Name, err)
			code = max(code, exitCode(err))
		}
	}
	return code
}

// checkRoot returns nil when root is a corpus root: one with a real
// sources/ecfr/title-12/ directory. l0.RealDirs alone judges what is on the way
// (a symlink or a file: a *l0.Conflict, exit 1), here as under every
// source; then a root without it is not the corpus root (exit 2), and the
// error says to run cmd from there.
func checkRoot(root, cmd string) error {
	if err := l0.RealDirs(root, sourcesDir); err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(sourcesDir))); err != nil {
		return fmt.Errorf("%s is not a corpus root (no %s/ directory); run %s from the corpus root", root, sourcesDir, cmd)
	}
	return nil
}

// latest stores s's latest eCFR version under sources/, and its meta.yaml
// when that is missing, and returns the lines to print. It succeeds only when,
// after it, raw/ holds exactly one entry, a regular file named for the latest
// date with eCFR's bytes, and meta.yaml names that file's date and sha256. It
// reads s's directory once (l0.Read) and, before storing anything, refuses
// what contradicts that: another entry in raw/ (admits), and an existing
// meta.yaml that fails the check every writer of one makes (l0's Stored).
func (f fetcher) latest(ctx context.Context, s Source, asOf Date) ([]string, error) {
	vs, err := f.client.Versions(ctx, s)
	if err != nil {
		return nil, err
	}
	v, err := Latest(vs)
	if err != nil {
		return nil, err
	}
	if v.Date > asOf {
		return nil, fmt.Errorf("eCFR's latest version of %s is dated %s, after %s, the date title %s is up to date as of; fetch stores no version eCFR isn't up to date for",
			s.Name, v.Date, asOf, title)
	}
	body, err := f.client.XML(ctx, s, v.Date)
	if err != nil {
		return nil, err
	}
	name := string(v.Date) + ".xml"
	raw := path.Join(s.dir(), "raw", name)
	st, err := l0.Read(f.root, s.stored())
	if err != nil {
		return nil, err
	}
	if err := admits(s, st.Raw, name); err != nil {
		return nil, err
	}
	if st.Meta != nil {
		// Then raw/ holds raw/<source_version_date>.xml alone, and admits
		// leaves that raw/<name>: StoreOnce compares it with eCFR's bytes.
		if _, err := st.Stored(f.root, s.stored()); err != nil {
			return nil, err
		}
	}
	stored, err := l0.StoreOnce(f.root, raw, body)
	if err != nil {
		return nil, err
	}
	prefix := s.Name + " " + string(v.Date) + ": "
	lines := []string{prefix + storedLine(stored, raw, body)}
	if st.Meta != nil {
		return lines, nil
	}
	m, err := meta.Marshal(fetchedMeta(s, v.Date, Date(f.now().UTC().Format(time.DateOnly)), asOf, body))
	if err != nil {
		return lines, err
	}
	metaPath := s.stored().MetaPath()
	if _, err := l0.StoreOnce(f.root, metaPath, m); err != nil {
		return lines, err
	}
	return append(lines, prefix+"wrote "+metaPath), nil
}

// fetchedMeta is the meta.yaml fetch writes for s's raw file body, eCFR's
// version at date, retrieved on retrieved when eCFR was up to date as of
// asOf: s's identity (l0's Source.Identity) and the fields that exist at
// fetch time (spec "L0 meta.yaml"). The rest stay absent until the plan
// steps that own them set them (internal/meta).
func fetchedMeta(s Source, date, retrieved, asOf Date, body []byte) meta.Meta {
	m := s.stored().Identity()
	m.SourceVersionDate, m.RetrievedAt, m.ECFRUpToDateAsOf = date, retrieved, asOf
	m.SHA256Raw = meta.SHA256(l0.SHA256Hex(body))
	return m
}

// pointInTime stores s's XML at date under .cache/, never under sources/.
func (f fetcher) pointInTime(ctx context.Context, s Source, date Date) ([]string, error) {
	body, err := f.client.XML(ctx, s, date)
	if err != nil {
		return nil, err
	}
	p := path.Join(cacheDir, s.Dir, string(date)+".xml")
	stored, err := l0.StoreOnce(f.root, p, body)
	if err != nil {
		return nil, err
	}
	return []string{s.Name + " " + string(date) + ": " + storedLine(stored, p, body)}, nil
}

func storedLine(stored bool, p string, body []byte) string {
	if stored {
		return "stored " + p + " (sha256 " + l0.SHA256Hex(body) + ")"
	}
	return "unchanged " + p
}

// admits returns a *l0.Conflict unless storing raw/<name> can leave s with
// exactly one raw entry, the regular file raw/<name> (R1): every entry in
// raw/, whatever its name or type, counts.
func admits(s Source, raw []fs.DirEntry, name string) error {
	rawDir := path.Join(s.dir(), "raw") + "/"
	var others []string
	for _, e := range raw {
		switch {
		case e.Name() != name:
			others = append(others, rawDir+e.Name())
		case !e.Type().IsRegular():
			return l0.Conflictf("%s is not a regular file; raw XML is stored only as one (R1)", rawDir+name)
		}
	}
	if len(others) > 0 {
		return l0.Conflictf("%s holds %s, and eCFR's latest version is %s: fetch keeps one raw file per source, so it stores nothing (R1)",
			rawDir, strings.Join(others, ", "), name)
	}
	return nil
}

// selectSources returns the named sources in the order given, or every
// source when none is named.
func selectSources(args []string) ([]Source, error) {
	if len(args) == 0 {
		return sources, nil
	}
	out := make([]Source, 0, len(args))
	for _, a := range args {
		s, ok := lookup(a)
		if !ok {
			return nil, fmt.Errorf("unknown section %q (known: %s)", a, strings.Join(names(), ", "))
		}
		out = append(out, s)
	}
	return out, nil
}

// exitCode is 1 for a conflict with a stored file or a finding of --verify
// or --latest, 2 for anything else.
func exitCode(err error) int {
	var c *l0.Conflict
	var f *Finding
	if errors.As(err, &c) || errors.As(err, &f) {
		return 1
	}
	return 2
}
