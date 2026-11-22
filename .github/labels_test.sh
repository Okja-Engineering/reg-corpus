#!/usr/bin/env bash
# Tests for labels as code (.github/labels.sh and .github/labels.json).
#
# Runs labels.sh against a stub gh on PATH that logs each call. The
# invariants: every label in the list is created or updated with --force (so
# a rerun changes nothing), with its own color and description as single
# arguments; the list holds the labels the issue forms and the epics use; and
# a list that is malformed is refused before gh is called at all, so a sync
# never stops half-done on a bad file. A gh failure fails the run.
# Prints one PASS/FAIL line per case; exits non-zero on any failure.
# Run: bash .github/labels_test.sh   (also run by `make test`)
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd -P)
command -v jq >/dev/null 2>&1 || { echo "labels_test: jq is required" >&2; exit 1; }
tmp=$(mktemp -d "${TMPDIR:-/tmp}/labels-test.XXXXXX") || exit 1
trap 'rm -rf "$tmp"' EXIT

# The stub gh: one line per call, each argument in brackets. It fails the
# call for the label named in $STUB_FAIL.
mkdir -p "$tmp/bin"
cat >"$tmp/bin/gh" <<'STUB'
#!/usr/bin/env bash
for a in "$@"; do printf '[%s]' "$a"; done >>"$STUB_LOG"
printf '\n' >>"$STUB_LOG"
[ -n "${STUB_FAIL:-}" ] && [ "${3:-}" = "$STUB_FAIL" ] && exit 1
exit 0
STUB
chmod +x "$tmp/bin/gh"
export STUB_LOG="$tmp/gh.log"

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $2"; fails=$((fails + 1)); }
# run FILE: run labels.sh on FILE with the stub gh; sets $code and $out.
run() {
  : >"$STUB_LOG"
  out=$(PATH="$tmp/bin:$PATH" bash "$here/labels.sh" "$@" 2>&1)
  code=$?
}
calls() { wc -l <"$STUB_LOG" | tr -d ' '; }

# 1. The committed list: one create --force per label, arguments intact.
run
want=$(jq -r '.[] | "[label][create][\(.name)][--color][\(.color)][--description][\(.description)][--force]"' "$here/labels.json")
if [ "$code" -ne 0 ]; then fail "committed list syncs" "exit $code: $out"
elif [ "$(cat "$STUB_LOG")" != "$want" ]; then fail "committed list syncs" "calls differ:
$(diff <(printf '%s\n' "$want") "$STUB_LOG")"
else pass "committed list syncs, one create --force per label"; fi

# 2. A rerun makes the same calls (idempotent by --force, nothing deleted).
first=$(cat "$STUB_LOG"); run
if [ "$code" -eq 0 ] && [ "$(cat "$STUB_LOG")" = "$first" ] && ! grep -q '\[delete\]' "$STUB_LOG"
then pass "rerun makes the same calls and deletes nothing"
else fail "rerun" "exit $code or calls changed"; fi

# 3. The list holds the labels the mandate names.
missing=
for l in bug enhancement documentation decision epic story stage:01-fetch-parse \
  stage:02-extract stage:03-verify stage:04-release stage:05-evaluate; do
  jq -e --arg l "$l" 'any(.[]; .name == $l)' "$here/labels.json" >/dev/null || missing="$missing $l"
done
[ -z "$missing" ] && pass "list holds the required labels" || fail "required labels" "missing:$missing"

# 4. Every label an issue form applies is in the list.
missing=
for f in "$here"/ISSUE_TEMPLATE/{bug,feature,decision}.yml; do
  [ -f "$f" ] || { missing="$missing $(basename "$f"):(no such form)"; continue; }
  for l in $(sed -n 's/^labels: *\[\(.*\)\]/\1/p' "$f" | tr ',' ' '); do
    jq -e --arg l "$l" 'any(.[]; .name == $l)' "$here/labels.json" >/dev/null || missing="$missing $(basename "$f"):$l"
  done
done
[ -z "$missing" ] && pass "issue forms apply only listed labels" || fail "issue form labels" "not listed:$missing"

# 5. Bad lists are refused before any gh call.
bad() { # NAME JSON
  printf '%s\n' "$2" >"$tmp/bad.json"; run "$tmp/bad.json"
  if [ "$code" -eq 1 ] && [ "$(calls)" -eq 0 ] && printf '%s' "$out" | grep -q "^labels: $tmp/bad.json: "
  then pass "refuses $1, no gh call"
  else fail "refuses $1" "exit $code, $(calls) gh calls: $out"; fi
}
ok='{"name":"a","color":"d73a4a","description":"d"}'
bad "malformed JSON" '[{"name":'
bad "an empty list" '[]'
bad "a color with #" "[$ok,{\"name\":\"b\",\"color\":\"#d73a4a\",\"description\":\"d\"}]"
bad "a short color" '[{"name":"a","color":"fff","description":"d"}]'
bad "a missing description" '[{"name":"a","color":"d73a4a"}]'
bad "an empty name" '[{"name":"","color":"d73a4a","description":"d"}]'
bad "a duplicate name, any case" "[$ok,{\"name\":\"A\",\"color\":\"000000\",\"description\":\"e\"}]"
bad "an unknown key" '[{"name":"a","color":"d73a4a","description":"d","x":1}]'
bad "a description over 100 characters" "[{\"name\":\"a\",\"color\":\"d73a4a\",\"description\":\"$(printf 'x%.0s' $(seq 101))\"}]"

# 6. A gh failure fails the run.
STUB_FAIL=epic run
if [ "$code" -eq 1 ] && grep -q '^\[label\]\[create\]\[epic\]' "$STUB_LOG" && printf '%s' "$out" | grep -q '^labels: gh failed on epic'
then pass "a gh failure fails the run"; else fail "gh failure" "exit $code: $out"; fi

# 7. Without jq the script says so and calls nothing.
mkdir -p "$tmp/nojq"; ln -sf "$(command -v bash)" "$tmp/nojq/bash"
for t in dirname pwd; do p=$(command -v "$t") && [ "${p#/}" != "$p" ] && ln -sf "$p" "$tmp/nojq/$t"; done
: >"$STUB_LOG"
out=$(PATH="$tmp/bin:$tmp/nojq" "$tmp/nojq/bash" "$here/labels.sh" 2>&1); code=$?
if [ "$code" -ne 0 ] && [ "$(calls)" -eq 0 ] && printf '%s' "$out" | grep -q 'jq is required'
then pass "without jq: refused, no gh call"; else fail "without jq" "exit $code: $out"; fi

[ "$fails" -eq 0 ] || { echo "labels_test: $fails failed" >&2; exit 1; }
