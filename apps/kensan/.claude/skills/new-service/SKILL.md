---
name: new-service
description: Scaffold a new Go microservice with the standard Kensan layered architecture
argument-hint: [service-name]
disable-model-invocation: true
---

# New Go Microservice Scaffold

Create a new microservice `$ARGUMENTS[0]` following the Kensan backend conventions.

## Prerequisites

Read `backend/ARCHITECTURE.md` and an existing service (e.g., `backend/services/memo/`) for reference.

## Steps

1. **Create directory structure**:
```
backend/services/$0/
├── cmd/main.go
├── internal/
│   ├── model.go
│   ├── handler/handler.go
│   ├── service/service.go
│   ├── service/interface.go
│   ├── service/service_test.go
│   └── repository/
│       ├── interface.go
│       └── repository.go
├── Dockerfile
└── Makefile
```

2. **cmd/main.go**: Use `bootstrap.New("$0")` for initialization. Register routes with `svc.RegisterRoutes()` and `svc.RegisterPublicRoutes()` as needed.

3. **internal/model.go**: Define domain types and DTOs. All entities must have `ID`, `UserID`, `CreatedAt`, `UpdatedAt`.

4. **internal/repository/interface.go**: Define repository interface following ISP (split by entity if multiple).

5. **internal/repository/repository.go**: PostgreSQL implementation using pgx. All queries must include `WHERE user_id = $1`. Use `pgx.ErrNoRows` → `errors.NotFound()`.

6. **internal/service/interface.go**: Define service interface (Reader + Writer pattern).

7. **internal/service/service.go**: Business logic. Use `shared/errors` for domain errors. Accept `ctx context.Context` and `userID string` as first parameters.

8. **internal/service/service_test.go**: Table-driven tests with mock repository.

9. **internal/handler/handler.go**: HTTP handlers. Use `middleware.GetUserID(r.Context())`, `middleware.JSON()`, `middleware.Error()`. Follow the error mapping pattern.

10. **Dockerfile**: Copy from existing service, update binary name.

11. **Makefile**: Copy from existing service, update service name.

12. **Database migration**: If needed, create `backend/migrations/NNN_create_$0_tables.sql` with `user_id`, UUID PKs, timestamps, and update trigger.

13. **docker-compose.yml**: Add service entry with appropriate port.

## After Scaffold

- Run `cd backend && make test` to verify compilation
- Update `backend/ARCHITECTURE.md` service list
