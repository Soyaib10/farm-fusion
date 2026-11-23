-- Add RBAC role column to users and enforce domain security constraints at the database level
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'user';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'valid_user_role'
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT valid_user_role CHECK (role IN ('user', 'admin'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'valid_farm_coordinates'
    ) THEN
        ALTER TABLE farms
            ADD CONSTRAINT valid_farm_coordinates CHECK (
                latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'valid_weather_alert_operator'
    ) THEN
        ALTER TABLE weather_alerts
            ADD CONSTRAINT valid_weather_alert_operator CHECK (
                operator IN ('<', '<=', '>', '>=', '=')
            );
    END IF;
END $$;

-- Existing databases may hold rows that violate the unique indexes below. Clean them up first.

-- Duplicate refresh-token hashes are ambiguous credentials: drop every copy (affected sessions re-login).
DELETE FROM refresh_tokens
WHERE token_hash IN (
    SELECT token_hash FROM refresh_tokens GROUP BY token_hash HAVING COUNT(*) > 1
);

-- Case-variant duplicate emails: keep the oldest account, move farms and logs onto it, delete the rest.
DROP TABLE IF EXISTS pg_temp.user_email_merge;
CREATE TEMP TABLE user_email_merge AS
SELECT dup.id AS duplicate_id, keeper.id AS keeper_id
FROM users dup
JOIN LATERAL (
    SELECT u.id
    FROM users u
    WHERE LOWER(u.email) = LOWER(dup.email)
    ORDER BY u.created_at, u.id
    LIMIT 1
) keeper ON keeper.id <> dup.id;

UPDATE farms f SET user_id = m.keeper_id
FROM user_email_merge m WHERE f.user_id = m.duplicate_id;

UPDATE notification_log nl SET user_id = m.keeper_id
FROM user_email_merge m WHERE nl.user_id = m.duplicate_id;

DELETE FROM users u USING user_email_merge m WHERE u.id = m.duplicate_id;
DROP TABLE user_email_merge;

UPDATE users SET email = LOWER(email) WHERE email <> LOWER(email);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users (LOWER(email));
CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash_unique ON refresh_tokens (token_hash);
