-- ============================================================================
-- Demo Seed: Time Entries — 山田拓也 (~180件)
-- ============================================================================
-- Week1-4: 計画の50%しか実行しない（サボりがち）
-- Week5-8: 先輩のアドバイスで改善、80%実行

DO $$
DECLARE
    base_date DATE := CURRENT_DATE - 56;
    d DATE;
    day_offset INT;
    dow INT;
    week_num INT;
    seq INT := 0;
    skip_block BOOLEAN;

    uid UUID := 'd2222222-2222-2222-2222-222222222222';

    g1_id UUID := 'd2000001-0000-0000-0000-000000000000';
    g1_name TEXT := 'AWS SAA取得';
    g1_color TEXT := '#F59E0B';
    g2_id UUID := 'd2000002-0000-0000-0000-000000000000';
    g2_name TEXT := 'Pythonで個人ツール';
    g2_color TEXT := '#10B981';
    g3_id UUID := 'd2000003-0000-0000-0000-000000000000';
    g3_name TEXT := '応用情報技術者取得';
    g3_color TEXT := '#3B82F6';

    m_aws_base UUID := 'd2010001-0000-0000-0000-000000000000';
    m_aws_mock UUID := 'd2010002-0000-0000-0000-000000000000';
    m_py_base UUID := 'd2020001-0000-0000-0000-000000000000';
    m_cli UUID := 'd2020002-0000-0000-0000-000000000000';
    m_ap_am UUID := 'd2030001-0000-0000-0000-000000000000';
    m_ap_pm UUID := 'd2030002-0000-0000-0000-000000000000';

    t_udemy1 UUID := 'd2100001-0000-0000-0000-000000000000';
    t_handson UUID := 'd2100002-0000-0000-0000-000000000000';
    t_udemy2 UUID := 'd2100003-0000-0000-0000-000000000000';
    t_mock1 UUID := 'd2100004-0000-0000-0000-000000000000';
    t_weak UUID := 'd2100005-0000-0000-0000-000000000000';
    t_py1 UUID := 'd2100006-0000-0000-0000-000000000000';
    t_py2 UUID := 'd2100007-0000-0000-0000-000000000000';
    t_click UUID := 'd2100008-0000-0000-0000-000000000000';
    t_cli_impl UUID := 'd2100009-0000-0000-0000-000000000000';
    t_ap_past UUID := 'd2100010-0000-0000-0000-000000000000';
    t_ap_text UUID := 'd2100011-0000-0000-0000-000000000000';
    t_algo UUID := 'd2100012-0000-0000-0000-000000000000';
    t_db UUID := 'd2100013-0000-0000-0000-000000000000';
    t_spring UUID := 'd2100014-0000-0000-0000-000000000000';
    t_git UUID := 'd2100015-0000-0000-0000-000000000000';

    tag_dev UUID := 'd20a0001-0000-0000-0000-000000000000';
    tag_learn UUID := 'd20a0002-0000-0000-0000-000000000000';
    tag_cert UUID := 'd20a0003-0000-0000-0000-000000000000';

    jitter INT;

