-- ============================================================================
-- 008_users_name_transit.sql
--
-- Stage 6 (Vault Transit) Phase 1: users.name を Vault Transit ciphertext と
-- HMAC-SHA256 (deterministic 検索用) に置き換える準備として、新しい列を追加する。
--
-- このファイルは「列の追加」だけを行う追加マイグレーション (idempotent)。
-- 既存データを encrypt して埋めるのは backend/cmd/encrypt-users-name/ の Go CLI、
-- 旧 name 列の DROP は 009_drop_users_name.sql (CLI 完走確認後に手動 apply)。
--
-- 列の意味:
--   name_enc  BYTEA NULL — Vault Transit ciphertext "vault:v1:..." の生バイト
--   name_hash BYTEA NULL — Vault Transit HMAC "vault:v1:..." の生バイト
--                         (重複確認 / 等価検索に使う、平文を露出しない)
--
-- 制約:
--   - NULL 許容で追加する。NOT NULL 化は 009 に同梱 (encrypt CLI 完走後)
--   - name_hash に UNIQUE は付けない (同名同姓を許容)
--   - 検索高速化が必要なら名前 prefix 検索は不可なので index は最小限
-- ============================================================================

BEGIN;

-- 1) ciphertext + HMAC 列 (NULL 許容で追加)
ALTER TABLE users ADD COLUMN IF NOT EXISTS name_enc  BYTEA;
ALTER TABLE users ADD COLUMN IF NOT EXISTS name_hash BYTEA;

-- 2) HMAC を index 化 (= 検索 / 重複確認用、ciphertext には index 不要)
CREATE INDEX IF NOT EXISTS idx_users_name_hash ON users (name_hash);

COMMIT;
