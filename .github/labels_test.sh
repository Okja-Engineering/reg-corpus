#!/usr/bin/env bash
# Tests for labels as code (.github/labels.sh and .github/labels.json).
#
# Runs labels.sh against a stub gh on PATH that reads its arguments the way
# gh does and logs the label each call creates. The invariants: every label
# in the list reaches gh as a name, with its own color and description, and is
# created or updated with --force (so a rerun changes nothing); the list holds
# the labels the issue forms and the epics use; and a list that breaks any
# rule in labels.sh's header is refused before gh is called at all, so a sync
# never stops half-done on a bad file. A gh failure fails the run.
# Prints one PASS/FAIL line per case; exits non-zero on any failure.
# Run: bash .github/labels_test.sh   (also run by `make test`)
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd -P)
command -v jq >/dev/null 2>&1 || { echo "labels_test: jq is required" >&2; exit 1; }
tmp=$(mktemp -d "${TMPDIR:-/tmp}/labels-test.XXXXXX") || exit 1
trap 'rm -rf "$tmp"' EXIT

# The stub gh logs each call, one line, each argument in brackets. It reads
# `label create` as gh does: before `--`, an argument starting with `-` is an
# option (--color and --description take the next argument; --help prints
# help and succeeds, creating nothing), and the one other argument is the
# name. Each label it creates is logged as [name][color][description][force].
# It fails the call for the label named in $STUB_FAIL.
mkdir -p "$tmp/bin"
cat >"$tmp/bin/gh" <<'STUB'
#!/usr/bin/env bash
for a in "$@"; do printf '[%s]' "$a"; done >>"$STUB_LOG"
printf '\n' >>"$STUB_LOG"
[ "${1:-} ${2:-}" = "label create" ] || { echo "stub gh: not label create" >&2; exit 1; }
shift 2
names=() color= desc= force=
while [ $# -gt 0 ]; do
  case $1 in
    --) shift; names+=("$@"); break ;;
    -h | --help) echo "Create a new label on GitHub"; exit 0 ;;
    -c | --color) [ $# -ge 2 ] || exit 1; color=$2; shift 2 ;;
    -d | --description) [ $# -ge 2 ] || exit 1; desc=$2; shift 2 ;;
    -f | --force) force=--force; shift ;;
    -*) echo "unknown flag: $1" >&2; exit 1 ;;
    *) names+=("$1"); shift ;;
  esac
done
[ "${#names[@]}" -eq 1 ] || { echo "cannot create label: name argument required" >&2; exit 1; }
[ -n "${STUB_FAIL:-}" ] && [ "${names[0]}" = "$STUB_FAIL" ] && exit 1
printf '[%s][%s][%s][%s]\n' "${names[0]}" "$color" "$desc" "$force" >>"$STUB_CREATED"
exit 0
STUB
chmod +x "$tmp/bin/gh"
export STUB_LOG="$tmp/gh.log" STUB_CREATED="$tmp/created.log"

fails=0
pass() { echo "PASS $1"; }
fail() { echo "FAIL $1: $2"; fails=$((fails + 1)); }
# run [FILE]: run labels.sh on FILE with the stub gh; sets $code and $out.
run() {
  : >"$STUB_LOG"; : >"$STUB_CREATED"
  out=$(PATH="$tmp/bin:$PATH" bash "$here/labels.sh" "$@" 2>&1)
  code=$?
}
calls() { wc -l <"$STUB_LOG" | tr -d ' '; }
# want FILE: the created log a full sync of FILE must leave.
want() { jq -r '.[] | "[\(.name)][\(.color)][\(.description)][--force]"' "$1"; }
# synced NAME FILE: FILE (run already) exited 0 and created exactly its labels.
synced() {
  if [ "$code" -ne 0 ]; then fail "$1" "exit $code: $out"
  elif [ "$(cat "$STUB_CREATED")" != "$(want "$2")" ]; then fail "$1" "labels created differ:
$(diff <(want "$2") "$STUB_CREATED")"
  else pass "$1"; fi
}

