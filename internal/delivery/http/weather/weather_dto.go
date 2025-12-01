package weather

import (
	"github.com/google/uuid"
)

type CreateWeatherJSON struct {
	Metric   string  `json:"metric" validate:"required,min=3,max=50"`
	Operator string  `json:"operator" validate:"required,oneof='<' '>' '=' '<=' '>='"`
	Value    float64 `json:"value" validate:"required"`
}

type UpdateWeatherJSON struct {
	Metric    string  `json:"metric" validate:"required,min=3,max=50"`
	Operator  string  `json:"operator" validate:"required,oneof='<' '>' '=' '<=' '>='"`
	Value     float64 `json:"value" validate:"required"`
	IsEnabled bool    `json:"is_enabled"`
}

type WeatherResponse struct {
	ID        uuid.UUID `json:"id"`
	FarmID    uuid.UUID `json:"farm_id"`
	Metric    string    `json:"metric"`
	Operator  string    `json:"operator"`
	Value     float64   `json:"value"`
	IsEnabled bool      `json:"is_enabled"`
}

