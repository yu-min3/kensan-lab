-- ============================================================================
-- Demo Seed: AI Challenges & Ratings — 山田拓也
-- ============================================================================

-- ==============================================================================
-- Section A: Per-user ai_contexts (copy all system templates)
-- ==============================================================================

-- chat context
INSERT INTO ai_contexts (
    id, name, situation, version, is_active, is_default,
    system_prompt, allowed_tools, max_turns, temperature,
    description, user_id, source_template_id
)
SELECT
    'd2c00001-0000-0000-0000-000000000000'::uuid,
    name, situation, version, true, true,
    system_prompt, allowed_tools, max_turns, temperature,
    description,
    'd2222222-2222-2222-2222-222222222222'::uuid,
    id
FROM ai_contexts
WHERE situation = 'chat' AND user_id IS NULL AND is_default = true AND is_active = true
LIMIT 1;

-- review context
INSERT INTO ai_contexts (
    id, name, situation, version, is_active, is_default,
    system_prompt, allowed_tools, max_turns, temperature,
    description, user_id, source_template_id
)
SELECT
    'd2c00002-0000-0000-0000-000000000000'::uuid,
    name, situation, version, true, true,
    system_prompt, allowed_tools, max_turns, temperature,
    description,
    'd2222222-2222-2222-2222-222222222222'::uuid,
    id
FROM ai_contexts
WHERE situation = 'review' AND user_id IS NULL AND is_default = true AND is_active = true
LIMIT 1;

-- daily_advice context
INSERT INTO ai_contexts (
    id, name, situation, version, is_active, is_default,
    system_prompt, allowed_tools, max_turns, temperature,
    description, user_id, source_template_id
)
SELECT
    'd2c00003-0000-0000-0000-000000000000'::uuid,
    name, situation, version, true, true,
    system_prompt, allowed_tools, max_turns, temperature,
    description,
    'd2222222-2222-2222-2222-222222222222'::uuid,
    id
FROM ai_contexts
WHERE situation = 'daily_advice' AND user_id IS NULL AND is_default = true AND is_active = true
LIMIT 1;

-- persona context
INSERT INTO ai_contexts (
    id, name, situation, version, is_active, is_default,
    system_prompt, allowed_tools, max_turns, temperature,
    description, user_id, source_template_id
)
SELECT
    'd2c00004-0000-0000-0000-000000000000'::uuid,
    name, situation, version, true, true,
    system_prompt, allowed_tools, max_turns, temperature,
    description,
    'd2222222-2222-2222-2222-222222222222'::uuid,
    id
FROM ai_contexts
WHERE situation = 'persona' AND user_id IS NULL AND is_default = true AND is_active = true
LIMIT 1;

-- ==============================================================================
-- Section B: ai_context_versions
-- ==============================================================================
-- ORDER: v1 (template) → v3 (AI-optimized, from full template) → v2 (manual append) → update to v2
-- v3 must be created BEFORE the ai_contexts row is overwritten with the v2 append.

-- chat v1 (manual)
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'd2c10001-0000-0000-0000-000000000000'::uuid,
    'd2c00001-0000-0000-0000-000000000000'::uuid,
    1,
    system_prompt, allowed_tools, max_turns, temperature,
    'テンプレートからコピー',
    'manual', NULL, NULL
FROM ai_contexts WHERE id = 'd2c00001-0000-0000-0000-000000000000'::uuid;

