#!/usr/bin/env bash
# Interactive official subscription login. Does not start a model turn.
set -euo pipefail
fail() { printf 'login-provider: %s\n' "$1" >&2; exit 1; }
[[ $# == 3 ]] || fail 'expected runtime, provider, and auth directory arguments'
runtime=$1
provider=$2
auth=$3
[[ $runtime == /* && $runtime != / && -d $runtime ]] || fail 'runtime must be an existing absolute directory'
[[ $auth == /* && $auth != / && -d $auth ]] || fail 'auth home must be an existing absolute directory'
runtime=$(realpath "$runtime")
auth=$(realpath "$auth")
case "$auth/" in "$runtime/"*) fail 'auth home must be outside runtime' ;; esac
case "$runtime/" in "$auth/"*) fail 'runtime must be outside auth home' ;; esac
[[ $(stat -c %a "$auth") == 700 ]] || fail 'auth home must have mode 700'
[[ $(stat -c %u "$auth") == "$(id -u)" ]] || fail 'auth home must belong to the invoking user'
args=()
case "$provider" in
  claude) args=(--setenv CLAUDE_CONFIG_DIR /agent-auth); command=(/usr/local/bin/claude auth login --claudeai) ;;
  codex) args=(--setenv CODEX_HOME /agent-auth); command=(/usr/local/bin/codex login --device-auth) ;;
  *) fail 'provider must be claude or codex' ;;
esac
exec /usr/bin/bwrap --unshare-all --share-net --die-with-parent --new-session --clearenv \
  --setenv HOME /agent-auth --setenv PATH /usr/local/bin:/usr/bin:/bin \
  "${args[@]}" --ro-bind "$runtime" / --proc /proc --dev /dev --tmpfs /tmp \
  --bind "$auth" /agent-auth --chdir /tmp -- "${command[@]}"
