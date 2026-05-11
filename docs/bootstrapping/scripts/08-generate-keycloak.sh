#!/bin/bash
# Keycloak Helm リポジトリ追加
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Prod 環境マニフェスト生成
helm template keycloak-prod bitnami/keycloak \
  --namespace platform-auth \
  --values ../../kubernetes/keycloak/overlays/prod/values.yaml \
  > ../../kubernetes/keycloak/overlays/prod/keycloak.yaml

# Dev 環境マニフェスト生成
helm template keycloak-dev bitnami/keycloak \
  --namespace platform-auth \
  --values ../../kubernetes/keycloak/overlays/dev/values.yaml \
  > ../../kubernetes/keycloak/overlays/dev/keycloak.yaml
