#!/usr/bin/env bash
# Builds the C6 checker (cmd/regcorpus; C6, R5) from a trusted commit. The
# pre-commit hook and CI (.github/workflows/c6.yml) both call it, so the build
# lives in one place.
#
# Usage: build-checker.sh [--auto-toolchain] OUT UNDER-TEST [TRUSTED...]
#   --auto-toolchain  build with the Go the source's go.mod names, downloaded
#               when the installed Go is older (CI). Without it the installed
#               Go builds, and one too old fails, naming the version (the
#               hook: nothing is downloaded on commit).
#   OUT         where to write the binary
#   UNDER-TEST  the tree-ish being checked (hook: the staged tree; CI: the commit)
#   TRUSTED...  revisions to build from, preferred first (hook: the known mains;
#               CI: origin/main, then the base, both resolved by c6.sh). One
#               that doesn't resolve is skipped.
# Builds from the first TRUSTED that holds cmd/regcorpus/main.go, and prints
# its commit oid on stdout. Only when none does (bootstrap: a fresh repo, or
# before the checker first lands on main) does it build from UNDER-TEST: it
# then warns on stderr and prints UNDER-TEST's tree oid.
#
# The source (go.mod, go.sum when present, cmd/, internal/) is copied as raw
# blobs (ls-tree, cat-file, no replace refs), the way the check reads a commit,
# so no filter or replace ref can change it. Go settings that could swap it
# (GOFLAGS, a go env file, go.work, a toolchain the environment names) are
# off for the build. Any failure exits 1 with the reason on stderr.
set -euo pipefail
export GIT_NO_REPLACE_OBJECTS=1

die() {
  echo "C6 checker: $*" >&2
  exit 1
}

toolchain=local
if [ "${1-}" = --auto-toolchain ]; then
  toolchain=auto
  shift
fi
[ $# -ge 2 ] || die "usage: build-checker.sh [--auto-toolchain] OUT UNDER-TEST [TRUSTED...]"
out=$1 under=$2
shift 2
case $out in /*) ;; *) out=$PWD/$out ;; esac

src= what= trusted=0
for rev in "$@"; do
  if oid=$(git rev-parse -q --verify "$rev^{commit}") &&
    git cat-file -e "$oid:cmd/regcorpus/main.go" 2>/dev/null; then
    src=$oid what="$rev ($oid)" trusted=1
    break
  fi
done
if [ "$trusted" = 0 ]; then
  src=$(git rev-parse -q --verify "$under^{tree}") || die "cannot read $under"
  what="the commit under test ($under)"
  echo "C6 checker: warning: bootstrap: no trusted main (${*:-none given}) has" \
    "cmd/regcorpus/main.go, so $what is checked by its own checker. This stops" \
    "once main has the checker." >&2
fi

tmp=$(mktemp -d "${TMPDIR:-/tmp}/regcorpus-checker.XXXXXX") || die "mktemp"
trap 'rm -rf "$tmp"' EXIT
git ls-tree -r -z --full-tree "$src" -- go.mod go.sum cmd internal >"$tmp/files" ||
  die "listing the checker in $what"
while IFS= read -r -d '' entry; do # "<mode> blob <oid>\t<path>"
  meta=${entry%%$'\t'*} path=${entry#*$'\t'}
  case $meta in
  '100644 blob '* | '100755 blob '*) ;;
  *) die "checker file $path in $what is not a regular file" ;;
  esac
  case $path in */*) mkdir -p "$tmp/src/${path%/*}" ;; esac
  git cat-file blob "${meta##* }" >"$tmp/src/$path" || die "reading $path from $what"
done <"$tmp/files"
[ -f "$tmp/src/go.mod" ] || die "no go.mod in $what"
if ! (cd "$tmp/src" && env GOENV=off GOFLAGS= GOWORK=off GOTOOLCHAIN="$toolchain" \
  go build -o "$out" ./cmd/regcorpus); then
  [ "$trusted" = 1 ] || die "building the checker from $what failed (go output above)."
  die "building the checker from $what failed (go output above).
Every commit is checked by main's checker, so it must build here. To recover:
- if go names a Go version, install that version (main's go.mod sets it;
  GOTOOLCHAIN=local never downloads one);
- otherwise main's checker is broken: the person who merges repairs main, and
  commits build again once your main ref has the repair (git fetch)."
fi
echo "$src"
