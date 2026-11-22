#!/usr/bin/env bash
# Labels as code: create or update each label in .github/labels.json on this
# checkout's GitHub repo (gh's default). `gh label create --force` updates a
# label that exists, so a rerun changes nothing. Labels not in the file, such
# as GitHub's other defaults, are left alone. The whole file is checked before
# gh is called, so a bad file changes nothing. Needs gh (signed in) and jq.
# Run: make labels   (or bash .github/labels.sh [FILE]; tests: labels_test.sh)
set -uo pipefail

file=${1:-$(cd "$(dirname "$0")" && pwd -P)/labels.json}
for tool in jq gh; do
  command -v "$tool" >/dev/null 2>&1 || { echo "labels: $tool is required" >&2; exit 1; }
done

# A non-empty list of {name, color, description}: GitHub's limits (name up to
# 50 characters, description up to 100), a 6-hex color without '#', and names
# unique ignoring case, as GitHub compares them.
valid='type == "array" and length > 0
  and all(.[]; type == "object" and (keys == ["color", "description", "name"])
    and (.name | type == "string" and test("^\\S(.{0,48}\\S)?$"))
    and (.color | type == "string" and test("^[0-9a-fA-F]{6}$"))
    and (.description | type == "string" and length > 0 and length <= 100))
  and (map(.name | ascii_downcase) | length == (unique | length))'
jq -e "$valid" "$file" >/dev/null 2>&1 || {
  echo "labels: $file: not a list of {name, color, description} with unique names, 6-hex colors and GitHub's lengths" >&2
  exit 1
}

n=$(jq length "$file")
i=0
while [ "$i" -lt "$n" ]; do
  name=$(jq -r ".[$i].name" "$file")
  gh label create "$name" --color "$(jq -r ".[$i].color" "$file")" \
    --description "$(jq -r ".[$i].description" "$file")" --force ||
    { echo "labels: gh failed on $name" >&2; exit 1; }
  i=$((i + 1))
done
