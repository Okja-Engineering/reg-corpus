#!/usr/bin/env bash
# Tests for the per-commit corpus check (.githooks/pre-commit, a seatbelt; CI is the gate; C6, R5).
#
# Builds throwaway corpus repos from this checkout's go.mod, cmd/, internal/
# and stages/ (working-tree copies), installs the hook under test and its
# build-checker.sh with core.hooksPath, and commits through it; the known-main
# cases commit in clones of such a repo. Each case says whether the commit
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
trap 'chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT

export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
unset GOFLAGS

g() { git -C "$R" "$@"; }
# scaffold DIR: copy the checker, the stages and the hook under test into DIR.
scaffold() {
  mkdir -p "$1/.githooks" "$1/staging"
  (cd "$src" && tar -cf - go.mod cmd internal stages) | tar -xf - -C "$1" || exit 1
  cp -p "$here/pre-commit" "$here/build-checker.sh" "$1/.githooks/"
}

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
reset() { g checkout -q -f main && g reset -q --hard "$base" && g clean -qfdx; } # back to the logged-record commit on main

passed=0
failed=0
out=$tmp/out
opts=() # extra `git commit` options for the next commit_case

# commit_case WANT NAME [LINE]: commit through the hook. WANT 0: it lands, and
# the output holds LINE if one is given. WANT 1: it is blocked, HEAD stays,
# and the output holds LINE.
commit_case() {
  local want=$1 name=$2 line=${3-} before after rc ok=0
  before=$(g rev-parse -q --verify HEAD || echo none)
  g commit -q ${opts[@]+"${opts[@]}"} -m "$name" >"$out" 2>&1
  rc=$?
  after=$(g rev-parse -q --verify HEAD || echo none)
  if [ "$want" = 0 ]; then
    [ "$rc" = 0 ] && [ "$after" != "$before" ] && { [ -z "$line" ] || grep -qF -- "$line" "$out"; } && ok=1
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
boot="C6 checker: warning: bootstrap" # no known main has a checker yet

# --- No main yet (a fresh repo): the checker is built from the staged tree.
R=$tmp/fresh
scaffold "$R"
g init -q -b main && g config core.hooksPath .githooks || exit 1
rec RX-41-TEST-01 'id: RX-41-TEST-01
'
g add -A
commit_case 1 "no main yet: the staged tree's checker blocks an unlogged record" \
  "$C6/RX-41-TEST-01.yaml: no matching verify.log line (registry/verify.log is missing)"
g rm -q --cached registry/records/RX-41-TEST-01.yaml
commit_case 0 "no main yet: the first commit is checked by its own staged checker, with a warning" "$boot"

# --- main without a checker (main before PR #1 merges): same fallback.
R=$tmp/repo
mkdir -p "$R"
g init -q -b main || exit 1
printf 'scaffold\n' >"$R/README.md"
g add README.md && g commit -q -m "main before the checker, before the hook is installed" || exit 1
scaffold "$R"
g config core.hooksPath .githooks || exit 1
g checkout -q -b scaffold
rec RX-41-TEST-01 'id: RX-41-TEST-01
'
g add -A
commit_case 1 "main has no checker: the staged tree's checker blocks an unlogged record" \
  "$C6/RX-41-TEST-01.yaml: no matching verify.log line (registry/verify.log is missing)"
g rm -q --cached registry/records/RX-41-TEST-01.yaml
commit_case 0 "main has no checker: the scaffold lands, checked by its own staged checker, with a warning" "$boot"
g checkout -q main && g merge -q --ff-only scaffold || exit 1 # a person merges it

# --- main has the checker.
g add -A
commit_case 1 "unlogged record" \
  "$C6/RX-41-TEST-01.yaml: no matching verify.log line (registry/verify.log is missing)"

log RX-41-TEST-01 "$(sha "$R/registry/records/RX-41-TEST-01.yaml")"
g add -A
commit_case 0 "logged record"
base=$(g rev-parse HEAD)
if grep -qF "$boot" "$out"; then
  echo "FAIL  main has a checker, yet the hook warned of a bootstrap"
  failed=$((failed + 1))
fi

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

# A checker that passes everything: Violations() returns nothing.
sed 's/vs := s.Violations()$/vs := s.Violations()[:0]/' "$R/internal/check/check.go" >"$tmp/evil.go"
cmp -s "$tmp/evil.go" "$R/internal/check/check.go" && { echo "pre-commit_test: sabotage edit did not apply" >&2; exit 1; }

rec RX-41-TEST-06 'id: RX-41-TEST-06
'
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

# --- A branch can't choose its own checker: it is built from main.
g checkout -q -b weak
cp "$tmp/evil.go" "$R/internal/check/check.go"
g add -A
commit_case 0 "a branch may commit a weakened checker; main's checker checks that commit"
rec RX-41-TEST-08 'id: RX-41-TEST-08
'
g add -A
commit_case 1 "a record after a weakened checker on the branch is checked by main's checker" \
  "$C6/RX-41-TEST-08.yaml: no matching verify.log line"
opts=(--amend)
commit_case 1 "--amend onto the weakened checker is checked by main's checker" \
  "$C6/RX-41-TEST-08.yaml: no matching verify.log line"
opts=()
g checkout -q --orphan orphan
commit_case 1 "an orphan branch (no HEAD) is checked by main's checker" \
  "$C6/RX-41-TEST-08.yaml: no matching verify.log line"
reset

g checkout -q -b moved
g mv cmd/regcorpus/main.go cmd/regcorpus/entry.go
cp "$tmp/evil.go" "$R/internal/check/check.go"
g add -A
commit_case 0 "a branch may move cmd/regcorpus/main.go and weaken the checker"
rec RX-41-TEST-09 'id: RX-41-TEST-09
'
g add -A
commit_case 1 "with main.go moved on the branch, main's checker still checks the record" \
  "$C6/RX-41-TEST-09.yaml: no matching verify.log line"
reset

g checkout -q -b broken
printf 'this line does not compile\n' >>"$R/internal/check/check.go"
g add -A
commit_case 0 "a branch may commit a checker that doesn't build"
mkdir -p "$R/staging" && printf 'notes\n' >"$R/staging/notes.txt"
g add -A
commit_case 0 "a branch whose own checker doesn't build can still commit"
reset

# --- The checker's source is read as raw blobs: no replace refs, no filters.
evil=$(g hash-object -w "$tmp/evil.go")
g replace "$(g rev-parse main:internal/check/check.go)" "$evil"
rec RX-41-TEST-10 'id: RX-41-TEST-10
'
g add -A
commit_case 1 "a replace ref can't swap the checker's source" \
  "$C6/RX-41-TEST-10.yaml: no matching verify.log line"
g replace -d "$(g rev-parse main:internal/check/check.go)" >/dev/null
reset

printf 'internal/check/check.go filter=swap\n' >"$R/.git/info/attributes"
g config filter.swap.smudge "cat '$tmp/evil.go'"
g config filter.swap.clean cat
rec RX-41-TEST-11 'id: RX-41-TEST-11
'
g add registry
commit_case 1 "a smudge filter can't swap the checker's source" \
  "$C6/RX-41-TEST-11.yaml: no matching verify.log line"
rm "$R/.git/info/attributes"
g config --unset filter.swap.smudge
g config --unset filter.swap.clean
reset

# --- Go settings in the committer's environment don't reach the checker's build.
mkdir -p "$tmp/ws/m"
printf 'module example.com/m\n\ngo 1.21\n' >"$tmp/ws/m/go.mod"
printf 'go 1.21\n\nuse ./m\n' >"$tmp/ws/go.work"
mkdir -p "$R/staging" && printf 'notes\n' >"$R/staging/notes.txt"
g add -A
GOWORK=$tmp/ws/go.work commit_case 0 "a go.work named by GOWORK doesn't reach the checker's build"
reset
mkdir -p "$R/staging" && printf 'notes\n' >"$R/staging/notes.txt"
g add -A
GOTOOLCHAIN=go1.999.0 GOPROXY=off commit_case 0 "GOTOOLCHAIN can't switch the checker's toolchain"
reset

# --- Known mains: once refs/heads/main or refs/remotes/origin/main has a
# checker, no commit is checked by its own. Each clone below has origin/main
# with the checker (this repo's main) and stages a weakened checker beside an
# unlogged record.
pre=$(g rev-list --max-parents=0 main) # main before the checker
clone() {                              # clone NAME: a clone of this repo, on branch work
  git clone -q "$tmp/repo" "$tmp/$1" || exit 1
  R=$tmp/$1
  g config core.hooksPath .githooks && g checkout -q -b work || exit 1
}
weaken() { # stage a weakened checker and the unlogged record RX-41-TEST-12
  cp "$tmp/evil.go" "$R/internal/check/check.go"
  rec RX-41-TEST-12 'id: RX-41-TEST-12
'
  g add -A
}

clone stale
g branch -q -f main "$pre"
weaken
commit_case 1 "a clone whose local main is stale: origin/main's checker checks the commit" \
  "$C6/RX-41-TEST-12.yaml: no matching verify.log line"

clone nomain
g branch -q -D main
weaken
commit_case 1 "a clone with no local main: origin/main's checker checks the commit" \
  "$C6/RX-41-TEST-12.yaml: no matching verify.log line"

clone diverged
g checkout -q -b side origin/main^
cp "$tmp/evil.go" "$R/internal/check/check.go"
g commit -qam "weaken the checker beside main" >"$out" 2>&1 || { sed 's/^/      /' "$out"; exit 1; }
g branch -q -f main side
g checkout -q work
rec RX-41-TEST-12 'id: RX-41-TEST-12
'
g add -A
commit_case 1 "a local main that diverged from origin/main: origin/main's checker checks the commit" \
  "$C6/RX-41-TEST-12.yaml: no matching verify.log line"

clone ahead
g update-ref refs/remotes/origin/main "$pre"
weaken
commit_case 1 "origin/main without a checker, local main with one: local main's checker checks the commit" \
  "$C6/RX-41-TEST-12.yaml: no matching verify.log line"

clone unbuildable
g checkout -q -b break
printf 'this line does not compile\n' >>"$R/internal/check/check.go"
g commit -qam "a checker that doesn't build" >"$out" 2>&1 || { sed 's/^/      /' "$out"; exit 1; }
g update-ref refs/remotes/origin/main HEAD
g checkout -q work
mkdir -p "$R/staging" && printf 'notes\n' >"$R/staging/notes.txt"
g add -A
commit_case 1 "main's checker doesn't build: every commit is blocked, and the error says how to recover" \
  "To recover"
R=$tmp/repo

# --- go.sum travels with go.mod: once a dependency lands, commits still build.
# A local module proxy serves example.com/dep; nothing is fetched from the network.
export GOPROXY=file://$tmp/proxy GOSUMDB=off GOMODCACHE=$tmp/modcache
dep=$tmp/dep
mkdir -p "$dep" "$tmp/proxy/example.com/dep/@v"
printf 'module example.com/dep\n\ngo 1.21\n' >"$dep/go.mod"
printf 'package dep\n\n// X exists so the checker can import this module.\nconst X = 1\n' >"$dep/dep.go"
git -C "$dep" init -q && git -C "$dep" add -A && git -C "$dep" commit -q -m dep &&
  git -C "$dep" archive --format=zip --prefix=example.com/dep@v1.0.0/ \
    -o "$tmp/proxy/example.com/dep/@v/v1.0.0.zip" HEAD || exit 1
cp "$dep/go.mod" "$tmp/proxy/example.com/dep/@v/v1.0.0.mod"
printf 'v1.0.0\n' >"$tmp/proxy/example.com/dep/@v/list"
printf '{"Version":"v1.0.0","Time":"2026-11-21T00:00:00Z"}\n' >"$tmp/proxy/example.com/dep/@v/v1.0.0.info"
printf 'package check\n\nimport _ "example.com/dep"\n' >"$R/internal/check/dep.go"
(cd "$R" && GOFLAGS=-modcacherw go mod tidy) >"$out" 2>&1 || { sed 's/^/      /' "$out"; exit 1; }
[ -f "$R/go.sum" ] || { echo "pre-commit_test: go mod tidy wrote no go.sum" >&2; exit 1; }
g add -A
commit_case 0 "a dependency and its go.sum land on main"
mkdir -p "$R/staging" && printf 'notes\n' >"$R/staging/notes.txt"
g add -A
commit_case 0 "with a dependency on main, the checker builds (go.sum is extracted)"

echo "pre-commit_test: $passed passed, $failed failed"
[ "$failed" -eq 0 ]
