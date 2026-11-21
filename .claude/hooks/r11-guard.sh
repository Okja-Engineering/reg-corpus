#!/usr/bin/env bash
# R11 agent guard: a Claude Code PreToolUse hook (spec R11).
#
# Threat model. This guards against a well-meaning agent's shortcuts; it is
# not a security boundary. The gate is C6 (the pre-commit hook now, CI later),
# and `regcorpus verify` separately requires a real terminal.
# It stops: edit/write tools and common shell writers and redirects into
# registry/, .githooks/ or .claude/ of this checkout or its worktrees;
# skipping the commit hook (--no-verify and its abbreviations, commit -n,
# hooksPath set via git config, -c or GIT_CONFIG_*); and running
# `regcorpus verify`.
# Design priority: never block normal work, even when that leaves a hole.
# Accepted residual, left to C6 and CI (spec decision 3):
#   - obfuscation: eval, encoded strings, interpreters writing files (python,
#     node); $(...) inside unquoted here-doc bodies; globs and links; git
#     plumbing (commit-tree, update-ref, update-index) and aliases; pathspec
#     magic; paths read from stdin (xargs without -I); operations on an
#     ancestor of a protected dir; writers it doesn't know (find -delete,
#     git apply);
#   - targets it can't resolve (by design, below): variables other than $HOME
#     and $TMPDIR, relative paths after a cd it can't follow, relative paths
#     given to MCP tools;
#   - the local toolchain and refs: GOROOT, $GOROOT/go.env, PATH shadowing,
#     moving the local main ref;
#   - commits that skip pre-commit: merge, rebase, cherry-pick, am;
#   - a hook that times out does not block on this Claude Code version
#     (2.1.221); if this file goes missing while settings.json still runs it,
#     the wrapper blocks every tool call (suspected; the two change together).
#
# .claude/settings.json runs it for Bash, Edit, Write, MultiEdit, NotebookEdit
# and MCP tools (the matcher is an unanchored regex, so tools like TodoWrite
# arrive too and pass), through a wrapper that blocks when this file is
# missing or not executable. Exit 2 blocks, with "R11 guard: blocked —
# <reason>" on stderr; exit 0 allows, silently.
#
# How a target is judged, three invariants:
#   1. Only a concrete path is judged. ~, $HOME and $TMPDIR are expanded; a
#      word holding any other expansion, an arithmetic or [[ ]] operand and an
#      option's value are not write targets and are never judged.
#   2. Git places the path. The deepest existing directory of the physical
#      path (symlinks followed) is handed to git rev-parse, which answers in
#      its own spelling (case, Unicode form, firmlinks); cached per directory.
#      The path is protected when its repo shares this guard's git common dir
#      (this checkout or one of its worktrees, never a clone) and it lies
#      under registry/, .githooks/ or .claude/ of its work tree, except
#      .claude/worktrees/ housekeeping. $CLAUDE_PROJECT_DIR is never used.
#   3. The working directory is followed through cd, pushd/popd, env -C,
#      subshells, and $(...) and `...` bodies (which run where the shell is
#      when it reaches them). When it can't be followed it is unknown, and
#      relative paths are then not judged.
#
# Blocks:
#   - File tools (Edit, Write, MultiEdit, NotebookEdit): a protected path, or
#     one holding a control character. MCP tools whose name says they write
#     (write, edit, move, create, delete...): any absolute or ~ path value
#     under a path-like key (file_path, path, paths, source, destination...)
#     that is protected.
#   - Bash, one simple command at a time (split on && || ; | & newlines and
#     parentheses; quotes and here-docs respected; $(...), `...`, <(...) and
#     sh -c strings checked as commands too; wrappers env, command, builtin,
#     exec, nohup, nice, time, timeout, stdbuf and xargs, find -exec and
#     parallel see through to the command they run):
#     . hook bypass: --no-verify on commit/merge/pull/push/rebase/am (or any
#       abbreviation from --no-v), -n in commit's own options, core.hooksPath
#       or include.path set through git config, git -c or GIT_CONFIG_*;
#     . regcorpus verify: run directly, by path, with go run, or under
#       script/expect/unbuffer;
#     . writes: a redirect, or a known writer (cp mv rm rmdir touch mkdir ln
#       install truncate chmod chown chgrp tee dd unlink rsync, sed -i,
#       perl -i, their g-prefixed GNU names, git rm/mv/clean, git restore and
#       git checkout of paths) whose target is protected. git rm --cached and
#       git restore --staged only touch the index and pass.
# Fails closed: unreadable input, jq or git missing, any internal error, a
# terminating signal and a command nested too deep to check all exit 2.
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

protected_msg='registry/, .githooks/, .claude/ and the git config are not writable by agents (R11)'
verify_msg='regcorpus verify is run by a person in a terminal, never by an agent (R2a, R11)'
DYN=$'\x01' # marks where a word stops being concrete (an expansion starts)
HOME_V=${HOME-}
TMPDIR_V=${TMPDIR-}
MAX_DEPTH=16 # nested command strings ($(...), sh -c) checked, at most

# ---- paths -----------------------------------------------------------------

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

