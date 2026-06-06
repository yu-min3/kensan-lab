-- ============================================================================
-- Demo Seed: Time Blocks — 山田拓也 (~200件)
-- ============================================================================
-- 不規則パターン: Week1-4サボりがち、Week5-8で改善
-- 全時刻はUTC（Asia/Tokyo - 9h）

DO $$
DECLARE
    base_date DATE := CURRENT_DATE - 56;
    d DATE;
    day_offset INT;
    dow INT;
    week_num INT;
    seq INT := 0;

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
    t_cli UUID := 'd2100009-0000-0000-0000-000000000000';
    t_ap_past UUID := 'd2100010-0000-0000-0000-000000000000';
    t_ap_text UUID := 'd2100011-0000-0000-0000-000000000000';
    t_algo UUID := 'd2100012-0000-0000-0000-000000000000';
    t_db UUID := 'd2100013-0000-0000-0000-000000000000';
    t_spring UUID := 'd2100014-0000-0000-0000-000000000000';
    t_git UUID := 'd2100015-0000-0000-0000-000000000000';

    tag_dev UUID := 'd20a0001-0000-0000-0000-000000000000';
    tag_learn UUID := 'd20a0002-0000-0000-0000-000000000000';
    tag_cert UUID := 'd20a0003-0000-0000-0000-000000000000';
    tag_work UUID := 'd20a0004-0000-0000-0000-000000000000';

