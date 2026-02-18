# ADR-0009: Unified Chat Agent Architecture

## Status

Accepted

## Context

The Kensan AI service originally had separate agent implementations for different use cases:

- `planning_agent.py` -- Dedicated agent for morning planning and daily advice
- `weekly_review.py` -- Dedicated agent for weekly review generation

Each agent had its own system prompt, tool selection logic, and execution flow. This led to duplicated code across agents, difficulty in maintaining consistent behavior, the need to update multiple files when changing shared logic (e.g., tool execution, memory injection), and no easy way to A/B test prompts across different situations.

Meanwhile, the `ai_contexts` database table was introduced to store situation-based prompts with configurable tools, temperature, and max turns. This table-driven approach made dedicated Python agent classes unnecessary.

## Decision

Consolidate all agent implementations into a single `ChatAgent` class that dynamically adapts its behavior based on the detected situation (chat, review, daily_advice). The situation is resolved at request time via:

1. **Explicit parameter** -- The frontend can pass a `situation` field in the request
2. **Default fallback** -- Defaults to `chat` if not specified

The system prompt, allowed tools, temperature, and max turns are all loaded from the `ai_contexts` table at request time. Dynamic prompt variables (e.g., `{user_memory}`, `{today_schedule}`, `{emotion_summary}`) are resolved and injected into the system prompt before execution.

Tool selection is further refined by the `select_tools()` function, which analyzes the user message to include only relevant tool groups (e.g., planning tools for schedule-related queries, review tools for review requests).

## Consequences

**Positive:**
- Single code path for all AI interactions, reducing maintenance burden
- Prompt changes can be made via database updates or the frontend Prompt Editor (A03) without code deployment
- A/B testing of prompts is supported natively via `experiment_id` and `traffic_weight`
- Version history and rollback are available for all prompts via `ai_context_versions`

**Negative:**
- Less specialized behavior per situation compared to dedicated agent classes
- The single agent must handle all edge cases, increasing complexity in tool selection logic
- Debugging prompt issues requires checking both the database content and the variable replacement logic
