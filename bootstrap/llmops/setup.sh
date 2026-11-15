#!/usr/bin/env bash
# Konro LLMOps rollout: show where each step stands, and walk through the rest.
#
#   bootstrap/llmops/setup.sh          # status only (read-only)
#   bootstrap/llmops/setup.sh run      # walk the remaining steps, asking before each
#
# Every step is checked against live state (Vault, Keycloak, GitHub, the cluster,
# sense), so rerunning is safe: finished steps are skipped. Secrets are never
# printed. Merges stay manual: the script shows the PR and waits.
#
# Overrides: KUBE_CONTEXT, KUBECONFIG_FILE, KENSAN_LAB_DIR, KOUBOU_DIR,
#            KONRO_VERSION, LANGFUSE_LOCAL_PORT
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$HERE/../.." && pwd)
KUBE_CONTEXT=${KUBE_CONTEXT:-kubernetes-admin@kubernetes}
KUBECONFIG_FILE=${KUBECONFIG_FILE:-$HOME/.kube/config}
KENSAN_LAB_DIR=${KENSAN_LAB_DIR:-$HOME/kensan-workspace/repositories/kensan-lab}
KOUBOU_DIR=${KOUBOU_DIR:-$HOME/koubou}
KONRO_VERSION=${KONRO_VERSION:-v0.13.0}
LANGFUSE_LOCAL_PORT=${LANGFUSE_LOCAL_PORT:-3300}
EVAL_SECRET_FILE=$HOME/.config/konro-eval/client-secret
LAB_PR=592
KOUBOU_PR=41
LANGFUSE_PATH=secret/observability/langfuse
COLLECTOR_PATH=secret/observability/otel-collector
LANGFUSE_KEYS="salt encryption-key nextauth-secret postgres-password postgres-admin-password redis-password clickhouse-password s3-password konro-public-key konro-secret-key init-user-email init-user-password"

if [ -t 1 ]; then
  G=$'\e[32m' Y=$'\e[33m' R=$'\e[31m' D=$'\e[2m' B=$'\e[1m' N=$'\e[0m'
else
  G='' Y='' R='' D='' B='' N=''
fi

k() { kubectl --kubeconfig "$KUBECONFIG_FILE" --context "$KUBE_CONTEXT" --request-timeout=10s "$@"; }
sense() { ssh -o BatchMode=yes -o ConnectTimeout=5 sense "$@"; }
have() { command -v "$1" >/dev/null 2>&1; }

# Each check_* sets STATE (ok|todo|ng|unknown|manual) and DETAIL.
STATE='' DETAIL=''
set_state() { STATE=$1; DETAIL=$2; }

# pad TEXT WIDTH: left-justify by display width (CJK counts as 2).
pad() {
  local chars bytes w
  chars=$(printf '%s' "$1" | LC_ALL=en_US.UTF-8 wc -m | tr -d ' ')
  bytes=$(printf '%s' "$1" | LC_ALL=C wc -c | tr -d ' ')
  w=$((chars + (bytes - chars) / 2))
  printf '%s%*s' "$1" $(( $2 > w ? $2 - w : 0 )) ''
}

mark() {
  case "$1" in
    ok) printf '%s✓%s' "$G" "$N" ;;
    todo) printf '%s·%s' "$Y" "$N" ;;
    ng) printf '%s✗%s' "$R" "$N" ;;
    manual) printf '%s◇%s' "$D" "$N" ;;
    *) printf '%s?%s' "$D" "$N" ;;
  esac
}

# ---- prerequisites --------------------------------------------------------

P_VAULT=no P_KUBE=no P_SENSE=no P_GH=no P_SUDO=no
check_prereqs() {
  have vault && vault token lookup >/dev/null 2>&1 && P_VAULT=yes || P_VAULT=no
  have kubectl && k get ns default >/dev/null 2>&1 && P_KUBE=yes || P_KUBE=no
  sense true >/dev/null 2>&1 && P_SENSE=yes || P_SENSE=no
  P_SUDO=no
  [ $P_SENSE = yes ] && sense 'sudo -n true' >/dev/null 2>&1 && P_SUDO=yes
  have gh && gh auth status >/dev/null 2>&1 && P_GH=yes || P_GH=no
}

