package ecfr

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const usage = `usage: regcorpus fetch [--date YYYY-MM-DD] [section ...]

Fetches each named section (default: all of 1024.41, 1024.30, 1024.31) from
the eCFR versioner API, run from the corpus root (R1).

Without --date: stores eCFR's latest version, untouched, at
sources/ecfr/title-12/<section>/raw/<date>.xml, and writes meta.yaml's
fetch-owned fields beside it. A stored file is never overwritten: identical
bytes are a no-op, different bytes exit 1. meta.yaml is written only when
it is missing; one that sits beside no raw file for the latest version exits 1.

  --date YYYY-MM-DD   fetch that point in time into .cache/ecfr/title-12/
                      (git-ignored, exploratory); never into sources/

Exit 0: ok. Exit 1: a stored file a fetch would change. Exit 2: a usage
error, a network or eCFR error, or a response that isn't valid UTF-8 XML.
`

// fetcher runs `regcorpus fetch` in a corpus root.
type fetcher struct {
	root   string
	client *Client
	now    func() time.Time
}

// Main runs `regcorpus fetch` with args (excluding the subcommand name) in the
// current directory and returns the exit code: 0 ok, 1 a conflict with a
// stored file, 2 usage, network or unreadable input.
func Main(args []string, stdout, stderr io.Writer) int {
	return fetcher{root: ".", client: NewClient(), now: time.Now}.main(args, stdout, stderr)
}

func (f fetcher) main(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("regcorpus fetch", flag.ContinueOnError)
	fl.SetOutput(io.Discard)
	dateFlag := fl.String("date", "", "")
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprintf(stderr, "regcorpus fetch: %v\n%s", err, usage)
		return 2
	}
	srcs, err := selectSources(fl.Args())
	if err != nil {
		fmt.Fprintf(stderr, "regcorpus fetch: %v\n", err)
		return 2
	}
	var at Date
	if flagSet(fl, "date") { // an empty --date is an error, never "latest"
		if at, err = ParseDate(*dateFlag); err != nil {
			fmt.Fprintf(stderr, "regcorpus fetch: --date %v\n", err)
			return 2
		}
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
// when that is missing, and returns the lines to print. A meta.yaml beside no
// raw file for the latest version describes another version; fetch leaves it
// and fails rather than replace it.
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
	raw := path.Join(s.dir(), "raw", string(v.Date)+".xml")
	metaPath := path.Join(s.dir(), "meta.yaml")
	rawExists, err := f.exists(raw)
	if err != nil {
		return nil, err
	}
	metaExists, err := f.exists(metaPath)
	if err != nil {
		return nil, err
	}
	if metaExists && !rawExists {
		return nil, &conflictError{fmt.Sprintf("%s exists but %s, eCFR's latest version, is not stored: meta.yaml describes another version, and fetch never replaces it (R1)",
			metaPath, raw)}
	}
	prefix := s.Name + " " + string(v.Date) + ": "
	var lines []string
	stored, err := storeOnce(f.root, raw, body)
	if err != nil {
		return nil, err
	}
	lines = append(lines, prefix+storedLine(stored, raw, body))
	if metaExists {
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

// exists reports whether slash path p under the root exists.
func (f fetcher) exists(p string) (bool, error) {
	_, err := os.Lstat(filepath.Join(f.root, filepath.FromSlash(p)))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
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

func flagSet(fl *flag.FlagSet, name string) bool {
	set := false
	fl.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}

// exitCode is 1 for a conflict with a stored file, 2 for anything else.
func exitCode(err error) int {
	var c *conflictError
	if errors.As(err, &c) {
		return 1
	}
	return 2
}
