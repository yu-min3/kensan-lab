#!/bin/bash
# This script should be run from the backstage-app directory.
# cd backstage-app

# 依存関係インストール（2GB+ メモリ必要）
export NODE_OPTIONS="--max-old-space-size=4096"
make install

# Docker イメージビルド
make build TAG=v1.0.0

# GHCR にプッシュ
make push TAG=v1.0.0

# Kubernetes にデプロイ
kubectl apply -f ../base-infra/backstage/
