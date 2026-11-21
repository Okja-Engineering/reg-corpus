#!/usr/bin/env bash
# R11 agent guard: a Claude Code PreToolUse hook (spec R11).
#
# Threat model. This guards against a well-meaning agent's shortcuts; it is
# not a security boundary. The gate is C6 (the pre-commit hook now, CI later),
# and `regcorpus verify` separately requires a real terminal.
# It stops: edit/write tools and common shell writers and redirects into
# registry/, .githooks/ or .claude/ (also from inside those dirs and through
# path-qualified tools like /bin/cp); skipping the commit hook (--no-verify and
# its abbreviations, commit -n, hooksPath set via git config, -c or
# GIT_CONFIG_*); and running `regcorpus verify`.
# Design priority: never block normal work, even when that leaves a hole.
# It deliberately does not stop (left to C6 and CI): obfuscated commands
# (quoting tricks, encoded strings, variables, eval), git aliases and plumbing
# (commit-tree, update-ref, update-index), interpreters writing files (python,
# node), writers it doesn't know (find -delete, git apply), .git/config edits,
# and commits that skip pre-commit (merge, rebase, cherry-pick, am). A hook
# that times out does not block on this Claude Code version (2.1.221).
#
# .claude/settings.json runs it for Bash, Edit, Write, MultiEdit, NotebookEdit
# and MCP tools, through a wrapper that blocks when this file is missing or not
# executable. Exit 2 blocks, with "R11 guard: blocked — <reason>" on stderr;
# exit 0 allows, silently.
#
# Paths are judged physically. A path is resolved against the tool call's cwd
# (symlinks followed), then taken relative to the git toplevel of its deepest
# existing directory; it is protected when that toplevel is a corpus tree (it
# carries this guard) and the relative path is under registry/, .githooks/ or
# .claude/. A git worktree is its own toplevel, so worktree sessions are
# judged against their own tree. $CLAUDE_PROJECT_DIR is never used to judge a
# path. Paths outside a corpus work tree (other repos, ~/.claude) are allowed.
#
# Blocks:
#   - File tools and MCP tools: any path-like input (file_path, notebook_path,
#     path, paths, source, destination, target, old_path, new_path) that is
#     protected or holds a control character.
#   - Bash, one simple command at a time (split on && || ; | & newlines and
#     parentheses; quotes and here-docs respected; $(...), `...` and sh -c
#     strings checked as commands too):
#     . hook bypass: --no-verify on commit/merge/pull/push/rebase/am (or any
#       abbreviation from --no-v), -n in commit's own options, core.hooksPath
#       or include.path set through git config, git -c or GIT_CONFIG_*;
#     . regcorpus verify: run directly, by path, with go run, or under
#       script/expect/unbuffer;
#     . writes: a redirect, or a known writer (cp mv rm rmdir touch mkdir ln
#       install truncate chmod chown tee dd unlink rsync, sed -i, perl -i,
#       git rm/mv/restore/clean/checkout --) whose target is protected.
# Fails closed: unreadable input, jq or git missing, any internal error and a
# terminating signal all exit 2.
#
# Bash 3.2 compatible (macOS /bin/bash): no empty "${a[@]}" under set -u, no
# ${x,,}, no associative arrays. Results come back in globals, not $(...), so
# an error is never lost in a subshell.
set -u
export LC_ALL=C
shopt -s extglob

block() {
  trap - EXIT
  printf 'R11 guard: blocked — %s\n' "$1" >&2
  exit 2
}
on_exit() {
  local rc=$?
  printf 'R11 guard: blocked — internal error (exit %s), so the tool call cannot be checked (fail closed)\n' "$rc" >&2
  exit 2
}
trap on_exit EXIT
trap 'block "stopped by a signal, so the tool call cannot be checked (fail closed)"' HUP INT TERM

protected_msg='registry/, .githooks/ and .claude/ are not writable by agents (R11)'

# ---- paths ---------------------------------------------------------------

