# 0012. A label counts in a paragraph's lead; the next label decides (i)/(v)/(x)

- **Status:** Accepted
- **Date:** 2026-11-22
- **Decided by:** the owner, in chat (answers to the chief of staff's questions), recorded on PR #8: (1) 2026-11-22 19:41 CST, "Paragraph lead (Recommended)"; (2) 2026-11-22 20:43 CST, "Look one label ahead (Recommended)", which replaced the 19:41 answer "Close it (Recommended)" to the lookahead question

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

So the spec's goldens (c)(2)(v)(A)(*1*) and (c)(4)(ii)(B)(*2*) could not
exist, and neither could the anchor (c)(3)(i) that plan step 8's golden
41(c)(3)(i)-3 must resolve to. This is the text's layout, not a source
defect: an `overrides.yaml` entry would have to change verbatim text. In
PR #8, a parser that takes a label after a heading only past a dash (the
literal wording), and one that takes a label only at the start of a `<P>`,
each turn the goldens and the tree-shape test red.

The spec's "Carried forward" table also held:

> | (i)/(v)/(x) lookahead in R1a | before parsing any section where (h), (u) or (w) has children |

1024.41's (h) has children (1) to (4). At line 66, `(i)` can be placed
either way: as the letter after (h), or as the first roman under (h)(4). It
is the only label in 1024.41 that can be placed both ways; the next label,
(j), can be placed only after the letter.

The question at 19:41 said that without lookahead the only gap was a lone
roman (i) that is a section's last label. The chief of staff's record of
the 19:41 calls on PR #8 (comment 7089352475) gives "the one gap (a lone
roman (i) that is a section's last label)", and the 20:43 record (comment
7090167770) says "the first question said the only gap without lookahead
was a lone roman (i) at a section's end. That was wrong." Reviewers of PR #8
showed, and the parser at `b06dfa2` (no lookahead) confirms:
- an (i), (v) or (x) that ends a section where both readings can be placed
  is read as a letter with no error, at any depth: (a) to (u), (1), (i) to
  (v) gives the letter (v); (a) to (w), (1), (i) to (x) gives the letter
  (x);
- valid structure fails: (a) to (h), (1), (i), (ii) fails at (ii), "roman
  labels sit under number labels; the open levels are (i)", and no override
  can fix that, since the text is not a defect.

So the chief of staff asked again, and the owner answered at 20:43.

## Options considered
As asked by the chief of staff at 19:41, each option's text as offered:
- The label rule:
  1. **Paragraph lead (Recommended).** "A label counts in a paragraph's
     lead: at the start of the <P>, right after a heading past the dash or
     space that ends it, or right after another lead label. The lead ends at
     the first other text, so labels in running text never count. This is
     what's built and tested. It's recorded as ADR 0012 in this PR, amending
     R1a." Chosen.
  2. **Dash or period only.** "Same as the recommended rule, but a heading
     must end in "—" or in "." plus a space, so a heading ending any other
     way fails. It's stricter and fits today's XML. It needs a small code
     change."

  The chief of staff's note, not offered: all three 1024.41 cases above fit
  both options. They differ only where a heading that doesn't end in a
  period is followed by a space and a label: option 1 chains that label,
  option 2 would fail.
- The lookahead:
  1. **Close it (Recommended).** "No lookahead. The stated rule plus failing
     on any sequence break decides it. The spec's carried-forward row is
     removed in ADR 0012, which also records the one gap." Chosen at 19:41,
     replaced at 20:43.
  2. **Keep it open.** "No lookahead for 1024.41, where (j) confirms the
     reading. The item stays carried forward for Supplement I and later
     sources."
  3. **Add lookahead now.** "Before deciding (i), the parser also checks the
     label that follows. It costs more code and tests now."

As asked again by the chief of staff at 20:43, each option's text as
offered:
1. **Look one label ahead (Recommended).** "When (i)/(v)/(x) could be
   either a letter or a roman, the parser looks at the next label: the next
   letter means letter; the next roman, e.g. (ii), means roman. If there is
   no next label or it's neither, the parse fails. This closes the silent
   misread and parses the (h)(1)(i)(ii) shape. It's a small code change plus
   tests in this PR, and ADR 0012 records it." Chosen.
2. **Keep it closed.** "No code change. ADR 0012 and the spec's decisions
   row 12 state the full gap: the silent misread at a section's last label,
   and correct (h)(1)(i)(ii)-shaped sections that fail. These are decided
   again if a later source hits them."
3. **Fail on the silent case.** "No lookahead, but when an ambiguous label
   ends a section while a deeper level is open, the parse fails instead of
   reading a letter. That closes the silent misread, but correct
   (h)(1)(i)(ii) shapes still fail."

