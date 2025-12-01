CREATE TABLE weather_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    farm_id UUID NOT NULL,
    metric VARCHAR(50) NOT NULL,
    operator VARCHAR(10) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (farm_id) REFERENCES farms(id) ON DELETE CASCADE
);

CREATE INDEX idx_weather_alerts_farm_id ON weather_alerts(farm_id);
CREATE INDEX idx_weather_alerts_is_enabled ON weather_alerts(is_enabled);