BEGIN
    FOR day_offset IN 0..55 LOOP
        d := base_date + day_offset;
        dow := EXTRACT(DOW FROM d)::INT;
        week_num := day_offset / 7 + 1;

        jitter := (day_offset % 5) * 3 - 6;

        -- Skip logic: Week1-4 skip ~50%, Week5-8 skip ~20%
        IF week_num <= 4 THEN
            skip_block := (day_offset % 2 = 0);
        ELSE
            skip_block := (day_offset % 5 = 0);
        END IF;

        -- ============================================================
        -- WEEKDAY ENTRIES
        -- ============================================================
        IF dow BETWEEN 1 AND 5 THEN

            -- === Morning (only when planned: Week3-4 first 3 days, Week6+) ===
            IF week_num BETWEEN 3 AND 4 AND day_offset % 7 < 3 THEN
                -- 3日坊主: execute only first 2 days (skip 3rd)
                IF day_offset % 7 < 2 THEN
                    seq := seq + 1;
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:35' + (jitter || ' minutes')::INTERVAL,
                        (d - INTERVAL '1 day') + TIME '21:55' + (jitter || ' minutes')::INTERVAL,
                        t_udemy1, 'Udemy AWS SAA講座', m_aws_base, 'AWS基礎理解', g1_id, g1_name, g1_color,
                        ARRAY[tag_cert], NULL
                    );
                END IF;
            ELSIF week_num >= 6 AND NOT skip_block THEN
                seq := seq + 1;
                IF day_offset % 2 = 0 THEN
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:33' + (jitter || ' minutes')::INTERVAL,
                        (d - INTERVAL '1 day') + TIME '21:58' + (jitter || ' minutes')::INTERVAL,
                        t_ap_past, '応用情報 過去問 午前', m_ap_am, '午前問題80%以上', g3_id, g3_name, g3_color,
                        ARRAY[tag_cert], NULL
                    );
                ELSE
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:33' + (jitter || ' minutes')::INTERVAL,
                        (d - INTERVAL '1 day') + TIME '21:56' + (jitter || ' minutes')::INTERVAL,
                        t_udemy2, 'Udemy AWS SAA講座 セクション6-10', m_aws_base, 'AWS基礎理解', g1_id, g1_name, g1_color,
                        ARRAY[tag_cert], NULL
                    );
                END IF;
            END IF;

            -- === Evening block ===
            IF NOT skip_block THEN
                IF week_num <= 2 THEN
                    IF dow IN (1, 3, 5) THEN
                        seq := seq + 1;
                        IF dow = 1 THEN
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '12:10' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '13:15' + (jitter || ' minutes')::INTERVAL,
                                t_udemy1, 'Udemy AWS SAA講座 セクション1-5', m_aws_base, 'AWS基礎理解', g1_id, g1_name, g1_color,
                                ARRAY[tag_cert], NULL
                            );
                        ELSIF dow = 3 THEN
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '12:10' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '13:10' + (jitter || ' minutes')::INTERVAL,
                                t_py1, 'Python入門書 前半', m_py_base, 'Python基礎習得', g2_id, g2_name, g2_color,
                                ARRAY[tag_learn], NULL
                            );
                        ELSE
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '12:10' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '13:05' + (jitter || ' minutes')::INTERVAL,
                                t_git, 'Git/GitHub使い方まとめ', m_py_base, 'Python基礎習得', g2_id, g2_name, g2_color,
                                ARRAY[tag_learn], NULL
                            );
                        END IF;
                    END IF;
                ELSIF week_num <= 4 THEN
                    IF dow <= 4 THEN
                        seq := seq + 1;
                        IF dow <= 2 THEN
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '12:08' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '13:20' + (jitter || ' minutes')::INTERVAL,
                                t_udemy1, 'Udemy AWS SAA講座', m_aws_base, 'AWS基礎理解', g1_id, g1_name, g1_color,
                                ARRAY[tag_cert], NULL
                            );
                        ELSE
                            INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                                task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                            VALUES (
                                uuid_generate_v4(), uid,
                                d + TIME '12:08' + (jitter || ' minutes')::INTERVAL,
                                d + TIME '13:15' + (jitter || ' minutes')::INTERVAL,
                                t_py1, 'Python入門書 前半', m_py_base, 'Python基礎習得', g2_id, g2_name, g2_color,
                                ARRAY[tag_learn], NULL
                            );
                        END IF;
                    END IF;
                ELSE
                    -- Week 5-8: stable
                    seq := seq + 1;
                    IF dow <= 2 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '12:05' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '13:28' + (jitter || ' minutes')::INTERVAL,
                            CASE WHEN week_num <= 6 THEN t_udemy2 ELSE t_weak END,
                            CASE WHEN week_num <= 6 THEN 'Udemy AWS SAA講座 セクション6-10' ELSE '弱点分野復習' END,
                            CASE WHEN week_num <= 6 THEN m_aws_base ELSE m_aws_mock END,
                            CASE WHEN week_num <= 6 THEN 'AWS基礎理解' ELSE '模擬試験70%以上' END,
                            g1_id, g1_name, g1_color,
                            ARRAY[tag_cert], NULL
                        );
                    ELSIF dow = 3 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '12:05' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '13:25' + (jitter || ' minutes')::INTERVAL,
                            CASE WHEN week_num <= 6 THEN t_py2 ELSE t_click END,
                            CASE WHEN week_num <= 6 THEN 'Python入門書 後半' ELSE 'Click ライブラリ調査' END,
                            CASE WHEN week_num <= 6 THEN m_py_base ELSE m_cli END,
                            CASE WHEN week_num <= 6 THEN 'Python基礎習得' ELSE 'CLIツールのMVP完成' END,
                            g2_id, g2_name, g2_color,
                            ARRAY[CASE WHEN week_num <= 6 THEN tag_learn ELSE tag_dev END], NULL
                        );
                    ELSIF dow = 4 THEN
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '12:05' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '13:25' + (jitter || ' minutes')::INTERVAL,
                            t_ap_text, '応用情報テキスト読み込み', m_ap_am, '午前問題80%以上', g3_id, g3_name, g3_color,
                            ARRAY[tag_cert], NULL
                        );
                    ELSE
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '12:05' + (jitter || ' minutes')::INTERVAL,
                            d + TIME '13:22' + (jitter || ' minutes')::INTERVAL,
                            t_algo, 'アルゴリズム問題演習', m_ap_pm, '午後問題対策', g3_id, g3_name, g3_color,
                            ARRAY[tag_cert], NULL
                        );
                    END IF;
                END IF;
            END IF;

            -- === Running Wed (Week 5+) ===
            IF dow = 3 AND week_num >= 5 AND NOT skip_block THEN
                seq := seq + 1;
                INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                    task_name, goal_id, goal_name, goal_color, tag_ids, description)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '10:05', d + TIME '10:28',
                    'ランニング', NULL, NULL, NULL,
                    NULL, NULL
                );
            END IF;

        -- ============================================================
        -- WEEKEND ENTRIES
        -- ============================================================
        ELSE
            IF dow = 6 THEN
                IF week_num <= 4 THEN
                    -- Sporadic Saturday (skip some)
                    IF day_offset % 3 <> 0 AND NOT skip_block THEN
                        seq := seq + 1;
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '05:15', d + TIME '06:45',
                            t_handson, 'AWS ハンズオン（EC2 + VPC）', m_aws_base, 'AWS基礎理解', g1_id, g1_name, g1_color,
                            ARRAY[tag_cert], NULL
                        );
                    END IF;
                ELSE
                    -- Week 5-8: full Saturday
                    seq := seq + 1;
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '01:10', d + TIME '02:50',
                        CASE WHEN week_num <= 6 THEN t_udemy2 ELSE t_weak END,
                        CASE WHEN week_num <= 6 THEN 'Udemy AWS SAA講座' ELSE '弱点分野復習' END,
                        CASE WHEN week_num <= 6 THEN m_aws_base ELSE m_aws_mock END,
                        CASE WHEN week_num <= 6 THEN 'AWS基礎理解' ELSE '模擬試験70%以上' END,
                        g1_id, g1_name, g1_color,
                        ARRAY[tag_cert], NULL
                    );
                    seq := seq + 1;
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '05:10', d + TIME '06:50',
                        CASE WHEN week_num <= 6 THEN t_py2 ELSE t_cli_impl END,
                        CASE WHEN week_num <= 6 THEN 'Python入門書 後半' ELSE 'タスク管理CLI 実装' END,
                        CASE WHEN week_num <= 6 THEN m_py_base ELSE m_cli END,
                        CASE WHEN week_num <= 6 THEN 'Python基礎習得' ELSE 'CLIツールのMVP完成' END,
                        g2_id, g2_name, g2_color,
                        ARRAY[CASE WHEN week_num <= 6 THEN tag_learn ELSE tag_dev END], NULL
                    );

                    -- Running Saturday
                    IF NOT skip_block THEN
                        seq := seq + 1;
                        INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                            task_name, goal_id, goal_name, goal_color, tag_ids, description)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '08:05', d + TIME '08:28',
                            'ランニング', NULL, NULL, NULL, NULL, NULL
                        );
                    END IF;
                END IF;

                -- Mock exam
                IF week_num = 4 OR week_num = 7 THEN
                    seq := seq + 1;
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '08:10', d + TIME '09:20',
                        t_mock1, 'SAA模擬試験', m_aws_mock, '模擬試験70%以上', g1_id, g1_name, g1_color,
                        ARRAY[tag_cert],
                        CASE WHEN week_num = 4 THEN '結果: 52%。全然ダメだった...IAMとVPCが壊滅的'
                             ELSE '結果: 68%。かなり改善！あと少しで70%' END
                    );
                END IF;

            ELSE
                -- Sunday: mostly gaming, but Week 5+ adds study
                IF week_num >= 5 AND NOT skip_block THEN
                    seq := seq + 1;
                    INSERT INTO time_entries (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids, description)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '05:10', d + TIME '05:55',
                        t_ap_text, '応用情報テキスト読み込み', m_ap_am, '午前問題80%以上', g3_id, g3_name, g3_color,
                        ARRAY[tag_cert], NULL
                    );
                END IF;
            END IF;
        END IF;

    END LOOP;

    RAISE NOTICE 'Takuya: Inserted % time entry operations', seq;
END $$;
