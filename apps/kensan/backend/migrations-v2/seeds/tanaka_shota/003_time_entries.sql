-- ============================================================================
-- Demo Seed: Time Entries — 田中翔太 (実績データ ~210件)
-- ============================================================================
-- 計画の約85%を実行。夜型バックエンドエンジニア（Go + Google Cloud）。
-- 木曜はMTG日で疲れて ~40% スキップ。ブログ系は実際の作業時間が短め。
-- 全時刻はUTC（JST - 9h）。
--
-- JST→UTC変換:
--   JST 10:00 = UTC 01:00
--   JST 12:15 = UTC 03:15
--   JST 12:45 = UTC 03:45
--   JST 14:00 = UTC 05:00
--   JST 15:00 = UTC 06:00
--   JST 18:00 = UTC 09:00
--   JST 19:00 = UTC 10:00
--   JST 20:30 = UTC 11:30
--   JST 21:00 = UTC 12:00
--   JST 21:30 = UTC 12:30

DO $$
DECLARE
    base_date DATE := CURRENT_DATE - 56;  -- 8 weeks ago
    d DATE;
    day_offset INT;
    dow INT;  -- 0=Sun ... 6=Sat
    week_num INT;
    seq INT := 0;
    skip_evening BOOLEAN;
    skip_lunch BOOLEAN;
    skip_thu BOOLEAN;
    skip_gym BOOLEAN;

    -- User ID
    uid UUID := 'dddddddd-dddd-dddd-dddd-dddddddddddd';

    -- Goal info (denormalized)
    g1_id UUID := 'dd000001-0000-0000-0000-000000000000';  -- Google Cloud
    g1_name TEXT := 'Google Cloudスキルアップ';
    g1_color TEXT := '#0EA5E9';
    g2_id UUID := 'dd000002-0000-0000-0000-000000000000';  -- 技術アウトプット
    g2_name TEXT := '技術アウトプット';
    g2_color TEXT := '#F59E0B';
    g3_id UUID := 'dd000003-0000-0000-0000-000000000000';  -- 個人開発
    g3_name TEXT := '個人開発プロダクト';
    g3_color TEXT := '#10B981';

    -- Milestone info
    m_ace UUID := 'dd010001-0000-0000-0000-000000000000';     -- ACE合格
    m_pca UUID := 'dd010002-0000-0000-0000-000000000000';     -- PCA合格
    m_blog UUID := 'dd020001-0000-0000-0000-000000000000';    -- ブログ月4本
    m_lt UUID := 'dd020002-0000-0000-0000-000000000000';      -- LT登壇年3回
    m_mvp UUID := 'dd030001-0000-0000-0000-000000000000';     -- SaaS MVPリリース

    -- Task IDs
    t_ace_q UUID := 'dd100001-0000-0000-0000-000000000000';   -- Cloud ACE公式問題集
    t_ace_mock UUID := 'dd100002-0000-0000-0000-000000000000';-- ACE模擬試験×3回
    t_qwiklabs UUID := 'dd100003-0000-0000-0000-000000000000';-- Qwiklabs ACEラボ
    t_pca UUID := 'dd100004-0000-0000-0000-000000000000';     -- PCA学習ロードマップ
    t_blog1 UUID := 'dd100005-0000-0000-0000-000000000000';   -- ブログ：Cloud Run
    t_blog2 UUID := 'dd100006-0000-0000-0000-000000000000';   -- ブログ：Firestore
    t_cfp UUID := 'dd100007-0000-0000-0000-000000000000';     -- CFP提出
    t_slide UUID := 'dd100008-0000-0000-0000-000000000000';   -- LTスライド
    t_db UUID := 'dd100009-0000-0000-0000-000000000000';      -- DB設計＆API設計
    t_api UUID := 'dd100010-0000-0000-0000-000000000000';     -- Go API実装
    t_next UUID := 'dd100011-0000-0000-0000-000000000000';    -- Next.jsフロント
    t_deploy UUID := 'dd100012-0000-0000-0000-000000000000';  -- Cloud Runデプロイ
    t_book UUID := 'dd100013-0000-0000-0000-000000000000';    -- Go並行処理読了
    t_lt_practice UUID := 'dd100015-0000-0000-0000-000000000000'; -- LT練習

    -- Tag IDs
    tag_dev UUID := 'dd0a0001-0000-0000-0000-000000000000';
    tag_input UUID := 'dd0a0002-0000-0000-0000-000000000000';
    tag_gc UUID := 'dd0a0003-0000-0000-0000-000000000000';
    tag_exercise UUID := 'dd0a0004-0000-0000-0000-000000000000';
    tag_writing UUID := 'dd0a0005-0000-0000-0000-000000000000';
    tag_read UUID := 'dd0a0006-0000-0000-0000-000000000000';

    -- Time jitter (minutes)
    jitter INT;

