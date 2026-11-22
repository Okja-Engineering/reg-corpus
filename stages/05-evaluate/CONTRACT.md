# Stage 05: evaluate (R7, decision 5)

## Purpose
Measure whether the corpus plus the check catches seeded errors in a spec,
clearly better than the verbatim text alone, and apply the pre-registered pass
rule (spec decision 5) to reach accept, revise or kill.

## Inputs
- `eval/seed/`: per-type eligibility lists and the seed string, committed before
  selection; seeds are drawn by lowest `sha256(seed‖anchor)` (plan step 10).
- `eval/fixture-src/`: the CFR-cite fixture (spec, applicability file at
  `level: strict`, `key.yaml`), frozen with its hash recorded before any
  extraction (plan step 10).
- `eval/judge-v1.md`: the committed judge prompt, written before any run.
- Per arm (spec "Arms"), each run in its own temp directory:
  - (a): the L0 text files and `eval/fixture-src/` without `key.yaml` (plan step 11);
  - (b): L0, the lookup-only binary (`make build LOOKUP_ONLY=1`), the skill and
    `eval/fixture/`;
  - (c): as (b), with the full binary and the `regcorpus check` output.
  - No arm's run directory holds `key.yaml`; only grading reads it (spec
    decision 8, ADR 0008).
- `eval/render/`: maps `fixture-src` to `eval/fixture/` through the derived
  inventory (plan step 25). The skill is installed as a project skill in arms
  (b) and (c) (plan steps 23, 25 and 26).

## Outputs
- `eval/runs/<arm>-<model>-<n>.jsonl`: stream-json transcripts, 5 runs per arm
  per model (arms a, b, c; two models).
- `eval/results/<YYYY-MM-DD>.yaml`: per-run grades and metrics (recall,
  precision, trace rate), model ID string, CLI version, exact command,
  transcript path, fixture hash, judge-to-author agreement, and the decision 5
  outcome per model with the final decision (stage 04 reads accept from here).
- `eval/walk/` (R10 walk test) and `eval/guard/` (R11 guard check):
  transcripts and the corpus commit they ran against.

## Check that ends it
The evidence is the results file; no command checks it. The stage ends when
`eval/results/<YYYY-MM-DD>.yaml` holds 30 graded runs with the fields above,
the judge-to-author agreement rate, and the decision 5 outcome per model with
the final decision (R7 acceptance). Two gates come before it:
- Plan step 11 (first part): a model with mean recall(a) > 0.6 has failed
  decision 5; if both fail, stop and record a kill.
- Plan step 25: on the rendered fixture, `regcorpus check` exits non-zero and
  reports exactly the seeds `key.yaml` marks `check_detectable` (R4/R5).

## Who may run it
- Every run uses project-only settings: no user config, memory, skills or MCP,
  and web tools disallowed (spec "Arms"; plan step 11 records the command).
- A person launches each run and records the exact command.
- An agent may prepare fixtures with the person's approval, run `eval/render`,
  and run the committed judge on normalized, shuffled outputs.
- The author (a person) blind-adjudicates unmatched findings and blind-grades a
  random 20% (spec "Grading"). Below 90% agreement, the author's grades replace
  the judge's.
- Walk test (R10; plan step 18): while staged records exist, a person starts a
  fresh session given only the repo and asks it to verify the next record. It
  passes if the session names `stages/03-verify/CONTRACT.md` and the correct
  staged record. Save the transcript and the corpus commit under `eval/walk/`.
- Guard check (R11; plan step 21): a person, in a Claude Code session in the
  repo, tries `git commit --no-verify`, an edit under `registry/` and
  `regcorpus verify`; each must be blocked. Save the transcript under
  `eval/guard/`.

## Requirements
- R7: three arms on two models, five runs each; graded with the committed judge
  and the blind author sample; results recorded; decision 5 applied.
- Decision 5: the per-model pass rule and its pre-registered rules, applied as
  written in `docs/03-spec.md`.
- Spec "Evaluation (R7)": fixture source, seeds, `key.yaml`, arms, grading,
  metrics and results fields.
- R10 walk test (plan step 18): a fresh session given only the repo names
  `stages/03-verify/CONTRACT.md` and the correct staged record; the transcript
  and corpus commit are saved under `eval/walk/`.
- R11 guard check (plan step 21): `git commit --no-verify`, an edit under
  `registry/` and `regcorpus verify` are each blocked; the transcript is saved
  under `eval/guard/`.
