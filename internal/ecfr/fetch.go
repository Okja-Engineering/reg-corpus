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
)

const usage = `usage: regcorpus fetch [--date YYYY-MM-DD] [section ...]

Fetches each named section (default: all of 1024.41, 1024.30, 1024.31) from
the eCFR versioner API, run from the corpus root (R1). Flags go before the
sections.

Without --date: stores eCFR's latest version, untouched, at
sources/ecfr/title-12/<section>/raw/<date>.xml, and writes meta.yaml's
fetch-owned fields beside it when it is missing. A source keeps one raw
file, the one its meta.yaml describes, and a stored file is never
overwritten: identical bytes are a no-op; different bytes, another raw file,
or a meta.yaml whose raw file is missing exit 1 and store nothing. Refuses
(exit 2) while eCFR reports an import in progress.

  --date YYYY-MM-DD   fetch that point in time into .cache/ecfr/title-12/
                      (git-ignored, exploratory); never into sources/

Exit 0: ok. Exit 1: a conflict with a stored file. Exit 2: a usage error, a
network or eCFR error, a response that isn't the section's XML, or a local
file that can't be read or written. Each section is fetched whatever
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
	if info, err := os.Stat(filepath.Join(f.root, filepath.FromSlash(sourcesDir))); err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "regcorpus fetch: %s is not a corpus root (no %s/ directory); run fetch from the corpus root\n", f.root, sourcesDir)
		return 2
	}

	ctx := context.Background()
	var asOf Date
	if at == "" {
		if asOf, err = f.client.UpToDateAsOf(ctx); err != nil {
			fmt.Fprintf(stderr, "regcorpus fetch: %v\n", err)
			return 2
		}
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

// latest stores s's latest eCFR version under sources/, and its meta.yaml
// when that is missing, and returns the lines to print. It reads what s's
// directory holds once (readState) and stores only what keeps it at one raw
// file, the one its meta.yaml describes (admits).
func (f fetcher) latest(ctx context.Context, s Source, asOf Date) ([]string, error) {
	vs, err := f.client.Versions(ctx, s)
	if err != nil {
		return nil, err
	}
	v, err := Latest(vs)
	if err != nil {
		return nil, err
	}
	body, err := f.client.XML(ctx, s, v.Date)
	if err != nil {
		return nil, err
	}
	name := string(v.Date) + ".xml"
	raw := path.Join(s.dir(), "raw", name)
	metaPath := path.Join(s.dir(), "meta.yaml")
	st, err := f.readState(s)
	if err != nil {
		return nil, err
	}
	if err := st.admits(s, name); err != nil {
		return nil, err
	}
	prefix := s.Name + " " + string(v.Date) + ": "
	stored, err := storeOnce(f.root, raw, body)
	if err != nil {
		return nil, err
	}
	lines := []string{prefix + storedLine(stored, raw, body)}
	if st.meta {
		return lines, nil
	}
	m := metaYAML(s, v.Date, Date(f.now().UTC().Format(time.DateOnly)), asOf, body)
	if _, err := storeOnce(f.root, metaPath, m); err != nil {
		return lines, err
	}
	return append(lines, prefix+"wrote "+metaPath), nil
}

// pointInTime stores s's XML at date under .cache/, never under sources/.
func (f fetcher) pointInTime(ctx context.Context, s Source, date Date) ([]string, error) {
	body, err := f.client.XML(ctx, s, date)
	if err != nil {
		return nil, err
	}
	p := path.Join(cacheDir, s.Dir, string(date)+".xml")
	stored, err := storeOnce(f.root, p, body)
	if err != nil {
		return nil, err
	}
	return []string{s.Name + " " + string(date) + ": " + storedLine(stored, p, body)}, nil
}

func storedLine(stored bool, p string, body []byte) string {
	if stored {
		return "stored " + p + " (sha256 " + sha256Hex(body) + ")"
	}
	return "unchanged " + p
}

// sourceState is what a source directory holds before fetch writes to it:
// the names of the XML files in raw/, and whether meta.yaml exists.
type sourceState struct {
	raws []string
	meta bool
}

// readState reads s's directory under the root, once.
func (f fetcher) readState(s Source) (sourceState, error) {
	var st sourceState
	dir := filepath.Join(f.root, filepath.FromSlash(s.dir()))
	ents, err := os.ReadDir(filepath.Join(dir, "raw"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return st, err
	}
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".xml") {
			st.raws = append(st.raws, e.Name())
		}
	}
	switch _, err := os.Lstat(filepath.Join(dir, "meta.yaml")); {
	case err == nil:
		st.meta = true
	case !errors.Is(err, fs.ErrNotExist):
		return st, err
	}
	return st, nil
}

// admits returns a *conflictError unless storing raw/<name> keeps s at one
// raw file, the one its meta.yaml describes (R1): raw/ may hold no other XML,
// and a meta.yaml needs its raw file. fetch doesn't read meta.yaml, so a
// meta.yaml beside no raw file at all is refused rather than guessed at.
func (st sourceState) admits(s Source, name string) error {
	rawDir := path.Join(s.dir(), "raw") + "/"
	var others []string
	for _, r := range st.raws {
		if r != name {
			others = append(others, rawDir+r)
		}
	}
	if len(others) > 0 {
		return &conflictError{fmt.Sprintf("%s holds %s, and eCFR's latest version is %s: fetch keeps one raw file per source, so it stores nothing (R1)",
			rawDir, strings.Join(others, ", "), name)}
	}
	if st.meta && len(st.raws) == 0 {
		return &conflictError{fmt.Sprintf("%s exists but %s holds no raw XML: the raw file it describes is missing. Restore it from git (git restore %s); fetch never stores a raw file beside an existing meta.yaml (R1)",
			path.Join(s.dir(), "meta.yaml"), rawDir, rawDir)}
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
	var c *conflictError
	if errors.As(err, &c) {
		return 1
	}
	return 2
}