## Decision
The owner decided:
1. (19:41, "Paragraph lead (Recommended)") "A label counts in a
   paragraph's lead: at the start of the <P>, right after a heading past the
   dash or space that ends it, or right after another lead label. The lead
   ends at the first other text, so labels in running text never count."
2. (20:43, replacing "Close it") "When (i)/(v)/(x) could be either a letter
   or a roman, the parser looks at the next label: the next letter means
   letter; the next roman, e.g. (ii), means roman. If there is no next label
   or it's neither, the parse fails."

How it is carried out was the chief of staff's (an agent's) choice, not the
owner's call. Each choice and its reason:
- **When a label "could be either".** Only (i), (v) and (x) read both ways;
  (c), (d), (l) and (m) are letters only. Each of the three is a letter
  only when the last letter-level sibling directly precedes it, as R1a
  says; it could be either only when the roman reading can be placed too
  (a roman level could take it at that value). Otherwise its one placeable
  reading is taken, and if neither can be placed the roman reading's
  failure is reported. Why: R1a names only (i), (v) and (x), and its rule
  already decides every other case; reading (c) as a roman gave wrong
  failure reasons.
- **What "the next letter means letter; the next roman means roman"
  covers.** The next label in document order, in the same lead or the next
  `<P>`'s, is tried after each reading, and the reading after which it can
  be placed is taken. So a child counts as well as a letter or a roman: a
  letter (i) followed by (1); a roman (i) followed by (A), or by the number
  (2). If it can be placed after neither, or no label follows, the parse
  fails at the ambiguous label, naming both readings. Why: those labels
  decide it as surely as a letter or a roman, and the option's words don't
  name them. No label can be placed after both readings in this order of
  kinds (the reason is in `internal/parse/label.go`, `choose`), so that
  case never arises.
- **The same letter again: a departure from the owner's words.** When the
  next label is the same letter once more, as in (h)(1)(i) followed by
  (i), "the next letter means letter" would read the first (i) as the
  letter (i); the second (i) then can't be placed, and the parse fails. As
  carried out, the first (i) is read as the roman (h)(1)(i), because only
  that reading places the second (i), as the letter after (h). The same
  holds for (v) after (u)(1)(v) and (x) after (w)(1)(x). This departure is
  the chief of staff's choice, not the owner's call. Why: only that reading
  places the next label, so it is no guess, and the literal reading would
  reject a valid structure. The owner can overrule it; that case then
  fails instead.
- **What a heading is.** The `<I>` text after a label and exactly one
  space (U+0020) outside the `<I>` (`(b) <I>…</I>`). Why: it is how every
  heading in 1024.41 is written.
- **The dash or space that ends a heading.** Exactly one "—" or one space
  (U+0020), right after the heading's `</I>`. Why: the owner's rule says
  "past the dash or space that ends it"; a label past no separator, or any
  other, fails (below).
- **Where the lead almost goes on, it fails.** One function decides it
  (`follow` in `internal/parse/label.go`), and the package doc names each
  case. The run is the text after a label, or after a heading, up to the
  first letter or digit. Each case:
  1. A chained label: right after a label with nothing between, or right
     after a heading past exactly one "—" or one space. A bad label shape
     there fails.
  2. Any other "(" in the run fails as unplaceable, at that "(". If a label
     starts there, it is past the wrong separator: a space after a label
     (`(a) (1)`), or after a heading nothing, an en dash, a hyphen, two
     spaces, a no-break space or ". ". If none does, it is a "(" that starts
     no label (`(1 )`, `(1.)`, `()`, `(é)`), which fails as a "(" at the
     start of a `<P>` that starts no label does.
  3. Italic text in the run, or right after it, past anything but one space
     outside the `<I>` (nothing, two spaces, a no-break space, a quotation
     mark), or with a space first inside the `<I>`
     (`(a)<I> Head</I>—(1)`), fails as malformed, at its first italic
     character. A heading that begins with a label (an italic "(" too:
     `(a) <I>(</I>1)`), or holds a label after a dash ("—", "–" or "-"),
     fails as unplaceable at that label. Any other parenthesis in a heading
     is its text (`(a) <I>Self-help (see (1)).</I>`).
  4. Otherwise the lead ends at the run's end. A digit ends the run as a
     letter does: in `(a) 2(1) text.`, "(1)" is running text.

  Why: R1a never guesses, and reading such a label as running text would
  drop a possible anchor without a word. (Reviewers on PR #8 suggested
  keeping it as running text; that is not done, for this reason.) A
  misplaced heading fails as malformed, not unplaceable, because no label
  is out of place there, only the markup around a heading. A "(" that
  starts no label fails as unplaceable wherever a label goes, since it may
  be a label the source broke. A digit ends the run because the owner's
  rule ends the lead "at the first other text", and a digit is text.