# lexical PATH: RES = absolute PATH with ., .. and repeated / removed by
# spelling alone (how cd moves without -P).
lexical() {
  local todo=$1/ cur= comp
  while [ -n "$todo" ]; do
    comp=${todo%%/*}
    todo=${todo#*/}
    case $comp in
      '' | .) ;;
      ..) cur=${cur%/*} ;;
      *) cur=$cur/$comp ;;
    esac
  done
  RES=${cur:-/}
}

# join_dir BASE DIR: RES = DIR taken relative to BASE; "?" when DIR is not
# concrete, or is relative and BASE is unknown ("?").
join_dir() {
  case $2 in
    '' | *"$DYN"*) RES='?' ;;
    /*) RES=$2 ;;
    *) if [ "$1" = '?' ]; then RES='?'; else RES=$1/$2; fi ;;
  esac
}

# place DIR: ask git where the existing directory DIR is. Returns 1 when DIR
# is not inside a git work tree; otherwise PFX = DIR relative to its work
# tree's top (ending in "/", or empty at the top) and CDIR = the repo's
# common git dir, both in git's own spelling. Cached per directory.
PL_D=()
PL_R=()
PL_P=()
PL_C=()
place() {
  local i=0 out rc r=1 l1 rest
  while [ "$i" -lt "${#PL_D[@]}" ]; do
    if [ "${PL_D[i]}" = "$1" ]; then
      PFX=${PL_P[i]}
      CDIR=${PL_C[i]}
      return "${PL_R[i]}"
    fi
    i=$((i + 1))
  done
  command -v git >/dev/null 2>&1 || block "git is not installed, so $1 cannot be checked (fail closed)"
  out=$(git -C "$1" rev-parse --path-format=absolute --is-inside-work-tree --git-common-dir --show-prefix 2>/dev/null)
  rc=$?
  PFX=
  CDIR=
  if [ "$rc" -eq 0 ]; then
    l1=${out%%$'\n'*}
    rest=${out#*$'\n'}
    [ "$rest" != "$out" ] || rest=
    CDIR=${rest%%$'\n'*}
    case $rest in *$'\n'*) PFX=${rest#*$'\n'} ;; esac
    case $l1 in
      true) [ -n "$CDIR" ] && r=0 ;;
      false) ;;
      *) block "git rev-parse gave no answer in $1, so the path cannot be checked (fail closed)" ;;
    esac
  elif [ "$rc" -ne 128 ]; then
    block "git rev-parse failed in $1 (exit $rc), so the path cannot be checked (fail closed)"
  fi
  PL_D[i]=$1
  PL_R[i]=$r
  PL_P[i]=$PFX
  PL_C[i]=$CDIR
  return "$r"
}

# guard_cdir: GUARD_CDIR = the common git dir of the checkout this guard
# file lives in.
GUARD_CDIR=
guard_cdir() {
  local here
  [ -z "$GUARD_CDIR" ] || return 0
  here=${BASH_SOURCE[0]%/*}
  [ "$here" != "${BASH_SOURCE[0]}" ] || here=.
  GUARD_CDIR=$(git -C "$here" rev-parse --path-format=absolute --git-common-dir 2>/dev/null)
  [ -n "$GUARD_CDIR" ] ||
    block "cannot find the git repository this guard belongs to, so the path cannot be checked (fail closed)"
}

# corpus CDIR: succeed when CDIR is GUARD_CDIR: this checkout or one of its
# worktrees, never a clone.
corpus() {
  guard_cdir
  [ "$1" = "$GUARD_CDIR" ]
}

# protected REL: succeed when REL (a path inside a work tree) is under
# registry/, .githooks/ or .claude/. .claude/worktrees/ holds other work trees
# and their housekeeping (prune, stale dirs): a path inside one of them is
# judged as inside that tree.
protected() {
  local r=$1 hit=1
  shopt -s nocasematch # case-insensitive file systems (macOS default)
  while [[ $r == .claude/worktrees/*/* ]]; do
    r=${r#*/}
    r=${r#*/}
    r=${r#*/}
  done
  case $r in
    .claude/worktrees | .claude/worktrees/*) ;;
    registry | registry/* | .githooks | .githooks/* | .claude | .claude/*) hit=0 ;;
  esac
  shopt -u nocasematch
  return "$hit"
}

# judge PATH BASE: return 1 when PATH, resolved against BASE, is protected;
# REL is then the path inside its work tree, or the git config's path.
judge() {
  local p=$1 d rest
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
  rest=${p#"$d"}
  rest=${rest#/}
  case $rest in
    [Cc][Oo][Nn][Ff][Ii][Gg]) # the repo's git config, which holds core.hooksPath
      guard_cdir
      if [ "$d" -ef "$GUARD_CDIR" ]; then
        REL=$GUARD_CDIR/config
        return 1
      fi
      ;;
  esac
  place "$d" || return 0
  REL=$PFX$rest
  REL=${REL%/}
  protected "$REL" || return 0
  corpus "$CDIR" || return 0
  return 1
}

# judge_sh PATH BASE WHAT: judge a path named in a shell command. A word that
# is not concrete is judged by the directory its concrete leading part names
# (registry/ in registry/$id.yaml), and not at all when that part holds no
# "/". A relative path when BASE is unknown ("?") is not judged.
judge_sh() {
  local p=$1
  case $p in
    *"$DYN"*)
      p=${p%%"$DYN"*}
      [[ $p == */* ]] || return 0
      p=${p%/*}/
      ;;
  esac
  [ -n "$p" ] || return 0
  if [[ $p != /* ]] && [ "$2" = '?' ]; then return 0; fi
  judge "$p" "$2" || block "$3 $REL: $protected_msg"
}

# dyn WORD MARK: RES = WORD, not concrete from its first MARK ({} filled in by
# xargs, find or parallel) on.
dyn() {
  local p=${1%%"$2"*}
  case $p in
    *"$DYN"*) RES=$1 ;;
    *) RES=$p$DYN${1#"$p"} ;;
  esac
}

# ---- shell lexer -------------------------------------------------------------
# Tokens: TT[i] type, TV[i] value, TX[i] redirect target. Types: W word,
# R redirect (TV the operator, TX the target word), O operator, S the body of
# a $(...), `...` or <(...) found in the next word (checked as a command of
# its own, in the directory the shell is in when it reaches it). Quotes are
# removed from words; a word that holds an expansion other than ~, $HOME and
# $TMPDIR keeps its source text, with DYN inserted where the first starts.

re_plain=$'^[^ \t\n;&|()<>\\\'"$`~]+'
re_dqplain=$'^[^"\\\\$`]+'
re_fdredir='^[0-9]+[<>]'
re_redir='^([0-9]*|&)(>>|>\||>&|>|<<<|<<-|<<|<>|<&|<)'
re_fddup='^([0-9]+|-)$'
re_assign='^[A-Za-z_][A-Za-z0-9_]*='
re_assign_head='^[A-Za-z_][A-Za-z0-9_]*=$'
re_name='^[A-Za-z_][A-Za-z0-9_]*'
re_tilde='^~[A-Za-z0-9._+-]*'
re_rcverify='(^|[^[:alnum:]_.-])regcorpus[[:space:]]+verify([^[:alnum:]_-]|$)'

emit() { # TYPE VALUE [TARGET]
  local i=${#TT[@]}
  TT[i]=$1
  TV[i]=$2
  TX[i]=${3-}
}

# word_value: the current word (WV, WD, WP) as a token value.
word_value() {
  if [ "$WD" = 1 ]; then WORD=${WV:0:WP}$DYN${WV:WP}; else WORD=$WV; fi
}

# mark_dyn: an expansion that is not followed starts at the end of WV.
mark_dyn() {
  if [ "$WD" = 0 ]; then
    WD=1
    WP=${#WV}
  fi
}

nest() { # BODY CWD DEPTH: queue a command string (its source text) to check
  NESTED[${#NESTED[@]}]=${1//$DYN/}
  NESTED_CWD[${#NESTED_CWD[@]}]=$2
  NESTED_DEPTH[${#NESTED_DEPTH[@]}]=$3
}

# subst: I is just past "$(", "<(" or ">("; consume through the closing ")"
# and emit the body as an S token.
subst() {
  local start=$I keep=${#TT[@]} i body wv=$WV wd=$WD wp=$WP
  lex 1
  WV=$wv
  WD=$wd
  WP=$wp
  body=${S:start:I-start}
  [ "$I" -lt "$N" ] && I=$((I + 1))
  i=${#TT[@]}
  while [ "$i" -gt "$keep" ]; do
    i=$((i - 1))
    unset "TT[i]" "TV[i]" "TX[i]"
  done
  emit S "$body"
}

# skip_balanced OPEN CLOSE DEPTH: consume through the CLOSE that brings DEPTH
# (already open) to zero, as in $(( )), (( )) and $[ ].
skip_balanced() {
  local depth=$3 c
  while [ "$I" -lt "$N" ] && [ "$depth" -gt 0 ]; do
    c=${S:I:1}
    if [ "$c" = "$1" ]; then
      depth=$((depth + 1))
    elif [ "$c" = "$2" ]; then
      depth=$((depth - 1))
    fi
    I=$((I + 1))
  done
}

backtick() { # I is at "`"
  local start=$I body= c
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
  [ "$I" -le "$N" ] || I=$N
  emit S "$body"
  mark_dyn
  WV=$WV${S:start:I-start}
}

# expand_var NAME: append $HOME or $TMPDIR to WV when set; 1 otherwise.
expand_var() {
  case $1 in
    HOME) [ -n "$HOME_V" ] || return 1; WV=$WV$HOME_V ;;
    TMPDIR) [ -n "$TMPDIR_V" ] || return 1; WV=$WV$TMPDIR_V ;;
    *) return 1 ;;
  esac
}

# tilde: I is at an unquoted "~" that starts a word or follows NAME=.
tilde() {
  local pre c
  [[ ${S:I:256} =~ $re_tilde ]] || return 0
  pre=${BASH_REMATCH[0]}
  c=${S:I+${#pre}:1}
  case $c in
    '' | / | ' ' | $'\t' | $'\n' | ';' | '&' | '|' | '(' | ')' | '<' | '>') ;;
    *) return 0 ;; # a quoted tilde prefix is not expanded
  esac
  I=$((I + ${#pre}))
  if [ "$pre" = '~' ] && [ -n "$HOME_V" ]; then
    WV=$WV$HOME_V
  else
    mark_dyn # ~user, ~+, ~-: not followed
    WV=$WV$pre
  fi
}

dollar() { # I is at "$"; $1: u (unquoted) or d (in double quotes)
  local c=${S:I+1:1} start=$I name depth
  case $c in
    '(')
      if [ "${S:I+2:1}" = '(' ]; then
        I=$((I + 3))
        skip_balanced '(' ')' 2
      else
        I=$((I + 2))
        subst
      fi
      mark_dyn
      WV=$WV${S:start:I-start}
      ;;
    '[')
      I=$((I + 2))
      skip_balanced '[' ']' 1
      mark_dyn
      WV=$WV${S:start:I-start}
      ;;
    '{')
      I=$((I + 2))
      depth=1
      while [ "$I" -lt "$N" ] && [ "$depth" -gt 0 ]; do
        case ${S:I:1} in
          '{') depth=$((depth + 1)) ;;
          '}') depth=$((depth - 1)) ;;
        esac
        I=$((I + 1))
      done
      name=${S:start+2:I-start-3}
      if ! expand_var "$name"; then
        mark_dyn
        WV=$WV${S:start:I-start}
      fi
      ;;
    "'")
      if [ "$1" = d ]; then
        WV="$WV\$"
        I=$((I + 1))
        return
      fi
      I=$((I + 2)) # ANSI-C quoting: kept raw; with an escape it is not concrete
      while [ "$I" -lt "$N" ]; do
        c=${S:I:1}
        if [ "$c" = '\' ]; then
          mark_dyn
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
    [A-Za-z_])
      [[ ${S:I+1:256} =~ $re_name ]]
      name=${BASH_REMATCH[0]}
      I=$((I + 1 + ${#name}))
      if ! expand_var "$name"; then
        mark_dyn
        WV="$WV\$$name"
      fi
      ;;
    [0-9] | '@' | '*' | '#' | '?' | '$' | '!' | '-')
      mark_dyn
      WV=$WV${S:I:2}
      I=$((I + 2))
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

# read_word: WV = the unquoted value of the word at I; WD = 1 when it holds
# an expansion that is not followed, which starts at WP.
WV=
WD=0
WP=0
read_word() {
  local c part rest
  WV=
  WD=0
  while [ "$I" -lt "$N" ]; do
    if [[ ${S:I:512} =~ $re_plain ]]; then
      WV=$WV${BASH_REMATCH[0]}
      I=$((I + ${#BASH_REMATCH[0]}))
      continue
    fi
    c=${S:I:1}
    case $c in
      ' ' | $'\t' | $'\n' | ';' | '&' | '|' | '(' | ')' | '<' | '>') return ;;
      '~') # expanded at the start of a word and after NAME= (of=~/x too)
        if [ -z "$WV" ] || [[ $WV =~ $re_assign_head ]]; then tilde; fi
        if [ "${S:I:1}" = '~' ]; then
          WV=$WV'~'
          I=$((I + 1))
        fi
        ;;
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
    *'<<<') ;; # a here-string, not a here-doc
    *'<<-') HD_DELIM[${#HD_DELIM[@]}]=$WV; HD_STRIP[${#HD_STRIP[@]}]=1 ;;
    *'<<') HD_DELIM[${#HD_DELIM[@]}]=$WV; HD_STRIP[${#HD_STRIP[@]}]=0 ;;
  esac
  word_value
  emit R "$op" "$WORD"
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
# inside $( ) or <( )) stop at the ")" that closes it. Inside [[ ]], < > ( )
# are operands, not redirects or subshells; (( )) is arithmetic, skipped.
lex() {
  local depth=0 c rest cs=1 dbl=0 start
  while [ "$I" -lt "$N" ]; do
    c=${S:I:1}
    if [ "$dbl" = 1 ]; then
      case $c in
        '<' | '>' | '(' | ')') I=$((I + 1)); emit W "$c"; continue ;;
      esac
    fi
    case $c in
      ' ' | $'\t') I=$((I + 1)) ;;
      $'\n') I=$((I + 1)); emit O nl; cs=1; heredocs ;;
      '#')
        rest=${S:I}
        rest=${rest%%$'\n'*}
        I=$((I + ${#rest}))
        ;;
      ';') [ "${S:I+1:1}" = ';' ] && I=$((I + 2)) || I=$((I + 1)); emit O ';'; cs=1 ;;
      '&')
        case ${S:I+1:1} in
          '&') I=$((I + 2)); emit O '&&'; cs=1 ;;
          '>') redirect ;;
          *) I=$((I + 1)); emit O '&'; cs=1 ;;
        esac
        ;;
      '|')
        case ${S:I+1:1} in
          '|') I=$((I + 2)); emit O '||' ;;
          '&') I=$((I + 2)); emit O '|' ;;
          *) I=$((I + 1)); emit O '|' ;;
        esac
        cs=1
        ;;
      '(')
        if [ "${S:I+1:1}" = '(' ]; then
          start=$I
          I=$((I + 2))
          skip_balanced '(' ')' 2
          emit W "$DYN${S:start:I-start}"
          cs=0
        else
          I=$((I + 1))
          depth=$((depth + 1))
          emit O '('
          cs=1
        fi
        ;;
      ')')
        [ "$1" = 1 ] && [ "$depth" -eq 0 ] && return 0
        I=$((I + 1))
        depth=$((depth - 1))
        emit O ')'
        cs=1
        ;;
      '<' | '>')
        if [ "${S:I+1:1}" = '(' ]; then
          start=$I
          WV=
          I=$((I + 2))
          subst
          emit W "$DYN${S:start:I-start}"
          cs=0
        else
          redirect
        fi
        ;;
      *)
        if [[ ${S:I:24} =~ $re_fdredir ]]; then
          redirect
        else
          read_word
          word_value
          emit W "$WORD"
          if [ "$dbl" = 1 ]; then
            [ "$WORD" = ']]' ] && dbl=0
          elif [ "$cs" = 1 ] && [ "$WORD" = '[[' ]; then
            dbl=1
          fi
          case $WORD in
            if | then | else | elif | do | while | until | for | time | '!' | '{') cs=1 ;;
            *) cs=0 ;;
          esac
        fi
        ;;
    esac
  done
}

# ---- options -----------------------------------------------------------------

# parse_opts SHORT LONG ARG...: split ARG GNU style into options and operands
# (options may follow operands; "--" ends them). SHORT holds the letters of
# options that take a value, LONG the long options that do (space-separated;
# an unambiguous abbreviation counts). Sets OPS (operands), OPS_DD (index in
# OPS of the first operand after "--", or -1), OPT_K/OPT_V (options given a
# value) and OPT_F (flags seen: letters, and long ones as --NAME).
parse_opts() {
  local short=$1 long=$2 a rest ch name dd=0
  shift 2
  OPS=()
  OPS_DD=-1
  OPT_K=()
  OPT_V=()
  OPT_F=()
  while [ $# -gt 0 ]; do
    a=$1
    shift
    if [ "$dd" = 1 ]; then
      OPS[${#OPS[@]}]=$a
      continue
    fi
    case $a in
      --) dd=1; OPS_DD=${#OPS[@]} ;;
      --?*)
        name=${a#--}
        if [[ $name == *=* ]]; then
          long_opt "${name%%=*}" "$long"
          OPT_K[${#OPT_K[@]}]=$LONG
          OPT_V[${#OPT_V[@]}]=${name#*=}
        elif long_opt "$name" "$long"; then
          OPT_K[${#OPT_K[@]}]=$LONG
          OPT_V[${#OPT_V[@]}]=${1-}
          [ $# -eq 0 ] || shift
        else
          OPT_F[${#OPT_F[@]}]=$a
        fi
        ;;
      -?*)
        rest=${a#-}
        while [ -n "$rest" ]; do
          ch=${rest:0:1}
          rest=${rest:1}
          case $short in
            *"$ch"*)
              OPT_K[${#OPT_K[@]}]=$ch
              if [ -n "$rest" ]; then
                OPT_V[${#OPT_V[@]}]=$rest
              else
                OPT_V[${#OPT_V[@]}]=${1-}
                [ $# -eq 0 ] || shift
              fi
              break
              ;;
          esac
          OPT_F[${#OPT_F[@]}]=$ch
        done
        ;;
      *) OPS[${#OPS[@]}]=$a ;;
    esac
  done
}

# long_opt NAME LIST: LONG = the option in LIST that NAME names or uniquely
# abbreviates; returns 1 (LONG = NAME) when there is none.
long_opt() {
  local o hit= n=0
  LONG=$1
  for o in $2; do
    [ "$o" = "$1" ] && return 0
    case $o in "$1"*) hit=$o; n=$((n + 1)) ;; esac
  done
  [ "$n" = 1 ] || return 1
  LONG=$hit
}

optval() { # NAME...: OPTVAL = the value of the last of these options given
  local i=${#OPT_K[@]} n
  while [ "$i" -gt 0 ]; do
    i=$((i - 1))
    for n in "$@"; do
      if [ "${OPT_K[i]}" = "$n" ]; then
        OPTVAL=${OPT_V[i]}
        return 0
      fi
    done
  done
  return 1
}

# hasflag NAME...: succeed when one of these flags was given. A long NAME
# (more than one letter) counts in any abbreviation (--work for --worktree),
# as git and the GNU tools accept.
hasflag() {
  local n i=0
  while [ "$i" -lt "${#OPT_F[@]}" ]; do
    for n in "$@"; do
      if [ "${#n}" = 1 ]; then
        [ "${OPT_F[i]}" = "$n" ] && return 0
      else
        case ${OPT_F[i]} in --?*) [[ $n == "${OPT_F[i]#--}"* ]] && return 0 ;; esac
      fi
    done
    i=$((i + 1))
  done
  return 1
}

judge_ops() { # CW WHAT FIRST: judge OPS from index FIRST on
  local i=$3
  while [ "$i" -lt "${#OPS[@]}" ]; do
    judge_sh "${OPS[i]}" "$1" "$2"
    i=$((i + 1))
  done
}

# ---- checks on one command -----------------------------------------------------

risky_config() { # a git config key, key=value or section that can move or replace hooks
  local hit=1
  shopt -s nocasematch
  [[ $1 == *hookspath* || $1 == *include.path* || $1 == *includeif.* || $1 == core ]] && hit=0
  shopt -u nocasematch
  return $hit
}

check_assignment() { # NAME=VALUE
  [[ ${1%%=*} == GIT_CONFIG* ]] && risky_config "${1#*=}" &&
    block "${1%%=*} sets core.hooksPath or include.path; hooks are configured only by make setup (R11)"
  return 0
}

# rc_verify WORD...: succeed when the command WORD... runs regcorpus verify.
rc_verify() {
  local w=$1 base
  shift
  base=${w##*/}
  if [ "$base" = go ] && [ "${1-}" = run ]; then
    shift
    while [ $# -gt 0 ]; do
      case $1 in
        -C | -o | -p | -tags | -ldflags | -gcflags | -asmflags | -mod | -modfile | -overlay | -pkgdir | -exec | -toolexec | -pgo)
          shift $(($# > 1 ? 2 : 1)) ;;
        -*) shift ;;
        *) break ;;
      esac
    done
    [ $# -gt 0 ] || return 1
    w=${1%/}
    w=${w%@*}
    base=${w##*/}
    shift
  fi
  [ "$base" = regcorpus ] || return 1
  while [ $# -gt 0 ] && [[ $1 == -* ]]; do shift; done
  [ $# -gt 0 ] && [ "$1" = verify ]
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

# exists_at BASE PATH: succeed when the concrete PATH exists (or is a link).
exists_at() {
  case $2 in '' | *"$DYN"*) return 1 ;; esac
  join_dir "$1" "$2"
  [ "$RES" != '?' ] && { [ -e "$RES" ] || [ -L "$RES" ]; }
}

check_git() { # CW ARG...: ARG follows "git"
  local base=$1 sub a
  shift
  while [ $# -gt 0 ]; do
    a=$1
    case $a in
      -C)
        join_dir "$base" "${2-}"
        base=$RES
        shift $(($# > 1 ? 2 : 1))
        ;;
      -c | --config-env)
        risky_config "${2-}" && block "git $a ${2-} changes the hooks path (R11)"
        shift $(($# > 1 ? 2 : 1))
        ;;
      -c* | --config-env=*)
        risky_config "$a" && block "git $a changes the hooks path (R11)"
        shift
        ;;
      --git-dir | --work-tree | --namespace | --super-prefix | --attr-source) shift $(($# > 1 ? 2 : 1)) ;;
      -*) shift ;;
      *) break ;;
    esac
  done
  [ $# -gt 1 ] || return 0
  sub=$1
  shift
  case $sub in
    commit) check_commit_opts "$@" ;;
    merge | pull | push | rebase | am)
      for a in "$@"; do
        [ "$a" = -- ] && break
        no_verify_word "$a" && block "git $sub $a skips the git hooks (R11)"
      done
      ;;
    config) check_config_args "$@" ;;
    rm)
      parse_opts '' 'pathspec-from-file' "$@"
      hasflag cached || judge_ops "$base" "git rm" 0
      ;;
    mv) parse_opts '' '' "$@"; judge_ops "$base" "git mv" 0 ;;
    clean) parse_opts e 'exclude' "$@"; judge_ops "$base" "git clean" 0 ;;
    restore)
      parse_opts s 'source pathspec-from-file' "$@"
      if hasflag S staged && ! hasflag W worktree; then return 0; fi # index only
      judge_ops "$base" "git restore" 0
      ;;
    checkout)
      # Without "--" the first operand is a branch or commit unless it names
      # an existing path; the operands after it are paths.
      parse_opts bB 'orphan pathspec-from-file' "$@"
      if [ "$OPS_DD" -ge 0 ]; then
        judge_ops "$base" "git checkout" "$OPS_DD"
      elif [ "${#OPS[@]}" -gt 0 ]; then
        exists_at "$base" "${OPS[0]}" && judge_sh "${OPS[0]}" "$base" "git checkout"
        judge_ops "$base" "git checkout" 1
      fi
      ;;
  esac
}

