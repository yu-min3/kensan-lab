-- ============================================================================
-- Demo Seed: Todo Completions — 高橋彩 (~110件)
-- ============================================================================
-- 規律が高い。1on1振り返り: 90%, 読書: 85%, チーム日報確認: 95%, ヨガ: 80%

DO $$
DECLARE
    base_date DATE := CURRENT_DATE - 56;
    d DATE;
    day_offset INT;
    dow INT;
    week_num INT;
    seq INT := 0;
    uid UUID := 'd3333333-3333-3333-3333-333333333333';

    todo_1on1 UUID := 'd30b0001-0000-0000-0000-000000000000';
    todo_read UUID := 'd30b0002-0000-0000-0000-000000000000';
    todo_daily UUID := 'd30b0003-0000-0000-0000-000000000000';
    todo_yoga UUID := 'd30b0004-0000-0000-0000-000000000000';

BEGIN
    FOR day_offset IN 0..55 LOOP
        d := base_date + day_offset;
        dow := EXTRACT(DOW FROM d)::INT;
        week_num := day_offset / 7 + 1;

        -- 1on1振り返りメモ (Mon/Wed/Fri, 90%)
        IF dow IN (1, 3, 5) THEN
            IF day_offset % 10 <> 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_1on1, d, d + TIME '21:00' + INTERVAL '9 hours');
            END IF;
        END IF;

        -- ビジネス書読書 (daily, 85%)
        IF day_offset % 7 <> 0 THEN
            seq := seq + 1;
            INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
            VALUES (uuid_generate_v4(), uid, todo_read, d, d + TIME '03:45' + INTERVAL '9 hours');
        END IF;

        -- チーム日報確認 (Mon-Fri, 95%)
        IF dow BETWEEN 1 AND 5 THEN
            IF day_offset % 20 <> 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_daily, d, d + TIME '00:00' + INTERVAL '9 hours');
            END IF;
        END IF;

        -- ヨガ (Mon/Wed/Fri, 80%)
        IF dow IN (1, 3, 5) THEN
            IF day_offset % 5 <> 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_yoga, d, d + TIME '21:30' + INTERVAL '9 hours');
            END IF;
        END IF;

    END LOOP;

    RAISE NOTICE 'Aya: Inserted % todo completions', seq;
END $$;
