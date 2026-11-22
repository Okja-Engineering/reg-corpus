# Architecture decision records

One file per technical decision made after the spec was accepted: how the
tools, the gate, the layout or the evaluation work. Never a product or
compliance call, and never an interpretation of the regulation.

An accepted ADR is part of the spec (0009). The intent, spec and plan never
conflict with one: the PR that adds an ADR amends the text it changes, and the
ADR keeps the why. The spec's decisions table holds decisions 1 to 6, then one
row per ADR, so ADR numbers continue from 0007.

## Flow
1. Open a Decision issue (label `decision`): the call, context, options with a
   recommended one, what it blocks.
2. The owner decides. Quote the call on the issue, with its date, time and
   timezone and where it was given.
3. A PR adds the next `NNNN-short-title.md`, from `template.md`, and a line in
   the index below. The same PR amends the intent, spec or plan text the
   decision changes, adds the ADR's row to the spec's decisions table, and adds
   an "Amended" line to each changed doc's header.
4. The issue is closed when that PR merges.

A later ADR that replaces one sets the old one's status to "Superseded by NNNN",
and amends the spec's row for the old one.

## Index
| # | Decision | Status |
|---|---|---|
| [0007](0007-ci-is-the-c6-gate.md) | CI is the C6 gate; the hook and the R11 guard are seatbelts | Accepted |
| [0008](0008-no-arm-sees-key-yaml.md) | No eval arm sees `key.yaml`; only grading reads it | Accepted |
| [0009](0009-adrs-are-part-of-the-spec.md) | ADRs are part of the spec | Accepted |
