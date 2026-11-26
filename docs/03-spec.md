# Spec: regulatory corpus, phase 1

- **Author:** Matt
- **Status:** draft, revised after adversarial review 2026-11-21
- **Date:** 2026-11-21
- **Amended:** 2026-11-22 by ADRs 0007–0012; 2026-11-24 by ADR 0013; 2026-11-25 by ADR 0014
- **Intent:** `docs/01-intent.md` (accepted 2026-11-21)

## Decisions on the intent's open questions
Rows 1–5 answer the intent's open questions, and row 6 settles its CLI-only no-go. Rows 7 on are accepted ADRs (`docs/adr/`), which are part of this spec.

| # | Question | Decision |
|---|---|---|
| 1 | What is one obligation? | One duty-holder, one action, one deadline. A paragraph can hold several obligations, and one obligation can draw on several paragraphs. A notice's required contents belong to one obligation, held in `contents`. A permission is a record with `modality: permission`, and its limit (e.g. the (e)(1) 14- or 7-day floor) is a `deadline` with `bound: floor`. A deadline that depends on a condition stays one record, using `combine: by_condition`. |
| 2 | What evidence does each verification level require? | Three levels: `fetched` (staging only; no review), `text-verified` (a person checked every cited field against the text using `regcorpus verify`; quotes machine-matched by C6), `second-reviewed` (a second person re-checks the decomposition; rules carried forward). `registry/` holds only records at `text-verified` or above. |
| 3 | Hard gate or advisory? | Hard gate at whatever consumer level a repo adopts (R4), and always for the corpus repo (C6). In the corpus repo the gate is CI from phase 1, once `main`'s branch protection requires it (decision 7); its git pre-commit hook and the agent guard (R11) are seatbelts, which a person can bypass. In consumers, phase 1 is local only: a git pre-commit hook runs the check, a person can still bypass it, and CI closes that later. |
| 4 | Revision in IDs? | Yes. Citations take the form `ID@rev`. Any change to a record's meaning bumps `rev`, and so does a change to an anchor rule that applies to it. |
| 5 | Pass rule | Applied separately to each model (Opus 5.5 and Sonnet 5.5), over 5 runs per arm. A model passes only if all three hold: mean recall(c) ≥ 0.8; mean recall(c) − mean recall(a) ≥ 0.4; mean precision(c) ≥ 0.5. Pass on both = accept; one = revise; neither = kill. Pre-registered rules: <br>• mean recall(a) > 0.6 fails that model, and the fixture isn't reseeded; <br>• if only one model runs, or any in-scope record is below `text-verified` at run time, the best outcome is revise; <br>• arm (b) is diagnostic: if (c) passes and mean recall(c) − mean recall(b) < 0.1 on both models, record that lookups, not the check, carry the result, and move C1 and the strict level to phase 2. |
| 6 | Interface | CLI only in phase 1, with no server, API or MCP. Every phase-1 consumer has a shell, and the gate needs a binary anyway. |
| 7 | What is the corpus repo's gate? | CI: the `C6 gate` check runs C6 on the commit under test (for a pull request, GitHub's test merge commit), then `make test`. It is the gate once branch protection on `main` requires the `C6 gate` check and up-to-date branches (the owner's setting). The pre-commit hook and the R11 guard are seatbelts. [ADR 0007](adr/0007-ci-is-the-c6-gate.md) |
| 8 | What may an eval arm see? | No arm's run directory holds `key.yaml`; only grading reads it. [ADR 0008](adr/0008-no-arm-sees-key-yaml.md) |
| 9 | How do ADRs relate to this spec? | An accepted ADR is part of this spec. The intent, spec, plan, stage contracts and AGENTS.md never conflict with one: the PR that adds an ADR amends their text where it states what the ADR changes, and adds its row here. [ADR 0009](adr/0009-adrs-are-part-of-the-spec.md) |
| 10 | Who writes an L0 source's `meta.yaml`? | One shared writer, standard library only: it reads, checks and rewrites the file in spec order, and each plan step sets only its own fields through it, keeping the rest. `internal/l0` is the writer and `internal/meta` owns the schema (two packages: the chief of staff's choice). [ADR 0010](adr/0010-one-writer-owns-meta-yaml.md) |
| 11 | Which page does `version_fr_citations` record, and for which sources does plan step 3 set the version fields? | For 1024.41, 1024.30 and 1024.31: the amendment page, as the citation note (`<CITA>`) of the stored raw XML cites it, and plan step 3 sets both version fields for all three. Supplement I is open until plan step 8. [ADR 0011](adr/0011-fr-citation-is-the-amendment-page.md) |
| 12 | Where does a label count, and how is an (i), (v) or (x) that could be either a letter or a roman decided? | A label counts in a paragraph's lead: at the start of the `<P>`, right after a heading past the dash or space that ends it, or right after another lead label; the lead ends at the first other text. When (i)/(v)/(x) could be either a letter or a roman, the parser looks at the next label: the next letter means letter; the next roman, e.g. (ii), means roman; if there is no next label or it's neither, the parse fails (as carried out: the reading after which the next label can be placed is taken, so the same letter again, as in (h)(1)(i) then (i), means roman; with no next label, or one placed after neither reading, the parse fails; the chief of staff's choice). [ADR 0012](adr/0012-labels-count-in-the-paragraph-lead.md) |
| 13 | How is text normalized, which anchors are operative, how does a multi-anchor cite join, and what do the hashes cover? | Normalized text is NFC, then the curly quotes U+2018, U+2019, U+201C and U+201D mapped to ASCII and the en and em dashes to `-`, then every run of Unicode White_Space collapsed to one space, never trimmed. An anchor is operative when its own text, minus its label, the one space before its heading, the heading, and the one dash or space that ends the heading (one `-` or one space, read on the whole own text normalized as one string), is non-empty. A multi-anchor cite joins own texts in document order with nothing between two in one `<P>` and one space between `<P>`s, and is normalized after joining. A node's `sha256` covers its whole normalized own text, a heading's separator included; `sha256_text` covers the section heading, one space and every anchor joined, normalized as one string, not the citation note; for the Supplement I 41() slice it is open until plan step 8 (the owner's calls; the normalization detail, the separator read normalized, `text.md`'s form, and the offline `regcorpus parse` that writes `tree.json`, `text.md` and `sha256_text`: the chief of staff's choices). [ADR 0013](adr/0013-cites-join-as-the-document-reads.md) |
| 14 | What makes a re-served copy the same version, how is govinfo's text compared, what date does `verified_on` hold, and what does `fetch --latest` do past exiting non-zero? | A version is its date plus its normalized text: the tree's node paths in order, each node's `sha256`, and `sha256_text`. `fetch --verify` first derives the stored `tree.json`, `text.md` and `sha256_text` again from the raw XML, and refuses, writing nothing, when one differs; then it compares eCFR's latest version, read from its version list, which must be `source_version_date`, with them, printing both `sha256_raw` values and the nodes whose raw text differs as information only; then, only if that matches, the section's granule in the govinfo CFR 2026 annual edition (title 12, revised as of 2026-01-01), through govinfo's link service, read only in the form a granule has (no CR; inside its root no CDATA section and no character reference to a White_Space character; one or more `<P>`s, then the citation note once and last): each line break and up to 12 spaces after it are deleted as layout and nothing else is deleted or trimmed, the heading is SECTNO, one space, SUBJECT, a paragraph keeps `<E>`'s text and drops PRTPAGE, the citation note is left out, and the heading and each paragraph, each side normalized by decision 13 alone, must equal the stored version's, derived again, one for one, and joined and normalized as decision 13 joins them, hash to `sha256_text`; so a change in govinfo's layout can give a false match in one way only: a maximal run of one or more literal LFs, each followed by up to 12 literal spaces, standing where govinfo meant a word space that the stored text lacks. `verified_on` is the UTC date of the first matching run, kept by a later match and overwritten with `null` by a mismatch, which prints each differing paragraph and exits 1; only then is the granule kept in the git-ignored `.cache/govinfo/`, a cache, never committed. `fetch --latest` reads `titles.json`, each `meta.yaml` (checked) and the version lists, and writes nothing: eCFR's latest version newer or older than the stored one, or one that removed the section, exits 1 with the stored and latest dates; an entry for the stored date issued on or after `retrieved_at`'s day is printed as information. Stage 04's release check runs `fetch --verify` too. No schema change (the owner's calls: the normalized text as the version, govinfo compared exactly with its layout deleted, the first matching run's date, and `--latest` reporting only; the bound of 12, the granule, its form and cache, the stored files derived again, the paragraph-by-paragraph comparison with the stored version, the order, the exit codes, the issue-date note and the sources each flag covers: the chief of staff's rulings; the citation note once and last in the granule's form: the implementer's choice). [ADR 0014](adr/0014-normalized-text-is-the-version.md) |

## Definitions
- **Anchor:** a stable ID for one labeled unit of L0 text: a regulation paragraph at any depth (`1024-41-b-2-i-B`), one official comment (`1024-41-b-2-i-interp-1`), or a definition (`1024-31-day`).
- **Own text:** an anchor's text excluding its children, with its label and heading.
- **Operative anchor:** an anchor whose own text, minus its label, the one space before its heading, the heading, and the one dash or space that ends the heading, is non-empty; it is read on the whole own text normalized as one string, where that separator is one `-` or one space after the normalized heading (decision 13). Leaves and lead-ins both count; e.g. `1024-41-b-2-i`, whose lead-in holds the 45-day condition. Coverage (R3) is checked at operative anchors.
- **Conditional lead-in:** an operative anchor with children whose own text contains "if", "unless", "except", "provided that" or "notwithstanding".
- **Record:** one L1 entry, either `kind: obligation` or `kind: anchor_rule` (a rule that changes how other records apply, e.g. facial completeness).
- **Cited field:** a field that restates the rule and must carry `cite` and `text`:
  - `action`, `trigger` and `effect`
  - `deadline`, each of its `parts`, and each part's `when`
  - each entry in `conditions`, `exceptions`, `contents` and `release_conditions`

  Label fields (`title`, `summary`, `duty_holder`, `modality`) are not checked against the text.
- **Normalized text** (decision 13), in this order:
  - Unicode NFC (not NFKC)
  - curly quotes mapped to ASCII (U+2018 and U+2019 to `'`, U+201C and U+201D to `"`); en and em dashes (U+2013, U+2014) mapped to `-`; no other character is mapped
  - every run of Unicode White_Space (including U+00A0) collapsed to one space, never trimmed

  C6 matches `text` against normalized own text. Each `tree.json` node carries the `sha256` of its whole normalized own text, a heading's separator included, and `sha256_text` hashes the normalized source: the section heading, one space, then every anchor's own text joined as a multi-anchor cite joins them, normalized as one string, without the citation note. For the Supplement I 41() slice, what `sha256_text` covers is open until plan step 8.
- **Multi-anchor cite:** `cite` may be a list. The anchors' own texts are joined in document order, with nothing between two in one paragraph (`<P>`) and one space between paragraphs, so the join reads as the document does; it is normalized after joining (decision 13). A quote may cross a join only between anchors that are consecutive in the document.
- **Quote:** every `text` is at least 4 words.
- **Cite-only source:** an L0 source that records may cite, but which is neither inventoried nor extracted (1024.30, 1024.31).
- **Tier and effective date:** set per L0 source. Every cite inherits both from its anchor's source; records carry neither of their own.
- **In scope (consumer):** every active record whose anchors fall under a section listed in the consumer's `scope`.

## Requirements
**R1 Rule text (L0).** Store 12 CFR 1024.41 and its Supplement I comments (the 41() portion) verbatim, plus 1024.30 and 1024.31 as cite-only sources. Each source records:
- its tier
- the source version date
- `version_effective_date` and `version_fr_citations`
- the retrieval date
- the eCFR "up to date as of" date
- `verified_on`
- raw and normalized hashes
- `official: false`

**R1a Deterministic parse.** L0 anchors and `text.md` are generated from `tree.json`. That is a nested tree produced from eCFR XML by a rule-based parser; no heuristics or models are involved. The parser:
- infers depth from the label sequence (letter → number → lowercase roman → capital letter → italic number)
- recognizes a label only in a paragraph's lead: at the start of a paragraph element, right after a heading past the dash or space that ends it, or right after another lead label; the lead ends at the first other text, and labels inside running text (cross-references) are never structural (decision 12)
- decides between letter and roman "(i)", "(v)" and "(x)" by sequence: it is a letter only when the last letter-level sibling directly precedes it ("(h)", "(u)", "(w)"); when it could be either, the next label decides: the reading after which the next label can be placed, and with no next label, or one placed after neither reading, the parse fails (decision 12)
- **fails** on any skipped, repeated or out-of-order label, and on any label it can't place; it never guesses
- **Supplement I:** comment headings ("41(x)…" or "Paragraph 41(x)…") must resolve to an existing regulation anchor and may skip; comment numbers ("1.", then "i.") are sequential within a heading
- **1024.31:** anchors each unlabeled definition paragraph by its defined term (`1024-31-day`), and fails on a definition paragraph with no recognizable term
- **Overrides:** a source defect is fixed only through a reviewed `overrides.yaml` entry (location, replacement, reason) applied before parsing, and listed by `fetch --verify` once applying overrides is built (until then `fetch --verify` refuses a source with an `overrides.yaml`; decision 14); never in parser code
- **Goldens** are built from eCFR or official-edition XML only, never from secondary sites

**R1b Newer-version check.** `regcorpus fetch --latest` exits non-zero in either case:
- eCFR lists a version of any stored section dated after its `source_version_date`
- a freshly fetched Supplement I 41() slice's normalized hash differs from the stored one

It runs before every release. Future-effective rules not yet in eCFR stay with the phase-2 Federal Register watch.

**R2 Registry (L1).**
- Every obligation and anchor rule in 1024.41 and its 41() comments has exactly one record.
- Every cited field carries `cite` and `text`.
- `deadline.anchor_event` comes from the fixed event list.
- `registry/` holds only records at `text-verified` or above; C6 fails any other.

**R2a Extraction contract.** Extraction is the only step where a model does judgment work, so it is bounded:
- **Input:** one extraction unit, listed in `stages/02-extract/units.yaml`, which covers every operative anchor exactly once. A unit is a paragraph with its whole subtree, e.g. (c)(3)(i) with (A)–(F). The input also includes the unit's ancestors' own text, the own text of anchors it cross-references, the record schema, and the golden records.
- **Golden records:** `stages/02-extract/golden/` holds one hand-verified record per shape:
  - a notice with `contents`
  - an earliest-of deadline (comment 41(b)(2)(ii)-2)
  - a by-condition deadline ((e)(1))
  - a floor ((e)(1) or comment 41(b)(2)(ii)-3)
  - a prohibition with `release_conditions` ((g))
  - an anchor rule with `effect` ((b)(3))
- **Output:** proposed records in the fixed schema, written to `staging/` at `fetched`. A model never writes to `registry/`.
- **Gate:** `regcorpus check --staging` runs C6 on staged records, and fails when two staged records share the same `action` cite and text. `regcorpus verify <ID>` is the only path into `registry/`. It:
  - requires an interactive terminal
  - walks a person through each cited field beside its anchor text
  - writes `verification` and moves the record
  - appends `ID, rev, record sha256, by, on, method` to `registry/verify.log`
- **Contract:** the extraction prompt is a versioned file, and each record names the prompt version that produced it, or `manual` if a person wrote it.

**R3 Inventory.** Every operative anchor in 1024.41 and its 41() comments is either cited by a cited field of at least one active record, or marked `na` with a reason. Anchor-to-record mappings are derived from cites, never written by hand. Each inventory entry carries `reviewed: {by, on}`, a person's sign-off that every duty in that anchor has a record.

**R4 Consumer levels.** A consuming repo opts into one level, and each level includes everything below it:

| Level | What the repo gets | Opt in by | Gate fails on |
|---|---|---|---|
| read | CLI lookups and the agent skill | installing the binary | nothing |
| cite | Citations must resolve and be current | `regcorpus.lock` | C2–C4 |
| cover | Every in-scope obligation addressed, marked `na`, or waived | `regcorpus.applicability.yaml` with `scope:` | C2–C5 |
| strict | Uncited regulatory claims fail | `level: strict` in the applicability file | C1–C5 |

A lock file with no applicability file means cite. Strict is never the default.

**R5 Corpus check.** In the corpus repo, the gate fails on C6 (per-commit rules). The decision gate and releases also require `--complete`.

**R6 Releases.**
- Releases are immutable git tags with `VERSION` and `CHANGES.json`.
- Consumers pin a release.
- Every ID in the previous release still exists in the next one, either active or marked superseded or withdrawn.
- `regcorpus release` fails while any active record is below `text-verified`.

**R7 Evaluation.** Run the seeded fixture through three arms on two models, five runs each (see Evaluation below). Grade every run with the committed judge and the blind author sample, record the results, and apply decision 5.

**R8 Lookup.** Agents and people read the corpus only through deterministic CLI lookups:
- `regcorpus show <ID>[@rev]` prints one record with the verbatim text of its anchors, plus every anchor rule that applies to it.
- `regcorpus text <anchor>` prints one anchor's text and its source metadata.
- `regcorpus topics <term>` prints the IDs mapped to that term in the hand-built `registry/topics.yaml`. Matching is exact and case-insensitive on terms and their aliases; there is no fuzzy search. An unknown term exits non-zero and lists the available terms.

Output is plain text by default, with `--json` for tools. No command prints more than the records asked for.

**R9 Agent skill.** The release ships `skill/regcorpus/SKILL.md`. It has a one-line description that triggers on work touching Reg X loss mitigation, and a body under 100 lines covering:
- the lookup commands
- the citation syntax
- the rule to look things up, never answer from memory

It contains no rule content.

**R10 Walk test.** The corpus repo has a root `AGENTS.md`, under 100 lines, giving the layout, the stages and the rules. Each build stage has a numbered folder whose `CONTRACT.md` states its inputs, its outputs, the check that ends it, and who may run it:
- `stages/01-fetch-parse`
- `stages/02-extract`
- `stages/03-verify`
- `stages/04-release`
- `stages/05-evaluate`

A fresh agent session with no prior context must reach the right stage and its contract from these files alone.

**R11 Agent guard.** The corpus repo ships `.claude/settings.json` with deny rules and a PreToolUse hook that blocks:
- `--no-verify` and `commit -n`
- edits and writes under `registry/`, `.githooks/` and `.claude/`
- running `regcorpus verify`

`make setup` sets `core.hooksPath`. These are guards, not proof: the gate stays C6, and `verify` requires a terminal.

### Acceptance
Each item lists the command or evidence that proves it.

**Decision gate (by 2026-12-20)**
- [ ] **R1:** `regcorpus fetch --verify` exits 0: eCFR's latest version is dated `source_version_date`, and its tree (node paths in order, each node's `sha256`) and `sha256_text` match a fresh fetch (`sha256_raw` is informational). `verified_on` is set by comparing normalized text to the govinfo 2026 edition, its layout deleted first (decision 14). The output is saved.
- [ ] **R1a:** `make test` passes these goldens:
  - letter (i) after (h), and roman (i)
  - a heading-dash chained label ((b)(2)(i)(B)) and (c)(2)(iv)
  - the italic level ((c)(2)(v)(A)(*1*) and (c)(4)(ii)(B)(*2*))
  - a cross-reference sentence that must not split
  - a skipped commentary heading
  - the `1024-31-day` definition

  A corrupted-sequence fixture exits non-zero.
- [ ] **R1b:** `regcorpus fetch --latest` exits 0 live, and non-zero on a fixture whose stored version is one version back.
- [ ] **R2/R3:** `regcorpus check --corpus --complete` exits 0. Its report shows:
  - 100% of operative anchors cited or `na`, and `reviewed`
  - every registry record at `text-verified`, with a matching `verify.log` line
  - the blind re-derive disagreement rate
- [ ] **R2a:** every record names its prompt version; `check --staging` fails on a bad quote and on a duplicate proposal.
- [ ] **R4/R5:**
  - `regcorpus check` exits 0 on `eval/levels/strict`.
  - On `eval/fixture`, it exits non-zero and reports exactly the seeds that `key.yaml` marks `check_detectable`.
  - Negative fixtures for C1, C5 and C6 each exit non-zero and print their rule ID.
- [ ] **R7:** the results file has 30 graded runs with recall, precision, trace rate, model ID and transcript path; the judge-to-author agreement rate; and the decision 5 outcome per model, with the final decision.
- [ ] **R8:** `regcorpus show RX-41-ACK-01` prints the record with its verbatim anchor text and its anchor rules. `regcorpus topics acknowledgment` prints the ID. An unknown term exits non-zero.
- [ ] **R9:** the skill exists, its body is under 100 lines, and C6 finds no 8-word run of anchor text in it.
- [ ] **R10:** given only the repo, a fresh session asked to verify the next record names `stages/03-verify/CONTRACT.md` and the correct staged record. The transcript and corpus commit are saved under `eval/walk/`.
- [ ] **R11:** in a Claude Code session in the corpus repo, `git commit --no-verify`, an edit under `registry/` and `regcorpus verify` are each blocked. The transcript is saved under `eval/guard/`.

**Phase 1 done (only after accept)**
- [ ] **R4 levels:** fixtures for read, cite and cover each pass clean and fail only on their own rules.
- [ ] **R6:** tag `v0.1.0` exists, and the fixture's `regcorpus.lock` pins it.
- [ ] Negative fixtures for C2–C4.

## Design

### Granularity
- **L0:** one file set per CFR section. eCFR versions whole sections and whole appendices, not paragraphs. Supplement I is fetched whole and sliced to its 41() comments.
- **Change detection** is per anchor: each node has its own hash, and records store the hashes of the anchors they cite. An amendment marks stale only the records that cite changed text.
- **L0 files per source:** `raw/<date>.xml`, `tree.json`, `text.md`, `meta.yaml`, and an optional `overrides.yaml`.
- **L1 and indexes:** one file per record, one inventory file per section, and `registry/topics.yaml`, which maps terms to IDs.

### L0 `meta.yaml`
```yaml
cfr: "12 CFR 1024.41"
tier: law                          # law | regulator_interpretation | investor_guide | standard
role: inventoried                  # inventoried | cite_only
source: ecfr
official: false
source_version_date: <YYYY-MM-DD>  # eCFR version date
version_effective_date: <YYYY-MM-DD>   # effective date of the change that produced this version
version_fr_citations: ["<vol> FR <page>"]  # 1024.41, 1024.30, 1024.31: the amendment page the raw XML's citation note (<CITA>) cites (decision 11); Supplement I: open until plan step 8
retrieved_at: <YYYY-MM-DD>
ecfr_up_to_date_as_of: <YYYY-MM-DD>
verified_on: <YYYY-MM-DD or null>  # UTC date of the first run whose normalized text matched the govinfo 2026 edition (decision 14); null on mismatch, the diff in the saved output
sha256_raw: <hash>                 # informational
sha256_text: <hash>                # normalized source
```
The Supplement I slice has its own `meta.yaml`, with `tier: regulator_interpretation`.

### L1 record
```yaml
id: RX-41-ACK-01
rev: 1
prompt_version: extract-v1         # or `manual`
kind: obligation                   # obligation | anchor_rule
status: active                     # active | superseded | withdrawn
superseded_by: []                  # one or more IDs; required when superseded
title: Acknowledge receipt of a loss mitigation application
modality: obligation               # obligation | prohibition | permission
duty_holder: servicer
action:  { summary: <label>, cite: <anchor>, text: "<verbatim>" }
trigger: { cite: <anchor>, text: "<verbatim>" }
deadline:                          # optional: omit when the text sets no time
  amount: 5
  day_type: excluding_weekends_holidays   # calendar | excluding_weekends_holidays | entity_business_day | qualitative
  day_type_cite: <anchor>          # required for calendar: 1024-31-day
  anchor_event: application_received
  direction: after                 # after | before
  bound: ceiling                   # ceiling ("within", "no later than") | floor ("no earlier than", "at least")
  inclusive: true                  # false for "more than" / "less than"
  cite: <anchor>
  text: "<verbatim>"
# compound form, used instead of the above:
#   deadline: { combine: earliest_of | by_condition, parts: [<deadline>, …] }
#   each by_condition part adds `when: { cite, text }`
contents: []                       # required contents of a notice; each { cite, text }
conditions: []
exceptions: []
release_conditions: []             # prohibitions: what ends them; each { cite, text }
verification:
  level: text-verified             # text-verified | second-reviewed (registry); fetched (staging only)
  by: <name>
  on: <YYYY-MM-DD>
  method: proposal_review          # proposal_review | blind_rederive
  source_hashes: { <anchor>: <node sha256> }   # every anchor the record cites
```

**Anchor events (fixed list; adding one takes a spec change):**
- `application_received`
- `application_complete`
- `application_facially_complete`
- `acknowledgment_notice_provided`
- `determination_notice_provided`
- `offer_provided`
- `appeal_made`
- `foreclosure_sale_date`
- `first_notice_or_filing`
- `delinquency_start`
- `servicing_transfer_date`

**Qualitative deadlines** ("promptly", "reasonable date") use `day_type: qualitative` with no amount.

**Anchor rules:**
```yaml
id: RX-41-FACIAL-01
kind: anchor_rule
effect: { summary: <label>, cite: <anchor>, text: "<verbatim>" }
substitution: { from: application_complete, to: application_facially_complete }   # optional; pure event swaps only
applies_to: [<record IDs>]
conditions: [{ cite: <anchor>, text: "<verbatim>" }]
```
- `regcorpus show <ID>` prints every anchor rule whose `applies_to` lists the record.
- A `rev` bump of an anchor rule bumps the `rev` of every record in its `applies_to`.
- The other known anchor rule is (b)(3): sale proximity is measured as of the complete application, and no scheduled sale counts as more than 90 days. It uses `effect` and `conditions`, with no `substitution`.

**Superseded vs withdrawn:** superseded means replaced by one or more records (`superseded_by` is required). Withdrawn means the source no longer imposes the duty.

### Inventory
Records are derived from cites, so each entry carries only a `reviewed` sign-off or an `na`:
```yaml
1024-41-b-2-i: { reviewed: { by: <name>, on: <date> } }
1024-41-a:     { na: "<reason>", reviewed: { by: <name>, on: <date> } }
```

### Consumer side
- **Citation syntax** in specs: `[obl:RX-41-ACK-01@1]`. A citation without `@rev` is malformed and fails C2.
- **`regcorpus.lock`:** `{ version, ref, sha }` of the corpus snapshot.
- **`regcorpus.applicability.yaml`:**
  - `level: cover` or `level: strict`
  - a `scope:` list of sections
  - one status per in-scope ID:
    - `addressed: <path>`: the file must exist and contain `[obl:ID@rev]`
    - `na: { why: not_our_role | product_out_of_scope | covered_elsewhere, note: "<text>" }`
    - `waived: <reason>` with `expires: <YYYY-MM-DD>`
    - `untriaged`: written by `regcorpus upgrade` for IDs added in a new release

### Check rules
| Rule | Fails when | Repo |
|---|---|---|
| C1 | A paragraph matches the initial pattern set and has no `[obl:…]`. The pattern set covers day counts as digits or number words ("5 days", "five days"), ordinals ("45th day"), "business day", "calendar day", "before a foreclosure sale", and CFR citations. C1 checks only that a citation is present, not that it's right. | consumer |
| C2 | A cited ID isn't in the pinned snapshot; a citation lacks `@rev`; or `regcorpus.lock` doesn't match the embedded snapshot. | consumer |
| C3 | A cited `@rev` isn't the revision in the pinned snapshot. | consumer |
| C4 | A cited ID is superseded or withdrawn. | consumer |
| C5 | An in-scope ID is missing from applicability; is `untriaged`; has an expired waiver; or is `addressed` at a path that doesn't exist or doesn't cite it. Output prints counts of addressed, `na` (by `why`) and waived. | consumer |
| C6 | Per commit: see the list below. | corpus |
| C6 `--complete` | Decision gate and release: an operative anchor neither cited nor `na`; an inventory entry without `reviewed`; an active record in no topic. | corpus |

**C6 per-commit failures:**
- **Quotes:**
  - a cited field missing `cite` or `text`
  - a quote under 4 words, or not found at its anchors under the join rule
- **Values the quote must support:**
  - `deadline.amount` not present in `deadline.text` as digits or a number word
  - `excluding_weekends_holidays` without "Saturdays" in the quote
  - `calendar` without `day_type_cite: 1024-31-day`
  - `qualitative` with an `amount`
- **Lead-ins:** a record citing a child of a conditional lead-in that doesn't also cite the lead-in.
- **Hashes:** a mismatch on any per-anchor `source_hashes`, or on a section hash for L0.
- **Registry state:**
  - a registry record below `text-verified`, with incomplete `verification`, or with no matching `verify.log` line
  - a record with no `prompt_version`
- **References:**
  - an `applies_to` or `superseded_by` ID that doesn't exist
  - `superseded` with an empty `superseded_by`
  - an ID from the previous release missing
  - a topic mapping an ID that doesn't exist
- **Skill:** any 8-word run of anchor text in the skill file.

**Suppressing C1:** use `[obl:none reason="…"]`. An empty reason fails C1, and `regcorpus check` reports the suppression count.

**Enforcement:** in the corpus, CI's `C6 gate` check runs per-commit C6 and is the control once `main`'s branch protection requires it (decision 7); the pre-commit hook and the R11 guard are seatbelts. In consumers, in phase 1 the pre-commit hook runs the adopted level; CI takes over later. An agent hook running the check on edit is a convenience, not the control.

### Releases and distribution
- **Version numbers:**
  - **Major:** a `rev` bump, supersession or withdrawal (existing citations break).
  - **Minor:** new records (consumers must triage).
  - **Patch:** metadata or verification changes only.
  - **Before 1.0.0:** any release may break, so consumers pin exactly.
- **`CHANGES.json`** lists IDs under `added`, `rev_bumped`, `superseded`, `withdrawn` and `verification_changed`.
- **Separate versions:** the corpus version is independent of regulatory effective dates.
- **Distribution:**
  - `make build CORPUS_REF=<tag or commit>` embeds one snapshot (registry, inventory, topics and L0) with Go `embed`.
  - `regcorpus version` prints the snapshot's ref and sha.
  - `make build LOOKUP_ONLY=1` omits `check`; eval arm (b) uses this build.

### Upstream
- **Rule text:** fetched point-in-time from the eCFR versioner API. `fetch --latest` (R1b) detects newer versions by date; a reissue of a stored date is compared by `fetch --verify` (decision 14).
- **Official-edition check:** govinfo's CFR 2026 annual edition, for `verified_on`: each section's granule XML, through govinfo's link service, kept at run time only in the git-ignored `.cache/govinfo/`, a cache never committed; a granule may be committed apart from that as a test fixture, as 1024.41's is (decision 14).
- **Change labels:** the Federal Register supplies the effective date and the amending document; for 1024.41, 1024.30 and 1024.31, `version_fr_citations` records the amendment page as the stored raw XML's citation note cites it (decision 11).
- **Network:** eCFR, govinfo and the Federal Register API are blocked from the cloud workspace, so fetch steps run on the owner's machine.
- **Carried forward:** a Federal Register watch for future-effective rules.

### Evaluation (R7)
- **Fixture source** (`eval/fixture-src/`, in CFR-cite form): a spec, an applicability file listing every operative anchor's status at `level: strict`, and `key.yaml`.
  - It is frozen, with its hash recorded, before extraction.
  - `eval/render` then maps anchors to IDs through the derived inventory, producing `eval/fixture/`. An ID takes the status of its `action` anchor.
- **Seeds:** ten, two of each type: missing trigger, wrong date anchor, missing exception, missing interim protection, too-narrow scope.
  - **At least four are `check_detectable`:** C5 (an in-scope duty absent from applicability, or `addressed` at a path that doesn't cite it), or C1 (an uncited numeric claim).
  - **At least four aren't:** wrong content in a cited claim, or `na`/`addressed` without basis.
  - **Selection:** seed anchors are drawn by lowest `sha256(seed‖anchor)` from per-type eligibility lists committed before selection.
- **`key.yaml`:** per seed `id`, `location`, `type`, `check_detectable: C1 | C5 | false`, and `must_say` (the substance a finding has to state).
- **Arms:** five runs each per model, all with the same prompt ("review this spec for regulatory errors; one finding per line with location and problem"):
  - (a) L0 text files + `fixture-src` without `key.yaml`
  - (b) L0 + the lookup-only binary + the skill + `fixture`
  - (c) as (b), but with the full binary and the `regcorpus check` output

  Every run uses project-only settings (no user config, memory, skills or MCP), with web tools disallowed. The skill is installed as a project skill in arms (b) and (c). No arm's run directory holds `key.yaml`; only grading reads it (decision 8).
- **Grading:**
  - **A catch** is a finding at the seed's location that states its `must_say`.
  - **The judge:** a committed prompt (`eval/judge-v1.md`, written before any run) grades normalized outputs. Normalization maps IDs to CFR cites, strips arm, model and `regcorpus` mentions, and shuffles findings.
  - **Author checks:** the author blind-adjudicates unmatched findings (real or false) and blind-grades a random 20%. Below 90% agreement, the author's blind grades replace the judge's.
- **Metrics per run:**
  - recall
  - precision: (catches + adjudicated real) ÷ findings
  - trace rate: seeds whose anchor or ID the run looked up; diagnostic only, and it answers "how often"
- **Results:** per-run grades and metrics, model ID string, CLI version, exact command, transcript path, fixture hash, and the decision 5 outcome.
- **Known limits:**
  - Five runs per arm is still small.
  - The author writes the seed text.
  - The judge is a model.
  - A strong arm (a) is a legitimate fail: it means text alone is enough on this fixture.

## Concerns
| # | Concern | Owner | Resolution |
|---|---|---|---|
| 1 | The intent bars free paraphrase, but L1 fields are structured restatements. | Matt | **Resolved:** cited fields carry verbatim quotes. C6 checks the quotes and the derived values, and a person verifies the rest, with a blind re-derive sample measuring their error rate. |
| 2 | Putting the commentary in phase 1 roughly doubles extraction. | Matt | **Resolved:** keep it. Commentary-only duties are the omission case the intent exists for. |
| 3 | Appetite vs. scope. | Matt | **Resolved:** four weekends plus a buffer week. Off-path work waits for accept, and the fallbacks cap the outcome at revise (see plan). |
| 4 | `verified_on` needs an official edition. | Matt | **Resolved:** the govinfo 2026 edition (revised as of 2026-01-01) is available. Set it by normalized-text comparison. |

## Carried forward
| Item | When |
|---|---|
| eCFR lists 1024.41 versions on both 2025-05-16 (publication of FR 2025-08643) and 2025-07-15 (its effective date). The same pattern appears in 2021 (2021-06-30 published, 2021-08-31 effective), which suggests eCFR also records a version on the publication date. Diff the point-in-time XML locally, then set `version_fr_citations` and `version_effective_date`. | plan, step 3 |
| Rules for `second-reviewed`, and who grants it | phase 2 |
| A Federal Register watch for future-effective rules | phase 2 |
| Investor-guide tier records | phase 2+ |
| C1 pattern expansion from `topics.yaml`, and C1 recall measured on the fixture's labeled claims | after the decision |
| `regcorpus init` (writes the consumer hook) | first non-fixture consumer |
| Whether `fetch --verify` against a stored version dated after the govinfo edition's 2026-01-01 records `verified_on: null` or refuses, since that edition can't hold it (moot while every stored version predates 2026-01-01; decision 14) | the first stored version after 2026-01-01 |
