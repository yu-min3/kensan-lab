# ADR-0003: サードパーティAPIキーの保存方法

**ステータス**: Deprecated (Clockify連携を廃止したため本ADRは無効)
**日付**: 2026-01-21
**決定者**: Yu
**廃止日**: 2026-02-01
**廃止理由**: Clockify連携を完全に廃止。DBカラム・テーブルはマイグレーション `009_cleanup_legacy.sql` で削除済み。タイマー機能は自前実装 (running_timers) に移行。

---

## コンテキスト

KensanはClockifyと連携して時間記録を同期する機能を持つ。この連携にはClockifyのAPIキーが必要であり、ユーザーが入力したAPIキーをどのように保存・管理するかを決定する必要がある。

### 一般的なアプローチ

#### 1. OAuth 2.0（業界標準）

ほとんどの商用サービスはOAuthを使用する。

```
[ユーザー] → [連携ボタン] → [Clockify認証画面] → [アクセストークン取得]
```

**メリット**:
- ユーザーがAPIキーを入力する必要がない
- アクセストークンは自動更新可能
- いつでも権限を取り消せる
- セキュリティリスクが低い

**デメリット**:
- OAuth認証フローの実装が必要
- コールバックURL、クライアントID/Secretの管理が必要

#### 2. APIキーの暗号化保存

```
[ユーザー入力] → [アプリ暗号化] → [DB保存（BYTEA）]
                      ↓
              マスターキー（環境変数 or Secrets Manager）
```

**実装オプション**:
- **シンプル**: AES暗号化 + 環境変数のマスターキー（pgcrypto拡張）
- **本格的**: AWS KMS / HashiCorp Vault / Google Secret Manager

**メリット**:
- 実装がOAuthより単純
- 外部サービスへの依存が少ない

**デメリット**:
- マスターキーの管理が必要
- キー漏洩時の影響が大きい

#### 3. 平文保存（開発用）

```
[ユーザー入力] → [DB保存（TEXT）]
```

**メリット**:
- 実装が最も単純
- 開発・デバッグが容易

**デメリット**:
- セキュリティリスクが高い
- 本番環境には不適切

## 決定

### 現段階（開発フェーズ）

**平文でAPIキーを保存する。**

DBカラムを `clockify_api_key_encrypted (BYTEA)` から `clockify_api_key (TEXT)` に変更し、暗号化なしで保存する。

### 将来（本番リリース前）

**ClockifyのOAuth 2.0を実装する。**

Clockifyは OAuth 2.0 をサポートしているため、本番環境ではOAuthフローを採用する。

```
/auth/clockify/connect    → Clockify認証画面へリダイレクト
/auth/clockify/callback   → アクセストークン取得・保存
```

## 理由

### 1. 開発速度の優先

- 現段階はモックアップ・プロトタイプフェーズ
- 暗号化やOAuthの実装に時間をかけるより、機能検証を優先

### 2. 個人利用の前提

- 現時点では開発者本人のみが使用
- ローカル環境または限定的な環境での運用

### 3. 将来のOAuth移行

- APIキー方式は一時的な措置
- 本番リリース前にOAuth実装に移行する計画

## 結果

### 現段階での変更

1. `user_settings.clockify_api_key_encrypted (BYTEA)` → `clockify_api_key (TEXT)` に変更
2. user-service, sync-service のrepositoryを平文保存に対応

### 将来の作業（TODO）

1. ClockifyのOAuthアプリケーション登録
2. OAuth認証フロー実装（`/auth/clockify/*` エンドポイント）
3. アクセストークンのリフレッシュ処理
4. APIキーカラムの削除

## 参考

- [Clockify API - OAuth 2.0](https://clockify.me/developers-api#section/Authentication/OAuth-2.0)
- [OWASP - Cryptographic Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cryptographic_Storage_Cheat_Sheet.html)
