# 0009. ADRs are part of the spec

- **Status:** Accepted
- **Date:** 2026-11-22
- **Decided by:** the owner, 2026-11-22 about 15:20 CST, in chat: "architectural decision records take precedence over the spec, or rather, they should be considered as a part of the spec, and there shouldn't be conflict. We should identify conflicts between the two and adjust the spec, right?"

## Context
`docs/adr/README.md` said: "The accepted intent, spec and plan are not edited
here; where an ADR departs from them, it quotes the text it departs from." So
0007 and 0008 were recorded as departures: spec decision 3's "CI closes that
later", the spec's arm (a) line ("L0 text files + `fixture-src`", which holds
`key.yaml`) and the plan's stack line ("Local git only; the gate is a
pre-commit hook plus the R11 agent guard") stayed as written, and a reader of
the spec alone met text the project no longer followed.

## Options considered
1. Keep the docs as written; an ADR quotes the text it departs from (the
   README's rule until now). A reader of the spec alone meets text the project
   no longer follows.
2. An accepted ADR is part of the spec, and the PR that adds one amends the
   docs to match. Chosen: the owner's call above.

## Decision
An accepted ADR is part of the spec. The intent, spec, plan, stage contracts
and AGENTS.md never conflict with an accepted ADR. The PR that adds an ADR
amends their text where it states what the ADR changes, in the same PR, and
adds the ADR's row to the spec's decisions table. The ADR keeps the why: the
context, the options and the call.

## Spec changes
Row 9 of the spec's decisions table is this ADR. This PR also brings the docs
in line with 0007 and 0008 (before → after):
- `docs/03-spec.md`: decision 3 "Phase 1 is local only … CI closes that later"
  → in the corpus the gate is CI from phase 1, once `main`'s branch protection
  requires it (decision 7), hook and guard seatbelts; consumers stay local, CI
  later. "Enforcement" "the pre-commit hook … and the R11 guard enforce; CI
  takes over later" → the same split. Rows 7, 8 and 9 are added, with a line
  saying what rows 1–6 and rows 7 on are. Arm (a) "L0 text files +
  `fixture-src`" → "… without `key.yaml`", and no arm's run directory holds it
  (decision 8).
- `docs/04-plan.md`: stack "Local git only; the gate is a pre-commit hook plus
  the R11 agent guard" → CI's `C6 gate` check, once `main`'s branch protection
  requires it, with hook and guard as seatbelts. Files: the hook is "a
  seatbelt", and `.github/workflows/c6.yml` is added. Step 1 adds the CI gate
  and the owner's branch-protection setting. Risks "`verify.log` checked by
  C6; CI later" → CI also runs C6, which binds once `main` is protected, and
  C6 can't tell who wrote a record.
- `docs/01-intent.md`: "Enforced, not advised" "run as hooks (CI later) … CI
  closes that" → CI closes the bypass for the check, in the corpus from phase 1
  once `main` is protected, in consumers later; the no-model-edits rule rests
  on the R11 guard and on `verify` needing a terminal.
- `stages/05-evaluate/CONTRACT.md`: "(owner's decision, 2026-11-21; the spec's
  arm (a) line says `fixture-src` whole)" → "(spec decision 8, ADR 0008)".
- `AGENTS.md`: "Spec (authoritative)" → "Spec (authoritative; the accepted
  ADRs in `docs/adr/` are part of it)".
- Headers: the spec gains "Amended: 2026-11-22 by ADRs 0007–0009", the intent
  and plan "… by ADR 0007". The spec's `intent.md` and the plan's `spec.md`
  references → `docs/01-intent.md` and `docs/03-spec.md`.
- ADRs 0007 and 0008 gain an "Amended" line, and their Requirement IDs name
  decisions 7 and 8. `.github/workflows/c6.yml`'s header comment "Spec
  decision 3 left … for "CI later"; this is that CI" → spec decisions 3 and 7,
  binding once `main`'s branch protection requires the check.

## Consequences
- The docs are not frozen once written or accepted: an accepted ADR amends
  them. The intent, spec and plan carry an "Amended" header line naming the
  ADRs that changed each; AGENTS.md and the stage contracts have no header, so
  the ADR's "Spec changes" section is the record of their amendments.
- Doc text that conflicts with an accepted ADR is a review finding.
- ADRs 0007 and 0008 carry an "Amended" line: their sentences saying the spec
  departs from them or is unchanged describe it when they were recorded.
- `docs/adr/README.md`, `docs/adr/template.md` (an "Options considered" and a
  "Spec changes" section), the Decision issue form and `AGENTS.md` state the
  new flow. A call the owner makes directly, as here, needs no Decision issue:
  its quote goes on the ADR's PR.

## Requirement IDs
R10 (AGENTS.md routes to the spec); spec decisions 3, 7, 8 and 9.

## Links
- PR #4 (https://github.com/Okja-Engineering/reg-corpus/pull/4), which records
  this ADR, and its comment quoting the call
  (https://github.com/Okja-Engineering/reg-corpus/pull/4#issuecomment-7086089775).
- ADRs 0007 and 0008, the departures this ends.
- `docs/adr/README.md`, "Flow".
