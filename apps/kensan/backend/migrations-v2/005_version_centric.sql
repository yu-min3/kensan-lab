-- 005: Version-centric architecture migration
-- Drop experiment/comparison tables, add version metadata columns

-- Drop dependent tables first (prompt_experiments references prompt_comparisons)
DROP TABLE IF EXISTS prompt_experiments;
DROP TABLE IF EXISTS prompt_comparisons;

-- Remove experiment-related columns from ai_contexts
DROP INDEX IF EXISTS idx_ai_contexts_experiment_id;
ALTER TABLE ai_contexts DROP COLUMN IF EXISTS experiment_id;
ALTER TABLE ai_contexts DROP COLUMN IF EXISTS traffic_weight;

-- Add metadata columns to ai_context_versions
ALTER TABLE ai_context_versions
  ADD COLUMN IF NOT EXISTS source VARCHAR(20) NOT NULL DEFAULT 'manual',
  ADD COLUMN IF NOT EXISTS eval_summary JSONB,
  ADD COLUMN IF NOT EXISTS candidate_status VARCHAR(20);

-- Add active_version tracking to ai_contexts
ALTER TABLE ai_contexts
  ADD COLUMN IF NOT EXISTS active_version INTEGER;

-- Initialize active_version from current max version_number
UPDATE ai_contexts c SET active_version = (
  SELECT MAX(version_number) FROM ai_context_versions WHERE context_id = c.id
) WHERE active_version IS NULL;
