# reg-corpus: start here (R10)

This repo holds verbatim rule text (L0) for 12 CFR 1024.41, its Supplement I
41() comments and the cite-only 1024.30 and 1024.31, plus a person-verified
registry of records (L1) and the `regcorpus` CLI that checks them. This file
only routes you. It holds no rule content and no product decisions.

Spec (authoritative): `docs/03-spec.md`. Plan: `docs/04-plan.md`. Intent:
`docs/01-intent.md`. Requirement IDs (R1..R11, C1..C6) come from the spec.

## Where things live
- `stages/0N-*/CONTRACT.md`: one contract per build stage (see below).
- `sources/ecfr/title-12/`: L0. `1024.41/`, `supplement-i/41/`, and cite-only
  `1024.30/`, `1024.31/`. Each holds `raw/<date>.xml`, `tree.json`, `text.md`,
  `meta.yaml` and an optional reviewed `overrides.yaml`.
- `staging/`: records proposed by extraction, at `fetched`.
- `registry/`: written by a person, never by an agent:
  - `records/RX-*.yaml` enter only through `regcorpus verify` (text-verified or above);
  - `verify.log` is appended only by `regcorpus verify`;
  - `inventory/` is created by `regcorpus inventory --init`, run by a person; its
    `reviewed` and `na` entries are written by a person;
  - `topics.yaml` is built by hand by a person.
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
2. 05 evaluate, first part: freeze `eval/fixture-src/` and the judge (plan step 10),
   run arm (a) and pass its checkpoint (plan step 11). All before any extraction.
3. 02 extract: propose the whole set into `staging/` before verification starts.
4. 03 verify: the blind sample first, then the rest, in the order its contract defines.
5. 05 evaluate, second part: arms (b) and (c), grading, decision 5.
6. 04 release, only if the decision is accept.

## Plan steps that build tools (outside the five stages)
| Step | Builds | Who | Governed by |
|---|---|---|---|
| 12 | `internal/registry` types and enums | agent | spec "L1 record", "Anchor rules", "Inventory" |
| 13 | C6 per-commit and `--complete` | agent | spec "Check rules"; R5 |
| 14 | `regcorpus inventory --init` | agent builds; a person runs it | R3; spec "Inventory" |
| 15 | `regcorpus show` and `--staging` | agent | R8; stage 03 (blind-sample rule) |
| 20 | `regcorpus topics` | agent builds; a person writes `registry/topics.yaml` | R8; C6 `--complete` |
| 22 | C1–C5; `eval/levels/{cover,strict}` | agent | R4; spec "Check rules" |
| 23 | `skill/regcorpus/SKILL.md`; 8-word check | agent drafts; a person reviews | R9 |
| 24 | `eval/negative/`, `make negative` | agent | spec "Acceptance" R4/R5 |
| 25 | `make build CORPUS_REF=`/`LOOKUP_ONLY=1`; `eval/render` | agent | spec "Releases and distribution"; stage 05 |

A tool step ends when `make test` passes and its spec "Acceptance" command behaves
as stated. `stages/02-extract/units.yaml` and `prompts/extract-v1.md` may be drafted
by an agent; a person approves them before extraction (stage 02).

To pick up work, open the contract for your task (fetch or parse: 01; propose
records: 02; verify a record: 03; tag: 04; eval runs or grading: 05; a tool: the
table above), read its Inputs and Check, and act within "Who may run it".

## Hard rules
- No agent writes, moves or deletes anything under `registry/` (writers above).
- `regcorpus verify` is human-only and refuses to run without an interactive
  terminal. Never run it, wrap it (`script`, `expect`, `unbuffer`) or emulate it.
- Never bypass hooks: no `--no-verify`, no `git commit -n`, no changes to
  `core.hooksPath`, no edits under `.githooks/` or `.claude/`.
- L0 text is verbatim. A source defect is fixed only through a reviewed
  `overrides.yaml` entry (location, replacement, reason), never in parser code
  and never by editing stored text.
- No interpretations of the regulation. Records hold verbatim quotes in their
  cited fields, plus the label fields and decomposition the spec's schema asks
  for; the eval fixture deliberately holds seeded errors and `must_say`
  statements. Neither is guidance. Product decisions and compliance guidance
  never go in this repo.
- CLI only: no server, API or MCP (decision 6).

## Make targets
- `make setup`: point git at `.githooks/`; run by a person, once per clone.
- `make build`: build `./regcorpus` (not on PATH; build it before running it).
- `make test`: Go tests, the R11 guard tests and the pre-commit gate tests.
- `make check`: `./regcorpus check --corpus` on the working tree (C6). The
  pre-commit hook checks the commit itself (`--tree`), which can differ.
- `make help` (default): list targets.

`./regcorpus help` lists the commands that exist today; a contract naming a
command not built yet names the plan step that builds it.

## Reporting at the end of a stage
1. Run the "Check that ends it" command from that stage's contract.
2. Report the stage, the exact command, its full output and its exit code.
3. If the check is not built yet, say so, name the plan step, and report what
   you did instead. Do not claim the stage is done.
4. List any file you changed. Never report a `registry/` change as yours.
