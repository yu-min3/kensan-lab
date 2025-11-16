#!/bin/bash
# ランダムパスワード生成
GRAFANA_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
echo "Generated Grafana admin password: $GRAFANA_PASSWORD"

# 生 Secret 作成（temp/ ディレクトリに保存 - git-ignored）
kubectl create secret generic prometheus-grafana \
  --from-literal=admin-user=admin \
  --from-literal=admin-password="$GRAFANA_PASSWORD" \
  --namespace=monitoring \
  --dry-run=client -o yaml > ../../temp/grafana-secret-raw.yaml

# シークレットを暗号化
kubeseal --format=yaml \
  --controller-name=sealed-secrets \
  --controller-namespace=kube-system \
  < ../../temp/grafana-secret-raw.yaml \
  > ../../base-infra/prometheus/grafana-sealed-secret.yaml
