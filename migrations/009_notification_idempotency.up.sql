-- Delivery records are dedupe keys and must outlive the farm and user they refer to,
-- so the cascading foreign keys are dropped.
ALTER TABLE notification_log DROP CONSTRAINT IF EXISTS fk_notification_log_farm;
ALTER TABLE notification_log DROP CONSTRAINT IF EXISTS fk_notification_log_user;

ALTER TABLE notification_log ADD COLUMN IF NOT EXISTS notification_id UUID;
UPDATE notification_log SET notification_id = id WHERE notification_id IS NULL;
ALTER TABLE notification_log ALTER COLUMN notification_id SET NOT NULL;
ALTER TABLE notification_log ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_log_notification_id ON notification_log (notification_id);

-- One scheduled alert per farm per day. The row is reserved before publishing and marked
-- published only after the broker confirms, so reruns and second schedulers do not resend.
CREATE TABLE IF NOT EXISTS notification_outbox (
    notification_id UUID PRIMARY KEY,
    farm_id UUID NOT NULL,
    alert_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    CONSTRAINT uq_notification_outbox_farm_date UNIQUE (farm_id, alert_date)
);