# physical PATH: RES = absolute PATH with every symlink resolved and . and ..
# removed. Components that don't exist yet are kept as written.
physical() {
  local todo=$1 cur= comp t n=0
  while :; do
    while [[ $todo == /* ]]; do todo=${todo#/}; done
    [ -n "$todo" ] || break
    if [[ $todo == */* ]]; then
      comp=${todo%%/*}
      todo=${todo#*/}
    else
      comp=$todo
      todo=
    fi
    case $comp in
      .) continue ;;
      ..) cur=${cur%/*}; continue ;;
    esac
    if [ -L "$cur/$comp" ]; then
      n=$((n + 1))
      [ "$n" -le 64 ] || block "too many symbolic links in $1 (fail closed)"
      t=$(readlink "$cur/$comp") || block "cannot read the symbolic link $cur/$comp (fail closed)"
      [[ $t == /* ]] && cur=
      todo=$t/$todo
      continue
    fi
    cur=$cur/$comp
  done
  RES=${cur:-/}
}

# toplevel DIR: TOP = the git toplevel of DIR; returns 1 when DIR is not in a
# git work tree. Any other git failure blocks.
toplevel() {
  local out rc
  command -v git >/dev/null 2>&1 || block "git is not installed, so $1 cannot be checked (fail closed)"
  out=$(git -C "$1" rev-parse --show-toplevel 2>/dev/null)
  rc=$?
  if [ "$rc" -eq 0 ] && [ -n "$out" ]; then
    TOP=$out
    return 0
  fi
  [ "$rc" -eq 128 ] && return 1
  block "git rev-parse failed in $1 (exit $rc), so the path cannot be checked (fail closed)"
}

# judge PATH BASE: return 1 when PATH, resolved against BASE, is protected;
# REL is then the path relative to its toplevel.
judge() {
  local p=$1 d top hit=0
  if [[ $p != /* ]]; then
    [[ $2 == /* ]] || block "cannot resolve $p: the hook input has no working directory (fail closed)"
    p=$2/$p
  fi
  physical "$p"
  p=$RES
  d=$p
  while [ ! -d "$d" ]; do
    d=${d%/*}
    [ -n "$d" ] || d=/
  done
  toplevel "$d" || return 0
  physical "$TOP"
  top=$RES
  [ -e "$top/.claude/hooks/r11-guard.sh" ] || return 0 # not a corpus tree
  shopt -s nocasematch # case-insensitive file systems (macOS default)
  if [[ $p == "$top"/* ]]; then
    REL=${p:${#top}+1}
    case $REL in
      registry | registry/* | .githooks | .githooks/* | .claude | .claude/*) hit=1 ;;
    esac
  elif [[ $p != "$top" ]]; then # spelled unlike git's toplevel (e.g. Unicode form)
    shopt -u nocasematch
    block "cannot place $p inside its git toplevel $top, so it cannot be checked (fail closed)"
  fi
  shopt -u nocasematch
  [ "$hit" -eq 0 ]
}

# judge_sh PATH BASE WHAT: judge a path named in a shell command. BASE "?"
# means a cd we couldn't follow; relative paths are then not judged.
judge_sh() {
  [ -n "$1" ] || return 0
  if [[ $1 != /* ]] && [ "$2" = '?' ]; then return 0; fi
  judge "$1" "$2" || block "$3 $REL: $protected_msg"
}

# ---- shell lexer -----------------------------------------------------------
# Tokens: TT[i] type (W word, R redirect, O operator), TV[i] word value or
# operator or redirect op, TX[i] redirect target. Quotes are removed from word
# values; $(...), `...` and <(...) bodies are queued in NESTED for checking as
# commands of their own.

re_plain=$'^[^ \t\n;&|()<>\\\'"$`]+'
re_dqplain=$'^[^"\\\\$`]+'
re_fdredir='^[0-9]+[<>]'
re_redir='^([0-9]*|&)(>>|>\||>&|>|<<<|<<-|<<|<>|<&|<)'
re_fddup='^([0-9]+|-)$'
re_assign='^[A-Za-z_][A-Za-z0-9_]*='
re_rcverify='(^|[^[:alnum:]_.-])regcorpus[[:space:]]+verify([^[:alnum:]_-]|$)'

emit() { # TYPE VALUE [TARGET]
  local i=${#TT[@]}
  TT[i]=$1
  TV[i]=$2
  TX[i]=${3-}
}

nest() { # queue a command string found inside the current one
  NESTED[${#NESTED[@]}]=$1
  NESTED_CWD[${#NESTED_CWD[@]}]=$2
}

# subst: I is just past "$(" or "<(" or ">("; consume through the closing ")".
subst() {
  local start=$I keep=${#TT[@]} i
  lex 1
  nest "${S:start:I-start}" "$CWD"
  [ "$I" -lt "$N" ] && I=$((I + 1))
  i=${#TT[@]}
  while [ "$i" -gt "$keep" ]; do
    i=$((i - 1))
    unset "TT[i]" "TV[i]" "TX[i]"
  done
  WV="$WV\$()"
}

backtick() { # I is at "`"
  local body= c
  I=$((I + 1))
  while [ "$I" -lt "$N" ]; do
    c=${S:I:1}
    if [ "$c" = '\' ]; then
      body=$body${S:I+1:1}
      I=$((I + 2))
    elif [ "$c" = '`' ]; then
      I=$((I + 1))
      break
    else
      body=$body$c
      I=$((I + 1))
    fi
  done
  nest "$body" "$CWD"
  WV="$WV\$()"
}

dollar() { # I is at "$"; CTX u (unquoted) or d (in double quotes)
  local c=${S:I+1:1} depth=1
  case $c in
    '(') I=$((I + 2)); subst ;;
    '{')
      I=$((I + 2))
      while [ "$I" -lt "$N" ] && [ "$depth" -gt 0 ]; do
        case ${S:I:1} in
          '{') depth=$((depth + 1)) ;;
          '}') depth=$((depth - 1)) ;;
        esac
        I=$((I + 1))
      done
      WV="$WV\${}"
      ;;
    "'")
      if [ "$1" = d ]; then
        WV="$WV\$"
        I=$((I + 1))
        return
      fi
      I=$((I + 2)) # ANSI-C quoting: kept raw, escapes not decoded
      while [ "$I" -lt "$N" ]; do
        c=${S:I:1}
        if [ "$c" = '\' ]; then
          WV=$WV${S:I:2}
          I=$((I + 2))
        elif [ "$c" = "'" ]; then
          I=$((I + 1))
          break
        else
          WV=$WV$c
          I=$((I + 1))
        fi
      done
      ;;
    '"')
      if [ "$1" = d ]; then
        WV="$WV\$"
        I=$((I + 1))
      else
        I=$((I + 2))
        dquote
      fi
      ;;
    *) WV="$WV\$"; I=$((I + 1)) ;;
  esac
}

dquote() { # I is just past the opening quote
  local c
  while [ "$I" -lt "$N" ]; do
    if [[ ${S:I:512} =~ $re_dqplain ]]; then
      WV=$WV${BASH_REMATCH[0]}
      I=$((I + ${#BASH_REMATCH[0]}))
      continue
    fi
    c=${S:I:1}
    case $c in
      '"') I=$((I + 1)); return ;;
      '\')
        case ${S:I+1:1} in
          '$' | '`' | '"' | '\') WV=$WV${S:I+1:1}; I=$((I + 2)) ;;
          $'\n') I=$((I + 2)) ;;
          *) WV=$WV'\'; I=$((I + 1)) ;;
        esac
        ;;
      '$') dollar d ;;
      '`') backtick ;;
    esac
  done
}

# read_word: WV = the unquoted value of the word at I.
read_word() {
  local c part rest
  WV=
  while [ "$I" -lt "$N" ]; do
    if [[ ${S:I:512} =~ $re_plain ]]; then
      WV=$WV${BASH_REMATCH[0]}
      I=$((I + ${#BASH_REMATCH[0]}))
      continue
    fi
    c=${S:I:1}
    case $c in
      ' ' | $'\t' | $'\n' | ';' | '&' | '|' | '(' | ')' | '<' | '>') return ;;
      '\')
        [ "${S:I+1:1}" = $'\n' ] || WV=$WV${S:I+1:1}
        I=$((I + 2))
        ;;
      "'")
        rest=${S:I+1}
        part=${rest%%\'*}
        WV=$WV$part
        I=$((I + ${#part} + 2))
        ;;
      '"') I=$((I + 1)); dquote ;;
      '$') dollar u ;;
      '`') backtick ;;
    esac
  done
  [ "$I" -le "$N" ] || I=$N
}

skip_blanks() {
  while [ "$I" -lt "$N" ]; do
    case ${S:I:1} in
      ' ' | $'\t') I=$((I + 1)) ;;
      '\') [ "${S:I+1:1}" = $'\n' ] && I=$((I + 2)) || return 0 ;;
      *) return 0 ;;
    esac
  done
}

redirect() {
  local op
  [[ ${S:I:24} =~ $re_redir ]] || block "internal: no redirect at offset $I"
  op=${BASH_REMATCH[0]}
  I=$((I + ${#op}))
  skip_blanks
  read_word
  case $op in
    *'<<-') HD_DELIM[${#HD_DELIM[@]}]=$WV; HD_STRIP[${#HD_STRIP[@]}]=1 ;;
    *'<<') HD_DELIM[${#HD_DELIM[@]}]=$WV; HD_STRIP[${#HD_STRIP[@]}]=0 ;;
  esac
  emit R "$op" "$WV"
}

# heredocs: I is just past a newline; skip the bodies of pending here-docs.
heredocs() {
  local k=0 T pre rest line d
  while [ "$k" -lt "${#HD_DELIM[@]}" ]; do
    d=${HD_DELIM[k]}
    T=$'\n'${S:I}$'\n'
    if [ "${HD_STRIP[k]}" = 1 ]; then
      pre=${T%%$'\n'*($'\t')"$d"$'\n'*}
    else
      pre=${T%%$'\n'"$d"$'\n'*}
    fi
    if [ "${#pre}" -eq "${#T}" ]; then
      I=$N
    else
      rest=${T:${#pre}+1}
      line=${rest%%$'\n'*}
      I=$((I + ${#pre} + ${#line} + 1))
      [ "$I" -le "$N" ] || I=$N
    fi
    k=$((k + 1))
  done
  HD_DELIM=()
  HD_STRIP=()
}

# lex NESTED: tokenize S from I. At top level (0) run to the end; nested (1,
# inside $( ) or <( )) stop at the ")" that closes it.
lex() {
  local depth=0 c rest
  while [ "$I" -lt "$N" ]; do
    c=${S:I:1}
    case $c in
      ' ' | $'\t') I=$((I + 1)) ;;
      $'\n') I=$((I + 1)); emit O nl; heredocs ;;
      '#')
        rest=${S:I}
        rest=${rest%%$'\n'*}
        I=$((I + ${#rest}))
        ;;
      ';') [ "${S:I+1:1}" = ';' ] && I=$((I + 2)) || I=$((I + 1)); emit O ';' ;;
      '&')
        case ${S:I+1:1} in
          '&') I=$((I + 2)); emit O '&&' ;;
          '>') redirect ;;
          *) I=$((I + 1)); emit O '&' ;;
        esac
        ;;
      '|')
        case ${S:I+1:1} in
          '|') I=$((I + 2)); emit O '||' ;;
          '&') I=$((I + 2)); emit O '|' ;;
          *) I=$((I + 1)); emit O '|' ;;
        esac
        ;;
      '(') I=$((I + 1)); depth=$((depth + 1)); emit O '(' ;;
      ')')
        [ "$1" = 1 ] && [ "$depth" -eq 0 ] && return 0
        I=$((I + 1))
        depth=$((depth - 1))
        emit O ')'
        ;;
      '<' | '>')
        if [ "${S:I+1:1}" = '(' ]; then
          WV=
          I=$((I + 2))
          subst
          emit W "$WV"
        else
          redirect
        fi
        ;;
      *)
        if [[ ${S:I:24} =~ $re_fdredir ]]; then
          redirect
        else
          read_word
          emit W "$WV"
        fi
        ;;
    esac
  done
}

# ---- checks on one simple command ------------------------------------------
# SW: the words; SR_OP / SR_T: its redirects; CWD: the directory it runs in.

risky_config() { # a git config key or key=value that can move or replace hooks
  local hit=1
  shopt -s nocasematch
  [[ $1 == *hookspath* || $1 == *include.path* || $1 == *includeif.* ]] && hit=0
  shopt -u nocasematch
  return $hit
}

check_assignment() { # NAME=VALUE
  [[ ${1%%=*} == GIT_CONFIG* ]] && risky_config "${1#*=}" &&
    block "${1%%=*} sets core.hooksPath or include.path; hooks are configured only by make setup (R11)"
  return 0
}

# rc_verify_at K: SW[K] is a command word; succeed when it runs regcorpus verify.
rc_verify_at() {
  local k=$1 n=${#SW[@]} w base
  w=${SW[k]}
  base=${w##*/}
  if [ "$w" = go ] && [ "${SW[k+1]-}" = run ]; then
    k=$((k + 2))
    while [ "$k" -lt "$n" ]; do
      case ${SW[k]} in
        -C | -o | -p | -tags | -ldflags | -gcflags | -asmflags | -mod | -modfile | -overlay | -pkgdir | -exec | -toolexec | -pgo) k=$((k + 2)) ;;
        -*) k=$((k + 1)) ;;
        *) break ;;
      esac
    done
    [ "$k" -lt "$n" ] || return 1
    w=${SW[k]%/}
    w=${w%@*}
    base=${w##*/}
  fi
  [ "$base" = regcorpus ] || return 1
  k=$((k + 1))
  while [ "$k" -lt "$n" ] && [[ ${SW[k]} == -* ]]; do k=$((k + 1)); done
  [ "$k" -lt "$n" ] && [ "${SW[k]}" = verify ]
}

