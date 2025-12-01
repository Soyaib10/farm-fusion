package weather

import "github.com/google/uuid"

type CreateWeatherCommand struct {
	UserID   uuid.UUID
	FarmID   uuid.UUID
	Metric   string
	Operator string
	Value    float64
}

type UpdateWeatherCommand struct {
	UserID    uuid.UUID
	FarmID    uuid.UUID
	WeatherID uuid.UUID
	Metric    string
	Operator  string
	Value     float64
	IsEnabled bool
}
