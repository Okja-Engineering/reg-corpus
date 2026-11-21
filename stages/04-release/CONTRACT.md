# Stage 04: release (R6, R1b, C6 --complete)

## Purpose
Cut an immutable release that consumers pin (R6). Releases happen only after
the phase-1 decision is accept (plan, "After accept").

## Inputs
- The accept decision, recorded in `eval/results/<YYYY-MM-DD>.yaml` (stage 05).
  No accept there, no release.
- The registry, inventory, topics and L0 at the commit being released.
- The previous release tag, if any, to diff IDs against.

## Outputs
- An immutable git tag (first: `v0.1.0`) with `VERSION` and `CHANGES.json` at
  the repo root (R6). `CHANGES.json` lists IDs under `added`, `rev_bumped`,
  `superseded`, `withdrawn` and `verification_changed`.
- A binary embedding the snapshot: `make build CORPUS_REF=<tag or commit>`
  (plan step 25). `regcorpus version` prints the snapshot's ref and sha.

## Check that ends it
Run in this order, on the owner's machine (step 1 needs eCFR, which the cloud
workspace can't reach); each must exit 0:
1. `regcorpus fetch --latest` (R1b; plan step 6). Exit 0 means no stored
   section has a newer eCFR version and the Supplement I 41() slice is unchanged.
2. `regcorpus check --corpus --complete` (R5, C6; plan step 13). Exit 0 means
   the per-commit C6 rules pass, including ID continuity (every ID in the
   previous release still exists, active or superseded or withdrawn), every
   operative anchor is cited or `na`, every inventory entry is `reviewed`, and
   every active record is in a topic.
3. `regcorpus release` (R6; plan, "After accept"). Exit 0 means no active record
   is below `text-verified`.

## Who may run it
- A person creates the tag; tags are never moved or deleted.
- An agent may run the three checks above on the owner's machine and report
  their output.

## Requirements
- R6: immutable tags, `VERSION`, `CHANGES.json`, pinning, ID continuity (held
  by C6 on every commit), and the `text-verified` floor (`release`).
- R1b: the newer-version check runs before every release.
- R5 and C6 `--complete`: required for the decision gate and for releases.
- Spec "Releases and distribution": version-number rules (major, minor, patch;
  exact pins before 1.0.0).
