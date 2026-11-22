# 0011. `version_fr_citations` cites the amendment page the stored XML cites; step 3 covers 1024.30 and 1024.31

- **Status:** Accepted
- **Date:** 2026-11-22
- **Decided by:** the owner, 2026-11-22 15:48 CST, in chat, recorded on issue #6: (a) "Amendment page (Recommended)"; (b) "Yes, all three (Recommended)"

## Context
The spec's "L0 `meta.yaml`" gives the field's form, not which page it names:

> `version_fr_citations: ["<vol> FR <page>"]`

A Federal Register document spans pages, and two pages are on offer:

- the document's first page, which the Federal Register API returns as
  `citation`. The plan's upstream calls point there:
  > **Federal Register:** `documents/{doc}.json` for the FR citation and effective date.
- the page where the amendment to the section is printed, which eCFR's
  citation note (`<CITA>`) in the stored raw XML cites.

For the stored versions these differ:

| Section @ version | FR document | API `citation` (first page) | Amendment page, as `<CITA>` cites it |
|---|---|---|---|
| 1024.41 @ 2025-07-15 | 2025-08643 | 90 FR 20791 | 90 FR 20792 |
| 1024.31 @ 2025-07-15 | 2025-08643 | 90 FR 20791 | 90 FR 20792 |
| 1024.30 @ 2018-04-19 | 2016-18901 | 81 FR 72160 | 81 FR 72370 |

Plan step 3 names 1024.41 only:

> 3. **Version-date question:**
>    - Fetch 1024.41 at 2025-05-15, 2025-05-16 and 2025-07-15, and diff them.
>    - Record what changed on each date.
>    - Set `version_effective_date` and `version_fr_citations` from what the diff shows.

No plan step sets these fields for 1024.30 or 1024.31, though R1 requires
them for every source ("Each source records: … `version_effective_date` and
`version_fr_citations`").

## Decision
(a) `version_fr_citations` records, for the change that produced the stored
version, the page the stored raw XML's citation note (`<CITA>`) cites, so it
can be checked against L0 itself. `regcorpus meta set` refuses a citation that
note doesn't cite. This departs from the plan's "`documents/{doc}.json` for
the FR citation".

(b) Plan step 3 sets both fields for 1024.41, 1024.30 and 1024.31. This
departs from plan step 3 naming 1024.41 only. Supplement I stays with plan
step 8.

## Consequences
- Values set in step 3: 1024.41 and 1024.31, `version_effective_date`
  2025-07-15 and `version_fr_citations` ["90 FR 20792"]; 1024.30, 2018-04-19
  and ["81 FR 72370"]. The evidence is in the step 3 PR's diff record.
- `version_effective_date` follows the spec's comment, "effective date of the
  change that produced this version". For 1024.30 the Federal Register API's
  `effective_on` (2017-10-19) is not that date: the document's DATES text makes
  its amendatory instruction 5, the one that amends § 1024.30, effective on
  April 19, 2018, the date eCFR gives the version. A tool that copies
  `effective_on` would record the wrong date; `meta set` takes the date as
  given and checks only its form.
- Supplement I (plan step 8) sets these fields under the same rule.

## Requirement IDs
R1 (each source records `version_effective_date` and `version_fr_citations`);
spec "L0 `meta.yaml`"; spec "Carried forward" (the 2025-05-16 / 2025-07-15
version question, plan step 3).

## Links
- Issue #6 (the decision).
- The plan step 3 PR, which records this ADR and carries the diff record.
- Federal Register documents:
  https://www.federalregister.gov/api/v1/documents/2025-08643.json and
  https://www.federalregister.gov/api/v1/documents/2016-18901.json.
