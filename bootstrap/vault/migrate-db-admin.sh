#!/usr/bin/env bash
# Migration: 既存の Postgres admin cred を Vault KV の convention path に複製する
#
# 旧 path (Stage 3/3.5 で投入された static admin cred):
#   secret/backstage/postgresql              POSTGRES_USER       POSTGRES_PASSWORD
#   secret/kensan/data/db                    POSTGRES_USER       POSTGRES_PASSWORD
#   secret/kensan/data/lakehouse             DAGSTER_PG_USER     DAGSTER_PG_PASSWORD
#   secret/platform-auth/prod/postgresql     POSTGRES_USER       POSTGRES_PASSWORD
#   secret/platform-auth/dev/postgresql      POSTGRES_USER       POSTGRES_PASSWORD
#
# 新 convention path (Stage 5a の Vault Database engine が読む):
#   secret/db-admin/postgres-backstage           username  password
#   secret/db-admin/postgres-kensan-app          username  password
#   secret/db-admin/postgres-kensan-dagster      username  password
#   secret/db-admin/postgres-keycloak-prod       username  password
#   secret/db-admin/postgres-keycloak-dev        username  password
#
# 旧 path は ESO ExternalSecret consumer (各 app の static cred) のため残置。
# Phase 5b 以降で app が dynamic cred に切り替わったら旧 path を削除する。
#
# 一度だけ走らせる。idempotent (再実行しても害なし、kv put の version が増えるだけ)。
#
# Prerequisite:
#   port-forward して Vault root token export 済 (詳細は bootstrap/vault/README.md)
#     kubectl -n vault port-forward svc/vault-active 8200:8200 &
#     export VAULT_ADDR=http://127.0.0.1:8200
#     export VAULT_TOKEN=<root token>

set -euo pipefail

if [[ -z "${VAULT_ADDR:-}" || -z "${VAULT_TOKEN:-}" ]]; then
  echo "ERROR: VAULT_ADDR + VAULT_TOKEN must be set" >&2
  exit 1
fi

migrate() {
  local name=$1 src_path=$2 src_user_key=$3 src_pw_key=$4
  local user pw

  user=$(vault kv get -field="$src_user_key" "$src_path")
  pw=$(vault kv get -field="$src_pw_key" "$src_path")

  vault kv put "secret/db-admin/${name}" username="$user" password="$pw" >/dev/null
  echo "  OK: secret/db-admin/${name} (from ${src_path})"
}

echo "Migrating Postgres admin credentials to convention path 'secret/db-admin/<name>'..."

migrate postgres-backstage         secret/backstage/postgresql            POSTGRES_USER     POSTGRES_PASSWORD
migrate postgres-kensan-app        secret/kensan/data/db                  POSTGRES_USER     POSTGRES_PASSWORD
migrate postgres-kensan-dagster    secret/kensan/data/lakehouse           DAGSTER_PG_USER   DAGSTER_PG_PASSWORD
migrate postgres-keycloak-prod     secret/platform-auth/prod/postgresql   POSTGRES_USER     POSTGRES_PASSWORD
migrate postgres-keycloak-dev      secret/platform-auth/dev/postgresql    POSTGRES_USER     POSTGRES_PASSWORD

echo ""
echo "Done. New paths in Vault KV:"
vault kv list secret/db-admin
