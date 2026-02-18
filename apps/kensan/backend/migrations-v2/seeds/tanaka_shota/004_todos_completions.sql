-- ============================================================================
-- Demo Seed: Todo Completions (~120件)
-- ============================================================================
-- 技術ニュース: 90%, Google Cloud問題集: 75% (Week5-6激減→Week7-8回復),
-- 英語: 65% (Week5-8低下), ジム: 60% (水土)

DO $$
DECLARE
    base_date DATE := CURRENT_DATE - 56;
    d DATE;
    day_offset INT;
    dow INT;
    week_num INT;
    seq INT := 0;
    uid UUID := 'dddddddd-dddd-dddd-dddd-dddddddddddd';

    -- Todo IDs
    todo_news UUID := 'dd0b0001-0000-0000-0000-000000000000';
    todo_gc UUID := 'dd0b0002-0000-0000-0000-000000000000';
    todo_eng UUID := 'dd0b0003-0000-0000-0000-000000000000';
    todo_gym UUID := 'dd0b0004-0000-0000-0000-000000000000';

BEGIN
    FOR day_offset IN 0..55 LOOP
        d := base_date + day_offset;
        dow := EXTRACT(DOW FROM d)::INT;
        week_num := day_offset / 7 + 1;

        -- ============================================================
        -- 技術ニュース読む (daily, 90% completion)
        -- ============================================================
        IF day_offset % 11 <> 0 THEN  -- skip ~9%
            seq := seq + 1;
            INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
            VALUES (uuid_generate_v4(), uid, todo_news, d, d + TIME '03:30' + INTERVAL '9 hours');
        END IF;

        -- ============================================================
        -- Google Cloud問題集 (daily, 75% overall — Week5-6 drops to ~30%)
        -- ============================================================
        IF week_num <= 4 THEN
            -- Week 1-4: ~85%
            IF day_offset % 7 <> 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_gc, d, d + TIME '12:30' + INTERVAL '9 hours');
            END IF;
        ELSIF week_num <= 6 THEN
            -- Week 5-6: ~30% (ブログ・Next.js没頭でGoogle Cloud後回し)
            IF day_offset % 3 = 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_gc, d, d + TIME '12:30' + INTERVAL '9 hours');
            END IF;
        ELSE
            -- Week 7-8: ~70% (立て直し)
            IF day_offset % 3 <> 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_gc, d, d + TIME '12:30' + INTERVAL '9 hours');
            END IF;
        END IF;

        -- ============================================================
        -- 英語リーディング (daily, 65% overall — drops in later weeks)
        -- ============================================================
        IF week_num <= 4 THEN
            -- Week 1-4: ~80% completion
            IF day_offset % 5 <> 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_eng, d, d + TIME '03:45' + INTERVAL '9 hours');
            END IF;
        ELSE
            -- Week 5-8: ~50% completion (busy with dev/blog)
            IF day_offset % 2 = 0 THEN
                seq := seq + 1;
                INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                VALUES (uuid_generate_v4(), uid, todo_eng, d, d + TIME '03:45' + INTERVAL '9 hours');
            END IF;
        END IF;

        -- ============================================================
        -- ジム (custom: 水土, 60% overall)
        -- ============================================================
        IF dow IN (3, 6) THEN
            IF week_num <= 4 THEN
                -- Week 1-4: ~70%
                IF day_offset % 3 <> 0 THEN
                    seq := seq + 1;
                    INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                    VALUES (uuid_generate_v4(), uid, todo_gym, d, d + TIME '10:00' + INTERVAL '9 hours');
                END IF;
            ELSIF week_num <= 6 THEN
                -- Week 5-6: ~50%
                IF day_offset % 2 = 0 THEN
                    seq := seq + 1;
                    INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                    VALUES (uuid_generate_v4(), uid, todo_gym, d, d + TIME '10:00' + INTERVAL '9 hours');
                END IF;
            ELSE
                -- Week 7-8: ~55%
                IF day_offset % 4 <> 0 AND dow = 6 THEN
                    seq := seq + 1;
                    INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                    VALUES (uuid_generate_v4(), uid, todo_gym, d, d + TIME '10:00' + INTERVAL '9 hours');
                ELSIF dow = 3 AND day_offset % 3 <> 0 THEN
                    seq := seq + 1;
                    INSERT INTO todo_completions (id, user_id, todo_id, completed_date, completed_at)
                    VALUES (uuid_generate_v4(), uid, todo_gym, d, d + TIME '10:00' + INTERVAL '9 hours');
                END IF;
            END IF;
        END IF;

    END LOOP;

    RAISE NOTICE 'Inserted % todo completions', seq;
END $$;
