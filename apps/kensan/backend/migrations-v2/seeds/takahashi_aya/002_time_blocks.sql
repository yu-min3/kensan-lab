-- ============================================================================
-- Demo Seed: Time Blocks — 高橋彩 (~280件)
-- ============================================================================
-- 朝型（5:30-7:00が学習時間）、昼休みに読書
-- 全時刻はUTC（Asia/Tokyo - 9h）

DO $$
DECLARE
    base_date DATE := CURRENT_DATE - 56;
    d DATE;
    day_offset INT;
    dow INT;
    week_num INT;
    seq INT := 0;

    uid UUID := 'd3333333-3333-3333-3333-333333333333';

    g1_id UUID := 'd3000001-0000-0000-0000-000000000000';
    g1_name TEXT := '1on1スキル向上';
    g1_color TEXT := '#8B5CF6';
    g2_id UUID := 'd3000002-0000-0000-0000-000000000000';
    g2_name TEXT := 'チーム生産性20%向上';
    g2_color TEXT := '#3B82F6';
    g3_id UUID := 'd3000003-0000-0000-0000-000000000000';
    g3_name TEXT := '月2冊ビジネス書読書';
    g3_color TEXT := '#10B981';

    m_1on1 UUID := 'd3010001-0000-0000-0000-000000000000';
    m_coach UUID := 'd3010002-0000-0000-0000-000000000000';
    m_survey UUID := 'd3010003-0000-0000-0000-000000000000';
    m_velocity UUID := 'd3020001-0000-0000-0000-000000000000';
    m_process UUID := 'd3020002-0000-0000-0000-000000000000';
    m_go UUID := 'd3020003-0000-0000-0000-000000000000';
    m_read4 UUID := 'd3030001-0000-0000-0000-000000000000';
    m_blog UUID := 'd3030002-0000-0000-0000-000000000000';

    t_1on1_tmpl UUID := 'd3100001-0000-0000-0000-000000000000';
    t_1on1_memo UUID := 'd3100002-0000-0000-0000-000000000000';
    t_grow UUID := 'd3100003-0000-0000-0000-000000000000';
    t_sbi UUID := 'd3100004-0000-0000-0000-000000000000';
    t_survey UUID := 'd3100005-0000-0000-0000-000000000000';
    t_jira UUID := 'd3100006-0000-0000-0000-000000000000';
    t_velocity UUID := 'd3100007-0000-0000-0000-000000000000';
    t_pr_review UUID := 'd3100008-0000-0000-0000-000000000000';
    t_daily UUID := 'd3100009-0000-0000-0000-000000000000';
    t_test UUID := 'd3100010-0000-0000-0000-000000000000';
    t_go_tour UUID := 'd3100011-0000-0000-0000-000000000000';
    t_go_api UUID := 'd3100012-0000-0000-0000-000000000000';
    t_em_book UUID := 'd3100013-0000-0000-0000-000000000000';
    t_high_output UUID := 'd3100014-0000-0000-0000-000000000000';
    t_blog_post UUID := 'd3100015-0000-0000-0000-000000000000';

    tag_mgmt UUID := 'd30a0001-0000-0000-0000-000000000000';
    tag_learn UUID := 'd30a0002-0000-0000-0000-000000000000';
    tag_read UUID := 'd30a0003-0000-0000-0000-000000000000';
    tag_tech UUID := 'd30a0004-0000-0000-0000-000000000000';
    tag_yoga UUID := 'd30a0005-0000-0000-0000-000000000000';

