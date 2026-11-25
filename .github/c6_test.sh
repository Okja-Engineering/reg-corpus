#!/usr/bin/env bash
# Tests for CI's corpus gate (.github/c6.sh and the .githooks/build-checker.sh
# it calls; C6, R5).
#
# Builds a throwaway corpus repo (ORIGIN) from this checkout's go.mod, cmd/,
# internal/ and stages/ (working-tree copies): main is a commit before the
# checker, then the checker. Each case runs c6.sh, as the workflow does, in a
# clone of it (so refs/remotes/origin/main is origin's main) holding the
# c6.sh and build-checker.sh under test, on a BASE and a COMMIT made there.
# The invariant: once origin/main has a checker, no run uses another, whatever
# the base (the base's is used only while origin/main has none); a base that
# is missing or doesn't resolve fails the run. Each case says whether the
# run must pass (0) or fail (1), and a line its output must hold. Prints one
# PASS/FAIL line per case; exits non-zero on any failure.
# Run: bash .github/c6_test.sh   (also run by `make test`)
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd -P)
src=$(cd "$here/.." && pwd -P)
for tool in git go; do
  command -v "$tool" >/dev/null 2>&1 || { echo "c6_test: $tool is required" >&2; exit 1; }
done
tmp=$(mktemp -d "${TMPDIR:-/tmp}/c6-test.XXXXXX") || exit 1
tmp=$(cd "$tmp" && pwd -P)
trap 'chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT

export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com
unset GOFLAGS

die() { echo "c6_test: $*" >&2; exit 1; }
g() { git -C "$R" "$@"; }
commit() { g add -A && g commit -q -m "$1" && g rev-parse HEAD; } # commit all, print the oid

# scaffold DIR: copy the checker and the stages into DIR.
scaffold() {
  (cd "$src" && tar -cf - go.mod $(test -f go.sum && echo go.sum) cmd internal stages) | tar -xf - -C "$1" || die "scaffold"
}
rec() { # write the unlogged record RX-41-TEST-01 in R
  mkdir -p "$R/registry/records"
  printf 'id: RX-41-TEST-01\n' >"$R/registry/records/RX-41-TEST-01.yaml"
}
# A checker that passes everything: Violations() returns nothing.
sed 's/vs := s.Violations()$/vs := s.Violations()[:0]/' "$src/internal/check/check.go" >"$tmp/evil.go"
cmp -s "$tmp/evil.go" "$src/internal/check/check.go" && die "sabotage edit did not apply"

# ORIGIN: main is PRE (no checker), then MAIN (the checker).
R=$tmp/origin
mkdir -p "$R" && g init -q -b main || die "init"
printf 'scaffold\n' >"$R/README.md"
pre=$(commit "main before the checker") || die "commit"
scaffold "$R"
main=$(commit "the checker") || die "commit"

# The clone the cases run in, with the scripts under test in its work tree.
R=$tmp/clone
git clone -q "$tmp/origin" "$R" || die "clone"
mkdir -p "$R/.github" "$R/.githooks"
cp "$here/c6.sh" "$R/.github/c6.sh"
cp "$src/.githooks/build-checker.sh" "$R/.githooks/build-checker.sh"
printf '.github/\n.githooks/\n' >>"$R/.git/info/exclude"

# Commits under test.
g checkout -q -b good "$main"
mkdir -p "$R/staging" && printf 'notes\n' >"$R/staging/notes.txt"
good=$(commit "a good change") || die "commit"
g checkout -q -b weak "$main"
cp "$tmp/evil.go" "$R/internal/check/check.go" && rec
weak=$(commit "a weakened checker and an unlogged record") || die "commit"
g checkout -q -b boot "$pre"
scaffold "$R"
boot=$(commit "the checker, before main has one") || die "commit"
rec
bootrec=$(commit "an unlogged record, before main has a checker") || die "commit"
g checkout -q -b oldweak "$pre"
scaffold "$R" && cp "$tmp/evil.go" "$R/internal/check/check.go" && rec
oldweak=$(commit "a weakened checker on a base without one") || die "commit"
g checkout -q -b weakbase "$main" # a non-main base whose own checker is weakened
cp "$tmp/evil.go" "$R/internal/check/check.go"
weakbase=$(commit "a branch that weakens its checker") || die "commit"
rec
onweak=$(commit "an unlogged record, into the weakened branch") || die "commit"
g checkout -q -b renamed "$main" # main keeps its checker under another file name
g mv cmd/regcorpus/main.go cmd/regcorpus/entry.go
renamed=$(commit "main renames the checker's main.go") || die "commit"
g checkout -q -b gomain "$main"
gover=$(sed -n 's/^go //p' "$R/go.mod")
sed "s/^go $gover\$/go 1.99.0/" "$R/go.mod" >"$tmp/go.mod" && cp "$tmp/go.mod" "$R/go.mod"
grep -q '^go 1.99.0$' "$R/go.mod" || die "raising main's Go version did not apply"
gomain=$(commit "main raises its Go version past the installed one") || die "commit"
g checkout -q --detach "$good"

passed=0
failed=0
out=$tmp/out
C6="C6 registry-state: registry/records/RX-41-TEST-01.yaml: no matching verify.log line"
boot_warn="::warning title=C6 bootstrap::"