# judge_inplace CW WHAT INPLACE SCRIPT TAKES ARG...: sed and perl edit files
# only with an in-place option. Option letters: INPLACE turn it on, SCRIPT
# give the script (so the first operand is a file), TAKES take an argument
# (ending the cluster). Without a SCRIPT option the first operand is the
# script. sed's bare -i may be followed by a BSD backup suffix ('' or .bak).
judge_inplace() {
  local cw=$1 what=$2 inl=$3 scl=$4 takes=$5 a inplace=0 script=0 skip=0 suffix=0 rest ch
  local -a files
  shift 5
  files=()
  for a in "$@"; do
    if [ "$skip" = 1 ]; then
      skip=0
      continue
    fi
    if [ "$suffix" = 1 ]; then
      suffix=0
      case $a in '' | .*) [[ $a == */* ]] || continue ;; esac
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
          case $inl in
            *"$ch"*)
              inplace=1
              [ -z "$rest" ] && [ "$what" = "sed -i" ] && suffix=1
              break
              ;;
          esac
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
  for a in "${files[@]}"; do judge_sh "$a" "$cw" "$what"; done
}

# chmod_mode WORD: succeed when WORD is a chmod mode (644, u+x, a=rw,g-w).
re_mode='^([0-7]+|[ugoa]*[-+=][rwxXstugo]*([-+=][rwxXstugo]*)*(,[ugoa]*[-+=][rwxXstugo]*([-+=][rwxXstugo]*)*)*)$'

# write_cmd BASE MODE CW ARG...: a known writer and the operands it writes.
write_cmd() {
  local base=$1 mode=$2 cw=$3 n a
  shift 3
  case $base in
    rm | rmdir | unlink | tee) parse_opts '' '' "$@"; judge_ops "$cw" "$base" 0 ;;
    touch) parse_opts rdtA 'reference date time' "$@"; judge_ops "$cw" touch 0 ;;
    mkdir) parse_opts m 'mode' "$@"; judge_ops "$cw" mkdir 0 ;;
    truncate) parse_opts sr 'size reference' "$@"; judge_ops "$cw" truncate 0 ;;
    chmod)
      parse_opts '' 'reference' "$@"
      n=0
      if ! optval reference && [ "${#OPS[@]}" -gt 0 ] && [[ ${OPS[0]} =~ $re_mode ]]; then n=1; fi
      judge_ops "$cw" chmod "$n"
      ;;
    chown | chgrp)
      parse_opts '' 'reference from' "$@"
      n=1
      optval reference && n=0
      judge_ops "$cw" "$base" "$n"
      ;;
    mv)
      parse_opts tS 'target-directory suffix' "$@"
      optval t target-directory && judge_sh "$OPTVAL" "$cw" mv
      judge_ops "$cw" mv 0 # mv also removes its sources
      ;;
    cp | ln | install)
      parse_opts tSgmo 'target-directory suffix group mode owner strip-program' "$@"
      if optval t target-directory; then
        judge_sh "$OPTVAL" "$cw" "$base" # -t: the operands are sources
        return 0
      fi
      if [ "$base" = install ] && hasflag d directory; then
        judge_ops "$cw" install 0
        return 0
      fi
      [ "$mode" = tail ] && return 0 # the destination comes from stdin
      n=${#OPS[@]}
      if [ "$n" -ge 2 ]; then
        judge_sh "${OPS[n-1]}" "$cw" "$base"
      elif [ "$n" = 1 ] && [ "$base" = ln ]; then
        a=${OPS[0]}
        while [[ $a == */ ]]; do a=${a%/}; done
        judge_sh "${a##*/}" "$cw" ln # ln TARGET links TARGET's name here
      fi
      ;;
    rsync)
      parse_opts eBfTM 'rsh filter exclude include exclude-from include-from files-from block-size temp-dir partial-dir backup-dir suffix compare-dest copy-dest link-dest remote-option rsync-path log-file log-file-format out-format chmod chown usermap groupmap timeout contimeout port password-file bwlimit max-size min-size max-delete modify-window info debug iconv sockopts address' "$@"
      [ "$mode" = tail ] && return 0
      n=${#OPS[@]}
      [ "$n" -ge 2 ] && judge_sh "${OPS[n-1]}" "$cw" rsync
      ;;
    dd)
      for a in "$@"; do
        case $a in of=*) judge_sh "${a#of=}" "$cw" "dd of=" ;; esac
      done
      ;;
    sed) judge_inplace "$cw" "sed -i" iI ef l "$@" ;;
    perl) judge_inplace "$cw" "perl -i" i eE MmIxld0DCV "$@" ;;
  esac
  return 0
}
writers=' rm rmdir unlink tee touch mkdir truncate chmod chown chgrp mv cp ln install rsync dd sed perl '

