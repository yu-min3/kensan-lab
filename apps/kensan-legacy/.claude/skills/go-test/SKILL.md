---
name: go-test
description: Run Go backend tests, analyze failures, and auto-fix
disable-model-invocation: true
allowed-tools: Bash(cd backend *), Bash(make *), Bash(go *), Read, Edit, Grep, Glob
---

# Go Test Runner

Run backend tests and fix failures automatically.

## Process

1. **Run tests**:
   ```bash
   cd backend && make test
   ```

2. **If all pass**: Report success with summary.

3. **If failures occur**:
   - Parse error output to identify failing tests
   - Read the failing test file and the source code being tested
   - Analyze the root cause
   - Fix the issue (prefer fixing source code over test expectations, unless the test expectation is wrong)
   - Re-run `cd backend && make test`
   - Repeat until all tests pass (max 3 iterations)

4. **If compilation errors**:
   - Parse error messages
   - Fix syntax/type errors
   - Re-run

5. **Report**: Summary of what was found and fixed.

## Notes

- Never skip or delete tests to make them pass
- If a fix requires changing the public API, flag it for review instead of auto-fixing
- Run `make lint` after fixes to check for style issues
