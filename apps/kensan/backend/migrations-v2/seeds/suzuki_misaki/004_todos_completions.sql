-- ============================================================================
-- Demo Seed: Todo Completions — 鈴木美咲 (~100件)
-- ============================================================================
-- Zenn/Qiita記事読む: 85%, TypeScript型パズル: 70%, ストレッチ: 60%, ポートフォリオ更新: 50%

DO $$
DECLARE
    base_date DATE := CURRENT_DATE - 56;
    d DATE;
    day_offset INT;
    dow INT;
    week_num INT;
    seq INT := 0;
    uid UUID := 'd1111111-1111-1111-1111-111111111111';

    todo_read UUID := 'd10b0001-0000-0000-0000-000000000000';
    todo_ts UUID := 'd10b0002-0000-0000-0000-000000000000';
    todo_stretch UUID := 'd10b0003-0000-0000-0000-000000000000';
    todo_portfolio UUID := 'd10b0004-0000-0000-0000-000000000000';

BEGIN
    FOR day_offset IN 0..55 LOOP
        d := base_date + day_offset;
        dow := EXTRACT(DOW FROM d)::INT;
        week_num := day_offset / 7 + 1;

        -- Zenn/Qiita記事読む (daily, 85%)
        IF day_offset % 7 <> 0 THEN
            seq := seq + 1;
            INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
            VALUES (uuid_generate_v4(), uid, todo_read, d, d + TIME '03:30' + INTERVAL '9 hours');
        END IF;

        -- TypeScript型パズル (daily, 70%)
        IF week_num <= 4 THEN
            IF day_offset % 3 <> 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_ts, d, d + TIME '14:00' + INTERVAL '9 hours');
            END IF;
        ELSE
            -- Week 5-8: 副業で忙しくなり50%に低下
            IF day_offset % 2 = 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_ts, d, d + TIME '14:00' + INTERVAL '9 hours');
            END IF;
        END IF;

        -- ストレッチ (daily, 60%)
        IF day_offset % 5 < 3 THEN
            seq := seq + 1;
            INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
            VALUES (uuid_generate_v4(), uid, todo_stretch, d, d + TIME '13:00' + INTERVAL '9 hours');
        END IF;

        -- ポートフォリオ更新 (Saturday, 50%)
        IF dow = 6 AND day_offset % 4 < 2 THEN
            seq := seq + 1;
            INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
            VALUES (uuid_generate_v4(), uid, todo_portfolio, d, d + TIME '04:00' + INTERVAL '9 hours');
        END IF;

    END LOOP;

    RAISE NOTICE 'Misaki: Inserted % todo completions', seq;
END $$;
