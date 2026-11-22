# Plan: regulatory corpus, phase 1

- **Author:** Matt
- **Status:** draft, revised after adversarial review 2026-11-21
- **Date:** 2026-11-21
- **Amended:** 2026-11-22 by ADR 0007
- **Spec:** `docs/03-spec.md`
- **Appetite:** four weekends plus the following week as buffer; decision by 2026-12-20

**Stack:**
- Go: one CLI, `regcorpus`
- Dependencies: `gopkg.in/yaml.v3` and `golang.org/x/text/unicode/norm`
- Git and GitHub; the gate is CI's `C6 gate` check, once `main`'s branch protection requires it (step 1), and the pre-commit hook plus the R11 agent guard are seatbelts (ADR 0007)

## Files that change
New repo `regcorpus/`:
```
AGENTS.md                         # walk test: layout, stages, rules (R10)
.claude/settings.json             # R11: deny agent writes to registry/**, .githooks, .claude; block --no-verify and `regcorpus verify`
stages/0{1..5}-*/CONTRACT.md      # fetch-parse, extract, verify, release, evaluate
stages/02-extract/units.yaml      # extraction units; every operative anchor exactly once (R2a)
stages/02-extract/golden/         # six hand-verified shape records (R2a)
prompts/extract-v1.md             # versioned extraction prompt
staging/                          # model-proposed records at `fetched`
skill/regcorpus/SKILL.md          # agent skill, no rule content (R9)
go.mod
Makefile                          # setup, build (CORPUS_REF, LOOKUP_ONLY), test, check, negative, levels, release
.githooks/pre-commit              # per-commit C6 (a seatbelt)
.github/workflows/c6.yml          # the C6 gate (ADR 0007): .github/c6.sh runs C6 with the checker .githooks/build-checker.sh builds
cmd/regcorpus/main.go             # fetch, check, inventory, verify, upgrade, release, show, text, topics, version
internal/ecfr/                    # versioner client; full/{date} XML
internal/parse/                   # deterministic label state machine: XML → tree.json (R1a)
internal/anchors/                 # tree → anchors, text.md, normalization, per-node hashes
internal/lookup/                  # show, text, topics
internal/registry/                # record, anchor-rule, inventory types; schema validation
internal/verify/                  # interactive verification; verify.log
internal/check/                   # C1–C5 (consumer), C6 per-commit and --complete (corpus)
internal/release/                 # VERSION, CHANGES.json, embedded snapshot
sources/ecfr/title-12/1024.41/    # raw/<date>.xml, tree.json, text.md, meta.yaml, overrides.yaml (if needed)
sources/ecfr/title-12/supplement-i/41/   # whole raw appendix; 41() slice tree/text/meta
sources/ecfr/title-12/{1024.30,1024.31}/ # cite-only L0
registry/records/RX-41-*.yaml     # text-verified only
registry/verify.log               # append-only; written only by `regcorpus verify`
registry/inventory/1024.41.yaml   # every operative anchor: `na` and `reviewed`; mappings derived
registry/topics.yaml              # hand-built term → IDs index
eval/seed/                        # eligibility lists, seed string, selection output
eval/fixture-src/                 # CFR-cite fixture + key.yaml (frozen before extraction)
eval/render/                      # fixture-src → fixture mapper
eval/fixture/                     # ID form; lock pinned to a corpus commit
eval/judge-v1.md                  # committed judge prompt
eval/runs/<arm>-<model>-<n>.jsonl # stream-json transcripts
eval/negative/{C1,C5,C6}/         # (C2–C4 after accept)
eval/levels/{cover,strict}/       # (read, cite after accept)
eval/walk/  eval/guard/           # walk-test and agent-guard transcripts
eval/results/<YYYY-MM-DD>.yaml    # 30 graded runs + decision
VERSION  CHANGES.json             # after accept
```