# move_cwd cd|pushd|popd ARG...: follow the shell's working directory (CWD)
# and directory stack (DIRS). What can't be followed makes CWD unknown ("?").
move_cwd() {
  local kind=$1 phys=0
  shift
  while [ $# -gt 0 ]; do
    case $1 in
      --) shift; break ;;
      -n) return 0 ;; # pushd/popd -n: the stack only
      [-+][0-9]*) CWD='?'; return 0 ;;
      -) break ;;
      -*)
        case $1 in *P*) phys=1 ;; *L*) phys=0 ;; esac
        shift
        ;;
      *) break ;;
    esac
  done
  if [ "$kind" = popd ]; then
    if [ "${#DIRS[@]}" -gt 0 ]; then
      CWD=${DIRS[${#DIRS[@]}-1]}
      unset "DIRS[${#DIRS[@]}-1]"
    else
      CWD='?'
    fi
    return 0
  fi
  if [ $# -eq 0 ]; then
    if [ "$kind" = pushd ]; then
      CWD='?' # swaps the top two entries
      return 0
    fi
    set -- "${HOME_V:-$DYN}"
  fi
  [ "$1" = - ] && set -- "$DYN" # cd -: back to $OLDPWD
  join_dir "$CWD" "$1"
  if [ "$RES" != '?' ]; then
    if [ "$phys" = 1 ]; then physical "$RES"; else lexical "$RES"; fi
  fi
  [ "$kind" = pushd ] && DIRS[${#DIRS[@]}]=$CWD
  CWD=$RES
}

# env_cmd CW ARG...: env [options] [NAME=VALUE...] [command]
env_cmd() {
  local cw=$1 s= a q u
  shift
  while [ $# -gt 0 ]; do
    case $1 in
      -C | --chdir) join_dir "$cw" "${2-}"; cw=$RES; shift $(($# > 1 ? 2 : 1)) ;;
      -C?*) join_dir "$cw" "${1:2}"; cw=$RES; shift ;;
      --chdir=*) join_dir "$cw" "${1#*=}"; cw=$RES; shift ;;
      -S | --split-string) s=${2-}; shift $(($# > 1 ? 2 : 1)) ;;
      -S?*) s=${1:2}; shift ;;
      --split-string=*) s=${1#*=}; shift ;;
      -u | -P | -L | -U | --unset) shift $(($# > 1 ? 2 : 1)) ;;
      --) shift; break ;;
      -?*) shift ;;
      *) break ;;
    esac
  done
  while [ $# -gt 0 ]; do
    [[ $1 =~ $re_assign ]] || break
    check_assignment "$1"
    shift
  done
  if [ -n "$s" ]; then # -S: the string is split into the command's words
    for a in "$@"; do
      case $a in
        *"$DYN"*) s="$s \"\$_\"" ;;
        *) printf -v q '%q' "$a"; s="$s $q" ;;
      esac
    done
    nest "$s" "$cw" $((DEPTH + 1))
    return 0
  fi
  cmd sub "$cw" "$@"
}

# xargs_cmd CW ARG...: the command gets more operands from stdin, unless -I
# (or -J, -i) names where they go.
xargs_cmd() {
  local cw=$1 rep= a
  local -a ws
  shift
  while [ $# -gt 0 ]; do
    case $1 in
      -I | -J) rep=${2-}; shift $(($# > 1 ? 2 : 1)) ;;
      -I?* | -J?*) rep=${1:2}; shift ;;
      -i | --replace) rep='{}'; shift ;;
      -i?*) rep=${1:2}; shift ;;
      --replace=*) rep=${1#*=}; shift ;;
      -a | -d | -E | -L | -n | -P | -R | -S | -s | --arg-file | --delimiter | --eof | --max-lines | --max-args | --max-procs | --max-chars | --process-slot-var)
        shift $(($# > 1 ? 2 : 1)) ;;
      --) shift; break ;;
      -?*) shift ;;
      *) break ;;
    esac
  done
  [ $# -gt 0 ] || return 0
  if [ -z "$rep" ]; then
    cmd tail "$cw" "$@"
    return 0
  fi
  ws=()
  for a in "$@"; do
    case $a in
      *"$rep"*) dyn "$a" "$rep"; ws[${#ws[@]}]=$RES ;;
      *) ws[${#ws[@]}]=$a ;;
    esac
  done
  cmd sub "$cw" "${ws[@]}"
}

# find_cmd CW ARG...: each -exec/-execdir/-ok/-okdir command, with {} taken
# as the start point that every path find passes it lies under.
find_cmd() {
  local cw=$1 a kind sp cwx
  local -a starts ex ws
  shift
  starts=()
  while [ $# -gt 0 ]; do
    case $1 in
      -H | -L | -P | -E | -X | -d | -s | -x | -O*) shift ;;
      -D) shift $(($# > 1 ? 2 : 1)) ;;
      -f) starts[${#starts[@]}]=${2-}; shift $(($# > 1 ? 2 : 1)) ;;
      *) break ;;
    esac
  done
  while [ $# -gt 0 ]; do
    case $1 in -* | '(' | '!' | ')' | ',') break ;; esac
    starts[${#starts[@]}]=$1
    shift
  done
  [ "${#starts[@]}" -gt 0 ] || starts=(.)
  while [ $# -gt 0 ]; do
    a=$1
    shift
    case $a in -exec | -execdir | -ok | -okdir) ;; *) continue ;; esac
    kind=$a
    ex=()
    while [ $# -gt 0 ]; do
      a=$1
      shift
      [ "$a" = ';' ] && break
      [ "$a" = '+' ] && [ "${#ex[@]}" -gt 0 ] && [ "${ex[${#ex[@]}-1]}" = '{}' ] && break
      ex[${#ex[@]}]=$a
    done
    [ "${#ex[@]}" -gt 0 ] || continue
    for sp in "${starts[@]}"; do
      cwx=$cw
      case $kind in
        -execdir | -okdir) # runs in each found file's directory
          cwx='?'
          join_dir "$cw" "$sp"
          if [ "$RES" = '?' ]; then sp=$DYN$sp; else sp=$RES; fi
          ;;
      esac
      ws=()
      for a in "${ex[@]}"; do
        case $a in
          '{}') ws[${#ws[@]}]=$sp ;;
          *'{}'*) dyn "$a" '{}'; ws[${#ws[@]}]=$RES ;;
          *) ws[${#ws[@]}]=$a ;;
        esac
      done
      cmd sub "$cwx" "${ws[@]}"
    done
  done
}

# parallel_cmd CW ARG...: GNU parallel runs its command once per input (from
# ::: lists when given), substituting {} or appending the input.
parallel_cmd() {
  local cw=$1 a in rep=0 ng=0 known=1
  local -a tpl ins ws
  shift
  tpl=()
  ins=()
  while [ $# -gt 0 ]; do
    case $1 in
      --workdir | --wd)
        case ${2-} in . ) ;; ...) cw='?' ;; *) join_dir "$cw" "${2-}"; cw=$RES ;; esac
        shift $(($# > 1 ? 2 : 1))
        ;;
      -j | -S | -a | -d | -E | -I | -C | -n | -N | -L | -P | -s | --jobs | --sshlogin | --sshloginfile | --slf | --arg-file | --delimiter | --eof | --colsep | --max-args | --max-replace-args | --max-lines | --max-procs | --max-chars | --joblog | --results | --res | --tmpdir | --tagstring | --tag-string | --delay | --timeout | --retries | --memfree | --load | --halt | --basefile | --bf | --return | --env | --rpl | --header | --block | --recstart | --recend | --termseq)
        shift $(($# > 1 ? 2 : 1)) ;;
      ::: | :::: | :::+ | ::::+) break ;;
      --) shift; break ;;
      -?*) shift ;;
      *) break ;;
    esac
  done
  while [ $# -gt 0 ]; do
    case $1 in ::: | :::: | :::+ | ::::+) break ;; esac
    tpl[${#tpl[@]}]=$1
    shift
  done
  while [ $# -gt 0 ]; do
    case $1 in
      ::: | :::+) ng=$((ng + 1)) ;;
      :::: | ::::+) known=0 ;;
      *) ins[${#ins[@]}]=$1 ;;
    esac
    shift
  done
  [ "$ng" = 1 ] && [ "${#ins[@]}" -gt 0 ] || known=0
  if [ "${#tpl[@]}" -eq 0 ]; then # the inputs are the commands
    if [ "$known" = 1 ]; then
      for in in "${ins[@]}"; do
        case $in in *"$DYN"*) ;; *) nest "$in" "$cw" $((DEPTH + 1)) ;; esac
      done
    fi
    return 0
  fi
  for a in "${tpl[@]}"; do
    case $a in *'{'*'}'*) rep=1 ;; esac
  done
  if [ "$known" = 0 ]; then
    ws=()
    for a in "${tpl[@]}"; do
      case $a in *'{'*'}'*) dyn "$a" '{'; ws[${#ws[@]}]=$RES ;; *) ws[${#ws[@]}]=$a ;; esac
    done
    if [ "$rep" = 1 ]; then cmd sub "$cw" "${ws[@]}"; else cmd tail "$cw" "${ws[@]}"; fi
    return 0
  fi
  for in in "${ins[@]}"; do
    ws=()
    for a in "${tpl[@]}"; do
      case $a in
        '{}') ws[${#ws[@]}]=$in ;;
        *'{'*'}'*) dyn "$a" '{'; ws[${#ws[@]}]=$RES ;;
        *) ws[${#ws[@]}]=$a ;;
      esac
    done
    [ "$rep" = 1 ] || ws[${#ws[@]}]=$in
    cmd sub "$cw" "${ws[@]}"
  done
}

# cmd MODE CW WORD...: check one command given as its words. MODE: top (run
# by the shell itself, so cd, pushd and popd move CWD), sub (run by another
# program: env, xargs, find -exec...) or tail (as sub, but more operands
# follow that aren't known: xargs without -I).
cmd() {
  local mode=$1 cw=$2 w u base j
  local -a ws
  shift 2
  while [ $# -gt 0 ]; do
    if [[ $1 =~ $re_assign ]]; then
      check_assignment "$1"
      shift
      continue
    fi
    case $1 in
      '!' | '{' | '}' | if | then | else | elif | do | while | until) shift ;;
      *) break ;;
    esac
  done
  [ $# -gt 0 ] || return 0
  w=$1
  shift
  case $w in *"$DYN"*) return 0 ;; esac # not a concrete command name
  rc_verify "$w" "$@" && block "$verify_msg"
  base=${w##*/}
  case $base in
    g?*) case $writers in *" ${base#g} "*) base=${base#g} ;; esac ;; # GNU names on macOS
  esac
  case $base in
    command)
      case ${1-} in
        -v | -V) return 0 ;;
        -p) shift ;;
      esac
      cmd "$mode" "$cw" "$@"
      ;;
    builtin) cmd "$mode" "$cw" "$@" ;;
    exec)
      while [ $# -gt 0 ]; do
        case $1 in
          -a) shift $(($# > 1 ? 2 : 1)) ;;
          -?*) shift ;;
          *) break ;;
        esac
      done
      cmd sub "$cw" "$@"
      ;;
    time)
      while [ $# -gt 0 ]; do
        case $1 in
          -o | --output) judge_sh "${2-}" "$cw" "time -o"; shift $(($# > 1 ? 2 : 1)) ;;
          --output=*) judge_sh "${1#*=}" "$cw" "time -o"; shift ;;
          -f | --format) shift $(($# > 1 ? 2 : 1)) ;;
          --) shift; break ;;
          -?*) shift ;;
          *) break ;;
        esac
      done
      if [ "$w" = time ]; then cmd "$mode" "$cw" "$@"; else cmd sub "$cw" "$@"; fi
      ;;
    nohup)
      [ "${1-}" = -- ] && shift
      cmd sub "$cw" "$@"
      ;;
    nice)
      while [ $# -gt 0 ]; do
        case $1 in
          -n | --adjustment) shift $(($# > 1 ? 2 : 1)) ;;
          --) shift; break ;;
          -?*) shift ;;
          *) break ;;
        esac
      done
      cmd sub "$cw" "$@"
      ;;
    timeout)
      while [ $# -gt 0 ]; do
        case $1 in
          -s | -k | --signal | --kill-after) shift $(($# > 1 ? 2 : 1)) ;;
          --) shift; break ;;
          -?*) shift ;;
          *) break ;;
        esac
      done
      [ $# -gt 0 ] && shift # the duration
      cmd sub "$cw" "$@"
      ;;
    stdbuf)
      while [ $# -gt 0 ]; do
        case $1 in
          -i | -o | -e | --input | --output | --error) shift $(($# > 1 ? 2 : 1)) ;;
          --) shift; break ;;
          -?*) shift ;;
          *) break ;;
        esac
      done
      cmd sub "$cw" "$@"
      ;;
    env) env_cmd "$cw" "$@" ;;
    xargs) xargs_cmd "$cw" "$@" ;;
    find) find_cmd "$cw" "$@" ;;
    parallel) parallel_cmd "$cw" "$@" ;;
    git) check_git "$cw" "$@" ;;
    script | expect | unbuffer)
      ws=("$w" "$@")
      for ((j = 1; j < ${#ws[@]}; j++)); do
        if rc_verify "${ws[@]:j}" || [[ ${ws[j]} =~ $re_rcverify ]]; then
          block "$verify_msg"
        fi
      done
      ;;
    sh | bash | zsh | dash | ksh)
      while [ $# -gt 0 ]; do
        case $1 in
          -o | +o | -O | +O) shift $(($# > 1 ? 2 : 1)) ;;
          --*) shift ;;
          -*c*)
            [ $# -gt 1 ] && nest "$2" "$cw" $((DEPTH + 1))
            break
            ;;
          [-+]*) shift ;;
          *) break ;;
        esac
      done
      ;;
    cd | pushd | popd) [ "$mode" = top ] && move_cwd "$base" "$@" ;;
    export | declare | typeset | readonly | local)
      for u in "$@"; do check_assignment "$u"; done
      ;;
    *) case $writers in *" $base "*) write_cmd "$base" "$mode" "$cw" "$@" ;; esac ;;
  esac
  return 0
}

