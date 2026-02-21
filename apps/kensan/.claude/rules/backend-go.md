---
description: Go backend coding conventions
globs: backend/**/*.go
---

# Go Backend Conventions

## Layered Architecture (厳守)

```
Handler (HTTP) → Service (Business Logic) → Repository (Data Access) → PostgreSQL
```

- Handler: HTTP関心事のみ（パース、バリデーション、レスポンス）。ビジネスロジックを書かない。
- Service: ドメインロジック。HTTPやSQLの知識を持たない。
- Repository: SQLクエリのみ。ビジネスルールを書かない。

## Service Directory Structure

新サービスは必ずこの構造に従う:

```
services/<name>/
├── cmd/main.go              # bootstrap.New("<name>") で初期化
├── internal/
│   ├── model.go             # ドメイン型 + DTO
│   ├── handler/handler.go   # HTTP handlers
│   ├── service/service.go   # Business logic
│   ├── service/interface.go # Service interface (ISP)
│   ├── service/service_test.go
│   └── repository/
│       ├── interface.go     # Repository interface (ISP)
│       └── repository.go    # PostgreSQL implementation
├── Dockerfile
└── Makefile
```

## Key Patterns

- Bootstrap: `svc := bootstrap.New("service-name")` で初期化。RegisterRoutes/RegisterPublicRoutes でルート登録。
- UserID 取得: `middleware.GetUserID(r.Context())`
- Response: `middleware.JSON(w, r, status, data)`, `middleware.Error(w, r, status, code, msg)`
- Error: `shared/errors` パッケージ使用。`errors.NotFound("entity")`, `errors.IsNotFound(err)`
- Optional Update: ポインタ型で「未指定」と「nullに設定」を区別: `Name *string`
- Context: 全メソッドに `ctx context.Context` を第1引数で渡す

## Dependencies

- Router: `github.com/go-chi/chi/v5`
- DB: `github.com/jackc/pgx/v5` (pgxpool)
- UUID: `github.com/google/uuid`
- JWT: `github.com/golang-jwt/jwt/v5`
- Logging: `slog` (Go標準)
