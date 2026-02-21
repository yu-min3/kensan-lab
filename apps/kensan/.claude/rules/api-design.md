---
description: API design conventions for backend handlers and frontend services
globs: backend/**/handler/*.go, src/api/**
---

# API Design Conventions

## Response Envelope (厳守)

Success:
```json
{
  "data": { ... },
  "meta": { "requestId": "uuid", "timestamp": "ISO8601" },
  "pagination": { "page": 1, "perPage": 20, "total": 100 }
}
```

Error:
```json
{
  "error": { "code": "ERROR_CODE", "message": "Human readable", "details": [] },
  "meta": { "requestId": "...", "timestamp": "..." }
}
```

## Response Helpers (Go)

```go
middleware.JSON(w, r, http.StatusOK, data)
middleware.JSONWithPagination(w, r, status, data, pagination)
middleware.Error(w, r, http.StatusNotFound, "NOT_FOUND", "message")
middleware.ValidationError(w, r, []ErrorDetail{{Field: "email", Message: "required"}})
```

## Error Code Mapping

| Service Error | HTTP Status | Code |
|--------------|-------------|------|
| ErrNotFound | 404 | NOT_FOUND |
| ErrInvalidInput | 400 | INVALID_INPUT |
| ErrUnauthorized | 401 | UNAUTHORIZED |
| ErrAlreadyExists | 409 | ALREADY_EXISTS |
| (other) | 500 | INTERNAL |

## URL Patterns

- Collection: `GET /api/v1/{resources}`
- Single: `GET /api/v1/{resources}/{id}`
- Create: `POST /api/v1/{resources}`
- Update: `PATCH /api/v1/{resources}/{id}`
- Delete: `DELETE /api/v1/{resources}/{id}`

## Timestamp Convention

- API は UTC ISO 8601 文字列 (`2026-01-20T15:00:00.000Z`)
- 日付範囲クエリ: `?start_timestamp=...&end_timestamp=...`