**Upstream calls** (no API key; run on your machine, because these hosts are blocked from the cloud workspace):
- **eCFR version lists:** `versions/title-12.json?section=1024.41`, and the same for `?part=1024&appendix=Supplement I to Part 1024`.
- **eCFR text:** `full/{date}/title-12.xml?part=1024&section=<section>`. Send gzip and decode as UTF-8.
- **Federal Register:** `documents/{doc}.json` for the FR citation and effective date.
- **govinfo:** the CFR 2026 annual edition, for `verified_on`.

**Decision-critical path:** L0 → fixture freeze → arm (a) → extraction → verification → C5/C6 + C1 → skill → render → arms (b)/(c) → decision. Off-path work waits for "accept": the read and cite level fixtures, `upgrade`, release tagging, C2–C4 negatives and `init`.

## Order of work
### Weekend 1 (Nov 21–22): 1024.41 L0
1. Init the repo, the Go module and the Makefile, and run `make setup` (sets `core.hooksPath`). Write `AGENTS.md`, the five `stages/*/CONTRACT.md` files and `.claude/settings.json` (R11) first. Add the CI gate, `.github/workflows/c6.yml` (ADR 0007). The owner then sets branch protection on `main` to require the `C6 gate` check and up-to-date branches (a person's step); CI is the gate only from then.
2. `internal/ecfr`: fetch the version lists and point-in-time XML for 1024.41, 1024.30 and 1024.31, and store the raw XML untouched.
3. **Version-date question:**
   - Fetch 1024.41 at 2025-05-15, 2025-05-16 and 2025-07-15, and diff them.
   - Record what changed on each date.
   - Set `version_effective_date` and `version_fr_citations` from what the diff shows.
4. **`internal/parse` (R1a).** Write the golden tests before the code, built from the fetched XML only (CFPB pages still show removed 41(f)(3) comments). The goldens cover:
   - letter (i) after (h), and roman (i)
   - the duplicative-requests paragraph
   - the heading-dash chained label (b)(2)(i)(B)
   - (c)(2)(iv) and (f)(1)(i)
   - the italic level: (c)(2)(v)(A)(*1*) and (c)(4)(ii)(B)(*2*)
   - a cross-reference sentence that must not split
   - a corrupted-sequence fixture that must fail

   Then the parser itself:
   - **Label position:** a label counts only at the start of `<P>` or after a heading dash.
   - **State machine:** a stack of levels, each holding its last sibling label.
   - **The (i)/(v)/(x) rule:** it's a letter only when the last letter-level sibling directly precedes it.
   - **Fail loudly:** exit non-zero with the XML location; fix source defects only through `overrides.yaml`.
5. `internal/anchors`:
   - Generate `text.md` from the tree.
   - Normalize the text, and write a per-node `sha256` of normalized own text into `tree.json`.
   - Write `meta.yaml`.
6. `regcorpus fetch --verify`: compare the version date, tree and `sha256_text`, recording `sha256_raw` as information only. Then compare the normalized text to the govinfo CFR 2026 edition and set `verified_on`. Add `regcorpus fetch --latest` (R1b).
7. `regcorpus text <anchor>`.

### Weekend 2 (Nov 28–29): rest of L0, eval freeze, early arm (a), schema, start extraction
8. **Supplement I:** parse the whole appendix and slice out the 41() comments.
   - Headings must resolve to a regulation anchor, and may skip.
   - Comments number "1." then "i.".
   - Goldens: 41(b)(2)(i)-1, 41(c)(3)(i)-3, and a skipped heading.
9. 1024.30 and 1024.31 as cite-only L0. Definitions are anchored by term, with a `1024-31-day` golden.
10. **Freeze the eval, before any extraction:**
    - Commit the per-type eligibility lists and the seed string, then select the 10 seeds.
    - Write `eval/fixture-src/` and `key.yaml` (with `must_say` and `check_detectable` for each seed) and `eval/judge-v1.md`.
    - Commit, and record the fixture hash.
11. **Arm (a), early:** 5 runs × 2 models, each in its own temp directory. Use `--setting-sources project`, `--strict-mcp-config`, `--disallowedTools WebFetch,WebSearch` and `--output-format stream-json`, and record the exact command. Grade with the judge, but hold the author's blind sample until arms (b) and (c) exist.
    - **Checkpoint:** a model with mean recall(a) > 0.6 has failed decision 5. If both models fail, stop: keep L0 and record a kill.
12. `internal/registry`: types for the new schema, plus the enums, including `verification.level`, `method`, `bound` and `combine`.
13. C6, split into per-commit and `--complete`. The pre-commit hook runs per-commit only, so incomplete work can still be committed.
14. `regcorpus inventory --init`: list every operative anchor with an empty entry.
15. `regcorpus show` (and `--staging`).
16. Write `prompts/extract-v1.md`, `units.yaml` and the six golden records (hand-written, `prompt_version: manual`), plus `check --staging` with dedup. Start extraction over the regulation, one unit per call, with records kept in `staging/`.

### Weekend 3 (Dec 5–6): commentary, verification, topics, walk test
17. Extraction over the 41() commentary units. Cross-referenced text is already part of each unit's input, so no separate cross-reference pass is needed. Write the anchor rules by hand ((c)(2)(iv) and (b)(3)), with `prompt_version: manual`.
18. **Walk test (R10),** run while staged records exist. Save the transcript and the corpus commit to `eval/walk/`.
19. **Verification, through `regcorpus verify` only:**
    - First pick 10% of staged records by lowest `sha256(id)`. Re-derive their fields from the text **before** opening each proposal (`method: blind_rederive`), and record the disagreement rate.
    - Verify the rest with `proposal_review`.
    - Sign off `reviewed` for every inventory anchor.
20. `topics.yaml` and `regcorpus topics`. Topic completeness is checked by `--complete`.
21. **Agent-guard check (R11):** in a Claude Code session, try `--no-verify`, an edit under `registry/`, and `regcorpus verify`. Save the transcript to `eval/guard/`.

### Weekend 4 (Dec 12–13): consumer checks, arms (b) and (c), decision
22. C1–C5, the `cover` and `strict` levels, the C1 non-empty-reason rule, and the C5 counts. Add `eval/levels/{cover,strict}`.
23. The skill, and the 8-word skill check in C6.
24. Negatives for C1, C5 and C6, run with `make negative`.
25. `make build CORPUS_REF=<commit>` and `make build LOOKUP_ONLY=1`. Run `eval/render`, pin `eval/fixture/regcorpus.lock` to the commit, and confirm that `regcorpus check` on the fixture reports exactly the `check_detectable` seeds.
26. Arms (b) and (c): 5 runs × 2 models each, with the same isolation as step 11. The skill is a project skill in both arms' temp directories.
27. **Grade all 30 runs:**
    - the judge, on normalized and shuffled outputs
    - the author's blind adjudication of unmatched findings, and the 20% blind sample
    - metrics, the results file, decision 5, and accept / revise / kill

### Buffer and fallbacks
- **Buffer:** Dec 14–19, weekday evenings.
- **Fallback 1:** if verification isn't done by Dec 18, run arms (b) and (c) with unverified records rendered from `staging/`, and flag that in the results. The best outcome is then revise.
- **Fallback 2:** if time forces one model, run Opus only. The best outcome is then revise.
- **Decision** by Dec 20.

**Checkpoints:**
- **End of weekend 2:** if the steps on the decision-critical path aren't done, move steps 14–16 into weekend 3. Steps 10–11 are never cut.
- **End of weekend 3:** if verification won't finish by Dec 13, move step 21 to the buffer, then apply fallback 1.

**After accept (outside the appetite):**
- the `eval/levels/{read,cite}` fixtures and `upgrade`
- `regcorpus release` and the `v0.1.0` tag
- C2–C4 negatives, and re-pinning the fixture to the tag
- `regcorpus init`

## Risks
| Risk | Effect | Mitigation |
|---|---|---|
| eCFR XML doesn't tag paragraph depth; labels like (i) are ambiguous; cross-references contain labels | Wrong anchors poison every cite | Deterministic parser; labels recognized only by position; goldens written before the code, from the XML; fail on any sequence break |
| A source defect blocks the parser | Weekend 1 stalls | A reviewed `overrides.yaml` entry, never a parser hack |
| Supplement I's structure differs from the regulation's | Comment anchors wrong or missing | Inspect the raw XML first; goldens on known comments and a skipped heading |
| Extraction misses or duplicates duties (~90% first-pass recall is typical) | Omissions inside the corpus | Subtree units with cross-references included; inventory at operative anchors; `reviewed` sign-off; dedup in `check --staging` |
| Self-verification bias (one person proposes, verifies, seeds and grades) | "Verified" overstates accuracy; the eval is inflated | Blind re-derive sample with a measured disagreement rate; random seed selection from committed lists; fixture frozen before extraction; committed judge prompt; blind author sample; precision bar |
| The early arm (a) is too strong | Margin unreachable | Pre-registered as a fail for that model; stop if both fail (step 11) |
| Weekend 2 is Thanksgiving | Lost time | Steps 10–11 are fixed; 14–16 can slip to weekend 3 |
| Full verification overruns | Decision slips | Buffer evenings, then fallback 1 (which caps the outcome at revise), never an undefined outcome |
| An agent bypasses the git hook or edits `registry/` | Unverified content lands | R11 guard; `verify` needs a terminal; `verify.log` checked by C6, which CI also runs, so a bypassed hook doesn't skip it once `main`'s branch protection requires the `C6 gate` check (ADR 0007); C6 can't tell who wrote a record |
| eCFR, govinfo and the Federal Register API are blocked from the cloud workspace | Cloud agents can't fetch | Fetch steps run on your machine; later steps work offline from committed XML |

## Proof
| Acceptance (spec) | Command | Evidence |
|---|---|---|
| R1 | `regcorpus fetch --verify` | Exit 0; tree and normalized hash match; `verified_on` set; output saved |
| R1a | `make test` | All goldens pass; the corrupted fixture exits non-zero with the XML location |
| R1b | `regcorpus fetch --latest` (live, and on a stale fixture) | Exit 0 live; non-zero on the stale fixture |
| R2/R3 | `regcorpus check --corpus --complete` | Exit 0; 100% of operative anchors cited or `na`, and `reviewed`; every record `text-verified` with a `verify.log` line; blind re-derive rate reported |
| R2a | `regcorpus check --staging` on a bad quote and on a duplicate | Non-zero exit for each; every record names its prompt version |
| R4/R5 | `regcorpus check` in `eval/levels/strict` and in `eval/fixture`; `make negative` | Exit 0 on strict; on the fixture, non-zero, reporting exactly the `check_detectable` seeds; each negative exits non-zero with its rule ID |
| R7 | `eval/results/<date>.yaml` | 30 runs, each with model ID, arm, command, transcript, recall, precision and trace rate; judge-to-author agreement; decision 5 per model; the final decision |
| R8 | `regcorpus show RX-41-ACK-01`; `regcorpus topics acknowledgment`; `regcorpus topics xyz` | The record prints with verbatim text and its anchor rules; the ID is listed; the unknown term exits non-zero |
| R9 | `regcorpus check --corpus`; `wc -l skill/regcorpus/SKILL.md` | No 8-word anchor run in the skill; body under 100 lines |
| R10 | `eval/walk/<date>.md` | The fresh session names the verify contract and the correct staged record |
| R11 | `eval/guard/<date>.md` | `--no-verify`, a `registry/` edit and `regcorpus verify` are each blocked |
| R4 levels, R6 | `make levels`; `git tag -l v0.1.0` | After accept |
| Unit level | `make test` | Parsing, normalization, schema, C1 patterns, C6 value checks and the CHANGES diff all pass |