# simple: check the simple command in SW (words), SR_OP / SR_T (redirects).
simple() {
  local r=0
  while [ "$r" -lt "${#SR_OP[@]}" ]; do
    case ${SR_OP[r]} in
      *'<<' | *'<<-' | *'<<<' | *'<' | *'<&') ;;
      *'>&') [[ ${SR_T[r]} =~ $re_fddup ]] || judge_sh "${SR_T[r]}" "$CWD" "redirect to" ;;
      *) judge_sh "${SR_T[r]}" "$CWD" "redirect to" ;;
    esac
    r=$((r + 1))
  done
  [ "${#SW[@]}" -gt 0 ] || return 0
  cmd top "$CWD" "${SW[@]}"
}

# check_command CMD CWD: lex CMD and check each simple command in it, then
# every command string nested inside it, each in the directory it runs in.
# A pipeline's commands and a background job run in subshells: a cd there
# does not move the shell (pipe_cwd is where the pipeline started).
check_command() {
  local q=0 t nt pipe_cwd inpipe
  NESTED=("$1")
  NESTED_CWD=("$2")
  NESTED_DEPTH=(0)
  while [ "$q" -lt "${#NESTED[@]}" ]; do
    DEPTH=${NESTED_DEPTH[q]}
    [ "$DEPTH" -le "$MAX_DEPTH" ] || block "command nested too deeply to check (fail closed)"
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
    DIRS=()
    SW=()
    SR_OP=()
    SR_T=()
    pipe_cwd=
    inpipe=0
    nt=${#TT[@]}
    for ((t = 0; t < nt; t++)); do
      case ${TT[t]} in
        W) SW[${#SW[@]}]=${TV[t]} ;;
        R)
          SR_OP[${#SR_OP[@]}]=${TV[t]}
          SR_T[${#SR_T[@]}]=${TX[t]}
          ;;
        S) nest "${TV[t]}" "$CWD" $((DEPTH + 1)) ;;
        O)
          [ "$inpipe" = 1 ] || pipe_cwd=$CWD
          simple
          SW=()
          SR_OP=()
          SR_T=()
          case ${TV[t]} in
            '|') inpipe=1; CWD=$pipe_cwd ;;
            '&') inpipe=0; CWD=$pipe_cwd ;;
            *) [ "$inpipe" = 1 ] && CWD=$pipe_cwd; inpipe=0 ;; # a pipeline's last command
          esac
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
    simple
    q=$((q + 1))
  done
}

