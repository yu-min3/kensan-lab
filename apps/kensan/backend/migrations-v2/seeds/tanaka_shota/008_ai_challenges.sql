-- ============================================================================
-- Demo Seed: AI Challenges & Ratings — 田中翔太
-- ============================================================================
-- per-user ai_contexts, ai_context_versions (v1 + v2 basic + v3 AI-optimized for chat),
-- ai_interactions rating/context_id updates, prompt_evaluations
-- Persona: 30歳バックエンドエンジニア, Go + Google Cloud, 夜型, ブログ・LT・SaaS
--
-- A/B テストデモ用:
--   v2 (active) = パーソナライズ版（スケジューリング最適化・目標整合性・感情配慮）
--   v3 (candidate) = AI最適化版（全変数＋エネルギーベース・厳密な根拠明示）
-- → v2=実用的なパーソナライズ / v3=データ引用を必須とする上位最適化 の差が出る

-- ==============================================================================
-- Section A: Per-user ai_contexts (copy all system templates)
-- ==============================================================================

-- chat context (starts with full template, v2 appends lighter AI optimizations)
INSERT INTO ai_contexts (
    id, name, situation, version, is_active, is_default,
    system_prompt, allowed_tools, max_turns, temperature,
    description, user_id, source_template_id
)
SELECT
    'ddc00001-0000-0000-0000-000000000000'::uuid,
    name, situation, version, true, true,
    system_prompt, allowed_tools, max_turns, temperature,
    description,
    'dddddddd-dddd-dddd-dddd-dddddddddddd'::uuid,
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
    'ddc00002-0000-0000-0000-000000000000'::uuid,
    name, situation, version, true, true,
    system_prompt, allowed_tools, max_turns, temperature,
    description,
    'dddddddd-dddd-dddd-dddd-dddddddddddd'::uuid,
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
    'ddc00003-0000-0000-0000-000000000000'::uuid,
    name, situation, version, true, true,
    system_prompt, allowed_tools, max_turns, temperature,
    description,
    'dddddddd-dddd-dddd-dddd-dddddddddddd'::uuid,
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
    'ddc00004-0000-0000-0000-000000000000'::uuid,
    name, situation, version, true, true,
    system_prompt, allowed_tools, max_turns, temperature,
    description,
    'dddddddd-dddd-dddd-dddd-dddddddddddd'::uuid,
    id
FROM ai_contexts
WHERE situation = 'persona' AND user_id IS NULL AND is_default = true AND is_active = true
LIMIT 1;

-- ==============================================================================
-- Section B: ai_context_versions
-- ==============================================================================
-- ORDER: v1 (template) → v3 (AI-optimized, from full template) → append v2 optimizations → v2 (personalized)
-- v3 must be created BEFORE the ai_contexts row is appended with the v2 optimizations.

-- v1: exact copy of template prompt (manual)
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'ddc10001-0000-0000-0000-000000000000'::uuid,
    'ddc00001-0000-0000-0000-000000000000'::uuid,
    1,
    system_prompt, allowed_tools, max_turns, temperature,
    'テンプレートからコピー',
    'manual', NULL, NULL
FROM ai_contexts WHERE id = 'ddc00001-0000-0000-0000-000000000000'::uuid;