# run WANT NAME BASE COMMIT LINE: run c6.sh BASE COMMIT in the clone. WANT 0:
# it passes; 1: it fails. Either way the output holds LINE, and it warns of a
# bootstrap exactly when BOOT is 1.
BOOT=0
run() {
  local want=$1 name=$2 rc ok=0 warned=0
  (cd "$R" && bash .github/c6.sh "$3" "$4") >"$out" 2>&1
  rc=$?
  grep -qF -- "$boot_warn" "$out" && warned=1
  if { { [ "$want" = 0 ] && [ "$rc" = 0 ]; } || { [ "$want" = 1 ] && [ "$rc" != 0 ]; }; } &&
    grep -qF -- "$5" "$out" && [ "$warned" = "$BOOT" ]; then
    ok=1
  fi
  if [ "$ok" = 1 ]; then
    echo "PASS  c6.sh exit $rc  $name"
    passed=$((passed + 1))
  else
    echo "FAIL  c6.sh exit $rc (want $want)  $name"
    sed 's/^/      /' "$out"
    failed=$((failed + 1))
  fi
}
origin_main() { g update-ref refs/remotes/origin/main "$1"; }

# --- Bootstrap: neither the base nor origin/main has a checker.
origin_main "$pre"
BOOT=1
run 0 "bootstrap: the commit is checked by its own checker, with a warning" "$pre" "$boot" "$boot_warn"
run 1 "bootstrap: the commit's own checker still rejects an unlogged record" "$pre" "$bootrec" "$C6"
run 0 "bootstrap on push: origin/main is the base" refs/remotes/origin/main "$boot" "$boot_warn"
BOOT=0

# --- origin/main has no checker, the base has one: the base's checker.
run 1 "origin/main without a checker: the base's checker rejects a weakened one" "$main" "$weak" "$C6"
run 0 "origin/main without a checker: the base's checker passes a good change" "$main" "$good" \
  "C6 checker built from the base"

# --- origin/main has the checker.
origin_main "$main"
run 0 "a base with a checker passes a good change" "$main" "$good" "C6 checker built from origin/main"
run 1 "a base with a checker rejects a weakened checker's commit" "$main" "$weak" "$C6"
run 1 "on push: origin/main's checker rejects a weakened checker's commit" refs/remotes/origin/main "$weak" "$C6"
run 1 "a non-main base with its own weakened checker: origin/main's checker rejects" \
  "$weakbase" "$onweak" "$C6"
run 1 "a base without a checker: origin/main's checker rejects a weakened one" "$pre" "$oldweak" "$C6"
run 0 "a base without a checker: origin/main's checker passes a good change" "$pre" "$good" \
  "C6 checker built from origin/main"
origin_main "$renamed"
run 1 "origin/main renames cmd/regcorpus/main.go: its checker rejects a weakened one" \
  "$pre" "$oldweak" "$C6"
run 1 "on push, main renamed cmd/regcorpus/main.go: its checker rejects a weakened one" \
  refs/remotes/origin/main "$weak" "$C6"
origin_main "$main"
run 1 "an unresolvable base fails the run" 0123456789abcdef0123456789abcdef01234567 "$good" "cannot resolve"
run 1 "a missing base fails the run" "" "$good" "cannot resolve"
run 1 "a base that names no commit fails the run" refs/heads/no-such-branch "$good" "cannot resolve"
g update-ref -d refs/remotes/origin/main
run 1 "no origin/main fails the run" "$main" "$good" "cannot resolve"

# --- main's go.mod names a Go newer than the installed one (setup-go reads
# the commit's go.mod). The trusted build uses the Go main's go.mod names:
# go finds go1.99.0 on PATH before it would download it. This stand-in notes
# that it ran, then builds with the installed Go.
mkdir -p "$tmp/fakego"
realgo=$(command -v go)
cat >"$tmp/fakego/go1.99.0" <<EOF
#!/bin/sh
echo ran >>'$tmp/fakego/ran'
sed 's/^go 1.99.0\$/go $gover/' go.mod >go.mod.installed && mv go.mod.installed go.mod
exec env -u GOTOOLCHAIN_INTERNAL_SWITCH_VERSION -u GOTOOLCHAIN_INTERNAL_SWITCH_COUNT -u GOROOT \\
  GOTOOLCHAIN=local '$realgo' "\$@"
EOF
chmod +x "$tmp/fakego/go1.99.0"
origin_main "$gomain"
PATH=$tmp/fakego:$PATH run 0 "main's Go raised: a good change on an older main still passes" \
  refs/remotes/origin/main "$good" "C6 checker built from origin/main"
if [ -s "$tmp/fakego/ran" ]; then
  echo "PASS  main's Go raised: the checker was built with the Go main's go.mod names"
  passed=$((passed + 1))
else
  echo "FAIL  main's Go raised: the checker was not built with the Go main's go.mod names"
  failed=$((failed + 1))
fi
PATH=$tmp/fakego:$PATH run 1 "main's Go raised: a weakened checker's commit is still rejected" \
  refs/remotes/origin/main "$weak" "$C6"

echo "c6_test: $passed passed, $failed failed"
[ "$failed" -eq 0 ]
