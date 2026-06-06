# Lakehouse カタログ + クエリエンジン 構成比較

## 1. 構成パターン概要

```
Pattern A:  Dremio + Nessie        ← 今の kensan の構成
Pattern B:  Trino + Polaris
Pattern C:  Trino + Unity Catalog
```

---

## 2. ポジションマップ

```
                    カタログ層                    クエリエンジン層
                 (テーブル管理)                   (SQL 実行)
               ┌──────────────┐               ┌──────────────┐
 Pattern A:    │    Nessie    │ ────────────→ │    Dremio    │  ← UIあり
               └──────────────┘               └──────────────┘
               ┌──────────────┐               ┌──────────────┐
 Pattern B:    │   Polaris    │ ────────────→ │    Trino     │  ← CLI/API
               └──────────────┘               └──────────────┘
               ┌──────────────┐               ┌──────────────┐
 Pattern C:    │Unity Catalog │ ────────────→ │    Trino     │  ← CLI/API + UC簡易UI
               └──────────────┘               └──────────────┘
```

---

## 3. カタログ比較

| 観点 | Nessie | Polaris | Unity Catalog |
|------|--------|---------|---------------|
| 開発元 | Dremio (OSS) | Snowflake → Apache | Databricks → LF |
| ライセンス | Apache 2.0 | Apache 2.0 | Apache 2.0 |
| Iceberg REST API | ✅ | ✅ | ✅ |
| Git-like ブランチ | ✅ | - | - |
| RBAC | - | ✅ | ✅ |
| テーブル説明・タグ | - | ロードマップ | ✅ |
| AI モデル管理 | - | - | ✅ |
| UI | - | - | 簡易 |
| RAM 目安 | ~500 MB | ~500 MB〜1 GB | ~500 MB〜1 GB |

**選び方:**
- ブランチ管理が欲しい → Nessie
- ガバナンス・RBAC 重視 → Polaris
- AI/ML も含めて管理 → Unity Catalog

---

## 4. クエリエンジン比較

| 観点 | Dremio | Trino |
|------|--------|-------|
| 開発元 | Dremio Inc. | Trino Community (元 Facebook Presto) |
| ライセンス | Apache 2.0 (OSS版は機能制限) | Apache 2.0 (フルOSS) |
| 実行モデル | Arrow ベース | Java パイプライン |
| Web UI / SQL Editor | ✅ (リッチ) | - (CLI のみ、別途 Superset 等が必要) |
| Reflection (自動高速化) | ✅ | - |
| セマンティックレイヤー | ✅ (仮想データセット) | - |
| フェデレーション | ✅ (主要DB) | ✅ (コネクタ最多: 40+) |
| 生クエリ性能 | 中 (Reflection OFF 時) | 高 |
| RAM 目安 (ローカル) | 2〜4 GB | 1〜2 GB |
| RAM 目安 (本番) | 8 GB+ | 32 GB+ |

**選び方:**
- UI でポチポチ操作したい → Dremio
- 純粋なクエリ性能・OSSの自由度 → Trino

---

## 5. 構成パターン詳細比較

### 開発体験

| 観点 | A: Dremio+Nessie | B: Trino+Polaris | C: Trino+UC |
|------|:-:|:-:|:-:|
| セットアップの簡単さ | ◎ | ○ | △ |
| ブラウザで SQL 実行 | ◎ | ✕ (※1) | ✕ (※1) |
| テーブル一覧の閲覧 | ◎ | ✕ | ○ (UC UI) |
| 初心者の取っ付きやすさ | ◎ | △ | △ |

### 機能

| 観点 | A: Dremio+Nessie | B: Trino+Polaris | C: Trino+UC |
|------|:-:|:-:|:-:|
| Iceberg ネイティブ対応 | ◎ | ◎ | ◎ |
| Git-like ブランチ | ◎ | ✕ | ✕ |
| RBAC / ガバナンス | ○ | ◎ | ◎ |
| フェデレーション (多DB接続) | ○ | ◎ | ◎ |
| AI モデル管理 | ✕ | ✕ | ◎ |
| クエリ自動高速化 | ◎ | ✕ | ✕ |

### 運用

| 観点 | A: Dremio+Nessie | B: Trino+Polaris | C: Trino+UC |
|------|:-:|:-:|:-:|
| 合計 RAM (ローカル) | ~2.5 GB | ~1.5〜2.5 GB | ~1.5〜2.5 GB |
| コンテナ数 | 2 | 2 | 2 |
| OSS の透明性 | △ | ◎ | ◎ |
| コミュニティの大きさ | ○ | ○ | ◎ |
| 業界での採用実績 | ○ | ○ (急成長中) | ◎ (Databricks 圏) |

### 学習価値

| 観点 | A: Dremio+Nessie | B: Trino+Polaris | C: Trino+UC |
|------|:-:|:-:|:-:|
| 汎用スキルとして | ○ | ◎ | ◎ |
| 転職市場での需要 | ○ | ◎ (Trino) | ◎ (Databricks) |
| Iceberg エコシステム理解 | ◎ | ◎ | ○ |

> ※1: Trino で Web UI が必要な場合は Apache Superset や Redash を別途追加する

---

## 6. 今の kensan での推奨

```
普段使い:       Dremio + Nessie (今の構成のまま)
                → UI があって日常のデータ確認に便利

練習用に追加:   Polaris + Trino
                → Nessie と同じ Iceberg REST Catalog なので
                   PyIceberg の接続先を変えるだけで試せる
                → Trino は業界標準のクエリエンジン

余裕があれば:   Unity Catalog
                → Databricks エコシステムの理解
                → AI モデル管理の概念を学ぶ
```

---

## 7. 参考リンク

- [Dremio vs Trino の違い](https://ixdb.de/the-difference-between-trino-and-dremio/)
- [Apache Polaris Quick Start](https://polaris.apache.org/releases/0.9.0/quickstart/)
- [Unity Catalog Docker Compose](https://docs.unitycatalog.io/docker_compose/)
- [Trino デプロイガイド](https://trino.io/docs/current/installation/deployment.html)
- [Iceberg Catalogs 2025](https://www.e6data.com/blog/iceberg-catalogs-2025-emerging-catalogs-modern-metadata-management)
