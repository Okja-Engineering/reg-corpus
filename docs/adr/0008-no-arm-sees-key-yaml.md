# 0008. No eval arm sees `key.yaml`; only grading reads it

- **Status:** Accepted
- **Date:** 2026-11-21
- **Decided by:** the owner, 2026-11-21 20:55 CST, in chat: "Yes, hide key.yaml"

## Context
The spec's "Evaluation (R7)" section puts `key.yaml`, the fixture's answer key,
in `eval/fixture-src/`, and its Arms line gives arm (a) "L0 text files +
`fixture-src`", which includes the key. An arm that can read the key can
report the seeded errors without finding them.

## Decision
No arm's run directory holds `key.yaml`; only grading reads it. Arm (a) gets
the L0 text files and `eval/fixture-src/` without `key.yaml`. This departs
from the spec's arm (a) line.

## Consequences
- `stages/05-evaluate/CONTRACT.md` states the rule under Inputs.
- The spec's arm (a) line is unchanged; PR #1's Findings note that it needs
  the owner's update.

## Requirement IDs
R7; spec decision 5 (the pass rule the arms feed).

## Links
- PR #1 review comment recording the call
  (https://github.com/Okja-Engineering/reg-corpus/pull/1#discussion_r4656334228).
- Commit ad68179 (stage 05 contract).
- PR #1 (https://github.com/Okja-Engineering/reg-corpus/pull/1): Findings,
  "Arm (a) must not see the answer key".
