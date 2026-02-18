# ADR-0011: AI Prompt Versioning System

## Status

Accepted

## Context

AI behavior in Kensan is driven by system prompts stored in the `ai_contexts` table. As the team iterates on prompt quality -- adjusting tone, adding instructions, tuning tool selection -- there is a need to:

- Track what changed between prompt versions
- Roll back to a previous version if a new prompt degrades quality
- Compare different versions side-by-side
- Maintain an audit trail of prompt evolution

Without versioning, prompt changes were destructive updates with no history. A bad prompt change could only be reverted by manually restoring from memory or backups.

## Decision

Introduce a prompt versioning system with the following components:

**Database layer** (migration 050):
- `ai_context_versions` table stores snapshots of `ai_contexts` on every update
- Each version has a `version_number` (auto-incrementing per context), `system_prompt`, `allowed_tools`, `max_turns`, `temperature`, and an optional `changelog`
- The current version number is tracked on the `ai_contexts` table itself

**API layer** (kensan-ai routes):
- `PATCH /prompts/{id}` -- Updates a context and automatically creates a new version
- `GET /prompts/{id}/versions` -- Lists all versions for a context
- `GET /prompts/{id}/versions/{version_number}` -- Retrieves a specific version
- `POST /prompts/{id}/rollback/{version_number}` -- Rolls back to a specific version (creates a new version with the old content)

**Frontend layer** (A03_PromptEditor page):
- `usePromptStore` manages prompt and version state
- `PromptEditor` component for editing system prompts with syntax highlighting
- `VersionHistory` component shows version timeline with changelog entries
- `VersionDiffDialog` component enables side-by-side comparison of two versions

## Consequences

**Positive:**
- Full audit trail of prompt changes with timestamps and changelogs
- Safe iteration: any prompt change can be rolled back with one click
- Side-by-side diff enables informed decisions about prompt quality
- Enables future A/B testing workflows where specific versions are assigned to experiment variants
- Non-engineers can manage prompts through the UI without code deployments

**Negative:**
- Adds storage overhead for version snapshots (each version stores the full prompt text)
- Rollback creates a new version rather than restoring in place, so version numbers always increase
- The versioning system does not currently track the causal relationship between version performance and user satisfaction metrics (would require integration with the feedback system)
