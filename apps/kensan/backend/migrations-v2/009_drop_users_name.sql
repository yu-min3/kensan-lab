-- ============================================================================
-- 009_drop_users_name.sql
--
-- Stage 6 Phase 2: 旧 plaintext `users.name` カラムを drop する。
-- これを apply する前提:
--   1. 008_users_name_transit.sql を apply 済み (name_enc / name_hash 列が存在)
--   2. backend/cmd/encrypt-users-name/ の CLI を完走させ、全 users 行で
--      name_enc / name_hash が NOT NULL になっている
--   3. user-service Pod が新しい image (PR #2) で起動し、name_enc / name_hash
--      経由で読み書きしているのを確認 (kubectl logs / 動作確認)
--
-- safety check: name_enc が NULL の行が 1 つでもあれば ERROR で中断する。
-- ============================================================================

BEGIN;

-- 1) safety: encrypt 漏れがあれば fail
DO $$
DECLARE
    missing_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO missing_count
    FROM users
    WHERE name_enc IS NULL OR name_hash IS NULL;

    IF missing_count > 0 THEN
        RAISE EXCEPTION 'cannot drop users.name: % rows still have NULL name_enc/name_hash. Run encrypt-users-name CLI first.', missing_count;
    END IF;
END$$;

-- 2) 新カラムを NOT NULL 化 (今後 INSERT で必ず埋まる)
ALTER TABLE users ALTER COLUMN name_enc  SET NOT NULL;
ALTER TABLE users ALTER COLUMN name_hash SET NOT NULL;

-- 3) 旧 name カラムを drop
ALTER TABLE users DROP COLUMN IF EXISTS name;

COMMIT;