-- v3: AI-optimized version (full template + energy scheduling + emotional follow-up)
-- IMPORTANT: must be inserted while ai_contexts still has the full template prompt
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'ddc40001-0000-0000-0000-000000000000'::uuid,
    'ddc00001-0000-0000-0000-000000000000'::uuid,
    3,
    system_prompt || E'\n\n## AI最適化: エネルギーベース・スケジューリング\nスケジュール提案時は {user_patterns} の生産性データを**必ず**活用し、テキストでも理由として言及する:\n- 集中力が必要なタスク（開発、学習、資格勉強、執筆）→ 生産性ピーク時間帯に配置し、「○時台が集中力のピークなので」と理由を明記\n- ルーチン作業（MTG準備、メール確認、整理）→ 低エネルギー時間帯に配置\n- 90分以上の集中ブロックの後には30分の休憩 or 軽作業を入れる\n- 1日のブロック数は4〜5個以内。詰め込みすぎず余白を残す\n- 既に入っている予定を確認し、空きスロットだけに提案する\n\n## AI最適化: 目標整合性チェック\n{goal_progress} と {weekly_summary} を参照し、以下を考慮する:\n- 時間配分の偏りがあれば「今週は○○に偏っているので△△の時間も確保」と提案\n- 期限が近いマイルストーンがあれば優先的にスケジューリング\n- {recent_learning_notes} の学習記録から、学習の継続性を考慮した提案を行う\n\n## AI最適化: 感情フォロー\n- スケジュール提案や振り返りの冒頭で、最近の達成や前向きな変化に1文だけ触れる\n- 例: 「先週ブログを1本公開できた勢いで」「ACE模擬試験のスコアが上がっている流れを活かして」\n- 過度な励ましは不要。事実ベースで自然に触れる程度にする\n- ユーザーが困っている・落ち込んでいるときは、まず共感してから提案に移る\n\n## AI最適化: 日時範囲の厳密化\nスケジュール提案時、ユーザーが指定した期間を厳密に守ること:\n- 「今週」= {current_datetime} が含まれる週の月曜〜日曜のみ\n- 「来週」= 翌週の月曜〜日曜のみ\n- 期間が曖昧な場合は「今週の残り」（今日〜日曜）として解釈する\n- create_time_block の date が指定期間外になっていないか、呼ぶ前に確認する\n\n## AI最適化: 提案根拠の明示（必須）\nスケジュールを提案する際は、タイムブロックを提示する**前に**、以下のような根拠を具体的なデータとともに箇条書きで示すこと:\n- {user_patterns} のピーク時間帯を引用し、「○時台が集中力のピークなので○○をここに配置」と具体的に説明\n- {goal_progress} の期限や進捗を引用し、「○○の期限が△週間後なので優先」と説明\n- {weekly_summary} の時間配分を引用し、「先週は○○にN時間偏っていたので△△の時間を確保」と説明\n\n**例:**\n「ポイントは3つです:\n- **19時台が集中力のピーク**なので、Next.jsフロント実装をこの時間帯に配置しました\n- **ACE受験が3週間後**に迫っているので、毎日19:00-19:30のACE対策枠を確保\n- **先週はSaaS開発に15h偏っていた**ので、ブログ執筆の時間を意識的に増やしました」\n\n「集中しやすい時間帯を考慮して」のような抽象的な表現は**禁止**。具体的なデータ・数字・期限に基づく根拠を必ず示す。',
    allowed_tools, max_turns, temperature,
    'AI最適化: エネルギーベース・スケジューリング、目標整合性チェック、感情フォロー、日時範囲の厳密化、提案根拠の明示',
    'ai', 'pending',
    '{"interaction_count": 17, "avg_rating": 4.1, "strengths": ["技術的な質問への回答精度が高い", "コード例を活用した説明が分かりやすい"], "weaknesses": ["行動パターンデータを活用したスケジューリングが不十分", "感情面へのフォローが不足", "目標の時間配分バランスへの言及がない"]}'::jsonb
FROM ai_contexts WHERE id = 'ddc00001-0000-0000-0000-000000000000'::uuid;

