DROP TABLE IF EXISTS notification_outbox;

DROP INDEX IF EXISTS idx_notification_log_notification_id;
ALTER TABLE notification_log DROP COLUMN IF EXISTS updated_at;
ALTER TABLE notification_log DROP COLUMN IF EXISTS notification_id;

DELETE FROM notification_log nl WHERE NOT EXISTS (SELECT 1 FROM farms f WHERE f.id = nl.farm_id);
DELETE FROM notification_log nl WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = nl.user_id);

ALTER TABLE notification_log
    ADD CONSTRAINT fk_notification_log_farm FOREIGN KEY (farm_id) REFERENCES farms(id) ON DELETE CASCADE;
ALTER TABLE notification_log
    ADD CONSTRAINT fk_notification_log_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
