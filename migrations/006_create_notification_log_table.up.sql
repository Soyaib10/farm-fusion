CREATE TABLE notification_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    farm_id UUID NOT NULL,
    user_id UUID NOT NULL,
    notification_type VARCHAR(20) NOT NULL,
    alert_count INTEGER NOT NULL DEFAULT 0,
    email_sent BOOLEAN NOT NULL DEFAULT false,
    email_content TEXT,
    error_message TEXT,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT fk_notification_log_farm FOREIGN KEY (farm_id) REFERENCES farms(id) ON DELETE CASCADE,
    CONSTRAINT fk_notification_log_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT valid_notification_type CHECK (notification_type IN ('scheduled', 'immediate'))
);

CREATE INDEX idx_notification_log_farm_id ON notification_log(farm_id);
CREATE INDEX idx_notification_log_user_id ON notification_log(user_id);
CREATE INDEX idx_notification_log_sent_at ON notification_log(sent_at);
CREATE INDEX idx_notification_log_email_sent ON notification_log(email_sent);
