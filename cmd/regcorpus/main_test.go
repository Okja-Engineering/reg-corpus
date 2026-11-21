// Tests for the regcorpus command line (decision 6: CLI only; R5: the gate
// needs a binary): dispatch, help, usage errors, stdout vs stderr, and the
// exit codes 0/1/2 as a built binary returns them.
package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func TestUsageListsCommandsAndWhatIsNotBuilt(t *testing.T) {
	in, out, errf := files(t)
	run([]string{"help"}, in, out, errf)
	u := read(t, out)
	for _, want := range []string{
		"check --corpus [--root DIR] [--tree TREE-ISH]",
		"verify <ID>",
		"not implemented yet (plan step 19)",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("usage %q lacks %q", u, want)
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
		{[]string{"fetch"}, `regcorpus: unknown command "fetch"`},
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
		{"help", []string{"--help"}, 0, usageLine, ""},
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