BEGIN
    FOR day_offset IN 0..55 LOOP
        d := base_date + day_offset;
        dow := EXTRACT(DOW FROM d)::INT;
        week_num := day_offset / 7 + 1;

        -- ============================================================
        -- WEEKDAY BLOCKS (Mon-Fri)
        -- ============================================================
        IF dow BETWEEN 1 AND 5 THEN

            -- Week 1-4: スカスカ（やる気はあるが続かない）
            -- Week 3-4: 朝活チャレンジ（3日坊主）
            -- Week 5-8: 改善後（夜メイン + 朝も少し）

            -- === Morning 6:30-7:00 JST (UTC: prev day 21:30-22:00) ===
            -- Only in Week 3-4 (3日坊主) and Week 6-8 (改善後)
            IF week_num BETWEEN 3 AND 4 AND day_offset % 7 < 3 THEN
                -- 3日坊主: 各週の最初の3日だけ
                seq := seq + 1;
                INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                    task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                VALUES (
                    uuid_generate_v4(), uid,
                    (d - INTERVAL '1 day') + TIME '21:30', (d - INTERVAL '1 day') + TIME '22:00',
                    t_udemy1, 'Udemy AWS SAA講座', m_aws_base, 'AWS基礎理解', g1_id, g1_name, g1_color,
                    ARRAY[tag_cert]
                );
            ELSIF week_num >= 6 THEN
                -- 改善後: 安定した朝活
                seq := seq + 1;
                IF day_offset % 2 = 0 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:30', (d - INTERVAL '1 day') + TIME '22:00',
                        t_ap_past, '応用情報 過去問 午前', m_ap_am, '午前問題80%以上', g3_id, g3_name, g3_color,
                        ARRAY[tag_cert]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        (d - INTERVAL '1 day') + TIME '21:30', (d - INTERVAL '1 day') + TIME '22:00',
                        t_udemy2, 'Udemy AWS SAA講座 セクション6-10', m_aws_base, 'AWS基礎理解', g1_id, g1_name, g1_color,
                        ARRAY[tag_cert]
                    );
                END IF;
            END IF;

            -- === Evening block: 21:00-22:30 JST (UTC: 12:00-13:30) ===
            -- Week 1-2: 散発的（週2-3日のみ計画）
            -- Week 3-4: 少し増える
            -- Week 5-8: 安定

            IF week_num <= 2 THEN
                -- 散発的: 月水金のみ
                IF dow IN (1, 3, 5) THEN
                    seq := seq + 1;
                    IF dow = 1 THEN
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '12:00', d + TIME '13:30',
                            t_udemy1, 'Udemy AWS SAA講座 セクション1-5', m_aws_base, 'AWS基礎理解', g1_id, g1_name, g1_color,
                            ARRAY[tag_cert]
                        );
                    ELSIF dow = 3 THEN
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '12:00', d + TIME '13:30',
                            t_py1, 'Python入門書 前半', m_py_base, 'Python基礎習得', g2_id, g2_name, g2_color,
                            ARRAY[tag_learn]
                        );
                    ELSE
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '12:00', d + TIME '13:30',
                            t_git, 'Git/GitHub使い方まとめ', m_py_base, 'Python基礎習得', g2_id, g2_name, g2_color,
                            ARRAY[tag_learn]
                        );
                    END IF;
                END IF;
            ELSIF week_num <= 4 THEN
                -- 少し増える: 月〜木
                IF dow <= 4 THEN
                    seq := seq + 1;
                    IF dow <= 2 THEN
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '12:00', d + TIME '13:30',
                            t_udemy1, 'Udemy AWS SAA講座', m_aws_base, 'AWS基礎理解', g1_id, g1_name, g1_color,
                            ARRAY[tag_cert]
                        );
                    ELSE
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '12:00', d + TIME '13:30',
                            t_py1, 'Python入門書 前半', m_py_base, 'Python基礎習得', g2_id, g2_name, g2_color,
                            ARRAY[tag_learn]
                        );
                    END IF;
                END IF;
            ELSE
                -- Week 5-8: 安定（毎日）
                seq := seq + 1;
                IF dow <= 2 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '12:00', d + TIME '13:30',
                        CASE WHEN week_num <= 6 THEN t_udemy2 ELSE t_weak END,
                        CASE WHEN week_num <= 6 THEN 'Udemy AWS SAA講座 セクション6-10' ELSE '弱点分野復習' END,
                        CASE WHEN week_num <= 6 THEN m_aws_base ELSE m_aws_mock END,
                        CASE WHEN week_num <= 6 THEN 'AWS基礎理解' ELSE '模擬試験70%以上' END,
                        g1_id, g1_name, g1_color,
                        ARRAY[tag_cert]
                    );
                ELSIF dow = 3 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '12:00', d + TIME '13:30',
                        CASE WHEN week_num <= 6 THEN t_py2 ELSE t_click END,
                        CASE WHEN week_num <= 6 THEN 'Python入門書 後半' ELSE 'Click ライブラリ調査' END,
                        CASE WHEN week_num <= 6 THEN m_py_base ELSE m_cli END,
                        CASE WHEN week_num <= 6 THEN 'Python基礎習得' ELSE 'CLIツールのMVP完成' END,
                        g2_id, g2_name, g2_color,
                        ARRAY[CASE WHEN week_num <= 6 THEN tag_learn ELSE tag_dev END]
                    );
                ELSIF dow = 4 THEN
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '12:00', d + TIME '13:30',
                        t_ap_text, '応用情報テキスト読み込み', m_ap_am, '午前問題80%以上', g3_id, g3_name, g3_color,
                        ARRAY[tag_cert]
                    );
                ELSE
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '12:00', d + TIME '13:30',
                        t_algo, 'アルゴリズム問題演習', m_ap_pm, '午後問題対策', g3_id, g3_name, g3_color,
                        ARRAY[tag_cert]
                    );
                END IF;
            END IF;

            -- === Running Wed/Sat 19:00-19:30 JST (UTC: 10:00-10:30) ===
            IF dow = 3 AND week_num >= 5 THEN
                seq := seq + 1;
                INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                    task_name, goal_id, goal_name, goal_color, tag_ids)
                VALUES (
                    uuid_generate_v4(), uid,
                    d + TIME '10:00', d + TIME '10:30',
                    'ランニング', NULL, NULL, NULL,
                    NULL
                );
            END IF;

        -- ============================================================
        -- WEEKEND BLOCKS
        -- ============================================================
        ELSE
            IF dow = 6 THEN
                -- Saturday: まとまった時間（Week 1-4: 少ない, Week 5-8: 充実）
                IF week_num <= 4 THEN
                    -- 午後のみ 14:00-16:00 JST (UTC: 05:00-07:00)
                    IF day_offset % 3 <> 0 THEN  -- サボる日もある
                        seq := seq + 1;
                        INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                            task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                        VALUES (
                            uuid_generate_v4(), uid,
                            d + TIME '05:00', d + TIME '07:00',
                            t_handson, 'AWS ハンズオン（EC2 + VPC）', m_aws_base, 'AWS基礎理解', g1_id, g1_name, g1_color,
                            ARRAY[tag_cert]
                        );
                    END IF;
                ELSE
                    -- 午前 10:00-12:00 JST (UTC: 01:00-03:00) + 午後 14:00-16:00 (UTC: 05:00-07:00)
                    seq := seq + 1;
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '01:00', d + TIME '03:00',
                        CASE WHEN week_num <= 6 THEN t_udemy2 ELSE t_weak END,
                        CASE WHEN week_num <= 6 THEN 'Udemy AWS SAA講座' ELSE '弱点分野復習' END,
                        CASE WHEN week_num <= 6 THEN m_aws_base ELSE m_aws_mock END,
                        CASE WHEN week_num <= 6 THEN 'AWS基礎理解' ELSE '模擬試験70%以上' END,
                        g1_id, g1_name, g1_color,
                        ARRAY[tag_cert]
                    );
                    seq := seq + 1;
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '05:00', d + TIME '07:00',
                        CASE WHEN week_num <= 6 THEN t_py2 ELSE t_cli END,
                        CASE WHEN week_num <= 6 THEN 'Python入門書 後半' ELSE 'タスク管理CLI 実装' END,
                        CASE WHEN week_num <= 6 THEN m_py_base ELSE m_cli END,
                        CASE WHEN week_num <= 6 THEN 'Python基礎習得' ELSE 'CLIツールのMVP完成' END,
                        g2_id, g2_name, g2_color,
                        ARRAY[CASE WHEN week_num <= 6 THEN tag_learn ELSE tag_dev END]
                    );

                    -- Running on Saturday
                    seq := seq + 1;
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '08:00', d + TIME '08:30',
                        'ランニング', NULL, NULL, NULL,
                        NULL
                    );
                END IF;

                -- Mock exam on specific weeks
                IF week_num = 4 OR week_num = 7 THEN
                    seq := seq + 1;
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '08:00', d + TIME '09:30',
                        t_mock1, 'SAA模擬試験', m_aws_mock, '模擬試験70%以上', g1_id, g1_name, g1_color,
                        ARRAY[tag_cert]
                    );
                END IF;

            ELSE
                -- Sunday: ゲームの日（学習少なめ）
                IF week_num >= 5 THEN
                    -- 改善後: 日曜午後に少し勉強 14:00-15:00 JST (UTC: 05:00-06:00)
                    seq := seq + 1;
                    INSERT INTO time_blocks (id, user_id, start_datetime, end_datetime,
                        task_id, task_name, milestone_id, milestone_name, goal_id, goal_name, goal_color, tag_ids)
                    VALUES (
                        uuid_generate_v4(), uid,
                        d + TIME '05:00', d + TIME '06:00',
                        t_ap_text, '応用情報テキスト読み込み', m_ap_am, '午前問題80%以上', g3_id, g3_name, g3_color,
                        ARRAY[tag_cert]
                    );
                END IF;
            END IF;
        END IF;

    END LOOP;

    RAISE NOTICE 'Takuya: Inserted % time block operations', seq;
END $$;
