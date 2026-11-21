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
- `eval/render/`: maps `fixture-src` to `eval/fixture/` through the derived
  inventory (plan step 25).
- For arms (b) and (c): the lookup-only binary (`make build LOOKUP_ONLY=1`), the
  full binary, and the skill (`skill/regcorpus/SKILL.md`), installed as a
  project skill (plan steps 23, 25 and 26).

## Outputs
- `eval/runs/<arm>-<model>-<n>.jsonl`: stream-json transcripts, 5 runs per arm
  per model (arms a, b, c; two models).
- `eval/results/<YYYY-MM-DD>.yaml`: per-run grades and metrics (recall,
  precision, trace rate), model ID string, CLI version, exact command,
  transcript path, fixture hash, judge-to-author agreement, and the decision 5
  outcome per model with the final decision.
- `eval/walk/` (R10 walk test) and `eval/guard/` (R11 guard check):
  transcripts and the corpus commit they ran against.

## Check that ends it
- No command exists for this check. Its evidence is the results file (R7
  acceptance): 30 graded runs with the fields above, the judge-to-author
  agreement rate, and the decision 5 outcome per model with the final decision.
- On the rendered fixture, `regcorpus check` exits non-zero and reports exactly
  the seeds `key.yaml` marks `check_detectable` (R4/R5; plan step 25).
- Checkpoint (plan step 11): a model with mean recall(a) > 0.6 has failed
  decision 5; if both models fail, stop and record a kill.

## Who may run it
- A person launches each run in its own temp directory with project-only
  settings, no MCP and web tools disallowed, and records the exact command
  (plan step 11).
- The committed judge prompt grades normalized, shuffled outputs.
- The author (a person) blind-adjudicates unmatched findings and blind-grades a
  random 20% (spec "Grading"). Below 90% agreement, the author's grades replace
  the judge's.

## Requirements
- R7: three arms on two models, five runs each; graded with the committed judge
  and the blind author sample; results recorded; decision 5 applied.
- Decision 5: the per-model pass rule and its pre-registered rules, applied as
  written in `docs/03-spec.md`.
- Spec "Evaluation (R7)": fixture source, seeds, `key.yaml`, arms, grading,
  metrics and results fields.
