# セクション11: Grafana独立デプロイ

## 概要

Grafana を kube-prometheus-stack から分離し、Observability スタック全体（Prometheus/Tempo/Loki）の統一された可視化レイヤーとして独立デプロイします。

## 前提条件

- セクション7: Prometheus/Grafana デプロイ完了
- セクション8: Tempo デプロイ完了
- セクション9: Loki デプロイ完了
- セクション10: OTel Collector デプロイ完了

## 1. Admin Password取得

既存の Prometheus-Grafana から admin password を取得：

```bash
kubectl get secret prometheus-grafana -n monitoring \
  -o jsonpath="{.data.admin-password}" | base64 -d
```

## 2. Sealed Secret作成

```bash
# Raw secret作成
kubectl create secret generic grafana-admin-secret \
  --namespace=monitoring \
  --from-literal=admin-user=admin \
  --from-literal=admin-password=<PASSWORD_FROM_STEP1> \
  --dry-run=client -o yaml > temp/grafana-admin-secret-raw.yaml

# Sealed Secret化
kubeseal --controller-name=sealed-secrets \
  --controller-namespace=kube-system \
  --format=yaml \
  < temp/grafana-admin-secret-raw.yaml \
  > infrastructure/observability/grafana/grafana-admin-sealed-secret.yaml

# Raw secret削除（機密情報）
rm temp/grafana-admin-secret-raw.yaml
```

## 3. Grafana values作成

ファイル: `infrastructure/observability/grafana/values.yaml` は既に作成済み

重要な設定項目:
- `admin.existingSecret`: Sealed Secretから admin credentials取得
- `sidecar.datasources.enabled`: ConfigMapから datasources自動検出
- `sidecar.dashboards.enabled`: ConfigMapから dashboards自動検出
- `sidecar.datasources.defaultDatasourceEnabled: false`: 組み込みPrometheusを無効化

## 4. Grafana manifest生成

```bash
# Helmリポジトリ追加
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update grafana

# Manifest生成
helm template grafana grafana/grafana \
  --version 7.3.0 \
  --namespace monitoring \
  --values infrastructure/observability/grafana/values.yaml \
  > infrastructure/observability/grafana/grafana-manifests.yaml 2>/dev/null

echo "✓ Grafana manifest generated ($(wc -l < infrastructure/observability/grafana/grafana-manifests.yaml) lines)"
```

## 5. OTel Dashboards生成

Grafana.comから公式ダッシュボードをダウンロード：

```bash
docs/bootstrapping/scripts/13-generate-grafana-dashboards.sh
```

**処理内容**:
1. Grafana.com APIからダッシュボードJSON取得
2. Datasource UID を置換 (`${DS_PROMETHEUS}` → `prometheus` 等)
3. ConfigMapとして `infrastructure/observability/grafana/dashboards.yaml` に保存

**出力**: 2個のOTelダッシュボード
- OpenTelemetry APM (ID: 19419)
- OpenTelemetry for HTTP Services (ID: 21587)

## 6. Prometheus側のGrafana無効化

`infrastructure/observability/prometheus/prometheus-values.yaml` を更新：

```yaml
grafana:
  enabled: false  # Grafana を独立デプロイに移行
```

Prometheus manifest再生成:

```bash
cd infrastructure/observability/prometheus

helm template prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --include-crds \
  --values prometheus-values.yaml \
  > prometheus-stack.yaml 2>/dev/null

echo "✓ Prometheus manifest regenerated without Grafana"

# Grafana関連リソースが削除されたことを確認
grep -c "grafana" prometheus-stack.yaml || echo "✓ No Grafana resources found"
```

## 6.5. Prometheusダッシュボード ConfigMaps作成

**背景**: `grafana.enabled: false` の場合、Helm chartはダッシュボード ConfigMapsを生成しません。

**解決策**: スクリプトで自動抽出

```bash
docs/bootstrapping/scripts/14-extract-prometheus-dashboards.sh
```

**処理内容**:
1. 一時的に `grafana.enabled: true` でHelm manifest生成
2. `grafana_dashboard: "1"` ラベルを持つConfigMapsを抽出
3. `infrastructure/observability/prometheus/dashboards-configmaps.yaml` に保存
4. 一時ファイルをクリーンアップ

