DROP INDEX IF EXISTS idx_revoked_access_tokens_expires_at;
DROP TABLE IF EXISTS revoked_access_tokens;

ALTER TABLE users
    DROP COLUMN IF EXISTS locked_until,
    DROP COLUMN IF EXISTS failed_login_count,
    DROP COLUMN IF EXISTS token_version;
