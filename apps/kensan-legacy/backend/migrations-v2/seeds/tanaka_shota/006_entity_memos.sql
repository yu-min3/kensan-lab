-- ============================================================================
-- Demo Seed: Entity Memos (~15件)
-- ============================================================================
-- ゴール・マイルストーン・タスクに紐づくメモ

INSERT INTO entity_memos (id, user_id, entity_type, entity_id, content, pinned, created_at, updated_at) VALUES

-- Goal: Google Cloudスキルアップ
('dd600001-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'goal', 'dd000001-0000-0000-0000-000000000000',
 'ACE模擬試験2回目で78%。合格ライン見えてきた。Networkingをもう少し固めれば行ける。',
 true, CURRENT_DATE - 8, CURRENT_DATE - 8),

('dd600002-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'goal', 'dd000001-0000-0000-0000-000000000000',
 'ACE→PCAのステップアップが理想。ACEで基礎を固めて、PCAで設計力を証明する流れ。',
 false, CURRENT_DATE - 30, CURRENT_DATE - 30),

-- Goal: 技術アウトプット
('dd600003-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'goal', 'dd000002-0000-0000-0000-000000000000',
 'ブログ1本目公開できた！はてブ3件。思ったより反応があって嬉しい。完璧じゃなくても出すことが大事。',
 true, CURRENT_DATE - 12, CURRENT_DATE - 12),

('dd600004-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'goal', 'dd000002-0000-0000-0000-000000000000',
 'Go ConferenceのCFPが通った。5分LTだけど初登壇は緊張する。',
 false, CURRENT_DATE - 10, CURRENT_DATE - 10),

-- Goal: 個人開発プロダクト
('dd600005-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'goal', 'dd000003-0000-0000-0000-000000000000',
 'Go APIはほぼ完成。Next.jsフロントが残り。Cloud Runへのデプロイも試したい。',
 true, CURRENT_DATE - 15, CURRENT_DATE - 15),

('dd600006-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'goal', 'dd000003-0000-0000-0000-000000000000',
 'リポジトリ名は「taskai」に決定。Go + Cloud Run + Firestore。',
 false, CURRENT_DATE - 50, CURRENT_DATE - 50),

-- Milestone: ACE合格
('dd600007-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'milestone', 'dd010001-0000-0000-0000-000000000000',
 '1回目65% → 2回目78%。あと少しで合格ライン。受験日を来月に仮予約した。',
 true, CURRENT_DATE - 8, CURRENT_DATE - 8),

-- Milestone: ブログ月4本
('dd600008-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'milestone', 'dd020001-0000-0000-0000-000000000000',
 '下書き3本溜まってるのに公開できない自分が情けない。でも1本出せた。2本目のハードルは下がったはず。',
 false, CURRENT_DATE - 12, CURRENT_DATE - 12),

-- Milestone: SaaS MVPリリース
('dd600009-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'milestone', 'dd030001-0000-0000-0000-000000000000',
 'DB設計はスムーズだった。API実装も楽しい。問題はフロント...Next.jsのApp Routerに苦戦中。',
 false, CURRENT_DATE - 22, CURRENT_DATE - 22),

-- Task: Cloud ACE公式問題集
('dd600010-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'task', 'dd100001-0000-0000-0000-000000000000',
 'Compute EngineとGKEの問題が多い。IAMのサービスアカウント周りが特に複雑。',
 false, CURRENT_DATE - 40, CURRENT_DATE - 40),

-- Task: Go Conference CFP提出
('dd600011-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'task', 'dd100007-0000-0000-0000-000000000000',
 '「GoでCloud Runを使い倒す」で応募。実体験ベースの話ができるので自信あり。',
 true, CURRENT_DATE - 36, CURRENT_DATE - 36),

-- Task: Next.jsフロント実装
('dd600012-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'task', 'dd100011-0000-0000-0000-000000000000',
 'App RouterのServer ComponentsとClient Componentsの切り分けに苦戦。バックエンドエンジニアには馴染みにくい。',
 false, CURRENT_DATE - 24, CURRENT_DATE - 24),

('dd600013-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'task', 'dd100011-0000-0000-0000-000000000000',
 'やっとApp Routerに慣れてきた。Server Actionsが便利。API呼び出しがシンプルになる。',
 false, CURRENT_DATE - 15, CURRENT_DATE - 15),

-- Task: ブログ：Go×Cloud Run構成紹介
('dd600014-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'task', 'dd100005-0000-0000-0000-000000000000',
 '構成案：1.Cloud Runの概要 2.Goとの相性 3.Dockerfileの書き方 4.CI/CDパイプライン 5.コールドスタート対策',
 false, CURRENT_DATE - 26, CURRENT_DATE - 26),

-- Milestone: LT登壇年3回
('dd600015-0000-0000-0000-000000000000', 'dddddddd-dddd-dddd-dddd-dddddddddddd',
 'milestone', 'dd020002-0000-0000-0000-000000000000',
 'CFP通過！初LT。スライドの構成を練り始めた。デモを入れるかどうか悩む。',
 true, CURRENT_DATE - 10, CURRENT_DATE - 10);
