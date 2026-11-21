# reg-corpus: start here (R10)

This repo holds verbatim rule text (L0) for 12 CFR 1024.41, its Supplement I
41() comments and the cite-only 1024.30 and 1024.31, plus a person-verified
registry of records (L1) and the `regcorpus` CLI that checks them. This file
only routes you. It holds no rule content and no product decisions.

- Spec (authoritative): `docs/03-spec.md`. Plan: `docs/04-plan.md`.
  Intent: `docs/01-intent.md`. Requirement IDs (R1..R11, C1..C6) come from the spec.

## Where things live
- `stages/0N-*/CONTRACT.md`: one contract per build stage (see below).
- `sources/ecfr/title-12/`: L0. `1024.41/`, `supplement-i/41/`, and cite-only
  `1024.30/`, `1024.31/`. Each holds `raw/<date>.xml`, `tree.json`, `text.md`,
  `meta.yaml` and an optional reviewed `overrides.yaml`.
- `staging/`: records proposed by extraction, at `fetched`.
- `registry/`: `records/RX-*.yaml` (text-verified or above only), `verify.log`,
  `inventory/`, `topics.yaml`. Only a person changes it, through `regcorpus verify`.
- `prompts/`: versioned extraction prompts. `skill/regcorpus/`: the agent skill (R9).
- `cmd/regcorpus/`, `internal/*/`: the Go CLI. Each package's `doc.go` names its job.
- `eval/`: seeds, fixtures, judge prompt, runs, results, walk and guard transcripts (R7).
- `.githooks/pre-commit`: the per-commit C6 gate. `.claude/`: the agent guard (R11).

## Stages and their order
| Stage | Contract | Who acts |
|---|---|---|
| 01 fetch-parse | `stages/01-fetch-parse/CONTRACT.md` | person or agent; fetches run on the owner's machine |
| 02 extract | `stages/02-extract/CONTRACT.md` | agent proposes into `staging/` only |
| 03 verify | `stages/03-verify/CONTRACT.md` | a person only, in a terminal |
| 04 release | `stages/04-release/CONTRACT.md` | a person tags; only after the decision is accept |
| 05 evaluate | `stages/05-evaluate/CONTRACT.md` | a person launches runs and does the blind author checks |

Order (plan, decision-critical path):
1. 01 fetch-parse (L0 for all four sources).
2. 05 evaluate, first part: freeze `eval/fixture-src/` and the judge, run arm (a).
   This happens before any extraction.
3. 02 extract, then 03 verify, record by record.
4. 05 evaluate, second part: arms (b) and (c), grading, decision 5.
5. 04 release, only if the decision is accept.

To pick up work: open the contract for the task you were given (fetching or
parsing: 01; proposing records: 02; verifying a record: 03; tagging: 04; eval
runs or grading: 05), read its Inputs and Check, and act within "Who may run it".

## Hard rules
- No model edits, writes, moves or verifies anything under `registry/`. Only a
  person adds records there, through `regcorpus verify`.
- `regcorpus verify` is human-only and refuses to run without an interactive
  terminal. Never run it, wrap it (`script`, `expect`, `unbuffer`) or emulate it.
- Never bypass hooks: no `--no-verify`, no `git commit -n`, no changes to
  `core.hooksPath`, no edits under `.githooks/` or `.claude/`.
- L0 text is verbatim. A source defect is fixed only through a reviewed
  `overrides.yaml` entry (location, replacement, reason), never in parser code
  and never by editing stored text.
- No interpretations of the regulation and no product decisions anywhere in
  this repo. Records restate the text with verbatim quotes; that is all.
- CLI only: no server, API or MCP (decision 6).

## Make targets
- `make setup`: point git at `.githooks/` and make the hooks executable. Run once per clone.
- `make build`: build `./regcorpus`.
- `make test`: Go tests, then the R11 guard tests.
- `make check`: `regcorpus check --corpus` (per-commit C6, the same gate as pre-commit).
- `make help` (default): list targets.

`./regcorpus` with no arguments prints the commands that exist today. A contract
that names a command not built yet also names the plan step that builds it.

## Reporting at the end of a stage
1. Run the "Check that ends it" command from that stage's contract.
2. Report the stage, the exact command, its full output and its exit code.
3. If the check is not built yet, say so, name the plan step, and report what
   you did instead. Do not claim the stage is done.
4. List any file you changed. Never report a `registry/` change as yours.
