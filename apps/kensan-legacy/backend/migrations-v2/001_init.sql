-- ============================================================================
-- Kensan Database Schema (Consolidated)
-- ============================================================================
-- This is the consolidated schema from migrations 001-070.
-- Use this for fresh setups instead of running 70+ incremental migrations.
-- ============================================================================

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS vector;

-- ============================================
-- Users
-- ============================================
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);

-- ============================================
-- User Settings
-- ============================================
CREATE TABLE user_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    timezone VARCHAR(50) DEFAULT 'Asia/Tokyo',
    theme VARCHAR(20) DEFAULT 'system' CHECK (theme IN ('light', 'dark', 'system')),
    is_configured BOOLEAN DEFAULT FALSE,
    ai_enabled BOOLEAN DEFAULT FALSE,
    ai_consent_given BOOLEAN DEFAULT FALSE,
    ai_consented_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================
-- Goals
-- ============================================
CREATE TABLE goals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    color VARCHAR(7) NOT NULL DEFAULT '#0EA5E9',
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'archived')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_goals_user_id ON goals(user_id);
CREATE INDEX idx_goals_sort_order ON goals(user_id, sort_order);

-- ============================================
-- Milestones
-- ============================================
CREATE TABLE milestones (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    goal_id UUID NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    start_date DATE,
    target_date DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'archived')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_milestones_user_id ON milestones(user_id);
CREATE INDEX idx_milestones_goal_id ON milestones(goal_id);
CREATE INDEX idx_milestones_status ON milestones(status);
CREATE INDEX idx_milestones_start_date ON milestones(start_date);

-- ============================================
-- Tags
-- ============================================
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    color VARCHAR(7) NOT NULL DEFAULT '#6B7280',
    type VARCHAR(10) NOT NULL DEFAULT 'task' CHECK (type IN ('task', 'note')),
    pinned BOOLEAN NOT NULL DEFAULT FALSE,
    usage_count INTEGER NOT NULL DEFAULT 0,
    category VARCHAR(20) DEFAULT 'general' CHECK (category IN ('general', 'trait', 'tech', 'project')),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, name, type)
);

CREATE INDEX idx_tags_user_id ON tags(user_id);
CREATE INDEX idx_tags_pinned ON tags(user_id, pinned) WHERE pinned = TRUE;
CREATE INDEX idx_tags_usage_count ON tags(user_id, usage_count DESC);
CREATE INDEX idx_tags_type ON tags(user_id, type);
CREATE INDEX idx_tags_category ON tags(user_id, category);

-- ============================================
-- Tasks
-- ============================================
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    milestone_id UUID REFERENCES milestones(id) ON DELETE SET NULL,
    parent_task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    estimated_minutes INTEGER,
    completed BOOLEAN DEFAULT FALSE,
    due_date DATE,
    frequency VARCHAR(20) CHECK (frequency IN ('daily', 'weekly', 'monthly', 'custom')),
    days_of_week INTEGER[],
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_tasks_user_id ON tasks(user_id);
CREATE INDEX idx_tasks_milestone_id ON tasks(milestone_id);
CREATE INDEX idx_tasks_parent_task_id ON tasks(parent_task_id);
CREATE INDEX idx_tasks_completed ON tasks(completed);
CREATE INDEX idx_tasks_frequency ON tasks(frequency) WHERE frequency IS NOT NULL;

-- ============================================
-- Task-Tags junction
-- ============================================
CREATE TABLE task_tags (
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, tag_id)
);

CREATE INDEX idx_task_tags_task_id ON task_tags(task_id);
CREATE INDEX idx_task_tags_tag_id ON task_tags(tag_id);

-- ============================================
-- Time Blocks (計画)
-- ============================================
CREATE TABLE time_blocks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    start_datetime TIMESTAMPTZ NOT NULL,
    end_datetime TIMESTAMPTZ NOT NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    task_name VARCHAR(255) NOT NULL,
    milestone_id UUID REFERENCES milestones(id) ON DELETE SET NULL,
    milestone_name VARCHAR(255),
    goal_id UUID REFERENCES goals(id) ON DELETE SET NULL,
    goal_name VARCHAR(255),
    goal_color VARCHAR(7),
    tag_ids UUID[] DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_time_blocks_user_id ON time_blocks(user_id);
