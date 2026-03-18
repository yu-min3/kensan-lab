# Grafana 独立デプロイ

## 概要

Grafana を kube-prometheus-stack から分離し、Observability スタック全体（Prometheus/Tempo/Loki）の統一された可視化レイヤーとして独立デプロイしています。

現在は Argo CD の Helm multi-source Application として管理されています。
- Application CR: `infrastructure/gitops/argocd/applications/observability/grafana/app.yaml`
- Helm values: `infrastructure/observability/grafana/values.yaml`
- カスタムリソース: `infrastructure/observability/grafana/resources/`

## Grafana values の主要設定

ファイル: `infrastructure/observability/grafana/values.yaml`

- `admin.existingSecret`: Sealed Secret から admin credentials を取得
- `sidecar.datasources.enabled`: ConfigMap から datasources を自動検出
- `sidecar.dashboards.enabled`: ConfigMap から dashboards を自動検出
- `sidecar.datasources.defaultDatasourceEnabled: false`: 組み込み Prometheus を無効化

## Sealed Secret 作成手順

Grafana の admin パスワードを Sealed Secret として管理しています。新規クラスターでは再作成が必要です。

```bash
# Raw secret 作成
kubectl create secret generic grafana-admin-secret \
  --namespace=monitoring \
  --from-literal=admin-user=admin \
  --from-literal=admin-password=<YOUR_PASSWORD> \
  --dry-run=client -o yaml > temp/grafana-admin-secret-raw.yaml

# Sealed Secret 化
kubeseal --controller-name=sealed-secrets \
  --controller-namespace=kube-system \
  --format=yaml \
  < temp/grafana-admin-secret-raw.yaml \
  > infrastructure/observability/grafana/resources/grafana-admin-sealed-secret.yaml

# Raw secret 削除
rm temp/grafana-admin-secret-raw.yaml
```

## OTel ダッシュボード更新

Grafana.com から公式ダッシュボードを取得してConfigMapを更新するスクリプト：

```bash
docs/bootstrapping/scripts/13-generate-grafana-dashboards.sh
```

出力先: `infrastructure/observability/grafana/resources/dashboards.yaml`

## 動作確認

### Datasources

Grafana UI → Configuration → Data Sources で以下が表示されること：
- **Prometheus** (default, exemplars 有効)
- **Tempo** (tracing)
- **Loki** (logs)

### Dashboards

Grafana UI → Dashboards → Browse で以下が表示されること：
- **OpenTelemetry** フォルダ: OTel APM, OTel for HTTP Services
- **General** フォルダ: Kubernetes 監視ダッシュボード群

### Sidecar ログ

```bash
kubectl logs -n monitoring deployment/grafana -c grafana-sc-datasources --tail=10
kubectl logs -n monitoring deployment/grafana -c grafana-sc-dashboard --tail=10
```

## トラブルシューティング

### Grafana Pod が CrashLoopBackOff

**症状**: `Only one datasource per organization can be marked as default`

**解決策**: `values.yaml` で `sidecar.datasources.defaultDatasourceEnabled: false` を設定

### Datasources が表示されない

ConfigMap に `grafana_datasource: "1"` ラベルが必要。

```bash
kubectl get configmap -n monitoring -l grafana_datasource=1
```

### Dashboards が表示されない

ConfigMap に `grafana_dashboard: "1"` ラベルが必要。

```bash
kubectl get configmap -n monitoring -l grafana_dashboard=1
```

## ファイル構成

```
infrastructure/observability/grafana/
├── values.yaml                         # Helm values
└── resources/
    ├── datasources.yaml                # Prometheus/Tempo/Loki datasource 定義
    ├── dashboards.yaml                 # OTel ダッシュボード ConfigMap
    ├── grafana-admin-sealed-secret.yaml
    └── httproute.yaml

infrastructure/observability/prometheus/
├── values.yaml                         # grafana.enabled: false
└── resources/
    ├── httproute-prometheus.yaml
    └── grafana-sealed-secret.yaml
```

## 参考リンク

- [Grafana Official Helm Chart](https://github.com/grafana/helm-charts/tree/main/charts/grafana)
- [Grafana Sidecar Documentation](https://github.com/kiwigrid/k8s-sidecar)
- [OpenTelemetry APM Dashboard](https://grafana.com/grafana/dashboards/19419)
- [OpenTelemetry for HTTP Services](https://grafana.com/grafana/dashboards/21587)