# 1. The committed list: every label created with --force, arguments intact.
run
synced "committed list syncs, every label created with --force" "$here/labels.json"

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

# form_labels FILE: print each label an issue form applies, one per line. It
# reads the top-level `labels` key in each shape GitHub accepts (a flow list,
# a block list, or a comma-separated string; items plain or quoted) and exits
# 2 on any other shape, so a form it can't read fails the check, never passes.
form_labels() {
  awk -v q="'" '
    function unquote(s) {
      gsub(/^[ \t]+|[ \t]+$/, "", s)
      if (length(s) >= 2 && (substr(s, 1, 1) == "\"" || substr(s, 1, 1) == q) && substr(s, length(s)) == substr(s, 1, 1))
        s = substr(s, 2, length(s) - 2)
      return s
    }
    function emit(s) { s = unquote(s); if (s == "") exit 2; print s; n++ }
    function emit_all(s,   a, k, i) { k = split(s, a, ","); for (i = 1; i <= k; i++) emit(a[i]) }
    function value(s) { sub(/[ \t]+#.*$/, "", s); gsub(/^[ \t]+|[ \t]+$/, "", s); return s }
    block && /^[ \t]*(#.*)?$/ { next }
    block && /^[ \t]*-([ \t]|$)/ { s = $0; sub(/^[ \t]*-/, "", s); emit(value(s)); next }
    { block = 0 }
    $0 ~ "^(\"labels\"|" q "labels" q "|labels)[ \t]*:" {
      if (seen++) exit 2
      s = $0; sub(/^[^:]*:/, "", s); s = value(s)
      if (s == "") { block = 1; n = 0; want_items = 1 }
      else if (s ~ /^\[.*\]$/) emit_all(substr(s, 2, length(s) - 2))
      else if (s ~ /^[[{&*|>!]/) exit 2
      else emit_all(unquote(s))
    }
    END { if (want_items && n == 0) exit 2 }
  ' "$1"
}

# 4. The form reader reads each shape and refuses what it can't read.
reader_case() { # NAME WANT (labels one per line, or "unreadable") FORM
  printf '%s\n' "$3" >"$tmp/form.yml"
  got=$(form_labels "$tmp/form.yml") || got=unreadable
  [ "$got" = "$2" ] && pass "form reader: $1" || fail "form reader: $1" "got [$got], want [$2]"
}
nl='
'
reader_case "flow list" "bug${nl}good first issue" 'name: x
labels: [bug, "good first issue"]'
reader_case "block list, quoted, with a comment" "bug${nl}help wanted" "name: x
labels:
  - bug
  # a comment
  - 'help wanted'
body: []"
reader_case "comma-separated string" "bug${nl}documentation" 'labels: "bug, documentation" # two'
reader_case "plain scalar" "bug" 'labels: bug'
reader_case "no labels key" "" 'name: x
body: []'
reader_case "a flow list over two lines" unreadable 'labels: [bug,
  documentation]'
reader_case "an empty block list" unreadable 'labels:
body: []'
reader_case "an alias" unreadable 'labels: *common'

# 5. Every label each issue form applies is in the list; every form is read.
missing= forms=0
for f in "$here"/ISSUE_TEMPLATE/*.yml "$here"/ISSUE_TEMPLATE/*.yaml; do
  case $(basename "$f") in config.yml | config.yaml) continue ;; esac
  [ -f "$f" ] || continue
  forms=$((forms + 1))
  ls=$(form_labels "$f") || { missing="$missing $(basename "$f"):(labels unreadable)"; continue; }
  while IFS= read -r l; do
    [ -n "$l" ] || continue
    jq -e --arg l "$l" 'any(.[]; .name == $l)' "$here/labels.json" >/dev/null || missing="$missing $(basename "$f"):$l"
  done <<<"$ls"
done
if [ "$forms" -eq 0 ]; then fail "issue form labels" "no issue forms found"
elif [ -z "$missing" ]; then pass "issue forms ($forms) apply only listed labels"
else fail "issue form labels" "not listed:$missing"; fi

# 6. Bad lists are refused before any gh call.
bad() { # NAME JSON
  printf '%s\n' "$2" >"$tmp/bad.json"; run "$tmp/bad.json"
  if [ "$code" -eq 1 ] && [ "$(calls)" -eq 0 ] && printf '%s' "$out" | grep -q "^labels: $tmp/bad.json: "
  then pass "refuses $1, no gh call"
  else fail "refuses $1" "exit $code, $(calls) gh calls: $out"; fi
}
x() { printf "$1%.0s" $(seq "$2"); } # x CHAR N: CHAR N times
ok='{"name":"a","color":"d73a4a","description":"d"}'
bad "malformed JSON" '[{"name":'
bad "an empty file" ''
bad "an empty list" '[]'
bad "two top-level lists" "[$ok]
[{\"name\":\"b\",\"color\":\"d73a4a\",\"description\":\"d\"}]"
bad "a color with #" "[$ok,{\"name\":\"b\",\"color\":\"#d73a4a\",\"description\":\"d\"}]"
bad "a short color" '[{"name":"a","color":"fff","description":"d"}]'
bad "a missing description" '[{"name":"a","color":"d73a4a"}]'
bad "an empty description" '[{"name":"a","color":"d73a4a","description":""}]'
bad "an empty name" '[{"name":"","color":"d73a4a","description":"d"}]'
bad "a name over 50 characters" "[{\"name\":\"$(x n 51)\",\"color\":\"d73a4a\",\"description\":\"d\"}]"
bad "a name with a leading space" '[{"name":" a","color":"d73a4a","description":"d"}]'
bad "a name with a trailing space" '[{"name":"a ","color":"d73a4a","description":"d"}]'
bad "a duplicate name, any case" "[$ok,{\"name\":\"A\",\"color\":\"000000\",\"description\":\"e\"}]"
bad "an unknown key" '[{"name":"a","color":"d73a4a","description":"d","x":1}]'
bad "a description over 100 characters" "[{\"name\":\"a\",\"color\":\"d73a4a\",\"description\":\"$(x x 101)\"}]"

# 7. Names at the limits sync: one character, 50 characters, inner spaces, a
# description of 100 characters, and names starting with `-`, which reach gh
# as names, not options.
cat >"$tmp/edge.json" <<EDGE
[{"name":"a","color":"d73a4a","description":"$(x d 100)"},
 {"name":"$(x n 50)","color":"d73a4a","description":"d"},
 {"name":"good first issue","color":"7057ff","description":"d"},
 {"name":"--help","color":"d73a4a","description":"d"},
 {"name":"-f","color":"d73a4a","description":"d"}]
EDGE
run "$tmp/edge.json"
synced "names at the limits and names starting with - sync as names" "$tmp/edge.json"

# 8. A gh failure fails the run.
STUB_FAIL=epic run
if [ "$code" -eq 1 ] && grep -q '^\[label\]\[create\].*\[epic\]' "$STUB_LOG" && printf '%s' "$out" | grep -q '^labels: gh failed on epic'
then pass "a gh failure fails the run"; else fail "gh failure" "exit $code: $out"; fi

# 9. Without jq the script says so and calls nothing.
mkdir -p "$tmp/nojq"; ln -sf "$(command -v bash)" "$tmp/nojq/bash"
for t in dirname pwd; do p=$(command -v "$t") && [ "${p#/}" != "$p" ] && ln -sf "$p" "$tmp/nojq/$t"; done
: >"$STUB_LOG"
out=$(PATH="$tmp/bin:$tmp/nojq" "$tmp/nojq/bash" "$here/labels.sh" 2>&1); code=$?
if [ "$code" -ne 0 ] && [ "$(calls)" -eq 0 ] && printf '%s' "$out" | grep -q 'jq is required'
then pass "without jq: refused, no gh call"; else fail "without jq" "exit $code: $out"; fi

[ "$fails" -eq 0 ] || { echo "labels_test: $fails failed" >&2; exit 1; }
