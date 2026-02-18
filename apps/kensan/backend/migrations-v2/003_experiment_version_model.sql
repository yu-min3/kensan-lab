-- 003: Experiment → Version model migration
-- Experiments now reference versions within the same context instead of separate contexts

-- Add new version-based columns
ALTER TABLE prompt_experiments
  ADD COLUMN context_id UUID REFERENCES ai_contexts(id),
  ADD COLUMN control_version INTEGER,
  ADD COLUMN variant_version INTEGER,
  ADD COLUMN comparison_id UUID REFERENCES prompt_comparisons(id);

-- Drop old context-based columns and challenge columns
ALTER TABLE prompt_experiments
  DROP COLUMN control_context_id,
  DROP COLUMN variant_context_id,
  DROP COLUMN challenge_type,
  DROP COLUMN challenge_config,
  DROP COLUMN challenge_results,
  DROP COLUMN win_rate;