BEGIN
    FOR day_offset IN 0..55 LOOP
        d := base_date + day_offset;
        dow := EXTRACT(DOW FROM d)::INT;  -- 0=Sun, 1=Mon, ...6=Sat
        week_num := day_offset / 7 + 1;    -- 1-8

        -- Skip patterns: deterministic based on day_offset for reproducibility
        skip_evening := (day_offset % 9 = 0);           -- ~11% skip
        skip_lunch := (day_offset % 10 = 0);             -- ~10% skip
        skip_thu := (day_offset % 5 < 2);                -- ~40% skip on Thursday
        skip_gym := (day_offset % 10 < 3);               -- ~30% skip
        jitter := (day_offset % 5) * 3 - 6;              -- -6, -3, 0, 3, 6 minutes

        -- ============================================================
        -- WEEKDAY ENTRIES (Mon-Fri)
        -- ============================================================
        IF dow BETWEEN 1 AND 5 THEN

            -- === Lunch block: 12:15-12:45 JST (UTC: 03:15-03:45) ===
            -- ~90% completion (skip_lunch ~10%)
            IF NOT skip_lunch THEN
                seq := seq + 1;
                INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                    task_name, goal_id, goal_name, goal_color, tag_ids, description)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '03:15' + ((jitter / 2) || ' minutes')::INTERVAL,
                    d + TIME '03:42' + ((jitter / 2) || ' minutes')::INTERVAL,
                    CASE WHEN day_offset % 2 = 0 THEN '技術ニュース読む' ELSE '英語リーディング' END,
                    NULL, NULL, NULL,
                    ARRAY[tag_input], NULL
                );
            END IF;

            -- === Gym: Wed 18:00-19:00 JST (UTC: 09:00-10:00) ===
            -- ~70% completion (skip_gym ~30%)
            IF dow = 3 AND NOT skip_gym THEN
                seq := seq + 1;
                INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                    task_name, goal_id, goal_name, goal_color, tag_ids, description)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '09:03' + ((jitter / 2) || ' minutes')::INTERVAL,
                    d + TIME '09:55' + ((jitter / 2) || ' minutes')::INTERVAL,
                    'ジム', NULL, NULL, NULL,
                    ARRAY[tag_exercise], NULL
                );
            END IF;

            -- === Evening main block: 19:00-21:00 JST (UTC: 10:00-12:00) ===
            -- Thursday: 20:30-21:30 JST (UTC: 11:30-12:30) with ~40% skip
            IF dow <> 4 THEN
                -- Non-Thursday: ~89% completion (skip_evening ~11%)
                IF NOT skip_evening THEN
                    seq := seq + 1;
                    IF week_num <= 2 THEN
                        -- Week 1-2: Mon-Wed=ACE/Qwiklabs, Thu-Fri=SaaS DB
                        IF dow <= 3 THEN
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '10:03' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '11:52' + (jitter || ' minutes')::INTERVAL,
                                CASE WHEN week_num = 1 THEN t_ace_q ELSE t_qwiklabs END,
                                CASE WHEN week_num = 1 THEN 'Cloud ACE公式問題集' ELSE 'Qwiklabs ACEラボ' END,
                                m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                                ARRAY[tag_input, tag_gc], NULL
                            );
                        ELSE
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '10:05' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '11:55' + (jitter || ' minutes')::INTERVAL,
                                t_db, 'DB設計＆API設計', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                                ARRAY[tag_dev], NULL
                            );
                        END IF;
                    ELSIF week_num <= 4 THEN
                        -- Week 3-4: Mon-Wed=SaaS API, Thu-Fri=ACE mock
                        IF dow <= 3 THEN
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '10:05' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '11:58' + (jitter || ' minutes')::INTERVAL,
                                t_api, 'Go API実装', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                                ARRAY[tag_dev], NULL
                            );
                        ELSE
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '10:03' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '11:50' + (jitter || ' minutes')::INTERVAL,
                                t_ace_mock, 'ACE模擬試験×3回', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                                ARRAY[tag_input, tag_gc], NULL
                            );
                        END IF;
                    ELSIF week_num <= 6 THEN
                        -- Week 5-6: Mon-Tue=Blog, Wed-Fri=Next.js
                        IF dow <= 2 THEN
                            -- Blog entries: shorter actual time (planned 2h, actual ~1h20m-1h40m)
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '10:05' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '11:30' + (jitter || ' minutes')::INTERVAL,
                                CASE WHEN week_num = 5 THEN t_blog1 ELSE t_blog2 END,
                                CASE WHEN week_num = 5 THEN 'ブログ：Go×Cloud Run構成紹介' ELSE 'ブログ：Firestore設計パターン' END,
                                m_blog, 'ブログ月4本', g2_id, g2_name, g2_color,
                                ARRAY[tag_writing],
                                CASE WHEN week_num = 5 AND dow = 1 THEN 'アウトライン作成。構成が難しい'
                                     WHEN week_num = 5 AND dow = 2 THEN '下書き半分。コード例の準備に時間かかった'
                                     WHEN week_num = 6 AND dow = 1 THEN 'Firestore設計の整理。NoSQLパターンまとめ'
                                     ELSE NULL END
                            );
                        ELSE
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '10:03' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '11:55' + (jitter || ' minutes')::INTERVAL,
                                t_next, 'Next.jsフロント実装', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                                ARRAY[tag_dev], NULL
                            );
                        END IF;
                    ELSE
                        -- Week 7-8: Mon-Tue=LT slides, Wed=Next.js, Fri=ACE mock
                        IF dow <= 2 THEN
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '10:05' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '11:50' + (jitter || ' minutes')::INTERVAL,
                                t_slide, 'LTスライド作成', m_lt, 'LT登壇年3回', g2_id, g2_name, g2_color,
                                ARRAY[tag_writing], NULL
                            );
                        ELSIF dow = 3 THEN
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '10:03' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '11:52' + (jitter || ' minutes')::INTERVAL,
                                t_next, 'Next.jsフロント実装', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                                ARRAY[tag_dev], NULL
                            );
                        ELSE
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '10:03' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '11:48' + (jitter || ' minutes')::INTERVAL,
                                t_ace_mock, 'ACE模擬試験×3回', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                                ARRAY[tag_input, tag_gc], NULL
                            );
                        END IF;
                    END IF;
                END IF;
            ELSE
                -- Thursday (MTG day): shorter evening block 20:30-21:30 JST (UTC: 11:30-12:30)
                -- ~60% completion (skip_thu ~40%)
                IF NOT skip_thu THEN
                    seq := seq + 1;
                    IF week_num <= 4 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '11:35' + ((jitter / 2) || ' minutes')::INTERVAL,
                            d + TIME '12:22' + ((jitter / 2) || ' minutes')::INTERVAL,
                            t_ace_q, 'Cloud ACE公式問題集', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                            ARRAY[tag_input, tag_gc],
                            CASE WHEN week_num = 2 THEN 'MTG後で疲れたけど30分だけ' ELSE NULL END
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '11:35' + ((jitter / 2) || ' minutes')::INTERVAL,
                            d + TIME '12:20' + ((jitter / 2) || ' minutes')::INTERVAL,
                            t_book, '「Go言語による並行処理」読了', NULL, NULL, NULL, NULL, NULL,
                            ARRAY[tag_read],
                            CASE WHEN week_num = 6 THEN 'Chapter 5: goroutineパターン。面白い' ELSE NULL END
                        );
                    END IF;
                END IF;
            END IF;

            -- === Evening block 2: 21:00-21:30 JST (UTC: 12:00-12:30) ===
            -- Non-Thursday, W1-6 only. Skip with evening skip.
            IF dow <> 4 AND week_num <= 6 AND NOT skip_evening THEN
                -- Additional ~15% skip for this lighter block
                IF day_offset % 7 <> 0 THEN
                    seq := seq + 1;
                    IF day_offset % 2 = 0 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '12:02' + ((jitter / 2) || ' minutes')::INTERVAL,
                            d + TIME '12:25' + ((jitter / 2) || ' minutes')::INTERVAL,
                            t_ace_q, 'Cloud ACE公式問題集', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                            ARRAY[tag_gc], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '12:02' + ((jitter / 2) || ' minutes')::INTERVAL,
                            d + TIME '12:24' + ((jitter / 2) || ' minutes')::INTERVAL,
                            t_book, '「Go言語による並行処理」読了', NULL, NULL, NULL,
                            ARRAY[tag_read], NULL
                        );
                    END IF;
                END IF;
            END IF;

            -- === CFP作業: Week 3-4 Fri 21:00-21:30 JST ===
            IF week_num BETWEEN 3 AND 4 AND dow = 5 AND NOT skip_evening THEN
                seq := seq + 1;
                INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                    task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '12:05' + ((jitter / 2) || ' minutes')::INTERVAL,
                    d + TIME '12:28' + ((jitter / 2) || ' minutes')::INTERVAL,
                    t_cfp, 'Go Conference CFP提出', m_lt, 'LT登壇年3回', g2_id, g2_name, g2_color,
                    ARRAY[tag_writing],
                    CASE WHEN week_num = 3 THEN 'テーマ決め。Cloud Run × Goの運用知見で行く'
                         ELSE 'CFP提出完了！通るといいな' END
                );
            END IF;

        -- ============================================================
        -- WEEKEND ENTRIES (Sat=6, Sun=0)
        -- ============================================================
        ELSE
            IF dow = 6 THEN
                -- Saturday morning: まとまった開発 10:00-13:00 JST (UTC: 01:00-04:00)
                -- High completion on weekends (~90%)
                IF day_offset % 11 <> 0 THEN
                    seq := seq + 1;
                    IF week_num <= 2 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '01:08', d + TIME '03:52',
                            t_db, 'DB設計＆API設計', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                            ARRAY[tag_dev],
                            CASE WHEN week_num = 1 THEN 'ER図完成。users, projects, tasksテーブル'
                                 ELSE 'API設計書ドラフト。RESTful設計で進める' END
                        );
                    ELSIF week_num <= 4 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '01:05', d + TIME '03:55',
                            t_api, 'Go API実装', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                            ARRAY[tag_dev],
                            CASE WHEN week_num = 3 THEN 'CRUD基本実装完了。chi routerいい感じ'
                                 ELSE 'ミドルウェア追加。認証・ログ・エラーハンドリング' END
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '01:10', d + TIME '03:48',
                            t_next, 'Next.jsフロント実装', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                            ARRAY[tag_dev],
                            CASE WHEN week_num = 5 THEN 'プロジェクト初期設定。Tailwind + shadcn/ui'
                                 WHEN week_num = 6 THEN 'ダッシュボード画面。API連携'
                                 WHEN week_num = 7 THEN 'タスク一覧・詳細画面。CRUD動く'
                                 ELSE 'デプロイ準備。Dockerfileとcloud run設定' END
                        );
                    END IF;
                END IF;

                -- Saturday afternoon: ジム 15:00-16:00 JST (UTC: 06:00-07:00)
                -- ~70% completion
                IF NOT skip_gym THEN
                    seq := seq + 1;
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '06:05', d + TIME '06:52',
                        'ジム', NULL, NULL, NULL,
                        ARRAY[tag_exercise], NULL
                    );
                END IF;

                -- Saturday: ACE模擬試験 on Week 3 and Week 7
                IF week_num = 3 OR week_num = 7 THEN
                    seq := seq + 1;
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids,
                        description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '07:35', d + TIME '09:50',
                        t_ace_mock, 'ACE模擬試験×3回', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                        ARRAY[tag_input, tag_gc],
                        CASE WHEN week_num = 3 THEN '結果: 65%。Compute EngineとIAMが弱い'
                             ELSE '結果: 78%。前回より改善。Networkingがまだ弱い' END
                    );
                END IF;

            ELSE
                -- Sunday: 軽い学習 14:00-15:30 JST (UTC: 05:00-06:30)
                -- ~85% completion (skip on some days)
                IF day_offset % 8 <> 0 THEN
                    seq := seq + 1;
                    IF week_num <= 4 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '05:05', d + TIME '06:18',
                            t_ace_q, 'Cloud ACE公式問題集', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                            ARRAY[tag_input, tag_gc],
                            CASE WHEN week_num = 2 THEN 'IAMセクション。ロールの階層が複雑'
                                 WHEN week_num = 4 THEN 'GKEセクション復習。だいぶ理解できてきた'
                                 ELSE NULL END
                        );
                    ELSIF week_num <= 6 THEN
                        -- Blog on Sunday: shorter actual time
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '05:08', d + TIME '06:05',
                            CASE WHEN week_num = 5 THEN t_blog1 ELSE t_blog2 END,
                            CASE WHEN week_num = 5 THEN 'ブログ：Go×Cloud Run構成紹介' ELSE 'ブログ：Firestore設計パターン' END,
                            m_blog, 'ブログ月4本', g2_id, g2_name, g2_color,
                            ARRAY[tag_writing],
                            CASE WHEN week_num = 5 THEN '推敲とコード例確認。来週公開できそう'
                                 ELSE 'Firestoreの設計パターン整理中' END
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '05:05', d + TIME '06:15',
                            t_slide, 'LTスライド作成', m_lt, 'LT登壇年3回', g2_id, g2_name, g2_color,
                            ARRAY[tag_writing],
                            CASE WHEN week_num = 7 THEN 'スライド構成見直し。デモ部分追加'
                                 ELSE 'リハーサル1回目。時間オーバー気味' END
                        );
                    END IF;
                END IF;
            END IF;
        END IF;

    END LOOP;

    RAISE NOTICE 'Tanaka Shota: Inserted % time entry operations', seq;
END $$;
