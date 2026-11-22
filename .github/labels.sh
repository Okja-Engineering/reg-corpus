#!/usr/bin/env bash
# Labels as code: create or update each label in .github/labels.json on this
# checkout's GitHub repo (gh's default). `gh label create --force` updates a
# label that exists, so a rerun changes nothing. Labels not in the file, such
# as GitHub's other defaults, are left alone. The whole file is checked before
# gh is called, so a bad file changes nothing. Writes to GitHub. Needs gh
# (signed in) and jq.
# Run: make labels   (or bash .github/labels.sh [FILE]; tests: labels_test.sh)
set -uo pipefail

file=${1:-$(cd "$(dirname "$0")" && pwd -P)/labels.json}
for tool in jq gh; do
  command -v "$tool" >/dev/null 2>&1 || { echo "labels: $tool is required" >&2; exit 1; }
done

# The file holds exactly one JSON value: a non-empty list of {name, color,
# description}, with GitHub's limits (a name of 1 to 50 characters with no
# space at either end, a description of 1 to 100), a 6-hex color without '#',
# and names unique ignoring case, as GitHub compares them. The file is read
# once, slurped, and the labels synced are that one checked list.
valid='type == "array" and length > 0
  and all(.[]; type == "object" and (keys == ["color", "description", "name"])
    and (.name | type == "string" and test("^\\S(.{0,48}\\S)?$"))
    and (.color | type == "string" and test("^[0-9a-fA-F]{6}$"))
    and (.description | type == "string" and length > 0 and length <= 100))
  and (map(.name | ascii_downcase) | length == (unique | length))'
list=$(jq -c -e -s "if length == 1 and (.[0] | $valid) then .[0] else false end" "$file" 2>/dev/null) || {
  echo "labels: $file: not one list of {name, color, description} with unique names, 6-hex colors and GitHub's lengths" >&2
  exit 1
}

# Options first, then `--`: a name starting with '-' is a name, not an option.
n=$(jq length <<<"$list")
i=0
while [ "$i" -lt "$n" ]; do
  name=$(jq -r ".[$i].name" <<<"$list")
  gh label create --color "$(jq -r ".[$i].color" <<<"$list")" \
    --description "$(jq -r ".[$i].description" <<<"$list")" --force -- "$name" ||
    { echo "labels: gh failed on $name" >&2; exit 1; }
  i=$((i + 1))
done
