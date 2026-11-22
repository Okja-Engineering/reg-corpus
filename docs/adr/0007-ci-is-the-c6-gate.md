# 0007. CI is the C6 gate; the hook and the R11 guard are seatbelts

- **Status:** Accepted
- **Date:** 2026-11-21
- **Decided by:** the owner, 2026-11-21 (time not recorded), in chat: "CI gate, guard seatbelt"
- **Amended:** 2026-11-22 by ADR 0009, which made accepted ADRs part of the spec. The spec now states this decision: decision 3 and "Enforcement" make the corpus gate CI from phase 1, and decision 7 is this ADR. The Context, the Decision's "This departs from spec decision 3's 'CI closes that later'" and the Consequences line "The spec's decision 3 text is unchanged" describe the spec when this was recorded. Requirement IDs add decision 7.

## Context
Spec decision 3 made phase 1 local only: a git pre-commit hook runs the check,
the R11 agent guard blocks bypasses, and "A person can still bypass the git
hook; CI closes that later." Three review rounds of plan step 1 (PR #1) found
that every local layer can be switched off by whoever works on the machine:
merge, rebase, cherry-pick and `am` skip pre-commit; the hook script lives in
the branch's own working tree; the Go environment, the local `main` ref and
checkouts without `.githooks/` can each disable it; and the guard cannot cover
every shell spelling.

## Decision
A GitHub check, the `C6 gate` job in `.github/workflows/c6.yml`, is the gate.
It runs C6 (`check --corpus --tree`) on the commit under test, which for a
pull request is GitHub's test merge commit, then `make test`. The checker is
built from `origin/main` whenever it has one, else from the PR's base; only
while neither has one (bootstrap) from the commit itself, with a warning.
See `.github/c6.sh` and `.github/workflows/c6.yml`. The pre-commit hook and
the R11 guard stay as seatbelts: they catch honest mistakes and an agent's
shortcuts early and are documented as incomplete. This departs from spec
decision 3's "CI closes that later": CI arrives in phase 1, with plan step 1.

## Consequences
- The gate's files are `.github/workflows/c6.yml`, `.github/c6.sh` and
  `.githooks/build-checker.sh`. A PR can edit them; the owner, who alone
  merges, reviews such diffs.
- CI is trustworthy only once branch protection on `main` requires the
  `C6 gate` check and up-to-date branches. PR #1 deferred that to the owner.
- The spec's decision 3 text is unchanged; AGENTS.md's hard rules state this
  decision.

## Requirement IDs
C6, R5, R11; spec decisions 3 and 7.

## Links
- PR #1 (https://github.com/Okja-Engineering/reg-corpus/pull/1): Summary and
  Findings, "CI moved forward from 'later'".
- Commits 84aab46 (the CI gate) and 060ab8f (docs: CI is the gate).
- AGENTS.md, "Hard rules".
