#!/usr/bin/env bash
# CI's corpus gate (C6, R5), run from the repo root by
# .github/workflows/c6.yml in a clean environment.
#
# Usage: bash .github/c6.sh BASE COMMIT
# Builds the checker from BASE with .githooks/build-checker.sh, the same build
# the pre-commit hook uses, and runs `regcorpus check --corpus --tree COMMIT`.
# Only while BASE has no cmd/regcorpus/main.go (bootstrap: until PR #1 merges)
# is the checker built from COMMIT, and a warning annotation says so. Exits
# non-zero on any failure, with the check's violations on stdout.
set -euo pipefail
[ $# = 2 ] || { echo "usage: bash .github/c6.sh BASE COMMIT" >&2; exit 2; }
base=$1 commit=$2

tmp=$(mktemp -d "${TMPDIR:-/tmp}/regcorpus-ci.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
src=$(bash .githooks/build-checker.sh "$tmp/regcorpus" "$commit" "$base")
if [ "$src" = "$(git rev-parse -q --verify "$base^{commit}" || true)" ]; then
  echo "C6 checker built from the base, $base ($src)"
else
  echo "::warning title=C6 bootstrap::The base ($base) has no cmd/regcorpus/main.go, so $commit is checked by its own checker. This stops once main has the checker."
fi
echo "C6 check of $commit:"
"$tmp/regcorpus" check --corpus --tree "$commit"
