package domain

import (
	"time"

	"github.com/google/uuid"
)

// Allowed weather metrics and comparison operators for alert thresholds.
var (
	AllowedWeatherMetrics   = []string{"temperature", "humidity", "rainfall", "wind_speed"}
	AllowedWeatherOperators = []string{"<", "<=", ">", ">=", "="}
)

// Weather represents a user-configured weather alert rule on a farm.
type Weather struct {
	ID        uuid.UUID `json:"id"`
	FarmID    uuid.UUID `json:"farm_id"`
	Metric    string    `json:"metric"`
	Operator  string    `json:"operator"`
	Value     float64   `json:"value"`
	IsEnabled bool      `json:"is_enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
