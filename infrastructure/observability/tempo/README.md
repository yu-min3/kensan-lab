# Grafana Tempo

## 概要

Grafana Tempo は、分散トレーシングのデータストア（Trace TSDB）として、OpenTelemetry Collector から送信されるトレースデータを保存し、TraceID ベースのクエリ API を提供します。

## アーキテクチャ

```
┌──────────────────────────────────┐
│   OpenTelemetry Collector        │
│   (monitoring namespace)         │
└──────────────┬───────────────────┘
               │ OTLP (gRPC: 4317)
               ↓
┌──────────────────────────────────┐
│   Grafana Tempo StatefulSet      │
│   (monitoring namespace)         │
│                                  │
│  ┌────────────────────────────┐ │
│  │    Distributor             │ │
│  │  - Receives OTLP traces    │ │
│  └──────────┬─────────────────┘ │
│             │                    │
│  ┌──────────▼─────────────────┐ │
│  │    Ingester                │ │
│  │  - Writes to WAL           │ │
│  │  - Creates blocks          │ │
│  └──────────┬─────────────────┘ │
│             │                    │
│  ┌──────────▼─────────────────┐ │
│  │    Storage (local)         │ │
│  │  - /var/tempo/traces       │ │
│  │  - /var/tempo/wal          │ │
│  └──────────┬─────────────────┘ │
│             │                    │
│  ┌──────────▼─────────────────┐ │
│  │    Compactor               │ │
│  │  - Block retention: 7 days │ │
│  └────────────────────────────┘ │
│                                  │
│  ┌────────────────────────────┐ │
│  │    Query Frontend (3100)   │ │
│  │  - TraceID lookup          │ │
│  └────────────────────────────┘ │
└──────────────┬───────────────────┘
               │ PVC (10Gi)
               │ local-path
               ↓
        ┌──────────────┐
        │ Persistent   │
        │ Storage      │
        └──────────────┘
               ↑
               │ HTTP API (3100)
        ┌──────────────┐
        │   Grafana    │
        │  Datasource  │
        └──────────────┘
```

## データフロー

### 1. トレース受信（Distributor）
```
OTel Collector
  → OTLP gRPC (4317)
  → Tempo Distributor
  → トレーススパンを受信
```

### 2. トレース保存（Ingester）
```
Distributor
  → Ingester
  → WAL書き込み (/var/tempo/wal)
  → Block作成 (/var/tempo/traces)
```

### 3. トレース圧縮（Compactor）
```
定期的に実行
  → 古いブロックを圧縮
  → 7日以上経過したブロックを削除
```

### 4. トレースクエリ（Query Frontend）
```
Grafana
  → HTTP API (3100)
  → TraceID指定でクエリ
  → トレーススパンを返却
```

## values.yaml 設計意図

### Deployment Mode

```yaml
replicas: 1
```

**設計意図:**
- **Single Binary Mode**: Tempo を単一プロセスで実行（開発/小規模環境向け）
- **シンプルな運用**: Distributor/Ingester/Compactor/Query が1つのPodで動作
- **将来の拡張性**: トラフィック増加時は `tempo-distributed` chart に移行可能

### Image Configuration

```yaml
tempo:
  repository: grafana/tempo
  tag: "2.3.1"
```

**設計意図:**
- **安定版**: 2.3.1 は production-ready なバージョン
- **明示的なタグ指定**: latest を避けて再現性を確保

### Tempo Configuration

#### Server設定

```yaml
server:
  http_listen_port: 3100
```

**設計意図:**
- **ポート3100**: Grafana Loki と同じポート体系（統一感）
- **HTTP API**: Grafana からのクエリ、ヘルスチェックに使用

#### Distributor（OTLP Receiver）

```yaml
distributor:
  receivers:
    otlp:
      protocols:
        grpc:
          endpoint: 0.0.0.0:4317
        http:
          endpoint: 0.0.0.0:4318
```

**設計意図:**
- **OTLP gRPC (4317)**: メインプロトコル（OpenTelemetry Collector からの送信）
- **OTLP HTTP (4318)**: フォールバック用
- **0.0.0.0バインド**: Pod内の全インターフェースで受信

#### Ingester（トレース保存）

```yaml
ingester:
  trace_idle_period: 10s
  max_block_bytes: 1_000_000
  max_block_duration: 5m
```

**設計意図:**
- **trace_idle_period: 10s**: トレースが完了してから10秒後にフラッシュ
  - トレードオフ: 短いと頻繁なフラッシュ、長いとメモリ使用量増加
