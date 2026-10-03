#!/usr/bin/env bash
# Interactive official subscription login. Does not start a model turn.
set -euo pipefail
runtime=${1:?absolute credential-free runtime required}
provider=${2:?claude or codex required}
auth=${3:?dedicated private auth directory required}
[[ $runtime == /* && $runtime != / && -d $runtime && $auth == /* && $auth != / && -d $auth ]] || exit 1
runtime=$(realpath "$runtime")
auth=$(realpath "$auth")
case "$auth/" in "$runtime/"*) exit 1 ;; esac
case "$runtime/" in "$auth/"*) exit 1 ;; esac
[[ $(stat -c %a "$auth") == 700 && $(stat -c %u "$auth") == "$(id -u)" ]] || exit 1
args=()
case "$provider" in
  claude) args=(--setenv CLAUDE_CONFIG_DIR /agent-auth); command=(/usr/local/bin/claude auth login --claudeai) ;;
  codex) args=(--setenv CODEX_HOME /agent-auth); command=(/usr/local/bin/codex login --device-auth) ;;
  *) exit 1 ;;
esac
exec /usr/bin/bwrap --unshare-all --share-net --die-with-parent --new-session --clearenv \
  --setenv HOME /agent-auth --setenv PATH /usr/local/bin:/usr/bin:/bin \
  "${args[@]}" --ro-bind "$runtime" / --proc /proc --dev /dev --tmpfs /tmp \
  --bind "$auth" /agent-auth --chdir /tmp -- "${command[@]}"
