// Tests for the regcorpus command line (decision 6: CLI only; R5: the gate
// needs a binary): dispatch, help, usage errors, stdout vs stderr, and the
// exit codes 0/1/2 as a built binary returns them.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

const usageLine = "usage: regcorpus <command> [flags]"

// files returns stdin (a regular file: not a terminal) and stdout and stderr
// files whose contents read() returns.
func files(t *testing.T) (in, out, errf *os.File) {
	t.Helper()
	dir := t.TempDir()
	open := func(name string) *os.File {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		return f
	}
	return open("in"), open("out"), open("err")
}

func read(t *testing.T, f *os.File) string {
	t.Helper()
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// corpus returns a corpus root; with unlogged it holds a record that has no
// verify.log line.
func corpus(t *testing.T, unlogged bool) string {
	t.Helper()
	root := t.TempDir()
	fs := map[string]string{
		"go.mod":                            "module github.com/okja-engineering/reg-corpus\n",
		"stages/01-fetch-parse/CONTRACT.md": "x\n",
	}
	if unlogged {
		fs["registry/records/RX-41-TEST-01.yaml"] = "id: RX-41-TEST-01\n"
	}
	for p, body := range fs {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestHelpExitsZeroOnStdout(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"help"}} {
		in, out, errf := files(t)
		code := run(args, in, out, errf)
		stdout, stderr := read(t, out), read(t, errf)
		if code != 0 || !strings.HasPrefix(stdout, usageLine) || stderr != "" {
			t.Errorf("%q: exit %d, stdout %q, stderr %q; want 0 and usage on stdout", args, code, stdout, stderr)
		}
	}
}

// TestUsageGivesEveryCommandsExitCodes: AGENTS.md says `regcorpus help` and
// each command's -h give the exit codes, so help names where check's,
// fetch's, meta's and parse's are and states verify's, which has no -h of its
// own (R10).
func TestUsageGivesEveryCommandsExitCodes(t *testing.T) {
	in, out, errf := files(t)
	run([]string{"help"}, in, out, errf)
	u := read(t, out)
	for _, want := range []string{
		`"regcorpus check -h", "regcorpus fetch -h", "regcorpus meta -h" and`,
		`"regcorpus parse -h" show their flags and their exit codes`,
		"verify exits 2 without an interactive terminal and 1 until it is built",
		"A missing or unknown command exits 2.",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("usage %q lacks %q", u, want)
		}
	}
}

func TestUsageListsCommandsAndWhatIsNotBuilt(t *testing.T) {
	in, out, errf := files(t)
	run([]string{"help"}, in, out, errf)
	u := read(t, out)
	for _, want := range []string{
		"check --corpus [--root DIR] [--tree TREE-ISH]",
		"fetch [--date YYYY-MM-DD | --verify | --latest] [section ...]",
		"--verify: compare each stored section with a fresh fetch and the",
		"--latest: exit 1 unless eCFR's latest version of each stored",
		"meta set <section> version_effective_date YYYY-MM-DD",
		`meta set <section> version_fr_citations "<vol> FR <page>" ...`,
		"parse <section> ...",
		"verify <ID>",
		"not implemented yet (plan step 19)",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("usage %q lacks %q", u, want)
		}
	}
	if strings.Contains(u, "plan step 6") {
		t.Errorf("usage %q still says plan step 6 builds what it built", u)
	}
	// The exit summary names fetch's findings, the one exit 1 that writes,
	// and govinfo among what stops a command (worklist round 1, L3), read
	// with its line breaks as spaces.
	flat := strings.Join(strings.Fields(u), " ")
	for _, want := range []string{
		"it is newer, older, or removed the section",
		"a finding of fetch --verify or --latest",
		"the one exit 1 that writes is fetch --verify's govinfo mismatch, which writes verified_on: null",
		"2 anything that stops the command (usage, network, eCFR, govinfo,",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("usage lacks %q", want)
		}
	}
}

func TestUsageErrorsExitTwoOnStderr(t *testing.T) {
	for _, tc := range []struct {
		args []string
		msg  string
	}{
		{nil, ""},
		{[]string{"bogus"}, `regcorpus: unknown command "bogus"`},
		{[]string{"inventory"}, `regcorpus: unknown command "inventory"`},
	} {
		in, out, errf := files(t)
		code := run(tc.args, in, out, errf)
		stdout, stderr := read(t, out), read(t, errf)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, tc.msg) || !strings.Contains(stderr, usageLine) {
			t.Errorf("%q: exit %d, stdout %q, stderr %q; want 2, %q and usage on stderr", tc.args, code, stdout, stderr, tc.msg)
		}
	}
}

func TestDispatch(t *testing.T) {
	for _, tc := range []struct {
		name           string
		args           []string
		code           int
		stdout, stderr string // prefixes; "" means empty
	}{
		{"check clean corpus", []string{"check", "--corpus", "--root", corpus(t, false)}, 0, "C6 ok\n", ""},
		{"check violation", []string{"check", "--corpus", "--root", corpus(t, true)}, 1,
			"C6 registry-state: registry/records/RX-41-TEST-01.yaml: no matching verify.log line", ""},
		{"check not a corpus", []string{"check", "--corpus", "--root", t.TempDir()}, 2, "", "regcorpus check: "},
		{"check help", []string{"check", "-h"}, 0, "usage: regcorpus check", ""},
		{"fetch help", []string{"fetch", "-h"}, 0, "usage: regcorpus fetch", ""},
		{"fetch unknown section", []string{"fetch", "1024.99"}, 2, "", `regcorpus fetch: unknown section "1024.99"`},
		{"meta help", []string{"meta", "-h"}, 0, "usage: regcorpus meta set", ""},
		{"meta unknown section", []string{"meta", "set", "1024.99", "version_effective_date", "2025-07-15"}, 2, "",
			`regcorpus meta: unknown section "1024.99"`},
		{"parse help", []string{"parse", "-h"}, 0, "usage: regcorpus parse", ""},
		{"parse unknown section", []string{"parse", "1024.99"}, 2, "", `regcorpus parse: unknown section "1024.99"`},
		{"verify without a terminal", []string{"verify", "RX-41-TEST-01"}, 2, "",
			"regcorpus verify requires an interactive terminal (R2a); a person runs it\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, out, errf := files(t)
			code := run(tc.args, in, out, errf)
			stdout, stderr := read(t, out), read(t, errf)
			if code != tc.code || !prefixOrEmpty(stdout, tc.stdout) || !prefixOrEmpty(stderr, tc.stderr) {
				t.Fatalf("exit %d, stdout %q, stderr %q; want %d, %q, %q", code, stdout, stderr, tc.code, tc.stdout, tc.stderr)
			}
		})
	}
}

