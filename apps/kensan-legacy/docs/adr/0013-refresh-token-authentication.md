# ADR-0013: Refresh Token による認証改善

## Status

Proposed (将来要件)

## Context

現在の認証方式は単一の JWT アクセストークン (有効期限 30 日) のみで運用している。
暫定措置として有効期限を 24h → 30 日に延長したが、以下の課題が残る:

- トークン漏洩時に30日間有効なため、被害が拡大するリスク
- サーバー側でトークンを無効化する手段がない (ステートレス JWT)
- セッション管理の粒度が粗い

## Decision

将来的に **Access Token + Refresh Token** の2トークン方式へ移行する。

### 設計方針

| 項目 | Access Token | Refresh Token |
|------|-------------|---------------|
| 寿命 | 15分〜1時間 | 30日 |
| 保存場所 (Frontend) | メモリ (JS変数) | httpOnly Cookie |
| 用途 | API 認証 | Access Token の再発行 |
| 漏洩時の影響 | 短寿命で限定的 | httpOnly で XSS 耐性あり |

### 必要な変更

**Backend:**
1. `refresh_tokens` テーブルを追加 (token_hash, user_id, expires_at, revoked_at)
2. `POST /auth/refresh` エンドポイントを追加
3. ログイン時に Refresh Token を httpOnly Cookie で返却
4. トークンローテーション: refresh のたびに旧トークンを無効化し新トークンを発行
5. `POST /auth/logout` でサーバー側トークン無効化

**Frontend:**
- httpClient に 401 → 自動 refresh → リトライのインターセプター追加
- Access Token をメモリ保持に変更 (localStorage から移行)
- Refresh Token は Cookie で自動送信 (JS から触らない)

### フロー

```
Login → access_token (メモリ) + refresh_token (httpOnly cookie)
  ↓
API call (Authorization: Bearer <access_token>)
  ↓
401 Expired → POST /auth/refresh (cookie自動送信)
  ↓
新 access_token + 新 refresh_token → 元のリクエストをリトライ
```

## Consequences

**メリット:**
- アクセストークン漏洩の影響を最小化 (15分で失効)
- ユーザーは30日間再ログイン不要 (使い続けている限り)
- サーバー側でセッション無効化が可能

**デメリット:**
- DB への refresh_token 保存が必要 (ステートフル化)
- 実装量が多い (バックエンド + フロントエンド)
- 同時リクエスト時の refresh 競合を考慮する必要あり

## 現状の暫定措置

- `JWT_EXPIRE_HOUR` のデフォルトを 24 → 720 (30日) に変更
- 個人利用アプリのため、当面はこれで運用
