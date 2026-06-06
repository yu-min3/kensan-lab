-- ============================================================================
-- Demo Seed: Time Entries — 鈴木美咲 (~240件)
-- ============================================================================
-- 計画の約85%を実行。夜型なので夜のブロックは高完了率。
-- 週末の午前は寝坊でスキップすることがある。

DO $$
DECLARE
    base_date DATE := CURRENT_DATE - 56;
    d DATE;
    day_offset INT;
    dow INT;
    week_num INT;
    seq INT := 0;
    skip_weekend_morning BOOLEAN;
    skip_evening BOOLEAN;

    uid UUID := 'd1111111-1111-1111-1111-111111111111';

    g1_id UUID := 'd1000001-0000-0000-0000-000000000000';
    g1_name TEXT := 'フロントエンド力強化';
    g1_color TEXT := '#8B5CF6';
    g2_id UUID := 'd1000002-0000-0000-0000-000000000000';
    g2_name TEXT := '副業で個人開発';
    g2_color TEXT := '#EC4899';
    g3_id UUID := 'd1000003-0000-0000-0000-000000000000';
    g3_name TEXT := '技術ブログ月2本';
    g3_color TEXT := '#F97316';

    m_react UUID := 'd1010001-0000-0000-0000-000000000000';
    m_nextjs UUID := 'd1010002-0000-0000-0000-000000000000';
    m_test UUID := 'd1010003-0000-0000-0000-000000000000';
    m_portfolio UUID := 'd1020001-0000-0000-0000-000000000000';
    m_freelance UUID := 'd1020002-0000-0000-0000-000000000000';
    m_zenn4 UUID := 'd1030001-0000-0000-0000-000000000000';

    t_react_tut UUID := 'd1100001-0000-0000-0000-000000000000';
    t_hooks UUID := 'd1100002-0000-0000-0000-000000000000';
    t_zustand UUID := 'd1100003-0000-0000-0000-000000000000';
    t_approuter UUID := 'd1100004-0000-0000-0000-000000000000';
    t_sc UUID := 'd1100005-0000-0000-0000-000000000000';
    t_route UUID := 'd1100006-0000-0000-0000-000000000000';
    t_jest UUID := 'd1100007-0000-0000-0000-000000000000';
    t_rtl UUID := 'd1100008-0000-0000-0000-000000000000';
    t_pf_design UUID := 'd1100009-0000-0000-0000-000000000000';
    t_pf_impl UUID := 'd1100010-0000-0000-0000-000000000000';
    t_vercel UUID := 'd1100011-0000-0000-0000-000000000000';
    t_crowd UUID := 'd1100012-0000-0000-0000-000000000000';
    t_apply UUID := 'd1100013-0000-0000-0000-000000000000';
    t_zenn1 UUID := 'd1100014-0000-0000-0000-000000000000';
    t_zenn2 UUID := 'd1100015-0000-0000-0000-000000000000';

    tag_dev UUID := 'd10a0001-0000-0000-0000-000000000000';
    tag_learn UUID := 'd10a0002-0000-0000-0000-000000000000';
    tag_blog UUID := 'd10a0003-0000-0000-0000-000000000000';
    tag_freelance UUID := 'd10a0004-0000-0000-0000-000000000000';

    jitter INT;

