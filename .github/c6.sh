#!/usr/bin/env bash
# CI's corpus gate (C6, R5), run from the repo root by
# .github/workflows/c6.yml in a clean environment.
#
# Usage: bash .github/c6.sh BASE COMMIT
# Builds the checker with .githooks/build-checker.sh, the same build the
# pre-commit hook uses, and runs `regcorpus check --corpus --tree COMMIT`.
# The checker comes from BASE when it has cmd/regcorpus/main.go, else from
# origin/main (refs/remotes/origin/main): once origin/main has the checker, no
# run uses any other. Only while neither has it (bootstrap: before the checker
# first lands on main) is it built from COMMIT, and a warning annotation says
# so. BASE, origin/main and COMMIT must each name a commit; a run that can't
# resolve one fails. Exits non-zero on any failure, with the check's
# violations on stdout.
set -euo pipefail
[ $# = 2 ] || { echo "usage: bash .github/c6.sh BASE COMMIT" >&2; exit 2; }

resolve() { # WHAT REV: print the commit REV names, or fail the run
  git rev-parse -q --verify "$2^{commit}" ||
    { echo "C6: cannot resolve $1 ($2), so the gate can't run" >&2; exit 1; }
}
base=$(resolve "the base" "$1")
main=$(resolve origin/main refs/remotes/origin/main)
commit=$(resolve "the commit under test" "$2")

tmp=$(mktemp -d "${TMPDIR:-/tmp}/regcorpus-ci.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
src=$(bash .githooks/build-checker.sh "$tmp/regcorpus" "$commit" "$base" "$main")
case $src in
  "$base") echo "C6 checker built from the base, $1 ($base)" ;;
  "$main") echo "C6 checker built from origin/main ($main): the base, $1, has none" ;;
  *) echo "::warning title=C6 bootstrap::Neither the base ($1) nor origin/main has cmd/regcorpus/main.go, so $2 is checked by its own checker." ;;
esac
echo "C6 check of $2:"
"$tmp/regcorpus" check --corpus --tree "$commit"
