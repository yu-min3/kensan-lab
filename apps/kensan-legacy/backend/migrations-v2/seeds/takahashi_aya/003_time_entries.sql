-- ============================================================================
-- Demo Seed: Time Entries — 高橋彩 (~260件)
-- ============================================================================
-- 朝型で規律が高い。計画の約90%を実行。
-- たまに猫の通院で朝活をスキップ。

DO $$
DECLARE
    base_date DATE := CURRENT_DATE - 56;
    d DATE;
    day_offset INT;
    dow INT;
    week_num INT;
    seq INT := 0;
    skip_morning BOOLEAN;

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

    jitter INT;

BEGIN
    FOR day_offset IN 0..55 LOOP
        d := base_date + day_offset;
        dow := EXTRACT(DOW FROM d)::INT;
        week_num := day_offset / 7 + 1;

        skip_morning := (day_offset % 9 = 0);  -- ~11% skip (cat vet etc.)
        jitter := (day_offset % 5) * 2 - 4;

        -- ============================================================
        -- WEEKDAY ENTRIES
        -- ============================================================
        IF dow BETWEEN 1 AND 5 THEN

            -- === Early morning: 5:30-6:30 JST ===
            IF NOT skip_morning THEN
                seq := seq + 1;
                IF week_num <= 2 THEN
                    IF dow <= 3 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '20:32' + (jitter || ' minutes')::INTERVAL,
                            (d - INTERVAL '1 day') + TIME '21:28' + (jitter || ' minutes')::INTERVAL,
                            t_1on1_tmpl, '1on1テンプレート作成', m_1on1, '1on1テンプレート確立', g1_id, g1_name, g1_color,
                            ARRAY[tag_mgmt], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '20:32' + (jitter || ' minutes')::INTERVAL,
                            (d - INTERVAL '1 day') + TIME '21:25' + (jitter || ' minutes')::INTERVAL,
                            t_1on1_memo, 'メンバー5人分の1on1メモ整理', m_1on1, '1on1テンプレート確立', g1_id, g1_name, g1_color,
                            ARRAY[tag_mgmt], NULL
                        );
                    END IF;
                ELSIF week_num <= 4 THEN
                    IF dow <= 2 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '20:32' + (jitter || ' minutes')::INTERVAL,
                            (d - INTERVAL '1 day') + TIME '21:28' + (jitter || ' minutes')::INTERVAL,
                            t_grow, 'GROWモデル学習', m_coach, 'コーチング手法の実践', g1_id, g1_name, g1_color,
                            ARRAY[tag_mgmt, tag_learn], NULL
                        );
                    ELSIF dow <= 4 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '20:32' + (jitter || ' minutes')::INTERVAL,
                            (d - INTERVAL '1 day') + TIME '21:30' + (jitter || ' minutes')::INTERVAL,
                            t_jira, 'Jiraダッシュボード構築', m_velocity, 'ベロシティ可視化', g2_id, g2_name, g2_color,
                            ARRAY[tag_mgmt, tag_tech], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '20:32' + (jitter || ' minutes')::INTERVAL,
                            (d - INTERVAL '1 day') + TIME '21:25' + (jitter || ' minutes')::INTERVAL,
                            t_pr_review, 'PR レビュー待ち時間の短縮施策', m_process, '開発プロセス改善3施策', g2_id, g2_name, g2_color,
                            ARRAY[tag_mgmt], NULL
                        );
                    END IF;
                ELSIF week_num <= 6 THEN
                    IF dow <= 2 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '20:32' + (jitter || ' minutes')::INTERVAL,
                            (d - INTERVAL '1 day') + TIME '21:28' + (jitter || ' minutes')::INTERVAL,
                            t_daily, 'デイリースクラム改善', m_process, '開発プロセス改善3施策', g2_id, g2_name, g2_color,
                            ARRAY[tag_mgmt], NULL
                        );
                    ELSIF dow <= 4 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '20:32' + (jitter || ' minutes')::INTERVAL,
                            (d - INTERVAL '1 day') + TIME '21:30' + (jitter || ' minutes')::INTERVAL,
                            t_go_tour, 'Go Tour完走', m_go, 'Go学習再開', g2_id, g2_name, g2_color,
                            ARRAY[tag_tech, tag_learn], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '20:32' + (jitter || ' minutes')::INTERVAL,
                            (d - INTERVAL '1 day') + TIME '21:25' + (jitter || ' minutes')::INTERVAL,
                            t_sbi, 'SBIフィードバック実践', m_coach, 'コーチング手法の実践', g1_id, g1_name, g1_color,
                            ARRAY[tag_mgmt], NULL
                        );
                    END IF;
                ELSE
                    IF dow <= 2 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '20:32' + (jitter || ' minutes')::INTERVAL,
                            (d - INTERVAL '1 day') + TIME '21:30' + (jitter || ' minutes')::INTERVAL,
                            t_go_api, 'Goで小さなAPIを作る', m_go, 'Go学習再開', g2_id, g2_name, g2_color,
                            ARRAY[tag_tech, tag_learn], NULL
                        );
                    ELSIF dow <= 4 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '20:32' + (jitter || ' minutes')::INTERVAL,
                            (d - INTERVAL '1 day') + TIME '21:28' + (jitter || ' minutes')::INTERVAL,
                            t_survey, 'アンケート項目設計', m_survey, 'メンバー満足度調査実施', g1_id, g1_name, g1_color,
                            ARRAY[tag_mgmt], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '20:32' + (jitter || ' minutes')::INTERVAL,
                            (d - INTERVAL '1 day') + TIME '21:25' + (jitter || ' minutes')::INTERVAL,
                            t_blog_post, '読書メモ記事1本目 公開', m_blog, '読書メモを社内ブログで共有', g3_id, g3_name, g3_color,
                            ARRAY[tag_read], NULL
                        );
                    END IF;
                END IF;
            END IF;

            -- === Yoga Mon/Wed/Fri (85% completion) ===
            IF dow IN (1, 3, 5) AND NOT skip_morning AND day_offset % 7 <> 0 THEN
                seq := seq + 1;
                INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                    task_name, goal_id, goal_name, goal_color, tag_ids, description)
                VALUES (
                    uuid_generate_v4(), uid,
                    (d - INTERVAL '1 day') + TIME '21:32' + (jitter || ' minutes')::INTERVAL,
                    (d - INTERVAL '1 day') + TIME '21:58' + (jitter || ' minutes')::INTERVAL,
                    'ヨガ', NULL, NULL, NULL,
                    ARRAY[tag_yoga], NULL
                );
            END IF;

            -- === Lunch reading (95% completion) ===
            IF day_offset % 20 <> 0 THEN
                seq := seq + 1;
                IF week_num <= 3 THEN
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '03:16' + ((jitter/2) || ' minutes')::INTERVAL,
                        d + TIME '03:44' + ((jitter/2) || ' minutes')::INTERVAL,
                        t_em_book, 'エンジニアリングマネージャーのしごと 読了', m_read4, '2ヶ月で4冊読了', g3_id, g3_name, g3_color,
                        ARRAY[tag_read], NULL
                    );
                ELSE
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '03:16' + ((jitter/2) || ' minutes')::INTERVAL,
                        d + TIME '03:44' + ((jitter/2) || ' minutes')::INTERVAL,
                        t_high_output, 'HIGH OUTPUT MANAGEMENT 読了', m_read4, '2ヶ月で4冊読了', g3_id, g3_name, g3_color,
                        ARRAY[tag_read], NULL
                    );
                END IF;
            END IF;

        -- ============================================================
        -- WEEKEND ENTRIES
        -- ============================================================
        ELSE
            IF dow = 6 THEN
                -- Saturday morning learning
                IF NOT skip_morning THEN
                    seq := seq + 1;
                    IF week_num <= 4 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '21:05', (d - INTERVAL '1 day') + TIME '22:55',
                            t_velocity, 'ベロシティレポート自動化', m_velocity, 'ベロシティ可視化', g2_id, g2_name, g2_color,
                            ARRAY[tag_mgmt, tag_tech], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            (d - INTERVAL '1 day') + TIME '21:05', (d - INTERVAL '1 day') + TIME '22:50',
                            CASE WHEN week_num <= 6 THEN t_go_tour ELSE t_go_api END,
                            CASE WHEN week_num <= 6 THEN 'Go Tour完走' ELSE 'Goで小さなAPIを作る' END,
                            m_go, 'Go学習再開', g2_id, g2_name, g2_color,
                            ARRAY[tag_tech, tag_learn], NULL
                        );
                    END IF;
                END IF;

            ELSE
                -- Sunday yoga + reading
                IF NOT skip_morning THEN
                    seq := seq + 1;
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:05', (d - INTERVAL '1 day') + TIME '21:32',
                        'ヨガ', NULL, NULL, NULL,
                        ARRAY[tag_yoga], NULL
                    );
                END IF;

                seq := seq + 1;
                IF week_num <= 3 THEN
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:35', (d - INTERVAL '1 day') + TIME '22:55',
                        t_em_book, 'エンジニアリングマネージャーのしごと 読了', m_read4, '2ヶ月で4冊読了', g3_id, g3_name, g3_color,
                        ARRAY[tag_read], NULL
                    );
                ELSE
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:35', (d - INTERVAL '1 day') + TIME '22:50',
                        t_high_output, 'HIGH OUTPUT MANAGEMENT 読了', m_read4, '2ヶ月で4冊読了', g3_id, g3_name, g3_color,
                        ARRAY[tag_read], NULL
                    );
                END IF;
            END IF;
        END IF;

    END LOOP;

    RAISE NOTICE 'Aya: Inserted % time entry operations', seq;
END $$;
