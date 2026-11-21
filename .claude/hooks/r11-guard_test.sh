#!/usr/bin/env bash
# Tests for the R11 agent guard (.claude/hooks/r11-guard.sh).
# A table of PreToolUse payloads and the exit code each must produce:
# 2 = blocked (with "R11 guard: blocked — " on stderr), 0 = allowed.
# Prints one PASS/FAIL line per case and exits non-zero on any failure.
# Run: bash .claude/hooks/r11-guard_test.sh   (also run by `make test`)
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
guard="$here/r11-guard.sh"
proj=/work/corpus
errfile=$(mktemp "${TMPDIR:-/tmp}/r11-guard-test.XXXXXX")
trap 'rm -f "$errfile"' EXIT
passed=0
failed=0

command -v jq >/dev/null 2>&1 || { echo "r11-guard_test: jq is required to build payloads" >&2; exit 1; }

# payload TOOL KEY VALUE [CWD]: a PreToolUse payload with one tool_input field.
payload() {
  jq -cn --arg t "$1" --arg k "$2" --arg v "$3" --arg cwd "${4-$proj}" \
    '{hook_event_name: "PreToolUse", tool_name: $t, cwd: $cwd, tool_input: {($k): $v}}'
}

# expect WANT NAME JSON [PATH]: run the guard on JSON and compare the exit code.
expect() {
  local want=$1 name=$2 json=$3 path=${4-$PATH} got
  printf '%s' "$json" | PATH=$path CLAUDE_PROJECT_DIR=$proj "$BASH" "$guard" >/dev/null 2>"$errfile"
  got=$?
  if [ "$got" = "$want" ] && { [ "$want" != 2 ] || grep -q '^R11 guard: blocked — ' "$errfile"; } &&
     { [ "$want" != 0 ] || [ ! -s "$errfile" ]; }; then
    echo "PASS  exit $got  $name"
    passed=$((passed + 1))
  else
    echo "FAIL  exit $got (want $want)  $name  stderr: $(tr '\n' ' ' <"$errfile")"
    failed=$((failed + 1))
  fi
}

sh_case() { expect "$1" "Bash: $2" "$(payload Bash command "$2")"; }
file_case() { expect "$1" "$2: $3 (cwd ${4-$proj})" "$(payload "$2" file_path "$3" "${4-$proj}")"; }

echo "-- blocked: hook bypass"
sh_case 2 'git commit --no-verify -m "x"'
sh_case 2 'git push --no-verify'
sh_case 2 'git commit -n -m "x"'
sh_case 2 'git commit -an -m "x"'
sh_case 2 'git commit -m "x" -n'
sh_case 2 'git -C . commit -nm "x"'
sh_case 2 'git config core.hooksPath /dev/null'
sh_case 2 'git -c core.hookspath=/tmp/none commit -m "x"'
sh_case 2 'git config --unset core.hooksPath'

echo "-- blocked: regcorpus verify, any form"
sh_case 2 'regcorpus verify RX-41-ACK-01'
sh_case 2 './regcorpus verify RX-41-ACK-01'
sh_case 2 'go run ./cmd/regcorpus verify RX-41-ACK-01'
sh_case 2 'script -q /dev/null ./regcorpus verify RX-41-ACK-01'
sh_case 2 "expect -c 'spawn ./regcorpus verify RX-41-ACK-01; interact'"
sh_case 2 'unbuffer ./regcorpus verify RX-41-ACK-01'
sh_case 2 'cd /work/corpus && ./regcorpus verify'

echo "-- blocked: shell writes under registry/, .githooks/, .claude/"
sh_case 2 'echo x > registry/verify.log'
sh_case 2 'echo "RX-1, 1, abc, me, 2026-11-21, proposal_review" >> registry/verify.log'
sh_case 2 'cat staged.yaml | tee registry/records/RX-41-ACK-01.yaml'
sh_case 2 'cp staging/RX-41-ACK-01.yaml registry/records/'
sh_case 2 'mv staging/RX-41-ACK-01.yaml registry/records/'
sh_case 2 'rm .githooks/pre-commit'
sh_case 2 'git rm registry/records/RX-41-ACK-01.yaml'
sh_case 2 "sed -i '' 's/a/b/' .claude/settings.json"
sh_case 2 'touch registry/records/RX-41-ACK-01.yaml'
sh_case 2 'mkdir -p registry/records'
sh_case 2 'ln -sf /dev/null .githooks/pre-commit'
sh_case 2 'truncate -s 0 registry/verify.log'
sh_case 2 'chmod -x .githooks/pre-commit'
sh_case 2 'git checkout -- registry/verify.log'
sh_case 2 'git restore .claude/settings.json'
sh_case 2 'rm -rf registry'

echo "-- blocked: file tools under registry/, .githooks/, .claude/"
file_case 2 Edit 'registry/records/RX-41-ACK-01.yaml'
file_case 2 Write "$proj/registry/verify.log"
file_case 2 MultiEdit "$proj/.githooks/pre-commit"
file_case 2 Write "$proj/.claude/settings.json"
file_case 2 Edit '.claude/hooks/r11-guard.sh'
file_case 2 Write "$proj/registry"
file_case 2 Edit 'staging/../registry/records/RX-41-ACK-01.yaml'
file_case 2 Edit '../registry/verify.log' "$proj/staging"
file_case 2 Write "$proj/eval/../../corpus/./registry//records/RX-41-ACK-01.yaml"
file_case 2 Edit "$proj/Registry/verify.log"
file_case 2 Edit 'registry/verify.log' ''
expect 2 "NotebookEdit: $proj/.claude/x.ipynb" \
  "$(payload NotebookEdit notebook_path "$proj/.claude/x.ipynb")"
expect 2 "Edit: registry path, no cwd field" \
  "$(jq -cn '{tool_name: "Edit", tool_input: {file_path: "registry/verify.log"}}')"

echo "-- blocked: fail closed"
expect 2 "jq missing" "$(payload Bash command 'ls')" /var/empty
expect 2 "malformed hook input" '{"tool_name": "Bash", '
expect 2 "Edit with no file path" '{"tool_name": "Edit", "tool_input": {}}'

echo "-- allowed"
sh_case 0 'git commit -m "x"'
sh_case 0 'git commit -m "Add the C6 registry-state rule"'
sh_case 0 'git add registry/records/X.yaml'
sh_case 0 'make setup'
sh_case 0 'make test'
sh_case 0 'cat registry/verify.log'
sh_case 0 'grep -n RX-41 registry/verify.log'
sh_case 0 './regcorpus check --corpus'
sh_case 0 'go test ./internal/verify/'
sh_case 0 'git log -n 5'
sh_case 0 'echo hello > staging/notes.txt'
file_case 0 Edit 'AGENTS.md'
file_case 0 Write "$proj/AGENTS.md"
file_case 0 Edit "$proj/registry/../AGENTS.md"
file_case 0 Write "$proj/staging/RX-41-ACK-01.yaml"
file_case 0 Edit "$proj/internal/registry/doc.go"
file_case 0 Edit "$proj/registryx/notes.md"
file_case 0 Write '/tmp/elsewhere/registry/x'
expect 0 "Read: $proj/registry/verify.log" "$(payload Read file_path "$proj/registry/verify.log")"

echo "r11-guard_test: $passed passed, $failed failed"
[ "$failed" -eq 0 ]