BEGIN
    FOR day_offset IN 0..55 LOOP
        d := base_date + day_offset;
        dow := EXTRACT(DOW FROM d)::INT;
        week_num := day_offset / 7 + 1;

        skip_weekend_morning := (day_offset % 5 = 0);
        skip_evening := (day_offset % 13 = 0);
        jitter := (day_offset % 5) * 3 - 6;

        -- ============================================================
        -- WEEKDAY ENTRIES
        -- ============================================================
        IF dow BETWEEN 1 AND 5 THEN

            -- === Lunch reading (90% completion) ===
            IF day_offset % 10 <> 0 THEN
                seq := seq + 1;
                INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                    task_name, goal_id, goal_name, goal_color, tag_ids, description)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '03:15' + ((jitter/2) || ' minutes')::INTERVAL,
                    d + TIME '03:28' + ((jitter/2) || ' minutes')::INTERVAL,
                    'Zenn/Qiitaの記事読む', NULL, NULL, NULL,
                    ARRAY[tag_learn], NULL
                );
            END IF;

            -- === Evening block 1: 22:00-23:30 JST (high completion ~90%) ===
            IF NOT skip_evening THEN
                seq := seq + 1;
                IF week_num <= 2 THEN
                    IF day_offset % 2 = 0 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '13:05' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '14:28' + (jitter || ' minutes')::INTERVAL,
                            t_react_tut, 'React公式チュートリアル完走', m_react, 'React基礎固め', g1_id, g1_name, g1_color,
                            ARRAY[tag_learn], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '13:05' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '14:25' + (jitter || ' minutes')::INTERVAL,
                            t_hooks, 'hooks徹底理解', m_react, 'React基礎固め', g1_id, g1_name, g1_color,
                            ARRAY[tag_learn], NULL
                        );
                    END IF;
                ELSIF week_num <= 4 THEN
                    IF dow <= 3 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '13:03' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '14:30' + (jitter || ' minutes')::INTERVAL,
                            t_approuter, 'App Router基礎', m_nextjs, 'Next.js App Router習得', g1_id, g1_name, g1_color,
                            ARRAY[tag_learn], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '13:03' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '14:28' + (jitter || ' minutes')::INTERVAL,
                            t_sc, 'Server Components vs Client Components', m_nextjs, 'Next.js App Router習得', g1_id, g1_name, g1_color,
                            ARRAY[tag_learn], NULL
                        );
                    END IF;
                ELSIF week_num <= 6 THEN
                    IF dow <= 3 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '13:05' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '14:35' + (jitter || ' minutes')::INTERVAL,
                            t_pf_impl, 'ポートフォリオ実装', m_portfolio, 'ポートフォリオサイト完成', g2_id, g2_name, g2_color,
                            ARRAY[tag_dev, tag_freelance], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '13:05' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '14:20' + (jitter || ' minutes')::INTERVAL,
                            t_apply, '副業案件応募', m_freelance, '副業案件1件獲得', g2_id, g2_name, g2_color,
                            ARRAY[tag_freelance], NULL
                        );
                    END IF;
                ELSE
                    IF dow <= 2 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '13:05' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '14:25' + (jitter || ' minutes')::INTERVAL,
                            t_zenn2, 'Zenn記事②: Next.js App Router入門', m_zenn4, 'Zenn 4記事公開', g3_id, g3_name, g3_color,
                            ARRAY[tag_blog], NULL
                        );
                    ELSIF dow <= 4 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '13:05' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '14:22' + (jitter || ' minutes')::INTERVAL,
                            t_jest, 'Jest基礎', m_test, 'テスト駆動開発入門', g1_id, g1_name, g1_color,
                            ARRAY[tag_learn], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '13:05' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '14:20' + (jitter || ' minutes')::INTERVAL,
                            t_rtl, 'React Testing Library', m_test, 'テスト駆動開発入門', g1_id, g1_name, g1_color,
                            ARRAY[tag_learn], NULL
                        );
                    END IF;
                END IF;
            END IF;

            -- === Evening block 2 (when planned) ===
            IF week_num <= 2 AND dow <= 3 AND NOT skip_evening THEN
                seq := seq + 1;
                INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                    task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '14:32' + (jitter || ' minutes')::INTERVAL,
                    d + TIME '14:58' + (jitter || ' minutes')::INTERVAL,
                    t_zenn1, 'Zenn記事①: jQueryからReactへ', m_zenn4, 'Zenn 4記事公開', g3_id, g3_name, g3_color,
                    ARRAY[tag_blog], NULL
                );
            ELSIF week_num BETWEEN 3 AND 4 AND dow <= 3 AND NOT skip_evening AND day_offset % 3 <> 0 THEN
                seq := seq + 1;
                IF dow <= 2 THEN
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '14:32' + (jitter || ' minutes')::INTERVAL,
                        d + TIME '14:55' + (jitter || ' minutes')::INTERVAL,
                        t_zustand, 'Zustand でグローバルstate管理', m_react, 'React基礎固め', g1_id, g1_name, g1_color,
                        ARRAY[tag_learn], NULL
                    );
                ELSE
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '14:32' + (jitter || ' minutes')::INTERVAL,
                        d + TIME '14:55' + (jitter || ' minutes')::INTERVAL,
                        t_pf_design, 'ポートフォリオデザイン', m_portfolio, 'ポートフォリオサイト完成', g2_id, g2_name, g2_color,
                        ARRAY[tag_dev, tag_freelance], NULL
                    );
                END IF;
            END IF;

        -- ============================================================
        -- WEEKEND ENTRIES
        -- ============================================================
        ELSE
            IF dow = 6 THEN
                -- Saturday morning (skip sometimes due to oversleeping)
                IF NOT skip_weekend_morning THEN
                    seq := seq + 1;
                    IF week_num <= 2 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '01:10', d + TIME '03:50',
                            t_react_tut, 'React公式チュートリアル完走', m_react, 'React基礎固め', g1_id, g1_name, g1_color,
                            ARRAY[tag_learn], NULL
                        );
                    ELSIF week_num <= 6 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '01:10', d + TIME '03:55',
                            t_pf_impl, 'ポートフォリオ実装', m_portfolio, 'ポートフォリオサイト完成', g2_id, g2_name, g2_color,
                            ARRAY[tag_dev, tag_freelance], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '01:10', d + TIME '03:45',
                            t_zenn2, 'Zenn記事②: Next.js App Router入門', m_zenn4, 'Zenn 4記事公開', g3_id, g3_name, g3_color,
                            ARRAY[tag_blog], NULL
                        );
                    END IF;
                END IF;

                -- Saturday afternoon (when planned, Week 3+)
                IF week_num >= 3 THEN
                    seq := seq + 1;
                    IF week_num <= 6 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '05:10', d + TIME '06:20',
                            t_crowd, 'クラウドソーシング登録・案件探し', m_freelance, '副業案件1件獲得', g2_id, g2_name, g2_color,
                            ARRAY[tag_freelance], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '05:10', d + TIME '06:15',
                            t_jest, 'Jest基礎', m_test, 'テスト駆動開発入門', g1_id, g1_name, g1_color,
                            ARRAY[tag_learn], NULL
                        );
                    END IF;
                END IF;

            ELSE
                -- Sunday afternoon study
                seq := seq + 1;
                IF week_num <= 4 THEN
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '05:05', d + TIME '05:52',
                        t_hooks, 'hooks徹底理解', m_react, 'React基礎固め', g1_id, g1_name, g1_color,
                        ARRAY[tag_learn], NULL
                    );
                ELSE
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '05:05', d + TIME '05:50',
                        t_route, 'Route Handlers で API 実装', m_nextjs, 'Next.js App Router習得', g1_id, g1_name, g1_color,
                        ARRAY[tag_learn], NULL
                    );
                END IF;
            END IF;
        END IF;

    END LOOP;

    RAISE NOTICE 'Misaki: Inserted % time entry operations', seq;
END $$;
