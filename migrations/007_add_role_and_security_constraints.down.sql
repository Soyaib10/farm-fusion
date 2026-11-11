DROP INDEX IF EXISTS idx_refresh_tokens_token_hash_unique;
DROP INDEX IF EXISTS idx_users_email_lower;

ALTER TABLE weather_alerts DROP CONSTRAINT IF EXISTS valid_weather_alert_operator;
ALTER TABLE farms DROP CONSTRAINT IF EXISTS valid_farm_coordinates;
ALTER TABLE users DROP CONSTRAINT IF EXISTS valid_user_role;
ALTER TABLE users DROP COLUMN IF EXISTS role;
