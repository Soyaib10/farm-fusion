package notification

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Soyaib10/farm-fusion/internal/domain"
	"github.com/google/uuid"
)

// FarmReader lists farms for scheduled weather evaluation.
type FarmReader interface {
	ListAll(ctx context.Context) ([]*domain.Farm, error)
}

// AlertReader lists configured weather alert thresholds for a farm.
type AlertReader interface {
	ListByFarmID(ctx context.Context, farmID uuid.UUID) ([]*domain.Weather, error)
}

// UserReader resolves the farm owner's contact details.
type UserReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

// ForecastProvider fetches 24-hour weather forecasts for a coordinate pair.
type ForecastProvider interface {
	FetchForecast(ctx context.Context, lat, lon float64, locationKey string) (*domain.WeatherForecast, error)
}

// Publisher enqueues weather notification payloads for asynchronous delivery.
type Publisher interface {
	Publish(ctx context.Context, payload *domain.NotificationPayload) error
}

// RunResult summarizes a scheduled weather evaluation pass.
type RunResult struct {
	FarmsScanned           int      `json:"farms_scanned"`
	ForecastsFetched       int      `json:"forecasts_fetched"`
	NotificationsPublished int      `json:"notifications_published"`
	Errors                 []string `json:"errors,omitempty"`
}

// Scheduler evaluates weather forecasts against farm alert thresholds and publishes notifications.
type Scheduler struct {
	farms     FarmReader
	alerts    AlertReader
	users     UserReader
	forecasts ForecastProvider
	publisher Publisher
	now       func() time.Time
}

// NewScheduler constructs a weather notification Scheduler.
func NewScheduler(
	farms FarmReader,
	alerts AlertReader,
	users UserReader,
	forecasts ForecastProvider,
	publisher Publisher,
) *Scheduler {
	return &Scheduler{
		farms:     farms,
		alerts:    alerts,
		users:     users,
		forecasts: forecasts,
		publisher: publisher,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// RunScheduled executes one full weather check across all farms.
func (s *Scheduler) RunScheduled(ctx context.Context) (*RunResult, error) {
	farms, err := s.farms.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list farms: %w", err)
	}

	result := &RunResult{}
	forecastByLocation := make(map[string]*domain.WeatherForecast)

	for _, farm := range farms {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		result.FarmsScanned++

		locationKey := farm.LocationKey
		if locationKey == "" {
			locationKey = domain.NewLocationKey(farm.Latitude, farm.Longitude)
		}

		forecast, ok := forecastByLocation[locationKey]
		if !ok {
			forecast, err = s.forecasts.FetchForecast(ctx, farm.Latitude, farm.Longitude, locationKey)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("farm %s forecast: %v", farm.ID, err))
				continue
			}
			forecastByLocation[locationKey] = forecast
			result.ForecastsFetched++
		}

		farmAlerts, err := s.alerts.ListByFarmID(ctx, farm.ID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("farm %s alerts: %v", farm.ID, err))
			continue
		}

		weatherAlerts := buildWeatherAlerts(farmAlerts, forecast)
		if len(weatherAlerts) == 0 {
			continue
		}

		owner, err := s.users.GetByID(ctx, farm.UserID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("farm %s user %s: %v", farm.ID, farm.UserID, err))
			continue
		}

		payload := &domain.NotificationPayload{
			NotificationID: uuid.New(),
			FarmID:         farm.ID,
			UserID:         farm.UserID,
			UserEmail:      owner.Email,
			FarmName:       farm.Name,
			Location: domain.Location{
				Latitude:  farm.Latitude,
				Longitude: farm.Longitude,
			},
			Timestamp:       s.now(),
			Alerts:          weatherAlerts,
			ForecastSummary: summarizeForecast(forecast),
		}

		if err := s.publisher.Publish(ctx, payload); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("farm %s publish: %v", farm.ID, err))
			continue
		}
		result.NotificationsPublished++
	}

	return result, nil
}

func buildWeatherAlerts(thresholds []*domain.Weather, forecast *domain.WeatherForecast) []domain.WeatherAlert {
	alerts := make([]domain.WeatherAlert, 0, len(thresholds))
	for _, threshold := range thresholds {
		if !threshold.IsEnabled {
			continue
		}

		ranges := matchingRanges(threshold, forecast.DataPoints)
		if len(ranges) == 0 {
			continue
		}

		alerts = append(alerts, domain.WeatherAlert{
			AlertType: normalizeMetric(threshold.Metric),
			Threshold: domain.Threshold{
				Operator: threshold.Operator,
				Value:    threshold.Value,
			},
			Ranges: ranges,
		})
	}
	return alerts
}

func matchingRanges(threshold *domain.Weather, points []domain.ForecastPoint) []domain.AlertRange {
	var (
		ranges  []domain.AlertRange
		current []domain.ForecastPoint
	)

	flush := func() {
		if len(current) == 0 {
			return
		}

		minVal := math.Inf(1)
		maxVal := math.Inf(-1)
		for _, p := range current {
			val, _ := metricValue(threshold.Metric, p)
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
		}

		start := current[0].Timestamp
		end := current[len(current)-1].Timestamp.Add(3 * time.Hour)
		ranges = append(ranges, domain.AlertRange{
			Start:         start,
			End:           end,
			DurationHours: int(end.Sub(start).Hours()),
			MinValue:      minVal,
			MaxValue:      maxVal,
			DataPoints:    append([]domain.ForecastPoint(nil), current...),
		})
		current = nil
	}

	for _, point := range points {
		val, ok := metricValue(threshold.Metric, point)
		if !ok {
			flush()
			continue
		}
		if compare(val, threshold.Operator, threshold.Value) {
			current = append(current, point)
			continue
		}
		flush()
	}
	flush()

	return ranges
}

func metricValue(metric string, point domain.ForecastPoint) (float64, bool) {
	switch normalizeMetric(metric) {
	case "temperature", "temp":
		return point.Temperature, true
	case "humidity":
		return point.Humidity, true
	case "rainfall", "rain":
		return point.Rainfall, true
	case "wind_speed", "wind":
		return point.WindSpeed, true
	default:
		return 0, false
	}
}

func normalizeMetric(metric string) string {
	return strings.ToLower(strings.TrimSpace(metric))
}

func compare(actual float64, operator string, expected float64) bool {
	switch strings.TrimSpace(operator) {
	case "<":
		return actual < expected
	case "<=":
		return actual <= expected
	case ">":
		return actual > expected
	case ">=":
		return actual >= expected
	case "=", "==":
		return actual == expected
	default:
		return false
	}
}

func summarizeForecast(forecast *domain.WeatherForecast) domain.ForecastSummary {
	if len(forecast.DataPoints) == 0 {
		return domain.ForecastSummary{}
	}

	summary := domain.ForecastSummary{
		TempMin: forecast.DataPoints[0].Temperature,
		TempMax: forecast.DataPoints[0].Temperature,
	}
	for _, point := range forecast.DataPoints {
		if point.Temperature < summary.TempMin {
			summary.TempMin = point.Temperature
		}
		if point.Temperature > summary.TempMax {
			summary.TempMax = point.Temperature
		}
		summary.TotalRainfall += point.Rainfall
	}
	return summary
}
