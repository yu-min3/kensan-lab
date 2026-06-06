---
name: new-page
description: Create a new frontend page with routing, store, and API service following Kensan conventions
argument-hint: [PageName] [domain-prefix]
disable-model-invocation: true
---

# New Frontend Page

Create a new page `$ARGUMENTS[0]` with domain prefix `$ARGUMENTS[1]`.

## Prerequisites

Read `frontend/src/ARCHITECTURE.md` and an existing page for reference.

## Naming Convention

Page prefix determines the domain:
- S: Settings/System
- D: Daily
- N: Notes
- T: Task
- R: Routine
- A: Analytics/AI
- O: Observability

Example: `/new-page GoalDashboard A` → creates `A03_GoalDashboard`

## Steps

1. **Determine page number**: Check existing pages in `frontend/src/pages/` for the next available number in the prefix.

2. **Create page component** in `frontend/src/pages/{Prefix}{NN}_{PageName}.tsx`:
   - Import from `@/components/ui/` and `@/stores/`
   - Use Zustand stores for state
   - Follow existing page patterns

3. **Add route** in `frontend/src/App.tsx`:
   - Add inside the protected route group
   - Use lowercase kebab-case path

4. **Add sidebar navigation** in `frontend/src/components/layout/Sidebar.tsx`:
   - Add navigation item with Lucide icon

5. **Create/update Zustand store** if new state is needed:
   - Place in `frontend/src/stores/`
   - Use `createCrudStore` factory for standard CRUD
   - Or create custom store following existing patterns

6. **Create API service** if new endpoints:
   - Place in `frontend/src/api/services/`
   - Use `createApiService` factory
   - Follow response envelope unwrapping pattern

7. **Add MSW handler** in `frontend/src/mocks/handlers/` for development

## After Creation

- Run `cd frontend && npm run build` to verify TypeScript compilation
- Update `frontend/src/ARCHITECTURE.md` routing section and page naming table
