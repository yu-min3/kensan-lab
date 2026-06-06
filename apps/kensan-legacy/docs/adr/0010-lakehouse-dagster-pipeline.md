# ADR-0010: Medallion Architecture with Dagster + PyIceberg

## Status

Accepted

## Context

Kensan collects significant amounts of user behavior data through task management, time tracking, note-taking, and AI interactions. Analyzing this data directly against the OLTP PostgreSQL database would cause performance issues and couple analytics logic to the transactional schema.

Requirements for the analytics pipeline included:

- Decouple analytics workloads from the production database
- Support incremental processing of new data
- Enable LLM-based enrichment (emotion extraction, trait analysis) without blocking user-facing operations
- Provide aggregated profiles (interests, traits, emotions) for AI prompt personalization
- Allow schema evolution without breaking existing pipelines

## Decision

Adopt a **Medallion Architecture** (Bronze / Silver / Gold) using:

- **Dagster** for pipeline orchestration, scheduling, and dependency management
- **PyIceberg** for Apache Iceberg table management (schema evolution, time travel, partition pruning)
- **Nessie** as the Iceberg REST catalog (Git-like versioning of table metadata)

The three layers are:

| Layer | Purpose | Example Assets |
|-------|---------|---------------|
| **Bronze** | Raw ingestion from PostgreSQL (1:1 table copies) | `tasks_raw`, `time_blocks_raw`, `notes_raw`, `tags_raw` |
| **Silver** | Cleaned, enriched, and normalized data (LLM extraction) | `emotion_extractor`, `tag_usage_profile`, `user_trait_segments` |
| **Gold** | Aggregated profiles and summaries for consumption | `emotion_weekly`, `user_interest_profile`, `user_trait_profile` |

The AI service reads Gold layer tables via `LakehouseReader` and injects them as prompt variables (`{emotion_summary}`, `{interest_profile}`, `{user_traits}`, `{communication_style}`).

Bronze writes from the AI service (e.g., external tool results) use `LakehouseWriter` in a fire-and-forget pattern -- failures are logged but do not block the user response.

All Lakehouse operations are gated by the `LAKEHOUSE_ENABLED` configuration flag.

## Consequences

**Positive:**
- Analytics workloads are fully decoupled from OLTP, no performance impact on user operations
- Iceberg provides schema evolution, time travel queries, and efficient partition pruning
- Dagster gives visibility into pipeline health, retry logic, and dependency graphs
- LLM enrichment (emotion, traits) runs asynchronously in Silver layer pipelines
- Gold layer profiles enable deeply personalized AI interactions

**Negative:**
- Adds infrastructure complexity (Nessie catalog, Iceberg storage, Dagster scheduler)
- Requires managing a separate data stack alongside the main application
- Data freshness depends on pipeline scheduling (not real-time)
- LLM-based Silver layer processing incurs API costs proportional to data volume
