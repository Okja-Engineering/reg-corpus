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

const usage = `usage: regcorpus fetch [--date YYYY-MM-DD] [section ...]

Fetches each named section (default: all of 1024.41, 1024.30, 1024.31) from
the eCFR versioner API, run from the corpus root (R1). Flags go before the
sections.

Without --date: stores eCFR's latest version, untouched, at
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
sources/ down and under .cache/: fetch goes only through real directories.

Every fetch, --date's too, refuses (exit 2) while eCFR reports an import in
progress, and any date after the one eCFR is up to date as of.

  --date YYYY-MM-DD   fetch that point in time into .cache/ecfr/title-12/
                      (git-ignored, exploratory); never into sources/

Exit 0: ok. Exit 1: a conflict with a stored file, or a symlink or a file in
place of a directory. Exit 2: a usage error, no sources/ecfr/title-12/ (not
the corpus root), a network or eCFR error, a response that isn't the
section's XML, or a local file that can't be read or written. Each section is fetched whatever
another's outcome; when they fail differently the larger code wins (2 over 1).
`

// fetcher runs `regcorpus fetch` in a corpus root.
type fetcher struct {
	root   string
	client *Client
	now    func() time.Time
}

// Main runs `regcorpus fetch` with args (excluding the subcommand name) in the
// current directory and returns the exit code: 0 ok, 1 a conflict with a
// stored file, 2 usage, network, eCFR or local file errors; the larger code
// when sections fail differently.
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
	code := 0
	for _, s := range srcs {
		var lines []string
		if at == "" {
			lines, err = f.latest(ctx, s, asOf)
		} else {
			lines, err = f.pointInTime(ctx, s, at)
		}
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

// exitCode is 1 for a conflict with a stored file, 2 for anything else.
func exitCode(err error) int {
	var c *l0.Conflict
	if errors.As(err, &c) {
		return 1
	}
	return 2
}