- **max_block_bytes: 1MB**: ブロックサイズの上限
  - 小さいブロックで早めにディスクに書き込み
- **max_block_duration: 5m**: 最長5分でブロックを作成
  - 定期的なフラッシュでメモリ使用量を抑制

#### Compactor（データ圧縮）

```yaml
compactor:
  compaction:
    block_retention: 168h  # 7 days
```

**設計意図:**
- **168h = 7日間**: トレースデータの保持期間
  - 直近のトラブルシューティングには十分
  - ストレージコストを抑制
- **自動削除**: 7日以上経過したブロックは自動的に削除

#### Storage（ストレージバックエンド）

```yaml
storage:
  trace:
    backend: local
    local:
      path: /var/tempo/traces
    wal:
      path: /var/tempo/wal
    pool:
      max_workers: 100
      queue_depth: 10000
```

**設計意図:**
- **backend: local**: ローカルファイルシステムを使用
  - シンプルで運用が楽
  - PVCで永続化
  - 本番環境では S3/GCS/Azure Blob に変更可能
- **traces path**: 圧縮済みブロックの保存先
- **wal path**: Write-Ahead Log（WAL）の保存先
  - クラッシュ時のデータ復旧に使用
- **pool設定**:
  - max_workers: 100 - 同時書き込みワーカー数
  - queue_depth: 10000 - 書き込みキューの深さ

### Persistence Configuration

```yaml
persistence:
  enabled: true
  accessModes:
    - ReadWriteOnce
  size: 10Gi
  storageClassName: local-path
  enableStatefulSetAutoDeletePVC: false
```

**設計意図:**
- **enabled: true**: 永続化を有効化（トレースデータを保持）
- **ReadWriteOnce**: StatefulSet の単一 Pod で使用
- **size: 10Gi**: 7日間のトレースデータを保存
  - 推定: 1日あたり約1.4GBのストレージ使用
  - 余裕を持たせた容量設定
- **storageClassName: local-path**: ローカルストレージプロビジョナーを使用
  - Bare-metal環境で動的PVプロビジョニング
- **enableStatefulSetAutoDeletePVC: false**: StatefulSet削除時もPVCを保持
  - データ保護のため、手動削除を強制

### Service Configuration

```yaml
service:
  type: ClusterIP
  port: 3100
```

**設計意図:**
- **ClusterIP**: クラスタ内通信のみ
  - OpenTelemetry Collector からの OTLP 送信
  - Grafana からのクエリ
- **port: 3100**: HTTP API のメインポート
  - `/ready`: ヘルスチェック
  - `/api/traces/{traceID}`: トレースクエリ

### Resources Configuration

```yaml
resources:
  requests:
    cpu: 200m
    memory: 512Mi
  limits:
    cpu: 500m
    memory: 1Gi
```

**設計意図:**
- **requests**: 保証リソース
  - cpu: 200m - 通常の負荷で十分
  - memory: 512Mi - WAL + ブロックバッファ用
- **limits**: 上限
  - cpu: 500m - スパイク時の対応
  - memory: 1Gi - OOM防止
- **メモリ計算**:
  - Ingester バッファ: ~300MB
  - WAL: ~100MB
  - その他（Compactor等）: ~100MB
  - 合計: ~500MB（余裕を持って512Mi）

### Pod Labels

```yaml
podLabels:
  app.kubernetes.io/name: tempo
  app.kubernetes.io/component: observability
```

**設計意図:**
- **標準ラベル**: Kubernetes の推奨ラベルスキーマに準拠
- **component: observability**: 可観測性スタックの一部として識別

## アーキテクチャ設計の原則

### 1. Single Binary Mode の採用理由

**現在のトラフィック規模:**
- アプリケーション数: 少数（開発環境）
- トレース量: 1日あたり数千〜数万スパン
- クエリ頻度: 低頻度（トラブルシューティング時のみ）

**Single Binary Mode のメリット:**
- シンプルな運用（1つのPodのみ）
- リソース効率が良い（プロセス間通信が不要）
- デプロイが簡単

**Distributed Mode への移行タイミング:**
- トレース量が1日あたり100万スパンを超える
- クエリレイテンシが問題になる
- 高可用性が必要になる

### 2. ローカルストレージの使用

**現在の選択:**
- `backend: local` + PVC

**メリット:**
- シンプルな設定
- 追加のクラウドサービス不要
- コスト削減