CREATE INDEX idx_time_blocks_milestone_id ON time_blocks(milestone_id);
CREATE INDEX idx_time_blocks_goal_id ON time_blocks(goal_id);
CREATE INDEX idx_time_blocks_user_start ON time_blocks(user_id, start_datetime);

-- ============================================
-- Time Entries (実績)
-- ============================================
CREATE TABLE time_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    start_datetime TIMESTAMPTZ NOT NULL,
    end_datetime TIMESTAMPTZ NOT NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    task_name VARCHAR(255) NOT NULL,
    milestone_id UUID REFERENCES milestones(id) ON DELETE SET NULL,
    milestone_name VARCHAR(255),
    goal_id UUID REFERENCES goals(id) ON DELETE SET NULL,
    goal_name VARCHAR(255),
    goal_color VARCHAR(7),
    tag_ids UUID[] DEFAULT '{}',
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_time_entries_user_id ON time_entries(user_id);
CREATE INDEX idx_time_entries_milestone_id ON time_entries(milestone_id);
CREATE INDEX idx_time_entries_goal_id ON time_entries(goal_id);
CREATE INDEX idx_time_entries_user_start ON time_entries(user_id, start_datetime);

-- ============================================
-- Running Timers
-- ============================================
CREATE TABLE running_timers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    task_name VARCHAR(255) NOT NULL,
    milestone_id UUID,
    milestone_name VARCHAR(255),
    goal_id UUID,
    goal_name VARCHAR(255),
    goal_color VARCHAR(7),
    tag_ids UUID[] DEFAULT '{}',
    started_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_running_timers_user_id ON running_timers(user_id);

-- ============================================
-- Todos (unified: one-off + recurring)
-- ============================================
CREATE TABLE todos (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    frequency VARCHAR(20) CHECK (frequency IN ('daily', 'weekly', 'monthly', 'custom')),
    days_of_week INTEGER[],
    due_date DATE,
    estimated_minutes INTEGER,
    tag_ids UUID[] DEFAULT '{}',
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_todos_user_id ON todos(user_id);
CREATE INDEX idx_todos_enabled ON todos(enabled);
CREATE INDEX idx_todos_due_date ON todos(due_date);
CREATE INDEX idx_todos_frequency ON todos(frequency);

-- ============================================
-- Todo Completions
-- ============================================
CREATE TABLE todo_completions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    todo_id UUID NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
    completed_date DATE NOT NULL,
    completed_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(todo_id, completed_date)
);

CREATE INDEX idx_todo_completions_todo_id ON todo_completions(todo_id);
CREATE INDEX idx_todo_completions_date ON todo_completions(completed_date);
CREATE INDEX idx_todo_completions_user_id ON todo_completions(user_id);

