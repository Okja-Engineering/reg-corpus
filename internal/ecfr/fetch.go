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
	"strconv"
	"strings"
	"time"
)

const usage = `usage: regcorpus fetch [--date YYYY-MM-DD] [section ...]

Fetches each named section (default: all of 1024.41, 1024.30, 1024.31) from
the eCFR versioner API, run from the corpus root (R1). Flags go before the
sections.

Without --date: stores eCFR's latest version, untouched, at
sources/ecfr/title-12/<section>/raw/<date>.xml, and writes meta.yaml's
fetch-owned fields beside it when it is missing. A source keeps exactly one
entry in raw/, the latest version's file, and meta.yaml names its date and
sha256; a stored file is never overwritten. Identical bytes are a no-op.
Different bytes, any other entry in raw/, or a meta.yaml that isn't fetch's,
lacks its raw file or names another date or sha256 exit 1 and store nothing.

Every fetch, --date's too, refuses (exit 2) while eCFR reports an import in
progress, and any date after the one eCFR is up to date as of.

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

// latest stores s's latest eCFR version under sources/, and its meta.yaml
// when that is missing, and returns the lines to print. It succeeds only when,
// after it, raw/ holds exactly one entry, a regular file named for the latest
// date with eCFR's bytes, and meta.yaml names that file's date and sha256. It
// reads s's directory once (readState) and refuses what contradicts that
// before storing anything (admits), then checks an existing meta.yaml against
// the raw file (names).
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
	metaPath := path.Join(s.dir(), "meta.yaml")
	st, err := f.readState(s)
	if err != nil {
		return nil, err
	}
	if err := st.admits(s, name); err != nil {
		return nil, err
	}
	// With a meta.yaml, admits has seen raw/<name> stored: this compares it.
	stored, err := storeOnce(f.root, raw, body)
	if err != nil {
		return nil, err
	}
	prefix := s.Name + " " + string(v.Date) + ": "
	lines := []string{prefix + storedLine(stored, raw, body)}
	if st.meta != nil {
		if err := st.meta.names(metaPath, v.Date, body); err != nil {
			return nil, err
		}
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
// every entry in raw/, whatever its name or type, and what its meta.yaml says.
type sourceState struct {
	raw  []fs.DirEntry
	meta *storedMeta // nil when there is no meta.yaml
}

// storedMeta is what a stored meta.yaml says about the raw file it describes.
type storedMeta struct {
	date Date   // source_version_date
	sum  string // sha256_raw
}

// readState reads s's directory under the root, once. A meta.yaml that isn't
// a regular file in fetch's own format is a *conflictError: fetch never
// replaces one, and won't guess what it describes. A directory or file it
// can't read is an error as is (exit 2).
func (f fetcher) readState(s Source) (sourceState, error) {
	var st sourceState
	dir := filepath.Join(f.root, filepath.FromSlash(s.dir()))
	ents, err := os.ReadDir(filepath.Join(dir, "raw"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return st, err
	}
	st.raw = ents
	metaPath := path.Join(s.dir(), "meta.yaml")
	full := filepath.Join(dir, "meta.yaml")
	info, err := os.Lstat(full)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return st, nil
	case err != nil:
		return st, err
	case !info.Mode().IsRegular():
		return st, &conflictError{fmt.Sprintf("%s is not a regular file; fetch reads and writes meta.yaml only as one (R1)", metaPath)}
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return st, err
	}
	m, err := parseMeta(b)
	if err != nil {
		return st, &conflictError{fmt.Sprintf("%s: %v. It is not a meta.yaml as fetch writes it, and fetch never replaces one (R1)", metaPath, err)}
	}
	st.meta = &m
	return st, nil
}

// parseMeta reads source_version_date and sha256_raw from b, a meta.yaml in
// the format metaYAML writes: one "key: value" per line, these two values
// double-quoted, each key once. Other lines are not read. The values aren't
// validated here: names compares them with the raw file's date and sha256.
func parseMeta(b []byte) (storedMeta, error) {
	vals := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		key, v, _ := strings.Cut(line, ": ")
		if key != "source_version_date" && key != "sha256_raw" {
			continue
		}
		if _, ok := vals[key]; ok {
			return storedMeta{}, fmt.Errorf("%s appears twice", key)
		}
		u, err := strconv.Unquote(v)
		if err != nil || !strings.HasPrefix(v, `"`) {
			return storedMeta{}, fmt.Errorf("%s %s is not a double-quoted string", key, v)
		}
		vals[key] = u
	}
	for _, key := range []string{"source_version_date", "sha256_raw"} {
		if _, ok := vals[key]; !ok {
			return storedMeta{}, fmt.Errorf("no %s", key)
		}
	}
	return storedMeta{date: Date(vals["source_version_date"]), sum: vals["sha256_raw"]}, nil
}

// admits returns a *conflictError unless storing raw/<name> can leave s with
// exactly one raw entry, the regular file raw/<name> (R1). Every entry in
// raw/ counts, whatever its name or type. And a meta.yaml needs its raw file:
// one beside an empty raw/ is refused rather than refilled.
func (st sourceState) admits(s Source, name string) error {
	rawDir := path.Join(s.dir(), "raw") + "/"
	var others []string
	for _, e := range st.raw {
		switch {
		case e.Name() != name:
			others = append(others, rawDir+e.Name())
		case !e.Type().IsRegular():
			return &conflictError{fmt.Sprintf("%s is not a regular file; raw XML is stored only as one (R1)", rawDir+name)}
		}
	}
	if len(others) > 0 {
		return &conflictError{fmt.Sprintf("%s holds %s, and eCFR's latest version is %s: fetch keeps one raw file per source, so it stores nothing (R1)",
			rawDir, strings.Join(others, ", "), name)}
	}
	if st.meta != nil && len(st.raw) == 0 {
		return &conflictError{fmt.Sprintf("%s exists but %s holds no raw XML: the raw file it describes is missing. Restore it from git (git restore %s); fetch never stores a raw file beside an existing meta.yaml (R1)",
			path.Join(s.dir(), "meta.yaml"), rawDir, rawDir)}
	}
	return nil
}

// names returns a *conflictError unless m, the meta.yaml at metaPath, names
// the raw file stored for date with bytes body: its date and its sha256.
func (m storedMeta) names(metaPath string, date Date, body []byte) error {
	if m.date != date {
		return &conflictError{fmt.Sprintf("%s says source_version_date %q, but the raw file stored is eCFR's latest, %s: it describes another version, and fetch never replaces it (R1)",
			metaPath, m.date, date)}
	}
	if sum := sha256Hex(body); m.sum != sum {
		return &conflictError{fmt.Sprintf("%s says sha256_raw %s, but the raw file stored for %s has sha256 %s: it describes other bytes, and fetch never replaces it (R1)",
			metaPath, m.sum, date, sum)}
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
