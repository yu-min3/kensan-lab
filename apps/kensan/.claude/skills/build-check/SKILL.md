---
name: build-check
description: Run both frontend and backend builds to verify everything compiles
disable-model-invocation: true
allowed-tools: Bash(cd frontend *), Bash(cd backend *), Bash(make *), Read, Edit, Grep, Glob
---

# Full Build Check

Run frontend and backend builds to verify compilation.

## Process

1. **Frontend build**:
   ```bash
   cd frontend && npm run build
   ```
   This runs TypeScript type checking + Vite production build.

2. **Backend build**:
   ```bash
   cd backend && make build
   ```
   This compiles all Go services.

3. **If either fails**:
   - Parse error messages
   - Fix TypeScript or Go compilation errors
   - Re-run the failed build
   - Repeat until both pass (max 3 iterations)

4. **Report**: Summary of build status for both frontend and backend.

## Common Issues

- **TS errors**: Missing imports, type mismatches, unused variables (strict mode)
- **Go errors**: Unused imports, type mismatches, missing interface implementations
- **Both**: Check that API types match between frontend and backend
