-- ============================================================================
-- Demo Seed: Time Blocks (計画データ ~250件)
-- ============================================================================
-- 8週間分の計画データ。CURRENT_DATE基準の相対日付。
-- 夜型: 19:00-21:30 JST がメイン学習時間
-- 平日: 2-4ブロック/日、週末: 2-3ブロック
-- 全時刻はUTC（Asia/Tokyo - 9h）
--
-- JST→UTC変換:
--   JST 10:00 = UTC 01:00
--   JST 12:15 = UTC 03:15
--   JST 12:45 = UTC 03:45
--   JST 14:00 = UTC 05:00
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

BEGIN
    FOR day_offset IN 0..55 LOOP
        d := base_date + day_offset;
        dow := EXTRACT(DOW FROM d)::INT;  -- 0=Sun, 1=Mon, ...6=Sat
        week_num := day_offset / 7 + 1;    -- 1-8

        -- ============================================================
        -- WEEKDAY BLOCKS (Mon-Fri)
        -- ============================================================
        IF dow BETWEEN 1 AND 5 THEN

            -- === Lunch block: 12:15-12:45 JST (UTC: 03:15-03:45) ===
            seq := seq + 1;
            INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                task_id, task_name, goal_id, goal_name, goal_color, tag_ids)
            VALUES (
                uuid_generate_v4(), uid,
                d + TIME '03:15', d + TIME '03:45',
                NULL, CASE WHEN day_offset % 2 = 0 THEN '技術ニュース読む' ELSE '英語リーディング' END,
                NULL, NULL, NULL,
                ARRAY[tag_input]
            );

            -- === Gym blocks: Wed 18:00-19:00 JST (UTC: 09:00-10:00) ===
            IF dow = 3 THEN
                seq := seq + 1;
                INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                    task_name, goal_id, goal_name, goal_color, tag_ids)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '09:00', d + TIME '10:00',
                    'ジム', NULL, NULL, NULL,
                    ARRAY[tag_exercise]
                );
            END IF;

            -- === Evening main block: 19:00-21:00 JST (UTC: 10:00-12:00) ===
            -- Thursday is MTG day → shorter or skipped
            IF dow <> 4 THEN
                seq := seq + 1;
                IF week_num <= 2 THEN
                    -- Week 1-2: Google Cloud ACE対策 + SaaS DB設計
                    IF dow <= 3 THEN
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '10:00', d + TIME '12:00',
                            CASE WHEN week_num = 1 THEN t_ace_q ELSE t_qwiklabs END,
                            CASE WHEN week_num = 1 THEN 'Cloud ACE公式問題集' ELSE 'Qwiklabs ACEラボ' END,
                            m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                            ARRAY[tag_input, tag_gc]
                        );
                    ELSE
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '10:00', d + TIME '12:00',
                            t_db, 'DB設計＆API設計', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                            ARRAY[tag_dev]
                        );
                    END IF;
                ELSIF week_num <= 4 THEN
                    -- Week 3-4: SaaS API実装 + ACE模擬
                    IF dow <= 3 THEN
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '10:00', d + TIME '12:00',
                            t_api, 'Go API実装', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                            ARRAY[tag_dev]
                        );
                    ELSE
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '10:00', d + TIME '12:00',
                            t_ace_mock, 'ACE模擬試験×3回', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                            ARRAY[tag_input, tag_gc]
                        );
                    END IF;
                ELSIF week_num <= 6 THEN
                    -- Week 5-6: ブログ執筆 + Next.js
                    IF dow <= 2 THEN
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '10:00', d + TIME '12:00',
                            t_blog1, 'ブログ：Go×Cloud Run構成紹介', m_blog, 'ブログ月4本', g2_id, g2_name, g2_color,
                            ARRAY[tag_writing]
                        );
                    ELSE
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '10:00', d + TIME '12:00',
                            t_next, 'Next.jsフロント実装', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                            ARRAY[tag_dev]
                        );
                    END IF;
                ELSE
                    -- Week 7-8: LTスライド + 追い込み
                    IF dow <= 2 THEN
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '10:00', d + TIME '12:00',
                            t_slide, 'LTスライド作成', m_lt, 'LT登壇年3回', g2_id, g2_name, g2_color,
                            ARRAY[tag_writing]
                        );
                    ELSIF dow = 3 THEN
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '10:00', d + TIME '12:00',
                            t_next, 'Next.jsフロント実装', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                            ARRAY[tag_dev]
                        );
                    ELSE
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '10:00', d + TIME '12:00',
                            t_ace_mock, 'ACE模擬試験×3回', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                            ARRAY[tag_input, tag_gc]
                        );
                    END IF;
                END IF;
            ELSE
                -- Thursday (MTG day): shorter evening block 20:30-21:30 JST (UTC: 11:30-12:30)
                seq := seq + 1;
                IF week_num <= 4 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '11:30', d + TIME '12:30',
                        t_ace_q, 'Cloud ACE公式問題集', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                        ARRAY[tag_input, tag_gc]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '11:30', d + TIME '12:30',
                        t_book, '「Go言語による並行処理」読了', NULL, NULL, NULL, NULL, NULL,
                        ARRAY[tag_read]
                    );
                END IF;
            END IF;

            -- === Evening block 2: 21:00-21:30 JST (UTC: 12:00-12:30) ===
            -- Google Cloud問題集 or 読書（軽いタスク）
            IF dow <> 4 AND week_num <= 6 THEN
                seq := seq + 1;
                IF day_offset % 2 = 0 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '12:00', d + TIME '12:30',
                        t_ace_q, 'Cloud ACE公式問題集', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                        ARRAY[tag_gc]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '12:00', d + TIME '12:30',
                        t_book, '「Go言語による並行処理」読了', NULL, NULL, NULL,
                        ARRAY[tag_read]
                    );
                END IF;
            END IF;

            -- === CFP作業: Week 3-4 Fri 21:00-21:30 JST ===
            IF week_num BETWEEN 3 AND 4 AND dow = 5 THEN
                seq := seq + 1;
                INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                    task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '12:00', d + TIME '12:30',
                    t_cfp, 'Go Conference CFP提出', m_lt, 'LT登壇年3回', g2_id, g2_name, g2_color,
                    ARRAY[tag_writing]
                );
            END IF;

        -- ============================================================
        -- WEEKEND BLOCKS (Sat=6, Sun=0)
        -- ============================================================
        ELSE
            IF dow = 6 THEN
                -- Saturday: まとまった開発時間 10:00-13:00 JST (UTC: 01:00-04:00)
                seq := seq + 1;
                IF week_num <= 2 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '01:00', d + TIME '04:00',
                        t_db, 'DB設計＆API設計', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                        ARRAY[tag_dev]
                    );
                ELSIF week_num <= 4 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '01:00', d + TIME '04:00',
                        t_api, 'Go API実装', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                        ARRAY[tag_dev]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '01:00', d + TIME '04:00',
                        t_next, 'Next.jsフロント実装', m_mvp, 'SaaS MVPリリース', g3_id, g3_name, g3_color,
                        ARRAY[tag_dev]
                    );
                END IF;

                -- Saturday afternoon: ジム 15:00-16:00 JST (UTC: 06:00-07:00)
                seq := seq + 1;
                INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                    task_name, goal_id, goal_name, goal_color, tag_ids)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '06:00', d + TIME '07:00',
                    'ジム', NULL, NULL, NULL,
                    ARRAY[tag_exercise]
                );

                -- Saturday: ACE模擬試験 on specific weeks (Week 3, 7)
                IF week_num = 3 OR week_num = 7 THEN
                    seq := seq + 1;
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '07:30', d + TIME '10:00',
                        t_ace_mock, 'ACE模擬試験×3回', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                        ARRAY[tag_input, tag_gc]
                    );
                END IF;

            ELSE
                -- Sunday: 軽い学習 14:00-15:30 JST (UTC: 05:00-06:30)
                seq := seq + 1;
                IF week_num <= 4 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '05:00', d + TIME '06:30',
                        t_ace_q, 'Cloud ACE公式問題集', m_ace, 'ACE合格', g1_id, g1_name, g1_color,
                        ARRAY[tag_input, tag_gc]
                    );
                ELSIF week_num <= 6 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '05:00', d + TIME '06:30',
                        t_blog1, 'ブログ：Go×Cloud Run構成紹介', m_blog, 'ブログ月4本', g2_id, g2_name, g2_color,
                        ARRAY[tag_writing]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '05:00', d + TIME '06:30',
                        t_slide, 'LTスライド作成', m_lt, 'LT登壇年3回', g2_id, g2_name, g2_color,
                        ARRAY[tag_writing]
                    );
                END IF;
            END IF;
        END IF;

    END LOOP;

    RAISE NOTICE 'Inserted % time block operations', seq;
END $$;