-- v3: AI-optimized version (full template + energy scheduling + emotional follow-up)
-- IMPORTANT: must be inserted while ai_contexts still has the full template prompt
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'd2c40001-0000-0000-0000-000000000000'::uuid,
    'd2c00001-0000-0000-0000-000000000000'::uuid,
    3,
    system_prompt || E'\n\n## AI最適化: エネルギーベース・スケジューリング\nスケジュール提案時は {user_patterns} の生産性データを**必ず**活用し、テキストでも理由として言及する:\n- 集中力が必要なタスク（開発、学習、資格勉強、執筆）→ 生産性ピーク時間帯に配置し、「○時台が集中力のピークなので」と理由を明記\n- ルーチン作業（MTG準備、メール確認、整理）→ 低エネルギー時間帯に配置\n- 90分以上の集中ブロックの後には30分の休憩 or 軽作業を入れる\n- 1日のブロック数は4〜5個以内。詰め込みすぎず余白を残す\n- 既に入っている予定を確認し、空きスロットだけに提案する\n\n## AI最適化: 目標整合性チェック\n{goal_progress} と {weekly_summary} を参照し、以下を考慮する:\n- 時間配分の偏りがあれば「今週は○○に偏っているので△△の時間も確保」と提案\n- 期限が近いマイルストーンがあれば優先的にスケジューリング\n- {recent_learning_notes} の学習記録から、学習の継続性を考慮した提案を行う\n\n## AI最適化: 感情フォロー\n- スケジュール提案や振り返りの冒頭で、最近の達成や前向きな変化に1文だけ触れる\n- 過度な励ましは不要。事実ベースで自然に触れる程度にする\n- ユーザーが困っている・落ち込んでいるときは、まず共感してから提案に移る\n\n## AI最適化: 日時範囲の厳密化\nスケジュール提案時、ユーザーが指定した期間を厳密に守ること:\n- 「今週」= {current_datetime} が含まれる週の月曜〜日曜のみ\n- 「来週」= 翌週の月曜〜日曜のみ\n- 期間が曖昧な場合は「今週の残り」（今日〜日曜）として解釈する\n- create_time_block の date が指定期間外になっていないか、呼ぶ前に確認する',
    allowed_tools, max_turns, temperature,
    'AI最適化: エネルギーベース・スケジューリング、目標整合性チェック、感情フォロー、日時範囲の厳密化',
    'ai', 'pending',
    '{"interaction_count": 6, "avg_rating": 4.2, "strengths": ["AWS SAAの学習計画が体系的", "初心者目線での質問に的確に回答"], "weaknesses": ["学習の継続モチベーション維持のフォローが不足", "生産性データを活かしたスケジューリングがない", "資格勉強と業務のバランス提案が弱い"]}'::jsonb
FROM ai_contexts WHERE id = 'd2c00001-0000-0000-0000-000000000000'::uuid;

-- chat v2: 初心者向けの丁寧な説明強化 (manual)
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'd2c10002-0000-0000-0000-000000000000'::uuid,
    'd2c00001-0000-0000-0000-000000000000'::uuid,
    2,
    system_prompt || E'\n\n## パーソナライズ強化\n- 専門用語は必ず噛み砕いて説明する\n- ステップバイステップの手順を示す\n- 「なぜそうするか」の理由を添える\n\n## パーソナライズ: スケジューリング最適化\n{user_patterns} の生産性データを活用し:\n- 集中タスクはピーク時間帯に、ルーチンは低エネルギー時間帯に配置\n- 90分以上の集中ブロックの後には休憩を入れる\n- 既に入っている予定と重複しないよう配置する\n\n## パーソナライズ: 目標との整合性\n- {goal_progress} と {weekly_summary} の時間配分に偏りがあれば指摘・調整を提案\n- 期限が近いマイルストーンは優先的にスケジューリング\n\n## パーソナライズ: 感情への配慮\n- 最近の達成や前向きな変化があれば自然に触れる\n- 困っている場合はまず共感してから提案に移る\n\n## パーソナライズ: 日時範囲\n- 「今週」= 月曜〜日曜、「来週」= 翌週の月曜〜日曜を厳守',
    allowed_tools, max_turns, temperature,
    '初心者フレンドリー + パーソナライズ: スケジューリング、目標整合性、感情配慮',
    'manual', NULL, NULL
FROM ai_contexts WHERE id = 'd2c00001-0000-0000-0000-000000000000'::uuid;