prereq_line() {
  local s
  p() { if [ "$2" = yes ]; then s=ok; else s=ng; fi; printf ' %s %s' "$(mark $s)" "$1"; }
  printf '%s前提%s ' "$B" "$N"
  p vault $P_VAULT; p kubectl $P_KUBE; p 'ssh sense' $P_SENSE; p 'sense sudo' $P_SUDO; p gh $P_GH
  echo
}

# ---- step checks ----------------------------------------------------------

vault_keys() { vault kv get -format=json "$1" 2>/dev/null | jq -r '.data.data | keys[]' 2>/dev/null; }

check_1() {
  [ $P_VAULT = yes ] || { set_state unknown "vault にログインしていない"; return; }
  local have_keys n=0 total=0 key c
  have_keys=$(vault_keys $LANGFUSE_PATH)
  for key in $LANGFUSE_KEYS; do
    total=$((total + 1))
    grep -qx "$key" <<<"$have_keys" && n=$((n + 1))
  done
  vault_keys $COLLECTOR_PATH | grep -qx otlp-external-token && c=1 || c=0
  if [ $n -eq $total ] && [ $c -eq 1 ]; then
    set_state ok "langfuse $n/${total}・collector $c/1"
  else
    set_state todo "langfuse $n/${total}・collector $c/1"
  fi
}

KEYCLOAK_OUT=''
check_2() {
  [ $P_KUBE = yes ] || { set_state unknown "kubectl が通らない"; return; }
  KEYCLOAK_OUT=$(python3 "$REPO/bootstrap/keycloak/setup-agent-service.py" \
    --kubeconfig "$KUBECONFIG_FILE" --context "$KUBE_CONTEXT" \
    --client konro-eval-agent-runtime --secret-file "$EVAL_SECRET_FILE" 2>/dev/null | tail -1)
  local file=no
  [ -s "$EVAL_SECRET_FILE" ] && [ "$(stat -f %Lp "$EVAL_SECRET_FILE" 2>/dev/null)" = 600 ] && file=yes
  case "$KEYCLOAK_OUT" in
    *'"exists": false'*) set_state todo "client 未作成" ;;
    *audience_mapper_missing*) set_state todo "client はあるが audience 設定が無い" ;;
    *'"written"'*)
      if [ $file = yes ]; then set_state ok "client あり・secret 0600"
      else set_state todo "client はあるが手元の secret が無い"; fi ;;
    *) set_state ng "確認スクリプトが失敗した" ;;
  esac
}

pr_state() { # repo number -> MERGED | OPEN-draft | OPEN | CLOSED | ?
  [ $P_GH = yes ] || { echo '?'; return; }
  gh pr view "$2" --repo "$1" --json state,isDraft \
    --jq 'if .state=="OPEN" and .isDraft then "OPEN-draft" else .state end' 2>/dev/null || echo '?'
}

pr_label() {
  case "$1" in
    MERGED) echo "merge 済み" ;; OPEN-draft) echo "draft" ;; OPEN) echo "review 待ち" ;;
    CLOSED) echo "close 済み(!)" ;; *) echo "PR 状態不明" ;;
  esac
}

