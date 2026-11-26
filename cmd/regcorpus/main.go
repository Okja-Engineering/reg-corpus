// Command regcorpus is the corpus CLI (decision 6: CLI only; R5: the gate
// needs a binary). Plan step 1 builds two subcommands: check (C6, R5) and
// verify (R2a, terminal requirement only); plan step 2 adds fetch (R1); plan
// step 3 adds meta set (ADR 0010); plan step 5 adds parse (R1, R1a; ADR 0013);
// plan step 6 adds fetch --verify and --latest (R1, R1b; ADR 0014). The rest
// of the plan's command list (inventory, upgrade, release, show, text,
// topics, version) arrives in later plan steps. Every line it writes goes
// through internal/sink.
package main

import (
	"fmt"
	"os"

	"github.com/okja-engineering/reg-corpus/internal/check"
	"github.com/okja-engineering/reg-corpus/internal/ecfr"
	"github.com/okja-engineering/reg-corpus/internal/sink"
	"github.com/okja-engineering/reg-corpus/internal/verify"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run dispatches args and returns the exit code: the command's own, 0 for
// help, 2 for a missing or unknown command. Every command, and run itself,
// writes through the sink (internal/sink), never to stdout or stderr: the
// output is saved as evidence, and no byte in it may forge a line or move
// a terminal (worklist round 3, §1). verify is given the stdin and stdout
// files only to test them for a terminal; it writes to its stderr sink.
func run(args []string, stdin, stdoutFile, stderrFile *os.File) int {
	stdout, stderr := sink.New(stdoutFile), sink.New(stderrFile)
	if len(args) == 0 {
		sink.Print(stderr, usage)
		return 2
	}
	switch args[0] {
	case "check":
		return check.Main(args[1:], stdout, stderr)
	case "fetch":
		return ecfr.Main(args[1:], stdout, stderr)
	case "meta":
		return ecfr.MetaMain(args[1:], stdout, stderr)
	case "parse":
		return ecfr.ParseMain(args[1:], stdout, stderr)
	case "verify":
		return verify.Main(args[1:], stdin, stdoutFile, stderr)
	case "-h", "--help", "help":
		sink.Print(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "regcorpus: unknown command %q\n", args[0])
		sink.Print(stderr, usage)
		return 2
	}
}

const usage = `usage: regcorpus <command> [flags]

commands:
  check --corpus [--root DIR] [--tree TREE-ISH]
        run the per-commit corpus check (C6)
  fetch [--date YYYY-MM-DD | --verify | --latest] [section ...]
        store eCFR's latest raw XML for 1024.41, 1024.30, 1024.31 under
        sources/ (R1), or with --date a point-in-time copy under .cache/,
        from the corpus root;
        --verify: compare each stored section with a fresh fetch and the
        govinfo 2026 edition, and set verified_on in its meta.yaml (R1);
        --latest: exit 1 unless eCFR's latest version of each stored
        section is the stored one: it is newer, older, or removed the
        section (R1b), writing nothing
  meta set <section> version_effective_date YYYY-MM-DD
  meta set <section> version_fr_citations "<vol> FR <page>" ...
        set one of these two fields in a stored source's meta.yaml (R1),
        keeping the rest; each citation must be in the raw XML's <CITA>
  parse <section> ...
        write a stored source's tree.json and text.md and set sha256_text
        in its meta.yaml, offline, from its raw XML (R1, R1a)
  verify <ID>
        move a staged record into registry/ (a person, in a terminal; R2a);
        not implemented yet (plan step 19)

"regcorpus check -h", "regcorpus fetch -h", "regcorpus meta -h" and
"regcorpus parse -h" show their flags and their exit codes, which are the
authority. In short: 0 ok, 1 violations, a command a stored file refuses
(a conflict with it), or a finding of fetch --verify or --latest, which
writes nothing (the one exit 1 that writes is fetch --verify's govinfo
mismatch, which writes verified_on: null), 2 anything that stops the
command (usage, network, eCFR, govinfo, or a file it can't read or
write). A missing or unknown command exits 2.
verify exits 2 without an interactive terminal and 1 until it is built
(plan step 19).
`
