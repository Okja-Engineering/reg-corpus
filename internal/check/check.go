package check

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"path"
	"slices"
	"strings"
)

// Violation is one failed rule at one path. String renders the line the
// command prints, e.g.
// "C6 registry-state: registry/records/RX-41-TEST-01.yaml: no matching verify.log line".
type Violation struct {
	Rule string // e.g. "C6 registry-state"
	Path string // slash path relative to the corpus root
	Msg  string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s: %s: %s", v.Rule, v.Path, v.Msg)
}

const (
	ruleRegistryState = "C6 registry-state"
	recordsDir        = "registry/records"
	verifyLog         = "registry/verify.log"
)

// Violations runs the per-commit C6 rules built so far and returns one
// violation per failing path, sorted by path.
//
// Registry state: every entry under registry/ is a readable regular file or
// directory; every entry in registry/records/ is an RX-*.yaml file with a
// verify.log line whose ID is the file stem and whose sha256 is the file's
// bytes. A missing or empty registry passes.
func (s *Snapshot) Violations() []Violation {
	logged, logMissing := s.verifyLog()
	var out []Violation
	for _, p := range slices.Sorted(maps.Keys(s.registry)) {
		e := s.registry[p]
		fail := func(msg string) { out = append(out, Violation{Rule: ruleRegistryState, Path: p, Msg: msg}) }
		switch {
		case e.err != nil:
			fail("cannot read: " + e.err.Error())
		case e.kind != regular && e.kind != directory:
			fail("is a " + e.kind.String() + "; registry/ holds only regular files")
		case path.Dir(p) == recordsDir:
			id, ok := strings.CutSuffix(path.Base(p), ".yaml")
			if e.kind != regular || !ok || !strings.HasPrefix(id, "RX-") {
				fail("registry/records/ holds only RX-*.yaml record files")
				break
			}
			h := sha256.Sum256(e.body)
			got := hex.EncodeToString(h[:])
			hashes := logged[id]
			switch {
			case logMissing:
				fail("no matching verify.log line (registry/verify.log is missing)")
			case len(hashes) == 0:
				fail("no matching verify.log line")
			case !slices.Contains(hashes, got):
				fail(fmt.Sprintf("sha256 mismatch: file is %s; verify.log has %s for %s",
					got, strings.Join(hashes, ", "), id))
			}
		}
	}
	return out
}

// verifyLog maps record ID to every sha256 logged for it, from the fields
// "ID, rev, record sha256, by, on, method". Lines without exactly six fields
// are not matches and are skipped. missing reports that verify.log is absent;
// a verify.log that is not a readable regular file logs nothing.
func (s *Snapshot) verifyLog() (logged map[string][]string, missing bool) {
	e, ok := s.registry[verifyLog]
	if !ok {
		return nil, true
	}
	logged = map[string][]string{}
	if e.kind != regular || e.err != nil {
		return logged, false
	}
	for _, line := range strings.Split(string(e.body), "\n") {
		fields := strings.Split(line, ",")
		if len(fields) != 6 {
			continue
		}
		id := strings.TrimSpace(fields[0])
		logged[id] = append(logged[id], strings.ToLower(strings.TrimSpace(fields[2])))
	}
	return logged, false
}

const usage = `usage: regcorpus check --corpus [--root DIR] [--tree TREE-ISH]

Runs the per-commit corpus check (C6, R5). Exit 0: clean ("C6 ok").
Exit 1: violations, one line each on stdout. Exit 2: a usage error, or the
corpus can't be read (DIR is not a corpus root, or TREE-ISH can't be read).

  --corpus          run the corpus check (consumer checks C1-C5: plan step 22)
  --root DIR        the corpus root (default "."); with --tree, any
                    directory in its git repository
  --tree TREE-ISH   check this git tree's committed bytes instead of the
                    working tree; the pre-commit hook passes $(git write-tree)
`

// Main runs `regcorpus check` with args (excluding the subcommand name) and
// returns the exit code: 0 clean, 1 violations, 2 usage or unreadable corpus.
func Main(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("regcorpus check", flag.ContinueOnError)
	fl.SetOutput(io.Discard)
	corpus := fl.Bool("corpus", false, "")
	root := fl.String("root", ".", "")
	tree := fl.String("tree", "", "")
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprintf(stderr, "regcorpus check: %v\n%s", err, usage)
		return 2
	}
	if fl.NArg() != 0 {
		fmt.Fprintf(stderr, "regcorpus check: unexpected argument %q\n%s", fl.Arg(0), usage)
		return 2
	}
	if !*corpus {
		fmt.Fprintln(stderr, "regcorpus check: only --corpus exists; consumer checks C1–C5 arrive in plan step 22")
		return 2
	}
	var s *Snapshot
	var err error
	what := *root
	if *tree != "" {
		what = "tree " + *tree
		s, err = LoadTree(*root, *tree)
	} else {
		s, err = LoadDir(*root)
	}
	if err != nil {
		fmt.Fprintf(stderr, "regcorpus check: %v\n", err)
		return 2
	}
	if why := s.notCorpus(); why != "" {
		fmt.Fprintf(stderr, "regcorpus check: %s is not a corpus root (%s)\n", what, why)
		return 2
	}
	vs := s.Violations()
	if len(vs) == 0 {
		fmt.Fprintln(stdout, "C6 ok")
		return 0
	}
	for _, v := range vs {
		fmt.Fprintln(stdout, v)
	}
	return 1
}
