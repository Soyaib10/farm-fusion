package domain

import (
	"time"

	"github.com/google/uuid"
)

type Weather struct {
	ID        uuid.UUID `json:"id"`
	FarmID    uuid.UUID `json:"farm_id"`
	Metric    string    `json:"metric"` // e.g., "temperature", "humidity", "rainfall"
	Operator  string    `json:"operator"` // e.g., ">", "<", "="
	Value     float64   `json:"value"`
	IsEnabled bool      `json:"is_enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
