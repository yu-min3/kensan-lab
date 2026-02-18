-- ============================================================================
-- Demo Seed: Todo Completions — 山田拓也 (~80件)
-- ============================================================================
-- Week1-4: サボりがち（40-50%）、Week5-8: 改善（70-80%）

DO $$
DECLARE
    base_date DATE := CURRENT_DATE - 56;
    d DATE;
    day_offset INT;
    dow INT;
    week_num INT;
    seq INT := 0;
    uid UUID := 'd2222222-2222-2222-2222-222222222222';

    todo_aws UUID := 'd20b0001-0000-0000-0000-000000000000';
    todo_article UUID := 'd20b0002-0000-0000-0000-000000000000';
    todo_ap UUID := 'd20b0003-0000-0000-0000-000000000000';
    todo_run UUID := 'd20b0004-0000-0000-0000-000000000000';

BEGIN
    FOR day_offset IN 0..55 LOOP
        d := base_date + day_offset;
        dow := EXTRACT(DOW FROM d)::INT;
        week_num := day_offset / 7 + 1;

        -- AWS問題集 (daily)
        IF week_num <= 4 THEN
            -- Week 1-4: 40%
            IF day_offset % 5 < 2 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_aws, d, d + TIME '13:00' + INTERVAL '9 hours');
            END IF;
        ELSE
            -- Week 5-8: 80%
            IF day_offset % 5 <> 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_aws, d, d + TIME '06:30' + INTERVAL '9 hours');
            END IF;
        END IF;

        -- 技術記事を1本読む (daily)
        IF week_num <= 4 THEN
            -- 50%
            IF day_offset % 2 = 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_article, d, d + TIME '03:30' + INTERVAL '9 hours');
            END IF;
        ELSE
            -- 75%
            IF day_offset % 4 <> 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_article, d, d + TIME '03:30' + INTERVAL '9 hours');
            END IF;
        END IF;

        -- 応用情報 午前1問 (daily)
        IF week_num <= 4 THEN
            -- 30%
            IF day_offset % 10 < 3 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_ap, d, d + TIME '13:30' + INTERVAL '9 hours');
            END IF;
        ELSE
            -- 65%
            IF day_offset % 3 <> 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_ap, d, d + TIME '06:00' + INTERVAL '9 hours');
            END IF;
        END IF;

        -- ランニング (Wed/Sat)
        IF dow IN (3, 6) THEN
            IF week_num <= 4 THEN
                -- 20%
                IF day_offset % 14 < 3 THEN
                    seq := seq + 1;
                    INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                    VALUES (uuid_generate_v4(), uid, todo_run, d, d + TIME '10:30' + INTERVAL '9 hours');
                END IF;
            ELSE
                -- 60%
                IF day_offset % 5 < 3 THEN
                    seq := seq + 1;
                    INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                    VALUES (uuid_generate_v4(), uid, todo_run, d, d + TIME '10:30' + INTERVAL '9 hours');
                END IF;
            END IF;
        END IF;

    END LOOP;

    RAISE NOTICE 'Takuya: Inserted % todo completions', seq;
END $$;