// rawControl says where s, as a stream holds it, has a character that
// could end a line or move a terminal: a C0 control but LF, which ends a
// line there, DEL, a C1 control, U+2028 or U+2029, or a byte that isn't
// UTF-8. It is "" when s has none.
func rawControl(s string) string {
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && n == 1 || r < 0x20 && r != '\n' || r == 0x7f || 0x80 <= r && r <= 0x9f || r == 0x2028 || r == 0x2029 {
			return fmt.Sprintf("%q at byte %d", s[i:i+n], i)
		}
		i += n
	}
	return ""
}

// TestEveryLineGoesThroughTheSink: run gives every command the sink for
// stdout and stderr (internal/sink), so whatever a command prints, from a
// flag it doesn't know to a path, no byte forges a line of the saved
// output or moves the terminal (worklist round 3, §1). Each payload holds
// an LF and a line of fetch's own, an escape sequence, a line separator
// (from its code point) and a byte that isn't UTF-8.
func TestEveryLineGoesThroughTheSink(t *testing.T) {
	const forged = `1024.41: wrote sources/ecfr/title-12/1024.41/meta.yaml: verified_on: "2026-11-25"`
	payload := "x\n" + forged + "\x1b[2K" + string(rune(0x2028)) + "\x9b"
	for _, args := range [][]string{
		{"-" + payload},
		{"check", "-" + payload},
		{"check", "--corpus", "--root", filepath.Join(t.TempDir(), payload)},
		{"fetch", "-" + payload},
		{"fetch", "--date", payload},
		{"meta", "-" + payload},
		{"meta", "set", "-" + payload},
		{"parse", "-" + payload},
		{"verify", "-" + payload},
	} {
		in, out, errf := files(t)
		code := run(args, in, out, errf)
		stdout, stderr := read(t, out), read(t, errf)
		for _, s := range []struct{ name, out string }{{"stdout", stdout}, {"stderr", stderr}} {
			if at := rawControl(s.out); at != "" {
				t.Errorf("%q: %s holds %s, unescaped: %q", args, s.name, at, s.out)
			}
			for _, l := range strings.Split(s.out, "\n") {
				if strings.HasPrefix(l, "1024.41: wrote") {
					t.Errorf("%q: %s has a forged line %q", args, s.name, l)
				}
			}
		}
		if code == 0 {
			t.Errorf("%q: exit 0; want a usage or root error", args)
		}
	}
}

func prefixOrEmpty(got, want string) bool {
	if want == "" {
		return got == ""
	}
	return strings.HasPrefix(got, want)
}

// TestBuiltBinaryExitCodes builds regcorpus and checks that the exit codes
// and streams reach the caller (the pre-commit hook runs a built binary).
func TestBuiltBinaryExitCodes(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go is required to build the binary: %v", err)
	}
	bin := filepath.Join(t.TempDir(), "regcorpus")
	if out, err := exec.Command(gobin, "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	for _, tc := range []struct {
		name           string
		args           []string
		code           int
		stdout, stderr string
	}{
		{"clean", []string{"check", "--corpus", "--root", corpus(t, false)}, 0, "C6 ok\n", ""},
		{"violation", []string{"check", "--corpus", "--root", corpus(t, true)}, 1, "C6 registry-state: ", ""},
		{"not a corpus", []string{"check", "--corpus", "--root", t.TempDir()}, 2, "", "regcorpus check: "},
		{"no command", nil, 2, "", usageLine},
		{"bad check flag: one message, ours", []string{"check", "--bogus"}, 2, "",
			"regcorpus check: flag provided but not defined: -bogus\nusage: regcorpus check"},
		{"help", []string{"--help"}, 0, usageLine, ""},
		{"fetch unknown section", []string{"fetch", "1024.99"}, 2, "", `regcorpus fetch: unknown section "1024.99"`},
		{"meta unknown section", []string{"meta", "set", "1024.99", "version_effective_date", "2025-07-15"}, 2, "",
			`regcorpus meta: unknown section "1024.99"`},
		{"parse unknown section", []string{"parse", "1024.99"}, 2, "", `regcorpus parse: unknown section "1024.99"`},
		{"verify < /dev/null", []string{"verify", "RX-41-TEST-01"}, 2, "", "regcorpus verify requires an interactive terminal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(bin, tc.args...)
			cmd.Stdin = devnull
			var stdout, stderr strings.Builder
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			code := 0
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				code = exit.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != tc.code || !prefixOrEmpty(stdout.String(), tc.stdout) || !prefixOrEmpty(stderr.String(), tc.stderr) {
				t.Fatalf("exit %d, stdout %q, stderr %q; want %d, %q, %q",
					code, stdout.String(), stderr.String(), tc.code, tc.stdout, tc.stderr)
			}
		})
	}
}