**出力**: 28個のダッシュボード ConfigMaps (401KB)
- API Server, Controller Manager, etcd, Kubelet
- Nodes, Pods, Workloads, Persistent Volumes
- Networking, Scheduler, Prometheus自体の監視
- その他Kubernetes監視用ダッシュボード


## 7. デプロイ

```bash
# 1. Sealed Secret適用
kubectl apply -f infrastructure/observability/grafana/grafana-admin-sealed-secret.yaml

# 2. Datasources ConfigMap適用（Issue #90で作成済み）
kubectl apply -f infrastructure/observability/grafana/datasources.yaml

# 3. OTel Dashboards ConfigMap適用
kubectl apply -f infrastructure/observability/grafana/dashboards.yaml

# 4. Prometheus Dashboards ConfigMaps適用（28個）
kubectl apply -f infrastructure/observability/prometheus/dashboards-configmaps.yaml

# 5. Grafana manifest適用
kubectl apply -f infrastructure/observability/grafana/grafana-manifests.yaml

# 6. Pod起動確認
kubectl get pods -n monitoring -l app.kubernetes.io/name=grafana

# 出力例:
# NAME                       READY   STATUS    RESTARTS   AGE
# grafana-845c4575f8-klgw2   3/3     Running   0          2m

# 7. Dashboard ConfigMaps確認
kubectl get configmap -n monitoring -l grafana_dashboard=1

# 出力例:
# NAME                                                  DATA   AGE
# grafana-otel-dashboards                               2      4h
# prometheus-kube-prometheus-alertmanager-overview      1      10m
# prometheus-kube-prometheus-apiserver                  1      10m
# ... (合計30個)
```

## 8. 動作確認

### Grafana UI アクセス

```bash
# Port-forward
kubectl port-forward -n monitoring svc/grafana 3000:80

# ブラウザでアクセス
# http://localhost:3000
# User: admin
# Password: <STEP 1のパスワード>
```

### Datasources確認

Grafana UI で:
1. Configuration → Data Sources
2. 以下が表示されることを確認:
   - ✅ **Prometheus** (default, exemplars有効)
   - ✅ **Tempo** (tracing)
   - ✅ **Loki** (logs)

### Dashboards確認

Grafana UI で:
1. Dashboards → Browse
2. **OpenTelemetry** フォルダ内に以下が表示されることを確認:
   - ✅ OpenTelemetry APM
   - ✅ OpenTelemetry for HTTP Services
3. **General** フォルダ内に Prometheus関連ダッシュボード（28個）:
   - ✅ Kubernetes / API Server
   - ✅ Kubernetes / Compute Resources / Cluster
   - ✅ Kubernetes / Compute Resources / Namespace (Pods)
   - ✅ Kubernetes / Kubelet
   - ✅ Kubernetes / Networking / Namespace
   - ✅ Node Exporter / Nodes
   - ✅ Prometheus
   - その他多数

### Sidecar ログ確認

```bash
# Datasources sidecar
kubectl logs -n monitoring deployment/grafana -c grafana-sc-datasources --tail=20

# 期待される出力:
# {"time": "...", "msg": "Writing /etc/grafana/provisioning/datasources/datasource.yaml (ascii)", "level": "INFO"}

# Dashboards sidecar
kubectl logs -n monitoring deployment/grafana -c grafana-sc-dashboard --tail=20

# 期待される出力:
# {"time": "...", "msg": "Writing /tmp/dashboards/otel-apm-dashboard.json (ascii)", "level": "INFO"}
# {"time": "...", "msg": "Writing /tmp/dashboards/otel-http-services-dashboard.json (ascii)", "level": "INFO"}
```

## 9. HTTPRoute更新

**重要**: Grafana HTTPRouteが旧サービスを参照しているため更新が必要です。

```bash
# HTTPRouteファイルを確認
kubectl get httproute grafana-route -n monitoring

# サービス名を更新: prometheus-grafana → grafana
# ファイル: infrastructure/observability/grafana/httproute.yaml
kubectl apply -f infrastructure/observability/grafana/httproute.yaml

# HTTPRoute正常性確認
kubectl describe httproute grafana-route -n monitoring | grep -A 5 "Conditions"

# 期待される出力:
# Status: Accepted = True
# Status: ResolvedRefs = True
```

