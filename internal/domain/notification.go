package domain

import (
	"time"

	"github.com/google/uuid"
)

type NotificationLog struct {
	ID               uuid.UUID `json:"id"`
	FarmID           uuid.UUID `json:"farm_id"`
	UserID           uuid.UUID `json:"user_id"`
	NotificationType string    `json:"notification_type"` // "scheduled" or "immediate"
	AlertCount       int       `json:"alert_count"`
	EmailSent        bool      `json:"email_sent"`
	EmailContent     string    `json:"email_content,omitempty"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	SentAt           time.Time `json:"sent_at"`
}

type WeatherForecast struct {
	LocationKey string          `json:"location_key"`
	Latitude    float64         `json:"latitude"`
	Longitude   float64         `json:"longitude"`
	DataPoints  []ForecastPoint `json:"data_points"`
	FetchedAt   time.Time       `json:"fetched_at"`
}

type ForecastPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Temperature float64   `json:"temperature"` // Celsius
	Humidity    float64   `json:"humidity"`    // Percentage
	Rainfall    float64   `json:"rainfall"`    // mm
	WindSpeed   float64   `json:"wind_speed"`  // km/h
}

type AlertRange struct {
	Start         time.Time       `json:"start"`
	End           time.Time       `json:"end"`
	DurationHours int             `json:"duration_hours"`
	MinValue      float64         `json:"min_value,omitempty"`
	MaxValue      float64         `json:"max_value,omitempty"`
	DataPoints    []ForecastPoint `json:"data_points"`
}

type WeatherAlert struct {
	AlertType string       `json:"alert_type"` // "temperature", "rainfall", "humidity", "wind_speed"
	Threshold Threshold    `json:"threshold"`
	Ranges    []AlertRange `json:"ranges"`
}

type Threshold struct {
	Operator string  `json:"operator"` // "<", ">", "="
	Value    float64 `json:"value"`
}

type NotificationPayload struct {
	NotificationID  uuid.UUID       `json:"notification_id"`
	FarmID          uuid.UUID       `json:"farm_id"`
	UserID          uuid.UUID       `json:"user_id"`
	UserEmail       string          `json:"user_email"`
	FarmName        string          `json:"farm_name"`
	Location        Location        `json:"location"`
	Timestamp       time.Time       `json:"timestamp"`
	Alerts          []WeatherAlert  `json:"alerts"`
	ForecastSummary ForecastSummary `json:"forecast_summary"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type ForecastSummary struct {
	TempMin       float64 `json:"temp_min"`
	TempMax       float64 `json:"temp_max"`
	TotalRainfall float64 `json:"total_rainfall"`
}
