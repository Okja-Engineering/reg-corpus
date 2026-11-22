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

## Decision
An accepted ADR is part of the spec. The intent, spec and plan never conflict
with an accepted ADR. The PR that adds an ADR amends the doc text it changes,
in the same PR, and adds the ADR's row to the spec's decisions table. The ADR
keeps the why: the context, the options and the call.

## Spec changes
This PR brings the docs in line with 0007, 0008 and this ADR:
- `docs/03-spec.md`: decision 3 (the corpus gate is CI from phase 1; consumers
  keep the local hook, CI later); rows 7, 8 and 9 and a line saying rows 7 on
  are accepted ADRs; "Enforcement" (the same split as decision 3); arm (a)
  gets `fixture-src` without `key.yaml`, and no arm's run directory holds it.
- `docs/04-plan.md`: the stack line (CI is the gate; hook and guard are
  seatbelts); the files list adds `.github/workflows/c6.yml`; step 1 adds the
  CI gate; the agent-bypass risk's "CI later" becomes "which CI gates".
- `docs/01-intent.md`: "Enforced, not advised" (CI gate in the corpus from
  phase 1; consumers' hooks, CI later).
- `stages/05-evaluate/CONTRACT.md`: the `key.yaml` rule cites decision 8, not
  a differing spec line. `AGENTS.md`: the spec includes the accepted ADRs.
- Each changed doc's header gains "Amended: 2026-11-22 by ADRs 0007–0009"; the
  spec's and plan's stale `intent.md` and `spec.md` references name
  `docs/01-intent.md` and `docs/03-spec.md`.

## Consequences
- The intent, spec and plan are no longer frozen as accepted; each carries an
  "Amended" header line naming the ADRs that changed it.
- Doc text that conflicts with an accepted ADR is a review finding.
- The Consequences lines of 0007 and 0008 saying the spec text is unchanged
  describe the state when they were recorded; this PR amended that text.
- `docs/adr/README.md`, `docs/adr/template.md` (a "Spec changes" section), the
  Decision issue form and `AGENTS.md` state the new flow.

## Requirement IDs
R10 (AGENTS.md routes to the spec); spec decisions 3, 7, 8 and 9.

## Links
- ADRs 0007 and 0008, the departures this ends.
- `docs/adr/README.md`, "Flow".
