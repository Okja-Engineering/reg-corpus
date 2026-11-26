// Tests for the regcorpus command line (decision 6: CLI only; R5: the gate
// needs a binary): dispatch, help, usage errors, stdout vs stderr, and the
// exit codes 0/1/2 as a built binary returns them.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/okja-engineering/reg-corpus/internal/anchors"
	"github.com/okja-engineering/reg-corpus/internal/check"
	"github.com/okja-engineering/reg-corpus/internal/ecfr"
	"github.com/okja-engineering/reg-corpus/internal/meta"
	"github.com/okja-engineering/reg-corpus/internal/verify"
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

// TestUsageGivesEveryCommandsExitCodes: AGENTS.md says `regcorpus help` and
// each command's -h give the exit codes, so help names where check's,
// fetch's, meta's and parse's are and states verify's, which has no -h of its
// own (R10).
func TestUsageGivesEveryCommandsExitCodes(t *testing.T) {
	in, out, errf := files(t)
	run([]string{"help"}, in, out, errf)
	u := read(t, out)
	for _, want := range []string{
		`"regcorpus check -h", "regcorpus fetch -h", "regcorpus meta -h",`,
		`"regcorpus parse -h" and "regcorpus text -h" show their flags and their`,
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
		"text [--json] <anchor>",
		"verify <ID>",
		"not implemented yet (plan step 19)",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("usage %q lacks %q", u, want)
		}
	}
	for _, stale := range []string{"plan step 6", "plan step 7"} {
		if strings.Contains(u, stale) {
			t.Errorf("usage %q still says %s builds what it built", u, stale)
		}
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
		"an anchor text can't print (no such anchor, or one a later plan step builds)",
		"an argument that names no source, not the corpus root,",
		"a stdout that can't be written",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("usage lacks %q", want)
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
		{"text help", []string{"text", "-h"}, 0, "usage: regcorpus text [--json] <anchor>\n", ""},
		{"text no anchor", []string{"text"}, 2, "", "regcorpus text: no anchor"},
		{"text not a corpus", []string{"text", "1024-41-b-2-i-B"}, 2, "", "regcorpus text: . is not a corpus root"},
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

// forged is the line fetch --verify writes on a match: a payload that put
// it at the start of a line of the saved output would forge the verdict.
const forged = `1024.41: wrote sources/ecfr/title-12/1024.41/meta.yaml: verified_on: "2026-11-25"`

// carried is a payload as a command writes it (raw) and as the stream must
// hold it (escaped).
type carried struct{ raw, escaped string }

var (
	// argPayload, given as an argument, would forge a line (an LF, then
	// forged) and move the terminal (an escape sequence, a line separator,
	// and 0x9b, a byte that isn't UTF-8 and a CSI to a terminal reading C1).
	// The sink writes it escaped.
	argPayload = carried{
		"x\n" + forged + "\x1b[2K\u2028\x9b",
		`x\x0a` + forged + `\x1b[2K\u2028\x9b`,
	}
	// quotedPayload is argPayload as a command quotes it (%q) before the
	// sink, which writes it byte for byte.
	quotedPayload = carried{
		`x\n` + strings.ReplaceAll(forged, `"`, `\"`) + `\x1b[2K\u2028\x9b`,
		`x\n` + strings.ReplaceAll(forged, `"`, `\"`) + `\x1b[2K\u2028\x9b`,
	}
	// namePayload is an XML name holding a line separator and a C1 control,
	// which encoding/xml's message for raw XML that isn't well-formed holds
	// as it is (ADR 0014, round 2). The sink writes it escaped.
	namePayload = carried{"HEAD\u2028\u0085x", `HEAD\u2028\u0085x`}
)

// storedSource returns a maker of corpus roots, each holding 1024.41's
// committed meta.yaml and raw XML (read, never written), the raw XML as
// edit returns it, if edit is given, and sha256_raw its sha256.
func storedSource(t *testing.T, edit func(string) string) func(*testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.FromSlash("../../sources/ecfr/title-12/1024.41"))
	if err != nil {
		t.Fatal(err)
	}
	raws, err := filepath.Glob(filepath.Join(dir, "raw", "*.xml"))
	if err != nil || len(raws) != 1 {
		t.Fatalf("1024.41's raw XML: %q, %v; want one file", raws, err)
	}
	raw, err := os.ReadFile(raws[0])
	if err != nil {
		t.Fatal(err)
	}
	m, err := os.ReadFile(filepath.Join(dir, "meta.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sum := func(b string) string {
		h := sha256.Sum256([]byte(b))
		return hex.EncodeToString(h[:])
	}
	body := string(raw)
	if edit != nil {
		if body = edit(body); body == string(raw) {
			t.Fatal("the edit changes nothing in 1024.41's raw XML")
		}
	}
	if !strings.Contains(string(m), sum(string(raw))) {
		t.Fatal("1024.41's meta.yaml doesn't hold its raw XML's sha256")
	}
	meta := strings.Replace(string(m), sum(string(raw)), sum(body), 1)
	return func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		src := filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.41"))
		if err := os.MkdirAll(filepath.Join(src, "raw"), 0o755); err != nil {
			t.Fatal(err)
		}
		for name, b := range map[string]string{"meta.yaml": meta, filepath.Join("raw", filepath.Base(raws[0])): body} {
			if err := os.WriteFile(filepath.Join(src, name), []byte(b), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
}

// derivedSource returns a maker of corpus roots, each a storedSource(t,
// edit) root beside what regcorpus parse derives from its raw XML: its
// tree.json and text.md, and sha256_text in its meta.yaml, so text prints
// its anchors.
func derivedSource(t *testing.T, edit func(string) string) func(*testing.T) string {
	t.Helper()
	stored := storedSource(t, edit)
	return func(t *testing.T) string {
		t.Helper()
		root := stored(t)
		src := filepath.Join(root, filepath.FromSlash("sources/ecfr/title-12/1024.41"))
		raws, err := filepath.Glob(filepath.Join(src, "raw", "*.xml"))
		if err != nil || len(raws) != 1 {
			t.Fatalf("the raw XML: %q, %v; want one file", raws, err)
		}
		raw, err := os.ReadFile(raws[0])
		if err != nil {
			t.Fatal(err)
		}
		d, err := anchors.Build(raw)
		if err != nil {
			t.Fatal(err)
		}
		mb, err := os.ReadFile(filepath.Join(src, "meta.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		m, err := meta.Parse(mb)
		if err != nil {
			t.Fatal(err)
		}
		m.SHA256Text = d.SHA256Text
		if mb, err = meta.Marshal(m); err != nil {
			t.Fatal(err)
		}
		for name, b := range map[string][]byte{"tree.json": d.TreeJSON, "text.md": d.TextMD, "meta.yaml": mb} {
			if err := os.WriteFile(filepath.Join(src, name), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
}

// TestEveryCommandWritesThroughTheSink: for every command run dispatches,
// and for run's own lines, each stream holds byte for byte what the command
// writes, a line per Write, through the sink (worklist round 4, §1). So a
// help text, and a usage error's line and its usage, keep their lines, and
// a payload that reaches a command's output unquoted reaches the stream
// escaped, on its line. A row's want is what the same command writes to
// plain buffers, its payload escaped; a row whose payload doesn't reach
// those buffers fails, so no row passes vacuously.
//
// No payload can reach stdout from check (a Violation quotes its path and
// message), meta or parse (their lines are a source's name, its paths, a
// sha256 and a value checked to its field's form) or, offline, fetch (its
// stdout lines come only after eCFR answers, at a fixed URL). text's can: it
// prints an anchor's own text as regcorpus parse derives it from the raw
// XML, which may hold DEL and a C1 control (XML holds them; parse refuses
// a line break, and XML holds no other C0 control but a tab), so a row
// puts the payload in the raw XML, its files derived from it (textPayload).
// So for each
// command in commands one row puts a fake in its place, which writes the
// payload to stdout and stderr: run's wiring is pinned there. verify writes
// only fixed lines, run quotes an unknown command, and a raw file in place
// of a sink writes a help text and those lines as the sink does: the
// mutants that give them one (round 3's D9, D10 and D11) are equivalent.
func TestEveryCommandWritesThroughTheSink(t *testing.T) {
	// direct is the function each command name runs, as this test reads
	// the dispatch.
	direct := map[string]func(args []string, stdout, stderr io.Writer) int{
		"check": check.Main,
		"fetch": ecfr.Main,
		"meta":  ecfr.MetaMain,
		"parse": ecfr.ParseMain,
		"text":  ecfr.TextMain,
	}
	fake := func(_ []string, stdout, stderr io.Writer) int {
		fmt.Fprintln(stdout, argPayload.raw)
		fmt.Fprintln(stderr, argPayload.raw)
		return 3
	}
	help := func(stdout, _ io.Writer) int { io.WriteString(stdout, usage); return 0 }
	unknown := func(name string) func(stdout, stderr io.Writer) int {
		return func(_, stderr io.Writer) int {
			fmt.Fprintf(stderr, "regcorpus: unknown command %q\n", name)
			io.WriteString(stderr, usage)
			return 2
		}
	}
	stored := storedSource(t, nil)
	badName := storedSource(t, func(raw string) string {
		return strings.Replace(raw, "<HEAD>", "<"+namePayload.raw+">", 1)
	})
	notRoot := filepath.Join(t.TempDir(), argPayload.raw)
	// textPayload is what of an own text could move a terminal that the
	// raw XML can hold: a tab, DEL, and the C1 control sequence
	// introducer, U+009B, then "2K" (erase the line), each control written
	// from its code point.
	textPayload := carried{
		"\t\x7f" + string(rune(0x9b)) + "2K",
		`\x09\x7f` + `\` + "u009b" + "2K",
	}
	// jsonPayload is what of textPayload reaches the sink raw with --json:
	// U+009B, which encoding/json leaves as it is, and the sink writes as
	// \u009b, a JSON escape of itself; JSON escapes the rest.
	jsonPayload := carried{string(rune(0x9b)), `\` + "u009b"}
	tree := derivedSource(t, nil)
	payloadTree := derivedSource(t, func(raw string) string {
		return strings.Replace(raw, "(B) Notify the borrower", "(B) Notify"+textPayload.raw+" the borrower", 1)
	})
	faked := map[string]bool{}
	for _, tc := range []struct {
		name string
		args []string
		root func(*testing.T) string // a corpus root to run in, made afresh for each run
		fake bool                    // the command is replaced by fake
		// own is what run itself writes, for a row whose command isn't in
		// direct.
		own            func(stdout, stderr io.Writer) int
		stdout, stderr carried // the payload each stream carries, if any
		lines          bool    // the command writes stdout lines: none fails
	}{
		{name: "help", args: []string{"help"}, own: help},
		{name: "-h", args: []string{"-h"}, own: help},
		{name: "--help", args: []string{"--help"}, own: help},
		{name: "no command", own: func(_, stderr io.Writer) int { io.WriteString(stderr, usage); return 2 }},
		{name: "an unknown command", args: []string{"bogus"}, own: unknown("bogus")},
		{name: "a command not built yet", args: []string{"inventory"}, own: unknown("inventory")},
		{name: "an unknown command, quoted", args: []string{argPayload.raw}, own: unknown(argPayload.raw), stderr: quotedPayload},
		{name: "verify without a terminal", args: []string{"verify", "RX-41-TEST-01"},
			own: func(_, stderr io.Writer) int { fmt.Fprintln(stderr, verify.ErrNoTerminal); return 2 }},

		{name: "check -h", args: []string{"check", "-h"}, lines: true},
		{name: "check: a flag it doesn't know", args: []string{"check", "--bogus"}},
		{name: "check: an argument", args: []string{"check", "--corpus", "extra"}},
		{name: "check: no --corpus", args: []string{"check"}},
		{name: "check: a clean corpus", args: []string{"check", "--corpus", "--root", corpus(t, false)}, lines: true},
		{name: "check: a violation", args: []string{"check", "--corpus", "--root", corpus(t, true)}, lines: true},
		{name: "check: a payload flag", args: []string{"check", "-" + argPayload.raw}, stderr: argPayload},
		{name: "check: a payload root", args: []string{"check", "--corpus", "--root", notRoot}, stderr: argPayload},
		{name: "check: the sinks", args: []string{"check"}, fake: true, stdout: argPayload, stderr: argPayload},

		{name: "fetch -h", args: []string{"fetch", "-h"}, lines: true},
		{name: "fetch: a flag it doesn't know", args: []string{"fetch", "--bogus"}},
		{name: "fetch: a flag after a section", args: []string{"fetch", "1024.41", "--verify"}},
		{name: "fetch: two modes", args: []string{"fetch", "--verify", "--latest"}},
		{name: "fetch: an unknown section", args: []string{"fetch", "1024.99"}},
		{name: "fetch: a payload flag", args: []string{"fetch", "-" + argPayload.raw}, stderr: argPayload},
		{name: "fetch: a payload date", args: []string{"fetch", "--date", argPayload.raw}, stderr: quotedPayload},
		{name: "fetch: the sinks", args: []string{"fetch"}, fake: true, stdout: argPayload, stderr: argPayload},

		{name: "meta -h", args: []string{"meta", "-h"}, lines: true},
		{name: "meta set -h", args: []string{"meta", "set", "-h"}, lines: true},
		{name: "meta: no subcommand", args: []string{"meta"}},
		{name: "meta: an unknown subcommand", args: []string{"meta", "get"}},
		{name: "meta: set alone", args: []string{"meta", "set"}},
		{name: "meta: a value not a date", args: []string{"meta", "set", "1024.41", "version_effective_date", "2025-7-15"}},
		{name: "meta: an unknown section", args: []string{"meta", "set", "1024.99", "version_effective_date", "2025-07-15"}},
		{name: "meta: a payload flag", args: []string{"meta", "set", "-" + argPayload.raw, "x"}, stderr: quotedPayload},
		{name: "meta: a stored field", args: []string{"meta", "set", "1024.41", "version_effective_date", "2025-07-15"}, root: stored, lines: true},
		{name: "meta: a payload name in the raw XML", args: []string{"meta", "set", "1024.41", "version_fr_citations", "90 FR 20792"},
			root: badName, stderr: namePayload},
		{name: "meta: the sinks", args: []string{"meta"}, fake: true, stdout: argPayload, stderr: argPayload},

		{name: "parse -h", args: []string{"parse", "-h"}, lines: true},
		{name: "parse: no section", args: []string{"parse"}},
		{name: "parse: a flag", args: []string{"parse", "--all"}},
		{name: "parse: an unknown section", args: []string{"parse", "1024.99"}},
		{name: "parse: a payload flag", args: []string{"parse", "-" + argPayload.raw}, stderr: quotedPayload},
		{name: "parse: a stored source", args: []string{"parse", "1024.41"}, root: stored, lines: true},
		{name: "parse: a payload name in the raw XML", args: []string{"parse", "1024.41"}, root: badName, stderr: namePayload},
		{name: "parse: the sinks", args: []string{"parse"}, fake: true, stdout: argPayload, stderr: argPayload},

		{name: "text -h", args: []string{"text", "-h"}, lines: true},
		{name: "text: no anchor", args: []string{"text"}},
		{name: "text: two anchors", args: []string{"text", "1024-41-a", "1024-41-b"}},
		{name: "text: a flag it doesn't know", args: []string{"text", "--bogus", "1024-41-a"}},
		{name: "text: a payload flag", args: []string{"text", "-" + argPayload.raw, "1024-41-a"}, stderr: argPayload},
		{name: "text: not the corpus root", args: []string{"text", "1024-41-b-2-i-B"}},
		{name: "text: a payload anchor", args: []string{"text", argPayload.raw}, root: tree, stderr: quotedPayload},
		{name: "text: an anchor", args: []string{"text", "1024-41-b-2-i-B"}, root: tree, lines: true},
		{name: "text: --json", args: []string{"text", "--json", "1024-41-b-2"}, root: tree, lines: true},
		{name: "text: no such anchor", args: []string{"text", "1024-41-z"}, root: tree},
		{name: "text: a later step's anchor", args: []string{"text", "1024-31-day"}, root: tree},
		{name: "text: a payload in the stored text", args: []string{"text", "1024-41-b-2-i-B"}, root: payloadTree,
			stdout: textPayload, lines: true},
		{name: "text: a payload in the stored text, --json", args: []string{"text", "--json", "1024-41-b-2-i-B"}, root: payloadTree,
			stdout: jsonPayload, lines: true},
		{name: "text: the sinks", args: []string{"text"}, fake: true, stdout: argPayload, stderr: argPayload},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.own
			switch {
			case tc.fake:
				name := tc.args[0]
				orig, ok := commands[name]
				if !ok {
					t.Fatalf("run doesn't call %s from commands, with the sinks", name)
				}
				commands[name] = fake
				t.Cleanup(func() { commands[name] = orig })
				faked[name] = true
				want = func(stdout, stderr io.Writer) int { return fake(tc.args[1:], stdout, stderr) }
			case want == nil:
				cmd, ok := direct[tc.args[0]]
				if !ok {
					t.Fatalf("no function for %q", tc.args)
				}
				want = func(stdout, stderr io.Writer) int { return cmd(tc.args[1:], stdout, stderr) }
			}
			if tc.root != nil {
				t.Chdir(tc.root(t))
			}
			in, out, errf := files(t)
			code := run(tc.args, in, out, errf)
			if tc.root != nil { // afresh: the run may have written in its root
				t.Chdir(tc.root(t))
			}
			var stdout, stderr strings.Builder
			wantCode := want(&stdout, &stderr)
			if code != wantCode {
				t.Errorf("exit %d; want %d", code, wantCode)
			}
			if tc.lines && strings.Count(stdout.String(), "\n") == 0 {
				t.Fatalf("the command writes no stdout line: the row tests nothing")
			}
			for _, s := range []struct {
				name, got, want string
				payload         carried
			}{
				{"stdout", read(t, out), stdout.String(), tc.stdout},
				{"stderr", read(t, errf), stderr.String(), tc.stderr},
			} {
				if s.payload != (carried{}) {
					if !strings.Contains(s.want, s.payload.raw) {
						t.Fatalf("%s: the payload %q doesn't reach the command's output %q", s.name, s.payload.raw, s.want)
					}
					s.want = strings.ReplaceAll(s.want, s.payload.raw, s.payload.escaped)
				}
				if s.got != s.want {
					t.Errorf("%s:\n%q\nwant, a line per line the command writes:\n%q", s.name, s.got, s.want)
				}
				if at := rawControl(s.got); at != "" {
					t.Errorf("%s holds %s, unescaped", s.name, at)
				}
				for _, l := range strings.Split(s.got, "\n") {
					if strings.HasPrefix(l, forged) {
						t.Errorf("%s has a forged line %q", s.name, l)
					}
				}
			}
		})
	}
	for name := range commands {
		if !faked[name] {
			t.Errorf("no row puts a fake in place of %s: run's wiring of it is not pinned", name)
		}
	}
}

// TestAStdoutThatCantBeWrittenExits2: exit 0 means the output was written
// (round 1 of PR #11, §3). run decides it once, for every command and its
// own help, from the stdout sink's first error: a stdout that can't be
// written exits 2, whatever the command's own code, with a line on stderr
// naming the failure; nothing else changes.
func TestAStdoutThatCantBeWrittenExits2(t *testing.T) {
	tree := derivedSource(t, nil)
	for _, tc := range []struct {
		name string
		args []string
		root func(*testing.T) string
		code int // with a stdout that can be written
	}{
		{"help", []string{"help"}, nil, 0},
		{"text -h", []string{"text", "-h"}, nil, 0},
		{"text", []string{"text", "1024-41-b-2-i-B"}, tree, 0},
		{"text --json", []string{"text", "--json", "1024-41-b-2"}, tree, 0},
		{"check, clean", []string{"check", "--corpus", "--root", corpus(t, false)}, nil, 0},
		{"check, a violation", []string{"check", "--corpus", "--root", corpus(t, true)}, nil, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.root != nil {
				t.Chdir(tc.root(t))
			}
			in, out, errf := files(t)
			if code := run(tc.args, in, out, errf); code != tc.code || read(t, out) == "" {
				t.Fatalf("a stdout that can be written: exit %d, stdout %q; want %d and output", code, read(t, out), tc.code)
			}
			in, out, errf = files(t)
			out.Close()
			code := run(tc.args, in, out, errf)
			stderr := read(t, errf)
			if code != 2 || !strings.Contains(stderr, "regcorpus: writing stdout: ") {
				t.Errorf("stdout closed: exit %d, stderr %q; want 2 and regcorpus: writing stdout", code, stderr)
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
		{"bad check flag: one message, ours", []string{"check", "--bogus"}, 2, "",
			"regcorpus check: flag provided but not defined: -bogus\nusage: regcorpus check"},
		{"help", []string{"--help"}, 0, usageLine, ""},
		{"fetch unknown section", []string{"fetch", "1024.99"}, 2, "", `regcorpus fetch: unknown section "1024.99"`},
		{"meta unknown section", []string{"meta", "set", "1024.99", "version_effective_date", "2025-07-15"}, 2, "",
			`regcorpus meta: unknown section "1024.99"`},
		{"parse unknown section", []string{"parse", "1024.99"}, 2, "", `regcorpus parse: unknown section "1024.99"`},
		{"text no anchor", []string{"text"}, 2, "", "regcorpus text: no anchor"},
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
	// A stdout the binary can't write (a file opened read-only, so each
	// write fails) exits 2, not 0, naming it on stderr (round 1 of PR #11,
	// §3). A closed stdout (>&-) can't be tested so: Go's runtime opens
	// /dev/null in its place at start, and writes there succeed.
	for _, args := range [][]string{{"help"}, {"text", "-h"}} {
		t.Run("stdout read-only: "+strings.Join(args, " "), func(t *testing.T) {
			cmd := exec.Command(bin, args...)
			cmd.Stdin = devnull
			cmd.Stdout = devnull
			var stderr strings.Builder
			cmd.Stderr = &stderr
			var exit *exec.ExitError
			if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.HasPrefix(stderr.String(), "regcorpus: writing stdout: ") {
				t.Errorf("%v, stderr %q; want exit 2 and regcorpus: writing stdout", err, stderr.String())
			}
		})
	}
}
