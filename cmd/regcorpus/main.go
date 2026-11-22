// Command regcorpus is the corpus CLI (decision 6: CLI only; R5: the gate
// needs a binary). Plan step 1 builds two subcommands: check (C6, R5) and
// verify (R2a, terminal requirement only). The rest of the plan's command list
// (fetch, inventory, upgrade, release, show, text, topics, version) arrives in
// later plan steps.
package main

import (
	"fmt"
	"os"

	"github.com/okja-engineering/reg-corpus/internal/check"
	"github.com/okja-engineering/reg-corpus/internal/verify"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run dispatches args and returns the exit code: the command's own, 0 for
// help, 2 for a missing or unknown command. verify needs the real stdin and
// stdout files to test them for a terminal.
func run(args []string, stdin, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "check":
		return check.Main(args[1:], stdout, stderr)
	case "verify":
		return verify.Main(args[1:], stdin, stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "regcorpus: unknown command %q\n%s", args[0], usage)
		return 2
	}
}

const usage = `usage: regcorpus <command> [flags]

commands:
  check --corpus [--root DIR] [--tree TREE-ISH]
        run the per-commit corpus check (C6)
  verify <ID>
        move a staged record into registry/ (a person, in a terminal; R2a);
        not implemented yet (plan step 19)

"regcorpus check -h" shows check's flags. Exit codes: 0 ok, 1 check
violations, 2 usage errors or a corpus that can't be read.
`