## 10. 旧Grafanaの削除

**注意**: 新Grafanaの動作確認が完了してから実行してください。

```bash
# Git commit & push (Argo CD が自動的に旧Grafanaを削除)
git add infrastructure/observability/prometheus/prometheus-values.yaml
git add infrastructure/observability/prometheus/prometheus-stack.yaml
git add infrastructure/observability/prometheus/dashboards-configmaps.yaml
git add infrastructure/observability/grafana/
git commit -m "Separate Grafana from Prometheus stack with OTel dashboards"
git push

# Argo CDで sync後、旧Grafana削除を確認
kubectl get deployment prometheus-grafana -n monitoring
# 出力: Error: deployments.apps "prometheus-grafana" not found (正常)
```

## 11. Argo CD Application CR作成（任意）

独立Grafanaを Argo CD で管理する場合:

ファイル: `infrastructure/gitops/argocd/applications/grafana-app.yaml`

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: grafana
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
  labels:
    app.kubernetes.io/name: grafana
    app.kubernetes.io/component: observability
spec:
  project: platform-project

  source:
    repoURL: https://github.com/yu-min3/goldship-yumin3
    targetRevision: HEAD
    path: infrastructure/observability/grafana

  destination:
    server: https://kubernetes.default.svc
    namespace: monitoring

  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=false
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m
```

## トラブルシューティング

### Grafana Pod が CrashLoopBackOff

**症状**:
```
Error: ✗ Datasource provisioning error: datasource.yaml config is invalid. Only one datasource per organization can be marked as default
```

**原因**: 複数のdatasourceが `isDefault: true` になっている

**解決策**: `values.yaml` で `sidecar.datasources.defaultDatasourceEnabled: false` を設定

### Datasources が表示されない

**確認**:
```bash
# ConfigMap のラベル確認
kubectl get configmap grafana-otel-datasources -n monitoring -o yaml | grep -A 2 labels

# Sidecar ログ確認
kubectl logs -n monitoring deployment/grafana -c grafana-sc-datasources
```

**修正**: ラベル `grafana_datasource: "1"` が必須

### Dashboards が表示されない

**確認**:
```bash
# ConfigMap確認
kubectl get configmap grafana-otel-dashboards -n monitoring

# Sidecar ログ確認
kubectl logs -n monitoring deployment/grafana -c grafana-sc-dashboard
```

**修正**: ラベル `grafana_dashboard: "1"` とフォルダアノテーション `grafana_folder: "OpenTelemetry"` が必須

## 作成されたファイル

```
infrastructure/observability/grafana/
├── datasources.yaml                    # Issue #90で作成
├── dashboards.yaml                     # OTelダッシュボード (2個)
├── values.yaml                         # Grafana Helm values
├── grafana-manifests.yaml              # Helm生成manifest (494行)
├── grafana-admin-sealed-secret.yaml    # Admin password
└── httproute.yaml                      # HTTPRoute (prometheus/から移動)

infrastructure/observability/prometheus/
├── prometheus-values.yaml              # 更新: grafana.enabled: false
├── prometheus-stack.yaml               # 再生成: Grafana除外
└── dashboards-configmaps.yaml          # Prometheusダッシュボード (28個, 401KB)

docs/bootstrapping/scripts/
├── 13-generate-grafana-dashboards.sh   # OTelダッシュボード生成スクリプト
└── 14-extract-prometheus-dashboards.sh # Prometheusダッシュボード抽出スクリプト
```

## 次のステップ

- セクション12: サンプルアプリケーションデプロイ（OTel計装）
- OTel ダッシュボードの動作確認（traces/logs/metricsの相関）
- カスタムダッシュボードの作成

## 参考リンク

- [Grafana Official Helm Chart](https://github.com/grafana/helm-charts/tree/main/charts/grafana)
- [Grafana Sidecar Documentation](https://github.com/kiwigrid/k8s-sidecar)
- [OpenTelemetry APM Dashboard](https://grafana.com/grafana/dashboards/19419)
- [OpenTelemetry for HTTP Services](https://grafana.com/grafana/dashboards/21587)