BEGIN
    FOR day_offset IN 0..55 LOOP
        d := base_date + day_offset;
        dow := EXTRACT(DOW FROM d)::INT;
        week_num := day_offset / 7 + 1;

        -- ============================================================
        -- WEEKDAY BLOCKS (Mon-Fri)
        -- ============================================================
        IF dow BETWEEN 1 AND 5 THEN

            -- === Early morning: 5:30-6:30 JST (UTC: prev day 20:30-21:30) ===
            seq := seq + 1;
            IF week_num <= 2 THEN
                -- Week 1-2: 1on1関連
                IF dow <= 3 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '20:30', (d - INTERVAL '1 day') + TIME '21:30',
                        t_1on1_tmpl, '1on1テンプレート作成', m_1on1, '1on1テンプレート確立', g1_id, g1_name, g1_color,
                        ARRAY[tag_mgmt]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '20:30', (d - INTERVAL '1 day') + TIME '21:30',
                        t_1on1_memo, 'メンバー5人分の1on1メモ整理', m_1on1, '1on1テンプレート確立', g1_id, g1_name, g1_color,
                        ARRAY[tag_mgmt]
                    );
                END IF;
            ELSIF week_num <= 4 THEN
                -- Week 3-4: コーチング + ベロシティ
                IF dow <= 2 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '20:30', (d - INTERVAL '1 day') + TIME '21:30',
                        t_grow, 'GROWモデル学習', m_coach, 'コーチング手法の実践', g1_id, g1_name, g1_color,
                        ARRAY[tag_mgmt, tag_learn]
                    );
                ELSIF dow <= 4 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '20:30', (d - INTERVAL '1 day') + TIME '21:30',
                        t_jira, 'Jiraダッシュボード構築', m_velocity, 'ベロシティ可視化', g2_id, g2_name, g2_color,
                        ARRAY[tag_mgmt, tag_tech]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '20:30', (d - INTERVAL '1 day') + TIME '21:30',
                        t_pr_review, 'PR レビュー待ち時間の短縮施策', m_process, '開発プロセス改善3施策', g2_id, g2_name, g2_color,
                        ARRAY[tag_mgmt]
                    );
                END IF;
            ELSIF week_num <= 6 THEN
                -- Week 5-6: プロセス改善 + Go学習再開
                IF dow <= 2 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '20:30', (d - INTERVAL '1 day') + TIME '21:30',
                        t_daily, 'デイリースクラム改善', m_process, '開発プロセス改善3施策', g2_id, g2_name, g2_color,
                        ARRAY[tag_mgmt]
                    );
                ELSIF dow <= 4 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '20:30', (d - INTERVAL '1 day') + TIME '21:30',
                        t_go_tour, 'Go Tour完走', m_go, 'Go学習再開', g2_id, g2_name, g2_color,
                        ARRAY[tag_tech, tag_learn]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '20:30', (d - INTERVAL '1 day') + TIME '21:30',
                        t_sbi, 'SBIフィードバック実践', m_coach, 'コーチング手法の実践', g1_id, g1_name, g1_color,
                        ARRAY[tag_mgmt]
                    );
                END IF;
            ELSE
                -- Week 7-8: バランス（Go + コーチング + アンケート設計）
                IF dow <= 2 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '20:30', (d - INTERVAL '1 day') + TIME '21:30',
                        t_go_api, 'Goで小さなAPIを作る', m_go, 'Go学習再開', g2_id, g2_name, g2_color,
                        ARRAY[tag_tech, tag_learn]
                    );
                ELSIF dow <= 4 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '20:30', (d - INTERVAL '1 day') + TIME '21:30',
                        t_survey, 'アンケート項目設計', m_survey, 'メンバー満足度調査実施', g1_id, g1_name, g1_color,
                        ARRAY[tag_mgmt]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '20:30', (d - INTERVAL '1 day') + TIME '21:30',
                        t_blog_post, '読書メモ記事1本目 公開', m_blog, '読書メモを社内ブログで共有', g3_id, g3_name, g3_color,
                        ARRAY[tag_read]
                    );
                END IF;
            END IF;

            -- === Morning block 2: 6:30-7:00 JST (UTC: prev day 21:30-22:00) ===
            -- Yoga on Mon/Wed/Fri
            IF dow IN (1, 3, 5) THEN
                seq := seq + 1;
                INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                    task_name, goal_id, goal_name, goal_color, tag_ids)
                VALUES (
                    uuid_generate_v4(), uid,
                    (d - INTERVAL '1 day') + TIME '21:30', (d - INTERVAL '1 day') + TIME '22:00',
                    'ヨガ', NULL, NULL, NULL,
                    ARRAY[tag_yoga]
                );
            END IF;

            -- === Lunch: 12:15-12:45 JST (UTC: 03:15-03:45) — 読書 ===
            seq := seq + 1;
            IF week_num <= 3 THEN
                INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                    task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '03:15', d + TIME '03:45',
                    t_em_book, 'エンジニアリングマネージャーのしごと 読了', m_read4, '2ヶ月で4冊読了', g3_id, g3_name, g3_color,
                    ARRAY[tag_read]
                );
            ELSE
                INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                    task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '03:15', d + TIME '03:45',
                    t_high_output, 'HIGH OUTPUT MANAGEMENT 読了', m_read4, '2ヶ月で4冊読了', g3_id, g3_name, g3_color,
                    ARRAY[tag_read]
                );
            END IF;

        -- ============================================================
        -- WEEKEND BLOCKS
        -- ============================================================
        ELSE
            IF dow = 6 THEN
                -- Saturday: 朝の学習 6:00-8:00 JST (UTC: prev day 21:00-23:00)
                seq := seq + 1;
                IF week_num <= 4 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:00', (d - INTERVAL '1 day') + TIME '23:00',
                        t_velocity, 'ベロシティレポート自動化', m_velocity, 'ベロシティ可視化', g2_id, g2_name, g2_color,
                        ARRAY[tag_mgmt, tag_tech]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:00', (d - INTERVAL '1 day') + TIME '23:00',
                        CASE WHEN week_num <= 6 THEN t_go_tour ELSE t_go_api END,
                        CASE WHEN week_num <= 6 THEN 'Go Tour完走' ELSE 'Goで小さなAPIを作る' END,
                        m_go, 'Go学習再開', g2_id, g2_name, g2_color,
                        ARRAY[tag_tech, tag_learn]
                    );
                END IF;

            ELSE
                -- Sunday: 読書 + ヨガ
                seq := seq + 1;
                INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                    task_name, goal_id, goal_name, goal_color, tag_ids)
                VALUES (
                    uuid_generate_v4(), uid,
                    (d - INTERVAL '1 day') + TIME '21:00', (d - INTERVAL '1 day') + TIME '21:30',
                    'ヨガ', NULL, NULL, NULL,
                    ARRAY[tag_yoga]
                );

                seq := seq + 1;
                IF week_num <= 3 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:30', (d - INTERVAL '1 day') + TIME '23:00',
                        t_em_book, 'エンジニアリングマネージャーのしごと 読了', m_read4, '2ヶ月で4冊読了', g3_id, g3_name, g3_color,
                        ARRAY[tag_read]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:30', (d - INTERVAL '1 day') + TIME '23:00',
                        t_high_output, 'HIGH OUTPUT MANAGEMENT 読了', m_read4, '2ヶ月で4冊読了', g3_id, g3_name, g3_color,
                        ARRAY[tag_read]
                    );
                END IF;
            END IF;
        END IF;

    END LOOP;

    RAISE NOTICE 'Aya: Inserted % time block operations', seq;
END $$;
