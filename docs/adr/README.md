# Architecture decision records

One file per technical decision made after the spec was accepted: how the
tools, the gate, the layout or the evaluation work. Never a product or
compliance call, and never an interpretation of the regulation.

The spec's "Decisions on the intent's open questions" table holds decisions
1 to 6, so ADR numbers continue from 0007. The accepted intent, spec and plan
are not edited here; where an ADR departs from them, it quotes the text it
departs from.

## Flow
1. Open a Decision issue (label `decision`): the call, context, options with a
   recommended one, what it blocks.
2. The owner decides. Quote the call on the issue, with its date, time and
   timezone and where it was given.
3. A PR adds the next `NNNN-short-title.md`, from `template.md`, and a line in
   the index below.
4. The issue is closed when that PR merges.

A later ADR that replaces one sets the old one's status to "Superseded by NNNN".

## Index
| # | Decision | Status |
|---|---|---|
| [0007](0007-ci-is-the-c6-gate.md) | CI is the C6 gate; the hook and the R11 guard are seatbelts | Accepted |
| [0008](0008-no-arm-sees-key-yaml.md) | No eval arm sees `key.yaml`; only grading reads it | Accepted |
| [0010](0010-one-writer-owns-meta-yaml.md) | One writer owns L0 `meta.yaml`; `regcorpus meta set` sets step 3's fields | Accepted |
| [0011](0011-fr-citation-is-the-amendment-page.md) | `version_fr_citations` cites the amendment page the stored XML cites; step 3 covers 1024.30 and 1024.31 | Accepted |
