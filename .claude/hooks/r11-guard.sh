#!/usr/bin/env bash
# R11 agent guard: a Claude Code PreToolUse hook, registered in
# .claude/settings.json for Bash|Edit|Write|MultiEdit|NotebookEdit.
#
# Blocks (stderr "R11 guard: blocked — <reason>", exit 2):
#   - Bash: --no-verify; `git ... commit` with a short-flag cluster holding n;
#     any mention of core.hooksPath; `regcorpus verify` in any form.
#   - Bash, best-effort: a command that mentions registry, .githooks or .claude
#     and contains a write operation. `git add` alone is allowed.
#   - Edit/Write/MultiEdit/NotebookEdit on a path that is or is under
#     registry/, .githooks/ or .claude/ (relative to $CLAUDE_PROJECT_DIR).
# Fails closed: no jq, unreadable input or no path to judge all block.
#
# These are guards, not proof (R11): the gate stays C6 and `regcorpus verify`
# requires a terminal. Bash 3.2 compatible (macOS /bin/bash).
set -uo pipefail
shopt -s nocasematch # macOS paths are case-insensitive; so are git config keys

block() {
  printf 'R11 guard: blocked — %s\n' "$1" >&2
  exit 2
}

command -v jq >/dev/null 2>&1 || block "jq is not installed, so the tool call cannot be checked (fail closed)"

input=
IFS= read -r -d '' input || true

field() {
  jq -r "$1" <<<"$input" 2>/dev/null || block "unreadable hook input (fail closed)"
}
tool=$(field '.tool_name // ""') || exit 2
cmd=$(field '.tool_input.command // ""') || exit 2
file=$(field '.tool_input.file_path // .tool_input.notebook_path // ""') || exit 2
cwd=$(field '.cwd // ""') || exit 2

protected='(^|[^[:alnum:]_-])(registry|\.githooks|\.claude)(/|[[:space:]"'"'"';&|)]|$)'

check_bash() {
  local c=$1 after
  [[ $c == *--no-verify* ]] && block "--no-verify skips the git hooks (R11)"
  if [[ $c =~ (^|[^[:alnum:]_.-])git([[:space:]]|$) && $c == *commit* ]]; then
    after=" ${c#*commit}"
    [[ $after =~ [[:space:]]-[[:alpha:]]*n[[:alpha:]]*([[:space:]\;\&\|]|$) ]] &&
      block "git commit -n skips the pre-commit hook (R11)"
  fi
  [[ $c == *core.hookspath* ]] && block "core.hooksPath is set only by make setup (R11)"
  [[ $c =~ regcorpus.*[[:space:]\"\']verify([[:space:]\"\'\;\&\|\)]|$) ]] &&
    block "regcorpus verify is run by a person in a terminal, never by an agent (R2a, R11)"

  if [[ $c =~ $protected ]]; then
    if [[ $c == *">"* ]] ||
      [[ $c =~ (^|[^[:alnum:]_./-])(tee|cp|mv|rm|touch|mkdir|ln|truncate|chmod)([[:space:]]|$) ]] ||
      [[ $c =~ (^|[^[:alnum:]_.-])sed([[:space:]].*)?[[:space:]]-(-in-place|[[:alpha:]]*i) ]] ||
      [[ $c =~ (^|[^[:alnum:]_.-])git[[:space:]](.*[[:space:]])?(checkout|restore)([[:space:]]|$) ]]; then
      block "shell writes under registry/, .githooks/ or .claude/ are not allowed (R11)"
    fi
  fi
  return 0
}

# normalize PATH: collapse //, drop ., resolve .. lexically (no symlinks).
normalize() {
  local IFS=/ seg
  local -a parts out
  out=()
  read -r -a parts <<<"$1"
  for seg in ${parts[@]+"${parts[@]}"}; do
    case $seg in
      '' | .) ;;
      ..) [ ${#out[@]} -gt 0 ] && unset "out[$((${#out[@]} - 1))]" ;;
      *) out+=("$seg") ;;
    esac
  done
  if [ ${#out[@]} -eq 0 ]; then
    printf '/'
  else
    printf '/%s' "${out[@]}"
  fi
}

check_path() {
  local p=$1 base root norm rel d
  [ -n "$p" ] || block "$tool call has no file path to check (fail closed)"
  [ -n "${CLAUDE_PROJECT_DIR:-}" ] || block "CLAUDE_PROJECT_DIR is not set, so $p cannot be checked (fail closed)"
  root=$(normalize "$CLAUDE_PROJECT_DIR")
  if [[ $p != /* ]]; then
    base=${cwd:-$CLAUDE_PROJECT_DIR}
    [[ $base == /* ]] || base=$CLAUDE_PROJECT_DIR/$base
    p=$base/$p
  fi
  norm=$(normalize "$p")
  [[ $norm == "$root"/* ]] || return 0 # the project root itself, or outside it
  rel=${norm#"$root"/}
  for d in registry .githooks .claude; do
    if [[ $rel == "$d" || $rel == "$d"/* ]]; then
      block "$tool on $rel: $d/ is not writable by agents (R11)"
    fi
  done
  return 0
}

case $tool in
  Bash) check_bash "$cmd" ;;
  Edit | Write | MultiEdit | NotebookEdit) check_path "$file" ;;
esac
exit 0
