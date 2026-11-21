# Stage 03: verify records into the registry (R2a gate, R3, C6)

## Purpose
A person checks each proposed record against the text and, if it holds, moves
it into `registry/` at `text-verified` (spec decision 2). This is the only path
into `registry/`. A person also signs off the inventory (R3).

## Inputs
- The full proposal set from stage 02: every ID in `staging/` or already moved
  to `registry/records/`. Verification starts only once extraction is complete.
- L0 anchor text from stage 01, shown beside each cited field.
- `registry/inventory/1024.41.yaml`, created with empty entries by
  `regcorpus inventory --init`, run by a person (plan step 14).

### Which record is next (plan step 19)
- **Pending:** a staged record whose `ID@rev` is not in `registry/records/` at
  that rev.
- **Blind sample:** the ceil(10%) IDs of the full proposal set with the lowest
  sha256 of the ID string (UTF-8, no trailing newline; e.g.
  `printf %s RX-41-ACK-01 | shasum -a 256`). These are verified first, in
  ascending hash order, with `method: blind_rederive`: the person re-derives
  every field from the text before opening the proposal.
- **The rest:** in ascending ID order, with `method: proposal_review`.
- **Next record:** the first pending record in that order.

## Outputs
- `registry/records/RX-*.yaml`, each with a complete `verification` block
  (`level`, `by`, `on`, `method`, `source_hashes`).
- One appended line per verification in `registry/verify.log`:
  `ID, rev, record sha256, by, on, method` (R2a). The log is append-only.
- `reviewed: {by, on}` on every inventory entry, a person's sign-off that every
  duty in that anchor has a record or an `na` with a reason (R3).
- The blind re-derive disagreement rate (R2/R3 acceptance). The blind records
  are those logged with `method: blind_rederive`; the person compares each
  re-derivation with the proposal during `verify`. How `verify` records a
  disagreement is built in plan step 19; `check --complete` reports the rate.

## Check that ends it
- Per record and per commit: `regcorpus check --corpus` exits 0 (C6). The
  pre-commit hook runs it on the commit's own tree (`--tree`); `make check` runs
  it on the working tree. Exit 0 means every registry rule built so far passes.
  Today that is the registry-state rule: every entry under `registry/` is a
  regular file or directory, and each file in `registry/records/` is named
  `RX-*.yaml` and has a `verify.log` line whose ID and sha256 match it. The
  other C6 rules arrive in plan step 13.
- For the stage as a whole: `regcorpus check --corpus --complete` exits 0 (R2/R3;
  plan step 13). Exit 0 means every operative anchor is cited or `na`, every
  inventory entry is `reviewed`, and every active record is in a topic.

## Who may run it
- A person only, via `regcorpus verify <ID>` in an interactive terminal (R2a;
  plan step 19). The command refuses to run without one.
- An agent must not run, wrap or emulate `regcorpus verify`, and must not write
  anything under `registry/` (R11). An agent asked to verify a record names the
  next record (above) and this contract, and hands the terminal step to a person.
- An agent must not print, open or summarize a blind-sample proposal until
  `verify.log` holds that ID's `blind_rederive` line: the re-derivation must not
  see the proposal.
- Otherwise an agent may run `regcorpus check --staging` (plan step 16),
  `regcorpus show --staging <ID>` (plan step 15) and `regcorpus check --corpus`
  to prepare and to report.

## Requirements
- R2: `registry/` holds only records at `text-verified` or above.
- R2a gate: `regcorpus verify` is the only path into `registry/`; it needs a
  terminal and appends to `verify.log`.
- R3: inventory coverage and the `reviewed` sign-off; mappings derived from cites.
- C6 registry state: a registry record below `text-verified`, with incomplete
  `verification`, or with no matching `verify.log` line fails.
