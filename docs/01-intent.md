# Intent: regulatory corpus

- **Author:** Matt
- **Status:** accepted 2026-11-21; revised the same day after adversarial review
- **Date:** 2026-11-20

## Problem
When you build software under regulation, every design decision that touches a rule depends on getting the rule exactly right, including its conditions, exceptions and day counts. Today the verbatim text may exist somewhere, but people and agents work from restatements in specs, decision logs, code comments and config. Those restatements bend or drop conditions: the five-day acknowledgment duty applies only to an application received "45 days or more before a foreclosure sale," and a research note for this project restated that as "more than 45 days." They miss duties entirely. And they go stale without warning. 1024.41 has been amended by four rules since 2017, three of them COVID-era additions and their rescission, with quiet stretches of almost four years, so a copy that carries no source version can sit wrong for years. Nothing mechanically checks a restatement against the source as it is written, and an omission leaves nothing to check.

Agents make this worse. Models weight what is already in context heavily, including a wrong paraphrase, and don't reliably know when to go to the source. Instructions to check are advisory: they work most of the time, not every time. So nothing ensures a claim was traced to the source unless something deterministic requires it.

## Proposed outcome
In a repo that opts in, every regulatory claim the check can detect cites a current, person-verified registry record that quotes the rule text, and at the cover and strict levels every in-scope obligation the spec never addresses shows up before build. Whether a cited claim says what its record says is still checked in review. That gives fewer regulatory errors in design, including omissions.

The corpus holds:
- the verbatim rule text
- an exhaustive registry of obligations with stable IDs
- a check that fails when a claim is uncited, a citation is stale against the pinned release, or an obligation is unaddressed, and a corpus check that fails when eCFR has a newer version than the one stored

**Success looks like (a proxy):** given a spec with seeded regulatory errors, including duties it never mentions, the corpus plus the check catches nearly all of them with few false alarms, and clearly more than the verbatim text alone. If it doesn't, I keep the text and registry for my own reference and stop there. **First real-use signal:** a spec written with the corpus passes review with no regulatory corrections.

Phase 1 covers 12 CFR 1024.41 and its official commentary, plus, as cite-only text, the 1024.30 scope rules and 1024.31 definitions it depends on ("Day means calendar day"). The appetite is fixed at four weekends plus the following week as buffer, with a decision by 2026-12-20. Scope gives way first. Verifying every record and running both models are kept; if either is cut, the best outcome is "revise".

## Affected users and systems
- **Users:** me and the agents I work with first; later, anyone building under regulation.
- **Consumers:** spec and code repos, at the level each opts into: read (lookups only), cite (citations must resolve), cover (every in-scope obligation addressed), or strict (uncited regulatory claims fail).
- **Upstream:** eCFR (point-in-time rule text) and the Federal Register (amendments, including future-effective ones).

## Constraints
- **Grounded truth only.** The corpus holds source text, citations, dates, and a decomposition of the text into records. Each decomposition choice (unit, modality, condition or exception, day type, anchor event, not applicable) is recorded with its source span and verified by a person. No compliance guidance, no product decisions, no code. Those live with the decisions that cite the corpus.
- **The registry is an anchored restatement, never a free paraphrase.** Every field that restates the rule cites its source span and carries the text verbatim. A machine check proves the quote exists and that derived values (day count, day type) appear in it or in a cited definition. A person checks whether the quote is the whole operative span and whether the field means what the text says.
- **One home per obligation.** Everything else points at it by ID.
- **Omissions must be visible.** Every paragraph in scope, lead-ins included, is mapped to obligations or marked not applicable with a reason, and every in-scope obligation a spec doesn't address shows up. This makes whole-duty omissions visible, not correct. Three cases are caught only by verification and review: a duty mapped wrong, "not applicable" given without good reason, and a spec that cites a duty but drops part of it.
- **"Verified" means the whole rule** (trigger, deadline, day type, conditions, exceptions), recorded with who, when and against which source version.
- **Dates kept apart.** Each entry keeps its effective, retrieved and verified dates separate.
- **Sources labeled by tier:** law, regulator interpretation, investor guide, standard.
- **Enforced, not advised.** The check, and the rule that no model edits or verifies registry records, run as hooks (CI later), not as instructions. A person can still bypass phase-1 local hooks; CI closes that.
- **No-gos:**
  - CLI only in phase 1: no server, API or MCP until a consumer without a shell needs one (every phase-1 consumer has a shell, and the gate needs a binary anyway)
  - no search-based retrieval
  - no UI
  - nothing beyond Reg X subpart C until phase 1 proves out

## Open questions
1. What is one obligation? Proposed: one duty-holder, one action, one deadline. Exhaustive coverage depends on this.
2. What evidence does each verification level require?
3. Is the check a hard CI gate or advisory in phase 1?
4. Do IDs carry a revision, so a stale citation fails automatically?
5. What thresholds define "nearly all," "few false alarms" and "clearly more"? Answered in the spec (decision 5): recall at least 0.8, precision at least 0.5, and recall at least 0.4 above text alone, on each model.
