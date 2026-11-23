# 0012. Labels count in a paragraph's lead; no (i)/(v)/(x) lookahead

- **Status:** Accepted
- **Date:** 2026-11-22
- **Decided by:** the owner, 2026-11-22 19:41 CST, in chat (answers to the chief of staff's two questions), recorded on PR #8: (1) "Paragraph lead (Recommended)"; (2) "Close it (Recommended)"

## Context
R1a said where a label counts:

> - recognizes a label only at the start of a paragraph element, or right after the dash that ends a heading; labels inside running text (cross-references) are never structural

The stored 1024.41 XML (`sources/ecfr/title-12/1024.41/raw/2025-07-15.xml`)
also puts structural labels in two other places:
- right after a heading that ends in a period and a space:
  - line 18: `<P>(v) <I>Certain COVID-19-related loss mitigation options.</I> (A) Notwithstanding …`
  - line 23: `<P>(3) <I>Notice of complete application.</I> (i) Except as provided …`
  - line 37: `<P>(ii) <I>Effect in case of delay.</I> (A)(<I>1</I>) Except as provided …`
- right after another label, with nothing between them: line 37's
  `(A)(<I>1</I>)`.

Read literally, the rule leaves (c)(2)(v)(A) in (c)(2)(v)'s text, and the
parse fails at the next paragraph:

> parse: line 19, column 4: unplaceable label (1): italic labels sit under capital labels; the open levels are (c)(2)(v)

So the spec's own acceptance goldens (c)(2)(v)(A)(*1*) and (c)(4)(ii)(B)(*2*)
could not exist, and neither could the anchor (c)(3)(i) that plan step 8's
golden 41(c)(3)(i)-3 must resolve to. This is the text's layout, not a
source defect: an `overrides.yaml` entry would have to change verbatim text.
In PR #8, mutant M3 (a label after a heading only past a dash, the literal
wording) and M2 (a label only at the start of a `<P>`) each turn the goldens
and the tree-shape test red.

The spec's "Carried forward" table also held:

> | (i)/(v)/(x) lookahead in R1a | before parsing any section where (h), (u) or (w) has children |

1024.41's (h) has children (1) to (4). At line 66, `(i)` can be placed
either way: as the letter after (h), or as the first roman under (h)(4).
R1a's rule reads it as a letter; the next label, (j), confirms it, since a
roman `(i)` there would leave (j) skipping (i) and the parse would fail.

## Options considered
As asked by the chief of staff:
- The label rule:
  1. **Paragraph lead (recommended).** A label counts in a paragraph's lead:
     at the start of the `<P>`, right after a heading past the dash or space
     that ends it, or right after another lead label. The lead ends at the
     first other text. Chosen: the owner's call above.
  2. The same, but a heading must end in "—", or in "." plus a space. All
     three 1024.41 cases fit; a heading ending any other way would fail the
     parse instead of chaining.
- The lookahead:
  1. **Close it (recommended).** No lookahead: the stated rule, plus failing
     on any sequence break, decides it. Chosen: the owner's call above.
  2. Keep the row open for later sources.
  3. Add lookahead now: where both readings can be placed, read the next
     label before choosing.

## Decision
1. A label counts only in a paragraph's lead: at the start of the `<P>`,
   right after a heading past the dash or space that ends it, or right after
   another lead label. The lead ends at the first other text; labels in
   running text (cross-references) are never structural.
2. There is no (i)/(v)/(x) lookahead. The rule (a letter only when the last
   letter-level sibling directly precedes it) and failing on any skipped,
   repeated, out-of-order or unplaceable label decide every case but one: a
   lone roman (i) that is a section's last label, after a letter-level
   sibling (h), would be read as the letter (i) without a later label to
   fail on. That gap is recorded here and not handled.

How it is carried out was the implementer's (an agent's) choice, not the
owner's call:
- **What a heading is.** The italic text right after a label and one space
  (`(b) <I>…</I>`). Why: it is how every heading in 1024.41 is written.
