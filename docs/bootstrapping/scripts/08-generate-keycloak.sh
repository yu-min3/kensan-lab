#!/bin/bash
# Keycloak Helm リポジトリ追加
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Prod 環境マニフェスト生成
helm template keycloak-prod bitnami/keycloak \
  --namespace platform-auth \
  --values ../../base-infra/keycloak/overlays/prod/values.yaml \
  > ../../base-infra/keycloak/overlays/prod/keycloak.yaml

# Dev 環境マニフェスト生成
helm template keycloak-dev bitnami/keycloak \
  --namespace platform-auth \
  --values ../../base-infra/keycloak/overlays/dev/values.yaml \
  > ../../base-infra/keycloak/overlays/dev/keycloak.yaml
