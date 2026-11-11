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

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users (LOWER(email));
CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash_unique ON refresh_tokens (token_hash);