- **Where the dash or space after a heading goes.** It stays at the end of
  the parent's own text, so the own texts of one `<P>`, joined, are exactly
  its character data (`(b) Receipt of a loss mitigation application—`).
  Why: no character of the source is dropped or moved. Plan step 5 decides
  what that leftover means for the operative-anchor test and for
  multi-anchor joins.

## Spec changes
- `docs/03-spec.md`:
  - Decisions-table row 12 is added: "Where does a label count, and does
    R1a need (i)/(v)/(x) lookahead? A label counts in a paragraph's lead: at
    the start of the `<P>`, right after a heading past the dash or space that
    ends it, or right after another lead label; the lead ends at the first
    other text. No lookahead: the (i)/(v)/(x) rule plus failing on any
    sequence break decides it; the one gap is a lone roman (i) that is a
    section's last label."
  - R1a: "recognizes a label only at the start of a paragraph element, or
    right after the dash that ends a heading; labels inside running text
    (cross-references) are never structural" → "recognizes a label only in a
    paragraph's lead: at the start of a paragraph element, right after a
    heading past the dash or space that ends it, or right after another lead
    label; the lead ends at the first other text, and labels inside running
    text (cross-references) are never structural (decision 12)".
  - "Carried forward": the row "(i)/(v)/(x) lookahead in R1a | before
    parsing any section where (h), (u) or (w) has children" is removed.
  - Header "Amended: 2026-11-22 by ADRs 0007–0011" → "… by ADRs 0007–0012".
- `docs/04-plan.md`:
  - Step 4: "**Label position:** a label counts only at the start of `<P>`
    or after a heading dash." → "**Label position:** a label counts only in
    a paragraph's lead: at the start of `<P>`, right after a heading past the
    dash or space that ends it, or right after another lead label; the lead
    ends at the first other text (ADR 0012)."
  - Header "Amended: 2026-11-22 by ADRs 0007, 0010 and 0011" → "… by ADRs
    0007, 0010, 0011 and 0012".
- `stages/01-fetch-parse/CONTRACT.md`, Requirements, R1a: "Labels count only
  at the start of a paragraph element or right after a heading dash;" →
  "Labels count only in a paragraph's lead: at the start of a paragraph
  element, right after a heading past the dash or space that ends it, or
  right after another lead label (spec decision 12);".
- No other lines. The plan's Risks row ("labels recognized only by
  position") still holds. The intent and AGENTS.md say nothing about label
  position or lookahead.

## Consequences
- 1024.41 parses with no override: 91 nodes from 75 `<P>`, with the
  goldens (c)(2)(v)(A)(*1*), (c)(3)(i) and (c)(4)(ii)(A)(*1*) in place.
- A later source whose headings end some other way still chains (option 2's
  narrower test is not applied); a label there that breaks the sequence
  still fails the parse.
- The lone-roman-(i) gap stays. A source that holds it would parse
  silently wrong at that label; no current source does.
- Plan step 5 inherits the leftover dash or space at the end of a heading
  parent's own text (see the Decision's last item).

## Requirement IDs
R1a (deterministic parse: label position, the (i)/(v)/(x) rule); spec
"Carried forward"; decision 12.

## Links
- PR #8 (https://github.com/Okja-Engineering/reg-corpus/pull/8), which
  records this ADR, and its comment quoting the calls
  (https://github.com/Okja-Engineering/reg-corpus/pull/8#issuecomment-7089352475).
- Commits `7ec20c0` (goldens and corrupted fixtures, red) and `b06dfa2`
  (the parser, green): `internal/parse/parse.go` (`leads`, `place`),
  `internal/parse/doc.go`, `internal/parse/golden_test.go`,
  `internal/parse/rules_test.go` (`TestLabelPosition`, `TestLetterOrRoman`).
