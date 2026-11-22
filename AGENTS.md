# reg-corpus: start here (R10)

This repo holds verbatim rule text (L0) for 12 CFR 1024.41, its Supplement I 41() comments and the
cite-only 1024.30 and 1024.31, plus a person-verified registry of records (L1) and the `regcorpus`
CLI that checks them. This file only routes you. It holds no rule content and no product decisions.

Spec (authoritative): `docs/03-spec.md`. Plan: `docs/04-plan.md`. Intent: `docs/01-intent.md`.
Requirement IDs (R1..R11, C1..C6) come from the spec. ADRs: `docs/adr/`. Templates: `docs/templates/`.

## Where things live
- `stages/0N-*/CONTRACT.md`: one contract per build stage (see below).
- `sources/ecfr/title-12/`: L0. `1024.41/`, `supplement-i/41/`, and cite-only
  `1024.30/`, `1024.31/`. Each holds `raw/<date>.xml`, `tree.json`, `text.md`,
  `meta.yaml` and an optional reviewed `overrides.yaml`.
- `staging/`: records proposed by extraction, at `fetched`.
- `registry/`: written by a person, never by an agent. `records/RX-*.yaml` and `verify.log`
  change only through `regcorpus verify`; a person runs `regcorpus inventory --init` to make
  `inventory/`, and writes its `reviewed` and `na` entries and `topics.yaml` by hand.
- `prompts/`: versioned extraction prompts. `skill/regcorpus/`: the agent skill (R9).
- `cmd/regcorpus/`, `internal/*/`: the Go CLI. Each package's `doc.go` names its job.
- `eval/`: seeds, fixtures, judge prompt, runs, results, walk and guard transcripts (R7).
- `.github/`: CI (the C6 gate), issue forms, CODEOWNERS, Dependabot and labels (`make labels`
  writes `labels.json` to GitHub). `.githooks/`: the hook and the gate's build. `.claude/`: R11 guard.

## Stages and their order
| Stage | Contract | Who acts |
|---|---|---|
| 01 fetch-parse | `stages/01-fetch-parse/CONTRACT.md` | person or agent; fetches run on the owner's machine |
| 02 extract | `stages/02-extract/CONTRACT.md` | agent proposes into `staging/` only |
| 03 verify | `stages/03-verify/CONTRACT.md` | a person only: records via `verify` in a terminal, inventory sign-off by hand |
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

## Plan steps outside the five stages
| Step | Builds or does | Who | Governed by |
|---|---|---|---|
| 7 | `regcorpus text <anchor>` | agent | R8 |
| 12 | `internal/registry` types and enums | agent | spec "L1 record", "Anchor rules", "Inventory" |
| 13 | C6 per-commit and `--complete` | agent | spec "Check rules"; R5 |
| 14 | `regcorpus inventory --init` | agent builds; a person runs it | R3; spec "Inventory" |
| 15 | `regcorpus show` and `--staging` | agent | R8; stage 03 (blind-sample rule) |
| 18 | walk test, saved to `eval/walk/` | a person starts a fresh session | R10; stage 05 contract |
| 20 | `regcorpus topics` | agent builds; a person writes `registry/topics.yaml` | R8; C6 `--complete` |
| 21 | guard check, saved to `eval/guard/` | a person, in a Claude Code session | R11; stage 05 contract |
| 22 | C1–C5; `eval/levels/{cover,strict}` | agent | R4; spec "Check rules" |
| 23 | `skill/regcorpus/SKILL.md`; 8-word check | agent drafts; a person reviews | R9 |
| 24 | `eval/negative/`, `make negative` | agent | spec "Acceptance" R4/R5 |
| 25 | `make build CORPUS_REF=`/`LOOKUP_ONLY=1`; `eval/render` | agent | spec "Releases and distribution"; stage 05 |

A tool step ends when it is built, `make test` passes and its acceptance command behaves as
the spec says (e.g. `check --complete` names each gap), not when the requirement is met.
Needed across stages: 12 before 02; 13 before 02 ends (`check --staging` runs C6); 15 and 19
before 03 (`show --staging`, `verify`); 13, 14 and 20 before 03 ends (`check --complete`
reads the inventory and `registry/topics.yaml`); 22, 23 and 25 before 05's arms (b) and (c).
A person approves `units.yaml` and `prompts/extract-v1.md` (an agent may draft) before extraction.

To pick up work, open the contract for your task (fetch or parse: 01; propose
records: 02; verify a record: 03; tag: 04; eval runs or grading: 05; anything
else: the table above), read its Inputs and Check, and act within "Who may run it".

## Hard rules
- No agent writes, moves or deletes anything under `registry/` (writers above).
- `regcorpus verify` is human-only and refuses to run without an interactive
  terminal. Never run it, wrap it (`script`, `expect`, `unbuffer`) or emulate it.
- Never bypass hooks: no `--no-verify`, no `git commit -n`, no changes to
  `core.hooksPath`, no edits under `.githooks/` or `.claude/`.
- L0 text is verbatim. A source defect is fixed only by a reviewed `overrides.yaml`
  entry (location, replacement, reason), never in parser code or stored text.
- No interpretations of the regulation. Records hold verbatim quotes in cited fields plus
  the label fields and decomposition the schema asks for; the eval fixture's seeded errors
  and `must_say` are deliberate. Neither is guidance. No product or compliance guidance here.
- CLI only: no server, API or MCP (decision 6).
- CI is the C6 gate: `.github/workflows/c6.yml`, `.github/c6.sh`, `.githooks/build-checker.sh`.
  The hook and the R11 guard are seatbelts against shortcuts, not attackers. A PR can edit the gate;
  the user, who alone merges, reviews such diffs and protects main (`C6 gate`, branches up to date).

## Make targets
- `make help` (default): list targets. `make setup`: point git at `.githooks/` (a person, once).
- `make build`: build `./regcorpus` (not on PATH). `./regcorpus help` lists the commands
  built today; a contract naming one not built yet names the plan step that builds it.
- `make test`: Go tests; R11 guard, pre-commit hook, CI gate and label tests (CI runs it too).
- `make check`: the branch's checker on the working tree (`./regcorpus check --corpus`, C6).
  The hook and CI both use main's checker on the commit, CI on a PR's test merge (`--tree`); they can differ.
  make exits 2 on failure; `./regcorpus` exits 0 ok, 1 violations or a fetch conflict, 2 unreadable or network.

## Reporting at the end of a stage
1. Run the contract's "Check that ends it"; report the stage, exact command, full output, exit code.
2. If the check is not built yet, say so, name the plan step, and report what you did
   instead. Do not claim the stage is done.
3. List any file you changed. Never report a `registry/` change as yours.
