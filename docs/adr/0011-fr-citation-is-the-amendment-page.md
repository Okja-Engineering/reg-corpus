# 0011. `version_fr_citations` cites the amendment page the stored XML cites; step 3 covers 1024.30 and 1024.31

- **Status:** Accepted
- **Date:** 2026-11-22
- **Decided by:** the owner, 2026-11-22 15:48 CST, in chat, recorded on issue #6: (a) "Amendment page (Recommended)"; (b) "Yes, all three (Recommended)"

## Context
The spec's "L0 `meta.yaml`" gave the field's form, not which page it names:

> `version_fr_citations: ["<vol> FR <page>"]`

A Federal Register document spans pages, and two pages were on offer:

- the document's first page, which the Federal Register API returns as
  `citation`. The plan's upstream calls pointed there:
  > **Federal Register:** `documents/{doc}.json` for the FR citation and effective date.
- the page where the amendment to the section begins, which eCFR's citation
  note (`<CITA>`) in the stored raw XML cites.

For the stored versions these differ:

| Section @ version | FR document | API `citation` (first page) | `<CITA>` cites |
|---|---|---|---|
| 1024.41 @ 2025-07-15 | 2025-08643 | 90 FR 20791 | 90 FR 20792 |
| 1024.31 @ 2025-07-15 | 2025-08643 | 90 FR 20791 | 90 FR 20792 |
| 1024.30 @ 2018-04-19 | 2016-18901 | 81 FR 72160 | 81 FR 72370 |

In 2025-08643, amendatory instruction 4 (§ 1024.41) begins on page 20792 and
its text runs onto page 20793; instruction 2 (§ 1024.31) is on page 20792.
In 2016-18901, instruction 5 (§ 1024.30) is on page 72370.

Plan step 3 named 1024.41 only:

> 3. **Version-date question:**
>    - Fetch 1024.41 at 2025-05-15, 2025-05-16 and 2025-07-15, and diff them.
>    - Record what changed on each date.
>    - Set `version_effective_date` and `version_fr_citations` from what the diff shows.

R1 requires both fields for every source ("Each source records: …
`version_effective_date` and `version_fr_citations`"). Plan step 9 makes
1024.30 and 1024.31 cite-only L0 (their parse and anchors) and doesn't name
these two fields.

## Options considered
As on issue #6:
- (a) The amendment page, as the stored XML cites it (chosen), or the
  document's first page. The amendment page can be checked against the
  stored raw XML. The first page is the API's `citation`, which the stored
  XML doesn't hold.
- (b) All three sources in step 3 (chosen), or 1024.41 only. The facts for
  1024.30 and 1024.31 were already researched. With 1024.41 only, their two
  fields would stay unset until a later step that names them, and none
  does.

## Decision
The owner decided:
- (a) `version_fr_citations` records the amendment page, as the citation
  note (`<CITA>`) of the stored raw XML cites it, for the change that
  produced the stored version. For an amendment that runs over more than one
  page, that is the page the note cites, where the amendment begins.
- (b) Plan step 3 sets both version fields for 1024.41, 1024.30 and 1024.31.

How it is carried out was the chief of staff's (an agent's) choice, not the
owner's call. Each choice and its reason:
- **`regcorpus meta set` refuses a citation the note doesn't cite.** It
  reads each run of a `<CITA>` element's text on its own, any whitespace
  between a citation's parts, and eCFR's shorthand pages ("78 FR 63004,
  63005"). Why: issue #6 asked that the value be checkable against L0
  itself. The check is membership only. It doesn't prove that a cited page
  is the amendment that produced the version: that evidence is the step 3
  diff record in PR #7.
- **`version_effective_date` must not be after `source_version_date`.** Why:
  the spec defines it as the "effective date of the change that produced this
  version", and the version eCFR dates `source_version_date` holds that
  change, so the change took effect by that date. No other check is made on
  it.

## Spec changes
- `docs/03-spec.md`:
  - Decisions-table row 11 is added: "Which page does
    `version_fr_citations` record, and for which sources does plan step 3
    set the version fields? The amendment page, as the citation note
    (`<CITA>`) of the stored raw XML cites it. Plan step 3 sets both version
    fields for 1024.41, 1024.30 and 1024.31."
  - "L0 `meta.yaml`": `version_fr_citations: ["<vol> FR <page>"]` gains the
    comment "the amendment page, as the raw XML's citation note (<CITA>)
    cites it (decision 11)".
  - "Upstream": "the Federal Register supplies the FR citation and effective
    date" → "the Federal Register supplies the effective date and the
    amending document; `version_fr_citations` records the amendment page as
    the stored raw XML's citation note cites it (decision 11)".
- `docs/04-plan.md`:
  - Upstream calls: "`documents/{doc}.json` for the FR citation and
    effective date" → "… for the effective date and the amending document;
    `version_fr_citations` records the amendment page the stored XML's
    `<CITA>` cites (ADR 0011)".
  - Step 3: "Set `version_effective_date` and `version_fr_citations` from
    what the diff shows." → "… from what the diff shows, with `regcorpus meta
    set`. Set both for 1024.30 and 1024.31 too, each from the change that
    produced its stored version (ADRs 0010, 0011)."
- `stages/01-fetch-parse/CONTRACT.md`, Inputs: "The Federal Register, for
  `version_fr_citations` and `version_effective_date` (R1)." → "The Federal
  Register, for `version_effective_date` and the amending document (R1).
  `version_fr_citations` records the amendment page as the stored raw XML's
  citation note (`<CITA>`) cites it (spec decision 11); `regcorpus meta set`
  sets both fields."
- No other lines. AGENTS.md says nothing about these fields.

## Consequences
- Values set in step 3: for 1024.41 and 1024.31, `version_effective_date`
  2025-07-15 and `version_fr_citations` ["90 FR 20792"]; for 1024.30,
  2018-04-19 and ["81 FR 72370"]. The evidence is in PR #7's diff record.
- For 1024.30 the Federal Register API's `effective_on` (2017-10-19) is not
  the date of the change that produced the stored version. The document's
  DATES text makes its amendatory instruction 5, which amends § 1024.30,
  effective on April 19, 2018, the date eCFR gives the version. A tool that
  copied `effective_on` would record the wrong date; `meta set` takes the
  date as given.
- Supplement I (plan step 8) will need the same call for its two fields.
  This ADR doesn't make it.

## Requirement IDs
R1 (each source records `version_effective_date` and
`version_fr_citations`); spec "L0 `meta.yaml`", "Upstream" and "Carried
forward" (the 2025-05-16 / 2025-07-15 version question, plan step 3);
decision 11.

## Links
- Issue #6 (the decision): https://github.com/Okja-Engineering/reg-corpus/issues/6
- PR #7 (https://github.com/Okja-Engineering/reg-corpus/pull/7), which
  records this ADR and carries the diff record.
- Federal Register documents:
  https://www.federalregister.gov/api/v1/documents/2025-08643.json and
  https://www.federalregister.gov/api/v1/documents/2016-18901.json.