LAB_STATE='' LANGFUSE_READY=''
check_3() {
  LAB_STATE=$(pr_state yu-min3/kensan-lab $LAB_PR)
  local pr; pr=$(pr_label "$LAB_STATE")
  [ $P_KUBE = yes ] || { set_state unknown "PR ${pr}・kubectl が通らない"; return; }
  local pods ready total es port
  pods=$(k -n langfuse get pods --no-headers 2>/dev/null)
  total=$(grep -c . <<<"$pods")
  ready=$(awk '{split($2,a,"/"); if (a[1]==a[2] && $3=="Running") n++} END {print n+0}' <<<"$pods")
  es=$(k -n monitoring get externalsecret otel-collector-credentials \
        -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
  port=$(k -n monitoring get deploy otel-collector-opentelemetry-collector -o json 2>/dev/null \
        | jq '[.spec.template.spec.containers[].ports[]?.containerPort] | index(4319) != null' 2>/dev/null)
  LANGFUSE_READY=no
  [ "$total" -gt 0 ] && [ "$ready" -eq "$total" ] && LANGFUSE_READY=yes
  local detail="PR ${pr}・Langfuse pod ${ready}/${total}・collector secret ${es:-なし}・4319 ${port:-false}"
  if [ "$LAB_STATE" = MERGED ] && [ $LANGFUSE_READY = yes ] && [ "$es" = True ] && [ "$port" = true ]; then
    set_state ok "$detail"
  elif [ "$LAB_STATE" = MERGED ]; then
    set_state todo "${detail}（Argo の反映待ち）"
  else
    set_state todo "$detail"
  fi
}

konro_json() { k -n app-konro get deploy konro -o json 2>/dev/null; }

check_4() {
  [ $P_KUBE = yes ] || { set_state unknown "kubectl が通らない"; return; }
  local json image tag otel newest
  json=$(konro_json)
  image=$(jq -r '.spec.template.spec.containers[0].image' <<<"$json")
  tag=${image##*:}
  otel=$(jq '[.spec.template.spec.containers[0].env[]? | select(.name|startswith("OTEL_"))] | length' <<<"$json")
  newest=$(printf '%s\n%s\n' "$tag" "$KONRO_VERSION" | sort -V | tail -1)
  if [ "$newest" = "$tag" ] && [ "${otel:-0}" -gt 0 ]; then
    set_state ok "konro ${tag}・OTel 設定あり"
  elif [ "$newest" = "$tag" ]; then
    set_state todo "konro ${tag}・OTel 設定がまだ（#$LAB_PR の反映待ち）"
  else
    set_state todo "konro ${tag}（$KONRO_VERSION 以上が必要）"
  fi
}

KOUBOU_STATE='' SENSE_READY=no
check_5() {
  KOUBOU_STATE=$(pr_state yu-min3/koubou $KOUBOU_PR)
  local pr; pr=$(pr_label "$KOUBOU_STATE")
  SENSE_READY=no
  [ $P_SENSE = yes ] || { set_state unknown "PR ${pr}・ssh sense が通らない"; return; }
  local active flag token=?
  active=$(sense 'systemctl is-active agent-runtime.service agent-runtime-auth.service' 2>/dev/null | tr '\n' ' ')
  flag=$(sense 'systemctl show agent-runtime.service -p ExecStart --value' 2>/dev/null | grep -c otlp-endpoint)
  if [ $P_SUDO = yes ]; then
    sense 'sudo -n test -s /opt/agent-runtime/otlp-token' >/dev/null 2>&1 && token=あり || token=なし
  fi
  local detail="PR ${pr}・token ${token}・telemetry 起動 $([ "$flag" -gt 0 ] && echo あり || echo なし)・${active% }"
  if [ "$flag" -gt 0 ] && [ "$active" = "active active " ]; then
    SENSE_READY=yes
    set_state ok "$detail"
  else
    set_state todo "$detail"
  fi
}

check_6() {
  [ $P_KUBE = yes ] || { set_state unknown "kubectl が通らない"; return; }
  local v
  v=$(konro_json | jq -r '.spec.template.spec.containers[0].env[]? | select(.name=="KONRO_AGENT_TRACE") | .value')
  if [ "$v" = true ]; then
    if [ $SENSE_READY = yes ]; then set_state ok "KONRO_AGENT_TRACE=true"
    else set_state ng "true だが sense が未対応。取り込みが 400 になる"; fi
  else
    set_state todo "KONRO_AGENT_TRACE=${v:-未設定}"
  fi
}

check_7() { set_state manual "Langfuse の画面で目視（run で手順を表示）"; }
check_8() {
  if [ -s "$EVAL_SECRET_FILE" ]; then set_state manual "準備済み。source bootstrap/llmops/eval-env.sh で実行"
  else set_state manual "2 が済んでから"; fi
}

TITLES=("" "Vault へ資格を入れる" "評価用 Keycloak client" "kensan-lab #$LAB_PR と Langfuse" "Konro $KONRO_VERSION 配備" "koubou #$KOUBOU_PR と sense" "KONRO_AGENT_TRACE を true に" "本番 trace を確かめる" "評価を一周する")

NEXT=''
show_status() {
  check_prereqs
  echo
  printf '%sKonro LLMOps 配備状況%s  %s%s%s\n' "$B" "$N" "$D" "$(date '+%Y-%m-%d %H:%M')" "$N"
  prereq_line
  NEXT=''
  local i
  for i in 1 2 3 4 5 6 7 8; do
    "check_$i"
    printf ' %s %d %s %s%s%s\n' "$(mark "$STATE")" "$i" "$(pad "${TITLES[$i]}" 30)" "$D" "$DETAIL" "$N"
    if [ -z "$NEXT" ] && [ "$STATE" != ok ]; then NEXT=$i; fi
  done
  echo
  if [ -n "$NEXT" ]; then
    printf '次: %s%d %s%s  →  bootstrap/llmops/setup.sh run\n' "$B" "$NEXT" "${TITLES[$NEXT]}" "$N"
  fi
  printf '%s✓ 済み  · 未  ✗ 要対応  ? 確認できない  ◇ 手動%s\n' "$D" "$N"
}

# ---- run mode ---------------------------------------------------------------

ask() { # prompt -> 0 on yes
  local a; printf '%s [y/N] ' "$1"; read -r a </dev/tty; [ "$a" = y ] || [ "$a" = Y ]
}
pause() { printf '%s' "$1"; read -r _ </dev/tty; }
header() { echo; printf '%s── %d. %s ──%s\n' "$B" "$1" "${TITLES[$1]}" "$N"; }
skip_note() { printf '%s  後で setup.sh run を再実行すれば続きから。%s\n' "$D" "$N"; }

# wait_for STEP SECONDS: re-check every 15s until ok or timeout.
wait_for() {
  local step=$1 limit=$2 t=0
  while [ $t -lt "$limit" ]; do
    "check_$step"
    printf '\r  %s %s%s' "$(mark "$STATE")" "$DETAIL" $'\e[K'
    [ "$STATE" = ok ] && { echo; return 0; }
    sleep 15; t=$((t + 15))
  done
  echo; return 1
}

run_prereqs() {
  check_prereqs
  prereq_line
  if [ $P_VAULT = no ]; then
    [ -n "${VAULT_ADDR:-}" ] || echo "  VAULT_ADDR が未設定。例: export VAULT_ADDR=https://vault.platform.yu-min3.com"
    ask "  vault login -method=oidc role=platform-admin を実行する？" && vault login -method=oidc role=platform-admin >/dev/null
  fi
  [ $P_KUBE = yes ] || echo "  kubectl が $KUBE_CONTEXT に届かない。KUBE_CONTEXT / KUBECONFIG_FILE を確認"
  [ $P_SENSE = yes ] || echo "  ssh sense が通らない（鍵と ~/.ssh/config を確認）"
  [ $P_SENSE = no ] || [ $P_SUDO = yes ] || echo "  sense で sudo にパスワードが要る。5 の途中で聞かれる"
  [ $P_GH = yes ] || echo "  gh が未ログイン（gh auth login）。PR の状態が見えない"
  check_prereqs
}

run_1() {
  echo "  Langfuse・DB・collector の資格を生成して Vault へ入れる。既存の値は変えない。"
  echo "  必要な入力はログイン用メールだけ。メールは送られないので実在しなくていい。"
  local email default=owner@langfuse.local
  if vault_keys $LANGFUSE_PATH | grep -qx init-user-email; then
    email=''
    echo "  ログインメールは設定済み（変更しない）。"
  else
    printf '  ログインメール [%s]: ' "$default"; read -r email </dev/tty
    email=${email:-$default}
  fi
  local args=(); [ -n "$email" ] && args=(--owner-email "$email")
  "$REPO/bootstrap/vault/llmops-secrets.sh" "${args[@]}" --dry-run || return 1
  ask "  この内容で書き込む？" || { skip_note; return 1; }
  "$REPO/bootstrap/vault/llmops-secrets.sh" "${args[@]}"
}

run_2() {
  echo "  評価 CLI 用の service client を realm kensan に作り、secret を ${EVAL_SECRET_FILE}（0600）へ置く。"
  ask "  作る？" || { skip_note; return 1; }
  python3 "$REPO/bootstrap/keycloak/setup-agent-service.py" \
    --kubeconfig "$KUBECONFIG_FILE" --context "$KUBE_CONTEXT" \
    --client konro-eval-agent-runtime --secret-file "$EVAL_SECRET_FILE" --apply
}

wait_merge() { # repo number label
  local s
  while :; do
    s=$(pr_state "$1" "$2")
    [ "$s" = MERGED ] && return 0
    echo "  $3: $(pr_label "$s")  https://github.com/$1/pull/$2"
    echo "  GitHub で merge してから Enter（s で後回し）"
    local a; read -r a </dev/tty; [ "$a" = s ] && { skip_note; return 1; }
  done
}

run_3() {
  check_1
  [ "$STATE" = ok ] || { echo "  ${R}1 が済んでいない。先に merge すると collector の新 pod が起動しない。${N}"; return 1; }
  if [ "$LAB_STATE" != MERGED ]; then
    echo "  merge すると Argo CD が Langfuse を作り、collector と gateway を更新する。"
    wait_merge yu-min3/kensan-lab $LAB_PR "kensan-lab #$LAB_PR" || return 1
  fi
  echo "  Argo の反映を待つ（最大15分。ClickHouse の初回は数分かかる）"
  wait_for 3 900 && return 0
  echo "  まだ揃わない。見る場所:"
  echo "    kubectl -n langfuse get pods"
  echo "    kubectl -n monitoring get externalsecret otel-collector-credentials"
  return 1
}

run_4() {
  check_3
  [ "$STATE" = ok ] || { echo "  3 が済んでから。"; return 1; }
  echo "  $KENSAN_LAB_DIR で main を pull し、make deploy VERSION=$KONRO_VERSION を実行する。"
  echo "  GHCR への push に make login 済みであること。"
  ask "  実行する？" || { skip_note; return 1; }
  ( cd "$KENSAN_LAB_DIR" && git checkout main && git pull --ff-only && cd apps/konro && make deploy VERSION="$KONRO_VERSION" ) || return 1
  echo "  配備の反映を待つ"
  wait_for 4 600
}

run_5() {
  check_3
  [ "$STATE" = ok ] || { echo "  3 が済んでから（collector の受信口が要る）。"; return 1; }
  if [ "$KOUBOU_STATE" != MERGED ]; then
    wait_merge yu-min3/koubou $KOUBOU_PR "koubou #$KOUBOU_PR" || return 1
  fi
  echo "  $KOUBOU_DIR で main を pull し、build・token 配置・疎通確認をする（再起動はまだ）。"
  ask "  実行する？" || { skip_note; return 1; }
  ( cd "$KOUBOU_DIR" && git checkout main && git pull --ff-only && deploy/ship-agent-runtime.sh --telemetry ) || return 1
  echo "  再起動中に頼まれた job は Konro 側で待機になり、再起動後に再開できる。"
  ask "  今 runtime を再起動する？" || { skip_note; return 1; }
  ( cd "$KOUBOU_DIR" && deploy/ship-agent-runtime.sh --restart ) || return 1
  check_5; echo "  $(mark "$STATE") $DETAIL"
  [ "$STATE" = ok ]
}

run_6() {
  check_5
  [ "$STATE" = ok ] || { echo "  ${R}5 が済んでいない。今 true にすると取り込みが 400 で失敗する。${N}"; return 1; }
  local file=kubernetes/apps/app-konro/values.yaml branch=chore/konro-agent-trace-on
  echo "  $KENSAN_LAB_DIR に branch $branch を作り、$file の値を true にして PR を出す。"
  ask "  PR を作る？" || { skip_note; return 1; }
  (
    set -e
    cd "$KENSAN_LAB_DIR"
    [ -z "$(git status --porcelain)" ] || { echo "  作業ツリーが汚れている"; exit 1; }
    git checkout main && git pull --ff-only
    git checkout -b "$branch"
    perl -0pi -e 's/(name: KONRO_AGENT_TRACE\n\s+value: )"false"/$1"true"/' "$file"
    git diff --stat | grep -q values.yaml || { echo "  値が見つからない"; exit 1; }
    git commit -qam "feat(konro): agent-runtime へ traceparent を送る"
    git push -u origin "$branch"
    gh pr create --fill --base main
    git checkout main
  ) || return 1
  local url; url=$(gh pr view "$branch" --repo yu-min3/kensan-lab --json url --jq .url 2>/dev/null)
  echo "  merge してから Enter（s で後回し）  $url"
  local a; read -r a </dev/tty; [ "$a" = s ] && { skip_note; return 1; }
  echo "  反映を待つ"
  wait_for 6 600
}

run_7() {
  cat <<EOF
  1. 別の端末で:  kubectl -n langfuse port-forward svc/langfuse-web $LANGFUSE_LOCAL_PORT:3000
  2. http://localhost:$LANGFUSE_LOCAL_PORT を開く
     メール: vault kv get -field=init-user-email $LANGFUSE_PATH
     パスワード: vault kv get -field=init-user-password $LANGFUSE_PATH
  3. Konro でレシピを1件取り込む
  4. konro project の Tracing に次の形が1本で出れば完了

     konro.import
     ├─ konro.import.fetch / .extract
     ├─ konro.import.submit
     │   └─ claude_code.interaction
     │       └─ claude_code.llm_request   (token 数つき)
     └─ konro.import.apply

  Tempo 側の同じ trace に本文（langfuse.observation.input）が無いことも見る。
EOF
  pause "  確かめたら Enter "
}

run_8() {
  cat <<EOF
  port-forward（7 の 1）を開いたまま、kensan-lab の main で:

    source bootstrap/llmops/eval-env.sh
    cd apps/konro/backend
    go run ./cmd/konro-eval run -name baseline -split dev -model-label opus -out /tmp/baseline.json

  評価 client の日次枠は40件（dev は15件）。
EOF
}

run_all() {
  echo "${B}Konro LLMOps 配備${N}  各段で確認してから実行する。Ctrl-C でいつでも中断できる。"
  echo; run_prereqs
  local i
  for i in 1 2 3 4 5 6; do
    "check_$i"
    if [ "$STATE" = ok ]; then
      printf ' %s %d %s %s%s%s\n' "$(mark ok)" "$i" "${TITLES[$i]}" "$D" "$DETAIL" "$N"
      continue
    fi
    header "$i"
    printf '  いま: %s %s\n' "$(mark "$STATE")" "$DETAIL"
    if ! "run_$i"; then
      echo; show_status; return 1
    fi
  done
  header 7; run_7
  header 8; run_8
  echo; show_status
}

case "${1:-status}" in
  status) show_status ;;
  run) run_all ;;
  -h|--help|help) sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//' ;;
  *) echo "usage: $0 [status|run]" >&2; exit 2 ;;
esac
