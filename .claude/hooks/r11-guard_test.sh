#!/usr/bin/env bash
# Tests for the R11 agent guard (.claude/settings.json + .claude/hooks/r11-guard.sh).
#
# Each case feeds one PreToolUse payload to the hook exactly as Claude Code
# launches it: the command string from .claude/settings.json, run by a shell,
# with CLAUDE_PROJECT_DIR set to a main checkout. The fixture is a real git
# repo (MAIN) with a git worktree under MAIN/.claude/worktrees/wt1 (WT), so
# worktree sessions (cwd = WT, CLAUDE_PROJECT_DIR = MAIN) are tested as they
# happen. Want 2 = blocked ("R11 guard: blocked — " on stderr), 0 = allowed
# (silent). The whole table runs under every bash found (PATH bash and the
# macOS /bin/bash 3.2), because the hook's shebang is `env bash`.
# Prints one PASS/FAIL line per case and exits non-zero on any failure.
# Run: bash .claude/hooks/r11-guard_test.sh   (also run by `make test`)
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd -P)
src=$(cd "$here/../.." && pwd -P)
for tool in jq git; do
  command -v "$tool" >/dev/null 2>&1 || { echo "r11-guard_test: $tool is required" >&2; exit 1; }
done

tmp=$(mktemp -d "${TMPDIR:-/tmp}/r11-guard-test.XXXXXX") || exit 1
tmp=$(cd "$tmp" && pwd -P)
trap 'rm -rf "$tmp"' EXIT
errfile=$tmp/stderr

# The fixture: MAIN is a main checkout holding this repo's guard and settings,
# under a directory whose name is NFC "café"; WT is a git worktree of it at
# MAIN/.claude/worktrees/wt1; WTN is a worktree outside it whose guard file is
# gone (still this checkout); CLONE is a throwaway clone (not this checkout).
ROOT=$tmp/caf$'\xc3\xa9'
MAIN=$ROOT/main
WT=$MAIN/.claude/worktrees/wt1
WTN=$tmp/wt-nomarker
CLONE=$tmp/clone
mkdir -p "$MAIN/.claude/hooks" "$MAIN/registry/records" "$MAIN/.githooks" \
  "$MAIN/internal/registry" "$MAIN/staging" "$MAIN/docs"
cp "$src/.claude/settings.json" "$MAIN/.claude/settings.json"
cp -p "$here/r11-guard.sh" "$MAIN/.claude/hooks/r11-guard.sh"
printf 'x\n' >"$MAIN/registry/verify.log"
printf 'x\n' >"$MAIN/AGENTS.md"
printf '.claude/worktrees/\n' >"$MAIN/.gitignore"
(
  cd "$MAIN" && git init -q && git add -A &&
    git -c user.name=t -c user.email=t@example.com commit -qm base &&
    git worktree add -q "$WT" -b wt1 &&
    git worktree add -q --detach "$WTN" && rm "$WTN/.claude/hooks/r11-guard.sh" &&
    git clone -q "$MAIN" "$CLONE"
) || { echo "r11-guard_test: fixture setup failed" >&2; exit 1; }
ln -s registry "$MAIN/reglink"                # a directory symlink into registry/
ln -s registry/verify.log "$MAIN/loglink"     # a file symlink into registry/
ln -s ../registry/records "$MAIN/staging/recl" # cd staging/recl && cd .. is staging (logical)
mkdir -p "$MAIN/.claude/worktrees/stale-dir"  # a leftover non-git dir
mkdir -p "$tmp/outside/registry"              # not in any git repo
UPMAIN=$(printf '%s' "$MAIN" | tr '[:lower:]' '[:upper:]') # same dir if case-insensitive
NFD=$tmp/cafe$'\xcc\x81'/main                 # MAIN with its parent spelled NFD
OTHER=$tmp/other                              # a git repo that is not a corpus,
mkdir -p "$OTHER/registry" "$OTHER/.claude/hooks" && git -C "$OTHER" init -q && # whose guard
  ln -s "$MAIN/.claude/hooks/r11-guard.sh" "$OTHER/.claude/hooks/r11-guard.sh" || # is a link
  { echo "r11-guard_test: fixture setup failed" >&2; exit 1; }

hookcmd=$(jq -er '.hooks.PreToolUse[0].hooks[0].command' "$MAIN/.claude/settings.json") ||
  { echo "r11-guard_test: no PreToolUse hook command in settings.json" >&2; exit 1; }
matcher=$(jq -er '.hooks.PreToolUse[0].matcher' "$MAIN/.claude/settings.json") ||
  { echo "r11-guard_test: no PreToolUse matcher in settings.json" >&2; exit 1; }

passed=0
failed=0
pass() { echo "PASS  $1"; passed=$((passed + 1)); }
fail() { echo "FAIL  $1"; failed=$((failed + 1)); }

# bins DIR TOOL...: a PATH directory holding only the named tools.
bins() {
  local dir=$1 t p
  shift
  mkdir -p "$dir"
  for t in "$@"; do
    p=$(command -v "$t") && ln -sf "$p" "$dir/$t"
  done
}

# payload TOOL KEY VALUE [CWD]: a PreToolUse payload with one tool_input field.
payload() {
  jq -cn --arg t "$1" --arg k "$2" --arg v "$3" --arg cwd "${4-$MAIN}" \
    '{hook_event_name: "PreToolUse", tool_name: $t, cwd: $cwd, tool_input: {($k): $v}}'
}

