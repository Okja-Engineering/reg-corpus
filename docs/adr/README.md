# Architecture decision records

One file per technical decision made after the spec was written: how the
tools, the gate, the layout or the evaluation work, or how the docs and this
process do. Never a product or compliance call, and never an interpretation of
the regulation.

An accepted ADR is part of the spec (0009). The intent, spec, plan, stage
contracts and AGENTS.md never conflict with one: the PR that adds an ADR amends
their text where it states what the ADR changes, and the ADR keeps the why. The
spec's decisions table holds decisions 1 to 6, then one row per ADR, so ADR
numbers continue from 0007.

## Flow
1. Open a Decision issue (label `decision`): the call, context, options with a
   recommended one, what it blocks. A call the owner makes directly, in chat
   say, needs no issue.
2. The owner decides. Quote the call on the issue, or for a direct call on the
   ADR's PR, with its date, time and timezone and where it was given.
3. A PR adds the next `NNNN-short-title.md`, from `template.md`, and a line in
   the index below. The same PR adds the ADR's row to the spec's decisions
   table and amends the intent, spec, plan, stage contract or AGENTS.md text the
   decision changes. The intent, spec and plan each gain an "Amended" header
   line naming the ADRs that changed it; AGENTS.md and the contracts have no
   header, so the ADR's "Spec changes" section is their record.
4. The issue, if there is one, is closed when that PR merges.

A later ADR that replaces one sets the old one's status to "Superseded by NNNN",
and amends the spec's row for the old one. One that changes what an older ADR
says without replacing it adds an "Amended" line to that ADR's header, naming
the sentences that now describe the past, and leaves them as written.

## Index
| # | Decision | Status |
|---|---|---|
| [0007](0007-ci-is-the-c6-gate.md) | CI is the C6 gate; the hook and the R11 guard are seatbelts | Accepted |
| [0008](0008-no-arm-sees-key-yaml.md) | No eval arm sees `key.yaml`; only grading reads it | Accepted |
| [0009](0009-adrs-are-part-of-the-spec.md) | ADRs are part of the spec | Accepted |
| [0010](0010-one-writer-owns-meta-yaml.md) | One writer owns L0 `meta.yaml` | Accepted |
| [0011](0011-fr-citation-is-the-amendment-page.md) | `version_fr_citations` cites the amendment page the stored XML cites; step 3 covers 1024.30 and 1024.31 | Accepted |
| [0012](0012-labels-count-in-the-paragraph-lead.md) | A label counts in a paragraph's lead; the next label decides (i)/(v)/(x) | Accepted |
| [0013](0013-cites-join-as-the-document-reads.md) | A cite joins as the document reads; a heading's separator is not operative text | Accepted |