no_verify_word() { # an abbreviation of --no-verify from --no-v up
  [ "${#1}" -ge 6 ] && [[ --no-verify == "$1"* ]]
}

# check_commit_opts ARGS: -n in commit's own options, or --no-verify.
check_commit_opts() {
  local a skip=0 rest ch name
  for a in "$@"; do
    if [ "$skip" = 1 ]; then
      skip=0
      continue
    fi
    case $a in
      --) return 0 ;;
      --*=*) ;;
      --*)
        no_verify_word "$a" && block "git commit $a skips the pre-commit hook (R11)"
        name=${a#--}
        case " message file author date template reuse-message reedit-message fixup squash cleanup trailer pathspec-from-file " in
          *" $name"*) skip=1 ;;
        esac
        ;;
      -?*)
        rest=${a#-}
        while [ -n "$rest" ]; do
          ch=${rest:0:1}
          rest=${rest:1}
          case $ch in
            n) block "git commit -n skips the pre-commit hook (R11)" ;;
            m | F | C | c | t) [ -z "$rest" ] && skip=1; break ;;
            S | u) break ;;
          esac
        done
        ;;
    esac
  done
}

check_config_args() { # git config ARGS: block a write that mentions a hooks key
  local a risky=0 read=0 write=0 pos=0 skip=0 first=
  for a in "$@"; do
    if [ "$skip" = 1 ]; then
      skip=0
      continue
    fi
    risky_config "$a" && risky=1
    case $a in
      --get | --get-all | --get-regexp | --get-urlmatch | --get-color | --get-colorbool | -l | --list) read=1 ;;
      --unset | --unset-all | --add | --replace-all | --rename-section | --remove-section | -e | --edit) write=1 ;;
      -f | --file | --blob | --type | --default | --comment | --value) skip=1 ;;
      -*) ;;
      *)
        pos=$((pos + 1))
        [ "$pos" = 1 ] && first=$a
        ;;
    esac
  done
  [ "$risky" = 1 ] || return 0
  case $first in
    get | list) read=1 ;;
    set | unset | rename-section | remove-section | edit) write=1 ;;
  esac
  if [ "$write" = 0 ] && { [ "$read" = 1 ] || [ "$pos" -le 1 ]; }; then return 0; fi
  block "git config changes core.hooksPath or include.path; hooks are configured only by make setup (R11)"
}