# ---- tools -------------------------------------------------------------------

# tool_strings: set the positional parameters (via PATHS_SH) to every string
# under a key of the tool input that names a write target (a copy's or an
# upload's source is read, so it is not one).
tool_strings() {
  PATHS_SH=$(jq -r '[.tool_input | .. | objects | to_entries[]
      | select(.key | test("^(file_path|notebook_path|filePath|file|filename|path|paths|destination|dest|target|to|new_path)$"))
      | .value | if type == "array" then .[] else . end | select(type == "string")]
      | @sh "set -- \(.)"' <<<"$input" 2>/dev/null) ||
    block "unreadable tool input, so the tool call cannot be checked (fail closed)"
}

# reads_only TOOL: succeed when an MCP tool's verb says it leaves local files
# alone. The verb is the first word of the name after the server (split at
# _ - . and camelCase), past a repeat of the server's name (git in
# mcp__git__git_diff). A tool whose verb is not a read verb is judged.
reads_only() {
  local a=${1##*__} s=${1%__*} words= c i hit=1
  s=${s##*[_-]}
  for ((i = 0; i < ${#a}; i++)); do
    c=${a:i:1}
    case $c in
      [A-Z]) words="$words $c" ;;
      _ | - | .) words="$words " ;;
      *) words=$words$c ;;
    esac
  done
  set -- $words
  shopt -s nocasematch
  [[ ${1-} == "$s" ]] && shift
  case ${1-} in
    read | get | list | search | view | stat | info | find | query | fetch | show | diff | compare | upload)
      hit=0 ;;
  esac
  shopt -u nocasematch
  return "$hit"
}

# ---- main --------------------------------------------------------------------

command -v jq >/dev/null 2>&1 || block "jq is not installed, so the tool call cannot be checked (fail closed)"
input=
IFS= read -r -d '' input || true
fields=$(jq -r '@sh "tool=\(.tool_name // "" | tostring) cwd=\(.cwd // "" | tostring) cmd=\(.tool_input.command // "" | tostring)"' <<<"$input" 2>/dev/null) ||
  block "unreadable hook input, so the tool call cannot be checked (fail closed)"
[ -n "$fields" ] || block "empty hook input, so the tool call cannot be checked (fail closed)"
eval "$fields"

case $tool in
  Bash) check_command "$cmd" "$cwd" ;;
  Edit | Write | MultiEdit | NotebookEdit)
    tool_strings
    eval "$PATHS_SH"
    [ $# -gt 0 ] || block "$tool call has no file path to check (fail closed)"
    for p in "$@"; do
      [ -n "$p" ] || block "$tool call has an empty file path (fail closed)"
      case $p in *[[:cntrl:]]*) block "$tool path contains a control character (fail closed)" ;; esac
      judge "$p" "$cwd" || block "$tool on $REL: $protected_msg"
    done
    ;;
  mcp__*)
    # Every tool but a read, and only paths that resolve on their own: the
    # server's working directory is unknown, so a relative path (often a
    # remote repo path) is not judged, nor is a value holding a control
    # character (content, not a path).
    reads_only "$tool" && { trap - EXIT; exit 0; }
    tool_strings
    eval "$PATHS_SH"
    for p in "$@"; do
      case $p in
        *[[:cntrl:]]*) continue ;;
        '~' | '~/'*) [ -n "$HOME_V" ] || continue; p=$HOME_V${p#\~} ;;
        /*) ;;
        *) continue ;;
      esac
      judge "$p" / || block "$tool on $REL: $protected_msg"
    done
    ;;
esac
trap - EXIT
exit 0
