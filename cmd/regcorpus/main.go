// Command regcorpus is the corpus CLI (decision 6: CLI only; R5: the gate
// needs a binary). Plan step 1 builds two subcommands: check (C6, R5) and
// verify (R2a, terminal requirement only). The rest of the plan's command list
// (fetch, inventory, upgrade, release, show, text, topics, version) arrives in
// later plan steps.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/okja-engineering/reg-corpus/internal/check"
	"github.com/okja-engineering/reg-corpus/internal/verify"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin, stdout, stderr *os.File) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "check":
		return check.Main(args[1:], stdout, stderr)
	case "verify":
		return verify.Main(args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "regcorpus: unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: regcorpus <command> [flags]

commands:
  check --corpus [--root DIR]   run the per-commit corpus check (C6)
  verify <ID>                   move a staged record into registry/ (a person, in a terminal; R2a)
`)
}