- **A label after another in one lead opens the level below it.** A
  sibling or a higher level there ("(a)(b)", or "(a) <I>See</I> (b) of this
  section") fails as unplaceable; the level below at another value than its
  first fails as skipped, as any new level does. Why: in 1024.41 all 16
  labels after another in a lead open the level below at its first value,
  and the lead rule is what makes them structural.
- **The italic level.** An italic label is exactly `(`, a number in
  `<I>…</I>`, `)`. Italic parentheses, a label partly in italics and an
  italic letter fail as unplaceable. Why: it is how every italic label in
  1024.41 is written.
- **Where the dash or space after a heading goes.** It stays at the end of
  the parent's own text, so the own texts of one `<P>`, joined, are exactly
  its character data (`(b) Receipt of a loss mitigation application—`).
  Why: no character of the source is dropped or moved. Whether it counts in
  the operative-anchor test and the per-node hash, and how a multi-anchor
  cite joins across it or across a paragraph boundary, is plan step 5's
  (spec "Carried forward").
- **Each node records its `<P>`** (`para`, counted from 1, in the tree's
  JSON form). Why: the tree alone then gives back each paragraph's text and
  which anchors share a paragraph, which plan step 5's joins need.
- **What else fails as malformed.** CDATA, a namespaced element, a section
  with no N, no HEAD or no `<P>`, a processing instruction other than one
  XML declaration at the very start of the file, a character reference to a
  surrogate (U+D800 to U+DFFF), and character data outside HEAD, P and CITA
  that is not XML whitespace (a no-break space or a byte-order mark, say).
  Why: 1024.41's form holds none of them. The parser at `b06dfa2` already
  failed a byte-order mark, a `<P>` before the HEAD or with no HEAD before
  it, and a processing instruction inside the section or after it. It
  passed the rest without an error: CDATA read as text (a label in it
  counted, a "—" in it chained a label, and CDATA before a heading lost the
  heading and the label after it), a namespaced section read as this form,
  a section with no N read with an empty one, an empty section or one with
  no `<P>` read as an empty tree, a no-break space between elements
  dropped, a processing
  instruction before the section (a second XML declaration, or one not at
  the start) skipped, and a surrogate reference stored as U+FFFD.
- **Where an error is located.** A line ends at LF, at CRLF or at a lone
  CR, as XML reads line ends; an empty `<P>`, `<P></P>` or `<P/>`, is
  located at its start tag. Why: at `b06dfa2` a file with lone-CR line ends
  was one line, and an empty `<P>` was located after its start tag.

## Spec changes
- `docs/03-spec.md`:
  - Decisions-table row 12 is added: "Where does a label count, and how is
    an (i), (v) or (x) that could be either a letter or a roman decided? |
    A label counts in a paragraph's lead: at the start of the `<P>`, right
    after a heading past the dash or space that ends it, or right after
    another lead label; the lead ends at the first other text. When
    (i)/(v)/(x) could be either a letter or a roman, the parser looks at
    the next label: the next letter means letter; the next roman, e.g.
    (ii), means roman; if there is no next label or it's neither, the parse
    fails (as carried out: the reading after which the next label can be
    placed is taken, so the same letter again, as in (h)(1)(i) then (i),
    means roman; with no next label, or one placed after neither reading,
    the parse fails; the chief of staff's choice)."
  - R1a: "recognizes a label only at the start of a paragraph element, or
    right after the dash that ends a heading; labels inside running text
    (cross-references) are never structural" → "recognizes a label only in a
    paragraph's lead: at the start of a paragraph element, right after a
    heading past the dash or space that ends it, or right after another lead
    label; the lead ends at the first other text, and labels inside running
    text (cross-references) are never structural (decision 12)".
  - R1a: "decides between letter and roman "(i)", "(v)" and "(x)" by
    sequence: it is a letter only when the last letter-level sibling directly
    precedes it ("(h)", "(u)", "(w)")" → the same, then "; when it could be
    either, the next label decides: the reading after which the next label
    can be placed, and with no next label, or one placed after neither
    reading, the parse fails (decision 12)".
  - "Carried forward": the row "(i)/(v)/(x) lookahead in R1a | before
    parsing any section where (h), (u) or (w) has children" is removed, and
    a row is added: "Whether a heading's "—" or space left at the end of a
    parent's own text counts in the operative-anchor test and in the
    per-node hash, and how a multi-anchor cite joins own texts across a
    heading separator or a paragraph boundary (decision 12) | plan, step 5".
  - Header "Amended: 2026-11-22 by ADRs 0007–0011" → "… by ADRs 0007–0012".
- `docs/04-plan.md`:
  - Step 4: "**Label position:** a label counts only at the start of `<P>`
    or after a heading dash." → "**Label position:** a label counts only in
    a paragraph's lead: at the start of `<P>`, right after a heading past the
    dash or space that ends it, or right after another lead label; the lead
    ends at the first other text (ADR 0012)."
  - Step 4: "**The (i)/(v)/(x) rule:** it's a letter only when the last
    letter-level sibling directly precedes it." → "**The (i)/(v)/(x) rule:**
    it's a letter only when the last letter-level sibling directly precedes
    it; when it could be either, the next label decides, and with no next
    label, or one placed after neither reading, the parse fails (ADR 0012)."
  - Step 5 gains a bullet: "Decide whether a heading's "—" or space left at
    the end of a parent's own text counts in the operative-anchor test and
    the per-node hash, and how a multi-anchor cite joins own texts across a
    heading separator or a paragraph boundary (spec "Carried forward"; ADR
    0012)."
  - Risks, the eCFR labels row's mitigation: "Deterministic parser; labels
    recognized only by position; goldens written before the code, from the
    XML; fail on any sequence break" → "Deterministic parser; labels
    recognized only by position; an (i), (v) or (x) that could be either
    decided by the next label; goldens written before the code, from the
    XML; fail on any sequence break (ADR 0012)".
  - Header "Amended: 2026-11-22 by ADRs 0007, 0010 and 0011" → "… by ADRs
    0007, 0010, 0011 and 0012".
- `stages/01-fetch-parse/CONTRACT.md`, Requirements, R1a: "Labels count only
  at the start of a paragraph element or right after a heading dash;
  (i)/(v)/(x) resolved by sequence;" → "Labels count only in a paragraph's
  lead: at the start of a paragraph element, right after a heading past the
  dash or space that ends it, or right after another lead label; the lead
  ends at the first other text (spec decision 12). (i)/(v)/(x) resolved by
  sequence, and by the next label when it could be either;", with the item
  rewrapped.
- `docs/adr/README.md`, the index, gains the line "| [0012](0012-labels-count-in-the-paragraph-lead.md) | A label counts in a paragraph's lead; the next label decides (i)/(v)/(x) | Accepted |".
- `internal/parse/doc.go`: the package doc this PR writes states the label
  position and the (i)/(v)/(x) rule as decided here, and names each case
  where the lead almost goes on.
- No other lines. The intent and AGENTS.md say nothing about label position
  or the (i)/(v)/(x) rule.

## Consequences
- 1024.41 parses with no override: 91 nodes from 75 `<P>`, with the
  goldens (c)(2)(v)(A)(*1*) and (c)(4)(ii)(B)(*2*), and the anchor (c)(3)(i),
  in place. Its (i) at line 66 is read as the letter, because (j) follows.
- The gap recorded at 19:41 is closed: an (i), (v) or (x) that both
  readings can place and that ends a section fails, as does one whose next
  label fits after neither; (h)(1)(i), (h)(1)(ii) parses.
- A label after a heading that ends other than with one "—" or one space, a
  label after a label past a space, a sibling after a label in a lead, a
  heading placed other than past one space outside the `<I>`, a label
  after a dash in a heading, and a "(" in a lead that starts no label each
  fail the parse. A later source laid out that way needs a reviewed
  override, if it is a source defect, or an ADR.
- (h)(1)(i) followed by (i) parses as (h)(1)(i) and the letter (i), as do
  the same shapes under (u) and (w); if the owner overrules that choice,
  they fail.
- A sequence that is valid under these rules is read as that structure. If
  a source leaves out a label and what remains is still a valid sequence,
  the parse can't tell: (a) to (g), (1), (i) ending a section reads (i) as
  the roman (g)(1)(i).
- Plan step 5 inherits the dash or space at the end of a heading parent's
  own text (spec "Carried forward").

## Requirement IDs
R1a (deterministic parse: label position, the (i)/(v)/(x) rule); spec
"Carried forward"; decision 12.

## Links
- PR #8 (https://github.com/Okja-Engineering/reg-corpus/pull/8), which
  records this ADR; its comments quoting the calls: 19:41
  (https://github.com/Okja-Engineering/reg-corpus/pull/8#issuecomment-7089352475)
  and 20:43
  (https://github.com/Okja-Engineering/reg-corpus/pull/8#issuecomment-7090167770).
- Commits `7ec20c0` (goldens and corrupted fixtures, red), `b06dfa2` (the
  parser, green), `61646df` (the lookahead, lead, shape and malformed tests,
  red), `8f00eb5` (the parser that passes them, green), `fde0866` (the
  near-lead, processing-instruction, surrogate and location tests, red) and
  `c2edc86` (`follow` and the XML rules that pass them, green):
  `internal/parse/label.go` (`leads`, `follow`, `choose`, `fit`),
  `internal/parse/parse.go`, `internal/parse/doc.go`, and the tests
  `internal/parse/lookahead_test.go`, `internal/parse/lead_test.go`,
  `internal/parse/rules_test.go` and `internal/parse/malformed_test.go`.
