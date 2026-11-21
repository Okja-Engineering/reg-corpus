# Stage 02: extract proposed records (R2, R2a)

## Purpose
Propose L1 records from L0 text, one extraction unit at a time. This is the
only step where a model does judgment work, so it is bounded (R2a). Every field
that restates the rule carries a `cite` and a verbatim `text` quote. Proposals
are not verified records; stage 03 decides what enters `registry/`.

## Inputs
For one unit (R2a):
- The unit, listed in `stages/02-extract/units.yaml` (plan step 16). The file
  covers every operative anchor exactly once; a unit is a paragraph with its
  whole subtree.
- The own text of the unit's ancestors and of the anchors it cross-references,
  from stage 01's `tree.json`.
- The record schema (spec "L1 record"; types in `internal/registry`, plan step 12).
- The golden records in `stages/02-extract/golden/`: one hand-verified record per
  shape listed in R2a (plan step 16).
- The versioned extraction prompt, `prompts/extract-v1.md` (plan step 16).

Precondition: the eval fixture is frozen (stage 05, plan step 10) before any
extraction starts.

## Outputs
- `staging/RX-*.yaml`: proposed records in the fixed schema, at
  `verification.level: fetched`, each naming the `prompt_version` that produced
  it, or `manual` if a person wrote it (R2a). Nothing is written to `registry/`.

## Check that ends it
- `regcorpus check --staging` exits 0 (R2a; built in plan step 16). Exit 0 means
  C6 passes on every staged record and no two staged records share the same
  `action` cite and text.
- Until it exists, report that the check is not built and name plan step 16.

## Who may run it
- An agent may propose records into `staging/` only, using the prompt above.
- A model never writes, moves or edits anything under `registry/`.
- Golden records and anchor rules are written by a person, with
  `prompt_version: manual` (plan steps 16 and 17).

## Requirements
- R2: one record per obligation and anchor rule; every cited field carries
  `cite` and `text`; `deadline.anchor_event` comes from the spec's fixed list.
- R2a: unit input, golden records, output to `staging/` at `fetched`, the
  `check --staging` gate, and the versioned prompt.
- Spec "Definitions": cited field, quote (at least 4 words), multi-anchor cite.
