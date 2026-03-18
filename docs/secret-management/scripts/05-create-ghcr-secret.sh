#!/bin/bash
# This script requires manual input for secrets.
# Replace <github-username>, <PAT>, <github-email>, and <namespace> with your actual values.

# 生シークレット作成（temp/ ディレクトリに保存 - git-ignored）
kubectl create secret docker-registry ghcr-pull-secret \
  --docker-server=ghcr.io \
  --docker-username=<github-username> \
  --docker-password=<PAT> \
  --docker-email=<github-email> \
  --namespace=<namespace> \
  --dry-run=client -o yaml > ../../temp/ghcr-secret-raw.yaml

# シークレットを暗号化
kubeseal --format=yaml < ../../temp/ghcr-secret-raw.yaml \
  > ../../infrastructure/sealed-secret/ghcr-pull-secret-prod.yaml
