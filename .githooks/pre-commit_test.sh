#!/usr/bin/env bash
# Tests for the per-commit corpus gate (.githooks/pre-commit; C6, R5).
#
# Builds a throwaway corpus repo from this checkout's go.mod, cmd/, internal/
# and stages/ (working-tree copies), installs the hook under test with
# core.hooksPath, and commits through it. Each case says whether the commit
# must land (0) or be blocked (1, naming the C6 line), and checks HEAD moved
# or stayed. Prints one PASS/FAIL line per case; exits non-zero on any failure.
# Run: bash .githooks/pre-commit_test.sh   (also run by `make test`)
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd -P)
src=$(cd "$here/.." && pwd -P)
for tool in git go; do
  command -v "$tool" >/dev/null 2>&1 || { echo "pre-commit_test: $tool is required" >&2; exit 1; }
done
tmp=$(mktemp -d "${TMPDIR:-/tmp}/pre-commit-test.XXXXXX") || exit 1
tmp=$(cd "$tmp" && pwd -P)
trap 'rm -rf "$tmp"' EXIT

export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
unset GOFLAGS

R=$tmp/repo
mkdir -p "$R/.githooks" "$R/staging"
(cd "$src" && tar -cf - go.mod cmd internal stages) | tar -xf - -C "$R" || exit 1
cp -p "$here/pre-commit" "$R/.githooks/pre-commit"
g() { git -C "$R" "$@"; }
g init -q && g config core.hooksPath .githooks || exit 1

sha() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1
}
rec() { # write record ID with BODY
  mkdir -p "$R/registry/records"
  printf '%s' "$2" >"$R/registry/records/$1.yaml"
}
log() { # append a verify.log line for ID with HASH
  printf '%s, 1, %s, tester, 2026-11-21, proposal_review\n' "$1" "$2" >>"$R/registry/verify.log"
}
base=
reset() { g reset -q --hard "$base" && g clean -qfdx; } # back to the logged-record commit

passed=0
failed=0
out=$tmp/out

# commit_case WANT NAME [LINE]: commit through the hook. WANT 0: it lands.
# WANT 1: it is blocked, HEAD stays, and the output holds LINE.
commit_case() {
  local want=$1 name=$2 line=${3-} before after rc ok=0
  before=$(g rev-parse -q --verify HEAD || echo none)
  g commit -q -m "$name" >"$out" 2>&1
  rc=$?
  after=$(g rev-parse -q --verify HEAD || echo none)
  if [ "$want" = 0 ]; then
    [ "$rc" = 0 ] && [ "$after" != "$before" ] && ok=1
  else
    [ "$rc" != 0 ] && [ "$after" = "$before" ] && grep -qF -- "$line" "$out" && ok=1
  fi
  if [ "$ok" = 1 ]; then
    echo "PASS  commit exit $rc  $name"
    passed=$((passed + 1))
  else
    echo "FAIL  commit exit $rc (want $want)  $name"
    sed 's/^/      /' "$out"
    failed=$((failed + 1))
  fi
}

C6=C6\ registry-state:\ registry/records

g add -A
commit_case 0 "first commit: no HEAD, so the checker is built from the staged tree"

rec RX-41-TEST-01 'id: RX-41-TEST-01
'
g add -A
commit_case 1 "unlogged record" \
  "$C6/RX-41-TEST-01.yaml: no matching verify.log line (registry/verify.log is missing)"

log RX-41-TEST-01 "$(sha "$R/registry/records/RX-41-TEST-01.yaml")"
g add -A
commit_case 0 "logged record"
base=$(g rev-parse HEAD)

rec RX-41-TEST-01 'id: RX-41-TEST-01
changed: true
'
g add -A
commit_case 1 "sha256 mismatch after an edit" "$C6/RX-41-TEST-01.yaml: sha256 mismatch"
reset

rec RX-41-TEST-02 'id: RX-41-TEST-02
'
g add -A && g update-index --skip-worktree registry/records/RX-41-TEST-02.yaml
commit_case 1 "skip-worktree record" "$C6/RX-41-TEST-02.yaml: no matching verify.log line"
g update-index --no-skip-worktree registry/records/RX-41-TEST-02.yaml
reset

printf 'registry/records/*.yaml filter=fake\n' >"$R/.gitattributes"
g config filter.fake.clean cat
g config filter.fake.smudge 'sed s/unverified/verified/'
rec RX-41-TEST-03 'status: unverified
'
printf 'status: verified\n' >"$tmp/smudged"
log RX-41-TEST-03 "$(sha "$tmp/smudged")"
g add -A
commit_case 1 "smudge filter: the logged hash is of filtered bytes, not the committed blob" \
  "$C6/RX-41-TEST-03.yaml: sha256 mismatch"
reset
g config --unset filter.fake.clean
g config --unset filter.fake.smudge

rec RX-41-TEST-04 'id: RX-41-TEST-04
'
cp "$R/registry/verify.log" "$tmp/outside.log"
printf 'RX-41-TEST-04, 1, %s, tester, 2026-11-21, proposal_review\n' \
  "$(sha "$R/registry/records/RX-41-TEST-04.yaml")" >>"$tmp/outside.log"
rm "$R/registry/verify.log"
ln -s "$tmp/outside.log" "$R/registry/verify.log"
g add -A
commit_case 1 "symlinked verify.log" \
  "C6 registry-state: registry/verify.log: is a symlink; registry/ holds only regular files"
reset

rec RX-41-TEST-05 'id: RX-41-TEST-05
'
g add -N registry/records/RX-41-TEST-05.yaml
mkdir -p "$R/staging" && printf 'notes\n' >"$R/staging/notes.txt"
g add staging/notes.txt
commit_case 0 "intent-to-add record is not in the commit, so it does not block"
if g cat-file -e HEAD:registry/records/RX-41-TEST-05.yaml 2>/dev/null; then
  echo "FAIL  the intent-to-add record was committed"
  failed=$((failed + 1))
fi
g reset -q
reset

rec RX-41-TEST-06 'id: RX-41-TEST-06
'
sed 's/vs := s.Violations()$/vs := s.Violations()[:0]/' "$R/internal/check/check.go" >"$tmp/evil.go"
cmp -s "$tmp/evil.go" "$R/internal/check/check.go" && { echo "pre-commit_test: sabotage edit did not apply" >&2; exit 1; }
cp "$tmp/evil.go" "$R/internal/check/check.go"
g add -A
commit_case 1 "a staged change to the checker does not check its own commit" \
  "$C6/RX-41-TEST-06.yaml: no matching verify.log line"
reset

rec RX-41-TEST-07 'id: RX-41-TEST-07
'
g add -A
printf '{"Replace": {"internal/check/check.go": "%s"}}\n' "$tmp/evil.go" >"$tmp/overlay.json"
GOFLAGS=-overlay=$tmp/overlay.json commit_case 1 "GOFLAGS=-overlay can't swap the checker" \
  "$C6/RX-41-TEST-07.yaml: no matching verify.log line"
reset
rec RX-41-TEST-07 'id: RX-41-TEST-07
'
g add -A
printf 'GOFLAGS=-overlay=%s\n' "$tmp/overlay.json" >"$tmp/goenv"
GOENV=$tmp/goenv commit_case 1 "a go env file setting GOFLAGS can't swap the checker" \
  "$C6/RX-41-TEST-07.yaml: no matching verify.log line"
reset

echo "pre-commit_test: $passed passed, $failed failed"
[ "$failed" -eq 0 ]