# judge_positional BASE WHAT ARGS: judge every non-option argument.
judge_positional() {
  local base=$1 what=$2 a opts=1
  shift 2
  for a in "$@"; do
    if [ "$opts" = 1 ]; then
      case $a in
        --) opts=0; continue ;;
        -?*) continue ;;
      esac
    fi
    judge_sh "$a" "$base" "$what"
  done
}

check_git() { # SW[K] is git
  local k=$((${1} + 1)) n=${#SW[@]} base=$CWD sub a after=0
  local -a args
  while [ "$k" -lt "$n" ]; do
    a=${SW[k]}
    case $a in
      -C)
        a=${SW[k+1]-}
        if [[ $a == /* ]]; then base=$a; elif [ "$base" != '?' ]; then base=$base/$a; fi
        k=$((k + 2))
        ;;
      -c | --config-env)
        risky_config "${SW[k+1]-}" && block "git $a ${SW[k+1]-} changes the hooks path (R11)"
        k=$((k + 2))
        ;;
      -c* | --config-env=*)
        risky_config "$a" && block "git $a changes the hooks path (R11)"
        k=$((k + 1))
        ;;
      --git-dir | --work-tree | --namespace | --super-prefix | --attr-source) k=$((k + 2)) ;;
      -*) k=$((k + 1)) ;;
      *) break ;;
    esac
  done
  [ "$k" -lt "$n" ] || return 0
  sub=${SW[k]}
  args=()
  k=$((k + 1))
  while [ "$k" -lt "$n" ]; do
    args[${#args[@]}]=${SW[k]}
    k=$((k + 1))
  done
  [ "${#args[@]}" -gt 0 ] || return 0
  case $sub in
    commit) check_commit_opts "${args[@]}" ;;
    merge | pull | push | rebase | am)
      for a in "${args[@]}"; do
        [ "$a" = -- ] && break
        no_verify_word "$a" && block "git $sub $a skips the git hooks (R11)"
      done
      ;;
    config) check_config_args "${args[@]}" ;;
    rm | mv | clean) judge_positional "$base" "git $sub" "${args[@]}" ;;
    restore)
      k=0
      while [ "$k" -lt "${#args[@]}" ]; do
        case ${args[k]} in
          -s | --source | --pathspec-from-file) k=$((k + 2)); continue ;;
          --) after=1 ;;
          -?*) [ "$after" = 1 ] && judge_sh "${args[k]}" "$base" "git restore" ;;
          *) judge_sh "${args[k]}" "$base" "git restore" ;;
        esac
        k=$((k + 1))
      done
      ;;
    checkout)
      for a in "${args[@]}"; do
        if [ "$after" = 1 ]; then
          judge_sh "$a" "$base" "git checkout --"
        elif [ "$a" = -- ]; then
          after=1
        fi
      done
      ;;
  esac
}

# judge_dest BASE WHAT ARGS: copy-like commands write only their destination:
# the last argument, or the -t / --target-directory value.
judge_dest() {
  local base=$1 what=$2 a last= next=0 opts=1
  shift 2
  for a in "$@"; do
    if [ "$next" = 1 ]; then
      judge_sh "$a" "$base" "$what"
      next=0
      continue
    fi
    if [ "$opts" = 1 ]; then
      case $a in
        --) opts=0; continue ;;
        -t | --target-directory) next=1; continue ;;
        --target-directory=*) judge_sh "${a#*=}" "$base" "$what"; continue ;;
        -t?*) judge_sh "${a#-t}" "$base" "$what"; continue ;;
        -?*) continue ;;
      esac
    fi
    last=$a
  done
  judge_sh "$last" "$base" "$what"
}

# judge_inplace BASE WHAT INPLACE SCRIPT TAKES ARGS: sed and perl edit files
# only with an in-place option. Option letters: INPLACE turn it on, SCRIPT
# give the script (so the first operand is a file), TAKES take an argument
# (ending the cluster). Without a SCRIPT option the first operand is the script.
judge_inplace() {
  local base=$1 what=$2 inl=$3 scl=$4 takes=$5 a inplace=0 script=0 skip=0 rest ch
  local -a files
  shift 5
  files=()
  for a in "$@"; do
    if [ "$skip" = 1 ]; then
      skip=0
      continue
    fi
    case $a in
      --in-place*) inplace=1 ;;
      --expression | --file) skip=1; script=1 ;;
      --expression=* | --file=*) script=1 ;;
      --*) ;;
      -?*)
        rest=${a#-}
        while [ -n "$rest" ]; do
          ch=${rest:0:1}
          rest=${rest:1}
          case $inl in *"$ch"*) inplace=1; break ;; esac
          case $scl in
            *"$ch"*)
              script=1
              [ -z "$rest" ] && skip=1
              break
              ;;
          esac
          case $takes in *"$ch"*) break ;; esac
        done
        ;;
      *) files[${#files[@]}]=$a ;;
    esac
  done
  [ "$inplace" = 1 ] && [ "${#files[@]}" -gt 0 ] || return 0
  [ "$script" = 1 ] || files[0]=
  for a in "${files[@]}"; do judge_sh "$a" "$base" "$what"; done
}

analyze_simple() {
  local n=${#SW[@]} r=0 k=0 w base t j
  while [ "$r" -lt "${#SR_OP[@]}" ]; do
    case ${SR_OP[r]} in
      *'<<' | *'<<-' | *'<<<' | *'<' | *'<&') ;;
      *'>&') [[ ${SR_T[r]} =~ $re_fddup ]] || judge_sh "${SR_T[r]}" "$CWD" "redirect to" ;;
      *) judge_sh "${SR_T[r]}" "$CWD" "redirect to" ;;
    esac
    r=$((r + 1))
  done

  # Skip leading assignments, reserved words and wrappers.
  while [ "$k" -lt "$n" ]; do
    w=${SW[k]}
    if [[ $w =~ $re_assign ]]; then
      check_assignment "$w"
      k=$((k + 1))
      continue
    fi
    case $w in
      '!' | '{' | '}' | if | then | else | elif | do | while | until | nohup | builtin | exec | stdbuf) k=$((k + 1)) ;;
      time) k=$((k + 1)); [ "${SW[k]-}" = -p ] && k=$((k + 1)) ;;
      command)
        k=$((k + 1))
        case ${SW[k]-} in
          -v | -V) return 0 ;;
          -p) k=$((k + 1)) ;;
        esac
        ;;
      env)
        k=$((k + 1))
        while [ "$k" -lt "$n" ]; do
          case ${SW[k]} in
            -u | -C | -S | --unset | --chdir | --split-string) k=$((k + 2)) ;;
            -*) k=$((k + 1)) ;;
            *) break ;;
          esac
        done
        ;;
      nice)
        k=$((k + 1))
        case ${SW[k]-} in
          -n) k=$((k + 2)) ;;
          -*) k=$((k + 1)) ;;
        esac
        ;;
      timeout)
        k=$((k + 1))
        while [ "$k" -lt "$n" ]; do
          case ${SW[k]} in
            -s | -k | --signal | --kill-after) k=$((k + 2)) ;;
            -*) k=$((k + 1)) ;;
            *) break ;;
          esac
        done
        k=$((k + 1))
        ;;
      *) break ;;
    esac
  done
  [ "$k" -lt "$n" ] || return 0

  rc_verify_at "$k" && block "regcorpus verify is run by a person in a terminal, never by an agent (R2a, R11)"
  w=${SW[k]}
  base=${w##*/}
  t=$((k + 1))
  case $base in
    git) check_git "$k" ;;
    script | expect | unbuffer)
      for ((j = t; j < n; j++)); do
        if rc_verify_at "$j" || [[ ${SW[j]} =~ $re_rcverify ]]; then
          block "regcorpus verify is run by a person in a terminal, never by an agent (R2a, R11)"
        fi
      done
      ;;
    sh | bash | zsh | dash | ksh)
      for ((j = t; j < n; j++)); do
        case ${SW[j]} in
          --*) ;;
          -*c*) [ "$((j + 1))" -lt "$n" ] && nest "${SW[j+1]}" "$CWD"; break ;;
          -*) ;;
          *) break ;;
        esac
      done
      ;;
    cd)
      w=${SW[t]-}
      case $w in
        '' | -* | '~'* | *'$'*) CWD='?' ;;
        /*) CWD=$w ;;
        *) [ "$CWD" = '?' ] || CWD=$CWD/$w ;;
      esac
      ;;
    export | declare | typeset | readonly | local)
      for ((j = t; j < n; j++)); do check_assignment "${SW[j]}"; done
      ;;
    rm | rmdir | touch | mkdir | truncate | chmod | chown | chgrp | unlink | tee | mv)
      [ "$t" -lt "$n" ] && judge_positional "$CWD" "$base" "${SW[@]:t}"
      ;;
    cp | ln | install | rsync)
      [ "$t" -lt "$n" ] && judge_dest "$CWD" "$base" "${SW[@]:t}"
      ;;
    dd)
      for ((j = t; j < n; j++)); do
        case ${SW[j]} in of=*) judge_sh "${SW[j]#of=}" "$CWD" "dd of=" ;; esac
      done
      ;;
    sed) [ "$t" -lt "$n" ] && judge_inplace "$CWD" "sed -i" iI ef l "${SW[@]:t}" ;;
    perl) [ "$t" -lt "$n" ] && judge_inplace "$CWD" "perl -i" i eE MmIxld0DCV "${SW[@]:t}" ;;
  esac
  return 0
}

# check_command CMD CWD: lex CMD and check each simple command in it, then
# every command string nested inside it.
check_command() {
  local q=0 t nt
  NESTED=("$1")
  NESTED_CWD=("$2")
  while [ "$q" -lt "${#NESTED[@]}" ]; do
    [ "$q" -lt 256 ] || block "command too deeply nested to check (fail closed)"
    S=${NESTED[q]}
    N=${#S}
    I=0
    CWD=${NESTED_CWD[q]}
    TT=()
    TV=()
    TX=()
    HD_DELIM=()
    HD_STRIP=()
    lex 0
    CWD_STACK=()
    SW=()
    SR_OP=()
    SR_T=()
    nt=${#TT[@]}
    for ((t = 0; t < nt; t++)); do
      case ${TT[t]} in
        W) SW[${#SW[@]}]=${TV[t]} ;;
        R)
          SR_OP[${#SR_OP[@]}]=${TV[t]}
          SR_T[${#SR_T[@]}]=${TX[t]}
          ;;
        O)
          analyze_simple
          SW=()
          SR_OP=()
          SR_T=()
          case ${TV[t]} in
            '(') CWD_STACK[${#CWD_STACK[@]}]=$CWD ;;
            ')')
              if [ "${#CWD_STACK[@]}" -gt 0 ]; then
                CWD=${CWD_STACK[${#CWD_STACK[@]}-1]}
                unset "CWD_STACK[${#CWD_STACK[@]}-1]"
              fi
              ;;
          esac
          ;;
      esac
    done
    analyze_simple
    q=$((q + 1))
  done
}

# ---- main ------------------------------------------------------------------

command -v jq >/dev/null 2>&1 || block "jq is not installed, so the tool call cannot be checked (fail closed)"
input=
IFS= read -r -d '' input || true
fields=$(jq -r '@sh "tool=\(.tool_name // "" | tostring) cwd=\(.cwd // "" | tostring) cmd=\(.tool_input.command // "" | tostring)"' <<<"$input" 2>/dev/null) ||
  block "unreadable hook input, so the tool call cannot be checked (fail closed)"
[ -n "$fields" ] || block "empty hook input, so the tool call cannot be checked (fail closed)"
eval "$fields"

# tool_paths: set the positional parameters to every path-like input string.
tool_paths() {
  local list
  list=$(jq -r '[.tool_input | .. | objects | to_entries[]
      | select(.key | test("^(file_path|notebook_path|path|paths|source|destination|target|old_path|new_path)$"))
      | .value | if type == "array" then .[] else . end | select(type == "string")]
      | @sh "set -- \(.)"' <<<"$input" 2>/dev/null) ||
    block "unreadable tool input, so the tool call cannot be checked (fail closed)"
  PATHS_SH=$list
}

check_tool_path() {
  case $1 in
    *[[:cntrl:]]*) block "$tool path contains a control character (fail closed)" ;;
  esac
  judge "$1" "$cwd" || block "$tool on $REL: $protected_msg"
}

case $tool in
  Bash) check_command "$cmd" "$cwd" ;;
  Edit | Write | MultiEdit | NotebookEdit | mcp__*)
    tool_paths
    eval "$PATHS_SH"
    if [[ $tool != mcp__* ]]; then
      [ $# -gt 0 ] || block "$tool call has no file path to check (fail closed)"
    fi
    for p in "$@"; do
      if [ -z "$p" ]; then
        [[ $tool == mcp__* ]] && continue
        block "$tool call has an empty file path (fail closed)"
      fi
      check_tool_path "$p"
    done
    ;;
esac
trap - EXIT
exit 0
