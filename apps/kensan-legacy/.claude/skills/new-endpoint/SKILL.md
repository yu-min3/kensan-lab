---
name: new-endpoint
description: Add a new API endpoint to an existing service with full-stack integration (backend handler + frontend API service + store)
argument-hint: [service-name] [HTTP-method] [path] [description]
disable-model-invocation: true
---

# New API Endpoint

Add endpoint `$ARGUMENTS[1] $ARGUMENTS[2]` to `$ARGUMENTS[0]` service.
Description: $ARGUMENTS[3]

## Prerequisites

Read the target service's existing code:
- `backend/services/$0/internal/handler/handler.go`
- `backend/services/$0/internal/service/service.go`
- `backend/services/$0/internal/repository/repository.go`

## Backend Steps

1. **Model** (`internal/model.go`): Add request/response DTOs if needed.

2. **Repository** (`internal/repository/interface.go` + `repository.go`):
   - Add method to interface
   - Implement with SQL query (must include `WHERE user_id = $1`)

3. **Service** (`internal/service/interface.go` + `service.go`):
   - Add method to interface
   - Implement business logic
   - Use `shared/errors` for domain errors

4. **Handler** (`internal/handler/handler.go`):
   - Add handler method
   - Register route in router setup
   - Use `middleware.GetUserID(r.Context())` for user extraction
   - Use `middleware.JSON()` / `middleware.Error()` for responses

5. **Tests** (`internal/service/service_test.go`):
   - Add table-driven test for the new service method

## Frontend Steps

6. **API Service** (`frontend/src/api/services/`):
   - Add method to the corresponding API service
   - Use `httpClient.get/post/patch/delete()`

7. **Store** (`frontend/src/stores/`):
   - Add action to Zustand store if state management needed

8. **MSW Handler** (`frontend/src/mocks/handlers/`):
   - Add mock handler for development

## Verification

- Run `cd backend && make test`
- Run `cd frontend && npm run build`
- Update `backend/ARCHITECTURE.md` if new endpoint pattern
