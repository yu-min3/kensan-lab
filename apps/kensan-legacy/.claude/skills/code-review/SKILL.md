---
name: code-review
description: Review uncommitted changes for security issues, code quality, and best practices
disable-model-invocation: true
context: fork
agent: general-purpose
allowed-tools: Bash(git *), Read, Grep, Glob
---

# Code Review

Review all uncommitted changes for quality and security issues.

## Process

1. **Identify changes**: Run `git diff --name-only HEAD` and `git diff --staged --name-only`

2. **Security Review (CRITICAL)**:
   - Hardcoded credentials, API keys, tokens
   - SQL injection (string concatenation instead of `$1` placeholders)
   - Missing `WHERE user_id = $1` in queries (multi-tenancy violation)
   - XSS vulnerabilities in frontend
   - Unsafe dependencies or imports

3. **Architecture Review (HIGH)**:
   - Layer violations (handler doing business logic, service doing SQL)
   - Missing error handling
   - Response format not following envelope pattern
   - Missing context propagation in Go code

4. **Code Quality (MEDIUM)**:
   - Functions over 50 lines
   - Files over 800 lines
   - Deep nesting (> 4 levels)
   - Unused imports or variables
   - Inconsistent naming

5. **Kensan-specific Checks**:
   - Timezone: UTC stored in DB, local conversion in frontend only
   - Page naming convention followed
   - Zustand store patterns followed
   - Repository interface defined for new data access

## Output Format

For each issue found:
```
[SEVERITY] file:line - Description
  Recommendation: ...
```

Severities: CRITICAL > HIGH > MEDIUM > LOW

Summarize: total issues by severity, and whether commit should proceed.