# expect WANT NAME JSON [PROJECT_DIR [PATH]]: run the hook command from
# settings.json on JSON as Claude Code does and compare the exit code. XENV
# (NAME=VALUE words, default none) is added to the hook's environment.
XENV=
expect() {
  local want=$1 name=$2 json=$3 pd=${4-$MAIN} path=${5-$PATH} got
  [ -z "$XENV" ] || name="$name [$XENV]"
  # shellcheck disable=SC2086 # XENV is a list of NAME=VALUE words
  printf '%s' "$json" | env PATH="$path" CLAUDE_PROJECT_DIR="$pd" $XENV /bin/sh -c "$hookcmd" >/dev/null 2>"$errfile"
  got=$?
  if [ "$got" = "$want" ] && { [ "$want" != 2 ] || grep -q '^R11 guard: blocked — ' "$errfile"; } &&
    { [ "$want" != 0 ] || [ ! -s "$errfile" ]; }; then
    pass "exit $got  $name"
  else
    fail "exit $got (want $want)  $name  stderr: $(tr '\n' ' ' <"$errfile")"
  fi
}

sh_case() { expect "$1" "Bash: $2 (cwd ${3-$MAIN})" "$(payload Bash command "$2" "${3-$MAIN}")"; }
file_case() { expect "$1" "$2: $3 (cwd ${4-$MAIN})" "$(payload "$2" file_path "$3" "${4-$MAIN}")"; }
mcp_case() { # WANT TOOL JSON-INPUT
  expect "$1" "$2 $3" "$(jq -cn --arg t "$2" --argjson in "$3" --arg cwd "$MAIN" \
    '{hook_event_name: "PreToolUse", tool_name: $t, cwd: $cwd, tool_input: $in}')"
}

table() {
  echo "-- settings: matcher and deny rules"
  # Claude Code tests a regex matcher unanchored (JavaScript RegExp.test), so
  # it also runs the guard on tools whose names merely contain an alternative
  # (TodoWrite); the guard must let those through.
  local t
  for t in Bash Edit Write MultiEdit NotebookEdit mcp__plugin_desktop-commander_desktop-commander__write_file mcp__srv__put; do
    if [[ $t =~ $matcher ]]; then pass "matcher covers $t"; else fail "matcher misses $t"; fi
  done
  for t in Read Grep Glob WebFetch WebSearch Task; do
    if [[ $t =~ $matcher ]]; then fail "matcher runs the guard on $t"; else pass "matcher skips $t"; fi
  done
  for t in TodoWrite ExitPlanMode; do
    if [[ $t =~ $matcher ]]; then
      expect 0 "$t: matched unanchored, let through" \
        "$(jq -cn --arg t "$t" '{tool_name: $t, cwd: "/", tool_input: {todos: [{content: "rm -rf registry"}]}}')"
    else
      pass "matcher skips $t"
    fi
  done
  for t in 'Edit(/registry/**)' 'Edit(/.githooks/**)' 'Edit(/.claude/settings.json)' \
    'Edit(/.claude/settings.local.json)' 'Edit(/.claude/hooks/**)' 'Bash(git commit -n *)' \
    'Bash(git commit --no-verify *)' 'Bash(regcorpus verify *)' 'Bash(./regcorpus verify *)' \
    'mcp__plugin_desktop-commander_desktop-commander__*'; do
    if jq -e --arg r "$t" '.permissions.deny | index($r)' "$MAIN/.claude/settings.json" >/dev/null; then
      pass "deny has $t"
    else
      fail "deny lacks $t"
    fi
  done
  if jq -e '.permissions.deny | map(select(. == "Edit(/.claude/**)" or test("^Write\\("))) | length == 0' \
    "$MAIN/.claude/settings.json" >/dev/null; then
    pass "deny has no whole-.claude rule (it would cover .claude/worktrees/) and no Write() rule"
  else
    fail "deny has a whole-.claude rule or a Write() rule"
  fi

  echo "-- fail closed: launch and internal errors"
  local noexec=$tmp/noexec
  mkdir -p "$noexec/.claude/hooks" && cp "$here/r11-guard.sh" "$noexec/.claude/hooks/" &&
    chmod -x "$noexec/.claude/hooks/r11-guard.sh"
  expect 2 "hook script missing" "$(payload Bash command ls)" "$tmp/nowhere"
  expect 2 "hook script not executable" "$(payload Bash command ls)" "$noexec"
  expect 2 "CLAUDE_PROJECT_DIR empty" "$(payload Bash command ls)" ""
  bins "$tmp/bin-nojq" sh bash env git readlink
  expect 2 "jq not installed" "$(payload Bash command ls)" "$MAIN" "$tmp/bin-nojq"
  bins "$tmp/bin-nogit" sh bash env jq readlink
  expect 2 "git not installed, path to judge" "$(payload Edit file_path "$MAIN/AGENTS.md")" "$MAIN" "$tmp/bin-nogit"
  bins "$tmp/bin-badgit" sh bash env jq readlink
  printf '#!/bin/sh\nexit 1\n' >"$tmp/bin-badgit/git" && chmod +x "$tmp/bin-badgit/git"
  expect 2 "git fails unexpectedly" "$(payload Edit file_path "$MAIN/AGENTS.md")" "$MAIN" "$tmp/bin-badgit"
  bins "$tmp/bin-killjq" sh bash env git readlink
  printf '#!/bin/sh\nkill -TERM "$PPID"\nsleep 1\n' >"$tmp/bin-killjq/jq" && chmod +x "$tmp/bin-killjq/jq"
  expect 2 "guard terminated by a signal" "$(payload Bash command ls)" "$MAIN" "$tmp/bin-killjq"
  bins "$tmp/bin-failjq" sh bash env git readlink
  printf '#!/bin/sh\nexit 5\n' >"$tmp/bin-failjq/jq" && chmod +x "$tmp/bin-failjq/jq"
  expect 2 "jq fails" "$(payload Bash command ls)" "$MAIN" "$tmp/bin-failjq"
  expect 2 "malformed hook input" '{"tool_name": "Bash", '
  expect 2 "Edit with no file path" '{"tool_name": "Edit", "cwd": "/", "tool_input": {}}'
  file_case 2 Edit ''
  file_case 2 Write "$MAIN/staging/a
b.yaml"
  file_case 2 Edit 'staging/x.yaml' ''

  echo "-- allowed: normal work (false-block regressions)"
  sh_case 0 'go test ./internal/registry/'
  sh_case 0 'mkdir -p internal/registry/testdata'
  sh_case 0 'git commit -m x && git log --oneline -n 3'
  sh_case 0 'git status --short | head -n 20'
  sh_case 0 'grep -rn "regcorpus verify" docs/'
  sh_case 0 'ls registry 2>/dev/null'
  sh_case 0 'ls registry >/dev/null 2>&1'
  sh_case 0 'go test ./... 2>&1 | tail -n 5'
  sh_case 0 "git commit -F - <<'EOF'
Guard: git commit -n and --no-verify stay blocked; rm -rf registry
echo x > registry/verify.log; ./regcorpus verify RX-1
EOF"
  sh_case 0 "git commit -m \"\$(cat <<'EOF'
Guard: block \"git commit -n\" (and > registry/x); allow ls registry)
cp a registry/ && regcorpus verify RX-1
EOF
)\""
  sh_case 0 "gh pr create --title t --body \"\$(cat <<'EOF'