-- Now append lighter AI optimization sections to chat context (v2)
UPDATE ai_contexts
SET system_prompt = system_prompt || E'\n\n## パーソナライズ: スケジューリング最適化\nスケジュール提案時は {user_patterns} の生産性データを活用する:\n- 集中タスク（開発、学習、執筆）→ 生産性ピーク時間帯に配置\n- ルーチン作業（MTG準備、メール、整理）→ 低エネルギー時間帯に配置\n- 90分以上の集中ブロックの後には休憩 or 軽作業を入れる\n- 1日のブロック数は4〜5個以内。詰め込みすぎず余白を残す\n- 既に入っている予定と重複しないように配置する\n\n## パーソナライズ: 目標との整合性\n{goal_progress} と {weekly_summary} を確認し:\n- 時間配分の偏りがあれば指摘し、バランス調整を提案する\n- 期限が近いマイルストーンは優先的にスケジューリングする\n- {recent_learning_notes} から学習の継続性を考慮する\n\n## パーソナライズ: 感情への配慮\n- 最近の達成や前向きな変化があれば自然に触れる\n- ユーザーが困っている場合はまず共感してから提案に移る\n\n## パーソナライズ: 日時範囲\nスケジュール提案時:\n- 「今週」= 月曜〜日曜、「来週」= 翌週の月曜〜日曜\n- 期間が曖昧な場合は「今週の残り」として解釈する\n- create_time_block の date が指定期間外にならないよう確認する'
WHERE id = 'ddc00001-0000-0000-0000-000000000000'::uuid;

-- v2: パーソナライズ強化 (manual) — created from the updated ai_contexts row
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'ddc10002-0000-0000-0000-000000000000'::uuid,
    'ddc00001-0000-0000-0000-000000000000'::uuid,
    2,
    system_prompt, allowed_tools, max_turns, temperature,
    'パーソナライズ強化: スケジューリング最適化、目標整合性、感情配慮、日時範囲',
    'manual', NULL, NULL
FROM ai_contexts WHERE id = 'ddc00001-0000-0000-0000-000000000000'::uuid;

-- v1 for review context (manual)
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'ddc10003-0000-0000-0000-000000000000'::uuid,
    'ddc00002-0000-0000-0000-000000000000'::uuid,
    1,
    system_prompt, allowed_tools, max_turns, temperature,
    'テンプレートからコピー',
    'manual', NULL, NULL
FROM ai_contexts WHERE id = 'ddc00002-0000-0000-0000-000000000000'::uuid;

-- v1 for daily_advice context (manual)
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'ddc10004-0000-0000-0000-000000000000'::uuid,
    'ddc00003-0000-0000-0000-000000000000'::uuid,
    1,
    system_prompt, allowed_tools, max_turns, temperature,
    'テンプレートからコピー',
    'manual', NULL, NULL
FROM ai_contexts WHERE id = 'ddc00003-0000-0000-0000-000000000000'::uuid;

-- v1 for persona context (manual)
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'ddc10005-0000-0000-0000-000000000000'::uuid,
    'ddc00004-0000-0000-0000-000000000000'::uuid,
    1,
    system_prompt, allowed_tools, max_turns, temperature,
    'テンプレートからコピー',
    'manual', NULL, NULL
FROM ai_contexts WHERE id = 'ddc00004-0000-0000-0000-000000000000'::uuid;

-- review v2 (AI optimization candidate; v1=template)
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'ddc40002-0000-0000-0000-000000000000'::uuid,
    'ddc00002-0000-0000-0000-000000000000'::uuid,
    2,
    system_prompt || E'\n\n## AI最適化ガイドライン\n- 週次レビューでは具体的な数値目標を提示する\n- 前週との比較で進捗を可視化する\n- 達成できなかった項目には代替案を提案する',
    allowed_tools, max_turns, temperature,
    'AI最適化: 数値目標と前週比較の強化',
    'ai', 'pending',
    '{"interaction_count": 12, "avg_rating": 3.8, "strengths": ["週次の振り返り構成が体系的", "データに基づいた分析が正確"], "weaknesses": ["改善提案が抽象的になりがち", "数値目標の提示が少ない"]}'::jsonb
FROM ai_contexts WHERE id = 'ddc00002-0000-0000-0000-000000000000'::uuid;

