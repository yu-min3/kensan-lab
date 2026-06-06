---
description: Testing conventions for backend and frontend
globs: **/*_test.go, **/*.test.{ts,tsx}
---

# Testing Conventions

## Backend (Go)

- テストファイル: `internal/service/service_test.go`
- Repository のモック: interface を使った依存性注入
- テーブルドリブンテスト推奨
- 必ず `userID` を含むテストケース（マルチテナンシー確認）

```go
func TestServiceCreate(t *testing.T) {
    tests := []struct {
        name    string
        input   *CreateInput
        wantErr bool
    }{
        {"valid input", &CreateInput{Name: "test"}, false},
        {"empty name", &CreateInput{Name: ""}, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

## Frontend (TypeScript)

- `cd frontend && npm run build` で TypeScript 型チェック（テストスイートの代替）
- MSW ハンドラ: `frontend/src/mocks/handlers/` でモック API

## Test Commands

```bash
cd backend && make test              # Go unit tests
cd frontend && npm run build         # Frontend type check + build
```
