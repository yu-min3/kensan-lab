-- ============================================================================
-- Demo Seed: User & Master Data
-- ============================================================================
-- Persona: 田中翔太 (Tanaka Shota) — 30歳バックエンドエンジニア
-- Go + Google Cloud で5年。BtoB SaaS企業でAPI・マイクロサービスを担当。
-- 夜型: 19:00-21:30がゴールデンタイム。ハンズオン学習スタイル。
-- ブログ公開が怖い（完璧主義）。木曜MTGで学習ブロック。
-- Email: demo@kensan.dev / Password: demo1234

-- ==============================================================================
-- Demo User
-- ==============================================================================
-- Password hash for 'demo1234' (bcrypt cost 12, $2a$ prefix for Go compatibility)
INSERT INTO users (id, email, name, password_hash) VALUES
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', 'demo@kensan.dev', '田中翔太',
     '$2a$12$DCYla1Nq7wClb/5ycI4aHuz2Hynp9VLXvBjH38heLdNlTishoa3rm');

INSERT INTO user_settings (user_id, timezone, theme, is_configured, ai_enabled, ai_consent_given, ai_consented_at) VALUES
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', 'Asia/Tokyo', 'system', true, true, true, NOW() - INTERVAL '60 days');

-- ==============================================================================
-- Goals (3つ)
-- ==============================================================================
INSERT INTO goals (id, user_id, name, description, color, status) VALUES
    ('dd000001-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'Google Cloudスキルアップ', 'Cloud Engineer → Professional Cloud Architect へステップアップ。実務+資格で証明する', '#0EA5E9', 'active'),
    ('dd000002-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     '技術アウトプット', 'ブログ月4本+年3回のLT登壇で、社外にもアウトプットする習慣を作る', '#F59E0B', 'active'),
    ('dd000003-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     '個人開発プロダクト', 'Go + Cloud Run で SaaS MVP をリリースする', '#10B981', 'active');

-- ==============================================================================
-- Milestones (5つ)
-- ==============================================================================
INSERT INTO milestones (id, user_id, goal_id, name, description, target_date, status) VALUES
    -- Google Cloudスキルアップ
    ('dd010001-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd000001-0000-0000-0000-000000000000', 'ACE合格',
     'Associate Cloud Engineer 試験に合格する',
     (CURRENT_DATE + INTERVAL '40 days')::DATE, 'active'),
    ('dd010002-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd000001-0000-0000-0000-000000000000', 'PCA合格',
     'Professional Cloud Architect 試験に合格する',
     (CURRENT_DATE + INTERVAL '120 days')::DATE, 'active'),
    -- 技術アウトプット
    ('dd020001-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd000002-0000-0000-0000-000000000000', 'ブログ月4本',
     '技術ブログを月4本ペースで公開する。まずは1本目を出す',
     (CURRENT_DATE + INTERVAL '30 days')::DATE, 'active'),
    ('dd020002-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd000002-0000-0000-0000-000000000000', 'LT登壇年3回',
     'Go ConferenceやGDGなどで年3回LT登壇する',
     (CURRENT_DATE + INTERVAL '90 days')::DATE, 'active'),
    -- 個人開発プロダクト
    ('dd030001-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd000003-0000-0000-0000-000000000000', 'SaaS MVPリリース',
     'Go + Cloud Run + Firestore で MVP を作ってデプロイする',
     (CURRENT_DATE + INTERVAL '60 days')::DATE, 'active');

-- ==============================================================================
-- Tags (7つ)
-- ==============================================================================
INSERT INTO tags (id, user_id, name, color, type) VALUES
    ('dd0a0001-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', '開発', '#8B5CF6', 'task'),
    ('dd0a0002-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'Input', '#06B6D4', 'task'),
    ('dd0a0003-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'Google Cloud', '#4285F4', 'task'),
    ('dd0a0004-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', '運動', '#EF4444', 'task'),
    ('dd0a0005-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', '執筆', '#F59E0B', 'task'),
    ('dd0a0006-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', '読書', '#84CC16', 'task'),
    ('dd0a0007-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd', '振り返り', '#A855F7', 'task');

-- ==============================================================================
-- Tasks (15個)
-- ==============================================================================
INSERT INTO tasks (id, user_id, milestone_id, name, estimated_minutes, completed) VALUES
    -- ACE合格
    ('dd100001-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd010001-0000-0000-0000-000000000000', 'Cloud ACE公式問題集', 120, true),
    ('dd100002-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd010001-0000-0000-0000-000000000000', 'ACE模擬試験×3回', 90, false),
    ('dd100003-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd010001-0000-0000-0000-000000000000', 'Qwiklabs ACEラボ', 60, true),
    -- PCA合格
    ('dd100004-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd010002-0000-0000-0000-000000000000', 'PCA学習ロードマップ作成', 60, false),
    -- ブログ月4本
    ('dd100005-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd020001-0000-0000-0000-000000000000', 'ブログ：Go×Cloud Run構成紹介', 90, false),
    ('dd100006-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd020001-0000-0000-0000-000000000000', 'ブログ：Firestore設計パターン', 90, false),
    -- LT登壇年3回
    ('dd100007-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd020002-0000-0000-0000-000000000000', 'Go Conference CFP提出', 60, true),
    ('dd100008-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd020002-0000-0000-0000-000000000000', 'LTスライド作成', 120, false),
    -- SaaS MVPリリース
    ('dd100009-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd030001-0000-0000-0000-000000000000', 'DB設計＆API設計', 180, true),
    ('dd100010-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd030001-0000-0000-0000-000000000000', 'Go API実装', 300, true),
    ('dd100011-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd030001-0000-0000-0000-000000000000', 'Next.jsフロント実装', 240, false),
    -- SaaS MVP追加
    ('dd100012-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd030001-0000-0000-0000-000000000000', 'Cloud Run本番デプロイ', 120, false),
    -- 独立タスク
    ('dd100013-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     NULL, '「Go言語による並行処理」読了', 90, false),
    -- ACE追加
    ('dd100014-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd010001-0000-0000-0000-000000000000', 'ACE受験申込＆日程確定', 30, true),
    -- LT追加
    ('dd100015-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'dd020002-0000-0000-0000-000000000000', 'LT練習・リハーサル', 60, false);

-- ==============================================================================
-- Task-Tags
-- ==============================================================================
INSERT INTO task_tags (task_id, tag_id) VALUES
    -- 開発タグ
    ('dd100009-0000-0000-0000-000000000000', 'dd0a0001-0000-0000-0000-000000000000'),
    ('dd100010-0000-0000-0000-000000000000', 'dd0a0001-0000-0000-0000-000000000000'),
    ('dd100011-0000-0000-0000-000000000000', 'dd0a0001-0000-0000-0000-000000000000'),
    ('dd100012-0000-0000-0000-000000000000', 'dd0a0001-0000-0000-0000-000000000000'),
    -- Inputタグ
    ('dd100001-0000-0000-0000-000000000000', 'dd0a0002-0000-0000-0000-000000000000'),
    ('dd100002-0000-0000-0000-000000000000', 'dd0a0002-0000-0000-0000-000000000000'),
    ('dd100003-0000-0000-0000-000000000000', 'dd0a0002-0000-0000-0000-000000000000'),
    ('dd100004-0000-0000-0000-000000000000', 'dd0a0002-0000-0000-0000-000000000000'),
    ('dd100013-0000-0000-0000-000000000000', 'dd0a0002-0000-0000-0000-000000000000'),
    -- Google Cloudタグ
    ('dd100001-0000-0000-0000-000000000000', 'dd0a0003-0000-0000-0000-000000000000'),
    ('dd100002-0000-0000-0000-000000000000', 'dd0a0003-0000-0000-0000-000000000000'),
    ('dd100003-0000-0000-0000-000000000000', 'dd0a0003-0000-0000-0000-000000000000'),
    ('dd100004-0000-0000-0000-000000000000', 'dd0a0003-0000-0000-0000-000000000000'),
    -- 運動タグ
    ('dd100012-0000-0000-0000-000000000000', 'dd0a0004-0000-0000-0000-000000000000'),
    -- 執筆タグ
    ('dd100005-0000-0000-0000-000000000000', 'dd0a0005-0000-0000-0000-000000000000'),
    ('dd100006-0000-0000-0000-000000000000', 'dd0a0005-0000-0000-0000-000000000000'),
    ('dd100007-0000-0000-0000-000000000000', 'dd0a0005-0000-0000-0000-000000000000'),
    ('dd100008-0000-0000-0000-000000000000', 'dd0a0005-0000-0000-0000-000000000000'),
    ('dd100015-0000-0000-0000-000000000000', 'dd0a0005-0000-0000-0000-000000000000'),
    -- 読書タグ
    ('dd100013-0000-0000-0000-000000000000', 'dd0a0006-0000-0000-0000-000000000000');

-- ==============================================================================
-- Todos (ルーティン4つ)
-- ==============================================================================
INSERT INTO todos (id, user_id, name, frequency, days_of_week, estimated_minutes, tag_ids, enabled) VALUES
    ('dd0b0001-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     '技術ニュース読む', 'daily', NULL, 15,
     ARRAY['dd0a0002-0000-0000-0000-000000000000']::UUID[], true),
    ('dd0b0002-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'Google Cloud問題集', 'daily', NULL, 30,
     ARRAY['dd0a0003-0000-0000-0000-000000000000']::UUID[], true),
    ('dd0b0003-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     '英語リーディング', 'daily', NULL, 20,
     ARRAY['dd0a0002-0000-0000-0000-000000000000']::UUID[], true),
    ('dd0b0004-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
     'ジム', 'custom', ARRAY[3, 6], 60,
     ARRAY['dd0a0004-0000-0000-0000-000000000000']::UUID[], true);