-- daily_advice v2 (AI optimization candidate; v1=template)
INSERT INTO ai_context_versions (
    id, context_id, version_number,
    system_prompt, allowed_tools, max_turns, temperature, changelog,
    source, candidate_status, eval_summary
)
SELECT
    'ddc40003-0000-0000-0000-000000000000'::uuid,
    'ddc00003-0000-0000-0000-000000000000'::uuid,
    2,
    system_prompt || E'\n\n## AI最適化ガイドライン\n- 朝のアドバイスは3つ以内に絞る\n- 前日の振り返りを踏まえた提案をする\n- 体調やモチベーションに配慮したスケジュール提案',
    allowed_tools, max_turns, temperature,
    'AI最適化: 提案数の絞り込みと体調配慮',
    'ai', 'pending',
    '{"interaction_count": 22, "avg_rating": 4.3, "strengths": ["スケジュール提案の精度が高い", "ユーザーの習慣パターンを反映"], "weaknesses": ["提案数が多すぎることがある", "体調面への配慮が不足"]}'::jsonb
FROM ai_contexts WHERE id = 'ddc00003-0000-0000-0000-000000000000'::uuid;

-- Set active_version for each context
UPDATE ai_contexts SET active_version = 2 WHERE id = 'ddc00001-0000-0000-0000-000000000000'::uuid;  -- chat (v2 basic is active)
UPDATE ai_contexts SET active_version = 1 WHERE id = 'ddc00002-0000-0000-0000-000000000000'::uuid;  -- review
UPDATE ai_contexts SET active_version = 1 WHERE id = 'ddc00003-0000-0000-0000-000000000000'::uuid;  -- daily_advice
UPDATE ai_contexts SET active_version = 1 WHERE id = 'ddc00004-0000-0000-0000-000000000000'::uuid;  -- persona

-- ==============================================================================
-- Section C: Update ai_interactions with context_id and rating
-- ==============================================================================

-- chat interactions → chat context
UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'dd800001-0000-0000-0000-000000000000'::uuid;  -- SaaS DB設計レビュー（高評価）

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800003-0000-0000-0000-000000000000'::uuid;  -- Cloud Runデプロイ

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 3
WHERE id = 'dd800005-0000-0000-0000-000000000000'::uuid;  -- Google Cloud勉強できてない

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'dd800006-0000-0000-0000-000000000000'::uuid;  -- ACE模擬試験65%

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'dd800008-0000-0000-0000-000000000000'::uuid;  -- Go Conference CFP提出

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800009-0000-0000-0000-000000000000'::uuid;  -- ブログ公開できない（完璧主義）

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800011-0000-0000-0000-000000000000'::uuid;  -- 12日連続学習

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 3
WHERE id = 'dd800012-0000-0000-0000-000000000000'::uuid;  -- Firestore設計パターン

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'dd800014-0000-0000-0000-000000000000'::uuid;  -- ブログ1本公開！

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800016-0000-0000-0000-000000000000'::uuid;  -- Cloud Run vs App Engine

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800017-0000-0000-0000-000000000000'::uuid;  -- LTスライド構成

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800020-0000-0000-0000-000000000000'::uuid;  -- 木曜MTGが辛い

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'dd800021-0000-0000-0000-000000000000'::uuid;  -- GoのHTTPミドルウェア

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 3
WHERE id = 'dd800023-0000-0000-0000-000000000000'::uuid;  -- Next.js App Router

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800025-0000-0000-0000-000000000000'::uuid;  -- SaaS MVP残りタスク

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800026-0000-0000-0000-000000000000'::uuid;  -- ACE最終対策

UPDATE ai_interactions SET context_id = 'ddc00001-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'dd800028-0000-0000-0000-000000000000'::uuid;  -- 来月の目標設定

-- morning interactions → daily_advice context
UPDATE ai_interactions SET context_id = 'ddc00003-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800002-0000-0000-0000-000000000000'::uuid;  -- 今週の計画確認

UPDATE ai_interactions SET context_id = 'ddc00003-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'dd800007-0000-0000-0000-000000000000'::uuid;  -- 夜型学習3週間達成

