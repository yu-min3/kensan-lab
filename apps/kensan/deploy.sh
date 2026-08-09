#!/usr/bin/env bash
# kensan 1発デプロイ — 実体は scripts/deploy-app.sh（app 共通）。
# 使い方・フローは DEPLOY.md / scripts/deploy-app.sh のヘッダ参照。
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../../scripts/deploy-app.sh" kensan "$@"
