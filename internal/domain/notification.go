package domain

import (
	"time"

	"github.com/google/uuid"
)

// NotificationLog records an attempted weather alert email delivery.
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

// WeatherForecast represents 24-hour 3-hourly forecast points for a rounded location key.
type WeatherForecast struct {
	LocationKey string          `json:"location_key"`
	Latitude    float64         `json:"latitude"`
	Longitude   float64         `json:"longitude"`
	DataPoints  []ForecastPoint `json:"data_points"`
	FetchedAt   time.Time       `json:"fetched_at"`
}

// ForecastPoint represents weather metrics at a single forecast timestamp.
type ForecastPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Temperature float64   `json:"temperature"` // Celsius
	Humidity    float64   `json:"humidity"`    // Percentage
	Rainfall    float64   `json:"rainfall"`    // mm
	WindSpeed   float64   `json:"wind_speed"`  // km/h
}

// AlertRange represents a contiguous time window where a weather threshold is breached.
type AlertRange struct {
	Start         time.Time       `json:"start"`
	End           time.Time       `json:"end"`
	DurationHours int             `json:"duration_hours"`
	MinValue      float64         `json:"min_value"`
	MaxValue      float64         `json:"max_value"`
	DataPoints    []ForecastPoint `json:"data_points"`
}

// WeatherAlert groups breached time ranges for a single threshold rule.
type WeatherAlert struct {
	AlertType string       `json:"alert_type"`
	Threshold Threshold    `json:"threshold"`
	Ranges    []AlertRange `json:"ranges"`
}

// Threshold describes a comparison rule.
type Threshold struct {
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
}

// NotificationPayload is the message published to RabbitMQ for email delivery.
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

// Location holds geographic coordinates.
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ForecastSummary summarizes temperature extremes and cumulative rainfall over a forecast window.
type ForecastSummary struct {
	TempMin       float64 `json:"temp_min"`
	TempMax       float64 `json:"temp_max"`
	TotalRainfall float64 `json:"total_rainfall"`
}