UPDATE ai_interactions SET context_id = 'ddc00003-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'dd800018-0000-0000-0000-000000000000'::uuid;  -- ACE模擬試験78%

UPDATE ai_interactions SET context_id = 'ddc00003-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800024-0000-0000-0000-000000000000'::uuid;  -- ACE受験日仮予約

-- evening interactions → daily_advice context
UPDATE ai_interactions SET context_id = 'ddc00003-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'dd800004-0000-0000-0000-000000000000'::uuid;  -- ACE問題集進捗

UPDATE ai_interactions SET context_id = 'ddc00003-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800010-0000-0000-0000-000000000000'::uuid;  -- Next.js苦戦

UPDATE ai_interactions SET context_id = 'ddc00003-0000-0000-0000-000000000000'::uuid, rating = 5
WHERE id = 'dd800015-0000-0000-0000-000000000000'::uuid;  -- 8週間振り返り

UPDATE ai_interactions SET context_id = 'ddc00003-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800022-0000-0000-0000-000000000000'::uuid;  -- CFP通過！

-- briefing interactions → daily_advice context
UPDATE ai_interactions SET context_id = 'ddc00003-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800013-0000-0000-0000-000000000000'::uuid;  -- ブリーフィング

UPDATE ai_interactions SET context_id = 'ddc00003-0000-0000-0000-000000000000'::uuid, rating = 3
WHERE id = 'dd800019-0000-0000-0000-000000000000'::uuid;  -- ブリーフィング

UPDATE ai_interactions SET context_id = 'ddc00003-0000-0000-0000-000000000000'::uuid, rating = 4
WHERE id = 'dd800027-0000-0000-0000-000000000000'::uuid;  -- 週末ブリーフィング

-- ==============================================================================
-- Section E: Prompt Evaluations
-- ==============================================================================

INSERT INTO prompt_evaluations (
    id, context_id, period_start, period_end,
    interaction_count, avg_rating, rated_count,
    strengths, weaknesses, improvement_suggestions,
    user_id
) VALUES
(
    'ddc50001-0000-0000-0000-000000000000'::uuid,
    'ddc00001-0000-0000-0000-000000000000'::uuid,
    CURRENT_DATE - 7, CURRENT_DATE,
    17, 4.1, 17,
    ARRAY['技術的な質問への回答精度が高い', 'コード例を活用した説明が分かりやすい'],
    ARRAY['行動パターンデータを活用したスケジューリングが不十分', '感情面へのフォローが不足', '目標の時間配分バランスへの言及がない'],
    ARRAY['行動パターン・生産性ピークに基づくスケジュール最適化', 'ユーザーの達成に触れる感情フォローの追加', '目標別の時間配分チェック'],
    'dddddddd-dddd-dddd-dddd-dddddddddddd'::uuid
),
(
    'ddc50002-0000-0000-0000-000000000000'::uuid,
    'ddc00002-0000-0000-0000-000000000000'::uuid,
    CURRENT_DATE - 7, CURRENT_DATE,
    12, 3.8, 10,
    ARRAY['週次の振り返り構成が体系的', 'データに基づいた分析が正確'],
    ARRAY['改善提案が抽象的になりがち', '数値目標の提示が少ない'],
    ARRAY['具体的な数値目標の提示', '前週比較の自動化'],
    'dddddddd-dddd-dddd-dddd-dddddddddddd'::uuid
),
(
    'ddc50003-0000-0000-0000-000000000000'::uuid,
    'ddc00003-0000-0000-0000-000000000000'::uuid,
    CURRENT_DATE - 7, CURRENT_DATE,
    22, 4.3, 20,
    ARRAY['スケジュール提案の精度が高い', 'ユーザーの習慣パターンを反映'],
    ARRAY['提案数が多すぎることがある', '体調面への配慮が不足'],
    ARRAY['提案を3つ以内に絞る', '体調・モチベーション考慮の追加'],
    'dddddddd-dddd-dddd-dddd-dddddddddddd'::uuid
);
