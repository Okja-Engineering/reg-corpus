// `regcorpus check` command surface and the C6 registry-state rule (C6, R2, R5).
package check

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
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

// Corpus runs the per-commit C6 rules built so far against the corpus at root.
// It returns the violations in a stable order; the error is for I/O failures
// that stop the check from running at all.
func Corpus(root string) ([]Violation, error) {
	return registryState(root)
}

// registryState checks that every entry in registry/records/ is an RX-*.yaml
// file with a verify.log line whose ID is the file stem and whose sha256 is
// the file's bytes. A missing or empty registry passes.
func registryState(root string) ([]Violation, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(recordsDir)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	logged, logMissing, err := readVerifyLog(root)
	if err != nil {
		return nil, err
	}

	var out []Violation
	for _, e := range entries { // os.ReadDir sorts by name
		rel := path.Join(recordsDir, e.Name())
		fail := func(msg string) {
			out = append(out, Violation{Rule: ruleRegistryState, Path: rel, Msg: msg})
		}
		id, ok := strings.CutSuffix(e.Name(), ".yaml")
		if !ok || !strings.HasPrefix(id, "RX-") || !e.Type().IsRegular() {
			fail("registry/records/ holds only RX-*.yaml record files")
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		h := sha256.Sum256(body)
		got := hex.EncodeToString(h[:])
		hashes := logged[id]
		switch {
		case logMissing:
			fail("no matching verify.log line (registry/verify.log is missing)")
		case len(hashes) == 0:
			fail("no matching verify.log line")
		case !contains(hashes, got):
			fail(fmt.Sprintf("sha256 mismatch: file is %s; verify.log has %s for %s",
				got, strings.Join(hashes, ", "), id))
		}
	}
	return out, nil
}

// readVerifyLog maps record ID to every sha256 logged for it. Lines without
// the six fields are not matches and are skipped.
func readVerifyLog(root string) (map[string][]string, bool, error) {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(verifyLog)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	logged := map[string][]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), ",")
		if len(fields) != 6 {
			continue
		}
		id := strings.TrimSpace(fields[0])
		logged[id] = append(logged[id], strings.ToLower(strings.TrimSpace(fields[2])))
	}
	return logged, false, sc.Err()
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// Main runs `regcorpus check` with args (excluding the subcommand name) and
// returns the exit code: 0 clean, 1 violations or a failed run, 2 usage.
func Main(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("regcorpus check", flag.ContinueOnError)
	fl.SetOutput(stderr)
	corpus := fl.Bool("corpus", false, "run the corpus check C6 (per-commit rules)")
	root := fl.String("root", ".", "corpus root directory")
	fl.Usage = func() {
		fmt.Fprintln(stderr, "usage: regcorpus check --corpus [--root DIR]")
		fl.PrintDefaults()
	}
	if err := fl.Parse(args); err != nil {
		return 2
	}
	if fl.NArg() != 0 {
		fl.Usage()
		return 2
	}
	if !*corpus {
		fmt.Fprintln(stderr, "regcorpus check: only --corpus exists; consumer checks C1–C5 arrive in plan step 22")
		return 2
	}
	vs, err := Corpus(*root)
	if err != nil {
		fmt.Fprintf(stderr, "regcorpus check: %v\n", err)
		return 1
	}
	if len(vs) == 0 {
		fmt.Fprintln(stdout, "C6 ok")
		return 0
	}
	for _, v := range vs {
		fmt.Fprintln(stdout, v)
	}
	return 1
}
