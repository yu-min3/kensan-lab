# CLAUDE.md

Kensan - エンジニア向けパーソナル生産性アプリ（時間管理、タスク管理、学習記録、AI週次レビュー）。
React/TypeScript SPA + Go マイクロサービス + PostgreSQL 16。

---

## Development Commands

```bash
# Frontend
cd frontend && npm run dev          # Dev server (localhost:5173)
cd frontend && npm run build        # TypeScript check + production build
cd frontend && npm run lint         # ESLint

# Backend
cd backend && make build    # Build all services
cd backend && make test     # Run all tests
cd backend && make lint     # golangci-lint
cd backend && make fmt      # Format

# Docker
make up              # Start all services
make down            # Stop all services
make logs            # View logs
make dev-backend     # Backend only (for local frontend dev)
make prod-up         # Production: nginx HTTPS proxy, internal ports hidden
make deploy          # Deploy to GCE via SSH
```

---

## Architecture References

詳細は各 ARCHITECTURE.md を参照すること。コード変更後は対応するドキュメントも更新する。

| Document | Location | Content |
|----------|----------|---------|
| Backend | `backend/ARCHITECTURE.md` | Go services, API spec, DB schema, auth flow |
| Frontend | `frontend/src/ARCHITECTURE.md` | React/TS, Zustand, components, API client |
| AI Service | `kensan-ai/ARCHITECTURE.md` | Agents, tools, context, memory |
| Observability | `observability/ARCHITECTURE.md` | Monitoring setup |

---

## Services

| Service | Port | Domain |
|---------|------|--------|
| user-service | 8081 | Auth, Settings |
| task-service | 8082 | Goals, Tasks |
| timeblock-service | 8084 | Time Planning |
| analytics-service | 8088 | Analytics |
| memo-service | 8090 | Memo |
| note-service | 8091 | Notes |
| kensan-ai | 8089 | AI Chat/Review |

---

## 完了条件 (MANDATORY - 全モード共通)

タスクを「完了」と報告する前に、以下をすべて満たさなければならない。満たさずに完了を報告することは禁止。

1. **Go コード変更時**: `cd backend && make test` が全て pass している
2. **フロントエンド変更時**: `cd frontend && npm run build` が成功している
3. **構造的変更時**: 影響を受ける ARCHITECTURE.md が更新されている
4. **Plan モード時**: 計画の最終ステップにテスト実行・ドキュメント更新が含まれている

---

## Core Rules

### 全般
- `.claude/rules/` に詳細な規約あり（Go, React, DB, API, Security, Testing, Workflow）
- テスト実行とドキュメント更新は指示がなくても自動で行う（`.claude/rules/workflow.md`）

### Skills (Slash Commands)
- `/new-service <name>` - Go マイクロサービスのスキャフォルド
- `/new-page <PageName> <prefix>` - フロントエンドページ追加
- `/new-endpoint <service> <method> <path> <desc>` - API エンドポイント追加
- `/code-review` - 未コミット変更のレビュー
- `/go-test` - Go テスト実行＋自動修正
- `/build-check` - フロント＋バックエンドのビルド確認

### Test User
- Email: `test@kensan.dev` / Password: `password123` / Name: `Yu`

---

## Spec Documents

| Document | Location |
|----------|----------|
| Project Proposal | `docs/spec/kensan_proposal_v1.0.md` |
| API Specification | `docs/spec/api_specification.md` |
| ADRs | `docs/adr/` |
