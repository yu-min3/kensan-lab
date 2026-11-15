# Environment for konro-eval against production. Source it; nothing is printed.
#
#   kubectl -n langfuse port-forward svc/langfuse-web 3300:3000   # another terminal
#   source bootstrap/llmops/eval-env.sh
#
# Needs: vault login, kubectl, and the eval client from
# bootstrap/keycloak/setup-agent-service.py --client konro-eval-agent-runtime.

_lf_port=${LANGFUSE_LOCAL_PORT:-3300}
_ctx=${KUBE_CONTEXT:-kubernetes-admin@kubernetes}
_ca_dir=$HOME/.config/konro-eval

_konro_eval_env() {
  vault token lookup >/dev/null 2>&1 || { echo "vault login が必要" >&2; return 1; }
  [ -s "$_ca_dir/client-secret" ] || { echo "$_ca_dir/client-secret が無い（setup.sh の 2）" >&2; return 1; }
  kubectl --kubeconfig "$HOME/.kube/config" --context "$_ctx" -n app-konro get cm konro-agent-runtime-ca \
    -o jsonpath='{.data.ca\.crt}' >"$_ca_dir/agent-ca.crt" || return 1

  export KONRO_AGENT_URL=https://192.168.0.113:8791
  export KONRO_AGENT_CA_FILE=$_ca_dir/agent-ca.crt
  export KONRO_AGENT_TOKEN_URL=https://auth.yu-mins.com/realms/kensan/protocol/openid-connect/token
  export KONRO_AGENT_CLIENT_ID=konro-eval-agent-runtime
  export KONRO_AGENT_CLIENT_SECRET_FILE=$_ca_dir/client-secret
  export KONRO_AGENT_TRACE=true
  export LANGFUSE_HOST=http://localhost:$_lf_port
  LANGFUSE_PUBLIC_KEY=$(vault kv get -field=konro-public-key secret/observability/langfuse) || return 1
  LANGFUSE_SECRET_KEY=$(vault kv get -field=konro-secret-key secret/observability/langfuse) || return 1
  export LANGFUSE_PUBLIC_KEY LANGFUSE_SECRET_KEY
  export OTEL_EXPORTER_OTLP_ENDPOINT=$LANGFUSE_HOST/api/public/otel
  export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Basic%20$(printf '%s:%s' "$LANGFUSE_PUBLIC_KEY" "$LANGFUSE_SECRET_KEY" | base64 | tr -d '\n'),x-langfuse-ingestion-version=4"

  if curl -fsS -o /dev/null "$LANGFUSE_HOST/api/public/health"; then
    echo "konro-eval の環境を設定した（Langfuse $LANGFUSE_HOST に到達）"
  else
    echo "環境は設定したが $LANGFUSE_HOST に届かない。port-forward を開く:" >&2
    echo "  kubectl -n langfuse port-forward svc/langfuse-web $_lf_port:3000" >&2
  fi
}
_konro_eval_env
unset -f _konro_eval_env
unset _lf_port _ctx _ca_dir