**デメリット:**
- スケーラビリティに限界
- バックアップが手動

**本番環境への移行:**
```yaml
storage:
  trace:
    backend: s3  # or gcs, azure
    s3:
      bucket: tempo-traces
      endpoint: s3.amazonaws.com
```

### 3. 7日間の保持期間

**選定理由:**
- **トラブルシューティング**: 直近のトレースで十分
- **ストレージコスト**: 長期保存は不要
- **コンプライアンス**: 特に規制要件なし

**変更方法:**
```yaml
compactor:
  compaction:
    block_retention: 336h  # 14 days
```

## セキュリティ考慮事項

### 1. クラスタ内通信のみ

- Service type: ClusterIP（外部公開なし）
- OpenTelemetry Collector と Grafana のみアクセス可能

### 2. PVC の保護

- `enableStatefulSetAutoDeletePVC: false`: 誤削除防止
- 手動でのPVC削除を強制

### 3. リソース制限

- メモリ制限（1Gi）でOOMを防止
- CPU制限（500m）でノードリソースを保護

## 運用考慮事項

### 1. ストレージ監視

重要なメトリクス:
- PVC使用率: `kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes`
- 残り容量が20%を切ったらアラート

### 2. ブロックサイズ監視

```bash
# Tempo Pod内でブロック数を確認
kubectl exec -n monitoring tempo-0 -- ls -lh /var/tempo/traces
```

### 3. クエリパフォーマンス

- TraceID クエリのレイテンシ監視
- P95が1秒を超えたらアラート

### 4. バックアップ戦略

**現状:**
- PVC のスナップショット機能（CSIドライバー依存）

**推奨:**
```bash
# 定期的なバックアップ
kubectl exec -n monitoring tempo-0 -- tar czf /tmp/tempo-backup.tar.gz /var/tempo/traces
kubectl cp monitoring/tempo-0:/tmp/tempo-backup.tar.gz ./tempo-backup-$(date +%Y%m%d).tar.gz
```

## トラブルシューティング

### Problem 1: Pod が Pending

**症状:**
```bash
kubectl get pods -n monitoring
# NAME      READY   STATUS    RESTARTS   AGE
# tempo-0   0/1     Pending   0          5m
```

**原因と対処:**

1. **PVC がバインドされない**
   ```bash
   kubectl get pvc -n monitoring
   kubectl describe pvc tempo-storage-tempo-0 -n monitoring

   # local-path-provisioner の確認
   kubectl get pods -n local-path-storage
   kubectl logs -n local-path-storage -l app=local-path-provisioner
   ```

2. **ノードのリソース不足**
   ```bash
   kubectl describe node
   # Check: Allocated resources
   ```

### Problem 2: OTLP受信が失敗

**症状:**
```bash
kubectl logs -n monitoring -l app.kubernetes.io/name=otel-collector | grep tempo
# Error: connection refused
```

**対処:**

1. **Service確認**
   ```bash
   kubectl get svc tempo -n monitoring
   kubectl get endpoints tempo -n monitoring
   ```

2. **Pod確認**
   ```bash
   kubectl logs -n monitoring tempo-0 | grep -i otlp
   # "OTLP gRPC receiver started on :4317" が表示されるべき
   ```

### Problem 3: クエリが遅い

**症状:**
Grafana でのトレース検索に時間がかかる

**対処:**

1. **ブロック数確認**
   ```bash
   kubectl exec -n monitoring tempo-0 -- find /var/tempo/traces -name "*.tar.gz" | wc -l
   # ブロック数が多すぎる場合はcompaction設定を見直し
   ```

2. **メモリ使用量確認**
   ```bash
   kubectl top pod tempo-0 -n monitoring
   # メモリが限界に近い場合はlimitsを増やす
   ```

## 今後の拡張予定

1. **Distributed Mode への移行**: トラフィック増加時
2. **S3バックエンド**: 長期保存・スケーラビリティ向上
3. **TraceQL サポート**: 高度なクエリ機能
4. **Grafana Datasource 自動設定**: Terraform/Helm で自動化

## 参考リンク

- [Grafana Tempo 公式ドキュメント](https://grafana.com/docs/tempo/latest/)
- [Tempo Helm Chart](https://github.com/grafana/helm-charts/tree/main/charts/tempo)
- [OTLP Ingestion](https://grafana.com/docs/tempo/latest/configuration/network/otlp/)
- [Storage Configuration](https://grafana.com/docs/tempo/latest/configuration/storage/)