-- ============================================
-- Note Types (data-driven)
-- ============================================
CREATE TABLE note_types (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    display_name_en VARCHAR(100),
    description TEXT,
    icon VARCHAR(50) NOT NULL DEFAULT 'file-text',
    color VARCHAR(7) DEFAULT '#6B7280',
    constraints JSONB NOT NULL DEFAULT '{}',
    metadata_schema JSONB NOT NULL DEFAULT '[]',
    sort_order INT DEFAULT 0,
    is_system BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================
-- Notes (unified: diary, learning, memo, etc.)
-- ============================================
CREATE TABLE notes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL REFERENCES note_types(slug),
    title VARCHAR(255),
    content TEXT,
    format VARCHAR(20) NOT NULL DEFAULT 'markdown' CHECK (format IN ('markdown', 'drawio')),
    date DATE,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    milestone_id UUID REFERENCES milestones(id) ON DELETE SET NULL,
    goal_id UUID REFERENCES goals(id) ON DELETE SET NULL,
    milestone_name VARCHAR(255),
    goal_name VARCHAR(255),
    goal_color VARCHAR(7),
    related_time_entry_ids UUID[] DEFAULT '{}',
    file_url TEXT,
    archived BOOLEAN DEFAULT FALSE,
    index_status VARCHAR(20) DEFAULT 'pending' CHECK (index_status IN ('pending', 'processing', 'indexed', 'failed')),
    indexed_at TIMESTAMPTZ,
    embedding vector(1536),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_notes_user_id ON notes(user_id);
CREATE INDEX idx_notes_type ON notes(type);
CREATE INDEX idx_notes_date ON notes(date);
CREATE INDEX idx_notes_user_type ON notes(user_id, type);
CREATE INDEX idx_notes_task_id ON notes(task_id);
CREATE INDEX idx_notes_milestone_id ON notes(milestone_id);
CREATE INDEX idx_notes_goal_id ON notes(goal_id);
CREATE INDEX idx_notes_archived ON notes(archived);
CREATE INDEX idx_notes_search ON notes USING GIN (to_tsvector('simple', COALESCE(title, '') || ' ' || COALESCE(content, '')));
CREATE INDEX idx_notes_embedding ON notes USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

CREATE UNIQUE INDEX idx_notes_diary_unique ON notes(user_id, date) WHERE type = 'diary';
CREATE UNIQUE INDEX idx_notes_learning_unique ON notes(user_id, date) WHERE type = 'learning';

-- ============================================
-- Note-Tags junction
-- ============================================
CREATE TABLE note_tags (
    note_id UUID NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (note_id, tag_id)
);

CREATE INDEX idx_note_tags_note_id ON note_tags(note_id);
CREATE INDEX idx_note_tags_tag_id ON note_tags(tag_id);

-- ============================================
-- Note Metadata (key-value)
-- ============================================
CREATE TABLE note_metadata (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    note_id UUID NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(note_id, key)
);

CREATE INDEX idx_note_metadata_note ON note_metadata(note_id);
CREATE INDEX idx_note_metadata_key ON note_metadata(key);

-- ============================================
-- Note Contents (multi-content per note)
-- ============================================
CREATE TABLE note_contents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    note_id UUID NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    content_type VARCHAR(50) NOT NULL CHECK (content_type IN ('markdown', 'drawio', 'image', 'pdf', 'code')),
    content TEXT,
    storage_provider VARCHAR(20) CHECK (storage_provider IN ('minio', 'r2', 's3', 'local')),
    storage_key TEXT,
    file_name VARCHAR(255),
    mime_type VARCHAR(100),
    file_size_bytes BIGINT,
    checksum VARCHAR(64),
    thumbnail_base64 TEXT,
    sort_order INT DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_note_contents_note_id ON note_contents(note_id);
CREATE INDEX idx_note_contents_type ON note_contents(content_type);
CREATE INDEX idx_note_contents_storage ON note_contents(storage_provider, storage_key) WHERE storage_key IS NOT NULL;

-- ============================================
-- Note Content Chunks (AI/search indexing)
-- ============================================
CREATE TABLE note_content_chunks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    note_id UUID NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    note_content_id UUID NOT NULL REFERENCES note_contents(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    chunk_text TEXT NOT NULL,
    token_count INT,
    embedding vector(1536),
    embedding_model VARCHAR(50),
    content_type VARCHAR(50),
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_chunks_note_id ON note_content_chunks(note_id);
CREATE INDEX idx_chunks_content_id ON note_content_chunks(note_content_id);
CREATE INDEX idx_chunks_user_id ON note_content_chunks(user_id);
CREATE INDEX idx_chunks_fts ON note_content_chunks USING GIN (to_tsvector('simple', chunk_text));
CREATE INDEX idx_chunks_embedding ON note_content_chunks USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

-- ============================================
-- Entity Memos (attached to goals/milestones/tasks)
-- ============================================
CREATE TABLE entity_memos (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entity_type VARCHAR(20) NOT NULL CHECK (entity_type IN ('goal', 'milestone', 'task')),
    entity_id UUID NOT NULL,
    content TEXT NOT NULL,
    pinned BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_entity_memos_user ON entity_memos(user_id);
CREATE INDEX idx_entity_memos_lookup ON entity_memos(entity_type, entity_id);
CREATE INDEX idx_entity_memos_pinned ON entity_memos(entity_type, entity_id, pinned) WHERE pinned = TRUE;

-- ============================================
-- Memos (standalone quick memos)
-- ============================================
CREATE TABLE memos (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    archived BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_memos_user_id ON memos(user_id);
CREATE INDEX idx_memos_archived ON memos(archived);

-- ============================================
-- AI Review Reports
-- ============================================
CREATE TABLE ai_review_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    summary TEXT,
    good_points TEXT[],
    improvement_points TEXT[],
    advice TEXT[],
    tokens_input INTEGER,
    tokens_output INTEGER,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_ai_review_reports_user_id ON ai_review_reports(user_id);
CREATE INDEX idx_ai_review_reports_period ON ai_review_reports(period_start, period_end);

-- ============================================
-- AI Interactions (conversation logs)
-- ============================================
CREATE TABLE ai_interactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id UUID NOT NULL,
    conversation_id UUID,
    situation VARCHAR(50) NOT NULL DEFAULT 'chat',
    context_id UUID,
    persona_context_id UUID,
    user_input TEXT NOT NULL,
    ai_output TEXT NOT NULL,
    tool_calls JSONB DEFAULT '[]',
    tokens_input INTEGER,
    tokens_output INTEGER,
    latency_ms INTEGER,
    rating SMALLINT CHECK (rating >= 1 AND rating <= 5),
    feedback TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_ai_interactions_user_id ON ai_interactions(user_id);
CREATE INDEX idx_ai_interactions_session_id ON ai_interactions(session_id);
CREATE INDEX idx_ai_interactions_situation ON ai_interactions(situation);
CREATE INDEX idx_ai_interactions_created_at ON ai_interactions(created_at);
CREATE INDEX idx_ai_interactions_context_id ON ai_interactions(context_id);
CREATE INDEX idx_ai_interactions_conversation_id ON ai_interactions(conversation_id) WHERE conversation_id IS NOT NULL;
CREATE INDEX idx_ai_interactions_user_conversation ON ai_interactions(user_id, conversation_id, created_at DESC) WHERE conversation_id IS NOT NULL;
CREATE INDEX idx_ai_interactions_persona ON ai_interactions(persona_context_id) WHERE persona_context_id IS NOT NULL;

-- ============================================
-- AI Contexts (situation-specific prompts)
-- ============================================
CREATE TABLE ai_contexts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    situation VARCHAR(50) NOT NULL,
    version VARCHAR(20) NOT NULL DEFAULT '1.0.0',
    is_active BOOLEAN DEFAULT TRUE,
    is_default BOOLEAN DEFAULT FALSE,
    system_prompt TEXT NOT NULL,
    description TEXT,
    allowed_tools TEXT[] NOT NULL DEFAULT '{}',
    max_turns INTEGER DEFAULT 10,
    temperature FLOAT DEFAULT 0.7,
    experiment_id UUID,
    traffic_weight INTEGER DEFAULT 100,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    source_template_id UUID,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_ai_contexts_situation ON ai_contexts(situation);
CREATE INDEX idx_ai_contexts_is_active ON ai_contexts(is_active);
CREATE INDEX idx_ai_contexts_is_default ON ai_contexts(is_default);
CREATE INDEX idx_ai_contexts_experiment_id ON ai_contexts(experiment_id);
CREATE INDEX idx_ai_contexts_user_id ON ai_contexts(user_id) WHERE user_id IS NOT NULL;
-- System template default: one per situation
CREATE UNIQUE INDEX idx_ai_contexts_default_per_situation_system
    ON ai_contexts(situation) WHERE user_id IS NULL AND is_default = TRUE AND is_active = TRUE;
-- Per-user default: one per user+situation
CREATE UNIQUE INDEX idx_ai_contexts_default_per_situation_user
    ON ai_contexts(user_id, situation) WHERE user_id IS NOT NULL AND is_default = TRUE AND is_active = TRUE;

-- FK for source_template_id (self-ref, added after table creation)
ALTER TABLE ai_contexts ADD CONSTRAINT fk_ai_contexts_source_template
    FOREIGN KEY (source_template_id) REFERENCES ai_contexts(id);

-- ============================================
-- AI Context Versions (prompt version history)
-- ============================================
CREATE TABLE ai_context_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    context_id UUID NOT NULL REFERENCES ai_contexts(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    system_prompt TEXT NOT NULL,
    allowed_tools TEXT[] NOT NULL DEFAULT '{}',
    max_turns INTEGER NOT NULL,
    temperature FLOAT NOT NULL,
    changelog TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_ai_context_versions_unique
    ON ai_context_versions(context_id, version_number);
CREATE INDEX idx_ai_context_versions_context
    ON ai_context_versions(context_id, version_number DESC);

-- ============================================
-- Prompt Evaluations (periodic quality assessment)
-- ============================================
CREATE TABLE prompt_evaluations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    context_id UUID NOT NULL REFERENCES ai_contexts(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    interaction_count INT NOT NULL DEFAULT 0,
    avg_rating FLOAT,
    rated_count INT NOT NULL DEFAULT 0,
    tool_success_rate FLOAT,
    avg_turns FLOAT,
    avg_tokens FLOAT,
    strengths TEXT[] DEFAULT '{}',
    weaknesses TEXT[] DEFAULT '{}',
    improvement_suggestions TEXT[] DEFAULT '{}',
    sample_analysis JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_prompt_evaluations_unique
    ON prompt_evaluations(context_id, period_start, COALESCE(user_id, '00000000-0000-0000-0000-000000000000'::UUID));
CREATE INDEX idx_prompt_evaluations_context_id ON prompt_evaluations(context_id);

-- ============================================
-- Prompt Experiments (A/B testing)
-- ============================================
CREATE TABLE prompt_experiments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    situation TEXT NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    evaluation_id UUID NOT NULL REFERENCES prompt_evaluations(id) ON DELETE CASCADE,
    control_context_id UUID NOT NULL REFERENCES ai_contexts(id) ON DELETE CASCADE,
    variant_context_id UUID NOT NULL REFERENCES ai_contexts(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending_review'
        CHECK (status IN ('pending_review', 'in_challenge', 'promoted', 'rejected')),
    challenge_type TEXT NOT NULL DEFAULT 'side_by_side',
    challenge_config JSONB NOT NULL DEFAULT '{}',
    challenge_results JSONB NOT NULL DEFAULT '[]',
    win_rate FLOAT,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_prompt_experiments_status ON prompt_experiments(status);
CREATE INDEX idx_prompt_experiments_situation ON prompt_experiments(situation);
CREATE INDEX idx_prompt_experiments_user_id ON prompt_experiments(user_id) WHERE user_id IS NOT NULL;

-- ============================================
-- Prompt Comparisons (version-based A/B comparison)
-- ============================================
CREATE TABLE prompt_comparisons (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    context_id UUID NOT NULL REFERENCES ai_contexts(id) ON DELETE CASCADE,
    version_a INTEGER NOT NULL,
    version_b INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'adopted_a', 'adopted_b', 'dismissed')),
    rounds JSONB NOT NULL DEFAULT '[]'::jsonb,
    win_rate_b REAL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_prompt_comparisons_user_status ON prompt_comparisons(user_id, status);
CREATE INDEX idx_prompt_comparisons_context ON prompt_comparisons(context_id);

-- ============================================
-- User Memory (aggregated user profile for AI)
-- ============================================
CREATE TABLE user_memory (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    profile_summary TEXT,
    preferences JSONB DEFAULT '{}',
    strengths TEXT[] DEFAULT '{}',
    growth_areas TEXT[] DEFAULT '{}',
    last_updated TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================
-- User Facts (individual facts about users)
-- ============================================
CREATE TABLE user_facts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fact_type VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    source VARCHAR(50) NOT NULL DEFAULT 'conversation',
    source_interaction_id UUID REFERENCES ai_interactions(id) ON DELETE SET NULL,
    confidence FLOAT DEFAULT 1.0 CHECK (confidence >= 0 AND confidence <= 1),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_user_facts_user_id ON user_facts(user_id);
CREATE INDEX idx_user_facts_fact_type ON user_facts(fact_type);
CREATE INDEX idx_user_facts_created_at ON user_facts(created_at);
CREATE INDEX idx_user_facts_expires_at ON user_facts(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_user_facts_source_interaction ON user_facts(source_interaction_id);

-- ============================================
-- Analytics Views (read-only)
-- ============================================
CREATE OR REPLACE VIEW analytics_time_blocks AS
SELECT user_id, start_datetime, end_datetime, goal_id, goal_name, goal_color, milestone_id, milestone_name, tag_ids
FROM time_blocks;

CREATE OR REPLACE VIEW analytics_time_entries AS
SELECT user_id, start_datetime, end_datetime, goal_id, goal_name, goal_color, milestone_id, milestone_name, tag_ids
FROM time_entries;

CREATE OR REPLACE VIEW analytics_tasks AS
SELECT user_id, id, completed, updated_at
FROM tasks;

CREATE OR REPLACE VIEW analytics_goals AS
SELECT user_id, id, name, color, (status = 'archived') AS is_archived
FROM goals;

-- ============================================
-- Trigger: updated_at auto-update
-- ============================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_user_settings_updated_at BEFORE UPDATE ON user_settings FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_goals_updated_at BEFORE UPDATE ON goals FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_milestones_updated_at BEFORE UPDATE ON milestones FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_tasks_updated_at BEFORE UPDATE ON tasks FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_time_blocks_updated_at BEFORE UPDATE ON time_blocks FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_time_entries_updated_at BEFORE UPDATE ON time_entries FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_memos_updated_at BEFORE UPDATE ON memos FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_notes_updated_at BEFORE UPDATE ON notes FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_note_contents_updated_at BEFORE UPDATE ON note_contents FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_note_metadata_updated_at BEFORE UPDATE ON note_metadata FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_note_types_updated_at BEFORE UPDATE ON note_types FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_entity_memos_updated_at BEFORE UPDATE ON entity_memos FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_todos_updated_at BEFORE UPDATE ON todos FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_ai_contexts_updated_at BEFORE UPDATE ON ai_contexts FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- Trigger: Tag usage count auto-increment
-- ============================================
CREATE OR REPLACE FUNCTION increment_tag_usage_count()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE tags SET usage_count = usage_count + 1 WHERE id = NEW.tag_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER increment_task_tag_usage AFTER INSERT ON task_tags FOR EACH ROW EXECUTE FUNCTION increment_tag_usage_count();
CREATE TRIGGER increment_note_tag_usage AFTER INSERT ON note_tags FOR EACH ROW EXECUTE FUNCTION increment_tag_usage_count();

-- ============================================
-- Trigger: Denormalized field sync
-- ============================================

-- Goal name/color → time_blocks, time_entries, running_timers, notes
CREATE OR REPLACE FUNCTION sync_goal_denormalized_fields()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.name IS DISTINCT FROM OLD.name OR NEW.color IS DISTINCT FROM OLD.color THEN
        UPDATE time_blocks SET goal_name = NEW.name, goal_color = NEW.color WHERE goal_id = NEW.id;
        UPDATE time_entries SET goal_name = NEW.name, goal_color = NEW.color WHERE goal_id = NEW.id;
        UPDATE running_timers SET goal_name = NEW.name, goal_color = NEW.color WHERE goal_id = NEW.id;
        UPDATE notes SET goal_name = NEW.name, goal_color = NEW.color WHERE goal_id = NEW.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_sync_goal_denormalized_fields
    AFTER UPDATE OF name, color ON goals
    FOR EACH ROW EXECUTE FUNCTION sync_goal_denormalized_fields();

-- Milestone name → time_blocks, time_entries, running_timers, notes
CREATE OR REPLACE FUNCTION sync_milestone_denormalized_fields()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.name IS DISTINCT FROM OLD.name THEN
        UPDATE time_blocks SET milestone_name = NEW.name WHERE milestone_id = NEW.id;
        UPDATE time_entries SET milestone_name = NEW.name WHERE milestone_id = NEW.id;
        UPDATE running_timers SET milestone_name = NEW.name WHERE milestone_id = NEW.id;
        UPDATE notes SET milestone_name = NEW.name WHERE milestone_id = NEW.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_sync_milestone_denormalized_fields
    AFTER UPDATE OF name ON milestones
    FOR EACH ROW EXECUTE FUNCTION sync_milestone_denormalized_fields();

-- Task name → time_blocks, time_entries, running_timers
CREATE OR REPLACE FUNCTION sync_task_denormalized_fields()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.name IS DISTINCT FROM OLD.name THEN
        UPDATE time_blocks SET task_name = NEW.name WHERE task_id = NEW.id;
        UPDATE time_entries SET task_name = NEW.name WHERE task_id = NEW.id;
        UPDATE running_timers SET task_name = NEW.name WHERE task_id = NEW.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_sync_task_denormalized_fields
    AFTER UPDATE OF name ON tasks
    FOR EACH ROW EXECUTE FUNCTION sync_task_denormalized_fields();