-- Update chat context prompt to v2
UPDATE ai_contexts
SET system_prompt = system_prompt || E'\n\n## パーソナライズ強化\n- 専門用語は必ず噛み砕いて説明する\n- ステップバイステップの手順を示す\n- 「なぜそうするか」の理由を添える\n\n## パーソナライズ: スケジューリング最適化\n{user_patterns} の生産性データを活用し:\n- 集中タスクはピーク時間帯に、ルーチンは低エネルギー時間帯に配置\n- 90分以上の集中ブロックの後には休憩を入れる\n- 既に入っている予定と重複しないよう配置する\n\n## パーソナライズ: 目標との整合性\n- {goal_progress} と {weekly_summary} の時間配分に偏りがあれば指摘・調整を提案\n- 期限が近いマイルストーンは優先的にスケジューリング\n\n## パーソナライズ: 感情への配慮\n- 最近の達成や前向きな変化があれば自然に触れる\n- 困っている場合はまず共感してから提案に移る\n\n## パーソナライズ: 日時範囲\n- 「今週」= 月曜〜日曜、「来週」= 翌週の月曜〜日曜を厳守'
WHERE id = 'd2c00001-0000-0000-0000-000000000000'::uuid;

-- review v1 (manual)
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'd2c10003-0000-0000-0000-000000000000'::uuid,
    'd2c00002-0000-0000-0000-000000000000'::uuid,
    1,
    system_prompt, allowed_tools, max_turns, temperature,
    'テンプレートからコピー',
    'manual', NULL, NULL
FROM ai_contexts WHERE id = 'd2c00002-0000-0000-0000-000000000000'::uuid;

-- daily_advice v1 (manual)
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'd2c10004-0000-0000-0000-000000000000'::uuid,
    'd2c00003-0000-0000-0000-000000000000'::uuid,
    1,
    system_prompt, allowed_tools, max_turns, temperature,
    'テンプレートからコピー',
    'manual', NULL, NULL
FROM ai_contexts WHERE id = 'd2c00003-0000-0000-0000-000000000000'::uuid;

-- persona v1 (manual)
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'd2c10005-0000-0000-0000-000000000000'::uuid,
    'd2c00004-0000-0000-0000-000000000000'::uuid,
    1,
    system_prompt, allowed_tools, max_turns, temperature,
    'テンプレートからコピー',
    'manual', NULL, NULL
FROM ai_contexts WHERE id = 'd2c00004-0000-0000-0000-000000000000'::uuid;

-- Set active_version for each context
UPDATE ai_contexts SET active_version = 2 WHERE id = 'd2c00001-0000-0000-0000-000000000000'::uuid;  -- chat (v2 is latest manual)
UPDATE ai_contexts SET active_version = 1 WHERE id = 'd2c00002-0000-0000-0000-000000000000'::uuid;  -- review
UPDATE ai_contexts SET active_version = 1 WHERE id = 'd2c00003-0000-0000-0000-000000000000'::uuid;  -- daily_advice
UPDATE ai_contexts SET active_version = 1 WHERE id = 'd2c00004-0000-0000-0000-000000000000'::uuid;  -- persona

-- ==============================================================================
-- Section C: Update ai_interactions with context_id and rating
-- ==============================================================================

-- chat interactions
UPDATE ai_interactions SET context_id = 'd2c00001-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'd2800001-0000-0000-0000-000000000000'::uuid;  -- AWS SAA勉強開始

UPDATE ai_interactions SET context_id = 'd2c00001-0000-0000-0000-000000000000'::uuid, rating = 3
WHERE id = 'd2800002-0000-0000-0000-000000000000'::uuid;  -- 勉強が続かない

UPDATE ai_interactions SET context_id = 'd2c00001-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'd2800003-0000-0000-0000-000000000000'::uuid;  -- SAA模擬試験52%

UPDATE ai_interactions SET context_id = 'd2c00001-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'd2800004-0000-0000-0000-000000000000'::uuid;  -- 毎日30分1週間達成

UPDATE ai_interactions SET context_id = 'd2c00001-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'd2800005-0000-0000-0000-000000000000'::uuid;  -- SAA模擬試験68%

-- evening interaction → daily_advice context
UPDATE ai_interactions SET context_id = 'd2c00003-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'd2800006-0000-0000-0000-000000000000'::uuid;  -- 8週間振り返り
