---
description: Security rules for all code changes
globs:
---

# Security Rules

## Authentication (JWT)

- HS256 signing, secret は環境変数 `JWT_SECRET`
- 全 API エンドポイントは認証必須（PublicRoutes を除く）
- UserID は JWT claim から取得: `middleware.GetUserID(r.Context())`
- フロントエンドは `Authorization: Bearer <token>` を自動付与

## Data Isolation

- 全 DB クエリに `WHERE user_id = $1` (マルチテナンシー)
- 他ユーザーのデータにアクセスするクエリを絶対に書かない

## Forbidden in Code

- ハードコードされたパスワード、APIキー、トークン
- SQL 文字列結合（プレースホルダ `$1, $2` を使う）
- `eval()` やユーザー入力の直接実行
- CORS の `*` 許可（開発環境を除く）

## File Access

- `.env`, `credentials`, `secret` を含むファイルをコミットしない
- Docker build 時に secret を含めない