Blocks --no-verify, \`git commit -n\` and writes into registry/ (R11).
EOF
)\""
  sh_case 0 'git commit -m "-n and --no-verify are blocked"'
  sh_case 0 'git commit -m x -- registry/records/RX-41-ACK-01.yaml'
  sh_case 0 'git add registry/records/X.yaml'
  sh_case 0 'git log -n 5 --no-walk'
  sh_case 0 'git push origin HEAD'
  sh_case 0 'git config --get core.hooksPath'
  sh_case 0 'git config core.hooksPath'
  sh_case 0 'make setup'
  sh_case 0 'make test'
  sh_case 0 'make -C . check'
  sh_case 0 'cat registry/verify.log'
  sh_case 0 'grep -n RX-41 registry/verify.log > /tmp/r11-out.txt'
  sh_case 0 'cp registry/verify.log /tmp/r11-copy.log'
  sh_case 0 './regcorpus check --corpus'
  sh_case 0 './regcorpus check --corpus 2>&1 | head'
  sh_case 0 'echo "never run regcorpus verify" && echo "git commit -n"'
  sh_case 0 'echo hello > staging/notes.txt'
  sh_case 0 'echo hello > internal/registry/notes.txt'
  sh_case 0 'touch internal/registry/x_test.go && rm internal/registry/x_test.go'
  sh_case 0 "sed -i '' 's/a/b/' internal/registry/doc.go"
  sh_case 0 'cd internal/registry && go test . && cd ../.. && echo ok > staging/x.txt'
  sh_case 0 '(cd registry && ls) && echo ok > staging/x.txt'
  sh_case 0 "printf '%s\\n' a b | tee staging/list.txt"
  sh_case 0 'echo x > /tmp/elsewhere-registry.txt'
  sh_case 0 "echo x > $tmp/outside/registry/x"
  sh_case 0 '# rm -rf registry'
  file_case 0 Edit "$MAIN/AGENTS.md"
  file_case 0 Write "$MAIN/staging/RX-41-ACK-01.yaml"
  file_case 0 Edit "$MAIN/internal/registry/doc.go"
  file_case 0 Edit "$MAIN/registryx/notes.md"
  file_case 0 Edit "$MAIN/registry/../AGENTS.md"
  file_case 0 Write "$tmp/outside/registry/x"
  file_case 0 Write "$MAIN/newdir/sub/file.go"
  expect 0 "Read: $MAIN/registry/verify.log" "$(payload Read file_path "$MAIN/registry/verify.log")"
  file_case 0 Write "$OTHER/registry/x.yaml"
  file_case 0 Edit "$OTHER/.claude/settings.json"
  sh_case 0 "echo x > $OTHER/registry/x.yaml && rm -rf $OTHER/.claude"
  sh_case 0 'cp a registry/x && mkdir .claude/plans' "$OTHER"
  mcp_case 0 mcp__srv__search '{"query": "registry"}'
  mcp_case 0 mcp__srv__write_file "{\"path\": \"$MAIN/staging/x.yaml\"}"

  echo "-- allowed: worktree sessions (cwd = WT, CLAUDE_PROJECT_DIR = MAIN)"
  file_case 0 Edit "$WT/AGENTS.md" "$WT"
  file_case 0 Write "$WT/internal/check/x.go" "$WT"
  file_case 0 Write "$WT/.gitignore" "$WT"
  file_case 0 Edit "$WT/AGENTS.md" "$MAIN"
  sh_case 0 'echo x > notes.txt' "$WT"
  sh_case 0 'mkdir -p internal/registry/testdata && go test ./...' "$WT"
  sh_case 0 "git commit -m x && echo done > $WT/staging/x" "$WT"

  echo "-- blocked: hook bypass"
  sh_case 2 'git commit --no-verify -m "x"'
  sh_case 2 'git commit -n -m x'
  sh_case 2 'git commit -m x -n'
  sh_case 2 'git commit -nm x'
  sh_case 2 'git commit -an -m "x"'
  sh_case 2 'git commit --no-verif -m x'
  sh_case 2 'git commit --no-v -m x'
  sh_case 2 'git -C . commit -nm "x"'
  sh_case 2 'git commit -m "a" -m "b" --no-verify'
  sh_case 2 'git add -A && git commit -m x -n'
  sh_case 2 'if git commit -n -m x; then echo ok; fi'
  sh_case 2 'timeout 60 git commit -n -m x'
  sh_case 2 'git push --no-verify'
  sh_case 2 'git merge --no-verify topic'
  sh_case 2 'git -c core.hooksPath=/dev/null commit -m x'
  sh_case 2 'git -c core.hookspath=/tmp/none commit -m "x"'
  sh_case 2 'git -c include.path=/tmp/evil.cfg commit -m x'
  sh_case 2 'GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null git commit -m x'
  sh_case 2 'env GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null git commit -m x'
  sh_case 2 "export GIT_CONFIG_PARAMETERS=\"'core.hooksPath'='/dev/null'\""
  sh_case 2 'git config core.hooksPath x'
  sh_case 2 'git config core.hooksPath /dev/null'
  sh_case 2 'git config --unset core.hooksPath'
  sh_case 2 'git config set core.hooksPath x'
  sh_case 2 'git config --local include.path /tmp/evil.cfg'

  echo "-- blocked: regcorpus verify, any common form"
  sh_case 2 'regcorpus verify RX-41-ACK-01'
  sh_case 2 'regcorpus verify'
  sh_case 2 './regcorpus verify RX-1'
  sh_case 2 "$MAIN/regcorpus verify RX-1"
  sh_case 2 'go run ./cmd/regcorpus verify RX-41-ACK-01'
  sh_case 2 'script -q /dev/null ./regcorpus verify RX-1'
  sh_case 2 "expect -c 'spawn ./regcorpus verify RX-41-ACK-01; interact'"
  sh_case 2 'unbuffer ./regcorpus verify RX-41-ACK-01'
  sh_case 2 "cd $MAIN && ./regcorpus verify"
  sh_case 2 'make build && ./regcorpus verify RX-1 < /dev/null'

  echo "-- blocked: shell writes under registry/, .githooks/, .claude/"
  sh_case 2 'echo x > registry/verify.log'
  sh_case 2 'echo x >registry/verify.log'
  sh_case 2 'echo x 1>>registry/verify.log'
  sh_case 2 'echo x &> registry/verify.log'
  sh_case 2 '> registry/verify.log'
  sh_case 2 'echo "RX-1, 1, abc, me, 2026-11-21, proposal_review" >> registry/verify.log'
  sh_case 2 "echo x > $MAIN/registry/verify.log"
  sh_case 2 'echo x > ./staging/../registry/verify.log'
  sh_case 2 'echo x > reglink/verify.log'
  sh_case 2 'echo x > loglink'
  sh_case 2 'echo x > Registry/verify.log'
  sh_case 2 '{ echo x; } > registry/verify.log'
  sh_case 2 '(echo x) > registry/verify.log'
  sh_case 2 'echo "$(echo x > registry/verify.log)"'
  sh_case 2 "bash -c 'echo x > registry/verify.log'"
  sh_case 2 'cat staged.yaml | tee registry/records/RX-41-ACK-01.yaml'
  sh_case 2 'echo x | tee -a "registry/verify.log"'
  sh_case 2 'cp staging/RX-41-ACK-01.yaml registry/records/'
  sh_case 2 'cp a registry/records/'
  sh_case 2 'cp -t registry/records staging/a.yaml'
  sh_case 2 'mv staging/RX-41-ACK-01.yaml registry/records/'
  sh_case 2 'mv registry/verify.log /tmp/'
  sh_case 2 'rsync a registry/'
  sh_case 2 'rsync -av staging/ registry/records/'
  sh_case 2 'install -m 644 a registry/records/x.yaml'
  sh_case 2 'dd if=/dev/zero of=registry/verify.log count=1'
  sh_case 2 'rm .githooks/pre-commit'
  sh_case 2 'rm -rf registry'
  sh_case 2 'rm -f registry/records/*.yaml'
  sh_case 2 'rmdir registry/records'
  sh_case 2 'unlink registry/verify.log'
  sh_case 2 'touch registry/records/RX-41-ACK-01.yaml'
  sh_case 2 'mkdir -p registry/records'
  sh_case 2 'ln -sf /dev/null .githooks/pre-commit'
  sh_case 2 'truncate -s 0 registry/verify.log'
  sh_case 2 'chmod -x .githooks/pre-commit'
  sh_case 2 'chmod -x .claude/hooks/r11-guard.sh'
  sh_case 2 "sed -i '' s/a/b/ .claude/settings.json"
  sh_case 2 "sed -i.bak 's/a/b/' registry/verify.log"
  sh_case 2 "sed -e 's/a/b/' -i registry/verify.log"
  sh_case 2 "perl -pi -e 's/a/b/' registry/verify.log"
  sh_case 2 'git rm registry/records/RX-41-ACK-01.yaml'
  sh_case 2 'git mv registry/verify.log registry/old.log'
  sh_case 2 'git checkout -- registry/verify.log'
  sh_case 2 'git checkout HEAD -- .githooks/pre-commit'
  sh_case 2 'git restore .claude/settings.json'
  sh_case 2 'git restore --source=HEAD~1 registry/verify.log'
  sh_case 2 'git clean -fd registry/'
  sh_case 2 'cd registry && echo x > verify.log'
  sh_case 2 'cd registry; rm verify.log'
  sh_case 2 'git -C registry rm verify.log'

  echo "-- blocked: relative writes from a cwd inside a protected dir"
  sh_case 2 'echo x > pre-commit' "$MAIN/.githooks"
  sh_case 2 'echo x >> verify.log' "$MAIN/registry"
  sh_case 2 'echo {} > settings.json' "$MAIN/.claude"
  sh_case 2 'rm r11-guard.sh' "$MAIN/.claude/hooks"
  file_case 2 Edit 'verify.log' "$MAIN/registry"
  sh_case 0 'cat pre-commit && ls' "$MAIN/.githooks"
  sh_case 0 'echo x > ../staging/x.txt' "$MAIN/registry"

  echo "-- blocked: path-qualified writers"
  sh_case 2 '/bin/cp a registry/records/'
  sh_case 2 '/bin/rm .githooks/pre-commit'
  sh_case 2 "/usr/bin/sed -i '' s/a/b/ .claude/settings.json"
  sh_case 2 '/bin/chmod -x .githooks/pre-commit'
  sh_case 2 '/usr/bin/git commit -n -m x'

  echo "-- blocked: other spellings of the repo path (case, Unicode normalization)"
  if [ -d "$UPMAIN" ]; then
    file_case 2 Write "$UPMAIN/registry/verify.log"
    file_case 2 Edit "$UPMAIN/Registry/verify.log"
    file_case 2 Edit "$UPMAIN/REGISTRY/new/x.yaml"
    sh_case 2 "echo x > $UPMAIN/registry/verify.log"
    sh_case 2 'echo x > verify.log' "$UPMAIN/registry"
    file_case 0 Edit "$UPMAIN/AGENTS.md"
    sh_case 0 'echo x > staging/x.txt' "$UPMAIN"
  else
    pass "case-sensitive file system: upper-case spelling is another path (skipped)"
  fi
  if [ -d "$NFD" ]; then
    file_case 2 Write "$NFD/registry/verify.log"
    sh_case 2 'rm verify.log' "$NFD/registry"
    file_case 0 Write "$NFD/staging/x.yaml"
  else
    pass "normalization-sensitive file system: NFD spelling is another path (skipped)"
  fi

  echo "-- corpus: this checkout and its worktrees, never a clone (G-I2)"
  file_case 2 Write "$WTN/registry/x.yaml" "$WTN"
  sh_case 2 'echo x > .githooks/pre-commit' "$WTN"
  sh_case 2 'echo x > .claude/worktrees/wt1/registry/verify.log'
  sh_case 2 'rm -rf .claude/worktrees/*/registry'
  file_case 0 Write "$CLONE/registry/verify.log"
  sh_case 0 "echo bad >> $CLONE/registry/verify.log"
  sh_case 0 "cd $CLONE && echo bad >> registry/verify.log && git commit -qam neg"
  sh_case 0 "cp /tmp/bad.yaml $CLONE/registry/records/ && rm -rf $CLONE/.claude/worktrees/x"
  sh_case 0 'git clone -q . /tmp/negclone && echo bad >> /tmp/negclone/registry/verify.log'
  file_case 0 Write "$OTHER/registry/x.yaml"
  echo "-- allowed: housekeeping under .claude/worktrees/"
  sh_case 0 'rm -rf .claude/worktrees/stale-dir'
  sh_case 0 'mkdir -p .claude/worktrees'
  sh_case 0 'git worktree prune && rm -rf .claude/worktrees/*'
  sh_case 0 'rm -rf .claude/worktrees/agent-*'
  sh_case 0 'rm -rf .claude/worktrees/wt1'
  sh_case 0 'du -sh .claude/worktrees/* > /tmp/du.txt'

  echo "-- allowed: targets that are not concrete paths (G-I1)"
  sh_case 0 'x=$(mktemp); ls > "$x"'
  sh_case 0 'cat "$x" > "$y"; echo hi > "${OUT}/a"; cp a "$DEST"' "$MAIN/registry"
  sh_case 0 'echo hi > "$TMPDIR/out.txt"' "$MAIN/.claude/hooks"
  sh_case 0 'go test ./... 2>&1 | tee "$TMPDIR/t.log"' "$MAIN/registry"
  sh_case 0 'ls > ~/listing.txt' "$MAIN/registry"
  sh_case 0 'cd registry && grep -rn ACK . > ~/acks.txt'
  sh_case 0 'cd .claude/hooks && bash r11-guard_test.sh 2>&1 | tee ~/guard-test.log'
  sh_case 0 'if (( 2 > 1 )); then echo y; fi' "$MAIN/registry"
  sh_case 0 'cd registry && echo $(( 3 > 2 ))'
  sh_case 0 'cd registry && n=$(ls records | wc -l); if (( n > 0 )); then echo some; fi'
  sh_case 0 'for ((i=10; i>0; i--)); do echo $i; done' "$MAIN/registry"
  sh_case 0 'while (( $# > 0 )); do shift; done' "$MAIN/.claude"
  sh_case 0 '[[ b > a ]] && echo y' "$MAIN/.githooks"
  sh_case 0 'if [[ "$a" > "$b" || ( -n $c ) ]]; then echo gt; fi' "$MAIN/registry"
  sh_case 0 'echo $[1+2] $((x<<2)) $((x>>2))' "$MAIN/registry"
  sh_case 0 'touch -r registry/verify.log staging/stamp'
  sh_case 0 'git clean -fdx -e registry'
  sh_case 0 'git clean -fdx -e .claude --exclude=.githooks'
  sh_case 0 'truncate -s 0 /tmp/x.log' "$MAIN/registry"
  sh_case 0 'cp -t /tmp/snap registry/records/RX-41-ACK-01.yaml'
  sh_case 0 'cp -t /tmp/snap registry/verify.log registry/records/x.yaml'
  sh_case 0 'cp --target-directory=staging registry/verify.log'
  sh_case 0 'cp -rt staging registry/records'
  sh_case 0 'ln -st staging ../registry/verify.log'
  sh_case 0 'gcp -t staging/ registry/verify.log'
  sh_case 0 'rsync -t registry/verify.log /tmp/snap/'
  sh_case 0 'rsync -a --exclude .claude -e ssh registry/ host:/srv/snap/'
  sh_case 0 'ln -s registry/verify.log'
  sh_case 0 'chmod 644 /tmp/x && chown me:staff /tmp/x' "$MAIN/registry"
  sh_case 0 "sed -i '' s/a/b/ /tmp/x" "$MAIN/.claude/hooks"
  sh_case 0 "sed -i.bak -e s/a/b/ /tmp/x" "$MAIN/.claude/hooks"
  sh_case 0 'mkdir -m 755 /tmp/d' "$MAIN/registry"
  sh_case 0 'cd "$(git rev-parse --show-toplevel)" && ls registry'

  echo "-- blocked: \$HOME, ~ and \$TMPDIR are expanded (G-I1)"
  XENV="HOME=$ROOT"
  sh_case 2 'echo x > ~/main/registry/verify.log'
  sh_case 2 'cp a "$HOME/main/registry/"'
  sh_case 2 'echo x > ${HOME}/main/.githooks/pre-commit'
  sh_case 2 'cd ~/main && rm registry/verify.log' "$tmp"
  sh_case 2 'cd && rm main/registry/verify.log' "$tmp"
  sh_case 2 'dd if=/dev/zero of=~/main/registry/verify.log count=1' "$tmp"
  sh_case 0 'echo x > ~/main/staging/x.txt'
  sh_case 0 'echo x > "~/main/registry/verify.log"' "$tmp"
  XENV="TMPDIR=$ROOT"
  sh_case 2 'echo x > "$TMPDIR/main/registry/verify.log"'
  sh_case 2 'cd $TMPDIR/main/registry && rm verify.log' "$tmp"
  XENV=

  echo "-- cwd: followed through pushd/popd, env -C and \$(...) (G-I3)"
  sh_case 0 'pushd /tmp && rm -rf registry && popd'
  sh_case 0 'pushd /tmp >/dev/null; mkdir -p .claude/x; popd'
  sh_case 0 'env -C /tmp rm -rf registry'
  sh_case 0 '/usr/bin/env --chdir=/tmp rm -rf .claude'
  sh_case 0 'cd /tmp && x="$(touch registry)"'
  sh_case 0 'cd /tmp && echo `rm -rf registry`'
  sh_case 0 '(cd /tmp && rm -rf registry) && ls'
  sh_case 0 'cd .claude/hooks && bash r11-guard_test.sh > "$TMPDIR/out.txt" 2>&1'
  sh_case 0 'cd staging/recl && cd .. && echo x > verify.log'
  sh_case 0 'cd "$d" && rm -rf registry'
  sh_case 0 'pushd registry && popd && popd && rm -rf registry'
  sh_case 2 'pushd registry && rm verify.log'
  sh_case 2 'pushd /tmp && popd && rm registry/verify.log'
  sh_case 2 'pushd registry/records >/dev/null && pushd /tmp && popd && rm x.yaml'
  sh_case 2 'env -C registry rm verify.log'
  sh_case 2 '/usr/bin/env -C .githooks rm pre-commit'
  sh_case 2 'cd registry && echo "$(rm verify.log)"'
  sh_case 2 'cd registry && x=`touch new.yaml`'
  sh_case 2 'cd /tmp && echo "$(cd '"$MAIN"' && rm registry/verify.log)"'
  sh_case 2 'cd registry/records && cd .. && rm verify.log'
  sh_case 2 'cd -P staging/recl && cd .. && echo x > verify.log'
  sh_case 2 'cd reglink && cd ../.githooks && echo x > pre-commit'
  sh_case 2 'cd /tmp & rm -rf registry'
  sh_case 2 'cd /tmp | cat; rm -rf registry'
  sh_case 2 'ls | cd /tmp; rm -rf registry'
  sh_case 0 'cd registry | cat; echo x > verify.log'
  sh_case 2 'cd /tmp | rm -rf registry'
  sh_case 0 'cd registry | echo x > verify.log'

  echo "-- blocked: writers run by xargs, find, parallel and wrappers"
  sh_case 2 'ls staging | xargs -I{} cp {} registry/records/'
  sh_case 2 'echo a | xargs cp -t registry/records'
  sh_case 2 'xargs rm registry/verify.log < /dev/null'
  sh_case 2 'ls | xargs -0 -n 1 rm -f .githooks/pre-commit'
  sh_case 2 'find staging -name "*.yaml" -exec cp {} registry/records/ \;'
  sh_case 2 'find registry -name "*.yaml" -exec rm {} +'
  sh_case 2 'find . -execdir rm {} \;' "$MAIN/registry"
  sh_case 2 'find -L staging registry -type f -ok rm {} ";"'
  sh_case 2 'parallel cp {} registry/records/ ::: a b'
  sh_case 2 'parallel rm ::: staging/a registry/verify.log'
  sh_case 2 "parallel ::: 'rm registry/verify.log'"
  sh_case 2 '/usr/bin/env cp a registry/'
  sh_case 2 'env -i PATH=/bin cp a registry/'
  sh_case 2 "env -S 'rm registry/verify.log'"
  sh_case 2 'nohup cp a registry/x &'
  sh_case 2 'time cp a registry/'
  sh_case 2 'time -p cp a registry/'
  sh_case 2 '/usr/bin/time -p rm registry/verify.log'
  sh_case 2 '/usr/bin/time -o registry/t.txt ls'
  sh_case 2 'nice -n 5 rm registry/verify.log'
  sh_case 2 'stdbuf -oL tee registry/verify.log'
  sh_case 2 'timeout -s KILL 5 rm registry/verify.log'
  sh_case 2 'command -p rm registry/verify.log'
  sh_case 2 'git checkout registry/verify.log'
  sh_case 2 'git checkout HEAD registry/verify.log .githooks/pre-commit'
  sh_case 2 'git checkout .' "$MAIN/registry"
  sh_case 2 'gsed -i s/a/b/ registry/verify.log'
  sh_case 2 'gcp a registry/records/'
  sh_case 2 'gmv registry/verify.log /tmp/'
  sh_case 2 'grm -f .githooks/pre-commit'
  sh_case 2 'gtouch registry/x && gmkdir -p registry/y'
  sh_case 2 'ln -s /tmp/x' "$MAIN/registry"
  sh_case 2 'cp -rt registry/records staging/a'
  sh_case 2 'mv -t /tmp registry/verify.log'
  sh_case 2 'install -d registry/new'
  sh_case 2 'chmod -R u+w registry'
  sh_case 2 'chmod -x .githooks/pre-commit'
  sh_case 2 'chown -R me registry'
  sh_case 2 'touch -r /tmp/ref registry/verify.log'
  sh_case 2 "sed -i '' -e s/a/b/ .claude/settings.json"
  sh_case 2 "bash -o pipefail -c 'rm registry/verify.log | cat'"
  sh_case 0 'find registry -name "*.yaml" -print0 | xargs -0 cp -t /tmp/snap'
  sh_case 0 'find registry -name "*.yaml" -exec cp {} /tmp/snap/ \;'
  sh_case 0 'find . -name "*.tmp" -exec rm {} +'
  sh_case 0 'xargs -n1 echo < /tmp/list'
  sh_case 0 'ls registry | xargs -I{} cat registry/{}'
  sh_case 0 'ls registry | xargs cp -t /tmp/snap'
  sh_case 0 'parallel gzip ::: /tmp/a /tmp/b'
  sh_case 0 'parallel cp {} /tmp/snap/ ::: registry/verify.log'
  sh_case 0 'nohup make test > /tmp/nohup.out 2>&1 &'
  sh_case 0 'time go test ./... && timeout 60 go vet ./...'
  sh_case 0 'git checkout main && git checkout -b feature/registry-types'
  sh_case 0 'git checkout main' "$MAIN/registry"
  sh_case 0 'git checkout -'
  sh_case 0 'git checkout HEAD~1 -- internal/registry/doc.go'
  sh_case 0 'gsed -n p registry/verify.log && grep -c x registry/verify.log'

  echo "-- allowed: index-only git operations"
  sh_case 0 'git restore --staged registry/records/RX-41-ACK-01.yaml'
  sh_case 0 'git restore --staged .claude/settings.json'
  sh_case 0 'git restore -S -- registry/'
  sh_case 0 'git rm --cached .claude/settings.local.json'
  sh_case 0 'git rm -r --cached registry/records'
  sh_case 0 'git rm --cached registry/verify.log'
  sh_case 2 'git restore --staged --worktree registry/verify.log'
  sh_case 2 'git restore -SW registry/verify.log'

  echo "-- lexer: here-strings, nesting"
  sh_case 2 $'cat <<< x\necho bad > registry/verify.log'
  sh_case 2 $'grep -c a <<< "$v"\ngit commit --no-verify -m x'
  sh_case 0 'cat <<< "registry/verify.log" | xargs wc -l'
  local flat= deep=x i
  for ((i = 0; i < 300; i++)); do flat+='echo "$(date +%s)"'$'\n'; done
  sh_case 0 "$flat"
  for ((i = 0; i < 40; i++)); do deep="\$(echo $deep)"; done
  expect 2 "Bash: 40 nested \$(...) (fail closed)" "$(payload Bash command "echo $deep")"

  echo "-- blocked: file tools under registry/, .githooks/, .claude/"
  file_case 2 Edit "$MAIN/registry/records/RX-41-ACK-01.yaml"
  file_case 2 Write "$MAIN/registry/verify.log"
  file_case 2 Write "$MAIN/registry"
  file_case 2 MultiEdit "$MAIN/.githooks/pre-commit"
  file_case 2 Write "$MAIN/.claude/settings.json"
  file_case 2 Write "$MAIN/.claude/settings.local.json"
  file_case 2 Edit "$MAIN/.claude/hooks/r11-guard.sh"
  file_case 2 Edit "$MAIN/staging/../registry/records/RX-41-ACK-01.yaml"
  file_case 2 Write "$MAIN/eval/.././/registry//records/RX-41-ACK-01.yaml"
  file_case 2 Edit "$MAIN/Registry/verify.log"
  file_case 2 Edit "$MAIN/reglink/verify.log"
  file_case 2 Edit "$MAIN/loglink"
  file_case 2 Edit 'registry/verify.log'
  file_case 2 Edit '../registry/verify.log' "$MAIN/staging"
  expect 2 "NotebookEdit: $MAIN/.claude/x.ipynb" \
    "$(payload NotebookEdit notebook_path "$MAIN/.claude/x.ipynb")"

  echo "-- blocked: worktree sessions (cwd = WT, CLAUDE_PROJECT_DIR = MAIN)"
  file_case 2 Write "$WT/registry/verify.log" "$WT"
  file_case 2 Edit "$WT/.claude/settings.json" "$WT"
  file_case 2 Edit "$WT/.githooks/pre-commit" "$WT"
  file_case 2 Write "$MAIN/registry/records/x.yaml" "$WT"
  file_case 2 Write "$WT/registry/records/x.yaml" "$MAIN"
  sh_case 2 'echo x > registry/verify.log' "$WT"
  sh_case 2 'cp a registry/records/' "$WT"
  sh_case 2 "echo x > $MAIN/registry/verify.log" "$WT"

  echo "-- blocked: MCP file tools"
  mcp_case 2 mcp__plugin_desktop-commander_desktop-commander__write_file "{\"path\": \"$MAIN/registry/verify.log\", \"content\": \"x\"}"
  mcp_case 2 mcp__plugin_desktop-commander_desktop-commander__move_file "{\"source\": \"$MAIN/staging/x\", \"destination\": \"$MAIN/registry/records/x.yaml\"}"
  mcp_case 2 mcp__plugin_desktop-commander_desktop-commander__edit_block "{\"file_path\": \"$MAIN/.claude/settings.json\"}"
  mcp_case 2 mcp__srv__write_files "{\"paths\": [\"$MAIN/staging/a\", \"$MAIN/.githooks/pre-commit\"]}"
  mcp_case 2 mcp__filesystem__create_directory "{\"path\": \"$MAIN/registry/new\"}"
  mcp_case 2 mcp__srv__writeFile "{\"options\": {\"target\": \"$MAIN/registry/verify.log\"}}"
  XENV="HOME=$ROOT"
  mcp_case 2 mcp__filesystem__write_file '{"path": "~/main/registry/verify.log", "content": "x"}'
  XENV=

  echo "-- allowed: MCP reads, remote paths, non-path values"
  mcp_case 0 mcp__filesystem__read_text_file "{\"path\": \"$MAIN/registry/verify.log\"}"
  mcp_case 0 mcp__filesystem__list_directory "{\"path\": \"$MAIN/.claude\"}"
  mcp_case 0 mcp__filesystem__search_files "{\"path\": \"$MAIN\", \"pattern\": \"*.yaml\", \"excludePatterns\": [\"registry\"]}"
  mcp_case 0 mcp__github__get_file_contents '{"owner": "o", "repo": "r", "path": "registry/records/RX-41-ACK-01.yaml", "ref": "main"}'
  mcp_case 0 mcp__github__list_commits '{"owner": "o", "repo": "r", "path": ".claude/settings.json"}'
  mcp_case 0 mcp__github__create_or_update_file '{"owner": "o", "repo": "r", "path": "registry/verify.log", "content": "x"}'
  mcp_case 0 mcp__srv__write_file '{"path": "registry/verify.log"}'
  mcp_case 0 mcp__jupyter__insert_cell '{"cell_type": "code", "source": "import json\nprint(1)"}'
  mcp_case 0 mcp__jupyter__edit_cell '{"source": "/* js */\nconsole.log(1)"}'
  mcp_case 0 mcp__srv__x '{"path": "registry", "paths": ["a\tb"], "target": ".claude"}'
  mcp_case 0 mcp__vercel__create_redirect '{"source": "/old", "destination": "/new"}'
}

ran=
for b in "$(command -v bash)" /bin/bash; do
  [ -x "$b" ] || continue
  case " $ran " in *" $("$b" -c 'echo $BASH_VERSION') "*) continue ;; esac
  ran="$ran $("$b" -c 'echo $BASH_VERSION')"
  bins "$tmp/bash-$passed-$failed" bash
  ln -sf "$b" "$tmp/bash-$passed-$failed/bash"
  echo "== hook under bash $("$b" -c 'echo $BASH_VERSION') ($b)"
  PATH="$tmp/bash-$passed-$failed:$PATH" table
done

echo "r11-guard_test: $passed passed, $failed failed"
[ "$failed" -eq 0 ]
