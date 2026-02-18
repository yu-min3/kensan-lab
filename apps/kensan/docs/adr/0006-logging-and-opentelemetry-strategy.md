# ADR-0006: ロギングとOpenTelemetryの統合戦略

**Status**: Superseded (zerolog → slog + otelslog に移行完了)
**Date**: 2026-02-01
**Superseded Date**: 2026-02-01

---

## Context

Kensanのバックエンド（Goマイクロサービス7サービス）では、zerolog (v1.33.0) で構造化ログを出力し、OpenTelemetryでトレース・メトリクスを収集している。Observabilityバックエンドとして Grafana スタック (Loki + Tempo) を使用中。

現在のログとトレースの連携方式は「Loggerミドルウェアでzerologにtrace_id/span_idを注入」するパターン:

```
ミドルウェアチェーン: RequestID → OTelTrace → Logger → CORS → Auth

OTelTrace: otelhttp がスパンを生成し trace_id/span_id をコンテキストに設定
Logger:    zerolog がコンテキストから trace_id/span_id を読み取りログに埋め込む
```

OpenTelemetryは3シグナル (traces, metrics, logs) をすべてOTLPで統一する方向を推進しており、Go向けにもログブリッジが提供されつつある。この方向性に対して現在のアプローチを評価する必要があった。

## 検討した選択肢

### A. 現状維持: zerolog + trace_id注入

```
zerolog → ログ出力 (trace_id埋め込み) → Loki
OTel   → トレース/メトリクス → Collector → Tempo
Grafana で trace_id により相関
```

- 利点: 安定稼働中、zerologのパフォーマンスを維持、実装変更不要
- 欠点: ログとトレースが別パイプライン、OTelの推進する統一モデルとは異なる

### B. otelzerolog ブリッジ

```
zerolog → otelzerolog Hook → OTel Logs SDK → Collector → Loki
```

- 利点: zerolog をそのまま使える
- 欠点: **v0.0.0 で未リリース状態**。zerolog上流の制限 (rs/zerolog#493) により**構造化フィールドがブリッジされない**（メッセージとログレベルのみ）。実用に耐えない

### C. slog + otelslog ブリッジに移行

```
slog → otelslog Handler → OTel Logs SDK → Collector → Loki
```

- 利点: Go標準ライブラリ (1.21+)、構造化フィールドも正しくブリッジ、OTelチームの優先サポート対象
- 欠点: otelslog は v0.14.0 (2025年12月時点) でv1未満、zerolog→slogの移行コスト

### D. slog + trace_id手動注入 (ブリッジなし)

```
slog → slog.InfoContext(ctx, ...) → trace_id手動注入 → Loki
```

- 利点: OTel Logs SDKに依存しない、slogへの移行だけで完結
- 欠点: 現状のzerologと本質的に同じ方式、slog移行のメリットが限定的

## Decision

~~**Aを採用: zerolog + trace_id注入を維持する。OTel Logs Bridgeへの移行は保留。**~~

**Cを採用: slog + otelslog ブリッジに移行。** (2026-02-01 更新)

### 移行の理由

1. **OTel 3シグナル統一**: Traces, Metrics, Logs をすべて OTLP で統一し、Collector 経由で送信できる
2. **trace_id/span_id の自動注入**: otelslog ブリッジが `slog.InfoContext(ctx, ...)` からトレースコンテキストを自動で読み取る。ミドルウェアでの手動注入が不要に
3. **Go標準への統一**: zerolog (サードパーティ) → slog (Go 1.21+ 標準) への移行でメンテナンス負荷を軽減
4. **otelslog v0.x の許容**: 個人プロジェクトのフットワークの軽さを活かし、破壊的変更があれば追従する方針。v1 安定を待つよりも先に統一モデルのメリットを享受する

### 実装方式

- **logging パッケージ**: fanout ハンドラで stdout/stderr (TextHandler/JSONHandler) + otelslog の両方にログを送信
- **OTel 無効時**: stdout/stderr のみにフォールバック（fanout なし）
- **初期化順序**: `logging.Setup()` → `telemetry.Initialize()` → `logging.SetupWithOTel()` で OTel 初期化前もログ出力可能

## Consequences

- zerolog の依存関係を完全に除去
- OTel 3シグナル (Traces, Metrics, Logs) が Collector 経由で統一的に送信される
- otelslog の破壊的変更に追従する必要がある（v1 到達まで）
- slog.InfoContext/ErrorContext の使用で trace_id がログに自動的に含まれる
- mattn/go-colorable, mattn/go-isatty の間接依存も除去される
